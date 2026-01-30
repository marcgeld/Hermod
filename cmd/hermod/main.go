package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/marcgeld/Hermod/internal/config"
	"github.com/marcgeld/Hermod/internal/logger"
	"github.com/marcgeld/Hermod/internal/mqtt"
	"github.com/marcgeld/Hermod/internal/router"
	"github.com/marcgeld/Hermod/internal/schema"
	"github.com/marcgeld/Hermod/internal/storage"
)

var (
	version = "dev"
	commit  = "none"
	date    = "unknown"
)

func main() {
	dryRun := false
	sqlFlag := false
	configPath := flag.String("config", "config.toml", "Path to configuration file")
	versionFlag := flag.Bool("version", false, "Print version information")
	flag.BoolVar(&dryRun, "dry-run", false, "Don't execute SQL statements, just log them")
	flag.BoolVar(&sqlFlag, "sql", false, "Generate SQL schema from Lua scripts and exit")
	logLvl := flag.String("log", "", "Log level DEBUG, INFO, or ERROR (overrides config file)")
	flag.Parse()

	if *versionFlag {
		log.Printf("Hermod version %s (commit: %s, built: %s)\n", version, commit, date)
		os.Exit(0)
	}

	// Load configuration
	cfg, err := config.Load(*configPath)
	if err != nil {
		log.Fatalf("Failed to load configuration: %v", err)
	}

	// Handle -sql flag: generate schema and exit
	if sqlFlag {
		if err := generateSQL(cfg); err != nil {
			log.Fatalf("Failed to generate SQL: %v", err)
		}
		return
	}

	log.Printf("Starting Hermod %s...", version)

	// Initialize logger
	logLevel := logger.INFO
	if logLvl != nil && *logLvl != "" {
		logLevel = logger.ParseLevel(*logLvl)
	} else if cfg.Logging.Level != "" {
		logLevel = logger.ParseLevel(cfg.Logging.Level)
	}
	appLogger := logger.New(logLevel)
	appLogger.Infof("Log level set to: %s", cfg.Logging.Level)

	ctx := context.Background()

	// Initialize storage
	storageCfg := storage.Config{
		ConnectionString: cfg.Database.ConnectionString(),
		TableName:        cfg.Pipeline.TableName,
		DryRun:           dryRun,
		Logger:           appLogger,
	}
	store, err := storage.New(ctx, storageCfg)
	if err != nil {
		log.Fatalf("Failed to initialize storage: %v", err)
	}
	defer store.Close()
	if dryRun {
		appLogger.Info("Running in dry-run mode - SQL will be logged instead of executed")
	} else {
		appLogger.Info("Storage initialized successfully")
	}

	// Build routes from configuration
	routes := buildRoutes(cfg)

	// Initialize router
	r, err := router.New(ctx, routes, store, appLogger)
	if err != nil {
		log.Fatalf("Failed to initialize router: %v", err)
	}
	defer r.Close()
	appLogger.Info("Router initialized successfully")

	// Initialize MQTT client
	mqttCfg := mqtt.Config{
		Broker:   cfg.MQTT.Broker,
		ClientID: cfg.MQTT.ClientID,
		Username: cfg.MQTT.Username,
		Password: cfg.MQTT.Password,
		QoS:      cfg.MQTT.QoS,
		Logger:   appLogger,
	}
	client, err := mqtt.New(mqttCfg)
	if err != nil {
		log.Fatalf("Failed to initialize MQTT client: %v", err)
	}
	defer client.Disconnect()

	// Subscribe to topics using router
	// If routes are configured, subscribe to each route's filter
	// Otherwise, fall back to legacy topics from config
	if len(routes) > 0 {
		for _, route := range routes {
			err := client.Subscribe(route.Filter, cfg.MQTT.QoS, func(topic string, payload []byte) error {
				msg := router.Message{
					Topic:   topic,
					Payload: payload,
					QoS:     cfg.MQTT.QoS,
					Retain:  false, // MQTT callback doesn't provide retain flag easily
					Time:    time.Now().UTC(),
				}
				return r.Dispatch(msg)
			})
			if err != nil {
				log.Fatalf("Failed to subscribe to topic %s: %v", route.Filter, err)
			}
		}
	} else {
		// Legacy mode: subscribe to topics from config
		for _, topic := range cfg.MQTT.Topics {
			err := client.Subscribe(topic, cfg.MQTT.QoS, func(topic string, payload []byte) error {
				msg := router.Message{
					Topic:   topic,
					Payload: payload,
					QoS:     cfg.MQTT.QoS,
					Retain:  false,
					Time:    time.Now().UTC(),
				}
				return r.Dispatch(msg)
			})
			if err != nil {
				log.Fatalf("Failed to subscribe to topic %s: %v", topic, err)
			}
		}
	}

	appLogger.Info("Hermod is running. Press Ctrl+C to exit.")

	// Wait for interrupt signal
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)
	<-sigChan

	appLogger.Info("Shutting down Hermod...")
}

// buildRoutes creates router.Route from config
func buildRoutes(cfg *config.Config) []router.Route {
	if len(cfg.Routes) > 0 {
		// Use new routes configuration
		routes := make([]router.Route, len(cfg.Routes))
		for i, rc := range cfg.Routes {
			routes[i] = router.Route{
				Filter:    rc.Filter,
				Script:    rc.Script,
				Workers:   rc.Workers,
				QueueSize: rc.QueueSize,
				Table:     rc.Table,
			}
		}
		return routes
	}

	// Backward compatibility: create a single route from legacy config
	if cfg.Pipeline.LuaScript != "" || len(cfg.MQTT.Topics) > 0 {
		// If only one topic, use it as filter
		filter := "#" // Default: match all
		if len(cfg.MQTT.Topics) == 1 {
			filter = cfg.MQTT.Topics[0]
		}
		return []router.Route{
			{
				Filter:    filter,
				Script:    cfg.Pipeline.LuaScript,
				Workers:   1,
				QueueSize: 100,
				Table:     cfg.Pipeline.TableName,
			},
		}
	}

	// No routes configured, return empty (all messages go to passthrough)
	return []router.Route{}
}

// generateSQL loads all Lua scripts and generates SQL schema
func generateSQL(cfg *config.Config) error {
	var schemas []*schema.Schema

	// Load schema from each route's Lua script
	for _, route := range cfg.Routes {
		if route.Script != "" {
			s, err := schema.LoadFromLuaScript(route.Script)
			if err != nil {
				return fmt.Errorf("failed to load schema from %s: %w", route.Script, err)
			}
			schemas = append(schemas, s)
		}
	}

	// Legacy: also check pipeline.lua_script
	if cfg.Pipeline.LuaScript != "" {
		s, err := schema.LoadFromLuaScript(cfg.Pipeline.LuaScript)
		if err != nil {
			return fmt.Errorf("failed to load schema from %s: %w", cfg.Pipeline.LuaScript, err)
		}
		schemas = append(schemas, s)
	}

	// Merge all schemas
	merged := schema.Merge(schemas...)

	// Generate SQL
	sql := merged.GenerateSQL()
	if sql == "" {
		fmt.Println("-- No schemas defined in Lua scripts")
		return nil
	}

	fmt.Println(sql)
	return nil
}

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

	"github.com/marcgeld/hermod/internal/config"
	"github.com/marcgeld/hermod/internal/logger"
	"github.com/marcgeld/hermod/internal/mqtt"
	"github.com/marcgeld/hermod/internal/router"
	"github.com/marcgeld/hermod/internal/schema"
	"github.com/marcgeld/hermod/internal/storage"
)

var (
	version = "dev"
	commit  = "none"
	date    = "unknown"
)

func main() {
	// Parse CLI flags
	configPath, dryRun, sqlFlag, logLvl, versionFlag, scriptDir := parseFlags()

	if versionFlag {
		log.Printf("hermod version %s (commit: %s, built: %s)\n", version, commit, date)
		os.Exit(0)
	}

	// Load configuration
	cfg, err := config.Load(configPath)
	if err != nil {
		log.Fatalf("Failed to load configuration: %v", err)
	}

	// Handle -sql flag: generate schema and exit
	if sqlFlag {
		if err := generateSQL(cfg, configPath, scriptDir); err != nil {
			log.Fatalf("Failed to generate SQL: %v", err)
		}
		return
	}

	log.Printf("Starting hermod %s...", version)

	// Initialize logger
	appLogger := initLogger(cfg, logLvl)

	ctx := context.Background()

	// Initialize storage
	store := initStorageOrExit(ctx, cfg, dryRun, appLogger)
	defer store.Close()

	// Build routes and initialize router
	routes := buildRoutes(cfg, configPath, scriptDir)
	r := initRouterOrExit(ctx, routes, store, appLogger)
	defer r.Close()

	// Initialize MQTT client
	client := initMQTTOrExit(cfg, appLogger)
	defer client.Disconnect()

	// Subscribe to topics using router
	subscribeTopicsOrExit(client, routes, cfg, r)

	appLogger.Info("hermod is running. Press Ctrl+C to exit.")

	// Wait for interrupt signal
	waitForSignal(appLogger)
}

// parseFlags collects CLI flags and returns parsed values
func parseFlags() (configPath string, dryRun bool, sqlFlag bool, logLvl string, versionFlag bool, scriptDir string) {
	configPathPtr := flag.String("config", "config.toml", "Path to configuration file")
	versionPtr := flag.Bool("version", false, "Print version information")
	flag.BoolVar(&dryRun, "dry-run", false, "Don't execute SQL statements, just log them")
	flag.BoolVar(&sqlFlag, "sql", false, "Generate SQL schema from Lua scripts and exit")
	logLvlPtr := flag.String("log", "", "Log level DEBUG, INFO, or ERROR (overrides config file)")
	scriptDirPtr := flag.String("scriptdir", "", "Optional directory to look for Lua scripts before the config directory")
	flag.Parse()
	return *configPathPtr, dryRun, sqlFlag, *logLvlPtr, *versionPtr, *scriptDirPtr
}

// initLogger initializes and returns the application logger
func initLogger(cfg *config.Config, overrideLevel string) *logger.Logger {
	logLevel := logger.INFO
	if overrideLevel != "" {
		logLevel = logger.ParseLevel(overrideLevel)
	} else if cfg.Logging.Level != "" {
		logLevel = logger.ParseLevel(cfg.Logging.Level)
	}
	appLogger := logger.New(logLevel)
	appLogger.Infof("Log level set to: %s", cfg.Logging.Level)
	return appLogger
}

// initStorageOrExit initializes storage and exits on failure
func initStorageOrExit(ctx context.Context, cfg *config.Config, dryRun bool, appLogger *logger.Logger) *storage.Storage {
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
	if dryRun {
		appLogger.Info("Running in dry-run mode - SQL will be logged instead of executed")
	} else {
		appLogger.Info("Storage initialized successfully")
	}
	return store
}

// initRouterOrExit initializes the router and exits on failure
func initRouterOrExit(ctx context.Context, routes []router.Route, store router.Storage, appLogger *logger.Logger) *router.Router {
	r, err := router.New(ctx, routes, store, appLogger)
	if err != nil {
		log.Fatalf("Failed to initialize router: %v", err)
	}
	appLogger.Info("Router initialized successfully")
	return r
}

// initMQTTOrExit initializes the MQTT client and exits on failure
func initMQTTOrExit(cfg *config.Config, appLogger *logger.Logger) *mqtt.Client {
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
	return client
}

// subscribeTopicsOrExit subscribes to topics and exits on failure
func subscribeTopicsOrExit(client *mqtt.Client, routes []router.Route, cfg *config.Config, r *router.Router) {
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
		return
	}

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

// waitForSignal blocks until an interrupt/termination signal is received
func waitForSignal(appLogger *logger.Logger) {
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)
	<-sigChan

	appLogger.Info("Shutting down hermod...")
}

// buildRoutes creates router.Route from config; resolves script paths using scriptDir and configPath
func buildRoutes(cfg *config.Config, configPath string, scriptDir string) []router.Route {
	if len(cfg.Routes) > 0 {
		// Use new routes configuration
		routes := make([]router.Route, len(cfg.Routes))
		for i, rc := range cfg.Routes {
			resolvedScript, _ := config.ResolveScriptPath(rc.Script, configPath, scriptDir)
			routes[i] = router.Route{
				Filter:    rc.Filter,
				Script:    resolvedScript,
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
		resolvedScript, _ := config.ResolveScriptPath(cfg.Pipeline.LuaScript, configPath, scriptDir)
		return []router.Route{
			{
				Filter:    filter,
				Script:    resolvedScript,
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
func generateSQL(cfg *config.Config, configPath string, scriptDir string) error {
	var schemas []*schema.Schema

	// Load schema from each route's Lua script
	for _, route := range cfg.Routes {
		if route.Script != "" {
			resolved, err := config.ResolveScriptPath(route.Script, configPath, scriptDir)
			if err != nil {
				return fmt.Errorf("failed to resolve script %s: %w", route.Script, err)
			}
			s, err := schema.LoadFromLuaScript(resolved)
			if err != nil {
				return fmt.Errorf("failed to load schema from %s: %w", resolved, err)
			}
			schemas = append(schemas, s)
		}
	}

	// Legacy: also check pipeline.lua_script
	if cfg.Pipeline.LuaScript != "" {
		resolved, err := config.ResolveScriptPath(cfg.Pipeline.LuaScript, configPath, scriptDir)
		if err != nil {
			return fmt.Errorf("failed to resolve script %s: %w", cfg.Pipeline.LuaScript, err)
		}
		s, err := schema.LoadFromLuaScript(resolved)
		if err != nil {
			return fmt.Errorf("failed to load schema from %s: %w", resolved, err)
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

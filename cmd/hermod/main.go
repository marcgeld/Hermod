package main

import (
	"context"
	"flag"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/marcgeld/Hermod/internal/config"
	"github.com/marcgeld/Hermod/internal/logger"
	"github.com/marcgeld/Hermod/internal/lua"
	"github.com/marcgeld/Hermod/internal/mqtt"
	"github.com/marcgeld/Hermod/internal/pipeline"
	"github.com/marcgeld/Hermod/internal/storage"
)

var (
	version = "dev"
	commit  = "none"
	date    = "unknown"
)

func main() {
	configPath := flag.String("config", "config.toml", "Path to configuration file")
	versionFlag := flag.Bool("version", false, "Print version information")
	flag.Parse()

	if *versionFlag {
		log.Printf("Hermod version %s (commit: %s, built: %s)\n", version, commit, date)
		os.Exit(0)
	}

	log.Printf("Starting Hermod %s...", version)

	// Load configuration
	cfg, err := config.Load(*configPath)
	if err != nil {
		log.Fatalf("Failed to load configuration: %v", err)
	}

	// Initialize logger
	logLevel := logger.INFO
	if cfg.Logging.Level != "" {
		logLevel = logger.ParseLevel(cfg.Logging.Level)
	}
	appLogger := logger.New(logLevel)
	appLogger.Infof("Log level set to: %s", cfg.Logging.Level)

	ctx := context.Background()

	// Initialize storage
	storageCfg := storage.Config{
		ConnectionString: cfg.Database.ConnectionString(),
		TableName:        cfg.Pipeline.TableName,
		DryRun:           cfg.Logging.DryRun,
		Logger:           appLogger,
	}
	store, err := storage.New(ctx, storageCfg)
	if err != nil {
		log.Fatalf("Failed to initialize storage: %v", err)
	}
	defer store.Close()
	if cfg.Logging.DryRun {
		appLogger.Info("Running in dry-run mode - SQL will be logged instead of executed")
	} else {
		appLogger.Info("Storage initialized successfully")
	}

	// Initialize Lua transformer if script is provided
	var transformer *lua.Transformer
	if cfg.Pipeline.LuaScript != "" {
		transformer, err = lua.New(cfg.Pipeline.LuaScript)
		if err != nil {
			log.Fatalf("Failed to initialize Lua transformer: %v", err)
		}
		defer transformer.Close()
		appLogger.Info("Lua transformer initialized successfully")
	}

	// Initialize pipeline
	pipe := pipeline.New(transformer, store, appLogger)

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

	// Subscribe to topics
	for _, topic := range cfg.MQTT.Topics {
		err := client.Subscribe(topic, cfg.MQTT.QoS, func(topic string, payload []byte) error {
			return pipe.Process(ctx, topic, payload)
		})
		if err != nil {
			log.Fatalf("Failed to subscribe to topic %s: %v", topic, err)
		}
	}

	appLogger.Info("Hermod is running. Press Ctrl+C to exit.")

	// Wait for interrupt signal
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)
	<-sigChan

	appLogger.Info("Shutting down Hermod...")
}

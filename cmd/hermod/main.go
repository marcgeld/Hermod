package main

import (
	"context"
	"flag"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/marcgeld/Hermod/internal/config"
	"github.com/marcgeld/Hermod/internal/lua"
	"github.com/marcgeld/Hermod/internal/mqtt"
	"github.com/marcgeld/Hermod/internal/pipeline"
	"github.com/marcgeld/Hermod/internal/storage"
)

func main() {
	configPath := flag.String("config", "config.toml", "Path to configuration file")
	flag.Parse()

	log.Println("Starting Hermod...")

	// Load configuration
	cfg, err := config.Load(*configPath)
	if err != nil {
		log.Fatalf("Failed to load configuration: %v", err)
	}

	ctx := context.Background()

	// Initialize storage
	storageCfg := storage.Config{
		ConnectionString: cfg.Database.ConnectionString(),
		TableName:        cfg.Pipeline.TableName,
	}
	store, err := storage.New(ctx, storageCfg)
	if err != nil {
		log.Fatalf("Failed to initialize storage: %v", err)
	}
	defer store.Close()
	log.Println("Storage initialized successfully")

	// Initialize Lua transformer if script is provided
	var transformer *lua.Transformer
	if cfg.Pipeline.LuaScript != "" {
		transformer, err = lua.New(cfg.Pipeline.LuaScript)
		if err != nil {
			log.Fatalf("Failed to initialize Lua transformer: %v", err)
		}
		defer transformer.Close()
		log.Println("Lua transformer initialized successfully")
	}

	// Initialize pipeline
	pipe := pipeline.New(transformer, store)

	// Initialize MQTT client
	mqttCfg := mqtt.Config{
		Broker:   cfg.MQTT.Broker,
		ClientID: cfg.MQTT.ClientID,
		Username: cfg.MQTT.Username,
		Password: cfg.MQTT.Password,
		QoS:      cfg.MQTT.QoS,
	}
	client, err := mqtt.New(mqttCfg)
	if err != nil {
		log.Fatalf("Failed to initialize MQTT client: %v", err)
	}
	defer client.Disconnect(ctx)

	// Subscribe to topics
	for _, topic := range cfg.MQTT.Topics {
		err := client.Subscribe(topic, cfg.MQTT.QoS, func(topic string, payload []byte) error {
			return pipe.Process(ctx, topic, payload)
		})
		if err != nil {
			log.Fatalf("Failed to subscribe to topic %s: %v", topic, err)
		}
	}

	log.Println("Hermod is running. Press Ctrl+C to exit.")

	// Wait for interrupt signal
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)
	<-sigChan

	log.Println("Shutting down Hermod...")
}

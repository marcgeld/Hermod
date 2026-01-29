package mqtt

import (
	"fmt"
	"time"

	mqtt "github.com/eclipse/paho.mqtt.golang"
	"github.com/marcgeld/Hermod/internal/logger"
)

// Client represents an MQTT client wrapper
type Client struct {
	client   mqtt.Client
	handlers map[string]MessageHandler
	logger   *logger.Logger
}

// MessageHandler is a function that processes incoming MQTT messages
type MessageHandler func(topic string, payload []byte) error

// Config holds MQTT client configuration
type Config struct {
	Broker   string
	ClientID string
	Username string
	Password string
	QoS      byte
	Logger   *logger.Logger
}

// New creates a new MQTT client
func New(cfg Config) (*Client, error) {
	log := cfg.Logger
	if log == nil {
		log = logger.New(logger.INFO)
	}

	opts := mqtt.NewClientOptions()
	opts.AddBroker(cfg.Broker)
	opts.SetClientID(cfg.ClientID)
	opts.SetUsername(cfg.Username)
	opts.SetPassword(cfg.Password)
	opts.SetCleanSession(true)
	opts.SetAutoReconnect(true)
	opts.SetConnectTimeout(10 * time.Second)
	opts.SetKeepAlive(60 * time.Second)

	opts.OnConnect = func(c mqtt.Client) {
		log.Info("Connected to MQTT broker")
	}
	opts.OnConnectionLost = func(c mqtt.Client, err error) {
		log.Errorf("MQTT connection lost: %v", err)
	}

	client := mqtt.NewClient(opts)
	if token := client.Connect(); token.Wait() && token.Error() != nil {
		return nil, fmt.Errorf("failed to connect to MQTT broker: %w", token.Error())
	}

	return &Client{
		client:   client,
		handlers: make(map[string]MessageHandler),
		logger:   log,
	}, nil
}

// Subscribe subscribes to an MQTT topic with a handler
func (c *Client) Subscribe(topic string, qos byte, handler MessageHandler) error {
	c.handlers[topic] = handler

	token := c.client.Subscribe(topic, qos, func(client mqtt.Client, msg mqtt.Message) {
		if h, ok := c.handlers[msg.Topic()]; ok {
			if err := h(msg.Topic(), msg.Payload()); err != nil {
				c.logger.Errorf("Error processing message from topic %s: %v", msg.Topic(), err)
			}
		}
	})

	token.Wait()
	if token.Error() != nil {
		return fmt.Errorf("failed to subscribe to topic %s: %w", topic, token.Error())
	}

	c.logger.Infof("Subscribed to topic: %s", topic)
	return nil
}

// Disconnect disconnects from the MQTT broker
func (c *Client) Disconnect() {
	c.client.Disconnect(250)
	c.logger.Info("Disconnected from MQTT broker")
}

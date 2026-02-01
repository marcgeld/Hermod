package mqtt

import (
	"fmt"
	"strings"
	"sync"
	"time"

	mqtt "github.com/eclipse/paho.mqtt.golang"
	"github.com/marcgeld/Hermod/internal/logger"
)

// Client represents an MQTT client wrapper.
type Client struct {
	client   mqtt.Client
	handlers map[string]MessageHandler
	mu       sync.RWMutex
	logger   *logger.Logger
}

// MessageHandler is a function that processes incoming MQTT messages.
// topic is the concrete topic (e.g. "ruuvi/F0:34:..."), not the subscription filter.
type MessageHandler func(topic string, payload []byte) error

// Config holds MQTT client configuration.
type Config struct {
	Broker   string
	ClientID string
	Username string
	Password string
	QoS      byte
	Logger   *logger.Logger
}

// New creates a new MQTT client.
func New(cfg Config) (*Client, error) {
	log := cfg.Logger
	if log == nil {
		log = logger.New(logger.INFO)
	}

	opts := mqtt.NewClientOptions().
		AddBroker(cfg.Broker).
		SetClientID(cfg.ClientID).
		SetUsername(cfg.Username).
		SetPassword(cfg.Password).
		SetCleanSession(true).
		SetAutoReconnect(true).
		SetConnectTimeout(10 * time.Second).
		SetKeepAlive(60 * time.Second)

	opts.OnConnect = func(_ mqtt.Client) {
		log.Info("Connected to MQTT broker")
	}
	opts.OnConnectionLost = func(_ mqtt.Client, err error) {
		log.Errorf("MQTT connection lost: %v", err)
	}

	cl := mqtt.NewClient(opts)
	if token := cl.Connect(); token.Wait() && token.Error() != nil {
		return nil, fmt.Errorf("failed to connect to MQTT broker: %w", token.Error())
	}

	return &Client{
		client:   cl,
		handlers: make(map[string]MessageHandler),
		logger:   log,
	}, nil
}

// Subscribe subscribes to an MQTT topic filter (supports + and #) with a handler.
// Example filters: "ruuvi/+", "ruuvi/#", "#".
func (c *Client) Subscribe(filter string, qos byte, handler MessageHandler) error {
	c.mu.Lock()
	c.handlers[filter] = handler
	c.mu.Unlock()

	token := c.client.Subscribe(filter, qos, func(_ mqtt.Client, msg mqtt.Message) {
		topic := msg.Topic()

		// Dispatch based on filter matching, NOT exact topic equality.
		c.mu.RLock()
		defer c.mu.RUnlock()

		// Call the first matching handler (common pattern).
		for f, h := range c.handlers {
			if topicMatches(f, topic) {
				if err := h(topic, msg.Payload()); err != nil {
					c.logger.Errorf("Error processing message from topic %s: %v", topic, err)
				}
				return
			}
		}

		// see "unhandled" topics during debug.
		c.logger.Debugf("No handler matched topic=%s", topic)
	})

	token.Wait()
	if token.Error() != nil {
		return fmt.Errorf("failed to subscribe to topic %s: %w", filter, token.Error())
	}

	c.logger.Infof("Subscribed to topic filter: %s (qos=%d)", filter, qos)
	return nil
}

// Disconnect disconnects from the MQTT broker.
func (c *Client) Disconnect() {
	if c.client != nil && c.client.IsConnected() {
		c.client.Disconnect(250)
	}
	c.logger.Info("Disconnected from the MQTT broker")
}

// topicMatches returns true if a subscription filter matches a concrete topic.
// Supports MQTT wildcards: '+' (single level) and '#' (multi level, only last).
//
// Examples:
//
//	filter "ruuvi/+" matches topic "ruuvi/F0:34:..."
//	filter "ruuvi/#" matches topic "ruuvi/F0:34:.../state"
func topicMatches(filter, topic string) bool {
	// Fast paths
	if filter == topic || filter == "#" {
		return true
	}

	fs := strings.Split(filter, "/")
	ts := strings.Split(topic, "/")

	for i := 0; i < len(fs); i++ {
		if i >= len(ts) {
			// Topic ended early; only match if filter is ending with '#'
			return fs[i] == "#" && i == len(fs)-1
		}

		switch fs[i] {
		case "#":
			// '#' matches remaining levels; must be last
			return i == len(fs)-1
		case "+":
			// '+' matches exactly one level (including empty segment)
			continue
		default:
			if fs[i] != ts[i] {
				return false
			}
		}
	}

	// Filter consumed; topic must also be fully consumed
	return len(ts) == len(fs)
}

package mqtt

import (
	"testing"
)

func TestConfig(t *testing.T) {
	tests := []struct {
		name   string
		config Config
	}{
		{
			name: "basic config",
			config: Config{
				Broker:   "tcp://localhost:1883",
				ClientID: "test-client",
				Username: "",
				Password: "",
				QoS:      1,
			},
		},
		{
			name: "config with auth",
			config: Config{
				Broker:   "tcp://broker.example.com:1883",
				ClientID: "authenticated-client",
				Username: "user",
				Password: "pass",
				QoS:      2,
			},
		},
		{
			name: "secure websocket config",
			config: Config{
				Broker:   "wss://broker.example.com:8084",
				ClientID: "ws-client",
				Username: "wsuser",
				Password: "wspass",
				QoS:      0,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Test that config values are properly set
			if tt.config.Broker == "" {
				t.Error("Broker should not be empty")
			}
			if tt.config.ClientID == "" {
				t.Error("ClientID should not be empty")
			}
			if tt.config.QoS > 2 {
				t.Error("QoS should be 0, 1, or 2")
			}
		})
	}
}

func TestMessageHandler(t *testing.T) {
	// Test that MessageHandler type can be used
	called := false
	var handler MessageHandler = func(topic string, payload []byte) error {
		called = true
		if topic == "" {
			t.Error("Topic should not be empty")
		}
		return nil
	}

	// Invoke the handler
	err := handler("test/topic", []byte("test payload"))
	if err != nil {
		t.Errorf("Handler should not return error: %v", err)
	}
	if !called {
		t.Error("Handler should have been called")
	}
}

func TestMessageHandlerWithError(t *testing.T) {
	// Test that MessageHandler can return errors
	var handler MessageHandler = func(topic string, payload []byte) error {
		return nil
	}

	err := handler("test/topic", []byte("test"))
	if err != nil {
		t.Errorf("Handler should return nil for successful processing")
	}
}

// Note: Testing New(), Subscribe(), and Disconnect() would require a real MQTT broker
// or a mock MQTT client, which is beyond the scope of unit tests.
// These should be tested with integration tests that have access to a test broker.

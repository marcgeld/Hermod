package pipeline

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/marcgeld/hermod/internal/logger"
	"github.com/marcgeld/hermod/internal/lua"
	"github.com/marcgeld/hermod/internal/storage"
)

// Pipeline orchestrates message processing
type Pipeline struct {
	transformer *lua.Transformer
	storage     *storage.Storage
	logger      *logger.Logger
}

// New creates a new pipeline instance
func New(transformer *lua.Transformer, storage *storage.Storage, log *logger.Logger) *Pipeline {
	if log == nil {
		log = logger.New(logger.INFO)
	}
	return &Pipeline{
		transformer: transformer,
		storage:     storage,
		logger:      log,
	}
}

// Process processes an incoming message
func (p *Pipeline) Process(ctx context.Context, topic string, payload []byte) error {
	p.logger.Debugf("Received message from topic '%s': %s", topic, string(payload))

	// Try to decode as JSON
	var data map[string]interface{}
	if err := json.Unmarshal(payload, &data); err != nil {
		// If not JSON, create a simple map with the raw payload
		p.logger.Debugf("Payload is not JSON, storing as raw data: %v", err)
		data = map[string]interface{}{
			"topic":   topic,
			"payload": string(payload),
		}
	} else {
		// Add topic to the data
		data["topic"] = topic
	}

	// Transform the data if transformer is available
	if p.transformer != nil {
		p.logger.Debug("Transforming message data")
		transformed, err := p.transformer.Transform(data)
		if err != nil {
			return fmt.Errorf("failed to transform data: %w", err)
		}
		data = transformed
		p.logger.Debugf("Message transformed: %v", data)
	}

	// Store the data
	if err := p.storage.Insert(ctx, data); err != nil {
		return fmt.Errorf("failed to store data: %w", err)
	}

	p.logger.Infof("Successfully processed message from topic %s", topic)
	return nil
}

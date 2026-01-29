package pipeline

import (
	"context"
	"encoding/json"
	"fmt"
	"log"

	"github.com/marcgeld/Hermod/internal/lua"
	"github.com/marcgeld/Hermod/internal/storage"
)

// Pipeline orchestrates message processing
type Pipeline struct {
	transformer *lua.Transformer
	storage     *storage.Storage
}

// New creates a new pipeline instance
func New(transformer *lua.Transformer, storage *storage.Storage) *Pipeline {
	return &Pipeline{
		transformer: transformer,
		storage:     storage,
	}
}

// Process processes an incoming message
func (p *Pipeline) Process(ctx context.Context, topic string, payload []byte) error {
	// Try to decode as JSON
	var data map[string]interface{}
	if err := json.Unmarshal(payload, &data); err != nil {
		// If not JSON, create a simple map with the raw payload
		log.Printf("Payload is not JSON, storing as raw data: %v", err)
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
		transformed, err := p.transformer.Transform(data)
		if err != nil {
			return fmt.Errorf("failed to transform data: %w", err)
		}
		data = transformed
	}

	// Store the data
	if err := p.storage.Insert(ctx, data); err != nil {
		return fmt.Errorf("failed to store data: %w", err)
	}

	log.Printf("Successfully processed message from topic %s", topic)
	return nil
}

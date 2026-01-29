package pipeline

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/marcgeld/Hermod/internal/lua"
)

// StorageInterface defines the interface needed by Pipeline for testing
type StorageInterface interface {
	Insert(ctx context.Context, data map[string]interface{}) error
	Close()
}

// mockStorage is a mock implementation of storage for testing
type mockStorage struct {
	insertedData []map[string]interface{}
	insertError  error
}

func (m *mockStorage) Insert(ctx context.Context, data map[string]interface{}) error {
	if m.insertError != nil {
		return m.insertError
	}
	m.insertedData = append(m.insertedData, data)
	return nil
}

func (m *mockStorage) Close() {}

// testPipeline wraps Pipeline for testing with mock storage
type testPipeline struct {
	transformer *lua.Transformer
	storage     StorageInterface
}

func newTestPipeline(transformer *lua.Transformer, storage StorageInterface) *testPipeline {
	return &testPipeline{
		transformer: transformer,
		storage:     storage,
	}
}

// Process mimics the real Pipeline.Process for testing
func (p *testPipeline) Process(ctx context.Context, topic string, payload []byte) error {
	// Try to decode as JSON (same logic as real pipeline)
	var data map[string]interface{}
	if err := json.Unmarshal(payload, &data); err != nil {
		// If not JSON, create a simple map with the raw payload
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
			return err
		}
		data = transformed
	}

	// Store the data
	return p.storage.Insert(ctx, data)
}

func TestProcessWithJSONPayload(t *testing.T) {
	mockStore := &mockStorage{}
	pipeline := newTestPipeline(nil, mockStore)

	tests := []struct {
		name    string
		topic   string
		payload []byte
		wantErr bool
	}{
		{
			name:    "valid json payload",
			topic:   "sensors/temperature",
			payload: []byte(`{"temperature": 25.5, "humidity": 60}`),
			wantErr: false,
		},
		{
			name:    "json with nested object",
			topic:   "devices/sensor01",
			payload: []byte(`{"data": {"temp": 20, "pressure": 1013}}`),
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockStore.insertedData = nil // Reset

			err := pipeline.Process(context.Background(), tt.topic, tt.payload)
			if (err != nil) != tt.wantErr {
				t.Errorf("Process() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if !tt.wantErr {
				if len(mockStore.insertedData) != 1 {
					t.Errorf("Expected 1 insert, got %d", len(mockStore.insertedData))
					return
				}

				data := mockStore.insertedData[0]
				if data["topic"] != tt.topic {
					t.Errorf("Expected topic %s, got %v", tt.topic, data["topic"])
				}
			}
		})
	}
}

func TestProcessWithNonJSONPayload(t *testing.T) {
	mockStore := &mockStorage{}
	pipeline := newTestPipeline(nil, mockStore)

	topic := "sensors/raw"
	payload := []byte("not json data")

	err := pipeline.Process(context.Background(), topic, payload)
	if err != nil {
		t.Errorf("Process() should handle non-JSON payload without error, got: %v", err)
		return
	}

	if len(mockStore.insertedData) != 1 {
		t.Errorf("Expected 1 insert, got %d", len(mockStore.insertedData))
		return
	}

	data := mockStore.insertedData[0]
	if data["topic"] != topic {
		t.Errorf("Expected topic %s, got %v", topic, data["topic"])
	}

	if payload, ok := data["payload"].(string); !ok || payload != "not json data" {
		t.Errorf("Expected payload to be stored as raw string")
	}
}

func TestProcessWithTransformation(t *testing.T) {
	// Create a simple transformer
	tmpDir := t.TempDir()
	scriptPath := filepath.Join(tmpDir, "test.lua")
	scriptCode := `
function transform(data)
    data.transformed = true
    data.processed_by = "test"
    return data
end
`
	if err := os.WriteFile(scriptPath, []byte(scriptCode), 0644); err != nil {
		t.Fatalf("failed to write test script: %v", err)
	}

	transformer, err := lua.New(scriptPath)
	if err != nil {
		t.Fatalf("failed to create transformer: %v", err)
	}
	defer transformer.Close()

	mockStore := &mockStorage{}
	pipeline := newTestPipeline(transformer, mockStore)

	topic := "sensors/test"
	payload := []byte(`{"value": 42}`)

	err = pipeline.Process(context.Background(), topic, payload)
	if err != nil {
		t.Errorf("Process() error = %v", err)
		return
	}

	if len(mockStore.insertedData) != 1 {
		t.Errorf("Expected 1 insert, got %d", len(mockStore.insertedData))
		return
	}

	data := mockStore.insertedData[0]

	// Check that transformation was applied
	if transformed, ok := data["transformed"].(bool); !ok || !transformed {
		t.Errorf("Expected transformed flag to be true")
	}

	if processedBy, ok := data["processed_by"].(string); !ok || processedBy != "test" {
		t.Errorf("Expected processed_by to be 'test', got %v", data["processed_by"])
	}

	// Original data should still be present
	if value, ok := data["value"].(float64); !ok || value != 42 {
		t.Errorf("Expected value to be 42, got %v", data["value"])
	}
}

func TestProcessWithTransformationError(t *testing.T) {
	// Create a transformer that will fail
	tmpDir := t.TempDir()
	scriptPath := filepath.Join(tmpDir, "test.lua")
	scriptCode := `
function transform(data)
    error("intentional error")
end
`
	if err := os.WriteFile(scriptPath, []byte(scriptCode), 0644); err != nil {
		t.Fatalf("failed to write test script: %v", err)
	}

	transformer, err := lua.New(scriptPath)
	if err != nil {
		t.Fatalf("failed to create transformer: %v", err)
	}
	defer transformer.Close()

	mockStore := &mockStorage{}
	pipeline := newTestPipeline(transformer, mockStore)

	topic := "sensors/test"
	payload := []byte(`{"value": 42}`)

	err = pipeline.Process(context.Background(), topic, payload)
	if err == nil {
		t.Error("Process() should return error when transformation fails")
	}
}

func TestProcessAddsTopicToData(t *testing.T) {
	mockStore := &mockStorage{}
	pipeline := newTestPipeline(nil, mockStore)

	topic := "devices/sensor123/telemetry"
	payload := []byte(`{"reading": 100}`)

	err := pipeline.Process(context.Background(), topic, payload)
	if err != nil {
		t.Errorf("Process() error = %v", err)
		return
	}

	if len(mockStore.insertedData) != 1 {
		t.Fatalf("Expected 1 insert, got %d", len(mockStore.insertedData))
	}

	data := mockStore.insertedData[0]
	if data["topic"] != topic {
		t.Errorf("Expected topic to be added to data: got %v, want %v", data["topic"], topic)
	}

	// Original data should also be present
	if reading, ok := data["reading"].(float64); !ok || reading != 100 {
		t.Errorf("Expected reading to be 100, got %v", data["reading"])
	}
}

func TestJSONDecoding(t *testing.T) {
	// Test that JSON is properly decoded into the data map
	mockStore := &mockStorage{}
	pipeline := newTestPipeline(nil, mockStore)

	tests := []struct {
		name        string
		payload     []byte
		expectField string
		expectValue interface{}
	}{
		{
			name:        "string field",
			payload:     []byte(`{"sensor_id": "ABC123"}`),
			expectField: "sensor_id",
			expectValue: "ABC123",
		},
		{
			name:        "numeric field",
			payload:     []byte(`{"temperature": 25.5}`),
			expectField: "temperature",
			expectValue: float64(25.5),
		},
		{
			name:        "boolean field",
			payload:     []byte(`{"active": true}`),
			expectField: "active",
			expectValue: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockStore.insertedData = nil
			err := pipeline.Process(context.Background(), "test/topic", tt.payload)
			if err != nil {
				t.Errorf("Process() error = %v", err)
				return
			}

			if len(mockStore.insertedData) != 1 {
				t.Fatalf("Expected 1 insert, got %d", len(mockStore.insertedData))
			}

			data := mockStore.insertedData[0]
			if data[tt.expectField] != tt.expectValue {
				t.Errorf("Expected %s to be %v, got %v", tt.expectField, tt.expectValue, data[tt.expectField])
			}
		})
	}
}

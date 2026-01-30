package router

import (
	"context"
	"testing"
	"time"
)

func TestTopicMatches(t *testing.T) {
	tests := []struct {
		name    string
		filter  string
		topic   string
		matches bool
	}{
		{"exact match", "ruuvi/sensor1", "ruuvi/sensor1", true},
		{"wildcard #", "#", "any/topic/here", true},
		{"single level +", "ruuvi/+", "ruuvi/sensor1", true},
		{"single level + no match", "ruuvi/+", "ruuvi/sensor1/data", false},
		{"multi level #", "ruuvi/#", "ruuvi/sensor1/data", true},
		{"multi level # at end", "ruuvi/+/#", "ruuvi/sensor1/data/temp", true},
		{"no match different prefix", "ruuvi/+", "p1ib/sensor1", false},
		{"+ matches empty", "ruuvi/+/data", "ruuvi//data", true},
		{"complex pattern", "devices/+/telemetry", "devices/sensor123/telemetry", true},
		{"complex no match", "devices/+/telemetry", "devices/sensor123/status", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := topicMatches(tt.filter, tt.topic)
			if result != tt.matches {
				t.Errorf("topicMatches(%q, %q) = %v, want %v", tt.filter, tt.topic, result, tt.matches)
			}
		})
	}
}

func TestBuildPassthroughRecord(t *testing.T) {
	msg := Message{
		Topic:   "test/topic",
		Payload: []byte(`{"temperature": 25.5}`),
		QoS:     1,
		Retain:  true,
		Time:    time.Date(2024, 1, 1, 12, 0, 0, 0, time.UTC),
	}

	record := buildPassthroughRecord(msg)

	if record["topic"] != "test/topic" {
		t.Errorf("Expected topic 'test/topic', got %v", record["topic"])
	}

	if record["qos"] != 1 {
		t.Errorf("Expected qos 1, got %v", record["qos"])
	}

	if record["retain"] != true {
		t.Errorf("Expected retain true, got %v", record["retain"])
	}

	if record["raw"] != `{"temperature": 25.5}` {
		t.Errorf("Expected raw payload, got %v", record["raw"])
	}

	// Check json field was parsed
	if record["json"] == nil {
		t.Error("Expected json field to be populated for valid JSON payload")
	}

	// Test with non-JSON payload
	msg2 := Message{
		Topic:   "test/topic",
		Payload: []byte("not json"),
		QoS:     0,
		Retain:  false,
		Time:    time.Now().UTC(),
	}

	record2 := buildPassthroughRecord(msg2)
	if record2["json"] != nil {
		t.Error("Expected json field to be nil for non-JSON payload")
	}
	if record2["raw"] != "not json" {
		t.Errorf("Expected raw 'not json', got %v", record2["raw"])
	}
}

// mockStorage for testing
type mockStorage struct {
	inserts map[string][]map[string]interface{}
}

func newMockStorage() *mockStorage {
	return &mockStorage{
		inserts: make(map[string][]map[string]interface{}),
	}
}

func (m *mockStorage) InsertIntoTable(ctx context.Context, table string, data map[string]interface{}) error {
	m.inserts[table] = append(m.inserts[table], data)
	return nil
}

func TestRouterDispatch(t *testing.T) {
	ctx := context.Background()
	storage := newMockStorage()

	routes := []Route{
		{
			Filter:    "sensors/+",
			Script:    "", // Passthrough
			Workers:   1,
			QueueSize: 10,
			Table:     "sensor_data",
		},
	}

	r, err := New(ctx, routes, storage, nil)
	if err != nil {
		t.Fatalf("Failed to create router: %v", err)
	}
	defer r.Close()

	msg := Message{
		Topic:   "sensors/temp1",
		Payload: []byte("test"),
		QoS:     1,
		Retain:  false,
		Time:    time.Now().UTC(),
	}

	err = r.Dispatch(msg)
	if err != nil {
		t.Errorf("Dispatch failed: %v", err)
	}

	// Give worker time to process
	time.Sleep(100 * time.Millisecond)

	if len(storage.inserts["sensor_data"]) == 0 {
		t.Error("Expected message to be inserted into sensor_data table")
	}
}

func TestRouterPassthrough(t *testing.T) {
	ctx := context.Background()
	storage := newMockStorage()

	// No routes - all messages should go to passthrough
	routes := []Route{}

	r, err := New(ctx, routes, storage, nil)
	if err != nil {
		t.Fatalf("Failed to create router: %v", err)
	}
	defer r.Close()

	msg := Message{
		Topic:   "unmatched/topic",
		Payload: []byte("test data"),
		QoS:     0,
		Retain:  false,
		Time:    time.Now().UTC(),
	}

	err = r.Dispatch(msg)
	if err != nil {
		t.Errorf("Dispatch failed: %v", err)
	}

	// Give passthrough time to process
	time.Sleep(100 * time.Millisecond)

	if len(storage.inserts["iot_raw"]) == 0 {
		t.Error("Expected message to be inserted into iot_raw table via passthrough")
	}
}

func TestValidIdentifier(t *testing.T) {
	tests := []struct {
		name       string
		identifier string
		valid      bool
	}{
		{"valid simple", "table_name", true},
		{"valid with numbers", "table123", true},
		{"valid underscore start", "_table", true},
		{"invalid space", "table name", false},
		{"invalid dash", "table-name", false},
		{"invalid special char", "table$name", false},
		{"invalid dot", "schema.table", false},
		{"valid uppercase", "TableName", true},
		{"valid mixed case", "My_Table_123", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := validIdentifier.MatchString(tt.identifier)
			if result != tt.valid {
				t.Errorf("validIdentifier.MatchString(%q) = %v, want %v", tt.identifier, result, tt.valid)
			}
		})
	}
}

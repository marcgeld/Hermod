package router

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestWorkerWithLuaTransform(t *testing.T) {
	tmpDir := t.TempDir()
	scriptPath := filepath.Join(tmpDir, "test.lua")

	// Create a Lua script with schema and transform
	scriptCode := `
schema = {
  tables = {
    test_table = {
      time = "timestamptz",
      topic = "text",
      value = "double precision"
    }
  }
}

function transform(msg)
  local records = {}
  
  if msg.json and msg.json.value then
    local record = {
      table = "test_table",
      columns = {
        time = msg.ts,
        topic = msg.topic,
        value = msg.json.value
      }
    }
    table.insert(records, record)
  end
  
  return records
end
`

	if err := os.WriteFile(scriptPath, []byte(scriptCode), 0644); err != nil {
		t.Fatalf("failed to write test script: %v", err)
	}

	ctx := context.Background()
	storage := newMockStorage()
	msgChan := make(chan Message, 10)

	worker, err := newWorker(1, scriptPath, "test_table", msgChan, storage, ctx, nil)
	if err != nil {
		t.Fatalf("failed to create worker: %v", err)
	}
	defer worker.state.Close()

	// Test message with JSON payload
	msg := Message{
		Topic:   "sensors/temp1",
		Payload: []byte(`{"value": 25.5}`),
		QoS:     1,
		Retain:  false,
		Time:    time.Now().UTC(),
	}

	if err := worker.process(msg); err != nil {
		t.Errorf("process failed: %v", err)
	}

	// Check that data was inserted
	if len(storage.inserts["test_table"]) != 1 {
		t.Fatalf("Expected 1 insert into test_table, got %d", len(storage.inserts["test_table"]))
	}

	record := storage.inserts["test_table"][0]
	if record["topic"] != "sensors/temp1" {
		t.Errorf("Expected topic 'sensors/temp1', got %v", record["topic"])
	}

	if record["value"] != 25.5 {
		t.Errorf("Expected value 25.5, got %v", record["value"])
	}
}

func TestWorkerSchemaValidation(t *testing.T) {
	tmpDir := t.TempDir()
	scriptPath := filepath.Join(tmpDir, "test.lua")

	// Create a Lua script that emits undeclared column
	scriptCode := `
schema = {
  tables = {
    test_table = {
      time = "timestamptz",
      value = "double precision"
    }
  }
}

function transform(msg)
  return {
    {
      table = "test_table",
      columns = {
        time = msg.ts,
        value = 42,
        invalid_column = "should fail"  -- Not in schema!
      }
    }
  }
end
`

	if err := os.WriteFile(scriptPath, []byte(scriptCode), 0644); err != nil {
		t.Fatalf("failed to write test script: %v", err)
	}

	ctx := context.Background()
	storage := newMockStorage()
	msgChan := make(chan Message, 10)

	worker, err := newWorker(1, scriptPath, "test_table", msgChan, storage, ctx, nil)
	if err != nil {
		t.Fatalf("failed to create worker: %v", err)
	}
	defer worker.state.Close()

	msg := Message{
		Topic:   "test/topic",
		Payload: []byte("test"),
		QoS:     0,
		Retain:  false,
		Time:    time.Now().UTC(),
	}

	// Should fail due to invalid column
	err = worker.process(msg)
	if err == nil {
		t.Error("Expected error for undeclared column, got nil")
	}

	// No data should be inserted
	if len(storage.inserts["test_table"]) != 0 {
		t.Error("Expected no inserts due to validation error")
	}
}

func TestWorkerMultiTable(t *testing.T) {
	tmpDir := t.TempDir()
	scriptPath := filepath.Join(tmpDir, "test.lua")

	// Create a Lua script that writes to multiple tables
	scriptCode := `
schema = {
  tables = {
    readings = {
      time = "timestamptz",
      value = "double precision"
    },
    events = {
      time = "timestamptz",
      event = "text"
    }
  }
}

function transform(msg)
  return {
    {
      table = "readings",
      columns = {
        time = msg.ts,
        value = 123.45
      }
    },
    {
      table = "events",
      columns = {
        time = msg.ts,
        event = "data_received"
      }
    }
  }
end
`

	if err := os.WriteFile(scriptPath, []byte(scriptCode), 0644); err != nil {
		t.Fatalf("failed to write test script: %v", err)
	}

	ctx := context.Background()
	storage := newMockStorage()
	msgChan := make(chan Message, 10)

	worker, err := newWorker(1, scriptPath, "default_table", msgChan, storage, ctx, nil)
	if err != nil {
		t.Fatalf("failed to create worker: %v", err)
	}
	defer worker.state.Close()

	msg := Message{
		Topic:   "test/topic",
		Payload: []byte("test"),
		QoS:     0,
		Retain:  false,
		Time:    time.Now().UTC(),
	}

	if err := worker.process(msg); err != nil {
		t.Errorf("process failed: %v", err)
	}

	// Check both tables received data
	if len(storage.inserts["readings"]) != 1 {
		t.Error("Expected 1 insert into readings table")
	}

	if len(storage.inserts["events"]) != 1 {
		t.Error("Expected 1 insert into events table")
	}
}

func TestWorkerDefaultTable(t *testing.T) {
	tmpDir := t.TempDir()
	scriptPath := filepath.Join(tmpDir, "test.lua")

	// Create a Lua script that doesn't specify table name
	scriptCode := `
schema = {
  tables = {
    default_data = {
      time = "timestamptz",
      value = "double precision"
    }
  }
}

function transform(msg)
  return {
    {
      -- No table specified, should use default
      columns = {
        time = msg.ts,
        value = 99
      }
    }
  }
end
`

	if err := os.WriteFile(scriptPath, []byte(scriptCode), 0644); err != nil {
		t.Fatalf("failed to write test script: %v", err)
	}

	ctx := context.Background()
	storage := newMockStorage()
	msgChan := make(chan Message, 10)

	worker, err := newWorker(1, scriptPath, "default_data", msgChan, storage, ctx, nil)
	if err != nil {
		t.Fatalf("failed to create worker: %v", err)
	}
	defer worker.state.Close()

	msg := Message{
		Topic:   "test/topic",
		Payload: []byte("test"),
		QoS:     0,
		Retain:  false,
		Time:    time.Now().UTC(),
	}

	if err := worker.process(msg); err != nil {
		t.Errorf("process failed: %v", err)
	}

	// Check data was inserted into default table
	if len(storage.inserts["default_data"]) != 1 {
		t.Error("Expected 1 insert into default_data table")
	}
}

func TestWorkerJSONParsing(t *testing.T) {
	tmpDir := t.TempDir()
	scriptPath := filepath.Join(tmpDir, "test.lua")

	// Create a Lua script that uses msg.json
	scriptCode := `
function transform(msg)
  local records = {}
  
  -- Test that msg has expected fields
  if not msg.topic then
    error("msg.topic missing")
  end
  
  if not msg.payload then
    error("msg.payload missing")
  end
  
  if not msg.ts then
    error("msg.ts missing")
  end
  
  -- Check if JSON was parsed
  if msg.json then
    local record = {
      table = "parsed_data",
      columns = {
        topic = msg.topic,
        temp = msg.json.temperature
      }
    }
    table.insert(records, record)
  end
  
  return records
end
`

	if err := os.WriteFile(scriptPath, []byte(scriptCode), 0644); err != nil {
		t.Fatalf("failed to write test script: %v", err)
	}

	ctx := context.Background()
	storage := newMockStorage()
	msgChan := make(chan Message, 10)

	worker, err := newWorker(1, scriptPath, "parsed_data", msgChan, storage, ctx, nil)
	if err != nil {
		t.Fatalf("failed to create worker: %v", err)
	}
	defer worker.state.Close()

	// Test with JSON payload
	msg := Message{
		Topic:   "sensors/temp",
		Payload: []byte(`{"temperature": 22.5}`),
		QoS:     1,
		Retain:  false,
		Time:    time.Now().UTC(),
	}

	if err := worker.process(msg); err != nil {
		t.Errorf("process with JSON failed: %v", err)
	}

	if len(storage.inserts["parsed_data"]) != 1 {
		t.Error("Expected 1 insert for JSON payload")
	}

	// Test with non-JSON payload (msg.json should be nil)
	msg2 := Message{
		Topic:   "sensors/temp",
		Payload: []byte("not json"),
		QoS:     1,
		Retain:  false,
		Time:    time.Now().UTC(),
	}

	storage.inserts = make(map[string][]map[string]interface{}) // Reset
	if err := worker.process(msg2); err != nil {
		t.Errorf("process with non-JSON failed: %v", err)
	}

	// Should return empty records for non-JSON
	if len(storage.inserts["parsed_data"]) != 0 {
		t.Error("Expected no inserts for non-JSON payload")
	}
}

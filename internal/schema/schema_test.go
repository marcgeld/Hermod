package schema

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLoadFromLuaScript(t *testing.T) {
	tests := []struct {
		name       string
		scriptCode string
		wantTables int
		wantErr    bool
	}{
		{
			name: "valid schema",
			scriptCode: `
schema = {
  tables = {
    iot_metrics = {
      time = "timestamptz",
      device = "text",
      value = "double precision"
    }
  }
}

function transform(msg)
  return {}
end
`,
			wantTables: 1,
			wantErr:    false,
		},
		{
			name: "multiple tables",
			scriptCode: `
schema = {
  tables = {
    sensors = {
      id = "bigint",
      name = "text"
    },
    readings = {
      sensor_id = "bigint",
      value = "double precision",
      timestamp = "timestamptz"
    }
  }
}
`,
			wantTables: 2,
			wantErr:    false,
		},
		{
			name: "no schema defined",
			scriptCode: `
function transform(msg)
  return {}
end
`,
			wantTables: 0,
			wantErr:    false,
		},
		{
			name: "empty schema",
			scriptCode: `
schema = {
  tables = {}
}
`,
			wantTables: 0,
			wantErr:    false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tmpDir := t.TempDir()
			scriptPath := filepath.Join(tmpDir, "test.lua")

			if err := os.WriteFile(scriptPath, []byte(tt.scriptCode), 0644); err != nil {
				t.Fatalf("failed to write test script: %v", err)
			}

			schema, err := LoadFromLuaScript(scriptPath)
			if (err != nil) != tt.wantErr {
				t.Errorf("LoadFromLuaScript() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if !tt.wantErr {
				if len(schema.Tables) != tt.wantTables {
					t.Errorf("LoadFromLuaScript() got %d tables, want %d", len(schema.Tables), tt.wantTables)
				}
			}
		})
	}
}

func TestGenerateSQL(t *testing.T) {
	schema := &Schema{
		Tables: map[string]*TableSchema{
			"iot_metrics": {
				Name: "iot_metrics",
				Columns: map[string]string{
					"time":   "timestamptz",
					"device": "text",
					"value":  "double precision",
				},
			},
		},
	}

	sql := schema.GenerateSQL()

	// Check that SQL contains expected elements
	if !strings.Contains(sql, "CREATE TABLE IF NOT EXISTS iot_metrics") {
		t.Error("SQL should contain CREATE TABLE statement")
	}

	if !strings.Contains(sql, "time timestamptz") {
		t.Error("SQL should contain time column")
	}

	if !strings.Contains(sql, "device text") {
		t.Error("SQL should contain device column")
	}

	if !strings.Contains(sql, "value double precision") {
		t.Error("SQL should contain value column")
	}

	// Check that columns are sorted (deterministic output)
	lines := strings.Split(sql, "\n")
	var columnLines []string
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.Contains(trimmed, "timestamptz") ||
			strings.Contains(trimmed, "text") ||
			strings.Contains(trimmed, "double precision") {
			columnLines = append(columnLines, trimmed)
		}
	}

	// Columns should appear in alphabetical order: device, time, value
	if len(columnLines) >= 3 {
		if !strings.HasPrefix(columnLines[0], "device") {
			t.Errorf("First column should be 'device', got: %s", columnLines[0])
		}
		if !strings.HasPrefix(columnLines[1], "time") {
			t.Errorf("Second column should be 'time', got: %s", columnLines[1])
		}
		if !strings.HasPrefix(columnLines[2], "value") {
			t.Errorf("Third column should be 'value', got: %s", columnLines[2])
		}
	}
}

func TestGenerateSQLEmpty(t *testing.T) {
	schema := &Schema{
		Tables: map[string]*TableSchema{},
	}

	sql := schema.GenerateSQL()
	if sql != "" {
		t.Errorf("Expected empty SQL for empty schema, got: %s", sql)
	}
}

func TestMerge(t *testing.T) {
	schema1 := &Schema{
		Tables: map[string]*TableSchema{
			"table1": {
				Name: "table1",
				Columns: map[string]string{
					"col1": "text",
					"col2": "int",
				},
			},
		},
	}

	schema2 := &Schema{
		Tables: map[string]*TableSchema{
			"table2": {
				Name: "table2",
				Columns: map[string]string{
					"col3": "text",
				},
			},
		},
	}

	merged := Merge(schema1, schema2)

	if len(merged.Tables) != 2 {
		t.Errorf("Expected 2 tables after merge, got %d", len(merged.Tables))
	}

	if _, ok := merged.Tables["table1"]; !ok {
		t.Error("Merged schema should contain table1")
	}

	if _, ok := merged.Tables["table2"]; !ok {
		t.Error("Merged schema should contain table2")
	}
}

func TestMergeSameTable(t *testing.T) {
	schema1 := &Schema{
		Tables: map[string]*TableSchema{
			"shared": {
				Name: "shared",
				Columns: map[string]string{
					"col1": "text",
				},
			},
		},
	}

	schema2 := &Schema{
		Tables: map[string]*TableSchema{
			"shared": {
				Name: "shared",
				Columns: map[string]string{
					"col2": "int",
				},
			},
		},
	}

	merged := Merge(schema1, schema2)

	if len(merged.Tables) != 1 {
		t.Errorf("Expected 1 table after merge, got %d", len(merged.Tables))
	}

	sharedTable := merged.Tables["shared"]
	if len(sharedTable.Columns) != 2 {
		t.Errorf("Expected 2 columns in merged table, got %d", len(sharedTable.Columns))
	}

	if _, ok := sharedTable.Columns["col1"]; !ok {
		t.Error("Merged table should contain col1")
	}

	if _, ok := sharedTable.Columns["col2"]; !ok {
		t.Error("Merged table should contain col2")
	}
}

func TestValidateRecord(t *testing.T) {
	tableSchema := &TableSchema{
		Name: "test_table",
		Columns: map[string]string{
			"col1": "text",
			"col2": "int",
		},
	}

	tests := []struct {
		name    string
		columns map[string]interface{}
		wantErr bool
	}{
		{
			name: "valid record",
			columns: map[string]interface{}{
				"col1": "value1",
				"col2": 42,
			},
			wantErr: false,
		},
		{
			name: "subset of columns",
			columns: map[string]interface{}{
				"col1": "value1",
			},
			wantErr: false,
		},
		{
			name: "undeclared column",
			columns: map[string]interface{}{
				"col1":  "value1",
				"col99": "extra",
			},
			wantErr: true,
		},
		{
			name:    "empty record",
			columns: map[string]interface{}{},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tableSchema.ValidateRecord(tt.columns)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateRecord() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestGenerateCreateTable(t *testing.T) {
	table := &TableSchema{
		Name: "test_table",
		Columns: map[string]string{
			"id":        "bigint",
			"name":      "text",
			"timestamp": "timestamptz",
		},
	}

	sql := table.GenerateCreateTable()

	if !strings.Contains(sql, "CREATE TABLE IF NOT EXISTS test_table") {
		t.Error("SQL should contain CREATE TABLE statement")
	}

	if !strings.Contains(sql, "id bigint") {
		t.Error("SQL should contain id column")
	}

	if !strings.Contains(sql, "name text") {
		t.Error("SQL should contain name column")
	}

	if !strings.Contains(sql, "timestamp timestamptz") {
		t.Error("SQL should contain timestamp column")
	}

	// Check it ends with semicolon
	if !strings.HasSuffix(strings.TrimSpace(sql), ");") {
		t.Error("SQL should end with );")
	}
}

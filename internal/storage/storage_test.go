package storage

import (
	"testing"
)

func TestValidTableName(t *testing.T) {
	tests := []struct {
		name      string
		tableName string
		want      bool
	}{
		{
			name:      "valid simple name",
			tableName: "mqtt_messages",
			want:      true,
		},
		{
			name:      "valid with underscores",
			tableName: "my_test_table_123",
			want:      true,
		},
		{
			name:      "valid starting with underscore",
			tableName: "_private_table",
			want:      true,
		},
		{
			name:      "invalid with spaces",
			tableName: "my table",
			want:      false,
		},
		{
			name:      "invalid with special chars",
			tableName: "table-name",
			want:      false,
		},
		{
			name:      "invalid with semicolon (SQL injection attempt)",
			tableName: "table; DROP TABLE users;",
			want:      false,
		},
		{
			name:      "invalid starting with number",
			tableName: "123table",
			want:      false,
		},
		{
			name:      "invalid empty string",
			tableName: "",
			want:      false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := validTableName.MatchString(tt.tableName)
			if got != tt.want {
				t.Errorf("validTableName.MatchString(%q) = %v, want %v", tt.tableName, got, tt.want)
			}
		})
	}
}

func TestValidColumnName(t *testing.T) {
	tests := []struct {
		name       string
		columnName string
		want       bool
	}{
		{
			name:       "valid simple name",
			columnName: "temperature",
			want:       true,
		},
		{
			name:       "valid with underscores and numbers",
			columnName: "sensor_id_123",
			want:       true,
		},
		{
			name:       "valid starting with underscore",
			columnName: "_internal",
			want:       true,
		},
		{
			name:       "invalid with spaces",
			columnName: "column name",
			want:       false,
		},
		{
			name:       "invalid with dash",
			columnName: "column-name",
			want:       false,
		},
		{
			name:       "invalid with parentheses",
			columnName: "func()",
			want:       false,
		},
		{
			name:       "invalid with quotes",
			columnName: "column'name",
			want:       false,
		},
		{
			name:       "invalid starting with number",
			columnName: "1column",
			want:       false,
		},
		{
			name:       "invalid empty string",
			columnName: "",
			want:       false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := validColumnName.MatchString(tt.columnName)
			if got != tt.want {
				t.Errorf("validColumnName.MatchString(%q) = %v, want %v", tt.columnName, got, tt.want)
			}
		})
	}
}

func TestInsertValidation(t *testing.T) {
	// Testing validation logic only, without actual database operations
	tests := []struct {
		name    string
		data    map[string]interface{}
		wantErr bool
		errMsg  string
	}{
		{
			name: "valid data",
			data: map[string]interface{}{
				"temperature": 25.5,
				"humidity":    60,
				"sensor_id":   "sensor_001",
			},
			wantErr: false,
		},
		{
			name:    "empty data",
			data:    map[string]interface{}{},
			wantErr: true,
			errMsg:  "empty data",
		},
		{
			name: "invalid column name with space",
			data: map[string]interface{}{
				"invalid column": "value",
			},
			wantErr: true,
			errMsg:  "invalid column name",
		},
		{
			name: "invalid column name with special char",
			data: map[string]interface{}{
				"column;DROP TABLE": "value",
			},
			wantErr: true,
			errMsg:  "invalid column name",
		},
		{
			name: "valid column with nested object",
			data: map[string]interface{}{
				"metadata": map[string]interface{}{
					"location": "warehouse",
				},
			},
			wantErr: false, // Should not error on validation, will be JSON marshaled
		},
		{
			name: "valid column with array",
			data: map[string]interface{}{
				"readings": []interface{}{1, 2, 3},
			},
			wantErr: false, // Should not error on validation, will be JSON marshaled
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// We'll test the validation logic by checking if column names pass validation
			if !tt.wantErr {
				// Validate column names
				for key := range tt.data {
					if !validColumnName.MatchString(key) {
						t.Errorf("Expected valid column name %q to pass validation", key)
					}
				}
			} else {
				// Check if at least one validation would fail
				hasInvalidColumn := false
				isEmpty := len(tt.data) == 0

				for key := range tt.data {
					if !validColumnName.MatchString(key) {
						hasInvalidColumn = true
						break
					}
				}

				if !isEmpty && !hasInvalidColumn && tt.errMsg == "invalid column name" {
					t.Errorf("Expected to find invalid column name but all were valid")
				}
				if !isEmpty && tt.errMsg == "empty data" {
					t.Errorf("Expected empty data but got data")
				}
			}
		})
	}
}

func TestTableNameValidationInNew(t *testing.T) {
	// We cannot test New() without a real database connection,
	// but we've tested the validation regex patterns above
	// which is the key security feature

	// Test that our validation patterns are working
	invalidTableNames := []string{
		"table; DROP TABLE users",
		"table--comment",
		"table/**/name",
		"table name",
		"123invalid",
	}

	for _, name := range invalidTableNames {
		if validTableName.MatchString(name) {
			t.Errorf("validTableName should reject %q but accepted it", name)
		}
	}

	validTableNames := []string{
		"mqtt_messages",
		"sensor_data",
		"_private",
		"table123",
	}

	for _, name := range validTableNames {
		if !validTableName.MatchString(name) {
			t.Errorf("validTableName should accept %q but rejected it", name)
		}
	}
}

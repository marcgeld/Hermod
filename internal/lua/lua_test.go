package lua

import (
	"os"
	"path/filepath"
	"testing"
)

func TestNew(t *testing.T) {
	tests := []struct {
		name       string
		scriptCode string
		wantErr    bool
	}{
		{
			name: "valid lua script",
			scriptCode: `
function transform(data)
    return data
end
`,
			wantErr: false,
		},
		{
			name: "script with syntax error",
			scriptCode: `
function transform(data
    return data
end
`,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create temporary lua script
			tmpDir := t.TempDir()
			scriptPath := filepath.Join(tmpDir, "test.lua")

			if err := os.WriteFile(scriptPath, []byte(tt.scriptCode), 0644); err != nil {
				t.Fatalf("failed to write test script: %v", err)
			}

			transformer, err := New(scriptPath)
			if (err != nil) != tt.wantErr {
				t.Errorf("New() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if !tt.wantErr && transformer != nil {
				defer transformer.Close()
			}
		})
	}
}

func TestNewNonExistentFile(t *testing.T) {
	_, err := New("/nonexistent/script.lua")
	if err == nil {
		t.Error("New() should return error for nonexistent file")
	}
}

func TestTransform(t *testing.T) {
	tests := []struct {
		name       string
		scriptCode string
		input      map[string]interface{}
		want       map[string]interface{}
		wantErr    bool
	}{
		{
			name: "identity transform",
			scriptCode: `
function transform(data)
    return data
end
`,
			input: map[string]interface{}{
				"key": "value",
				"num": float64(42),
			},
			want: map[string]interface{}{
				"key": "value",
				"num": float64(42),
			},
			wantErr: false,
		},
		{
			name: "add field transform",
			scriptCode: `
function transform(data)
    data.new_field = "added"
    return data
end
`,
			input: map[string]interface{}{
				"original": "value",
			},
			want: map[string]interface{}{
				"original":  "value",
				"new_field": "added",
			},
			wantErr: false,
		},
		{
			name: "temperature conversion",
			scriptCode: `
function transform(data)
    local result = {}
    result.celsius = data.temperature
    result.fahrenheit = (data.temperature * 9/5) + 32
    return result
end
`,
			input: map[string]interface{}{
				"temperature": float64(0),
			},
			want: map[string]interface{}{
				"celsius":    float64(0),
				"fahrenheit": float64(32),
			},
			wantErr: false,
		},
		{
			name: "script without transform function",
			scriptCode: `
function other_function()
    return {}
end
`,
			input:   map[string]interface{}{"key": "value"},
			wantErr: true,
		},
		{
			name: "transform returns non-table",
			scriptCode: `
function transform(data)
    return "not a table"
end
`,
			input:   map[string]interface{}{"key": "value"},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create temporary lua script
			tmpDir := t.TempDir()
			scriptPath := filepath.Join(tmpDir, "test.lua")

			if err := os.WriteFile(scriptPath, []byte(tt.scriptCode), 0644); err != nil {
				t.Fatalf("failed to write test script: %v", err)
			}

			transformer, err := New(scriptPath)
			if err != nil {
				t.Fatalf("New() error = %v", err)
			}
			defer transformer.Close()

			got, err := transformer.Transform(tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("Transform() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if !tt.wantErr {
				// Compare the maps
				if len(got) != len(tt.want) {
					t.Errorf("Transform() result length = %v, want %v", len(got), len(tt.want))
				}
				for k, wantV := range tt.want {
					gotV, ok := got[k]
					if !ok {
						t.Errorf("Transform() missing key %v", k)
						continue
					}
					if gotV != wantV {
						t.Errorf("Transform() key %v = %v, want %v", k, gotV, wantV)
					}
				}
			}
		})
	}
}

func TestTransformDataTypes(t *testing.T) {
	scriptCode := `
function transform(data)
    return data
end
`
	tmpDir := t.TempDir()
	scriptPath := filepath.Join(tmpDir, "test.lua")
	if err := os.WriteFile(scriptPath, []byte(scriptCode), 0644); err != nil {
		t.Fatalf("failed to write test script: %v", err)
	}

	transformer, err := New(scriptPath)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	defer transformer.Close()

	tests := []struct {
		name  string
		input map[string]interface{}
	}{
		{
			name: "string value",
			input: map[string]interface{}{
				"text": "hello",
			},
		},
		{
			name: "numeric value",
			input: map[string]interface{}{
				"number": float64(123.45),
			},
		},
		{
			name: "boolean value",
			input: map[string]interface{}{
				"flag": true,
			},
		},
		{
			name: "mixed types",
			input: map[string]interface{}{
				"text":   "value",
				"number": float64(42),
			},
		},
		{
			name: "nested map",
			input: map[string]interface{}{
				"nested": map[string]interface{}{
					"inner": "value",
				},
			},
		},
		{
			name: "array value",
			input: map[string]interface{}{
				"array": []interface{}{float64(1), float64(2), float64(3)},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := transformer.Transform(tt.input)
			if err != nil {
				t.Errorf("Transform() error = %v", err)
				return
			}

			// Verify that the output contains all input keys
			for k := range tt.input {
				if _, ok := got[k]; !ok {
					t.Errorf("Transform() missing key %v", k)
				}
			}
		})
	}
}

func TestTransformConcurrency(t *testing.T) {
	scriptCode := `
function transform(data)
    data.processed = true
    return data
end
`
	tmpDir := t.TempDir()
	scriptPath := filepath.Join(tmpDir, "test.lua")
	if err := os.WriteFile(scriptPath, []byte(scriptCode), 0644); err != nil {
		t.Fatalf("failed to write test script: %v", err)
	}

	transformer, err := New(scriptPath)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	defer transformer.Close()

	// Run multiple transforms concurrently
	// Use channels to collect errors from goroutines safely
	errChan := make(chan error, 10)
	done := make(chan bool, 10)

	for i := 0; i < 10; i++ {
		go func(n int) {
			defer func() { done <- true }()
			input := map[string]interface{}{
				"id": float64(n),
			}
			if err := func() error {
				_, err := transformer.Transform(input)
				return err
			}(); err != nil {
				errChan <- err
			}
		}(i)
	}

	// Wait for all goroutines to complete
	for i := 0; i < 10; i++ {
		<-done
	}
	close(errChan)

	// Check for any errors
	for err := range errChan {
		t.Errorf("Transform() error = %v", err)
	}
}

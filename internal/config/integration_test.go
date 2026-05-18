package config

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/marcgeld/hermod/internal/schema"
)

func TestIntegration_ResolveAndLoadSchema(t *testing.T) {
	tmp := t.TempDir()
	configPath := filepath.Join(tmp, "config.toml")
	cfgContent := `
[database]
host = "localhost"
port = 5432
user = "hermod"
password = "pass"
database = "hermod"
sslmode = "disable"
pool_size = 10

[pipeline]
lua_script = "myscript.lua"
table_name = "test_table"
`
	if err := os.WriteFile(configPath, []byte(cfgContent), 0644); err != nil {
		t.Fatalf("failed to write config: %v", err)
	}

	// Create scriptdir and place a lua script declaring a schema
	scriptDir := filepath.Join(tmp, "scriptsdir")
	if err := os.Mkdir(scriptDir, 0755); err != nil {
		t.Fatalf("failed to create scriptdir: %v", err)
	}
	scriptPath := filepath.Join(scriptDir, "myscript.lua")
	lua := `
schema = {
  tables = {
    test_table = {
      ts = "timestamptz",
      value = "double precision"
    }
  }
}
`
	if err := os.WriteFile(scriptPath, []byte(lua), 0644); err != nil {
		t.Fatalf("failed to write lua script: %v", err)
	}

	// Load config
	cfg, err := Load(configPath)
	if err != nil {
		t.Fatalf("Load() error: %v", err)
	}

	// Resolve script path using scriptdir
	resolved, err := ResolveScriptPath(cfg.Pipeline.LuaScript, configPath, scriptDir)
	if err != nil {
		t.Fatalf("ResolveScriptPath error: %v", err)
	}

	// Load schema from resolved path
	s, err := schema.LoadFromLuaScript(resolved)
	if err != nil {
		t.Fatalf("LoadFromLuaScript error: %v", err)
	}

	if _, ok := s.Tables["test_table"]; !ok {
		t.Fatalf("expected table test_table in schema")
	}
}

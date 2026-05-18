package scriptpath

import (
	"os"
	"path/filepath"
	"testing"
)

func TestResolveScript_ScriptDirPriority(t *testing.T) {
	tmp := t.TempDir()
	configPath := filepath.Join(tmp, "config.toml")
	if err := os.WriteFile(configPath, []byte(""), 0644); err != nil {
		t.Fatalf("failed to write config: %v", err)
	}

	scriptDir := t.TempDir()
	file := filepath.Join(scriptDir, "s.lua")
	if err := os.WriteFile(file, []byte("-- a"), 0644); err != nil {
		t.Fatalf("failed to write script in scriptdir: %v", err)
	}

	// also put in config/scripts
	scriptsDir := filepath.Join(tmp, "scripts")
	if err := os.Mkdir(scriptsDir, 0755); err != nil {
		t.Fatalf("failed to create scripts dir: %v", err)
	}
	cfgFile := filepath.Join(scriptsDir, "s.lua")
	if err := os.WriteFile(cfgFile, []byte("-- b"), 0644); err != nil {
		t.Fatalf("failed to write script in config/scripts: %v", err)
	}

	resolved, err := Resolve("s.lua", configPath, scriptDir)
	if err != nil {
		t.Fatalf("Resolve error: %v", err)
	}
	if filepath.Clean(resolved) != filepath.Clean(file) {
		t.Fatalf("got %v want %v", resolved, file)
	}
}

func TestResolveScript_FallbackToConfigScripts(t *testing.T) {
	tmp := t.TempDir()
	configPath := filepath.Join(tmp, "config.toml")
	if err := os.WriteFile(configPath, []byte(""), 0644); err != nil {
		t.Fatalf("failed to write config: %v", err)
	}

	// Create scripts dir and file
	scriptsDir := filepath.Join(tmp, "scripts")
	if err := os.Mkdir(scriptsDir, 0755); err != nil {
		t.Fatalf("failed to make scripts dir: %v", err)
	}
	fileInScripts := filepath.Join(scriptsDir, "other.lua")
	if err := os.WriteFile(fileInScripts, []byte("-- lua"), 0644); err != nil {
		t.Fatalf("failed to write script in scripts dir: %v", err)
	}

	resolved, err := Resolve("other.lua", configPath, "")
	if err != nil {
		t.Fatalf("Resolve returned error: %v", err)
	}
	if filepath.Clean(resolved) != filepath.Clean(fileInScripts) {
		t.Fatalf("Resolve = %v, want %v", resolved, fileInScripts)
	}
}

func TestResolveScript_NotFound(t *testing.T) {
	tmp := t.TempDir()
	configPath := filepath.Join(tmp, "config.toml")
	if err := os.WriteFile(configPath, []byte(""), 0644); err != nil {
		t.Fatalf("failed to write config: %v", err)
	}

	_, err := Resolve("doesnotexist.lua", configPath, "")
	if err == nil {
		t.Fatalf("expected error for non-existent script")
	}
}

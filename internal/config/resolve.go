package config

import (
	"fmt"
	"os"
	"path/filepath"
)

// ResolveScriptPath finds the actual file path for a Lua script referenced in the
// configuration. Search order:
// 1) If script is empty, return empty string
// 2) If script exists as provided, return absolute path
// 3) If scriptDir provided: check scriptDir/script and scriptDir/<basename(script)>
// 4) Check <configDir>/script and <configDir>/<basename(script)>
// 5) Check <configDir>/scripts/script and <configDir>/scripts/<basename(script)>
// 6) Return error if not found
func ResolveScriptPath(script string, configPath string, scriptDir string) (string, error) {
	if script == "" {
		return "", nil
	}

	// If file exists as given
	if fileExists(script) {
		abs, err := filepath.Abs(script)
		if err == nil {
			return abs, nil
		}
		return script, nil
	}

	base := filepath.Base(script)
	// If scriptDir provided, try there first
	if scriptDir != "" {
		candidates := []string{
			filepath.Join(scriptDir, script),
			filepath.Join(scriptDir, base),
		}
		for _, c := range candidates {
			if fileExists(c) {
				abs, _ := filepath.Abs(c)
				return abs, nil
			}
		}
	}

	// Use config dir
	configDir := filepath.Dir(configPath)
	candidates := []string{
		filepath.Join(configDir, script),
		filepath.Join(configDir, base),
		filepath.Join(configDir, "scripts", script),
		filepath.Join(configDir, "scripts", base),
	}
	for _, c := range candidates {
		if fileExists(c) {
			abs, _ := filepath.Abs(c)
			return abs, nil
		}
	}

	return "", fmt.Errorf("script not found: %s (looked in scriptdir=%s and config dir=%s)", script, scriptDir, configDir)
}

func fileExists(path string) bool {
	if path == "" {
		return false
	}
	if _, err := os.Stat(path); err == nil {
		return true
	}
	return false
}

package scriptpath

import (
	"fmt"
	"os"
	"path/filepath"
)

// Resolve finds the actual file path for a Lua script referenced in the
// configuration. It searches in the following order:
// 1) If script is an absolute path or exists as provided, return it.
// 2) If scriptDir is non-empty, check scriptDir/script and scriptDir/<basename(script)>.
// 3) Check <configDir>/script and <configDir>/<basename(script)>.
// 4) Check <configDir>/scripts/script and <configDir>/scripts/<basename(script)>.
// 5) As a last resort, return an error.
//
// If script is empty, Resolve returns an empty string and no error (passthrough).
func Resolve(script string, configPath string, scriptDir string) (string, error) {
	if script == "" {
		return "", nil
	}

	// If file exists as given (absolute or relative to cwd), return it
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
				return filepath.Abs(c)
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
			return filepath.Abs(c)
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

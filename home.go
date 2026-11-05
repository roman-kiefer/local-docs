package main

import (
	"embed"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"runtime"
)

//go:embed all:.agents/skills config.example.json
var embeddedFS embed.FS

// dataDirOverride is set once at startup from config.json (data_dir).
var dataDirOverride string

func dataDir(root string) string {
	if dataDirOverride != "" {
		return dataDirOverride
	}
	return filepath.Join(root, "data")
}

// defaultHome is the per-user directory used when nothing else selects a root.
func defaultHome() (string, error) {
	if runtime.GOOS == "windows" {
		if d := os.Getenv("LOCALAPPDATA"); d != "" {
			return filepath.Join(d, "local-docs"), nil
		}
	}
	d, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(d, "local-docs"), nil
}

// resolveRoot picks the directory holding config.json: --root, LOCAL_DOCS_HOME,
// the working directory (if it already has a config.json), else the user default.
func resolveRoot(flagValue string) (string, error) {
	pick := flagValue
	if pick == "" {
		pick = os.Getenv("LOCAL_DOCS_HOME")
	}
	if pick == "" {
		if _, err := os.Stat("config.json"); err == nil {
			pick = "."
		}
	}
	if pick == "" {
		d, err := defaultHome()
		if err != nil {
			return "", err
		}
		pick = d
	}
	return filepath.Abs(pick)
}

// applyDataDir reads the optional data_dir from config.json (relative paths are
// relative to root). Errors are ignored here; readConfig reports config problems.
func applyDataDir(root string) {
	b, err := os.ReadFile(filepath.Join(root, "config.json"))
	if err != nil {
		return
	}
	var c struct {
		DataDir string `json:"data_dir"`
	}
	if json.Unmarshal(b, &c) != nil || c.DataDir == "" {
		return
	}
	if !filepath.IsAbs(c.DataDir) {
		c.DataDir = filepath.Join(root, c.DataDir)
	}
	dataDirOverride = filepath.Clean(c.DataDir)
}

var errNoExe = errors.New("cannot determine executable path")

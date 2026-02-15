package utils

import (
	"os"
	"path/filepath"
)

func GetProjectPath() string {
	d, err := os.UserHomeDir()

	if err != nil {
		d = "."
	}
	return filepath.Join(d, ".config", "lazylang")
}

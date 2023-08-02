package config

import (
	"os"
)

func DetermineEditor(cfg Config) (string, error) {
	editorCommand := os.Getenv("AZDO_EDITOR")
	if editorCommand == "" {
		editorCommand, _ = cfg.Get([]string{"", "editor"})
	}
	return editorCommand, nil
}

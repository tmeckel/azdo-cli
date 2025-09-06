package shared

import (
	"errors"

	"github.com/tmeckel/azdo-cli/internal/cmd/util"
)

func InitEditorMode(ctx util.CmdContext, editorMode bool, webMode bool, canPrompt bool) (bool, error) {
	if err := util.MutuallyExclusive(
		"specify only one of `--editor` or `--web`",
		editorMode,
		webMode,
	); err != nil {
		return false, err //nolint:error,wrapcheck
	}

	editorMode = !webMode && editorMode

	if editorMode && !canPrompt {
		return false, errors.New("--editor is not supported in non-tty mode")
	}

	return editorMode, nil
}

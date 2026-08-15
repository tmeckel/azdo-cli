package shared

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"unicode/utf8"

	"github.com/tmeckel/azdo-cli/internal/cmd/util"
	"github.com/tmeckel/azdo-cli/internal/iostreams"
)

const (
	maxDescriptionBytes = 1024 * 1024
	binaryCheckBytes    = 8 * 1024
)

// editorHeaderLines pre-populate the editor temp file. Only these exact lines
// are stripped when the description is read back; user Markdown headings
// starting with '#' are preserved.
var editorHeaderLines = []string{
	"# Enter the description for the work item below.",
	"# This header is removed automatically when the description is saved.",
}

// editorHeader is the text written to the temp file before the editor opens.
var editorHeader = strings.Join(editorHeaderLines, "\n") + "\n"

// ExecEditorCommand runs the resolved editor command against the given file.
// It is a package-level variable so tests can replace it with a fake.
var ExecEditorCommand = func(command []string, file string) error {
	cmd := exec.Command(command[0], append(command[1:], file)...)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

// DescriptionOptions collects the three description input sources.
type DescriptionOptions struct {
	Inline string   // --description
	Files  []string // --description-file (repeatable; "-" reads stdin)
	Editor bool     // --description-editor
	// EditorCommand is the editor resolved from configuration (AZDO_EDITOR /
	// config "editor" key). It takes precedence over $VISUAL/$EDITOR.
	EditorCommand string
}

// ResolveDescription returns the description from the highest-priority source
// (editor > file > inline) as raw input text without any format conversion,
// and warns on stderr when a lower-priority source is ignored. Returns ""
// when no source is configured. The caller emits the raw value for
// /fields/System.Description together with a /multilineFieldsFormat/
// System.Description op whose value is selected by NormalizeDescriptionFormat.
func ResolveDescription(ios *iostreams.IOStreams, opts DescriptionOptions) (string, error) {
	var raw string
	switch {
	case opts.Editor:
		switch {
		case len(opts.Files) > 0:
			fmt.Fprintf(ios.ErrOut, "warning: --description-editor takes precedence over --description-file\n")
		case opts.Inline != "":
			fmt.Fprintf(ios.ErrOut, "warning: --description-editor takes precedence over --description\n")
		}
		description, err := OpenEditor(opts.EditorCommand)
		if err != nil {
			return "", err
		}
		raw = description
	case len(opts.Files) > 0:
		if opts.Inline != "" {
			fmt.Fprintf(ios.ErrOut, "warning: --description-file takes precedence over --description\n")
		}
		description, err := ReadDescriptionFiles(ios, opts.Files)
		if err != nil {
			return "", err
		}
		raw = description
	default:
		raw = opts.Inline
	}
	return raw, nil
}

// descriptionFormatOpValues maps --description-format flag values to the JSON
// Patch value required by the /multilineFieldsFormat operation.
var descriptionFormatOpValues = map[string]string{
	"markdown": "Markdown",
	"html":     "Html",
}

// NormalizeDescriptionFormat validates the --description-format flag value
// and returns the JSON Patch value for the /multilineFieldsFormat operation.
// An empty value selects the default ("markdown"). Comparison is
// case-insensitive. Note: the Azure Boards API does not allow switching an
// existing Markdown field back to html; the server rejects such updates.
func NormalizeDescriptionFormat(format string) (string, error) {
	if format == "" {
		return "Markdown", nil
	}
	opValue, ok := descriptionFormatOpValues[strings.ToLower(format)]
	if !ok {
		return "", util.FlagErrorf("--description-format must be \"markdown\" or \"html\", got %q", format)
	}
	return opValue, nil
}

// ReadDescriptionFiles concatenates the given files (in order) with "\n".
// The token "-" reads from stdin. Files are validated against a 1 MB size cap,
// binary content, and invalid UTF-8.
func ReadDescriptionFiles(ios *iostreams.IOStreams, files []string) (string, error) {
	parts := make([]string, 0, len(files))
	for _, file := range files {
		var data []byte
		var err error
		if file == "-" {
			data, err = io.ReadAll(ios.In)
		} else {
			data, err = os.ReadFile(file) //nolint:gosec // path is user-supplied by design (--description-file)
		}
		if err != nil {
			return "", util.FlagErrorf("failed to read description file %q: %v", file, err)
		}
		if len(data) > maxDescriptionBytes {
			return "", util.FlagErrorf("description file %q exceeds 1 MB limit", file)
		}
		if bytes.IndexByte(data[:min(len(data), binaryCheckBytes)], 0) >= 0 {
			return "", util.FlagErrorf("description file %q appears to be binary", file)
		}
		if !utf8.Valid(data) {
			return "", util.FlagErrorf("description file %q is not valid UTF-8", file)
		}
		parts = append(parts, string(data))
	}
	return strings.Join(parts, "\n"), nil
}

// OpenEditor opens a .md temp file pre-populated with a header comment in the
// configured editor (config "editor" key / AZDO_EDITOR), falling back to
// $VISUAL, then $EDITOR, then vi/notepad. Only the generated header lines are
// stripped on read-back; Markdown headings the user writes are preserved. An
// empty result is an error.
func OpenEditor(editorCommand string) (string, error) {
	editor := editorCommand
	if editor == "" {
		editor = os.Getenv("VISUAL")
	}
	if editor == "" {
		editor = os.Getenv("EDITOR")
	}
	if editor == "" {
		if runtime.GOOS == "windows" {
			editor = "notepad"
		} else {
			editor = "vi"
		}
	}

	file, err := os.CreateTemp("", "azdo-description-*.md")
	if err != nil {
		return "", fmt.Errorf("failed to create temporary file: %w", err)
	}
	defer os.Remove(file.Name())

	if _, err := file.WriteString(editorHeader); err != nil {
		file.Close()
		return "", fmt.Errorf("failed to write temporary file: %w", err)
	}
	if err := file.Close(); err != nil {
		return "", fmt.Errorf("failed to close temporary file: %w", err)
	}

	if err := ExecEditorCommand(strings.Fields(editor), file.Name()); err != nil {
		return "", fmt.Errorf("failed to run editor %q: %w", editor, err)
	}

	data, err := os.ReadFile(file.Name())
	if err != nil {
		return "", fmt.Errorf("failed to read edited description: %w", err)
	}

	var kept []string
	for _, line := range strings.Split(string(data), "\n") {
		if isEditorHeaderLine(line) {
			continue
		}
		kept = append(kept, line)
	}
	description := strings.TrimSpace(strings.Join(kept, "\n"))
	if description == "" {
		return "", util.FlagErrorf("editor produced empty description")
	}
	return description, nil
}

// isEditorHeaderLine reports whether line is one of the generated header
// lines written to the editor temp file.
func isEditorHeaderLine(line string) bool {
	for _, headerLine := range editorHeaderLines {
		if line == headerLine {
			return true
		}
	}
	return false
}

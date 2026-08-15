package shared

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/tmeckel/azdo-cli/internal/iostreams"
)

func TestResolveDescription_ReturnsRawMarkdown(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		in   string
	}{
		{name: "empty", in: ""},
		{name: "plain text", in: "hello"},
		{name: "paragraphs", in: "one\n\ntwo"},
		{name: "headings", in: "# Title\n\nbody"},
		{name: "emphasis", in: "**bold** and *em*"},
		{name: "list", in: "- a\n- b"},
		{name: "raw html preserved", in: "<script>alert('x')</script>"},
	}
	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			ios, _, _, _ := iostreams.Test()
			got, err := ResolveDescription(ios, DescriptionOptions{Inline: tc.in})
			require.NoError(t, err)
			assert.Equal(t, tc.in, got)
		})
	}
}

func TestResolveDescription_FileRawAndPrecedenceWarning(t *testing.T) {
	ios, _, _, errOut := iostreams.Test()

	file := filepath.Join(t.TempDir(), "desc.md")
	require.NoError(t, os.WriteFile(file, []byte("file **bold**"), 0o600))

	got, err := ResolveDescription(ios, DescriptionOptions{Inline: "inline", Files: []string{file}})
	require.NoError(t, err)
	assert.Equal(t, "file **bold**", got)
	assert.Contains(t, errOut.String(), "takes precedence over --description")
}

func TestResolveDescription_EditorOverFile_PrecedenceWarning(t *testing.T) {
	file := filepath.Join(t.TempDir(), "desc.md")
	require.NoError(t, os.WriteFile(file, []byte("file content"), 0o600))

	original := ExecEditorCommand
	t.Cleanup(func() { ExecEditorCommand = original })
	ExecEditorCommand = fakeEditor("editor content")

	ios, _, _, errOut := iostreams.Test()
	got, err := ResolveDescription(ios, DescriptionOptions{Editor: true, Files: []string{file}})
	require.NoError(t, err)
	assert.Equal(t, "editor content", got)
	assert.Contains(t, errOut.String(), "takes precedence over --description-file")
}

func TestResolveDescription_EditorOverInline_PrecedenceWarning(t *testing.T) {
	original := ExecEditorCommand
	t.Cleanup(func() { ExecEditorCommand = original })
	ExecEditorCommand = fakeEditor("editor content")

	ios, _, _, errOut := iostreams.Test()
	got, err := ResolveDescription(ios, DescriptionOptions{Editor: true, Inline: "inline content"})
	require.NoError(t, err)
	assert.Equal(t, "editor content", got)
	assert.Contains(t, errOut.String(), "takes precedence over --description")
}

func TestReadDescriptionFiles_Single(t *testing.T) {
	t.Parallel()

	file := filepath.Join(t.TempDir(), "desc.md")
	require.NoError(t, os.WriteFile(file, []byte("file **bold**"), 0o600))

	ios, _, _, _ := iostreams.Test()
	got, err := ReadDescriptionFiles(ios, []string{file})
	require.NoError(t, err)
	assert.Equal(t, "file **bold**", got)
}

func TestReadDescriptionFiles_Stdin(t *testing.T) {
	t.Parallel()

	ios, in, _, _ := iostreams.Test()
	in.WriteString("stdin content")

	got, err := ReadDescriptionFiles(ios, []string{"-"})
	require.NoError(t, err)
	assert.Equal(t, "stdin content", got)
}

func TestReadDescriptionFiles_MultipleConcatenated(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	fileA := filepath.Join(dir, "a.md")
	fileB := filepath.Join(dir, "b.md")
	require.NoError(t, os.WriteFile(fileA, []byte("alpha"), 0o600))
	require.NoError(t, os.WriteFile(fileB, []byte("beta"), 0o600))

	ios, _, _, _ := iostreams.Test()
	got, err := ReadDescriptionFiles(ios, []string{fileA, fileB})
	require.NoError(t, err)
	assert.Equal(t, "alpha\nbeta", got)
}

func TestReadDescriptionFiles_NotFound(t *testing.T) {
	t.Parallel()

	ios, _, _, _ := iostreams.Test()
	_, err := ReadDescriptionFiles(ios, []string{filepath.Join(t.TempDir(), "missing.md")})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to read description file")
}

func TestReadDescriptionFiles_TooLarge(t *testing.T) {
	t.Parallel()

	file := filepath.Join(t.TempDir(), "big.md")
	require.NoError(t, os.WriteFile(file, bytes.Repeat([]byte("a"), 1024*1024+1), 0o600))

	ios, _, _, _ := iostreams.Test()
	_, err := ReadDescriptionFiles(ios, []string{file})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "exceeds 1 MB limit")
}

func TestReadDescriptionFiles_Binary(t *testing.T) {
	t.Parallel()

	file := filepath.Join(t.TempDir(), "bin.md")
	require.NoError(t, os.WriteFile(file, []byte("a\x00b"), 0o600))

	ios, _, _, _ := iostreams.Test()
	_, err := ReadDescriptionFiles(ios, []string{file})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "appears to be binary")
}

func TestReadDescriptionFiles_NotUTF8(t *testing.T) {
	t.Parallel()

	file := filepath.Join(t.TempDir(), "raw.md")
	require.NoError(t, os.WriteFile(file, []byte{0xff, 0xfe, 0xfd}, 0o600))

	ios, _, _, _ := iostreams.Test()
	_, err := ReadDescriptionFiles(ios, []string{file})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "not valid UTF-8")
}

func TestOpenEditor_StripsOnlyGeneratedHeaderLines(t *testing.T) {
	original := ExecEditorCommand
	t.Cleanup(func() { ExecEditorCommand = original })
	ExecEditorCommand = fakeEditor(editorHeader + "# My heading\n\nbody **text**\n")

	ios, _, _, _ := iostreams.Test()
	got, err := ResolveDescription(ios, DescriptionOptions{Editor: true})
	require.NoError(t, err)
	assert.Equal(t, "# My heading\n\nbody **text**", got)
}

func TestOpenEditor_PreservesMarkdownHeadings(t *testing.T) {
	original := ExecEditorCommand
	t.Cleanup(func() { ExecEditorCommand = original })
	ExecEditorCommand = fakeEditor("# Heading one\n## Heading two\n\n#comment-looking\n")

	ios, _, _, _ := iostreams.Test()
	got, err := ResolveDescription(ios, DescriptionOptions{Editor: true})
	require.NoError(t, err)
	assert.Equal(t, "# Heading one\n## Heading two\n\n#comment-looking", got)
}

func TestOpenEditor_HeaderOnlyIsEmptyError(t *testing.T) {
	original := ExecEditorCommand
	t.Cleanup(func() { ExecEditorCommand = original })
	ExecEditorCommand = fakeEditor(editorHeader)

	_, err := OpenEditor("")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "editor produced empty description")
}

func TestOpenEditor_NonZeroExit(t *testing.T) {
	original := ExecEditorCommand
	t.Cleanup(func() { ExecEditorCommand = original })
	ExecEditorCommand = func(_ []string, _ string) error {
		return errors.New("exit status 1")
	}

	_, err := OpenEditor("")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to run editor")
	assert.Contains(t, err.Error(), "exit status 1")
}

func TestOpenEditor_UsesResolvedEditorCommand(t *testing.T) {
	var got []string
	original := ExecEditorCommand
	t.Cleanup(func() { ExecEditorCommand = original })
	ExecEditorCommand = func(command []string, file string) error {
		got = command
		return os.WriteFile(file, []byte("content"), 0o600)
	}

	description, err := OpenEditor("myeditor --wait")
	require.NoError(t, err)
	assert.Equal(t, []string{"myeditor", "--wait"}, got)
	assert.Equal(t, "content", description)
}

func TestOpenEditor_EditorCommandFallback(t *testing.T) {
	captureFirstArg := func(t *testing.T, want string) {
		t.Helper()

		var got []string
		original := ExecEditorCommand
		t.Cleanup(func() { ExecEditorCommand = original })
		ExecEditorCommand = func(command []string, file string) error {
			got = command
			return os.WriteFile(file, []byte("body"), 0o600)
		}

		_, err := OpenEditor("")
		require.NoError(t, err)
		require.NotEmpty(t, got)
		assert.Equal(t, want, got[0])
	}

	t.Run("uses VISUAL when set", func(t *testing.T) {
		t.Setenv("VISUAL", "visual-editor")
		t.Setenv("EDITOR", "")
		captureFirstArg(t, "visual-editor")
	})

	t.Run("uses EDITOR when set", func(t *testing.T) {
		t.Setenv("VISUAL", "")
		t.Setenv("EDITOR", "editor-bin")
		captureFirstArg(t, "editor-bin")
	})

	t.Run("defaults to platform editor", func(t *testing.T) {
		t.Setenv("VISUAL", "")
		t.Setenv("EDITOR", "")
		want := "vi"
		if runtime.GOOS == "windows" {
			want = "notepad"
		}
		captureFirstArg(t, want)
	})
}

func TestNormalizeDescriptionFormat(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		input   string
		want    string
		wantErr bool
	}{
		{name: "empty defaults to markdown", input: "", want: "Markdown"},
		{name: "markdown lowercase", input: "markdown", want: "Markdown"},
		{name: "markdown capitalized", input: "Markdown", want: "Markdown"},
		{name: "html lowercase", input: "html", want: "Html"},
		{name: "html uppercase", input: "HTML", want: "Html"},
		{name: "invalid plaintext", input: "plaintext", wantErr: true},
		{name: "invalid markdown with space", input: "md ", wantErr: true},
	}
	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got, err := NormalizeDescriptionFormat(tc.input)
			if tc.wantErr {
				require.Error(t, err)
				assert.Contains(t, err.Error(), `--description-format must be "markdown" or "html"`)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tc.want, got)
		})
	}
}

func fakeEditor(content string) func([]string, string) error {
	return func(_ []string, file string) error {
		return os.WriteFile(file, []byte(content), 0o600)
	}
}

package template

import (
	"bytes"
	"fmt"
	"io"
	"strings"
	"testing"
	"time"

	"github.com/MakeNowJust/heredoc/v2"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func ptr(s string) *string { return &s }

func TestJsonScalarToString(t *testing.T) {
	tests := []struct {
		name    string
		input   any
		want    string
		wantErr bool
	}{
		{
			name:  "string",
			input: "hello",
			want:  "hello",
		},
		{
			name:  "int",
			input: float64(1234),
			want:  "1234",
		},
		{
			name:  "float",
			input: float64(12.34),
			want:  "12.34",
		},
		{
			name:  "null",
			input: nil,
			want:  "",
		},
		{
			name:  "true",
			input: true,
			want:  "true",
		},
		{
			name:  "false",
			input: false,
			want:  "false",
		},
		{
			name:    "object",
			input:   map[string]any{},
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := jsonScalarToString(tt.input)
			if tt.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestExecute(t *testing.T) {
	type args struct {
		json     io.Reader
		template string
		colorize bool
	}
	tests := []struct {
		name    string
		args    args
		wantW   string
		wantErr bool
	}{
		{
			name: "color",
			args: args{
				json:     strings.NewReader(`{}`),
				template: `{{color "blue+h" "songs are like tattoos"}}`,
			},
			wantW: "\x1b[0;94msongs are like tattoos\x1b[0m",
		},
		{
			name: "autocolor enabled",
			args: args{
				json:     strings.NewReader(`{}`),
				template: `{{autocolor "red" "stop"}}`,
				colorize: true,
			},
			wantW: "\x1b[0;31mstop\x1b[0m",
		},
		{
			name: "autocolor disabled",
			args: args{
				json:     strings.NewReader(`{}`),
				template: `{{autocolor "red" "go"}}`,
			},
			wantW: "go",
		},
		{
			name: "timefmt",
			args: args{
				json:     strings.NewReader(`{"created_at":"2008-02-25T20:18:33Z"}`),
				template: `{{.created_at | timefmt "Mon Jan 2, 2006"}}`,
			},
			wantW: "Mon Feb 25, 2008",
		},
		{
			name: "timeago",
			args: args{
				json:     strings.NewReader(fmt.Sprintf(`{"created_at":"%s"}`, time.Now().Add(-5*time.Minute).Format(time.RFC3339))),
				template: `{{.created_at | timeago}}`,
			},
			wantW: "5 minutes ago",
		},
		{
			name: "pluck",
			args: args{
				json: strings.NewReader(heredoc.Doc(`[
					{"name": "bug"},
					{"name": "feature request"},
					{"name": "chore"}
				]`)),
				template: `{{range(pluck "name" .)}}{{. | printf "%s\n"}}{{end}}`,
			},
			wantW: "bug\nfeature request\nchore\n",
		},
		{
			name: "join",
			args: args{
				json:     strings.NewReader(`[ "bug", "feature request", "chore" ]`),
				template: `{{join "\t" .}}`,
			},
			wantW: "bug\tfeature request\tchore",
		},
		{
			name: "table",
			args: args{
				json: strings.NewReader(heredoc.Doc(`[
					{"number": 1, "title": "One"},
					{"number": 20, "title": "Twenty"},
					{"number": 3000, "title": "Three thousand"}
				]`)),
				template: `{{range .}}{{tablerow (.number | printf "#%v") .title}}{{end}}`,
			},
			wantW: heredoc.Doc(`#1     One
			#20    Twenty
			#3000  Three thousand
			`),
		},
		{
			name: "table with multiline text",
			args: args{
				json: strings.NewReader(heredoc.Doc(`[
					{"number": 1, "title": "One\ranother line of text"},
					{"number": 20, "title": "Twenty\nanother line of text"},
					{"number": 3000, "title": "Three thousand\r\nanother line of text"}
				]`)),
				template: `{{range .}}{{tablerow (.number | printf "#%v") .title}}{{end}}`,
			},
			wantW: heredoc.Doc(`#1     One...
			#20    Twenty...
			#3000  Three thousand...
			`),
		},
		{
			name: "table with mixed value types",
			args: args{
				json: strings.NewReader(heredoc.Doc(`[
					{"number": 1, "title": null, "float": false},
					{"number": 20.1, "title": "Twenty-ish", "float": true},
					{"number": 3000, "title": "Three thousand", "float": false}
				]`)),
				template: `{{range .}}{{tablerow .number .title .float}}{{end}}`,
			},
			wantW: heredoc.Doc(`1                      false
			20.10  Twenty-ish      true
			3000   Three thousand  false
			`),
		},
		{
			name: "table with color",
			args: args{
				json: strings.NewReader(heredoc.Doc(`[
					{"number": 1, "title": "One"}
				]`)),
				template: `{{range .}}{{tablerow (.number | color "green") .title}}{{end}}`,
			},
			wantW: "\x1b[0;32m1\x1b[0m  One\n",
		},
		{
			name: "table with header and footer",
			args: args{
				json: strings.NewReader(heredoc.Doc(`[
					{"number": 1, "title": "One"},
					{"number": 2, "title": "Two"}
				]`)),
				template: heredoc.Doc(`HEADER
				{{range .}}{{tablerow .number .title}}{{end}}FOOTER
				`),
			},
			wantW: heredoc.Doc(`HEADER
			FOOTER
			1  One
			2  Two
			`),
		},
		{
			name: "table with header and footer using endtable",
			args: args{
				json: strings.NewReader(heredoc.Doc(`[
					{"number": 1, "title": "One"},
					{"number": 2, "title": "Two"}
				]`)),
				template: heredoc.Doc(`HEADER
				{{range .}}{{tablerow .number .title}}{{end}}{{tablerender}}FOOTER
				`),
			},
			wantW: heredoc.Doc(`HEADER
			1  One
			2  Two
			FOOTER
			`),
		},
		{
			name: "multiple tables with different columns",
			args: args{
				json: strings.NewReader(heredoc.Doc(`{
					"issues": [
						{"number": 1, "title": "One"},
						{"number": 2, "title": "Two"}
					],
					"prs": [
						{"number": 3, "title": "Three", "reviewDecision": "REVIEW_REQUESTED"},
						{"number": 4, "title": "Four", "reviewDecision": "CHANGES_REQUESTED"}
					]
				}`)),
				template: heredoc.Doc(`{{tablerow "ISSUE" "TITLE"}}{{range .issues}}{{tablerow .number .title}}{{end}}{{tablerender}}
				{{tablerow "PR" "TITLE" "DECISION"}}{{range .prs}}{{tablerow .number .title .reviewDecision}}{{end}}`),
			},
			wantW: heredoc.Docf(`ISSUE  TITLE
			1      One
			2      Two

			PR  TITLE  DECISION
			3   Three  REVIEW_REQUESTED
			4   Four   CHANGES_REQUESTED
			`),
		},
		{
			name: "truncate",
			args: args{
				json:     strings.NewReader(`{"title": "This is a long title"}`),
				template: `{{truncate 13 .title}}`,
			},
			wantW: "This is a ...",
		},
		{
			name: "truncate with JSON null",
			args: args{
				json:     strings.NewReader(`{}`),
				template: `{{ truncate 13 .title }}`,
			},
			wantW: "",
		},
		{
			name: "truncate with piped JSON null",
			args: args{
				json:     strings.NewReader(`{}`),
				template: `{{ .title | truncate 13 }}`,
			},
			wantW: "",
		},
		{
			name: "truncate with piped JSON null in parenthetical",
			args: args{
				json:     strings.NewReader(`{}`),
				template: `{{ (.title | truncate 13) }}`,
			},
			wantW: "",
		},
		{
			name: "truncate invalid type",
			args: args{
				json:     strings.NewReader(`{"title": 42}`),
				template: `{{ (.title | truncate 13) }}`,
			},
			wantErr: true,
		},
		{
			name: "hyperlink enabled",
			args: args{
				json:     strings.NewReader(`{"link":"https://github.com"}`),
				template: `{{ hyperlink .link "" }}`,
			},
			wantW: "\x1b]8;;https://github.com\x1b\\https://github.com\x1b]8;;\x1b\\",
		},
		{
			name: "hyperlink with text enabled",
			args: args{
				json:     strings.NewReader(`{"link":"https://github.com","text":"GitHub"}`),
				template: `{{ hyperlink .link .text }}`,
			},
			wantW: "\x1b]8;;https://github.com\x1b\\GitHub\x1b]8;;\x1b\\",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w := &bytes.Buffer{}
			tmpl := New(w, 80, tt.args.colorize)
			err := tmpl.Parse(tt.args.template)
			require.NoError(t, err)
			err = tmpl.Execute(tt.args.json)
			if tt.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			err = tmpl.Flush()
			require.NoError(t, err)
			assert.Equal(t, tt.wantW, w.String())
		})
	}
}

func TestTruncateMultiline(t *testing.T) {
	type args struct {
		max int
		s   string
	}
	tests := []struct {
		name string
		args args
		want string
	}{
		{
			name: "exactly minimum width",
			args: args{
				max: 5,
				s:   "short",
			},
			want: "short",
		},
		{
			name: "exactly minimum width with new line",
			args: args{
				max: 5,
				s:   "short\n",
			},
			want: "sh...",
		},
		{
			name: "less than minimum width",
			args: args{
				max: 4,
				s:   "short",
			},
			want: "shor",
		},
		{
			name: "less than minimum width with new line",
			args: args{
				max: 4,
				s:   "short\n",
			},
			want: "shor",
		},
		{
			name: "first line of multiple is short enough",
			args: args{
				max: 80,
				s:   "short\n\nthis is a new line",
			},
			want: "short...",
		},
		{
			name: "using Windows line endings",
			args: args{
				max: 80,
				s:   "short\r\n\r\nthis is a new line",
			},
			want: "short...",
		},
		{
			name: "using older MacOS line endings",
			args: args{
				max: 80,
				s:   "short\r\rthis is a new line",
			},
			want: "short...",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := truncateMultiline(tt.args.max, tt.args.s)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestFuncs(t *testing.T) {
	w := &bytes.Buffer{}
	tmpl := New(w, 80, false)

	// Override "truncate" and define a new "foo" function.
	tmpl.WithFuncs(map[string]any{
		"truncate": func(fields ...any) (string, error) {
			if l := len(fields); l != 2 {
				return "", fmt.Errorf("wrong number of args for truncate: want 2 got %d", l)
			}
			var ok bool
			var width int
			var input string
			if width, ok = fields[0].(int); !ok {
				return "", fmt.Errorf("invalid value; expected int")
			}
			if input, ok = fields[1].(string); !ok {
				return "", fmt.Errorf("invalid value; expected string")
			}
			return input[:width], nil
		},
		"foo": func(fields ...any) (string, error) {
			return "test", nil
		},
	})

	err := tmpl.Parse(`{{ .text | truncate 5 }} {{ .status | color "green" }} {{ foo }}`)
	require.NoError(t, err)

	r := strings.NewReader(`{"text":"truncated","status":"open"}`)
	err = tmpl.Execute(r)
	require.NoError(t, err)

	err = tmpl.Flush()
	require.NoError(t, err)
	assert.Equal(t, "trunc \x1b[0;32mopen\x1b[0m test", w.String())
}

func TestHasText(t *testing.T) {
	tests := []struct {
		name string
		v    any
		want bool
	}{
		{name: "nil", v: nil, want: false},
		{name: "non-nil non-pointer", v: 42, want: true},
		{name: "non-nil struct", v: struct{}{}, want: true},
		{name: "empty string", v: "", want: false},
		{name: "whitespace string", v: "  \t\n  ", want: false},
		{name: "non-empty string", v: "hello", want: true},
		{name: "nil *string", v: (*string)(nil), want: false},
		{name: "non-nil *string with empty value", v: ptr(""), want: false},
		{name: "non-nil *string with whitespace", v: ptr("  "), want: false},
		{name: "non-nil *string with text", v: ptr("hello"), want: true},
		{name: "nil *bool", v: (*bool)(nil), want: false},
		{name: "non-nil *bool", v: func() *bool { b := true; return &b }(), want: true},
		{name: "nil *int", v: (*int)(nil), want: false},
		{name: "pure whitespace string single char", v: " ", want: false},
		{name: "pure whitespace string tab", v: "\t", want: false},
		{name: "pure whitespace string newline", v: "\n", want: false},
		{name: "non-nil *string with newlines", v: ptr("\n\n"), want: false},
		{name: "empty interface value", v: any(""), want: false},
		{name: "interface with non-empty string", v: any("text"), want: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, HasText(tt.v))
		})
	}
}

func TestHasText_templateIntegration(t *testing.T) {
	tests := []struct {
		name   string
		tpl    string
		fields any
		want   string
	}{
		{
			name: "hasText returns false for empty string — block hidden",
			tpl:  `{{if hasText .Name}}SHOW{{end}}`,
			fields: struct {
				Name string
				Val  *string
			}{Name: "", Val: nil},
			want: "",
		},
		{
			name: "hasText returns true for non-empty string — block shown",
			tpl:  `{{if hasText .Name}}{{.Name}}{{end}}`,
			fields: struct {
				Name string
			}{Name: "hello"},
			want: "hello",
		},
		{
			name: "hasText returns false for nil *string — block hidden",
			tpl:  `{{if hasText .Val}}SHOW{{end}}`,
			fields: struct {
				Name string
				Val  *string
			}{Name: "hello", Val: nil},
			want: "",
		},
		{
			name: "hasText returns false for whitespace-only *string — block hidden",
			tpl:  `{{if hasText .Val}}SHOW{{end}}`,
			fields: struct {
				Val *string
			}{Val: ptr("  ")},
			want: "",
		},
		{
			name: "hasText returns true for non-empty *string — block shown",
			tpl:  `{{if hasText .Val}}{{.Val}}{{end}}`,
			fields: struct {
				Val *string
			}{Val: ptr("world")},
			want: "world",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var w bytes.Buffer
			tmpl := New(&w, 80, false)
			tmpl = tmpl.WithFuncs(map[string]any{"hasText": HasText})
			err := tmpl.Parse(tt.tpl)
			require.NoError(t, err)
			err = tmpl.ExecuteData(tt.fields)
			require.NoError(t, err)
			err = tmpl.Flush()
			require.NoError(t, err)
			assert.Equal(t, tt.want, w.String())
		})
	}
}

func TestStringOrEmpty(t *testing.T) {
	tests := []struct {
		name string
		v    any
		want string
	}{
		{name: "nil", v: nil, want: ""},
		{name: "non-nil non-string", v: 42, want: ""},
		{name: "non-nil struct", v: struct{}{}, want: ""},
		{name: "empty string", v: "", want: ""},
		{name: "non-empty string", v: "hello", want: "hello"},
		{name: "nil *string", v: (*string)(nil), want: ""},
		{name: "non-nil *string empty", v: ptr(""), want: ""},
		{name: "non-nil *string with text", v: ptr("world"), want: "world"},
		{name: "nil *int", v: (*int)(nil), want: ""},
		{name: "non-nil *int with value", v: func() *int { i := 42; return &i }(), want: ""},
		{name: "non-nil *bool", v: func() *bool { b := true; return &b }(), want: ""},
		{name: "whitespace string", v: "  ", want: "  "},
		{name: "nil **string", v: func() **string { return nil }(), want: ""},
		{name: "**string with inner nil", v: func() **string { s := (*string)(nil); return &s }(), want: ""},
		{name: "**string with inner value", v: func() **string { s := ptr("nested"); return &s }(), want: ""},
		{name: "nil map", v: map[string]string(nil), want: ""},
		{name: "empty slice", v: []int{}, want: ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, StringOrEmpty(tt.v))
		})
	}
}

func TestBoolString(t *testing.T) {
	tests := []struct {
		name string
		v    *bool
		want string
	}{
		{name: "nil", v: nil, want: ""},
		{name: "true", v: func() *bool { b := true; return &b }(), want: "true"},
		{name: "false", v: func() *bool { b := false; return &b }(), want: "false"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, BoolString(tt.v))
		})
	}
}

func TestUUIDString(t *testing.T) {
	id := uuid.MustParse("12345678-1234-5678-1234-567812345678")
	tests := []struct {
		name string
		v    *uuid.UUID
		want string
	}{
		{name: "nil", v: nil, want: ""},
		{name: "valid UUID", v: &id, want: "12345678-1234-5678-1234-567812345678"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, UUIDString(tt.v))
		})
	}
}

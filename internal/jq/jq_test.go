package jq

import (
	"bytes"
	"io"
	"strings"
	"testing"

	"github.com/MakeNowJust/heredoc"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestEvaluateFormatted(t *testing.T) {
	t.Setenv("CODE", "code_c")
	type args struct {
		json     io.Reader
		expr     string
		indent   string
		colorize bool
	}
	tests := []struct {
		name       string
		args       args
		wantW      string
		wantErr    bool
		wantErrMsg string
	}{
		{
			name: "simple",
			args: args{
				json:     strings.NewReader(`{"name":"Mona", "arms":8}`),
				expr:     `.name`,
				indent:   "",
				colorize: false,
			},
			wantW: "Mona\n",
		},
		{
			name: "multiple queries",
			args: args{
				json:     strings.NewReader(`{"name":"Mona", "arms":8}`),
				expr:     `.name,.arms`,
				indent:   "",
				colorize: false,
			},
			wantW: "Mona\n8\n",
		},
		{
			name: "object as JSON",
			args: args{
				json:     strings.NewReader(`{"user":{"login":"monalisa"}}`),
				expr:     `.user`,
				indent:   "",
				colorize: false,
			},
			wantW: "{\"login\":\"monalisa\"}\n",
		},
		{
			name: "object as JSON, indented",
			args: args{
				json:     strings.NewReader(`{"user":{"login":"monalisa"}}`),
				expr:     `.user`,
				indent:   "  ",
				colorize: false,
			},
			wantW: "{\n  \"login\": \"monalisa\"\n}\n",
		},
		{
			name: "object as JSON, indented & colorized",
			args: args{
				json:     strings.NewReader(`{"user":{"login":"monalisa"}}`),
				expr:     `.user`,
				indent:   "  ",
				colorize: true,
			},
			wantW: "\x1b[1;38m{\x1b[m\n" +
				"  \x1b[1;34m\"login\"\x1b[m\x1b[1;38m:\x1b[m" +
				" \x1b[32m\"monalisa\"\x1b[m\n" +
				"\x1b[1;38m}\x1b[m\n",
		},
		{
			name: "empty array",
			args: args{
				json:     strings.NewReader(`[]`),
				expr:     `., [], unique`,
				indent:   "",
				colorize: false,
			},
			wantW: "[]\n[]\n[]\n",
		},
		{
			name: "empty array, colorized",
			args: args{
				json:     strings.NewReader(`[]`),
				expr:     `.`,
				indent:   "",
				colorize: true,
			},
			wantW: "\x1b[1;38m[\x1b[m\x1b[1;38m]\x1b[m\n",
		},
		{
			name: "complex",
			args: args{
				json: strings.NewReader(heredoc.Doc(`[
					{
						"title": "First title",
						"labels": [{"name":"bug"}, {"name":"help wanted"}]
					},
					{
						"title": "Second but not last",
						"labels": []
					},
					{
						"title": "Alas, tis' the end",
						"labels": [{}, {"name":"feature"}]
					}
				]`)),
				expr:     `.[] | [.title,(.labels | map(.name) | join(","))] | @tsv`,
				indent:   "",
				colorize: false,
			},
			wantW: "First title\tbug,help wanted\nSecond but not last\t\nAlas, tis' the end\t,feature\n",
		},
		{
			name: "with env var",
			args: args{
				json: strings.NewReader(heredoc.Doc(`[
					{
						"title": "code_a",
						"labels": [{"name":"bug"}, {"name":"help wanted"}]
					},
					{
						"title": "code_b",
						"labels": []
					},
					{
						"title": "code_c",
						"labels": [{}, {"name":"feature"}]
					}
				]`)),
				expr:     `.[] | select(.title == env.CODE) | .labels`,
				indent:   "  ",
				colorize: false,
			},
			wantW: "[\n  {},\n  {\n    \"name\": \"feature\"\n  }\n]\n",
		},
		{
			name: "mixing scalars, arrays and objects",
			args: args{
				json: strings.NewReader(heredoc.Doc(`[
					"foo",
					true,
					42,
					[17, 23],
					{"foo": "bar"}
				]`)),
				expr:     `.[]`,
				indent:   "  ",
				colorize: true,
			},
			wantW: "foo\ntrue\n42\n" +
				"\x1b[1;38m[\x1b[m\n" +
				"  17\x1b[1;38m,\x1b[m\n" +
				"  23\n" +
				"\x1b[1;38m]\x1b[m\n" +
				"\x1b[1;38m{\x1b[m\n" +
				"  \x1b[1;34m\"foo\"\x1b[m\x1b[1;38m:\x1b[m" +
				" \x1b[32m\"bar\"\x1b[m\n" +
				"\x1b[1;38m}\x1b[m\n",
		},
		{
			name: "halt function",
			args: args{
				json: strings.NewReader("{}"),
				expr: `1,halt,2`,
			},
			wantW: "1\n",
		},
		{
			name: "halt_error function",
			args: args{
				json: strings.NewReader("{}"),
				expr: `1,halt_error,2`,
			},
			wantW:      "1\n",
			wantErr:    true,
			wantErrMsg: "halt error: {}",
		},
		{
			name: "invalid one-line query",
			args: args{
				json: strings.NewReader("{}"),
				expr: `[1,2,,3]`,
			},
			wantErr: true,
			wantErrMsg: `failed to parse jq expression (line 1, column 6)
    [1,2,,3]
         ^  unexpected token ","`,
		},
		{
			name: "invalid multi-line query",
			args: args{
				json: strings.NewReader("{}"),
				expr: `[
  1,,2
  ,3]`,
			},
			wantErr: true,
			wantErrMsg: `failed to parse jq expression (line 2, column 5)
      1,,2
        ^  unexpected token ","`,
		},
		{
			name: "invalid unterminated query",
			args: args{
				json: strings.NewReader("{}"),
				expr: `[1,`,
			},
			wantErr: true,
			wantErrMsg: `failed to parse jq expression (line 1, column 4)
    [1,
       ^  unexpected EOF`,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w := &bytes.Buffer{}
			err := EvaluateFormatted(tt.args.json, w, tt.args.expr, tt.args.indent, tt.args.colorize)
			if tt.wantErr {
				require.Error(t, err)
				require.EqualError(t, err, tt.wantErrMsg)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.wantW, w.String())
		})
	}
}

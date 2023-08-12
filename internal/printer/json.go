package printer

import (
	"encoding/json"
	"io"
	"time"

	"github.com/tmeckel/azdo-cli/internal/text"
)

type JsonPrinter interface {
	Printer
}

// NewJsonPrinter initializes a table printer with terminal mode and terminal width. When terminal mode is enabled, the
// output will be human-readable, column-formatted to fit available width, and rendered with color support.
// In non-terminal mode, the output is tab-separated and all truncation of values is disabled.
func NewJsonPrinter(w io.Writer) (jp JsonPrinter, err error) {
	jp = &jsonPrinter{
		out:           json.NewEncoder(w),
		columns:       []string{},
		currentColumn: -1,
		rows:          []map[string]string{},
	}
	return
}

type jsonPrinter struct {
	out           *json.Encoder
	columns       []string
	currentColumn int
	rows          []map[string]string
}

func (jp *jsonPrinter) AddColumns(columns ...string) {
	jp.columns = append(jp.columns, columns...)
}

func (jp *jsonPrinter) AddField(s string, opts ...FieldOption) {
	if jp.currentColumn < 0 {
		jp.rows = append(jp.rows, map[string]string{})
	}
	jp.currentColumn++
	rowI := len(jp.rows) - 1
	jp.rows[rowI][jp.columns[jp.currentColumn]] = s
}

func (jp *jsonPrinter) AddTimeField(now, t time.Time, c func(string) string) {
	tf := text.FuzzyAgo(now, t)
	jp.AddField(tf)
}

func (jp *jsonPrinter) EndRow() {
	jp.currentColumn = -1
}

func (jp *jsonPrinter) Render() error {
	return jp.out.Encode(jp.rows)
}

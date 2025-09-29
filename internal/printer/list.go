package printer

import (
    "fmt"
    "io"
    "time"

    "github.com/tmeckel/azdo-cli/internal/text"
)

// ListPrinter prints data as a list of key/value lines per object, separated by a blank line.
type ListPrinter interface {
    Printer
}

func NewListPrinter(w io.Writer) (lp ListPrinter, err error) {
    lp = &listPrinter{
        out:     w,
        columns: []string{},
    }
    return lp, err
}

type listPrinter struct {
    out           io.Writer
    columns       []string
    currentColumn int
    currentFields []string
    rows          [][]string
}

var _ ListPrinter = &listPrinter{}

func (lp *listPrinter) AddColumns(columns ...string) {
    lp.columns = append(lp.columns, columns...)
}

func (lp *listPrinter) AddField(s string, opts ...FieldOption) {
    if lp.currentColumn < 0 || len(lp.currentFields) == 0 && lp.currentColumn == 0 {
        lp.currentFields = []string{}
    }
    // ensure order aligns with lp.columns length
    lp.currentFields = append(lp.currentFields, s)
    lp.currentColumn++
}

func (lp *listPrinter) AddTimeField(now, t time.Time, c func(string) string) {
    tf := text.FuzzyAgo(now, t)
    lp.AddField(tf)
}

func (lp *listPrinter) EndRow() {
    if len(lp.currentFields) > 0 {
        lp.rows = append(lp.rows, lp.currentFields)
    }
    lp.currentFields = nil
    lp.currentColumn = 0
}

func (lp *listPrinter) Render() error {
    for ri, row := range lp.rows {
        for ci, val := range row {
            var key string
            if ci < len(lp.columns) {
                key = lp.columns[ci]
            } else {
                key = fmt.Sprintf("col%d", ci)
            }
            _, err := fmt.Fprintf(lp.out, "%s: %s\n", key, val)
            if err != nil {
                return err
            }
        }
        if ri < len(lp.rows)-1 {
            _, err := fmt.Fprint(lp.out, "\n")
            if err != nil {
                return err
            }
        }
    }
    return nil
}

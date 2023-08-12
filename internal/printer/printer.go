package printer

import (
	"fmt"
	"time"
)

type UnsupportedPrinterError struct {
	ptype string
}

func (e *UnsupportedPrinterError) Error() string {
	return fmt.Sprintf("unsupported printer type %s", e.ptype)
}

func NewUnsupportedPrinterError(ptype string) error {
	return &UnsupportedPrinterError{
		ptype: ptype,
	}
}

type Printer interface {
	AddColumns(columns ...string)
	AddField(string, ...FieldOption)
	AddTimeField(now, t time.Time, c func(string) string)
	EndRow()
	Render() error
}

type FieldOption func(*tableField)

// WithTruncate overrides the truncation function for the field. The function should transform a string
// argument into a string that fits within the given display width. The default behavior is to truncate the
// value by adding "..." in the end. Pass nil to disable truncation for this value.
func WithTruncate(fn func(int, string) string) FieldOption {
	return func(f *tableField) {
		f.truncateFunc = fn
	}
}

// WithColor sets the color function for the field. The function should transform a string value by wrapping
// it in ANSI escape codes. The color function will not be used if the table was initialized in non-terminal mode.
func WithColor(fn func(string) string) FieldOption {
	return func(f *tableField) {
		f.colorFunc = fn
	}
}

// Package text is a set of utility functions for text processing and outputting to the terminal.
package text

import (
	"fmt"
	"math"
	"regexp"
	"slices"
	"strings"
	"time"
	"unicode"

	"github.com/charmbracelet/lipgloss"
	"github.com/muesli/reflow/truncate"
	"golang.org/x/text/runes"
	"golang.org/x/text/transform"
	"golang.org/x/text/unicode/norm"
)

const (
	ellipsis            = "..."
	minWidthForEllipsis = len(ellipsis) + 2
)

var indentRE = regexp.MustCompile(`(?m)^`)

// Indent returns a copy of the string s with indent prefixed to it, will apply indent
// to each line of the string.
func Indent(s, indent string) string {
	if len(strings.TrimSpace(s)) == 0 {
		return s
	}
	return indentRE.ReplaceAllLiteralString(s, indent)
}

// DisplayWidth calculates what the rendered width of string s will be.
func DisplayWidth(s string) int {
	return lipgloss.Width(s)
}

// Truncate returns a copy of the string s that has been shortened to fit the maximum display width.
func Truncate(maxWidth int, s string) string {
	w := DisplayWidth(s)
	if w <= maxWidth {
		return s
	}
	tail := ""
	if maxWidth >= minWidthForEllipsis {
		tail = ellipsis
	}
	r := truncate.StringWithTail(s, uint(maxWidth), tail) //nolint:gosec
	if DisplayWidth(r) < maxWidth {
		r += " "
	}
	return r
}

// PadRight returns a copy of the string s that has been padded on the right with whitespace to fit
// the maximum display width.
func PadRight(maxWidth int, s string) string {
	if padWidth := maxWidth - DisplayWidth(s); padWidth > 0 {
		s += strings.Repeat(" ", padWidth)
	}
	return s
}

// Pluralize returns a concatenated string with num and the plural form of thing if necessary.
func Pluralize(num int, thing string) string {
	if num == 1 {
		return fmt.Sprintf("%d %s", num, thing)
	}
	return fmt.Sprintf("%d %ss", num, thing)
}

func fmtDuration(amount int, unit string) string {
	return fmt.Sprintf("about %s ago", Pluralize(amount, unit))
}

// RelativeTimeAgo returns a human readable string of the time duration between a and b that is estimated
// to the nearest unit of time.
func RelativeTimeAgo(a, b time.Time) string {
	ago := a.Sub(b)

	if ago < time.Minute {
		return "less than a minute ago"
	}
	if ago < time.Hour {
		return fmtDuration(int(ago.Minutes()), "minute")
	}
	if ago < 24*time.Hour {
		return fmtDuration(int(ago.Hours()), "hour")
	}
	if ago < 30*24*time.Hour {
		return fmtDuration(int(ago.Hours())/24, "day")
	}
	if ago < 365*24*time.Hour {
		return fmtDuration(int(ago.Hours())/24/30, "month")
	}

	return fmtDuration(int(ago.Hours()/24/365), "year")
}

// RemoveDiacritics returns the input value without "diacritics", or accent marks.
func RemoveDiacritics(value string) string {
	// Mn = "Mark, nonspacing" unicode character category
	removeMnTransfomer := runes.Remove(runes.In(unicode.Mn))

	// 1. Decompose the text into characters and diacritical marks
	// 2. Remove the diacriticals marks
	// 3. Recompose the text
	t := transform.Chain(norm.NFD, removeMnTransfomer, norm.NFC)
	normalized, _, err := transform.String(t, value)
	if err != nil {
		return value
	}
	return normalized
}

func FuzzyAgo(a, b time.Time) string {
	return RelativeTimeAgo(a, b)
}

// FormatSliceBuilder provides a fluent API to format string slices.
type FormatSliceBuilder struct {
	items   []string
	def     []string
	lineLen int
	indent  int
	prepend string
	append  string
	sort    bool
}

// NewSliceFormatter constructs a builder for the provided items.
func NewSliceFormatter(items []string) *FormatSliceBuilder {
	return &FormatSliceBuilder{
		items:   items,
		def:     nil,
		lineLen: math.MaxInt,
		indent:  0,
		prepend: "",
		append:  "",
		sort:    false,
	}
}

// WithDefault sets the default slice to use when items is nil or empty.
func (b *FormatSliceBuilder) WithDefault(d []string) *FormatSliceBuilder {
	b.def = d
	return b
}

// WithLineLength sets the maximum line length. Non-positive values disable wrapping.
func (b *FormatSliceBuilder) WithLineLength(n int) *FormatSliceBuilder {
	if n <= 0 {
		b.lineLen = math.MaxInt
	} else {
		b.lineLen = n
	}
	return b
}

// WithIndent sets the number of spaces to indent the first column.
func (b *FormatSliceBuilder) WithIndent(n int) *FormatSliceBuilder {
	if n < 0 {
		n = 0
	}
	b.indent = n
	return b
}

// WithPrepend sets a string to place before each element.
func (b *FormatSliceBuilder) WithPrepend(s string) *FormatSliceBuilder {
	b.prepend = s
	return b
}

// WithAppend sets a string to place after each element.
func (b *FormatSliceBuilder) WithAppend(s string) *FormatSliceBuilder {
	b.append = s
	return b
}

// WithSort enables or disables sorting of the elements.
func (b *FormatSliceBuilder) WithSort(yes bool) *FormatSliceBuilder {
	b.sort = yes
	return b
}

// String formats the slice according to the builder configuration.
func (b *FormatSliceBuilder) String() string {
	values := b.items
	if len(values) == 0 && b.def != nil {
		values = b.def
	}

	sortedValues := values
	if b.sort {
		sortedValues = slices.Clone(values)
		slices.Sort(sortedValues)
	}

	pre := strings.Repeat(" ", b.indent) //nolint:gosec
	if len(sortedValues) == 0 {
		return pre
	} else if len(sortedValues) == 1 {
		return pre + sortedValues[0]
	}

	builder := strings.Builder{}
	currentLineLength := 0
	sep := ","
	ws := " "

	for i := 0; i < len(sortedValues); i++ {
		v := b.prepend + sortedValues[i] + b.append
		isLast := i == -1+len(sortedValues)

		if currentLineLength == 0 {
			builder.WriteString(pre)
			builder.WriteString(v)
			currentLineLength += len(v)
			if !isLast {
				builder.WriteString(sep)
				currentLineLength += len(sep)
			}
		} else {
			if !isLast && currentLineLength+len(ws)+len(v)+len(sep) > int(b.lineLen) || //nolint:gosec
				isLast && currentLineLength+len(ws)+len(v) > int(b.lineLen) { //nolint:gosec
				currentLineLength = 0
				builder.WriteString("\n")
				i--
				continue
			}

			builder.WriteString(ws)
			builder.WriteString(v)
			currentLineLength += len(ws) + len(v)
			if !isLast {
				builder.WriteString(sep)
				currentLineLength += len(sep)
			}
		}
	}
	return builder.String()
}

// Package jsonpretty implements a terminal pretty-printer for JSON.
package jsonpretty

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"
)

const (
	colorDelim  = "\x1b[1;38m" // bright white
	colorKey    = "\x1b[1;34m" // bright blue
	colorNull   = "\x1b[36m"   // cyan
	colorString = "\x1b[32m"   // green
	colorBool   = "\x1b[33m"   // yellow
	colorReset  = "\x1b[m"
)

// Format reads JSON from r and writes a prettified version of it to w.
func Format(w io.Writer, r io.Reader, indent string, colorize bool) error {
	dec := json.NewDecoder(r)
	dec.UseNumber()

	c := func(ansi string) string {
		if !colorize {
			return ""
		}
		return ansi
	}

	var idx int
	var stack []json.Delim

	for {
		t, err := dec.Token()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return fmt.Errorf("failed to read token: %w", err)
		}

		switch tt := t.(type) {
		case json.Delim:
			switch tt {
			case '{', '[':
				stack = append(stack, tt)
				idx = 0
				if _, err := fmt.Fprint(w, c(colorDelim), tt, c(colorReset)); err != nil {
					return fmt.Errorf("%w", err)
				}
				if dec.More() {
					if _, err := fmt.Fprint(w, "\n", strings.Repeat(indent, len(stack))); err != nil {
						return fmt.Errorf("%w", err)
					}
				}
				continue
			case '}', ']':
				stack = stack[:len(stack)-1]
				idx = 0
				if _, err := fmt.Fprint(w, c(colorDelim), tt, c(colorReset)); err != nil {
					return fmt.Errorf("%w", err)
				}
			}
		default:
			b, err := marshalJSON(tt)
			if err != nil {
				return fmt.Errorf("failed to marshal JSON: %w", err)
			}

			isKey := len(stack) > 0 && stack[len(stack)-1] == '{' && idx%2 == 0
			idx++

			var color string
			if isKey {
				color = colorKey
			} else if tt == nil {
				color = colorNull
			} else {
				switch t.(type) {
				case string:
					color = colorString
				case bool:
					color = colorBool
				}
			}

			if color != "" {
				if _, err := fmt.Fprint(w, c(color)); err != nil {
					return fmt.Errorf("%w", err)
				}
			}
			if _, err := w.Write(b); err != nil {
				return fmt.Errorf("%w", err)
			}
			if color != "" {
				if _, err := fmt.Fprint(w, c(colorReset)); err != nil {
					return fmt.Errorf("%w", err)
				}
			}

			if isKey {
				if _, err := fmt.Fprint(w, c(colorDelim), ":", c(colorReset), " "); err != nil {
					return fmt.Errorf("%w", err)
				}
				continue
			}
		}

		if dec.More() {
			if _, err := fmt.Fprint(w, c(colorDelim), ",", c(colorReset), "\n", strings.Repeat(indent, len(stack))); err != nil {
				return fmt.Errorf("%w", err)
			}
		} else if len(stack) > 0 {
			if _, err := fmt.Fprint(w, "\n", strings.Repeat(indent, len(stack)-1)); err != nil {
				return fmt.Errorf("%w", err)
			}
		} else {
			if _, err := fmt.Fprint(w, "\n"); err != nil {
				return fmt.Errorf("%w", err)
			}
		}
	}

	return nil
}

// marshalJSON works like json.Marshal, but with HTML-escaping disabled.
func marshalJSON(v any) ([]byte, error) {
	buf := bytes.Buffer{}
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false)
	if err := enc.Encode(v); err != nil {
		return nil, fmt.Errorf("%w", err)
	}
	bb := buf.Bytes()
	// omit trailing newline added by json.Encoder
	if len(bb) > 0 && bb[len(bb)-1] == '\n' {
		return bb[:len(bb)-1], nil
	}
	return bb, nil
}

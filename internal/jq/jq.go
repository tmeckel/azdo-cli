// Package jq facilitates processing of JSON strings using jq expressions.
package jq

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"os"
	"reflect"
	"strconv"
	"strings"

	"github.com/itchyny/gojq"
	"github.com/tmeckel/azdo-cli/internal/jsonpretty"
)

// Evaluate a jq expression against an input and write it to an output.
// Any top-level scalar values produced by the jq expression are written out
// directly, as raw values and not as JSON scalars, similar to how jq --raw
// works.
func Evaluate(input io.Reader, output io.Writer, expr string) error {
	return EvaluateFormatted(input, output, expr, "", false)
}

func EvaluateData(expr string, data any) (any, error) {
	if expr != "" {

		code, err := CompileExpression(expr)
		if err != nil {
			return nil, err
		}
		iter := code.Run(data)

		var jqData any
		isArray := reflect.TypeOf(data).Kind() == reflect.Slice
		if isArray {
			jqData = []any{}
		}
		items := 0
		for {
			v, ok := iter.Next()
			if !ok {
				break
			}
			if err, isErr := v.(error); isErr {
				var e *gojq.HaltError
				if errors.As(err, &e) && e.Value() == nil {
					break
				}
				return nil, err
			}
			items++
			if isArray {
				jqData = append(jqData.([]any), v)
			} else {
				switch items {
				case 1:
					jqData = v
				case 2:
					jqData = []any{jqData, v}
				default:
					jqData = append(jqData.([]any), v)
				}
			}
		}
		data = jqData
	}
	return data, nil
}

func CompileExpression(expr string) (*gojq.Code, error) {
	query, err := gojq.Parse(expr)
	if err != nil {
		var e *gojq.ParseError
		if errors.As(err, &e) {
			str, line, column := getLineColumn(expr, e.Offset-len(e.Token))
			return nil, fmt.Errorf(
				"failed to parse jq expression (line %d, column %d)\n    %s\n    %*c  %w",
				line, column, str, column, '^', err,
			)
		}
		return nil, fmt.Errorf("failed to parse jq expression %q: %w", expr, err)
	}

	code, err := gojq.Compile(
		query,
		gojq.WithEnvironLoader(os.Environ))
	if err != nil {
		return nil, fmt.Errorf("failed to compile jq expression: %w", err)
	}
	return code, nil
}

// Evaluate a jq expression against an input and write it to an output,
// optionally with indentation and colorization.  Any top-level scalar values
// produced by the jq expression are written out directly, as raw values and not
// as JSON scalars, similar to how jq --raw works.
func EvaluateFormatted(input io.Reader, output io.Writer, expr string, indent string, colorize bool) error {
	code, err := CompileExpression(expr)
	if err != nil {
		return err
	}
	jsonData, err := io.ReadAll(input)
	if err != nil {
		return fmt.Errorf("failed to read from input stream: %w", err)
	}

	var responseData any
	err = json.Unmarshal(jsonData, &responseData)
	if err != nil {
		return fmt.Errorf("failed to unmarshal JSON data: %w", err)
	}

	enc := prettyEncoder{
		w:        output,
		indent:   indent,
		colorize: colorize,
	}

	iter := code.Run(responseData)
	for {
		v, ok := iter.Next()
		if !ok {
			break
		}
		if err, isErr := v.(error); isErr {
			var e *gojq.HaltError
			if errors.As(err, &e) && e.Value() == nil {
				break
			}
			return err
		}
		if text, e := jsonScalarToString(v); e == nil {
			_, err := fmt.Fprintln(output, text)
			if err != nil {
				return fmt.Errorf("failed to format text: %w", err)
			}
		} else {
			if err = enc.Encode(v); err != nil {
				return fmt.Errorf("failed to encode value %+v: %w", v, err)
			}
		}
	}

	return nil
}

func jsonScalarToString(input any) (string, error) {
	switch tt := input.(type) {
	case string:
		return tt, nil
	case float64:
		if math.Trunc(tt) == tt {
			return strconv.FormatFloat(tt, 'f', 0, 64), nil
		} else {
			return strconv.FormatFloat(tt, 'f', 2, 64), nil
		}
	case nil:
		return "", nil
	case bool:
		return fmt.Sprintf("%v", tt), nil
	default:
		return "", fmt.Errorf("cannot convert type to string: %v", tt)
	}
}

type prettyEncoder struct {
	w        io.Writer
	indent   string
	colorize bool
}

func (p prettyEncoder) Encode(v any) error {
	var b []byte
	var err error
	if p.indent == "" {
		b, err = json.Marshal(v)
	} else {
		b, err = json.MarshalIndent(v, "", p.indent)
	}
	if err != nil {
		return fmt.Errorf("failed to marshal value %+v: %w", v, err)
	}
	if !p.colorize {
		if _, err := p.w.Write(b); err != nil {
			return fmt.Errorf("failed to write data to output stream: %w", err)
		}
		if _, err := p.w.Write([]byte{'\n'}); err != nil {
			return fmt.Errorf("failed to write newline to output stream: %w", err)
		}
		return nil
	}
	err = jsonpretty.Format(p.w, bytes.NewReader(b), p.indent, true)
	if err != nil {
		return fmt.Errorf("failed to format JSON: %w", err)
	}
	return nil
}

func getLineColumn(expr string, offset int) (string, int, int) {
	for line := 1; ; line++ {
		index := strings.Index(expr, "\n")
		if index < 0 {
			return expr, line, offset + 1
		}
		if index >= offset {
			return expr[:index], line, offset + 1
		}
		expr = expr[index+1:]
		offset -= index + 1
	}
}

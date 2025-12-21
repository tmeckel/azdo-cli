package shared

import (
	"encoding/binary"
	"fmt"
	"strings"
	u "unicode"
	"unicode/utf8"

	"golang.org/x/text/encoding"
	"golang.org/x/text/encoding/unicode"
)

var EncodingAliases = map[string]string{
	"utf-8":    "utf-8",
	"utf8":     "utf-8",
	"ascii":    "ascii",
	"utf-16be": "utf-16be",
	"utf16be":  "utf-16be",
	"utf-16le": "utf-16le",
	"utf16le":  "utf-16le",
}

func NormalizeEncoding(value string) (string, error) {
	trimmed := strings.ToLower(strings.TrimSpace(value))
	if trimmed == "" {
		return "utf-8", nil
	}
	if normalized, ok := EncodingAliases[trimmed]; ok {
		return normalized, nil
	}
	return "", fmt.Errorf("unsupported encoding %q; supported values: utf-8, ascii, utf-16be, utf-16le", value)
}

func DescribeInput(path string) string {
	if strings.TrimSpace(path) == "-" {
		return "stdin"
	}
	return path
}

func DecodeContent(raw []byte, encodingName string) ([]byte, error) {
	var dec *encoding.Decoder
	switch encodingName {
	case "utf-8":
		if !utf8.Valid(raw) {
			return nil, fmt.Errorf("input is not valid UTF-8")
		}
		return raw, nil
	case "ascii":
		for i, b := range raw {
			if b > u.MaxASCII {
				return nil, fmt.Errorf("input contains non-ASCII byte at offset %d", i)
			}
		}
		return raw, nil
	case "utf-16le":
		if err := validateUTF16(raw, binary.LittleEndian); err != nil {
			return nil, err
		}
		dec = unicode.UTF16(unicode.LittleEndian, unicode.UseBOM).NewDecoder()
	case "utf-16be":
		if err := validateUTF16(raw, binary.BigEndian); err != nil {
			return nil, err
		}
		dec = unicode.UTF16(unicode.BigEndian, unicode.UseBOM).NewDecoder()
	default:
		return nil, fmt.Errorf("unsupported encoding %q", encodingName)
	}
	raw, err := dec.Bytes(raw)
	if err != nil {
		return nil, fmt.Errorf("failed to decode input: %w", err)
	}
	return raw, nil
}

func validateUTF16(raw []byte, order binary.ByteOrder) error {
	if len(raw)%2 != 0 {
		return fmt.Errorf("failed to decode input: invalid UTF-16 sequence length")
	}
	if len(raw) == 0 {
		return nil
	}
	units := make([]uint16, len(raw)/2)
	for i := range units {
		units[i] = order.Uint16(raw[2*i:])
	}
	start := 0
	if units[0] == 0xFEFF {
		start = 1
	}
	for i := start; i < len(units); {
		val := units[i]
		switch {
		case val >= 0xD800 && val <= 0xDBFF:
			if i+1 >= len(units) {
				return fmt.Errorf("failed to decode input: invalid UTF-16 surrogate pair")
			}
			next := units[i+1]
			if next < 0xDC00 || next > 0xDFFF {
				return fmt.Errorf("failed to decode input: invalid UTF-16 surrogate pair")
			}
			i += 2
		case val >= 0xDC00 && val <= 0xDFFF:
			return fmt.Errorf("failed to decode input: invalid UTF-16 surrogate pair")
		default:
			i++
		}
	}
	return nil
}

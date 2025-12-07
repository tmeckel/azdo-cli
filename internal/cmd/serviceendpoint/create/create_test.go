package create

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestDecodeContent(t *testing.T) {
	t.Run("UTF-8 encoding", func(t *testing.T) {
		t.Parallel()
		testCases := []struct {
			name     string
			input    []byte
			expected string
		}{
			{
				name:     "Valid UTF-8 with Unicode",
				input:    []byte{0x48, 0x65, 0x6C, 0x6C, 0x6F, 0x2C, 0x20, 0xE4, 0xB8, 0x96, 0xE7, 0x95, 0x8C, 0x21}, // "Hello, 世界!"
				expected: "Hello, 世界!",
			},
			{
				name:     "Empty input",
				input:    []byte(""),
				expected: "",
			},
			{
				name:     "ASCII only",
				input:    []byte("Hello, World!"),
				expected: "Hello, World!",
			},
		}

		for _, tc := range testCases {
			tc := tc
			t.Run(tc.name, func(t *testing.T) {
				t.Parallel()
				decoded, err := decodeContent(tc.input, "utf-8")
				require.NoError(t, err)
				require.Equal(t, tc.expected, string(decoded))
			})
		}
	})

	t.Run("ASCII encoding", func(t *testing.T) {
		t.Parallel()
		testCases := []struct {
			name     string
			input    []byte
			expected string
			errMsg   string
		}{
			{
				name:     "Valid ASCII",
				input:    []byte("Hello, World!"),
				expected: "Hello, World!",
			},
			{
				name:     "Empty input",
				input:    []byte(""),
				expected: "",
			},
			{
				name:   "Non-ASCII byte",
				input:  []byte{0x48, 0x65, 0x6C, 0x6C, 0x6F, 0x2C, 0x20, 0xE4, 0xB8, 0x96}, // "Hello, 世"
				errMsg: "input contains non-ASCII byte at offset 7",
			},
		}

		for _, tc := range testCases {
			tc := tc
			t.Run(tc.name, func(t *testing.T) {
				t.Parallel()
				decoded, err := decodeContent(tc.input, "ascii")
				if tc.errMsg != "" {
					require.ErrorContains(t, err, tc.errMsg)
					return
				}
				require.NoError(t, err)
				require.Equal(t, tc.expected, string(decoded))
			})
		}
	})

	t.Run("UTF-16LE encoding", func(t *testing.T) {
		t.Parallel()
		testCases := []struct {
			name     string
			input    []byte
			expected string
			errMsg   string
		}{
			{
				name:     "Valid UTF-16LE with BOM",
				input:    []byte{0xFF, 0xFE, 'H', 0x00, 'e', 0x00, 'l', 0x00, 'l', 0x00, 'o', 0x00, ',', 0x00, ' ', 0x00, 0x16, 0x4E, 0x4C, 0x75, '!', 0x00},
				expected: "Hello, 世界!",
			},
			{
				name:     "Valid UTF-16LE without BOM",
				input:    []byte{'H', 0x00, 'e', 0x00, 'l', 0x00, 'l', 0x00, 'o', 0x00},
				expected: "Hello",
			},
			{
				name:     "Empty input",
				input:    []byte(""),
				expected: "",
			},
			{
				name:   "Invalid UTF-16LE",
				input:  []byte{0xFF, 0xFE, 0x00, 0xDC}, // lone low surrogate
				errMsg: "failed to decode input",
			},
			{
				name:   "Odd length UTF-16LE",
				input:  []byte{0xFF, 0xFE, 'H'},
				errMsg: "failed to decode input",
			},
			{
				name:   "High surrogate without pair",
				input:  []byte{0xFF, 0xFE, 0x00, 0xD8, '!', 0x00},
				errMsg: "failed to decode input",
			},
		}

		for _, tc := range testCases {
			tc := tc
			t.Run(tc.name, func(t *testing.T) {
				t.Parallel()
				decoded, err := decodeContent(tc.input, "utf-16le")
				if tc.errMsg != "" {
					require.ErrorContains(t, err, tc.errMsg)
					return
				}
				require.NoError(t, err)
				require.Equal(t, tc.expected, string(decoded))
			})
		}
	})

	t.Run("UTF-16BE encoding", func(t *testing.T) {
		t.Parallel()
		testCases := []struct {
			name     string
			input    []byte
			expected string
			errMsg   string
		}{
			{
				name:     "Valid UTF-16BE with BOM",
				input:    []byte{0xFE, 0xFF, 0x00, 'H', 0x00, 'e', 0x00, 'l', 0x00, 'l', 0x00, 'o', 0x00, ',', 0x00, ' ', 0x4E, 0x16, 0x75, 0x4C, 0x00, '!'},
				expected: "Hello, 世界!",
			},
			{
				name:     "Valid UTF-16BE without BOM",
				input:    []byte{0x00, 'H', 0x00, 'e', 0x00, 'l', 0x00, 'l', 0x00, 'o'},
				expected: "Hello",
			},
			{
				name:     "Empty input",
				input:    []byte(""),
				expected: "",
			},
			{
				name:   "Invalid UTF-16BE",
				input:  []byte{0xFE, 0xFF, 0xDC, 0x00}, // lone low surrogate
				errMsg: "failed to decode input",
			},
			{
				name:   "Odd length UTF-16BE",
				input:  []byte{0xFE, 0xFF, 0x00},
				errMsg: "failed to decode input",
			},
			{
				name:   "High surrogate without pair",
				input:  []byte{0xFE, 0xFF, 0xD8, 0x00, 0x00, '!'},
				errMsg: "failed to decode input",
			},
		}

		for _, tc := range testCases {
			tc := tc
			t.Run(tc.name, func(t *testing.T) {
				t.Parallel()
				decoded, err := decodeContent(tc.input, "utf-16be")
				if tc.errMsg != "" {
					require.ErrorContains(t, err, tc.errMsg)
					return
				}
				require.NoError(t, err)
				require.Equal(t, tc.expected, string(decoded))
			})
		}
	})

	t.Run("Invalid encoding", func(t *testing.T) {
		t.Parallel()
		_, err := decodeContent([]byte("test"), "invalid-encoding")
		require.ErrorContains(t, err, "unsupported encoding \"invalid-encoding\"")
	})

	t.Run("Invalid UTF-8", func(t *testing.T) {
		t.Parallel()
		invalidUTF8 := []byte{0xff, 0xfe, 0xfd} // Invalid UTF-8 sequence
		_, err := decodeContent(invalidUTF8, "utf-8")
		require.ErrorContains(t, err, "input is not valid UTF-8")
	})
}

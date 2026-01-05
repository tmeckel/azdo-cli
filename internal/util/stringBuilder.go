package util

import (
	"encoding/hex"
	"errors"
	"fmt"
	"math/rand"
	"strings"
	"time"
)

const (
	upperLetters = "ABCDEFGHIJKLMNOPQRSTUVWXYZ"
	lowerLetters = "abcdefghijklmnopqrstuvwxyz"
	numbers      = "0123456789"
	specialChars = "!@#$%^&*()_+-=[]{}|;':\",./<>?"
)

// StringBuilder is a builder for configuring and generating custom random strings.
// By default, no character sets are included. You must add at least one using With* methods.
// Mutations like AsHexString or AsBinaryString can be applied to transform the output after generation.
type StringBuilder struct {
	includeUpper   bool
	includeLower   bool
	includeNumbers bool
	includeSpecial bool
	mutation       string // "" (none), "hex", or "binary"
	rand           *rand.Rand
}

// NewStringBuilder creates a new StringBuilder with no default character sets.
func NewStringBuilder() *StringBuilder {
	s := rand.NewSource(time.Now().UnixNano())
	return &StringBuilder{
		rand: rand.New(s),
	}
}

// WithUpperLetters adds uppercase letters (A-Z) to the character set.
func (sb *StringBuilder) WithUpperLetters() *StringBuilder {
	sb.includeUpper = true
	return sb
}

// WithLowerLetters adds lowercase letters (a-z) to the character set.
func (sb *StringBuilder) WithLowerLetters() *StringBuilder {
	sb.includeLower = true
	return sb
}

// WithNumbers adds digits (0-9) to the character set.
func (sb *StringBuilder) WithNumbers() *StringBuilder {
	sb.includeNumbers = true
	return sb
}

// WithSpecialCharacters adds special characters (!@#$%^&*()_+-=[]{}|;':",./<>?) to the character set.
func (sb *StringBuilder) WithSpecialCharacters() *StringBuilder {
	sb.includeSpecial = true
	return sb
}

// AsHexString sets the mutation to convert the generated string to its hexadecimal representation.
// This transforms each byte of the string into two hex characters, doubling the length.
func (sb *StringBuilder) AsHexString() *StringBuilder {
	sb.mutation = "hex"
	return sb
}

// AsBinaryString sets the mutation to convert the generated string to its binary representation.
// This transforms each byte into an 8-bit binary string, multiplying the length by 8.
func (sb *StringBuilder) AsBinaryString() *StringBuilder {
	sb.mutation = "binary"
	return sb
}

// Generate creates a random string of the specified length using the configured character set.
// If no character sets are selected, it returns an error.
// After generation, any set mutation (hex or binary) is applied to the output.
// Length specifies the pre-mutation length; the final length will differ if a mutation is applied.
// Returns an empty string and error if length <= 0 or no sets selected.
func (sb *StringBuilder) Generate(length int) (string, error) {
	if length <= 0 {
		return "", errors.New("length must be positive")
	}

	var charset string
	if sb.includeUpper {
		charset += upperLetters
	}
	if sb.includeLower {
		charset += lowerLetters
	}
	if sb.includeNumbers {
		charset += numbers
	}
	if sb.includeSpecial {
		charset += specialChars
	}

	if charset == "" {
		return "", errors.New("no character sets selected")
	}

	b := make([]byte, length)
	for i := range b {
		b[i] = charset[sb.rand.Int31n(int32(len(charset)))]
	}
	s := string(b)

	switch sb.mutation {
	case "hex":
		return hex.EncodeToString([]byte(s)), nil
	case "binary":
		var strBuilder strings.Builder
		for _, by := range []byte(s) {
			strBuilder.WriteString(fmt.Sprintf("%08b", by))
		}
		return strBuilder.String(), nil
	default:
		return s, nil
	}
}

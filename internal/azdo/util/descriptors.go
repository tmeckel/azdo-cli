package util

import (
	"regexp"
	"strings"
)

var (
	descriptorPattern = regexp.MustCompile(`^[a-zA-Z]+\.[a-zA-Z0-9-_]+$`)
	sidPattern        = regexp.MustCompile(`(?i)s-\d+-\d+(-\d+)+$`)
)

// A descriptor is a string containing two elements, which are separated by a period '.'
//
// <SubjectType> '.' <Identifier>
//
// The identifier is a base64 string with no padding:
// '=' removed
// '+' replaced by '-'
// '/' replaced by '_'
func IsDescriptor(value string) bool {
	value = strings.TrimSpace(value)
	if value == "" {
		return false
	}
	return descriptorPattern.MatchString(value)
}

func IsSecurityIdentifier(value string) bool {
	value = strings.TrimSpace(value)
	if value == "" {
		return false
	}
	return sidPattern.MatchString(value)
}

package util

import (
	"regexp"
	"strings"
)

const identitySIDPrefix = "Microsoft.TeamFoundation.Identity;"

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

// IsIdentitySID reports whether value is an Azure DevOps identity SID,
// either as a bare SID (s-1-5-...) or with the Microsoft.TeamFoundation.Identity; prefix.
func IsIdentitySID(value string) bool {
	value = strings.TrimSpace(value)
	if value == "" {
		return false
	}
	if IsSecurityIdentifier(value) {
		return true
	}
	return sidPattern.MatchString(strings.TrimPrefix(value, identitySIDPrefix))
}

// NormalizeIdentitySID returns the identity SID prefixed with the Microsoft.TeamFoundation.Identity;
// scope. The value is left unchanged if it already has the prefix or is empty.
func NormalizeIdentitySID(value string) string {
	value = strings.TrimSpace(value)
	if value == "" || strings.Contains(value, ";") {
		return value
	}
	return identitySIDPrefix + value
}

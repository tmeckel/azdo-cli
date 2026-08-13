package shared

import "fmt"

// FieldString returns the string representation of a work item field value.
// Missing or nil values render as the empty string.
func FieldString(fields map[string]any, key string) string {
	if fields == nil {
		return ""
	}
	v, ok := fields[key]
	if !ok || v == nil {
		return ""
	}
	switch t := v.(type) {
	case string:
		return t
	default:
		return fmt.Sprint(v)
	}
}

// FieldIdentityDisplay returns the display name of an identity field value.
// Plain strings are returned unchanged; identity maps prefer displayName over
// uniqueName.
func FieldIdentityDisplay(fields map[string]any, key string) string {
	if fields == nil {
		return ""
	}
	v, ok := fields[key]
	if !ok || v == nil {
		return ""
	}
	switch t := v.(type) {
	case string:
		return t
	case map[string]any:
		if displayName, ok := t["displayName"].(string); ok {
			return displayName
		}
		if uniqueName, ok := t["uniqueName"].(string); ok {
			return uniqueName
		}
		return fmt.Sprint(v)
	default:
		return fmt.Sprint(v)
	}
}

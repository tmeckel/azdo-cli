package shared

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/microsoft/azure-devops-go-api/azuredevops/v7/security"
	"github.com/tmeckel/azdo-cli/internal/types"
)

// ParsePermissionBits converts user-supplied textual or numeric permission identifiers into a bitmask.
// It accepts comma-separated values via Cobra's string slice parsing and validates tokens against the
// namespace action definitions when available.
func ParsePermissionBits(actions []security.ActionDefinition, parts []string) (int, error) {
	var val int

	// Build a map for quick name->bit lookup (case-insensitive) and track all allowed bits.
	nameMap := make(map[string]int)
	var allowedMask int
	for _, a := range actions {
		bit := types.GetValue(a.Bit, 0)
		if bit == 0 {
			continue
		}

		allowedMask |= bit

		if n := strings.TrimSpace(types.GetValue(a.Name, "")); n != "" {
			nameMap[strings.ToLower(n)] = bit
		}
		if dn := strings.TrimSpace(types.GetValue(a.DisplayName, "")); dn != "" {
			nameMap[strings.ToLower(dn)] = bit
		}
		nameMap[strings.ToLower(fmt.Sprintf("bit %d", bit))] = bit
	}

	checkAllowed := func(bitVal int) error {
		if bitVal == 0 {
			return fmt.Errorf("permission bit value cannot be zero")
		}
		if allowedMask != 0 && bitVal&^allowedMask != 0 {
			return fmt.Errorf("permission bit value %d is not defined for this namespace", bitVal)
		}
		return nil
	}

	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p == "" {
			continue
		}

		// numeric hex: 0x prefix
		if strings.HasPrefix(p, "0x") || strings.HasPrefix(p, "0X") {
			v, err := strconv.ParseInt(p[2:], 16, 32)
			if err != nil {
				return 0, fmt.Errorf("invalid bit value %q: %w", p, err)
			}
			candidate := int(v)
			if err := checkAllowed(candidate); err != nil {
				return 0, err
			}
			val |= candidate
			continue
		}

		// numeric decimal
		if d, err := strconv.ParseInt(p, 10, 32); err == nil {
			candidate := int(d)
			if err := checkAllowed(candidate); err != nil {
				return 0, err
			}
			val |= candidate
			continue
		}

		// textual name match (case-insensitive)
		l := strings.ToLower(p)
		if bit, ok := nameMap[l]; ok {
			val |= bit
			continue
		}

		return 0, fmt.Errorf("unrecognized permission token %q", p)
	}

	return val, nil
}

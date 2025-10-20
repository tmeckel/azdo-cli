package shared

import (
	"fmt"
	"sort"
	"strings"

	"github.com/microsoft/azure-devops-go-api/azuredevops/v7/security"
	"github.com/tmeckel/azdo-cli/internal/types"
)

// DescribeBitmask converts a pointer to a permission bitmask into a human-readable description.
// The description prefers the namespace action Name values and falls back to display name or the
// raw hexadecimal representation when no metadata exists.
func DescribeBitmask(actions []security.ActionDefinition, mask *int) *string {
	if mask == nil {
		return nil
	}
	return types.ToPtr(DescribeBitmaskValue(actions, *mask))
}

// DescribeBitmaskValue converts an integer bitmask into its human-readable representation.
// When the mask is zero, the string "None" is returned.
func DescribeBitmaskValue(actions []security.ActionDefinition, mask int) string {
	if mask == 0 {
		return "None"
	}

	if len(actions) == 0 {
		return fmt.Sprintf("0x%X", mask)
	}

	var matched int
	names := make([]string, 0)
	for _, action := range actions {
		actionBit := types.GetValue(action.Bit, 0)
		if actionBit == 0 || mask&actionBit != actionBit {
			continue
		}
		matched |= actionBit
		name := strings.TrimSpace(types.GetValue(action.Name, ""))
		if name == "" {
			name = strings.TrimSpace(types.GetValue(action.DisplayName, fmt.Sprintf("Bit %d", actionBit)))
		}
		if name == "" {
			name = fmt.Sprintf("Bit %d", actionBit)
		}
		names = append(names, name)
	}

	if matched != mask {
		unknown := mask &^ matched
		if unknown != 0 {
			names = append(names, fmt.Sprintf("Unknown (0x%X)", unknown))
		}
	}

	sort.Strings(names)
	if len(names) == 0 {
		return fmt.Sprintf("0x%X", mask)
	}
	return strings.Join(names, ", ")
}

// ExtractNamespaceActions returns a defensive copy of the action definitions for the provided namespace result.
func ExtractNamespaceActions(response *[]security.SecurityNamespaceDescription) []security.ActionDefinition {
	if response == nil || len(*response) == 0 {
		return nil
	}
	ns := (*response)[0]
	if ns.Actions == nil || len(*ns.Actions) == 0 {
		return nil
	}
	return append([]security.ActionDefinition(nil), (*ns.Actions)...)
}

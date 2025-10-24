package shared

import (
	"fmt"

	"github.com/google/uuid"
	"github.com/microsoft/azure-devops-go-api/azuredevops/v7/security"
	"github.com/tmeckel/azdo-cli/internal/types"
)

// NamespaceEntry represents the exported view of a security namespace used for CLI output.
type NamespaceEntry struct {
	NamespaceID       string        `json:"namespaceId"`
	Name              string        `json:"name,omitempty"`
	DisplayName       string        `json:"displayName,omitempty"`
	DataspaceCategory string        `json:"dataspaceCategory,omitempty"`
	IsRemotable       *bool         `json:"isRemotable,omitempty"`
	ExtensionType     string        `json:"extensionType,omitempty"`
	ElementLength     *int          `json:"elementLength,omitempty"`
	SeparatorValue    string        `json:"separatorValue,omitempty"`
	WritePermission   string        `json:"writePermission,omitempty"`
	ReadPermission    string        `json:"readPermission,omitempty"`
	UseTranslator     *bool         `json:"useTokenTranslator,omitempty"`
	SystemBitMask     string        `json:"systemBitMask,omitempty"`
	StructureValue    *int          `json:"structureValue,omitempty"`
	ActionsCount      *int          `json:"actionsCount,omitempty"`
	Actions           []ActionEntry `json:"actions,omitempty"`
}

// ActionEntry describes a single permission action within a security namespace.
type ActionEntry struct {
	Bit         *int   `json:"bit,omitempty"`
	BitHex      string `json:"bitHex,omitempty"`
	Name        string `json:"name,omitempty"`
	DisplayName string `json:"displayName,omitempty"`
	NamespaceID string `json:"namespaceId,omitempty"`
}

// TransformNamespace converts the Azure DevOps SDK response model into the CLI-friendly structure.
func TransformNamespace(ns security.SecurityNamespaceDescription) NamespaceEntry {
	var namespaceID string
	if ns.NamespaceId != nil {
		namespaceID = ns.NamespaceId.String()
	}

	entry := NamespaceEntry{
		NamespaceID:       namespaceID,
		Name:              types.GetValue(ns.Name, ""),
		DisplayName:       types.GetValue(ns.DisplayName, ""),
		DataspaceCategory: types.GetValue(ns.DataspaceCategory, ""),
		IsRemotable:       ns.IsRemotable,
		ExtensionType:     types.GetValue(ns.ExtensionType, ""),
		ElementLength:     ns.ElementLength,
		SeparatorValue:    types.GetValue(ns.SeparatorValue, ""),
		UseTranslator:     ns.UseTokenTranslator,
		StructureValue:    ns.StructureValue,
	}

	if ns.SystemBitMask != nil {
		entry.SystemBitMask = fmt.Sprintf("0x%X", *ns.SystemBitMask)
	}
	if ns.WritePermission != nil {
		entry.WritePermission = fmt.Sprintf("0x%X", *ns.WritePermission)
	}
	if ns.ReadPermission != nil {
		entry.ReadPermission = fmt.Sprintf("0x%X", *ns.ReadPermission)
	}

	if ns.Actions != nil {
		actions := make([]ActionEntry, 0, len(*ns.Actions))
		for _, action := range *ns.Actions {
			actionEntry := ActionEntry{
				Name:        types.GetValue(action.Name, ""),
				DisplayName: types.GetValue(action.DisplayName, ""),
			}
			if action.Bit != nil {
				bitValue := *action.Bit
				actionEntry.Bit = &bitValue
				actionEntry.BitHex = fmt.Sprintf("0x%X", bitValue)
			}
			if action.NamespaceId != nil && *action.NamespaceId != uuid.Nil {
				actionEntry.NamespaceID = action.NamespaceId.String()
			}
			actions = append(actions, actionEntry)
		}
		entry.Actions = actions
		count := len(actions)
		entry.ActionsCount = types.ToPtr(count)
	}

	return entry
}

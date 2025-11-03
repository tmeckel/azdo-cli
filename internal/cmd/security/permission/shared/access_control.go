package shared

import (
	"context"
	"fmt"
	"strings"

	"github.com/google/uuid"
	"github.com/microsoft/azure-devops-go-api/azuredevops/v7/security"
	"github.com/tmeckel/azdo-cli/internal/types"
)

// CloneAccessControlEntry returns a deep copy of the provided ACE, including any nested
// extended information pointers. Callers receive a brand new struct so they can mutate the
// result without affecting the original value.
func CloneAccessControlEntry(src *security.AccessControlEntry) *security.AccessControlEntry {
	if src == nil {
		return nil
	}

	clone := security.AccessControlEntry{}

	if src.Allow != nil {
		v := *src.Allow
		clone.Allow = &v
	}
	if src.Deny != nil {
		v := *src.Deny
		clone.Deny = &v
	}
	if src.Descriptor != nil {
		v := strings.TrimSpace(*src.Descriptor)
		clone.Descriptor = &v
	}
	if src.ExtendedInfo != nil {
		clone.ExtendedInfo = CloneAceExtendedInformation(src.ExtendedInfo)
	}

	return &clone
}

// CloneAceExtendedInformation produces a safe, deep copy of the extended ACE information.
func CloneAceExtendedInformation(src *security.AceExtendedInformation) *security.AceExtendedInformation {
	if src == nil {
		return nil
	}

	clone := security.AceExtendedInformation{}

	if src.EffectiveAllow != nil {
		v := *src.EffectiveAllow
		clone.EffectiveAllow = &v
	}
	if src.EffectiveDeny != nil {
		v := *src.EffectiveDeny
		clone.EffectiveDeny = &v
	}
	if src.InheritedAllow != nil {
		v := *src.InheritedAllow
		clone.InheritedAllow = &v
	}
	if src.InheritedDeny != nil {
		v := *src.InheritedDeny
		clone.InheritedDeny = &v
	}

	return &clone
}

// FindAccessControlEntry looks up a descriptor within the specified namespace/token and
// returns a cloned copy of the matching ACE when one exists.
func FindAccessControlEntry(ctx context.Context, client security.Client, namespaceID uuid.UUID, token, descriptor string) (*security.AccessControlEntry, error) {
	if client == nil {
		return nil, fmt.Errorf("security client is required")
	}

	descriptor = strings.TrimSpace(descriptor)
	if descriptor == "" {
		return nil, fmt.Errorf("descriptor is required")
	}

	token = strings.TrimSpace(token)
	args := security.QueryAccessControlListsArgs{
		SecurityNamespaceId: &namespaceID,
		Descriptors:         &descriptor,
		IncludeExtendedInfo: types.ToPtr(true),
	}

	if token != "" {
		tokenCopy := token
		args.Token = &tokenCopy
	}

	descCopy := descriptor
	args.Descriptors = &descCopy

	acls, err := client.QueryAccessControlLists(ctx, args)
	if err != nil {
		return nil, err
	}
	if acls == nil {
		return nil, nil
	}

	for i := range *acls {
		acl := (*acls)[i]
		if acl.AcesDictionary == nil {
			continue
		}
		for _, ace := range *acl.AcesDictionary {
			return CloneAccessControlEntry(&ace), nil
		}
	}

	return nil, nil
}

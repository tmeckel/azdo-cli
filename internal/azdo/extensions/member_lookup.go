package extensions

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"

	"github.com/microsoft/azure-devops-go-api/azuredevops/v7"
	"github.com/microsoft/azure-devops-go-api/azuredevops/v7/graph"
	"github.com/microsoft/azure-devops-go-api/azuredevops/v7/identity"
	"github.com/tmeckel/azdo-cli/internal/azdo/util"
	"github.com/tmeckel/azdo-cli/internal/types"
	"go.uber.org/zap"
)

// ResolveSubject resolves a member identifier (descriptor, email, or principal name) into a graph subject descriptor.
func (c *extensionClient) ResolveSubject(ctx context.Context, member string) (*graph.GraphSubject, error) {
	member = strings.TrimSpace(member)
	if member == "" {
		return nil, fmt.Errorf("member must not be empty")
	}

	graphClient, err := graph.NewClient(ctx, c.conn)
	if err != nil {
		return nil, fmt.Errorf("failed to create graph client: %w", err)
	}

	if util.IsDescriptor(member) {
		zap.L().Debug("attempting graph subject lookup for descriptor", zap.String("descriptor", member))
		subject, err := lookupGraphSubject(ctx, graphClient, member)
		if err != nil {
			return nil, err
		}
		if subject == nil {
			return nil, fmt.Errorf("descriptor %q was not found", member)
		}
		zap.L().Debug("resolved member via graph descriptor lookup",
			zap.String("descriptor", member),
			zap.String("displayName",
				types.GetValue(subject.DisplayName, "")),
			zap.String("subjectKind", types.GetValue(subject.SubjectKind, "")),
			zap.String("legacyDescriptor", types.GetValue(subject.LegacyDescriptor, "")),
		)
		return subject, nil
	}

	identityClient, err := identity.NewClient(ctx, c.conn)
	if err != nil {
		return nil, fmt.Errorf("failed to create identity client: %w", err)
	}

	resolvedIdentity, err := resolveIdentity(ctx, identityClient, member)
	if err != nil {
		return nil, err
	}
	if resolvedIdentity == nil {
		return nil, fmt.Errorf("no identity found for %q", member)
	}

	descriptor := types.GetValue(resolvedIdentity.SubjectDescriptor, "")
	if descriptor == "" {
		if resolvedIdentity.Id == nil {
			return nil, fmt.Errorf("identity for %q is missing descriptor and storage key", member)
		}
		desc, derr := graphClient.GetDescriptor(ctx, graph.GetDescriptorArgs{
			StorageKey: resolvedIdentity.Id,
		})
		if derr != nil {
			return nil, fmt.Errorf("failed to resolve descriptor from storage key: %w", derr)
		}
		descriptor = types.GetValue(desc.Value, "")
		if descriptor == "" {
			return nil, fmt.Errorf("descriptor lookup returned empty result for %q", member)
		}
	}

	subject, err := lookupGraphSubject(ctx, graphClient, descriptor)
	if err != nil {
		return nil, err
	}
	if subject != nil {
		return subject, nil
	}

	displayName := types.GetValue(resolvedIdentity.ProviderDisplayName, "")
	kind := memberSubjectKind(*resolvedIdentity)

	return &graph.GraphSubject{
		Descriptor:  types.ToPtr(descriptor),
		DisplayName: types.ToPtr(displayName),
		SubjectKind: types.ToPtr(kind),
	}, nil
}

func (c *extensionClient) ResolveIdentity(ctx context.Context, member string) (*identity.Identity, error) {
	member = strings.TrimSpace(member)
	if member == "" {
		return nil, fmt.Errorf("member must not be empty")
	}

	identityClient, err := identity.NewClient(ctx, c.conn)
	if err != nil {
		return nil, fmt.Errorf("failed to create identity client: %w", err)
	}

	return resolveIdentity(ctx, identityClient, member)
}

func lookupGraphSubject(ctx context.Context, client graph.Client, descriptor string) (*graph.GraphSubject, error) {
	if strings.TrimSpace(descriptor) == "" {
		return nil, nil
	}

	keys := []graph.GraphSubjectLookupKey{
		{
			Descriptor: types.ToPtr(descriptor),
		},
	}
	subjectLookup := graph.GraphSubjectLookup{
		LookupKeys: &keys,
	}

	result, err := client.LookupSubjects(ctx, graph.LookupSubjectsArgs{
		SubjectLookup: &subjectLookup,
	})
	if err != nil {
		var wrappedErr *azuredevops.WrappedError
		if errors.As(err, &wrappedErr) && wrappedErr != nil && wrappedErr.StatusCode != nil {
			if *wrappedErr.StatusCode == http.StatusNotFound {
				return nil, nil
			}
		}
		return nil, fmt.Errorf("failed to lookup descriptor %q: %w", descriptor, err)
	}

	if result == nil || len(*result) == 0 {
		return nil, nil
	}

	if subject, ok := (*result)[descriptor]; ok {
		res := subject
		return &res, nil
	}

	for _, subject := range *result {
		if types.GetValue(subject.Descriptor, "") == descriptor {
			res := subject
			return &res, nil
		}

		if subject.Descriptor != nil && strings.EqualFold(*subject.Descriptor, descriptor) {
			res := subject
			return &res, nil
		}
	}

	return nil, nil
}

func determineIdentitySearchFilters(member string) []string {
	member = strings.TrimSpace(member)
	memberLower := strings.ToLower(member)

	var filters []string
	if strings.Contains(member, " ") || strings.Contains(member, "@") {
		filters = append(filters, "General", "DirectoryAlias")
	} else {
		filters = append(filters, "DirectoryAlias", "General")
	}
	filters = append(filters, "MailAddress")
	if strings.Contains(member, "\\") {
		filters = append(filters, "AccountName")
	}

	seen := make(map[string]struct{}, len(filters))
	result := make([]string, 0, len(filters))
	for _, f := range filters {
		fl := strings.ToLower(f)
		if _, ok := seen[fl]; ok {
			continue
		}
		seen[fl] = struct{}{}
		result = append(result, f)
	}

	if !strings.Contains(memberLower, "@") && !strings.Contains(memberLower, "\\") {
		if _, ok := seen["localgroupname"]; !ok {
			seen["localgroupname"] = struct{}{}
			result = append(result, "LocalGroupName")
		}
	}

	return result
}

func memberSubjectKind(identity identity.Identity) string {
	if identity.IsContainer != nil && *identity.IsContainer {
		return "Group"
	}
	return "User"
}

func resolveIdentity(ctx context.Context, client identity.Client, member string) (*identity.Identity, error) {
	if util.IsSecurityIdentifier(member) {
		zap.L().Debug("member is a SID", zap.String("member", member))
		descriptor := member
		if !strings.Contains(member, ";") {
			descriptor = "Microsoft.TeamFoundation.Identity;" + member
		}
		identities, err := client.ReadIdentities(ctx, identity.ReadIdentitiesArgs{
			Descriptors:     &descriptor,
			QueryMembership: &identity.QueryMembershipValues.None,
		})
		if err != nil {
			return nil, fmt.Errorf("failed to resolve member %q: %w", member, err)
		}
		if identities == nil || len(*identities) == 0 {
			return nil, fmt.Errorf("identity %q not found", member)
		}
		if len(*identities) > 1 {
			return nil, fmt.Errorf("multiple identities found for %q; specify a more specific identifier", member)
		}

		return &(*identities)[0], nil
	}

	if util.IsDescriptor(member) {
		zap.L().Debug("member is a subject descriptor", zap.String("member", member))
		identities, err := client.ReadIdentities(ctx, identity.ReadIdentitiesArgs{
			SubjectDescriptors: &member,
			QueryMembership:    &identity.QueryMembershipValues.None,
		})
		if err != nil {
			return nil, fmt.Errorf("failed to resolve member %q: %w", member, err)
		}
		if identities == nil || len(*identities) == 0 {
			return nil, fmt.Errorf("identity %q not found", member)
		}
		if len(*identities) > 1 {
			return nil, fmt.Errorf("multiple identities found for %q; specify a more specific identifier", member)
		}

		return &(*identities)[0], nil
	}

	filters := determineIdentitySearchFilters(member)
	for _, filter := range filters {
		localFilter := filter
		zap.L().Debug("resolving member via identity search", zap.String("filter", localFilter), zap.String("value", member))

		identities, err := client.ReadIdentities(ctx, identity.ReadIdentitiesArgs{
			SearchFilter:                &localFilter,
			FilterValue:                 &member,
			QueryMembership:             &identity.QueryMembershipValues.None,
			IncludeRestrictedVisibility: types.ToPtr(true),
		})
		if err != nil {
			return nil, fmt.Errorf("failed to resolve member %q: %w", member, err)
		}
		if identities == nil || len(*identities) == 0 {
			continue
		}
		if len(*identities) > 1 {
			return nil, fmt.Errorf("multiple identities found for %q; specify a more specific identifier", member)
		}

		return &(*identities)[0], nil
	}

	return nil, nil
}

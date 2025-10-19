package shared

import (
	"errors"
	"fmt"
	"net/http"
	"regexp"
	"strings"

	"github.com/microsoft/azure-devops-go-api/azuredevops/v7"
	"github.com/microsoft/azure-devops-go-api/azuredevops/v7/graph"
	"github.com/microsoft/azure-devops-go-api/azuredevops/v7/identity"
	"github.com/tmeckel/azdo-cli/internal/cmd/util"
	"github.com/tmeckel/azdo-cli/internal/types"
	"go.uber.org/zap"
)

var descriptorPattern = regexp.MustCompile(`^[^@\s]+\.[^@\s]+$`)

// ResolveMemberDescriptor resolves a member identifier (descriptor, email, or principal name) into a graph subject descriptor.
func ResolveMemberDescriptor(ctx util.CmdContext, organization, member string) (*graph.GraphSubject, error) {
	member = strings.TrimSpace(member)
	if member == "" {
		return nil, util.FlagErrorf("member must not be empty")
	}

	graphClient, err := ctx.ClientFactory().Graph(ctx.Context(), organization)
	if err != nil {
		return nil, fmt.Errorf("failed to create graph client: %w", err)
	}

	if isDescriptor(member) {
		zap.L().Debug("attempting graph subject lookup for descriptor", zap.String("descriptor", member))
		subject, err := lookupGraphSubject(ctx, graphClient, member)
		if err != nil {
			return nil, err
		}
		if subject == nil {
			return nil, fmt.Errorf("descriptor %q was not found", member)
		}
		zap.L().Debug("resolved member via graph descriptor lookup", zap.String("descriptor", member))
		return subject, nil
	}

	identityClient, err := ctx.ClientFactory().Identity(ctx.Context(), organization)
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
		desc, derr := graphClient.GetDescriptor(ctx.Context(), graph.GetDescriptorArgs{
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

func lookupGraphSubject(ctx util.CmdContext, client graph.Client, descriptor string) (*graph.GraphSubject, error) {
	if strings.TrimSpace(descriptor) == "" {
		return nil, nil
	}

	keys := []graph.GraphSubjectLookupKey{
		{Descriptor: types.ToPtr(descriptor)},
	}
	subjectLookup := graph.GraphSubjectLookup{
		LookupKeys: &keys,
	}

	result, err := client.LookupSubjects(ctx.Context(), graph.LookupSubjectsArgs{
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

func isDescriptor(value string) bool {
	value = strings.TrimSpace(value)
	if value == "" {
		return false
	}
	return descriptorPattern.MatchString(value)
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

	// Avoid duplicates if the heuristics add the same filter multiple times.
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

	// When we suspect a local Azure DevOps group (display name without special characters),
	// include LocalGroupName as a final attempt.
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

func resolveIdentity(ctx util.CmdContext, client identity.Client, member string) (*identity.Identity, error) {
	filters := determineIdentitySearchFilters(member)
	for _, filter := range filters {
		localFilter := filter
		zap.L().Debug("resolving member via identity search", zap.String("filter", localFilter), zap.String("value", member))

		identities, err := client.ReadIdentities(ctx.Context(), identity.ReadIdentitiesArgs{
			SearchFilter:    &localFilter,
			FilterValue:     &member,
			QueryMembership: &identity.QueryMembershipValues.None,
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

		identityResult := (*identities)[0]
		return &identityResult, nil
	}

	return nil, nil
}

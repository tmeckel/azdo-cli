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

// ResolveSubject resolves a single member identifier by delegating to the batch path
// and translating the result back into the legacy single-item API.
func (c *extensionClient) ResolveSubject(ctx context.Context, member string) (*graph.GraphSubject, error) {
	member = strings.TrimSpace(member)
	if member == "" {
		return nil, errors.New("member must not be empty")
	}
	results, err := c.ResolveSubjects(ctx, []string{member})
	if err != nil {
		return nil, err
	}
	return results[member], nil
}

// ResolveSubjects resolves a batch of member identifiers (descriptors, SIDs, emails, or principal names)
// into graph subjects. The implementation partitions inputs by type and dispatches each partition to the
// most appropriate AzDO REST endpoint with native batch semantics:
//
//   - Subject descriptors: one batched graph.LookupSubjects call covering all descriptors.
//   - SIDs: one batched identity.ReadIdentities(Descriptors: csv) call.
//   - Free-form identifiers (emails, UPNs, etc.): per-input identity.ReadIdentities searches followed
//     by a single batched graph.LookupSubjects call to enrich the resulting descriptors.
//
// Inputs that cannot be resolved are simply absent from the result map; the caller can detect failure
// by checking map membership. The map key is the trimmed input string, so duplicate inputs collapse
// to a single entry. Only catastrophic failures (e.g. client construction) return a non-nil error.
func (c *extensionClient) ResolveSubjects(ctx context.Context, members []string) (map[string]*graph.GraphSubject, error) {
	trimmed := make([]string, 0, len(members))
	seen := make(map[string]struct{}, len(members))
	for _, m := range members {
		t := strings.TrimSpace(m)
		if t == "" {
			continue
		}
		if _, ok := seen[t]; ok {
			continue
		}
		seen[t] = struct{}{}
		trimmed = append(trimmed, t)
	}

	if len(trimmed) == 0 {
		return map[string]*graph.GraphSubject{}, nil
	}

	graphClient, err := graph.NewClient(ctx, c.conn)
	if err != nil {
		return nil, fmt.Errorf("failed to create graph client: %w", err)
	}
	identityClient, err := identity.NewClient(ctx, c.conn)
	if err != nil {
		return nil, fmt.Errorf("failed to create identity client: %w", err)
	}

	result := make(map[string]*graph.GraphSubject, len(trimmed))

	descriptorInputs, sidInputs, otherInputs := partitionInputs(trimmed)

	if len(descriptorInputs) > 0 {
		if err := resolveDescriptorBatch(ctx, graphClient, descriptorInputs, result); err != nil {
			return nil, err
		}
	}
	if len(sidInputs) > 0 {
		if err := resolveSIDBatch(ctx, identityClient, graphClient, sidInputs, result); err != nil {
			return nil, err
		}
	}
	if len(otherInputs) > 0 {
		if err := resolveSearchBatch(ctx, identityClient, graphClient, otherInputs, result); err != nil {
			return nil, err
		}
	}

	return result, nil
}

// partitionInputs splits inputs into the three batches the AzDO REST API can serve most efficiently.
func partitionInputs(inputs []string) (descriptors, sids, others []string) {
	for _, m := range inputs {
		switch {
		case util.IsDescriptor(m):
			descriptors = append(descriptors, m)
		case util.IsIdentitySID(m):
			sids = append(sids, m)
		default:
			others = append(others, m)
		}
	}
	return descriptors, sids, others
}

// resolveDescriptorBatch resolves a batch of subject descriptors via a single graph.LookupSubjects call.
func resolveDescriptorBatch(ctx context.Context, graphClient graph.Client, inputs []string, out map[string]*graph.GraphSubject) error {
	keys := subjectLookupKeys(inputs)

	subjects, err := graphClient.LookupSubjects(ctx, graph.LookupSubjectsArgs{
		SubjectLookup: &graph.GraphSubjectLookup{LookupKeys: &keys},
	})
	if err != nil {
		if isNotFoundError(err) {
			return nil
		}
		return fmt.Errorf("failed to lookup %d descriptor(s): %w", len(inputs), err)
	}
	if subjects == nil {
		return nil
	}

	indexed := indexSubjectsByDescriptor(subjects)
	for _, input := range inputs {
		if subject, ok := indexed[input]; ok {
			s := subject
			out[input] = &s
		}
	}
	return nil
}

// resolveSIDBatch resolves a batch of SIDs via a single identity.ReadIdentities(Descriptors: csv) call,
// then enriches the resulting descriptors with a single graph.LookupSubjects call.
//
// Correlation between the requested SIDs and the returned identities relies on the AzDO IMS API
// echoing the requested identity descriptor in the response's identity.Identity.Descriptor field.
// The AzDO documentation treats identity descriptors as a first-class lookup identifier, so a
// non-echoing response is treated as a contract drift and surfaced as an error rather than
// silently falling back to positional correlation. If this error is ever observed in production,
// the resolution pipeline needs a fallback path (per-input lookups or positional correlation
// over the request list).
func resolveSIDBatch(ctx context.Context, identityClient identity.Client, graphClient graph.Client, inputs []string, out map[string]*graph.GraphSubject) error {
	prefixedDescriptors := make([]string, 0, len(inputs))
	originalByDescriptor := make(map[string]string, len(inputs))
	for _, sid := range inputs {
		descriptor := util.NormalizeIdentitySID(sid)
		prefixedDescriptors = append(prefixedDescriptors, descriptor)
		originalByDescriptor[descriptor] = sid
	}

	csv := strings.Join(prefixedDescriptors, ",")
	identities, err := identityClient.ReadIdentities(ctx, identity.ReadIdentitiesArgs{
		Descriptors:     &csv,
		QueryMembership: &identity.QueryMembershipValues.None,
	})
	if err != nil {
		return fmt.Errorf("failed to resolve %d SID(s): %w", len(inputs), err)
	}
	if identities == nil || len(*identities) == 0 {
		return nil
	}

	type resolvedSlot struct {
		original   string
		descriptor string
		identity   identity.Identity
	}
	slots := make([]resolvedSlot, 0, len(*identities))

	for _, id := range *identities {
		requestedDescriptor := types.GetValue(id.Descriptor, "")
		original, ok := originalByDescriptor[requestedDescriptor]
		if !ok {
			return fmt.Errorf("ReadIdentities returned identity descriptor %q which was not requested", requestedDescriptor)
		}
		subjectDescriptor := types.GetValue(id.SubjectDescriptor, "")
		if subjectDescriptor == "" {
			continue
		}
		slots = append(slots, resolvedSlot{
			original:   original,
			descriptor: subjectDescriptor,
			identity:   id,
		})
	}
	if len(slots) == 0 {
		return nil
	}

	subjectDescriptors := make([]string, len(slots))
	for i, s := range slots {
		subjectDescriptors[i] = s.descriptor
	}
	keys := subjectLookupKeys(subjectDescriptors)
	subjects, err := graphClient.LookupSubjects(ctx, graph.LookupSubjectsArgs{
		SubjectLookup: &graph.GraphSubjectLookup{LookupKeys: &keys},
	})
	if err != nil && !isNotFoundError(err) {
		return fmt.Errorf("failed to enrich %d SID(s): %w", len(inputs), err)
	}

	indexed := indexSubjectsByDescriptor(subjects)
	for _, s := range slots {
		if subject, ok := indexed[s.descriptor]; ok {
			outS := subject
			out[s.original] = &outS
			continue
		}
		out[s.original] = graphSubjectFromIdentity(s.identity, s.descriptor)
	}
	return nil
}

// subjectLookupKeys builds a slice of GraphSubjectLookupKey with each descriptor wired up
// as the lookup key. Callers hand the result to graph.LookupSubjects.
func subjectLookupKeys(descriptors []string) []graph.GraphSubjectLookupKey {
	keys := make([]graph.GraphSubjectLookupKey, 0, len(descriptors))
	for _, d := range descriptors {
		keys = append(keys, graph.GraphSubjectLookupKey{Descriptor: &d})
	}
	return keys
}

// indexSubjectsByDescriptor returns a lookup map keyed by the exact descriptor of each
// subject, so callers can resolve subjects in O(1). Callers receive nil for a nil input
// map, which is safe to read from (every lookup misses).
func indexSubjectsByDescriptor(subjects *map[string]graph.GraphSubject) map[string]graph.GraphSubject {
	if subjects == nil {
		return nil
	}
	indexed := make(map[string]graph.GraphSubject, len(*subjects))
	for _, subject := range *subjects {
		if subject.Descriptor == nil {
			continue
		}
		indexed[*subject.Descriptor] = subject
	}
	return indexed
}

// graphSubjectFromIdentity builds a minimal graph.GraphSubject from an identity record
// when enrichment via LookupSubjects does not return a matching subject.
func graphSubjectFromIdentity(id identity.Identity, descriptor string) *graph.GraphSubject {
	return &graph.GraphSubject{
		Descriptor:  types.ToPtr(descriptor),
		DisplayName: types.ToPtr(types.GetValue(id.ProviderDisplayName, "")),
		SubjectKind: types.ToPtr(memberSubjectKind(id)),
	}
}

// isNotFoundError reports whether err is an azuredevops.WrappedError with a 404 status code.
// AzDO REST endpoints that are allowed to fail-soft return (nil, nil) for these, while other
// errors propagate to the caller.
func isNotFoundError(err error) bool {
	if err == nil {
		return false
	}
	var wrappedErr *azuredevops.WrappedError
	if !errors.As(err, &wrappedErr) || wrappedErr == nil || wrappedErr.StatusCode == nil {
		return false
	}
	return *wrappedErr.StatusCode == http.StatusNotFound
}

// resolveSearchBatch resolves free-form identifiers (emails, UPNs, account names) via per-input
// identity.ReadIdentities searches. Once all descriptors are collected, a single graph.LookupSubjects
// call enriches them with display name, subject kind, and legacy descriptor fields.
func resolveSearchBatch(ctx context.Context, identityClient identity.Client, graphClient graph.Client, inputs []string, out map[string]*graph.GraphSubject) error {
	descriptorToInput := make(map[string]string, len(inputs))
	descriptorFallback := make(map[string]identity.Identity, len(inputs))
	descriptors := make([]string, 0, len(inputs))

	for _, member := range inputs {
		identity, err := resolveIdentity(ctx, identityClient, member)
		if err != nil {
			return fmt.Errorf("failed to resolve member %q: %w", member, err)
		}
		if identity == nil {
			continue
		}

		descriptor := types.GetValue(identity.SubjectDescriptor, "")
		if descriptor == "" {
			if identity.Id == nil {
				continue
			}
			desc, derr := graphClient.GetDescriptor(ctx, graph.GetDescriptorArgs{
				StorageKey: identity.Id,
			})
			if derr != nil {
				return fmt.Errorf("failed to resolve descriptor from storage key for %q: %w", member, derr)
			}
			descriptor = types.GetValue(desc.Value, "")
			if descriptor == "" {
				continue
			}
		}

		if existing, dup := descriptorToInput[descriptor]; dup {
			zap.L().Debug(
				"multiple inputs resolve to same descriptor; keeping first",
				zap.String("kept", existing),
				zap.String("ignored", member),
				zap.String("descriptor", descriptor),
			)
			continue
		}
		descriptorToInput[descriptor] = member
		descriptorFallback[descriptor] = *identity
		descriptors = append(descriptors, descriptor)
	}

	if len(descriptors) == 0 {
		return nil
	}

	keys := subjectLookupKeys(descriptors)
	subjects, err := graphClient.LookupSubjects(ctx, graph.LookupSubjectsArgs{
		SubjectLookup: &graph.GraphSubjectLookup{LookupKeys: &keys},
	})
	if err != nil && !isNotFoundError(err) {
		return fmt.Errorf("failed to enrich %d member(s): %w", len(descriptorToInput), err)
	}

	indexed := indexSubjectsByDescriptor(subjects)
	for descriptor, originalInput := range descriptorToInput {
		if subject, ok := indexed[descriptor]; ok {
			s := subject
			out[originalInput] = &s
			continue
		}
		out[originalInput] = graphSubjectFromIdentity(descriptorFallback[descriptor], descriptor)
	}
	return nil
}

func (c *extensionClient) ResolveIdentity(ctx context.Context, member string) (*identity.Identity, error) {
	member = strings.TrimSpace(member)
	if member == "" {
		return nil, errors.New("member must not be empty")
	}

	identityClient, err := identity.NewClient(ctx, c.conn)
	if err != nil {
		return nil, fmt.Errorf("failed to create identity client: %w", err)
	}

	return resolveIdentity(ctx, identityClient, member)
}

// determineIdentitySearchFilters returns the AzDO Identity SearchFilter values appropriate
// for the given free-form member string. The Identities REST API supports a fixed set of
// filter values that can also be passed as a CSV list; the supported values are:
//
//   - "AccountName"
//   - "DisplayName"
//   - "AdministratorsGroup"
//   - "Identifier"
//   - "MailAddress"
//   - "General"
//   - "Alias"
//   - "DirectoryAlias"
//   - "TeamGroupName"
//   - "LocalGroupName"
func determineIdentitySearchFilters(member string) []string {
	var filters []string
	if strings.Contains(member, "@") {
		filters = append(filters, "MailAddress", "AccountName")
	}
	if strings.Contains(member, "\\") {
		filters = append(filters, "AccountName")
	}
	if len(filters) == 0 {
		filters = append(filters, "General", "AccountName", "DirectoryAlias", "LocalGroupName")
	}

	return types.UniqueComparable(filters, strings.ToLower)
}

func memberSubjectKind(identity identity.Identity) string {
	if identity.IsContainer != nil && *identity.IsContainer {
		return "Group"
	}
	return "User"
}

// singleIdentity returns the unique identity from a ReadIdentities result, or an error
// if the result is missing or ambiguous. The member string is included in error messages.
func singleIdentity(member string, identities *[]identity.Identity) (*identity.Identity, error) {
	if identities == nil || len(*identities) == 0 {
		return nil, fmt.Errorf("identity %q not found", member)
	}
	if len(*identities) > 1 {
		return nil, fmt.Errorf("multiple identities found for %q; specify a more specific identifier", member)
	}
	return &(*identities)[0], nil
}

func resolveIdentity(ctx context.Context, client identity.Client, member string) (*identity.Identity, error) {
	if util.IsIdentitySID(member) {
		zap.L().Debug("member is a SID", zap.String("member", member))
		descriptor := util.NormalizeIdentitySID(member)
		identities, err := client.ReadIdentities(ctx, identity.ReadIdentitiesArgs{
			Descriptors:     &descriptor,
			QueryMembership: &identity.QueryMembershipValues.None,
		})
		if err != nil {
			return nil, fmt.Errorf("failed to resolve member %q: %w", member, err)
		}
		return singleIdentity(member, identities)
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
		return singleIdentity(member, identities)
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
		return singleIdentity(member, identities)
	}

	return nil, nil
}

package extensions

import (
	"context"
	"errors"
	"net/http"
	"testing"

	"github.com/google/uuid"
	"github.com/microsoft/azure-devops-go-api/azuredevops/v7"
	"github.com/microsoft/azure-devops-go-api/azuredevops/v7/graph"
	"github.com/microsoft/azure-devops-go-api/azuredevops/v7/identity"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/tmeckel/azdo-cli/internal/types"
)

func Test_determineIdentitySearchFilters(t *testing.T) {
	testCases := []struct {
		name   string
		member string
		want   []string
	}{
		{
			name:   "email",
			member: "user@example.com",
			want:   []string{"AccountName", "MailAddress"},
		},
		{
			name:   "account name with slash",
			member: "DOMAIN\\user",
			want:   []string{"AccountName"},
		},
		{
			name:   "name with space",
			member: "First Last",
			want:   []string{"General", "AccountName", "DirectoryAlias", "LocalGroupName"},
		},
		{
			name:   "mixed content",
			member: "user@domain\\other",
			want:   []string{"AccountName", "MailAddress"},
		},
		{
			name:   "trimmed input is used",
			member: "  alias  ",
			want:   []string{"DirectoryAlias", "AccountName", "General", "LocalGroupName"},
		},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			got := determineIdentitySearchFilters(tc.member)
			assert.True(t, types.CompareUnorderedSlices(tc.want, got), "want:%+v != got:%+v", tc.want, got)
		})
	}
}

func Test_memberSubjectKind(t *testing.T) {
	t.Run("returns group when IsContainer true", func(t *testing.T) {
		kind := memberSubjectKind(identity.Identity{IsContainer: types.ToPtr(true)})
		assert.Equal(t, "Group", kind)
	})

	t.Run("returns user when IsContainer false", func(t *testing.T) {
		kind := memberSubjectKind(identity.Identity{IsContainer: types.ToPtr(false)})
		assert.Equal(t, "User", kind)
	})

	t.Run("returns user when IsContainer nil", func(t *testing.T) {
		kind := memberSubjectKind(identity.Identity{})
		assert.Equal(t, "User", kind)
	})
}

// stubGraphClient satisfies the graph.Client interface but only honors the methods invoked
// in the tests below. Any other call will panic, surfacing unexpected invocations loudly.
type stubGraphClient struct {
	graph.Client
	lookupSubjectsFunc func(context.Context, graph.LookupSubjectsArgs) (*map[string]graph.GraphSubject, error)
	getDescriptorFunc  func(context.Context, graph.GetDescriptorArgs) (*graph.GraphDescriptorResult, error)
}

func (s *stubGraphClient) LookupSubjects(ctx context.Context, args graph.LookupSubjectsArgs) (*map[string]graph.GraphSubject, error) {
	return s.lookupSubjectsFunc(ctx, args)
}

func (s *stubGraphClient) GetDescriptor(ctx context.Context, args graph.GetDescriptorArgs) (*graph.GraphDescriptorResult, error) {
	return s.getDescriptorFunc(ctx, args)
}

// stubIdentityClient satisfies the identity.Client interface but only honors the methods invoked
// in the tests below.
type stubIdentityClient struct {
	identity.Client
	readIdentitiesFunc func(context.Context, identity.ReadIdentitiesArgs) (*[]identity.Identity, error)
}

func (s *stubIdentityClient) ReadIdentities(ctx context.Context, args identity.ReadIdentitiesArgs) (*[]identity.Identity, error) {
	return s.readIdentitiesFunc(ctx, args)
}

var (
	_ graph.Client    = (*stubGraphClient)(nil)
	_ identity.Client = (*stubIdentityClient)(nil)
)

// lookupDescriptors extracts the descriptor strings from a LookupSubjectsArgs call.
func lookupDescriptors(t *testing.T, args graph.LookupSubjectsArgs) []string {
	t.Helper()
	require.NotNil(t, args.SubjectLookup)
	require.NotNil(t, args.SubjectLookup.LookupKeys)
	got := make([]string, 0, len(*args.SubjectLookup.LookupKeys))
	for _, k := range *args.SubjectLookup.LookupKeys {
		got = append(got, types.GetValue(k.Descriptor, ""))
	}
	return got
}

// wrappedStatus returns an azuredevops.WrappedError with the given HTTP status code.
func wrappedStatus(code int) error {
	return &azuredevops.WrappedError{StatusCode: types.ToPtr(code)}
}

func mustSubject(t *testing.T, m map[string]*graph.GraphSubject, key string) *graph.GraphSubject {
	t.Helper()
	got, ok := m[key]
	require.Truef(t, ok, "expected key %q in result map; got keys=%v", key, mapKeys(m))
	return got
}

func mapKeys(m map[string]*graph.GraphSubject) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	return keys
}

// splitCSV is a tiny local helper that mirrors the SDK expectation of comma-separated descriptor values.
func splitCSV(value string) []string {
	out := make([]string, 0)
	current := ""
	for _, r := range value {
		if r == ',' {
			out = append(out, current)
			current = ""
			continue
		}
		current += string(r)
	}
	out = append(out, current)
	return out
}

// subject builds a graph.GraphSubject with the given descriptor and display name. Use this
// in test fixtures to keep subjects readable; inline literals are still fine when SubjectKind
// is part of what the test is asserting on.
func subject(descriptor, name string) graph.GraphSubject {
	return graph.GraphSubject{
		Descriptor:  types.ToPtr(descriptor),
		DisplayName: types.ToPtr(name),
	}
}

// identityWithSubject builds an identity record correlated with a graph subject. Use this for
// test fixtures that need the identity descriptor, subject descriptor, and display name.
func identityWithSubject(identityDescriptor, subjectDescriptor, displayName string) identity.Identity {
	return identity.Identity{
		Descriptor:          types.ToPtr(identityDescriptor),
		SubjectDescriptor:   types.ToPtr(subjectDescriptor),
		ProviderDisplayName: types.ToPtr(displayName),
	}
}

// identityWithStorageKey builds an identity record that only carries a storage key (UUID).
// Use this for test fixtures that exercise the GetDescriptor fallback path.
func identityWithStorageKey(id string) identity.Identity {
	uid := uuid.MustParse(id)
	return identity.Identity{Id: &uid}
}

func Test_partitionInputs(t *testing.T) {
	t.Run("routes descriptors, SIDs, and others to correct buckets", func(t *testing.T) {
		inputs := []string{
			"aad.YR5kM",
			"s-1-2-34-5678",
			"alice@contoso.com",
			"vssgp.Uy0xLTkt",
			"DOMAIN\\user",
		}
		descriptors, sids, others := partitionInputs(inputs)
		assert.Equal(t, []string{"aad.YR5kM", "vssgp.Uy0xLTkt"}, descriptors)
		assert.Equal(t, []string{"s-1-2-34-5678"}, sids)
		assert.Equal(t, []string{"alice@contoso.com", "DOMAIN\\user"}, others)
	})

	t.Run("empty input yields all-empty slices", func(t *testing.T) {
		descriptors, sids, others := partitionInputs(nil)
		assert.Empty(t, descriptors)
		assert.Empty(t, sids)
		assert.Empty(t, others)
	})

	t.Run("prefixed SIDs route to the SID bucket", func(t *testing.T) {
		descriptors, sids, others := partitionInputs([]string{
			"Microsoft.TeamFoundation.Identity;s-1-2-34-5678",
		})
		assert.Empty(t, descriptors)
		assert.Equal(t, []string{"Microsoft.TeamFoundation.Identity;s-1-2-34-5678"}, sids)
		assert.Empty(t, others)
	})
}

func Test_resolveDescriptorBatch_SingleCallCoversAll(t *testing.T) {
	inputs := []string{"aad.A1", "aad.A2", "aad.A3"}
	subjects := map[string]graph.GraphSubject{
		"aad.A1": subject("aad.A1", "Alice"),
		"aad.A2": subject("aad.A2", "Bob"),
		"aad.A3": subject("aad.A3", "Carol"),
	}

	graphClient := &stubGraphClient{
		lookupSubjectsFunc: func(_ context.Context, args graph.LookupSubjectsArgs) (*map[string]graph.GraphSubject, error) {
			assert.ElementsMatch(t, inputs, lookupDescriptors(t, args), "all descriptors should be batched into one call")
			return &subjects, nil
		},
	}

	out := make(map[string]*graph.GraphSubject)
	require.NoError(t, resolveDescriptorBatch(context.Background(), graphClient, inputs, out))

	assert.Len(t, out, 3)
	assert.Equal(t, "Alice", types.GetValue(mustSubject(t, out, "aad.A1").DisplayName, ""))
	assert.Equal(t, "Bob", types.GetValue(mustSubject(t, out, "aad.A2").DisplayName, ""))
	assert.Equal(t, "Carol", types.GetValue(mustSubject(t, out, "aad.A3").DisplayName, ""))
}

func Test_resolveDescriptorBatch_PartialResults(t *testing.T) {
	subjects := map[string]graph.GraphSubject{
		"aad.A1": subject("aad.A1", "Alice"),
	}
	graphClient := &stubGraphClient{
		lookupSubjectsFunc: func(_ context.Context, _ graph.LookupSubjectsArgs) (*map[string]graph.GraphSubject, error) {
			return &subjects, nil
		},
	}

	out := make(map[string]*graph.GraphSubject)
	require.NoError(t, resolveDescriptorBatch(context.Background(), graphClient, []string{"aad.A1", "aad.MISSING"}, out))

	_, hasMissing := out["aad.MISSING"]
	assert.False(t, hasMissing, "missing descriptor should not appear in result map")
	_, hasA1 := out["aad.A1"]
	assert.True(t, hasA1, "found descriptor should be in result map")
}

func Test_resolveDescriptorBatch_NotFoundIsSilent(t *testing.T) {
	graphClient := &stubGraphClient{
		lookupSubjectsFunc: func(_ context.Context, _ graph.LookupSubjectsArgs) (*map[string]graph.GraphSubject, error) {
			return nil, wrappedStatus(http.StatusNotFound)
		},
	}

	out := make(map[string]*graph.GraphSubject)
	require.NoError(t, resolveDescriptorBatch(context.Background(), graphClient, []string{"aad.A1"}, out))
	assert.Empty(t, out, "404 from LookupSubjects should not produce a result, not an error")
}

func Test_resolveDescriptorBatch_OtherErrorsPropagate(t *testing.T) {
	graphClient := &stubGraphClient{
		lookupSubjectsFunc: func(_ context.Context, _ graph.LookupSubjectsArgs) (*map[string]graph.GraphSubject, error) {
			return nil, wrappedStatus(http.StatusInternalServerError)
		},
	}

	out := make(map[string]*graph.GraphSubject)
	err := resolveDescriptorBatch(context.Background(), graphClient, []string{"aad.A1"}, out)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to lookup")
}

func Test_resolveSIDBatch_SingleReadIdentitiesCall(t *testing.T) {
	enriched := map[string]graph.GraphSubject{
		"vssgp.U111": subject("vssgp.U111", "SidOne"),
		"vssgp.U222": subject("vssgp.U222", "SidTwo"),
	}
	graphClient := &stubGraphClient{
		lookupSubjectsFunc: func(_ context.Context, args graph.LookupSubjectsArgs) (*map[string]graph.GraphSubject, error) {
			assert.ElementsMatch(t, []string{"vssgp.U111", "vssgp.U222"}, lookupDescriptors(t, args))
			return &enriched, nil
		},
	}

	identityClient := &stubIdentityClient{
		readIdentitiesFunc: func(_ context.Context, args identity.ReadIdentitiesArgs) (*[]identity.Identity, error) {
			require.NotNil(t, args.Descriptors)
			got := splitCSV(*args.Descriptors)
			assert.ElementsMatch(t, []string{
				"Microsoft.TeamFoundation.Identity;s-1-2-34-1111",
				"Microsoft.TeamFoundation.Identity;s-1-2-34-2222",
			}, got)
			id1 := identityWithSubject("Microsoft.TeamFoundation.Identity;s-1-2-34-1111", "vssgp.U111", "SidOne")
			id2 := identityWithSubject("Microsoft.TeamFoundation.Identity;s-1-2-34-2222", "vssgp.U222", "SidTwo")
			return &[]identity.Identity{id2, id1}, nil
		},
	}

	inputs := []string{"s-1-2-34-1111", "s-1-2-34-2222"}
	out := make(map[string]*graph.GraphSubject)
	require.NoError(t, resolveSIDBatch(context.Background(), identityClient, graphClient, inputs, out))

	assert.Len(t, out, 2)
	assert.Equal(t, "vssgp.U111", types.GetValue(mustSubject(t, out, "s-1-2-34-1111").Descriptor, ""))
	assert.Equal(t, "vssgp.U222", types.GetValue(mustSubject(t, out, "s-1-2-34-2222").Descriptor, ""))
}

func Test_resolveSIDBatch_PreservesAlreadyPrefixedDescriptor(t *testing.T) {
	enriched := map[string]graph.GraphSubject{
		"vssgp.U111": subject("vssgp.U111", "SidOne"),
	}
	graphClient := &stubGraphClient{
		lookupSubjectsFunc: func(_ context.Context, _ graph.LookupSubjectsArgs) (*map[string]graph.GraphSubject, error) {
			return &enriched, nil
		},
	}
	identityClient := &stubIdentityClient{
		readIdentitiesFunc: func(_ context.Context, args identity.ReadIdentitiesArgs) (*[]identity.Identity, error) {
			require.NotNil(t, args.Descriptors)
			assert.Equal(t, "Microsoft.TeamFoundation.Identity;s-1-2-34-1111", *args.Descriptors)
			return &[]identity.Identity{
				identityWithSubject("Microsoft.TeamFoundation.Identity;s-1-2-34-1111", "vssgp.U111", "SidOne"),
			}, nil
		},
	}

	inputs := []string{"Microsoft.TeamFoundation.Identity;s-1-2-34-1111"}
	out := make(map[string]*graph.GraphSubject)
	require.NoError(t, resolveSIDBatch(context.Background(), identityClient, graphClient, inputs, out))

	assert.Len(t, out, 1)
	assert.Equal(t, "vssgp.U111", types.GetValue(mustSubject(t, out, inputs[0]).Descriptor, ""))
}

func Test_resolveSIDBatch_EmptyResultProducesNoEnrichmentCall(t *testing.T) {
	graphClient := &stubGraphClient{
		lookupSubjectsFunc: func(_ context.Context, _ graph.LookupSubjectsArgs) (*map[string]graph.GraphSubject, error) {
			t.Fatal("LookupSubjects should not be called when ReadIdentities returns no identities")
			return nil, nil
		},
	}
	identityClient := &stubIdentityClient{
		readIdentitiesFunc: func(_ context.Context, _ identity.ReadIdentitiesArgs) (*[]identity.Identity, error) {
			return &[]identity.Identity{}, nil
		},
	}

	out := make(map[string]*graph.GraphSubject)
	require.NoError(t, resolveSIDBatch(context.Background(), identityClient, graphClient, []string{"s-1-2-34-1111"}, out))
	assert.Empty(t, out)
}

func Test_resolveSIDBatch_FallsBackToIdentityWhenEnrichment404s(t *testing.T) {
	graphClient := &stubGraphClient{
		lookupSubjectsFunc: func(_ context.Context, _ graph.LookupSubjectsArgs) (*map[string]graph.GraphSubject, error) {
			return nil, wrappedStatus(http.StatusNotFound)
		},
	}
	identityClient := &stubIdentityClient{
		readIdentitiesFunc: func(_ context.Context, _ identity.ReadIdentitiesArgs) (*[]identity.Identity, error) {
			id := identityWithSubject("Microsoft.TeamFoundation.Identity;s-1-2-34-1111", "vssgp.U111", "SidOne")
			id.IsContainer = types.ToPtr(false)
			return &[]identity.Identity{id}, nil
		},
	}

	out := make(map[string]*graph.GraphSubject)
	require.NoError(t, resolveSIDBatch(context.Background(), identityClient, graphClient, []string{"s-1-2-34-1111"}, out))

	require.Len(t, out, 1)
	fallback := mustSubject(t, out, "s-1-2-34-1111")
	assert.Equal(t, "vssgp.U111", types.GetValue(fallback.Descriptor, ""))
	assert.Equal(t, "SidOne", types.GetValue(fallback.DisplayName, ""))
	assert.Equal(t, "User", types.GetValue(fallback.SubjectKind, ""))
}

func Test_resolveSIDBatch_PropagatesReadIdentitiesError(t *testing.T) {
	graphClient := &stubGraphClient{}
	identityClient := &stubIdentityClient{
		readIdentitiesFunc: func(_ context.Context, _ identity.ReadIdentitiesArgs) (*[]identity.Identity, error) {
			return nil, errors.New("network down")
		},
	}

	out := make(map[string]*graph.GraphSubject)
	err := resolveSIDBatch(context.Background(), identityClient, graphClient, []string{"s-1-2-34-1111"}, out)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to resolve")
}

func Test_resolveSIDBatch_ErrorsWhenIdentityDescriptorNotEchoed(t *testing.T) {
	graphClient := &stubGraphClient{
		lookupSubjectsFunc: func(_ context.Context, _ graph.LookupSubjectsArgs) (*map[string]graph.GraphSubject, error) {
			t.Fatal("LookupSubjects must not be called when correlation fails")
			return nil, nil
		},
	}
	identityClient := &stubIdentityClient{
		readIdentitiesFunc: func(_ context.Context, _ identity.ReadIdentitiesArgs) (*[]identity.Identity, error) {
			return &[]identity.Identity{
				{
					SubjectDescriptor: types.ToPtr("vssgp.U111"),
				},
			}, nil
		},
	}

	out := make(map[string]*graph.GraphSubject)
	err := resolveSIDBatch(context.Background(), identityClient, graphClient, []string{"s-1-2-34-1111"}, out)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "which was not requested")
	assert.Empty(t, out)
}

func Test_resolveSIDBatch_ErrorsWhenIdentityDescriptorMismatched(t *testing.T) {
	graphClient := &stubGraphClient{
		lookupSubjectsFunc: func(_ context.Context, _ graph.LookupSubjectsArgs) (*map[string]graph.GraphSubject, error) {
			t.Fatal("LookupSubjects must not be called when correlation fails")
			return nil, nil
		},
	}
	identityClient := &stubIdentityClient{
		readIdentitiesFunc: func(_ context.Context, _ identity.ReadIdentitiesArgs) (*[]identity.Identity, error) {
			return &[]identity.Identity{
				identityWithSubject("Microsoft.TeamFoundation.Identity;s-something-else", "vssgp.U111", ""),
			}, nil
		},
	}

	out := make(map[string]*graph.GraphSubject)
	err := resolveSIDBatch(context.Background(), identityClient, graphClient, []string{"s-1-2-34-1111"}, out)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "which was not requested")
	assert.Contains(t, err.Error(), "Microsoft.TeamFoundation.Identity;s-something-else")
	assert.Empty(t, out)
}

func Test_resolveSearchBatch_PerInputSearchesAndBatchedEnrichment(t *testing.T) {
	enriched := map[string]graph.GraphSubject{
		"aad.A1": subject("aad.A1", "Alice"),
		"aad.A2": subject("aad.A2", "Bob"),
	}
	graphClient := &stubGraphClient{
		lookupSubjectsFunc: func(_ context.Context, args graph.LookupSubjectsArgs) (*map[string]graph.GraphSubject, error) {
			assert.ElementsMatch(t, []string{"aad.A1", "aad.A2"}, lookupDescriptors(t, args))
			return &enriched, nil
		},
	}

	searchCalls := 0
	identityClient := &stubIdentityClient{
		readIdentitiesFunc: func(_ context.Context, args identity.ReadIdentitiesArgs) (*[]identity.Identity, error) {
			searchCalls++
			require.NotNil(t, args.SearchFilter)
			require.NotNil(t, args.FilterValue)
			assert.Contains(t, []string{"MailAddress", "AccountName"}, *args.SearchFilter, "should use one of the email/UPN filters")
			switch *args.FilterValue {
			case "alice@contoso.com":
				return &[]identity.Identity{{SubjectDescriptor: types.ToPtr("aad.A1")}}, nil
			case "bob@contoso.com":
				return &[]identity.Identity{{SubjectDescriptor: types.ToPtr("aad.A2")}}, nil
			}
			t.Fatalf("unexpected filter value %q", *args.FilterValue)
			return nil, nil
		},
	}

	inputs := []string{"alice@contoso.com", "bob@contoso.com"}
	out := make(map[string]*graph.GraphSubject)
	require.NoError(t, resolveSearchBatch(context.Background(), identityClient, graphClient, inputs, out))

	assert.Equal(t, 2, searchCalls, "each search input triggers a separate ReadIdentities call")
	assert.Len(t, out, 2)
	assert.Equal(t, "Alice", types.GetValue(mustSubject(t, out, "alice@contoso.com").DisplayName, ""))
	assert.Equal(t, "Bob", types.GetValue(mustSubject(t, out, "bob@contoso.com").DisplayName, ""))
}

func Test_resolveSearchBatch_NotFoundInputSkipped(t *testing.T) {
	graphClient := &stubGraphClient{
		lookupSubjectsFunc: func(_ context.Context, _ graph.LookupSubjectsArgs) (*map[string]graph.GraphSubject, error) {
			t.Fatal("LookupSubjects should not be called when nothing was resolved")
			return nil, nil
		},
	}
	identityClient := &stubIdentityClient{
		readIdentitiesFunc: func(_ context.Context, _ identity.ReadIdentitiesArgs) (*[]identity.Identity, error) {
			return &[]identity.Identity{}, nil
		},
	}

	out := make(map[string]*graph.GraphSubject)
	require.NoError(t, resolveSearchBatch(context.Background(), identityClient, graphClient, []string{"ghost@x.com"}, out))
	assert.Empty(t, out, "unresolved search input produces no entry, no error")
}

func Test_resolveSearchBatch_ResolvesDescriptorFromStorageKey(t *testing.T) {
	graphClient := &stubGraphClient{
		lookupSubjectsFunc: func(_ context.Context, _ graph.LookupSubjectsArgs) (*map[string]graph.GraphSubject, error) {
			return &map[string]graph.GraphSubject{
				"vssgp.STORAGE": subject("vssgp.STORAGE", "StorageUser"),
			}, nil
		},
		getDescriptorFunc: func(_ context.Context, _ graph.GetDescriptorArgs) (*graph.GraphDescriptorResult, error) {
			return &graph.GraphDescriptorResult{Value: types.ToPtr("vssgp.STORAGE")}, nil
		},
	}
	identityClient := &stubIdentityClient{
		readIdentitiesFunc: func(_ context.Context, _ identity.ReadIdentitiesArgs) (*[]identity.Identity, error) {
			return &[]identity.Identity{identityWithStorageKey("00000000-0000-0000-0000-000000000001")}, nil
		},
	}

	out := make(map[string]*graph.GraphSubject)
	require.NoError(t, resolveSearchBatch(context.Background(), identityClient, graphClient, []string{"alice@x.com"}, out))

	require.Len(t, out, 1)
	assert.Equal(t, "vssgp.STORAGE", types.GetValue(mustSubject(t, out, "alice@x.com").Descriptor, ""))
}

func Test_resolveSearchBatch_EmptyGetDescriptorValueSkipsInput(t *testing.T) {
	graphClient := &stubGraphClient{
		lookupSubjectsFunc: func(_ context.Context, _ graph.LookupSubjectsArgs) (*map[string]graph.GraphSubject, error) {
			t.Fatal("LookupSubjects should not be called when GetDescriptor returned empty")
			return nil, nil
		},
		getDescriptorFunc: func(_ context.Context, _ graph.GetDescriptorArgs) (*graph.GraphDescriptorResult, error) {
			return &graph.GraphDescriptorResult{Value: types.ToPtr("")}, nil
		},
	}
	identityClient := &stubIdentityClient{
		readIdentitiesFunc: func(_ context.Context, _ identity.ReadIdentitiesArgs) (*[]identity.Identity, error) {
			return &[]identity.Identity{identityWithStorageKey("00000000-0000-0000-0000-000000000001")}, nil
		},
	}

	out := make(map[string]*graph.GraphSubject)
	require.NoError(t, resolveSearchBatch(context.Background(), identityClient, graphClient, []string{"alice@x.com"}, out))
	assert.Empty(t, out, "empty GetDescriptor result should not produce an entry")
}

func Test_resolveSearchBatch_PropagatesSearchError(t *testing.T) {
	graphClient := &stubGraphClient{}
	identityClient := &stubIdentityClient{
		readIdentitiesFunc: func(_ context.Context, _ identity.ReadIdentitiesArgs) (*[]identity.Identity, error) {
			return nil, wrappedStatus(http.StatusInternalServerError)
		},
	}

	out := make(map[string]*graph.GraphSubject)
	err := resolveSearchBatch(context.Background(), identityClient, graphClient, []string{"alice@x.com"}, out)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to resolve member")
}

func Test_resolveSearchBatch_AmbiguousSearchResultsError(t *testing.T) {
	graphClient := &stubGraphClient{}
	identityClient := &stubIdentityClient{
		readIdentitiesFunc: func(_ context.Context, _ identity.ReadIdentitiesArgs) (*[]identity.Identity, error) {
			return &[]identity.Identity{
				{SubjectDescriptor: types.ToPtr("aad.A1")},
				{SubjectDescriptor: types.ToPtr("aad.A2")},
			}, nil
		},
	}

	out := make(map[string]*graph.GraphSubject)
	err := resolveSearchBatch(context.Background(), identityClient, graphClient, []string{"alice@x.com"}, out)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "multiple identities found")
}

func Test_resolveIdentity_PrefixedSIDGoesToSIDBranch(t *testing.T) {
	identityClient := &stubIdentityClient{
		readIdentitiesFunc: func(_ context.Context, args identity.ReadIdentitiesArgs) (*[]identity.Identity, error) {
			require.NotNil(t, args.Descriptors)
			assert.Nil(t, args.SubjectDescriptors)
			assert.Equal(t, "Microsoft.TeamFoundation.Identity;s-1-2-34-9999", *args.Descriptors)
			return &[]identity.Identity{{SubjectDescriptor: types.ToPtr("vssgp.U999")}}, nil
		},
	}

	got, err := resolveIdentity(context.Background(), identityClient, "Microsoft.TeamFoundation.Identity;s-1-2-34-9999")
	require.NoError(t, err)
	require.NotNil(t, got)
	assert.Equal(t, "vssgp.U999", types.GetValue(got.SubjectDescriptor, ""))
}

func Test_isNotFoundError(t *testing.T) {
	t.Run("nil error returns false", func(t *testing.T) {
		assert.False(t, isNotFoundError(nil))
	})
	t.Run("non-wrapped error returns false", func(t *testing.T) {
		assert.False(t, isNotFoundError(errors.New("boom")))
	})
	t.Run("404 status returns true", func(t *testing.T) {
		assert.True(t, isNotFoundError(wrappedStatus(http.StatusNotFound)))
	})
	t.Run("other status returns false", func(t *testing.T) {
		assert.False(t, isNotFoundError(wrappedStatus(http.StatusInternalServerError)))
	})
}

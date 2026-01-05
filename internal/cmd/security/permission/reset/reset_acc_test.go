package reset

import (
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/microsoft/azure-devops-go-api/azuredevops/v7/graph"
	"github.com/microsoft/azure-devops-go-api/azuredevops/v7/security"

	"github.com/tmeckel/azdo-cli/internal/cmd/security/permission/shared"
	inttest "github.com/tmeckel/azdo-cli/internal/test"
	"github.com/tmeckel/azdo-cli/internal/types"
	pollutil "github.com/tmeckel/azdo-cli/internal/util"
)

type aceContainer struct {
	Token                string                        `json:"token"`
	Merge                bool                          `json:"merge"`
	AccessControlEntries []security.AccessControlEntry `json:"accessControlEntries"`
}

func TestAccResetPermission(t *testing.T) {
	const (
		namespaceID = "2e9eb7ed-3c0a-47d4-87c1-0ffdd275fd87"
		token       = "repoV2"
	)

	nsUUID := uuid.MustParse(namespaceID)
	groupName := fmt.Sprintf("azdo-cli-reset-%s", uuid.New().String())

	var (
		groupDescriptor string
		groupIdentity   string
	)

	const (
		allowMaskInitial = 0x6 // Read (0x2) + Contribute (0x4)
		denyMaskInitial  = 0x8 // ForcePush
		resetBit         = "Read"
		expectedAllow    = 0x4 // Contribute should remain
		expectedDeny     = 0x8 // ForcePush remains denied
	)

	inttest.Test(t, inttest.TestCase{
		AcceptanceTest: true,
		Steps: []inttest.Step{
			{
				PreRun: func(ctx inttest.TestContext) error {
					graphClient, err := ctx.ClientFactory().Graph(ctx.Context(), ctx.Org())
					if err != nil {
						return fmt.Errorf("failed to create graph client: %w", err)
					}
					extClient, err := ctx.ClientFactory().Extensions(ctx.Context(), ctx.Org())
					if err != nil {
						return fmt.Errorf("failed to create extensions client: %w", err)
					}
					secClient, err := ctx.ClientFactory().Security(ctx.Context(), ctx.Org())
					if err != nil {
						return fmt.Errorf("failed to create security client: %w", err)
					}

					group, err := graphClient.CreateGroupVsts(ctx.Context(), graph.CreateGroupVstsArgs{
						CreationContext: &graph.GraphGroupVstsCreationContext{
							DisplayName: &groupName,
						},
					})
					if err != nil {
						return fmt.Errorf("failed to create test group: %w", err)
					}
					groupDescriptor = types.GetValue(group.Descriptor, "")
					if groupDescriptor == "" {
						return fmt.Errorf("group descriptor is empty")
					}

					identity, err := extClient.ResolveIdentity(ctx.Context(), groupDescriptor)
					if err != nil {
						return fmt.Errorf("failed to resolve identity for %q: %w", groupDescriptor, err)
					}
					groupIdentity = types.GetValue(identity.Descriptor, "")
					if groupIdentity == "" {
						return fmt.Errorf("resolved identity descriptor is empty")
					}

					container := aceContainer{
						Token: token,
						Merge: false,
						AccessControlEntries: []security.AccessControlEntry{
							{
								Descriptor: &groupIdentity,
								Allow:      types.ToPtr(allowMaskInitial),
								Deny:       types.ToPtr(denyMaskInitial),
							},
						},
					}
					_, err = secClient.SetAccessControlEntries(ctx.Context(), security.SetAccessControlEntriesArgs{
						Container:           container,
						SecurityNamespaceId: &nsUUID,
					})
					if err != nil {
						return fmt.Errorf("failed to seed ACE: %w", err)
					}

					return nil
				},
				Run: func(ctx inttest.TestContext) error {
					o := &opts{
						rawTarget:   fmt.Sprintf("%s/%s", ctx.Org(), groupDescriptor),
						namespaceID: namespaceID,
						token:       token,
						permission:  []string{resetBit},
						yes:         true,
					}
					return runCommand(ctx, o)
				},
				Verify: func(ctx inttest.TestContext) error {
					secClient, err := ctx.ClientFactory().Security(ctx.Context(), ctx.Org())
					if err != nil {
						return err
					}

					return pollutil.Poll(ctx.Context(), func() error {
						ace, err := shared.FindAccessControlEntry(ctx.Context(), secClient, nsUUID, token, groupIdentity)
						if err != nil {
							return err
						}
						if ace == nil {
							return fmt.Errorf("expected ACE for descriptor %q to exist", groupIdentity)
						}

						allow := types.GetValue(ace.Allow, 0)
						deny := types.GetValue(ace.Deny, 0)

						if allow != expectedAllow || deny != expectedDeny {
							return fmt.Errorf("unexpected permission state allow=0x%X deny=0x%X", allow, deny)
						}
						return nil
					}, pollutil.PollOptions{
						Tries:   10,
						Timeout: 30 * time.Second,
					})
				},
				PostRun: func(ctx inttest.TestContext) error {
					var errs []error

					if groupIdentity != "" {
						secClient, err := ctx.ClientFactory().Security(ctx.Context(), ctx.Org())
						if err != nil {
							errs = append(errs, fmt.Errorf("failed to create security client: %w", err))
						} else {
							_, err = secClient.RemoveAccessControlEntries(ctx.Context(), security.RemoveAccessControlEntriesArgs{
								SecurityNamespaceId: &nsUUID,
								Token:               types.ToPtr(token),
								Descriptors:         &groupIdentity,
							})
							if err != nil {
								errs = append(errs, fmt.Errorf("failed to remove ACE: %w", err))
							}
						}
					}

					if groupDescriptor != "" {
						graphClient, err := ctx.ClientFactory().Graph(ctx.Context(), ctx.Org())
						if err != nil {
							errs = append(errs, fmt.Errorf("failed to create graph client for cleanup: %w", err))
						} else {
							err = graphClient.DeleteGroup(ctx.Context(), graph.DeleteGroupArgs{
								GroupDescriptor: &groupDescriptor,
							})
							if err != nil {
								errs = append(errs, fmt.Errorf("failed to delete test group: %w", err))
							}
						}
					}

					if len(errs) > 0 {
						return errors.Join(errs...)
					}
					return nil
				},
			},
		},
	})
}

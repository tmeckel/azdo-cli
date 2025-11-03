# Azure DevOps Security Namespaces, ACLs and ACEs<!-- omit from toc -->

This document explains Azure DevOps security namespaces, access control lists (ACLs), and access control entries (ACEs) in plain language and with concrete examples. It's written for readers who have never worked with Azure DevOps security before.

- [Key concepts](#key-concepts)
- [How ACEs represent permissions](#how-aces-represent-permissions)
- [APIs and SDKs](#apis-and-sdks)
- [Create vs Update ACEs — merge semantics](#create-vs-update-aces--merge-semantics)
- [How clients (and CLI) typically work](#how-clients-and-cli-typically-work)
- [Example request body shape (JSON)](#example-request-body-shape-json)
- [Safety and best practices](#safety-and-best-practices)
- [Troubleshooting](#troubleshooting)
- [References](#references)
- [Security namespace structure (detailed)](#security-namespace-structure-detailed)
  - [What are security bits?](#what-are-security-bits)
  - [How ACEs are constructed from namespaces, bits, and a subject](#how-aces-are-constructed-from-namespaces-bits-and-a-subject)
  - [How are ACE tokens generated?](#how-are-ace-tokens-generated)
  - [Additional reading \& references](#additional-reading--references)
- [authoritative references and REST endpoints](#authoritative-references-and-rest-endpoints)
- [Concrete examples](#concrete-examples)
- [Notes on tokens and token formation](#notes-on-tokens-and-token-formation)
- [Small warnings and handy links](#small-warnings-and-handy-links)
- [Quick checklist (direct msdocs links)](#quick-checklist-direct-msdocs-links)
- [Using the `azdo` CLI to work with namespaces and ACEs](#using-the-azdo-cli-to-work-with-namespaces-and-aces)
- [Git repository token pattern (example)](#git-repository-token-pattern-example)
- [Token structures observed per namespace](#token-structures-observed-per-namespace)
  - [Namespace 58450c49-b02d-465a-ab12-59ae512d6531 (Analytics)](#namespace-58450c49-b02d-465a-ab12-59ae512d6531-analytics)
  - [Namespace d34d3680-dfe5-4cc6-a949-7d9c68f73cba (AnalyticsViews)](#namespace-d34d3680-dfe5-4cc6-a949-7d9c68f73cba-analyticsviews)
  - [Namespace 62a7ad6b-8b8d-426b-ba10-76a7090e94d5 (PipelineCachePrivileges)](#namespace-62a7ad6b-8b8d-426b-ba10-76a7090e94d5-pipelinecacheprivileges)
  - [Namespace 7c7d32f7-0e86-4cd6-892e-b35dbba870bd (ReleaseManagement)](#namespace-7c7d32f7-0e86-4cd6-892e-b35dbba870bd-releasemanagement)
  - [Namespace c788c23e-1b46-4162-8f5e-d7585343b5de (ReleaseManagement)](#namespace-c788c23e-1b46-4162-8f5e-d7585343b5de-releasemanagement)
  - [Namespace a6cc6381-a1ca-4b36-b3c1-4e65211e82b6 (AuditLog)](#namespace-a6cc6381-a1ca-4b36-b3c1-4e65211e82b6-auditlog)
  - [Namespace 445d2788-c5fb-4132-bbef-09c4045ad93f (WorkItemTrackingAdministration)](#namespace-445d2788-c5fb-4132-bbef-09c4045ad93f-workitemtrackingadministration)
  - [Namespace 2e9eb7ed-3c0a-47d4-87c1-0ffdd275fd87 (Git Repositories)](#namespace-2e9eb7ed-3c0a-47d4-87c1-0ffdd275fd87-git-repositories)
  - [Namespace 3c15a8b7-af1a-45c2-aa97-2cb97078332e (VersionControlItems2)](#namespace-3c15a8b7-af1a-45c2-aa97-2cb97078332e-versioncontrolitems2)
  - [Namespace 2bf24a2b-70ba-43d3-ad97-3d9e1f75622f (EventSubscriber)](#namespace-2bf24a2b-70ba-43d3-ad97-3d9e1f75622f-eventsubscriber)
  - [Namespace 5a6cd233-6615-414d-9393-48dbb252bd23 (WorkItemTrackingProvision)](#namespace-5a6cd233-6615-414d-9393-48dbb252bd23-workitemtrackingprovision)
  - [Namespace 49b48001-ca20-4adc-8111-5b60c903a50c (ServiceEndpoints)](#namespace-49b48001-ca20-4adc-8111-5b60c903a50c-serviceendpoints)
  - [Namespace cb594ebe-87dd-4fc9-ac2c-6a10a4c92046 (ServiceHooks)](#namespace-cb594ebe-87dd-4fc9-ac2c-6a10a4c92046-servicehooks)
  - [Namespace 3e65f728-f8bc-4ecd-8764-7e378b19bfa7 (Collection)](#namespace-3e65f728-f8bc-4ecd-8764-7e378b19bfa7-collection)
  - [Namespace cb4d56d2-e84b-457e-8845-81320a133fbb (Proxy)](#namespace-cb4d56d2-e84b-457e-8845-81320a133fbb-proxy)
  - [Namespace bed337f8-e5f3-4fb9-80da-81e17d06e7a8 (Plan)](#namespace-bed337f8-e5f3-4fb9-80da-81e17d06e7a8-plan)
  - [Namespace 2dab47f9-bd70-49ed-9bd5-8eb051e59c02 (Process)](#namespace-2dab47f9-bd70-49ed-9bd5-8eb051e59c02-process)
  - [Namespace 11238e09-49f2-40c7-94d0-8f0307204ce4 (AccountAdminSecurity)](#namespace-11238e09-49f2-40c7-94d0-8f0307204ce4-accountadminsecurity)
  - [Namespace b7e84409-6553-448a-bbb2-af228e07cbeb (Library)](#namespace-b7e84409-6553-448a-bbb2-af228e07cbeb-library)
  - [Namespace 83d4c2e6-e57d-4d6e-892b-b87222b7ad20 (Environment)](#namespace-83d4c2e6-e57d-4d6e-892b-b87222b7ad20-environment)
  - [Namespace 58b176e7-3411-457a-89d0-c6d0ccb3c52b (EventSubscription)](#namespace-58b176e7-3411-457a-89d0-c6d0ccb3c52b-eventsubscription)
  - [Namespace 83e28ad4-2d72-4ceb-97b0-c7726d5502c3 (CSS)](#namespace-83e28ad4-2d72-4ceb-97b0-c7726d5502c3-css)
  - [Namespace 9e4894c3-ff9a-4eac-8a85-ce11cafdc6f1 (TeamLabSecurity)](#namespace-9e4894c3-ff9a-4eac-8a85-ce11cafdc6f1-teamlabsecurity)
  - [Namespace fc5b7b85-5d6b-41eb-8534-e128cb10eb67 (ProjectAnalysisLanguageMetrics)](#namespace-fc5b7b85-5d6b-41eb-8534-e128cb10eb67-projectanalysislanguagemetrics)
  - [Namespace bb50f182-8e5e-40b8-bc21-e8752a1e7ae2 (Tagging)](#namespace-bb50f182-8e5e-40b8-bc21-e8752a1e7ae2-tagging)
  - [Namespace f6a4de49-dbe2-4704-86dc-f8ec1a294436 (MetaTask)](#namespace-f6a4de49-dbe2-4704-86dc-f8ec1a294436-metatask)
  - [Namespace bf7bfa03-b2b7-47db-8113-fa2e002cc5b1 (Iteration)](#namespace-bf7bfa03-b2b7-47db-8113-fa2e002cc5b1-iteration)
  - [Namespace 71356614-aad7-4757-8f2c-0fb3bff6f680 (WorkItemQueryFolders)](#namespace-71356614-aad7-4757-8f2c-0fb3bff6f680-workitemqueryfolders)
  - [Namespace fa557b48-b5bf-458a-bb2b-1b680426fe8b (Favorites)](#namespace-fa557b48-b5bf-458a-bb2b-1b680426fe8b-favorites)
  - [Namespace 4ae0db5d-8437-4ee8-a18b-1f6fb38bd34c (Registry)](#namespace-4ae0db5d-8437-4ee8-a18b-1f6fb38bd34c-registry)
  - [Namespace c2ee56c9-e8fa-4cdd-9d48-2c44f697a58e (Graph)](#namespace-c2ee56c9-e8fa-4cdd-9d48-2c44f697a58e-graph)
  - [Namespace dc02bf3d-cd48-46c3-8a41-345094ecc94b (ViewActivityPaneSecurity)](#namespace-dc02bf3d-cd48-46c3-8a41-345094ecc94b-viewactivitypanesecurity)
  - [Namespace 2a887f97-db68-4b7c-9ae3-5cebd7add999 (Job)](#namespace-2a887f97-db68-4b7c-9ae3-5cebd7add999-job)
  - [Namespace 7cd317f2-adc6-4b6c-8d99-6074faeaf173 (EventPublish)](#namespace-7cd317f2-adc6-4b6c-8d99-6074faeaf173-eventpublish)
  - [Namespace 73e71c45-d483-40d5-bdba-62fd076f7f87 (WorkItemTracking)](#namespace-73e71c45-d483-40d5-bdba-62fd076f7f87-workitemtracking)
  - [Namespace 4a9e8381-289a-4dfd-8460-69028eaa93b3 (StrongBox)](#namespace-4a9e8381-289a-4dfd-8460-69028eaa93b3-strongbox)
  - [Namespace 1f4179b3-6bac-4d01-b421-71ea09171400 (Server)](#namespace-1f4179b3-6bac-4d01-b421-71ea09171400-server)
  - [Namespace e06e1c24-e93d-4e4a-908a-7d951187b483 (TestManagement)](#namespace-e06e1c24-e93d-4e4a-908a-7d951187b483-testmanagement)
  - [Namespace 6ec4592e-048c-434e-8e6c-8671753a8418 (SettingEntries)](#namespace-6ec4592e-048c-434e-8e6c-8671753a8418-settingentries)
  - [Namespace 302acaca-b667-436d-a946-87133492041c (BuildAdministration)](#namespace-302acaca-b667-436d-a946-87133492041c-buildadministration)
  - [Namespace 2725d2bc-7520-4af4-b0e3-8d876494731f (Location)](#namespace-2725d2bc-7520-4af4-b0e3-8d876494731f-location)
  - [Namespace 251e12d9-bea3-43a8-bfdb-901b98c0125e (Boards)](#namespace-251e12d9-bea3-43a8-bfdb-901b98c0125e-boards)
  - [Namespace f0003bce-5f45-4f93-a25d-90fc33fe3aa9 (OrganizationLevelData)](#namespace-f0003bce-5f45-4f93-a25d-90fc33fe3aa9-organizationleveldata)
  - [Namespace 83abde3a-4593-424e-b45f-9898af99034d (UtilizationPermissions)](#namespace-83abde3a-4593-424e-b45f-9898af99034d-utilizationpermissions)
  - [Namespace c0e7a722-1cad-4ae6-b340-a8467501e7ce (WorkItemsHub)](#namespace-c0e7a722-1cad-4ae6-b340-a8467501e7ce-workitemshub)
  - [Namespace 0582eb05-c896-449a-b933-aa3d99e121d6 (WebPlatform)](#namespace-0582eb05-c896-449a-b933-aa3d99e121d6-webplatform)
  - [Namespace 66312704-deb5-43f9-b51c-ab4ff5e351c3 (VersionControlPrivileges)](#namespace-66312704-deb5-43f9-b51c-ab4ff5e351c3-versioncontrolprivileges)
  - [Namespace 93bafc04-9075-403a-9367-b7164eac6b5c (Workspaces)](#namespace-93bafc04-9075-403a-9367-b7164eac6b5c-workspaces)
  - [Namespace 093cbb02-722b-4ad6-9f88-bc452043fa63 (CrossProjectWidgetView)](#namespace-093cbb02-722b-4ad6-9f88-bc452043fa63-crossprojectwidgetview)
  - [Namespace 35e35e8e-686d-4b01-aff6-c369d6e36ce0 (WorkItemTrackingConfiguration)](#namespace-35e35e8e-686d-4b01-aff6-c369d6e36ce0-workitemtrackingconfiguration)
  - [Namespace 0d140cae-8ac1-4f48-b6d1-c93ce0301a12 (Discussion Threads)](#namespace-0d140cae-8ac1-4f48-b6d1-c93ce0301a12-discussion-threads)
  - [Namespace 5ab15bc8-4ea1-d0f3-8344-cab8fe976877 (BoardsExternalIntegration)](#namespace-5ab15bc8-4ea1-d0f3-8344-cab8fe976877-boardsexternalintegration)
  - [Namespace 7ffa7cf4-317c-4fea-8f1d-cfda50cfa956 (DataProvider)](#namespace-7ffa7cf4-317c-4fea-8f1d-cfda50cfa956-dataprovider)
  - [Namespace 81c27cc8-7a9f-48ee-b63f-df1e1d0412dd (Social)](#namespace-81c27cc8-7a9f-48ee-b63f-df1e1d0412dd-social)
  - [Namespace 9a82c708-bfbe-4f31-984c-e860c2196781 (Security)](#namespace-9a82c708-bfbe-4f31-984c-e860c2196781-security)
  - [Namespace a60e0d84-c2f8-48e4-9c0c-f32da48d5fd1 (IdentityPicker)](#namespace-a60e0d84-c2f8-48e4-9c0c-f32da48d5fd1-identitypicker)
  - [Namespace 84cc1aa4-15bc-423d-90d9-f97c450fc729 (ServicingOrchestration)](#namespace-84cc1aa4-15bc-423d-90d9-f97c450fc729-servicingorchestration)
  - [Namespace 33344d9c-fc72-4d6f-aba5-fa317101a7e9 (Build)](#namespace-33344d9c-fc72-4d6f-aba5-fa317101a7e9-build)
  - [Namespace 8adf73b7-389a-4276-b638-fe1653f7efc7 (DashboardsPrivileges)](#namespace-8adf73b7-389a-4276-b638-fe1653f7efc7-dashboardsprivileges)
  - [Namespace 52d39943-cb85-4d7f-8fa8-c6baac873819 (Project)](#namespace-52d39943-cb85-4d7f-8fa8-c6baac873819-project)
  - [Namespace a39371cf-0841-4c16-bbd3-276e341bc052 (VersionControlItems)](#namespace-a39371cf-0841-4c16-bbd3-276e341bc052-versioncontrolitems)
- [Authoritative reference and namespace examples](#authoritative-reference-and-namespace-examples)
  - [Small canonical mapping](#small-canonical-mapping)

## Key concepts

- Security namespace: a logical grouping of related permissions for a resource type (for example, Git repositories, work items, pipelines). Each namespace defines a set of actions that can be allowed or denied.

- Action: a single permission that can be granted or denied (for example, `Read`, `Contribute`, `Edit`). Each action is represented by a bitmask integer (e.g., `0x4`) and also has textual metadata (`Name` and `DisplayName`). Names live in the namespace's action definitions.

- Token: a string that identifies a particular securable resource within a namespace (for example a project ID, a repository path, or `/` for the root). An ACL is always associated with a single namespace and a single token.

- ACL (Access Control List): a container for ACEs that applies to a specific `token` within a namespace. An ACL includes properties such as `InheritPermissions` and a dictionary of ACEs keyed by descriptor.

- ACE (Access Control Entry): an entry in an ACL for a single identity (user or group). An ACE contains:
  - `Descriptor` — the identity descriptor (a unique string the server uses to identify a user or group)
  - `Allow` — an integer bitmask of allowed actions
  - `Deny` — an integer bitmask of denied actions

## How ACEs represent permissions

An ACE uses two integer bitmasks: `Allow` and `Deny`.
- Each action defined by the namespace corresponds to a single bit (for example `Bit 4` or `0x4`).
- To allow multiple actions, the bits are OR'd together. Example: `Read (0x1)` + `Contribute (0x4)` → `0x1 | 0x4 = 0x5`.
- Deny bits work similarly; the server evaluates both allow and deny masks when computing effective permissions (deny typically takes precedence for the specific actions denied).

Because bitmasks are compact but hard to read, the Azure DevOps APIs return action definitions so clients can translate between integer bitmasks and human-friendly names.

## APIs and SDKs

Azure DevOps provides REST APIs and SDKs to inspect namespaces/actions and to manage ACLs/ACEs. Two important operations are:

- QuerySecurityNamespaces (GET): returns namespace metadata including an `Actions` list. Each action has `Bit`, `Name`, and `DisplayName` fields. Use this to map textual names to the integer bits.

- SetAccessControlEntries (POST): add or update ACEs in an ACL for a given `token`. The request body includes the `token` and one or more ACEs. The call supports an optional `merge` parameter that controls collision behavior.

## Create vs Update ACEs — merge semantics

- Creating a new ACE: call `SetAccessControlEntries` with an ACE whose `Descriptor` is the user/group descriptor and the desired `Allow`/`Deny` bitmasks. If no ACE for that descriptor exists on the ACL, the server will create it.

- Updating an existing ACE: use `SetAccessControlEntries` as well. If an ACE already exists for the descriptor, the server's behavior depends on the `merge` parameter:
  - `merge=true`: the server merges incoming `Allow` and `Deny` masks with the existing ones (effectively a bitwise OR).
  - `merge=false` (or omitted): the incoming ACE replaces the existing ACE for that descriptor (displaces the old `Allow` and `Deny`).

The SDK's documentation explains this: SetAccessControlEntries "Add or update ACEs in the ACL for the provided token... In the case of a collision (by identity descriptor) with an existing ACE in the ACL, the 'merge' parameter determines the behavior. If set, the existing ACE has its allow and deny merged with the incoming ACE's allow and deny. If unset, the existing ACE is displaced." (azure-devops-go-api vendor docs)

## How clients (and CLI) typically work

A typical sequence to add or update ACEs for a subject via a CLI or SDK:

1. Identify the security namespace UUID for the resource type you want to operate on. Use existing documentation or listing commands (or the REST API) to find the namespace.

2. (Optional, recommended) Query the namespace actions with `QuerySecurityNamespaces --namespace-id <uuid>` so you can accept or display textual action names. The response includes action definitions `{ Bit, Name, DisplayName }`.

3. Resolve the subject to the ACL descriptor expected by the security API:
   - For a user or group string input (e.g. email or project-scoped group id), call `Extensions.ResolveMemberDescriptor` to canonicalize the input to a graph descriptor.
   - Call `Identity.ReadIdentities` with that graph descriptor to retrieve the identity's `Descriptor` field — this is the value used in ACEs.

4. Convert user-supplied permission tokens to an integer bitmask:
   - Acceptable input formats: hexadecimal (prefixed with `0x`), decimal integers, or textual action names that match the namespace's `ActionDefinition.Name` or `DisplayName`.
   - Map textual names to their corresponding `Bit` values and OR them together to produce a single `Allow` or `Deny` integer.

5. Build the ACE object: `{ descriptor: "<descriptor>", allow: <int>, deny: <int> }` and include it in the `SetAccessControlEntries` request body along with the `token`.

6. Call `SetAccessControlEntries` with `merge=true` if you want to merge with existing ACEs; omit or set false if you want to replace.

## Example request body shape (JSON)

```json
{
  "token": "/projects/00000000-0000-0000-0000-000000000000",
  "aces": [
    {
      "descriptor": "Microsoft.IdentityModel.Claims.ClaimsIdentity;...",
      "allow": 5,
      "deny": 0
    }
  ],
  "merge": true
}
```

Notes: the exact shape accepted by the server may vary across API versions; the Go SDK implements marshalling for you if you use its `SetAccessControlEntries` method.

## Safety and best practices

- Always prefer querying action definitions and using textual names in CLI/UI — this reduces the chance of granting the wrong numerical bitmask.
- When adding denies, be careful: denies can block permissions even if a group grants them via inheritance. Prefer minimal denial and prefer group-based permissions where possible.
- Use `merge=true` when you want to add specific allow/deny bits without wiping any other permissions for the identity.
- Ensure you have the required admin permissions to change ACLs (for example Manage permissions or Edit project-level info depending on the scope).
- Never hardcode PATs or secrets in scripts; use secure authentication (MS Entra/Azure AD) when possible.

## Troubleshooting

- If the server reports an error resolving a descriptor, double-check the identity input and use the Identity/Extensions APIs to verify the descriptor you will use.
- If action names don't match, call `QuerySecurityNamespaces` to fetch names/display names and copy them exactly (case-insensitive matching is recommended but display text can vary across versions/locales).
- If ACL changes don't take effect as expected, inspect effective allow/deny values using `QueryAccessControlLists` with `includeExtendedInfo=true` and examine `ExtendedInfo` values such as `EffectiveAllow` and `EffectiveDeny`.

## References

- Azure DevOps Learn: Security overview and namespaces — https://learn.microsoft.com/azure/devops/organizations/security
- Azure DevOps REST: security namespace and ACL APIs (see SDK or REST API docs)
- Vendored SDK: `vendor/github.com/microsoft/azure-devops-go-api/azuredevops/v7/security/client.go`


## Security namespace structure (detailed)

A security namespace is a data structure the server uses to describe a family of permissions for a resource type. Important fields typically include:

- Id (UUID): unique identifier for the namespace.
- Name / DisplayName: human-readable namespace name (e.g., "Git Repositories").
- Actions: an array of action definitions where each action has:
  - Bit: integer value representing the action as a single bit in a permission mask (e.g., 1, 2, 4, 8). These bits are powers of two so they can be combined with bitwise OR.
  - Name: canonical name used by APIs/clients (e.g., `Read`, `Contribute`).
  - DisplayName: user-friendly label shown in UI.
- Scopes / Hierarchy: some namespaces are hierarchical (for example, project-scoped tokens that have child tokens). For hierarchical namespaces, ACLs can be queried recursively.

Example (pseudo-JSON) of a namespace action definition:

```json
{
  "Actions": [
    { "Bit": 1, "Name": "Read", "DisplayName": "Read" },
    { "Bit": 2, "Name": "Write", "DisplayName": "Write" },
    { "Bit": 4, "Name": "Contribute", "DisplayName": "Contribute" },
    { "Bit": 8, "Name": "ManagePermissions", "DisplayName": "Manage permissions" }
  ]
}
```

### What are security bits?

Security bits are integer values where a single bit represents one action. They are typically powers of two so that multiple actions can be represented in a single integer using bitwise OR. For example:

- `Read` → bit 1 (0x1)
- `Write` → bit 2 (0x2)
- `Contribute` → bit 4 (0x4)

To represent `Read` + `Contribute`, compute `0x1 | 0x4 = 0x5`.

### How ACEs are constructed from namespaces, bits, and a subject

An ACE ties an identity (subject) to a set of allowed and denied bits for a specific token in a namespace. Construction steps:

1. Identify the namespace and token that represents the resource to protect.
2. Resolve the subject into an ACL descriptor (see below).
3. Choose which actions to allow/deny. Convert textual action names to their `Bit` values via the namespace Actions array.
4. Combine action bits using bitwise OR to produce `Allow` and `Deny` integers.
5. Create the ACE with `{ descriptor: "<descriptor>", allow: <int>, deny: <int> }` and submit with `SetAccessControlEntries`.

Example (JSON) ACE for a user allowing Read and Contribute:

```json
{
  "descriptor": "vssgp.XXXXXXXX-XXXX-XXXX-XXXX-XXXXXXXXXXXX",
  "allow": 5, // 0x1 | 0x4
  "deny": 0
}
```

### How are ACE tokens generated?

The `token` used by an ACL identifies which resource instance the ACL applies to. Token formats are namespace-specific. For example:

- For project-level permissions the token might be `/projects/{projectId}` or simply `/` for the root of a namespace.
- For repository-level permissions a token might reference a repository ID or path.

Token generation rules are defined by the namespace; the server accepts the token string and associates an ACL with it. To determine the correct token pattern for a namespace, consult the namespace documentation or call the API that manages that resource (for example, the Repos API returns repository IDs you can embed in tokens). The `azdo` CLI's `security permission list` and `show` commands can display tokens returned by the API so you can inspect existing ACLs and learn token formats.

Practical tip: when in doubt, list ACLs for a namespace (without a token filter) to see commonly used token values in your organization.

### Additional reading & references

- QuerySecurityNamespaces (REST API) — discover names and bits for actions
- SetAccessControlEntries (REST API) — create/update ACEs for a token
- QueryAccessControlLists (REST API) — inspect ACLs and ACEs with extended info

## Authoritative references and REST endpoints

This appendix contains direct references to Microsoft Learn pages and the REST API patterns you can use to operate on namespaces, ACLs, and ACEs.

- REST API pattern: Use the Azure DevOps REST API base pattern (include api-version):
  - GET https://dev.azure.com/{organization}/_apis/security/namespaces/{namespaceId}?api-version=7.1-preview.1
  - GET https://dev.azure.com/{organization}/_apis/security/ACLs?securityNamespaceId={namespaceId}&token={token}&api-version=7.1-preview.1
  - POST https://dev.azure.com/{organization}/_apis/security/ACLs/Entries?securityNamespaceId={namespaceId}&api-version=7.1-preview.1

  Note: the official REST docs are on Microsoft Learn; the exact endpoint path and query parameter names vary by API version — prefer using an SDK client (e.g., the vendored Go client) which marshals requests correctly.

- Microsoft Learn pages consulted:
  - Security overview & namespaces: https://learn.microsoft.com/en-us/azure/devops/organizations/security/about-security-identity
  - Security namespace & permission reference: https://learn.microsoft.com/en-us/azure/devops/organizations/security/namespace-reference
  - REST API guidance and samples: https://learn.microsoft.com/en-us/azure/devops/integrate/get-started/rest/samples
  - TFSSecurity command reference (historical): https://learn.microsoft.com/en-us/azure/devops/server/command-line/tfssecurity-cmd
  - General ACE/ACL concepts (Windows security): https://learn.microsoft.com/en-us/windows/win32/secauthz/access-control-entries

## Concrete examples

1) Discover actions for a namespace (pseudo-HTTP):

GET https://dev.azure.com/myorg/_apis/security/namespaces/{namespaceId}?api-version=7.1-preview.1

Response snippet contains:

```json
{
  "id": "{namespaceId}",
  "displayName": "Git Repositories",
  "actions": [
    { "bit": 1, "name": "Read", "displayName": "Read" },
    { "bit": 2, "name": "Write", "displayName": "Write" },
    { "bit": 4, "name": "Contribute", "displayName": "Contribute" }
  ]
}
```

2) Inspect ACLs for a token (pseudo-HTTP):

GET https://dev.azure.com/myorg/_apis/security/ACLs?securityNamespaceId={namespaceId}&token={token}&includeExtendedInfo=true&api-version=7.1-preview.1

Response contains ACL(s) with `acesDictionary` keyed by descriptor; each ACE includes `allow`, `deny`, and `extendedInfo` (effective/inherited values).

3) Create or update ACEs (pseudo-HTTP body for SetAccessControlEntries):

POST https://dev.azure.com/myorg/_apis/security/ACLs/Entries?securityNamespaceId={namespaceId}&api-version=7.1-preview.1

JSON body:

```json
{
  "token": "{token}",
  "aces": [
    { "descriptor": "{descriptor}", "allow": 5, "deny": 0 }
  ],
  "merge": true
}
```

If the specified descriptor does not exist in the ACL, the server will create that ACE. If it exists and `merge` is true, allow/deny are merged; otherwise the ACE is replaced.

## Notes on tokens and token formation

- There is no single global token format; tokens are defined by each namespace. Common tokens:
  - Root token `/` meaning namespace root
  - Project token such as `/projects/{projectId}`
  - Resource-specific tokens that include resource IDs (e.g., repository IDs)
- To learn tokens used in your org, list ACLs for a namespace and inspect `token` values returned by `QueryAccessControlLists`.

## Small warnings and handy links

- Warning about API versions: the exact REST endpoint paths, query parameter names, and request body shapes for security operations can vary across API versions (e.g., preview vs stable). When automating or scripting, prefer using the official client libraries (SDKs) which handle marshalling and API-versioning for you. If you call the REST API directly, explicitly include `api-version` and verify the expected request format for that version.

- Identity resolution (how to get the ACE descriptor):
  - Resolve the graph descriptor for a subject: `Extensions.ResolveMemberDescriptor` (or use the CLI helper that does this). See the Extensions API docs in the Azure DevOps REST reference.
  - Convert the graph descriptor to the ACL-style identity descriptor: `Identity.ReadIdentities` — use the returned identity's `Descriptor` field in ACEs.

## Quick checklist (direct msdocs links)

- Security overview & namespaces: https://learn.microsoft.com/en-us/azure/devops/organizations/security/about-security-identity
- Security namespace & permission reference: https://learn.microsoft.com/en-us/azure/devops/organizations/security/namespace-reference
- Query security namespaces (to discover Actions): see the Azure DevOps REST API reference and SDK `QuerySecurityNamespaces` method (browse: https://learn.microsoft.com/rest/api/azure/devops/)
- Query access control lists: check `QueryAccessControlLists` in the REST API reference / SDK
- Set access control entries: check `SetAccessControlEntries` in the REST API reference / SDK
- Extensions.ResolveMemberDescriptor: check the Extensions API in the REST reference
- Identity.ReadIdentities: check the Identity API in the REST reference

(Use the Azure DevOps REST API index at https://learn.microsoft.com/rest/api/azure/devops/ to find the exact pages for the API version you need.)

## Using the `azdo` CLI to work with namespaces and ACEs

This repository provides `azdo` commands to inspect namespaces, list and show ACLs/ACEs, and update permissions. Below are common workflows using the CLI you now have in this repository.

1. Discover available namespaces

   - List namespaces (shows namespace id and display name):

     `azdo security permission namespace list`

   - Show a single namespace (includes action definitions you can use by name):

     `azdo security permission namespace show --namespace-id <namespace-uuid>`

     Example output includes `actions` with `bit`, `name`, and `displayName` fields. Use those `name`/`displayName` values when supplying textual permission names to other commands.

2. Inspect ACLs and ACEs

   - List ACEs for a namespace (optionally filter by token or subject):

     `azdo security permission list [ORGANIZATION[/PROJECT/]SUBJECT] --namespace-id <namespace-uuid> [--token <token>] [--recurse]`

     Examples:

     - List all ACEs for a namespace using your default organization:
       
       `azdo security permission list --namespace-id 5a27515b-ccd7-42c9-84f1-54c998f03866`

     - List ACEs for a specific subject (email or group descriptor):
       
       `azdo security permission list fabrikam/contoso@example.com --namespace-id 5a27515b-...`

   - Show permissions for a single subject on a token:

     `azdo security permission show ORGANIZATION/SUBJECT --namespace-id <namespace-uuid> --token <token>`

     This returns explicit and effective allow/deny action lists for the subject.

3. Add or update an ACE (create if absent)

   - Use `azdo security permission update` (implemented in this repo) to add or update ACEs for a subject. `--allow-bit` and `--deny-bit` accept numeric or textual action names (from the namespace actions).

     Examples:
     - Allow a textual action for a subject:

       `azdo security permission update fabrikam/contoso@example.com --namespace-id <namespace-uuid> --token /projects/123 --allow-bit Read`

     - Allow multiple actions (names or numeric values):

       `azdo security permission update fabrikam/contoso@example.com --namespace-id <namespace-uuid> --token /projects/123 --allow-bit Read --allow-bit Contribute --allow-bit 0x4`

     - Deny a numeric bit and merge with existing ACEs:

       `azdo security permission update fabrikam/contoso@example.com --namespace-id <namespace-uuid> --token /projects/123 --deny-bit 8 --merge`

Notes:

- `azdo security permission update` will resolve the subject you provide (via Extensions + Identity APIs) into the ACL descriptor used in ACEs, and then call the security API to create or update the ACE. If the ACE did not exist it will be created.
- To avoid surprising replacements of existing ACEs, use `--merge` when you want to add bits without replacing existing allow/deny masks.

1) Helpful quick workflow

- Find a namespace ID for the resource you care about: `azdo security permission namespace list`.
- Inspect the actions for that namespace: `azdo security permission namespace show --namespace-id <id>`.
- Inspect existing ACLs/tokens to learn token formats: `azdo security permission list --namespace-id <id>`.
- Add or update an ACE: `azdo security permission update ORG/subject --namespace-id <id> --token <token> --allow-bit Read --merge`.

If you want, I can add these examples to the CLI `--help` output or the generated docs (via `make docs`) so they appear in the command reference pages.


## Git repository token pattern (example)

For the `repoV2` namespace tokens in this repository the pattern is:

- `repoV2` — permissions for ALL Git repositories across ALL projects.
- `repoV2/{projectId}` — permissions for all Git repositories in a specific Azure DevOps project.
- `repoV2/{projectId}/{repoId}` — permissions for a single repository in that project.

This hierarchy implies inheritance: project-level ACLs apply to repositories in that project unless a repo-level ACL overrides or refines them.


Example tree from your tokens (condensed):

```text
repoV2
├─ repoV2/f6ad111f-42cb-4e2d-b22a-cd0bd6f5aebd
│  ├─ repoV2/f6ad111f-42cb-4e2d-b22a-cd0bd6f5aebd/f4bc1443-0fd3-42fb-8ae0-2b6740994c52
│  └─ (other repoIds under project f6ad111f...)
├─ repoV2/040a2515-b6d4-4b53-8749-fa8b649ce73a
│  └─ repoV2/040a2515-.../75ff1ff1-...
├─ repoV2/12ee2112-bbe9-4b70-9fdb-6592ad84098a
│  └─ repoV2/12ee2112-.../5983a302-...
└─ (additional projects and their repo children...)
```

Use `azdo security permission list --namespace-id <repoV2-uuid> --recurse` to enumerate tokens and build this tree programmatically.


Tip: Determine token formats by listing ACLs

The canonical way to discover token formats for a namespace in your organization is to list the ACLs (which show token values) for that namespace. Using this repository's CLI you can run:

  azdo security permission list --namespace-id <namespace-uuid>

This calls the same logic implemented in `internal/cmd/security/permission/list/list.go` and will return ACL entries with `token` values you can inspect to learn the namespace's token patterns (including hierarchical tokens like `repoV2/{projectId}/{repoId}`).



## Token structures observed per namespace

### Namespace 58450c49-b02d-465a-ab12-59ae512d6531 (Analytics)
- Observed token prefixes: $
- Token depth counts:
  - depth 2: 5 tokens

  Typical interpretations:
  - Tokens prefixed with `$` often indicate collection/project-scoped items (historical TFVC or generic tokens). Example: `$/<projectId>`.

  Sample tokens:
  - $/040a2515-b6d4-4b53-8749-fa8b649ce73a
  - $/696416ee-f7ff-4ee3-934a-979b00dce74f
  - $/70e9c41e-68af-4300-baa3-4eee0f48b17e
  - $/964e3180-c2c0-4e82-80da-b3491fd6ed81
  - $/f6ad111f-42cb-4e2d-b22a-cd0bd6f5aebd


### Namespace d34d3680-dfe5-4cc6-a949-7d9c68f73cba (AnalyticsViews)
- Observed token prefixes: $
- Token depth counts:
  - depth 2: 1 tokens

  Typical interpretations:
  - Tokens prefixed with `$` often indicate collection/project-scoped items (historical TFVC or generic tokens). Example: `$/<projectId>`.

  Sample tokens:
  - $/Shared


### Namespace 62a7ad6b-8b8d-426b-ba10-76a7090e94d5 (PipelineCachePrivileges)
- Observed token prefixes: No
- Token depth counts:
  - depth 1: 1 tokens

  Typical interpretations:

  Sample tokens:
  - No


### Namespace 7c7d32f7-0e86-4cd6-892e-b35dbba870bd (ReleaseManagement)
- Observed token prefixes: No
- Token depth counts:
  - depth 1: 1 tokens

  Typical interpretations:

  Sample tokens:
  - No


### Namespace c788c23e-1b46-4162-8f5e-d7585343b5de (ReleaseManagement)
- Observed token prefixes: 696416ee-f7ff-4ee3-934a-979b00dce74f, 70e9c41e-68af-4300-baa3-4eee0f48b17e, 964e3180-c2c0-4e82-80da-b3491fd6ed81
- Token depth counts:
  - depth 1: 3 tokens

  Typical interpretations:

  Sample tokens:
  - 696416ee-f7ff-4ee3-934a-979b00dce74f
  - 70e9c41e-68af-4300-baa3-4eee0f48b17e
  - 964e3180-c2c0-4e82-80da-b3491fd6ed81


### Namespace a6cc6381-a1ca-4b36-b3c1-4e65211e82b6 (AuditLog)
- Observed token prefixes: No
- Token depth counts:
  - depth 1: 1 tokens

  Typical interpretations:

  Sample tokens:
  - No


### Namespace 445d2788-c5fb-4132-bbef-09c4045ad93f (WorkItemTrackingAdministration)
- Observed token prefixes: WorkItemTrackingPrivileges
- Token depth counts:
  - depth 1: 1 tokens

  Typical interpretations:

  Sample tokens:
  - WorkItemTrackingPrivileges


### Namespace 2e9eb7ed-3c0a-47d4-87c1-0ffdd275fd87 (Git Repositories)
- Observed token prefixes: repoV2
- Token depth counts:
  - depth 1: 1 tokens
  - depth 2: 15 tokens
  - depth 3: 34 tokens

  Typical interpretations:
  - `repoV2` tokens are for Git Repositories. Common pattern:
    - `repoV2` → namespace root (all repos across org)
    - `repoV2/{projectId}` → all repos in project {projectId}
    - `repoV2/{projectId}/{repoId}` → repo {repoId} in project {projectId}

  Sample tokens:
  - repoV2
  - repoV2/040a2515-b6d4-4b53-8749-fa8b649ce73a
  - repoV2/040a2515-b6d4-4b53-8749-fa8b649ce73a/75ff1ff1-bf20-4399-8ad8-739c2b80ddfe
  - repoV2/12ee2112-bbe9-4b70-9fdb-6592ad84098a
  - repoV2/12ee2112-bbe9-4b70-9fdb-6592ad84098a/5983a302-019e-4e28-9f63-204c5bc6a8b7
  - repoV2/1b39554e-193a-47b2-bae7-bba1e5697ace
  - repoV2/1b39554e-193a-47b2-bae7-bba1e5697ace/c8159776-866a-41b1-a065-06ab9f24b6bd
  - repoV2/58d7cbb5-9855-45e7-93d3-f08409cb272e


### Namespace 3c15a8b7-af1a-45c2-aa97-2cb97078332e (VersionControlItems2)
- Observed token prefixes: $
- Token depth counts:
  - depth 1: 1 tokens
  - depth 2: 15 tokens

  Typical interpretations:
  - Tokens prefixed with `$` often indicate collection/project-scoped items (historical TFVC or generic tokens). Example: `$/<projectId>`.

  Sample tokens:
  - $
  - $/040a2515-b6d4-4b53-8749-fa8b649ce73a
  - $/12ee2112-bbe9-4b70-9fdb-6592ad84098a
  - $/1b39554e-193a-47b2-bae7-bba1e5697ace
  - $/58d7cbb5-9855-45e7-93d3-f08409cb272e
  - $/5e02360f-bc08-49fa-b5f3-bab8a9d7a408
  - $/696416ee-f7ff-4ee3-934a-979b00dce74f
  - $/70e9c41e-68af-4300-baa3-4eee0f48b17e


### Namespace 2bf24a2b-70ba-43d3-ad97-3d9e1f75622f (EventSubscriber)
- Observed token prefixes: $SUBSCRIBER, $SUBSCRIBER:c4a53307-36c3-4ee9-8cac-fbde83eafaed
- Token depth counts:
  - depth 1: 2 tokens

  Typical interpretations:

  Sample tokens:
  - $SUBSCRIBER
  - $SUBSCRIBER:c4a53307-36c3-4ee9-8cac-fbde83eafaed


### Namespace 5a6cd233-6615-414d-9393-48dbb252bd23 (WorkItemTrackingProvision)
- Observed token prefixes: No
- Token depth counts:
  - depth 1: 1 tokens

  Typical interpretations:

  Sample tokens:
  - No


### Namespace 49b48001-ca20-4adc-8111-5b60c903a50c (ServiceEndpoints)
- Observed token prefixes: endpoints
- Token depth counts:
  - depth 2: 4 tokens
  - depth 3: 116 tokens

  Typical interpretations:

  Sample tokens:
  - endpoints/00000000-0000-0000-0000-000000000000/3bbdd3ed-6f9c-4dba-b086-c6010d935237
  - endpoints/00000000-0000-0000-0000-000000000000/51e80eef-3469-4387-95da-325b4a77c3a6
  - endpoints/00000000-0000-0000-0000-000000000000/ceac4ce4-6d29-43db-92e4-e5c82869c901
  - endpoints/040a2515-b6d4-4b53-8749-fa8b649ce73a
  - endpoints/040a2515-b6d4-4b53-8749-fa8b649ce73a/a212c44c-4074-4fe0-bd2b-940a469435ee
  - endpoints/696416ee-f7ff-4ee3-934a-979b00dce74f
  - endpoints/696416ee-f7ff-4ee3-934a-979b00dce74f/0d2aa2a7-6d35-4c8c-94ed-bc53a47ea7b1
  - endpoints/696416ee-f7ff-4ee3-934a-979b00dce74f/a6f19241-6a01-408d-aaac-b52b63d347f1


### Namespace cb594ebe-87dd-4fc9-ac2c-6a10a4c92046 (ServiceHooks)
- Observed token prefixes: PublisherSecurity
- Token depth counts:
  - depth 1: 1 tokens
  - depth 2: 15 tokens

  Typical interpretations:

  Sample tokens:
  - PublisherSecurity
  - PublisherSecurity/040a2515-b6d4-4b53-8749-fa8b649ce73a
  - PublisherSecurity/12ee2112-bbe9-4b70-9fdb-6592ad84098a
  - PublisherSecurity/1b39554e-193a-47b2-bae7-bba1e5697ace
  - PublisherSecurity/58d7cbb5-9855-45e7-93d3-f08409cb272e
  - PublisherSecurity/5e02360f-bc08-49fa-b5f3-bab8a9d7a408
  - PublisherSecurity/696416ee-f7ff-4ee3-934a-979b00dce74f
  - PublisherSecurity/70e9c41e-68af-4300-baa3-4eee0f48b17e


### Namespace 3e65f728-f8bc-4ecd-8764-7e378b19bfa7 (Collection)
- Observed token prefixes: NAMESPACE
- Token depth counts:
  - depth 1: 1 tokens

  Typical interpretations:

  Sample tokens:
  - NAMESPACE


### Namespace cb4d56d2-e84b-457e-8845-81320a133fbb (Proxy)
- Observed token prefixes: No
- Token depth counts:
  - depth 1: 1 tokens

  Typical interpretations:

  Sample tokens:
  - No


### Namespace bed337f8-e5f3-4fb9-80da-81e17d06e7a8 (Plan)
- Observed token prefixes: Plan
- Token depth counts:
  - depth 1: 1 tokens

  Typical interpretations:

  Sample tokens:
  - Plan


### Namespace 2dab47f9-bd70-49ed-9bd5-8eb051e59c02 (Process)
- Observed token prefixes: $PROCESS, $PROCESS:adcc42ab-9882-485e-a3ed-7678f01f66bc:1a9e818f-c3c8-42c5-8352-1315c45fbd75, $PROCESS:adcc42ab-9882-485e-a3ed-7678f01f66bc:2bcee563-f99a-4adf-925e-ce3aab36d591, $PROCESS:adcc42ab-9882-485e-a3ed-7678f01f66bc:302ad5c4-05c1-474b-96f6-943b7657b667
- Token depth counts:
  - depth 1: 4 tokens

  Typical interpretations:

  Sample tokens:
  - $PROCESS
  - $PROCESS:adcc42ab-9882-485e-a3ed-7678f01f66bc:1a9e818f-c3c8-42c5-8352-1315c45fbd75
  - $PROCESS:adcc42ab-9882-485e-a3ed-7678f01f66bc:2bcee563-f99a-4adf-925e-ce3aab36d591
  - $PROCESS:adcc42ab-9882-485e-a3ed-7678f01f66bc:302ad5c4-05c1-474b-96f6-943b7657b667


### Namespace 11238e09-49f2-40c7-94d0-8f0307204ce4 (AccountAdminSecurity)
- Observed token prefixes: 
- Token depth counts:
  - depth 2: 1 tokens

  Typical interpretations:

  Sample tokens:
  - /Ownership


### Namespace b7e84409-6553-448a-bbb2-af228e07cbeb (Library)
- Observed token prefixes: Library
- Token depth counts:
  - depth 2: 4 tokens
  - depth 4: 16 tokens

  Typical interpretations:

  Sample tokens:
  - Library/696416ee-f7ff-4ee3-934a-979b00dce74f
  - Library/696416ee-f7ff-4ee3-934a-979b00dce74f/VariableGroup/82
  - Library/696416ee-f7ff-4ee3-934a-979b00dce74f/VariableGroup/83
  - Library/696416ee-f7ff-4ee3-934a-979b00dce74f/VariableGroup/84
  - Library/696416ee-f7ff-4ee3-934a-979b00dce74f/VariableGroup/85
  - Library/696416ee-f7ff-4ee3-934a-979b00dce74f/VariableGroup/86
  - Library/696416ee-f7ff-4ee3-934a-979b00dce74f/VariableGroup/87
  - Library/696416ee-f7ff-4ee3-934a-979b00dce74f/VariableGroup/90


### Namespace 83d4c2e6-e57d-4d6e-892b-b87222b7ad20 (Environment)
- Observed token prefixes: Environments
- Token depth counts:
  - depth 2: 13 tokens
  - depth 3: 8 tokens

  Typical interpretations:

  Sample tokens:
  - Environments/040a2515-b6d4-4b53-8749-fa8b649ce73a
  - Environments/1
  - Environments/2
  - Environments/3
  - Environments/4
  - Environments/5
  - Environments/6
  - Environments/696416ee-f7ff-4ee3-934a-979b00dce74f


### Namespace 58b176e7-3411-457a-89d0-c6d0ccb3c52b (EventSubscription)
- Observed token prefixes: No
- Token depth counts:
  - depth 1: 1 tokens

  Typical interpretations:

  Sample tokens:
  - No


### Namespace 83e28ad4-2d72-4ceb-97b0-c7726d5502c3 (CSS)
- Observed token prefixes: vstfs:
- Token depth counts:
  - depth 6: 15 tokens
  - depth 11: 6 tokens
  - depth 16: 3 tokens
  - depth 21: 1 tokens

  Typical interpretations:

  Sample tokens:
  - vstfs:///Classification/Node/01d16433-9dab-476d-bd7a-32818015d580
  - vstfs:///Classification/Node/0380fec0-4406-45d1-8357-19adfaee14a1
  - vstfs:///Classification/Node/0f0430d4-d25a-4d11-b4af-e4216fb3aa40
  - vstfs:///Classification/Node/0f0430d4-d25a-4d11-b4af-e4216fb3aa40:vstfs:///Classification/Node/10962b4d-983d-43ab-ad50-5a89fb771481
  - vstfs:///Classification/Node/0f0430d4-d25a-4d11-b4af-e4216fb3aa40:vstfs:///Classification/Node/10962b4d-983d-43ab-ad50-5a89fb771481:vstfs:///Classification/Node/317dd773-306f-44cc-a6e2-37831ccf5532
  - vstfs:///Classification/Node/0f0430d4-d25a-4d11-b4af-e4216fb3aa40:vstfs:///Classification/Node/10962b4d-983d-43ab-ad50-5a89fb771481:vstfs:///Classification/Node/9d068246-758c-471d-871c-dfb964be8fcc
  - vstfs:///Classification/Node/0f0430d4-d25a-4d11-b4af-e4216fb3aa40:vstfs:///Classification/Node/10962b4d-983d-43ab-ad50-5a89fb771481:vstfs:///Classification/Node/a2ce7979-4d64-4db9-b576-f3a477c21eba
  - vstfs:///Classification/Node/0f0430d4-d25a-4d11-b4af-e4216fb3aa40:vstfs:///Classification/Node/10962b4d-983d-43ab-ad50-5a89fb771481:vstfs:///Classification/Node/a2ce7979-4d64-4db9-b576-f3a477c21eba:vstfs:///Classification/Node/eec92af5-1d70-402b-ba78-3b517131af56


### Namespace 9e4894c3-ff9a-4eac-8a85-ce11cafdc6f1 (TeamLabSecurity)
- Observed token prefixes: $
- Token depth counts:
  - depth 1: 1 tokens

  Typical interpretations:
  - Tokens prefixed with `$` often indicate collection/project-scoped items (historical TFVC or generic tokens). Example: `$/<projectId>`.

  Sample tokens:
  - $


### Namespace fc5b7b85-5d6b-41eb-8534-e128cb10eb67 (ProjectAnalysisLanguageMetrics)
- Observed token prefixes: No
- Token depth counts:
  - depth 1: 1 tokens

  Typical interpretations:

  Sample tokens:
  - No


### Namespace bb50f182-8e5e-40b8-bc21-e8752a1e7ae2 (Tagging)
- Observed token prefixes: , Microsoft.TeamFoundation.Identity;S-1-9-1551374245-1204400969-2402986413-2179408616-0-0-0-0-1, Microsoft.TeamFoundation.Identity;S-1-9-1551374245-1204400969-2402986413-2179408616-0-0-0-0-2
- Token depth counts:
  - depth 1: 2 tokens
  - depth 2: 15 tokens

  Typical interpretations:

  Sample tokens:
  - /040a2515-b6d4-4b53-8749-fa8b649ce73a
  - /12ee2112-bbe9-4b70-9fdb-6592ad84098a
  - /1b39554e-193a-47b2-bae7-bba1e5697ace
  - /58d7cbb5-9855-45e7-93d3-f08409cb272e
  - /5e02360f-bc08-49fa-b5f3-bab8a9d7a408
  - /696416ee-f7ff-4ee3-934a-979b00dce74f
  - /70e9c41e-68af-4300-baa3-4eee0f48b17e
  - /964e3180-c2c0-4e82-80da-b3491fd6ed81


### Namespace f6a4de49-dbe2-4704-86dc-f8ec1a294436 (MetaTask)
- Observed token prefixes: 696416ee-f7ff-4ee3-934a-979b00dce74f, 70e9c41e-68af-4300-baa3-4eee0f48b17e, 964e3180-c2c0-4e82-80da-b3491fd6ed81, f6ad111f-42cb-4e2d-b22a-cd0bd6f5aebd
- Token depth counts:
  - depth 1: 4 tokens

  Typical interpretations:

  Sample tokens:
  - 696416ee-f7ff-4ee3-934a-979b00dce74f
  - 70e9c41e-68af-4300-baa3-4eee0f48b17e
  - 964e3180-c2c0-4e82-80da-b3491fd6ed81
  - f6ad111f-42cb-4e2d-b22a-cd0bd6f5aebd


### Namespace bf7bfa03-b2b7-47db-8113-fa2e002cc5b1 (Iteration)
- Observed token prefixes: vstfs:
- Token depth counts:
  - depth 6: 15 tokens
  - depth 11: 45 tokens
  - depth 16: 1 tokens

  Typical interpretations:

  Sample tokens:
  - vstfs:///Classification/Node/18c76992-93fa-4eb2-aac0-0abc0be212d6
  - vstfs:///Classification/Node/18c76992-93fa-4eb2-aac0-0abc0be212d6:vstfs:///Classification/Node/12a070b9-73dd-454f-9c79-eb92ed7a7630
  - vstfs:///Classification/Node/2d667e7c-3682-4046-bfe3-c70bd6fe91c4
  - vstfs:///Classification/Node/2d667e7c-3682-4046-bfe3-c70bd6fe91c4:vstfs:///Classification/Node/4565dbc2-000c-493d-8a28-b29c38dfde38
  - vstfs:///Classification/Node/2d667e7c-3682-4046-bfe3-c70bd6fe91c4:vstfs:///Classification/Node/88500b38-c8fb-486d-adf2-a73c002e0589
  - vstfs:///Classification/Node/2d667e7c-3682-4046-bfe3-c70bd6fe91c4:vstfs:///Classification/Node/b8de7cb2-e4b6-4956-b02d-2ebf48121938
  - vstfs:///Classification/Node/3343188c-c408-446c-8abb-64e645a7315d
  - vstfs:///Classification/Node/3343188c-c408-446c-8abb-64e645a7315d:vstfs:///Classification/Node/1c4a6872-2c53-451d-a3fc-af39ad54f0e7


### Namespace 71356614-aad7-4757-8f2c-0fb3bff6f680 (WorkItemQueryFolders)
- Observed token prefixes: $
- Token depth counts:
  - depth 1: 1 tokens
  - depth 2: 1 tokens
  - depth 3: 4 tokens
  - depth 4: 1 tokens
  - depth 6: 1 tokens

  Typical interpretations:
  - Tokens prefixed with `$` often indicate collection/project-scoped items (historical TFVC or generic tokens). Example: `$/<projectId>`.

  Sample tokens:
  - $
  - $/696416ee-f7ff-4ee3-934a-979b00dce74f
  - $/696416ee-f7ff-4ee3-934a-979b00dce74f/8669ab9b-d21f-4fab-98d3-bee06ea54d3c
  - $/70e9c41e-68af-4300-baa3-4eee0f48b17e/51bd99bb-9cca-471b-832f-b24cb0070624
  - $/70e9c41e-68af-4300-baa3-4eee0f48b17e/51bd99bb-9cca-471b-832f-b24cb0070624/f6caba3d-7909-442c-afc0-265b761e452f
  - $/70e9c41e-68af-4300-baa3-4eee0f48b17e/51bd99bb-9cca-471b-832f-b24cb0070624/f6caba3d-7909-442c-afc0-265b761e452f/95bbde9a-8e50-4461-bf96-d7a8dc63ff92/6a16b459-8a21-4bda-a649-50564f7ff1a0
  - $/964e3180-c2c0-4e82-80da-b3491fd6ed81/bac9a8c5-7fbb-41be-abbd-082a28061336
  - $/f6ad111f-42cb-4e2d-b22a-cd0bd6f5aebd/3046cff6-83c9-449a-a024-be38fbaac2e5


### Namespace fa557b48-b5bf-458a-bb2b-1b680426fe8b (Favorites)
- Observed token prefixes: No
- Token depth counts:
  - depth 1: 1 tokens

  Typical interpretations:

  Sample tokens:
  - No


### Namespace 4ae0db5d-8437-4ee8-a18b-1f6fb38bd34c (Registry)
- Observed token prefixes: No
- Token depth counts:
  - depth 1: 1 tokens

  Typical interpretations:

  Sample tokens:
  - No


### Namespace c2ee56c9-e8fa-4cdd-9d48-2c44f697a58e (Graph)
- Observed token prefixes: No
- Token depth counts:
  - depth 1: 1 tokens

  Typical interpretations:

  Sample tokens:
  - No


### Namespace dc02bf3d-cd48-46c3-8a41-345094ecc94b (ViewActivityPaneSecurity)
- Observed token prefixes: No
- Token depth counts:
  - depth 1: 1 tokens

  Typical interpretations:

  Sample tokens:
  - No


### Namespace 2a887f97-db68-4b7c-9ae3-5cebd7add999 (Job)
- Observed token prefixes: No
- Token depth counts:
  - depth 1: 1 tokens

  Typical interpretations:

  Sample tokens:
  - No


### Namespace 7cd317f2-adc6-4b6c-8d99-6074faeaf173 (EventPublish)
- Observed token prefixes: No
- Token depth counts:
  - depth 1: 1 tokens

  Typical interpretations:

  Sample tokens:
  - No


### Namespace 73e71c45-d483-40d5-bdba-62fd076f7f87 (WorkItemTracking)
- Observed token prefixes: No
- Token depth counts:
  - depth 1: 1 tokens

  Typical interpretations:

  Sample tokens:
  - No


### Namespace 4a9e8381-289a-4dfd-8460-69028eaa93b3 (StrongBox)
- Observed token prefixes: No
- Token depth counts:
  - depth 1: 1 tokens

  Typical interpretations:

  Sample tokens:
  - No


### Namespace 1f4179b3-6bac-4d01-b421-71ea09171400 (Server)
- Observed token prefixes: FrameworkGlobalSecurity
- Token depth counts:
  - depth 1: 1 tokens

  Typical interpretations:

  Sample tokens:
  - FrameworkGlobalSecurity


### Namespace e06e1c24-e93d-4e4a-908a-7d951187b483 (TestManagement)
- Observed token prefixes: No
- Token depth counts:
  - depth 1: 1 tokens

  Typical interpretations:

  Sample tokens:
  - No


### Namespace 6ec4592e-048c-434e-8e6c-8671753a8418 (SettingEntries)
- Observed token prefixes: No
- Token depth counts:
  - depth 1: 1 tokens

  Typical interpretations:

  Sample tokens:
  - No


### Namespace 302acaca-b667-436d-a946-87133492041c (BuildAdministration)
- Observed token prefixes: BuildPrivileges
- Token depth counts:
  - depth 1: 1 tokens

  Typical interpretations:

  Sample tokens:
  - BuildPrivileges


### Namespace 2725d2bc-7520-4af4-b0e3-8d876494731f (Location)
- Observed token prefixes: No
- Token depth counts:
  - depth 1: 1 tokens

  Typical interpretations:

  Sample tokens:
  - No


### Namespace 251e12d9-bea3-43a8-bfdb-901b98c0125e (Boards)
- Observed token prefixes: No
- Token depth counts:
  - depth 1: 1 tokens

  Typical interpretations:

  Sample tokens:
  - No


### Namespace f0003bce-5f45-4f93-a25d-90fc33fe3aa9 (OrganizationLevelData)
- Observed token prefixes: No
- Token depth counts:
  - depth 1: 1 tokens

  Typical interpretations:

  Sample tokens:
  - No


### Namespace 83abde3a-4593-424e-b45f-9898af99034d (UtilizationPermissions)
- Observed token prefixes: No
- Token depth counts:
  - depth 1: 1 tokens

  Typical interpretations:

  Sample tokens:
  - No


### Namespace c0e7a722-1cad-4ae6-b340-a8467501e7ce (WorkItemsHub)
- Observed token prefixes: No
- Token depth counts:
  - depth 1: 1 tokens

  Typical interpretations:

  Sample tokens:
  - No


### Namespace 0582eb05-c896-449a-b933-aa3d99e121d6 (WebPlatform)
- Observed token prefixes: No
- Token depth counts:
  - depth 1: 1 tokens

  Typical interpretations:

  Sample tokens:
  - No


### Namespace 66312704-deb5-43f9-b51c-ab4ff5e351c3 (VersionControlPrivileges)
- Observed token prefixes: Global
- Token depth counts:
  - depth 1: 1 tokens

  Typical interpretations:

  Sample tokens:
  - Global


### Namespace 93bafc04-9075-403a-9367-b7164eac6b5c (Workspaces)
- Observed token prefixes: Microsoft.TeamFoundation.Identity;S-1-9-1551374245-1204400969-2402986413-2179408616-0-0-0-0-1, Microsoft.TeamFoundation.Identity;S-1-9-1551374245-1204400969-2402986413-2179408616-0-0-0-0-2
- Token depth counts:
  - depth 1: 2 tokens

  Typical interpretations:

  Sample tokens:
  - Microsoft.TeamFoundation.Identity;S-1-9-1551374245-1204400969-2402986413-2179408616-0-0-0-0-1
  - Microsoft.TeamFoundation.Identity;S-1-9-1551374245-1204400969-2402986413-2179408616-0-0-0-0-2


### Namespace 093cbb02-722b-4ad6-9f88-bc452043fa63 (CrossProjectWidgetView)
- Observed token prefixes: No
- Token depth counts:
  - depth 1: 1 tokens

  Typical interpretations:

  Sample tokens:
  - No


### Namespace 35e35e8e-686d-4b01-aff6-c369d6e36ce0 (WorkItemTrackingConfiguration)
- Observed token prefixes: No
- Token depth counts:
  - depth 1: 1 tokens

  Typical interpretations:

  Sample tokens:
  - No


### Namespace 0d140cae-8ac1-4f48-b6d1-c93ce0301a12 (Discussion Threads)
- Observed token prefixes: Microsoft.TeamFoundation.Identity;S-1-9-1551374245-1204400969-2402986413-2179408616-0-0-0-0-1, Microsoft.TeamFoundation.Identity;S-1-9-1551374245-1204400969-2402986413-2179408616-0-0-0-0-2, Microsoft.TeamFoundation.Identity;S-1-9-1551374245-1204400969-2402986413-2179408616-0-0-0-0-3
- Token depth counts:
  - depth 1: 3 tokens

  Typical interpretations:

  Sample tokens:
  - Microsoft.TeamFoundation.Identity;S-1-9-1551374245-1204400969-2402986413-2179408616-0-0-0-0-1
  - Microsoft.TeamFoundation.Identity;S-1-9-1551374245-1204400969-2402986413-2179408616-0-0-0-0-2
  - Microsoft.TeamFoundation.Identity;S-1-9-1551374245-1204400969-2402986413-2179408616-0-0-0-0-3


### Namespace 5ab15bc8-4ea1-d0f3-8344-cab8fe976877 (BoardsExternalIntegration)
- Observed token prefixes: $
- Token depth counts:
  - depth 2: 3 tokens

  Typical interpretations:
  - Tokens prefixed with `$` often indicate collection/project-scoped items (historical TFVC or generic tokens). Example: `$/<projectId>`.

  Sample tokens:
  - $/696416ee-f7ff-4ee3-934a-979b00dce74f
  - $/70e9c41e-68af-4300-baa3-4eee0f48b17e
  - $/f6ad111f-42cb-4e2d-b22a-cd0bd6f5aebd


### Namespace 7ffa7cf4-317c-4fea-8f1d-cfda50cfa956 (DataProvider)
- Observed token prefixes: No
- Token depth counts:
  - depth 1: 1 tokens

  Typical interpretations:

  Sample tokens:
  - No


### Namespace 81c27cc8-7a9f-48ee-b63f-df1e1d0412dd (Social)
- Observed token prefixes: No
- Token depth counts:
  - depth 1: 1 tokens

  Typical interpretations:

  Sample tokens:
  - No


### Namespace 9a82c708-bfbe-4f31-984c-e860c2196781 (Security)
- Observed token prefixes: No
- Token depth counts:
  - depth 1: 1 tokens

  Typical interpretations:

  Sample tokens:
  - No


### Namespace a60e0d84-c2f8-48e4-9c0c-f32da48d5fd1 (IdentityPicker)
- Observed token prefixes: No
- Token depth counts:
  - depth 1: 1 tokens

  Typical interpretations:

  Sample tokens:
  - No


### Namespace 84cc1aa4-15bc-423d-90d9-f97c450fc729 (ServicingOrchestration)
- Observed token prefixes: No
- Token depth counts:
  - depth 1: 1 tokens

  Typical interpretations:

  Sample tokens:
  - No


### Namespace 33344d9c-fc72-4d6f-aba5-fa317101a7e9 (Build)
- Observed token prefixes: 040a2515-b6d4-4b53-8749-fa8b649ce73a, 12ee2112-bbe9-4b70-9fdb-6592ad84098a, 1b39554e-193a-47b2-bae7-bba1e5697ace, 58d7cbb5-9855-45e7-93d3-f08409cb272e, 5e02360f-bc08-49fa-b5f3-bab8a9d7a408, 696416ee-f7ff-4ee3-934a-979b00dce74f, 70e9c41e-68af-4300-baa3-4eee0f48b17e, 964e3180-c2c0-4e82-80da-b3491fd6ed81, 97608c11-9c22-4f3f-9146-12f49d6f720d, b0281b3b-0ccb-4f8a-8761-c361ef9d9394, c1c630e4-abe1-4eed-9f30-0836ca9de4e6, ceee037d-2265-4e95-95e8-e1dd4eef9145, ead948c5-7ca4-43a4-8f94-558de8b873b2, f4eb664f-797c-4cf8-aa86-2e44bfaa2c81, f6ad111f-42cb-4e2d-b22a-cd0bd6f5aebd
- Token depth counts:
  - depth 1: 15 tokens
  - depth 2: 1 tokens

  Typical interpretations:

  Sample tokens:
  - 040a2515-b6d4-4b53-8749-fa8b649ce73a
  - 12ee2112-bbe9-4b70-9fdb-6592ad84098a
  - 1b39554e-193a-47b2-bae7-bba1e5697ace
  - 58d7cbb5-9855-45e7-93d3-f08409cb272e
  - 5e02360f-bc08-49fa-b5f3-bab8a9d7a408
  - 696416ee-f7ff-4ee3-934a-979b00dce74f
  - 696416ee-f7ff-4ee3-934a-979b00dce74f/237
  - 70e9c41e-68af-4300-baa3-4eee0f48b17e


### Namespace 8adf73b7-389a-4276-b638-fe1653f7efc7 (DashboardsPrivileges)
- Observed token prefixes: $
- Token depth counts:
  - depth 1: 1 tokens
  - depth 3: 3 tokens

  Typical interpretations:
  - Tokens prefixed with `$` often indicate collection/project-scoped items (historical TFVC or generic tokens). Example: `$/<projectId>`.

  Sample tokens:
  - $
  - $/964e3180-c2c0-4e82-80da-b3491fd6ed81/00000000-0000-0000-0000-000000000000
  - $/964e3180-c2c0-4e82-80da-b3491fd6ed81/b242d77b-d07c-4a5e-a9d8-3b29ef658448
  - $/f6ad111f-42cb-4e2d-b22a-cd0bd6f5aebd/00000000-0000-0000-0000-000000000000


### Namespace 52d39943-cb85-4d7f-8fa8-c6baac873819 (Project)
- Observed token prefixes: $PROJECT, $PROJECT:vstfs:
- Token depth counts:
  - depth 1: 1 tokens
  - depth 6: 15 tokens

  Typical interpretations:

  Sample tokens:
  - $PROJECT
  - $PROJECT:vstfs:///Classification/TeamProject/040a2515-b6d4-4b53-8749-fa8b649ce73a
  - $PROJECT:vstfs:///Classification/TeamProject/12ee2112-bbe9-4b70-9fdb-6592ad84098a
  - $PROJECT:vstfs:///Classification/TeamProject/1b39554e-193a-47b2-bae7-bba1e5697ace
  - $PROJECT:vstfs:///Classification/TeamProject/58d7cbb5-9855-45e7-93d3-f08409cb272e
  - $PROJECT:vstfs:///Classification/TeamProject/5e02360f-bc08-49fa-b5f3-bab8a9d7a408
  - $PROJECT:vstfs:///Classification/TeamProject/696416ee-f7ff-4ee3-934a-979b00dce74f
  - $PROJECT:vstfs:///Classification/TeamProject/70e9c41e-68af-4300-baa3-4eee0f48b17e


### Namespace a39371cf-0841-4c16-bbd3-276e341bc052 (VersionControlItems)
- Observed token prefixes: $
- Token depth counts:
  - depth 1: 1 tokens
  - depth 2: 15 tokens

  Typical interpretations:
  - Tokens prefixed with `$` often indicate collection/project-scoped items (historical TFVC or generic tokens). Example: `$/<projectId>`.

  Sample tokens:
  - $
  - $/AzDO
  - $/D12ee2112-bbe9-4b70-9fdb-6592ad84098a
  - $/D1b39554e-193a-47b2-bae7-bba1e5697ace
  - $/D58d7cbb5-9855-45e7-93d3-f08409cb272e
  - $/D5e02360f-bc08-49fa-b5f3-bab8a9d7a408
  - $/D97608c11-9c22-4f3f-9146-12f49d6f720d
  - $/Db0281b3b-0ccb-4f8a-8761-c361ef9d9394



## Authoritative reference and namespace examples

The Microsoft Learn article "Security namespace and permission reference" is the canonical source for namespace IDs, token formats and whether a namespace is hierarchical or flat. Key points from that page (https://learn.microsoft.com/en-us/azure/devops/organizations/security/namespace-reference):

- Namespaces are either hierarchical or flat; hierarchical namespaces support parent→child tokens and inheritance.
- Tokens are namespace-specific and case-insensitive; separators (commonly `/`) are used when path parts are variable-length.
- The page includes canonical token examples for many namespaces (Git Repositories, Build, Dashboards, etc.).

### Small canonical mapping

| Namespace (msdocs ID) | Typical token format (canonical) | Notes / observed samples |
|---|---|---|
| Git Repositories (`repoV2`, ID: 2e9eb7ed-...) | `repoV2` (root), `repoV2/{projectId}`, `repoV2/{projectId}/{repoId}` | Matches observed tokens in `namespace-aces.txt` and msdocs example. Use project-level token to affect all repos in a project.
| Build (`33344d9c-...`) | `PROJECT_ID` or `PROJECT_ID/{definitionId}` | msdocs shows build defs as `PROJECT_ID/12` examples; `namespace-aces.txt` shows service identity entries tied to project IDs.
| Dashboards (`8adf73b7-...`) | `$/PROJECT_ID/Team_ID/Dashboard_ID` | msdocs documents `$` prefixes for some UI-managed tokens; `namespace-aces.txt` shows `$` tokens for project-scoped entries.
| Analytics (`58450c49-...`) | `$/Shared/PROJECT_ID` | msdocs gives `$/Shared/PROJECT_ID` example for Analytics views.
| Project (`52d39943-...`) | `$PROJECT:vstfs:///Classification/TeamProject/{projectId}` | msdocs documents `$PROJECT` style root tokens for project-level permissions.

Notes:
- The exact namespace IDs are available in `namespaces.txt` and on msdocs; use `azdo security permission namespace list` or the msdocs page to map names ↔ UUIDs.
- Always validate by listing ACL tokens for a namespace in your org (see "Tip: Determine token formats by listing ACLs" in this doc).

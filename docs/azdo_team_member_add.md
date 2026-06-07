## Command `azdo team member add`

```
azdo team member add [ORGANIZATION/]PROJECT/TEAM [flags]
```

Add one or more users or groups as members of a team.

The positional argument accepts the team's project and team name in the
form [ORGANIZATION/]PROJECT/TEAM.


### Options


* `-q`, `--jq` `expression`

	Filter JSON output using a jq expression

* `--json` `fields`

	Output JSON with the specified fields. Prefix a field with &#39;-&#39; to exclude it.

* `-t`, `--template` `string`

	Format JSON output using a Go template; see &#34;azdo help formatting&#34;

* `-u`, `--user` `strings`

	Members to add. Accepts a descriptor, email, principal name, SID, or identity ID. Pass the flag multiple times to add several members.


### ALIASES

- `a`

### JSON Fields

`memberDescriptor`, `memberDisplayName`, `memberOrigin`, `memberOriginId`, `results`, `status`, `teamName`

### Examples

```bash
# Add a user by email
azdo team member add Fabrikam/FabrikamEngineering/MyTeam --user user@example.com

# Add multiple users in a single invocation
azdo team member add Fabrikam/MyProject/MyTeam -u alice@contoso.com -u bob@contoso.com

# Add a user by subject descriptor
azdo team member add MyOrg/Fabrikam/MyTeam --user vssgp.Uy0xLTItMw==
```

### See also

* [azdo team member](./azdo_team_member.md)

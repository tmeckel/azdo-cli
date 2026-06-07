## Command `azdo team member remove`

```
azdo team member remove [ORGANIZATION/]PROJECT/TEAM [flags]
```

Remove one or more users or groups from a team.

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

	Members to remove. Accepts a descriptor, email, principal name, SID, or identity ID. Pass the flag multiple times to remove several members.

* `-y`, `--yes`

	Skip the confirmation prompt.


### ALIASES

- `r`
- `rm`
- `del`
- `d`

### JSON Fields

`memberDescriptor`, `memberDisplayName`, `memberOrigin`, `memberOriginId`, `results`, `status`, `teamName`

### Examples

```bash
# Remove a user by email
azdo team member remove Fabrikam/FabrikamEngineering/MyTeam --user user@example.com

# Remove multiple users in a single invocation
azdo team member remove Fabrikam/MyProject/MyTeam -u alice@contoso.com -u bob@contoso.com

# Remove a user without confirmation prompt
azdo team member remove MyOrg/Fabrikam/MyTeam --user vssgp.Uy0xLTItMw== --yes
```

### See also

* [azdo team member](./azdo_team_member.md)

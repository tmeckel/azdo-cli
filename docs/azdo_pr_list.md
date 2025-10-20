## Command `azdo pr list`

```
azdo pr list [[organization/]project/repository] [flags]
```

List pull requests in a Azure DevOps repository or project.


### Options


* `-a`, `--author` `string`

	Filter by author

* `-B`, `--base` `string`

	Filter by base branch

* `-d`, `--draft`

	Filter by draft state

* `-H`, `--head` `string`

	Filter by head branch

* `-q`, `--jq` `expression`

	Filter JSON output using a jq expression

* `--json` `fields`

	Output JSON with the specified fields. Prefix a field with &#39;-&#39; to exclude it.

* `-l`, `--label` `strings`

	Filter by label

* `-L`, `--limit` `int`

	Maximum number of items to fetch

* `-m`, `--mergestate` `string`

	Filter by merge state: {succeeded|conflicts}

* `-r`, `--reviewer` `string`

	Filter by reviewer

* `-s`, `--state` `string`

	Filter by state: {abandoned|active|all|completed}

* `-t`, `--template` `string`

	Format JSON output using a Go template; see &#34;azdo help formatting&#34;


### ALIASES

- `ls`

### JSON Fields

`artifactId`, `autoCompleteSetBy`, `closedBy`, `closedDate`, `codeReviewId`, `commits`, `completionOptions`, `completionQueueTime`, `createdBy`, `creationDate`, `description`, `forkSource`, `hasMultipleMergeBases`, `isDraft`, `labels`, `lastMergeCommit`, `lastMergeSourceCommit`, `lastMergeTargetCommit`, `mergeFailureMessage`, `mergeFailureType`, `mergeId`, `mergeOptions`, `mergeStatus`, `pullRequestId`, `remoteUrl`, `repository`, `reviewers`, `sourceRefName`, `status`, `supportsIterations`, `targetRefName`, `title`, `url`, `workItemRefs`

### Examples

```bash
List open PRs authored by you
$ azdo pr list --author "@me"

List only PRs with all of the given labels
$ azdo pr list --label bug --label "priority 1"

Find a PRs that are completed
$ azdo pr list --state completed

List PRs using a template
$ azdo pr list --json pullRequestId,title --template '{{range.}}{{printf "#%.0f - %s\n" .pullRequestId .title}}{{end}}'

List PRs using a JQ filter and a template to render the result
$ azdo pr list \
     --json pullRequestId,title,isDraft,labels \
     --jq '.[] | select(.title | contains("dependency"))' \
     -t '{{range.}}{{printf "#%.0f - %s\n" .pullRequestId .title}}{{end}}'
 	```

### See also

* [azdo pr](./azdo_pr.md)

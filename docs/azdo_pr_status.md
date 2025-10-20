## Command `azdo pr status`

Show status of relevant pull requests

```
azdo pr status [flags]
```

### Options


* `-c`, `--conflict-status`

	Display the merge conflict status of each pull request

* `-q`, `--jq` `expression`

	Filter JSON output using a jq expression

* `--json` `fields`

	Output JSON with the specified fields. Prefix a field with &#39;-&#39; to exclude it.

* `-t`, `--template` `string`

	Format JSON output using a Go template; see &#34;azdo help formatting&#34;


### JSON Fields

`artifactId`, `autoCompleteSetBy`, `closedBy`, `closedDate`, `codeReviewId`, `commits`, `completionOptions`, `completionQueueTime`, `createdBy`, `creationDate`, `description`, `forkSource`, `hasMultipleMergeBases`, `isDraft`, `labels`, `lastMergeCommit`, `lastMergeSourceCommit`, `lastMergeTargetCommit`, `mergeFailureMessage`, `mergeFailureType`, `mergeId`, `mergeOptions`, `mergeStatus`, `pullRequestId`, `remoteUrl`, `repository`, `reviewers`, `sourceRefName`, `status`, `supportsIterations`, `targetRefName`, `title`, `url`, `workItemRefs`

### See also

* [azdo pr](./azdo_pr.md)

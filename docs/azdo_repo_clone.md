## Command `azdo repo clone`

```
azdo repo clone [organization/]project/repository [<directory>] [-- <gitflags>...]
```

Clone a GitHub repository locally. Pass additional `git clone` flags by listing
them after "--".

If the repository name does not specify an organization, the configured default orgnaization is used
or the value from the AZDO_ORGANIZATION environment variable.


### Options


* `--no-credential-helper`

	Don&#39;t configure azdo as credential helper for the cloned repository

* `--recurse-submodules`

	Update all submodules after checkout

* `-u`, `--upstream-remote-name` `string` (default `&#34;upstream&#34;`)

	Upstream remote name when cloning a fork


### ALIASES

- `c`

### See also

* [azdo repo](./azdo_repo.md)

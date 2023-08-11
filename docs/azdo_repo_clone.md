## azdo repo clone
```
azdo repo clone <repository> [<directory>] [-- <gitflags>...]
```
Clone a GitHub repository locally. Pass additional `git clone` flags by listing
them after "--".

### Options


* `--no-credential-helper`

	Don&#39;t configure azdo as credential helper for the cloned repository

* `-o`, `--organization` `string`

	Use organization

* `-p`, `--project` `string`

	Use project

* `-u`, `--upstream-remote-name` `string`

	Upstream remote name when cloning a fork


### See also

* [azdo repo](./azdo_repo)

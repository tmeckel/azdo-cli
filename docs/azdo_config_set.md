## Command `azdo config set`

Update configuration with a value for the given key

```
azdo config set <key> <value> [flags]
```

### Options


* `-o`, `--organization` `string`

	Set per-organization setting

* `-r`, `--remove`

	Remove config item for an organization, so that the default value will be in effect again


### Examples

```bash
$ azdo config set editor vim
$ azdo config set editor "code --wait"
$ azdo config set git_protocol ssh --organization myorg
$ azdo config set prompt disabled
$ azdo config set -r -o myorg git_protocol
```

### See also

* [azdo config](./azdo_config.md)

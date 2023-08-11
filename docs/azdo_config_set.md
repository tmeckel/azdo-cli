## azdo config set
Update configuration with a value for the given key
```
azdo config set <key> <value> [flags]
```
### Options


* `-o`, `--organization` `string`

	Set per-organization setting


### Examples

```bash
$ azdo config set editor vim
$ azdo config set editor "code --wait"
$ azdo config set git_protocol ssh --organization myorg
$ azdo config set prompt disabled
```

### See also

* [azdo config](./azdo_config.md)

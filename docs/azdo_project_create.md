## azdo project create
```
azdo project create [ORGANIZATION/]PROJECT [flags]
```
Create a new Azure DevOps project in the specified organization.

This command queues a project creation operation and polls for its completion.
By default, it waits for the project to be created and then displays the project details.

You can use the --no-wait flag to have the command return immediately after queuing the operation.
In this case, it will output the operation ID, status, and URL, which you can use to monitor the creation process.

The --max-wait flag allows you to specify a custom timeout for the polling operation.

If the organization name is omitted from the project argument, the default configured organization is used.

### Options


* `-d`, `--description` `string`

	Description for the new project

* `-q`, `--jq` `expression`

	Filter JSON output using a jq expression

* `--json` `fields`

	Output JSON with the specified fields

* `--max-wait` `int`

	Maximum wait time in seconds

* `--no-wait`

	Do not wait for the project to be created

* `-p`, `--process` `string`

	Process to use (e.g., Scrum, Agile, CMMI)

* `-s`, `--source-control` `string`

	Source control type (git or tfvc)

* `-t`, `--template` `string`

	Format JSON output using a Go template; see &#34;azdo help formatting&#34;

* `--visibility` `string`

	Project visibility (private or public)


### Examples

```bash
# Create a project in the default organization and wait for completion
azdo project create MyProject --description "A new project" --process "Scrum" --visibility private

# Create a public project with TFVC source control in a specific organization
azdo project create MyOrg/MyPublicProject --description "Public project" --source-control tfvc --visibility public

# Create a project and return immediately without waiting for completion
azdo project create MyOrg/MyAsyncProject --no-wait

# Create a project and wait for a maximum of 5 minutes for completion
azdo project create MyOrg/MyTimedProject --max-wait 300
```

### See also

* [azdo project](./azdo_project.md)

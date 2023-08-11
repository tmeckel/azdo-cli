## azdo config
Display or change configuration settings for azdo.

Current respected settings:
- git_protocol: the protocol to use for git clone and push operations (default: "https")
- editor: the text editor program to use for authoring text
- prompt: toggle interactive prompting in the terminal (default: "enabled")
- pager: the terminal pager program to send standard output to
- http_unix_socket: the path to a Unix socket through which to make an HTTP connection
- browser: the web browser to use for opening URLs
- default_organization: the default Azure DevOps organization to use, if no organization is specified

### Available commands
* [azdo config get](./azdo_config_get)
* [azdo config list](./azdo_config_list)
* [azdo config set](./azdo_config_set)

### See also

* [azdo](./azdo)

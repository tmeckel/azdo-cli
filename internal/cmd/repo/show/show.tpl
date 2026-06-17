{{bold "url:"}} {{hyperlink (s .Url) (s .Url)}}
{{bold "id:"}} {{u .Id}}
{{bold "name:"}} {{s .Name}}
{{bold "project:"}} {{s .Project.Name}}{{if hasText (u .Project.Id)}} ({{u .Project.Id}}){{end}}
{{if hasText (s .DefaultBranch)}}{{bold "default branch:"}} {{s .DefaultBranch}}{{end}}
{{bold "remote url:"}} {{s .RemoteUrl}}
{{bold "ssh url:"}} {{s .SshUrl}}
{{bold "web url:"}} {{s .WebUrl}}
{{bold "is fork:"}} {{b .IsFork}}
{{if hasText (parent .)}}{{bold "  parent:"}} {{parent .}}{{end}}
{{if hasBool .IsDisabled}}{{bold "is disabled:"}} {{b .IsDisabled}}{{end}}
{{if hasBool .IsInMaintenance}}{{bold "is in maintenance:"}} {{b .IsInMaintenance}}{{end}}
{{if .Size}}{{bold "size:"}} {{size .Size}}{{end}}

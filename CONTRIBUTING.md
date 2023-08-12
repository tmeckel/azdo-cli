# Contributing

❣ We love pull requests from everyone !

## All code changes happen through Pull Requests

Pull requests are the best way to propose changes to the codebase. We actively
welcome your pull requests:

1. Fork the repo and create your branch from `master`.
2. If you've added code that should be tested, add tests.
3. If you've added code that need documentation, update the documentation.
4. Write a [good commit message](http://tbaggery.com/2008/04/19/a-note-about-git-commit-messages.html).
5. Issue that pull request!

If you've never written Go in your life, then join the club! Go is widely
considered an easy-to-learn language, so if you're looking for an open source
project to gain dev experience, you've come to the right place.

## Running in a Github Codespace

If you want to start contributing to `azdo` with the click of a button, you can
open the `azdo` codebase in a Codespace. First fork the repo, then click to
create a codespace:

This allows you to contribute to `azdo` without needing to install anything on
your local machine. The Codespace has all the necessary tools and extensions
pre-installed.

## Code of conduct

Please note by participating in this project, you agree to abide by the [code of conduct](./CODE-OF-CONDUCT.md).

## Any contributions you make will be under the MIT Software License

In short, when you submit code changes, your submissions are understood to be
under the same [MIT License](./LICENSE) that
covers the project.

## Report bugs using Github's [issues](https://github.com/tmeckel/azdo-cli/issues)

We use GitHub issues to track public bugs. Report a bug by [opening a new
issue](https://github.com/tmeckel/azdo-cli/issues/new); it's that easy!

## Go

This project is written in Go. Go is an opinionated language with strict idioms, but some of those idioms are a little extreme. Some things we do differently:

1. There is no shame in using `self` as a receiver name in a struct method. In fact we encourage it
2. There is no shame in prefixing an interface with 'I' instead of suffixing with 'er' when there are several methods on the interface.
3. If a struct implements an interface, we make it explicit with something like:

```go
var _ MyInterface = &MyStruct{}
```

This makes the intent clearer and means that if we fail to satisfy the interface we'll get an error in the file that needs fixing.

### Code Formatting

To check code formatting [gofumpt](https://pkg.go.dev/mvdan.cc/gofumpt#section-readme) (which is a bit stricter than [gofmt](https://pkg.go.dev/cmd/gofmt)) is used.
VSCode will format the code correctly if you tell the Go extension to use `gofumpt` via your [`settings.json`](https://code.visualstudio.com/docs/getstarted/settings#_settingsjson)
by setting [`formatting.gofumpt`](https://github.com/golang/tools/blob/master/gopls/doc/settings.md#gofumpt-bool) to `true`:

```jsonc
// .vscode/settings.json
{
  "gopls": {
    "formatting.gofumpt": true
  }
}
```

To run gofumpt from your terminal go:

```bash
go install mvdan.cc/gofumpt@latest && gofumpt -l -w .
```

## Improvements

If you can think of any way to improve these docs let us know.

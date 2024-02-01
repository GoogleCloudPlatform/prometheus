# How to Contribute

We'd love to accept your patches and contributions to this project.

## Before you begin

### Sign our Contributor License Agreement

Contributions to this project must be accompanied by a
[Contributor License Agreement](https://cla.developers.google.com/about) (CLA).
You (or your employer) retain the copyright to your contribution; this simply
gives us permission to use and redistribute your contributions as part of the
project.

If you or your current employer have already signed the Google CLA (even if it
was for a different project), you probably don't need to do it again.

Visit <https://cla.developers.google.com/> to see your current agreements or to
sign a new one.

### Review our Community Guidelines

This project follows
[Google's Open Source Community Guidelines](https://opensource.google/conduct/).

## Contribution process

### Code Reviews

For complete instructions on how to compile see: [Building From Source](https://github.com/prometheus/prometheus#building-from-source)

For quickly compiling and testing your changes do:

```bash
# For building.
go build ./cmd/prometheus/
./prometheus

# For testing.
make test         # Make sure all the tests pass before you commit and push :)
```

To run a collection of Go linters through [`golangci-lint`](https://github.com/golangci/golangci-lint), do:
```bash
make lint
```

If it reports an issue and you think that the warning needs to be disregarded or is a false-positive, you can add a special comment `//nolint:linter1[,linter2,...]` before the offending line. Use this sparingly though, fixing the code to comply with the linter's recommendation is in general the preferred course of action. See [this section of the golangci-lint documentation](https://golangci-lint.run/usage/false-positives/#nolint-directive) for more information.

All our issues are regularly tagged so that you can also filter down the issues involving the components you want to work on. For our labeling policy refer [the wiki page](https://github.com/prometheus/prometheus/wiki/Label-Names-and-Descriptions).

## Pull Request Checklist

* Branch from the main branch and, if needed, rebase to the current main branch before submitting your pull request. If it doesn't merge cleanly with main you may be asked to rebase your changes.

* Commits should be as small as possible, while ensuring that each commit is correct independently (i.e., each commit should compile and pass tests).

* If your patch is not getting reviewed or you need a specific person to review it, you can @-reply a reviewer asking for a review in the pull request or a comment, or you can ask for a review on the IRC channel [#prometheus-dev](https://web.libera.chat/?channels=#prometheus-dev) on irc.libera.chat (for the easiest start, [join via Element](https://app.element.io/#/room/#prometheus-dev:matrix.org)).

* Add tests relevant to the fixed bug or new feature.

## Dependency management

The Prometheus project uses [Go modules](https://golang.org/cmd/go/#hdr-Modules__module_versions__and_more) to manage dependencies on external packages.

To add or update a new dependency, use the `go get` command:

```bash
# Pick the latest tagged release.
go get example.com/some/module/pkg@latest

# Pick a specific version.
go get example.com/some/module/pkg@vX.Y.Z
```

Tidy up the `go.mod` and `go.sum` files:

```bash
# The GO111MODULE variable can be omitted when the code isn't located in GOPATH.
GO111MODULE=on go mod tidy
```

You have to commit the changes to `go.mod` and `go.sum` before submitting the pull request.

## Working with the PromQL parser

The PromQL parser grammar is located in `promql/parser/generated_parser.y` and it can be built using `make parser`.
The parser is built using [goyacc](https://pkg.go.dev/golang.org/x/tools/cmd/goyacc)

If doing some sort of debugging, then it is possible to add some verbose output. After generating the parser, then you
can modify the `./promql/parser/generated_parser.y.go` manually.

```golang
// As of writing this was somewhere around line 600.
var (
	yyDebug        = 0 // This can be a number 0 -> 5.
	yyErrorVerbose = false  // This can be set to true.
)

```

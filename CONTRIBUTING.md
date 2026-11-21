## Contributing

[fork]: /fork
[pr]: /compare
[style]: https://go.dev/doc/effective_go
[code-of-conduct]: CODE_OF_CONDUCT.md

Hi there! We're thrilled that you'd like to contribute to this project. Your help is essential for keeping it great.

Please note that this project is released with a [Contributor Code of Conduct][code-of-conduct]. By participating in this project you agree to abide by its terms.

## Issues and PRs

If you have suggestions for how this project could be improved, or want to report a bug, open an issue! We'd love all and any contributions. If you have questions, too, we'd love to hear them.

We'd also love PRs. If you're thinking of a large PR, we advise opening up an issue first to talk about it, though! Look at the links below if you're not sure how to open a PR.

## Submitting a pull request

1. [Fork][fork] and clone the repository.
1. Make sure the checks pass on your machine:
   ```sh
   go test ./...
   go test -race ./...
   go vet ./...
   gofmt -l .          # must print nothing
   govulncheck ./...
   (cd contracttest && go vet ./... && go test ./... && govulncheck -test ./...)
   ```
   `contracttest` is a nested test-only module: it validates the responses recorded in `testdata/contract` against the specs in `testdata/openapi`.
1. Create a new branch: `git checkout -b my-branch-name`.
1. Make your change, add tests, and make sure the tests still pass.
   Golden files live in `testdata`; after an intended change to a generated spec or a recorded response, run `go test -run 'CompleteRoute' -update .` in the root and review the diff.
1. Push to your fork and [submit a pull request][pr].
1. Pat your self on the back and wait for your pull request to be reviewed and merged.

Here are a few things you can do that will increase the likelihood of your pull request being accepted:

- Follow [Effective Go][style] and keep the code `gofmt`-clean; `go vet` reports common mistakes.
- Write and update tests.
- Keep your changes as focused as possible. If there are multiple changes you would like to make that are not dependent upon each other, consider submitting them as separate pull requests.
- Write a [good commit message](http://tbaggery.com/2008/04/19/a-note-about-git-commit-messages.html).

Work in Progress pull requests are also welcome to get feedback early on, or if there is something blocked you.

## Release checklist

Before the owner tags a release, on the commit to tag:

1. Run the checks listed above, root and `contracttest`.
1. Validate the generated specs: `cd contracttest && go test -run TestSpecsAreValid ./...`. It validates every spec in `testdata/openapi` with kin-openapi, examples checked against their schemas. A test that writes a new spec there is covered without other changes.
1. Review the exported API with `go doc ./...`; deprecate before removing a public name.
1. Call out every change to a golden file in `testdata/openapi` in the release notes: it is a change to the spec services publish.

## Resources

- [How to Contribute to Open Source](https://opensource.guide/how-to-contribute/)
- [Using Pull Requests](https://help.github.com/articles/about-pull-requests/)
- [GitHub Help](https://help.github.com)

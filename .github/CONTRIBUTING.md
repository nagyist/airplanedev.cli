# Contributing to the Airplane CLI

## Releases

Releases are managed by [GoReleaser](https://github.com/goreleaser/goreleaser). This produces binaries for various architectures and uploads them as GitHub artifacts. It also releases to Homebrew through [`airplanedev/homebrew-tap`](https://github.com/airplanedev/homebrew-tap).

This all happens automatically via GitHub Actions whenever a new tag is published:

```sh
export AIRPLANE_CLI_TAG=v0.0.1-alpha.2 && \
  git tag ${AIRPLANE_CLI_TAG} && \
  git push origin ${AIRPLANE_CLI_TAG}
```

Note that tags ending in e.g. `-alpha.2` and `-rc1` are automatically marked as prerelease
in GitHub: https://goreleaser.com/customization/release/

You can test this build process locally by running:

```sh
# or https://goreleaser.com/install/
brew install goreleaser/tap/goreleaser

make test-release
```

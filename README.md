# Airplane CLI

> **Note**
> As of May 12, 2023, this repository is only used for GitHub releases. The code in this repository is no longer actively updated.

Provides CLI access to [app.airplane.dev](https://app.airplane.dev).

Once you install the CLI, run `airplane --help` to get started.

## Installation

### Mac/Linux

If you are using [Homebrew](https://brew.sh/):

```sh
brew install airplanedev/tap/airplane
```

To upgrade to the latest version with Homebrew:

```sh
brew update && brew upgrade airplane
```

Otherwise, you can install with `curl`:

```sh
curl -L https://github.com/airplanedev/cli/releases/latest/download/install.sh | sh
```

To upgrade, you can re-run this script and the latest version will be re-installed.

### Windows

On Windows, you can install with our PowerShell script:

```sh
iwr https://github.com/airplanedev/cli/releases/latest/download/install.ps1 -useb | iex
```

To upgrade, you can re-run this script and the latest version will be re-installed.

# Release Checklist

Fleet releases are produced by semantic-release when conventional commits land
on `main`. semantic-release determines the next version from commits since the
latest `v*` tag, creates the tag, creates the GitHub release, and uploads the
packaged artifacts.

## Local Dry Run

```sh
make release-local VERSION=0.1.0
(cd dist && shasum -c *.sha256)
```

This creates:

- `dist/fleet_VERSION_darwin_arm64.tar.gz`
- `dist/fleet_VERSION_darwin_arm64.tar.gz.sha256`
- `dist/FleetMenuBar_VERSION_darwin_arm64.zip`
- `dist/FleetMenuBar_VERSION_darwin_arm64.zip.sha256`

## GitHub Release

Merge conventional commits to `main`:

```sh
feat: add machine-readable process telemetry
fix: avoid stale state lock cleanup
feat!: change status JSON schema
```

semantic-release maps commits to versions:

- `fix:` creates a patch release.
- `feat:` creates a minor release.
- `feat!:` or `BREAKING CHANGE:` creates a major release.
- `docs:`, `test:`, `chore:`, and similar commits do not release by default.

The release workflow runs Go tests, menu bar tests, lets semantic-release choose
the version, packages the CLI and menu bar app, and uploads checksummed
artifacts to the GitHub release.

GitHub release notes are generated from conventional commits. Install guidance
is also documented here and in the README:

- Download `fleet_VERSION_darwin_arm64.tar.gz` and, when installing the menu
  bar app too, `FleetMenuBar_VERSION_darwin_arm64.zip` into the same directory.
- Verify downloads with the matching `.sha256` files from the download
  directory: `shasum -c *.sha256`.
- Extract the CLI archive and run `scripts/install.sh`. The installer copies
  `fleet` to `/opt/homebrew/bin`, initializes `~/.fleet` when needed, and
  installs `FleetMenuBar.app` when it finds the menu bar zip next to the
  extracted CLI directory.

## Signing And Notarization

By default, CI publishes an unsigned menu bar artifact. When Apple signing
secrets are configured, the release workflow also uploads a Developer ID signed
and notarized zip.

The release workflow is ready to sign when these GitHub secrets are configured:

- `APPLE_DEVELOPER_ID`
- `APPLE_ID`
- `APPLE_TEAM_ID`
- `APPLE_APP_PASSWORD`
- `APPLE_CERTIFICATE_P12_BASE64`
- `APPLE_CERTIFICATE_PASSWORD`

For local signing:

```sh
make menubar-build
APPLE_DEVELOPER_ID="Developer ID Application: Example (TEAMID)" \
APPLE_ID="you@example.com" \
APPLE_TEAM_ID="TEAMID" \
APPLE_APP_PASSWORD="app-specific-password" \
scripts/sign-notarize-menubar.sh
```

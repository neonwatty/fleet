# Release Checklist

Fleet release artifacts are produced from Git tags named `v*`.

## Local Dry Run

```sh
make release-local VERSION=0.1.0
shasum -c dist/*.sha256
```

This creates:

- `dist/fleet_VERSION_darwin_arm64.tar.gz`
- `dist/fleet_VERSION_darwin_arm64.tar.gz.sha256`
- `dist/FleetMenuBar_VERSION_darwin_arm64.zip`
- `dist/FleetMenuBar_VERSION_darwin_arm64.zip.sha256`

## GitHub Release

```sh
git tag v0.1.0
git push origin v0.1.0
```

Release candidate tags use the same workflow and are published as GitHub
prereleases automatically:

```sh
git tag v0.1.0-rc4
git push origin v0.1.0-rc4
```

The release workflow runs Go tests, menu bar tests, packages the CLI and menu
bar app, and uploads checksummed artifacts to the GitHub release.

Release notes include generated GitHub changes plus concise install guidance:

- Download `fleet_VERSION_darwin_arm64.tar.gz` and, when installing the menu
  bar app too, `FleetMenuBar_VERSION_darwin_arm64.zip` into the same directory.
- Verify downloads with the matching `.sha256` files.
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

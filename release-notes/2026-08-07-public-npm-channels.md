## Public npm channels

- Publish `@operatorstack/yield` with six platform-specific runtime packages.
- Send merged revisions to the `canary` dist-tag using immutable prerelease versions.
- Publish stable versions to `latest` only through an explicit release dispatch bound to an immutable Git tag.
- Authenticate from GitHub Actions with npm trusted publishing and automatic provenance; no long-lived npm publishing token is stored.

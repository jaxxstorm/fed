# Private container publishing

`.github/workflows/publish.yml` builds the production Dockerfile and publishes
to **ghcr.io/jaxxstorm/fed** on every push, plus manual workflow dispatches.
Pull requests do not publish. Each run pushes:

- `sha-<full commit SHA>` for selecting a specific build.
- `latest` for the most recently completed publish, including pushes to branches
  other than the default branch. Concurrent builds can finish out of commit order;
  use the SHA tag or image digest for reproducible deployments.

The workflow authenticates with GitHub's generated `GITHUB_TOKEN` and needs
`contents: read` and `packages: write`. No registry secret needs to be stored in
the repository. The image includes an OCI source label associating it with the
repository. Existing application checks run separately from publishing.

## Repository and package setup

1. Push this repository and workflow to a repository owned by `jaxxstorm`, with
   GitHub Actions enabled. A local commit alone does not trigger CI.
2. GHCR initially creates container packages with **private** visibility. Keep
   `jaxxstorm/fed` private in GitHub package settings. The workflow refuses to
   publish when it can see an existing non-private package.
3. If the package already exists, give this repository Actions write access in
   its package settings. A permission failure must be resolved there; the
   workflow does not change package visibility or grants automatically.

## Pull from a deployment host

Use a GitHub personal access token (classic) with `read:packages` from a user
allowed to read the private package. Supply it through the host's secret manager:

```sh
printf '%s' "$GHCR_TOKEN" | docker login ghcr.io -u jaxxstorm --password-stdin
docker pull ghcr.io/jaxxstorm/fed:latest
```

The image runs as non-root and listens on port 8080. Deployment still requires
PostgreSQL, migrations, persistent artifact storage, production OIDC settings,
and provider configuration. See `provider-applications.md` for connection secrets
and migration requirements. Do not use development identity on a public host.

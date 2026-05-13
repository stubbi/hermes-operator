# Release Process

> The release pipeline is fully automated except for: (1) creating the PAT
> once, (2) merging the release-please PR. Everything else fires on its own.

## One-time setup

See Plan 6, Task 1. Summary:

- Repository secret `RELEASE_PLEASE_TOKEN` is a classic PAT with `repo` +
  `workflow` scopes on `stubbi/hermes-operator`.
- Workflow permissions: default-write enabled.
- Packages: write enabled.

Calendar a yearly reminder to renew the PAT; the weekly
`verify-signing.yaml` workflow won't catch a stale PAT (it only verifies
already-published releases).

## Per-release flow

1. **Land conventional commits to `main`.** `feat:` and `fix:` show up in the
   changelog. `docs:`, `chore:`, `ci:`, `test:`, `build:`, `refactor:`, `perf:`
   are hidden but counted.
2. **release-please opens a release PR** named
   `chore(main): release vX.Y.Z`. The PR contains:
   - `CHANGELOG.md` update
   - `.release-please-manifest.json` bump
   - `charts/hermes-operator/Chart.yaml` version + appVersion bump
   - `charts/hermes-operator/values.yaml` image.tag bump
   - `bundle/manifests/hermes-operator.clusterserviceversion.yaml` version +
     metadata.name + containerImage bump
3. **Review and merge the PR.** Squash-and-merge is fine; the commit subject
   must remain `chore(main): release vX.Y.Z` for the tag creator step to
   recognise it.
4. **release-please.yaml's "Create release tag" step** detects the merge
   commit and pushes `vX.Y.Z` via the PAT. The PAT-authored push fires
   downstream workflows.
5. **release.yaml** fires on the tag and does the heavy lifting:
   - GoReleaser builds multi-arch binaries + Docker images
   - Cosign signs every tag (latest, vX.Y.Z, X.Y) at digest
   - syft generates an SPDX-JSON SBOM
   - cosign attests the SBOM at the same digest
   - SBOM uploaded as a release asset
   - Release is flipped from draft → published (via the PAT, so the
     `release.published` event fires)
   - Helm chart is packaged and pushed to `oci://ghcr.io/stubbi/charts`
6. **operatorhub-submit.yaml** fires on the published-release event:
   - Forks `k8s-operatorhub/community-operators`, creates a branch with the
     new bundle, opens a PR
   - Same for `redhat-openshift-ecosystem/community-operators-prod`
7. **Conformance suite** runs on the tag (`conformance.yaml`'s tag trigger).
   The release is considered shippable only after this passes.

## Cutting v1.0.0 from v0.1.0

The manifest starts at `0.1.0` (Plan 6 Task 2 explains why). To make
release-please cut a *major* version (`v1.0.0`):

1. Make a commit on main with a `!` to mark a breaking change:
   `feat!: declare v1 API stability`
2. release-please bumps from `0.1.0` directly to `1.0.0`.
3. Merge the release PR as normal.

After v1.0.0, regular `feat:` bumps minor, regular `fix:` bumps patch.

## Manual fallbacks

If the bundle PR didn't open (network blip, fork name collision), trigger
manually:

```bash
gh workflow run "OperatorHub Submission" -f tag=vX.Y.Z
```

If a release was tagged but the release workflow didn't run (very rare —
usually because the PAT expired), retag:

```bash
git tag -d vX.Y.Z
git push origin :refs/tags/vX.Y.Z
git tag vX.Y.Z <sha>
git push origin vX.Y.Z   # uses your local git creds (not GHA's GITHUB_TOKEN)
```

## Verifying a release manually

```bash
make verify-signing       # uses gh release view to find the latest tag
cosign verify ghcr.io/stubbi/hermes-operator:vX.Y.Z \
  --certificate-identity-regexp 'https://github.com/stubbi/hermes-operator/.github/workflows/.*' \
  --certificate-oidc-issuer https://token.actions.githubusercontent.com
```

See `docs/security/signing.md` for the full verification ritual.

## What ships with each release

- Multi-arch (linux/amd64 + linux/arm64) operator image:
  `ghcr.io/stubbi/hermes-operator:vX.Y.Z` and `:X.Y` and `:latest`
- Multi-arch agent image:
  `ghcr.io/stubbi/hermes-agent:vX.Y.Z` (built by a separate hermes-agent
  release; the operator's `appVersion` doesn't pin agent versions —
  `spec.image.tag` does)
- OLM bundle image:
  `ghcr.io/stubbi/hermes-operator-bundle:vX.Y.Z`
- Helm chart (OCI):
  `oci://ghcr.io/stubbi/charts/hermes-operator:X.Y.Z`
- Plain manifests:
  `https://github.com/stubbi/hermes-operator/releases/download/vX.Y.Z/install.yaml`
- SBOM:
  `https://github.com/stubbi/hermes-operator/releases/download/vX.Y.Z/sbom.spdx.json`
- Cosign signature + SBOM attestation against every image digest
- OperatorHub PRs (auto-opened): community-operators + community-operators-prod

## What does NOT ship

- Source archives (the tag itself is the source-of-truth)
- Pre-built operator binaries outside the Docker image (operator-only use is
  rare; we don't optimise for it)
- Krew plugin (post-v1; see spec §12)

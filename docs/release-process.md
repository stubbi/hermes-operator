# Release Process

> The release pipeline is fully automated except for merging the
> release-please PR. Everything else fires on its own.

## One-time setup

Repository settings (Settings, Secrets and variables, Actions). Every entry is
optional; the workflows degrade gracefully when one is missing.

| Name | Kind | Purpose |
|---|---|---|
| `RELEASE_BOT_APP_ID` | variable | GitHub App ID used by `release-please.yaml` and `release.yaml`. Preferred credential: the tag push and `release: published` then trigger `release.yaml` and `operatorhub-submit.yaml` on their own. |
| `RELEASE_BOT_PRIVATE_KEY` | secret | Private key of that App. The App must be installed on this repository with Contents, Pull requests and Issues read/write. |
| `RELEASE_PLEASE_TOKEN` | secret | PAT fallback when the App is not configured (classic `repo` + `workflow`, or fine-grained with the same permissions on this repository). |
| `IMAGE_NAMESPACE` | variable | GHCR namespace the operator image, the agent image and the chart are published to. Defaults to `paperclipinc`. Must match the chart's default `image.repository` (`release.yaml` fails early otherwise) and the bundle CSV. |
| `GHCR_PUSH_TOKEN` | secret | PAT with `write:packages` from a member of `IMAGE_NAMESPACE`. Needed whenever that namespace is not owned by the repository owner (the repository moved from `paperclipinc` to `stubbi`; the images still live under `paperclipinc`), because `GITHUB_TOKEN` can only push into the owner's own namespace. |
| `GHCR_PUSH_USER` | variable | Login of the `GHCR_PUSH_TOKEN` owner. |

With neither the App nor `RELEASE_PLEASE_TOKEN` set, both workflows fall back
to `GITHUB_TOKEN`. Events created with it do not trigger other workflows, so:

- the release PR gets no CI run on its own: push an empty commit to it, or
  close and reopen it, so the required checks run;
- `release-please.yaml` dispatches `release.yaml` for the new tag, and
  `release.yaml` dispatches `operatorhub-submit.yaml` after publishing.

Without `GHCR_PUSH_TOKEN` (and with `IMAGE_NAMESPACE` not equal to the
repository owner) the `docker` job on `main` builds but does not push the
`:dev` image, and the release publish steps fail at GHCR login or push.

## Per-release flow

1. **Land conventional commits to `main`.** `feat:` and `fix:` show up in the
   changelog. `docs:`, `chore:`, `ci:`, `test:`, `build:`, `refactor:`, `perf:`
   are hidden but counted.
2. **release-please opens a release PR** named
   `chore(main): release vX.Y.Z`. The PR contains:
   - `CHANGELOG.md` update
   - `.release-please-manifest.json` bump
   - `charts/hermes-operator/Chart.yaml` version + appVersion bump
   - `bundle/manifests/hermes-operator.clusterserviceversion.yaml`:
     `spec.version`, `metadata.name`, `containerImage`, the `relatedImages`
     operator entry and the manager deployment image. All five carry an inline
     `# x-release-please-version` marker and are rewritten by the *generic*
     updater, which swaps the semver in place and so preserves the leading `v`.
     Do not point a yaml/jsonpath updater at this file: it writes a bare
     version (breaking the image pull and `operatorhub-submit.yaml`'s sed, see
     #113) and it round-trips the document, dropping the markers so nothing
     else gets bumped at all (see #129).
     `hack/check-chart-image-tags.sh` guards both.
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
   - Helm chart is packaged and pushed to `oci://ghcr.io/paperclipinc/charts`
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

If a release was tagged but the release workflow didn't run (usually an
expired or missing App/PAT credential), dispatch it for the tag:

```bash
gh workflow run release.yaml --ref vX.Y.Z
```

Or retag:

```bash
git tag -d vX.Y.Z
git push origin :refs/tags/vX.Y.Z
git tag vX.Y.Z <sha>
git push origin vX.Y.Z   # uses your local git creds (not GHA's GITHUB_TOKEN)
```

## Verifying a release manually

```bash
make verify-signing       # uses gh release view to find the latest tag
cosign verify ghcr.io/paperclipinc/hermes-operator:vX.Y.Z \
  --certificate-identity-regexp '^https://github.com/(paperclipinc|stubbi)/hermes-operator/.github/workflows/.*' \
  --certificate-oidc-issuer https://token.actions.githubusercontent.com
```

See `docs/security/signing.md` for the full verification ritual.

## What ships with each release

- Multi-arch (linux/amd64 + linux/arm64) operator image:
  `ghcr.io/paperclipinc/hermes-operator:vX.Y.Z` and `:X.Y` and `:latest`
- Multi-arch agent image:
  `ghcr.io/paperclipinc/hermes-agent:vX.Y.Z` (built by a separate hermes-agent
  release; the operator's `appVersion` doesn't pin agent versions:
  `spec.image.tag` does)
- OLM bundle image:
  `ghcr.io/paperclipinc/hermes-operator-bundle:vX.Y.Z`
- Helm chart (OCI):
  `oci://ghcr.io/paperclipinc/charts/hermes-operator:X.Y.Z`
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

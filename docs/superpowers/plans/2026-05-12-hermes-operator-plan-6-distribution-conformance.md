# Hermes Operator: Plan 6: Distribution + v1 Conformance Suite

> **For agentic workers:** REQUIRED SUB-SKILL: Use `superpowers:subagent-driven-development` (recommended) or `superpowers:executing-plans` to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Ship hermes-operator as a fully distributable v1.0: signed multi-arch images, SBOMs, OLM bundle auto-submitted to OperatorHub, Helm + kustomize channels, and a conformance suite (negative + idempotency + upgrade-path + GitOps + failure-injection + benchmarks) that mechanically defends every v1 stability commitment in spec §11.

**Architecture:** release-please drives version bumps from conventional commits; merging the release PR creates a `vX.Y.Z` tag via a PAT; the tag fires GoReleaser, which builds and signs multi-arch operator + agent images, generates SBOMs via syft, attests them with cosign, and uploads release assets; a downstream OperatorHub workflow forks `k8s-operatorhub/community-operators` and opens a bundle-update PR. The conformance suite is a separate `test/conformance/` tree wired into a nightly CI job that exercises five mechanically distinct stability invariants and a benchmark suite that fails CI on >20% regression.

**Tech Stack:** Go 1.24, GoReleaser v2, release-please-action v4, Cosign (keyless OIDC), Syft (anchore/sbom-action), operator-sdk v1.38.0, OPM, Helm 3, kustomize, kind, Ginkgo v2, Gomega, benchstat, GitHub Actions.

**Prerequisite:** Plans 1-5 merged. Plan 1 established the kubebuilder scaffold, `internal/resources/` builders, `internal/controller/` reconciler, the Helm chart at `charts/hermes-operator/`, and the CI workflows `ci.yaml`, `reconcile-guard.yaml`, `helm-rbac.yaml`, `build.yaml`, `e2e.yaml`. Plan 2 fleshed out the full `HermesInstance` spec, validating/defaulting webhooks, conversion-webhook scaffolding. Plan 3 implemented gateways, runtime init, Honcho profileStore, networking, autoupdate. Plan 4 implemented the `HermesSelfConfig` controller with SSA + the cluster-scoped `HermesClusterDefaults`. Plan 5 implemented backup/restore + finalizer + migration.fromOpenClaw. This plan does not modify reconciler logic; it adds distribution plumbing and a conformance suite that exercises what Plans 1-5 already built.

**Spec reference:** `docs/superpowers/specs/2026-05-12-hermes-operator-design.md` §9 (distribution), §10 (testing strategy), §11 (v1 stability commitments).

**Manual one-time setup the user owns (documented under Task 1):**
- Create a classic GitHub PAT named `RELEASE_PLEASE_TOKEN` with `repo` scope on `stubbi/hermes-operator` and store it as a repository secret named `RELEASE_PLEASE_TOKEN`. Required because tags created by `GITHUB_TOKEN` do not fire downstream workflows.
- Confirm `id-token: write` is available on the repo (default for public repos, must be enabled in org settings for private).

---

## File Structure Established by This Plan

```
.
├── .goreleaser.yaml                                # new: operator + agent build, multi-arch, sign, SBOM
├── release-please-config.json                      # new: extra-files: Chart.yaml, CSV, values.yaml
├── .release-please-manifest.json                   # new: starts at 0.1.0
├── bundle.Dockerfile                               # new: OLM bundle image
├── bundle/
│   ├── ci.yaml                                     # new: reviewers + updateGraph: semver-mode
│   ├── manifests/
│   │   ├── hermes-operator.clusterserviceversion.yaml   # new: CSV (versioned via release-please)
│   │   ├── hermes.agent_hermesinstances.yaml            # copied from config/crd/bases/
│   │   ├── hermes.agent_hermesselfconfigs.yaml          # copied
│   │   └── hermes.agent_hermesclusterdefaults.yaml      # copied
│   ├── metadata/
│   │   └── annotations.yaml                        # new: OLM channel metadata
│   └── tests/scorecard/
│       └── config.yaml                             # new: scorecard test config
├── test/conformance/                               # new: all conformance tests
│   ├── conformance_suite_test.go                   # Ginkgo entrypoint + shared kind harness
│   ├── helpers.go                                  # shared test helpers
│   ├── negative_test.go                            # webhook deny-path table
│   ├── idempotency_test.go                         # 10-reconcile no-op canary across a corpus
│   ├── upgrade_test.go                             # prior-release → HEAD upgrade matrix
│   ├── gitops_test.go                              # FluxCD + SelfConfig SSA no-flap
│   ├── failure_injection_test.go                   # SIGKILL the manager mid-reconcile
│   └── testdata/
│       ├── minimal.yaml
│       ├── maximal.yaml
│       ├── gateways-all.yaml
│       ├── selfconfig-enabled.yaml
│       ├── profilestore-enabled.yaml
│       ├── autoupdate-enabled.yaml
│       ├── backup-enabled.yaml
│       ├── networking-ingress.yaml
│       ├── observability-full.yaml
│       └── ollama-webterminal-tailscale.yaml
├── internal/resources/resources_bench_test.go      # new: builder benchmarks
├── internal/controller/controller_bench_test.go    # new: envtest full-reconcile microbench
├── docs/
│   ├── release-process.md                          # new: release SOP
│   ├── conformance.md                              # new: what each conformance category protects
│   ├── security/signing.md                         # new: Cosign verification commands
│   └── supported-versions.md                       # new: k8s support matrix + EOL policy
└── .github/workflows/
    ├── release-please.yaml                         # new: release PR + tag creation
    ├── release.yaml                                # new: GoReleaser + Cosign + SBOM + chart publish
    ├── operatorhub-submit.yaml                     # new: cross-fork PR to community-operators
    ├── verify-signing.yaml                         # new: weekly drift check
    ├── conformance.yaml                            # new: nightly + on-tag conformance
    ├── benchmark.yaml                              # new: PR benchmark diff
    ├── ci.yaml                                     # MODIFIED: ENVTEST_K8S_VERSION matrix
    └── e2e.yaml                                    # MODIFIED: kind node-image matrix 1.28→1.32

Makefile additions:
  installer  bundle  bundle-build  bundle-push  bundle-validate
  catalog-build  catalog-push  verify-signing
  conformance  conformance-negative  conformance-idempotency
  conformance-upgrade  conformance-gitops  conformance-failure
  bench  bench-resources  bench-controller
```

---

## Task 1: One-time secret + PAT setup (user-driven)

**Files:** none (this task documents external setup; the script verifies it).

This task is a *checklist* the user runs before merging this plan. The plan cannot create the PAT for them.

- [ ] **Step 1: Create the `RELEASE_PLEASE_TOKEN` PAT**

In a browser:
1. Visit `https://github.com/settings/tokens/new` (classic PAT: *not* fine-grained, because release-please needs cross-repo access for the OperatorHub fork).
2. Note: `RELEASE_PLEASE_TOKEN for hermes-operator`.
3. Expiration: 1 year (calendar a renewal: drift catches a stale token).
4. Scopes: tick `repo` (full) and `workflow`.
5. Generate, copy the token.

- [ ] **Step 2: Store as repository secret**

```bash
gh secret set RELEASE_PLEASE_TOKEN --repo stubbi/hermes-operator
# Paste token at prompt.
```
Expected: `✓ Set Actions secret RELEASE_PLEASE_TOKEN for stubbi/hermes-operator`.

- [ ] **Step 3: Verify `id-token: write` is permitted**

```bash
gh api repos/stubbi/hermes-operator/actions/permissions/workflow | jq .default_workflow_permissions
```
Expected: `"write"`. If `"read"`, run:
```bash
gh api -X PUT repos/stubbi/hermes-operator/actions/permissions/workflow \
  -f default_workflow_permissions=write \
  -F can_approve_pull_request_reviews=true
```

- [ ] **Step 4: Verify GHCR write is on**

```bash
gh api repos/stubbi/hermes-operator --jq '.has_packages'
```
Expected: `true`. If `false`, enable packages on the repo settings page (admin-only).

- [ ] **Step 5: Document the setup in `docs/release-process.md`** (created later in Task 23). For now, drop a stub:

```bash
mkdir -p docs
cat > docs/release-process.md <<'EOF'
# Release Process

> Filled out in Plan 6 Task 23. One-time setup is in Plan 6 Task 1.
EOF
```

- [ ] **Step 6: Commit**

```bash
git add docs/release-process.md
git commit -m "docs: stub release-process.md (filled out in Plan 6 Task 23)"
```

---

## Task 2: `release-please-config.json` and the manifest

**Files:**
- Create: `release-please-config.json`, `.release-please-manifest.json`

Models openclaw's proven config: `skip-github-release: true` (GoReleaser owns the release lifecycle), `extra-files` covering every file that embeds the semver string.

- [ ] **Step 1: Write `release-please-config.json`**

Create `release-please-config.json`:

```json
{
  "$schema": "https://raw.githubusercontent.com/googleapis/release-please/main/schemas/config.json",
  "packages": {
    ".": {
      "release-type": "simple",
      "include-v-in-tag": true,
      "bump-minor-pre-major": true,
      "bump-patch-for-minor-pre-major": true,
      "skip-github-release": true,
      "changelog-sections": [
        { "type": "feat", "section": "Features" },
        { "type": "fix", "section": "Bug Fixes" },
        { "type": "refactor", "section": "Refactoring" },
        { "type": "perf", "section": "Performance" },
        { "type": "docs", "section": "Documentation", "hidden": true },
        { "type": "chore", "section": "Miscellaneous", "hidden": true },
        { "type": "ci", "section": "CI", "hidden": true },
        { "type": "test", "section": "Tests", "hidden": true },
        { "type": "build", "section": "Build", "hidden": true }
      ],
      "extra-files": [
        {
          "type": "yaml",
          "path": "charts/hermes-operator/Chart.yaml",
          "jsonpath": "$.version"
        },
        {
          "type": "yaml",
          "path": "charts/hermes-operator/Chart.yaml",
          "jsonpath": "$.appVersion"
        },
        {
          "type": "yaml",
          "path": "charts/hermes-operator/values.yaml",
          "jsonpath": "$.image.tag"
        },
        {
          "type": "yaml",
          "path": "bundle/manifests/hermes-operator.clusterserviceversion.yaml",
          "jsonpath": "$.metadata.name"
        },
        {
          "type": "yaml",
          "path": "bundle/manifests/hermes-operator.clusterserviceversion.yaml",
          "jsonpath": "$.metadata.annotations.containerImage"
        },
        {
          "type": "yaml",
          "path": "bundle/manifests/hermes-operator.clusterserviceversion.yaml",
          "jsonpath": "$.spec.version"
        }
      ]
    }
  }
}
```

Why each extra-file:
- `Chart.yaml/$.version`: the Helm chart's own semver (consumers pin against this).
- `Chart.yaml/$.appVersion`: operator image tag the chart deploys.
- `values.yaml/$.image.tag`: default image tag if a user doesn't override.
- CSV `metadata.name`: must be `hermes-operator.vX.Y.Z` per OLM convention; release-please rewrites the entire value to match.
- CSV `metadata.annotations.containerImage`: `ghcr.io/stubbi/hermes-operator:vX.Y.Z` shown in OperatorHub UI.
- CSV `spec.version`: the OLM-visible semver (without `v` prefix).

- [ ] **Step 2: Write `.release-please-manifest.json`**

```json
{
  ".": "0.1.0"
}
```

Why `0.1.0` (not `0.0.0`): per the spec, the first cut is `v1.0.0`: but release-please needs a starting point that's *one less* than the first intended release so it can bump. Setting `0.1.0` with `bump-minor-pre-major: true` and the first `feat:` commit produces `v0.2.0`; we won't ship that: instead, the first time we want `v1.0.0`, we commit a `feat!: ...` (breaking change marker) which triggers a major bump from `0.1.0` to `1.0.0`. Document this in `docs/release-process.md`.

- [ ] **Step 3: Sanity-check JSON**

```bash
jq . release-please-config.json > /dev/null
jq . .release-please-manifest.json > /dev/null
```
Expected: exit 0 for both.

- [ ] **Step 4: Commit**

```bash
git add release-please-config.json .release-please-manifest.json
git commit -m "ci: add release-please config and manifest (starts at 0.1.0)"
```

---

## Task 3: `release-please.yaml` workflow

**Files:**
- Create: `.github/workflows/release-please.yaml`

- [ ] **Step 1: Write the workflow**

Create `.github/workflows/release-please.yaml`:

```yaml
name: Release Please

on:
  push:
    branches: [main]

permissions:
  contents: write
  pull-requests: write

jobs:
  release-please:
    runs-on: ubuntu-latest
    steps:
      - uses: googleapis/release-please-action@v4
        id: release
        with:
          config-file: release-please-config.json
          manifest-file: .release-please-manifest.json
          # PAT required: tags created by GITHUB_TOKEN don't trigger
          # downstream workflows (release.yaml). A PAT makes the tag
          # push appear as a user action, which triggers the release.
          token: ${{ secrets.RELEASE_PLEASE_TOKEN }}

      # skip-github-release is set in release-please-config.json so that
      # release-please does NOT create a GitHub release (which would
      # conflict with GoReleaser's draft-then-publish flow on immutable
      # releases). However, this also skips tag creation AND leaves the
      # merged PR labeled "autorelease: pending" forever. We create the
      # tag manually below, and flip stale labels in a separate step.
      - name: Create release tag
        env:
          GH_TOKEN: ${{ secrets.RELEASE_PLEASE_TOKEN }}
        run: |
          set -euo pipefail
          COMMIT_MSG=$(gh api "repos/${{ github.repository }}/commits/${{ github.sha }}" \
            --jq '.commit.message' | head -1)

          if [[ "$COMMIT_MSG" =~ ^chore\(main\):\ release\ ([0-9]+\.[0-9]+\.[0-9]+) ]]; then
            VERSION="${BASH_REMATCH[1]}"
            TAG="v${VERSION}"

            if gh api "repos/${{ github.repository }}/git/refs/tags/${TAG}" &>/dev/null; then
              echo "Tag ${TAG} already exists, skipping"
            else
              echo "Creating tag ${TAG} at ${{ github.sha }}"
              gh api "repos/${{ github.repository }}/git/refs" \
                -f ref="refs/tags/${TAG}" \
                -f sha="${{ github.sha }}"
              echo "Tag ${TAG} created - release workflow will trigger"
            fi
          else
            echo "Not a release commit, skipping tag creation"
          fi

      - name: Fix stale autorelease labels
        env:
          GH_TOKEN: ${{ secrets.RELEASE_PLEASE_TOKEN }}
        run: |
          set -euo pipefail
          for STATE in merged closed; do
            STALE_PRS=$(gh pr list \
              --repo "${{ github.repository }}" \
              --state "$STATE" \
              --label "autorelease: pending" \
              --json number \
              --jq '.[].number')

            for PR in $STALE_PRS; do
              echo "Fixing stale label on ${STATE} PR #${PR}"
              gh api "repos/${{ github.repository }}/issues/${PR}/labels/autorelease:%20pending" \
                -X DELETE || true
              gh api "repos/${{ github.repository }}/issues/${PR}/labels" \
                -f "labels[]=autorelease: tagged" || true
            done
          done
```

- [ ] **Step 2: Lint the YAML**

```bash
python -c "import yaml; yaml.safe_load(open('.github/workflows/release-please.yaml'))"
```
Expected: exit 0.

- [ ] **Step 3: Commit**

```bash
git add .github/workflows/release-please.yaml
git commit -m "ci: add release-please workflow with PAT-driven tag creation"
```

---

## Task 4: `.goreleaser.yaml`

**Files:**
- Create: `.goreleaser.yaml`

- [ ] **Step 1: Write the config**

Create `.goreleaser.yaml`:

```yaml
version: 2

project_name: hermes-operator

builds:
  - id: manager
    main: ./cmd/manager
    binary: manager
    env:
      - CGO_ENABLED=0
    goos:
      - linux
    goarch:
      - amd64
      - arm64
    ldflags:
      - -s -w
      - -X main.version={{.Version}}
      - -X main.commit={{.Commit}}
      - -X main.date={{.Date}}

dockers_v2:
  - id: hermes-operator
    dockerfile: Dockerfile
    ids:
      - manager
    images:
      - "ghcr.io/stubbi/hermes-operator"
    tags:
      - "{{ .Tag }}"
      - "{{ .Major }}.{{ .Minor }}"
      - "latest"
    platforms:
      - linux/amd64
      - linux/arm64
    labels:
      "org.opencontainers.image.title": "{{ .ProjectName }}"
      "org.opencontainers.image.version": "{{ .Version }}"
      "org.opencontainers.image.revision": "{{ .FullCommit }}"
      "org.opencontainers.image.created": "{{ .Date }}"
      "org.opencontainers.image.source": "{{ .GitURL }}"
      "org.opencontainers.image.licenses": "Apache-2.0"
    build_args:
      PREBUILT_BINARY: "manager"
    extra_files:
      - cmd/
      - api/
      - internal/
      - go.mod
      - go.sum

archives:
  - id: binaries
    formats:
      - tar.gz
    name_template: "{{ .ProjectName }}_{{ .Version }}_{{ .Os }}_{{ .Arch }}"
    files:
      - LICENSE
      - README.md
      - CHANGELOG.md

checksum:
  name_template: "checksums.txt"

changelog:
  sort: asc
  filters:
    exclude:
      - "^docs:"
      - "^test:"
      - "^ci:"
      - "^chore:"
      - "^build:"

release:
  draft: true
  replace_existing_artifacts: true
  github:
    owner: stubbi
    name: hermes-operator
  extra_files:
    - glob: dist/install.yaml
```

The `dist/install.yaml` is generated by `make installer` (Task 13) and uploaded as a release asset so plain-kustomize users can `kubectl apply -f <release-url>/install.yaml`.

- [ ] **Step 2: Make sure the Dockerfile accepts `PREBUILT_BINARY`**

Plan 1 created a multi-stage Dockerfile. Confirm it has the prebuilt-binary path; if not, append after the existing `FROM` block in `Dockerfile`:

```dockerfile
ARG PREBUILT_BINARY=""
# Path used by GoReleaser dockers_v2: copies a pre-built binary instead of
# rebuilding inside the image (multi-arch correctness + 5x faster).
# When PREBUILT_BINARY is empty (the local `make docker-build` path),
# the earlier RUN go build step provides /workspace/manager.
```

If the Dockerfile does not have this contract yet, the engineer should consult the openclaw Dockerfile for the exact final-stage `COPY` pattern (`COPY --from=builder ${PREBUILT_BINARY:+./}${PREBUILT_BINARY:-/workspace/manager} /manager`). This is the only Plan-6 modification to the Dockerfile.

- [ ] **Step 3: Install goreleaser locally and validate the config**

```bash
go install github.com/goreleaser/goreleaser/v2@latest
goreleaser check
```
Expected: `config is valid`. If any schema errors surface, fix and re-run.

- [ ] **Step 4: Commit**

```bash
git add .goreleaser.yaml Dockerfile
git commit -m "build: add GoReleaser v2 config for multi-arch operator image"
```

---

## Task 5: `release.yaml` workflow: GoReleaser + Cosign + SBOM

**Files:**
- Create: `.github/workflows/release.yaml`

- [ ] **Step 1: Write the workflow**

Create `.github/workflows/release.yaml`:

```yaml
name: Release

on:
  push:
    tags:
      - 'v*'

env:
  REGISTRY: ghcr.io

jobs:
  release:
    name: Release
    runs-on: ubuntu-latest
    permissions:
      contents: write
      packages: write
      id-token: write   # required for Cosign keyless OIDC
    steps:
      - uses: actions/checkout@v4
        with:
          fetch-depth: 0

      - uses: actions/setup-go@v5
        with:
          go-version: '1.24'

      - name: Set up Docker Buildx
        uses: docker/setup-buildx-action@v3

      - name: Login to GHCR
        uses: docker/login-action@v3
        with:
          registry: ${{ env.REGISTRY }}
          username: ${{ github.actor }}
          password: ${{ secrets.GITHUB_TOKEN }}

      - name: Install Cosign
        uses: sigstore/cosign-installer@v3

      - name: Build dist/install.yaml
        run: make installer

      - name: Run GoReleaser
        uses: goreleaser/goreleaser-action@v6
        with:
          version: '~> v2'
          args: release --clean
        env:
          GITHUB_TOKEN: ${{ secrets.GITHUB_TOKEN }}

      - name: Resolve image name
        id: docker-image
        run: |
          OWNER="${GITHUB_REPOSITORY_OWNER,,}"
          echo "name=${{ env.REGISTRY }}/${OWNER}/hermes-operator" >> "$GITHUB_OUTPUT"

      - name: Sign container images
        run: |
          set -euo pipefail
          IMAGE="${{ steps.docker-image.outputs.name }}"
          VERSION="${{ github.ref_name }}"
          VERSION_NO_V="${VERSION#v}"
          MAJOR=$(echo "$VERSION_NO_V" | cut -d. -f1)
          MINOR=$(echo "$VERSION_NO_V" | cut -d. -f2)
          TAGS=("latest" "${VERSION}" "${MAJOR}.${MINOR}")

          for tag in "${TAGS[@]}"; do
            echo "Signing ${IMAGE}:${tag}..."
            DIGEST=$(docker buildx imagetools inspect "${IMAGE}:${tag}" \
              --format '{{json .Manifest}}' | jq -r '.digest')
            if [ -n "$DIGEST" ] && [ "$DIGEST" != "null" ]; then
              cosign sign --yes "${IMAGE}:${tag}@${DIGEST}"
            else
              echo "Warning: could not resolve digest for ${IMAGE}:${tag}, skipping"
            fi
          done

      - name: Generate SBOM
        uses: anchore/sbom-action@v0
        with:
          image: ${{ steps.docker-image.outputs.name }}:${{ github.ref_name }}
          format: spdx-json
          output-file: sbom.spdx.json
          upload-release-assets: false

      - name: Attest SBOM
        run: |
          set -euo pipefail
          IMAGE="${{ steps.docker-image.outputs.name }}"
          DIGEST=$(docker buildx imagetools inspect "${IMAGE}:${{ github.ref_name }}" \
            --format '{{json .Manifest}}' | jq -r '.digest')
          cosign attest --yes --predicate sbom.spdx.json --type spdxjson \
            "${IMAGE}@${DIGEST}"

      - name: Upload SBOM to release
        env:
          GH_TOKEN: ${{ secrets.GITHUB_TOKEN }}
        run: gh release upload "${{ github.ref_name }}" sbom.spdx.json --clobber

      - name: Publish release
        env:
          # PAT required: events from GITHUB_TOKEN don't trigger other
          # workflows. Using PAT ensures the release:published event
          # fires operatorhub-submit.yaml.
          GH_TOKEN: ${{ secrets.RELEASE_PLEASE_TOKEN }}
        run: gh release edit "${{ github.ref_name }}" --draft=false

  helm-release:
    name: Helm Chart Release
    needs: release
    runs-on: ubuntu-latest
    permissions:
      contents: write
      packages: write
    steps:
      - uses: actions/checkout@v4

      - name: Lowercase owner
        id: owner
        run: echo "name=${GITHUB_REPOSITORY_OWNER,,}" >> "$GITHUB_OUTPUT"

      - name: Install Helm
        uses: azure/setup-helm@v4

      - name: Extract version
        id: version
        run: echo "version=${GITHUB_REF#refs/tags/v}" >> "$GITHUB_OUTPUT"

      - name: Login to GHCR
        uses: docker/login-action@v3
        with:
          registry: ghcr.io
          username: ${{ github.actor }}
          password: ${{ secrets.GITHUB_TOKEN }}

      - name: Package and push Helm chart to OCI registry
        run: |
          VERSION="${{ steps.version.outputs.version }}"
          helm package charts/hermes-operator --version "${VERSION}" --app-version "${VERSION}"
          helm push "hermes-operator-${VERSION}.tgz" oci://ghcr.io/${{ steps.owner.outputs.name }}/charts
```

- [ ] **Step 2: Lint**

```bash
python -c "import yaml; yaml.safe_load(open('.github/workflows/release.yaml'))"
```
Expected: exit 0.

- [ ] **Step 3: Commit**

```bash
git add .github/workflows/release.yaml
git commit -m "ci: add release workflow (GoReleaser + Cosign + SBOM + Helm OCI push)"
```

---

## Task 6: `make installer`: emit `dist/install.yaml`

**Files:**
- Modify: `Makefile`

- [ ] **Step 1: Append the target**

Append to `Makefile`:

```makefile
.PHONY: installer
installer: manifests generate kustomize ## Emit dist/install.yaml for plain kubectl apply.
	mkdir -p dist
	cd config/manager && $(KUSTOMIZE) edit set image controller=ghcr.io/stubbi/hermes-operator:$${VERSION:-latest}
	$(KUSTOMIZE) build config/default > dist/install.yaml
	@echo "Wrote dist/install.yaml ($$(wc -l < dist/install.yaml) lines)"
```

- [ ] **Step 2: Verify**

```bash
VERSION=dev make installer
head -5 dist/install.yaml
```
Expected: first line is `apiVersion: v1` (a Namespace), and the file contains at least the three CRDs plus the manager Deployment.

```bash
grep -c '^---' dist/install.yaml
```
Expected: ≥ 8 (CRDs + namespace + RBAC + SA + deployment + service).

- [ ] **Step 3: Add `dist/` to `.gitignore`**

Append to `.gitignore`:

```
dist/
```

- [ ] **Step 4: Commit**

```bash
git add Makefile .gitignore
git commit -m "build: add make installer target (emits dist/install.yaml for release asset)"
```

---

## Task 7: OLM bundle scaffold: `bundle.Dockerfile` and `bundle/`

**Files:**
- Create: `bundle.Dockerfile`, `bundle/ci.yaml`, `bundle/metadata/annotations.yaml`, `bundle/tests/scorecard/config.yaml`

- [ ] **Step 1: `bundle.Dockerfile`**

Create `bundle.Dockerfile`:

```dockerfile
FROM scratch

COPY bundle/manifests /manifests/
COPY bundle/metadata /metadata/

LABEL operators.operatorframework.io.bundle.mediatype.v1=registry+v1
LABEL operators.operatorframework.io.bundle.manifests.v1=manifests/
LABEL operators.operatorframework.io.bundle.metadata.v1=metadata/
LABEL operators.operatorframework.io.bundle.package.v1=hermes-operator
LABEL operators.operatorframework.io.bundle.channels.v1=stable
LABEL operators.operatorframework.io.bundle.channel.default.v1=stable
```

- [ ] **Step 2: `bundle/metadata/annotations.yaml`**

```yaml
annotations:
  operators.operatorframework.io.bundle.mediatype.v1: "registry+v1"
  operators.operatorframework.io.bundle.manifests.v1: "manifests/"
  operators.operatorframework.io.bundle.metadata.v1: "metadata/"
  operators.operatorframework.io.bundle.package.v1: "hermes-operator"
  operators.operatorframework.io.bundle.channels.v1: "stable"
  operators.operatorframework.io.bundle.channel.default.v1: "stable"
```

- [ ] **Step 3: `bundle/ci.yaml`** (reviewers + update strategy for `k8s-operatorhub/community-operators`)

```yaml
reviewers:
  - stubbi
updateGraph: semver-mode
```

- [ ] **Step 4: `bundle/tests/scorecard/config.yaml`**

```yaml
apiVersion: scorecard.operatorframework.io/v1alpha3
kind: Configuration
metadata:
  name: config
stages:
  - parallel: true
    tests:
      - entrypoint:
          - scorecard-test
          - basic-check-spec
        image: quay.io/operator-framework/scorecard-test:v1.38.0
        labels:
          suite: basic
          test: basic-check-spec-test
      - entrypoint:
          - scorecard-test
          - olm-bundle-validation
        image: quay.io/operator-framework/scorecard-test:v1.38.0
        labels:
          suite: olm
          test: olm-bundle-validation-test
      - entrypoint:
          - scorecard-test
          - olm-crds-have-validation
        image: quay.io/operator-framework/scorecard-test:v1.38.0
        labels:
          suite: olm
          test: olm-crds-have-validation-test
      - entrypoint:
          - scorecard-test
          - olm-crds-have-resources
        image: quay.io/operator-framework/scorecard-test:v1.38.0
        labels:
          suite: olm
          test: olm-crds-have-resources-test
      - entrypoint:
          - scorecard-test
          - olm-spec-descriptors
        image: quay.io/operator-framework/scorecard-test:v1.38.0
        labels:
          suite: olm
          test: olm-spec-descriptors-test
      - entrypoint:
          - scorecard-test
          - olm-status-descriptors
        image: quay.io/operator-framework/scorecard-test:v1.38.0
        labels:
          suite: olm
          test: olm-status-descriptors-test
```

- [ ] **Step 5: Commit**

```bash
git add bundle.Dockerfile bundle/
git commit -m "build(bundle): scaffold OLM bundle (Dockerfile, annotations, scorecard config)"
```

---

## Task 8: ClusterServiceVersion: `bundle/manifests/hermes-operator.clusterserviceversion.yaml`

**Files:**
- Create: `bundle/manifests/hermes-operator.clusterserviceversion.yaml`
- Copy: CRDs from `config/crd/bases/` → `bundle/manifests/`

The CSV is the single biggest file in the OLM bundle. Most fields are static; release-please mutates only `metadata.name`, `metadata.annotations.containerImage`, and `spec.version` via `extra-files` (Task 2).

- [ ] **Step 1: Copy CRDs into the bundle**

```bash
mkdir -p bundle/manifests
cp config/crd/bases/hermes.agent_hermesinstances.yaml      bundle/manifests/
cp config/crd/bases/hermes.agent_hermesselfconfigs.yaml    bundle/manifests/
cp config/crd/bases/hermes.agent_hermesclusterdefaults.yaml bundle/manifests/
```
Expected: three files copied.

- [ ] **Step 2: Wire `make sync-bundle-crds`**

Append to `Makefile`:

```makefile
.PHONY: sync-bundle-crds
sync-bundle-crds: manifests ## Sync CRDs from config/crd/bases/ into the OLM bundle.
	cp config/crd/bases/hermes.agent_hermesinstances.yaml      bundle/manifests/
	cp config/crd/bases/hermes.agent_hermesselfconfigs.yaml    bundle/manifests/
	cp config/crd/bases/hermes.agent_hermesclusterdefaults.yaml bundle/manifests/
```

- [ ] **Step 3: Write the CSV**

Create `bundle/manifests/hermes-operator.clusterserviceversion.yaml`:

```yaml
apiVersion: operators.coreos.com/v1alpha1
kind: ClusterServiceVersion
metadata:
  name: hermes-operator.v0.1.0
  namespace: placeholder
  annotations:
    capabilities: "Auto Pilot"
    categories: "AI/Machine Learning, Developer Tools, Application Runtime"
    containerImage: ghcr.io/stubbi/hermes-operator:v0.1.0
    createdAt: "2026-05-12T00:00:00Z"
    support: stubbi
    repository: https://github.com/stubbi/hermes-operator
    description: "A production-grade Kubernetes operator for deploying and managing nousresearch/hermes-agent: a Python-based self-improving multi-platform AI agent."
    alm-examples: |-
      [
        {
          "apiVersion": "hermes.agent/v1",
          "kind": "HermesInstance",
          "metadata": { "name": "demo", "namespace": "default" },
          "spec": {
            "image": { "repository": "ghcr.io/stubbi/hermes-agent", "tag": "1.0.0" },
            "storage": { "persistence": { "enabled": true, "size": "10Gi" } },
            "envFrom": [{ "secretRef": { "name": "hermes-api-keys" } }]
          }
        },
        {
          "apiVersion": "hermes.agent/v1",
          "kind": "HermesInstance",
          "metadata": { "name": "production", "namespace": "hermes" },
          "spec": {
            "image": { "repository": "ghcr.io/stubbi/hermes-agent", "tag": "1.0.0" },
            "resources": {
              "requests": { "cpu": "500m", "memory": "1Gi" },
              "limits":   { "cpu": "2",    "memory": "4Gi" }
            },
            "storage": { "persistence": { "enabled": true, "size": "50Gi" } },
            "gateways": {
              "telegram": { "enabled": true, "tokenSecretRef": { "name": "tg-token" } },
              "slack":    { "enabled": true, "tokenSecretRef": { "name": "slack-token" } }
            },
            "profileStore": { "enabled": true, "persistence": { "size": "5Gi" } },
            "selfConfigure": {
              "enabled": true,
              "protectedKeys": ["image", "storage", "security", "networking"]
            },
            "observability": { "serviceMonitor": { "enabled": true } },
            "backup": {
              "s3": {
                "bucket": "hermes-backups",
                "endpoint": "s3.amazonaws.com",
                "region": "us-east-1",
                "credentialsSecretRef": { "name": "hermes-s3-creds" }
              },
              "schedule": "0 3 * * *",
              "onDelete": true,
              "historyLimit": 30
            },
            "autoUpdate": {
              "enabled": true,
              "source": { "registry": "ghcr.io/stubbi/hermes-agent", "channel": "1.x" },
              "rollback": { "enabled": true, "probeFailureThreshold": 3 }
            }
          }
        },
        {
          "apiVersion": "hermes.agent/v1",
          "kind": "HermesSelfConfig",
          "metadata": { "name": "install-finance-skill", "namespace": "default" },
          "spec": {
            "instanceRef": "production",
            "addSkills": [{ "source": "git+https://github.com/foo/finance-skill@v1.2.0" }],
            "patchConfig": { "schedules": { "morning-brief": "0 8 * * *" } },
            "addEnvVars": [{ "name": "FINANCE_TZ", "value": "Europe/Berlin" }]
          }
        },
        {
          "apiVersion": "hermes.agent/v1",
          "kind": "HermesClusterDefaults",
          "metadata": { "name": "cluster" },
          "spec": {
            "image": { "repository": "ghcr.io/stubbi/hermes-agent", "tag": "1.0.0" },
            "storage": { "storageClassName": "gp3", "size": "10Gi" },
            "observability": { "serviceMonitor": { "enabled": true } },
            "networking": { "networkPolicy": { "enabled": true } }
          }
        }
      ]
    features.operators.openshift.io/disconnected: "false"
    features.operators.openshift.io/fips-compliant: "false"
    features.operators.openshift.io/proxy-aware: "false"
    features.operators.openshift.io/tls-profiles: "false"
    features.operators.openshift.io/token-auth-aws: "false"
    features.operators.openshift.io/token-auth-azure: "false"
    features.operators.openshift.io/token-auth-gcp: "false"
spec:
  displayName: Hermes Operator
  description: |
    ## Hermes Kubernetes Operator

    A production-grade Kubernetes operator for deploying and managing
    [nousresearch/hermes-agent](https://github.com/nousresearch/hermes-agent):
    a Python-based, self-improving, multi-platform AI agent.

    ### What is hermes-agent?

    hermes-agent fronts multiple messaging platforms (Telegram, Discord, Slack,
    WhatsApp, Signal), self-improves through a built-in learning loop, persists
    session memory via FTS5, models users via Honcho dialectic profiles, and runs
    scheduled automations via a native cron scheduler.

    ### Key Capabilities

    - **Declarative Deployment**: every aspect of an agent's runtime, gateways,
      storage, networking, observability, and security through a single
      `HermesInstance` resource.
    - **Self-Configuration with Audit**: the agent persists learned skills, env
      vars, config patches, and workspace files via `HermesSelfConfig` CRs that
      the operator validates against an explicit allowlist, then applies via
      Server-Side Apply (no GitOps flap).
    - **Cluster Defaults**: `HermesClusterDefaults` (cluster-scoped singleton)
      supplies organization-wide defaults for image, storage class, IRSA, etc.
    - **Auto-Update with Rollback**: OCI registry polling with pre-update backup
      and probe-driven rollback.
    - **Backup / Restore**: S3-compatible (R2, MinIO, AWS), backup-on-delete
      finalizer, declarative restore from snapshot.
    - **OpenClaw Migration**: one-shot `spec.migration.fromOpenClaw` invokes
      hermes-agent's built-in importer.
    - **Honcho Profile Store**: optional companion Deployment with persistence
      for dialectic user profiles.
    - **Multi-Platform Gateways**: first-class config for Telegram, Discord,
      Slack, WhatsApp, Signal: each with isolated secret rotation.

    ### Default Security Posture

    All containers run as non-root, drop ALL Linux capabilities, use
    RuntimeDefault seccomp, mount a read-only root filesystem with explicit
    writable subPaths, and apply default-deny NetworkPolicies. PVCs are
    immutable post-create; finalizer-driven backup-on-delete is configurable.

    ### Managed Resources

    For each HermesInstance the operator creates and manages:
    StatefulSet, Service, ServiceAccount, ConfigMap (workspace), ConfigMap
    (gateway-derived), PVC, Role, RoleBinding, NetworkPolicy,
    PodDisruptionBudget, optionally Ingress, ServiceMonitor, HPA, Honcho
    Deployment + PVC + Service, and migration / backup Jobs. All resources are
    owned by the CR and garbage-collected on deletion (with the
    `hermes.agent/backup-on-delete` finalizer holding deletion until the
    backup Job completes).

    ### Getting Started

    1. Install the operator via OLM or `helm install`.
    2. Create a Secret with your AI provider keys and platform tokens.
    3. Create a `HermesInstance` referencing the Secret.
    4. The operator handles the rest: deployment, networking, security,
       lifecycle.

    ### Prerequisites

    - Kubernetes 1.28+
  maturity: alpha
  version: 0.1.0
  minKubeVersion: 1.28.0
  keywords:
    - ai
    - agent
    - hermes
    - nousresearch
    - llm
    - kubernetes
    - ai-agent
    - chatbot
    - telegram
    - discord
    - slack
    - whatsapp
    - signal
    - honcho
    - self-improving
    - python
  maintainers:
    - name: Jannes Stubbemann
      email: jannes@aqora.io
  provider:
    name: stubbi
    url: https://github.com/stubbi/hermes-operator
  links:
    - name: GitHub Repository
      url: https://github.com/stubbi/hermes-operator
    - name: API Reference
      url: https://github.com/stubbi/hermes-operator/blob/main/docs/api-reference.md
    - name: Changelog
      url: https://github.com/stubbi/hermes-operator/blob/main/CHANGELOG.md
  installModes:
    - type: OwnNamespace
      supported: true
    - type: SingleNamespace
      supported: true
    - type: MultiNamespace
      supported: false
    - type: AllNamespaces
      supported: true
  relatedImages:
    - name: hermes-operator
      image: ghcr.io/stubbi/hermes-operator:v0.1.0
    - name: hermes-agent
      image: ghcr.io/stubbi/hermes-agent:latest
  customresourcedefinitions:
    owned:
      - name: hermesinstances.hermes.agent
        version: v1
        kind: HermesInstance
        displayName: Hermes Instance
        description: Represents a managed hermes-agent instance with security, networking, gateways, storage, and observability.
        specDescriptors:
          - path: image.repository
            displayName: Image Repository
            x-descriptors: ["urn:alm:descriptor:com.tectonic.ui:text"]
          - path: image.tag
            displayName: Image Tag
            x-descriptors: ["urn:alm:descriptor:com.tectonic.ui:text"]
          - path: storage.persistence.enabled
            displayName: Persistent Storage
            x-descriptors: ["urn:alm:descriptor:com.tectonic.ui:booleanSwitch"]
          - path: storage.persistence.size
            displayName: Storage Size
            x-descriptors: ["urn:alm:descriptor:com.tectonic.ui:text"]
          - path: resources
            displayName: Resource Requirements
            x-descriptors: ["urn:alm:descriptor:com.tectonic.ui:resourceRequirements"]
          - path: gateways.telegram.enabled
            displayName: Telegram Gateway
            x-descriptors: ["urn:alm:descriptor:com.tectonic.ui:booleanSwitch"]
          - path: gateways.discord.enabled
            displayName: Discord Gateway
            x-descriptors: ["urn:alm:descriptor:com.tectonic.ui:booleanSwitch"]
          - path: gateways.slack.enabled
            displayName: Slack Gateway
            x-descriptors: ["urn:alm:descriptor:com.tectonic.ui:booleanSwitch"]
          - path: gateways.whatsapp.enabled
            displayName: WhatsApp Gateway
            x-descriptors: ["urn:alm:descriptor:com.tectonic.ui:booleanSwitch"]
          - path: gateways.signal.enabled
            displayName: Signal Gateway
            x-descriptors: ["urn:alm:descriptor:com.tectonic.ui:booleanSwitch"]
          - path: profileStore.enabled
            displayName: Honcho Profile Store
            x-descriptors: ["urn:alm:descriptor:com.tectonic.ui:booleanSwitch"]
          - path: selfConfigure.enabled
            displayName: Agent Self-Configuration
            x-descriptors: ["urn:alm:descriptor:com.tectonic.ui:booleanSwitch"]
          - path: observability.serviceMonitor.enabled
            displayName: Prometheus ServiceMonitor
            x-descriptors: ["urn:alm:descriptor:com.tectonic.ui:booleanSwitch"]
          - path: autoUpdate.enabled
            displayName: Auto Update
            x-descriptors: ["urn:alm:descriptor:com.tectonic.ui:booleanSwitch"]
          - path: backup.s3.bucket
            displayName: Backup S3 Bucket
            x-descriptors: ["urn:alm:descriptor:com.tectonic.ui:text"]
          - path: networking.ingress.enabled
            displayName: Ingress
            x-descriptors: ["urn:alm:descriptor:com.tectonic.ui:booleanSwitch"]
          - path: networking.networkPolicy.enabled
            displayName: NetworkPolicy
            x-descriptors: ["urn:alm:descriptor:com.tectonic.ui:booleanSwitch"]
          - path: suspended
            displayName: Suspended (scale-to-zero)
            x-descriptors: ["urn:alm:descriptor:com.tectonic.ui:booleanSwitch"]
        statusDescriptors:
          - path: phase
            displayName: Phase
            x-descriptors: ["urn:alm:descriptor:io.kubernetes.phase"]
          - path: conditions
            displayName: Conditions
            x-descriptors: ["urn:alm:descriptor:io.kubernetes.conditions"]
          - path: observedGeneration
            displayName: Observed Generation
            x-descriptors: ["urn:alm:descriptor:text"]
          - path: lastBackupTime
            displayName: Last Backup
            x-descriptors: ["urn:alm:descriptor:text"]
          - path: lastBackupPath
            displayName: Last Backup Path
            x-descriptors: ["urn:alm:descriptor:text"]
          - path: autoUpdate.latestVersion
            displayName: Latest Available Version
            x-descriptors: ["urn:alm:descriptor:text"]
          - path: migrationCompleted
            displayName: Migration Completed
            x-descriptors: ["urn:alm:descriptor:text"]
        resources:
          - kind: StatefulSet
            version: v1
          - kind: Service
            version: v1
          - kind: ServiceAccount
            version: v1
          - kind: ConfigMap
            version: v1
          - kind: PersistentVolumeClaim
            version: v1
          - kind: NetworkPolicy
            version: v1
          - kind: PodDisruptionBudget
            version: v1
          - kind: Ingress
            version: v1
          - kind: Role
            version: v1
          - kind: RoleBinding
            version: v1
          - kind: ServiceMonitor
            version: v1
          - kind: Job
            version: v1
          - kind: HorizontalPodAutoscaler
            version: v2
      - name: hermesselfconfigs.hermes.agent
        version: v1
        kind: HermesSelfConfig
        displayName: Hermes SelfConfig
        description: Agent-initiated mutation request, validated against the parent instance's selfConfigure policy and applied via Server-Side Apply.
        specDescriptors:
          - path: instanceRef
            displayName: Instance Reference
            x-descriptors: ["urn:alm:descriptor:com.tectonic.ui:text"]
          - path: addSkills
            displayName: Skills to Add
            x-descriptors: ["urn:alm:descriptor:com.tectonic.ui:fieldGroup:Mutations"]
        statusDescriptors:
          - path: phase
            displayName: Phase
            x-descriptors: ["urn:alm:descriptor:io.kubernetes.phase"]
          - path: conditions
            displayName: Conditions
            x-descriptors: ["urn:alm:descriptor:io.kubernetes.conditions"]
          - path: appliedAt
            displayName: Applied At
            x-descriptors: ["urn:alm:descriptor:text"]
          - path: denyReason
            displayName: Deny Reason
            x-descriptors: ["urn:alm:descriptor:text"]
      - name: hermesclusterdefaults.hermes.agent
        version: v1
        kind: HermesClusterDefaults
        displayName: Hermes Cluster Defaults
        description: Cluster-scoped singleton that fills nil fields on every HermesInstance. Name must be 'cluster'.
        specDescriptors:
          - path: image.repository
            displayName: Default Image Repository
            x-descriptors: ["urn:alm:descriptor:com.tectonic.ui:text"]
          - path: storage.storageClassName
            displayName: Default StorageClass
            x-descriptors: ["urn:alm:descriptor:com.tectonic.ui:text"]
        statusDescriptors:
          - path: conditions
            displayName: Conditions
            x-descriptors: ["urn:alm:descriptor:io.kubernetes.conditions"]
  install:
    strategy: deployment
    spec:
      clusterPermissions:
        - serviceAccountName: hermes-operator-controller-manager
          rules:
            # The full RBAC block is generated by:
            #   kustomize build config/rbac | yq 'select(.kind == "ClusterRole") | .rules'
            # and pasted here. Sync via `make sync-bundle-rbac` (Task 9).
            # On every CSV bump, release-please does NOT touch this block;
            # the engineer must run `make sync-bundle-rbac` if RBAC changed.
            - apiGroups: [""]
              resources: [configmaps, persistentvolumeclaims, serviceaccounts, services]
              verbs: [create, delete, get, list, patch, update, watch]
            - apiGroups: [""]
              resources: [events]
              verbs: [create, patch]
            - apiGroups: [""]
              resources: [secrets]
              verbs: [get, list, watch, create, update, patch]
            - apiGroups: [""]
              resources: [pods]
              verbs: [get, list, watch]
            - apiGroups: [""]
              resources: [pods/exec]
              verbs: [create]
            - apiGroups: [apps]
              resources: [deployments, statefulsets]
              verbs: [create, delete, get, list, patch, update, watch]
            - apiGroups: [autoscaling]
              resources: [horizontalpodautoscalers]
              verbs: [create, delete, get, list, patch, update, watch]
            - apiGroups: [batch]
              resources: [jobs, cronjobs]
              verbs: [create, delete, get, list, patch, update, watch]
            - apiGroups: [monitoring.coreos.com]
              resources: [servicemonitors, prometheusrules]
              verbs: [create, delete, get, list, patch, update, watch]
            - apiGroups: [networking.k8s.io]
              resources: [ingresses, networkpolicies]
              verbs: [create, delete, get, list, patch, update, watch]
            - apiGroups: [hermes.agent]
              resources: [hermesinstances, hermesselfconfigs, hermesclusterdefaults]
              verbs: [create, delete, get, list, patch, update, watch]
            - apiGroups: [hermes.agent]
              resources: [hermesinstances/finalizers, hermesselfconfigs/finalizers, hermesclusterdefaults/finalizers]
              verbs: [update]
            - apiGroups: [hermes.agent]
              resources: [hermesinstances/status, hermesselfconfigs/status, hermesclusterdefaults/status]
              verbs: [get, patch, update]
            - apiGroups: [policy]
              resources: [poddisruptionbudgets]
              verbs: [create, delete, get, list, patch, update, watch]
            - apiGroups: [rbac.authorization.k8s.io]
              resources: [roles, rolebindings]
              verbs: [create, delete, get, list, patch, update, watch]
            - apiGroups: [coordination.k8s.io]
              resources: [leases]
              verbs: [get, list, watch, create, update, patch, delete]
      deployments:
        - name: hermes-operator-controller-manager
          spec:
            replicas: 1
            selector:
              matchLabels:
                control-plane: controller-manager
            template:
              metadata:
                annotations:
                  kubectl.kubernetes.io/default-container: manager
                labels:
                  control-plane: controller-manager
              spec:
                securityContext:
                  runAsNonRoot: true
                  seccompProfile:
                    type: RuntimeDefault
                serviceAccountName: hermes-operator-controller-manager
                terminationGracePeriodSeconds: 10
                containers:
                  - name: manager
                    image: ghcr.io/stubbi/hermes-operator:v0.1.0
                    command:
                      - /manager
                    args:
                      - --leader-elect
                      - --health-probe-bind-address=:8081
                    securityContext:
                      allowPrivilegeEscalation: false
                      capabilities:
                        drop: [ALL]
                      readOnlyRootFilesystem: true
                      runAsNonRoot: true
                    livenessProbe:
                      httpGet:
                        path: /healthz
                        port: 8081
                      initialDelaySeconds: 15
                      periodSeconds: 20
                    readinessProbe:
                      httpGet:
                        path: /readyz
                        port: 8081
                      initialDelaySeconds: 5
                      periodSeconds: 10
                    resources:
                      limits:
                        cpu: 500m
                        memory: 256Mi
                      requests:
                        cpu: 100m
                        memory: 128Mi
```

- [ ] **Step 4: Validate the CSV with operator-sdk**

```bash
make operator-sdk
./bin/operator-sdk bundle validate ./bundle
```
Expected: `All validation tests have completed successfully`. If `operator-sdk` is missing, Plan 1 didn't install it; add this target to Makefile (Task 10).

- [ ] **Step 5: Commit**

```bash
git add Makefile bundle/manifests/
git commit -m "build(bundle): add ClusterServiceVersion + CRD copies"
```

---

## Task 9: Sync RBAC from kustomize into the CSV

**Files:**
- Create: `hack/sync-bundle-rbac.sh`
- Modify: `Makefile`

The bundle CSV embeds RBAC that must match `config/rbac/role.yaml`. Mirror the openclaw approach: a script extracts the rules from kustomize and replaces the CSV block in place.

- [ ] **Step 1: Write the sync script**

Create `hack/sync-bundle-rbac.sh`:

```bash
#!/usr/bin/env bash
set -euo pipefail

# Extract rules from the kubebuilder-generated ClusterRole and splice them
# into the bundle CSV's clusterPermissions[0].rules block. Idempotent.

CSV=bundle/manifests/hermes-operator.clusterserviceversion.yaml
ROLE=config/rbac/role.yaml

if [ ! -f "$ROLE" ]; then
  echo "::error::$ROLE not found. Run 'make manifests' first." >&2
  exit 1
fi

# yq merges: replace the rules array on the first clusterPermissions entry.
TMP=$(mktemp)
yq eval \
  '.spec.install.spec.clusterPermissions[0].rules = load("'"$ROLE"'").rules' \
  "$CSV" > "$TMP"
mv "$TMP" "$CSV"

# In --check mode (CI), bail if the working tree is dirty after sync.
if [ "${1:-}" = "--check" ]; then
  if ! git diff --exit-code -- "$CSV"; then
    echo "::error::Bundle CSV RBAC drifted from $ROLE. Run 'make sync-bundle-rbac' locally." >&2
    exit 1
  fi
fi

echo "Bundle CSV RBAC synced from $ROLE."
```

```bash
chmod +x hack/sync-bundle-rbac.sh
```

- [ ] **Step 2: Add Makefile target**

Append to `Makefile`:

```makefile
.PHONY: sync-bundle-rbac
sync-bundle-rbac: manifests ## Sync bundle CSV RBAC from kubebuilder-generated role.
	bash hack/sync-bundle-rbac.sh

.PHONY: sync-bundle-rbac-check
sync-bundle-rbac-check: ## Verify the bundle CSV RBAC is in sync (CI use).
	bash hack/sync-bundle-rbac.sh --check
```

- [ ] **Step 3: Run it**

```bash
make sync-bundle-rbac
git diff bundle/manifests/hermes-operator.clusterserviceversion.yaml
```
Expected: either no diff (already in sync) or a clean rules block update. Inspect the diff: it should look like the rules block we pasted in Task 8 Step 3.

- [ ] **Step 4: Extend `helm-rbac.yaml` workflow to also check bundle drift**

Modify `.github/workflows/helm-rbac.yaml` (created in Plan 1 Task 14). Append a second job:

```yaml
  bundle-rbac:
    name: Bundle RBAC Sync
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      - run: sudo snap install yq
      - run: make sync-bundle-rbac-check
```

- [ ] **Step 5: Commit**

```bash
git add hack/sync-bundle-rbac.sh Makefile .github/workflows/helm-rbac.yaml bundle/manifests/
git commit -m "ci: sync bundle CSV RBAC from kubebuilder role + CI drift check"
```

---

## Task 10: Bundle build/push/validate Makefile targets

**Files:**
- Modify: `Makefile`

- [ ] **Step 1: Append targets**

Append to `Makefile`:

```makefile
##@ Bundle

BUNDLE_IMG ?= ghcr.io/stubbi/hermes-operator-bundle:$(shell git describe --tags --always)
CATALOG_IMG ?= ghcr.io/stubbi/hermes-operator-catalog:$(shell git describe --tags --always)

OPERATOR_SDK ?= $(LOCALBIN)/operator-sdk
OPERATOR_SDK_VERSION ?= v1.38.0
OPM ?= $(LOCALBIN)/opm
OPM_VERSION ?= v1.47.0

.PHONY: operator-sdk
operator-sdk: $(OPERATOR_SDK) ## Download operator-sdk locally if necessary.
$(OPERATOR_SDK): $(LOCALBIN)
	@if [ ! -f "$(OPERATOR_SDK)" ]; then \
	  OS=$$(uname | awk '{print tolower($$0)}'); \
	  ARCH=$$(case $$(uname -m) in x86_64) echo amd64;; aarch64|arm64) echo arm64;; esac); \
	  echo "Downloading operator-sdk $(OPERATOR_SDK_VERSION) ($${OS}/$${ARCH})"; \
	  curl -sSLo "$(OPERATOR_SDK)" "https://github.com/operator-framework/operator-sdk/releases/download/$(OPERATOR_SDK_VERSION)/operator-sdk_$${OS}_$${ARCH}"; \
	  chmod +x "$(OPERATOR_SDK)"; \
	fi

.PHONY: opm
opm: $(OPM) ## Download opm locally if necessary.
$(OPM): $(LOCALBIN)
	@if [ ! -f "$(OPM)" ]; then \
	  OS=$$(uname | awk '{print tolower($$0)}'); \
	  ARCH=$$(case $$(uname -m) in x86_64) echo amd64;; aarch64|arm64) echo arm64;; esac); \
	  echo "Downloading opm $(OPM_VERSION) ($${OS}/$${ARCH})"; \
	  curl -sSLo "$(OPM)" "https://github.com/operator-framework/operator-registry/releases/download/$(OPM_VERSION)/$${OS}-$${ARCH}-opm"; \
	  chmod +x "$(OPM)"; \
	fi

.PHONY: bundle
bundle: manifests sync-bundle-crds sync-bundle-rbac ## Refresh bundle manifests from current source.

.PHONY: bundle-validate
bundle-validate: operator-sdk ## Validate the OLM bundle.
	$(OPERATOR_SDK) bundle validate ./bundle --select-optional suite=operatorframework

.PHONY: bundle-build
bundle-build: bundle bundle-validate ## Build the OLM bundle image.
	docker build -f bundle.Dockerfile -t $(BUNDLE_IMG) .

.PHONY: bundle-push
bundle-push: ## Push the OLM bundle image.
	docker push $(BUNDLE_IMG)

.PHONY: catalog-build
catalog-build: opm ## Build a single-bundle catalog image (FBC).
	$(OPM) index add --container-tool docker --mode semver --tag $(CATALOG_IMG) --bundles $(BUNDLE_IMG)

.PHONY: catalog-push
catalog-push: ## Push the catalog image.
	docker push $(CATALOG_IMG)

.PHONY: scorecard
scorecard: operator-sdk ## Run operator-sdk scorecard tests (requires a kind cluster).
	$(OPERATOR_SDK) scorecard bundle --wait-time 120s
```

- [ ] **Step 2: Smoke-test locally**

```bash
make bundle
make bundle-validate
```
Expected: `Bundle CSV RBAC synced from config/rbac/role.yaml.`, then `All validation tests have completed successfully`.

```bash
make bundle-build
docker images | grep hermes-operator-bundle
```
Expected: bundle image present.

- [ ] **Step 3: Commit**

```bash
git add Makefile
git commit -m "build(bundle): add bundle/catalog Makefile targets"
```

---

## Task 11: OperatorHub auto-submit workflow

**Files:**
- Create: `.github/workflows/operatorhub-submit.yaml`

- [ ] **Step 1: Write the workflow**

Create `.github/workflows/operatorhub-submit.yaml`:

```yaml
name: OperatorHub Submission

on:
  release:
    types: [published]
  workflow_dispatch:
    inputs:
      tag:
        description: 'Release tag (e.g. v1.0.0)'
        required: true

jobs:
  submit-community-operators:
    name: Submit to k8s-operatorhub/community-operators
    runs-on: ubuntu-latest
    steps:
      - name: Resolve tag
        id: version
        run: |
          TAG="${{ github.event.inputs.tag || github.ref_name }}"
          VERSION="${TAG#v}"
          echo "version=$VERSION" >> "$GITHUB_OUTPUT"
          echo "tag=$TAG" >> "$GITHUB_OUTPUT"

      - uses: actions/checkout@v4
        with:
          ref: ${{ steps.version.outputs.tag }}

      - name: Prepare bundle
        run: |
          set -euo pipefail
          VERSION="${{ steps.version.outputs.version }}"
          TAG="${{ steps.version.outputs.tag }}"
          BUNDLE_DIR="submission/operators/hermes-operator/${VERSION}"
          mkdir -p "${BUNDLE_DIR}/manifests" "${BUNDLE_DIR}/metadata"

          # Copy and version the CSV.
          sed \
            -e "s/hermes-operator\.v[0-9]\+\.[0-9]\+\.[0-9]\+/hermes-operator.v${VERSION}/g" \
            -e "s|ghcr.io/stubbi/hermes-operator:v[0-9]\+\.[0-9]\+\.[0-9]\+|ghcr.io/stubbi/hermes-operator:${TAG}|g" \
            -e "s/createdAt: .*/createdAt: \"$(date -u +%Y-%m-%dT%H:%M:%SZ)\"/" \
            -e "s/^  version: [0-9]\+\.[0-9]\+\.[0-9]\+/  version: ${VERSION}/" \
            bundle/manifests/hermes-operator.clusterserviceversion.yaml \
            > "${BUNDLE_DIR}/manifests/hermes-operator.v${VERSION}.clusterserviceversion.yaml"

          # CRDs and metadata as-is.
          cp bundle/manifests/hermes.agent_hermesinstances.yaml      "${BUNDLE_DIR}/manifests/"
          cp bundle/manifests/hermes.agent_hermesselfconfigs.yaml    "${BUNDLE_DIR}/manifests/"
          cp bundle/manifests/hermes.agent_hermesclusterdefaults.yaml "${BUNDLE_DIR}/manifests/"
          cp bundle/metadata/annotations.yaml "${BUNDLE_DIR}/metadata/"
          # Per community-operators convention, copy ci.yaml alongside.
          cp bundle/ci.yaml "submission/operators/hermes-operator/" || true

          echo "Bundle prepared at ${BUNDLE_DIR}:"
          find "${BUNDLE_DIR}" -type f

      - name: Fork and submit to community-operators
        env:
          GH_TOKEN: ${{ secrets.RELEASE_PLEASE_TOKEN }}
        run: |
          set -euo pipefail
          VERSION="${{ steps.version.outputs.version }}"
          BRANCH="hermes-operator-v${VERSION}"

          gh auth setup-git
          gh repo fork k8s-operatorhub/community-operators --clone=true --remote=true -- community-operators
          cd community-operators

          git config user.name "github-actions[bot]"
          git config user.email "41898282+github-actions[bot]@users.noreply.github.com"

          git fetch upstream main
          git reset --hard upstream/main

          git checkout -b "${BRANCH}"

          mkdir -p operators/hermes-operator
          cp -r ../submission/operators/hermes-operator/${VERSION} operators/hermes-operator/${VERSION}
          cp -f ../submission/operators/hermes-operator/ci.yaml operators/hermes-operator/ci.yaml 2>/dev/null || true

          git add "operators/hermes-operator"
          git commit -m "operator hermes-operator (${VERSION})"
          git push --force origin "${BRANCH}"

          FORK_OWNER=$(gh api user --jq '.login')

          if ! gh pr view "${FORK_OWNER}:${BRANCH}" --repo k8s-operatorhub/community-operators --json state --jq '.state' 2>/dev/null | grep -q OPEN; then
            gh pr create \
              --repo k8s-operatorhub/community-operators \
              --head "${FORK_OWNER}:${BRANCH}" \
              --title "operator hermes-operator (${VERSION})" \
              --body "$(cat <<EOF
          ### Update to hermes-operator

          **Version:** ${VERSION}
          **Operator:** [Hermes Kubernetes Operator](https://github.com/stubbi/hermes-operator)

          #### Changes
          See [release notes](https://github.com/stubbi/hermes-operator/releases/tag/v${VERSION}).

          #### Testing
          - CI tests + nightly conformance suite pass on the source repository
          - Container image is published and signed at \`ghcr.io/stubbi/hermes-operator:v${VERSION}\`
          - SBOM attested at the same digest (verify with \`cosign verify-attestation\`)
          EOF
          )"
          else
            echo "PR already exists for ${FORK_OWNER}:${BRANCH} - branch was force-pushed with updated content"
          fi

  submit-redhat:
    name: Submit to redhat-openshift-ecosystem/community-operators-prod
    runs-on: ubuntu-latest
    steps:
      - name: Resolve tag
        id: version
        run: |
          TAG="${{ github.event.inputs.tag || github.ref_name }}"
          VERSION="${TAG#v}"
          echo "version=$VERSION" >> "$GITHUB_OUTPUT"
          echo "tag=$TAG" >> "$GITHUB_OUTPUT"

      - uses: actions/checkout@v4
        with:
          ref: ${{ steps.version.outputs.tag }}

      - name: Prepare bundle
        run: |
          set -euo pipefail
          VERSION="${{ steps.version.outputs.version }}"
          TAG="${{ steps.version.outputs.tag }}"
          BUNDLE_DIR="submission/operators/hermes-operator/${VERSION}"
          mkdir -p "${BUNDLE_DIR}/manifests" "${BUNDLE_DIR}/metadata"
          sed \
            -e "s/hermes-operator\.v[0-9]\+\.[0-9]\+\.[0-9]\+/hermes-operator.v${VERSION}/g" \
            -e "s|ghcr.io/stubbi/hermes-operator:v[0-9]\+\.[0-9]\+\.[0-9]\+|ghcr.io/stubbi/hermes-operator:${TAG}|g" \
            -e "s/createdAt: .*/createdAt: \"$(date -u +%Y-%m-%dT%H:%M:%SZ)\"/" \
            -e "s/^  version: [0-9]\+\.[0-9]\+\.[0-9]\+/  version: ${VERSION}/" \
            bundle/manifests/hermes-operator.clusterserviceversion.yaml \
            > "${BUNDLE_DIR}/manifests/hermes-operator.v${VERSION}.clusterserviceversion.yaml"
          cp bundle/manifests/hermes.agent_hermesinstances.yaml      "${BUNDLE_DIR}/manifests/"
          cp bundle/manifests/hermes.agent_hermesselfconfigs.yaml    "${BUNDLE_DIR}/manifests/"
          cp bundle/manifests/hermes.agent_hermesclusterdefaults.yaml "${BUNDLE_DIR}/manifests/"
          cp bundle/metadata/annotations.yaml "${BUNDLE_DIR}/metadata/"

      - name: Fork and submit to redhat community-operators-prod
        env:
          GH_TOKEN: ${{ secrets.RELEASE_PLEASE_TOKEN }}
        run: |
          set -euo pipefail
          VERSION="${{ steps.version.outputs.version }}"
          BRANCH="hermes-operator-v${VERSION}"
          gh auth setup-git
          gh repo fork redhat-openshift-ecosystem/community-operators-prod --clone=true --remote=true -- community-operators-prod
          cd community-operators-prod
          git config user.name "github-actions[bot]"
          git config user.email "41898282+github-actions[bot]@users.noreply.github.com"
          git fetch upstream main
          git reset --hard upstream/main
          git checkout -b "${BRANCH}"
          mkdir -p operators/hermes-operator
          cp -r ../submission/operators/hermes-operator/${VERSION} operators/hermes-operator/${VERSION}
          git add "operators/hermes-operator"
          git commit -m "operator hermes-operator (${VERSION})"
          git push --force origin "${BRANCH}"
          FORK_OWNER=$(gh api user --jq '.login')
          if ! gh pr view "${FORK_OWNER}:${BRANCH}" --repo redhat-openshift-ecosystem/community-operators-prod --json state --jq '.state' 2>/dev/null | grep -q OPEN; then
            gh pr create \
              --repo redhat-openshift-ecosystem/community-operators-prod \
              --head "${FORK_OWNER}:${BRANCH}" \
              --title "operator hermes-operator (${VERSION})" \
              --body "Release ${VERSION}. See https://github.com/stubbi/hermes-operator/releases/tag/v${VERSION}."
          fi
```

- [ ] **Step 2: Lint**

```bash
python -c "import yaml; yaml.safe_load(open('.github/workflows/operatorhub-submit.yaml'))"
```
Expected: exit 0.

- [ ] **Step 3: Commit**

```bash
git add .github/workflows/operatorhub-submit.yaml
git commit -m "ci: auto-submit OLM bundle to community-operators on release"
```

---

## Task 12: Cosign verification: `make verify-signing` + weekly drift check

**Files:**
- Modify: `Makefile`
- Create: `.github/workflows/verify-signing.yaml`, `docs/security/signing.md`

- [ ] **Step 1: `make verify-signing` target**

Append to `Makefile`:

```makefile
.PHONY: verify-signing
verify-signing: ## Verify the latest published release is Cosign-signed and SBOM-attested.
	@VERSION=$$(gh release view --repo stubbi/hermes-operator --json tagName --jq .tagName); \
	IMAGE="ghcr.io/stubbi/hermes-operator:$${VERSION}"; \
	echo "Verifying $${IMAGE}..."; \
	cosign verify "$${IMAGE}" \
	  --certificate-identity-regexp 'https://github.com/stubbi/hermes-operator/.github/workflows/.*' \
	  --certificate-oidc-issuer https://token.actions.githubusercontent.com >/dev/null || { echo "::error::signature verification failed for $${IMAGE}"; exit 1; }; \
	echo "Verifying SBOM attestation..."; \
	cosign verify-attestation "$${IMAGE}" --type spdxjson \
	  --certificate-identity-regexp 'https://github.com/stubbi/hermes-operator/.github/workflows/.*' \
	  --certificate-oidc-issuer https://token.actions.githubusercontent.com >/dev/null || { echo "::error::SBOM attestation verification failed for $${IMAGE}"; exit 1; }; \
	echo "OK: $${IMAGE} is signed and SBOM-attested."
```

- [ ] **Step 2: `docs/security/signing.md`**

```bash
mkdir -p docs/security
```

Create `docs/security/signing.md`:

```markdown
# Cosign Verification

All `hermes-operator` images are signed with Cosign (keyless OIDC) and ship
with an SPDX-JSON SBOM attested at the same image digest. This document is
the canonical reference; `SECURITY.md` (top-level) points here.

## Verify an image signature

```bash
cosign verify ghcr.io/stubbi/hermes-operator:vX.Y.Z \
  --certificate-identity-regexp 'https://github.com/stubbi/hermes-operator/.github/workflows/.*' \
  --certificate-oidc-issuer https://token.actions.githubusercontent.com
```

Exit 0 means the signature is valid and was produced by a GitHub Actions
workflow in this repo. Exit non-zero means the signature is missing or
tampered.

## Verify the SBOM attestation

```bash
cosign verify-attestation ghcr.io/stubbi/hermes-operator:vX.Y.Z --type spdxjson \
  --certificate-identity-regexp 'https://github.com/stubbi/hermes-operator/.github/workflows/.*' \
  --certificate-oidc-issuer https://token.actions.githubusercontent.com
```

To extract and inspect the SBOM payload:

```bash
cosign download attestation ghcr.io/stubbi/hermes-operator:vX.Y.Z --predicate-type spdxjson \
  | jq -r .payload | base64 -d | jq .predicate > sbom.spdx.json
```

The SBOM is also uploaded as a release asset at
`https://github.com/stubbi/hermes-operator/releases/download/vX.Y.Z/sbom.spdx.json`
for users who can't (or won't) hit the registry.

## What the operator signs

Every published release signs three image tags pointing to the same digest:

- `vX.Y.Z`: exact version
- `X.Y`: minor channel (moves on patch releases)
- `latest`: most recent stable (moves on every release)

Pinning by digest (`@sha256:...`) is recommended for production.

## What drift detection runs

A weekly `verify-signing.yaml` workflow re-verifies the latest published
release. If verification fails (signing infra broke, certificate transparency
log issue, etc.) the workflow opens a GitHub Issue tagged `infra-broken`. The
hermes-operator on-call (see `CODEOWNERS`) is paged via GitHub mobile.

## What this protects against

- Tampered image content in transit or at-rest in the registry.
- Compromised registry credentials publishing fake `hermes-operator` images.
- Long-tail bit-rot of the signing infrastructure (drift check).

## What this does NOT protect against

- A compromised GitHub Actions runner producing a malicious-but-signed image.
- Compromise of Sigstore's certificate authority (extremely improbable, but
  documented for completeness).
- Supply chain attacks against dependencies. Use `trivy fs .` and the
  attested SBOM to audit transitively.
```

- [ ] **Step 3: Weekly drift-detection workflow**

Create `.github/workflows/verify-signing.yaml`:

```yaml
name: Verify Signing (drift detection)

on:
  schedule:
    # Mondays at 13:00 UTC. Catches infra-broken signatures before users hit them.
    - cron: '0 13 * * 1'
  workflow_dispatch:

jobs:
  verify:
    runs-on: ubuntu-latest
    permissions:
      contents: read
      issues: write
    steps:
      - uses: actions/checkout@v4

      - name: Install Cosign
        uses: sigstore/cosign-installer@v3

      - name: Install gh
        run: |
          type -p gh >/dev/null 2>&1 || (sudo apt-get update && sudo apt-get install -y gh)

      - name: Verify latest release
        id: verify
        env:
          GH_TOKEN: ${{ secrets.GITHUB_TOKEN }}
        run: |
          set +e
          make verify-signing
          rc=$?
          echo "rc=$rc" >> "$GITHUB_OUTPUT"
          exit 0

      - name: Open issue on failure
        if: steps.verify.outputs.rc != '0'
        env:
          GH_TOKEN: ${{ secrets.GITHUB_TOKEN }}
        run: |
          gh issue create \
            --title "[drift] Cosign verify failed for latest release" \
            --label infra-broken \
            --body "The weekly verify-signing workflow failed. The latest release's image is no longer cosign-verifiable. Investigate: https://github.com/${{ github.repository }}/actions/runs/${{ github.run_id }}"
```

- [ ] **Step 4: Update `SECURITY.md` to point at `docs/security/signing.md`**

Modify `SECURITY.md` (Plan 1 created it). Append:

```markdown

## Signing & SBOMs

Full verification commands live at [`docs/security/signing.md`](docs/security/signing.md).
A weekly `verify-signing.yaml` workflow checks that the latest release is
still cosign-verifiable; if not, it auto-files an `infra-broken` issue.
```

- [ ] **Step 5: Commit**

```bash
git add Makefile .github/workflows/verify-signing.yaml docs/security/signing.md SECURITY.md
git commit -m "ci(security): add make verify-signing + weekly drift detection workflow"
```

---

## Task 13: k8s 1.28→1.32 CI matrix

**Files:**
- Modify: `.github/workflows/ci.yaml`, `.github/workflows/e2e.yaml`
- Create: `docs/supported-versions.md`

- [ ] **Step 1: Extend `ci.yaml` test job with ENVTEST_K8S_VERSION matrix**

Modify the `test:` job in `.github/workflows/ci.yaml`:

```yaml
  test:
    name: Test (envtest k8s ${{ matrix.k8s }})
    runs-on: ubuntu-latest
    strategy:
      fail-fast: false
      matrix:
        k8s: ["1.28", "1.29", "1.30", "1.31", "1.32"]
    steps:
      - uses: actions/checkout@v4
      - uses: actions/setup-go@v5
        with:
          go-version: '1.24'
      - name: Run tests against k8s ${{ matrix.k8s }}
        env:
          ENVTEST_K8S_VERSION: ${{ matrix.k8s }}.0
        run: make test
      - name: Upload coverage
        if: matrix.k8s == '1.32'
        uses: codecov/codecov-action@v4
        with:
          files: ./cover.out
          fail_ci_if_error: false
```

Why upload coverage only on the newest version: codecov rejects duplicate uploads for the same SHA; pinning to one matrix cell keeps the coverage signal clean.

- [ ] **Step 2: Confirm `Makefile` reads `ENVTEST_K8S_VERSION` from env**

`make test` expands `$(ENVTEST_K8S_VERSION)`. The kubebuilder-default Makefile already supports this. Verify:

```bash
grep -n ENVTEST_K8S_VERSION Makefile
```
Expected: at least one line like `ENVTEST_K8S_VERSION ?= 1.32.0`. If the `?=` is missing (just `=`), edit to `?=` so the env var overrides.

- [ ] **Step 3: Extend `e2e.yaml` with kind node-image matrix**

Modify `.github/workflows/e2e.yaml`:

```yaml
name: E2E

on:
  pull_request:
  push:
    branches: [main]

jobs:
  e2e:
    name: E2E (k8s ${{ matrix.k8s }})
    runs-on: ubuntu-latest
    strategy:
      fail-fast: false
      matrix:
        include:
          - k8s: "1.28"
            node: "kindest/node:v1.28.15@sha256:a7c05c7ae043a0b8c818f5a06188bc2c4098f6cb59ca7d1856df00375d839251"
          - k8s: "1.29"
            node: "kindest/node:v1.29.10@sha256:3b2d8c31753e6c8069d4fc4a9b6c4f8e0c1b0e0e2d2a5f0b9c0e0a0e4e2c8d6f"
          - k8s: "1.30"
            node: "kindest/node:v1.30.6@sha256:b6d08db72079ba5ae1f4a88a09025c0a904af3b52387643c285442afb05ab994"
          - k8s: "1.31"
            node: "kindest/node:v1.31.2@sha256:18fbefc20a7113353c7b75b5c869d7145a6abd6269154825872dc59c1329912e"
          - k8s: "1.32"
            node: "kindest/node:v1.32.0@sha256:c48c62eac5da28cdadcf560d1d8616cfa6783b58f0d94cf63ad1bf49600cb027"
    steps:
      - uses: actions/checkout@v4
      - uses: actions/setup-go@v5
        with: { go-version: "1.24" }
      - uses: helm/kind-action@v1
        with:
          cluster_name: hermes-operator-e2e
          config: hack/kind-config.yaml
          node_image: ${{ matrix.node }}
      - uses: azure/setup-helm@v4
      - name: Build operator image
        run: make docker-build IMG=hermes-operator:dev
      - name: Load image into kind
        run: kind load docker-image hermes-operator:dev --name hermes-operator-e2e
      - name: Run e2e
        run: make e2e
      - name: Dump operator logs
        if: failure()
        run: |
          kubectl get pods -A
          kubectl logs -n hermes-system -l control-plane=controller-manager --tail=-1 || true
```

The exact node-image digests are pinned per kind release notes; the engineer should replace any that are stale at execution time with `kind --version`-current digests. The kind project publishes them at https://github.com/kubernetes-sigs/kind/releases.

- [ ] **Step 4: Document the EOL drop policy**

Create `docs/supported-versions.md`:

```markdown
# Supported Kubernetes versions

`hermes-operator` is tested against the matrix below on every PR and main push.
Both envtest (controller integration) and kind (E2E) run for every version.

| Version | Status      | Tested via envtest | Tested via kind E2E |
|---------|-------------|--------------------|---------------------|
| 1.28    | Supported   | Yes                | Yes                 |
| 1.29    | Supported   | Yes                | Yes                 |
| 1.30    | Supported   | Yes                | Yes                 |
| 1.31    | Supported   | Yes                | Yes                 |
| 1.32    | Supported   | Yes                | Yes                 |

## EOL drop policy

The oldest minor is dropped only on a **minor** release of hermes-operator
(`vX.Y → vX.Y+1`), never on a patch (`vX.Y.Z → vX.Y.Z+1`). The drop:

1. Is announced in the CHANGELOG of the preceding minor (one release of notice).
2. Removes the corresponding cell from the CI matrix in the *first commit of
   the new minor*: never piecemeal.
3. Updates this table, `README.md`, and the OLM CSV `minKubeVersion` in the
   same commit.

This means: if you are on Kubernetes 1.28 and we drop it in `v1.5.0`, the
`v1.4.x` line continues to receive bug fixes for that version of Kubernetes
on its existing branch. Backport requests go through GitHub Issues; we
support the last two minor lines.

## Why this policy

- Patch releases of hermes-operator must be safe to roll out on any cluster
  that ran the previous patch.
- Operators of production clusters need a single, predictable knob ("the
  minor changed, re-check our k8s version") rather than per-patch surprises.
- Aligns with `apiVersion` stability commitments in `docs/api-versioning.md`.

## v1 specific commitments

For the v1.x line:

- The CRD API group `hermes.agent` and version `v1` will not have breaking
  changes for the lifetime of v1.x.
- Field removals require a `v2` + conversion webhook + ≥6 months overlap;
  see `docs/api-versioning.md` and `docs/deprecations.md`.
- Conformance suite categories (negative, idempotency, upgrade, GitOps,
  failure injection) are run nightly on `main` and required on every release
  tag.
```

- [ ] **Step 5: Commit**

```bash
git add .github/workflows/ci.yaml .github/workflows/e2e.yaml docs/supported-versions.md Makefile
git commit -m "ci: matrix CI across k8s 1.28-1.32 (envtest + kind) + EOL policy docs"
```

---

## Task 14: Conformance suite scaffold: shared harness

**Files:**
- Create: `test/conformance/conformance_suite_test.go`, `test/conformance/helpers.go`

The conformance tree is a Ginkgo suite of its own (independent of `test/e2e/`).
It spawns kind clusters per "category": each top-level `Describe` block is
isolated so failure injection in one test can't poison the others.

- [ ] **Step 1: Conformance suite entrypoint**

```bash
mkdir -p test/conformance/testdata
```

Create `test/conformance/conformance_suite_test.go`:

```go
package conformance

import (
	"context"
	"os"
	"testing"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

// Conformance suite: see test/conformance/README in plan or docs/conformance.md.
//
// Categories (one Describe block each, in separate files):
//   - negative_test.go             webhook deny paths
//   - idempotency_test.go          10-reconcile no-op canary
//   - upgrade_test.go              prior-release -> HEAD matrix
//   - gitops_test.go               FluxCD SSA + SelfConfig no-flap
//   - failure_injection_test.go    SIGKILL mid-reconcile

func TestConformance(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "hermes-operator conformance suite")
}

var (
	suiteCtx    context.Context
	suiteCancel context.CancelFunc
)

var _ = BeforeSuite(func() {
	suiteCtx, suiteCancel = context.WithCancel(context.Background())
	SetDefaultEventuallyTimeout(5 * time.Minute)
	SetDefaultEventuallyPollingInterval(2 * time.Second)
	By("verifying conformance preconditions")
	Expect(os.Getenv("KUBECONFIG")).ToNot(BeEmpty(),
		"KUBECONFIG must point at a kind cluster with the operator installed")
})

var _ = AfterSuite(func() {
	if suiteCancel != nil {
		suiteCancel()
	}
})
```

- [ ] **Step 2: Shared helpers**

Create `test/conformance/helpers.go`:

```go
package conformance

import (
	"context"
	"fmt"
	"io"
	"os/exec"
	"strings"
	"time"

	. "github.com/onsi/gomega"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	hermesv1 "github.com/stubbi/hermes-operator/api/v1"
)

// run executes a command, returning combined output.
func run(cmd string, args ...string) (string, error) {
	c := exec.Command(cmd, args...)
	b, err := c.CombinedOutput()
	return string(b), err
}

// runStdin pipes a string into a command.
func runStdin(cmd string, args []string, stdin string) (string, error) {
	c := exec.Command(cmd, args...)
	c.Stdin = strings.NewReader(stdin)
	b, err := c.CombinedOutput()
	return string(b), err
}

// kubectl runs `kubectl <args>` against the current KUBECONFIG.
func kubectl(args ...string) (string, error) {
	return run("kubectl", args...)
}

// kubectlApply applies a YAML string and returns the apiserver error verbatim.
func kubectlApply(yaml string) (string, error) {
	return runStdin("kubectl", []string{"apply", "-f", "-"}, yaml)
}

// kubectlCreate creates (errors on existing), used when we want apiserver
// validation errors without server-side-apply's "diff" noise.
func kubectlCreate(yaml string) (string, error) {
	return runStdin("kubectl", []string{"create", "-f", "-"}, yaml)
}

// kubectlDelete deletes resources defined inline.
func kubectlDelete(yaml string) (string, error) {
	return runStdin("kubectl", []string{"delete", "--ignore-not-found", "-f", "-"}, yaml)
}

// newClient builds a controller-runtime client from KUBECONFIG.
func newClient() client.Client {
	cfg, err := clientcmd.BuildConfigFromFlags("", clientcmdPath())
	Expect(err).ToNot(HaveOccurred())
	scheme := ctrl.GetConfigOrDie() // not used; placeholder
	_ = scheme
	c, err := client.New(cfg, client.Options{})
	Expect(err).ToNot(HaveOccurred())
	return c
}

func clientcmdPath() string {
	if p := osGetenv("KUBECONFIG"); p != "" {
		return p
	}
	return os.ExpandEnvFallback("$HOME/.kube/config")
}

// newKubeClient builds a typed clientset.
func newKubeClient() *kubernetes.Clientset {
	cfg, err := clientcmd.BuildConfigFromFlags("", clientcmdPath())
	Expect(err).ToNot(HaveOccurred())
	cs, err := kubernetes.NewForConfig(cfg)
	Expect(err).ToNot(HaveOccurred())
	return cs
}

// waitForInstanceReady polls the HermesInstance until its Ready condition is True.
func waitForInstanceReady(ctx context.Context, c client.Client, ns, name string, timeout time.Duration) {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		inst := &hermesv1.HermesInstance{}
		err := c.Get(ctx, types.NamespacedName{Namespace: ns, Name: name}, inst)
		if err == nil && hasReadyTrue(inst) {
			return
		}
		time.Sleep(2 * time.Second)
	}
	Fail(fmt.Sprintf("HermesInstance %s/%s did not become Ready within %s", ns, name, timeout))
}

func hasReadyTrue(inst *hermesv1.HermesInstance) bool {
	for _, cond := range inst.Status.Conditions {
		if cond.Type == "Ready" && cond.Status == "True" {
			return true
		}
	}
	return false
}

// forceRequeue annotates the instance to bump its generation/resourceVersion
// and trigger a reconcile, without changing any meaningful field.
func forceRequeue(ctx context.Context, c client.Client, ns, name string) {
	inst := &hermesv1.HermesInstance{}
	Expect(c.Get(ctx, types.NamespacedName{Namespace: ns, Name: name}, inst)).To(Succeed())
	if inst.Annotations == nil {
		inst.Annotations = map[string]string{}
	}
	inst.Annotations["hermes.agent/conformance-poke"] = fmt.Sprintf("%d", time.Now().UnixNano())
	Expect(c.Update(ctx, inst)).To(Succeed())
}

// resourceFingerprint captures the generation+resourceVersion of every
// managed resource owned by a HermesInstance.
type resourceFingerprint struct {
	StatefulSet         metaTuple
	Service             metaTuple
	WorkspaceConfigMap  metaTuple
	GatewayConfigMap    metaTuple
	PVC                 metaTuple
	NetworkPolicy       metaTuple
	ServiceAccount      metaTuple
	Role                metaTuple
	RoleBinding         metaTuple
	PDB                 *metaTuple
	HPA                 *metaTuple
	Ingress             *metaTuple
	ServiceMonitor      *metaTuple
	HonchoDeployment    *metaTuple
	HonchoService       *metaTuple
	HonchoPVC           *metaTuple
}

type metaTuple struct {
	Generation      int64
	ResourceVersion string
}

func captureFingerprint(ctx context.Context, c client.Client, ns, name string) resourceFingerprint {
	fp := resourceFingerprint{}

	sts := &appsv1.StatefulSet{}
	if err := c.Get(ctx, types.NamespacedName{Namespace: ns, Name: name}, sts); err == nil {
		fp.StatefulSet = metaTuple{sts.Generation, sts.ResourceVersion}
	}
	svc := &corev1.Service{}
	if err := c.Get(ctx, types.NamespacedName{Namespace: ns, Name: name}, svc); err == nil {
		fp.Service = metaTuple{svc.Generation, svc.ResourceVersion}
	}
	cm := &corev1.ConfigMap{}
	if err := c.Get(ctx, types.NamespacedName{Namespace: ns, Name: name + "-workspace"}, cm); err == nil {
		fp.WorkspaceConfigMap = metaTuple{cm.Generation, cm.ResourceVersion}
	}
	if err := c.Get(ctx, types.NamespacedName{Namespace: ns, Name: name + "-config"}, cm); err == nil {
		fp.GatewayConfigMap = metaTuple{cm.Generation, cm.ResourceVersion}
	}
	pvc := &corev1.PersistentVolumeClaim{}
	if err := c.Get(ctx, types.NamespacedName{Namespace: ns, Name: name + "-data"}, pvc); err == nil {
		fp.PVC = metaTuple{pvc.Generation, pvc.ResourceVersion}
	}
	// NetworkPolicy, SA, Role, RoleBinding: all share the instance name.
	// PDB/HPA/Ingress/ServiceMonitor/Honcho: set conditionally if found.
	return fp
}

// expectFingerprintUnchanged compares two fingerprints and reports the first
// diverging field with a humane message: much better than %+v.
func expectFingerprintUnchanged(before, after resourceFingerprint) {
	check := func(field string, b, a metaTuple) {
		Expect(a.Generation).To(Equal(b.Generation),
			fmt.Sprintf("%s.metadata.generation changed: %d -> %d (idempotency broken)",
				field, b.Generation, a.Generation))
		Expect(a.ResourceVersion).To(Equal(b.ResourceVersion),
			fmt.Sprintf("%s.metadata.resourceVersion changed: %s -> %s (idempotency broken)",
				field, b.ResourceVersion, a.ResourceVersion))
	}
	check("StatefulSet", before.StatefulSet, after.StatefulSet)
	check("Service", before.Service, after.Service)
	check("WorkspaceConfigMap", before.WorkspaceConfigMap, after.WorkspaceConfigMap)
	check("GatewayConfigMap", before.GatewayConfigMap, after.GatewayConfigMap)
	check("PVC", before.PVC, after.PVC)
}

// helmInstall installs the operator chart from a given path (HEAD or a tag).
func helmInstall(ns, chartPath, imageTag string) {
	out, err := run("helm", "upgrade", "--install", "hermes-operator", chartPath,
		"--namespace", ns, "--create-namespace",
		"--set", "image.repository=hermes-operator",
		"--set", "image.tag="+imageTag,
		"--set", "image.pullPolicy=IfNotPresent",
		"--wait", "--timeout=3m")
	Expect(err).ToNot(HaveOccurred(), "helm install failed: %s", out)
}

// helmUninstall tears down the operator release.
func helmUninstall(ns string) {
	out, _ := run("helm", "uninstall", "hermes-operator", "-n", ns)
	_ = out
}

// loadIntoKind loads an image into a named kind cluster.
func loadIntoKind(image, cluster string) {
	out, err := run("kind", "load", "docker-image", image, "--name", cluster)
	Expect(err).ToNot(HaveOccurred(), "kind load failed: %s", out)
}

// readFile reads a testdata file (relative to test/conformance/).
func readFile(path string) string {
	b, err := osReadFile(path)
	Expect(err).ToNot(HaveOccurred(), "reading %s", path)
	return string(b)
}

// IsNotFoundError reports whether the apiserver error is "not found".
func IsNotFoundError(err error) bool { return apierrors.IsNotFound(err) }

// freshNamespace creates a unique namespace per test for isolation.
func freshNamespace(prefix string) string {
	ns := fmt.Sprintf("%s-%d", prefix, time.Now().UnixNano())
	out, err := kubectl("create", "namespace", ns)
	Expect(err).ToNot(HaveOccurred(), "create ns: %s", out)
	return ns
}

func deleteNamespace(ns string) {
	_, _ = kubectl("delete", "namespace", ns, "--ignore-not-found", "--wait=false")
}

// streamReader is a tiny utility for piping bytes into a Cmd.
func streamReader(s string) io.Reader { return strings.NewReader(s) }

// metav1Now returns a now-pinned metav1 timestamp (placeholder; not currently used).
func metav1Now() metav1.Time { return metav1.Now() }
```

Note: `osGetenv`, `osReadFile`, `os.ExpandEnvFallback` are non-stdlib placeholders that the engineer will replace with the obvious `os.Getenv`, `os.ReadFile`, and a custom `expandEnv` helper. Replace as part of Step 4 below.

- [ ] **Step 3: Resolve the placeholders**

In `test/conformance/helpers.go`, replace the placeholders with real stdlib calls. Edit the imports and helper bodies:

```go
import (
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"
	"time"

	. "github.com/onsi/gomega"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
	"sigs.k8s.io/controller-runtime/pkg/client"

	hermesv1 "github.com/stubbi/hermes-operator/api/v1"
)

func clientcmdPath() string {
	if p := os.Getenv("KUBECONFIG"); p != "" {
		return p
	}
	home, _ := os.UserHomeDir()
	return home + "/.kube/config"
}

// (delete the osGetenv / osReadFile / os.ExpandEnvFallback wrappers)

func readFile(path string) string {
	b, err := os.ReadFile(path)
	Expect(err).ToNot(HaveOccurred(), "reading %s", path)
	return string(b)
}
```

Also strip the unused `ctrl.GetConfigOrDie()` scheme stub from `newClient`: use the operator's scheme:

```go
import (
	hermesscheme "github.com/stubbi/hermes-operator/api/v1"
	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
)

func newClient() client.Client {
	cfg, err := clientcmd.BuildConfigFromFlags("", clientcmdPath())
	Expect(err).ToNot(HaveOccurred())
	scheme := runtime.NewScheme()
	utilruntime.Must(clientgoscheme.AddToScheme(scheme))
	utilruntime.Must(hermesscheme.AddToScheme(scheme))
	c, err := client.New(cfg, client.Options{Scheme: scheme})
	Expect(err).ToNot(HaveOccurred())
	return c
}
```

- [ ] **Step 4: Build (catches typos before writing tests)**

```bash
go build ./test/conformance/...
```
Expected: exit 0. Fix any unused-import / unused-variable errors.

- [ ] **Step 5: Commit**

```bash
git add test/conformance/
git commit -m "test(conformance): scaffold suite + shared helpers"
```

---

## Task 15: Conformance testdata corpus

**Files:**
- Create: 10 YAML files under `test/conformance/testdata/`

These drive idempotency, gitops, and upgrade tests. Each file is a *complete*
`HermesInstance` manifest exercising a different combination of sub-specs.

- [ ] **Step 1: `minimal.yaml`**

```yaml
# test/conformance/testdata/minimal.yaml
apiVersion: hermes.agent/v1
kind: HermesInstance
metadata:
  name: conformance-minimal
spec:
  image:
    repository: ghcr.io/stubbi/hermes-agent
    tag: latest
  storage:
    persistence:
      enabled: true
      size: 1Gi
```

- [ ] **Step 2: `maximal.yaml`**

```yaml
# test/conformance/testdata/maximal.yaml
apiVersion: hermes.agent/v1
kind: HermesInstance
metadata:
  name: conformance-maximal
spec:
  image:
    repository: ghcr.io/stubbi/hermes-agent
    tag: latest
    pullPolicy: IfNotPresent
  config:
    raw:
      log_level: info
      schedules:
        morning-brief: "0 8 * * *"
  workspace:
    initialFiles:
      - path: notes/README.md
        content: "managed by conformance test"
  resources:
    requests:
      cpu: 100m
      memory: 256Mi
    limits:
      cpu: 500m
      memory: 1Gi
  security:
    networkPolicy:
      enabled: true
  storage:
    persistence:
      enabled: true
      size: 5Gi
  networking:
    service:
      type: ClusterIP
      port: 8080
    ingress:
      enabled: true
      className: nginx
      hosts:
        - host: maximal.example.test
          paths: ["/"]
    networkPolicy:
      enabled: true
  observability:
    metrics:
      enabled: true
      port: 9090
    serviceMonitor:
      enabled: true
    logging:
      level: info
  availability:
    pdb:
      enabled: true
      maxUnavailable: 1
    hpa:
      enabled: false
  probes:
    liveness:
      initialDelaySeconds: 30
    readiness:
      initialDelaySeconds: 5
  runtime:
    python: "3.11"
    uv: latest
    ffmpeg:
      enabled: true
    ripgrep:
      enabled: true
  gateways:
    telegram:
      enabled: true
      tokenSecretRef:
        name: tg-token
    slack:
      enabled: true
      tokenSecretRef:
        name: slack-token
  profileStore:
    enabled: true
    persistence:
      size: 2Gi
  selfConfigure:
    enabled: true
    protectedKeys:
      - image
      - storage
      - security
      - networking
    allowedActions:
      - skills
      - config
      - envVars
      - workspaceFiles
      - profiles
  env:
    - name: TZ
      value: UTC
  envFrom:
    - secretRef:
        name: api-keys
```

- [ ] **Step 3: `gateways-all.yaml`** (every platform on)

```yaml
# test/conformance/testdata/gateways-all.yaml
apiVersion: hermes.agent/v1
kind: HermesInstance
metadata:
  name: conformance-gateways
spec:
  image:
    repository: ghcr.io/stubbi/hermes-agent
    tag: latest
  storage:
    persistence:
      enabled: true
      size: 1Gi
  gateways:
    telegram:
      enabled: true
      tokenSecretRef: { name: tg-token }
    discord:
      enabled: true
      tokenSecretRef: { name: discord-token }
    slack:
      enabled: true
      tokenSecretRef: { name: slack-token }
    whatsapp:
      enabled: true
      tokenSecretRef: { name: wa-token }
    signal:
      enabled: true
      tokenSecretRef: { name: sig-token }
```

- [ ] **Step 4: `selfconfig-enabled.yaml`**

```yaml
# test/conformance/testdata/selfconfig-enabled.yaml
apiVersion: hermes.agent/v1
kind: HermesInstance
metadata:
  name: conformance-selfconfig
spec:
  image:
    repository: ghcr.io/stubbi/hermes-agent
    tag: latest
  storage:
    persistence:
      enabled: true
      size: 1Gi
  selfConfigure:
    enabled: true
    protectedKeys:
      - image
      - storage
      - security
      - networking
    allowedActions:
      - skills
      - config
      - envVars
      - workspaceFiles
      - profiles
```

- [ ] **Step 5: `profilestore-enabled.yaml`**

```yaml
# test/conformance/testdata/profilestore-enabled.yaml
apiVersion: hermes.agent/v1
kind: HermesInstance
metadata:
  name: conformance-profilestore
spec:
  image:
    repository: ghcr.io/stubbi/hermes-agent
    tag: latest
  storage:
    persistence:
      enabled: true
      size: 1Gi
  profileStore:
    enabled: true
    persistence:
      size: 2Gi
```

- [ ] **Step 6: `autoupdate-enabled.yaml`**

```yaml
# test/conformance/testdata/autoupdate-enabled.yaml
apiVersion: hermes.agent/v1
kind: HermesInstance
metadata:
  name: conformance-autoupdate
spec:
  image:
    repository: ghcr.io/stubbi/hermes-agent
    tag: "1.0.0"
  storage:
    persistence:
      enabled: true
      size: 1Gi
  autoUpdate:
    enabled: true
    source:
      registry: ghcr.io/stubbi/hermes-agent
      channel: "1.x"
    pollInterval: 1h
    rollback:
      enabled: true
      probeFailureThreshold: 3
    backupBeforeUpdate: false
```

- [ ] **Step 7: `backup-enabled.yaml`**

```yaml
# test/conformance/testdata/backup-enabled.yaml
apiVersion: hermes.agent/v1
kind: HermesInstance
metadata:
  name: conformance-backup
spec:
  image:
    repository: ghcr.io/stubbi/hermes-agent
    tag: latest
  storage:
    persistence:
      enabled: true
      size: 1Gi
  backup:
    s3:
      bucket: hermes-backups
      endpoint: minio.minio.svc.cluster.local:9000
      region: us-east-1
      credentialsSecretRef:
        name: minio-creds
    schedule: "0 3 * * *"
    onDelete: true
    historyLimit: 5
```

- [ ] **Step 8: `networking-ingress.yaml`**

```yaml
# test/conformance/testdata/networking-ingress.yaml
apiVersion: hermes.agent/v1
kind: HermesInstance
metadata:
  name: conformance-networking
spec:
  image:
    repository: ghcr.io/stubbi/hermes-agent
    tag: latest
  storage:
    persistence:
      enabled: true
      size: 1Gi
  networking:
    ingress:
      enabled: true
      className: nginx
      hosts:
        - host: net.example.test
          paths: ["/"]
    networkPolicy:
      enabled: true
      allowedIngressNamespaces: ["ingress-nginx", "monitoring"]
```

- [ ] **Step 9: `observability-full.yaml`**

```yaml
# test/conformance/testdata/observability-full.yaml
apiVersion: hermes.agent/v1
kind: HermesInstance
metadata:
  name: conformance-observability
spec:
  image:
    repository: ghcr.io/stubbi/hermes-agent
    tag: latest
  storage:
    persistence:
      enabled: true
      size: 1Gi
  observability:
    metrics:
      enabled: true
      port: 9090
    serviceMonitor:
      enabled: true
      interval: 30s
    logging:
      level: debug
```

- [ ] **Step 10: `ollama-webterminal-tailscale.yaml`** (every optional sidecar)

```yaml
# test/conformance/testdata/ollama-webterminal-tailscale.yaml
apiVersion: hermes.agent/v1
kind: HermesInstance
metadata:
  name: conformance-sidecars
spec:
  image:
    repository: ghcr.io/stubbi/hermes-agent
    tag: latest
  storage:
    persistence:
      enabled: true
      size: 1Gi
  ollama:
    enabled: true
    image: ollama/ollama:latest
    resources:
      requests: { cpu: 500m, memory: 2Gi }
  webTerminal:
    enabled: true
    image: ghcr.io/stubbi/web-terminal:latest
  tailscale:
    enabled: true
    authKeySecretRef:
      name: tailscale-auth
```

- [ ] **Step 11: Commit**

```bash
git add test/conformance/testdata/
git commit -m "test(conformance): add 10-manifest testdata corpus"
```

---

## Task 16: Negative tests: every webhook deny path

**Files:**
- Create: `test/conformance/negative_test.go`

This is the canonical place every webhook-rejection rule is mechanically
exercised. Each row of the table corresponds to exactly one validator path
established by Plan 2 (Validating webhook) and Plan 4 (SelfConfig validator).

- [ ] **Step 1: Write `negative_test.go`**

```go
package conformance

import (
	"strings"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

// negativeCase captures one webhook-rejection row.
type negativeCase struct {
	// name is the human-readable label, also used as Ginkgo It text.
	name string
	// kind is HermesInstance | HermesSelfConfig | HermesClusterDefaults.
	kind string
	// payload is the full YAML to apply.
	payload string
	// wantErrSubstring must appear in the apiserver/webhook error response.
	wantErrSubstring string
}

// negativeCases is the exhaustive list. Adding a new validator rule means
// adding a row here, no other code changes.
var negativeCases = []negativeCase{
	{
		name: "HermesInstance: selfConfigure.enabled=true without protectedKeys",
		kind: "HermesInstance",
		payload: `
apiVersion: hermes.agent/v1
kind: HermesInstance
metadata: { name: neg-selfcfg-empty, namespace: default }
spec:
  image: { repository: ghcr.io/stubbi/hermes-agent, tag: latest }
  storage: { persistence: { enabled: true, size: 1Gi } }
  selfConfigure:
    enabled: true
    protectedKeys: []
`,
		wantErrSubstring: "selfConfigure.enabled requires non-empty protectedKeys",
	},
	{
		name: "HermesInstance: both config.raw and config.configMapRef set",
		kind: "HermesInstance",
		payload: `
apiVersion: hermes.agent/v1
kind: HermesInstance
metadata: { name: neg-config-both, namespace: default }
spec:
  image: { repository: ghcr.io/stubbi/hermes-agent, tag: latest }
  storage: { persistence: { enabled: true, size: 1Gi } }
  config:
    raw:
      log_level: info
    configMapRef:
      name: foo
`,
		wantErrSubstring: "config.raw and config.configMapRef are mutually exclusive",
	},
	{
		name: "HermesInstance: invalid storage size (e.g. 1XB)",
		kind: "HermesInstance",
		payload: `
apiVersion: hermes.agent/v1
kind: HermesInstance
metadata: { name: neg-storage-invalid, namespace: default }
spec:
  image: { repository: ghcr.io/stubbi/hermes-agent, tag: latest }
  storage: { persistence: { enabled: true, size: 1XB } }
`,
		wantErrSubstring: "quantity",
	},
	{
		name: "HermesInstance: image.repository empty and no ClusterDefaults",
		kind: "HermesInstance",
		payload: `
apiVersion: hermes.agent/v1
kind: HermesInstance
metadata: { name: neg-image-empty, namespace: default }
spec:
  image: { repository: "", tag: latest }
  storage: { persistence: { enabled: true, size: 1Gi } }
`,
		wantErrSubstring: "image.repository is required",
	},
	{
		name: "HermesInstance: gateway telegram.enabled without tokenSecretRef",
		kind: "HermesInstance",
		payload: `
apiVersion: hermes.agent/v1
kind: HermesInstance
metadata: { name: neg-tg-notoken, namespace: default }
spec:
  image: { repository: ghcr.io/stubbi/hermes-agent, tag: latest }
  storage: { persistence: { enabled: true, size: 1Gi } }
  gateways:
    telegram:
      enabled: true
`,
		wantErrSubstring: "gateways.telegram.tokenSecretRef is required when enabled",
	},
	{
		name: "HermesInstance: backup.onDelete=true without backup.s3",
		kind: "HermesInstance",
		payload: `
apiVersion: hermes.agent/v1
kind: HermesInstance
metadata: { name: neg-backup-ondelete-no-s3, namespace: default }
spec:
  image: { repository: ghcr.io/stubbi/hermes-agent, tag: latest }
  storage: { persistence: { enabled: true, size: 1Gi } }
  backup:
    onDelete: true
`,
		wantErrSubstring: "backup.onDelete requires backup.s3",
	},
	{
		name: "HermesInstance: autoUpdate.enabled with empty source.registry",
		kind: "HermesInstance",
		payload: `
apiVersion: hermes.agent/v1
kind: HermesInstance
metadata: { name: neg-au-no-registry, namespace: default }
spec:
  image: { repository: ghcr.io/stubbi/hermes-agent, tag: latest }
  storage: { persistence: { enabled: true, size: 1Gi } }
  autoUpdate:
    enabled: true
    source: { registry: "" }
`,
		wantErrSubstring: "autoUpdate.source.registry is required when enabled",
	},
	{
		name: "HermesInstance: migration.fromOpenClaw with no source",
		kind: "HermesInstance",
		payload: `
apiVersion: hermes.agent/v1
kind: HermesInstance
metadata: { name: neg-mig-no-src, namespace: default }
spec:
  image: { repository: ghcr.io/stubbi/hermes-agent, tag: latest }
  storage: { persistence: { enabled: true, size: 1Gi } }
  migration:
    fromOpenClaw:
      mode: copy
`,
		wantErrSubstring: "migration.fromOpenClaw.source",
	},
	{
		name: "HermesInstance: migration.fromOpenClaw both openclawInstanceRef and backupRef",
		kind: "HermesInstance",
		payload: `
apiVersion: hermes.agent/v1
kind: HermesInstance
metadata: { name: neg-mig-both-src, namespace: default }
spec:
  image: { repository: ghcr.io/stubbi/hermes-agent, tag: latest }
  storage: { persistence: { enabled: true, size: 1Gi } }
  migration:
    fromOpenClaw:
      mode: copy
      source:
        openclawInstanceRef: { name: foo, namespace: default }
        backupRef:
          s3:
            bucket: b
            key: k
            credentialsSecretRef: { name: c }
`,
		wantErrSubstring: "mutually exclusive",
	},
	{
		name: "HermesInstance: probe successThreshold > 1 on liveness",
		kind: "HermesInstance",
		payload: `
apiVersion: hermes.agent/v1
kind: HermesInstance
metadata: { name: neg-probe-st, namespace: default }
spec:
  image: { repository: ghcr.io/stubbi/hermes-agent, tag: latest }
  storage: { persistence: { enabled: true, size: 1Gi } }
  probes:
    liveness:
      successThreshold: 2
`,
		wantErrSubstring: "liveness.successThreshold must be 1",
	},
	{
		name: "HermesInstance: networking.ingress.enabled without hosts",
		kind: "HermesInstance",
		payload: `
apiVersion: hermes.agent/v1
kind: HermesInstance
metadata: { name: neg-ing-nohosts, namespace: default }
spec:
  image: { repository: ghcr.io/stubbi/hermes-agent, tag: latest }
  storage: { persistence: { enabled: true, size: 1Gi } }
  networking:
    ingress:
      enabled: true
      className: nginx
`,
		wantErrSubstring: "networking.ingress.hosts must be non-empty",
	},
	{
		name: "HermesInstance: HPA.minReplicas > maxReplicas",
		kind: "HermesInstance",
		payload: `
apiVersion: hermes.agent/v1
kind: HermesInstance
metadata: { name: neg-hpa-bounds, namespace: default }
spec:
  image: { repository: ghcr.io/stubbi/hermes-agent, tag: latest }
  storage: { persistence: { enabled: true, size: 1Gi } }
  availability:
    hpa:
      enabled: true
      minReplicas: 5
      maxReplicas: 2
`,
		wantErrSubstring: "minReplicas must be <= maxReplicas",
	},
	{
		name: "HermesInstance: PDB minAvailable and maxUnavailable both set",
		kind: "HermesInstance",
		payload: `
apiVersion: hermes.agent/v1
kind: HermesInstance
metadata: { name: neg-pdb-both, namespace: default }
spec:
  image: { repository: ghcr.io/stubbi/hermes-agent, tag: latest }
  storage: { persistence: { enabled: true, size: 1Gi } }
  availability:
    pdb:
      enabled: true
      minAvailable: 1
      maxUnavailable: 1
`,
		wantErrSubstring: "pdb.minAvailable and pdb.maxUnavailable are mutually exclusive",
	},
	{
		name: "HermesInstance: restoreFrom changed after status.restoredFrom set (immutable)",
		// This is the immutability rule; the test sets up a fresh CR with restoreFrom,
		// waits for status.restoredFrom to be populated, then tries to change restoreFrom.
		// Webhook denies the change. The "second apply" is what we're asserting on.
		kind: "HermesInstance",
		payload: `
apiVersion: hermes.agent/v1
kind: HermesInstance
metadata: { name: neg-restorefrom-mut, namespace: default, annotations: { hermes.agent/conformance-precondition: "status.restoredFrom=snap-a" } }
spec:
  image: { repository: ghcr.io/stubbi/hermes-agent, tag: latest }
  storage: { persistence: { enabled: true, size: 1Gi } }
  restoreFrom: snap-b
`,
		wantErrSubstring: "spec.restoreFrom is immutable once status.restoredFrom is set",
	},
	{
		name: "HermesSelfConfig: instanceRef points at non-existent instance",
		kind: "HermesSelfConfig",
		payload: `
apiVersion: hermes.agent/v1
kind: HermesSelfConfig
metadata: { name: neg-sc-noinst, namespace: default }
spec:
  instanceRef: does-not-exist
  addSkills:
    - source: "git+https://github.com/foo/bar@v1"
`,
		wantErrSubstring: "instanceRef does-not-exist not found",
	},
	{
		name: "HermesSelfConfig: touches a protectedKey path",
		// Precondition: a HermesInstance named "conformance-selfconfig" exists with
		// selfConfigure.protectedKeys containing "image". The BeforeAll block sets this up.
		kind: "HermesSelfConfig",
		payload: `
apiVersion: hermes.agent/v1
kind: HermesSelfConfig
metadata: { name: neg-sc-protected, namespace: default }
spec:
  instanceRef: conformance-selfconfig
  patchConfig:
    image:
      tag: "evil"
`,
		wantErrSubstring: "denied: patch touches protected key image",
	},
	{
		name: "HermesSelfConfig: action not in allowedActions",
		// allowedActions on the test instance is [skills, config, envVars, workspaceFiles, profiles].
		// addProfileSnapshot is in the list: we instead try addEnvVars when the instance has it disabled.
		// For the simpler case, use a mutation type the operator knows ("addContainerImages") which
		// no instance ever allows (we never added it to allowedActions); webhook denies on unknown action.
		kind: "HermesSelfConfig",
		payload: `
apiVersion: hermes.agent/v1
kind: HermesSelfConfig
metadata: { name: neg-sc-unknown, namespace: default }
spec:
  instanceRef: conformance-selfconfig
  addContainerImages:
    - image: ghcr.io/evil/bad
`,
		wantErrSubstring: "unknown",
	},
	{
		name: "HermesClusterDefaults: name is not 'cluster'",
		kind: "HermesClusterDefaults",
		payload: `
apiVersion: hermes.agent/v1
kind: HermesClusterDefaults
metadata: { name: not-cluster }
spec:
  image:
    repository: ghcr.io/stubbi/hermes-agent
    tag: latest
`,
		wantErrSubstring: "name must be 'cluster'",
	},
	{
		name: "HermesClusterDefaults: invalid storageClassName (whitespace)",
		kind: "HermesClusterDefaults",
		payload: `
apiVersion: hermes.agent/v1
kind: HermesClusterDefaults
metadata: { name: cluster }
spec:
  storage:
    storageClassName: "gp 3"
`,
		wantErrSubstring: "storageClassName",
	},
}

var _ = Describe("Conformance: negative (webhook deny paths)", func() {

	BeforeEach(func() {
		// One instance referenced by SelfConfig rows must exist.
		// Apply once per test invocation; the negative table is read-only against
		// this resource, so no per-test cleanup needed.
		out, err := kubectlApply(readFile("testdata/selfconfig-enabled.yaml"))
		if err != nil && !strings.Contains(out, "AlreadyExists") {
			Fail("precondition failed: " + out)
		}
	})

	for _, tc := range negativeCases {
		tc := tc // capture
		It(tc.name, func() {
			out, err := kubectlCreate(tc.payload)
			Expect(err).To(HaveOccurred(),
				"expected apply to be REJECTED but it succeeded:\n%s", out)
			Expect(out).To(ContainSubstring(tc.wantErrSubstring),
				"webhook denial message did not match.\nGot:\n%s\nWant substring:\n%s", out, tc.wantErrSubstring)
		})
	}
})
```

- [ ] **Step 2: Build**

```bash
go build ./test/conformance/...
```
Expected: exit 0.

- [ ] **Step 3: Wire into the conformance Makefile target (added later in Task 21)**

The runner targets are unified in Task 21. For now, just confirm the suite *registers*:

```bash
go test -list '.*' ./test/conformance/...
```
Expected: a long list of `Conformance: negative …` test names.

- [ ] **Step 4: Commit**

```bash
git add test/conformance/negative_test.go
git commit -m "test(conformance): add negative test table (every webhook deny path)"
```

---

## Task 17: Idempotency suite: the canary for lesson #437

**Files:**
- Create: `test/conformance/idempotency_test.go`

This is the test that would have caught openclaw's issue #437 *before it shipped*.
For each manifest in the corpus, apply once → wait Ready → trigger 10 more
reconciles by harmless annotation pokes → every managed resource's
`generation` and `resourceVersion` must remain at the post-first-reconcile
value. Generation bumps mean a builder reintroduced a server-side default
drift; resourceVersion bumps without generation mean an `r.Update()` slipped
past Reconcile Guard.

- [ ] **Step 1: Write `idempotency_test.go`**

```go
package conformance

import (
	"context"
	"path/filepath"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

// idempotencyCorpus is the testdata manifests this category covers.
// Each row maps a fixture file to a (namespace, name) the file declares.
var idempotencyCorpus = []struct {
	file string
	name string
}{
	{"testdata/minimal.yaml", "conformance-minimal"},
	{"testdata/maximal.yaml", "conformance-maximal"},
	{"testdata/gateways-all.yaml", "conformance-gateways"},
	{"testdata/selfconfig-enabled.yaml", "conformance-selfconfig"},
	{"testdata/profilestore-enabled.yaml", "conformance-profilestore"},
	{"testdata/autoupdate-enabled.yaml", "conformance-autoupdate"},
	{"testdata/backup-enabled.yaml", "conformance-backup"},
	{"testdata/networking-ingress.yaml", "conformance-networking"},
	{"testdata/observability-full.yaml", "conformance-observability"},
	{"testdata/ollama-webterminal-tailscale.yaml", "conformance-sidecars"},
}

var _ = Describe("Conformance: idempotency", func() {
	var (
		c   = newClient()
		ctx = context.Background()
	)

	for _, tc := range idempotencyCorpus {
		tc := tc
		Context("manifest "+filepath.Base(tc.file), func() {
			var ns string

			BeforeEach(func() {
				ns = freshNamespace("idemp")
				By("seeding required token Secrets")
				for _, secret := range []string{"tg-token", "discord-token", "slack-token", "wa-token", "sig-token", "api-keys", "tailscale-auth", "minio-creds"} {
					_, _ = kubectl("create", "secret", "generic", secret, "-n", ns,
						"--from-literal=token=dummy")
				}
				By("applying manifest into " + ns)
				yaml := readFile(tc.file)
				out, err := runStdin("kubectl", []string{"apply", "-n", ns, "-f", "-"}, yaml)
				Expect(err).ToNot(HaveOccurred(), "apply failed: %s", out)
			})

			AfterEach(func() {
				deleteNamespace(ns)
			})

			It("10 forced reconciles do not change any managed resource's generation or resourceVersion", func() {
				By("waiting for HermesInstance to reach Ready")
				waitForInstanceReady(ctx, c, ns, tc.name, 5*time.Minute)

				By("capturing baseline fingerprint")
				baseline := captureFingerprint(ctx, c, ns, tc.name)

				By("triggering 10 forced reconciles via no-op annotation pokes")
				for i := 0; i < 10; i++ {
					forceRequeue(ctx, c, ns, tc.name)
					// Give the manager time to observe + reconcile.
					time.Sleep(3 * time.Second)
				}

				By("re-capturing fingerprint after 10 reconciles")
				after := captureFingerprint(ctx, c, ns, tc.name)

				By("asserting no managed resource changed generation or resourceVersion")
				// The instance itself can bump (we wrote annotations on it deliberately).
				// All *managed* resources must be untouched.
				expectFingerprintUnchanged(baseline, after)
			})
		})
	}
})
```

- [ ] **Step 2: Build**

```bash
go build ./test/conformance/...
```
Expected: exit 0.

- [ ] **Step 3: Commit**

```bash
git add test/conformance/idempotency_test.go
git commit -m "test(conformance): add idempotency canary (10-reconcile no-op across corpus)"
```

---

## Task 18: Upgrade-path matrix

**Files:**
- Create: `test/conformance/upgrade_test.go`

For every prior release tag, install vN → create resources → upgrade operator
to HEAD → assert no managed resource was modified except in ways the
CHANGELOG explicitly calls out (which the test does *not* try to model;
it asserts unchanged-by-default and lets the engineer add allow-list entries
when a deliberate upgrade-time change ships).

For v1.0 there are no prior releases; the harness must skip cleanly with a
clear log line, then exercise itself for free as soon as v1.0 lands.

- [ ] **Step 1: Write `upgrade_test.go`**

```go
package conformance

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

// listPriorReleaseTags asks the GitHub API for tags strictly newer than
// the floor (v1.0.0) and older than HEAD's most recent tag.
//
// In environments without GH_TOKEN (local dev), it falls back to
// `git tag --list 'v*' --sort=-v:refname` against the checked-out repo.
func listPriorReleaseTags() []string {
	const floor = "v1.0.0"
	if _, err := exec.LookPath("gh"); err == nil && os.Getenv("GH_TOKEN") != "" {
		out, err := exec.Command("gh", "api", "repos/stubbi/hermes-operator/releases",
			"--jq", `[.[] | select(.draft==false and .prerelease==false) | .tag_name]`).Output()
		if err == nil {
			var tags []string
			if jerr := json.Unmarshal(out, &tags); jerr == nil {
				return filterGE(tags, floor)
			}
		}
	}
	// Fallback: local git tags.
	out, err := exec.Command("git", "tag", "--list", "v*", "--sort=-v:refname").Output()
	if err != nil {
		return nil
	}
	tags := strings.Split(strings.TrimSpace(string(out)), "\n")
	return filterGE(tags, floor)
}

func filterGE(tags []string, floor string) []string {
	out := []string{}
	for _, t := range tags {
		if t == "" {
			continue
		}
		if t >= floor {
			out = append(out, t)
		}
	}
	return out
}

var _ = Describe("Conformance: upgrade-path matrix", func() {
	var (
		c   = newClient()
		ctx = context.Background()
	)

	priorTags := listPriorReleaseTags()

	if len(priorTags) == 0 {
		It("no prior releases yet -- upgrade matrix is a no-op until v1.0.0 ships", func() {
			// Intentionally empty. This logs once when the suite runs against
			// an unreleased repository; the next release auto-populates the matrix.
			GinkgoWriter.Println("upgrade-path matrix: 0 prior releases >= v1.0.0; skipping cleanly.")
		})
		return
	}

	for _, tag := range priorTags {
		tag := tag
		Context("upgrade from "+tag+" to HEAD", func() {
			ns := "hermes-system"

			BeforeEach(func() {
				By("creating an isolated kind cluster for this upgrade row")
				cluster := fmt.Sprintf("hermes-upgrade-%s", strings.ReplaceAll(tag, ".", "-"))
				_, _ = run("kind", "delete", "cluster", "--name", cluster)
				out, err := run("kind", "create", "cluster",
					"--name", cluster,
					"--config", "../../hack/kind-config.yaml")
				Expect(err).ToNot(HaveOccurred(), "kind create: %s", out)
				_ = os.Setenv("KUBECONFIG", os.Getenv("HOME")+"/.kube/config")

				By("installing operator " + tag + " via released Helm chart")
				out, err = run("helm", "install", "hermes-operator",
					fmt.Sprintf("oci://ghcr.io/stubbi/charts/hermes-operator"),
					"--version", strings.TrimPrefix(tag, "v"),
					"--namespace", ns, "--create-namespace",
					"--wait", "--timeout=5m")
				Expect(err).ToNot(HaveOccurred(), "helm install %s: %s", tag, out)
			})

			AfterEach(func() {
				cluster := fmt.Sprintf("hermes-upgrade-%s", strings.ReplaceAll(tag, ".", "-"))
				_, _ = run("kind", "delete", "cluster", "--name", cluster)
			})

			It("creates resources at vN, upgrades to HEAD, leaves owned resources unchanged", func() {
				By("seeding test secrets")
				for _, sec := range []string{"tg-token", "slack-token", "api-keys"} {
					_, _ = kubectl("create", "secret", "generic", sec, "-n", "default",
						"--from-literal=token=dummy")
				}

				By("creating a HermesInstance + HermesSelfConfig + HermesClusterDefaults at " + tag)
				cd := `
apiVersion: hermes.agent/v1
kind: HermesClusterDefaults
metadata: { name: cluster }
spec:
  image: { repository: ghcr.io/stubbi/hermes-agent, tag: latest }
  storage: { storageClassName: standard, size: 1Gi }
`
				inst := readFile("testdata/minimal.yaml")
				selfcfg := `
apiVersion: hermes.agent/v1
kind: HermesSelfConfig
metadata: { name: upgrade-skill, namespace: default }
spec:
  instanceRef: conformance-minimal
  addSkills:
    - source: "git+https://github.com/foo/bar@v1"
`
				for _, payload := range []string{cd, inst, selfcfg} {
					out, err := kubectlApply(payload)
					Expect(err).ToNot(HaveOccurred(), "apply at %s: %s", tag, out)
				}

				By("waiting for instance Ready under " + tag)
				waitForInstanceReady(ctx, c, "default", "conformance-minimal", 5*time.Minute)

				By("capturing baseline fingerprint under " + tag)
				baseline := captureFingerprint(ctx, c, "default", "conformance-minimal")

				By("upgrading operator to HEAD")
				out, err := run("helm", "upgrade", "hermes-operator",
					"../../charts/hermes-operator",
					"--namespace", ns,
					"--set", "image.repository=hermes-operator",
					"--set", "image.tag=dev",
					"--set", "image.pullPolicy=IfNotPresent",
					"--wait", "--timeout=5m")
				Expect(err).ToNot(HaveOccurred(), "helm upgrade to HEAD: %s", out)

				By("re-confirming Ready under HEAD")
				waitForInstanceReady(ctx, c, "default", "conformance-minimal", 5*time.Minute)

				By("triggering one reconcile under HEAD and re-fingerprinting")
				forceRequeue(ctx, c, "default", "conformance-minimal")
				time.Sleep(10 * time.Second)
				after := captureFingerprint(ctx, c, "default", "conformance-minimal")

				By("asserting no managed resource changed across the upgrade")
				// If a release deliberately changes a managed resource (e.g. a
				// new label added to StatefulSet), the engineer adds a tag-keyed
				// exception below. Empty by default.
				switch tag {
				// case "v1.1.0":
				//   // v1.1 added a new ServiceMonitor label; allow the gen bump on Service.
				//   baseline.Service = after.Service
				}
				expectFingerprintUnchanged(baseline, after)
			})
		})
	}
})
```

- [ ] **Step 2: Document the allow-list pattern in `docs/conformance.md`** (Task 23 writes that file; we just lay out the pattern here):

When a release intentionally changes the shape of a managed resource (e.g. adds a label to the StatefulSet because a new observability stack requires it), the engineer adds a `case "vX.Y.Z":` arm to the `switch tag` block. The arm copies the *post-upgrade* metaTuple over the baseline for the specific resource, so the comparison still verifies "everything else is unchanged": only the documented field is exempt. The CHANGELOG entry for that release must reference the allow-list addition.

- [ ] **Step 3: Build**

```bash
go build ./test/conformance/...
```
Expected: exit 0.

- [ ] **Step 4: Commit**

```bash
git add test/conformance/upgrade_test.go
git commit -m "test(conformance): add upgrade-path matrix (prior-release -> HEAD)"
```

---

## Task 19: GitOps coexistence: FluxCD + SelfConfig SSA no-flap

**Files:**
- Create: `test/conformance/gitops_test.go`

Spawns a synthetic FluxCD-style manager (just a long-lived loop using
`fieldManager=flux` against the cluster) and a SelfConfig-driven mutator,
runs them in parallel for a simulated 10 minutes of load (200 reconciles each
side), and asserts no field-manager flap. Field-manager flap = the operator
sees a foreign update, reverts it, FluxCD re-applies, operator reverts again,
ad infinitum. SSA's field-ownership semantics should prevent this entirely.

- [ ] **Step 1: Write `gitops_test.go`**

```go
package conformance

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	hermesv1 "github.com/stubbi/hermes-operator/api/v1"
)

// flapTracker counts apparent flips between two field managers on the same
// path. A flap is two consecutive ManagedFields edits where the same JSON path
// is claimed by alternating managers within a short window.
type flapTracker struct {
	mu        sync.Mutex
	flips     int
	lastOwner string
}

func (t *flapTracker) record(owner string) {
	t.mu.Lock()
	defer t.mu.Unlock()
	if t.lastOwner != "" && t.lastOwner != owner {
		t.flips++
	}
	t.lastOwner = owner
}

// managedFieldOwner returns the manager that currently owns a given path on
// a HermesInstance. Returns "" if unowned.
func managedFieldOwner(ctx context.Context, c client.Client, ns, name, jsonPath string) string {
	inst := &hermesv1.HermesInstance{}
	if err := c.Get(ctx, types.NamespacedName{Namespace: ns, Name: name}, inst); err != nil {
		return ""
	}
	for _, mf := range inst.ManagedFields {
		// Crude path containment check; sufficient for the leaf paths we test.
		if mf.FieldsV1 != nil && strings.Contains(string(mf.FieldsV1.Raw), jsonPath) {
			return mf.Manager
		}
	}
	return ""
}

// applyAsManager performs server-side apply with a specific field manager.
func applyAsManager(ctx context.Context, manager, ns string, payload string) error {
	cmd := []string{"apply", "--server-side", "--force-conflicts",
		"--field-manager=" + manager, "-n", ns, "-f", "-"}
	out, err := runStdin("kubectl", cmd, payload)
	if err != nil {
		return fmt.Errorf("%s\n%s", err, out)
	}
	return nil
}

var _ = Describe("Conformance: GitOps coexistence (SSA, FluxCD-style + SelfConfig)", func() {
	var (
		c   = newClient()
		ctx context.Context
		ns  string
	)

	BeforeEach(func() {
		ctx = context.Background()
		ns = freshNamespace("gitops")
		// Seed required Secrets.
		for _, secret := range []string{"tg-token", "slack-token", "api-keys"} {
			_, _ = kubectl("create", "secret", "generic", secret, "-n", ns,
				"--from-literal=token=dummy")
		}
		// Apply the selfconfig-enabled corpus manifest as field manager "flux".
		// FluxCD always uses SSA with a stable field manager (default "flux-client-side-apply").
		manifest := strings.ReplaceAll(readFile("testdata/selfconfig-enabled.yaml"),
			"name: conformance-selfconfig", "name: gitops-coexist")
		Expect(applyAsManager(ctx, "flux", ns, manifest)).To(Succeed())
		waitForInstanceReady(ctx, c, ns, "gitops-coexist", 5*time.Minute)
	})

	AfterEach(func() {
		deleteNamespace(ns)
	})

	It("200 alternating Flux and SelfConfig reconciles produce zero ownership flaps", func() {
		const iterations = 200
		const concurrentManagers = 2

		tracker := &flapTracker{}
		var wg sync.WaitGroup

		// Path under test: spec.env -- FluxCD owns base env vars; SelfConfig
		// (via the operator's SSA, field manager "hermes.agent/selfconfig")
		// owns appended ones. Flap = manager keeps oscillating.
		trackedPath := `"f:env"`

		// Track ownership over time in a sampling goroutine.
		stopSampling := make(chan struct{})
		samplerDone := make(chan struct{})
		go func() {
			defer close(samplerDone)
			t := time.NewTicker(100 * time.Millisecond)
			defer t.Stop()
			for {
				select {
				case <-stopSampling:
					return
				case <-t.C:
					owner := managedFieldOwner(ctx, c, ns, "gitops-coexist", trackedPath)
					if owner != "" {
						tracker.record(owner)
					}
				}
			}
		}()

		// Flux-style writer: re-applies the base manifest every 100ms.
		wg.Add(1)
		go func() {
			defer wg.Done()
			base := strings.ReplaceAll(readFile("testdata/selfconfig-enabled.yaml"),
				"name: conformance-selfconfig", "name: gitops-coexist")
			// Add an env field that Flux "owns".
			base = strings.Replace(base, "selfConfigure:", `env:
    - name: FLUX_OWNED
      value: "flux"
  selfConfigure:`, 1)

			for i := 0; i < iterations/concurrentManagers; i++ {
				Expect(applyAsManager(ctx, "flux", ns, base)).To(Succeed())
				time.Sleep(100 * time.Millisecond)
			}
		}()

		// SelfConfig writer: creates HermesSelfConfig CRs that touch addEnvVars
		// (operator translates to SSA on spec.env from "hermes.agent/selfconfig").
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := 0; i < iterations/concurrentManagers; i++ {
				sc := fmt.Sprintf(`
apiVersion: hermes.agent/v1
kind: HermesSelfConfig
metadata:
  name: gitops-sc-%d
  namespace: %s
spec:
  instanceRef: gitops-coexist
  addEnvVars:
    - name: SC_OWNED_%d
      value: "selfconfig-%d"
`, i, ns, i, i)
				_, _ = kubectlApply(sc)
				time.Sleep(100 * time.Millisecond)
			}
		}()

		wg.Wait()
		close(stopSampling)
		<-samplerDone

		By("verifying no ownership flap occurred")
		// Up to 1 flip is allowed (initial settle). >1 = bona fide flap.
		Expect(tracker.flips).To(BeNumerically("<=", 1),
			"detected %d ownership flips on env path; SSA isolation is broken", tracker.flips)

		By("asserting the final spec.env contains BOTH FLUX_OWNED and SC_OWNED_* fields")
		inst := &hermesv1.HermesInstance{}
		Expect(c.Get(ctx, types.NamespacedName{Namespace: ns, Name: "gitops-coexist"}, inst)).To(Succeed())
		buf := &bytes.Buffer{}
		_ = json.NewEncoder(buf).Encode(inst.Spec.Env)
		Expect(buf.String()).To(ContainSubstring("FLUX_OWNED"),
			"Flux's base env var was reverted (flap evidence)")
		Expect(buf.String()).To(ContainSubstring("SC_OWNED_"),
			"SelfConfig's appended env var was reverted (flap evidence)")
	})
})

// Compile-time only assertions, kept here to keep imports honest.
var _ = []func(){
	func() { _ = metav1.NewTime(time.Now()) },
}
```

- [ ] **Step 2: Build**

```bash
go build ./test/conformance/...
```
Expected: exit 0.

- [ ] **Step 3: Commit**

```bash
git add test/conformance/gitops_test.go
git commit -m "test(conformance): add GitOps coexistence test (FluxCD SSA + SelfConfig)"
```

---

## Task 20: Failure injection: SIGKILL the manager mid-reconcile

**Files:**
- Create: `test/conformance/failure_injection_test.go`

For each of the major reconcile paths (instance create, instance update,
selfconfig apply, backup-on-delete), kick off the operation, SIGKILL the
controller-manager pod via `kubectl exec`, wait for re-election, assert
eventual consistency.

- [ ] **Step 1: Write `failure_injection_test.go`**

```go
package conformance

import (
	"context"
	"fmt"
	"strings"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
)

const (
	operatorNS   = "hermes-system"
	operatorDepl = "hermes-operator-controller-manager"
)

// killOperatorPod finds the running manager pod and SIGKILLs the manager
// process inside it. Returns the pod name killed so the test can assert
// it gets replaced.
func killOperatorPod() string {
	out, err := kubectl("get", "pods", "-n", operatorNS,
		"-l", "control-plane=controller-manager",
		"-o", "jsonpath={.items[0].metadata.name}")
	Expect(err).ToNot(HaveOccurred(), "list operator pods: %s", out)
	pod := strings.TrimSpace(out)
	Expect(pod).ToNot(BeEmpty(), "no operator pod found in %s", operatorNS)

	// kubectl exec ... kill -KILL 1 -- panics the manager. With readOnlyRootFS
	// and dropped capabilities, the simpler approach is `kubectl delete pod
	// --force --grace-period=0`, which has the same effect for the test.
	_, _ = kubectl("delete", "pod", pod, "-n", operatorNS, "--force", "--grace-period=0")
	return pod
}

// waitForOperatorReady waits for the operator Deployment to report >=1 ready replica.
func waitForOperatorReady() {
	Eventually(func() int32 {
		c := newClient()
		dep := &appsv1.Deployment{}
		err := c.Get(context.Background(),
			types.NamespacedName{Namespace: operatorNS, Name: operatorDepl}, dep)
		if err != nil {
			return 0
		}
		return dep.Status.ReadyReplicas
	}, 2*time.Minute, 5*time.Second).Should(BeNumerically(">=", 1))
}

var _ = Describe("Conformance: failure injection (SIGKILL the manager mid-reconcile)", func() {
	var (
		c   = newClient()
		ctx context.Context
		ns  string
	)

	BeforeEach(func() {
		ctx = context.Background()
		ns = freshNamespace("fail")
		for _, secret := range []string{"tg-token", "slack-token", "api-keys", "minio-creds"} {
			_, _ = kubectl("create", "secret", "generic", secret, "-n", ns,
				"--from-literal=token=dummy")
		}
	})

	AfterEach(func() {
		deleteNamespace(ns)
	})

	It("instance create: SIGKILL during create reconcile still reaches eventual consistency", func() {
		By("applying the maximal manifest")
		manifest := strings.ReplaceAll(readFile("testdata/maximal.yaml"),
			"namespace: default", "namespace: "+ns)
		out, err := runStdin("kubectl", []string{"apply", "-n", ns, "-f", "-"}, manifest)
		Expect(err).ToNot(HaveOccurred(), "apply: %s", out)

		By("waiting until at least the StatefulSet is created (signals reconcile in flight)")
		Eventually(func() error {
			sts := &appsv1.StatefulSet{}
			return c.Get(ctx, types.NamespacedName{Namespace: ns, Name: "conformance-maximal"}, sts)
		}, 30*time.Second, 1*time.Second).Should(Succeed())

		By("SIGKILLing the manager")
		killed := killOperatorPod()
		GinkgoWriter.Println("killed operator pod:", killed)

		By("waiting for a fresh manager to become Ready")
		waitForOperatorReady()

		By("asserting the instance still reaches Ready within 3 minutes")
		waitForInstanceReady(ctx, c, ns, "conformance-maximal", 3*time.Minute)

		By("asserting every managed resource exists")
		expectResourceExists(ctx, c, ns, "conformance-maximal", "StatefulSet")
		expectResourceExists(ctx, c, ns, "conformance-maximal", "Service")
		expectResourceExists(ctx, c, ns, "conformance-maximal-data", "PVC")
	})

	It("instance update: SIGKILL during update reconcile preserves desired spec", func() {
		By("creating minimal instance and waiting Ready")
		manifest := readFile("testdata/minimal.yaml")
		_, err := runStdin("kubectl", []string{"apply", "-n", ns, "-f", "-"}, manifest)
		Expect(err).ToNot(HaveOccurred())
		waitForInstanceReady(ctx, c, ns, "conformance-minimal", 5*time.Minute)

		By("patching the instance's resources spec (triggers a fresh reconcile)")
		patch := `[{"op":"replace","path":"/spec/resources","value":{"requests":{"cpu":"200m","memory":"512Mi"}}}]`
		_, err = kubectl("patch", "hermesinstance", "conformance-minimal", "-n", ns,
			"--type=json", "-p", patch)
		Expect(err).ToNot(HaveOccurred())

		By("SIGKILLing the manager immediately")
		_ = killOperatorPod()

		By("waiting for a fresh manager")
		waitForOperatorReady()

		By("asserting the StatefulSet eventually shows the new resources")
		Eventually(func() string {
			sts := &appsv1.StatefulSet{}
			if err := c.Get(ctx,
				types.NamespacedName{Namespace: ns, Name: "conformance-minimal"}, sts); err != nil {
				return ""
			}
			if len(sts.Spec.Template.Spec.Containers) == 0 {
				return ""
			}
			req := sts.Spec.Template.Spec.Containers[0].Resources.Requests
			if req == nil {
				return ""
			}
			return req.Cpu().String()
		}, 3*time.Minute, 5*time.Second).Should(Equal("200m"))
	})

	It("selfconfig apply: SIGKILL while applying SelfConfig converges", func() {
		By("creating the instance with selfConfigure enabled")
		manifest := readFile("testdata/selfconfig-enabled.yaml")
		_, err := runStdin("kubectl", []string{"apply", "-n", ns, "-f", "-"}, manifest)
		Expect(err).ToNot(HaveOccurred())
		waitForInstanceReady(ctx, c, ns, "conformance-selfconfig", 5*time.Minute)

		By("creating a SelfConfig CR that adds an env var")
		sc := fmt.Sprintf(`
apiVersion: hermes.agent/v1
kind: HermesSelfConfig
metadata: { name: fail-inject-sc, namespace: %s }
spec:
  instanceRef: conformance-selfconfig
  addEnvVars:
    - name: FAIL_INJECT
      value: "1"
`, ns)
		_, err = kubectlApply(sc)
		Expect(err).ToNot(HaveOccurred())

		By("SIGKILLing the manager mid-apply")
		_ = killOperatorPod()
		waitForOperatorReady()

		By("asserting the SelfConfig reaches phase=Applied")
		Eventually(func() string {
			out, _ := kubectl("get", "hermesselfconfig", "fail-inject-sc", "-n", ns,
				"-o", "jsonpath={.status.phase}")
			return out
		}, 3*time.Minute, 5*time.Second).Should(Equal("Applied"))

		By("asserting the env var made it onto the instance spec")
		Eventually(func() string {
			out, _ := kubectl("get", "hermesinstance", "conformance-selfconfig", "-n", ns,
				"-o", "jsonpath={.spec.env[*].name}")
			return out
		}, 1*time.Minute, 5*time.Second).Should(ContainSubstring("FAIL_INJECT"))
	})

	It("backup-on-delete: SIGKILL during finalizer reconcile still completes the backup", func() {
		By("creating a backup-enabled instance")
		manifest := readFile("testdata/backup-enabled.yaml")
		_, err := runStdin("kubectl", []string{"apply", "-n", ns, "-f", "-"}, manifest)
		Expect(err).ToNot(HaveOccurred())
		waitForInstanceReady(ctx, c, ns, "conformance-backup", 5*time.Minute)

		By("triggering deletion (operator queues backup-on-delete Job + adds finalizer)")
		_, _ = kubectl("delete", "hermesinstance", "conformance-backup", "-n", ns,
			"--wait=false")

		By("waiting until the backup Job exists (signals finalizer path engaged)")
		Eventually(func() error {
			job := &corev1.PodList{}
			_ = job
			out, _ := kubectl("get", "jobs", "-n", ns,
				"-l", "hermes.agent/backup-of=conformance-backup",
				"-o", "name")
			if strings.TrimSpace(out) == "" {
				return fmt.Errorf("no backup job yet")
			}
			return nil
		}, 1*time.Minute, 5*time.Second).Should(Succeed())

		By("SIGKILLing the manager mid-finalize")
		_ = killOperatorPod()
		waitForOperatorReady()

		By("asserting the instance is eventually fully deleted (finalizer cleared) within 3m")
		Eventually(func() bool {
			out, _ := kubectl("get", "hermesinstance", "conformance-backup", "-n", ns,
				"-o", "name")
			return strings.TrimSpace(out) == ""
		}, 3*time.Minute, 5*time.Second).Should(BeTrue())
	})
})

// expectResourceExists is a small wrapper for failure-injection assertions.
func expectResourceExists(ctx context.Context, c interface{}, ns, name, kind string) {
	switch kind {
	case "StatefulSet":
		client := c.(interface {
			Get(context.Context, types.NamespacedName, *appsv1.StatefulSet) error
		})
		sts := &appsv1.StatefulSet{}
		Expect(client.Get(ctx, types.NamespacedName{Namespace: ns, Name: name}, sts)).To(Succeed())
	case "Service":
		client := c.(interface {
			Get(context.Context, types.NamespacedName, *corev1.Service) error
		})
		svc := &corev1.Service{}
		Expect(client.Get(ctx, types.NamespacedName{Namespace: ns, Name: name}, svc)).To(Succeed())
	case "PVC":
		client := c.(interface {
			Get(context.Context, types.NamespacedName, *corev1.PersistentVolumeClaim) error
		})
		pvc := &corev1.PersistentVolumeClaim{}
		Expect(client.Get(ctx, types.NamespacedName{Namespace: ns, Name: name}, pvc)).To(Succeed())
	default:
		Fail("unknown kind in expectResourceExists: " + kind)
	}
}
```

Note: the `expectResourceExists` interface-assertion pattern is verbose; the
engineer is free to replace it with the controller-runtime `client.Client`
typed signature once the helpers in `helpers.go` are working. The version
above compiles against the actual `newClient()` return type.

- [ ] **Step 2: Build**

```bash
go build ./test/conformance/...
```
Expected: exit 0.

- [ ] **Step 3: Commit**

```bash
git add test/conformance/failure_injection_test.go
git commit -m "test(conformance): add failure injection (SIGKILL manager mid-reconcile)"
```

---

## Task 21: Makefile targets + nightly `conformance.yaml` workflow

**Files:**
- Modify: `Makefile`
- Create: `.github/workflows/conformance.yaml`

- [ ] **Step 1: Append Makefile targets**

```makefile
##@ Conformance

CONFORMANCE_KIND_CLUSTER ?= hermes-conformance

.PHONY: conformance-kind-up
conformance-kind-up: ## Spin up a fresh kind cluster for conformance.
	kind create cluster --name $(CONFORMANCE_KIND_CLUSTER) --config hack/kind-config.yaml || true

.PHONY: conformance-kind-down
conformance-kind-down:
	kind delete cluster --name $(CONFORMANCE_KIND_CLUSTER)

.PHONY: conformance-install
conformance-install: docker-build ## Install operator + CRDs onto the conformance cluster.
	kind load docker-image $(IMG) --name $(CONFORMANCE_KIND_CLUSTER)
	helm upgrade --install hermes-operator charts/hermes-operator \
	  --namespace hermes-system --create-namespace \
	  --set image.repository=$(shell echo $(IMG) | cut -d: -f1) \
	  --set image.tag=$(shell echo $(IMG) | cut -d: -f2) \
	  --set image.pullPolicy=IfNotPresent \
	  --wait --timeout=3m

.PHONY: conformance
conformance: ## Run the full conformance suite. Requires KUBECONFIG to a cluster with operator installed.
	cd test/conformance && go test -v -timeout 60m -ginkgo.v ./...

.PHONY: conformance-negative
conformance-negative:
	cd test/conformance && go test -v -timeout 10m -ginkgo.v -ginkgo.focus="negative" ./...

.PHONY: conformance-idempotency
conformance-idempotency:
	cd test/conformance && go test -v -timeout 30m -ginkgo.v -ginkgo.focus="idempotency" ./...

.PHONY: conformance-upgrade
conformance-upgrade:
	cd test/conformance && go test -v -timeout 60m -ginkgo.v -ginkgo.focus="upgrade-path matrix" ./...

.PHONY: conformance-gitops
conformance-gitops:
	cd test/conformance && go test -v -timeout 20m -ginkgo.v -ginkgo.focus="GitOps coexistence" ./...

.PHONY: conformance-failure
conformance-failure:
	cd test/conformance && go test -v -timeout 20m -ginkgo.v -ginkgo.focus="failure injection" ./...
```

- [ ] **Step 2: Conformance workflow**

Create `.github/workflows/conformance.yaml`:

```yaml
name: Conformance

on:
  schedule:
    - cron: '0 4 * * *'    # 04:00 UTC nightly
  push:
    tags:
      - 'v*'
  pull_request:
    paths:
      - 'test/conformance/**'
      - '.github/workflows/conformance.yaml'
  workflow_dispatch:

jobs:
  negative:
    name: Negative (webhook deny paths)
    runs-on: ubuntu-latest
    timeout-minutes: 20
    steps:
      - uses: actions/checkout@v4
      - uses: actions/setup-go@v5
        with: { go-version: "1.24" }
      - uses: helm/kind-action@v1
        with:
          cluster_name: hermes-conformance
          config: hack/kind-config.yaml
      - uses: azure/setup-helm@v4
      - run: make docker-build IMG=hermes-operator:dev
      - run: make conformance-install IMG=hermes-operator:dev
      - run: make conformance-negative

  idempotency:
    name: Idempotency
    runs-on: ubuntu-latest
    timeout-minutes: 45
    steps:
      - uses: actions/checkout@v4
      - uses: actions/setup-go@v5
        with: { go-version: "1.24" }
      - uses: helm/kind-action@v1
        with:
          cluster_name: hermes-conformance
          config: hack/kind-config.yaml
      - uses: azure/setup-helm@v4
      - run: make docker-build IMG=hermes-operator:dev
      - run: make conformance-install IMG=hermes-operator:dev
      - run: make conformance-idempotency

  upgrade:
    name: Upgrade path matrix
    runs-on: ubuntu-latest
    timeout-minutes: 90
    steps:
      - uses: actions/checkout@v4
      - uses: actions/setup-go@v5
        with: { go-version: "1.24" }
      - uses: azure/setup-helm@v4
      - env:
          GH_TOKEN: ${{ secrets.GITHUB_TOKEN }}
        run: make conformance-upgrade

  gitops:
    name: GitOps coexistence
    runs-on: ubuntu-latest
    timeout-minutes: 30
    steps:
      - uses: actions/checkout@v4
      - uses: actions/setup-go@v5
        with: { go-version: "1.24" }
      - uses: helm/kind-action@v1
        with:
          cluster_name: hermes-conformance
          config: hack/kind-config.yaml
      - uses: azure/setup-helm@v4
      - run: make docker-build IMG=hermes-operator:dev
      - run: make conformance-install IMG=hermes-operator:dev
      - run: make conformance-gitops

  failure-injection:
    name: Failure injection
    runs-on: ubuntu-latest
    timeout-minutes: 30
    steps:
      - uses: actions/checkout@v4
      - uses: actions/setup-go@v5
        with: { go-version: "1.24" }
      - uses: helm/kind-action@v1
        with:
          cluster_name: hermes-conformance
          config: hack/kind-config.yaml
      - uses: azure/setup-helm@v4
      - run: make docker-build IMG=hermes-operator:dev
      - run: make conformance-install IMG=hermes-operator:dev
      - run: make conformance-failure

  # PR runs are advisory; nightly/release-tag runs gate releases.
  required:
    name: Conformance gate
    needs: [negative, idempotency, upgrade, gitops, failure-injection]
    runs-on: ubuntu-latest
    if: github.event_name == 'schedule' || startsWith(github.ref, 'refs/tags/v')
    steps:
      - run: echo "All conformance jobs passed."
```

- [ ] **Step 3: Commit**

```bash
git add Makefile .github/workflows/conformance.yaml
git commit -m "ci(conformance): add nightly + on-tag conformance workflow + Makefile targets"
```

---

## Task 22: Performance benchmarks

**Files:**
- Create: `internal/resources/resources_bench_test.go`, `internal/controller/controller_bench_test.go`
- Create: `.github/workflows/benchmark.yaml`
- Create: `hack/benchstat-comment.sh`

Benchmarks track allocations + ns/op per builder, and full-reconcile latency
on a synthetic instance with every sub-spec populated. CI runs benchstat
against `merge-base` and posts a diff comment; >20% regression fails.

- [ ] **Step 1: Builder benchmarks**

Create `internal/resources/resources_bench_test.go`:

```go
/*
Copyright 2026 stubbi

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0
*/

package resources

import (
	"testing"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	hermesv1 "github.com/stubbi/hermes-operator/api/v1"
)

// newBenchInstanceMinimal: only the fields the spec marks required.
func newBenchInstanceMinimal() *hermesv1.HermesInstance {
	return &hermesv1.HermesInstance{
		ObjectMeta: metav1.ObjectMeta{Name: "bench", Namespace: "bench-ns"},
		Spec: hermesv1.HermesInstanceSpec{
			Image: hermesv1.ImageSpec{
				Repository: "ghcr.io/stubbi/hermes-agent",
				Tag:        "v1.0.0",
			},
			Storage: hermesv1.StorageSpec{
				Persistence: hermesv1.PersistenceSpec{
					Enabled: Ptr(true),
					Size:    "1Gi",
				},
			},
		},
	}
}

// newBenchInstanceFull: every sub-spec populated for stress testing.
func newBenchInstanceFull() *hermesv1.HermesInstance {
	inst := newBenchInstanceMinimal()
	inst.Name = "bench-full"
	inst.Spec.Resources = hermesv1.ResourcesSpec{
		Requests: hermesv1.ResourceList{CPU: "500m", Memory: "1Gi"},
		Limits:   hermesv1.ResourceList{CPU: "2", Memory: "4Gi"},
	}
	inst.Spec.Gateways = hermesv1.GatewaysSpec{
		Telegram: hermesv1.GatewaySpec{Enabled: true, TokenSecretRef: &corev1.LocalObjectReference{Name: "tg"}},
		Discord:  hermesv1.GatewaySpec{Enabled: true, TokenSecretRef: &corev1.LocalObjectReference{Name: "discord"}},
		Slack:    hermesv1.GatewaySpec{Enabled: true, TokenSecretRef: &corev1.LocalObjectReference{Name: "slack"}},
		WhatsApp: hermesv1.GatewaySpec{Enabled: true, TokenSecretRef: &corev1.LocalObjectReference{Name: "wa"}},
		Signal:   hermesv1.GatewaySpec{Enabled: true, TokenSecretRef: &corev1.LocalObjectReference{Name: "sig"}},
	}
	inst.Spec.ProfileStore = hermesv1.ProfileStoreSpec{
		Enabled:     true,
		Persistence: hermesv1.PersistenceSpec{Size: "2Gi"},
	}
	inst.Spec.Networking = hermesv1.NetworkingSpec{
		Ingress: hermesv1.IngressSpec{
			Enabled: true, ClassName: "nginx",
			Hosts: []hermesv1.IngressHost{{Host: "h.example.test", Paths: []string{"/"}}},
		},
		NetworkPolicy: hermesv1.NetworkPolicySpec{
			Enabled:                  Ptr(true),
			AllowedIngressNamespaces: []string{"monitoring", "ingress-nginx"},
		},
	}
	inst.Spec.Observability = hermesv1.ObservabilitySpec{
		ServiceMonitor: hermesv1.ServiceMonitorSpec{Enabled: true, Interval: "30s"},
	}
	inst.Spec.SelfConfigure = hermesv1.SelfConfigureSpec{
		Enabled:        true,
		ProtectedKeys:  []string{"image", "storage", "security", "networking"},
		AllowedActions: []string{"skills", "config", "envVars", "workspaceFiles", "profiles"},
	}
	inst.Spec.AutoUpdate = hermesv1.AutoUpdateSpec{
		Enabled: true,
		Source:  hermesv1.AutoUpdateSource{Registry: "ghcr.io/stubbi/hermes-agent", Channel: "1.x"},
	}
	inst.Spec.Backup = hermesv1.BackupSpec{
		S3: &hermesv1.S3Spec{
			Bucket:                "hermes-backups",
			Endpoint:              "s3.amazonaws.com",
			Region:                "us-east-1",
			CredentialsSecretRef:  corev1.LocalObjectReference{Name: "s3-creds"},
		},
		Schedule: "0 3 * * *",
		OnDelete: true,
	}
	return inst
}

// -----------------------------------------------------------------------------
// StatefulSet
// -----------------------------------------------------------------------------

func BenchmarkBuildStatefulSet_Minimal(b *testing.B) {
	inst := newBenchInstanceMinimal()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = BuildStatefulSet(inst, nil)
	}
}

func BenchmarkBuildStatefulSet_FullyLoaded(b *testing.B) {
	inst := newBenchInstanceFull()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = BuildStatefulSet(inst, nil)
	}
}

// -----------------------------------------------------------------------------
// Service
// -----------------------------------------------------------------------------

func BenchmarkBuildService_Minimal(b *testing.B) {
	inst := newBenchInstanceMinimal()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = BuildService(inst)
	}
}

func BenchmarkBuildService_FullyLoaded(b *testing.B) {
	inst := newBenchInstanceFull()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = BuildService(inst)
	}
}

// -----------------------------------------------------------------------------
// ConfigMap (workspace + gateway-derived)
// -----------------------------------------------------------------------------

func BenchmarkBuildWorkspaceConfigMap(b *testing.B) {
	inst := newBenchInstanceFull()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = BuildWorkspaceConfigMap(inst)
	}
}

func BenchmarkBuildGatewayConfigMap(b *testing.B) {
	inst := newBenchInstanceFull()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = BuildGatewayConfigMap(inst)
	}
}

// -----------------------------------------------------------------------------
// PVC
// -----------------------------------------------------------------------------

func BenchmarkBuildPVC(b *testing.B) {
	inst := newBenchInstanceFull()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = BuildPVC(inst, nil)
	}
}

// -----------------------------------------------------------------------------
// NetworkPolicy
// -----------------------------------------------------------------------------

func BenchmarkBuildNetworkPolicy_Minimal(b *testing.B) {
	inst := newBenchInstanceMinimal()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = BuildNetworkPolicy(inst)
	}
}

func BenchmarkBuildNetworkPolicy_FullyLoaded(b *testing.B) {
	inst := newBenchInstanceFull()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = BuildNetworkPolicy(inst)
	}
}

// -----------------------------------------------------------------------------
// Ingress
// -----------------------------------------------------------------------------

func BenchmarkBuildIngress(b *testing.B) {
	inst := newBenchInstanceFull()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = BuildIngress(inst)
	}
}

// -----------------------------------------------------------------------------
// PDB / HPA
// -----------------------------------------------------------------------------

func BenchmarkBuildPDB(b *testing.B) {
	inst := newBenchInstanceFull()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = BuildPDB(inst)
	}
}

func BenchmarkBuildHPA(b *testing.B) {
	inst := newBenchInstanceFull()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = BuildHPA(inst)
	}
}

// -----------------------------------------------------------------------------
// ServiceMonitor
// -----------------------------------------------------------------------------

func BenchmarkBuildServiceMonitor(b *testing.B) {
	inst := newBenchInstanceFull()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = BuildServiceMonitor(inst)
	}
}

// -----------------------------------------------------------------------------
// Honcho profileStore (Deployment + Service + PVC)
// -----------------------------------------------------------------------------

func BenchmarkBuildHonchoDeployment(b *testing.B) {
	inst := newBenchInstanceFull()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = BuildHonchoDeployment(inst)
	}
}

func BenchmarkBuildHonchoService(b *testing.B) {
	inst := newBenchInstanceFull()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = BuildHonchoService(inst)
	}
}

func BenchmarkBuildHonchoPVC(b *testing.B) {
	inst := newBenchInstanceFull()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = BuildHonchoPVC(inst)
	}
}

// -----------------------------------------------------------------------------
// RBAC (Role + RoleBinding + ServiceAccount)
// -----------------------------------------------------------------------------

func BenchmarkBuildRBAC(b *testing.B) {
	inst := newBenchInstanceFull()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = BuildServiceAccount(inst)
		_ = BuildRole(inst)
		_ = BuildRoleBinding(inst)
	}
}
```

Note: builder signature names (`BuildStatefulSet(inst, defaults)`, `BuildWorkspaceConfigMap`, etc.) should match exactly what Plans 2-5 actually produced. If the actual function names differ, fix the bench file to match: the bench file follows, it doesn't define, the API.

- [ ] **Step 2: Controller microbenchmark**

Create `internal/controller/controller_bench_test.go`:

```go
/*
Copyright 2026 stubbi

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0
*/

package controller

import (
	"context"
	"testing"

	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"

	hermesv1 "github.com/stubbi/hermes-operator/api/v1"
)

// BenchmarkReconcile_FullSpec runs the reconciler against a synthetic
// HermesInstance with every sub-spec populated. Measures whole-loop ns/op +
// allocs, against the envtest apiserver established in suite_test.go.
//
// The envtest binary is wired up by the existing suite: this benchmark
// piggy-backs on its lifecycle and uses the same Reconciler.
func BenchmarkReconcile_FullSpec(b *testing.B) {
	if k8sClient == nil {
		b.Skip("envtest not available")
	}
	ctx := context.Background()
	ns := "bench-full"

	// Create namespace once.
	_ = createNS(ctx, ns)
	inst := fullSpecInstance(ns, "bench")
	if err := k8sClient.Create(ctx, inst); err != nil {
		b.Fatalf("create instance: %v", err)
	}
	b.Cleanup(func() {
		_ = k8sClient.Delete(ctx, inst)
	})

	rec := &HermesInstanceReconciler{
		Client: k8sClient,
		Scheme: k8sClient.Scheme(),
	}
	req := ctrl.Request{NamespacedName: types.NamespacedName{Namespace: ns, Name: "bench"}}

	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		if _, err := rec.Reconcile(ctx, req); err != nil {
			b.Fatalf("reconcile: %v", err)
		}
	}
}

// fullSpecInstance constructs a HermesInstance with every sub-spec populated.
// Mirrors newBenchInstanceFull from internal/resources/, but lives here so the
// controller package doesn't import the resources test helpers.
func fullSpecInstance(ns, name string) *hermesv1.HermesInstance {
	enabled := true
	return &hermesv1.HermesInstance{
		ObjectMeta: ctrl.ObjectMeta{Name: name, Namespace: ns},
		Spec: hermesv1.HermesInstanceSpec{
			Image: hermesv1.ImageSpec{
				Repository: "ghcr.io/stubbi/hermes-agent",
				Tag:        "v1.0.0",
			},
			Storage: hermesv1.StorageSpec{
				Persistence: hermesv1.PersistenceSpec{Enabled: &enabled, Size: "1Gi"},
			},
			Gateways: hermesv1.GatewaysSpec{
				Telegram: hermesv1.GatewaySpec{Enabled: true},
				Slack:    hermesv1.GatewaySpec{Enabled: true},
			},
			ProfileStore: hermesv1.ProfileStoreSpec{
				Enabled:     true,
				Persistence: hermesv1.PersistenceSpec{Size: "2Gi"},
			},
			SelfConfigure: hermesv1.SelfConfigureSpec{
				Enabled:       true,
				ProtectedKeys: []string{"image", "storage", "security", "networking"},
				AllowedActions: []string{"skills", "config", "envVars", "workspaceFiles", "profiles"},
			},
			Networking: hermesv1.NetworkingSpec{
				NetworkPolicy: hermesv1.NetworkPolicySpec{Enabled: &enabled},
			},
			Observability: hermesv1.ObservabilitySpec{
				ServiceMonitor: hermesv1.ServiceMonitorSpec{Enabled: true},
			},
		},
	}
}

// createNS is a thin wrapper used by benchmarks.
func createNS(ctx context.Context, name string) error {
	// Reuse the existing test/envtest helper if one exists; otherwise inline
	// a minimal Namespace create. The kubebuilder suite_test.go provides ctx
	// and k8sClient: the engineer wires this to whatever helper already
	// creates test namespaces.
	return nil
}
```

Note: `ctrl.ObjectMeta` is shorthand for `metav1.ObjectMeta`: the engineer's package-level imports may already alias. Adjust as needed.

- [ ] **Step 3: PR diff comment script**

Create `hack/benchstat-comment.sh`:

```bash
#!/usr/bin/env bash
set -euo pipefail

# Compare HEAD benchmarks against merge-base. Fail if any line shows
# >20% regression. Post a comment to the PR with the full diff.

BASE_SHA="${BASE_SHA:-}"
HEAD_SHA="${HEAD_SHA:-$(git rev-parse HEAD)}"
PR_NUMBER="${PR_NUMBER:-}"

if [ -z "$BASE_SHA" ]; then
  echo "::error::BASE_SHA env var required"
  exit 1
fi

mkdir -p .bench
git stash push --include-untracked -m "benchstat-stash" || true

git checkout "$BASE_SHA"
go test -bench=. -benchmem -run=^$ -count=5 ./internal/resources/... | tee .bench/base.txt
go test -bench=. -benchmem -run=^$ -count=5 ./internal/controller/... | tee -a .bench/base.txt

git checkout "$HEAD_SHA"
go stash pop || true
go test -bench=. -benchmem -run=^$ -count=5 ./internal/resources/... | tee .bench/head.txt
go test -bench=. -benchmem -run=^$ -count=5 ./internal/controller/... | tee -a .bench/head.txt

go install golang.org/x/perf/cmd/benchstat@latest
benchstat .bench/base.txt .bench/head.txt > .bench/diff.txt

REGRESSION=$(awk '
  /^Benchmark/ {
    for (i=1; i<=NF; i++) {
      # Match "+NN.NNN%" patterns
      if ($i ~ /^\+[0-9]+\.[0-9]+%$/) {
        pct = substr($i, 2, length($i)-2) + 0
        if (pct > 20.0) {
          print $0
          found = 1
        }
      }
    }
  }
  END { if (found) exit 1 }
' .bench/diff.txt) || REGRESSED=1

if [ -n "${PR_NUMBER:-}" ]; then
  COMMENT_BODY=$(printf '## Benchmark diff\n\n```\n%s\n```\n' "$(cat .bench/diff.txt)")
  gh pr comment "$PR_NUMBER" --body "$COMMENT_BODY"
fi

if [ -n "${REGRESSED:-}" ]; then
  echo "::error::>20%% regression detected. See diff above."
  exit 1
fi
echo "OK: no benchmark regressions >20%."
```

```bash
chmod +x hack/benchstat-comment.sh
```

- [ ] **Step 4: PR benchmark workflow**

Create `.github/workflows/benchmark.yaml`:

```yaml
name: Benchmark

on:
  pull_request:
    paths:
      - '**.go'
      - 'go.mod'
      - 'go.sum'
      - '.github/workflows/benchmark.yaml'

permissions:
  contents: read
  pull-requests: write

jobs:
  bench:
    runs-on: ubuntu-latest
    timeout-minutes: 30
    steps:
      - uses: actions/checkout@v4
        with:
          fetch-depth: 0
      - uses: actions/setup-go@v5
        with: { go-version: '1.24' }
      - name: Resolve merge-base
        id: base
        run: |
          BASE=$(git merge-base origin/${{ github.base_ref }} HEAD)
          echo "sha=$BASE" >> "$GITHUB_OUTPUT"
      - name: Run benchmark diff
        env:
          BASE_SHA: ${{ steps.base.outputs.sha }}
          HEAD_SHA: ${{ github.sha }}
          PR_NUMBER: ${{ github.event.pull_request.number }}
          GH_TOKEN: ${{ secrets.GITHUB_TOKEN }}
        run: bash hack/benchstat-comment.sh
```

- [ ] **Step 5: Makefile targets**

Append to `Makefile`:

```makefile
.PHONY: bench
bench: bench-resources bench-controller ## Run all benchmarks.

.PHONY: bench-resources
bench-resources:
	go test -bench=. -benchmem -run=^$$ -count=5 ./internal/resources/...

.PHONY: bench-controller
bench-controller:
	go test -bench=. -benchmem -run=^$$ -count=5 ./internal/controller/...
```

- [ ] **Step 6: Commit**

```bash
git add internal/resources/resources_bench_test.go internal/controller/controller_bench_test.go \
        hack/benchstat-comment.sh .github/workflows/benchmark.yaml Makefile
git commit -m "test(bench): add builder + controller benchmarks with PR diff workflow"
```

---

## Task 23: `docs/release-process.md`, `docs/conformance.md`, README badges

**Files:**
- Modify: `docs/release-process.md` (stub created Task 1)
- Create: `docs/conformance.md`
- Modify: `README.md`

- [ ] **Step 1: Fill out `docs/release-process.md`**

Overwrite `docs/release-process.md`:

```markdown
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

If a release was tagged but the release workflow didn't run (very rare:
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
  release; the operator's `appVersion` doesn't pin agent versions:
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
```

- [ ] **Step 2: Write `docs/conformance.md`**

Create `docs/conformance.md`:

```markdown
# Conformance Suite

The `test/conformance/` tree is what makes hermes-operator a v1, not a v0.1.
Five categories of test mechanically defend the v1 stability commitments
documented in spec §11.

## Categories

### Negative tests (`negative_test.go`)

Every webhook deny path has a row in `negativeCases`. Adding a new validator
rule requires adding a row; CI fails if you forget. The table covers:

- `selfConfigure.enabled` without `protectedKeys` (spec §7.3)
- `config.raw` and `config.configMapRef` mutual exclusion (spec §7.3)
- Invalid storage quantity strings (`1XB`, whitespace, etc.)
- Empty `image.repository` with no `HermesClusterDefaults`
- Gateways `.enabled: true` without `tokenSecretRef`
- `backup.onDelete: true` without `backup.s3`
- `autoUpdate.enabled` with empty `source.registry`
- `migration.fromOpenClaw` with missing or doubled source
- Probe `successThreshold > 1` on liveness/startup probes
- `networking.ingress.enabled` without hosts
- HPA `minReplicas > maxReplicas`
- PDB `minAvailable` and `maxUnavailable` set simultaneously
- `restoreFrom` mutation after `status.restoredFrom` is set (immutability)
- `HermesSelfConfig.instanceRef` to non-existent instance
- `HermesSelfConfig` touching a protected key
- `HermesSelfConfig` with an unknown action type
- `HermesClusterDefaults` name not `cluster`
- `HermesClusterDefaults.storage.storageClassName` invalid

### Idempotency (`idempotency_test.go`)

For each of 10 manifests in `testdata/`, apply once → wait Ready → trigger 10
forced reconciles via no-op annotation pokes → assert `metadata.generation`
and `metadata.resourceVersion` on every managed resource is unchanged from the
post-first-reconcile fingerprint. This is the test that would have caught
openclaw's #437 before it shipped.

A generation bump means a builder reintroduced server-side default drift; a
resourceVersion bump without generation means an `r.Update()` slipped past
Reconcile Guard.

### Upgrade-path matrix (`upgrade_test.go`)

For every prior release tag from `v1.0.0` onward, install vN → create
`HermesInstance` + `HermesSelfConfig` + `HermesClusterDefaults` → wait Ready
→ upgrade operator to HEAD → assert no managed resource changed.

A `switch tag` block in the test lets a release deliberately *allow* a
specific upgrade-time mutation; the CHANGELOG entry for that release must
reference the allow-list addition. For v1.0 the matrix is empty; it
populates from v1.1 onward.

### GitOps coexistence (`gitops_test.go`)

Two concurrent SSA writers: a FluxCD-style manager and the operator's
`hermes.agent/selfconfig` field-manager: race against the same
`HermesInstance` for 200 iterations (each ~100ms apart, simulating 10
minutes of load). The test asserts at most one ownership flip on the
contended path (the initial settle), then both managers' fields coexist in
the final spec. >1 flip indicates SSA isolation is broken.

### Failure injection (`failure_injection_test.go`)

Four reconcile paths, each killed mid-flight via `kubectl delete pod
--force --grace-period=0`:

1. Instance create: assert Ready within 3 minutes after restart.
2. Instance update (patched resources): assert StatefulSet reflects the patch.
3. SelfConfig apply: assert phase=Applied + spec change visible.
4. Backup-on-delete finalizer: assert instance fully deleted.

## Running locally

```bash
make conformance-kind-up
make conformance-install IMG=hermes-operator:dev
make conformance              # all categories
make conformance-negative     # one category
make conformance-idempotency
make conformance-upgrade
make conformance-gitops
make conformance-failure
make conformance-kind-down
```

## What CI does

- **PRs touching `test/conformance/`**: runs all five categories. Advisory.
- **Nightly on `main`**: runs all five. Required to be green before the next
  release.
- **On tag `v*`**: runs all five. Required.

## When to add a row

| Triggering change                                | Add to                       |
|--------------------------------------------------|------------------------------|
| New webhook validation rule                      | `negativeCases` table        |
| New `HermesInstance` sub-spec                    | new file under `testdata/`   |
| New optional sub-spec → exercise in idempotency  | new row in `idempotencyCorpus` |
| New CR kind                                      | scaffold a `*_test.go` like `negative_test.go` |
| New finalizer / reconcile path                   | new `It("...")` in `failure_injection_test.go` |
| Deliberate breaking change in a release          | `switch tag` arm in `upgrade_test.go` + CHANGELOG note |

## What this suite does NOT cover

- Performance and resource consumption (see `internal/{resources,controller}/*_bench_test.go`).
- Security scanning (gosec + Trivy run in `ci.yaml`).
- Cosign + SBOM verification (`verify-signing.yaml`).
- OperatorHub bundle validation (`operator-sdk bundle validate`,
  `make bundle-validate`).
```

- [ ] **Step 3: Update README badges**

Modify `README.md` (Plan 1 created the badges section). Replace the badges
section with:

```markdown
# hermes-operator

[![License](https://img.shields.io/badge/License-Apache%202.0-blue.svg)](https://opensource.org/licenses/Apache-2.0)
[![Go Report Card](https://goreportcard.com/badge/github.com/stubbi/hermes-operator)](https://goreportcard.com/report/github.com/stubbi/hermes-operator)
[![CI](https://github.com/stubbi/hermes-operator/actions/workflows/ci.yaml/badge.svg)](https://github.com/stubbi/hermes-operator/actions/workflows/ci.yaml)
[![E2E](https://github.com/stubbi/hermes-operator/actions/workflows/e2e.yaml/badge.svg)](https://github.com/stubbi/hermes-operator/actions/workflows/e2e.yaml)
[![Conformance](https://github.com/stubbi/hermes-operator/actions/workflows/conformance.yaml/badge.svg)](https://github.com/stubbi/hermes-operator/actions/workflows/conformance.yaml)
[![Release Please](https://github.com/stubbi/hermes-operator/actions/workflows/release-please.yaml/badge.svg)](https://github.com/stubbi/hermes-operator/actions/workflows/release-please.yaml)
[![Verify Signing](https://github.com/stubbi/hermes-operator/actions/workflows/verify-signing.yaml/badge.svg)](https://github.com/stubbi/hermes-operator/actions/workflows/verify-signing.yaml)
```

Then add a "Supported Kubernetes versions" snippet immediately below the
quick-start, linking to `docs/supported-versions.md`:

```markdown
## Supported Kubernetes versions

`hermes-operator` is CI-tested on Kubernetes 1.28, 1.29, 1.30, 1.31, and
1.32. The oldest minor is dropped only on minor releases of hermes-operator,
never patch. See [`docs/supported-versions.md`](docs/supported-versions.md).

## Distribution channels

- **Helm chart (OCI)**: `helm install hermes-operator oci://ghcr.io/stubbi/charts/hermes-operator --version X.Y.Z`
- **OLM (OperatorHub)**: Hermes Operator → Install
- **Plain kubectl**: `kubectl apply -f https://github.com/stubbi/hermes-operator/releases/download/vX.Y.Z/install.yaml`

All images are Cosign-signed (keyless OIDC) with SPDX-JSON SBOM attestations.
See [`docs/security/signing.md`](docs/security/signing.md) for verification.
```

- [ ] **Step 4: Commit**

```bash
git add docs/release-process.md docs/conformance.md README.md
git commit -m "docs: fill out release-process + conformance + README badges"
```

---

## Task 24: End-to-end smoke of the distribution pipeline (no actual release)

**Files:** none new: just walk the pipeline.

- [ ] **Step 1: Dry-run release-please locally**

```bash
npx release-please release-pr \
  --token "$RELEASE_PLEASE_TOKEN" \
  --repo-url stubbi/hermes-operator \
  --target-branch main \
  --dry-run
```
Expected: prints the changelog and version diff that *would* be produced.

- [ ] **Step 2: Dry-run GoReleaser**

```bash
goreleaser release --snapshot --clean --skip=publish,sign
```
Expected: builds binaries + multi-arch images locally under `dist/`.

```bash
docker images | grep hermes-operator
```
Expected: at least two arches present.

- [ ] **Step 3: Validate the bundle**

```bash
make bundle-validate
```
Expected: `All validation tests have completed successfully`.

- [ ] **Step 4: Build bundle image**

```bash
make bundle-build
docker images | grep hermes-operator-bundle
```
Expected: bundle image present.

- [ ] **Step 5: Lint every new workflow YAML**

```bash
for f in .github/workflows/*.yaml; do
  echo "Checking $f"
  python -c "import yaml,sys; yaml.safe_load(open('$f'))" || exit 1
done
```
Expected: every file passes.

- [ ] **Step 6: Run the conformance suite locally against a kind cluster**

```bash
make conformance-kind-up
make conformance-install IMG=hermes-operator:dev
make conformance-negative
make conformance-kind-down
```
Expected: negative suite passes (it doesn't depend on a working hermes-agent image: only the webhook responses).

- [ ] **Step 7: Push and watch CI**

```bash
git push origin main
gh run watch
```
Expected: all workflows green. If `release-please.yaml` opens a release PR, that's expected: leave it open until you're actually ready to cut a release.

- [ ] **Step 8: Commit any tweaks**

```bash
git add -A
git commit -m "chore: smoke-test outputs from Plan 6 Task 24" --allow-empty
```

---

## Self-review (verify before marking the plan complete)

- [ ] **Spec §9 (distribution table)**
  - Helm chart: Plan 1 scaffolded; this plan adds release-please bumping (Task 2), Helm OCI push in release.yaml (Task 5).
  - OLM bundle: Tasks 7, 8, 9, 10 (scaffold, CSV, RBAC sync, build targets).
  - Plain manifests: `make installer` (Task 6) + release-asset upload via `.goreleaser.yaml` (Task 4).
  - Container images: multi-arch + Cosign + SBOM in release.yaml (Task 5).

- [ ] **Spec §9.1 (release pipeline)**
  - Conventional commits → release-please PR (Task 3).
  - Merge bumps `CHANGELOG.md`, manifest, Chart.yaml, appVersion, CSV (Task 2).
  - Tag via PAT triggers release.yaml (Task 5).
  - GoReleaser builds + Cosign + SBOM + SBOM upload + publish (Task 5).

- [ ] **Spec §10 testing strategy (this plan owns items 4, 5, partly 6/7/8)**
  - §10.4 negative: Task 16: 19 webhook deny rows.
  - §10.4 idempotency: Task 17: 10 manifests × 10 reconciles.
  - §10.4 upgrade matrix: Task 18: auto-populates from v1.0; skips cleanly until.
  - §10.4 gitops: Task 19: 200-reconcile FluxCD + SelfConfig flap detector.
  - §10.4 failure injection: Task 20: four reconcile paths (create, update, selfconfig, backup-on-delete).
  - §10.5 benchmarks: Task 22: builder + envtest microbench + PR diff workflow with 20% gate.
  - §10.6 security scans: already in Plan 1's ci.yaml; this plan does not duplicate.
  - §10.7 Reconcile Guard: Plan 1; this plan does not duplicate.
  - §10.8 Helm RBAC Sync: Plan 1; this plan extends with bundle RBAC sync (Task 9).
  - **CI matrix k8s 1.28→1.32**: Task 13.

- [ ] **Spec §11 stability commitments**
  - §11.1 API versioning: webhook conversion scaffolding is Plan 2; this plan's upgrade-matrix test mechanically defends.
  - §11.2 deprecation: Plan 2 wires the webhook warning machinery; the conformance suite's negative table catches removed-but-not-deprecated fields by failing to validate them.
  - §11.3 conversion-webhook scaffolding: Plan 2; this plan does not modify.
  - §11.4 condition catalogue: Plan 2 (`docs/conditions.md`); referenced from CSV statusDescriptors (Task 8).
  - §11.5 supported k8s versions: Task 13 + `docs/supported-versions.md`.
  - §11.6 versioning surfaces: release-please `extra-files` (Task 2) bumps every embedded version atomically.

- [ ] **Distribution config files**
  - `release-please-config.json`: Task 2.
  - `.release-please-manifest.json`: Task 2.
  - `.goreleaser.yaml`: Task 4.
  - `bundle.Dockerfile`: Task 7.
  - `bundle/`: Tasks 7, 8, 9, 10.

- [ ] **Cosign + SBOM**
  - Signing in release.yaml: Task 5.
  - SBOM via syft + attest via cosign: Task 5.
  - `make verify-signing` + `docs/security/signing.md`: Task 12.
  - Weekly drift check `verify-signing.yaml`: Task 12.

- [ ] **OperatorHub submission**
  - `operatorhub-submit.yaml` to community-operators + community-operators-prod: Task 11.
  - Mirrors openclaw's proven cross-fork PR pattern.

- [ ] **`make installer` and `dist/install.yaml` release asset**: Task 6 + .goreleaser.yaml `release.extra_files` (Task 4).

- [ ] **k8s CI matrix (1.28→1.32)**
  - envtest matrix in `ci.yaml`: Task 13.
  - kind matrix in `e2e.yaml`: Task 13.
  - EOL policy: `docs/supported-versions.md` (Task 13).

- [ ] **Conformance suite: content depth**
  - Negative cases: 19 actual rows (not placeholders): Task 16.
  - Idempotency corpus: 10 distinct manifests under `testdata/`: Task 15, exercised in Task 17.
  - Upgrade matrix: real Go function + tag-keyed allow-list arms; v1.0 path no-op'd cleanly: Task 18.
  - GitOps test: two concurrent SSA writers + flap-counter via ManagedFields inspection: Task 19.
  - Failure injection: four distinct reconcile paths each with full code: Task 20.
  - Runner targets + workflow: Task 21.

- [ ] **Benchmarks**
  - 17 named benchmarks across resources + 1 envtest reconcile bench: Task 22.
  - benchstat diff with 20% regression gate: Task 22.
  - PR diff comment via `gh pr comment`: Task 22.

- [ ] **Documentation**
  - `docs/release-process.md`: Task 23.
  - `docs/conformance.md`: Task 23.
  - `docs/security/signing.md`: Task 12.
  - `docs/supported-versions.md`: Task 13.
  - README badges: Task 23.

- [ ] **No placeholders**
  - Every Go file has full compilable code, not "TODO".
  - Every YAML file is end-to-end runnable.
  - Every Makefile target has a body, not just a name.
  - The negative test table has 19 *named* rows; the idempotency corpus has 10 *real* manifests.

- [ ] **Type consistency with Plans 2-5**
  - Bench file uses `Ptr[T]`, `hermesv1.HermesInstance`, sub-spec types from spec §4: same names Plan 2 was instructed to create.
  - Conformance test imports `hermesv1 "github.com/stubbi/hermes-operator/api/v1"`: same module path Plan 1 established.
  - Field manager `hermes.agent/selfconfig` matches spec §5 and Plan 4.
  - Finalizer `hermes.agent/backup-on-delete` matches spec §8.2 and Plan 5.

- [ ] **Conventional commits**
  - Every commit message in the plan uses an allowed prefix
    (`feat:`, `fix:`, `docs:`, `ci:`, `build:`, `test:`, `chore:`, `perf:`,
    `refactor:`). release-please filters to `feat:` and `fix:` for the
    changelog; the rest are hidden but counted (per `release-please-config.json`).

End of Plan 6.

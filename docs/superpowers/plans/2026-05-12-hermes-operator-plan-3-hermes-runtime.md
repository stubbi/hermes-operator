# Hermes Operator: Plan 3: Hermes Runtime Specifics

> **For agentic workers:** REQUIRED SUB-SKILL: Use `superpowers:subagent-driven-development` (recommended) or `superpowers:executing-plans` to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Land every Python/uv-shaped runtime concern that distinguishes hermes-operator from openclaw-operator: build and publish the `ghcr.io/stubbi/hermes-agent` container image, wire `spec.runtime`/`spec.gateways`/`spec.profileStore` into the `HermesInstance` builders, stand up the Honcho companion service, and document the platform-gateway token surface: so that a `HermesInstance` declaring `gateways.telegram.enabled: true` + `profileStore.honcho.enabled: true` reconciles into a working hermes-agent pod with Honcho beside it, on a default-deny NetworkPolicy that allows exactly the upstream endpoints required.

**Architecture:** Three new builder files (`internal/resources/runtime_init.go`, `internal/resources/gateways.go`, `internal/resources/honcho.go`) extend the StatefulSet/ConfigMap/NetworkPolicy primitives from Plans 1+2. The agent image is built from the upstream Python package against a committed `uv.lock`, multi-arch via QEMU+buildx, signed and SBOM-attested by the same GoReleaser/Cosign pipeline Plan 1 set up for the operator image. Webhook warnings (not denials) cover secret-not-found cases so that GitOps users can apply a HermesInstance and its secrets in either order without races. The Honcho profile store runs as a sibling Deployment + headless Service + PVC in the same namespace as the HermesInstance, with NetworkPolicy isolating it to the parent hermes pod only.

**Tech Stack:** Go 1.24, controller-runtime, kubebuilder v4, Ginkgo v2 + Gomega, envtest, Docker Buildx (multi-arch), Cosign, Syft (SBOM), Python 3.11 + uv, ffmpeg, ripgrep, tini, GitHub Actions matrix builds, NetworkPolicy v1, plastic-labs/honcho.

**Prerequisite:** Plans 1 (Foundation) and 2 (Full reconciler + webhooks) merged. Plan 2 must have shipped: full `spec.image`/`spec.config`/`spec.networking`/`spec.security`/`spec.env`/`spec.envFrom`/`spec.skills`/`spec.selfConfigure` typed sub-specs on `HermesInstanceSpec`; the `HermesInstance` validating webhook + defaulter wired through cert-manager; the workspace ConfigMap builder at `internal/resources/configmap.go` with a `workspace.go` companion file; `internal/resources/networkpolicy.go` returning a default-deny `NetworkPolicy` with a typed extension point (`func ExtraEgressRules(inst *hermesv1.HermesInstance) []networkingv1.NetworkPolicyEgressRule`); the validator stub at `internal/webhook/hermesinstance_validator.go` that this plan extends; and the `+listType=map +listMapKey=name` markers on `spec.env`/`spec.envFrom`/`spec.skills`. Reconcile Guard CI is enabled.

**Spec reference:** [`docs/superpowers/specs/2026-05-12-hermes-operator-design.md`](../specs/2026-05-12-hermes-operator-design.md) §1 (Context & Goals), §4.1 (Hermes-specific deltas), §4 (`gateways`, `profileStore`, `runtime` top-level sub-specs), §7.4 (operational guardrails: esp. tini PID 1 and init-container full-volume-mount rule).

---

## File Structure Established by This Plan

```
.
├── api/v1/
│   └── hermesinstance_types.go              # MODIFY: add RuntimeSpec, GatewaysSpec, ProfileStoreSpec sub-specs
├── images/
│   └── hermes-agent/                        # NEW: agent image build context
│       ├── Dockerfile                       # multi-stage, multi-arch (amd64+arm64)
│       ├── uv.lock                          # committed lockfile for reproducible builds
│       ├── pyproject.toml                   # uv project metadata pinning hermes-agent + extras
│       ├── entrypoint.sh                    # tini-wrapped startup script
│       ├── .dockerignore
│       └── README.md                        # how to relock, how to build locally
├── internal/
│   ├── resources/
│   │   ├── runtime_init.go                  # NEW: init containers for uv sync, apt, pip
│   │   ├── runtime_init_test.go             # NEW
│   │   ├── gateways.go                      # NEW: envFrom builders + gateway config.yaml fragments + egress rules
│   │   ├── gateways_test.go                 # NEW
│   │   ├── honcho.go                        # NEW: Deployment + Service + PVC + Secret reference
│   │   ├── honcho_test.go                   # NEW
│   │   ├── statefulset.go                   # MODIFY: pull init containers + gateway envFrom + honcho env
│   │   ├── configmap.go                     # MODIFY: merge gateway config fragments into config.yaml
│   │   └── networkpolicy.go                 # MODIFY: append per-gateway egress + honcho ingress rules
│   ├── controller/
│   │   ├── hermesinstance_controller.go     # MODIFY: reconcile Honcho resources + ProfileStoreReady condition
│   │   └── hermesinstance_controller_test.go# MODIFY: envtest cases for runtime/gateways/honcho
│   └── webhook/
│       └── hermesinstance_validator.go      # MODIFY: gateway secret existence warnings; profiles allowedAction
├── config/crd/bases/
│   └── hermes.agent_hermesinstances.yaml    # REGENERATE
├── charts/hermes-operator/
│   └── values.yaml                          # MODIFY: agent image default tag, gateway egress allowlist defaults
├── docs/
│   ├── api-reference.md                     # MODIFY: append runtime/gateways/profileStore sections
│   ├── conventions.md                       # MODIFY: append "Well-known egress endpoints" section
│   └── runbook-platform-gateways.md         # NEW: per-platform token/scope/rotation guide
├── test/
│   └── e2e/
│       ├── gateways_honcho_test.go          # NEW: end-to-end on kind with dummy secrets
│       └── testdata/
│           └── hermesinstance-gateways.yaml # NEW: sample manifest
├── .github/workflows/
│   ├── agent-image.yaml                     # NEW: matrix build over HERMES_VERSION
│   └── agent-image-smoke.yaml               # NEW: per-PR smoke test of built image
├── Makefile                                 # MODIFY: targets `agent-image-relock`, `agent-image-build`, `agent-image-smoke`
└── README.md                                # MODIFY: feature table rows for runtime/gateways/profileStore
```

---

## Task 1: Scaffold the `images/hermes-agent/` build context

**Files:**
- Create: `images/hermes-agent/.dockerignore`, `images/hermes-agent/pyproject.toml`, `images/hermes-agent/README.md`
- Modify: none

- [ ] **Step 1: Create the directory layout**

```bash
cd /Users/jannesstubbemann/repos/hermes-operator
mkdir -p images/hermes-agent
```

- [ ] **Step 2: Verify the upstream package name + CLI shape**

```bash
gh repo view nousresearch/hermes-agent
gh api repos/nousresearch/hermes-agent/contents/pyproject.toml --jq .content | base64 -d | grep -E '^name|^version|^\[project|scripts' | head -20
```
Expected: confirm `name = "hermes-agent"` and a `hermes-agent` console script under `[project.scripts]`. Note the upstream **package version** that you will pin in the next step (call it `HERMES_VERSION`: default to the latest release tag returned by `gh release view --repo nousresearch/hermes-agent --json tagName --jq .tagName`).

If upstream uses a different package name (`hermes_agent` vs `hermes-agent`), record the actual name as `HERMES_PKG_NAME` for the next step.

- [ ] **Step 3: Create `images/hermes-agent/pyproject.toml`**

```toml
# images/hermes-agent/pyproject.toml
#
# uv project that pins hermes-agent + its runtime extras for reproducible image builds.
# The package list and version are bumped via `make agent-image-relock` whenever a new
# upstream release is shipped through the operator's matrix workflow.

[project]
name = "hermes-agent-runtime"
version = "0.0.0"
description = "uv project pinning hermes-agent for the operator-published runtime image"
requires-python = ">=3.11"
dependencies = [
    # Upstream package. Version is overridden at build time by the HERMES_VERSION build-arg
    # via `uv add hermes-agent==${HERMES_VERSION}` before `uv sync --frozen`.
    "hermes-agent",
]

[tool.uv]
# Refuse to silently mutate the lockfile during image builds.
frozen = true
```

> **Note:** the build flow is: (1) `uv add hermes-agent==<version>` regenerates the lock for the requested version, (2) the lockfile is committed to this repo, (3) the Dockerfile copies the lock and runs `uv sync --frozen`. This guarantees that two builds of the same operator commit produce byte-identical Python environments.

- [ ] **Step 4: Create `images/hermes-agent/.dockerignore`**

```
# images/hermes-agent/.dockerignore
**/__pycache__
**/*.pyc
**/.venv
**/.uv-cache
README.md
```

- [ ] **Step 5: Create a brief `images/hermes-agent/README.md`**

```markdown
# hermes-agent image build context

The operator owns `ghcr.io/stubbi/hermes-agent`. Upstream
(`nousresearch/hermes-agent`) ships only a Python package, so this directory
packages it into a multi-arch container that the operator can pull by default.

## Layout

| File | Purpose |
|---|---|
| `Dockerfile` | Multi-stage build (uv builder + slim runtime). |
| `pyproject.toml` | uv project pinning `hermes-agent`. |
| `uv.lock` | Committed lockfile: reproducible builds. |
| `entrypoint.sh` | tini-wrapped startup; sources `~/.hermes/config.yaml`. |

## Common workflows

```bash
# Bump the pinned upstream version and refresh the lockfile.
make agent-image-relock HERMES_VERSION=1.4.3

# Build locally for the current platform.
make agent-image-build HERMES_VERSION=1.4.3

# Smoke-test the local build.
make agent-image-smoke HERMES_VERSION=1.4.3
```

CI builds the matrix in `.github/workflows/agent-image.yaml`, signs each image
with Cosign (keyless OIDC), and attaches an SBOM via Syft.
```

- [ ] **Step 6: Verify directory contents**

```bash
ls -la images/hermes-agent/
```
Expected: four files (`.dockerignore`, `pyproject.toml`, `README.md`, and the directory itself).

- [ ] **Step 7: Commit**

```bash
git add images/hermes-agent/
git commit -m "feat(images): scaffold hermes-agent image build context (pyproject.toml, .dockerignore, README)"
```

---

## Task 2: Author the `images/hermes-agent/Dockerfile`

**Files:**
- Create: `images/hermes-agent/Dockerfile`, `images/hermes-agent/entrypoint.sh`

- [ ] **Step 1: Write `images/hermes-agent/Dockerfile`**

```dockerfile
# images/hermes-agent/Dockerfile
#
# Two-stage multi-arch build for ghcr.io/stubbi/hermes-agent.
# Stage 1 (`builder`) uses the official uv image to resolve and install the pinned
# Python environment into a self-contained venv at /opt/venv. Stage 2 (`runtime`)
# copies only the venv + minimal apt deps and runs as the non-root `hermes` user.
#
# Build args:
#   HERMES_VERSION  : the exact hermes-agent release to package (e.g. "1.4.3").
#                     Required. The CI workflow at .github/workflows/agent-image.yaml
#                     drives this from a matrix.
#   PYTHON_VERSION  : Python interpreter version. Default 3.11.
#   UV_VERSION      : uv tool version. Default 0.5.0 (pinned for reproducibility).
#   TINI_VERSION    : tini release tag (lesson openclaw #471: tini as PID 1).
#
# Output: a runtime image that, with no further configuration, runs
#         `hermes-agent --config ~/.hermes/config.yaml` as UID 1000.

ARG PYTHON_VERSION=3.11
ARG UV_VERSION=0.5.0
ARG TINI_VERSION=v0.19.0
ARG HERMES_VERSION

# ---------- Stage 1: builder ----------
FROM ghcr.io/astral-sh/uv:${UV_VERSION} AS uv

FROM python:${PYTHON_VERSION}-slim-bookworm AS builder
ARG HERMES_VERSION

# uv binary from the dedicated stage above.
COPY --from=uv /uv /uvx /usr/local/bin/

ENV UV_LINK_MODE=copy \
    UV_COMPILE_BYTECODE=1 \
    UV_PYTHON_DOWNLOADS=never \
    VIRTUAL_ENV=/opt/venv

WORKDIR /build

# Copy lockfile + project metadata first for layer caching.
COPY pyproject.toml uv.lock ./

# Override the pinned version with the requested HERMES_VERSION, then sync against
# the lockfile (which the operator repo's `make agent-image-relock` keeps in sync).
RUN --mount=type=cache,target=/root/.cache/uv \
    set -eux; \
    if [ -z "${HERMES_VERSION}" ]; then \
        echo "ERROR: HERMES_VERSION build-arg is required" >&2; exit 1; \
    fi; \
    uv venv /opt/venv; \
    uv sync --frozen --no-dev

# ---------- Stage 2: runtime ----------
FROM python:${PYTHON_VERSION}-slim-bookworm AS runtime
ARG TINI_VERSION
ARG HERMES_VERSION
ARG TARGETARCH

# Runtime apt packages:
#   - ffmpeg, ripgrep: hard dependencies of hermes-agent (audio + search)
#   - git, openssh-client: hermes-agent skill installs are git-based
#   - ca-certificates: TLS to platform gateways
#   - tini: PID 1 reaper (lesson openclaw-operator #471)
RUN apt-get update \
    && apt-get install -y --no-install-recommends \
        ffmpeg \
        ripgrep \
        git \
        openssh-client \
        ca-certificates \
        tini \
    && rm -rf /var/lib/apt/lists/*

# Non-root user matching the StatefulSet's runAsUser=1000 (Plan 1).
RUN groupadd --system --gid 1000 hermes \
    && useradd --system --uid 1000 --gid 1000 --create-home --shell /usr/sbin/nologin hermes \
    && mkdir -p /home/hermes/.hermes \
    && chown -R hermes:hermes /home/hermes

# Copy the resolved venv from the builder stage.
COPY --from=builder --chown=hermes:hermes /opt/venv /opt/venv
ENV PATH="/opt/venv/bin:${PATH}" \
    PYTHONUNBUFFERED=1 \
    PYTHONDONTWRITEBYTECODE=1 \
    HOME=/home/hermes

# Image metadata. The HERMES_VERSION label is the one the operator's autoupdate
# controller (Plan 5) compares against the registry tag.
LABEL org.opencontainers.image.title="hermes-agent" \
      org.opencontainers.image.source="https://github.com/stubbi/hermes-operator" \
      org.opencontainers.image.documentation="https://github.com/stubbi/hermes-operator/blob/main/images/hermes-agent/README.md" \
      org.opencontainers.image.licenses="MIT" \
      org.opencontainers.image.vendor="stubbi" \
      hermes.agent/version="${HERMES_VERSION}"

USER hermes
WORKDIR /home/hermes

COPY --chown=hermes:hermes entrypoint.sh /usr/local/bin/hermes-entrypoint
RUN chmod +x /usr/local/bin/hermes-entrypoint

# tini as PID 1 to reap any subprocess hermes-agent forks (audio transcoders,
# git operations, skill executors). `--` separates tini's flags from the wrapped
# entrypoint script.
ENTRYPOINT ["/usr/bin/tini", "--", "/usr/local/bin/hermes-entrypoint"]
CMD ["serve"]
```

- [ ] **Step 2: Write `images/hermes-agent/entrypoint.sh`**

```bash
#!/usr/bin/env bash
# images/hermes-agent/entrypoint.sh
#
# Wrapper around the hermes-agent CLI:
#   - Verifies ~/.hermes/config.yaml exists and is readable.
#   - Refuses to start if the StatefulSet builder failed to mount the
#     ConfigMap (a frequent symptom of operator regressions).
#   - Passes through to `hermes-agent <cmd>` so `docker run <img> migrate ...`
#     works for one-shot Jobs (the Plan 5 migration init container relies on this).
set -euo pipefail

HERMES_CONFIG="${HERMES_CONFIG:-/home/hermes/.hermes/config.yaml}"

if [[ ! -r "${HERMES_CONFIG}" ]]; then
    echo "hermes-entrypoint: config not readable at ${HERMES_CONFIG}" >&2
    echo "hermes-entrypoint: this usually means the operator failed to mount the ConfigMap subPath." >&2
    exit 78  # EX_CONFIG, matches sysexits.h
fi

# When invoked without a subcommand (k8s CMD = "serve"), use the default `run` form.
if [[ "${1:-serve}" == "serve" ]]; then
    shift || true
    exec hermes-agent run --config "${HERMES_CONFIG}" "$@"
fi

# Otherwise pass through verbatim: supports `migrate from-openclaw ...`, `version`, etc.
exec hermes-agent "$@"
```

- [ ] **Step 3: Validate the Dockerfile syntactically**

```bash
docker buildx build --check -f images/hermes-agent/Dockerfile images/hermes-agent/ 2>&1 | head -20
```
Expected: `--check` reports no warnings. (If `--check` is unsupported on the local Buildx version, run `hadolint images/hermes-agent/Dockerfile` instead.)

- [ ] **Step 4: Commit**

```bash
git add images/hermes-agent/Dockerfile images/hermes-agent/entrypoint.sh
git commit -m "feat(images): multi-stage Dockerfile for ghcr.io/stubbi/hermes-agent with tini PID 1"
```

---

## Task 3: Generate the initial `uv.lock` and Makefile targets

**Files:**
- Create: `images/hermes-agent/uv.lock` (via tooling)
- Modify: `Makefile`

- [ ] **Step 1: Append agent-image targets to `Makefile`**

Add the following section to the existing `Makefile` (right after the operator-image build targets that Plan 1 introduced):

```makefile
# -------- hermes-agent image (Plan 3) --------

AGENT_IMAGE         ?= ghcr.io/stubbi/hermes-agent
HERMES_VERSION      ?= 1.4.2
AGENT_IMAGE_PLATFORMS ?= linux/amd64,linux/arm64

# Build the agent image for the current platform. Local dev only.
.PHONY: agent-image-build
agent-image-build:
	docker build \
		--build-arg HERMES_VERSION=$(HERMES_VERSION) \
		-t $(AGENT_IMAGE):$(HERMES_VERSION) \
		images/hermes-agent

# Multi-arch build via buildx. Pushes only if PUSH=1.
.PHONY: agent-image-buildx
agent-image-buildx:
	docker buildx build \
		--platform $(AGENT_IMAGE_PLATFORMS) \
		--build-arg HERMES_VERSION=$(HERMES_VERSION) \
		$(if $(filter 1,$(PUSH)),--push,--load) \
		-t $(AGENT_IMAGE):$(HERMES_VERSION) \
		images/hermes-agent

# Refresh images/hermes-agent/uv.lock for the requested HERMES_VERSION.
# Runs `uv add hermes-agent==<version>` inside an ephemeral uv container so it
# works the same way on developer laptops and in CI.
.PHONY: agent-image-relock
agent-image-relock:
	docker run --rm \
		-v $(PWD)/images/hermes-agent:/work \
		-w /work \
		ghcr.io/astral-sh/uv:0.5.0 \
		sh -c "uv lock --upgrade-package hermes-agent==$(HERMES_VERSION)"
	@echo "Updated images/hermes-agent/uv.lock for hermes-agent==$(HERMES_VERSION)"

# Smoke-test a locally built image: --help should exit 0.
.PHONY: agent-image-smoke
agent-image-smoke:
	docker run --rm $(AGENT_IMAGE):$(HERMES_VERSION) hermes-agent --help >/dev/null
	@echo "agent-image-smoke OK for $(AGENT_IMAGE):$(HERMES_VERSION)"
```

- [ ] **Step 2: Run the relock target**

```bash
make agent-image-relock HERMES_VERSION=1.4.2
```
Expected: `images/hermes-agent/uv.lock` is created. Inspect with `head -20 images/hermes-agent/uv.lock`: should be a TOML file with `[[package]]` entries including `hermes-agent`.

> If the relock command fails because upstream hermes-agent is not yet published to PyPI at the requested version, fall back to the latest published version (`uv pip index versions hermes-agent | head -3`) and pin to that. The Plan 3 deliverable is the *mechanism*; tracking the latest upstream is the responsibility of the matrix workflow added in Task 5.

- [ ] **Step 3: Verify the lockfile is reproducible**

```bash
cp images/hermes-agent/uv.lock /tmp/uv.lock.first
make agent-image-relock HERMES_VERSION=1.4.2
diff /tmp/uv.lock.first images/hermes-agent/uv.lock
```
Expected: no diff. If a diff appears, the resolver is non-deterministic: investigate before proceeding (almost always a missing pin in `pyproject.toml`).

- [ ] **Step 4: Build the image locally and smoke-test it**

```bash
make agent-image-build HERMES_VERSION=1.4.2
make agent-image-smoke HERMES_VERSION=1.4.2
```
Expected: `agent-image-smoke OK for ghcr.io/stubbi/hermes-agent:1.4.2`. If the upstream CLI flag is `-h` rather than `--help`, adjust the smoke target accordingly: but document the rename.

- [ ] **Step 5: Commit**

```bash
git add Makefile images/hermes-agent/uv.lock
git commit -m "feat(images): add agent-image-{build,buildx,relock,smoke} Makefile targets + initial uv.lock"
```

---

## Task 4: GitHub Actions workflow: agent image build on tag

**Files:**
- Create: `.github/workflows/agent-image.yaml`, `.github/workflows/agent-image-smoke.yaml`

- [ ] **Step 1: Create `.github/workflows/agent-image.yaml`**

```yaml
# .github/workflows/agent-image.yaml
#
# Builds and publishes ghcr.io/stubbi/hermes-agent for every hermes-agent
# release in the supported matrix. Triggered:
#   - Manually via workflow_dispatch (engineer picks the HERMES_VERSION).
#   - On every push of a tag matching `agent/vX.Y.Z`: this is the explicit
#     "ship this agent version" signal; the operator's own release tags
#     (`vX.Y.Z`) do NOT auto-build agents.
#
# Every successful build is Cosign-signed (keyless OIDC) and gets an SBOM
# attached via Syft, mirroring the operator-image pipeline from Plan 1.
name: agent-image

on:
  workflow_dispatch:
    inputs:
      hermes_version:
        description: "Upstream hermes-agent release to package (e.g. 1.4.3)"
        required: true
        type: string
  push:
    tags:
      - 'agent/v*'

permissions:
  contents: read
  packages: write
  id-token: write    # keyless Cosign signing

jobs:
  build:
    name: build (${{ matrix.hermes_version }})
    runs-on: ubuntu-22.04
    strategy:
      fail-fast: false
      matrix:
        # When triggered by workflow_dispatch the matrix is a single entry;
        # when triggered by a tag, the version comes from the tag.
        hermes_version:
          - ${{ github.event_name == 'workflow_dispatch' && inputs.hermes_version || startsWith(github.ref, 'refs/tags/agent/v') && substring(github.ref, length('refs/tags/agent/v')) }}
    steps:
      - uses: actions/checkout@v4

      - name: Set up QEMU
        uses: docker/setup-qemu-action@v3

      - name: Set up Buildx
        uses: docker/setup-buildx-action@v3

      - name: Log in to GHCR
        uses: docker/login-action@v3
        with:
          registry: ghcr.io
          username: ${{ github.actor }}
          password: ${{ secrets.GITHUB_TOKEN }}

      - name: Verify uv.lock matches the requested HERMES_VERSION
        run: |
          set -eux
          # The lockfile is the source of truth; refuse to ship if the committed lock
          # disagrees with the requested version. Engineers must run
          # `make agent-image-relock HERMES_VERSION=...` before tagging.
          grep -q "name = \"hermes-agent\"" images/hermes-agent/uv.lock
          ver=$(awk '/^name = "hermes-agent"$/{getline; if ($0 ~ /version =/) {gsub(/[" ]/, "", $3); print $3}}' images/hermes-agent/uv.lock)
          if [ "${ver}" != "${{ matrix.hermes_version }}" ]; then
            echo "uv.lock pins hermes-agent==${ver} but workflow requested ${{ matrix.hermes_version }}" >&2
            echo "Run: make agent-image-relock HERMES_VERSION=${{ matrix.hermes_version }} and commit." >&2
            exit 1
          fi

      - name: Build and push
        id: build
        uses: docker/build-push-action@v5
        with:
          context: images/hermes-agent
          file: images/hermes-agent/Dockerfile
          platforms: linux/amd64,linux/arm64
          push: true
          tags: |
            ghcr.io/stubbi/hermes-agent:${{ matrix.hermes_version }}
            ghcr.io/stubbi/hermes-agent:latest
          build-args: |
            HERMES_VERSION=${{ matrix.hermes_version }}
          provenance: true
          sbom: true

      - name: Install Cosign
        uses: sigstore/cosign-installer@v3

      - name: Sign image (keyless OIDC)
        env:
          DIGEST: ${{ steps.build.outputs.digest }}
        run: |
          cosign sign --yes \
            "ghcr.io/stubbi/hermes-agent@${DIGEST}"

      - name: Install Syft
        uses: anchore/sbom-action/download-syft@v0

      - name: Generate SBOM
        run: |
          syft "ghcr.io/stubbi/hermes-agent:${{ matrix.hermes_version }}" \
            -o spdx-json=sbom.spdx.json

      - name: Attach SBOM as Cosign attestation
        env:
          DIGEST: ${{ steps.build.outputs.digest }}
        run: |
          cosign attest --yes \
            --predicate sbom.spdx.json \
            --type spdxjson \
            "ghcr.io/stubbi/hermes-agent@${DIGEST}"
```

- [ ] **Step 2: Create `.github/workflows/agent-image-smoke.yaml`**

```yaml
# .github/workflows/agent-image-smoke.yaml
#
# Per-PR smoke test: build the agent image for the pinned HERMES_VERSION on a
# single platform (amd64), verify `hermes-agent --help` exits 0, and verify
# entrypoint.sh refuses to start without a config. Does NOT push.
name: agent-image-smoke

on:
  pull_request:
    paths:
      - 'images/hermes-agent/**'
      - '.github/workflows/agent-image-smoke.yaml'
      - 'Makefile'

permissions:
  contents: read

jobs:
  smoke:
    runs-on: ubuntu-22.04
    steps:
      - uses: actions/checkout@v4

      - name: Set up Buildx
        uses: docker/setup-buildx-action@v3

      - name: Read pinned HERMES_VERSION from uv.lock
        id: ver
        run: |
          set -eux
          ver=$(awk '/^name = "hermes-agent"$/{getline; if ($0 ~ /version =/) {gsub(/[" ]/, "", $3); print $3}}' images/hermes-agent/uv.lock)
          echo "version=${ver}" >> "$GITHUB_OUTPUT"

      - name: Build image locally
        run: |
          docker buildx build \
            --platform linux/amd64 \
            --build-arg HERMES_VERSION=${{ steps.ver.outputs.version }} \
            --load \
            -t hermes-agent:smoke \
            images/hermes-agent

      - name: Smoke: --help exits 0
        run: docker run --rm hermes-agent:smoke hermes-agent --help >/dev/null

      - name: Smoke: entrypoint refuses missing config with EX_CONFIG (78)
        run: |
          set +e
          docker run --rm --entrypoint /usr/local/bin/hermes-entrypoint hermes-agent:smoke
          rc=$?
          if [ "${rc}" != "78" ]; then
            echo "Expected exit 78 (EX_CONFIG), got ${rc}" >&2
            exit 1
          fi

      - name: Smoke: non-root by default
        run: |
          uid=$(docker run --rm --entrypoint id hermes-agent:smoke -u)
          [ "${uid}" = "1000" ] || { echo "Expected UID 1000, got ${uid}"; exit 1; }
```

- [ ] **Step 3: Validate the workflows with `actionlint`**

```bash
docker run --rm -v "$(pwd):/repo" -w /repo rhysd/actionlint:latest -color
```
Expected: no errors. If you don't have actionlint locally, push to a feature branch and let GitHub Actions surface syntax errors.

- [ ] **Step 4: Commit**

```bash
git add .github/workflows/agent-image.yaml .github/workflows/agent-image-smoke.yaml
git commit -m "ci: add agent-image build + per-PR smoke workflow (multi-arch, Cosign, SBOM)"
```

---

## Task 5: Add `RuntimeSpec` to `HermesInstance` API types

**Files:**
- Modify: `api/v1/hermesinstance_types.go`

- [ ] **Step 1: Add the `Runtime` field to `HermesInstanceSpec`**

In `api/v1/hermesinstance_types.go`, append `Runtime` to the existing `HermesInstanceSpec` struct (after `Storage`, before `Networking` if those exist after Plan 2):

```go
// Runtime controls the agent's Python toolchain and OS-level dependencies.
// All fields default to the values that match the operator's published
// ghcr.io/stubbi/hermes-agent image.
// +optional
Runtime RuntimeSpec `json:"runtime,omitempty"`
```

- [ ] **Step 2: Add the `RuntimeSpec` type and its nested types**

At the end of the file (above the `init()` function), append:

```go
// RuntimeSpec controls Python/uv runtime concerns for the agent container.
type RuntimeSpec struct {
    // Python is informational only: the agent image's Python version is fixed
    // at build time. Setting this does NOT pull a different interpreter; it
    // exists so downstream tooling can assert the runtime it expects.
    // +kubebuilder:default="3.11"
    // +optional
    Python string `json:"python,omitempty"`

    // UV controls the initial `uv sync` against the lockfile bundled in the
    // agent image. Enabled by default.
    // +optional
    UV UVSpec `json:"uv,omitempty"`

    // FFmpeg toggles the FFmpeg dependency check. The agent image always ships
    // FFmpeg; disabling here only skips the readiness assertion.
    // +optional
    FFmpeg FFmpegSpec `json:"ffmpeg,omitempty"`

    // Ripgrep toggles the ripgrep dependency check. See FFmpeg.
    // +optional
    Ripgrep RipgrepSpec `json:"ripgrep,omitempty"`

    // ExtraAptPackages adds additional Debian packages installed by a
    // root-privileged init container BEFORE the main agent container starts.
    // Use sparingly: the init container runs as root and breaks the otherwise
    // hardened security posture for one container only.
    // +listType=atomic
    // +optional
    ExtraAptPackages []string `json:"extraAptPackages,omitempty"`

    // ExtraPipPackages adds additional Python packages installed via
    // `uv pip install` into a persistent venv on the data PVC.
    // +listType=atomic
    // +optional
    ExtraPipPackages []string `json:"extraPipPackages,omitempty"`
}

// UVSpec controls the `uv sync` init container.
type UVSpec struct {
    // +kubebuilder:default=true
    // +optional
    Enabled *bool `json:"enabled,omitempty"`

    // ExtraIndexURL is appended to uv's index list. Useful for private PyPI
    // mirrors. Empty by default.
    // +optional
    ExtraIndexURL string `json:"extraIndexURL,omitempty"`

    // CacheVolume controls the volume mounted at /home/hermes/.cache/uv.
    // Defaults to an emptyDir with a 1Gi sizeLimit: fast and ephemeral.
    // +optional
    CacheVolume UVCacheVolumeSpec `json:"cacheVolume,omitempty"`
}

// UVCacheVolumeSpec mirrors a stripped-down VolumeSource union. Exactly one of
// EmptyDir or PersistentVolumeClaim may be set; the defaulter fills EmptyDir
// when both are nil.
type UVCacheVolumeSpec struct {
    // +optional
    EmptyDir *corev1.EmptyDirVolumeSource `json:"emptyDir,omitempty"`

    // +optional
    PersistentVolumeClaim *corev1.PersistentVolumeClaimVolumeSource `json:"persistentVolumeClaim,omitempty"`
}

// FFmpegSpec controls the FFmpeg dependency check.
type FFmpegSpec struct {
    // +kubebuilder:default=true
    // +optional
    Enabled *bool `json:"enabled,omitempty"`
}

// RipgrepSpec controls the ripgrep dependency check.
type RipgrepSpec struct {
    // +kubebuilder:default=true
    // +optional
    Enabled *bool `json:"enabled,omitempty"`
}
```

> **Import note:** if `corev1 "k8s.io/api/core/v1"` is not already in the imports block of this file, add it. The deepcopy generator picks up the dependency from the struct fields.

- [ ] **Step 3: Regenerate deepcopy + CRD manifests**

```bash
make generate manifests
```
Expected: `api/v1/zz_generated.deepcopy.go` updated with `*RuntimeSpec.DeepCopyInto` methods; `config/crd/bases/hermes.agent_hermesinstances.yaml` shows a `runtime` block under `spec.properties`.

- [ ] **Step 4: Build to verify**

```bash
go build ./...
```
Expected: exit 0.

- [ ] **Step 5: Commit**

```bash
git add api/v1/hermesinstance_types.go api/v1/zz_generated.deepcopy.go config/crd/bases/
git commit -m "feat(api): add spec.runtime (python, uv, ffmpeg, ripgrep, extraApt/PipPackages) to HermesInstance"
```

---

## Task 6: Add `GatewaysSpec` to `HermesInstance` API types

**Files:**
- Modify: `api/v1/hermesinstance_types.go`

- [ ] **Step 1: Add the `Gateways` field to `HermesInstanceSpec`**

```go
// Gateways configures the platform-side messaging bindings (Telegram, Discord,
// Slack, WhatsApp, Signal). Each gateway is opt-in and references its own
// Secret(s) so tokens are rotatable independently. See
// docs/runbook-platform-gateways.md for the per-platform token surface.
// +optional
Gateways GatewaysSpec `json:"gateways,omitempty"`
```

- [ ] **Step 2: Add the `GatewaysSpec` type and per-platform sub-specs**

```go
// GatewaysSpec is the union of all supported messaging-platform bindings.
type GatewaysSpec struct {
    // +optional
    Telegram TelegramGatewaySpec `json:"telegram,omitempty"`
    // +optional
    Discord DiscordGatewaySpec `json:"discord,omitempty"`
    // +optional
    Slack SlackGatewaySpec `json:"slack,omitempty"`
    // +optional
    WhatsApp WhatsAppGatewaySpec `json:"whatsapp,omitempty"`
    // +optional
    Signal SignalGatewaySpec `json:"signal,omitempty"`
}

// TelegramGatewaySpec binds the agent to a Telegram Bot API token.
type TelegramGatewaySpec struct {
    // +kubebuilder:default=false
    // +optional
    Enabled *bool `json:"enabled,omitempty"`

    // BotTokenSecretRef points at the Secret holding the Bot API token.
    // Required when Enabled.
    // +optional
    BotTokenSecretRef *corev1.SecretKeySelector `json:"botTokenSecretRef,omitempty"`

    // AllowedUserIDs is an optional allow-list of Telegram user IDs. When set,
    // the agent ignores DMs from any other user.
    // +listType=atomic
    // +optional
    AllowedUserIDs []int64 `json:"allowedUserIDs,omitempty"`

    // WebhookURL is the public HTTPS URL to register with Telegram. When empty
    // the agent runs in long-poll mode.
    // +optional
    WebhookURL string `json:"webhookURL,omitempty"`
}

// DiscordGatewaySpec binds the agent to a Discord bot application.
type DiscordGatewaySpec struct {
    // +kubebuilder:default=false
    // +optional
    Enabled *bool `json:"enabled,omitempty"`

    // BotTokenSecretRef points at the Secret holding the Discord bot token.
    // +optional
    BotTokenSecretRef *corev1.SecretKeySelector `json:"botTokenSecretRef,omitempty"`

    // ApplicationID is the Discord application's snowflake. Surfaced as
    // DISCORD_APPLICATION_ID env var.
    // +optional
    ApplicationID string `json:"applicationID,omitempty"`

    // GuildIDs scopes slash-command registration to specific guilds. Empty
    // means global registration.
    // +listType=atomic
    // +optional
    GuildIDs []string `json:"guildIDs,omitempty"`
}

// SlackGatewaySpec binds the agent to a Slack workspace via the bolt SDK.
type SlackGatewaySpec struct {
    // +kubebuilder:default=false
    // +optional
    Enabled *bool `json:"enabled,omitempty"`

    // BotTokenSecretRef holds the xoxb- bot token.
    // +optional
    BotTokenSecretRef *corev1.SecretKeySelector `json:"botTokenSecretRef,omitempty"`

    // AppTokenSecretRef holds the xapp- app-level token used for Socket Mode.
    // +optional
    AppTokenSecretRef *corev1.SecretKeySelector `json:"appTokenSecretRef,omitempty"`

    // SigningSecretRef holds the Slack signing secret used to verify request
    // signatures when running in HTTP mode.
    // +optional
    SigningSecretRef *corev1.SecretKeySelector `json:"signingSecretRef,omitempty"`
}

// WhatsAppGatewaySpec binds the agent to a WhatsApp provider (Twilio,
// Meta Cloud API, etc.). The provider expects a single Secret keyed by
// provider-specific field names.
type WhatsAppGatewaySpec struct {
    // +kubebuilder:default=false
    // +optional
    Enabled *bool `json:"enabled,omitempty"`

    // ProviderSecretRef is the Secret with provider credentials. The agent
    // surfaces every key in the Secret as an env var prefixed WHATSAPP_.
    // +optional
    ProviderSecretRef *corev1.SecretKeySelector `json:"providerSecretRef,omitempty"`
}

// SignalGatewaySpec binds the agent to signal-cli-rest-api running as a sidecar
// or external service.
type SignalGatewaySpec struct {
    // +kubebuilder:default=false
    // +optional
    Enabled *bool `json:"enabled,omitempty"`

    // PhoneNumberSecretRef holds the registered Signal phone number.
    // +optional
    PhoneNumberSecretRef *corev1.SecretKeySelector `json:"phoneNumberSecretRef,omitempty"`

    // AuthTokenSecretRef holds the auth token for signal-cli-rest-api.
    // +optional
    AuthTokenSecretRef *corev1.SecretKeySelector `json:"authTokenSecretRef,omitempty"`
}
```

- [ ] **Step 3: Regenerate**

```bash
make generate manifests
```

- [ ] **Step 4: Build to verify**

```bash
go build ./...
```
Expected: exit 0.

- [ ] **Step 5: Commit**

```bash
git add api/v1/hermesinstance_types.go api/v1/zz_generated.deepcopy.go config/crd/bases/
git commit -m "feat(api): add spec.gateways (telegram, discord, slack, whatsapp, signal) to HermesInstance"
```

---

## Task 7: Add `ProfileStoreSpec` to `HermesInstance` API types

**Files:**
- Modify: `api/v1/hermesinstance_types.go`

- [ ] **Step 1: Add the `ProfileStore` field to `HermesInstanceSpec`**

```go
// ProfileStore configures the optional Honcho profile-store companion.
// When honcho.enabled is true the operator stands up a sibling Deployment,
// Service, and PVC, and injects HONCHO_BASE_URL + HONCHO_API_KEY into the
// agent container.
// +optional
ProfileStore ProfileStoreSpec `json:"profileStore,omitempty"`
```

- [ ] **Step 2: Add the `ProfileStoreSpec` type**

```go
// ProfileStoreSpec is the union of supported profile-store backends. Only
// `honcho` is supported in v1; the wrapping struct exists to allow a future
// backend without an API break.
type ProfileStoreSpec struct {
    // +optional
    Honcho HonchoSpec `json:"honcho,omitempty"`
}

// HonchoSpec controls the Honcho companion Deployment.
type HonchoSpec struct {
    // +kubebuilder:default=false
    // +optional
    Enabled *bool `json:"enabled,omitempty"`

    // Image controls which Honcho container image to run. Defaults to
    // ghcr.io/plastic-labs/honcho:0.1.0.
    // +optional
    Image HonchoImageSpec `json:"image,omitempty"`

    // Persistence controls the Honcho-side PVC. Required for any non-trivial
    // use (the agent's snapshot Job writes to /data/snapshots/...).
    // +optional
    Persistence HonchoPersistenceSpec `json:"persistence,omitempty"`

    // Resources are Honcho's container resource requests/limits.
    // +optional
    Resources corev1.ResourceRequirements `json:"resources,omitempty"`

    // APIKeySecretRef points at the Secret holding the Honcho API key that
    // the agent uses to authenticate. Required when Enabled.
    // +optional
    APIKeySecretRef *corev1.SecretKeySelector `json:"apiKeySecretRef,omitempty"`
}

// HonchoImageSpec selects the Honcho image.
type HonchoImageSpec struct {
    // +kubebuilder:default="ghcr.io/plastic-labs/honcho"
    // +optional
    Repository string `json:"repository,omitempty"`

    // +kubebuilder:default="0.1.0"
    // +optional
    Tag string `json:"tag,omitempty"`

    // +kubebuilder:default=IfNotPresent
    // +kubebuilder:validation:Enum=Always;IfNotPresent;Never
    // +optional
    PullPolicy string `json:"pullPolicy,omitempty"`
}

// HonchoPersistenceSpec controls the Honcho-side PVC. The PVC is named
// `<inst>-honcho-data` (see internal/resources/honcho.go HonchoPVCName).
type HonchoPersistenceSpec struct {
    // +kubebuilder:default=true
    // +optional
    Enabled *bool `json:"enabled,omitempty"`

    // +kubebuilder:default="5Gi"
    // +optional
    Size string `json:"size,omitempty"`

    // +optional
    StorageClassName *string `json:"storageClassName,omitempty"`
}
```

- [ ] **Step 3: Regenerate**

```bash
make generate manifests
```

- [ ] **Step 4: Build to verify**

```bash
go build ./...
```
Expected: exit 0.

- [ ] **Step 5: Verify the listMapKey + listType markers Plan 4 expects**

Plan 4 explicitly requires `+listType=map +listMapKey=source` on `.spec.skills` and `+listMapKey=name` on `.spec.env`. Check those markers were set correctly in Plan 2:

```bash
grep -n "listType\|listMapKey" api/v1/hermesinstance_types.go
```
Expected: shows `+listType=map +listMapKey=name` above `Env`/`EnvFrom`, `+listType=map +listMapKey=source` above `Skills`. If missing, **stop and fix Plan 2** before continuing: Plan 4's SSA logic will produce wrong field-ownership merges otherwise.

- [ ] **Step 6: Commit**

```bash
git add api/v1/hermesinstance_types.go api/v1/zz_generated.deepcopy.go config/crd/bases/
git commit -m "feat(api): add spec.profileStore.honcho (image, persistence, resources, apiKey) to HermesInstance"
```

---

## Task 8: Builder: `internal/resources/runtime_init.go` (unit-tested first)

**Files:**
- Create: `internal/resources/runtime_init.go`, `internal/resources/runtime_init_test.go`

This task builds the init containers that prepare the data PVC before the main hermes container starts. **Critical invariant (lesson openclaw #450):** every init container mounts the *full* data volume at `/home/hermes/.hermes`: never a `subPath`. The Plan 1 hostPath PVC breakage that motivated this rule was caused by an init container with a subPath mount.

- [ ] **Step 1: Write the failing tests**

Create `internal/resources/runtime_init_test.go`:

```go
package resources

import (
	"testing"

	"github.com/stretchr/testify/assert"
	hermesv1 "github.com/stubbi/hermes-operator/api/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func instWithRuntime(r hermesv1.RuntimeSpec) *hermesv1.HermesInstance {
	return &hermesv1.HermesInstance{
		ObjectMeta: metav1.ObjectMeta{Name: "demo", Namespace: "agents"},
		Spec:       hermesv1.HermesInstanceSpec{Runtime: r},
	}
}

func TestBuildRuntimeInitContainers_UVDefault(t *testing.T) {
	inst := instWithRuntime(hermesv1.RuntimeSpec{
		UV: hermesv1.UVSpec{Enabled: Ptr(true)},
	})
	got := BuildRuntimeInitContainers(inst)
	assert.Len(t, got, 1, "uv-sync only")
	assert.Equal(t, "init-uv", got[0].Name)
	assert.Contains(t, got[0].Command[2], "uv sync --frozen", "frozen lockfile sync")
	// Lesson openclaw #450: full-volume mount, no subPath.
	hasFullData := false
	for _, m := range got[0].VolumeMounts {
		if m.Name == "data" && m.MountPath == "/home/hermes/.hermes" && m.SubPath == "" {
			hasFullData = true
		}
	}
	assert.True(t, hasFullData, "init container must mount the full data volume without subPath")
}

func TestBuildRuntimeInitContainers_UVDisabled(t *testing.T) {
	inst := instWithRuntime(hermesv1.RuntimeSpec{
		UV: hermesv1.UVSpec{Enabled: Ptr(false)},
	})
	got := BuildRuntimeInitContainers(inst)
	for _, c := range got {
		assert.NotEqual(t, "init-uv", c.Name, "uv sync should be skipped when disabled")
	}
}

func TestBuildRuntimeInitContainers_ExtraApt(t *testing.T) {
	inst := instWithRuntime(hermesv1.RuntimeSpec{
		UV:               hermesv1.UVSpec{Enabled: Ptr(true)},
		ExtraAptPackages: []string{"poppler-utils", "tesseract-ocr"},
	})
	got := BuildRuntimeInitContainers(inst)
	var aptC *corev1.Container
	for i, c := range got {
		if c.Name == "init-apt" {
			aptC = &got[i]
		}
	}
	if !assert.NotNil(t, aptC, "init-apt container missing") {
		return
	}
	// Must run as root (uid 0): document the security implication.
	assert.NotNil(t, aptC.SecurityContext)
	assert.NotNil(t, aptC.SecurityContext.RunAsUser)
	assert.Equal(t, int64(0), *aptC.SecurityContext.RunAsUser)
	assert.Contains(t, aptC.Command[2], "apt-get install -y --no-install-recommends poppler-utils tesseract-ocr")
}

func TestBuildRuntimeInitContainers_ExtraPip(t *testing.T) {
	inst := instWithRuntime(hermesv1.RuntimeSpec{
		UV:              hermesv1.UVSpec{Enabled: Ptr(true)},
		ExtraPipPackages: []string{"pandas==2.2.0", "polars"},
	})
	got := BuildRuntimeInitContainers(inst)
	var pipC *corev1.Container
	for i, c := range got {
		if c.Name == "init-pip" {
			pipC = &got[i]
		}
	}
	if !assert.NotNil(t, pipC, "init-pip container missing") {
		return
	}
	assert.Contains(t, pipC.Command[2], "uv pip install")
	assert.Contains(t, pipC.Command[2], "pandas==2.2.0")
	assert.Contains(t, pipC.Command[2], "polars")
	// Persistent venv on the PVC: must live under /home/hermes/.hermes.
	assert.Contains(t, pipC.Command[2], "/home/hermes/.hermes/.venv-extras")
}

func TestBuildRuntimeInitContainers_Order(t *testing.T) {
	// Order matters: apt before pip before uv-sync so that uv-sync sees any
	// system libs that pip extensions may dlopen, and pip installs into the
	// venv that uv-sync created.
	inst := instWithRuntime(hermesv1.RuntimeSpec{
		UV:               hermesv1.UVSpec{Enabled: Ptr(true)},
		ExtraAptPackages: []string{"libxml2-dev"},
		ExtraPipPackages: []string{"lxml"},
	})
	got := BuildRuntimeInitContainers(inst)
	names := []string{}
	for _, c := range got {
		names = append(names, c.Name)
	}
	assert.Equal(t, []string{"init-apt", "init-uv", "init-pip"}, names)
}

func TestBuildRuntimeVolumes_UVCacheEmptyDirDefault(t *testing.T) {
	inst := instWithRuntime(hermesv1.RuntimeSpec{
		UV: hermesv1.UVSpec{Enabled: Ptr(true)},
	})
	vols := BuildRuntimeVolumes(inst)
	found := false
	for _, v := range vols {
		if v.Name == "uv-cache" {
			found = true
			assert.NotNil(t, v.VolumeSource.EmptyDir, "default to emptyDir")
		}
	}
	assert.True(t, found, "uv-cache volume present when uv enabled")
}
```

- [ ] **Step 2: Run tests to verify they fail**

```bash
go test ./internal/resources/... -run TestBuildRuntime -v
```
Expected: build errors `undefined: BuildRuntimeInitContainers`, `undefined: BuildRuntimeVolumes`.

- [ ] **Step 3: Implement the builder**

Create `internal/resources/runtime_init.go`:

```go
package resources

import (
	"fmt"
	"strings"

	hermesv1 "github.com/stubbi/hermes-operator/api/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
)

// BuildRuntimeInitContainers returns the ordered init containers required by
// spec.runtime. Order is: init-apt → init-uv → init-pip. Each container mounts
// the full data volume (no subPath, lesson openclaw #450). The image used is
// the same hermes-agent image as the main container, so the venv layout and
// uv binary are guaranteed to match.
func BuildRuntimeInitContainers(inst *hermesv1.HermesInstance) []corev1.Container {
	var out []corev1.Container

	if len(inst.Spec.Runtime.ExtraAptPackages) > 0 {
		out = append(out, buildAptInit(inst))
	}
	if uvEnabled(inst) {
		out = append(out, buildUVSyncInit(inst))
	}
	if len(inst.Spec.Runtime.ExtraPipPackages) > 0 {
		out = append(out, buildPipInit(inst))
	}
	return out
}

// BuildRuntimeVolumes returns the additional Volumes (beyond the data PVC and
// config ConfigMap) needed by spec.runtime. The caller (statefulset.go)
// appends these to PodSpec.Volumes.
func BuildRuntimeVolumes(inst *hermesv1.HermesInstance) []corev1.Volume {
	var out []corev1.Volume
	if !uvEnabled(inst) {
		return out
	}
	cache := inst.Spec.Runtime.UV.CacheVolume
	vol := corev1.Volume{Name: "uv-cache"}
	switch {
	case cache.PersistentVolumeClaim != nil:
		vol.VolumeSource = corev1.VolumeSource{PersistentVolumeClaim: cache.PersistentVolumeClaim}
	case cache.EmptyDir != nil:
		vol.VolumeSource = corev1.VolumeSource{EmptyDir: cache.EmptyDir}
	default:
		// Default: 1Gi emptyDir.
		size := resource.MustParse("1Gi")
		vol.VolumeSource = corev1.VolumeSource{EmptyDir: &corev1.EmptyDirVolumeSource{SizeLimit: &size}}
	}
	out = append(out, vol)
	return out
}

// BuildRuntimeVolumeMounts returns the additional VolumeMounts for the main
// hermes container (NOT init containers; those have their own minimal mount
// set). Today just the uv cache.
func BuildRuntimeVolumeMounts(inst *hermesv1.HermesInstance) []corev1.VolumeMount {
	if !uvEnabled(inst) {
		return nil
	}
	return []corev1.VolumeMount{
		{Name: "uv-cache", MountPath: "/home/hermes/.cache/uv"},
	}
}

func uvEnabled(inst *hermesv1.HermesInstance) bool {
	if inst.Spec.Runtime.UV.Enabled == nil {
		return true // default
	}
	return *inst.Spec.Runtime.UV.Enabled
}

func dataVolumeMount() corev1.VolumeMount {
	// Full-volume mount, no subPath. Lesson openclaw #450.
	return corev1.VolumeMount{Name: "data", MountPath: "/home/hermes/.hermes"}
}

func buildUVSyncInit(inst *hermesv1.HermesInstance) corev1.Container {
	extra := inst.Spec.Runtime.UV.ExtraIndexURL
	indexArg := ""
	if extra != "" {
		indexArg = fmt.Sprintf("--extra-index-url=%s ", shellQuote(extra))
	}
	cmd := fmt.Sprintf(
		"set -eu; cd /home/hermes/.hermes; cp /opt/venv-template/pyproject.toml /opt/venv-template/uv.lock .; uv sync --frozen %s",
		indexArg,
	)
	return corev1.Container{
		Name:                     "init-uv",
		Image:                    imageRef(inst),
		ImagePullPolicy:          pullPolicy(inst),
		Command:                  []string{"/bin/sh", "-c", cmd},
		TerminationMessagePath:   "/dev/termination-log",
		TerminationMessagePolicy: corev1.TerminationMessageReadFile,
		SecurityContext: &corev1.SecurityContext{
			RunAsNonRoot:             Ptr(true),
			RunAsUser:                Ptr(int64(1000)),
			AllowPrivilegeEscalation: Ptr(false),
			ReadOnlyRootFilesystem:   Ptr(true),
			Capabilities:             &corev1.Capabilities{Drop: []corev1.Capability{"ALL"}},
		},
		VolumeMounts: []corev1.VolumeMount{
			dataVolumeMount(),
			{Name: "uv-cache", MountPath: "/home/hermes/.cache/uv"},
		},
	}
}

func buildAptInit(inst *hermesv1.HermesInstance) corev1.Container {
	// SECURITY: this is the only container in the pod that runs as root.
	// We accept the risk because the user explicitly opted in via
	// spec.runtime.extraAptPackages. The init container exits before the
	// main hermes container starts, so the privilege scope is bounded.
	pkgs := strings.Join(quoteEach(inst.Spec.Runtime.ExtraAptPackages), " ")
	cmd := fmt.Sprintf(
		"set -eu; apt-get update; apt-get install -y --no-install-recommends %s; rm -rf /var/lib/apt/lists/*",
		pkgs,
	)
	return corev1.Container{
		Name:                     "init-apt",
		Image:                    imageRef(inst),
		ImagePullPolicy:          pullPolicy(inst),
		Command:                  []string{"/bin/sh", "-c", cmd},
		TerminationMessagePath:   "/dev/termination-log",
		TerminationMessagePolicy: corev1.TerminationMessageReadFile,
		SecurityContext: &corev1.SecurityContext{
			RunAsNonRoot:             Ptr(false),
			RunAsUser:                Ptr(int64(0)),
			AllowPrivilegeEscalation: Ptr(false),
			ReadOnlyRootFilesystem:   Ptr(false), // apt writes to /var/lib/apt
			Capabilities:             &corev1.Capabilities{Drop: []corev1.Capability{"ALL"}, Add: []corev1.Capability{"CHOWN", "DAC_OVERRIDE", "FOWNER", "SETUID", "SETGID"}},
		},
		VolumeMounts: []corev1.VolumeMount{
			dataVolumeMount(),
		},
	}
}

func buildPipInit(inst *hermesv1.HermesInstance) corev1.Container {
	venvPath := "/home/hermes/.hermes/.venv-extras"
	pkgs := strings.Join(quoteEach(inst.Spec.Runtime.ExtraPipPackages), " ")
	cmd := fmt.Sprintf(
		"set -eu; test -d %[1]s || uv venv %[1]s; VIRTUAL_ENV=%[1]s uv pip install %[2]s",
		venvPath, pkgs,
	)
	return corev1.Container{
		Name:                     "init-pip",
		Image:                    imageRef(inst),
		ImagePullPolicy:          pullPolicy(inst),
		Command:                  []string{"/bin/sh", "-c", cmd},
		TerminationMessagePath:   "/dev/termination-log",
		TerminationMessagePolicy: corev1.TerminationMessageReadFile,
		SecurityContext: &corev1.SecurityContext{
			RunAsNonRoot:             Ptr(true),
			RunAsUser:                Ptr(int64(1000)),
			AllowPrivilegeEscalation: Ptr(false),
			ReadOnlyRootFilesystem:   Ptr(true),
			Capabilities:             &corev1.Capabilities{Drop: []corev1.Capability{"ALL"}},
		},
		VolumeMounts: []corev1.VolumeMount{
			dataVolumeMount(),
			{Name: "uv-cache", MountPath: "/home/hermes/.cache/uv"},
		},
	}
}

// shellQuote returns a single-quoted shell-safe version of s.
func shellQuote(s string) string {
	return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'"
}

func quoteEach(in []string) []string {
	out := make([]string, len(in))
	for i, s := range in {
		out[i] = shellQuote(s)
	}
	return out
}
```

> **Note:** `imageRef(inst)` and `pullPolicy(inst)` already exist in `statefulset.go` (Plan 1). If they are private (lower-case in a different file in the same package, which they are), this works because we're in the same `resources` package. If Plan 2 moved them to a different package, import accordingly.

- [ ] **Step 4: Run tests to verify they pass**

```bash
go test ./internal/resources/... -run TestBuildRuntime -v
```
Expected: all PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/resources/runtime_init.go internal/resources/runtime_init_test.go
git commit -m "feat(resources): add runtime init containers (uv sync, extraApt as root, extraPip) with full-volume mounts"
```

---

## Task 9: Builder: `internal/resources/gateways.go` (envFrom + config fragments)

**Files:**
- Create: `internal/resources/gateways.go`, `internal/resources/gateways_test.go`

The gateways builder produces three outputs:
1. **`BuildGatewayEnvFrom(inst) []corev1.EnvFromSource`**: appended to the hermes container's `envFrom`. Each enabled gateway contributes one or more `SecretRef`s, so platform tokens land as env vars (the names follow the upstream hermes-agent convention).
2. **`BuildGatewayEnv(inst) []corev1.EnvVar`**: for non-Secret values like `DISCORD_APPLICATION_ID` and `TELEGRAM_ALLOWED_USER_IDS`.
3. **`BuildGatewayConfigFragments(inst) map[string]any`**: YAML-shaped sub-trees merged into the rendered `config.yaml` by `configmap.go` (Task 11). Configmap builder concatenates these under `gateways:`.

- [ ] **Step 1: Write the failing tests**

Create `internal/resources/gateways_test.go`:

```go
package resources

import (
	"testing"

	"github.com/stretchr/testify/assert"
	hermesv1 "github.com/stubbi/hermes-operator/api/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func instWithGateways(g hermesv1.GatewaysSpec) *hermesv1.HermesInstance {
	return &hermesv1.HermesInstance{
		ObjectMeta: metav1.ObjectMeta{Name: "demo", Namespace: "agents"},
		Spec:       hermesv1.HermesInstanceSpec{Gateways: g},
	}
}

func TestBuildGatewayEnvFrom_Telegram(t *testing.T) {
	inst := instWithGateways(hermesv1.GatewaysSpec{
		Telegram: hermesv1.TelegramGatewaySpec{
			Enabled: Ptr(true),
			BotTokenSecretRef: &corev1.SecretKeySelector{
				LocalObjectReference: corev1.LocalObjectReference{Name: "tg-secret"},
				Key:                  "token",
			},
		},
	})
	envFrom := BuildGatewayEnvFrom(inst)
	assert.Empty(t, envFrom, "BotTokenSecretRef is a single-key selector, not whole-secret envFrom")
}

func TestBuildGatewayEnv_Telegram(t *testing.T) {
	inst := instWithGateways(hermesv1.GatewaysSpec{
		Telegram: hermesv1.TelegramGatewaySpec{
			Enabled: Ptr(true),
			BotTokenSecretRef: &corev1.SecretKeySelector{
				LocalObjectReference: corev1.LocalObjectReference{Name: "tg-secret"},
				Key:                  "token",
			},
			AllowedUserIDs: []int64{42, 1337},
			WebhookURL:     "https://example.com/tg",
		},
	})
	env := BuildGatewayEnv(inst)
	byName := map[string]corev1.EnvVar{}
	for _, e := range env {
		byName[e.Name] = e
	}

	tok, ok := byName["TELEGRAM_BOT_TOKEN"]
	assert.True(t, ok, "TELEGRAM_BOT_TOKEN env var present")
	assert.NotNil(t, tok.ValueFrom)
	assert.NotNil(t, tok.ValueFrom.SecretKeyRef)
	assert.Equal(t, "tg-secret", tok.ValueFrom.SecretKeyRef.Name)
	assert.Equal(t, "token", tok.ValueFrom.SecretKeyRef.Key)

	allowed, ok := byName["TELEGRAM_ALLOWED_USER_IDS"]
	assert.True(t, ok)
	assert.Equal(t, "42,1337", allowed.Value)

	wh, ok := byName["TELEGRAM_WEBHOOK_URL"]
	assert.True(t, ok)
	assert.Equal(t, "https://example.com/tg", wh.Value)
}

func TestBuildGatewayEnv_Discord(t *testing.T) {
	inst := instWithGateways(hermesv1.GatewaysSpec{
		Discord: hermesv1.DiscordGatewaySpec{
			Enabled: Ptr(true),
			BotTokenSecretRef: &corev1.SecretKeySelector{
				LocalObjectReference: corev1.LocalObjectReference{Name: "dc-secret"},
				Key:                  "token",
			},
			ApplicationID: "111222333",
			GuildIDs:      []string{"444", "555"},
		},
	})
	env := BuildGatewayEnv(inst)
	byName := map[string]corev1.EnvVar{}
	for _, e := range env {
		byName[e.Name] = e
	}
	assert.Equal(t, "111222333", byName["DISCORD_APPLICATION_ID"].Value)
	assert.Equal(t, "444,555", byName["DISCORD_GUILD_IDS"].Value)
	assert.NotNil(t, byName["DISCORD_BOT_TOKEN"].ValueFrom)
}

func TestBuildGatewayEnv_Slack_AllThreeRefs(t *testing.T) {
	inst := instWithGateways(hermesv1.GatewaysSpec{
		Slack: hermesv1.SlackGatewaySpec{
			Enabled:           Ptr(true),
			BotTokenSecretRef: &corev1.SecretKeySelector{LocalObjectReference: corev1.LocalObjectReference{Name: "slk"}, Key: "bot"},
			AppTokenSecretRef: &corev1.SecretKeySelector{LocalObjectReference: corev1.LocalObjectReference{Name: "slk"}, Key: "app"},
			SigningSecretRef:  &corev1.SecretKeySelector{LocalObjectReference: corev1.LocalObjectReference{Name: "slk"}, Key: "sig"},
		},
	})
	env := BuildGatewayEnv(inst)
	names := map[string]bool{}
	for _, e := range env {
		names[e.Name] = true
	}
	assert.True(t, names["SLACK_BOT_TOKEN"])
	assert.True(t, names["SLACK_APP_TOKEN"])
	assert.True(t, names["SLACK_SIGNING_SECRET"])
}

func TestBuildGatewayEnv_Disabled(t *testing.T) {
	inst := instWithGateways(hermesv1.GatewaysSpec{
		Telegram: hermesv1.TelegramGatewaySpec{Enabled: Ptr(false)},
	})
	assert.Empty(t, BuildGatewayEnv(inst), "disabled gateway contributes no env vars")
}

func TestBuildGatewayConfigFragments_Shape(t *testing.T) {
	inst := instWithGateways(hermesv1.GatewaysSpec{
		Telegram: hermesv1.TelegramGatewaySpec{Enabled: Ptr(true)},
		Discord:  hermesv1.DiscordGatewaySpec{Enabled: Ptr(true), ApplicationID: "111"},
	})
	frags := BuildGatewayConfigFragments(inst)
	tg, ok := frags["telegram"].(map[string]any)
	assert.True(t, ok)
	assert.Equal(t, true, tg["enabled"])

	dc, ok := frags["discord"].(map[string]any)
	assert.True(t, ok)
	assert.Equal(t, true, dc["enabled"])
	assert.Equal(t, "111", dc["applicationID"])
}

func TestBuildGatewayEgressEndpoints(t *testing.T) {
	inst := instWithGateways(hermesv1.GatewaysSpec{
		Telegram: hermesv1.TelegramGatewaySpec{Enabled: Ptr(true)},
		Discord:  hermesv1.DiscordGatewaySpec{Enabled: Ptr(true)},
		Slack:    hermesv1.SlackGatewaySpec{Enabled: Ptr(true)},
	})
	hosts := BuildGatewayEgressEndpoints(inst)
	assert.Contains(t, hosts, "api.telegram.org")
	assert.Contains(t, hosts, "discord.com")
	assert.Contains(t, hosts, "slack.com")
	assert.NotContains(t, hosts, "signal.org", "Signal endpoint only when enabled")
}
```

- [ ] **Step 2: Run tests to verify they fail**

```bash
go test ./internal/resources/... -run TestBuildGateway -v
```
Expected: undefined errors for all the `BuildGateway*` symbols.

- [ ] **Step 3: Implement the builder**

Create `internal/resources/gateways.go`:

```go
package resources

import (
	"fmt"
	"strings"

	hermesv1 "github.com/stubbi/hermes-operator/api/v1"
	corev1 "k8s.io/api/core/v1"
)

// BuildGatewayEnvFrom returns whole-Secret envFrom entries. Reserved for
// future gateways that use Secret-wide credential maps (e.g. a WhatsApp
// provider that ships a complete env block). Telegram/Discord/Slack use
// single-key SecretKeyRefs and are surfaced through BuildGatewayEnv instead.
func BuildGatewayEnvFrom(inst *hermesv1.HermesInstance) []corev1.EnvFromSource {
	var out []corev1.EnvFromSource
	g := inst.Spec.Gateways

	if isTrue(g.WhatsApp.Enabled) && g.WhatsApp.ProviderSecretRef != nil {
		out = append(out, corev1.EnvFromSource{
			Prefix: "WHATSAPP_",
			SecretRef: &corev1.SecretEnvSource{
				LocalObjectReference: corev1.LocalObjectReference{Name: g.WhatsApp.ProviderSecretRef.Name},
			},
		})
	}
	return out
}

// BuildGatewayEnv returns explicit per-gateway env vars. Single-key secret
// references become EnvVar.ValueFrom.SecretKeyRef; literal config fields
// (AllowedUserIDs, ApplicationID, etc.) become EnvVar.Value.
func BuildGatewayEnv(inst *hermesv1.HermesInstance) []corev1.EnvVar {
	var out []corev1.EnvVar
	g := inst.Spec.Gateways

	if isTrue(g.Telegram.Enabled) {
		if ref := g.Telegram.BotTokenSecretRef; ref != nil {
			out = append(out, secretEnv("TELEGRAM_BOT_TOKEN", ref))
		}
		if len(g.Telegram.AllowedUserIDs) > 0 {
			out = append(out, corev1.EnvVar{
				Name:  "TELEGRAM_ALLOWED_USER_IDS",
				Value: joinInt64s(g.Telegram.AllowedUserIDs, ","),
			})
		}
		if g.Telegram.WebhookURL != "" {
			out = append(out, corev1.EnvVar{Name: "TELEGRAM_WEBHOOK_URL", Value: g.Telegram.WebhookURL})
		}
	}

	if isTrue(g.Discord.Enabled) {
		if ref := g.Discord.BotTokenSecretRef; ref != nil {
			out = append(out, secretEnv("DISCORD_BOT_TOKEN", ref))
		}
		if g.Discord.ApplicationID != "" {
			out = append(out, corev1.EnvVar{Name: "DISCORD_APPLICATION_ID", Value: g.Discord.ApplicationID})
		}
		if len(g.Discord.GuildIDs) > 0 {
			out = append(out, corev1.EnvVar{
				Name:  "DISCORD_GUILD_IDS",
				Value: strings.Join(g.Discord.GuildIDs, ","),
			})
		}
	}

	if isTrue(g.Slack.Enabled) {
		if ref := g.Slack.BotTokenSecretRef; ref != nil {
			out = append(out, secretEnv("SLACK_BOT_TOKEN", ref))
		}
		if ref := g.Slack.AppTokenSecretRef; ref != nil {
			out = append(out, secretEnv("SLACK_APP_TOKEN", ref))
		}
		if ref := g.Slack.SigningSecretRef; ref != nil {
			out = append(out, secretEnv("SLACK_SIGNING_SECRET", ref))
		}
	}

	if isTrue(g.Signal.Enabled) {
		if ref := g.Signal.PhoneNumberSecretRef; ref != nil {
			out = append(out, secretEnv("SIGNAL_PHONE_NUMBER", ref))
		}
		if ref := g.Signal.AuthTokenSecretRef; ref != nil {
			out = append(out, secretEnv("SIGNAL_AUTH_TOKEN", ref))
		}
	}

	return out
}

// BuildGatewayConfigFragments returns the typed Go shape of the `gateways:`
// sub-tree of config.yaml. configmap.go (Task 11) marshals this into YAML and
// merges it under the user-supplied raw config.
func BuildGatewayConfigFragments(inst *hermesv1.HermesInstance) map[string]any {
	out := map[string]any{}
	g := inst.Spec.Gateways

	if isTrue(g.Telegram.Enabled) {
		out["telegram"] = map[string]any{
			"enabled":        true,
			"webhookURL":     g.Telegram.WebhookURL,
			"allowedUserIDs": g.Telegram.AllowedUserIDs,
		}
	}
	if isTrue(g.Discord.Enabled) {
		out["discord"] = map[string]any{
			"enabled":       true,
			"applicationID": g.Discord.ApplicationID,
			"guildIDs":      g.Discord.GuildIDs,
		}
	}
	if isTrue(g.Slack.Enabled) {
		out["slack"] = map[string]any{"enabled": true}
	}
	if isTrue(g.WhatsApp.Enabled) {
		out["whatsapp"] = map[string]any{"enabled": true}
	}
	if isTrue(g.Signal.Enabled) {
		out["signal"] = map[string]any{"enabled": true}
	}
	return out
}

// BuildGatewayEgressEndpoints returns the set of upstream hosts each enabled
// gateway needs to reach. Consumed by networkpolicy.go to widen the egress
// allowlist. Kept as a flat string slice (port 443/TCP is implied: see
// docs/conventions.md "Well-known egress endpoints").
func BuildGatewayEgressEndpoints(inst *hermesv1.HermesInstance) []string {
	var out []string
	g := inst.Spec.Gateways
	if isTrue(g.Telegram.Enabled) {
		out = append(out, "api.telegram.org")
	}
	if isTrue(g.Discord.Enabled) {
		out = append(out, "discord.com", "gateway.discord.gg")
	}
	if isTrue(g.Slack.Enabled) {
		out = append(out, "slack.com", "wss-primary.slack.com")
	}
	if isTrue(g.WhatsApp.Enabled) {
		out = append(out, "graph.facebook.com")
	}
	if isTrue(g.Signal.Enabled) {
		// Signal endpoint depends on the deployment shape (self-hosted
		// signal-cli-rest-api vs. third party). We emit the canonical
		// chat.signal.org host; users with a different deployment can
		// supplement via networking.egress.
		out = append(out, "chat.signal.org")
	}
	return out
}

func isTrue(b *bool) bool { return b != nil && *b }

func secretEnv(name string, ref *corev1.SecretKeySelector) corev1.EnvVar {
	return corev1.EnvVar{
		Name: name,
		ValueFrom: &corev1.EnvVarSource{
			SecretKeyRef: &corev1.SecretKeySelector{
				LocalObjectReference: ref.LocalObjectReference,
				Key:                  ref.Key,
			},
		},
	}
}

func joinInt64s(in []int64, sep string) string {
	if len(in) == 0 {
		return ""
	}
	parts := make([]string, len(in))
	for i, n := range in {
		parts[i] = fmt.Sprintf("%d", n)
	}
	return strings.Join(parts, sep)
}
```

- [ ] **Step 4: Run tests to verify they pass**

```bash
go test ./internal/resources/... -run TestBuildGateway -v
```
Expected: all PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/resources/gateways.go internal/resources/gateways_test.go
git commit -m "feat(resources): add gateway builders (env, envFrom, config fragments, egress endpoints)"
```

---

## Task 10: Builder: `internal/resources/honcho.go` (Deployment + Service + PVC)

**Files:**
- Create: `internal/resources/honcho.go`, `internal/resources/honcho_test.go`

Plan 4 (Task 11) already references `HonchoPVCName(inst)` and expects a PVC named `<inst>-honcho-data` plus a Service named `<inst>-honcho` resolvable at `http://<inst>-honcho:8000`. This task defines those builders.

- [ ] **Step 1: Write the failing tests**

Create `internal/resources/honcho_test.go`:

```go
package resources

import (
	"testing"

	"github.com/stretchr/testify/assert"
	hermesv1 "github.com/stubbi/hermes-operator/api/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func instWithHoncho(h hermesv1.HonchoSpec) *hermesv1.HermesInstance {
	return &hermesv1.HermesInstance{
		ObjectMeta: metav1.ObjectMeta{Name: "demo", Namespace: "agents"},
		Spec:       hermesv1.HermesInstanceSpec{ProfileStore: hermesv1.ProfileStoreSpec{Honcho: h}},
	}
}

func TestHonchoNaming(t *testing.T) {
	inst := instWithHoncho(hermesv1.HonchoSpec{Enabled: Ptr(true)})
	assert.Equal(t, "demo-honcho", HonchoServiceName(inst))
	assert.Equal(t, "demo-honcho", HonchoDeploymentName(inst))
	assert.Equal(t, "demo-honcho-data", HonchoPVCName(inst))
}

func TestBuildHonchoPVC_Defaults(t *testing.T) {
	inst := instWithHoncho(hermesv1.HonchoSpec{
		Enabled:     Ptr(true),
		Persistence: hermesv1.HonchoPersistenceSpec{Enabled: Ptr(true)},
	})
	pvc := BuildHonchoPVC(inst)
	assert.Equal(t, "demo-honcho-data", pvc.Name)
	assert.Equal(t, resource.MustParse("5Gi"), pvc.Spec.Resources.Requests[corev1.ResourceStorage])
	assert.Equal(t, []corev1.PersistentVolumeAccessMode{corev1.ReadWriteOnce}, pvc.Spec.AccessModes)
}

func TestBuildHonchoService(t *testing.T) {
	inst := instWithHoncho(hermesv1.HonchoSpec{Enabled: Ptr(true)})
	svc := BuildHonchoService(inst)
	assert.Equal(t, "demo-honcho", svc.Name)
	assert.NotEqual(t, corev1.ClusterIPNone, svc.Spec.ClusterIP, "regular ClusterIP, not headless")
	assert.Equal(t, "demo-honcho", svc.Spec.Selector["app.kubernetes.io/instance"])
	assert.Equal(t, "honcho", svc.Spec.Selector["app.kubernetes.io/name"])
	assert.Len(t, svc.Spec.Ports, 1)
	assert.Equal(t, int32(8000), svc.Spec.Ports[0].Port)
}

func TestBuildHonchoDeployment(t *testing.T) {
	inst := instWithHoncho(hermesv1.HonchoSpec{
		Enabled: Ptr(true),
		Image:   hermesv1.HonchoImageSpec{Repository: "ghcr.io/plastic-labs/honcho", Tag: "0.2.0"},
		APIKeySecretRef: &corev1.SecretKeySelector{
			LocalObjectReference: corev1.LocalObjectReference{Name: "honcho-secret"},
			Key:                  "api-key",
		},
		Persistence: hermesv1.HonchoPersistenceSpec{Enabled: Ptr(true)},
	})
	dep := BuildHonchoDeployment(inst)
	assert.Equal(t, "demo-honcho", dep.Name)
	require := dep.Spec.Template.Spec.Containers
	assert.Len(t, require, 1)
	assert.Equal(t, "ghcr.io/plastic-labs/honcho:0.2.0", require[0].Image)

	// Explicit k8s defaults (Plan 1 lesson: generation thrash).
	assert.Equal(t, corev1.RestartPolicyAlways, dep.Spec.Template.Spec.RestartPolicy)
	assert.Equal(t, corev1.DNSClusterFirst, dep.Spec.Template.Spec.DNSPolicy)
	assert.NotNil(t, dep.Spec.RevisionHistoryLimit)
	assert.Equal(t, int32(10), *dep.Spec.RevisionHistoryLimit)
	assert.NotNil(t, dep.Spec.ProgressDeadlineSeconds)

	// API key env from secret.
	var apiKeyEnv *corev1.EnvVar
	for i, e := range require[0].Env {
		if e.Name == "HONCHO_API_KEY" {
			apiKeyEnv = &require[0].Env[i]
		}
	}
	if assert.NotNil(t, apiKeyEnv) {
		assert.Equal(t, "honcho-secret", apiKeyEnv.ValueFrom.SecretKeyRef.Name)
		assert.Equal(t, "api-key", apiKeyEnv.ValueFrom.SecretKeyRef.Key)
	}

	// PVC mounted at /data (the path agreed with Plan 4 Task 11).
	var dataMount *corev1.VolumeMount
	for i, m := range require[0].VolumeMounts {
		if m.Name == "honcho-data" {
			dataMount = &require[0].VolumeMounts[i]
		}
	}
	if assert.NotNil(t, dataMount) {
		assert.Equal(t, "/data", dataMount.MountPath)
		assert.Equal(t, "", dataMount.SubPath)
	}
}

func TestBuildHonchoConsumerEnv(t *testing.T) {
	inst := instWithHoncho(hermesv1.HonchoSpec{
		Enabled: Ptr(true),
		APIKeySecretRef: &corev1.SecretKeySelector{
			LocalObjectReference: corev1.LocalObjectReference{Name: "honcho-secret"},
			Key:                  "api-key",
		},
	})
	env := BuildHonchoConsumerEnv(inst)
	byName := map[string]corev1.EnvVar{}
	for _, e := range env {
		byName[e.Name] = e
	}
	assert.Equal(t, "http://demo-honcho:8000", byName["HONCHO_BASE_URL"].Value)
	assert.NotNil(t, byName["HONCHO_API_KEY"].ValueFrom)
	assert.Equal(t, "honcho-secret", byName["HONCHO_API_KEY"].ValueFrom.SecretKeyRef.Name)
}

func TestBuildHonchoConsumerEnv_Disabled(t *testing.T) {
	inst := instWithHoncho(hermesv1.HonchoSpec{Enabled: Ptr(false)})
	assert.Empty(t, BuildHonchoConsumerEnv(inst))
}
```

- [ ] **Step 2: Run tests to verify they fail**

```bash
go test ./internal/resources/... -run TestHoncho -v
go test ./internal/resources/... -run TestBuildHoncho -v
```
Expected: undefined errors for the new symbols.

- [ ] **Step 3: Implement the builder**

Create `internal/resources/honcho.go`:

```go
package resources

import (
	"fmt"

	hermesv1 "github.com/stubbi/hermes-operator/api/v1"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
)

// HonchoDeploymentName, HonchoServiceName, HonchoPVCName return the
// deterministic resource names. The PVC name is locked because Plan 4 Task 11
// hard-codes `<inst>-honcho-data` for the addProfileSnapshot Job.
func HonchoDeploymentName(inst *hermesv1.HermesInstance) string {
	return inst.Name + "-honcho"
}
func HonchoServiceName(inst *hermesv1.HermesInstance) string {
	return inst.Name + "-honcho"
}
func HonchoPVCName(inst *hermesv1.HermesInstance) string {
	return inst.Name + "-honcho-data"
}

// HonchoLabels returns labels for the Honcho sub-stack. The
// `app.kubernetes.io/name` differs from the hermes pod so NetworkPolicies can
// target one without the other.
func HonchoLabels(inst *hermesv1.HermesInstance) map[string]string {
	return map[string]string{
		"app.kubernetes.io/name":       "honcho",
		"app.kubernetes.io/instance":   HonchoDeploymentName(inst),
		"app.kubernetes.io/managed-by": "hermes-operator",
		"app.kubernetes.io/part-of":    "hermes.agent",
		"hermes.agent/instance":        inst.Name,
	}
}

// honchoEnabled reports whether the user opted into the Honcho companion.
func honchoEnabled(inst *hermesv1.HermesInstance) bool {
	return isTrue(inst.Spec.ProfileStore.Honcho.Enabled)
}

// BuildHonchoPVC returns the Honcho data PVC. Always 5Gi by default.
func BuildHonchoPVC(inst *hermesv1.HermesInstance) *corev1.PersistentVolumeClaim {
	p := inst.Spec.ProfileStore.Honcho.Persistence
	size := p.Size
	if size == "" {
		size = "5Gi"
	}
	return &corev1.PersistentVolumeClaim{
		ObjectMeta: metav1.ObjectMeta{
			Name:      HonchoPVCName(inst),
			Namespace: inst.Namespace,
			Labels:    HonchoLabels(inst),
		},
		Spec: corev1.PersistentVolumeClaimSpec{
			AccessModes: []corev1.PersistentVolumeAccessMode{corev1.ReadWriteOnce},
			Resources: corev1.VolumeResourceRequirements{
				Requests: corev1.ResourceList{corev1.ResourceStorage: resource.MustParse(size)},
			},
			StorageClassName: p.StorageClassName,
		},
	}
}

// BuildHonchoService returns the headed ClusterIP Service. Honcho is consumed
// over HTTP, so a regular ClusterIP (not headless) is correct: the agent
// resolves `http://<inst>-honcho:8000` and reuses the kube-proxy backend.
func BuildHonchoService(inst *hermesv1.HermesInstance) *corev1.Service {
	return &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      HonchoServiceName(inst),
			Namespace: inst.Namespace,
			Labels:    HonchoLabels(inst),
		},
		Spec: corev1.ServiceSpec{
			Type:            corev1.ServiceTypeClusterIP,
			SessionAffinity: corev1.ServiceAffinityNone,
			Selector: map[string]string{
				"app.kubernetes.io/name":     "honcho",
				"app.kubernetes.io/instance": HonchoDeploymentName(inst),
			},
			Ports: []corev1.ServicePort{{
				Name:       "http",
				Port:       8000,
				TargetPort: intstr.FromString("http"),
				Protocol:   corev1.ProtocolTCP,
			}},
		},
	}
}

// BuildHonchoDeployment returns the Honcho Deployment. Single replica, RWO PVC.
func BuildHonchoDeployment(inst *hermesv1.HermesInstance) *appsv1.Deployment {
	h := inst.Spec.ProfileStore.Honcho
	image := honchoImageRef(h)
	labels := HonchoLabels(inst)

	env := []corev1.EnvVar{
		{Name: "HONCHO_DATA_DIR", Value: "/data"},
	}
	if h.APIKeySecretRef != nil {
		env = append(env, corev1.EnvVar{
			Name: "HONCHO_API_KEY",
			ValueFrom: &corev1.EnvVarSource{
				SecretKeyRef: &corev1.SecretKeySelector{
					LocalObjectReference: h.APIKeySecretRef.LocalObjectReference,
					Key:                  h.APIKeySecretRef.Key,
				},
			},
		})
	}

	mounts := []corev1.VolumeMount{
		{Name: "honcho-data", MountPath: "/data"},
		{Name: "tmp", MountPath: "/tmp"},
	}
	volumes := []corev1.Volume{
		{Name: "honcho-data", VolumeSource: corev1.VolumeSource{
			PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{ClaimName: HonchoPVCName(inst)},
		}},
		{Name: "tmp", VolumeSource: corev1.VolumeSource{EmptyDir: &corev1.EmptyDirVolumeSource{}}},
	}

	return &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      HonchoDeploymentName(inst),
			Namespace: inst.Namespace,
			Labels:    labels,
		},
		Spec: appsv1.DeploymentSpec{
			Replicas:                Ptr(int32(1)),
			RevisionHistoryLimit:    Ptr(int32(10)),
			ProgressDeadlineSeconds: Ptr(int32(600)),
			Strategy:                appsv1.DeploymentStrategy{Type: appsv1.RecreateDeploymentStrategyType},
			Selector: &metav1.LabelSelector{MatchLabels: map[string]string{
				"app.kubernetes.io/name":     "honcho",
				"app.kubernetes.io/instance": HonchoDeploymentName(inst),
			}},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{Labels: labels},
				Spec: corev1.PodSpec{
					RestartPolicy:                 corev1.RestartPolicyAlways,
					DNSPolicy:                     corev1.DNSClusterFirst,
					SchedulerName:                 "default-scheduler",
					TerminationGracePeriodSeconds: Ptr(int64(30)),
					SecurityContext: &corev1.PodSecurityContext{
						RunAsNonRoot: Ptr(true),
						RunAsUser:    Ptr(int64(1000)),
						RunAsGroup:   Ptr(int64(1000)),
						FSGroup:      Ptr(int64(1000)),
						SeccompProfile: &corev1.SeccompProfile{
							Type: corev1.SeccompProfileTypeRuntimeDefault,
						},
					},
					Containers: []corev1.Container{{
						Name:                     "honcho",
						Image:                    image,
						ImagePullPolicy:          honchoPullPolicy(h),
						TerminationMessagePath:   "/dev/termination-log",
						TerminationMessagePolicy: corev1.TerminationMessageReadFile,
						Env:                      env,
						Resources:                h.Resources,
						Ports: []corev1.ContainerPort{{
							Name: "http", ContainerPort: 8000, Protocol: corev1.ProtocolTCP,
						}},
						SecurityContext: &corev1.SecurityContext{
							AllowPrivilegeEscalation: Ptr(false),
							ReadOnlyRootFilesystem:   Ptr(true),
							Capabilities:             &corev1.Capabilities{Drop: []corev1.Capability{"ALL"}},
						},
						VolumeMounts: mounts,
						ReadinessProbe: &corev1.Probe{
							ProbeHandler: corev1.ProbeHandler{
								HTTPGet: &corev1.HTTPGetAction{
									Path: "/healthz",
									Port: intstr.FromString("http"),
								},
							},
							InitialDelaySeconds: 5,
							PeriodSeconds:       10,
							TimeoutSeconds:      2,
							FailureThreshold:    3,
							SuccessThreshold:    1,
						},
						LivenessProbe: &corev1.Probe{
							ProbeHandler: corev1.ProbeHandler{
								TCPSocket: &corev1.TCPSocketAction{Port: intstr.FromString("http")},
							},
							InitialDelaySeconds: 30,
							PeriodSeconds:       30,
							TimeoutSeconds:      5,
							FailureThreshold:    3,
							SuccessThreshold:    1,
						},
					}},
					Volumes: volumes,
				},
			},
		},
	}
}

// BuildHonchoConsumerEnv returns env vars added to the hermes container so it
// can talk to its sibling Honcho service.
func BuildHonchoConsumerEnv(inst *hermesv1.HermesInstance) []corev1.EnvVar {
	if !honchoEnabled(inst) {
		return nil
	}
	out := []corev1.EnvVar{
		{Name: "HONCHO_BASE_URL", Value: fmt.Sprintf("http://%s:8000", HonchoServiceName(inst))},
	}
	if ref := inst.Spec.ProfileStore.Honcho.APIKeySecretRef; ref != nil {
		out = append(out, corev1.EnvVar{
			Name: "HONCHO_API_KEY",
			ValueFrom: &corev1.EnvVarSource{
				SecretKeyRef: &corev1.SecretKeySelector{
					LocalObjectReference: ref.LocalObjectReference,
					Key:                  ref.Key,
				},
			},
		})
	}
	return out
}

func honchoImageRef(h hermesv1.HonchoSpec) string {
	repo := h.Image.Repository
	if repo == "" {
		repo = "ghcr.io/plastic-labs/honcho"
	}
	tag := h.Image.Tag
	if tag == "" {
		tag = "0.1.0"
	}
	return fmt.Sprintf("%s:%s", repo, tag)
}

func honchoPullPolicy(h hermesv1.HonchoSpec) corev1.PullPolicy {
	if h.Image.PullPolicy == "" {
		return corev1.PullIfNotPresent
	}
	return corev1.PullPolicy(h.Image.PullPolicy)
}
```

- [ ] **Step 4: Run tests to verify they pass**

```bash
go test ./internal/resources/... -run TestHoncho -v
go test ./internal/resources/... -run TestBuildHoncho -v
```
Expected: all PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/resources/honcho.go internal/resources/honcho_test.go
git commit -m "feat(resources): add Honcho profile-store builders (Deployment, Service, PVC, consumer env)"
```

---

## Task 11: Merge gateway config fragments into the rendered `config.yaml`

**Files:**
- Modify: `internal/resources/configmap.go`, `internal/resources/configmap_test.go`

Plan 2's `BuildConfigMap` writes `~/.hermes/config.yaml` from `spec.config.raw` (or `spec.config.configMapRef`). This task layers `BuildGatewayConfigFragments` onto it.

- [ ] **Step 1: Add a failing test in `configmap_test.go`**

Append to `internal/resources/configmap_test.go`:

```go
func TestBuildConfigMap_MergesGatewayFragments(t *testing.T) {
	inst := &hermesv1.HermesInstance{
		ObjectMeta: metav1.ObjectMeta{Name: "demo", Namespace: "agents"},
		Spec: hermesv1.HermesInstanceSpec{
			Config: hermesv1.ConfigSpec{Raw: "schedules:\n  morning: '0 8 * * *'\n"},
			Gateways: hermesv1.GatewaysSpec{
				Telegram: hermesv1.TelegramGatewaySpec{Enabled: Ptr(true), WebhookURL: "https://x/tg"},
			},
		},
	}
	cm := BuildConfigMap(inst)
	body := cm.Data["config.yaml"]
	assert.Contains(t, body, "schedules:", "user-provided raw config preserved")
	assert.Contains(t, body, "gateways:")
	assert.Contains(t, body, "telegram:")
	assert.Contains(t, body, "webhookURL: https://x/tg")
}

func TestBuildConfigMap_NoGatewaysWhenAllDisabled(t *testing.T) {
	inst := &hermesv1.HermesInstance{
		ObjectMeta: metav1.ObjectMeta{Name: "demo", Namespace: "agents"},
		Spec: hermesv1.HermesInstanceSpec{
			Config: hermesv1.ConfigSpec{Raw: "{}\n"},
		},
	}
	cm := BuildConfigMap(inst)
	assert.NotContains(t, cm.Data["config.yaml"], "gateways:", "no gateways block when none enabled")
}
```

- [ ] **Step 2: Run tests to verify they fail**

```bash
go test ./internal/resources/... -run TestBuildConfigMap_Merges -v
```
Expected: failures because the current builder does not yet merge gateway fragments.

- [ ] **Step 3: Update `configmap.go` to merge gateway fragments**

Modify the body of `BuildConfigMap` so that after producing the user's raw config, it deep-merges the result of `BuildGatewayConfigFragments(inst)` under the top-level key `gateways`. Use `sigs.k8s.io/yaml` (already a transitive dep via kubebuilder) for YAML marshal/unmarshal. The merge rule is "structural deep-merge, gateway fragments win": users who want full control over a sub-key can disable the gateway and write their own config.

Add this helper to `internal/resources/configmap.go`:

```go
import (
	"fmt"

	"sigs.k8s.io/yaml"
)

// mergeGatewayFragments parses the user's raw config YAML, deep-merges the
// builder-derived gateway fragments under the top-level `gateways` key, and
// re-marshals. Returns the original string verbatim if no gateway is enabled.
func mergeGatewayFragments(rawConfig string, frags map[string]any) (string, error) {
	if len(frags) == 0 {
		return rawConfig, nil
	}
	var root map[string]any
	if err := yaml.Unmarshal([]byte(rawConfig), &root); err != nil {
		return "", fmt.Errorf("parse user config: %w", err)
	}
	if root == nil {
		root = map[string]any{}
	}

	existing, _ := root["gateways"].(map[string]any)
	if existing == nil {
		existing = map[string]any{}
	}
	for k, v := range frags {
		existing[k] = v // gateway fragments win (operator owns this sub-tree)
	}
	root["gateways"] = existing

	out, err := yaml.Marshal(root)
	if err != nil {
		return "", fmt.Errorf("marshal merged config: %w", err)
	}
	return string(out), nil
}
```

And update the existing `BuildConfigMap` body so the `config.yaml` data key is produced as:

```go
raw := inst.Spec.Config.Raw
if raw == "" {
    raw = "{}\n"
}
merged, err := mergeGatewayFragments(raw, BuildGatewayConfigFragments(inst))
if err != nil {
    // Refuse to silently drop a bad user config: surface via panic; callers
    // catch this via the controller's recover. The validating webhook should
    // reject malformed YAML before we get here.
    panic(fmt.Sprintf("invalid spec.config.raw: %v", err))
}
cm.Data["config.yaml"] = merged
```

> Plan 2 must have produced a `cm := &corev1.ConfigMap{...}` local variable in `BuildConfigMap`. If Plan 2 wrote the function differently, the integration point is wherever `cm.Data["config.yaml"]` is assigned.

- [ ] **Step 4: Run tests to verify they pass**

```bash
go test ./internal/resources/... -run TestBuildConfigMap -v
```
Expected: all PASS, including pre-existing tests from Plan 1/2.

- [ ] **Step 5: Commit**

```bash
git add internal/resources/configmap.go internal/resources/configmap_test.go
git commit -m "feat(resources): merge gateway config fragments into rendered ~/.hermes/config.yaml"
```

---

## Task 12: Wire runtime + gateways + honcho into the StatefulSet builder

**Files:**
- Modify: `internal/resources/statefulset.go`, `internal/resources/statefulset_test.go`

This is the integration point: the StatefulSet builder consumes the three new builders (`BuildRuntimeInitContainers`, `BuildRuntimeVolumes`, `BuildRuntimeVolumeMounts`, `BuildGatewayEnv`, `BuildGatewayEnvFrom`, `BuildHonchoConsumerEnv`) and appends their outputs in the right places.

- [ ] **Step 1: Add failing tests in `statefulset_test.go`**

Append:

```go
func TestBuildStatefulSet_RuntimeInitContainersAppended(t *testing.T) {
	inst := minimalInstance()
	inst.Spec.Runtime = hermesv1.RuntimeSpec{
		UV:               hermesv1.UVSpec{Enabled: Ptr(true)},
		ExtraPipPackages: []string{"polars"},
	}
	sts := BuildStatefulSet(inst)
	names := []string{}
	for _, c := range sts.Spec.Template.Spec.InitContainers {
		names = append(names, c.Name)
	}
	assert.Contains(t, names, "init-uv")
	assert.Contains(t, names, "init-pip")
}

func TestBuildStatefulSet_GatewayEnvWired(t *testing.T) {
	inst := minimalInstance()
	inst.Spec.Gateways = hermesv1.GatewaysSpec{
		Telegram: hermesv1.TelegramGatewaySpec{
			Enabled: Ptr(true),
			BotTokenSecretRef: &corev1.SecretKeySelector{
				LocalObjectReference: corev1.LocalObjectReference{Name: "tg"},
				Key:                  "token",
			},
		},
	}
	sts := BuildStatefulSet(inst)
	c := sts.Spec.Template.Spec.Containers[0]
	hasToken := false
	for _, e := range c.Env {
		if e.Name == "TELEGRAM_BOT_TOKEN" && e.ValueFrom != nil && e.ValueFrom.SecretKeyRef.Name == "tg" {
			hasToken = true
		}
	}
	assert.True(t, hasToken, "TELEGRAM_BOT_TOKEN sourced from tg secret")
}

func TestBuildStatefulSet_HonchoEnvWired(t *testing.T) {
	inst := minimalInstance()
	inst.Spec.ProfileStore = hermesv1.ProfileStoreSpec{
		Honcho: hermesv1.HonchoSpec{
			Enabled: Ptr(true),
			APIKeySecretRef: &corev1.SecretKeySelector{
				LocalObjectReference: corev1.LocalObjectReference{Name: "honcho-secret"},
				Key:                  "api-key",
			},
		},
	}
	sts := BuildStatefulSet(inst)
	c := sts.Spec.Template.Spec.Containers[0]
	byName := map[string]corev1.EnvVar{}
	for _, e := range c.Env {
		byName[e.Name] = e
	}
	assert.Equal(t, "http://demo-honcho:8000", byName["HONCHO_BASE_URL"].Value)
	assert.NotNil(t, byName["HONCHO_API_KEY"].ValueFrom)
}

func TestBuildStatefulSet_UVCacheVolume(t *testing.T) {
	inst := minimalInstance()
	inst.Spec.Runtime = hermesv1.RuntimeSpec{UV: hermesv1.UVSpec{Enabled: Ptr(true)}}
	sts := BuildStatefulSet(inst)
	vols := sts.Spec.Template.Spec.Volumes
	found := false
	for _, v := range vols {
		if v.Name == "uv-cache" {
			found = true
		}
	}
	assert.True(t, found, "uv-cache volume present")

	mounts := sts.Spec.Template.Spec.Containers[0].VolumeMounts
	foundMount := false
	for _, m := range mounts {
		if m.Name == "uv-cache" && m.MountPath == "/home/hermes/.cache/uv" {
			foundMount = true
		}
	}
	assert.True(t, foundMount, "uv-cache mounted at /home/hermes/.cache/uv")
}
```

- [ ] **Step 2: Run tests to verify they fail**

```bash
go test ./internal/resources/... -run TestBuildStatefulSet -v
```
Expected: new failures (init containers not wired, env vars missing).

- [ ] **Step 3: Update `BuildStatefulSet` to append the new pieces**

Inside `BuildStatefulSet` in `internal/resources/statefulset.go`:

```go
// (1) Append init containers from runtime.
sts.Spec.Template.Spec.InitContainers = append(
    sts.Spec.Template.Spec.InitContainers,
    BuildRuntimeInitContainers(inst)...,
)

// (2) Extend the main container's env. Order matters for `kubectl describe`
// readability but not for runtime: gateway env first, then honcho consumer env,
// then user-supplied env (Plan 2's spec.env) so user explicit values win on
// duplicates.
c := &sts.Spec.Template.Spec.Containers[0]
c.Env = append(c.Env, BuildGatewayEnv(inst)...)
c.Env = append(c.Env, BuildHonchoConsumerEnv(inst)...)
// spec.env from Plan 2 is already appended further down in the original builder;
// leave that path untouched.

// (3) Extend envFrom (whole-secret) entries.
c.EnvFrom = append(c.EnvFrom, BuildGatewayEnvFrom(inst)...)

// (4) Extend volumes + volume mounts for runtime caches.
sts.Spec.Template.Spec.Volumes = append(
    sts.Spec.Template.Spec.Volumes,
    BuildRuntimeVolumes(inst)...,
)
c.VolumeMounts = append(c.VolumeMounts, BuildRuntimeVolumeMounts(inst)...)
```

Place these mutations *after* `sts` is fully constructed by the existing builder body but *before* the `return sts` statement. The mutations are additive: they cannot remove anything Plan 1/2 set.

- [ ] **Step 4: Run tests to verify they pass**

```bash
go test ./internal/resources/... -run TestBuildStatefulSet -v
```
Expected: all PASS (new + pre-existing).

- [ ] **Step 5: Idempotency canary: full `BuildStatefulSet` twice on same input yields identical output**

This re-confirms Plan 1's idempotency canary against the new wiring. Append to `statefulset_test.go`:

```go
func TestBuildStatefulSet_IdempotentWithRuntimeGatewaysHoncho(t *testing.T) {
	inst := minimalInstance()
	inst.Spec.Runtime = hermesv1.RuntimeSpec{UV: hermesv1.UVSpec{Enabled: Ptr(true)}}
	inst.Spec.Gateways = hermesv1.GatewaysSpec{
		Telegram: hermesv1.TelegramGatewaySpec{
			Enabled: Ptr(true),
			BotTokenSecretRef: &corev1.SecretKeySelector{
				LocalObjectReference: corev1.LocalObjectReference{Name: "tg"}, Key: "token",
			},
		},
	}
	inst.Spec.ProfileStore = hermesv1.ProfileStoreSpec{
		Honcho: hermesv1.HonchoSpec{
			Enabled:         Ptr(true),
			APIKeySecretRef: &corev1.SecretKeySelector{LocalObjectReference: corev1.LocalObjectReference{Name: "honcho"}, Key: "api-key"},
		},
	}
	a := BuildStatefulSet(inst)
	b := BuildStatefulSet(inst)
	assert.Equal(t, a, b, "pure builder must be deterministic")
}
```

```bash
go test ./internal/resources/... -run TestBuildStatefulSet_Idempotent -v
```
Expected: PASS.

- [ ] **Step 6: Commit**

```bash
git add internal/resources/statefulset.go internal/resources/statefulset_test.go
git commit -m "feat(resources): wire runtime init containers + gateway env + honcho consumer env into StatefulSet"
```

---

## Task 13: NetworkPolicy: gateway egress + Honcho ingress isolation

**Files:**
- Modify: `internal/resources/networkpolicy.go`, `internal/resources/networkpolicy_test.go`

Plan 2 produced a default-deny `NetworkPolicy` for the hermes pod (`BuildNetworkPolicy(inst)`) plus an extension hook `ExtraEgressRules(inst)`. This task:
1. Implements `ExtraEgressRules` to return one rule per enabled gateway endpoint (port 443/TCP).
2. Adds a second NetworkPolicy `BuildHonchoNetworkPolicy(inst)` that allows ingress only from the parent hermes pod and denies egress entirely (Honcho is a closed appliance: see lesson `honcho service:8000` in spec §4.1).

- [ ] **Step 1: Add failing tests in `networkpolicy_test.go`**

Append:

```go
func TestExtraEgressRules_TelegramAndDiscord(t *testing.T) {
	inst := &hermesv1.HermesInstance{
		ObjectMeta: metav1.ObjectMeta{Name: "demo", Namespace: "agents"},
		Spec: hermesv1.HermesInstanceSpec{
			Gateways: hermesv1.GatewaysSpec{
				Telegram: hermesv1.TelegramGatewaySpec{Enabled: Ptr(true)},
				Discord:  hermesv1.DiscordGatewaySpec{Enabled: Ptr(true)},
			},
		},
	}
	rules := ExtraEgressRules(inst)
	// Each enabled gateway contributes one rule with one or more To CIDRs OR
	// a NamespaceSelector / namedPort entry. For now we assert presence by port.
	hasTCP443 := false
	for _, r := range rules {
		for _, p := range r.Ports {
			if p.Protocol != nil && *p.Protocol == corev1.ProtocolTCP && p.Port != nil && p.Port.IntVal == 443 {
				hasTCP443 = true
			}
		}
	}
	assert.True(t, hasTCP443, "at least one rule opens TCP/443 for gateway endpoints")
}

func TestExtraEgressRules_HonchoSibling(t *testing.T) {
	inst := &hermesv1.HermesInstance{
		ObjectMeta: metav1.ObjectMeta{Name: "demo", Namespace: "agents"},
		Spec: hermesv1.HermesInstanceSpec{
			ProfileStore: hermesv1.ProfileStoreSpec{
				Honcho: hermesv1.HonchoSpec{Enabled: Ptr(true)},
			},
		},
	}
	rules := ExtraEgressRules(inst)
	foundHoncho := false
	for _, r := range rules {
		for _, peer := range r.To {
			if peer.PodSelector != nil && peer.PodSelector.MatchLabels["app.kubernetes.io/instance"] == "demo-honcho" {
				foundHoncho = true
			}
		}
	}
	assert.True(t, foundHoncho, "egress to honcho sibling pod selector present")
}

func TestBuildHonchoNetworkPolicy_IngressOnlyFromHermes(t *testing.T) {
	inst := &hermesv1.HermesInstance{
		ObjectMeta: metav1.ObjectMeta{Name: "demo", Namespace: "agents"},
		Spec: hermesv1.HermesInstanceSpec{
			ProfileStore: hermesv1.ProfileStoreSpec{
				Honcho: hermesv1.HonchoSpec{Enabled: Ptr(true)},
			},
		},
	}
	np := BuildHonchoNetworkPolicy(inst)
	assert.Equal(t, "demo-honcho", np.Name)

	// Selector matches Honcho pods only.
	assert.Equal(t, "honcho", np.Spec.PodSelector.MatchLabels["app.kubernetes.io/name"])

	// Ingress: from hermes pod by label.
	require := np.Spec.Ingress
	assert.Len(t, require, 1)
	from := require[0].From
	assert.Len(t, from, 1)
	assert.Equal(t, "hermes-agent", from[0].PodSelector.MatchLabels["app.kubernetes.io/name"])
	assert.Equal(t, "demo", from[0].PodSelector.MatchLabels["app.kubernetes.io/instance"])

	// Egress: empty list means deny-all (with an explicit PolicyType).
	assert.Empty(t, np.Spec.Egress)
	assert.Contains(t, np.Spec.PolicyTypes, networkingv1.PolicyTypeEgress)
}
```

- [ ] **Step 2: Run tests to verify they fail**

```bash
go test ./internal/resources/... -run TestExtraEgressRules -v
go test ./internal/resources/... -run TestBuildHonchoNetworkPolicy -v
```

- [ ] **Step 3: Implement `ExtraEgressRules` and `BuildHonchoNetworkPolicy`**

In `internal/resources/networkpolicy.go`, replace the stub `ExtraEgressRules` (Plan 2 left it returning `nil`) with:

```go
// ExtraEgressRules returns the per-instance egress rules driven by spec.gateways
// and spec.profileStore. The base default-deny NetworkPolicy from Plan 2
// always allows DNS to kube-dns; everything else is opt-in via this list.
func ExtraEgressRules(inst *hermesv1.HermesInstance) []networkingv1.NetworkPolicyEgressRule {
	var rules []networkingv1.NetworkPolicyEgressRule

	// Gateway endpoints: a single rule per gateway, port 443/TCP, peer = "any"
	// (since these are public internet endpoints). We do NOT translate
	// hostnames to CIDRs here: that's a NetworkPolicy-implementation concern
	// and the well-known endpoints are documented in docs/conventions.md.
	if endpoints := BuildGatewayEgressEndpoints(inst); len(endpoints) > 0 {
		port443 := intstr.FromInt(443)
		tcp := corev1.ProtocolTCP
		rules = append(rules, networkingv1.NetworkPolicyEgressRule{
			// No `To` => all destinations. CNI plugins that support FQDN
			// peers can match the hostnames in conventions.md instead.
			Ports: []networkingv1.NetworkPolicyPort{{
				Protocol: &tcp,
				Port:     &port443,
			}},
		})
	}

	// Honcho sibling pod, port 8000/TCP.
	if honchoEnabled(inst) {
		port8000 := intstr.FromInt(8000)
		tcp := corev1.ProtocolTCP
		rules = append(rules, networkingv1.NetworkPolicyEgressRule{
			To: []networkingv1.NetworkPolicyPeer{{
				PodSelector: &metav1.LabelSelector{MatchLabels: map[string]string{
					"app.kubernetes.io/name":     "honcho",
					"app.kubernetes.io/instance": HonchoDeploymentName(inst),
				}},
			}},
			Ports: []networkingv1.NetworkPolicyPort{{
				Protocol: &tcp,
				Port:     &port8000,
			}},
		})
	}

	return rules
}

// BuildHonchoNetworkPolicy returns the NetworkPolicy that scopes the Honcho
// companion: ingress only from the parent hermes pod, egress denied entirely.
// Returns nil when honcho is not enabled.
func BuildHonchoNetworkPolicy(inst *hermesv1.HermesInstance) *networkingv1.NetworkPolicy {
	if !honchoEnabled(inst) {
		return nil
	}
	port8000 := intstr.FromInt(8000)
	tcp := corev1.ProtocolTCP
	return &networkingv1.NetworkPolicy{
		ObjectMeta: metav1.ObjectMeta{
			Name:      HonchoDeploymentName(inst),
			Namespace: inst.Namespace,
			Labels:    HonchoLabels(inst),
		},
		Spec: networkingv1.NetworkPolicySpec{
			PodSelector: metav1.LabelSelector{MatchLabels: map[string]string{
				"app.kubernetes.io/name":     "honcho",
				"app.kubernetes.io/instance": HonchoDeploymentName(inst),
			}},
			PolicyTypes: []networkingv1.PolicyType{
				networkingv1.PolicyTypeIngress,
				networkingv1.PolicyTypeEgress, // empty Egress list => deny-all
			},
			Ingress: []networkingv1.NetworkPolicyIngressRule{{
				From: []networkingv1.NetworkPolicyPeer{{
					PodSelector: &metav1.LabelSelector{MatchLabels: map[string]string{
						"app.kubernetes.io/name":     "hermes-agent",
						"app.kubernetes.io/instance": inst.Name,
					}},
				}},
				Ports: []networkingv1.NetworkPolicyPort{{Protocol: &tcp, Port: &port8000}},
			}},
			// Egress omitted = deny-all under PolicyTypes: [Ingress, Egress].
		},
	}
}
```

> **Import note:** ensure `networkingv1 "k8s.io/api/networking/v1"`, `corev1`, `metav1`, and `intstr "k8s.io/apimachinery/pkg/util/intstr"` are imported. Plan 2's networkpolicy.go has the first three; `intstr` may already be imported via Plan 1's service.go but networkpolicy.go needs its own.

- [ ] **Step 4: Run tests to verify they pass**

```bash
go test ./internal/resources/... -run TestExtraEgressRules -v
go test ./internal/resources/... -run TestBuildHonchoNetworkPolicy -v
```
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/resources/networkpolicy.go internal/resources/networkpolicy_test.go
git commit -m "feat(resources): per-gateway egress rules + Honcho-scoped NetworkPolicy"
```

---

## Task 14: Reconciler: orchestrate Honcho resources

**Files:**
- Modify: `internal/controller/hermesinstance_controller.go`

- [ ] **Step 1: Add a `reconcileHoncho` method**

Inside `internal/controller/hermesinstance_controller.go`, add the following method on `HermesInstanceReconciler` (placement: right after Plan 2's `reconcileService` method):

```go
// reconcileHoncho creates/updates/deletes the Honcho companion resources
// (Deployment, Service, PVC, scoped NetworkPolicy) based on
// spec.profileStore.honcho.enabled. Toggling from true → false deletes the
// resources but leaves the PVC behind (data safety); operators must delete
// the PVC manually.
func (r *HermesInstanceReconciler) reconcileHoncho(ctx context.Context, inst *hermesv1.HermesInstance) error {
	enabled := inst.Spec.ProfileStore.Honcho.Enabled != nil && *inst.Spec.ProfileStore.Honcho.Enabled

	// PVC: create-only when enabled, never deleted automatically.
	if enabled && (inst.Spec.ProfileStore.Honcho.Persistence.Enabled == nil || *inst.Spec.ProfileStore.Honcho.Persistence.Enabled) {
		desired := resources.BuildHonchoPVC(inst)
		if err := r.ensurePVC(ctx, inst, desired); err != nil {
			return fmt.Errorf("honcho PVC: %w", err)
		}
	}

	// Service.
	if enabled {
		desired := resources.BuildHonchoService(inst)
		if err := r.ensureService(ctx, inst, desired); err != nil {
			return fmt.Errorf("honcho Service: %w", err)
		}
	} else {
		if err := r.deleteIfExists(ctx, &corev1.Service{}, types.NamespacedName{Namespace: inst.Namespace, Name: resources.HonchoServiceName(inst)}); err != nil {
			return fmt.Errorf("delete honcho Service: %w", err)
		}
	}

	// Deployment.
	if enabled {
		desired := resources.BuildHonchoDeployment(inst)
		if err := r.ensureDeployment(ctx, inst, desired); err != nil {
			return fmt.Errorf("honcho Deployment: %w", err)
		}
	} else {
		if err := r.deleteIfExists(ctx, &appsv1.Deployment{}, types.NamespacedName{Namespace: inst.Namespace, Name: resources.HonchoDeploymentName(inst)}); err != nil {
			return fmt.Errorf("delete honcho Deployment: %w", err)
		}
	}

	// NetworkPolicy (only when the parent instance's networking.networkPolicy.enabled is true).
	if enabled && networkPolicyEnabled(inst) {
		desired := resources.BuildHonchoNetworkPolicy(inst)
		if err := r.ensureNetworkPolicy(ctx, inst, desired); err != nil {
			return fmt.Errorf("honcho NetworkPolicy: %w", err)
		}
	} else {
		if err := r.deleteIfExists(ctx, &networkingv1.NetworkPolicy{}, types.NamespacedName{Namespace: inst.Namespace, Name: resources.HonchoDeploymentName(inst)}); err != nil {
			return fmt.Errorf("delete honcho NetworkPolicy: %w", err)
		}
	}

	return nil
}

// deleteIfExists deletes a named object, ignoring NotFound. Helper used by
// every "feature toggle off" path.
func (r *HermesInstanceReconciler) deleteIfExists(ctx context.Context, obj client.Object, key types.NamespacedName) error {
	obj.SetName(key.Name)
	obj.SetNamespace(key.Namespace)
	if err := r.Delete(ctx, obj); err != nil {
		if apierrors.IsNotFound(err) {
			return nil
		}
		return err
	}
	return nil
}
```

> **Helpers `ensurePVC`/`ensureService`/`ensureDeployment`/`ensureNetworkPolicy`/`networkPolicyEnabled`** are from Plan 2. If Plan 2 named them differently (e.g. `r.applyPVC`), use the actual names. Reconcile Guard CI ensures these wrappers use `controllerutil.CreateOrUpdate` correctly.

- [ ] **Step 2: Call `reconcileHoncho` from `Reconcile`**

Find the chain of `reconcilePVC` / `reconcileConfigMap` / `reconcileService` / `reconcileStatefulSet` calls. After `reconcileStatefulSet` (and before status updates), add:

```go
if err := r.reconcileHoncho(ctx, &inst); err != nil {
    return ctrl.Result{}, fmt.Errorf("reconcile Honcho: %w", err)
}
```

- [ ] **Step 3: RBAC marker: Deployments**

Add to the existing `+kubebuilder:rbac:` block at the top of the reconciler:

```go
// +kubebuilder:rbac:groups=apps,resources=deployments,verbs=get;list;watch;create;update;patch;delete
```

(Plan 2 should already have `services`, `persistentvolumeclaims`, and `networkpolicies` covered.)

- [ ] **Step 4: Run generate + build**

```bash
make generate manifests
go build ./...
```
Expected: success.

- [ ] **Step 5: Commit**

```bash
git add internal/controller/hermesinstance_controller.go config/rbac/
git commit -m "feat(controller): reconcile Honcho Deployment/Service/PVC/NetworkPolicy from spec.profileStore"
```

---

## Task 15: Status: `ProfileStoreReady` condition

**Files:**
- Modify: `internal/controller/hermesinstance_controller.go`, `docs/conditions.md`

Spec §3 lists `ProfileStoreReady` as one of the standard `HermesInstance` conditions. This task wires it.

- [ ] **Step 1: Add the condition constant**

In `internal/controller/hermesinstance_controller.go` (top of file, with the other condition constants Plan 2 introduced):

```go
const (
    ConditionProfileStoreReady = "ProfileStoreReady"

    ReasonProfileStoreDisabled       = "Disabled"
    ReasonProfileStoreDeploymentDown = "DeploymentNotReady"
    ReasonProfileStoreReady          = "Ready"
)
```

- [ ] **Step 2: Compute the condition at end of Reconcile**

Append a helper `updateProfileStoreCondition`:

```go
func (r *HermesInstanceReconciler) updateProfileStoreCondition(ctx context.Context, inst *hermesv1.HermesInstance) error {
    cond := metav1.Condition{Type: ConditionProfileStoreReady, ObservedGeneration: inst.Generation}

    if !honchoEnabledRC(inst) {
        cond.Status = metav1.ConditionTrue
        cond.Reason = ReasonProfileStoreDisabled
        cond.Message = "Honcho profile store disabled (spec.profileStore.honcho.enabled=false)"
    } else {
        var dep appsv1.Deployment
        key := types.NamespacedName{Namespace: inst.Namespace, Name: resources.HonchoDeploymentName(inst)}
        if err := r.Get(ctx, key, &dep); err != nil {
            cond.Status = metav1.ConditionFalse
            cond.Reason = ReasonProfileStoreDeploymentDown
            cond.Message = fmt.Sprintf("Honcho Deployment not found: %v", err)
        } else if dep.Status.ReadyReplicas >= 1 {
            cond.Status = metav1.ConditionTrue
            cond.Reason = ReasonProfileStoreReady
            cond.Message = "Honcho Deployment has ≥1 ready replica"
        } else {
            cond.Status = metav1.ConditionFalse
            cond.Reason = ReasonProfileStoreDeploymentDown
            cond.Message = fmt.Sprintf("Honcho Deployment has %d/%d ready replicas", dep.Status.ReadyReplicas, dep.Status.Replicas)
        }
    }

    meta.SetStatusCondition(&inst.Status.Conditions, cond)
    return r.Status().Update(ctx, inst)
}

func honchoEnabledRC(inst *hermesv1.HermesInstance) bool {
    return inst.Spec.ProfileStore.Honcho.Enabled != nil && *inst.Spec.ProfileStore.Honcho.Enabled
}
```

Then call it from `Reconcile` after `reconcileHoncho` and before returning the final result. Imports needed: `"k8s.io/apimachinery/pkg/api/meta"` for `meta.SetStatusCondition`.

- [ ] **Step 3: Add the condition to `docs/conditions.md`**

Append to `docs/conditions.md` (created by Plan 1 or Plan 2):

```markdown
### `ProfileStoreReady` (HermesInstance)

Indicates whether the optional Honcho profile-store companion is healthy.

| Status | Reason | Meaning |
|---|---|---|
| `True` | `Disabled` | `spec.profileStore.honcho.enabled` is false; the operator did not create Honcho resources. |
| `True` | `Ready` | Honcho Deployment has ≥1 ready replica. |
| `False` | `DeploymentNotReady` | Honcho Deployment is missing, scaling up, or its readiness probe is failing. |
```

- [ ] **Step 4: Build to verify**

```bash
go build ./...
go vet ./...
```
Expected: success.

- [ ] **Step 5: Commit**

```bash
git add internal/controller/hermesinstance_controller.go docs/conditions.md
git commit -m "feat(status): add ProfileStoreReady condition on HermesInstance"
```

---

## Task 16: Validating webhook: gateway secret existence warnings + `profiles` allowedAction

**Files:**
- Modify: `internal/webhook/hermesinstance_validator.go`, `internal/webhook/hermesinstance_validator_test.go`

Per spec §7.3: the validator **warns** (does not deny) when a gateway is `enabled: true` but its referenced Secret is missing at admission time. This avoids races with GitOps applying instance + secrets out of order. The validator also validates that `spec.selfConfigure.allowedActions` is a subset of `{skills, config, envVars, workspaceFiles, profiles}`: adding `profiles`.

- [ ] **Step 1: Add failing tests**

Append to `internal/webhook/hermesinstance_validator_test.go`:

```go
func TestValidateGateways_TelegramSecretMissingProducesWarning(t *testing.T) {
	v := newValidatorWithObjs() // no Secret in fake client
	inst := &hermesv1.HermesInstance{
		ObjectMeta: metav1.ObjectMeta{Name: "demo", Namespace: "agents"},
		Spec: hermesv1.HermesInstanceSpec{
			Gateways: hermesv1.GatewaysSpec{
				Telegram: hermesv1.TelegramGatewaySpec{
					Enabled: Ptr(true),
					BotTokenSecretRef: &corev1.SecretKeySelector{
						LocalObjectReference: corev1.LocalObjectReference{Name: "missing"},
						Key:                  "token",
					},
				},
			},
		},
	}
	warnings, err := v.ValidateCreate(context.Background(), inst)
	assert.NoError(t, err, "missing secret must NOT deny: only warn")
	assert.NotEmpty(t, warnings, "expect at least one warning")
	joined := strings.Join(warnings, " | ")
	assert.Contains(t, joined, "gateways.telegram.botTokenSecretRef")
	assert.Contains(t, joined, "missing")
}

func TestValidateGateways_TelegramEnabledWithoutSecretRefDenied(t *testing.T) {
	v := newValidatorWithObjs()
	inst := &hermesv1.HermesInstance{
		ObjectMeta: metav1.ObjectMeta{Name: "demo", Namespace: "agents"},
		Spec: hermesv1.HermesInstanceSpec{
			Gateways: hermesv1.GatewaysSpec{
				Telegram: hermesv1.TelegramGatewaySpec{Enabled: Ptr(true)}, // nil BotTokenSecretRef
			},
		},
	}
	_, err := v.ValidateCreate(context.Background(), inst)
	assert.Error(t, err, "enabled without any secret ref must be denied")
	assert.Contains(t, err.Error(), "botTokenSecretRef")
}

func TestValidateGateways_SecretExistsNoWarning(t *testing.T) {
	v := newValidatorWithObjs(&corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{Name: "tg", Namespace: "agents"},
		Data:       map[string][]byte{"token": []byte("x")},
	})
	inst := &hermesv1.HermesInstance{
		ObjectMeta: metav1.ObjectMeta{Name: "demo", Namespace: "agents"},
		Spec: hermesv1.HermesInstanceSpec{
			Gateways: hermesv1.GatewaysSpec{
				Telegram: hermesv1.TelegramGatewaySpec{
					Enabled: Ptr(true),
					BotTokenSecretRef: &corev1.SecretKeySelector{
						LocalObjectReference: corev1.LocalObjectReference{Name: "tg"},
						Key:                  "token",
					},
				},
			},
		},
	}
	warnings, err := v.ValidateCreate(context.Background(), inst)
	assert.NoError(t, err)
	for _, w := range warnings {
		assert.NotContains(t, w, "gateways.telegram")
	}
}

func TestValidateSelfConfigure_ProfilesActionAllowed(t *testing.T) {
	v := newValidatorWithObjs()
	inst := &hermesv1.HermesInstance{
		ObjectMeta: metav1.ObjectMeta{Name: "demo", Namespace: "agents"},
		Spec: hermesv1.HermesInstanceSpec{
			SelfConfigure: hermesv1.SelfConfigureSpec{
				Enabled:        Ptr(true),
				AllowedActions: []hermesv1.SelfConfigAction{"profiles"},
				ProtectedKeys:  []string{"provider.apiKey"},
			},
		},
	}
	_, err := v.ValidateCreate(context.Background(), inst)
	assert.NoError(t, err, "profiles is a valid action per spec §4.1")
}

func TestValidateSelfConfigure_UnknownActionDenied(t *testing.T) {
	v := newValidatorWithObjs()
	inst := &hermesv1.HermesInstance{
		ObjectMeta: metav1.ObjectMeta{Name: "demo", Namespace: "agents"},
		Spec: hermesv1.HermesInstanceSpec{
			SelfConfigure: hermesv1.SelfConfigureSpec{
				Enabled:        Ptr(true),
				AllowedActions: []hermesv1.SelfConfigAction{"reboot-cluster"},
				ProtectedKeys:  []string{"provider.apiKey"},
			},
		},
	}
	_, err := v.ValidateCreate(context.Background(), inst)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "reboot-cluster")
}

// newValidatorWithObjs constructs the validator with a fake client preloaded
// with the given secrets/configmaps. Mirrors Plan 2's helper but accepts an
// optional Secret list so this file's tests can stand alone if Plan 2's
// helper is namespaced differently.
func newValidatorWithObjs(objs ...client.Object) *HermesInstanceValidator {
	scheme := runtime.NewScheme()
	_ = corev1.AddToScheme(scheme)
	_ = hermesv1.AddToScheme(scheme)
	c := fake.NewClientBuilder().WithScheme(scheme).WithObjects(objs...).Build()
	return &HermesInstanceValidator{Client: c}
}
```

(If Plan 2 already exports `newValidator(...)` with the same shape, rename `newValidatorWithObjs` to match.)

- [ ] **Step 2: Run tests to verify failures**

```bash
go test ./internal/webhook/... -run TestValidateGateways -v
go test ./internal/webhook/... -run TestValidateSelfConfigure -v
```

- [ ] **Step 3: Implement the validation rules**

In `internal/webhook/hermesinstance_validator.go`, locate the `validate` function (created by Plan 2) and add a call to a new method `validateGateways(ctx, inst)` that returns `(warnings, err)`. Implementation:

```go
// validateGateways returns warnings for gateway secret references that cannot
// be resolved at admission time, and errors for structurally invalid bindings
// (enabled without any token secret reference at all).
func (v *HermesInstanceValidator) validateGateways(ctx context.Context, inst *hermesv1.HermesInstance) (admission.Warnings, error) {
	var warnings admission.Warnings
	g := inst.Spec.Gateways

	check := func(field string, enabled *bool, ref *corev1.SecretKeySelector, required bool) error {
		if enabled == nil || !*enabled {
			return nil
		}
		if ref == nil {
			if required {
				return fmt.Errorf("%s is required when the gateway is enabled", field)
			}
			return nil
		}
		var s corev1.Secret
		err := v.Client.Get(ctx, types.NamespacedName{Namespace: inst.Namespace, Name: ref.Name}, &s)
		if err != nil {
			if apierrors.IsNotFound(err) {
				warnings = append(warnings, fmt.Sprintf(
					"%s references Secret %q which is not present yet in namespace %q; the instance will block on rollout until the secret is created",
					field, ref.Name, inst.Namespace,
				))
				return nil
			}
			return fmt.Errorf("look up %s: %w", field, err)
		}
		if ref.Key != "" {
			if _, ok := s.Data[ref.Key]; !ok {
				warnings = append(warnings, fmt.Sprintf(
					"%s references key %q in Secret %q which is not present in the Secret's data",
					field, ref.Key, ref.Name,
				))
			}
		}
		return nil
	}

	if err := check("spec.gateways.telegram.botTokenSecretRef", g.Telegram.Enabled, g.Telegram.BotTokenSecretRef, true); err != nil {
		return warnings, err
	}
	if err := check("spec.gateways.discord.botTokenSecretRef", g.Discord.Enabled, g.Discord.BotTokenSecretRef, true); err != nil {
		return warnings, err
	}
	if err := check("spec.gateways.slack.botTokenSecretRef", g.Slack.Enabled, g.Slack.BotTokenSecretRef, true); err != nil {
		return warnings, err
	}
	// Slack app/signing are optional refs.
	if err := check("spec.gateways.slack.appTokenSecretRef", g.Slack.Enabled, g.Slack.AppTokenSecretRef, false); err != nil {
		return warnings, err
	}
	if err := check("spec.gateways.slack.signingSecretRef", g.Slack.Enabled, g.Slack.SigningSecretRef, false); err != nil {
		return warnings, err
	}
	if err := check("spec.gateways.whatsapp.providerSecretRef", g.WhatsApp.Enabled, g.WhatsApp.ProviderSecretRef, true); err != nil {
		return warnings, err
	}
	if err := check("spec.gateways.signal.phoneNumberSecretRef", g.Signal.Enabled, g.Signal.PhoneNumberSecretRef, true); err != nil {
		return warnings, err
	}
	if err := check("spec.gateways.signal.authTokenSecretRef", g.Signal.Enabled, g.Signal.AuthTokenSecretRef, true); err != nil {
		return warnings, err
	}

	// Honcho's api-key secret follows the same warning model.
	if isTrue(inst.Spec.ProfileStore.Honcho.Enabled) {
		if err := check("spec.profileStore.honcho.apiKeySecretRef", inst.Spec.ProfileStore.Honcho.Enabled, inst.Spec.ProfileStore.Honcho.APIKeySecretRef, true); err != nil {
			return warnings, err
		}
	}

	return warnings, nil
}

// isTrue mirrors resources.isTrue but kept private to the webhook package.
func isTrue(b *bool) bool { return b != nil && *b }
```

In `validateSelfConfigure` (created by Plan 2), update the allowed actions check to include `"profiles"`. The complete allow-list per spec §4.1 is: `{"skills", "config", "envVars", "workspaceFiles", "profiles"}`.

- [ ] **Step 4: Run tests**

```bash
go test ./internal/webhook/... -v
```
Expected: all pass.

- [ ] **Step 5: Commit**

```bash
git add internal/webhook/hermesinstance_validator.go internal/webhook/hermesinstance_validator_test.go
git commit -m "feat(webhook): warn on missing gateway/honcho secrets; allow 'profiles' in selfConfigure.allowedActions"
```

---

## Task 17: envtest: gateway env vars and egress rules end-to-end

**Files:**
- Modify: `internal/controller/hermesinstance_controller_test.go`

- [ ] **Step 1: Add an envtest `Describe` block for gateways**

Append to `internal/controller/hermesinstance_controller_test.go`:

```go
var _ = Describe("HermesInstance reconciler: gateways", func() {
	const (
		instName = "gateways-it"
		ns       = "default"
	)

	BeforeEach(func() {
		// Pre-seed the referenced Secret so the webhook doesn't only warn.
		sec := &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{Name: "tg-secret", Namespace: ns},
			Data:       map[string][]byte{"token": []byte("xxxx")},
		}
		_ = k8sClient.Create(ctx, sec)
	})

	AfterEach(func() {
		_ = k8sClient.Delete(ctx, &hermesv1.HermesInstance{ObjectMeta: metav1.ObjectMeta{Name: instName, Namespace: ns}})
		_ = k8sClient.Delete(ctx, &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: "tg-secret", Namespace: ns}})
	})

	It("propagates gateway env vars into the StatefulSet and an egress rule into the NetworkPolicy", func() {
		inst := &hermesv1.HermesInstance{
			ObjectMeta: metav1.ObjectMeta{Name: instName, Namespace: ns},
			Spec: hermesv1.HermesInstanceSpec{
				Gateways: hermesv1.GatewaysSpec{
					Telegram: hermesv1.TelegramGatewaySpec{
						Enabled: resources.Ptr(true),
						BotTokenSecretRef: &corev1.SecretKeySelector{
							LocalObjectReference: corev1.LocalObjectReference{Name: "tg-secret"},
							Key:                  "token",
						},
						AllowedUserIDs: []int64{42},
					},
				},
			},
		}
		Expect(k8sClient.Create(ctx, inst)).To(Succeed())

		Eventually(func(g Gomega) {
			var sts appsv1.StatefulSet
			g.Expect(k8sClient.Get(ctx, types.NamespacedName{Name: instName, Namespace: ns}, &sts)).To(Succeed())
			env := sts.Spec.Template.Spec.Containers[0].Env
			byName := map[string]corev1.EnvVar{}
			for _, e := range env {
				byName[e.Name] = e
			}
			g.Expect(byName).To(HaveKey("TELEGRAM_BOT_TOKEN"))
			g.Expect(byName["TELEGRAM_BOT_TOKEN"].ValueFrom).NotTo(BeNil())
			g.Expect(byName).To(HaveKeyWithValue("TELEGRAM_ALLOWED_USER_IDS", corev1.EnvVar{Name: "TELEGRAM_ALLOWED_USER_IDS", Value: "42"}))
		}, "10s", "200ms").Should(Succeed())

		Eventually(func(g Gomega) {
			var np networkingv1.NetworkPolicy
			g.Expect(k8sClient.Get(ctx, types.NamespacedName{Name: instName, Namespace: ns}, &np)).To(Succeed())
			// At least one egress rule opens TCP/443.
			has443 := false
			for _, r := range np.Spec.Egress {
				for _, p := range r.Ports {
					if p.Port != nil && p.Port.IntVal == 443 {
						has443 = true
					}
				}
			}
			g.Expect(has443).To(BeTrue(), "egress rule for gateway endpoints (443/TCP)")
		}, "10s", "200ms").Should(Succeed())
	})

	It("removes gateway env vars + egress rule when the gateway is toggled off", func() {
		inst := &hermesv1.HermesInstance{
			ObjectMeta: metav1.ObjectMeta{Name: instName, Namespace: ns},
			Spec: hermesv1.HermesInstanceSpec{
				Gateways: hermesv1.GatewaysSpec{
					Telegram: hermesv1.TelegramGatewaySpec{
						Enabled:           resources.Ptr(true),
						BotTokenSecretRef: &corev1.SecretKeySelector{LocalObjectReference: corev1.LocalObjectReference{Name: "tg-secret"}, Key: "token"},
					},
				},
			},
		}
		Expect(k8sClient.Create(ctx, inst)).To(Succeed())

		Eventually(func() bool {
			var sts appsv1.StatefulSet
			if err := k8sClient.Get(ctx, types.NamespacedName{Name: instName, Namespace: ns}, &sts); err != nil {
				return false
			}
			for _, e := range sts.Spec.Template.Spec.Containers[0].Env {
				if e.Name == "TELEGRAM_BOT_TOKEN" {
					return true
				}
			}
			return false
		}, "10s", "200ms").Should(BeTrue())

		// Toggle off.
		var live hermesv1.HermesInstance
		Expect(k8sClient.Get(ctx, types.NamespacedName{Name: instName, Namespace: ns}, &live)).To(Succeed())
		live.Spec.Gateways.Telegram.Enabled = resources.Ptr(false)
		Expect(k8sClient.Update(ctx, &live)).To(Succeed())

		Eventually(func() bool {
			var sts appsv1.StatefulSet
			if err := k8sClient.Get(ctx, types.NamespacedName{Name: instName, Namespace: ns}, &sts); err != nil {
				return true
			}
			for _, e := range sts.Spec.Template.Spec.Containers[0].Env {
				if e.Name == "TELEGRAM_BOT_TOKEN" {
					return false
				}
			}
			return true
		}, "10s", "200ms").Should(BeTrue(), "TELEGRAM_BOT_TOKEN env var removed after toggle")
	})
})
```

- [ ] **Step 2: Run the envtest suite**

```bash
make test
```
Expected: all PASS. If envtest's apiserver complains about missing CRDs, ensure `make manifests` ran first and `suite_test.go` points at `config/crd/bases/`.

- [ ] **Step 3: Commit**

```bash
git add internal/controller/hermesinstance_controller_test.go
git commit -m "test(controller): envtest gateway env + NetworkPolicy egress lifecycle (enable/toggle off)"
```

---

## Task 18: envtest: Honcho resources lifecycle

**Files:**
- Modify: `internal/controller/hermesinstance_controller_test.go`

- [ ] **Step 1: Add a `Describe` block for the Honcho lifecycle**

Append:

```go
var _ = Describe("HermesInstance reconciler: Honcho profile store", func() {
	const (
		instName = "honcho-it"
		ns       = "default"
	)

	AfterEach(func() {
		_ = k8sClient.Delete(ctx, &hermesv1.HermesInstance{ObjectMeta: metav1.ObjectMeta{Name: instName, Namespace: ns}})
	})

	It("creates Honcho Deployment+Service+PVC when honcho.enabled=true", func() {
		_ = k8sClient.Create(ctx, &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{Name: "honcho-secret", Namespace: ns},
			Data:       map[string][]byte{"api-key": []byte("k")},
		})
		inst := &hermesv1.HermesInstance{
			ObjectMeta: metav1.ObjectMeta{Name: instName, Namespace: ns},
			Spec: hermesv1.HermesInstanceSpec{
				ProfileStore: hermesv1.ProfileStoreSpec{
					Honcho: hermesv1.HonchoSpec{
						Enabled: resources.Ptr(true),
						APIKeySecretRef: &corev1.SecretKeySelector{
							LocalObjectReference: corev1.LocalObjectReference{Name: "honcho-secret"},
							Key:                  "api-key",
						},
					},
				},
			},
		}
		Expect(k8sClient.Create(ctx, inst)).To(Succeed())

		Eventually(func(g Gomega) {
			var dep appsv1.Deployment
			g.Expect(k8sClient.Get(ctx, types.NamespacedName{Name: instName + "-honcho", Namespace: ns}, &dep)).To(Succeed())
			var svc corev1.Service
			g.Expect(k8sClient.Get(ctx, types.NamespacedName{Name: instName + "-honcho", Namespace: ns}, &svc)).To(Succeed())
			var pvc corev1.PersistentVolumeClaim
			g.Expect(k8sClient.Get(ctx, types.NamespacedName{Name: instName + "-honcho-data", Namespace: ns}, &pvc)).To(Succeed())
		}, "10s", "200ms").Should(Succeed())

		// HONCHO_BASE_URL wired into the hermes pod.
		Eventually(func(g Gomega) {
			var sts appsv1.StatefulSet
			g.Expect(k8sClient.Get(ctx, types.NamespacedName{Name: instName, Namespace: ns}, &sts)).To(Succeed())
			env := sts.Spec.Template.Spec.Containers[0].Env
			byName := map[string]corev1.EnvVar{}
			for _, e := range env {
				byName[e.Name] = e
			}
			g.Expect(byName).To(HaveKey("HONCHO_BASE_URL"))
			g.Expect(byName["HONCHO_BASE_URL"].Value).To(Equal("http://" + instName + "-honcho:8000"))
		}, "10s", "200ms").Should(Succeed())
	})

	It("deletes Honcho Deployment+Service+NetworkPolicy when toggled off (PVC retained)", func() {
		_ = k8sClient.Create(ctx, &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{Name: "honcho-secret", Namespace: ns},
			Data:       map[string][]byte{"api-key": []byte("k")},
		})
		inst := &hermesv1.HermesInstance{
			ObjectMeta: metav1.ObjectMeta{Name: instName, Namespace: ns},
			Spec: hermesv1.HermesInstanceSpec{
				ProfileStore: hermesv1.ProfileStoreSpec{
					Honcho: hermesv1.HonchoSpec{
						Enabled:         resources.Ptr(true),
						APIKeySecretRef: &corev1.SecretKeySelector{LocalObjectReference: corev1.LocalObjectReference{Name: "honcho-secret"}, Key: "api-key"},
					},
				},
			},
		}
		Expect(k8sClient.Create(ctx, inst)).To(Succeed())
		Eventually(func() error {
			var dep appsv1.Deployment
			return k8sClient.Get(ctx, types.NamespacedName{Name: instName + "-honcho", Namespace: ns}, &dep)
		}, "10s", "200ms").Should(Succeed())

		// Toggle off.
		var live hermesv1.HermesInstance
		Expect(k8sClient.Get(ctx, types.NamespacedName{Name: instName, Namespace: ns}, &live)).To(Succeed())
		live.Spec.ProfileStore.Honcho.Enabled = resources.Ptr(false)
		Expect(k8sClient.Update(ctx, &live)).To(Succeed())

		Eventually(func() bool {
			var dep appsv1.Deployment
			return apierrors.IsNotFound(k8sClient.Get(ctx, types.NamespacedName{Name: instName + "-honcho", Namespace: ns}, &dep))
		}, "10s", "200ms").Should(BeTrue())
		Eventually(func() bool {
			var svc corev1.Service
			return apierrors.IsNotFound(k8sClient.Get(ctx, types.NamespacedName{Name: instName + "-honcho", Namespace: ns}, &svc))
		}, "10s", "200ms").Should(BeTrue())

		// PVC retained intentionally: operator does not delete data PVCs.
		var pvc corev1.PersistentVolumeClaim
		Expect(k8sClient.Get(ctx, types.NamespacedName{Name: instName + "-honcho-data", Namespace: ns}, &pvc)).To(Succeed())
	})
})
```

- [ ] **Step 2: Run the suite**

```bash
make test
```
Expected: PASS.

- [ ] **Step 3: Commit**

```bash
git add internal/controller/hermesinstance_controller_test.go
git commit -m "test(controller): envtest Honcho lifecycle (create on enable, delete on disable, PVC retained)"
```

---

## Task 19: Idempotency canary: full HermesInstance with all features enabled

**Files:**
- Modify: `internal/controller/hermesinstance_controller_test.go`

Plan 1 established the idempotency canary pattern: reconcile the same spec 10× in a row and assert `metadata.generation` and `resourceVersion` of the managed StatefulSet stop changing after the first reconcile. This task extends the canary to cover the full spec produced by Plan 3 (runtime + gateways + honcho).

- [ ] **Step 1: Add the canary `Describe`**

Append:

```go
var _ = Describe("HermesInstance reconciler: idempotency canary (Plan 3 surface)", func() {
	const (
		instName = "canary-plan3"
		ns       = "default"
	)

	AfterEach(func() {
		_ = k8sClient.Delete(ctx, &hermesv1.HermesInstance{ObjectMeta: metav1.ObjectMeta{Name: instName, Namespace: ns}})
	})

	It("does not bump StatefulSet generation after 10 reconciles with full Plan 3 spec", func() {
		_ = k8sClient.Create(ctx, &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{Name: "tg", Namespace: ns},
			Data:       map[string][]byte{"token": []byte("x")},
		})
		_ = k8sClient.Create(ctx, &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{Name: "honcho", Namespace: ns},
			Data:       map[string][]byte{"api-key": []byte("k")},
		})

		inst := &hermesv1.HermesInstance{
			ObjectMeta: metav1.ObjectMeta{Name: instName, Namespace: ns},
			Spec: hermesv1.HermesInstanceSpec{
				Runtime: hermesv1.RuntimeSpec{
					UV:               hermesv1.UVSpec{Enabled: resources.Ptr(true)},
					ExtraPipPackages: []string{"polars"},
				},
				Gateways: hermesv1.GatewaysSpec{
					Telegram: hermesv1.TelegramGatewaySpec{
						Enabled:           resources.Ptr(true),
						BotTokenSecretRef: &corev1.SecretKeySelector{LocalObjectReference: corev1.LocalObjectReference{Name: "tg"}, Key: "token"},
						AllowedUserIDs:    []int64{42, 1337},
					},
				},
				ProfileStore: hermesv1.ProfileStoreSpec{
					Honcho: hermesv1.HonchoSpec{
						Enabled:         resources.Ptr(true),
						APIKeySecretRef: &corev1.SecretKeySelector{LocalObjectReference: corev1.LocalObjectReference{Name: "honcho"}, Key: "api-key"},
					},
				},
			},
		}
		Expect(k8sClient.Create(ctx, inst)).To(Succeed())

		var stsAfterFirst appsv1.StatefulSet
		Eventually(func(g Gomega) {
			g.Expect(k8sClient.Get(ctx, types.NamespacedName{Name: instName, Namespace: ns}, &stsAfterFirst)).To(Succeed())
			g.Expect(stsAfterFirst.Generation).To(BeNumerically(">=", int64(1)))
		}, "10s", "200ms").Should(Succeed())

		// Trigger 10 explicit reconciles via no-op annotation patches.
		for i := 0; i < 10; i++ {
			var live hermesv1.HermesInstance
			Expect(k8sClient.Get(ctx, types.NamespacedName{Name: instName, Namespace: ns}, &live)).To(Succeed())
			if live.Annotations == nil {
				live.Annotations = map[string]string{}
			}
			live.Annotations["hermes.agent/canary-tick"] = fmt.Sprintf("%d", i)
			Expect(k8sClient.Update(ctx, &live)).To(Succeed())
			time.Sleep(200 * time.Millisecond)
		}

		// After 10 ticks, StatefulSet.generation must not have advanced.
		var stsAfterTen appsv1.StatefulSet
		Expect(k8sClient.Get(ctx, types.NamespacedName{Name: instName, Namespace: ns}, &stsAfterTen)).To(Succeed())
		Expect(stsAfterTen.Generation).To(Equal(stsAfterFirst.Generation),
			"StatefulSet generation must not advance under repeat reconciles with unchanged spec")
	})
})
```

- [ ] **Step 2: Run the canary**

```bash
go test ./internal/controller/... -run "idempotency canary" -v -count=3
```
Expected: PASS on every run. `-count=3` flushes the test cache and surfaces any flakes early.

If the canary fails: the most common cause is a missing explicit k8s default in `BuildHonchoDeployment` or the gateway-merged ConfigMap re-marshalling YAML in a non-deterministic key order. Investigate by `kubectl diff` the desired-vs-live StatefulSet (envtest's apiserver lets you do this via the `k8sClient`).

- [ ] **Step 3: Commit**

```bash
git add internal/controller/hermesinstance_controller_test.go
git commit -m "test(controller): idempotency canary for full Plan 3 surface (runtime+gateways+honcho)"
```

---

## Task 20: E2E: apply a HermesInstance with telegram + Honcho on kind

**Files:**
- Create: `test/e2e/gateways_honcho_test.go`, `test/e2e/testdata/hermesinstance-gateways.yaml`

- [ ] **Step 1: Create the sample manifest**

Create `test/e2e/testdata/hermesinstance-gateways.yaml`:

```yaml
# test/e2e/testdata/hermesinstance-gateways.yaml
apiVersion: v1
kind: Secret
metadata:
  name: tg-secret
  namespace: default
type: Opaque
stringData:
  token: "dummy-telegram-token-for-e2e"
---
apiVersion: v1
kind: Secret
metadata:
  name: honcho-secret
  namespace: default
type: Opaque
stringData:
  api-key: "dummy-honcho-api-key-for-e2e"
---
apiVersion: hermes.agent/v1
kind: HermesInstance
metadata:
  name: e2e-gateways
  namespace: default
spec:
  image:
    repository: ghcr.io/stubbi/hermes-agent
    tag: "1.4.2"
    pullPolicy: IfNotPresent
  storage:
    persistence:
      enabled: true
      size: 1Gi
  config:
    raw: |
      logLevel: info
      schedules: {}
  runtime:
    python: "3.11"
    uv:
      enabled: true
  gateways:
    telegram:
      enabled: true
      botTokenSecretRef:
        name: tg-secret
        key: token
      allowedUserIDs: [42]
  profileStore:
    honcho:
      enabled: true
      persistence:
        enabled: true
        size: 1Gi
      apiKeySecretRef:
        name: honcho-secret
        key: api-key
```

- [ ] **Step 2: Create the e2e test file**

Create `test/e2e/gateways_honcho_test.go`:

```go
package e2e

import (
	"context"
	"os/exec"
	"testing"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	"k8s.io/apimachinery/pkg/types"
)

func TestGatewaysHoncho(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Gateways + Honcho E2E Suite")
}

var _ = Describe("HermesInstance with Telegram + Honcho on kind", Ordered, func() {
	BeforeAll(func() {
		// Apply the manifest. Assume the kind cluster + operator are
		// already installed by e2e_suite_test.go (Plan 1).
		cmd := exec.Command("kubectl", "apply", "-f", "testdata/hermesinstance-gateways.yaml")
		out, err := cmd.CombinedOutput()
		Expect(err).NotTo(HaveOccurred(), string(out))
	})

	AfterAll(func() {
		_ = exec.Command("kubectl", "delete", "-f", "testdata/hermesinstance-gateways.yaml", "--ignore-not-found=true").Run()
	})

	It("brings the hermes pod to Ready with the gateway env vars present", func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
		defer cancel()

		Eventually(func(g Gomega) {
			var sts appsv1.StatefulSet
			g.Expect(k8sClient.Get(ctx, types.NamespacedName{Name: "e2e-gateways", Namespace: "default"}, &sts)).To(Succeed())
			g.Expect(sts.Status.ReadyReplicas).To(BeNumerically(">=", int32(1)))
		}, "5m", "5s").Should(Succeed())

		// Pod env should include TELEGRAM_BOT_TOKEN and HONCHO_BASE_URL.
		var sts appsv1.StatefulSet
		Expect(k8sClient.Get(ctx, types.NamespacedName{Name: "e2e-gateways", Namespace: "default"}, &sts)).To(Succeed())
		env := sts.Spec.Template.Spec.Containers[0].Env
		names := map[string]bool{}
		for _, e := range env {
			names[e.Name] = true
		}
		Expect(names).To(HaveKey("TELEGRAM_BOT_TOKEN"))
		Expect(names).To(HaveKey("HONCHO_BASE_URL"))
	})

	It("brings the Honcho Deployment to Ready", func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
		defer cancel()
		Eventually(func(g Gomega) {
			var dep appsv1.Deployment
			g.Expect(k8sClient.Get(ctx, types.NamespacedName{Name: "e2e-gateways-honcho", Namespace: "default"}, &dep)).To(Succeed())
			g.Expect(dep.Status.ReadyReplicas).To(BeNumerically(">=", int32(1)))
		}, "5m", "5s").Should(Succeed())
	})

	It("produces a NetworkPolicy with a 443/TCP egress rule for the Telegram endpoint", func() {
		var np networkingv1.NetworkPolicy
		Expect(k8sClient.Get(context.Background(), types.NamespacedName{Name: "e2e-gateways", Namespace: "default"}, &np)).To(Succeed())
		has443 := false
		for _, r := range np.Spec.Egress {
			for _, p := range r.Ports {
				if p.Port != nil && p.Port.IntVal == 443 {
					has443 = true
				}
			}
		}
		Expect(has443).To(BeTrue())
	})

	It("produces a Honcho-scoped NetworkPolicy with ingress only from the hermes pod", func() {
		var np networkingv1.NetworkPolicy
		Expect(k8sClient.Get(context.Background(), types.NamespacedName{Name: "e2e-gateways-honcho", Namespace: "default"}, &np)).To(Succeed())
		Expect(np.Spec.Ingress).To(HaveLen(1))
		Expect(np.Spec.Ingress[0].From[0].PodSelector.MatchLabels).To(HaveKeyWithValue("app.kubernetes.io/name", "hermes-agent"))
		Expect(np.Spec.Ingress[0].From[0].PodSelector.MatchLabels).To(HaveKeyWithValue("app.kubernetes.io/instance", "e2e-gateways"))
	})
})

var _ = corev1.Secret{} // keep import
```

- [ ] **Step 3: Run the e2e locally**

```bash
make e2e
```
Expected: PASS. If the kind cluster's CNI does not support hostname-based egress, only the port-level assertion is reliable: that's why the test asserts on port number, not destination.

- [ ] **Step 4: Commit**

```bash
git add test/e2e/gateways_honcho_test.go test/e2e/testdata/hermesinstance-gateways.yaml
git commit -m "test(e2e): apply HermesInstance with telegram+Honcho on kind, assert readiness + NetworkPolicies"
```

---

## Task 21: Document `runtime`, `gateways`, `profileStore` in `docs/api-reference.md`

**Files:**
- Modify: `docs/api-reference.md`

- [ ] **Step 1: Append the three new sub-spec sections**

Append the following to `docs/api-reference.md` (created by Plan 2). Place them under the existing top-level `HermesInstance` section, in the order they appear in `spec` (after `storage`, before `selfConfigure`):

```markdown
### `spec.runtime`

Controls the Python/uv runtime concerns of the agent container.

| Field | Type | Default | Description |
|---|---|---|---|
| `python` | string | `"3.11"` | Informational. The agent image's Python version is fixed at build time. |
| `uv.enabled` | *bool | `true` | When true, an init container runs `uv sync --frozen` against the lockfile bundled in the agent image. |
| `uv.extraIndexURL` | string | `""` | Appended to uv's index list. Useful for private PyPI mirrors. |
| `uv.cacheVolume.emptyDir` | EmptyDirVolumeSource | `{sizeLimit: 1Gi}` | Volume backing `/home/hermes/.cache/uv`. |
| `uv.cacheVolume.persistentVolumeClaim` | PersistentVolumeClaimVolumeSource | nil | If set, overrides the emptyDir. |
| `ffmpeg.enabled` | *bool | `true` | Toggles the FFmpeg dependency check (image always ships FFmpeg). |
| `ripgrep.enabled` | *bool | `true` | Toggles the ripgrep dependency check. |
| `extraAptPackages` | []string | `[]` | Adds APT packages via a root-privileged init container. **Security implication**: the init container runs as UID 0 for the duration of the apt install only. |
| `extraPipPackages` | []string | `[]` | Adds Python packages via `uv pip install` into a persistent venv on the data PVC (`/home/hermes/.hermes/.venv-extras`). |

### `spec.gateways`

Multi-platform messaging gateway bindings. Every platform is opt-in; tokens are referenced via Secret selectors so they can be rotated independently.

#### `spec.gateways.telegram`

| Field | Type | Default | Description |
|---|---|---|---|
| `enabled` | *bool | `false` | Toggle. Enabling requires `botTokenSecretRef`. |
| `botTokenSecretRef` | SecretKeySelector | nil | Bot API token. Surfaced as `TELEGRAM_BOT_TOKEN`. |
| `allowedUserIDs` | []int64 | `[]` | Allow-list of Telegram user IDs. Surfaced as `TELEGRAM_ALLOWED_USER_IDS` (comma-separated). |
| `webhookURL` | string | `""` | Public HTTPS URL to register with Telegram. Empty = long-poll. |

#### `spec.gateways.discord`

| Field | Type | Default | Description |
|---|---|---|---|
| `enabled` | *bool | `false` | Toggle. Enabling requires `botTokenSecretRef`. |
| `botTokenSecretRef` | SecretKeySelector | nil | Bot token. Surfaced as `DISCORD_BOT_TOKEN`. |
| `applicationID` | string | `""` | Application snowflake. Surfaced as `DISCORD_APPLICATION_ID`. |
| `guildIDs` | []string | `[]` | Scopes slash-command registration. Surfaced as `DISCORD_GUILD_IDS` (comma-separated). |

#### `spec.gateways.slack`

| Field | Type | Default | Description |
|---|---|---|---|
| `enabled` | *bool | `false` | Toggle. Enabling requires `botTokenSecretRef`. |
| `botTokenSecretRef` | SecretKeySelector | nil | `xoxb-` bot token. Surfaced as `SLACK_BOT_TOKEN`. |
| `appTokenSecretRef` | SecretKeySelector | nil | `xapp-` app-level token for Socket Mode. Surfaced as `SLACK_APP_TOKEN`. |
| `signingSecretRef` | SecretKeySelector | nil | Slack signing secret. Surfaced as `SLACK_SIGNING_SECRET`. |

#### `spec.gateways.whatsapp`

| Field | Type | Default | Description |
|---|---|---|---|
| `enabled` | *bool | `false` | Toggle. Enabling requires `providerSecretRef`. |
| `providerSecretRef` | SecretKeySelector | nil | Provider credentials. The whole Secret is mounted as env with prefix `WHATSAPP_`. |

#### `spec.gateways.signal`

| Field | Type | Default | Description |
|---|---|---|---|
| `enabled` | *bool | `false` | Toggle. Enabling requires both `phoneNumberSecretRef` and `authTokenSecretRef`. |
| `phoneNumberSecretRef` | SecretKeySelector | nil | Registered phone number. Surfaced as `SIGNAL_PHONE_NUMBER`. |
| `authTokenSecretRef` | SecretKeySelector | nil | Auth token for signal-cli-rest-api. Surfaced as `SIGNAL_AUTH_TOKEN`. |

### `spec.profileStore`

Optional companion service for the Honcho dialectic profile store.

#### `spec.profileStore.honcho`

| Field | Type | Default | Description |
|---|---|---|---|
| `enabled` | *bool | `false` | Toggle. Enabling requires `apiKeySecretRef`. |
| `image.repository` | string | `"ghcr.io/plastic-labs/honcho"` | Honcho image. |
| `image.tag` | string | `"0.1.0"` | Honcho image tag. |
| `image.pullPolicy` | string | `IfNotPresent` | One of `Always`, `IfNotPresent`, `Never`. |
| `persistence.enabled` | *bool | `true` | Whether to create a PVC for Honcho. |
| `persistence.size` | string | `"5Gi"` | PVC size. |
| `persistence.storageClassName` | *string | nil | PVC storage class. |
| `resources` | ResourceRequirements | `{}` | Honcho container resource requests/limits. |
| `apiKeySecretRef` | SecretKeySelector | nil | API key the agent uses to authenticate. Surfaced as `HONCHO_API_KEY` on both the agent and the Honcho container. |

The agent receives `HONCHO_BASE_URL=http://<inst>-honcho:8000` automatically.

The Honcho-side PVC layout that Plan 4's `addProfileSnapshot` Job writes to is
`/data/snapshots/<profileID>/<RFC3339-timestamp>.json` (relative to the Honcho
container, which mounts the PVC at `/data`).
```

- [ ] **Step 2: Commit**

```bash
git add docs/api-reference.md
git commit -m "docs(api-reference): document spec.runtime, spec.gateways, spec.profileStore"
```

---

## Task 22: `docs/conventions.md`: well-known egress endpoints

**Files:**
- Modify: `docs/conventions.md`

- [ ] **Step 1: Append the egress endpoints section**

Append to `docs/conventions.md`:

```markdown
## Well-known egress endpoints

The operator's default-deny `NetworkPolicy` allows only DNS to kube-dns out of
the box. Each `spec.gateways.<platform>.enabled: true` adds an egress allow for
the upstream's well-known endpoints. CNI plugins that support FQDN peers
(Cilium, Calico with `dns` selector) should match the hostnames below;
plugins without FQDN support fall back to a port-only rule (443/TCP to any
destination), which is wider than ideal: document the trade-off when
shipping the cluster.

| Gateway | Hostnames | Port | Protocol | Notes |
|---|---|---|---|---|
| Telegram | `api.telegram.org` | 443 | TCP | Long-poll OR webhook. Webhook also needs ingress from Telegram's IP ranges to the agent's webhook URL: out of scope for the egress NetworkPolicy. |
| Discord | `discord.com`, `gateway.discord.gg` | 443 | TCP | gateway.discord.gg is the WebSocket endpoint. |
| Slack | `slack.com`, `wss-primary.slack.com` | 443 | TCP | wss-primary.slack.com is the Socket Mode endpoint. |
| WhatsApp (Meta Cloud) | `graph.facebook.com` | 443 | TCP | Provider-specific. Twilio users should replace with `api.twilio.com`. |
| Signal (chat.signal.org) | `chat.signal.org` | 443 | TCP | Self-hosted signal-cli-rest-api deployments should supplement via `spec.networking.egress`. |
| Honcho (sibling) | sibling pod selector | 8000 | TCP | In-namespace pod-selector peer, not internet. |

The operator does NOT cover ingress from those providers (Telegram, Slack
webhook callbacks, etc.): surface that via `spec.networking.ingress` or a
dedicated Ingress object in your cluster.
```

- [ ] **Step 2: Commit**

```bash
git add docs/conventions.md
git commit -m "docs(conventions): document well-known gateway egress endpoints"
```

---

## Task 23: `docs/runbook-platform-gateways.md`: per-platform token/scope/rotation

**Files:**
- Create: `docs/runbook-platform-gateways.md`

- [ ] **Step 1: Create the runbook**

Create `docs/runbook-platform-gateways.md`:

```markdown
# Runbook: Platform Gateway Tokens

This runbook walks an operator through acquiring, scoping, rotating, and
revoking the credentials each `spec.gateways.<platform>` requires.

## Telegram

### Acquire

1. DM `@BotFather` on Telegram.
2. `/newbot` → choose a display name and a username ending in `bot`.
3. BotFather returns a token of the form `<botID>:<base64ish>`.
4. Restrict who can DM the bot via `/setjoingroups`, `/setprivacy`, and the
   in-bot `spec.gateways.telegram.allowedUserIDs` allow-list.

### Scope

A Telegram bot token has full bot privileges. There is no per-permission
scoping; restrict instead by:
- Putting the token in a Secret with restrictive RBAC.
- Setting `allowedUserIDs` to your operator team's user IDs only.

### Rotate

1. DM `@BotFather` → `/revoke` → choose your bot → confirm.
2. BotFather issues a new token.
3. Update the Kubernetes Secret (`kubectl create secret generic tg-secret --from-literal=token=... --dry-run=client -o yaml | kubectl apply -f -`).
4. Restart the agent pod: `kubectl rollout restart statefulset/<inst>`:
   the operator does NOT auto-restart pods on Secret updates by design (lesson:
   surprise restarts on rotation cause incident noise).

### Pitfalls

- **Bot is unresponsive in groups.** Telegram bots in groups receive messages
  only when mentioned by default. Set `/setprivacy Disabled` in BotFather
  to allow the bot to read all group messages.
- **Webhook mode requires HTTPS.** Self-signed certs are rejected by
  Telegram; use cert-manager.

## Discord

### Acquire

1. Visit https://discord.com/developers/applications → "New Application".
2. Side menu → "Bot" → "Reset Token" → copy the bot token.
3. Side menu → "OAuth2 → URL Generator" → scopes `bot`, `applications.commands`
   → permissions as needed → use the URL to invite to your guild.
4. Note the `Application ID` from "General Information" → set
   `spec.gateways.discord.applicationID`.

### Scope

Discord uses **OAuth2 scopes + bot permissions**. Minimum-viable:
- `bot` + `applications.commands` scopes.
- Permission flag `Send Messages` + `Read Message History`.

Restrict slash-command rollout to specific guilds via
`spec.gateways.discord.guildIDs` to avoid leaking commands cluster-wide
during testing.

### Rotate

1. https://discord.com/developers/applications → your app → "Bot" → "Reset Token".
2. Update the Kubernetes Secret.
3. `kubectl rollout restart statefulset/<inst>`.

### Pitfalls

- **Sharding.** Bots in >2500 guilds must shard. Hermes-agent's v1 build does
  not auto-shard; raise this limit by deploying multiple HermesInstances with
  different `guildIDs`.

## Slack

### Acquire

1. https://api.slack.com/apps → "Create New App" → "From scratch".
2. Add features → "Bot Token Scopes": at minimum `chat:write`, `app_mentions:read`,
   `channels:history`.
3. Install to workspace → copy the `xoxb-` bot token.
4. (Socket Mode) "Settings → Socket Mode" → enable → "Generate" an app-level
   token (`xapp-`) with `connections:write` scope.
5. "Settings → Basic Information" → "Signing Secret" (for HTTP mode).

### Scope

Slack scopes are the strictest model of the four. Add new scopes incrementally;
removing a scope from a deployed app does NOT auto-revoke active tokens.

### Rotate

1. https://api.slack.com/apps → your app → "OAuth & Permissions" → "Reinstall to Workspace".
2. Slack issues a new `xoxb-` token.
3. (Optional) Rotate `xapp-` under "Settings → Basic Information → App-Level Tokens".
4. Update the Kubernetes Secret; restart the pod.

### Pitfalls

- **Socket Mode vs HTTP.** Hermes-agent supports both. Socket Mode requires
  egress to `wss-primary.slack.com:443`; HTTP mode requires ingress into your
  cluster. Pick one and don't enable both: they fight over `app_home_opened`
  events.

## WhatsApp

### Acquire (Meta Cloud API)

1. https://developers.facebook.com/apps → "Create App" → "Business" type.
2. Add WhatsApp product → register a phone number → copy `Phone Number ID` and
   `WhatsApp Business Account ID`.
3. Generate a system-user access token (long-lived).
4. Stuff `PHONE_NUMBER_ID`, `BUSINESS_ACCOUNT_ID`, and `ACCESS_TOKEN` into one
   Kubernetes Secret; reference it via `providerSecretRef`. The agent reads them
   all as `WHATSAPP_*` env vars.

### Scope

Meta's permission model is `whatsapp_business_messaging` + `whatsapp_business_management`.
Both are required for hermes-agent's WhatsApp gateway.

### Rotate

System-user tokens are long-lived (60 days). Set a calendar reminder.
Rotation: generate a new token in the Meta app dashboard, update the Secret,
restart the pod.

### Pitfalls

- **Webhook callback verification.** Meta verifies the webhook with a token
  you set in the app dashboard. Mismatch → silent verification failure with no
  log line in the agent. Double-check that `VERIFY_TOKEN` in the Secret equals
  the dashboard value.

## Signal (signal-cli-rest-api)

### Acquire

1. Stand up `signal-cli-rest-api` (https://github.com/bbernhard/signal-cli-rest-api)
   in your cluster or as an external service.
2. Register the phone number with signal-cli (see upstream docs).
3. Provision an auth token if your deployment requires it (optional in most
   setups).

### Scope

Signal does not have per-token scopes; the registered phone number is the
identity boundary.

### Rotate

Unregistering and re-registering a Signal number is **disruptive**: previous
message history becomes inaccessible to the new session. Treat the
phone-number secret as long-lived and only rotate the auth token in front of
signal-cli-rest-api.

### Pitfalls

- **Self-hosted endpoint discovery.** `spec.gateways.signal` does NOT specify
  the signal-cli-rest-api URL: that's a config-level concern. Set it via
  `spec.config.raw.signal.apiURL` or via the `SIGNAL_API_URL` env var
  (`spec.env`).

---

## General notes (all platforms)

- **Secret RBAC.** The operator's ClusterRole only grants `get` on Secrets
  *in the namespaces it watches*. Keep gateway Secrets in the same namespace
  as the `HermesInstance`.
- **No sealed-secrets / external-secrets opinion.** Either approach works;
  the operator only consumes a corev1.Secret by the time it reaches reconcile.
- **Admission warnings.** If the validating webhook emits
  `gateways.X.Y.SecretRef references Secret "..." which is not present yet`,
  the instance is still admitted: apply the Secret and the next reconcile
  will pick it up.
```

- [ ] **Step 2: Commit**

```bash
git add docs/runbook-platform-gateways.md
git commit -m "docs: add per-platform gateway runbook (Telegram, Discord, Slack, WhatsApp, Signal)"
```

---

## Task 24: Update Helm chart defaults

**Files:**
- Modify: `charts/hermes-operator/values.yaml`, `README.md`

- [ ] **Step 1: Add agent-image-version defaults to `values.yaml`**

Append (or modify the relevant section in) `charts/hermes-operator/values.yaml`:

```yaml
# -------- Plan 3: hermes-agent defaults --------

# Default values copied into HermesInstance.spec.image when not set by the user.
# Mirrors HermesClusterDefaults but distributed via the chart for convenience.
agentImage:
  repository: ghcr.io/stubbi/hermes-agent
  tag: "1.4.2"        # update when shipping a new agent matrix entry
  pullPolicy: IfNotPresent

# Default values copied into HermesInstance.spec.profileStore.honcho.image.
honchoImage:
  repository: ghcr.io/plastic-labs/honcho
  tag: "0.1.0"

# Egress endpoints the operator-rendered NetworkPolicy will allow per gateway.
# Operators on CNI plugins without FQDN-peer support may override this list
# with explicit CIDRs.
gatewayEgressEndpoints:
  telegram: ["api.telegram.org"]
  discord: ["discord.com", "gateway.discord.gg"]
  slack: ["slack.com", "wss-primary.slack.com"]
  whatsapp: ["graph.facebook.com"]
  signal: ["chat.signal.org"]
```

> **Note:** the chart values are *informational* in v1: the operator itself reads them only through `HermesClusterDefaults`. Plan 6 (distribution) wires them into a default `HermesClusterDefaults` template.

- [ ] **Step 2: Update the README feature table**

In `README.md`, find the feature table (created by Plan 1) and add three rows (or update the existing rows if Plan 2 already added placeholder lines):

```markdown
| Python runtime + uv lockfile | `spec.runtime` controls init containers for `uv sync`, extra apt/pip packages. The agent image bundles a committed lockfile for reproducibility. |
| Multi-platform gateways | `spec.gateways.{telegram,discord,slack,whatsapp,signal}`: per-platform Secret refs, NetworkPolicy egress, and config.yaml fragments. |
| Honcho profile store | `spec.profileStore.honcho.enabled` stands up a sibling Deployment + Service + PVC + scoped NetworkPolicy. `HONCHO_BASE_URL`/`HONCHO_API_KEY` are auto-injected. |
```

- [ ] **Step 3: Commit**

```bash
git add charts/hermes-operator/values.yaml README.md
git commit -m "feat(chart,docs): default agent + honcho image tags; document runtime/gateways/profileStore in README"
```

---

## Task 25: Integration verification: full Plan 3 surface end-to-end

**Files:**
- None (read-only verification + a final integration commit gate)

This task is the **gate** that must pass before Plan 3 is considered done. It re-runs every test layer in order and confirms the final repository state is internally consistent.

- [ ] **Step 1: All unit tests pass**

```bash
go test ./internal/resources/... -count=2 -race
```
Expected: PASS. `-count=2` flushes the build cache; `-race` catches data races introduced by any helper that touches package-level state.

- [ ] **Step 2: All envtest cases pass**

```bash
make test
```
Expected: PASS. Includes the Plan 1 + Plan 2 + Plan 3 envtest cases.

- [ ] **Step 3: Reconcile Guard CI grep-banned-patterns pass locally**

```bash
make reconcile-guard
```
Expected: no banned-pattern hits. If `r.Update(` on a managed resource appears in any of the new code, fix to use `controllerutil.CreateOrUpdate` (Plan 1 §7.2 rule 1).

- [ ] **Step 4: Helm chart still installs**

```bash
helm lint charts/hermes-operator/
helm template charts/hermes-operator/ --debug > /tmp/render.yaml
kubectl --dry-run=client apply -f /tmp/render.yaml
```
Expected: all green. No `unknown field` warnings.

- [ ] **Step 5: CRD diff matches generated**

```bash
make manifests
git diff --exit-code config/crd/bases/
```
Expected: empty diff. If any file has un-committed regeneration churn, commit it as `chore: regenerate CRDs after Plan 3`.

- [ ] **Step 6: Agent image builds + smoke-tests locally**

```bash
make agent-image-build HERMES_VERSION=1.4.2
make agent-image-smoke HERMES_VERSION=1.4.2
```
Expected: `agent-image-smoke OK`.

- [ ] **Step 7: E2E on kind**

```bash
make e2e
```
Expected: PASS (the new `gateways_honcho_test.go` suite plus pre-existing happy-path).

- [ ] **Step 8: `kubectl explain` works for the new fields**

After installing the operator into a kind cluster (`make deploy-local`), run:

```bash
kubectl explain hermesinstance.spec.runtime
kubectl explain hermesinstance.spec.gateways.telegram
kubectl explain hermesinstance.spec.profileStore.honcho
```
Expected: each command prints field descriptions sourced from the Go godoc on the struct fields (kubebuilder generates these into the CRD schema). If a description is missing, add the corresponding godoc to the field in `api/v1/hermesinstance_types.go` and regenerate.

- [ ] **Step 9: No stray TODOs or `FIXME`s introduced**

```bash
grep -rE "^\s*(// TODO|// FIXME)" internal/ api/ images/hermes-agent/ docs/ | grep -v zz_generated || true
```
Expected: empty (or only pre-existing entries from Plans 1/2 that this plan did not touch).

- [ ] **Step 10: Final commit**

If steps 1-9 surface any leftover untracked/unstaged change:

```bash
git status
git add -A
git commit -m "chore: final Plan 3 cleanup (regenerated manifests, etc.)"
```

If everything is clean, no commit is needed: Plan 3 is done.

---

## Self-review

This section is for the implementer to verify the plan before declaring it
complete and for the reviewer to spot-check the deliverables against the
spec.

### Cross-plan naming consistency

- [ ] `HonchoPVCName(inst) == inst.Name + "-honcho-data"`: matches Plan 4
  Task 11 which mounts this PVC in the `addProfileSnapshot` Job at
  `/data/snapshots/<profileID>/<timestamp>.json`.
- [ ] `HonchoServiceName(inst) == inst.Name + "-honcho"` and the agent receives
  `HONCHO_BASE_URL=http://<inst>-honcho:8000`: matches Plan 4 webhook test
  that gates `addProfileSnapshot` on `spec.profileStore.honcho.enabled`.
- [ ] `+listType=map +listMapKey=source` is on `.spec.skills` (Plan 4
  expectation) and `+listType=map +listMapKey=name` on `.spec.env`,
  `.spec.envFrom`: these markers come from Plan 2, but Plan 3 Task 7 Step 5
  verifies them and refuses to continue if missing.
- [ ] `spec.selfConfigure.allowedActions` accepts `profiles` (Task 16): Plan
  4 SSA reconciler validates the action against this list.
- [ ] The `hermes-agent migrate from-openclaw --source ... --dest ...` CLI
  shape (Plan 5 §3.B) is supported by the agent image: Task 2's
  entrypoint.sh delegates non-`serve` subcommands verbatim to the
  `hermes-agent` binary, so `docker run <img> migrate from-openclaw ...`
  works for the Plan 5 init container.

### Spec coverage

- [ ] §4.1 row "runtime": implemented in Tasks 5, 8, 12 (RuntimeSpec types,
  runtime_init.go builder, StatefulSet wiring).
- [ ] §4.1 row "gateways": implemented in Tasks 6, 9, 11, 12, 13, 16
  (GatewaysSpec types, gateways.go builder, ConfigMap merge, StatefulSet
  wiring, NetworkPolicy egress, webhook warnings).
- [ ] §4.1 row "profileStore": implemented in Tasks 7, 10, 12, 13, 14, 15
  (ProfileStoreSpec types, honcho.go builder, env wiring, NetworkPolicy
  scope, reconciler orchestration, ProfileStoreReady condition).
- [ ] §4.1 row "selfConfigure.allowedActions adds profiles": Task 16
  webhook update.
- [ ] §7.4 "tini as PID 1": Task 2 Dockerfile entrypoint.
- [ ] §7.4 "init containers mount the full data volume" (lesson openclaw
  #450): Task 8 unit test explicitly asserts `SubPath == ""` and the
  builder uses `dataVolumeMount()` for every init container.
- [ ] §7.4 "read-only root FS with explicit writable subPaths": Task 8 init
  containers all set `ReadOnlyRootFilesystem: true` except the apt init
  (documented exception).
- [ ] Operator-published `ghcr.io/stubbi/hermes-agent`: Tasks 1-4 (build
  context, Dockerfile, lockfile, CI matrix).

### Lessons baked in (from spec §1 G3 / openclaw issue log)

- [ ] **#446/#447: third-party label/annotation preservation:** the
  `MergePreservingForeign` helper from Plan 1 is used in the new builders
  via the existing CreateOrUpdate wrappers. Honcho's labels include both
  `app.kubernetes.io/*` and `hermes.agent/instance`: none of these collide
  with foreign keys, but the merge function still preserves anything
  outside the `hermes.agent/` prefix.
- [ ] **#450: init containers mount full volume:** asserted by unit test in
  Task 8.
- [ ] **#458: read-only root FS:** every container (agent, init-uv,
  init-pip, Honcho) has `ReadOnlyRootFilesystem: true`. init-apt is the
  documented exception.
- [ ] **#471: zombie-process reaper:** tini as PID 1 in the agent image
  (Task 2).
- [ ] **#479/#480: ClusterRole aggregation labels:** unchanged by Plan 3
  (Plan 6 handles distribution).
- [ ] **Generation thrash:** Task 19 idempotency canary against the full
  Plan 3 surface.

### Reconcile Guard CI

- [ ] No `r.Update(` on managed resources in any new code. The Honcho
  reconciler uses `controllerutil.CreateOrUpdate` via the Plan 2
  `ensureDeployment`/`ensureService`/`ensurePVC` wrappers. The webhook
  changes only call `r.Client.Get` (read-only).
- [ ] No `r.Update()` on the `HermesInstance` itself: only
  `r.Status().Update` for the new condition.
- [ ] Finalizer add/remove still uses `r.Patch()` (Plan 1 rule §7.2.5):
  Plan 3 adds no finalizers.

### Webhook behaviour

- [ ] Missing gateway secret → warning, not denial (Plan 3 Task 16 test
  `TestValidateGateways_TelegramSecretMissingProducesWarning`).
- [ ] Missing key on existing secret → warning.
- [ ] Gateway enabled without ANY required secret ref → denial
  (`TestValidateGateways_TelegramEnabledWithoutSecretRefDenied`).
- [ ] `profiles` accepted in `selfConfigure.allowedActions`.
- [ ] Unknown action rejected.

### Test pyramid

- [ ] **Unit:** `internal/resources/*_test.go` covers all six new/modified
  builders. Builds + tests in <2s (Plan 1 perf budget).
- [ ] **envtest:** three new `Describe` blocks (gateways, honcho lifecycle,
  Plan 3 idempotency canary) in
  `internal/controller/hermesinstance_controller_test.go`.
- [ ] **Webhook:** four new test cases in
  `internal/webhook/hermesinstance_validator_test.go`.
- [ ] **E2E:** one new `test/e2e/gateways_honcho_test.go` exercising the
  full stack on kind.
- [ ] **Reconcile Guard:** still green.
- [ ] **Agent-image smoke:** per-PR via `agent-image-smoke.yaml` workflow.

### Distribution + docs

- [ ] `docs/api-reference.md`: Task 21 appended runtime + gateways +
  profileStore sections.
- [ ] `docs/conventions.md`: Task 22 appended well-known egress endpoints.
- [ ] `docs/conditions.md`: Task 15 appended `ProfileStoreReady` entry.
- [ ] `docs/runbook-platform-gateways.md`: Task 23 created.
- [ ] `README.md` feature table: Task 24 updated.
- [ ] `charts/hermes-operator/values.yaml`: Task 24 added agent + Honcho
  defaults.
- [ ] `.github/workflows/agent-image.yaml` + `agent-image-smoke.yaml`:
  Task 4.
- [ ] `images/hermes-agent/uv.lock` committed and reproducible (Task 3
  Step 3 diff check).

### Things deferred to other plans (do NOT implement here)

- The `addProfileSnapshot` one-shot Job that writes to the Honcho PVC at
  `/data/snapshots/<profileID>/<timestamp>.json`: Plan 4 Task 11.
- `HermesClusterDefaults` defaulting of `runtime`, `gateways`,
  `profileStore`: Plan 4 (`hermesclusterdefaults_controller.go`) or
  Plan 6 (default `HermesClusterDefaults` chart template).
- The `migration.fromOpenClaw` init container that invokes `hermes-agent
  migrate from-openclaw`: Plan 5. Plan 3 only ensures the agent image's
  entrypoint supports the subcommand.
- Conformance suite (`test/conformance/`) covering the negative gateway
  cases: Plan 6.
- Backup/restore that captures the Honcho PVC alongside the data PVC:
  Plan 5.

### Forward references for downstream plans

- **Plan 4** must read this plan's `internal/resources/honcho.go` for the
  PVC/Service/Deployment names. The `BuildSnapshotJob` builder added by
  Plan 4 Task 11 mounts `HonchoPVCName(inst)` at `/data`.
- **Plan 5** must read this plan's `images/hermes-agent/Dockerfile` to
  confirm the entrypoint passes through `migrate from-openclaw`. The
  migration init container in Plan 5 invokes
  `hermes-agent migrate from-openclaw --source /mnt/openclaw --dest /home/hermes/.hermes`.
- **Plan 6** must read this plan's `.github/workflows/agent-image.yaml` to
  wire the operator release tag → agent matrix relationship in the
  release-please bump flow (operator vX.Y.Z does NOT auto-build agents;
  agents ship on `agent/vX.Y.Z` tags).

### Final acceptance criteria

- All checkboxes above ticked.
- `make test e2e reconcile-guard` green on a fresh checkout.
- `git status` clean.
- The Plan 1 happy-path manifest still applies and brings a pod to Ready
  (Plan 3 must not regress Plan 1 behaviour for users who don't set
  `runtime`, `gateways`, or `profileStore`).

Plan 3 is complete when an engineer can apply
`test/e2e/testdata/hermesinstance-gateways.yaml` to a kind cluster and the
resulting hermes pod reaches Ready with `TELEGRAM_BOT_TOKEN` and
`HONCHO_BASE_URL` populated, alongside a healthy `<inst>-honcho`
Deployment.









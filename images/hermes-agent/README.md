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

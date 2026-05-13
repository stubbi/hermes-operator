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

# git + ca-certificates are needed at lock-resolve time because the lockfile
# pins hermes-agent by git ref (upstream is not on PyPI).
RUN apt-get update \
    && apt-get install -y --no-install-recommends git ca-certificates \
    && rm -rf /var/lib/apt/lists/*

# Copy lockfile + project metadata first for layer caching.
COPY pyproject.toml uv.lock ./

# Sync the frozen lockfile. `make agent-image-relock HERMES_VERSION=vX.Y.Z`
# rewrites the @ ref in pyproject.toml and regenerates uv.lock; at build time
# we simply trust the lock. HERMES_VERSION is informational for the labels stage.
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

# images/hermes-agent/Dockerfile
#
# ghcr.io/stubbi/hermes-agent is the operator's published agent image. It
# IS the upstream NousResearch/hermes-agent image — an s6-overlay runtime that
# bundles the gateway, the dashboard, the OpenAI-compatible API server, a
# Playwright/Chromium browser, node, and every Python dependency — with operator
# metadata layered on top.
#
# Why FROM-upstream instead of building the venv ourselves: the agent is designed
# to run under s6 supervision (/init as PID 1, per-profile gateways, profile
# reconcile on boot, browser at /opt/hermes/.playwright). The previous hand-rolled
# `uv sync` image reproduced none of that, so no HermesInstance could reach Ready
# (see #89). The operator now orchestrates the upstream image declaratively: it
# runs `hermes gateway run` with the API server enabled and probes /health on the
# gateway port (internal/resources/statefulset.go).
#
# Pinned by digest for reproducibility + supply chain. To bump, resolve a new
# digest for the desired upstream release and update both the digest and
# HERMES_VERSION:
#   docker buildx imagetools inspect nousresearch/hermes-agent:<tag>
#
# Build arg:
#   HERMES_VERSION : the upstream release this digest corresponds to. Surfaced as
#                    the hermes.agent/version label the autoupdate controller reads.
ARG HERMES_VERSION=v0.16.0

# Multi-arch (linux/amd64 + linux/arm64) manifest-list digest of
# nousresearch/hermes-agent:latest (v0.16.0). buildx selects the per-platform
# image from the list at build time.
FROM nousresearch/hermes-agent@sha256:be6912a818cb4a479fa384024a599fee6796d56d1e6a5ca56834deec328d9981

ARG HERMES_VERSION

# Operator metadata. hermes.agent/version is what the autoupdate controller
# (Plan 5) compares against the registry tag. The image's upstream ENTRYPOINT
# (/init → main-wrapper) and env (HERMES_HOME=/opt/data, ...) are inherited.
LABEL org.opencontainers.image.title="hermes-agent" \
      org.opencontainers.image.source="https://github.com/stubbi/hermes-operator" \
      org.opencontainers.image.documentation="https://github.com/stubbi/hermes-operator/blob/main/images/hermes-agent/README.md" \
      org.opencontainers.image.description="Operator-published NousResearch/hermes-agent (s6 runtime: gateway + API server + dashboard + browser) for hermes-operator" \
      org.opencontainers.image.licenses="MIT" \
      org.opencontainers.image.vendor="stubbi" \
      org.opencontainers.image.base.name="docker.io/nousresearch/hermes-agent" \
      hermes.agent/version="${HERMES_VERSION}"

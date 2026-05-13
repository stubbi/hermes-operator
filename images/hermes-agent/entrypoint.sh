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

# Otherwise pass through verbatim — supports `migrate from-openclaw ...`, `version`, etc.
exec hermes-agent "$@"

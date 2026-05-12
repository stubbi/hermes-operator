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

- `vX.Y.Z` — exact version
- `X.Y` — minor channel (moves on patch releases)
- `latest` — most recent stable (moves on every release)

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

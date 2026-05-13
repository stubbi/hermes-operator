# Security Policy

## Reporting

Use [GitHub's private vulnerability reporting](https://github.com/stubbi/hermes-operator/security/advisories/new) — this routes a private advisory to the maintainers and is the preferred path. As a fallback, email **jannes@aqora.io** with subject "SECURITY: hermes-operator".

We aim to acknowledge within 72 hours and provide a remediation timeline within 7 days.

Operator images are signed with Cosign (keyless OIDC); SBOMs are attested and attached to releases. Verify with:

```bash
cosign verify ghcr.io/stubbi/hermes-operator:vX.Y.Z \
  --certificate-identity-regexp 'https://github.com/stubbi/hermes-operator/.github/workflows/.*' \
  --certificate-oidc-issuer https://token.actions.githubusercontent.com
```

## Signing & SBOMs

Full verification commands live at [`docs/security/signing.md`](docs/security/signing.md).
A weekly `verify-signing.yaml` workflow checks that the latest release is
still cosign-verifiable; if not, it auto-files an `infra-broken` issue.

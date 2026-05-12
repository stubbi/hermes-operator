# Security Policy

Report vulnerabilities by email to **jannes@aqora.io** with the subject "SECURITY: hermes-operator".

We aim to acknowledge within 72 hours and provide a remediation timeline within 7 days.

Operator images are signed with Cosign (keyless OIDC); SBOMs are attested and attached to releases. Verify with:

```bash
cosign verify ghcr.io/stubbi/hermes-operator:vX.Y.Z \
  --certificate-identity-regexp 'https://github.com/stubbi/hermes-operator/.github/workflows/.*' \
  --certificate-oidc-issuer https://token.actions.githubusercontent.com
```

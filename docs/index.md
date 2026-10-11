# Hermes Operator

A Kubernetes operator for managing Hermes AI agent instances with production-grade security, observability, and lifecycle management.

This site is the rendered documentation for the operator. The source of truth for every page lives under `docs/` in the [stubbi/hermes-operator](https://github.com/stubbi/hermes-operator) repository.

## Getting started

- [API conventions](conventions.md) explain the spec patterns shared across all resources.
- [API reference](api-reference.md) is the curated, human-readable field guide.
- [Generated API reference](api-reference-generated.md) is produced from the Go types by `make api-docs`.

## Operations

- [Agent runtime](runtime.md) explains the upstream s6 image, how the operator runs it, the `/opt/data` state path, the security/SCC tradeoff, and LLM provider configuration.
- [Conditions](conditions.md) documents the status conditions the operator sets.
- [Backup and restore](backup-restore.md) and the [backup format](backup-format.md) cover data protection.
- [Auto-update](autoupdate.md) describes registry polling and rollback.
- [Self-configuration](selfconfig.md) covers the HermesSelfConfig server-side-apply flow.
- [Platform gateways runbook](runbook-platform-gateways.md) is the on-call reference.

## Migration and compatibility

- [Migration](migration.md) walks through importing from a sibling operator or a backup.
- [API versioning](api-versioning.md), [supported versions](supported-versions.md), and [deprecations](deprecations.md) track compatibility.

## Distribution and conformance

- [Conformance](conformance.md) describes the nightly suite.
- [Release process](release-process.md) documents how releases are cut.
- [Signing](security/signing.md) covers image and SBOM attestation.

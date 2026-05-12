# Contributing

## Development

- Go 1.24+, kubebuilder v4, kind, helm, golangci-lint v1.64.5.
- `make test` runs unit + envtest. `make lint` runs golangci-lint.
- `make sync-chart-crds` after `make manifests` (CI enforces this).
- Use conventional commits: `feat:` `fix:` `docs:` `ci:` `chore:` `refactor:` `test:`.
- Use git worktrees rather than switching branches in the main checkout (`git worktree add ../hermes-operator-<suffix> -b <branch> main`).

## Reconciliation rules

See [`docs/conventions.md`](docs/conventions.md). The `Reconcile Guard` CI job enforces a subset; you are responsible for the rest.

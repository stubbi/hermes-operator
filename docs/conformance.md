# Conformance Suite

The `test/conformance/` tree is what makes hermes-operator a v1, not a v0.1.
Five categories of test mechanically defend the v1 stability commitments
documented in spec §11.

## Categories

### Negative tests (`negative_test.go`)

Every webhook deny path has a row in `negativeCases`. Adding a new validator
rule requires adding a row; CI fails if you forget. The table covers:

- `selfConfigure.enabled` without `protectedKeys` (spec §7.3)
- `config.raw` and `config.configMapRef` mutual exclusion (spec §7.3)
- Invalid storage quantity strings (`1XB`, whitespace, etc.)
- Empty `image.repository` with no `HermesClusterDefaults`
- Gateways `.enabled: true` without `tokenSecretRef`
- `backup.onDelete: true` without `backup.s3`
- `autoUpdate.enabled` with empty `source.registry`
- `migration.fromOpenClaw` with missing or doubled source
- Probe `successThreshold > 1` on liveness/startup probes
- `networking.ingress.enabled` without hosts
- HPA `minReplicas > maxReplicas`
- PDB `minAvailable` and `maxUnavailable` set simultaneously
- `restoreFrom` mutation after `status.restoredFrom` is set (immutability)
- `HermesSelfConfig.instanceRef` to non-existent instance
- `HermesSelfConfig` touching a protected key
- `HermesSelfConfig` with an unknown action type
- `HermesClusterDefaults` name not `cluster`
- `HermesClusterDefaults.storage.storageClassName` invalid

### Idempotency (`idempotency_test.go`)

For each of 10 manifests in `testdata/`, apply once → wait Ready → trigger 10
forced reconciles via no-op annotation pokes → assert `metadata.generation`
and `metadata.resourceVersion` on every managed resource is unchanged from the
post-first-reconcile fingerprint. This is the test that would have caught
openclaw's #437 before it shipped.

A generation bump means a builder reintroduced server-side default drift; a
resourceVersion bump without generation means an `r.Update()` slipped past
Reconcile Guard.

### Upgrade-path matrix (`upgrade_test.go`)

For every prior release tag from `v1.0.0` onward, install vN → create
`HermesInstance` + `HermesSelfConfig` + `HermesClusterDefaults` → wait Ready
→ upgrade operator to HEAD → assert no managed resource changed.

A `switch tag` block in the test lets a release deliberately *allow* a
specific upgrade-time mutation; the CHANGELOG entry for that release must
reference the allow-list addition. For v1.0 the matrix is empty; it
populates from v1.1 onward.

### GitOps coexistence (`gitops_test.go`)

Two concurrent SSA writers — a FluxCD-style manager and the operator's
`hermes.agent/selfconfig` field-manager — race against the same
`HermesInstance` for 200 iterations (each ~100ms apart, simulating 10
minutes of load). The test asserts at most one ownership flip on the
contended path (the initial settle), then both managers' fields coexist in
the final spec. >1 flip indicates SSA isolation is broken.

### Failure injection (`failure_injection_test.go`)

Four reconcile paths, each killed mid-flight via `kubectl delete pod
--force --grace-period=0`:

1. Instance create — assert Ready within 3 minutes after restart.
2. Instance update (patched resources) — assert StatefulSet reflects the patch.
3. SelfConfig apply — assert phase=Applied + spec change visible.
4. Backup-on-delete finalizer — assert instance fully deleted.

## Running locally

```bash
make conformance-kind-up
make conformance-install IMG=hermes-operator:dev
make conformance              # all categories
make conformance-negative     # one category
make conformance-idempotency
make conformance-upgrade
make conformance-gitops
make conformance-failure
make conformance-kind-down
```

## What CI does

- **PRs touching `test/conformance/`** — runs all five categories. Advisory.
- **Nightly on `main`** — runs all five. Required to be green before the next
  release.
- **On tag `v*`** — runs all five. Required.

## When to add a row

| Triggering change                                | Add to                       |
|--------------------------------------------------|------------------------------|
| New webhook validation rule                      | `negativeCases` table        |
| New `HermesInstance` sub-spec                    | new file under `testdata/`   |
| New optional sub-spec → exercise in idempotency  | new row in `idempotencyCorpus` |
| New CR kind                                      | scaffold a `*_test.go` like `negative_test.go` |
| New finalizer / reconcile path                   | new `It("...")` in `failure_injection_test.go` |
| Deliberate breaking change in a release          | `switch tag` arm in `upgrade_test.go` + CHANGELOG note |

## What this suite does NOT cover

- Performance and resource consumption (see `internal/{resources,controller}/*_bench_test.go`).
- Security scanning (gosec + Trivy run in `ci.yaml`).
- Cosign + SBOM verification (`verify-signing.yaml`).
- OperatorHub bundle validation (`operator-sdk bundle validate`,
  `make bundle-validate`).

## Summary

<!-- 1-3 sentences. What does this change and why? -->

## Test plan

- [ ] Lint passes (`make lint`)
- [ ] Tests pass (`make test`)
- [ ] Reconcile-guard passes (`bash hack/reconcile-guard.sh`)
- [ ] Helm RBAC sync passes (`bash hack/check-helm-rbac.sh`)
- [ ] For CRD/API changes: `make manifests` + `make generate` regenerated
- [ ] For RBAC changes: `make sync-bundle-rbac` if the bundle is affected
- [ ] For behavior changes: added/updated an envtest or e2e test

## Related issues

<!-- Closes #123 / refs #456 -->

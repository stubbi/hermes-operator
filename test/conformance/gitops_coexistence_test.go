/*
Copyright 2026 stubbi. Apache-2.0.

This file is a placeholder for the full Plan-6 conformance suite. Plan 4
already proves SSA-based GitOps coexistence in
`internal/controller/hermesselfconfig_ssa_test.go` against envtest. Plan 6
will re-use the same scenario at higher scale (real kind cluster, multiple
concurrent Flux/SelfConfig writers, latency assertions) and parameterise
across Kubernetes versions 1.28-1.32.

Until Plan 6 lands, this test compiles but is skipped: it's here so a
future Plan-6 engineer can `git grep gitops_coexistence_test` to find the
entry point.
*/

package conformance

import "testing"

func TestGitOpsCoexistenceConformance(t *testing.T) {
	t.Skip("Plan 6 (conformance) wires this; see internal/controller/hermesselfconfig_ssa_test.go for the envtest version")
}

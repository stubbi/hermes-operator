package conformance

import (
	"context"
	"os"
	"testing"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

// Conformance suite — categories live in sibling files:
//   - negative_test.go             webhook deny paths
//   - idempotency_test.go          10-reconcile no-op canary
//   - upgrade_test.go              prior-release -> HEAD matrix
//   - gitops_test.go               FluxCD SSA + SelfConfig no-flap
//   - failure_injection_test.go    SIGKILL mid-reconcile
func TestConformance(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "hermes-operator conformance suite")
}

var (
	suiteCtx    context.Context
	suiteCancel context.CancelFunc
)

var _ = BeforeSuite(func() {
	suiteCtx, suiteCancel = context.WithCancel(context.Background())
	SetDefaultEventuallyTimeout(5 * time.Minute)
	SetDefaultEventuallyPollingInterval(2 * time.Second)
	if os.Getenv("KUBECONFIG") == "" {
		Skip("KUBECONFIG not set — conformance suite requires a live kind cluster with the operator installed")
	}
})

var _ = AfterSuite(func() {
	if suiteCancel != nil {
		suiteCancel()
	}
})

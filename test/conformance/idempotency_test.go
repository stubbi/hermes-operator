package conformance

import (
	"fmt"
	"path/filepath"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

// idempotencyCorpus maps a human-readable label to the testdata fixture file.
// Each fixture is applied once, allowed to become Ready, then force-requeued 9
// more times. After each requeue we assert the resourceFingerprint is unchanged
// (generation + resourceVersion must not move). This catches lesson #437
// regressions: a reconciler that always re-writes owned objects will fail here.
var idempotencyCorpus = []struct {
	label   string
	fixture string
}{
	{"minimal", "minimal.yaml"},
	{"maximal", "maximal.yaml"},
	{"gateways-all", "gateways-all.yaml"},
	{"selfconfig-enabled", "selfconfig-enabled.yaml"},
	{"profilestore-enabled", "profilestore-enabled.yaml"},
	{"autoupdate-enabled", "autoupdate-enabled.yaml"},
	{"backup-enabled", "backup-enabled.yaml"},
	{"networking-ingress", "networking-ingress.yaml"},
	{"observability-full", "observability-full.yaml"},
	{"ollama-webterminal-tailscale", "ollama-webterminal-tailscale.yaml"},
}

const (
	idempotencyReconciles = 10
	idempotencyReadyWait  = 3 * time.Minute
	idempotencyPokeWait   = 15 * time.Second
)

var _ = Describe("idempotency canary", Ordered, func() {
	var (
		ns string
		c  = newClient
	)

	BeforeAll(func() {
		ns = freshNamespace("idempotency")
		DeferCleanup(func() {
			deleteNamespace(ns)
		})
	})

	for _, entry := range idempotencyCorpus {
		entry := entry // capture

		Describe(fmt.Sprintf("corpus entry: %s", entry.label), Ordered, func() {
			var instName string

			BeforeAll(func() {
				fixturePath := filepath.Join("testdata", entry.fixture)
				yaml := readFile(fixturePath)
				// Inject the test namespace into the fixture.
				namespaced := addNamespace(yaml, ns)

				out, err := kubectlApply(namespaced)
				Expect(err).ToNot(HaveOccurred(),
					"applying fixture %s: %s", entry.fixture, out)

				// Extract the instance name from the fixture (first `name:` under metadata).
				instName = extractName(yaml)
				Expect(instName).ToNot(BeEmpty(), "could not extract name from fixture %s", entry.fixture)

				DeferCleanup(func() {
					_, _ = kubectlDelete(namespaced)
				})
			})

			It("becomes Ready", func() {
				waitForInstanceReady(suiteCtx, c(), ns, instName, idempotencyReadyWait)
			})

			It(fmt.Sprintf("resource fingerprint is stable across %d reconciles", idempotencyReconciles), func() {
				cl := c()
				before := captureFingerprint(suiteCtx, cl, ns, instName)

				for i := 1; i < idempotencyReconciles; i++ {
					forceRequeue(suiteCtx, cl, ns, instName)
					// Give the controller a moment to process the requeue.
					time.Sleep(idempotencyPokeWait)
					after := captureFingerprint(suiteCtx, cl, ns, instName)
					expectFingerprintUnchanged(before, after)
					before = after
				}
			})
		})
	}
})

// extractName parses the `name:` field from the first metadata block in a
// YAML manifest. It is intentionally naive: it walks lines looking for the
// pattern "  name: <value>" after a "metadata:" line.
func extractName(yaml string) string {
	inMeta := false
	for _, line := range splitLines(yaml) {
		if line == "metadata:" {
			inMeta = true
			continue
		}
		if inMeta {
			trimmed := trimPrefix(line, "  name: ")
			if trimmed != line {
				return trimmed
			}
			// Any non-indented line ends the metadata block.
			if len(line) > 0 && line[0] != ' ' {
				inMeta = false
			}
		}
	}
	return ""
}

func splitLines(s string) []string {
	var lines []string
	start := 0
	for i := 0; i < len(s); i++ {
		if s[i] == '\n' {
			lines = append(lines, s[start:i])
			start = i + 1
		}
	}
	if start < len(s) {
		lines = append(lines, s[start:])
	}
	return lines
}

func trimPrefix(s, prefix string) string {
	if len(s) >= len(prefix) && s[:len(prefix)] == prefix {
		return s[len(prefix):]
	}
	return s
}

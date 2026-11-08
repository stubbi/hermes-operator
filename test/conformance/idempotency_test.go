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
// The Ready-gated corpus now runs. The runtime blockers (#68 → #89) are fixed in
// #90: the operator runs the upstream s6 hermes-agent image with `gateway run` +
// the OpenAI API server, probes HTTPGet /health, and injects a placeholder LLM
// provider so the gateway comes up without live calls. The fixtures pin
// `ghcr.io/stubbi/hermes-agent:v0.16.0`, which is the upstream-based
// runtime. Validated end-to-end on kind (an instance reaches Ready=True).
// waitForInstanceReady dumps pod diagnostics on timeout if anything regresses.

var idempotencyCorpus = []struct {
	label   string
	fixture string
	// skip, when non-empty, skips this corpus entry with the given reason.
	// Used for fixtures that cannot reach Ready in CI for reasons unrelated to
	// operator idempotency (e.g. they require live external credentials).
	skip string
}{
	{label: "minimal", fixture: "minimal.yaml"},
	{label: "maximal", fixture: "maximal.yaml"},
	{label: "gateways-all", fixture: "gateways-all.yaml"},
	{label: "selfconfig-enabled", fixture: "selfconfig-enabled.yaml"},
	{label: "profilestore-enabled", fixture: "profilestore-enabled.yaml"},
	{label: "autoupdate-enabled", fixture: "autoupdate-enabled.yaml"},
	{label: "backup-enabled", fixture: "backup-enabled.yaml"},
	{label: "networking-ingress", fixture: "networking-ingress.yaml"},
	{label: "observability-full", fixture: "observability-full.yaml"},
	{
		label:   "ollama-webterminal-tailscale",
		fixture: "ollama-webterminal-tailscale.yaml",
		// Blocked by the operator-managed tailscale sidecar: it runs
		// `containerboot`, which exits when TS_AUTHKEY cannot join a tailnet.
		// The fixture ships a dummy auth key (no real ephemeral key is available
		// in CI), so the sidecar container never becomes Ready, the pod stays
		// NotReady, and the HermesInstance never reaches Ready=True. Unskip only
		// once a real ephemeral tailnet auth key is injected via secret in CI.
		// See #64. (#68, which blocked every other entry, is now fixed.)
		skip: "requires a live tailscale ephemeral auth key to reach Ready (dummy key cannot join a tailnet); see #64",
	},
}

const (
	idempotencyReconciles = 10
	idempotencyReadyWait  = 3 * time.Minute
	idempotencyPokeWait   = 15 * time.Second
)

// seedConformanceSecrets creates the dummy Secrets the feature-rich corpus
// fixtures reference (gateway tokens, Honcho API key, maximal's extra-env). The
// operator wires these into the agent container via non-optional secretKeyRefs,
// so they must exist or the pod fails with CreateContainerConfigError. Values are
// placeholders — Ready only needs the env to resolve, not the upstream to accept.
func seedConformanceSecrets(ns string) {
	manifest := fmt.Sprintf(`
apiVersion: v1
kind: Secret
metadata: {name: tg-token, namespace: %[1]s}
stringData: {token: dummy}
---
apiVersion: v1
kind: Secret
metadata: {name: discord-token, namespace: %[1]s}
stringData: {token: dummy}
---
apiVersion: v1
kind: Secret
metadata: {name: slack-token, namespace: %[1]s}
stringData: {bot-token: dummy, app-token: dummy, signing-secret: dummy}
---
apiVersion: v1
kind: Secret
metadata: {name: wa-token, namespace: %[1]s}
stringData: {token: dummy}
---
apiVersion: v1
kind: Secret
metadata: {name: sig-token, namespace: %[1]s}
stringData: {phone-number: "+10000000000", auth-token: dummy}
---
apiVersion: v1
kind: Secret
metadata: {name: api-keys, namespace: %[1]s}
stringData: {honcho-api-key: dummy}
---
apiVersion: v1
kind: Secret
metadata: {name: hermes-maximal-extra-env, namespace: %[1]s}
stringData: {HERMES_EXTRA: "1"}
`, ns)
	out, err := kubectlApply(manifest)
	Expect(err).ToNot(HaveOccurred(), "seed conformance secrets: %s", out)
}

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
		// Seed the dummy Secrets the feature-rich fixtures reference (gateway
		// tokens, Honcho key, maximal's extra-env). A real deployment ships these
		// alongside the instance; the operator injects them into the agent via
		// non-optional secretKeyRefs, so they must exist for the pod to start.
		seedConformanceSecrets(ns)
	})

	for _, entry := range idempotencyCorpus {
		entry := entry // capture

		Describe(fmt.Sprintf("corpus entry: %s", entry.label), Ordered, func() {
			var instName string

			BeforeAll(func() {
				if entry.skip != "" {
					Skip(entry.skip)
				}
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
					// Non-blocking: a fixture's on-delete finalizer (e.g.
					// backup-enabled's snapshot Job on placeholder creds) must not
					// stall the rest of the corpus. The namespace is torn down at end.
					_, _ = kubectlDeleteNoWait(namespaced)
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

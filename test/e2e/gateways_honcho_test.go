package e2e

import (
	"os"
	"strings"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("HermesInstance with Telegram + Honcho on kind", Ordered, func() {
	BeforeAll(func() {
		if os.Getenv("HERMES_E2E_FULL") != "1" {
			Skip("set HERMES_E2E_FULL=1 to enable the Plan-3 gateway+honcho e2e (requires the agent image to be published)")
		}
		out, err := kubectl("apply", "-f", "testdata/hermesinstance-gateways.yaml")
		Expect(err).NotTo(HaveOccurred(), out)
	})

	AfterAll(func() {
		if os.Getenv("HERMES_E2E_FULL") != "1" {
			return
		}
		_, _ = kubectl("delete", "-f", "testdata/hermesinstance-gateways.yaml", "--ignore-not-found=true")
	})

	It("brings the hermes pod to Ready", func() {
		Eventually(func(g Gomega) {
			out, err := kubectl("get", "statefulset", "e2e-gateways",
				"-n", "default",
				"-o", "jsonpath={.status.readyReplicas}")
			g.Expect(err).NotTo(HaveOccurred(), out)
			g.Expect(strings.TrimSpace(out)).To(Equal("1"))
		}).Should(Succeed())
	})

	It("brings the Honcho Deployment to Ready", func() {
		Eventually(func(g Gomega) {
			out, err := kubectl("get", "deployment", "e2e-gateways-honcho",
				"-n", "default",
				"-o", "jsonpath={.status.readyReplicas}")
			g.Expect(err).NotTo(HaveOccurred(), out)
			g.Expect(strings.TrimSpace(out)).To(Equal("1"))
		}).Should(Succeed())
	})

	It("emits a NetworkPolicy with a 443/TCP egress rule for the Telegram endpoint", func() {
		out, err := kubectl("get", "networkpolicy", "e2e-gateways",
			"-n", "default",
			"-o", "jsonpath={.spec.egress[*].ports[*].port}")
		Expect(err).NotTo(HaveOccurred(), out)
		Expect(out).To(ContainSubstring("443"))
	})

	It("emits a Honcho-scoped NetworkPolicy with ingress only from the hermes pod", func() {
		out, err := kubectl("get", "networkpolicy", "e2e-gateways-honcho",
			"-n", "default",
			"-o", `jsonpath={.spec.ingress[*].from[*].podSelector.matchLabels.app\.kubernetes\.io/name}`)
		Expect(err).NotTo(HaveOccurred(), out)
		Expect(strings.TrimSpace(out)).To(Equal("hermes-agent"))
	})
})

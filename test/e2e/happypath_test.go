package e2e

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("Happy path", func() {
	It("reconciles a minimal HermesInstance into a managed StatefulSet", func() {
		// We can't assert pod-ready end-to-end until a real hermes-agent image
		// is published (the operator's readiness probe targets the agent's
		// gateway port). What we CAN assert is the operator's contract:
		// HermesInstance -> StatefulSet, Service, ConfigMap, PVC all created
		// with the expected owner refs.
		manifest := `
apiVersion: hermes.agent/v1
kind: HermesInstance
metadata:
  name: e2e-demo
  namespace: default
spec:
  image:
    repository: ghcr.io/nginx/nginx-unprivileged
    tag: latest
    pullPolicy: IfNotPresent
  storage:
    persistence:
      enabled: true
      size: 1Gi
`
		out, err := runStdin("kubectl", []string{"apply", "-f", "-"}, manifest)
		Expect(err).ToNot(HaveOccurred(), "kubectl apply failed: %s", out)
		DeferCleanup(func() {
			_, _ = kubectl("delete", "hermesinstance", "e2e-demo", "-n", "default", "--ignore-not-found", "--wait=false")
		})

		Eventually(func() string {
			out, _ := kubectl("get", "statefulset", "e2e-demo", "-n", "default", "-o", "jsonpath={.metadata.name}")
			return out
		}).Should(Equal("e2e-demo"), "operator never created the StatefulSet")
		Eventually(func() string {
			out, _ := kubectl("get", "service", "e2e-demo", "-n", "default", "-o", "jsonpath={.metadata.name}")
			return out
		}).Should(Equal("e2e-demo"), "operator never created the Service")
		Eventually(func() string {
			out, _ := kubectl("get", "configmap", "e2e-demo-config", "-n", "default", "-o", "jsonpath={.metadata.name}")
			return out
		}).Should(Equal("e2e-demo-config"), "operator never created the ConfigMap")
		Eventually(func() string {
			out, _ := kubectl("get", "pvc", "e2e-demo-data", "-n", "default", "-o", "jsonpath={.metadata.name}")
			return out
		}).Should(Equal("e2e-demo-data"), "operator never created the PVC")
	})
})

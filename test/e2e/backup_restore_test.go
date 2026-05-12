package e2e

import (
	"fmt"
	"strings"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("Backup → delete → restore cycle (MinIO)", func() {
	const ns = "default"

	It("performs a full backup, on-delete final backup, and restore", func() {
		manifest := `
apiVersion: hermes.agent/v1
kind: HermesInstance
metadata:
  name: e2e-br
  namespace: default
spec:
  image:
    repository: ghcr.io/nginx/nginx-unprivileged
    tag: stable
  storage:
    persistence:
      enabled: true
      size: 1Gi
  backup:
    onDelete: true
    schedule: "*/2 * * * *"
    s3:
      bucket: hermes-backups
      endpoint: minio.minio.svc:9000
      region: us-east-1
      pathPrefix: e2e/
      credentialsSecretRef:
        name: hermes-s3-creds
`
		out, err := runStdin("kubectl", []string{"apply", "-f", "-"}, manifest)
		Expect(err).ToNot(HaveOccurred(), "kubectl apply: %s", out)

		Eventually(func() string {
			out, _ := kubectl("get", "cronjob/e2e-br-backup-cron", "-n", ns, "-o", "jsonpath={.metadata.name}")
			return strings.TrimSpace(out)
		}, 2*time.Minute).Should(Equal("e2e-br-backup-cron"))

		out, err = kubectl("create", "job", "manual-1", "-n", ns, "--from=cronjob/e2e-br-backup-cron")
		Expect(err).ToNot(HaveOccurred(), "create manual job: %s", out)

		Eventually(func() string {
			out, _ := kubectl("get", "job/manual-1", "-n", ns, "-o", "jsonpath={.status.succeeded}")
			return strings.TrimSpace(out)
		}, 3*time.Minute).Should(Equal("1"))

		snapshotKey := findFirstSnapshotKey(ns, "e2e-br")
		Expect(snapshotKey).NotTo(BeEmpty(), "expected at least one snapshot in the bucket")
		GinkgoWriter.Printf("found scheduled snapshot: %s\n", snapshotKey)

		out, err = kubectl("delete", "hermesinstance/e2e-br", "-n", ns, "--wait=false")
		Expect(err).ToNot(HaveOccurred(), "delete: %s", out)

		Eventually(func() bool {
			out, _ := kubectl("get", "hermesinstance/e2e-br", "-n", ns, "--ignore-not-found")
			return strings.TrimSpace(out) == ""
		}, 5*time.Minute).Should(BeTrue())

		restoreManifest := fmt.Sprintf(`
apiVersion: hermes.agent/v1
kind: HermesInstance
metadata:
  name: e2e-restore
  namespace: default
spec:
  image:
    repository: ghcr.io/nginx/nginx-unprivileged
    tag: stable
  storage:
    persistence:
      enabled: true
      size: 1Gi
  restoreFrom: %q
  backup:
    s3:
      bucket: hermes-backups
      endpoint: minio.minio.svc:9000
      region: us-east-1
      pathPrefix: e2e/
      credentialsSecretRef:
        name: hermes-s3-creds
`, snapshotKey)
		out, err = runStdin("kubectl", []string{"apply", "-f", "-"}, restoreManifest)
		Expect(err).ToNot(HaveOccurred(), "apply restore: %s", out)

		Eventually(func() string {
			out, _ := kubectl("get", "hermesinstance/e2e-restore", "-n", ns, "-o", "jsonpath={.status.restoredFrom}")
			return strings.TrimSpace(out)
		}, 5*time.Minute).Should(Equal(snapshotKey))

		_, _ = kubectl("delete", "hermesinstance/e2e-restore", "-n", ns, "--ignore-not-found")
	})
})

// findFirstSnapshotKey lists the bucket prefix and returns the first key, or "" if nothing.
func findFirstSnapshotKey(namespace, instance string) string {
	cmd := []string{
		"run", "mc-list", "--namespace", "minio", "--rm", "-i", "--restart=Never",
		"--image=minio/mc:RELEASE.2024-09-16T17-43-14Z",
		"--", "/bin/sh", "-c",
		`mc alias set local http://minio:9000 minioadmin minioadmin >/dev/null 2>&1 && mc ls --recursive "local/hermes-backups/e2e/` + namespace + `/` + instance + `/" | awk '{print $NF}' | head -n 1`,
	}
	out, err := kubectl(cmd...)
	if err != nil {
		return ""
	}
	return strings.TrimSpace(out)
}

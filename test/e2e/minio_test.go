package e2e

import (
	"fmt"
	"strings"

	. "github.com/onsi/gomega"
)

const minioManifest = `
---
apiVersion: v1
kind: Namespace
metadata:
  name: minio
---
apiVersion: v1
kind: Secret
metadata:
  name: minio-root
  namespace: minio
type: Opaque
stringData:
  rootUser: minioadmin
  rootPassword: minioadmin
---
apiVersion: apps/v1
kind: Deployment
metadata:
  name: minio
  namespace: minio
spec:
  replicas: 1
  selector: { matchLabels: { app: minio } }
  template:
    metadata: { labels: { app: minio } }
    spec:
      containers:
        - name: minio
          image: quay.io/minio/minio:RELEASE.2024-09-13T20-26-02Z
          args: ["server", "/data", "--console-address", ":9001"]
          env:
            - name: MINIO_ROOT_USER
              valueFrom: { secretKeyRef: { name: minio-root, key: rootUser } }
            - name: MINIO_ROOT_PASSWORD
              valueFrom: { secretKeyRef: { name: minio-root, key: rootPassword } }
          ports:
            - containerPort: 9000
            - containerPort: 9001
          volumeMounts:
            - { name: data, mountPath: /data }
      volumes:
        - name: data
          emptyDir: {}
---
apiVersion: v1
kind: Service
metadata:
  name: minio
  namespace: minio
spec:
  selector: { app: minio }
  ports:
    - name: api
      port: 9000
      targetPort: 9000
    - name: console
      port: 9001
      targetPort: 9001
---
apiVersion: batch/v1
kind: Job
metadata:
  name: mc-mkbucket
  namespace: minio
spec:
  backoffLimit: 6
  template:
    spec:
      restartPolicy: OnFailure
      containers:
        - name: mc
          image: minio/mc:RELEASE.2024-09-16T17-43-14Z
          command: ["/bin/sh", "-c"]
          args:
            - |
              until mc alias set local http://minio:9000 "$MINIO_ROOT_USER" "$MINIO_ROOT_PASSWORD"; do sleep 2; done
              mc mb --ignore-existing local/hermes-backups
              mc mb --ignore-existing local/openclaw-backups
          env:
            - name: MINIO_ROOT_USER
              valueFrom: { secretKeyRef: { name: minio-root, key: rootUser } }
            - name: MINIO_ROOT_PASSWORD
              valueFrom: { secretKeyRef: { name: minio-root, key: rootPassword } }
`

// InstallMinIO deploys MinIO + creates a bucket. Idempotent.
func InstallMinIO() {
	out, err := runStdin("kubectl", []string{"apply", "-f", "-"}, minioManifest)
	Expect(err).ToNot(HaveOccurred(), "kubectl apply failed: %s", out)

	Eventually(func() string {
		out, _ := kubectl("get", "deploy/minio", "-n", "minio", "-o", "jsonpath={.status.readyReplicas}")
		return strings.TrimSpace(out)
	}).Should(Equal("1"))

	Eventually(func() string {
		out, _ := kubectl("get", "job/mc-mkbucket", "-n", "minio", "-o", "jsonpath={.status.succeeded}")
		return strings.TrimSpace(out)
	}).Should(Equal("1"))
}

// CreateHermesS3CredsSecret writes the MinIO credentials into the agent namespace.
func CreateHermesS3CredsSecret(namespace string) {
	manifest := fmt.Sprintf(`
apiVersion: v1
kind: Secret
metadata:
  name: hermes-s3-creds
  namespace: %s
stringData:
  S3_ACCESS_KEY_ID: minioadmin
  S3_SECRET_ACCESS_KEY: minioadmin
`, namespace)
	out, err := runStdin("kubectl", []string{"apply", "-f", "-"}, manifest)
	Expect(err).ToNot(HaveOccurred(), "kubectl apply minio creds failed: %s", out)
}

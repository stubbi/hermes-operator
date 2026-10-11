package e2e

import (
	"fmt"
	"strings"

	. "github.com/onsi/gomega"
)

const (
	// s3ServerImage is the S3-compatible server that stands in for MinIO.
	s3ServerImage = "docker.io/versity/versitygw:v1.8.0"
	// s3ClientImage creates buckets and lists objects in the e2e.
	s3ClientImage = "docker.io/amazon/aws-cli:2.35.6"
)

// minioManifest deploys an in-cluster S3-compatible endpoint for the
// backup/restore e2e. The namespace, Service and Secret keep their historical
// "minio" names because the e2e and conformance fixtures point at
// minio.minio.svc:9000, but the server is Versity S3 Gateway: the upstream
// MinIO images (quay.io/minio/minio, minio/minio, minio/mc) are no longer
// publicly pullable, which broke the e2e BeforeSuite for every PR.
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
        - name: s3
          image: ` + s3ServerImage + `
          args: ["--port", ":9000", "posix", "/data"]
          env:
            - name: ROOT_ACCESS_KEY
              valueFrom: { secretKeyRef: { name: minio-root, key: rootUser } }
            - name: ROOT_SECRET_KEY
              valueFrom: { secretKeyRef: { name: minio-root, key: rootPassword } }
          ports:
            - containerPort: 9000
          readinessProbe:
            tcpSocket: { port: 9000 }
            periodSeconds: 2
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
        - name: mkbucket
          image: ` + s3ClientImage + `
          command: ["/bin/sh", "-c"]
          args:
            - |
              set -e
              S3="aws --endpoint-url http://minio:9000 s3api"
              until $S3 list-buckets >/dev/null; do sleep 2; done
              for b in hermes-backups openclaw-backups; do
                $S3 head-bucket --bucket "$b" 2>/dev/null || $S3 create-bucket --bucket "$b"
              done
          env:
            - name: AWS_DEFAULT_REGION
              value: us-east-1
            - name: AWS_ACCESS_KEY_ID
              valueFrom: { secretKeyRef: { name: minio-root, key: rootUser } }
            - name: AWS_SECRET_ACCESS_KEY
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

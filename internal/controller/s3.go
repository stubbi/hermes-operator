package controller

import (
	"context"
	"fmt"
	"strings"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	hermesv1 "github.com/stubbi/hermes-operator/api/v1"
)

// S3Creds is the minimal pair we need to authenticate against any S3-compatible API.
type S3Creds struct {
	AccessKeyID     string
	SecretAccessKey string
}

// ResticImage is the pinned default snapshot-tool image. Override via spec.backup.image.
const ResticImage = "restic/restic:0.16.4"

// ReadS3CredsFromSecret loads S3_ACCESS_KEY_ID + S3_SECRET_ACCESS_KEY from a Secret.
func ReadS3CredsFromSecret(ctx context.Context, c client.Client, namespace, name string) (*S3Creds, error) {
	secret := &corev1.Secret{}
	if err := c.Get(ctx, types.NamespacedName{Namespace: namespace, Name: name}, secret); err != nil {
		return nil, fmt.Errorf("fetch S3 credentials secret %s/%s: %w", namespace, name, err)
	}
	get := func(k string) (string, error) {
		v, ok := secret.Data[k]
		if !ok || len(v) == 0 {
			return "", fmt.Errorf("S3 credentials secret %s/%s missing key %q", namespace, name, k)
		}
		return string(v), nil
	}
	id, err := get("S3_ACCESS_KEY_ID")
	if err != nil {
		return nil, err
	}
	sec, err := get("S3_SECRET_ACCESS_KEY")
	if err != nil {
		return nil, err
	}
	return &S3Creds{AccessKeyID: id, SecretAccessKey: sec}, nil
}

// SnapshotKey returns the canonical S3 key for a snapshot of inst.
// kind is "scheduled" | "preUpdate" | "onDelete" | "failed".
func SnapshotKey(inst *hermesv1.HermesInstance, kind, timestamp string) string {
	prefix := ""
	if inst.Spec.Backup.S3 != nil {
		prefix = inst.Spec.Backup.S3.PathPrefix
		if prefix != "" && !strings.HasSuffix(prefix, "/") {
			prefix += "/"
		}
	}
	if kind == "failed" {
		return fmt.Sprintf("%s%s/%s/failed/%s.tar.zst", prefix, inst.Namespace, inst.Name, timestamp)
	}
	return fmt.Sprintf("%s%s/%s/%s.tar.zst", prefix, inst.Namespace, inst.Name, timestamp)
}

// S3EnvVars returns the env vars that the restic container expects.
func S3EnvVars(creds *S3Creds, s3 *hermesv1.BackupS3Spec) []corev1.EnvVar {
	env := []corev1.EnvVar{
		{Name: "AWS_ACCESS_KEY_ID", Value: creds.AccessKeyID},
		{Name: "AWS_SECRET_ACCESS_KEY", Value: creds.SecretAccessKey},
		{Name: "RESTIC_REPOSITORY", Value: fmt.Sprintf("s3:%s/%s", s3.Endpoint, s3.Bucket)},
	}
	if s3.Region != "" {
		env = append(env, corev1.EnvVar{Name: "AWS_DEFAULT_REGION", Value: s3.Region})
	}
	return env
}

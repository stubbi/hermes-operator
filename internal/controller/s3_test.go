package controller

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	hermesv1 "github.com/stubbi/hermes-operator/api/v1"
)

func TestReadS3CredsFromSecret_RequiredKeys(t *testing.T) {
	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{Name: "s3-creds", Namespace: "agents"},
		Data: map[string][]byte{
			"S3_ACCESS_KEY_ID":     []byte("AKIATEST"),
			"S3_SECRET_ACCESS_KEY": []byte("supersecret"),
		},
	}
	c := fake.NewClientBuilder().WithObjects(secret).Build()
	creds, err := ReadS3CredsFromSecret(context.Background(), c, "agents", "s3-creds")
	require.NoError(t, err)
	assert.Equal(t, "AKIATEST", creds.AccessKeyID)
	assert.Equal(t, "supersecret", creds.SecretAccessKey)
}

func TestReadS3CredsFromSecret_MissingKey(t *testing.T) {
	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{Name: "s3-creds", Namespace: "agents"},
		Data:       map[string][]byte{"S3_ACCESS_KEY_ID": []byte("AKIATEST")},
	}
	c := fake.NewClientBuilder().WithObjects(secret).Build()
	_, err := ReadS3CredsFromSecret(context.Background(), c, "agents", "s3-creds")
	assert.ErrorContains(t, err, "S3_SECRET_ACCESS_KEY")
}

func TestSnapshotKey_Format(t *testing.T) {
	inst := &hermesv1.HermesInstance{
		ObjectMeta: metav1.ObjectMeta{Name: "demo", Namespace: "agents"},
		Spec: hermesv1.HermesInstanceSpec{
			Backup: hermesv1.BackupSpec{
				S3: &hermesv1.BackupS3Spec{Bucket: "b", Endpoint: "e", PathPrefix: "prod/"},
			},
		},
	}
	assert.Equal(t, "prod/agents/demo/2026-05-10T03-00-00Z.tar.zst",
		SnapshotKey(inst, "scheduled", "2026-05-10T03-00-00Z"))
	assert.Equal(t, "prod/agents/demo/failed/2026-05-10T03-00-00Z.tar.zst",
		SnapshotKey(inst, "failed", "2026-05-10T03-00-00Z"))
}

package webhook

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	hermesv1 "github.com/paperclipinc/hermes-operator/api/v1"
)

func brokerInstance(sb *hermesv1.SecretsBrokerSpec) *hermesv1.HermesInstance {
	return &hermesv1.HermesInstance{
		ObjectMeta: metav1.ObjectMeta{Name: "demo"},
		Spec: hermesv1.HermesInstanceSpec{
			Image:         hermesv1.ImageSpec{Repository: "ghcr.io/paperclipinc/hermes-agent"},
			Storage:       hermesv1.StorageSpec{Persistence: hermesv1.PersistenceSpec{Size: "1Gi"}},
			SecretsBroker: sb,
		},
	}
}

func secretRef(name, key string) *hermesv1.SecretsBrokerHeaderSource {
	return &hermesv1.SecretsBrokerHeaderSource{SecretKeyRef: corev1.SecretKeySelector{
		LocalObjectReference: corev1.LocalObjectReference{Name: name}, Key: key,
	}}
}

func TestValidator_SecretsBroker(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name    string
		sb      hermesv1.SecretsBrokerSpec
		wantErr string
	}{
		{name: "https with secret-sourced auth", sb: hermesv1.SecretsBrokerSpec{
			URL: "https://broker.example/resolve/${EMPLOYEE_ID}",
			Headers: []hermesv1.SecretsBrokerHeader{
				{Name: "Authorization", ValueFrom: secretRef("broker-auth", "header")},
				{Name: "X-Employee-Id", Value: "${EMPLOYEE_ID}"},
			},
		}},
		{name: "placeholder authorization is fine", sb: hermesv1.SecretsBrokerSpec{
			URL:     "https://b/x",
			Headers: []hermesv1.SecretsBrokerHeader{{Name: "Authorization", Value: "Bearer ${BROKER_AUTH_TOKEN}"}},
		}},
		{name: "http allowed when waived", sb: hermesv1.SecretsBrokerSpec{URL: "http://broker.ns.svc/x", AllowInsecureHTTP: true}},
		{name: "http refused by default", sb: hermesv1.SecretsBrokerSpec{URL: "http://broker.ns.svc/x"}, wantErr: "must use https"},
		{name: "not a url", sb: hermesv1.SecretsBrokerSpec{URL: "broker"}, wantErr: "absolute https"},
		{name: "literal authorization", sb: hermesv1.SecretsBrokerSpec{
			URL: "https://b/x", Headers: []hermesv1.SecretsBrokerHeader{{Name: "authorization", Value: "Bearer abc"}},
		}, wantErr: "credential"},
		{name: "both sources", sb: hermesv1.SecretsBrokerSpec{
			URL: "https://b/x", Headers: []hermesv1.SecretsBrokerHeader{{Name: "A", Value: "x", ValueFrom: secretRef("s", "k")}},
		}, wantErr: "exactly one"},
		{name: "no source", sb: hermesv1.SecretsBrokerSpec{
			URL: "https://b/x", Headers: []hermesv1.SecretsBrokerHeader{{Name: "A"}},
		}, wantErr: "exactly one"},
		{name: "incomplete secret ref", sb: hermesv1.SecretsBrokerSpec{
			URL: "https://b/x", Headers: []hermesv1.SecretsBrokerHeader{{Name: "A", ValueFrom: secretRef("s", "")}},
		}, wantErr: "name and key"},
		{name: "duplicate header ignoring case", sb: hermesv1.SecretsBrokerSpec{
			URL: "https://b/x", Headers: []hermesv1.SecretsBrokerHeader{{Name: "X-A", Value: "1"}, {Name: "x-a", Value: "2"}},
		}, wantErr: "more than once"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			sb := tc.sb
			_, err := (&HermesInstanceValidator{}).ValidateCreate(context.Background(), brokerInstance(&sb))
			if tc.wantErr == "" {
				require.NoError(t, err)
				return
			}
			require.Error(t, err)
			assert.Contains(t, err.Error(), tc.wantErr)
		})
	}
}

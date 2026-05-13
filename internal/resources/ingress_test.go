package resources

import (
	"testing"

	"github.com/stretchr/testify/assert"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	hermesv1 "github.com/stubbi/hermes-operator/api/v1"
)

func TestDetectIngressProvider(t *testing.T) {
	t.Parallel()
	cases := map[string]IngressProvider{
		"nginx":      IngressProviderNginx,
		"NGINX":      IngressProviderNginx,
		"traefik":    IngressProviderTraefik,
		"traefik-lb": IngressProviderTraefik,
		"haproxy":    IngressProviderUnknown,
		"":           IngressProviderUnknown,
	}
	for in, want := range cases {
		var ptr *string
		if in != "" {
			ptr = Ptr(in)
		}
		assert.Equal(t, want, DetectIngressProvider(ptr), "input=%q", in)
	}
}

func TestBuildIngress_BasicShape(t *testing.T) {
	t.Parallel()
	inst := &hermesv1.HermesInstance{
		ObjectMeta: metav1.ObjectMeta{Name: "demo", Namespace: "agents"},
		Spec: hermesv1.HermesInstanceSpec{
			Networking: hermesv1.NetworkingSpec{
				Ingress: hermesv1.IngressSpec{
					Enabled:         Ptr(true),
					Host:            "hermes.example.com",
					ClassName:       Ptr("nginx"),
					Path:            "/",
					PathType:        "Prefix",
					ServicePortName: "gateway",
				},
			},
		},
	}
	ing := BuildIngress(inst)
	assert.Equal(t, "demo", ing.Name)
	assert.Equal(t, "agents", ing.Namespace)
	assert.NotNil(t, ing.Spec.IngressClassName)
	assert.Equal(t, "nginx", *ing.Spec.IngressClassName)
	assert.Len(t, ing.Spec.Rules, 1)
	assert.Equal(t, "hermes.example.com", ing.Spec.Rules[0].Host)
	assert.Equal(t, "true", ing.Annotations["nginx.ingress.kubernetes.io/ssl-redirect"])
}

func TestBuildIngress_TraefikAnnotations(t *testing.T) {
	t.Parallel()
	inst := &hermesv1.HermesInstance{
		ObjectMeta: metav1.ObjectMeta{Name: "demo"},
		Spec: hermesv1.HermesInstanceSpec{
			Networking: hermesv1.NetworkingSpec{
				Ingress: hermesv1.IngressSpec{
					Enabled:         Ptr(true),
					Host:            "hermes.example.com",
					ClassName:       Ptr("traefik"),
					ServicePortName: "gateway",
				},
			},
		},
	}
	ing := BuildIngress(inst)
	assert.NotEmpty(t, ing.Annotations["traefik.ingress.kubernetes.io/router.entrypoints"])
}

func TestBuildIngress_UserAnnotationsWin(t *testing.T) {
	t.Parallel()
	inst := &hermesv1.HermesInstance{
		ObjectMeta: metav1.ObjectMeta{Name: "demo"},
		Spec: hermesv1.HermesInstanceSpec{
			Networking: hermesv1.NetworkingSpec{
				Ingress: hermesv1.IngressSpec{
					Enabled:   Ptr(true),
					Host:      "x",
					ClassName: Ptr("nginx"),
					Annotations: map[string]string{
						"nginx.ingress.kubernetes.io/ssl-redirect": "false",
						"team": "platform",
					},
					ServicePortName: "gateway",
				},
			},
		},
	}
	ing := BuildIngress(inst)
	assert.Equal(t, "false", ing.Annotations["nginx.ingress.kubernetes.io/ssl-redirect"])
	assert.Equal(t, "platform", ing.Annotations["team"])
}

func TestBuildIngress_TLS(t *testing.T) {
	t.Parallel()
	inst := &hermesv1.HermesInstance{
		ObjectMeta: metav1.ObjectMeta{Name: "demo"},
		Spec: hermesv1.HermesInstanceSpec{
			Networking: hermesv1.NetworkingSpec{
				Ingress: hermesv1.IngressSpec{
					Enabled:         Ptr(true),
					Host:            "x.example.com",
					TLS:             []hermesv1.IngressTLSSpec{{SecretName: "tls", Hosts: []string{"x.example.com"}}},
					ServicePortName: "gateway",
				},
			},
		},
	}
	ing := BuildIngress(inst)
	assert.Len(t, ing.Spec.TLS, 1)
	assert.Equal(t, "tls", ing.Spec.TLS[0].SecretName)
}

func TestIngressName(t *testing.T) {
	t.Parallel()
	inst := &hermesv1.HermesInstance{ObjectMeta: metav1.ObjectMeta{Name: "demo"}}
	assert.Equal(t, "demo", IngressName(inst))
	_ = metav1.ObjectMeta{}
}

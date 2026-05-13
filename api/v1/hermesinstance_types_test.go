package v1

import (
	"reflect"
	"testing"

	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	intstr "k8s.io/apimachinery/pkg/util/intstr"
)

// Ptr is a local test helper (the resources package's Ptr is not visible here).
func Ptr[T any](v T) *T { return &v }

// TestHermesInstanceSpec_HasAllSubSpecs is the schema canary — every sub-spec
// from design §4 must be addressable on HermesInstanceSpec. Tasks 3-9 fill the
// bodies; this test only guards the shape so the field-tag / json-name choices
// are reviewable in one place.
func TestHermesInstanceSpec_HasAllSubSpecs(t *testing.T) {
	t.Parallel()

	specType := reflect.TypeOf(HermesInstanceSpec{})
	required := []string{
		"Image", "Config", "Workspace", "Resources", "Security", "Storage",
		"Networking", "Observability", "Availability", "Probes",
		"Scheduling", "InitContainers", "Sidecars", "ExtraVolumes",
		"ExtraVolumeMounts", "EnvFrom", "Env", "Skills",
		"SelfConfigure", "Suspended",
	}
	for _, name := range required {
		_, ok := specType.FieldByName(name)
		assert.Truef(t, ok, "HermesInstanceSpec is missing field %q (design §4)", name)
	}
}

func TestConfigSpec_RawAndRef(t *testing.T) {
	t.Parallel()
	cs := ConfigSpec{
		Raw:          &RawConfig{RawExtension: runtime.RawExtension{Raw: []byte(`{"a":1}`)}},
		ConfigMapRef: &corev1.LocalObjectReference{Name: "user-config"},
		MergeMode:    ConfigMergeModeMerge,
	}
	assert.NotNil(t, cs.Raw)
	assert.NotNil(t, cs.ConfigMapRef)
	assert.Equal(t, ConfigMergeModeMerge, cs.MergeMode)
}

func TestWorkspaceSpec_NestedPath(t *testing.T) {
	t.Parallel()
	ws := WorkspaceSpec{
		InitialFiles: []WorkspaceFile{
			{Path: "notes/finance/2026.md", Content: "hi"},
			{Path: "shallow.txt", Content: "ok"},
		},
		InitialDirs:  []string{"data", "data/raw"},
		ConfigMapRef: &corev1.LocalObjectReference{Name: "user-ws"},
		Bootstrap:    WorkspaceBootstrap{Enabled: Ptr(false)},
	}
	assert.Len(t, ws.InitialFiles, 2)
	assert.Equal(t, "notes/finance/2026.md", ws.InitialFiles[0].Path)
	assert.NotNil(t, ws.Bootstrap.Enabled)
	assert.False(t, *ws.Bootstrap.Enabled)
}

func TestResourcesSpec_RequestsLimits(t *testing.T) {
	t.Parallel()
	rs := ResourcesSpec{
		Requests: corev1.ResourceList{
			corev1.ResourceCPU:    resource.MustParse("100m"),
			corev1.ResourceMemory: resource.MustParse("256Mi"),
		},
		Limits: corev1.ResourceList{
			corev1.ResourceCPU:    resource.MustParse("500m"),
			corev1.ResourceMemory: resource.MustParse("512Mi"),
		},
	}
	assert.Equal(t, resource.MustParse("100m"), rs.Requests[corev1.ResourceCPU])
	assert.Equal(t, resource.MustParse("512Mi"), rs.Limits[corev1.ResourceMemory])
}

func TestSecuritySpec_Shape(t *testing.T) {
	t.Parallel()
	ss := SecuritySpec{
		PodSecurityContext:       &corev1.PodSecurityContext{RunAsNonRoot: Ptr(true)},
		ContainerSecurityContext: &corev1.SecurityContext{ReadOnlyRootFilesystem: Ptr(true)},
		RBAC: RBACSpec{
			CreateServiceAccount: Ptr(true),
			ServiceAccountName:   "",
			Annotations: map[string]string{
				"eks.amazonaws.com/role-arn": "arn:aws:iam::1:role/hermes",
			},
		},
		NetworkPolicy: NetworkPolicySpec{
			Enabled:                  Ptr(true),
			AllowDNS:                 Ptr(true),
			AllowedIngressNamespaces: []string{"prometheus"},
			AllowedIngressCIDRs:      []string{"10.0.0.0/8"},
			AllowedEgressCIDRs:       []string{"203.0.113.0/24"},
		},
		CABundle: CABundleSpec{ConfigMapName: "corp-ca", Key: "ca.crt"},
	}
	assert.True(t, *ss.PodSecurityContext.RunAsNonRoot)
	assert.True(t, *ss.RBAC.CreateServiceAccount)
	assert.True(t, *ss.NetworkPolicy.Enabled)
	assert.Equal(t, "corp-ca", ss.CABundle.ConfigMapName)
}

func TestNetworkingSpec_ServiceAndIngress(t *testing.T) {
	t.Parallel()
	port := int32(8443)
	ns := NetworkingSpec{
		Service: ServiceSpec{
			Type: corev1.ServiceTypeClusterIP,
			Ports: []NamedServicePort{
				{Name: "gateway", Port: 8443, TargetPort: &port, Protocol: corev1.ProtocolTCP},
			},
		},
		Ingress: IngressSpec{
			Enabled:     Ptr(true),
			Host:        "hermes.example.com",
			ClassName:   Ptr("nginx"),
			TLS:         []IngressTLSSpec{{SecretName: "hermes-tls", Hosts: []string{"hermes.example.com"}}},
			Annotations: map[string]string{"foo": "bar"},
		},
	}
	assert.Equal(t, corev1.ServiceTypeClusterIP, ns.Service.Type)
	assert.Len(t, ns.Service.Ports, 1)
	assert.True(t, *ns.Ingress.Enabled)
	assert.Equal(t, "hermes.example.com", ns.Ingress.Host)
}

func TestObservabilitySpec_Shape(t *testing.T) {
	t.Parallel()
	o := ObservabilitySpec{
		Metrics: MetricsSpec{
			Enabled: Ptr(true), Port: 9090, Secure: Ptr(false),
		},
		ServiceMonitor: ServiceMonitorSpec{
			Enabled:       Ptr(true),
			Labels:        map[string]string{"team": "platform"},
			Interval:      "30s",
			ScrapeTimeout: "10s",
		},
		PrometheusRule: PrometheusRuleSpec{Enabled: Ptr(true)},
		Logging:        LoggingSpec{Format: LogFormatJSON, Level: "info"},
	}
	assert.True(t, *o.Metrics.Enabled)
	assert.Equal(t, int32(9090), o.Metrics.Port)
	assert.Equal(t, LogFormatJSON, o.Logging.Format)
}

func TestAvailabilitySpec_Shape(t *testing.T) {
	t.Parallel()
	pdbMin := intstr.FromString("50%")
	pdbMax := intstr.FromInt(1)
	a := AvailabilitySpec{
		PodDisruptionBudget: PDBSpec{Enabled: Ptr(true), MinAvailable: &pdbMin, MaxUnavailable: &pdbMax},
		HorizontalPodAutoscaler: HPASpec{Enabled: Ptr(true), MinReplicas: Ptr(int32(2)), MaxReplicas: Ptr(int32(5)),
			TargetCPUUtilization: Ptr(int32(70))},
		TopologySpreadConstraints: []corev1.TopologySpreadConstraint{
			{TopologyKey: "topology.kubernetes.io/zone", WhenUnsatisfiable: corev1.ScheduleAnyway, MaxSkew: 1,
				LabelSelector: &metav1.LabelSelector{MatchLabels: map[string]string{"app": "x"}}},
		},
	}
	assert.True(t, *a.PodDisruptionBudget.Enabled)
	assert.Equal(t, "50%", a.PodDisruptionBudget.MinAvailable.StrVal)
	assert.Equal(t, int32(70), *a.HorizontalPodAutoscaler.TargetCPUUtilization)
}

func TestProbesSpec_Overrides(t *testing.T) {
	t.Parallel()
	p := ProbesSpec{
		Liveness:  &corev1.Probe{InitialDelaySeconds: 10},
		Readiness: &corev1.Probe{InitialDelaySeconds: 5},
		Startup:   &corev1.Probe{InitialDelaySeconds: 0, PeriodSeconds: 2, FailureThreshold: 30},
	}
	assert.Equal(t, int32(10), p.Liveness.InitialDelaySeconds)
}

func TestSchedulingSpec_Shape(t *testing.T) {
	t.Parallel()
	s := SchedulingSpec{
		NodeSelector:      map[string]string{"disktype": "ssd"},
		Tolerations:       []corev1.Toleration{{Key: "gpu", Operator: corev1.TolerationOpExists}},
		PriorityClassName: "high-prio",
	}
	assert.Equal(t, "ssd", s.NodeSelector["disktype"])
	assert.Equal(t, "high-prio", s.PriorityClassName)
}

func TestSelfConfigureSpec_AllowList(t *testing.T) {
	t.Parallel()
	sc := SelfConfigureSpec{
		Enabled:        Ptr(true),
		AllowedActions: []SelfConfigAction{ActionSkills, ActionEnvVars},
		ProtectedKeys:  []string{"spec.image.repository"},
	}
	assert.True(t, *sc.Enabled)
	assert.Len(t, sc.AllowedActions, 2)
}

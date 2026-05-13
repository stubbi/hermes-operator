package resources

import (
	"strings"

	networkingv1 "k8s.io/api/networking/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	hermesv1 "github.com/stubbi/hermes-operator/api/v1"
)

// IngressProvider is the detected ingress controller flavour.
type IngressProvider string

const (
	IngressProviderNginx   IngressProvider = "nginx"
	IngressProviderTraefik IngressProvider = "traefik"
	IngressProviderUnknown IngressProvider = "unknown"
)

// IngressName returns the deterministic Ingress name.
func IngressName(inst *hermesv1.HermesInstance) string {
	return inst.Name
}

// DetectIngressProvider classifies the className by substring match.
func DetectIngressProvider(className *string) IngressProvider {
	if className == nil {
		return IngressProviderUnknown
	}
	lower := strings.ToLower(*className)
	switch {
	case strings.Contains(lower, "nginx"):
		return IngressProviderNginx
	case strings.Contains(lower, "traefik"):
		return IngressProviderTraefik
	default:
		return IngressProviderUnknown
	}
}

// BuildIngress constructs the desired Ingress. User annotations always win on
// key conflict with operator-supplied defaults.
func BuildIngress(inst *hermesv1.HermesInstance) *networkingv1.Ingress {
	ing := inst.Spec.Networking.Ingress
	annotations := buildIngressAnnotations(inst)
	pathType := ing.PathType
	if pathType == "" {
		pathType = networkingv1.PathTypePrefix
	}
	path := ing.Path
	if path == "" {
		path = "/"
	}
	portName := ing.ServicePortName
	if portName == "" {
		portName = GatewayPortName
	}

	rules := []networkingv1.IngressRule{}
	if ing.Host != "" {
		rules = append(rules, networkingv1.IngressRule{
			Host: ing.Host,
			IngressRuleValue: networkingv1.IngressRuleValue{
				HTTP: &networkingv1.HTTPIngressRuleValue{
					Paths: []networkingv1.HTTPIngressPath{
						{
							Path:     path,
							PathType: Ptr(pathType),
							Backend: networkingv1.IngressBackend{
								Service: &networkingv1.IngressServiceBackend{
									Name: ServiceName(inst),
									Port: networkingv1.ServiceBackendPort{Name: portName},
								},
							},
						},
					},
				},
			},
		})
	}

	tls := []networkingv1.IngressTLS{}
	for _, t := range ing.TLS {
		tls = append(tls, networkingv1.IngressTLS{SecretName: t.SecretName, Hosts: t.Hosts})
	}

	return &networkingv1.Ingress{
		ObjectMeta: metav1.ObjectMeta{
			Name:        IngressName(inst),
			Namespace:   inst.Namespace,
			Labels:      LabelsForInstance(inst),
			Annotations: annotations,
		},
		Spec: networkingv1.IngressSpec{
			IngressClassName: ing.ClassName,
			Rules:            rules,
			TLS:              tls,
		},
	}
}

func buildIngressAnnotations(inst *hermesv1.HermesInstance) map[string]string {
	annotations := map[string]string{}
	provider := DetectIngressProvider(inst.Spec.Networking.Ingress.ClassName)
	switch provider {
	case IngressProviderNginx:
		annotations["nginx.ingress.kubernetes.io/ssl-redirect"] = "true"
		annotations["nginx.ingress.kubernetes.io/force-ssl-redirect"] = "true"
		annotations["nginx.ingress.kubernetes.io/proxy-body-size"] = "32m"
	case IngressProviderTraefik:
		annotations["traefik.ingress.kubernetes.io/router.entrypoints"] = "websecure"
		annotations["traefik.ingress.kubernetes.io/router.tls"] = "true"
	}
	for k, v := range inst.Spec.Networking.Ingress.Annotations {
		annotations[k] = v
	}
	return annotations
}

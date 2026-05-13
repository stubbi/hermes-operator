package resources

import (
	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"

	hermesv1 "github.com/stubbi/hermes-operator/api/v1"
)

// NetworkPolicyName returns the deterministic NetworkPolicy name.
func NetworkPolicyName(inst *hermesv1.HermesInstance) string {
	return inst.Name
}

// BuildNetworkPolicy returns a default-deny baseline plus selective allow rules.
func BuildNetworkPolicy(inst *hermesv1.HermesInstance) *networkingv1.NetworkPolicy {
	return &networkingv1.NetworkPolicy{
		ObjectMeta: metav1.ObjectMeta{
			Name:      NetworkPolicyName(inst),
			Namespace: inst.Namespace,
			Labels:    LabelsForInstance(inst),
		},
		Spec: networkingv1.NetworkPolicySpec{
			PodSelector: metav1.LabelSelector{MatchLabels: SelectorLabels(inst)},
			PolicyTypes: []networkingv1.PolicyType{
				networkingv1.PolicyTypeIngress,
				networkingv1.PolicyTypeEgress,
			},
			Ingress: buildIngressRules(inst),
			Egress:  buildEgressRules(inst),
		},
	}
}

func networkPolicyIngressPorts(inst *hermesv1.HermesInstance) []networkingv1.NetworkPolicyPort {
	if len(inst.Spec.Networking.Service.Ports) > 0 {
		ports := make([]networkingv1.NetworkPolicyPort, 0, len(inst.Spec.Networking.Service.Ports))
		for _, p := range inst.Spec.Networking.Service.Ports {
			protocol := p.Protocol
			if protocol == "" {
				protocol = corev1.ProtocolTCP
			}
			port := p.Port
			if p.TargetPort != nil {
				port = *p.TargetPort
			}
			ports = append(ports, networkingv1.NetworkPolicyPort{
				Protocol: Ptr(protocol),
				Port:     Ptr(intstr.FromInt32(port)),
			})
		}
		return ports
	}
	return []networkingv1.NetworkPolicyPort{
		{Protocol: Ptr(corev1.ProtocolTCP), Port: Ptr(intstr.FromInt32(GatewayPort))},
	}
}

func buildIngressRules(inst *hermesv1.HermesInstance) []networkingv1.NetworkPolicyIngressRule {
	rules := []networkingv1.NetworkPolicyIngressRule{}
	ports := networkPolicyIngressPorts(inst)

	rules = append(rules, networkingv1.NetworkPolicyIngressRule{
		From: []networkingv1.NetworkPolicyPeer{
			{
				NamespaceSelector: &metav1.LabelSelector{
					MatchLabels: map[string]string{"kubernetes.io/metadata.name": inst.Namespace},
				},
			},
		},
		Ports: ports,
	})

	for _, ns := range inst.Spec.Security.NetworkPolicy.AllowedIngressNamespaces {
		rules = append(rules, networkingv1.NetworkPolicyIngressRule{
			From: []networkingv1.NetworkPolicyPeer{
				{
					NamespaceSelector: &metav1.LabelSelector{
						MatchLabels: map[string]string{"kubernetes.io/metadata.name": ns},
					},
				},
			},
			Ports: ports,
		})
	}

	for _, cidr := range inst.Spec.Security.NetworkPolicy.AllowedIngressCIDRs {
		rules = append(rules, networkingv1.NetworkPolicyIngressRule{
			From: []networkingv1.NetworkPolicyPeer{
				{IPBlock: &networkingv1.IPBlock{CIDR: cidr}},
			},
			Ports: ports,
		})
	}

	return rules
}

func buildEgressRules(inst *hermesv1.HermesInstance) []networkingv1.NetworkPolicyEgressRule {
	rules := []networkingv1.NetworkPolicyEgressRule{}

	allowDNS := BoolValueOrDefault(inst.Spec.Security.NetworkPolicy.AllowDNS, true)
	if allowDNS {
		rules = append(rules, networkingv1.NetworkPolicyEgressRule{
			To: []networkingv1.NetworkPolicyPeer{},
			Ports: []networkingv1.NetworkPolicyPort{
				{Protocol: Ptr(corev1.ProtocolUDP), Port: Ptr(intstr.FromInt(53))},
				{Protocol: Ptr(corev1.ProtocolTCP), Port: Ptr(intstr.FromInt(53))},
			},
		})
	}

	rules = append(rules, networkingv1.NetworkPolicyEgressRule{
		To: []networkingv1.NetworkPolicyPeer{},
		Ports: []networkingv1.NetworkPolicyPort{
			{Protocol: Ptr(corev1.ProtocolTCP), Port: Ptr(intstr.FromInt(443))},
		},
	})

	for _, cidr := range inst.Spec.Security.NetworkPolicy.AllowedEgressCIDRs {
		rules = append(rules, networkingv1.NetworkPolicyEgressRule{
			To: []networkingv1.NetworkPolicyPeer{{IPBlock: &networkingv1.IPBlock{CIDR: cidr}}},
		})
	}

	rules = append(rules, inst.Spec.Security.NetworkPolicy.AdditionalEgress...)

	rules = append(rules, ExtraEgressRules(inst)...)
	return rules
}

// ExtraEgressRules returns the per-instance egress rules driven by spec.gateways
// and spec.profileStore. Plan 2's base default-deny baseline opens DNS + TCP/443
// already; these rules add (1) explicit per-gateway endpoints (still TCP/443
// but documented per gateway) and (2) egress to the Honcho sibling pod on
// TCP/8000 when ProfileStore.Honcho is enabled.
func ExtraEgressRules(inst *hermesv1.HermesInstance) []networkingv1.NetworkPolicyEgressRule {
	var rules []networkingv1.NetworkPolicyEgressRule

	if endpoints := BuildGatewayEgressEndpoints(inst); len(endpoints) > 0 {
		port443 := intstr.FromInt(443)
		tcp := corev1.ProtocolTCP
		rules = append(rules, networkingv1.NetworkPolicyEgressRule{
			// No `To` => all destinations. Hostname-level allow-listing is a
			// CNI-implementation concern; see docs/conventions.md.
			Ports: []networkingv1.NetworkPolicyPort{{Protocol: &tcp, Port: &port443}},
		})
	}

	if honchoEnabled(inst) {
		port8000 := intstr.FromInt(8000)
		tcp := corev1.ProtocolTCP
		rules = append(rules, networkingv1.NetworkPolicyEgressRule{
			To: []networkingv1.NetworkPolicyPeer{{
				PodSelector: &metav1.LabelSelector{MatchLabels: map[string]string{
					"app.kubernetes.io/name":     "honcho",
					"app.kubernetes.io/instance": HonchoDeploymentName(inst),
				}},
			}},
			Ports: []networkingv1.NetworkPolicyPort{{Protocol: &tcp, Port: &port8000}},
		})
	}

	return rules
}

// BuildHonchoNetworkPolicy returns the NetworkPolicy that scopes the Honcho
// companion: ingress only from the parent hermes pod, egress denied entirely.
// Returns nil when honcho is not enabled.
func BuildHonchoNetworkPolicy(inst *hermesv1.HermesInstance) *networkingv1.NetworkPolicy {
	if !honchoEnabled(inst) {
		return nil
	}
	port8000 := intstr.FromInt(8000)
	tcp := corev1.ProtocolTCP
	return &networkingv1.NetworkPolicy{
		ObjectMeta: metav1.ObjectMeta{
			Name:      HonchoDeploymentName(inst),
			Namespace: inst.Namespace,
			Labels:    HonchoLabels(inst),
		},
		Spec: networkingv1.NetworkPolicySpec{
			PodSelector: metav1.LabelSelector{MatchLabels: map[string]string{
				"app.kubernetes.io/name":     "honcho",
				"app.kubernetes.io/instance": HonchoDeploymentName(inst),
			}},
			PolicyTypes: []networkingv1.PolicyType{
				networkingv1.PolicyTypeIngress,
				networkingv1.PolicyTypeEgress,
			},
			Ingress: []networkingv1.NetworkPolicyIngressRule{{
				From: []networkingv1.NetworkPolicyPeer{{
					PodSelector: &metav1.LabelSelector{MatchLabels: map[string]string{
						"app.kubernetes.io/name":     "hermes-agent",
						"app.kubernetes.io/instance": inst.Name,
					}},
				}},
				Ports: []networkingv1.NetworkPolicyPort{{Protocol: &tcp, Port: &port8000}},
			}},
		},
	}
}

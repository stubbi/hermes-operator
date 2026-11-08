/*
Copyright 2026 Paperclip Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package resources

import (
	"testing"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	hermesv1 "github.com/paperclipinc/hermes-operator/api/v1"
)

// fullBenchInstance creates a fully-loaded instance for benchmarking so the
// builders exercise their richer code paths (scheduling, network policy,
// resources, etc.) rather than only the empty defaults.
func fullBenchInstance() *hermesv1.HermesInstance {
	return &hermesv1.HermesInstance{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "bench-full",
			Namespace: "bench-ns",
		},
		Spec: hermesv1.HermesInstanceSpec{
			Image: hermesv1.ImageSpec{
				Repository: "ghcr.io/stubbi/hermes-agent",
				Tag:        "v1.0.0",
				PullPolicy: "IfNotPresent",
			},
			Resources: hermesv1.ResourcesSpec{
				Requests: corev1.ResourceList{
					corev1.ResourceCPU:    resource.MustParse("500m"),
					corev1.ResourceMemory: resource.MustParse("512Mi"),
				},
				Limits: corev1.ResourceList{
					corev1.ResourceCPU:    resource.MustParse("2"),
					corev1.ResourceMemory: resource.MustParse("2Gi"),
				},
			},
			Scheduling: hermesv1.SchedulingSpec{
				NodeSelector: map[string]string{"node-type": "agents"},
				Tolerations: []corev1.Toleration{
					{Key: "agents", Operator: corev1.TolerationOpEqual, Value: "true", Effect: corev1.TaintEffectNoSchedule},
				},
				PriorityClassName: "high",
			},
			Security: hermesv1.SecuritySpec{
				NetworkPolicy: hermesv1.NetworkPolicySpec{
					Enabled:                  Ptr(true),
					AllowDNS:                 Ptr(true),
					AllowedIngressNamespaces: []string{"monitoring", "ingress-nginx"},
					AllowedIngressCIDRs:      []string{"10.0.0.0/8"},
					AllowedEgressCIDRs:       []string{"10.0.0.0/8"},
				},
			},
		},
	}
}

// ---------------------------------------------------------------------------
// StatefulSet benchmarks
// ---------------------------------------------------------------------------

func BenchmarkBuildStatefulSet_Minimal(b *testing.B) {
	inst := minimalInstance()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		BuildStatefulSet(inst, nil)
	}
}

func BenchmarkBuildStatefulSet_FullyLoaded(b *testing.B) {
	inst := fullBenchInstance()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		BuildStatefulSet(inst, nil)
	}
}

// ---------------------------------------------------------------------------
// Service benchmarks
// ---------------------------------------------------------------------------

func BenchmarkBuildService_Minimal(b *testing.B) {
	inst := minimalInstance()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		BuildService(inst)
	}
}

func BenchmarkBuildService_FullyLoaded(b *testing.B) {
	inst := fullBenchInstance()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		BuildService(inst)
	}
}

// ---------------------------------------------------------------------------
// NetworkPolicy benchmarks
// ---------------------------------------------------------------------------

func BenchmarkBuildNetworkPolicy_Minimal(b *testing.B) {
	inst := minimalInstance()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		BuildNetworkPolicy(inst)
	}
}

func BenchmarkBuildNetworkPolicy_FullyLoaded(b *testing.B) {
	inst := fullBenchInstance()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		BuildNetworkPolicy(inst)
	}
}

// ---------------------------------------------------------------------------
// ConfigMap benchmarks
// ---------------------------------------------------------------------------

func BenchmarkBuildConfigMap_Minimal(b *testing.B) {
	inst := minimalInstance()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		BuildConfigMap(inst, "")
	}
}

func BenchmarkBuildConfigMap_WithBody(b *testing.B) {
	inst := fullBenchInstance()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		BuildConfigMap(inst, "key: value\nnested:\n  a: 1\n  b: c\n")
	}
}

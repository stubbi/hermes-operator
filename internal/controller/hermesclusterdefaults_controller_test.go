/*
Copyright 2026 stubbi.

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

package controller

import (
	"context"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"

	hermesv1 "github.com/stubbi/hermes-operator/api/v1"
)

var _ = Describe("HermesClusterDefaults controller", func() {
	AfterEach(func() {
		ctx := context.Background()
		hcd := &hermesv1.HermesClusterDefaults{ObjectMeta: metav1.ObjectMeta{Name: "cluster"}}
		_ = k8sClient.Delete(ctx, hcd)
	})

	It("sets Ready=True on the cluster singleton", func() {
		ctx := context.Background()
		hcd := &hermesv1.HermesClusterDefaults{
			ObjectMeta: metav1.ObjectMeta{Name: "cluster"},
			Spec: hermesv1.HermesClusterDefaultsSpec{
				Image: hermesv1.ImageSpec{Repository: "ghcr.io/stubbi/hermes-agent", Tag: "1.0.0"},
			},
		}
		Expect(k8sClient.Create(ctx, hcd)).To(Succeed())

		Eventually(func(g Gomega) {
			got := &hermesv1.HermesClusterDefaults{}
			g.Expect(k8sClient.Get(ctx, types.NamespacedName{Name: "cluster"}, got)).To(Succeed())
			g.Expect(got.Status.Conditions).ToNot(BeEmpty())
			var ready bool
			for _, c := range got.Status.Conditions {
				if c.Type == "Ready" && c.Status == metav1.ConditionTrue {
					ready = true
				}
			}
			g.Expect(ready).To(BeTrue())
			g.Expect(got.Status.ObservedGeneration).To(Equal(got.Generation))
		}).Within(30 * time.Second).WithPolling(250 * time.Millisecond).Should(Succeed())
	})
})

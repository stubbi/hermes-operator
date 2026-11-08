package webhook

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	hermesv1 "github.com/paperclipinc/hermes-operator/api/v1"
)

func TestSelfConfigValidator_PreventsRebindingAppliedAuthorization(t *testing.T) {
	parent := &hermesv1.HermesInstance{ObjectMeta: metav1.ObjectMeta{Name: "agent", UID: "old-parent"}}
	bound := metav1.NewControllerRef(parent, hermesv1.GroupVersion.WithKind("HermesInstance"))
	for _, scenario := range []string{"change UID", "legacy claim", "preserve", "orphan", "initial bind"} {
		t.Run(scenario, func(t *testing.T) {
			old := &hermesv1.HermesSelfConfig{
				ObjectMeta: metav1.ObjectMeta{Generation: 1, OwnerReferences: []metav1.OwnerReference{*bound}},
				Spec:       hermesv1.HermesSelfConfigSpec{InstanceRef: parent.Name},
				Status:     hermesv1.HermesSelfConfigStatus{Phase: hermesv1.SelfConfigPhaseApplied, ObservedGeneration: 1},
			}
			updated := old.DeepCopy()
			switch scenario {
			case "change UID":
				updated.OwnerReferences[0].UID = "replacement-parent"
			case "legacy claim":
				old.OwnerReferences = nil
			case "orphan":
				updated.OwnerReferences = nil
			case "initial bind":
				old.OwnerReferences = nil
				old.Status = hermesv1.HermesSelfConfigStatus{}
			}
			_, err := (&HermesSelfConfigValidator{}).ValidateUpdate(context.Background(), old, updated)
			if scenario == "change UID" || scenario == "legacy claim" {
				assert.Error(t, err, "an Applied status must not authorize a different parent UID")
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

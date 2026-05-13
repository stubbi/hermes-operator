package resources

import (
	policyv1 "k8s.io/api/policy/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"

	hermesv1 "github.com/stubbi/hermes-operator/api/v1"
)

// PDBName returns the deterministic PDB name.
func PDBName(inst *hermesv1.HermesInstance) string {
	return inst.Name
}

// BuildPDB constructs the desired PodDisruptionBudget. When both MinAvailable
// and MaxUnavailable are set, MinAvailable wins (k8s forbids both — the
// validating webhook rejects the spec). When neither is set, MaxUnavailable=1.
func BuildPDB(inst *hermesv1.HermesInstance) *policyv1.PodDisruptionBudget {
	spec := inst.Spec.Availability.PodDisruptionBudget

	pdb := &policyv1.PodDisruptionBudget{
		ObjectMeta: metav1.ObjectMeta{
			Name:      PDBName(inst),
			Namespace: inst.Namespace,
			Labels:    LabelsForInstance(inst),
		},
		Spec: policyv1.PodDisruptionBudgetSpec{
			Selector: &metav1.LabelSelector{MatchLabels: SelectorLabels(inst)},
		},
	}

	switch {
	case spec.MinAvailable != nil:
		pdb.Spec.MinAvailable = spec.MinAvailable
	case spec.MaxUnavailable != nil:
		pdb.Spec.MaxUnavailable = spec.MaxUnavailable
	default:
		def := intstr.FromInt(1)
		pdb.Spec.MaxUnavailable = &def
	}
	return pdb
}

package webhook

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	hermesv1 "github.com/stubbi/hermes-operator/api/v1"
)

func TestHCDValidator_AllowSingleton(t *testing.T) {
	t.Parallel()
	v := &HermesClusterDefaultsValidator{}
	hcd := &hermesv1.HermesClusterDefaults{ObjectMeta: metav1.ObjectMeta{Name: "cluster"}}
	_, err := v.ValidateCreate(context.Background(), hcd)
	assert.NoError(t, err)
}

func TestHCDValidator_DenyOtherNames(t *testing.T) {
	t.Parallel()
	v := &HermesClusterDefaultsValidator{}
	for _, n := range []string{"default", "foo", "Cluster", "CLUSTER"} {
		hcd := &hermesv1.HermesClusterDefaults{ObjectMeta: metav1.ObjectMeta{Name: n}}
		_, err := v.ValidateCreate(context.Background(), hcd)
		assert.Errorf(t, err, "expected reject for name %q", n)
	}
}

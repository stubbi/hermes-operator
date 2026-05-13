package webhook

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	hermesv1 "github.com/stubbi/hermes-operator/api/v1"
)

func TestSelfConfigValidator_RejectsEmptyInstanceRef(t *testing.T) {
	t.Parallel()
	// Client is nil: skips parent lookup, but instanceRef="" is still rejected.
	v := &HermesSelfConfigValidator{}
	sc := &hermesv1.HermesSelfConfig{ObjectMeta: metav1.ObjectMeta{Name: "demo"}}
	_, err := v.ValidateCreate(context.Background(), sc)
	assert.Error(t, err, "empty instanceRef must be rejected by the real validator")
}

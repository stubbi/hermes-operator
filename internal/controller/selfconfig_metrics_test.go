package controller

import (
	"strings"
	"testing"

	"github.com/prometheus/client_golang/prometheus/testutil"
	"github.com/stretchr/testify/assert"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	hermesv1 "github.com/stubbi/hermes-operator/api/v1"
)

func TestIncSelfConfigApplied(t *testing.T) {
	selfConfigAppliedTotal.Reset()
	parent := &hermesv1.HermesInstance{ObjectMeta: metav1.ObjectMeta{Name: "x", Namespace: "y"}}
	incSelfConfigApplied(parent, "envVars")
	incSelfConfigApplied(parent, "envVars")
	incSelfConfigApplied(parent, "skills")

	want := `
		# HELP hermes_selfconfig_applied_total Count of HermesSelfConfig requests successfully applied.
		# TYPE hermes_selfconfig_applied_total counter
		hermes_selfconfig_applied_total{action="envVars",instance="x",namespace="y"} 2
		hermes_selfconfig_applied_total{action="skills",instance="x",namespace="y"} 1
	`
	assert.NoError(t, testutil.CollectAndCompare(selfConfigAppliedTotal, strings.NewReader(want)))
}

func TestIncSelfConfigDenied(t *testing.T) {
	selfConfigDeniedTotal.Reset()
	parent := &hermesv1.HermesInstance{ObjectMeta: metav1.ObjectMeta{Name: "x", Namespace: "y"}}
	incSelfConfigDenied(parent, "selfconfig disabled on parent")
	incSelfConfigDenied(parent, "selfconfig disabled on parent")
	want := `
		# HELP hermes_selfconfig_denied_total Count of HermesSelfConfig requests denied by policy or validation.
		# TYPE hermes_selfconfig_denied_total counter
		hermes_selfconfig_denied_total{instance="x",namespace="y",reason="selfconfig disabled on parent"} 2
	`
	assert.NoError(t, testutil.CollectAndCompare(selfConfigDeniedTotal, strings.NewReader(want)))
}

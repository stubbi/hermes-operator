/*
Copyright 2026 stubbi. Apache-2.0.
*/

package controller

import (
	"github.com/prometheus/client_golang/prometheus"
	"sigs.k8s.io/controller-runtime/pkg/metrics"

	hermesv1 "github.com/stubbi/hermes-operator/api/v1"
)

var (
	selfConfigAppliedTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "hermes_selfconfig_applied_total",
			Help: "Count of HermesSelfConfig requests successfully applied.",
		},
		[]string{"namespace", "instance", "action"},
	)
	selfConfigDeniedTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "hermes_selfconfig_denied_total",
			Help: "Count of HermesSelfConfig requests denied by policy or validation.",
		},
		[]string{"namespace", "instance", "reason"},
	)
)

func init() {
	metrics.Registry.MustRegister(selfConfigAppliedTotal, selfConfigDeniedTotal)
}

func incSelfConfigApplied(parent *hermesv1.HermesInstance, action string) {
	if parent == nil {
		return
	}
	selfConfigAppliedTotal.WithLabelValues(parent.Namespace, parent.Name, action).Inc()
}

func incSelfConfigDenied(parent *hermesv1.HermesInstance, reason string) {
	ns, name := "", ""
	if parent != nil {
		ns, name = parent.Namespace, parent.Name
	}
	selfConfigDeniedTotal.WithLabelValues(ns, name, reason).Inc()
}

/*
Copyright 2026 stubbi. Apache-2.0.
*/

package controller

import (
	corev1 "k8s.io/api/core/v1"
	"k8s.io/client-go/tools/record"

	hermesv1 "github.com/stubbi/hermes-operator/api/v1"
)

const (
	EventReasonSelfConfigApplied = "SelfConfigApplied"
	EventReasonSelfConfigDenied  = "SelfConfigDenied"
)

// emitSelfConfigEvent fires a paired event on the SelfConfig (and on the
// parent HermesInstance when present). When the recorder is nil — e.g. unit
// tests that don't wire one — the call is a no-op.
func emitSelfConfigEvent(
	r record.EventRecorder,
	sc *hermesv1.HermesSelfConfig,
	parent *hermesv1.HermesInstance,
	eventType, reason, message string,
) {
	if r == nil {
		return
	}
	r.Event(sc, eventType, reason, message)
	if parent != nil && parent.Name != "" {
		r.Event(parent, eventType, reason, "selfconfig "+sc.Name+": "+message)
	}
	_ = corev1.EventTypeNormal
}

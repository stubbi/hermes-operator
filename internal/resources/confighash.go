/*
Copyright 2026 Paperclip.inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0
*/

package resources

import (
	"crypto/sha256"
	"encoding/hex"
	"sort"

	appsv1 "k8s.io/api/apps/v1"
)

// Pod template annotations carrying a digest of the ConfigMaps the agent
// reads only at startup. config.yaml is subPath-mounted, so kubelet never
// refreshes it in a running pod, and the workspace seed is copied once by
// the agent's init. Changing the digest changes the pod template, which
// makes the StatefulSet roll the pod so the new content takes effect.
const (
	ConfigHashAnnotation    = "hermes.agent/config-hash"
	WorkspaceHashAnnotation = "hermes.agent/workspace-hash"
)

// HashConfigMapData returns a deterministic sha256 hex digest of a
// ConfigMap's Data, independent of map iteration order. Nil and empty maps
// hash identically.
func HashConfigMapData(data map[string]string) string {
	keys := make([]string, 0, len(data))
	for k := range data {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	h := sha256.New()
	for _, k := range keys {
		// Length-prefix both halves so "a"+"bc" and "ab"+"c" cannot collide.
		writeLenPrefixed(h, k)
		writeLenPrefixed(h, data[k])
	}
	return hex.EncodeToString(h.Sum(nil))
}

func writeLenPrefixed(h interface{ Write([]byte) (int, error) }, s string) {
	var buf [8]byte
	n := uint64(len(s))
	for i := 7; i >= 0; i-- {
		buf[i] = byte(n)
		n >>= 8
	}
	_, _ = h.Write(buf[:])
	_, _ = h.Write([]byte(s))
}

// SetConfigHashAnnotations stamps the pod template of sts with digests of
// the rendered config and workspace ConfigMap data. Existing template
// annotations are preserved.
func SetConfigHashAnnotations(sts *appsv1.StatefulSet, configData, workspaceData map[string]string) {
	if sts.Spec.Template.Annotations == nil {
		sts.Spec.Template.Annotations = map[string]string{}
	}
	sts.Spec.Template.Annotations[ConfigHashAnnotation] = HashConfigMapData(configData)
	sts.Spec.Template.Annotations[WorkspaceHashAnnotation] = HashConfigMapData(workspaceData)
}

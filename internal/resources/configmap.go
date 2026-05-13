package resources

import (
	"fmt"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/yaml"

	hermesv1 "github.com/stubbi/hermes-operator/api/v1"
)

// ConfigMapName returns the deterministic ConfigMap name for the rendered config.
func ConfigMapName(inst *hermesv1.HermesInstance) string {
	return inst.Name + "-config"
}

// BuildConfigMap returns the desired ConfigMap holding ~/.hermes/config.yaml.
//
// `resolvedBody` is the body the reconciler has already resolved for the case
// where spec.config.configMapRef is set. The builder is pure: it does not
// reach out to the apiserver.
//
//   - Empty resolvedBody + Raw set         → use Raw verbatim (YAML-serialised).
//   - Empty resolvedBody + Raw unset       → emit "{}\n".
//   - resolvedBody non-empty + Raw unset   → use resolvedBody verbatim.
//   - resolvedBody non-empty + Raw set     → caller is responsible for merging
//     (use MergeYAMLBodies) and passing the merged result as resolvedBody.
func BuildConfigMap(inst *hermesv1.HermesInstance, resolvedBody string) *corev1.ConfigMap {
	body := "{}\n"
	switch {
	case resolvedBody != "":
		body = resolvedBody
	case inst.Spec.Config.Raw != nil && len(inst.Spec.Config.Raw.Raw) > 0:
		y, err := yaml.JSONToYAML(inst.Spec.Config.Raw.Raw)
		if err == nil {
			body = string(y)
		}
	}
	if merged, err := mergeGatewayFragments(body, BuildGatewayConfigFragments(inst)); err == nil {
		body = merged
	}
	// On parse error we keep the original body: the validating webhook is
	// responsible for rejecting malformed config; we don't want a pure builder
	// to panic.
	return &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      ConfigMapName(inst),
			Namespace: inst.Namespace,
			Labels:    LabelsForInstance(inst),
		},
		Data: map[string]string{
			"config.yaml": body,
		},
	}
}

// mergeGatewayFragments deep-merges builder-derived gateway config fragments
// under the top-level `gateways` key of the rendered config.yaml. Gateway
// fragments win over any user-provided `gateways:` entries because the operator
// owns this sub-tree (users should disable the gateway and write their own if
// they need full control).
func mergeGatewayFragments(body string, frags map[string]any) (string, error) {
	if len(frags) == 0 {
		return body, nil
	}
	root := map[string]any{}
	if body != "" {
		if err := yaml.Unmarshal([]byte(body), &root); err != nil {
			return "", fmt.Errorf("parse rendered config: %w", err)
		}
	}
	existing, _ := root["gateways"].(map[string]any)
	if existing == nil {
		existing = map[string]any{}
	}
	for k, v := range frags {
		existing[k] = v
	}
	root["gateways"] = existing
	out, err := yaml.Marshal(root)
	if err != nil {
		return "", fmt.Errorf("marshal merged config: %w", err)
	}
	return string(out), nil
}

// MergeYAMLBodies performs a YAML deep-merge of `overlay` (JSON or YAML) onto
// `base` (YAML). Overlay wins on conflict. Used when spec.config.mergeMode=merge.
func MergeYAMLBodies(base, overlay string) (string, error) {
	baseMap := map[string]any{}
	if base != "" {
		if err := yaml.Unmarshal([]byte(base), &baseMap); err != nil {
			return "", fmt.Errorf("parse base YAML: %w", err)
		}
	}
	overlayMap := map[string]any{}
	if overlay != "" {
		if err := yaml.Unmarshal([]byte(overlay), &overlayMap); err != nil {
			return "", fmt.Errorf("parse overlay: %w", err)
		}
	}
	merged := deepMergeMaps(baseMap, overlayMap)
	out, err := yaml.Marshal(merged)
	if err != nil {
		return "", fmt.Errorf("marshal merged: %w", err)
	}
	return string(out), nil
}

func deepMergeMaps(base, overlay map[string]any) map[string]any {
	out := make(map[string]any, len(base)+len(overlay))
	for k, v := range base {
		out[k] = v
	}
	for k, v := range overlay {
		if bv, ok := out[k]; ok {
			bm, bok := bv.(map[string]any)
			vm, vok := v.(map[string]any)
			if bok && vok {
				out[k] = deepMergeMaps(bm, vm)
				continue
			}
		}
		out[k] = v
	}
	return out
}

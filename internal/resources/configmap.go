package resources

import (
	"fmt"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/yaml"

	hermesv1 "github.com/paperclipinc/hermes-operator/api/v1"
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
	return BuildConfigMapWithPatches(inst, resolvedBody, nil)
}

// BuildConfigMapWithPatches is BuildConfigMap with an ordered list of
// HermesSelfConfig patchConfig bodies (JSON merge patches, RFC 7396) layered
// on top of the user config. Patches are deep-merged in order, later ones win
// on conflict, and all of them sit below the operator-owned gateway fragments
// so a patch can never override the operator's own sub-tree. A patch that
// fails to parse is skipped: the HermesSelfConfig webhook and reconciler are
// responsible for rejecting malformed patches before they reach the builder.
func BuildConfigMapWithPatches(inst *hermesv1.HermesInstance, resolvedBody string, patches []string) *corev1.ConfigMap {
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
	for _, p := range patches {
		if merged, err := ApplyJSONMergePatch(body, p); err == nil {
			body = merged
		}
	}
	if merged, err := mergeGatewayFragments(body, BuildGatewayConfigFragments(inst)); err == nil {
		body = merged
	}
	// The upstream `gateway run` refuses to start without an LLM provider
	// configured. Ensure one is present so the instance can reach Ready: a real
	// deployment sets spec.config.raw with a `model:`/provider (and credentials
	// via env-from-secret); when none is given we inject a non-routable placeholder
	// so the gateway + API server come up (and serve /health) without making any
	// live LLM calls. Inference then fails clearly until a real provider is set.
	if withModel, err := ensureModelDefault(body); err == nil {
		body = withModel
	}
	// On parse error we keep the original body: the validating webhook is
	// responsible for rejecting malformed config; we don't want a pure builder
	// to panic.
	cm := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      ConfigMapName(inst),
			Namespace: inst.Namespace,
			Labels:    LabelsForInstance(inst),
		},
		Data: map[string]string{
			"config.yaml": body,
		},
	}
	if tailscaleEnabled(inst) {
		cm.Data[tailscaleServeKey] = BuildTailscaleServeConfig(inst)
	}
	return cm
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

// ensureModelDefault guarantees the rendered config has a top-level `model` so
// `hermes gateway run` can initialise. If the user already set one (directly or
// via spec.config), it is left untouched. Otherwise a non-routable placeholder
// provider is injected: enough for the gateway + API server to start and serve
// /health, but with no reachable upstream, so it never makes a live LLM call.
func ensureModelDefault(body string) (string, error) {
	root := map[string]any{}
	if body != "" {
		if err := yaml.Unmarshal([]byte(body), &root); err != nil {
			return "", fmt.Errorf("parse rendered config: %w", err)
		}
	}
	if _, ok := root["model"]; ok {
		return body, nil
	}
	root["model"] = "gpt-4o-mini"
	root["base_url"] = "http://127.0.0.1:9/v1"
	root["api_key"] = "placeholder-no-live-calls"
	out, err := yaml.Marshal(root)
	if err != nil {
		return "", fmt.Errorf("marshal config with model default: %w", err)
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

// ApplyJSONMergePatch applies `patch` (a JSON merge patch, RFC 7396, in JSON
// or YAML form) onto `base` (YAML). It differs from MergeYAMLBodies in one
// way: a null value in the patch deletes the key from the base, as the RFC
// requires, instead of writing a literal null. Used to layer HermesSelfConfig
// patchConfig bodies onto the rendered config.
func ApplyJSONMergePatch(base, patch string) (string, error) {
	baseMap := map[string]any{}
	if base != "" {
		if err := yaml.Unmarshal([]byte(base), &baseMap); err != nil {
			return "", fmt.Errorf("parse base YAML: %w", err)
		}
	}
	patchMap := map[string]any{}
	if patch != "" {
		if err := yaml.Unmarshal([]byte(patch), &patchMap); err != nil {
			return "", fmt.Errorf("parse patch: %w", err)
		}
	}
	merged := mergePatchMaps(baseMap, patchMap)
	out, err := yaml.Marshal(merged)
	if err != nil {
		return "", fmt.Errorf("marshal merged: %w", err)
	}
	return string(out), nil
}

// mergePatchMaps is deepMergeMaps with RFC 7396 null-deletes-key semantics.
func mergePatchMaps(base, patch map[string]any) map[string]any {
	out := make(map[string]any, len(base)+len(patch))
	for k, v := range base {
		out[k] = v
	}
	for k, v := range patch {
		if v == nil {
			delete(out, k)
			continue
		}
		if pm, pok := v.(map[string]any); pok {
			bm, bok := out[k].(map[string]any)
			if !bok {
				bm = map[string]any{}
			}
			out[k] = mergePatchMaps(bm, pm)
			continue
		}
		out[k] = v
	}
	return out
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

/*
Copyright 2026 stubbi.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package controller

import (
	"encoding/json"
	"fmt"
	"sort"

	"github.com/gobwas/glob"

	hermesv1 "github.com/stubbi/hermes-operator/api/v1"
)

// DetermineActions inspects a HermesSelfConfig and returns the set of action
// categories it requests. Used to compare against the parent's allowlist.
func DetermineActions(sc *hermesv1.HermesSelfConfig) []hermesv1.SelfConfigAction {
	var out []hermesv1.SelfConfigAction
	if len(sc.Spec.AddSkills) > 0 {
		out = append(out, hermesv1.ActionSkills)
	}
	if sc.Spec.PatchConfig != nil && len(sc.Spec.PatchConfig.Raw) > 0 {
		out = append(out, hermesv1.ActionConfig)
	}
	if len(sc.Spec.AddEnvVars) > 0 {
		out = append(out, hermesv1.ActionEnvVars)
	}
	if len(sc.Spec.AddWorkspaceFiles) > 0 {
		out = append(out, hermesv1.ActionWorkspaceFiles)
	}
	if sc.Spec.AddProfileSnapshot != nil {
		out = append(out, hermesv1.ActionProfiles)
	}
	return out
}

// CheckAllowedActions returns the subset of `requested` that is NOT in `allowed`.
// Empty result means everything is permitted.
func CheckAllowedActions(requested, allowed []hermesv1.SelfConfigAction) []hermesv1.SelfConfigAction {
	allowSet := make(map[hermesv1.SelfConfigAction]bool, len(allowed))
	for _, a := range allowed {
		allowSet[a] = true
	}
	var denied []hermesv1.SelfConfigAction
	for _, a := range requested {
		if !allowSet[a] {
			denied = append(denied, a)
		}
	}
	return denied
}

// CheckProtectedPaths walks the JSON merge patch and returns the first dotted
// path that matches any pattern in `protected`. Patterns support glob syntax
// via gobwas/glob with '.' as the path separator.
// Returns ("", nil) if no path matches. Returns "", err on JSON parse failure
// or invalid pattern.
func CheckProtectedPaths(patch []byte, protected []string) (string, error) {
	if len(patch) == 0 || len(protected) == 0 {
		return "", nil
	}
	var tree map[string]interface{}
	if err := json.Unmarshal(patch, &tree); err != nil {
		return "", fmt.Errorf("invalid JSON merge patch: %w", err)
	}

	globs := make([]glob.Glob, 0, len(protected))
	for _, p := range protected {
		g, err := glob.Compile(p, '.')
		if err != nil {
			return "", fmt.Errorf("invalid protectedKeys pattern %q: %w", p, err)
		}
		globs = append(globs, g)
	}

	hit := ""
	walk(tree, "", func(path string) bool {
		for _, g := range globs {
			if g.Match(path) {
				hit = path
				return true
			}
		}
		return false
	})
	return hit, nil
}

// walk depth-first calls fn(path) for every leaf and every interior node.
// Returns early when fn returns true.
func walk(v interface{}, prefix string, fn func(string) bool) bool {
	if prefix != "" && fn(prefix) {
		return true
	}
	if m, ok := v.(map[string]interface{}); ok {
		keys := make([]string, 0, len(m))
		for k := range m {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		for _, k := range keys {
			child := joinPath(prefix, k)
			if walk(m[k], child, fn) {
				return true
			}
		}
	}
	return false
}

func joinPath(prefix, key string) string {
	if prefix == "" {
		return key
	}
	return prefix + "." + key
}

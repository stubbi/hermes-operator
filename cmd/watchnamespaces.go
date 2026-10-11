/*
Copyright 2026 Paperclip.inc.

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

package main

import (
	"sort"
	"strings"

	"sigs.k8s.io/controller-runtime/pkg/cache"
)

// watchNamespaceEnv is the environment variable OLM populates from the
// olm.targetNamespaces annotation. An empty value means all namespaces.
const watchNamespaceEnv = "WATCH_NAMESPACE"

// parseWatchNamespaces splits a comma-separated namespace list, trimming
// blanks and dropping duplicates. An empty result means "all namespaces".
func parseWatchNamespaces(raw string) []string {
	seen := map[string]struct{}{}
	out := []string{}
	for _, ns := range strings.Split(raw, ",") {
		ns = strings.TrimSpace(ns)
		if ns == "" {
			continue
		}
		if _, dup := seen[ns]; dup {
			continue
		}
		seen[ns] = struct{}{}
		out = append(out, ns)
	}
	sort.Strings(out)
	return out
}

// cacheOptions restricts the informer cache for namespaced objects to the
// watched namespaces. Cluster-scoped objects (HermesClusterDefaults) are still
// cached cluster-wide. With no namespaces, the cache watches everything, which
// requires the cluster-wide RBAC the chart renders when watchNamespaces is
// empty. With namespaces set, the per-namespace Roles the chart renders are
// sufficient because the manager never lists or watches outside them.
func cacheOptions(namespaces []string) cache.Options {
	if len(namespaces) == 0 {
		return cache.Options{}
	}
	defaults := make(map[string]cache.Config, len(namespaces))
	for _, ns := range namespaces {
		defaults[ns] = cache.Config{}
	}
	return cache.Options{DefaultNamespaces: defaults}
}

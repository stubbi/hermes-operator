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
	"reflect"
	"testing"
)

func TestParseWatchNamespaces(t *testing.T) {
	cases := map[string][]string{
		"":                       {},
		" , ,":                   {},
		"team-a":                 {"team-a"},
		"team-b, team-a ,team-b": {"team-a", "team-b"},
	}
	for in, want := range cases {
		if got := parseWatchNamespaces(in); !reflect.DeepEqual(got, want) {
			t.Errorf("parseWatchNamespaces(%q) = %v, want %v", in, got, want)
		}
	}
}

func TestCacheOptions(t *testing.T) {
	if got := cacheOptions(nil); got.DefaultNamespaces != nil {
		t.Fatalf("no namespaces must watch all namespaces, got %v", got.DefaultNamespaces)
	}
	got := cacheOptions([]string{"team-a", "team-b"})
	if len(got.DefaultNamespaces) != 2 {
		t.Fatalf("want 2 namespaces, got %v", got.DefaultNamespaces)
	}
	for _, ns := range []string{"team-a", "team-b"} {
		if _, ok := got.DefaultNamespaces[ns]; !ok {
			t.Errorf("namespace %q missing from cache options", ns)
		}
	}
}

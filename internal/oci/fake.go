package oci

import (
	"context"
	"sync"
)

// Fake is an in-memory Registry for tests.
type Fake struct {
	mu        sync.Mutex
	tags      map[string][]string
	CallCount map[string]int
}

func NewFake() *Fake {
	return &Fake{tags: map[string][]string{}, CallCount: map[string]int{}}
}

func (f *Fake) SetTags(repository string, tags []string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.tags[repository] = append([]string(nil), tags...)
}

func (f *Fake) ListTags(_ context.Context, repository string) ([]string, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.CallCount[repository]++
	out := append([]string(nil), f.tags[repository]...)
	return out, nil
}

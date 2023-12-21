package util

import (
	"sync"

	"k8s.io/apimachinery/pkg/runtime/schema"
)

type SafeIntMap struct {
	mu sync.Mutex
	v  map[string]int
}

func NewSafeIntMap() *SafeIntMap {
	return &SafeIntMap{
		v: make(map[string]int),
	}
}

func (s *SafeIntMap) IncrementValueForKey(key string) {
	s.mu.Lock()
	s.v[key]++
	s.mu.Unlock()
}

func (s *SafeIntMap) DecrementValueForKey(key string) {
	s.mu.Lock()
	s.v[key]--
	s.mu.Unlock()
}

func (s *SafeIntMap) Get(key string) int {
	return s.v[key]
}

//***********************************

// a struct to keep track of resources tracked in applied manifest work
type AppliedManifestInfo struct {
	ObjectUIDs []string
	GVRs       []*schema.GroupVersionResource
}

type SafeAppliedManifestMap struct {
	mu sync.Mutex
	v  map[string]AppliedManifestInfo
}

func NewSafeAppliedManifestMap() *SafeAppliedManifestMap {
	return &SafeAppliedManifestMap{
		v: make(map[string]AppliedManifestInfo),
	}
}

func (s *SafeAppliedManifestMap) Set(key string, info AppliedManifestInfo) {
	s.mu.Lock()
	s.v[key] = info
	s.mu.Unlock()
}

func (s *SafeAppliedManifestMap) Delete(key string) {
	s.mu.Lock()
	delete(s.v, key)
	s.mu.Unlock()
}

func (s *SafeAppliedManifestMap) Get(key string) (AppliedManifestInfo, bool) {
	v, ok := s.v[key]
	return v, ok
}

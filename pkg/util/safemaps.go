package util

import (
	"strings"
	"sync"

	"k8s.io/apimachinery/pkg/runtime/schema"
)

type SafeUIDMap struct {
	mu sync.Mutex
	v  map[string]map[string]bool
}

func NewSafeUIDMap() *SafeUIDMap {
	return &SafeUIDMap{
		v: make(map[string]map[string]bool),
	}
}

func (s *SafeUIDMap) AddUID(key, uid string) {
	s.mu.Lock()
	if k := s.v[key]; k == nil {
		s.v[key] = make(map[string]bool)
	}
	s.v[key][uid] = true
	s.mu.Unlock()
}

func (s *SafeUIDMap) DeleteUID(key, uid string) {
	s.mu.Lock()
	if k := s.v[key]; k != nil {
		delete(s.v[key], uid)
	}
	s.mu.Unlock()
}

func (s *SafeUIDMap) GetUIDCount(key string) int {
	if k := s.v[key]; k == nil {
		return 0
	}
	return len(s.v[key])
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

func HasPrefixInMap(m map[string]string, prefix string) bool {
	for key := range m {
		if strings.HasPrefix(key, prefix) {
			return true
		}
	}
	return false
}

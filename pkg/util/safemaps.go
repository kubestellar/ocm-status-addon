package util

import (
	"strings"
	"sync"

	"k8s.io/apimachinery/pkg/runtime/schema"
)

// SafeMap provides a simple thread-safe map for concurrent access
type SafeMap struct {
	mu sync.Mutex
	v  map[string]interface{}
}

func NewSafeMap() *SafeMap {
	return &SafeMap{
		v: make(map[string]interface{}),
	}
}

func (s *SafeMap) Set(key string, value interface{}) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.v[key] = value
}

func (s *SafeMap) Delete(key string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.v, key)
}

func (s *SafeMap) Get(key string) (interface{}, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	v, ok := s.v[key]
	return v, ok
}

func (s *SafeMap) ListValues() []interface{} {
	s.mu.Lock()
	defer s.mu.Unlock()
	values := []interface{}{}
	for _, value := range s.v {
		values = append(values, value)
	}
	return values
}

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
	defer s.mu.Unlock()
	if k := s.v[key]; k == nil {
		s.v[key] = make(map[string]bool)
	}
	s.v[key][uid] = true
}

func (s *SafeUIDMap) DeleteUID(key, uid string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if k := s.v[key]; k != nil {
		delete(s.v[key], uid)
	}
}

func (s *SafeUIDMap) GetUIDCount(key string) int {
	s.mu.Lock()
	defer s.mu.Unlock()
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

// a struct to maintain the names of manifestwork and status
type Names struct {
	ManifestName   string
	WorkStatusName string
}

func HasPrefixInMap(m map[string]string, prefix string) bool {
	for key := range m {
		if strings.HasPrefix(key, prefix) {
			return true
		}
	}
	return false
}

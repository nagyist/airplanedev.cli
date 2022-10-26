package state

import (
	"sync"

	"github.com/pkg/errors"
)

// Generic store that is concurrency safe
type Store[K comparable, V any] struct {
	items map[K]V
	mu    sync.RWMutex
}

func NewStore[K comparable, V any](items map[K]V) Store[K, V] {
	if items != nil {
		return Store[K, V]{items: items}
	}
	return Store[K, V]{items: map[K]V{}}
}

func (s *Store[K, V]) Add(key K, val V) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.items[key] = val
}

// Returns a copy of the items map for reading
func (s *Store[K, V]) Items() map[K]V {
	s.mu.RLock()
	defer s.mu.RUnlock()
	m := make(map[K]V, len(s.items))
	for k, v := range s.items {
		m[k] = v
	}
	return m
}

func (s *Store[K, V]) Len() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return len(s.items)
}

func (s *Store[K, V]) Get(key K) (V, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	item, ok := s.items[key]
	return item, ok
}

func (s *Store[K, V]) Update(key K, f func(val *V) error) (V, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	res, ok := s.items[key]
	if !ok {
		return res, errors.Errorf("item with id %v not found", key)
	}
	if err := f(&res); err != nil {
		return res, err
	}
	s.items[key] = res
	return res, nil
}

func (s *Store[K, V]) Delete(key K) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.items, key)
}

func (s *Store[K, V]) ReplaceItems(items map[K]V) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.items = items
}

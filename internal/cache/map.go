package cache

import (
	"encoding/json"
	"fmt"
	"io"
	"sync"
)

type nestedConcurrentMap[T any] struct {
	set   map[string]*concurrentMap[T]
	mutex sync.Mutex
}

func newNestedConcurrentMap[T any]() *nestedConcurrentMap[T] {
	return &nestedConcurrentMap[T]{
		set:   map[string]*concurrentMap[T]{},
		mutex: sync.Mutex{},
	}
}

func (ncm *nestedConcurrentMap[T]) get(key string) (*concurrentMap[T], bool) {
	ncm.mutex.Lock()
	val, ok := ncm.set[key]
	ncm.mutex.Unlock()
	return val, ok
}

func (ncm *nestedConcurrentMap[T]) getOrPut(key string) (*concurrentMap[T], bool) {
	ncm.mutex.Lock()
	val, ok := ncm.set[key]
	if !ok {
		val = newConcurrentMap[T]()
		ncm.set[key] = val
	}
	ncm.mutex.Unlock()
	return val, ok
}

// func (ncm *nestedConcurrentMap[T]) toUnsafeMap() map[string]map[string]T {
// 	set := map[string]map[string]T{}
// 	for key, val := range ncm.set {
// 		set[key] = val.toUnsafeMap()
// 	}
// 	return set
// }

type concurrentMap[T any] struct {
	set   map[string]T
	mutex sync.RWMutex
}

func newConcurrentMap[T any]() *concurrentMap[T] {
	return &concurrentMap[T]{
		set:   map[string]T{},
		mutex: sync.RWMutex{},
	}
}

func (cm *concurrentMap[T]) get(key string) (T, bool) {
	cm.mutex.RLock()
	val, ok := cm.set[key]
	cm.mutex.RUnlock()
	return val, ok
}

func (cm *concurrentMap[T]) put(key string, val T) {
	cm.mutex.Lock()
	cm.set[key] = val
	cm.mutex.Unlock()
}

// func (cm *concurrentMap[T]) toUnsafeMap() map[string]T {
// 	return cm.set
// }

func (cm *concurrentMap[T]) contains(keys ...string) bool {
	cm.mutex.RLock()
	defer cm.mutex.RUnlock()

	for _, key := range keys {
		if _, ok := cm.set[key]; !ok {
			return false
		}
	}

	return true
}

func (cm *concurrentMap[T]) loadFromReader(r io.Reader) error {
	b, err := io.ReadAll(r)
	if err != nil {
		return fmt.Errorf("failed to read workspace cache: %v", err)
	}

	if err := json.Unmarshal(b, &cm.set); err != nil {
		return fmt.Errorf("failed to unmarshal workspace cache: %v", err)
	}
	return nil
}

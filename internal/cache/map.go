package cache

import (
	"encoding/json"
	"fmt"
	"io"
	"sync"
)

type concurrentMap[T any] struct {
	data  map[string]T
	mutex sync.RWMutex
}

func newConcurrentMap[T any]() *concurrentMap[T] {
	return &concurrentMap[T]{
		data:  map[string]T{},
		mutex: sync.RWMutex{},
	}
}

func (cm *concurrentMap[T]) get(key string) (T, bool) {
	cm.mutex.RLock()
	val, ok := cm.data[key]
	cm.mutex.RUnlock()
	return val, ok
}

func (cm *concurrentMap[T]) put(key string, val T) {
	cm.mutex.Lock()
	cm.data[key] = val
	cm.mutex.Unlock()
}

func (cm *concurrentMap[T]) contains(keys ...string) bool {
	cm.mutex.RLock()
	defer cm.mutex.RUnlock()

	for _, key := range keys {
		if _, ok := cm.data[key]; !ok {
			return false
		}
	}

	return true
}

func (cm *concurrentMap[T]) loadFromReader(r io.Reader) error {
	b, err := io.ReadAll(r)
	if err != nil {
		return fmt.Errorf("failed to read from reader: %v", err)
	}

	if err := json.Unmarshal(b, &cm.data); err != nil {
		return fmt.Errorf("failed to unmarshal reader data: %v", err)
	}
	return nil
}

type nestedConcurrentMap[T any] struct {
	data  map[string]*concurrentMap[T]
	mutex sync.Mutex
}

func newNestedConcurrentMap[T any]() *nestedConcurrentMap[T] {
	return &nestedConcurrentMap[T]{
		data:  map[string]*concurrentMap[T]{},
		mutex: sync.Mutex{},
	}
}

func (ncm *nestedConcurrentMap[T]) get(key string) (*concurrentMap[T], bool) {
	ncm.mutex.Lock()
	val, ok := ncm.data[key]
	ncm.mutex.Unlock()
	return val, ok
}

func (ncm *nestedConcurrentMap[T]) getOrPut(key string) (*concurrentMap[T], bool) {
	ncm.mutex.Lock()
	val, ok := ncm.data[key]
	if !ok {
		val = newConcurrentMap[T]()
		ncm.data[key] = val
	}
	ncm.mutex.Unlock()
	return val, ok
}

func (ncm *nestedConcurrentMap[T]) toUnsafeMap() map[string]map[string]T {
	hashMap := map[string]map[string]T{}
	for key, val := range ncm.data {
		hashMap[key] = val.data
	}
	return hashMap
}

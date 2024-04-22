package exec

import "sync"

type runtimeMetrics struct {
	hits   *intMetric
	total  *intMetric
	failed *mapMetric
	errors *errorMetric
}

func newRuntimeMetrics() *runtimeMetrics {
	return &runtimeMetrics{
		hits:   newIntMetric(),
		total:  newIntMetric(),
		failed: newMapMetric(),
		errors: newErrorMetric(),
	}
}

type intMetric struct {
	val   int
	mutex sync.Mutex
}

func newIntMetric() *intMetric {
	return &intMetric{
		mutex: sync.Mutex{},
	}
}

func (m *intMetric) increment() {
	m.mutex.Lock()
	m.val += 1
	m.mutex.Unlock()
}

type mapMetric struct {
	val   map[string]struct{}
	mutex sync.Mutex
}

func newMapMetric() *mapMetric {
	return &mapMetric{
		val:   map[string]struct{}{},
		mutex: sync.Mutex{},
	}
}

func (m *mapMetric) contains(key string) bool {
	m.mutex.Lock()
	_, ok := m.val[key]
	m.mutex.Unlock()
	return ok
}

func (m *mapMetric) put(key string) {
	m.mutex.Lock()
	m.val[key] = struct{}{}
	m.mutex.Unlock()
}

type errorMetric struct {
	val   []error
	mutex sync.Mutex
}

func newErrorMetric() *errorMetric {
	return &errorMetric{
		val:   []error{},
		mutex: sync.Mutex{},
	}
}

func (metric *errorMetric) append(err error) {
	metric.mutex.Lock()
	metric.val = append(metric.val, err)
	metric.mutex.Unlock()
}

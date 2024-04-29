package exec

import "sync"

type statistics struct {
	hits   *intStatistic
	total  *intStatistic
	failed *mapStatistic
	errors *errorStatistic
}

func newStatistics() *statistics {
	return &statistics{
		hits:   newIntMetric(),
		total:  newIntMetric(),
		failed: newMapMetric(),
		errors: newErrorMetric(),
	}
}

type intStatistic struct {
	val   int
	mutex sync.Mutex
}

func newIntMetric() *intStatistic {
	return &intStatistic{
		mutex: sync.Mutex{},
	}
}

func (m *intStatistic) increment() {
	m.mutex.Lock()
	m.val += 1
	m.mutex.Unlock()
}

type mapStatistic struct {
	val   map[string]struct{}
	mutex sync.Mutex
}

func newMapMetric() *mapStatistic {
	return &mapStatistic{
		val:   map[string]struct{}{},
		mutex: sync.Mutex{},
	}
}

func (m *mapStatistic) contains(key string) bool {
	m.mutex.Lock()
	_, ok := m.val[key]
	m.mutex.Unlock()
	return ok
}

func (m *mapStatistic) put(key string) {
	m.mutex.Lock()
	m.val[key] = struct{}{}
	m.mutex.Unlock()
}

type errorStatistic struct {
	val   []error
	mutex sync.Mutex
}

func newErrorMetric() *errorStatistic {
	return &errorStatistic{
		val:   []error{},
		mutex: sync.Mutex{},
	}
}

func (metric *errorStatistic) append(err error) {
	metric.mutex.Lock()
	metric.val = append(metric.val, err)
	metric.mutex.Unlock()
}

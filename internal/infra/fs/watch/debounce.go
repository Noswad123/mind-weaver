package watch

import (
	"sync"
	"time"
)

type debouncer struct {
	mu     sync.Mutex
	timers map[string]*time.Timer
	delay  time.Duration
}

func newDebouncer(delay time.Duration) *debouncer {
	return &debouncer{
		timers: map[string]*time.Timer{},
		delay:  delay,
	}
}

func (d *debouncer) Do(key string, fn func()) {
	d.mu.Lock()
	defer d.mu.Unlock()

	if t, ok := d.timers[key]; ok {
		t.Stop()
	}
	d.timers[key] = time.AfterFunc(d.delay, fn)
}

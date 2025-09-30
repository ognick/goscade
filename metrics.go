package goscade

import (
	"sync"
	"time"
)

// DefaultMetrics is a simple in-memory metrics collector that logs metrics.
type DefaultMetrics struct {
	mu sync.RWMutex
	// ComponentStartTimes tracks when components started
	ComponentStartTimes map[string]time.Time
	// ComponentReadyDurations tracks how long components took to become ready
	ComponentReadyDurations map[string]time.Duration
	// ComponentStopDurations tracks how long components took to stop
	ComponentStopDurations map[string]time.Duration
	// ComponentErrors tracks component errors by type
	ComponentErrors map[string]map[string]int
}

// NewDefaultMetrics creates a new DefaultMetrics instance.
func NewDefaultMetrics() *DefaultMetrics {
	return &DefaultMetrics{
		ComponentStartTimes:     make(map[string]time.Time),
		ComponentReadyDurations: make(map[string]time.Duration),
		ComponentStopDurations:  make(map[string]time.Duration),
		ComponentErrors:         make(map[string]map[string]int),
	}
}

// ComponentStartTime records the time when a component starts its readiness probe.
func (m *DefaultMetrics) ComponentStartTime(componentName string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.ComponentStartTimes[componentName] = time.Now()
}

// ComponentReadyTime records the time when a component becomes ready.
func (m *DefaultMetrics) ComponentReadyTime(componentName string, duration time.Duration) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.ComponentReadyDurations[componentName] = duration
}

// ComponentStopTime records the time when a component stops.
func (m *DefaultMetrics) ComponentStopTime(componentName string, duration time.Duration) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.ComponentStopDurations[componentName] = duration
}

// ComponentError records when a component encounters an error.
func (m *DefaultMetrics) ComponentError(componentName string, errorType string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.ComponentErrors[componentName] == nil {
		m.ComponentErrors[componentName] = make(map[string]int)
	}
	m.ComponentErrors[componentName][errorType]++
}

// GetComponentStartTime returns the start time for a component.
func (m *DefaultMetrics) GetComponentStartTime(componentName string) (time.Time, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	startTime, exists := m.ComponentStartTimes[componentName]
	return startTime, exists
}

// GetComponentReadyDuration returns the ready duration for a component.
func (m *DefaultMetrics) GetComponentReadyDuration(componentName string) (time.Duration, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	duration, exists := m.ComponentReadyDurations[componentName]
	return duration, exists
}

// GetComponentStopDuration returns the stop duration for a component.
func (m *DefaultMetrics) GetComponentStopDuration(componentName string) (time.Duration, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	duration, exists := m.ComponentStopDurations[componentName]
	return duration, exists
}

// GetComponentErrorCount returns the error count for a component and error type.
func (m *DefaultMetrics) GetComponentErrorCount(componentName, errorType string) int {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if errors, exists := m.ComponentErrors[componentName]; exists {
		return errors[errorType]
	}
	return 0
}

// GetAllMetrics returns a snapshot of all metrics.
func (m *DefaultMetrics) GetAllMetrics() map[string]interface{} {
	m.mu.RLock()
	defer m.mu.RUnlock()
	
	return map[string]interface{}{
		"component_start_times":     m.ComponentStartTimes,
		"component_ready_durations": m.ComponentReadyDurations,
		"component_stop_durations":  m.ComponentStopDurations,
		"component_errors":          m.ComponentErrors,
	}
}

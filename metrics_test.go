package goscade

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

// TestDefaultMetrics_ComponentStartTime tests the ComponentStartTime method
func TestDefaultMetrics_ComponentStartTime(t *testing.T) {
	metrics := NewDefaultMetrics()
	componentName := "test-component"

	// Record start time
	metrics.ComponentStartTime(componentName)

	// Verify start time was recorded
	startTime, exists := metrics.GetComponentStartTime(componentName)
	assert.True(t, exists)
	assert.False(t, startTime.IsZero())
	assert.True(t, time.Since(startTime) < 100*time.Millisecond)
}

// TestDefaultMetrics_ComponentReadyTime tests the ComponentReadyTime method
func TestDefaultMetrics_ComponentReadyTime(t *testing.T) {
	metrics := NewDefaultMetrics()
	componentName := "test-component"
	duration := 50 * time.Millisecond

	// Record ready time
	metrics.ComponentReadyTime(componentName, duration)

	// Verify ready duration was recorded
	readyDuration, exists := metrics.GetComponentReadyDuration(componentName)
	assert.True(t, exists)
	assert.Equal(t, duration, readyDuration)
}

// TestDefaultMetrics_ComponentStopTime tests the ComponentStopTime method
func TestDefaultMetrics_ComponentStopTime(t *testing.T) {
	metrics := NewDefaultMetrics()
	componentName := "test-component"
	duration := 100 * time.Millisecond

	// Record stop time
	metrics.ComponentStopTime(componentName, duration)

	// Verify stop duration was recorded
	stopDuration, exists := metrics.GetComponentStopDuration(componentName)
	assert.True(t, exists)
	assert.Equal(t, duration, stopDuration)
}

// TestDefaultMetrics_ComponentError tests the ComponentError method
func TestDefaultMetrics_ComponentError(t *testing.T) {
	metrics := NewDefaultMetrics()
	componentName := "test-component"
	errorType := "test-error"

	// Record error
	metrics.ComponentError(componentName, errorType)

	// Verify error count was recorded
	errorCount := metrics.GetComponentErrorCount(componentName, errorType)
	assert.Equal(t, 1, errorCount)

	// Record same error again
	metrics.ComponentError(componentName, errorType)
	errorCount = metrics.GetComponentErrorCount(componentName, errorType)
	assert.Equal(t, 2, errorCount)

	// Record different error type
	metrics.ComponentError(componentName, "different-error")
	errorCount = metrics.GetComponentErrorCount(componentName, "different-error")
	assert.Equal(t, 1, errorCount)
}

// TestDefaultMetrics_GetAllMetrics tests the GetAllMetrics method
func TestDefaultMetrics_GetAllMetrics(t *testing.T) {
	metrics := NewDefaultMetrics()
	componentName := "test-component"

	// Record various metrics
	metrics.ComponentStartTime(componentName)
	metrics.ComponentReadyTime(componentName, 50*time.Millisecond)
	metrics.ComponentStopTime(componentName, 100*time.Millisecond)
	metrics.ComponentError(componentName, "test-error")

	// Get all metrics
	allMetrics := metrics.GetAllMetrics()

	// Verify all expected keys are present
	assert.Contains(t, allMetrics, "component_start_times")
	assert.Contains(t, allMetrics, "component_ready_durations")
	assert.Contains(t, allMetrics, "component_stop_durations")
	assert.Contains(t, allMetrics, "component_errors")

	// Verify data types
	startTimes, ok := allMetrics["component_start_times"].(map[string]time.Time)
	assert.True(t, ok)
	assert.Contains(t, startTimes, componentName)

	readyDurations, ok := allMetrics["component_ready_durations"].(map[string]time.Duration)
	assert.True(t, ok)
	assert.Contains(t, readyDurations, componentName)
	assert.Equal(t, 50*time.Millisecond, readyDurations[componentName])

	stopDurations, ok := allMetrics["component_stop_durations"].(map[string]time.Duration)
	assert.True(t, ok)
	assert.Contains(t, stopDurations, componentName)
	assert.Equal(t, 100*time.Millisecond, stopDurations[componentName])

	errors, ok := allMetrics["component_errors"].(map[string]map[string]int)
	assert.True(t, ok)
	assert.Contains(t, errors, componentName)
	assert.Equal(t, 1, errors[componentName]["test-error"])
}

// TestDefaultMetrics_ConcurrentAccess tests concurrent access to metrics
func TestDefaultMetrics_ConcurrentAccess(t *testing.T) {
	metrics := NewDefaultMetrics()
	componentName := "test-component"

	// Run concurrent operations
	done := make(chan bool, 10)
	
	for i := 0; i < 10; i++ {
		go func(index int) {
			defer func() { done <- true }()
			
			// Each goroutine records different metrics
			metrics.ComponentStartTime(componentName)
			metrics.ComponentReadyTime(componentName, time.Duration(index)*time.Millisecond)
			metrics.ComponentStopTime(componentName, time.Duration(index+1)*time.Millisecond)
			metrics.ComponentError(componentName, "concurrent-error")
		}(i)
	}

	// Wait for all goroutines to complete
	for i := 0; i < 10; i++ {
		<-done
	}

	// Verify metrics were recorded (last values should be preserved)
	startTime, exists := metrics.GetComponentStartTime(componentName)
	assert.True(t, exists)
	assert.False(t, startTime.IsZero())

	readyDuration, exists := metrics.GetComponentReadyDuration(componentName)
	assert.True(t, exists)
	assert.True(t, readyDuration >= 0)

	stopDuration, exists := metrics.GetComponentStopDuration(componentName)
	assert.True(t, exists)
	assert.True(t, stopDuration > 0)

	errorCount := metrics.GetComponentErrorCount(componentName, "concurrent-error")
	assert.Equal(t, 10, errorCount)
}

// TestDefaultMetrics_NonExistentComponent tests behavior with non-existent components
func TestDefaultMetrics_NonExistentComponent(t *testing.T) {
	metrics := NewDefaultMetrics()
	componentName := "non-existent-component"

	// Try to get metrics for non-existent component
	startTime, exists := metrics.GetComponentStartTime(componentName)
	assert.False(t, exists)
	assert.True(t, startTime.IsZero())

	readyDuration, exists := metrics.GetComponentReadyDuration(componentName)
	assert.False(t, exists)
	assert.Equal(t, time.Duration(0), readyDuration)

	stopDuration, exists := metrics.GetComponentStopDuration(componentName)
	assert.False(t, exists)
	assert.Equal(t, time.Duration(0), stopDuration)

	errorCount := metrics.GetComponentErrorCount(componentName, "any-error")
	assert.Equal(t, 0, errorCount)
}

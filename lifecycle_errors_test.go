package goscade

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type lifecycleErrorComponent struct {
	name string
	run  func(context.Context, func(error)) error
}

func (c *lifecycleErrorComponent) Run(ctx context.Context, probe func(error)) error {
	return c.run(ctx, probe)
}

func (c *lifecycleErrorComponent) delegateName() string {
	return c.name
}

func runLifecycleForErrors(lc Lifecycle, ctx context.Context) (<-chan error, <-chan error) {
	ready := make(chan error, 1)
	done := make(chan error, 1)
	go func() {
		done <- lc.Run(ctx, func(err error) { ready <- err })
	}()
	return ready, done
}

func receiveLifecycleError(t *testing.T, ch <-chan error) error {
	t.Helper()
	select {
	case err := <-ch:
		return err
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for lifecycle")
		return nil
	}
}

func TestLifecycle_JoinsIndependentComponentErrors(t *testing.T) {
	firstErr := errors.New("database failed")
	secondErr := errors.New("broker failed")
	release := make(chan struct{})
	lc := NewLifecycle(&mockLogger{})

	for i, componentErr := range []error{firstErr, secondErr} {
		componentErr := componentErr
		lc.Register(&lifecycleErrorComponent{
			name: fmt.Sprintf("component-%d", i),
			run: func(_ context.Context, probe func(error)) error {
				probe(nil)
				<-release
				return componentErr
			},
		})
	}

	ready, done := runLifecycleForErrors(lc, context.Background())
	require.NoError(t, receiveLifecycleError(t, ready))
	close(release)

	runErr := receiveLifecycleError(t, done)
	assert.ErrorIs(t, runErr, firstErr)
	assert.ErrorIs(t, runErr, secondErr)
}

func TestLifecycle_JoinsIndependentReadinessErrors(t *testing.T) {
	firstErr := fmt.Errorf("database unavailable: %w", context.Canceled)
	secondErr := fmt.Errorf("broker unavailable: %w", context.Canceled)
	var probes sync.WaitGroup
	probes.Add(2)
	lc := NewLifecycle(&mockLogger{})

	for i, probeErr := range []error{firstErr, secondErr} {
		probeErr := probeErr
		lc.Register(&lifecycleErrorComponent{
			name: fmt.Sprintf("component-%d", i),
			run: func(_ context.Context, probe func(error)) error {
				probe(probeErr)
				probes.Done()
				probes.Wait()
				return nil
			},
		})
	}

	ready, done := runLifecycleForErrors(lc, context.Background())
	assert.ErrorIs(t, receiveLifecycleError(t, ready), context.Canceled)
	runErr := receiveLifecycleError(t, done)
	assert.ErrorIs(t, runErr, firstErr)
	assert.ErrorIs(t, runErr, secondErr)
}

func TestLifecycle_RemovesCascadeCancellationButKeepsCleanupError(t *testing.T) {
	primaryErr := errors.New("database failed")
	cleanupErr := errors.New("flush failed")
	fail := make(chan struct{})
	lc := NewLifecycle(&mockLogger{})
	lc.Register(&lifecycleErrorComponent{name: "database", run: func(_ context.Context, probe func(error)) error {
		probe(nil)
		<-fail
		return primaryErr
	}})
	lc.Register(&lifecycleErrorComponent{name: "api", run: func(ctx context.Context, probe func(error)) error {
		probe(nil)
		<-ctx.Done()
		return fmt.Errorf("shutdown: %w", errors.Join(ctx.Err(), cleanupErr))
	}})

	ready, done := runLifecycleForErrors(lc, context.Background())
	require.NoError(t, receiveLifecycleError(t, ready))
	close(fail)

	runErr := receiveLifecycleError(t, done)
	assert.ErrorIs(t, runErr, primaryErr)
	assert.ErrorIs(t, runErr, cleanupErr)
	assert.NotErrorIs(t, runErr, context.Canceled)
}

func TestLifecycle_DeduplicatesPrimaryCause(t *testing.T) {
	primaryErr := errors.New("database failed")
	fail := make(chan struct{})
	lc := NewLifecycle(&mockLogger{})
	lc.Register(&lifecycleErrorComponent{name: "database", run: func(_ context.Context, probe func(error)) error {
		probe(nil)
		<-fail
		return primaryErr
	}})
	lc.Register(&lifecycleErrorComponent{name: "api", run: func(ctx context.Context, probe func(error)) error {
		probe(nil)
		<-ctx.Done()
		return context.Cause(ctx)
	}})

	ready, done := runLifecycleForErrors(lc, context.Background())
	require.NoError(t, receiveLifecycleError(t, ready))
	close(fail)

	runErr := receiveLifecycleError(t, done)
	assert.ErrorIs(t, runErr, primaryErr)
	assert.Equal(t, 1, strings.Count(runErr.Error(), primaryErr.Error()))
}

func TestLifecycle_JoinsShutdownTimeoutWithPrimaryCause(t *testing.T) {
	primaryErr := errors.New("database failed")
	fail := make(chan struct{})
	release := make(chan struct{})
	defer close(release)
	lc := NewLifecycle(&mockLogger{}, WithShutdownTimeout(50*time.Millisecond))
	lc.Register(&lifecycleErrorComponent{name: "database", run: func(_ context.Context, probe func(error)) error {
		probe(nil)
		<-fail
		return primaryErr
	}})
	lc.Register(&lifecycleErrorComponent{name: "stuck", run: func(_ context.Context, probe func(error)) error {
		probe(nil)
		<-release
		return nil
	}})

	ready, done := runLifecycleForErrors(lc, context.Background())
	require.NoError(t, receiveLifecycleError(t, ready))
	close(fail)

	runErr := receiveLifecycleError(t, done)
	assert.ErrorIs(t, runErr, primaryErr)
	assert.ErrorIs(t, runErr, ShutdownTimeoutError)
}

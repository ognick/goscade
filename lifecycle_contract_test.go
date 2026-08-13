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

type contractComponent struct {
	run func(context.Context, func(error)) error
}

func (c *contractComponent) Run(ctx context.Context, readinessProbe func(error)) error {
	return c.run(ctx, readinessProbe)
}

type namedContractComponent struct {
	name string
	run  func(context.Context, func(error)) error
}

func (c *namedContractComponent) Run(ctx context.Context, readinessProbe func(error)) error {
	return c.run(ctx, readinessProbe)
}

func (c *namedContractComponent) delegateName() string {
	return c.name
}

type delayedProbeLogger struct {
	delayName string
}

func (l *delayedProbeLogger) Infof(string, ...interface{}) {}

func (l *delayedProbeLogger) Errorf(_ string, args ...interface{}) {
	if len(args) > 0 && args[0] == l.delayName {
		time.Sleep(20 * time.Millisecond)
	}
}

func runContractLifecycle(
	lc Lifecycle,
	ctx context.Context,
) (<-chan error, <-chan error) {
	ready := make(chan error, 1)
	done := make(chan error, 1)
	go func() {
		done <- lc.Run(ctx, func(err error) {
			ready <- err
		})
	}()
	return ready, done
}

func receiveContractResult(t *testing.T, result <-chan error) error {
	t.Helper()
	select {
	case err := <-result:
		return err
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for lifecycle result")
		return nil
	}
}

func TestLifecycle_Run_PreservesInputCause(t *testing.T) {
	tests := []struct {
		name    string
		newCtx  func() (context.Context, context.CancelFunc)
		wantErr error
	}{
		{
			name: "cancellation",
			newCtx: func() (context.Context, context.CancelFunc) {
				return context.WithCancel(context.Background())
			},
			wantErr: context.Canceled,
		},
		{
			name: "deadline",
			newCtx: func() (context.Context, context.CancelFunc) {
				return context.WithTimeout(context.Background(), 20*time.Millisecond)
			},
			wantErr: context.DeadlineExceeded,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx, cancel := tt.newCtx()
			defer cancel()

			lc := NewLifecycle(&mockLogger{})
			lc.Register(&contractComponent{run: func(ctx context.Context, probe func(error)) error {
				probe(nil)
				<-ctx.Done()
				return nil
			}})

			ready, done := runContractLifecycle(lc, ctx)
			require.NoError(t, receiveContractResult(t, ready))
			if tt.wantErr == context.Canceled {
				cancel()
			}

			assert.ErrorIs(t, receiveContractResult(t, done), tt.wantErr)
		})
	}
}

func TestLifecycle_Run_UnexpectedCloseCause(t *testing.T) {
	lc := NewLifecycle(&mockLogger{})
	lc.Register(&contractComponent{run: func(_ context.Context, probe func(error)) error {
		probe(nil)
		return nil
	}})

	ready, done := runContractLifecycle(lc, context.Background())
	require.NoError(t, receiveContractResult(t, ready))

	runErr := receiveContractResult(t, done)
	assert.ErrorIs(t, runErr, UnexpectedCloseComponentError)
	assert.Contains(t, runErr.Error(), "component *goscade.contractComponent")
}

func TestLifecycle_Run_ReadinessCause(t *testing.T) {
	readinessErr := errors.New("database is unavailable")
	lc := NewLifecycle(&mockLogger{})
	lc.Register(&contractComponent{run: func(_ context.Context, probe func(error)) error {
		probe(readinessErr)
		return readinessErr
	}})

	ready, done := runContractLifecycle(lc, context.Background())
	probeErr := receiveContractResult(t, ready)
	runErr := receiveContractResult(t, done)

	assert.ErrorIs(t, probeErr, readinessErr)
	assert.ErrorIs(t, runErr, readinessErr)
	assert.Contains(t, probeErr.Error(), "component *goscade.contractComponent readiness")
	assert.Contains(t, runErr.Error(), "component *goscade.contractComponent readiness")
}

func TestLifecycle_Run_ReadinessCauseNamesFailingComponent(t *testing.T) {
	readinessErr := errors.New("database is unavailable")
	parent := &namedContractComponent{name: "database", run: func(ctx context.Context, probe func(error)) error {
		probe(readinessErr)
		<-ctx.Done()
		return nil
	}}
	child := &namedContractComponent{name: "api", run: func(ctx context.Context, probe func(error)) error {
		probe(nil)
		<-ctx.Done()
		return nil
	}}
	lc := NewLifecycle(&delayedProbeLogger{delayName: "database"})
	lc.Register(child, parent)

	ready, done := runContractLifecycle(lc, context.Background())
	probeErr := receiveContractResult(t, ready)
	runErr := receiveContractResult(t, done)

	assert.ErrorIs(t, probeErr, readinessErr)
	assert.Equal(t, "component database readiness: database is unavailable", probeErr.Error())
	assert.ErrorIs(t, runErr, readinessErr)
	assert.Equal(t, "component database readiness: database is unavailable", runErr.Error())
}

func TestLifecycle_Run_ReadinessCanceledIsFailure(t *testing.T) {
	readinessErr := fmt.Errorf("configuration aborted: %w", context.Canceled)
	lc := NewLifecycle(&mockLogger{})
	lc.Register(&namedContractComponent{name: "config", run: func(_ context.Context, probe func(error)) error {
		probe(readinessErr)
		return readinessErr
	}})

	ready, done := runContractLifecycle(lc, context.Background())
	probeErr := receiveContractResult(t, ready)
	runErr := receiveContractResult(t, done)

	require.ErrorIs(t, probeErr, readinessErr)
	assert.Equal(t, "component config readiness: configuration aborted: context canceled", probeErr.Error())
	require.ErrorIs(t, runErr, readinessErr)
	assert.Equal(t, "component config readiness: configuration aborted: context canceled", runErr.Error())
}

func TestLifecycle_Run_JoinsIndependentErrors(t *testing.T) {
	firstErr := errors.New("first component failed")
	secondErr := errors.New("second component failed")
	release := make(chan struct{})
	lc := NewLifecycle(&mockLogger{})

	for _, componentErr := range []error{firstErr, secondErr} {
		componentErr := componentErr
		lc.Register(&contractComponent{run: func(_ context.Context, probe func(error)) error {
			probe(nil)
			<-release
			return componentErr
		}})
	}

	ready, done := runContractLifecycle(lc, context.Background())
	require.NoError(t, receiveContractResult(t, ready))
	close(release)

	runErr := receiveContractResult(t, done)
	assert.ErrorIs(t, runErr, firstErr)
	assert.ErrorIs(t, runErr, secondErr)
	assert.Contains(t, runErr.Error(), "component *goscade.contractComponent")
}

func TestLifecycle_Run_JoinsIndependentReadinessErrors(t *testing.T) {
	firstErr := errors.New("database is unavailable")
	secondErr := errors.New("broker is unavailable")
	var probed sync.WaitGroup
	probed.Add(2)
	lc := NewLifecycle(&mockLogger{})

	for _, readinessErr := range []error{firstErr, secondErr} {
		readinessErr := readinessErr
		lc.Register(&contractComponent{run: func(_ context.Context, probe func(error)) error {
			probe(readinessErr)
			probed.Done()
			probed.Wait()
			return nil
		}})
	}

	ready, done := runContractLifecycle(lc, context.Background())
	probeErr := receiveContractResult(t, ready)
	runErr := receiveContractResult(t, done)

	assert.True(t, errors.Is(probeErr, firstErr) || errors.Is(probeErr, secondErr))
	assert.ErrorIs(t, runErr, firstErr)
	assert.ErrorIs(t, runErr, secondErr)
}

func TestLifecycle_Run_JoinsShutdownErrorWithInputCause(t *testing.T) {
	cleanupErr := errors.New("flush failed")
	ctx, cancel := context.WithCancel(context.Background())
	lc := NewLifecycle(&mockLogger{})
	lc.Register(&contractComponent{run: func(ctx context.Context, probe func(error)) error {
		probe(nil)
		<-ctx.Done()
		return cleanupErr
	}})

	ready, done := runContractLifecycle(lc, ctx)
	require.NoError(t, receiveContractResult(t, ready))
	cancel()

	runErr := receiveContractResult(t, done)
	assert.ErrorIs(t, runErr, context.Canceled)
	assert.ErrorIs(t, runErr, cleanupErr)
}

func TestLifecycle_Run_PreservesWrappedCleanupErrorContext(t *testing.T) {
	cleanupErr := errors.New("flush failed")
	ctx, cancel := context.WithCancel(context.Background())
	lc := NewLifecycle(&mockLogger{})
	lc.Register(&contractComponent{run: func(ctx context.Context, probe func(error)) error {
		probe(nil)
		<-ctx.Done()
		return fmt.Errorf("flush users: %w", cleanupErr)
	}})

	ready, done := runContractLifecycle(lc, ctx)
	require.NoError(t, receiveContractResult(t, ready))
	cancel()

	runErr := receiveContractResult(t, done)
	assert.ErrorIs(t, runErr, context.Canceled)
	assert.ErrorIs(t, runErr, cleanupErr)
	assert.Contains(t, runErr.Error(), "flush users: flush failed")
}

func TestLifecycle_Run_DoesNotDuplicateCascadeCause(t *testing.T) {
	componentErr := errors.New("database failed")
	fail := make(chan struct{})
	lc := NewLifecycle(&mockLogger{})
	lc.Register(&contractComponent{run: func(_ context.Context, probe func(error)) error {
		probe(nil)
		<-fail
		return componentErr
	}})
	lc.Register(&contractComponent{run: func(ctx context.Context, probe func(error)) error {
		probe(nil)
		<-ctx.Done()
		return context.Cause(ctx)
	}})

	ready, done := runContractLifecycle(lc, context.Background())
	require.NoError(t, receiveContractResult(t, ready))
	close(fail)

	runErr := receiveContractResult(t, done)
	assert.ErrorIs(t, runErr, componentErr)
	assert.Equal(t, 1, strings.Count(runErr.Error(), componentErr.Error()))
}

func TestLifecycle_Run_DoesNotCollectCascadeContextCanceled(t *testing.T) {
	componentErr := errors.New("database failed")
	fail := make(chan struct{})
	lc := NewLifecycle(&mockLogger{})
	lc.Register(&contractComponent{run: func(_ context.Context, probe func(error)) error {
		probe(nil)
		<-fail
		return componentErr
	}})
	lc.Register(&contractComponent{run: func(ctx context.Context, probe func(error)) error {
		probe(nil)
		<-ctx.Done()
		return ctx.Err()
	}})

	ready, done := runContractLifecycle(lc, context.Background())
	require.NoError(t, receiveContractResult(t, ready))
	close(fail)

	runErr := receiveContractResult(t, done)
	assert.ErrorIs(t, runErr, componentErr)
	assert.NotErrorIs(t, runErr, context.Canceled)
}

func TestLifecycle_Run_PreservesCleanupErrorJoinedWithCascadeCancellation(t *testing.T) {
	componentErr := errors.New("database failed")
	cleanupErr := errors.New("flush failed")
	fail := make(chan struct{})
	lc := NewLifecycle(&mockLogger{})
	lc.Register(&contractComponent{run: func(_ context.Context, probe func(error)) error {
		probe(nil)
		<-fail
		return componentErr
	}})
	lc.Register(&contractComponent{run: func(ctx context.Context, probe func(error)) error {
		probe(nil)
		<-ctx.Done()
		return errors.Join(ctx.Err(), cleanupErr)
	}})

	ready, done := runContractLifecycle(lc, context.Background())
	require.NoError(t, receiveContractResult(t, ready))
	close(fail)

	runErr := receiveContractResult(t, done)
	assert.ErrorIs(t, runErr, componentErr)
	assert.ErrorIs(t, runErr, cleanupErr)
	assert.NotErrorIs(t, runErr, context.Canceled)
}

func TestLifecycle_Run_PreservesCleanupErrorInWrappedJoin(t *testing.T) {
	componentErr := errors.New("database failed")
	cleanupErr := errors.New("flush failed")
	fail := make(chan struct{})
	lc := NewLifecycle(&mockLogger{})
	lc.Register(&contractComponent{run: func(_ context.Context, probe func(error)) error {
		probe(nil)
		<-fail
		return componentErr
	}})
	lc.Register(&contractComponent{run: func(ctx context.Context, probe func(error)) error {
		probe(nil)
		<-ctx.Done()
		return fmt.Errorf("shutdown failed: %w", errors.Join(ctx.Err(), cleanupErr))
	}})

	ready, done := runContractLifecycle(lc, context.Background())
	require.NoError(t, receiveContractResult(t, ready))
	close(fail)

	runErr := receiveContractResult(t, done)
	assert.ErrorIs(t, runErr, componentErr)
	assert.ErrorIs(t, runErr, cleanupErr)
	assert.NotErrorIs(t, runErr, context.Canceled)
}

func TestLifecycle_Run_Empty(t *testing.T) {
	lc := NewLifecycle(&mockLogger{})
	assert.NoError(t, lc.Run(context.Background(), nil))
}

func TestLifecycle_Run_ShutdownTimeout(t *testing.T) {
	release := make(chan struct{})
	defer close(release)
	ctx, cancel := context.WithCancel(context.Background())
	lc := NewLifecycle(&mockLogger{}, WithShutdownTimeout(20*time.Millisecond))
	lc.Register(&contractComponent{run: func(_ context.Context, probe func(error)) error {
		probe(nil)
		<-release
		return nil
	}})

	ready, done := runContractLifecycle(lc, ctx)
	require.NoError(t, receiveContractResult(t, ready))
	cancel()

	runErr := receiveContractResult(t, done)
	assert.ErrorIs(t, runErr, context.Canceled)
	assert.ErrorIs(t, runErr, ShutdownTimeoutError)
	assert.ErrorIs(t, runErr, context.DeadlineExceeded)
}

func TestLifecycle_Run_ShutdownTimeoutPreservesSentinelAfterInputDeadline(t *testing.T) {
	release := make(chan struct{})
	defer close(release)
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancel()
	lc := NewLifecycle(&mockLogger{}, WithShutdownTimeout(20*time.Millisecond))
	lc.Register(&contractComponent{run: func(_ context.Context, probe func(error)) error {
		probe(nil)
		<-release
		return nil
	}})

	ready, done := runContractLifecycle(lc, ctx)
	require.NoError(t, receiveContractResult(t, ready))

	runErr := receiveContractResult(t, done)
	assert.ErrorIs(t, runErr, context.DeadlineExceeded)
	assert.ErrorIs(t, runErr, ShutdownTimeoutError)
}

func TestLifecycle_Run_ShutdownTimeoutJoinsPrimaryFailure(t *testing.T) {
	componentErr := errors.New("database failed")
	fail := make(chan struct{})
	release := make(chan struct{})
	defer close(release)
	lc := NewLifecycle(&mockLogger{}, WithShutdownTimeout(20*time.Millisecond))
	lc.Register(&contractComponent{run: func(_ context.Context, probe func(error)) error {
		probe(nil)
		<-fail
		return componentErr
	}})
	lc.Register(&contractComponent{run: func(_ context.Context, probe func(error)) error {
		probe(nil)
		<-release
		return nil
	}})

	ready, done := runContractLifecycle(lc, context.Background())
	require.NoError(t, receiveContractResult(t, ready))
	close(fail)

	runErr := receiveContractResult(t, done)
	assert.ErrorIs(t, runErr, componentErr)
	assert.ErrorIs(t, runErr, ShutdownTimeoutError)
}

func TestLifecycle_Run_ShutdownTimeoutNotAddedToFastShutdown(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	lc := NewLifecycle(&mockLogger{}, WithShutdownTimeout(50*time.Millisecond))
	lc.Register(&contractComponent{run: func(ctx context.Context, probe func(error)) error {
		probe(nil)
		<-ctx.Done()
		return nil
	}})

	ready, done := runContractLifecycle(lc, ctx)
	require.NoError(t, receiveContractResult(t, ready))
	cancel()

	runErr := receiveContractResult(t, done)
	assert.ErrorIs(t, runErr, context.Canceled)
	assert.NotErrorIs(t, runErr, ShutdownTimeoutError)
}

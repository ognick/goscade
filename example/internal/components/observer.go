package components

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"strconv"
	"sync"
	"time"

	"github.com/ognick/goscade"
)

type Status string

const (
	StatusPending  Status = "pending"
	StatusRunning  Status = "running"
	StatusReady    Status = "ready"
	StatusError    Status = "error"
	StatusStopping Status = "stopping"
	StatusStopped  Status = "stopped"
)

type State struct {
	Status Status
}

type CompCfg struct {
	Err   *string
	Delay time.Duration
}

type Observer struct {
	mu        sync.Mutex
	idToState map[uint64]State
	idToKill  map[uint64]chan error
	idToCfg   map[uint64]CompCfg
}

func NewObserver() *Observer {
	return &Observer{
		idToState: make(map[uint64]State),
		idToKill:  make(map[uint64]chan error),
		idToCfg:   make(map[uint64]CompCfg),
	}
}

func (o *Observer) run(ctx context.Context, comp goscade.Component, readinessProbe func(err error)) error {
	compID := uint64(reflect.ValueOf(comp).Pointer())
	kill := make(chan error)

	o.mu.Lock()
	o.idToKill[compID] = kill
	cfg, ok := o.idToCfg[compID]
	o.mu.Unlock()
	if !ok {
		return errors.New("cfg not found")
	}

	o.setStatus(compID, StatusRunning)
	<-time.After(cfg.Delay)

	if cfg.Err != nil {
		o.setStatus(compID, StatusError)
		err := errors.New(*cfg.Err)
		readinessProbe(err)
		return err
	}
	readinessProbe(nil)

	o.setStatus(compID, StatusReady)
	select {
	case err := <-kill:
		o.setStatus(compID, StatusError)
		return err
	case <-ctx.Done():
		o.mu.Lock()
		cfg = o.idToCfg[compID]
		o.mu.Unlock()
		if cfg.Err != nil {
			o.setStatus(compID, StatusError)
			err := errors.New(*cfg.Err)
			return err
		}
	}

	o.setStatus(compID, StatusStopping)
	<-time.After(cfg.Delay)

	if ctx.Err() == nil || errors.Is(ctx.Err(), context.Canceled) {
		defer o.setStatus(compID, StatusStopped)
		return nil
	}

	o.setStatus(compID, StatusError)
	return ctx.Err()
}

func (o *Observer) setStatus(compID uint64, status Status) {
	o.mu.Lock()
	defer o.mu.Unlock()

	state := o.idToState[compID]
	state.Status = status
	o.idToState[compID] = state
}

func (o *Observer) Register(comp goscade.Component) goscade.Component {
	id := uint64(reflect.ValueOf(comp).Pointer())
	o.mu.Lock()
	defer o.mu.Unlock()
	o.idToCfg[id] = CompCfg{
		Err:   nil,
		Delay: 1 * time.Second,
	}
	return comp
}

func (o *Observer) GetStatus(comp goscade.Component) Status {
	o.mu.Lock()
	defer o.mu.Unlock()

	id := uint64(reflect.ValueOf(comp).Pointer())
	if state, ok := o.idToState[id]; ok {
		return state.Status
	}

	return StatusPending
}

func (o *Observer) KillComponent(compIDstr string) error {
	compID, err := strconv.Atoi(compIDstr)
	if err != nil {
		return err
	}

	o.mu.Lock()
	defer o.mu.Unlock()
	if kill, ok := o.idToKill[uint64(compID)]; ok {
		kill <- fmt.Errorf("kill")
		return nil
	}

	return errors.New("kill signal not found")
}

func (o *Observer) UpdateComponent(compIDstr string, cfg CompCfg) error {
	compID, err := strconv.Atoi(compIDstr)
	if err != nil {
		return err
	}

	o.mu.Lock()
	defer o.mu.Unlock()
	if _, ok := o.idToCfg[uint64(compID)]; !ok {
		return errors.New("cfg not found")
	}
	o.idToCfg[uint64(compID)] = cfg
	return nil
}

func (o *Observer) GetCfg(comp goscade.Component) CompCfg {
	o.mu.Lock()
	defer o.mu.Unlock()

	id := uint64(reflect.ValueOf(comp).Pointer())
	if cfg, ok := o.idToCfg[id]; ok {
		return cfg
	}

	return CompCfg{}
}

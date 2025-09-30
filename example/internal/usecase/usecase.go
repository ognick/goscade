package usecase

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"strings"
	"sync"
	"time"

	"github.com/ognick/goscade"
	"github.com/ognick/goscade/example/internal/components"
	"github.com/ognick/goscade/example/internal/domain"
)

type logger interface {
	Infof(format string, args ...interface{})
	Errorf(format string, args ...interface{})
}

type graph struct {
	ctx                  context.Context
	shutdown             context.CancelFunc
	observer             *components.Observer
	waitGracefulShutdown func() error
	log                  logger
	lc                   goscade.Lifecycle
}

func newGraph(log logger) *graph {
	ctx, shutdown := context.WithCancel(context.Background())
	observer := components.NewObserver()
	lc := goscade.NewLifecycle(log)

	// infra
	web := goscade.Register(lc, components.NewWebServer(observer))
	redis := goscade.Register(lc, components.NewRedisClient(observer))
	cache := goscade.Register(lc, components.NewLruCache(observer, redis))
	postgres := goscade.Register(lc, components.NewPostgresqlClient(observer))
	kafka := goscade.Register(lc, components.NewKafkaClient(observer))
	listener := goscade.Register(lc, components.NewKafkaListener(observer, kafka))
	// repos
	bookRepo := goscade.Register(lc, components.NewBookRepo(observer, cache, listener))
	userRepo := goscade.Register(lc, components.NewUserRepo(observer, postgres))
	// apis
	bookAPI := goscade.Register(lc, components.NewBookAPI(observer, web))
	userAPI := goscade.Register(lc, components.NewUserAPI(observer, listener))
	// services
	bookSrv := goscade.Register(lc, components.NewBookService(observer, bookAPI, bookRepo))
	goscade.Register(lc, components.NewUserService(observer, userAPI, userRepo, bookSrv))

	return &graph{
		ctx:      ctx,
		shutdown: shutdown,
		observer: observer,
		log:      log,
		lc:       lc,
	}
}

type Usecase struct {
	idToGraph map[string]*graph
	mu        sync.Mutex
	log       logger
}

func NewUsecase(log logger) *Usecase {
	return &Usecase{
		idToGraph: make(map[string]*graph),
		log:       log,
	}
}

func (u *Usecase) acquireGraph(id string, renewIfFinished bool) *graph {
	u.mu.Lock()
	defer u.mu.Unlock()

	g, ok := u.idToGraph[id]
	if !ok || (g.lc.Status() == goscade.LifecycleStatusStopped && renewIfFinished) {
		g = newGraph(u.log)
		u.idToGraph[id] = g
	}

	return g
}

func (u *Usecase) Graph(_ context.Context, graphID string) (domain.Graph, error) {
	graph := u.acquireGraph(graphID, false)
	comps := make([]domain.Component, 0)
	for comp, deps := range graph.lc.Dependencies() {
		val := reflect.ValueOf(comp)
		dependsOn := make([]uint64, len(deps))
		for i, dep := range deps {
			dependsOn[i] = uint64(reflect.ValueOf(dep).Pointer())
		}

		cfg := graph.observer.GetCfg(comp)
		name := strings.ReplaceAll(strings.ReplaceAll(val.Type().String(), "*components.", ""),
			"*github.com/ognick/goscade/example/internal/components.",
			"")
		comps = append(comps, domain.Component{
			ID:        uint64(val.Pointer()),
			Name:      name,
			DependsOn: dependsOn,
			Status:    string(graph.observer.GetStatus(comp)),
			Error:     cfg.Err,
			Delay:     cfg.Delay,
		})
	}

	return domain.Graph{
		ID:         graphID,
		Components: comps,
		Status:     string(graph.lc.Status()),
	}, nil
}

func (u *Usecase) StartAll(_ context.Context, graphID string) error {
	graph := u.acquireGraph(graphID, true)
	status := graph.lc.Status()
	if status != goscade.LifecycleStatusIdle && status != goscade.LifecycleStatusStopped {
		return fmt.Errorf("graph %s has status %s", graphID, status)
	}

	graph.waitGracefulShutdown = graph.lc.Run(graph.ctx, func(err error) {
		if err != nil {
			u.log.Errorf("Run graph:%s probe: %v", graphID, err)
			return
		}
		u.log.Infof("Graph %s is ready", graphID)
	})
	return nil
}

func (u *Usecase) StopAll(ctx context.Context, graphID string) error {
	graph := u.acquireGraph(graphID, false)
	status := graph.lc.Status()
	if status != goscade.LifecycleStatusReady {
		return fmt.Errorf("graph %s has status %s", graphID, status)
	}
	graph.shutdown()
	err := graph.waitGracefulShutdown()
	if err != nil && !errors.Is(err, context.Canceled) {
		return fmt.Errorf("graph %s shutdown: %w", graphID, err)
	}

	return nil
}

func (u *Usecase) UpdateComponent(ctx context.Context, graphID, compID string, delay time.Duration, err *string) error {
	graph := u.acquireGraph(graphID, false)
	return graph.observer.UpdateComponent(compID, components.CompCfg{Err: err, Delay: delay})
}

func (u *Usecase) KillComponent(ctx context.Context, graphID, compID string) error {
	graph := u.acquireGraph(graphID, false)
	return graph.observer.KillComponent(compID)
}

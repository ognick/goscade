package usecase

import (
	"context"
	"fmt"
	"reflect"
	"strings"
	"sync"
	"time"

	"github.com/ognick/goscade"
	"github.com/ognick/goscade/example/internal/components"
	"github.com/ognick/goscade/example/internal/domain"
	"golang.org/x/sync/errgroup"
)

type logger interface {
	Infof(format string, args ...interface{})
	Errorf(format string, args ...interface{})
}

type graph struct {
	gracefulCtx      context.Context
	gracefulShutdown context.CancelFunc
	runner           *errgroup.Group
	observer         *components.Observer
	log              logger
	lc               goscade.Lifecycle
}

func newGraph(log logger) *graph {
	gracefulCtx, gracefulShutdown := context.WithCancel(context.Background())
	runner := &errgroup.Group{}
	observer := components.NewObserver()
	lc := goscade.NewLifecycle(log)

	// infra
	web := goscade.RegisterComponent(lc, components.NewWebServer(observer))
	redis := goscade.RegisterComponent(lc, components.NewRedisClient(observer))
	cache := goscade.RegisterComponent(lc, components.NewLruCache(observer, redis))
	postgres := goscade.RegisterComponent(lc, components.NewPostgresqlClient(observer))
	kafka := goscade.RegisterComponent(lc, components.NewKafkaClient(observer))
	listener := goscade.RegisterComponent(lc, components.NewKafkaListener(observer, kafka))
	// repos
	bookRepo := goscade.RegisterComponent(lc, components.NewBookRepo(observer, cache, listener))
	userRepo := goscade.RegisterComponent(lc, components.NewUserRepo(observer, postgres))
	// apis
	bookAPI := goscade.RegisterComponent(lc, components.NewBookAPI(observer, web))
	userAPI := goscade.RegisterComponent(lc, components.NewUserAPI(observer, listener))
	// services
	bookSrv := goscade.RegisterComponent(lc, components.NewBookService(observer, bookAPI, bookRepo))
	goscade.RegisterComponent(lc, components.NewUserService(observer, userAPI, userRepo, bookSrv))

	return &graph{
		gracefulCtx:      gracefulCtx,
		gracefulShutdown: gracefulShutdown,
		runner:           runner,
		observer:         observer,
		log:              log,
		lc:               lc,
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
		comps = append(comps, domain.Component{
			ID:        uint64(val.Pointer()),
			Name:      strings.ReplaceAll(val.Type().String(), "*components.", ""),
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

	graph.lc.RunAllComponents(graph.runner, graph.gracefulCtx)
	return nil
}

func (u *Usecase) StopAll(ctx context.Context, graphID string) error {
	graph := u.acquireGraph(graphID, false)
	status := graph.lc.Status()
	if status != goscade.LifecycleStatusReady {
		return fmt.Errorf("graph %s has status %s", graphID, status)
	}
	graph.gracefulShutdown()
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

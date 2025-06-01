package components

import (
	"context"

	"github.com/ognick/goscade"
)

type RedisClient struct {
	observer *Observer
	deps     []goscade.Component
}

func NewRedisClient(observer *Observer, deps ...goscade.Component) goscade.Component {
	return observer.Register(&RedisClient{observer: observer, deps: deps})
}

func (c *RedisClient) Run(ctx context.Context, readinessProbe func(err error)) error {
	return c.observer.run(ctx, c, readinessProbe)
}

type LruCache struct {
	observer *Observer
	deps     []goscade.Component
}

func NewLruCache(observer *Observer, deps ...goscade.Component) goscade.Component {
	return observer.Register(&LruCache{observer: observer, deps: deps})
}

func (c *LruCache) Run(ctx context.Context, readinessProbe func(err error)) error {
	return c.observer.run(ctx, c, readinessProbe)
}

type PostgresqlClient struct {
	observer *Observer
	deps     []goscade.Component
}

func NewPostgresqlClient(observer *Observer, deps ...goscade.Component) goscade.Component {
	return observer.Register(&PostgresqlClient{observer: observer, deps: deps})
}

func (c *PostgresqlClient) Run(ctx context.Context, readinessProbe func(err error)) error {
	return c.observer.run(ctx, c, readinessProbe)
}

type KafkaClient struct {
	observer *Observer
	deps     []goscade.Component
}

func NewKafkaClient(observer *Observer, deps ...goscade.Component) goscade.Component {
	return observer.Register(&KafkaClient{observer: observer, deps: deps})
}

func (c *KafkaClient) Run(ctx context.Context, readinessProbe func(err error)) error {
	return c.observer.run(ctx, c, readinessProbe)
}

type KafkaListener struct {
	observer *Observer
	deps     []goscade.Component
}

func NewKafkaListener(observer *Observer, deps ...goscade.Component) goscade.Component {
	return observer.Register(&KafkaListener{observer: observer, deps: deps})
}

func (c *KafkaListener) Run(ctx context.Context, readinessProbe func(err error)) error {
	return c.observer.run(ctx, c, readinessProbe)
}

type WebServer struct {
	observer *Observer
	deps     []goscade.Component
}

func NewWebServer(observer *Observer, deps ...goscade.Component) goscade.Component {
	return observer.Register(&WebServer{observer: observer, deps: deps})
}

func (c *WebServer) Run(ctx context.Context, readinessProbe func(err error)) error {
	return c.observer.run(ctx, c, readinessProbe)
}

type UserRepo struct {
	observer *Observer
	deps     []goscade.Component
}

func NewUserRepo(observer *Observer, deps ...goscade.Component) goscade.Component {
	return observer.Register(&UserRepo{observer: observer, deps: deps})
}

func (c *UserRepo) Run(ctx context.Context, readinessProbe func(err error)) error {
	return c.observer.run(ctx, c, readinessProbe)
}

type BookRepo struct {
	observer *Observer
	deps     []goscade.Component
}

func NewBookRepo(observer *Observer, deps ...goscade.Component) goscade.Component {
	return observer.Register(&BookRepo{observer: observer, deps: deps})
}

func (c *BookRepo) Run(ctx context.Context, readinessProbe func(err error)) error {
	return c.observer.run(ctx, c, readinessProbe)
}

type RoomRepo struct {
	observer *Observer
	deps     []goscade.Component
}

func NewRoomRepo(observer *Observer, deps ...goscade.Component) goscade.Component {
	return observer.Register(&RoomRepo{observer: observer, deps: deps})
}

func (c *RoomRepo) Run(ctx context.Context, readinessProbe func(err error)) error {
	return c.observer.run(ctx, c, readinessProbe)
}

type UserAPI struct {
	observer *Observer
	deps     []goscade.Component
}

func NewUserAPI(observer *Observer, deps ...goscade.Component) goscade.Component {
	return observer.Register(&UserAPI{observer: observer, deps: deps})
}

func (c *UserAPI) Run(ctx context.Context, readinessProbe func(err error)) error {
	return c.observer.run(ctx, c, readinessProbe)
}

type BookAPI struct {
	observer *Observer
	deps     []goscade.Component
}

func NewBookAPI(observer *Observer, deps ...goscade.Component) goscade.Component {
	return observer.Register(&BookAPI{observer: observer, deps: deps})
}

func (c *BookAPI) Run(ctx context.Context, readinessProbe func(err error)) error {
	return c.observer.run(ctx, c, readinessProbe)
}

type RoomAPI struct {
	observer *Observer
	deps     []goscade.Component
}

func NewRoomAPI(observer *Observer, deps ...goscade.Component) goscade.Component {
	return observer.Register(&RoomAPI{observer: observer, deps: deps})
}

func (c *RoomAPI) Run(ctx context.Context, readinessProbe func(err error)) error {
	return c.observer.run(ctx, c, readinessProbe)
}

type UserService struct {
	observer *Observer
	deps     []goscade.Component
}

func NewUserService(observer *Observer, deps ...goscade.Component) goscade.Component {
	return observer.Register(&UserService{observer: observer, deps: deps})
}

func (c *UserService) Run(ctx context.Context, readinessProbe func(err error)) error {
	return c.observer.run(ctx, c, readinessProbe)
}

type BookService struct {
	observer *Observer
	deps     []goscade.Component
}

func NewBookService(observer *Observer, deps ...goscade.Component) goscade.Component {
	return observer.Register(&BookService{observer: observer, deps: deps})
}

func (c *BookService) Run(ctx context.Context, readinessProbe func(err error)) error {
	return c.observer.run(ctx, c, readinessProbe)
}

type RoomService struct {
	observer *Observer
	deps     []goscade.Component
}

func NewRoomService(observer *Observer, deps ...goscade.Component) goscade.Component {
	return observer.Register(&RoomService{observer: observer, deps: deps})
}

func (c *RoomService) Run(ctx context.Context, readinessProbe func(err error)) error {
	return c.observer.run(ctx, c, readinessProbe)
}

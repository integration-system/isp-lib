package redis

import (
	"context"
	"time"

	"github.com/go-redis/redis/v8"
	"github.com/integration-system/isp-lib/v2/structure"
)

type DB int

const (
	ApplicationTokenDb DB = iota
	ApplicationPermissionDb
	UserTokenDb
	UserPermissionDb
	DeviceTokenDb
	DevicePermissionDb

	defaultTimeout = 5 * time.Second
)

type Client struct {
	*redis.Client
}

func (c *Client) UseDb(dbIndex DB, f func(p redis.Pipeliner) error) ([]redis.Cmder, error) {
	return c.Pipelined(context.Background(), useDb(dbIndex, f))
}

func (c *Client) UseDbTx(dbIndex DB, f func(p redis.Pipeliner) error) ([]redis.Cmder, error) {
	return c.TxPipelined(context.Background(), useDb(dbIndex, f))
}

func NewClient(cfg structure.RedisConfiguration) (*Client, error) {
	c := newClient(cfg)
	ctx, cancel := context.WithTimeout(context.Background(), defaultTimeout)
	defer cancel()
	err := c.Ping(ctx).Err()
	if err != nil {
		return nil, err
	}

	return &Client{Client: c}, nil
}

func useDb(dbIndex DB, f func(p redis.Pipeliner) error) func(p redis.Pipeliner) error {
	return func(p redis.Pipeliner) error {
		err := p.Select(context.Background(), int(dbIndex)).Err()
		if err != nil {
			return err
		}
		return f(p)
	}
}

func newClient(cfg structure.RedisConfiguration) *redis.Client {
	sentinel := cfg.Sentinel
	if sentinel != nil {
		return redis.NewFailoverClient(&redis.FailoverOptions{
			MasterName:       sentinel.MasterName,
			SentinelAddrs:    sentinel.SentinelAddresses,
			SentinelPassword: sentinel.SentinelPassword,
			Username:         cfg.Username,
			Password:         cfg.Password,
			DB:               cfg.DefaultDB,
		})
	}
	return redis.NewClient(&redis.Options{
		Addr:     cfg.Address.GetAddress(),
		Username: cfg.Username,
		Password: cfg.Password,
		DB:       cfg.DefaultDB,
	})
}

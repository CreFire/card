package redis

import (
	"context"
	"fmt"
	"time"

	"backend/src/common/configdoc"

	goredis "github.com/redis/go-redis/v9"
)

type Client struct {
	raw goredis.UniversalClient
}

func NewClient(cfg configdoc.RedisConfig, dialTimeout, readTimeout, writeTimeout time.Duration) (*Client, error) {
	options := &goredis.UniversalOptions{
		Addrs:        cfg.Addresses,
		Username:     cfg.Username,
		Password:     cfg.Password,
		DB:           cfg.DB,
		PoolSize:     cfg.PoolSize,
		MinIdleConns: cfg.MinIdleConns,
		DialTimeout:  dialTimeout,
		ReadTimeout:  readTimeout,
		WriteTimeout: writeTimeout,
	}

	client := goredis.NewUniversalClient(options)
	if err := client.Ping(context.Background()).Err(); err != nil {
		_ = client.Close()
		return nil, fmt.Errorf("ping redis: %w", err)
	}

	return &Client{raw: client}, nil
}

func NewFromConfig(cfg *configdoc.ConfigBase) (*Client, error) {
	return NewClient(
		cfg.Redis,
		cfg.RedisDialTimeout(),
		cfg.RedisReadTimeout(),
		cfg.RedisWriteTimeout(),
	)
}

func (c *Client) Raw() goredis.UniversalClient {
	return c.raw
}

func (c *Client) Ping(ctx context.Context) error {
	return c.raw.Ping(ctx).Err()
}

func (c *Client) Close() error {
	return c.raw.Close()
}

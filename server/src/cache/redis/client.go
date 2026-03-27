package redis

import (
	"time"

	"backend/src/common/configdoc"
	persistredis "backend/src/persist/redis"
)

type Client = persistredis.Client

func NewClient(cfg configdoc.RedisConfig, dialTimeout, readTimeout, writeTimeout time.Duration) (*Client, error) {
	return persistredis.NewClient(cfg, dialTimeout, readTimeout, writeTimeout)
}

func NewFromConfig(cfg *configdoc.ConfigBase) (*Client, error) {
	return persistredis.NewFromConfig(cfg)
}

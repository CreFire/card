package configdoc

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/spf13/viper"
)

type ConfigBase struct {
	App     AppConfig     `mapstructure:"app"`
	Server  ServerConfig  `mapstructure:"server"`
	Etcd    EtcdConfig    `mapstructure:"etcd"`
	Redis   RedisConfig   `mapstructure:"redis"`
	MongoDB MongoDBConfig `mapstructure:"mongodb"`
	Log     LogConfig     `mapstructure:"log"`
}

type AppConfig struct {
	Env string `mapstructure:"env"`
}

type ServerConfig struct {
	Name     string `mapstructure:"name"`
	Host     string `mapstructure:"host"`
	Port     int    `mapstructure:"port"`
	TCPPort  int    `mapstructure:"tcp_port"`
	GRPCPort int    `mapstructure:"grpc_port"`
}

type EtcdConfig struct {
	Endpoints       []string `mapstructure:"endpoints"`
	DialTimeoutSec  int      `mapstructure:"dial_timeout_sec"`
	LeaseTTLSec     int64    `mapstructure:"lease_ttl_sec"`
	ServicePrefix   string   `mapstructure:"service_prefix"`
	AdvertiseHost   string   `mapstructure:"advertise_host"`
	AdvertisePort   int      `mapstructure:"advertise_port"`
	EnableDiscovery bool     `mapstructure:"enable_discovery"`
}

type RedisConfig struct {
	Enabled         bool     `mapstructure:"enabled"`
	Addresses       []string `mapstructure:"addresses"`
	Username        string   `mapstructure:"username"`
	Password        string   `mapstructure:"password"`
	DB              int      `mapstructure:"db"`
	DialTimeoutSec  int      `mapstructure:"dial_timeout_sec"`
	ReadTimeoutSec  int      `mapstructure:"read_timeout_sec"`
	WriteTimeoutSec int      `mapstructure:"write_timeout_sec"`
	PoolSize        int      `mapstructure:"pool_size"`
	MinIdleConns    int      `mapstructure:"min_idle_conns"`
}

type MongoDBConfig struct {
	Enabled            bool   `mapstructure:"enabled"`
	URI                string `mapstructure:"uri"`
	Database           string `mapstructure:"database"`
	ConnectTimeoutSec  int    `mapstructure:"connect_timeout_sec"`
	ServerSelectTimout int    `mapstructure:"server_selection_timeout_sec"`
	MaxPoolSize        uint64 `mapstructure:"max_pool_size"`
	MinPoolSize        uint64 `mapstructure:"min_pool_size"`
}

type LogConfig struct {
	Level         string `mapstructure:"level"`
	Path          string `mapstructure:"path"`
	Rotation      string `mapstructure:"rotation"`
	MaxFileSizeMB int    `mapstructure:"max_file_size_mb"`
	RetentionDays int    `mapstructure:"retention_days"`
	StdOut        bool   `mapstructure:"stdout"`
	FileOut       bool   `mapstructure:"fileout"`
	Sync          bool   `mapstructure:"sync"`
	Skip          int    `mapstructure:"skip"`
}

func Load(path, env string) (*ConfigBase, error) {
	absPath, err := filepath.Abs(path)
	if err != nil {
		return nil, fmt.Errorf("resolve config path %s: %w", path, err)
	}

	v := viper.New()
	v.SetConfigFile(absPath)
	v.SetConfigType("yaml")
	v.AutomaticEnv()

	if err := v.ReadInConfig(); err != nil {
		return nil, fmt.Errorf("read config %s: %w", absPath, err)
	}
	if env = resolveEnv(env); env != "" {
		overridePath := envConfigPath(absPath, env)
		if _, err := os.Stat(overridePath); err == nil {
			v.SetConfigFile(overridePath)
			if err := v.MergeInConfig(); err != nil {
				return nil, fmt.Errorf("merge config %s: %w", overridePath, err)
			}
		} else if !os.IsNotExist(err) {
			return nil, fmt.Errorf("stat env config %s: %w", overridePath, err)
		}
	}

	var cfg ConfigBase
	if err := v.Unmarshal(&cfg); err != nil {
		return nil, fmt.Errorf("parse config %s: %w", absPath, err)
	}

	cfg.App.Env = resolveEnv(cfg.App.Env)
	if env != "" {
		cfg.App.Env = env
	}
	cfg.applyDefaults()
	cfg.Log.Path = absolutePath(cfg.Log.Path)

	if cfg.Server.Name == "" {
		return nil, fmt.Errorf("server.name is required")
	}
	if cfg.Server.Host == "" {
		return nil, fmt.Errorf("server.host is required")
	}
	if cfg.Server.Port <= 0 {
		return nil, fmt.Errorf("server.port must be greater than zero")
	}
	if cfg.Redis.Enabled && len(cfg.Redis.Addresses) == 0 {
		return nil, fmt.Errorf("redis.addresses is required when redis.enabled=true")
	}
	if cfg.MongoDB.Enabled {
		if cfg.MongoDB.URI == "" {
			return nil, fmt.Errorf("mongodb.uri is required when mongodb.enabled=true")
		}
		if cfg.MongoDB.Database == "" {
			return nil, fmt.Errorf("mongodb.database is required when mongodb.enabled=true")
		}
	}

	return &cfg, nil
}

func (c *ConfigBase) applyDefaults() {
	if c.App.Env == "" {
		c.App.Env = "dev"
	}
	if c.Etcd.DialTimeoutSec <= 0 {
		c.Etcd.DialTimeoutSec = 5
	}
	if c.Etcd.LeaseTTLSec <= 0 {
		c.Etcd.LeaseTTLSec = 10
	}
	if c.Etcd.ServicePrefix == "" {
		c.Etcd.ServicePrefix = "/wuziqi/services"
	}
	if c.Etcd.AdvertiseHost == "" {
		c.Etcd.AdvertiseHost = c.Server.Host
	}
	if c.Etcd.AdvertisePort <= 0 {
		c.Etcd.AdvertisePort = c.Server.Port
	}
	if c.Server.TCPPort <= 0 {
		c.Server.TCPPort = c.Server.Port + 1000
	}
	if c.Server.GRPCPort <= 0 {
		c.Server.GRPCPort = c.Server.Port + 2000
	}
	if c.Redis.DialTimeoutSec <= 0 {
		c.Redis.DialTimeoutSec = 5
	}
	if c.Redis.ReadTimeoutSec <= 0 {
		c.Redis.ReadTimeoutSec = 3
	}
	if c.Redis.WriteTimeoutSec <= 0 {
		c.Redis.WriteTimeoutSec = 3
	}
	if c.Redis.PoolSize <= 0 {
		c.Redis.PoolSize = 10
	}
	if c.MongoDB.ConnectTimeoutSec <= 0 {
		c.MongoDB.ConnectTimeoutSec = 5
	}
	if c.MongoDB.ServerSelectTimout <= 0 {
		c.MongoDB.ServerSelectTimout = 5
	}
	if c.Log.Level == "" {
		if c.IsProd() {
			c.Log.Level = "info"
		} else {
			c.Log.Level = "debug"
		}
	}
	if c.Log.Path == "" {
		c.Log.Path = filepath.Join("logs", c.Server.Name+".log")
	}
	if c.Log.Rotation == "" {
		c.Log.Rotation = "daily"
	}
	if c.Log.MaxFileSizeMB <= 0 {
		c.Log.MaxFileSizeMB = 10
	}
	if c.Log.RetentionDays <= 0 {
		if c.IsProd() {
			c.Log.RetentionDays = 30
		} else {
			c.Log.RetentionDays = 7
		}
	}
	if c.Log.Skip <= 0 {
		c.Log.Skip = 2
	}
	if !c.Log.FileOut && !c.Log.StdOut {
		c.Log.FileOut = true
		if !c.IsProd() {
			c.Log.StdOut = true
		}
	}
}

func (c ConfigBase) ListenAddress() string {
	return fmt.Sprintf("%s:%d", c.Server.Host, c.Server.Port)
}

func (c ConfigBase) TCPListenAddress() string {
	return fmt.Sprintf("%s:%d", c.Server.Host, c.Server.TCPPort)
}

func (c ConfigBase) GRPCListenAddress() string {
	return fmt.Sprintf("%s:%d", c.Server.Host, c.Server.GRPCPort)
}

func (c ConfigBase) AdvertiseAddress() string {
	return fmt.Sprintf("%s:%d", c.Etcd.AdvertiseHost, c.Etcd.AdvertisePort)
}

func (c ConfigBase) HasEtcd() bool {
	return len(c.Etcd.Endpoints) > 0
}

func (c ConfigBase) DialTimeout() time.Duration {
	return time.Duration(c.Etcd.DialTimeoutSec) * time.Second
}

func (c ConfigBase) RedisDialTimeout() time.Duration {
	return time.Duration(c.Redis.DialTimeoutSec) * time.Second
}

func (c ConfigBase) RedisReadTimeout() time.Duration {
	return time.Duration(c.Redis.ReadTimeoutSec) * time.Second
}

func (c ConfigBase) RedisWriteTimeout() time.Duration {
	return time.Duration(c.Redis.WriteTimeoutSec) * time.Second
}

func (c ConfigBase) MongoConnectTimeout() time.Duration {
	return time.Duration(c.MongoDB.ConnectTimeoutSec) * time.Second
}

func (c ConfigBase) MongoServerSelectionTimeout() time.Duration {
	return time.Duration(c.MongoDB.ServerSelectTimout) * time.Second
}

func (c ConfigBase) IsProd() bool {
	return c.App.Env == "prod"
}

func resolveEnv(env string) string {
	if env != "" {
		return env
	}
	if value := os.Getenv("APP_ENV"); value != "" {
		return value
	}
	return "dev"
}

func envConfigPath(basePath, env string) string {
	ext := filepath.Ext(basePath)
	base := basePath[:len(basePath)-len(ext)]
	return base + "." + env + ext
}

func absolutePath(path string) string {
	if path == "" {
		return ""
	}

	absPath, err := filepath.Abs(path)
	if err != nil {
		return path
	}

	return absPath
}

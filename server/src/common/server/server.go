package server

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"backend/deps/xlog"
	"backend/src/common/configdoc"
	"backend/src/common/discovery"
	"backend/src/common/logger"
	mongostore "backend/src/persist/mongo"
	redisstore "backend/src/persist/redis"
)

type Server struct {
	Service
	confBase *configdoc.ConfigBase
}

func Run(serviceName, defaultConfigPath string, services ...Service) error {
	configPath := flag.String("config", defaultConfigPath, "path to service config file")
	env := flag.String("env", "", "runtime environment, such as dev or prod")
	flag.Parse()

	absConfigPath, err := filepath.Abs(*configPath)
	if err != nil {
		return fmt.Errorf("resolve config path %s: %w", *configPath, err)
	}

	cfg, err := configdoc.Load(absConfigPath, *env)
	if err != nil {
		return err
	}

	if cfg.Server.Name != serviceName {
		return fmt.Errorf("config server.name=%s does not match service %s", cfg.Server.Name, serviceName)
	}

	logger.Init(cfg)
	defer logger.Close()
	xlog.Infof("[%s] logger initialized env=%s path=%s level=%s", serviceName, cfg.App.Env, cfg.Log.Path, cfg.Log.Level)

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	var redisClient *redisstore.Client
	if cfg.Redis.Enabled {
		redisClient, err = redisstore.NewFromConfig(cfg)
		if err != nil {
			return err
		}
		defer func() {
			if closeErr := redisClient.Close(); closeErr != nil {
				xlog.Errorf("[%s] close redis: %v", serviceName, closeErr)
			}
		}()
		xlog.Infof("[%s] redis connected: %v", serviceName, cfg.Redis.Addresses)
	}

	var mongoClient *mongostore.Client
	if cfg.MongoDB.Enabled {
		mongoClient, err = mongostore.NewFromConfig(cfg)
		if err != nil {
			return err
		}
		defer func() {
			closeCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			if closeErr := mongoClient.Close(closeCtx); closeErr != nil {
				xlog.Errorf("[%s] close mongodb: %v", serviceName, closeErr)
			}
		}()
		xlog.Infof("[%s] mongodb connected: %s/%s", serviceName, cfg.MongoDB.URI, cfg.MongoDB.Database)
	}

	var registry *discovery.Registry
	if cfg.HasEtcd() {
		registry, err = discovery.NewRegistry(ctx, cfg.Etcd.Endpoints, cfg.DialTimeout(), cfg.Etcd.ServicePrefix)
		if err != nil {
			return err
		}
		defer registry.Close()

		instance := discovery.Instance{
			ID:        fmt.Sprintf("%s-%d", serviceName, time.Now().UnixNano()),
			Name:      serviceName,
			Address:   cfg.AdvertiseAddress(),
			StartedAt: time.Now(),
			Metadata: map[string]string{
				"config_path": absConfigPath,
			},
		}

		if err := registry.Register(ctx, serviceName, cfg.Etcd.LeaseTTLSec, instance); err != nil {
			return err
		}

		xlog.Infof("[%s] registered to etcd: %s", serviceName, instance.Address)
	}

	runtime := &Runtime{
		ServiceName: serviceName,
		Config:      cfg,
		Registry:    registry,
		Redis:       redisClient,
		Mongo:       mongoClient,
	}
	for _, svc := range services {
		runtimeAware, ok := svc.(RuntimeAware)
		if !ok {
			continue
		}
		if err := runtimeAware.SetRuntime(runtime); err != nil {
			return fmt.Errorf("configure runtime for service %s: %w", serviceName, err)
		}
	}

	xlog.Infof("[%s] listening on %s", serviceName, cfg.ListenAddress())
	if err := runHTTP(ctx, serviceName, cfg, registry, services...); err != nil {
		return err
	}
	xlog.Infof("[%s] shutting down", serviceName)
	return nil
}

func ExitOnError(serviceName, defaultConfigPath string, services ...Service) {
	if err := Run(serviceName, defaultConfigPath, services...); err != nil {
		xlog.Errorf("[%s] startup failed: %v", serviceName, err)
		os.Exit(1)
	}
}

func stopServices(serviceName string, services []Service) error {
	for i := len(services) - 1; i >= 0; i-- {
		svc := services[i]
		if err := svc.BeforeStop(); err != nil {
			return fmt.Errorf("before stop service %s: %w", serviceName, err)
		}
		if err := svc.Stop(); err != nil {
			return fmt.Errorf("stop service %s: %w", serviceName, err)
		}
		if err := svc.AfterStop(); err != nil {
			return fmt.Errorf("after stop service %s: %w", serviceName, err)
		}
	}
	return nil
}

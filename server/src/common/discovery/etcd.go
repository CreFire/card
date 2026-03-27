package discovery

import (
	"context"
	"encoding/json"
	"fmt"
	"path"
	"strings"
	"time"

	clientv3 "go.etcd.io/etcd/client/v3"
)

type Instance struct {
	ID        string            `json:"id"`
	Name      string            `json:"name"`
	Address   string            `json:"address"`
	Metadata  map[string]string `json:"metadata,omitempty"`
	StartedAt time.Time         `json:"started_at"`
}

type Registry struct {
	client *clientv3.Client
	prefix string
}

func NewRegistry(ctx context.Context, endpoints []string, dialTimeout time.Duration, prefix string) (*Registry, error) {
	client, err := clientv3.New(clientv3.Config{
		Context:     ctx,
		Endpoints:   endpoints,
		DialTimeout: dialTimeout,
	})
	if err != nil {
		return nil, fmt.Errorf("create etcd client: %w", err)
	}

	return &Registry{client: client, prefix: strings.TrimRight(prefix, "/")}, nil
}

func (r *Registry) Close() error {
	return r.client.Close()
}

func (r *Registry) Register(ctx context.Context, serviceName string, ttl int64, instance Instance) error {
	payload, err := json.Marshal(instance)
	if err != nil {
		return fmt.Errorf("marshal instance: %w", err)
	}

	leaseResp, err := r.client.Grant(ctx, ttl)
	if err != nil {
		return fmt.Errorf("grant lease: %w", err)
	}

	key := r.serviceKey(serviceName, instance.ID)
	if _, err := r.client.Put(ctx, key, string(payload), clientv3.WithLease(leaseResp.ID)); err != nil {
		return fmt.Errorf("put service key: %w", err)
	}

	keepAliveCh, err := r.client.KeepAlive(ctx, leaseResp.ID)
	if err != nil {
		return fmt.Errorf("keepalive lease: %w", err)
	}

	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			case _, ok := <-keepAliveCh:
				if !ok {
					return
				}
			}
		}
	}()

	return nil
}

func (r *Registry) Discover(ctx context.Context, serviceName string) ([]Instance, error) {
	key := r.serviceDir(serviceName)
	resp, err := r.client.Get(ctx, key, clientv3.WithPrefix())
	if err != nil {
		return nil, fmt.Errorf("get service instances: %w", err)
	}

	instances := make([]Instance, 0, len(resp.Kvs))
	for _, kv := range resp.Kvs {
		var instance Instance
		if err := json.Unmarshal(kv.Value, &instance); err != nil {
			return nil, fmt.Errorf("decode instance %s: %w", string(kv.Key), err)
		}
		instances = append(instances, instance)
	}

	return instances, nil
}

func (r *Registry) serviceDir(serviceName string) string {
	return path.Join(r.prefix, serviceName)
}

func (r *Registry) serviceKey(serviceName, instanceID string) string {
	return path.Join(r.serviceDir(serviceName), instanceID)
}

package etcd

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"maps"
	"strings"
	"sync"

	clientv3 "go.etcd.io/etcd/client/v3"
)

// Registrar is service registrar.
type Registrar interface {
	// Register the registration.
	Register(ctx context.Context, service *ServiceInstance) error
	// Deregister the registration.
	Deregister(ctx context.Context, service *ServiceInstance) error
}

// Discovery is service discovery.
type Discovery interface {
	// GetService return the service instances in memory according to the service name.
	GetService(ctx context.Context, serviceName string) ([]*ServiceInstance, error)
	// Watch creates a watcher according to the service name.
	Watch(ctx context.Context, serviceName string) (<-chan *WatchResponse, error)
}

// RegistryDiscovery implements both Registrar and Discovery interfaces.
type RegistryDiscovery struct {
	client     *EtcdClient
	mu         sync.Mutex
	registrars map[string]*ServiceRegistrar
	regist     bool
	watcher    []*ClientWatcher
}

// NewRegistry constructs a registry backed by the provided EtcdClient.
func NewRegistry(client *EtcdClient, regist bool) *RegistryDiscovery {
	return &RegistryDiscovery{
		client:     client,
		registrars: make(map[string]*ServiceRegistrar),
		regist:     regist,
	}
}
func (r *RegistryDiscovery) Close() {
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, v := range r.registrars {
		v.Close()
	}
	clear(r.registrars)
	for _, v := range r.watcher {
		v.Close()
	}
	r.watcher = nil
}

// Register registers a service instance and starts its heartbeat.
func (r *RegistryDiscovery) Register(ctx context.Context, service *ServiceInstance) error {
	if r.regist == false {
		return nil
	}
	if service == nil {
		return errors.New("service instance is nil")
	}
	if r == nil || r.client == nil || r.client.Client == nil {
		return errors.New("etcd client is nil")
	}
	if ctx != nil && ctx.Err() != nil {
		return ctx.Err()
	}

	key := RegistrarKey(service.ClusterName, service.ServiceName, service.InstanceId)
	r.mu.Lock()
	if _, ok := r.registrars[key]; ok {
		r.mu.Unlock()
		return fmt.Errorf("service already registered: %s", key)
	}
	registrar := NewServiceRegistrar(r.client, service.ClusterName, service)
	r.registrars[key] = registrar
	r.mu.Unlock()

	if err := registrar.Register(); err != nil {
		r.mu.Lock()
		delete(r.registrars, key)
		r.mu.Unlock()
		return err
	}
	return nil
}

// Deregister removes a service instance from etcd.
func (r *RegistryDiscovery) Deregister(ctx context.Context, service *ServiceInstance) error {
	if r.regist == false {
		return nil
	}

	if service == nil {
		return errors.New("service instance is nil")
	}
	if r == nil || r.client == nil || r.client.Client == nil {
		return errors.New("etcd client is nil")
	}
	if ctx != nil && ctx.Err() != nil {
		return ctx.Err()
	}

	key := RegistrarKey(service.ClusterName, service.ServiceName, service.InstanceId)
	r.mu.Lock()
	registrar := r.registrars[key]
	if registrar != nil {
		delete(r.registrars, key)
	}
	r.mu.Unlock()

	if registrar != nil {
		registrar.Close()
		return nil
	}

	_, err := r.client.Client.Delete(ctxOrBackground(ctx), key)
	return err
}

// GetService returns service instances under the given service name prefix.
func (r *RegistryDiscovery) GetService(ctx context.Context, serviceName string) ([]*ServiceInstance, error) {
	if r == nil || r.client == nil || r.client.Client == nil {
		return nil, errors.New("etcd client is nil")
	}
	prefix, err := servicePrefix(serviceName)
	if err != nil {
		return nil, err
	}

	resp, err := r.client.Client.Get(ctxOrBackground(ctx), prefix, clientv3.WithPrefix())
	if err != nil {
		return nil, err
	}

	instances := make([]*ServiceInstance, 0, len(resp.Kvs))
	for _, kv := range resp.Kvs {
		inst, err := unmarshal(kv.Value)
		if err != nil || inst == nil {
			continue
		}
		instances = append(instances, inst)
	}
	return instances, nil
}

// Watch creates a watcher for the given service name prefix.
func (r *RegistryDiscovery) Watch(ctx context.Context, serviceName string) (<-chan *WatchResponse, error) {
	if r == nil || r.client == nil || r.client.Client == nil {
		return nil, errors.New("etcd client is nil")
	}
	prefix, err := servicePrefix(serviceName)
	if err != nil {
		return nil, err
	}
	watcher, err := NewClientWatcher(r.client.Client, prefix, ctxOrBackground(ctx), clientv3.WithPrefix())
	if err != nil {
		return nil, err
	}
	r.watcher = append(r.watcher, watcher)
	return watcher.EventChan(), nil
}

func ctxOrBackground(ctx context.Context) context.Context {
	if ctx == nil {
		return context.Background()
	}
	return ctx
}

func servicePrefix(serviceName string) (string, error) {
	name := strings.TrimSpace(serviceName)
	if name == "" {
		return "", errors.New("service name is empty")
	}
	if !strings.HasPrefix(name, "/") {
		name = "/" + name
	}
	if !strings.HasSuffix(name, "/") {
		name += "/"
	}
	return name, nil
}

// ServiceInstance is an instance of a service in a discovery system.
type ServiceInstance struct {
	Id          int32    `protobuf:"varint,1,opt,name=id,proto3" json:"Id,omitempty"`                                      // @inject_tag: json:"Id"                    // 服务器唯一ID
	Name        string   `protobuf:"bytes,2,opt,name=name,proto3" json:"name,omitempty" bson:"name"`                       // @inject_tag: bson:"name"           // 服务器名称唯一
	Type        string   `protobuf:"bytes,3,opt,name=type,proto3" json:"type,omitempty" bson:"type"`                       // @inject_tag: bson:"type"           // 服务器类型
	IsMaster    bool     `protobuf:"varint,6,opt,name=IsMaster,proto3" json:"IsMaster,omitempty" bson:"isMaster"`          // @inject_tag: bson:"isMaster"         // 是否主服务器
	MasterIndex int64    `protobuf:"varint,7,opt,name=MasterIndex,proto3" json:"MasterIndex,omitempty" bson:"masterIndex"` // @inject_tag: bson:"masterIndex"     // 主服务器索引
	LastTick    int64    `protobuf:"varint,8,opt,name=LastTick,proto3" json:"LastTick,omitempty" bson:"LastTick"`          // @inject_tag: bson:"LastTick"       // 最新心跳时间
	Version     string   `protobuf:"varint,9,opt,name=Version,proto3" json:"Version,omitempty" bson:"version"`             // @inject_tag: bson:"version"   		// 版本号
	Endpoints   []string `protobuf:"bytes,10,rep,name=endpoints,proto3" json:"endpoints,omitempty" bson:"endpoints"`

	ClusterName string            `json:"cluster" bson:"cluster"`
	ServiceName string            `json:"service" bson:"service"`
	InstanceId  int32             `json:"id" bson:"id"`
	Host        string            `json:"host" bson:"host"`
	Port        int32             `json:"port" bson:"port"`
	Healthy     string            `json:"health" bson:"health"`
	Enable      bool              `json:"enable" bson:"enable"`
	OnlineCount int32             `json:"online" bson:"online"`
	Weight      int32             `json:"weight" bson:"weight"`
	MetaData    map[string]string `json:"meta" bson:"meta"`
	UpdateTime  string            `json:"update_time" bson:"update_time"`
}

func (i *ServiceInstance) String() string {
	return fmt.Sprintf("%s-%d", i.Type, i.Id)
}

// Equal returns whether i and o are equivalent.
func (i *ServiceInstance) Equal(o any) bool {
	if i == nil && o == nil {
		return true
	}

	if i == nil || o == nil {
		return false
	}

	t, ok := o.(*ServiceInstance)
	if !ok {
		return false
	}
	if i.InstanceId != t.InstanceId || i.ClusterName != t.ClusterName || i.ServiceName != t.ServiceName ||
		i.Healthy != t.Healthy || i.Enable != t.Enable || i.Host != t.Host || i.Port != t.Port {
		return false
	}

	if !maps.Equal(i.MetaData, t.MetaData) {
		return false
	}

	return true
}

func marshal(si *ServiceInstance) (string, error) {
	data, err := json.Marshal(si)
	if err != nil {
		return "", err
	}
	return string(data), nil
}

func unmarshal(data []byte) (si *ServiceInstance, err error) {
	err = json.Unmarshal(data, &si)
	return
}

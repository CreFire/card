package etcd

import (
	"backend/deps/kit"
	"backend/deps/xlog"
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/samber/lo"
	"github.com/sasha-s/go-deadlock"
	clientv3 "go.etcd.io/etcd/client/v3"
	"go.etcd.io/etcd/client/v3/concurrency"
	"go.uber.org/zap/zapcore"
)

type EtcdClient struct {
	Config *clientv3.Config
	Client *clientv3.Client
}

func NewEtcdClient(dsn string, logger *xlog.MyLogger) (*EtcdClient, error) {
	cfg, err := ParseDSN(dsn)
	if err != nil {
		return nil, fmt.Errorf("failed to parse etcd DSN: %w", err)
	}

	if cfg.DialTimeout == 0 {
		cfg.DialTimeout = 10 * time.Second // 设置默认的拨号超时
	}

	cfg.Logger = xlog.ZapLogger(logger, zapcore.WarnLevel, 0)

	client, err := clientv3.New(*cfg)
	if err != nil {
		return nil, fmt.Errorf("failed to create etcd client: %w", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	// 尝试连接到 etcd 服务器
	err = client.Sync(ctx)
	if err != nil {
		return nil, fmt.Errorf("etcd a PING fail: %w", err)
	}

	ec := &EtcdClient{
		Config: cfg,
		Client: client,
	}

	if err := ec.Ping(); err != nil {
		return nil, fmt.Errorf("etcd a PING fail: %w", err)
	}

	return ec, nil
}

// ParseDSN 将一个自定义格式的 DSN 字符串解析为一个 *clientv3.Config 对象。
// 支持的 DSN 格式: etcd(s)://<user>:<password>@<host>:<port>?addr=<host2>:<port2>&addr=<host3>:<port3>&dialTimeout=5s
func ParseDSN(dsn string) (*clientv3.Config, error) {
	if dsn == "" {
		return nil, errors.New("etcd DSN cannot be empty")
	}

	u, err := url.Parse(dsn)
	if err != nil {
		return nil, fmt.Errorf("failed to parse etcd DSN: %w", err)
	}

	cfg := &clientv3.Config{}

	// 1. 根据 Scheme 设置 TLS
	switch u.Scheme {
	case "etcds":
		// 表示需要 TLS 连接。此处仅启用，更复杂的 TLS 配置（如自定义 CA）需要扩展
		cfg.TLS = &tls.Config{
			// InsecureSkipVerify: true, // 在生产中应避免使用，除非你知道你在做什么
		}
	case "etcd":
		// 纯文本连接
		cfg.TLS = nil
	default:
		return nil, fmt.Errorf("unsupported etcd DSN scheme: %s", u.Scheme)
	}

	// 2. 解析用户信息 (Username, Password)
	if u.User != nil {
		cfg.Username = u.User.Username()
		if password, ok := u.User.Password(); ok {
			cfg.Password = password
		}
	}

	// 3. 组装 Endpoints 列表
	endpoints := []string{}
	// 添加 URL host 部分作为第一个 endpoint
	if u.Host != "" {
		endpoints = append(endpoints, u.Host)
	}
	// 添加查询参数中所有的 'addr' 作为额外的 endpoints
	if addrs, ok := u.Query()["addr"]; ok {
		endpoints = append(endpoints, addrs...)
	}

	// 清理并验证 endpoints
	cleanedEndpoints := []string{}
	for _, ep := range endpoints {
		trimmed := strings.TrimSpace(ep)
		if trimmed != "" {
			cleanedEndpoints = append(cleanedEndpoints, trimmed)
		}
	}

	if len(cleanedEndpoints) == 0 {
		return nil, errors.New("etcd DSN must contain at least one endpoint in host or 'addr' query parameter")
	}
	cfg.Endpoints = cleanedEndpoints

	// 4. 解析其他查询参数, 例如 dialTimeout
	if timeoutStr := u.Query().Get("dialTimeout"); timeoutStr != "" {
		timeout, err := time.ParseDuration(timeoutStr)
		if err != nil {
			return nil, fmt.Errorf("invalid dialTimeout format in DSN: %w", err)
		}
		cfg.DialTimeout = timeout
	}

	return cfg, nil
}

func (ec *EtcdClient) Close() {
	ec.Client.Close()
}

func (ec *EtcdClient) Ping() error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	_, err := ec.Client.Get(ctx, "_etcd_ping_test_key_")
	if err != nil && !strings.Contains(err.Error(), "requested key not found") {
		return fmt.Errorf("etcd ping failed: %w", err)
	}

	return nil
}

// ServiceRegistrar 管理单个服务的注册和心跳。
type ServiceRegistrar struct {
	client  *clientv3.Client
	service *ServiceInstance
	key     string
	ttl     int64
	leaseID clientv3.LeaseID
	lease   clientv3.Lease

	mu     deadlock.RWMutex
	ctx    context.Context
	cancel context.CancelFunc
}

func RegistrarKey(Cluster, serverType string, serverId int32) string {
	return fmt.Sprintf("/%s/%s/%d", Cluster, serverType, serverId)
}

func RegistrarPrefix(Cluster, serverType string) string {
	return fmt.Sprintf("/%s/%s", Cluster, serverType)
}

func ParseServerInfoFromKey(key string) (cluster, serverType string, serverId int) {
	parts := strings.Split(key, "/")
	if len(parts) >= 4 {
		return parts[1], parts[2], lo.Must(strconv.Atoi(parts[3]))
	}

	return "", "", 0
}

// NewServiceRegistrar 创建并启动一个单服务的注册器。
// 它会立即注册服务并启动心跳和状态更新协程。
// ttl 是租约的生命周期（秒），建议设置为15秒或以上。
func NewServiceRegistrar(client *EtcdClient, Cluster string, service *ServiceInstance) *ServiceRegistrar {
	ttl := int64(15) // 确保TTL是合理的值，至少10秒
	ctx, cancel := context.WithCancel(context.Background())
	ssr := &ServiceRegistrar{
		client:  client.Client,
		service: service,
		key:     RegistrarKey(Cluster, service.ServiceName, service.InstanceId), // key 示例: /cluster1/logic/1
		ttl:     ttl,
		lease:   clientv3.NewLease(client.Client),
		ctx:     ctx,
		cancel:  cancel,
	}

	return ssr
}

func (ssr *ServiceRegistrar) Register() error {
	// 获取分布式锁，避免并发注册同一个 key
	lockCtx, lockCancel := context.WithTimeout(ssr.ctx, 5*time.Second)
	defer lockCancel()
	lockSession, err := concurrency.NewSession(ssr.client, concurrency.WithTTL(5))
	if err != nil {
		return fmt.Errorf("failed to create lock session: %w", err)
	}

	lockKey := fmt.Sprintf("lock-%s", ssr.key)
	mutex := concurrency.NewMutex(lockSession, lockKey)
	if err := mutex.Lock(lockCtx); err != nil {
		lockSession.Close()
		return fmt.Errorf("failed to acquire lock for key '%s': %w", ssr.key, err)
	}
	defer func() {
		unlockCtx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		if err := mutex.Unlock(unlockCtx); err != nil {
			xlog.Warnf("ServiceRegistrar: failed to release lock for key '%s': %v", ssr.key, err)
		}
		cancel()
		lockSession.Close()
	}()

	getCtx, cancel := context.WithTimeout(ssr.ctx, 3*time.Second)
	defer cancel()
	resp, err := ssr.client.Get(getCtx, ssr.key)
	if err != nil {
		return fmt.Errorf("failed to check if service exists: %w", err)
	}

	if len(resp.Kvs) > 0 {
		// 服务已存在
		return fmt.Errorf("ServiceRegistrar: service '%s' instId: '%d' value: '%s', refusing to start duplicate instance",
			ssr.service.ServiceName, ssr.service.InstanceId, string(resp.Kvs[0].Value))
	}

	// 首次注册
	if err := ssr.register(); err != nil {
		ssr.Close()
		return fmt.Errorf("initial registration failed: %w", err)
	}

	// 启动后台协程
	go ssr.keepAliveLoop() // 租约续期
	go ssr.updateLoop()    // 定时上报状态

	xlog.Infof("ServiceRegistrar: service '%s' registered successfully at key '%s'", ssr.service.ServiceName, ssr.key)
	return nil
}

// register 使用一个新的租约执行首次注册。
func (ssr *ServiceRegistrar) register() error {
	ssr.mu.Lock()
	defer ssr.mu.Unlock()
	return ssr.registerInternal()
}

// keepAliveLoop 维持与 etcd 服务器的租约。
func (ssr *ServiceRegistrar) keepAliveLoop() {
	defer kit.Exception(func(err error) {
		xlog.Errorf("ServiceRegistrar: keepAliveLoop panic: %v", err)
	})

	for {
		select {
		case <-ssr.ctx.Done():
			xlog.Infof("ServiceRegistrar: keepAliveLoop stopped for key '%s'.", ssr.key)
			return
		default:
		}

		// 确保有有效的租约ID
		if ssr.leaseID == 0 {
			xlog.Errorf("ServiceRegistrar: invalid leaseID, attempting to re-register for key '%s'", ssr.key)
			if err := ssr.reRegister(); err != nil {
				xlog.Errorf("ServiceRegistrar: re-register failed for key '%s': %v", ssr.key, err)
				// 等待一段时间后重试
				select {
				case <-time.After(5 * time.Second):
					continue
				case <-ssr.ctx.Done():
					return
				}
			}
		}

		ch, err := ssr.lease.KeepAlive(ssr.ctx, ssr.leaseID)
		if err != nil {
			xlog.Errorf("ServiceRegistrar: KeepAlive setup failed for key '%s': %v. Will retry.", ssr.key, err)
			ssr.mu.Lock()
			ssr.leaseID = 0 // 重置租约ID
			ssr.mu.Unlock()
			// 等待一段时间后重试
			select {
			case <-time.After(5 * time.Second):
				continue
			case <-ssr.ctx.Done():
				return
			}
		}

		// 处理 keep-alive 响应
		keepAliveActive := true
		for keepAliveActive {
			select {
			case ka, ok := <-ch:
				if !ok {
					xlog.Warnf("ServiceRegistrar: KeepAlive channel closed for key '%s'. Will re-register.", ssr.key)
					ssr.mu.Lock()
					ssr.leaseID = 0 // 重置租约ID
					ssr.mu.Unlock()
					keepAliveActive = false // 退出内层循环，重新注册
					break
				}
				xlog.Debugf("ServiceRegistrar: key %s (lease %x) keep-alive, TTL: %d", ssr.key, ka.ID, ka.TTL)
			case <-ssr.ctx.Done():
				xlog.Infof("ServiceRegistrar: keepAliveLoop stopped for key '%s'.", ssr.key)
				return
			}
		}
	}
}

// reRegister 尝试重新注册服务，线程安全
func (ssr *ServiceRegistrar) reRegister() error {
	ssr.mu.Lock()
	defer ssr.mu.Unlock()

	// 先清理旧的租约ID
	oldLeaseID := ssr.leaseID
	ssr.leaseID = 0

	// 如果有旧的租约，尝试撤销它
	if oldLeaseID != 0 {
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		_, err := ssr.lease.Revoke(ctx, oldLeaseID)
		cancel()
		if err != nil {
			xlog.Warnf("ServiceRegistrar: failed to revoke old lease %x: %v", oldLeaseID, err)
		}
	}

	// 执行重新注册
	return ssr.registerInternal()
}

// registerInternal 内部注册方法，调用前需要持有锁
func (ssr *ServiceRegistrar) registerInternal() error {
	val, err := marshal(ssr.service)
	if err != nil {
		return fmt.Errorf("failed to marshal service instance: %w", err)
	}

	ctx, cancel := context.WithTimeout(ssr.ctx, 5*time.Second)
	defer cancel()
	// 申请一个新的租约
	grantResp, err := ssr.lease.Grant(ctx, ssr.ttl)
	if err != nil {
		return fmt.Errorf("failed to grant lease: %w", err)
	}
	ssr.leaseID = grantResp.ID

	// 将服务信息和租约绑定并写入 etcd
	_, err = ssr.client.Put(ssr.ctx, ssr.key, val, clientv3.WithLease(ssr.leaseID))
	if err != nil {
		// 如果写入失败，立即撤销租约并重置ID
		ssr.lease.Revoke(context.Background(), ssr.leaseID)
		ssr.leaseID = 0
		return fmt.Errorf("failed to put service key: %w", err)
	}

	return nil
}

// updateValueToEtcd 将当前服务信息写入 etcd。
func (ssr *ServiceRegistrar) updateValueToEtcd() error {
	if ssr.leaseID == 0 {
		xlog.Warnf("ServiceRegistrar: updateValueToEtcd skipped, no valid lease for key '%s'", ssr.key)
		return errors.New("no valid lease")
	}

	//优化，先拉取比较，有差异再更新
	getCtx, getCancel := context.WithTimeout(ssr.ctx, 3*time.Second)
	resp, err := ssr.client.Get(getCtx, ssr.key)
	getCancel()

	var oldService *ServiceInstance
	if err != nil {
		xlog.Warnf("ServiceRegistrar: failed to get current service value for key '%s', will attempt to PUT anyway: %v", ssr.key, err)
	} else if len(resp.Kvs) > 0 {
		var unmarshalErr error
		oldService, unmarshalErr = unmarshal(resp.Kvs[0].Value)
		if unmarshalErr != nil {
			xlog.Warnf("ServiceRegistrar: failed to unmarshal existing service value for key '%s', will attempt to PUT anyway: %v", ssr.key, unmarshalErr)
			oldService = nil
		}
	}

	ssr.mu.RLock()
	if oldService != nil && oldService.Equal(ssr.service) {
		ssr.mu.RUnlock()
		xlog.Debugf("ServiceRegistrar: service value for key '%s' is already up-to-date. Skipping update.", ssr.key)
		return nil
	}

	newVal, err := marshal(ssr.service)
	ssr.mu.RUnlock()

	if err != nil {
		xlog.Errorf("ServiceRegistrar: failed to marshal service for update: %v", err)
		return err
	}

	putCtx, putCancel := context.WithTimeout(ssr.ctx, 3*time.Second)
	defer putCancel()
	_, err = ssr.client.Put(putCtx, ssr.key, newVal, clientv3.WithLease(ssr.leaseID))

	if err != nil {
		xlog.Errorf("ServiceRegistrar: failed to update service value for key '%s': %v", ssr.key, err)
	} else {
		oldVal, _ := marshal(oldService)
		xlog.Debugf("ServiceRegistrar: successfully updated service value for key: '%s' old: '%s'  new : '%s'", ssr.key, oldVal, newVal)
	}
	return err
}

// updateLoop 每隔15秒更新一次 etcd 中的服务信息。
// 这确保了元数据(MetaData)的变更能够被及时同步。
func (ssr *ServiceRegistrar) updateLoop() {
	defer kit.Exception(nil)
	ticker := time.NewTicker(15 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			_ = ssr.updateValueToEtcd() // 定期刷新，忽略错误
		case <-ssr.ctx.Done():
			xlog.Infof("ServiceRegistrar: updateLoop stopped for key '%s'.", ssr.key)
			return
		}
	}
}

// UpdateInstanceData 线程安全地修改服务的元数据并立即更新到 etcd。
// 这是一个阻塞操作，会返回更新是否成功。
func (ssr *ServiceRegistrar) UpdateInstanceData(inst *ServiceInstance) error {
	// 先加锁更新内存中的数据，然后立即释放锁
	ssr.mu.Lock()
	ssr.service.OnlineCount = inst.OnlineCount
	ssr.service.Enable = inst.Enable
	ssr.service.Healthy = inst.Healthy
	ssr.service.MetaData = lo.Assign(ssr.service.MetaData, inst.MetaData)
	ssr.mu.Unlock()

	// 在没有锁的情况下执行网络IO操作，避免死锁
	return ssr.updateValueToEtcd()
}

// Close 停止注册器并从 etcd 注销服务。
func (ssr *ServiceRegistrar) Close() {
	ssr.cancel() // 停止所有后台协程

	// 撤销租约，etcd 会自动删除关联的 key
	if ssr.leaseID != 0 {
		// 使用一个独立的 context 来确保即使父 context 被取消，也能完成撤销操作
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		_, err := ssr.lease.Revoke(ctx, ssr.leaseID)
		cancel()
		if err != nil {
			xlog.Errorf("ServiceRegistrar: failed to revoke lease %x for key '%s': %v", ssr.leaseID, ssr.key, err)
		}
	}

	ssr.lease.Close()
	xlog.Infof("ServiceRegistrar: service '%s' deregistered from key '%s'", ssr.service.ServiceName, ssr.key)
}

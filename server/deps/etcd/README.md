# etcd
## Purpose
Expose lightweight wrappers around `clientv3` for registries, discovery, and watchers.

## Use When
You need to register/deregister instances, watch service keys, or drive discovery-based load balancing.

## Avoid When
Your environment already has a different service discovery or registry layer.

## Key Entry Points
- `NewEtcdClient(dsn string, logger *xlog.MyLogger)`
- `NewClientWatcher`, `NewMultiWatcher`
- `NewRegistry(client *EtcdClient, regist bool)`
- `NewServiceRegistrar`, `RegistrarKey`, `ParseServerInfoFromKey`

## Notes
Watchers require a parent context and must be closed to stop the goroutine. Registrations start and stop with the caller’s service lifecycle.

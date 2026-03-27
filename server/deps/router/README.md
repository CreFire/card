# 路由器
## 目的
通过配置的RPC管理器将传入消息路由到RPC、服务或自定义处理器。
## 使用时机
当您需要将`rpcmgr.RpcMgr`插入请求处理并为C2S/S2S流暴露处理器工厂时。
## 避免使用时机
逻辑位于gate/public/logic管道之外且不需要`Router`抽象时。
## 关键入口点
- `NewRouter(*rpcmgr.RpcMgr)` 来构建路由器。
- `C2SHandlerFunc`、`S2SHandlerFunc` 以及注册的`Router`映射用于消息分发。
## 注意事项
路由器布线发生在更高层模块（gate/logic）中；保持处理器实现与netmgr队列对齐。

## 业务使用
- Gate/public/logic模块管理器为每个服务角色构建一个路由器，并按协议消息注册处理器。业务代码假设路由是明确的和集中的，而不是基于约定的。
- 在逻辑处理器中，`router.C2SHandlerFunc`位于解析之后和模块业务逻辑之前；不要将其误读为验证层。大多数语义检查发生在模块处理器本身内部。
- 路由器所有权遵循服务边界。Gate路由器不是通用的跨服务总线；代理不应将gate/public/logic处理器混合到一个注册表中。

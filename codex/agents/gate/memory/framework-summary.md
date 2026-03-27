# gate 代理框架总结

## 代理定位

gate 是当前仓库里的接入与编排层。
它负责把外部连接、鉴权结果和内部服务状态连接起来，但不负责账户体系本身，也不负责游戏规则本身。

## 当前实现框架

gate 的主入口在 `gateSvr.go`，通过公共 server 生命周期装配 `GateModule`。
`GateModule` 内部组合了四类能力：
- `GateService`：核心连接与绑定逻辑
- `Handler`：HTTP 管理接口
- `TCPServer`：面向长连接的接入层
- `GRPCServer`：面向内部控制面的 gRPC 接口

`GateService` 依赖 `AuthClient` 和 `GamerManager`。
`AuthClient` 当前是 HTTP 客户端实现，负责调用 auth 服务验证 ticket 或 session。
`GamerManager` 负责维护连接、会话和在线状态的本地内存索引。

## 主要职责

- 接收连接并创建 `conn_id`
- 绑定 ticket 或 session 到连接
- 维护连接和会话映射
- 对外暴露网关统计和连接查询接口
- 提供控制面 gRPC，便于查询、列表和踢人
- 在 socket 关闭时清理本地连接状态

## 边界

gate 只处理接入、鉴权绑定、连接状态和转发入口。
它不应承担：
- auth 的账号、session、ticket 生成
- logic 的房间、落子、胜负规则
- client 的 UI 和交互逻辑
- proto 的契约定义本身

## 主要输入

- TCP 新连接
- HTTP 调试和管理请求
- gRPC 控制面请求
- 来自 auth 的 session 和 ticket 验证结果
- 运行时配置中的监听地址和服务发现信息

## 主要输出

- `Gamer` 连接快照
- `GateStats` 统计信息
- auth 绑定成功或失败结果
- TCP 协议响应
- gRPC 返回的连接查询和踢出结果

## 常见协作点

- 与 `auth` 协作：验证 ticket、校验 session、获取登录态
- 与 `proto` 协作：对外暴露的控制面和快照结构依赖 protobuf 类型
- 与 `client` 协作：客户端通过网关进入长连接并完成绑定
- 与 `logic` 协作：网关后续通常把已认证玩家转交给业务层，但当前仓库里尚未形成完整对局链路

## 运行特征

- `TCPServer` 使用 `bufio.Scanner` 处理 newline-delimited JSON
- `TCPServer` 目前支持 `ping`、`auth.ticket`、`auth.session`
- `GRPCServer` 使用手写 `grpc.ServiceDesc` 注册控制面接口
- `HTTPAuthClient` 默认优先通过服务发现找 auth，失败后回退到本地默认地址
- `GamerManager` 是进程内状态，不是持久化存储

## 代理工作约束

- 改动 gate 时优先保持连接生命周期清晰
- 不把认证规则写进网关
- 不把对局规则写进网关
- 如需跨层改动，先确认协议或接口边界，再推进实现

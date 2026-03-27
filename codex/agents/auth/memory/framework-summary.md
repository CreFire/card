# Auth Framework Summary

## 角色定位

`auth` 是认证与会话子代理，负责账号登录、账号绑定、session 生命周期、login ticket、退出登录和会话校验。

## 当前框架

服务入口是 `server/src/service/auth/authSvr.go`，通过 `server/src/common/server` 的生命周期框架启动。`OnInit` 构造 `AuthModule`，`AuthModule.Init` 负责初始化索引，`RegisterHTTP` 绑定 HTTP 路由。

`AuthModule` 组装了三个核心依赖：
- `AccountRepository`，使用 MongoDB 存储账号与绑定关系
- `SessionRepository`，使用 Redis 存储 session
- `TicketRepository`，使用 Redis 存储 login ticket

业务核心在 `AuthService`，提供登录、消费 ticket、校验 session、退出登录四类能力。

## 职责边界

允许负责：
- 账号绑定与账号状态更新
- session 创建、查询、删除
- login ticket 创建与一次性消费
- 认证相关 HTTP 接口
- 初始化索引和基础存储校验

不负责：
- 连接生命周期
- 网关接入和 socket 管理
- 棋局规则和房间逻辑
- 客户端 UI
- 协议字段最终定义

## 主要输入

- `configdoc.ConfigBase`
- `MongoDB` 客户端
- `Redis` 客户端
- HTTP 请求体中的登录、ticket、session 参数

## 主要输出

- `LoginResult`
- `Session`
- `ok` 响应
- 认证错误码，例如参数错误、ticket 不存在、session 不存在

## 关键数据流

1. `Login` 归一化输入，校验 `channel`、`subject`、`device_id`
2. `AccountRepository` 查找或创建账号绑定
3. `SessionRepository` 创建 session 并覆盖旧 session
4. `TicketRepository` 创建一次性 login ticket
5. `ConsumeLoginTicket` 消费 ticket 后返回 session
6. `ValidateSession` 直接查询 session
7. `Logout` 删除 session

## 常见协作点

- 与 `gate` 协作：gate 需要消费 ticket 或校验 session 后，把连接绑定到 session
- 与 `client` 协作：client 需要知道登录响应、ticket 和 session 的使用方式
- 与 `planning` 协作：如果登录参数、渠道或配置语义变化，先由策划子代理确认字段含义
- 与 `protocol` 协作：如果对外消息字段变化，先冻结协议，再改实现

## 风险点

- 登录流程依赖 MongoDB 和 Redis 同时可用
- `ticket` 设计为一次性消费，调用方必须处理重复消费或过期失败
- `session` 以 Redis 为主，需注意 TTL 与删除一致性
- 绑定逻辑依赖 `channel + subject` 唯一约束，字段语义变化会影响历史数据

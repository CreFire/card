# Wuziqi Server Framework

这是当前 `server/` 目录下已经落地的服务端框架说明。本文只描述现有代码中的真实结构、启动链路和基础设施接入方式，不把 `robot.md` 里的长期规划混写成已实现能力。

## 当前架构概览

当前框架是一个基于 Go 的轻量微服务骨架，核心目标是把以下公共能力先统一起来：

- 服务配置加载
- 日志初始化
- Redis / MongoDB 客户端创建
- etcd 服务注册与发现
- HTTP 健康检查与实例发现接口
- 多个服务的统一启动入口

现在每个服务都以独立进程运行，入口位于 `src/service/<service>/main.go`，由公共启动器 `src/common/bootstrap/bootstrap.go` 完成初始化。当前真正跑起来的是 HTTP 进程模型，而不是完整的业务 RPC 框架。

## 启动链路

以任意一个服务为例，启动过程如下：

1. `main.go` 调用 `bootstrap.ExitOnError(serviceName, defaultConfigPath)`
2. `bootstrap.Run(...)` 解析命令行参数 `-config` 和 `-env`
3. `src/common/config` 读取 YAML 配置，并自动合并环境覆盖配置
4. 初始化日志系统
5. 按配置决定是否连接 Redis
6. 按配置决定是否连接 MongoDB
7. 如果配置了 etcd，则注册当前服务实例并维持租约
8. 启动 HTTP Server，并暴露框架级接口

当前框架级 HTTP 接口：

- `/healthz`：返回服务状态
- `/discover/<service>`：通过 etcd 查询目标服务实例列表

## Robot 自动化登录测试

`robot` 服务已接入认证与网关登录联调能力，提供两个测试接口：

- `POST /robot/auth/login`：只测试 auth 登录签发
- `POST /robot/cases/login`：串联 auth 登录 + gate 开连 + gate token 登录

启动示例：

```powershell
cd F:\work\wuziqi\server
go run ./src/service/robot -config F:\work\wuziqi\server\conf\robot.yaml -env dev
```

联调示例：

```powershell
curl -X POST "http://127.0.0.1:9001/robot/cases/login" `
  -H "Content-Type: application/json" `
  -d "{\"channel\":\"guest\",\"device_id\":\"robot-dev-001\",\"subject\":\"\",\"remote_addr\":\"127.0.0.1:50001\"}"
```

## 服务划分

当前代码目录中已经预留了以下服务入口：

- `auth`
- `battle`
- `gate`
- `logic`
- `public`
- `query`
- `robot`

现状上，这些服务入口都已经接入统一 bootstrap；但业务实现还处于骨架阶段，只有 `auth/authSvr.go` 提供了一个 `common/server.Service` 生命周期示例，尚未接入当前主启动流程。

## 核心公共层

### 1. bootstrap

`src/common/bootstrap` 是当前框架的主入口，负责把配置、日志、Redis、MongoDB、etcd、HTTP 服务编排起来。

它也是当前最接近“应用装配层”的目录。

### 2. config

`src/common/config` 负责统一读取服务配置，具备以下行为：

- 支持基础配置文件，例如 `conf/auth.yaml`
- 支持环境覆盖配置，例如 `conf/auth.dev.yaml`、`conf/auth.prod.yaml`
- 环境优先级：命令行 `-env` > 环境变量 `APP_ENV` > 默认 `dev`
- 自动填充默认值
- 校验关键配置项
- 将配置路径和日志路径统一解析为绝对路径

### 3. logger

`src/common/logger` 基于 `deps/xlog` 完成日志初始化。日志配置由 `log` 配置段控制，包括：

- 日志级别
- 输出文件路径
- 是否同时输出到标准输出
- 切割策略
- 单文件大小
- 保留天数

### 4. discovery

`src/common/discovery/etcd.go` 封装了 etcd 注册与发现，当前支持：

- 以租约方式注册服务实例
- 自动 keepalive
- 按服务名查询实例列表

注册信息包含：

- 实例 ID
- 服务名
- 服务地址
- 启动时间
- 元数据，例如配置文件绝对路径

### 5. server

`src/common/server` 定义了一个较完整的服务生命周期接口：

- `OnInit`
- `BeforeStart`
- `Start`
- `AfterStart`
- `BeforeStop`
- `Stop`
- `AfterStop`
- `OnReload`

并提供了 `Run(...)` 负责顺序启动、逆序停止服务实例。

不过要注意：这一层当前还没有成为各服务的主流程入口，属于“已实现但尚未全面接入”的公共抽象。

## 存储与基础设施接入

### Redis

正式 Redis helper 位于 `src/persist/redis/client.go`，提供：

- 基于配置构建 `go-redis/v9` Universal Client
- 启动时 `Ping` 检查
- `Raw()` 获取底层客户端
- `Close()` 关闭连接

`src/cache/redis/client.go` 目前只是兼容旧路径的转发层。

### MongoDB

`src/persist/mongo/client.go` 提供 MongoDB 客户端初始化，包含：

- 连接超时和服务选择超时配置
- 连接后 `Ping` 主节点
- 默认数据库对象获取
- 显式关闭连接

## 目录结构

当前 `server/` 的主要目录职责如下：

- `conf/`：各服务配置文件
- `configdoc/`：配置文档占位目录
- `docconf/`：配置表数据目录
- `deps/`：底层依赖封装，目前主要有日志等基础模块
- `Doc/`：项目文档目录
- `logs/`：运行时日志目录
- `src/`：核心源码目录
- `tools/`：工具脚本

`src/` 下当前的实际分层：

- `src/common/`：公共能力
- `src/persist/`：持久化与外部存储客户端
- `src/cache/`：缓存相关兼容层
- `src/proto/`：protobuf 与配置协议生成产物
- `src/service/`：各服务入口

## 配置约定

当前配置结构统一包含以下段：

- `app`
- `server`
- `etcd`
- `redis`
- `mongodb`
- `log`

典型运行方式：

```powershell
cd F:\work\wuziqi\server
go run ./src/service/auth -config F:\work\wuziqi\server\conf\auth.yaml -env dev
```

如果不传 `-config`，各服务会使用各自 `main.go` 中定义的默认配置文件名；启动时框架会把它解析成绝对路径。

## 协议与配置表

### proto

`src/proto/` 当前用于保存 Luban 导出的 `schema.proto` 和生成后的 `schema.pb.go`。

生成脚本：

```powershell
cd F:\work\wuziqi
.\server\tools\gen_conf.bat
```

### docconf

`docconf/` 用于放置游戏配置表数据。当前框架已经预留了配置表生成与读取的位置，但完整业务消费链路还没有在这个骨架里展开。

## 当前阶段的边界

已经具备的能力：

- 多服务统一启动入口
- 统一配置体系
- 日志体系
- Redis / MongoDB 接入
- etcd 注册发现
- HTTP 健康检查与发现接口

尚未在当前框架中落地的能力：

- 服务间 gRPC 通信
- 消息总线 / MQ
- 业务模块拆分
- 玩家 actor 模型
- 完整网关长连接
- 完整认证、战斗、查询业务逻辑

## 建议的阅读顺序

如果要快速理解当前框架，建议按下面顺序阅读：

1. `src/service/<service>/main.go`
2. `src/common/bootstrap/bootstrap.go`
3. `src/common/config/config.go`
4. `src/common/logger/logger.go`
5. `src/common/discovery/etcd.go`
6. `src/persist/redis/client.go`
7. `src/persist/mongo/client.go`
8. `src/common/server/server.go`

这样能先看到“服务怎么起来”，再看“公共组件如何被装配”。

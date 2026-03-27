role: 
1. 你是一个测试和后端游戏服务器开发
2. 回复使用中文
3. 不去删除文件
4. 代码跳转超链接直接使用绝对值路径
5. 长期值得记忆的点记忆在D:\remoteWork\LcuGame\Product\LcuGameServer\game3.0\robot.md下




项目记忆:
auth 负责鉴权和发 session；
gate 负责长连接、路由和选定 logic；
logic 负责 actor 化登录和后续业务消息处理。


# LcuGame 3.0 项目摘要

## 项目概述
LcuGame 3.0 是一个类似 AFK2 的游戏服务器项目，采用 Go 语言开发。这是一个单人PVE的场景战斗非实时同步的2.5D战斗策略类型游戏，预计支持100万在线用户，要求达到200万PCU和300TPS/Login。

## 技术栈
- 开发语言: Go 1.25.0
- Web框架: Gin
- 数据库: MongoDB (作为单一权威数据源)
- 缓存: Redis
- 消息队列: NATS + JetStream
- 服务发现: etcd
- 协议: gRPC + Protobuf

## 项目架构
游戏服务器采用微服务架构，主要包含以下组件：

### 核心服务
1. **Gate (网关服)**: 与客户端维持TCP长连接，接收消息并转发到Logic服务
2. **Logic (逻辑服)**: 处理实际业务逻辑，从Mongo读写玩家数据，通过MQ发送异步消息
3. **Auth (认证服)**: 提供登录、注册、充值、版本更新、停服公告等功能
4. **Battle (战斗服)**: 处理游戏战斗相关逻辑
5. **Public (公共服务)**: 提供公共功能模块
6. **Query (查询服)**: 处理查询相关请求
7. **Robot (机器人服)**: 可能用于自动化测试或AI行为模拟

### 架构特点
- 使用NATS + JetStream作为消息中间件，用于事件解耦/同步更新广播
- 采用分布式锁机制防止并发问题
- 定期状态落盘(快照)确保数据一致性
- 支持故障转移和负载均衡

## 项目结构
- `src/`: 源代码目录
  - `backend/`: 后端服务相关代码
  - `cache/`: 缓存相关实现
  - `common/`: 通用定义和协议
  - `configdoc/`: 配置文档相关
  - `msghandler/`: 消息处理器
  - `persist/`: 持久化相关
  - `proto/`: Protobuf定义
  - `service/`: 各个服务的具体实现
- `conf/`: 配置文件目录，包含各服务的YAML配置
- `docconf/`: 游戏配置数据，以JSON格式存储各种游戏配置表
- `bin/`: 编译后的可执行文件
- `logs/`: 日志文件目录
- `Doc/`: 文档目录
- `tools/`: 工具脚本

## 配置文件
项目包含多个服务的配置文件，如auth.yaml, battle.yaml, gate.yaml, logic.yaml, public.yaml, query.yaml, robot.yaml等，并为不同实例提供了编号配置(如auth_1.yaml, auth_1001.yaml等)。

## 游戏配置数据
docconf目录包含了丰富的游戏配置数据，涵盖：
- 装备系统 (equips_)
- 关卡和奖励 (gamelevel_)
- 英雄系统 (hero_)
- 符文系统 (inscription_)
- 物品系统 (item_)
- 商店系统 (shop_)
- 任务系统 (task_)
- 排行榜 (rank_)
- 错误码 (tberrorcodes)
- 全局配置 (tbglobal)

## 构建和运行
- 使用build.bat脚本编译并启动所有服务
- 需要预先安装etcd, Redis, MongoDB, Go 1.25, .NET 10
- 需要复制配置文件并修改为适当的实例配置(如xx_1001.yaml)

## 设计原则
- 数据库写入必须同步落地，确保关键数据(如奖励、背包变化、战斗结果)的一致性
- 使用Redis分布式锁保证同一时间只有一个逻辑服处理特定玩家
- 实现状态锁定机制防止并发问题
- 定期快照和TTL锁机制确保系统可靠性

proto源文件目录在

## 项目结构全览（更新）

> 根目录：`D:\remoteWork\LcuGame\Product\LcuGameServer\game3.0`

### 1) 顶层目录
- `src/`：核心源码目录
- `conf/`：各服务运行配置（含多实例配置）
- `docconf/`：游戏配置表 JSON（由工具导出/生成）
- `bin/`：各服务可执行文件与运行时资源
- `logs/`：运行日志
- `Doc/`：项目文档与架构说明
- `tools/`：构建、导表、生成 proto 等脚本工具

### 2) src 代码分层
- `src/backend/`：后台/管理相关 HTTP 路由与服务入口
- `src/cache/`：Redis 等缓存封装、键规则、排行/登录队列等
- `src/common/`：通用常量、协议、原因码、公共能力
- `src/configdoc/`：配置表读取、扩展映射与二次索引逻辑
- `src/msghandler/`：消息处理聚合/分发相关
- `src/persist/`：持久化相关逻辑
- `src/proto/`：协议生成代码
  - `docpb/`：配置表对应 pb
  - `errorpb/`：错误码 pb
  - `eventpb/`：事件 pb
  - `msgbase/`：消息基础定义
  - `pb/`：业务消息 pb
  - `pbrpc/`：RPC 协议 pb
- `src/service/`：各微服务实现

### 3) 服务拆分（src/service）
- `auth/`：登录鉴权、会话签发、认证相关接口
- `gate/`：客户端长连接接入、消息路由、转发
- `logic/`：核心业务服（玩家 actor、各玩法模块）
- `public/`：公共能力服务
- `query/`：查询服务
- `robot/`：机器人/自动化测试相关

### 4) logic 服务内部结构（src/service/logic）
- `actor/`：玩家 Actor 模型与生命周期
- `ctxresolver/`：上下文解析
- `eventbus/`：事件总线
- `gamedata/`：游戏数据聚合访问
- `iface/`：接口定义层
- `season/`：赛季逻辑
- `module/`：玩法模块集合（按业务域拆分）
  - 已有模块：`activity`、`afk`、`battle`、`equip`、`function`、`gm`、`guide`、`hero`、`insc`、`item`、`level`、`link`、`lottery`、`mail`、`pass`、`player`、`quest`、`realm`、`shop`、`skin`、`system`、`task`、`worldboss`
  - `module_mgr.go`：模块注册与管理入口

### 5) 运行与配置约定
- `conf/*.yaml`：单服务默认配置
- `conf/*_1.yaml`、`conf/*_1001.yaml` 等：多实例配置模板
- `bin/<service>/`：各服务运行目录（含日志子目录）

### 6) 当前协作记忆（简版）
- `auth` 负责鉴权与 session 签发
- `gate` 负责长连接接入、路由和选择 logic
- `logic` 负责玩家 actor 化处理与业务消息执行
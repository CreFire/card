---
name: auth
description: 处理认证、登录、session、ticket、账号绑定和 auth 服务实现。适用于修改或理解 server/src/service/auth、登录接口、会话校验、ticket 消费、Redis/Mongo 认证存储，或需要为 auth 子代理分配任务、冻结边界、生成认证相关实现与协作约束时。
---

# Auth Skill

使用认证子代理模式处理 auth 相关工作。

## 什么时候使用

当任务涉及以下内容时，启用本 skill：
- 登录流程
- session 创建、校验、删除
- login ticket 生成和消费
- 账号绑定
- auth HTTP 接口
- Redis / Mongo 认证存储

## 如何工作

先阅读 auth 入口、模块、service、repository 和错误定义，再判断改动属于认证层还是需要上游协作。

优先按以下顺序处理：
1. 明确输入和输出
2. 确认 `MongoDB` 与 `Redis` 依赖
3. 冻结认证语义和错误码
4. 再修改 service、repository、handler 或启动装配
5. 最后补充框架总结和协作说明

## 可写范围

只在以下范围内写入：
- `server/src/service/auth/`
- `codex/agents/auth/`
- `codex/skills/auth/`

如果需要跨目录改动，先确认是否真的属于 auth 子代理职责。

## 禁止范围

不要修改：
- `server/src/service/gate/`
- `server/src/service/logic/`
- `client/`
- `proto/`

不要擅自改变：
- login ticket 的一次性消费语义
- session 的 Redis 存储边界
- MongoDB 账号绑定约束
- 对外错误码语义

## 协作方式

和 `gate` 协作时，先固定认证返回值和 session 使用方式，再让网关接入。

和 `client` 协作时，只提供稳定的登录、ticket 和 session 语义，不直接介入 UI。

和 `planning` 协作时，先确认渠道、账号主体、设备号等字段语义，再实现登录逻辑。

和 `protocol` 协作时，协议字段变化必须先冻结，再同步实现。

## 风格约束

- 保持实现短而明确
- 优先复用已有 repository 和 handler 结构
- 不引入与认证无关的抽象
- 输出时说明改了哪些文件、哪些接口、哪些风险

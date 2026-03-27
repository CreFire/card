---
name: planning-config-orchestrator
description: 用于策划配置、Excel 表结构、导表生成、客户端配置消费和配置兼容性管理。适用于需要理解 planning 子代理职责、修改 `excel/`、调整生成产物、同步客户端配置读取，或用户要求让策划代理先总结框架再生成自身 skill 的场景。
---

# Planning Config Orchestrator

把策划配置链路作为独立职责来处理。

## 何时使用

- 需要修改 `excel/` 中的配置表或枚举表时。
- 需要同步服务端导表、客户端生成配置、读取路径时。
- 需要确认配置字段语义、命名规则或默认值时。
- 需要让策划代理先总结自己的框架，再输出可复用 skill 时。

## 如何工作

1. 先读取 `codex/agents/planning/agent.md` 和 `codex/ARCHITECTURE.md`。
2. 再检查 `excel/`、客户端配置读取层和导表脚本。
3. 先冻结表结构、字段语义和生成路径，再改实现。
4. 把配置源、生成物和客户端读取点当作一个整体来维护。
5. 对跨代理影响，优先写入对方的 `inbox.md`，不要靠口头假设。

## 写入范围

- `excel/`
- `client/wuziqi/scripts/data/`
- `client/wuziqi/generated/config/`
- `codex/agents/planning/`

## 禁止范围

- `server/src/service/auth/`
- `server/src/service/gate/`
- `server/src/service/logic/`
- `server/src/service/battle/`
- `proto/`

## 协作方式

- 和 `lead` 协作时，先确认配置变更是否需要冻结顺序。
- 和 `client` 协作时，统一配置读取入口和生成物格式。
- 和 `protocol` 协作时，只在共享语义受影响时联动，不擅自扩展协议。
- 和其他代理通信时，直接把消息追加到目标代理的 `inbox.md`。

## 风格约束

- 先定义结构，再填数据。
- 先确认语义，再改生成。
- 用简洁、工程化语言说明变更影响。
- 输出时必须说明影响到哪些表、哪些配置键、哪些客户端读取点。

## 常见产出

- 配置表结构说明。
- 枚举和字段语义说明。
- 导表链路调整说明。
- 客户端配置消费约定。
- 兼容性和迁移风险说明。

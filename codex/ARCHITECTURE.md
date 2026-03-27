# AI 目录与协作架构

## 1. 基础定位

当前仓库采用“根目录作为 AI 基础工作区，`codex/` 作为 AI 内部状态目录”的模式。

含义如下：
- 仓库根目录是总代理和全部子代理共享的物理工作区。
- `codex/` 是 AI 的写入和记忆根。
- 其他业务目录是业务工作目录，不存放代理私有记忆。
- 默认命令行工具固定为 `D:/Work/Ide/Git/bin/bash.exe`。

## 2. 目录职责

### 2.1 治理目录

- `codex/Skill.md`

职责：
- 规定总代理与子代理模式
- 规定会话隔离和写入边界
- 作为 AI 行为约束的上层规则源

### 2.2 AI 基础目录

- `codex/`

职责：
- 保存总代理目录
- 保存子代理目录
- 保存代理接收文件
- 保存代理记忆目录
- 保存 AI 协作文档

### 2.3 业务工作目录

- `client/`
- `server/`
- `proto/`
- `tool/`
- `excel/`

职责：
- 存放实际业务实现
- 由被授权的代理修改
- 不存放代理私有记忆

## 3. 总代理与子代理拓扑

### 3.1 总代理

目录：
- `codex/lead/`

职责：
- 接收用户目标
- 识别依赖和边界
- 冻结协议、契约和顺序
- 创建或约束子代理
- 回收子代理结果
- 组织最终集成

### 3.2 子代理

目录：
- `codex/agents/planning/`
- `codex/agents/protocol/`
- `codex/agents/auth/`
- `codex/agents/gate/`
- `codex/agents/logic/`
- `codex/agents/client/`

职责：
- 在明确授权范围内工作
- 在自己的记忆目录保存上下文
- 向其他代理发送消息时，只写入对方 `inbox.md`

## 4. 会话隔离

每个代理都被视为独立上下文。

因此必须遵守：
- 不假设其他代理知道本代理的中间结论
- 不把共享文件系统当成隐式记忆同步
- 重要结论必须显式写入对方的 `inbox.md`
- 本代理自己的工作状态只写入自己的 `memory/`

## 5. 消息流规则

### 5.1 接收文件

每个代理必须有：
- `inbox.md`

作用：
- 接收其他代理推送过来的任务、依赖、决策、阻塞说明

### 5.2 发送方式

发送方不写自己的 `outbox.md` 作为主渠道。
发送方直接把消息追加到接收方的 `inbox.md`。

这样做的好处：
- 读取位置单一
- 接收方恢复工作时只看自己的目录
- 减少双写和状态不一致

### 5.3 建议消息格式

```md
## 2026-03-26 14:00:00 +08:00 | from: gate | type: dependency

- 目标代理：protocol
- 主题：需要确认落子消息字段
- 内容：
  - 需要 `room_id`
  - 需要 `x`
  - 需要 `y`
  - 需要请求流水号
- 阻塞级别：high
```

## 6. 记忆目录规则

每个代理必须有：
- `memory/`

`memory/` 只保存该代理自己的工作上下文，不直接作为别的代理的同步通道。

建议保存：
- `current-task.md`
- `decisions.md`
- `todo.md`
- `risks.md`

当前先放一个 `README.md` 作为占位和约定说明，后续由代理自行扩展。

## 7. 默认写入边界

### 总代理

允许：
- `codex/`
- 必要时在接口冻结阶段写协议说明文档

默认不直接承担大规模业务实现。

### 协议子代理

允许：
- `proto/`
- 必要时写 `codex/agents/protocol/`

### 策划子代理

允许：
- `excel/`
- `client/wuziqi/scripts/data/`
- `client/wuziqi/generated/config/`
- 必要时写 `codex/agents/planning/`

职责：
- 维护配置表结构和字段语义
- 维护策划数据源与客户端配置消费约定
- 协调配置生成链路与客户端读取方式

约束：
- 不直接实现网关、认证或对局服务逻辑
- 不擅自修改协议层语义
- 优先保证配置源、生成产物和客户端读取语义一致

### 认证子代理

允许：
- `server/src/service/auth/`
- 必要时写 `codex/agents/auth/`

### 网关子代理

允许：
- `server/src/service/gate/`
- 必要时写 `codex/agents/gate/`

### 逻辑子代理

允许：
- `server/src/service/logic/`
- `server/src/service/battle/`
- 必要时写 `codex/agents/logic/`

### 客户端子代理

允许：
- `client/`
- 必要时写 `codex/agents/client/`

职责重点：
- 网络接入
- 协议映射
- 本地状态同步
- UI 接入

默认不拥有：
- `excel/` 作为配置源
- 策划表字段定义权

## 8. 推荐运行顺序

1. 总代理读取 `codex/Skill.md`
2. 总代理在 `codex/lead/` 记录任务目标和边界
3. 总代理冻结接口或协议
4. 总代理向各子代理 `inbox.md` 下发任务
5. 子代理在各自 `memory/` 记录执行上下文
6. 子代理将跨代理依赖写入目标代理 `inbox.md`
7. 总代理统一回收并集成

## 9. 当前项目的默认映射

- `protocol` 对应 `proto/`
- `planning` 对应 `excel/`、`client/wuziqi/scripts/data/`、`client/wuziqi/generated/config/`
- `auth` 对应 `server/src/service/auth/`
- `gate` 对应 `server/src/service/gate/`
- `logic` 对应 `server/src/service/logic/` 和 `server/src/service/battle/`
- `client` 对应 `client/`

这套结构适合当前五子棋仓库的 `client + server + proto` 形态。

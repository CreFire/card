# AI 协作基础目录

本目录是当前仓库的 AI 写入、记忆和代理协作基础目录。

治理规则来源：
- `./codex/Skill.md`

基础原则：
- 仓库根目录 `F:\work\wuziqi` 是 AI 协作的基础工作区。
- `./codex/` 只存放 AI 的规则、记忆、角色定义、协作消息和过程文档。
- `./client/`、`./server/`、`./proto/`、`./tool/`、`./excel/` 是业务工作目录。
- `./codex/` 是 AI 规则目录，不承载具体业务实现。
- 默认命令行工具使用 `D:/Work/Ide/Git/bin/bash.exe`。
- 总代理负责拆分任务、冻结接口、回收结果、集成验证。
- 子代理只在自己被授权的工作目录内改动业务文件。

## 顶层结构

```text
codex/
  README.md
  ARCHITECTURE.md
  Skill.md
  lead/
    agent.md
    inbox.md
    memory/
      README.md
  agents/
    planning/
      agent.md
      inbox.md
      memory/
        README.md
    protocol/
      agent.md
      inbox.md
      memory/
        README.md
    auth/
      agent.md
      inbox.md
      memory/
        README.md
    gate/
      agent.md
      inbox.md
      memory/
        README.md
    logic/
      agent.md
      inbox.md
      memory/
        README.md
    client/
      agent.md
      inbox.md
      memory/
        README.md
```

## 固定规则

1. 每个代理必须有独立目录。
2. 每个代理必须有自己的 `memory/` 目录。
3. 每个代理必须有自己的 `inbox.md` 接收文件。
4. 代理之间传递消息时，发送方把消息追加写入接收方的 `inbox.md`。
5. 代理自己的推理摘要、局部决策、待办和临时上下文写入自己的 `memory/`。
6. 同一个业务文件不能同时分配给多个子代理。
7. 跨层改动先由总代理冻结协议或接口，再并行推进。
8. 指向文件的输出统一使用 Markdown 链接格式：`[符号名 (line 行号)](/绝对路径#L行号)`，例如：`[ResAfkCollectRewards (line 34)](/d:/remoteWork/LcuGame/Product/LcuGameClient/Assets/Scripts/Net/Modules/AFK/AFKNet.cs#L34)`。

## 当前默认工作目录

- `F:\work\wuziqi\client`
- `F:\work\wuziqi\server`
- `F:\work\wuziqi\proto`
- `F:\work\wuziqi\tool`
- `F:\work\wuziqi\excel`

## 默认代理分工

- 总代理：统一编排、协议冻结、冲突收敛、最终集成
- 策划子代理：`excel/`、客户端配置消费约定
- 协议子代理：`proto/`
- 认证子代理：`server/src/service/auth/`
- 网关子代理：`server/src/service/gate/`
- 逻辑子代理：`server/src/service/logic/`
- 客户端子代理：客户端网络、状态同步、UI 接入

## 消息写入约定

向其他代理发送消息时，追加写入目标代理的 `inbox.md`，建议使用如下结构：

```md
## 2026-03-26 14:00:00 +08:00 | from: lead | type: handoff

- 目标：补全登录协议字段
- 范围：仅 `proto/`
- 依赖：不得修改 `server/` 和 `client/`
- 输出：更新后的协议说明和风险
```

## 记忆写入约定

每个代理把自己的上下文沉淀到自己的 `memory/` 内，建议至少包括：

- 当前目标
- 已确认约束
- 已完成事项
- 待确认问题
- 输出给其他代理的消息索引

详细规则见 `./codex/ARCHITECTURE.md`。

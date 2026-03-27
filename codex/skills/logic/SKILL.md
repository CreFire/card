---
name: logic-service-orchestrator
description: 用于五子棋后端 logic 与 battle 子代理的框架理解、领域规则编排和实现任务。适用于处理房间、匹配、回合、落子、胜负、结算、状态流转，以及围绕 server/src/service/logic 和 server/src/service/battle 的实现、重构和联调任务。
---

# Logic 子 Skill

把 `logic` 视为领域规则中心，把 `battle` 视为可独立拆分的战斗服务入口。

## 何时使用

在用户要求你处理以下内容时使用：
- 房间和匹配
- 回合和落子
- 胜负和结算
- 状态机或领域规则调整
- `server/src/service/logic`
- `server/src/service/battle`

## 工作方式

先理解现有框架，再做实现。

先确认：
- 当前服务入口如何启动
- 哪些逻辑属于 `logic`
- 哪些逻辑需要放到 `battle`
- 输入从哪里来
- 输出要交给谁

再按最小改动实现。

## 写入范围

允许修改：
- `F:\work\wuziqi\server\src\service\logic`
- `F:\work\wuziqi\server\src\service\battle`
- `F:\work\wuziqi\codex\agents\logic`
- `F:\work\wuziqi\codex\skills\logic`

## 禁止范围

不要修改：
- `F:\work\wuziqi\server\src\service\auth`
- `F:\work\wuziqi\server\src\service\gate`
- `F:\work\wuziqi\client`
- `F:\work\wuziqi\proto`

## 协作方式

和其他子代理协作时遵守以下规则：
- 向 `protocol` 代理确认消息结构，不要私自扩字段
- 向 `gate` 代理确认接入请求，不要处理连接生命周期
- 向 `planning` 代理确认配置语义，不要擅自解释策划表
- 向 `client` 代理确认状态消费方式，不要直接改 UI

## 角色约束

保持领域优先，不要把接入逻辑和规则逻辑混在一起。

优先做这些事：
- 先冻结状态机
- 先冻结输入输出
- 先划分 `logic` 和 `battle`
- 再补实现

## 输出要求

完成后说明：
- 改了哪些文件
- 领域边界怎么定的
- 还有哪些联调风险

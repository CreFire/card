---
name: protocol-contract-maintainer
description: 维护协议契约、消息字段、错误码和生成产物一致性。用于处理 proto 定义、跨服务共享消息结构、服务端和客户端协议对齐、协议冻结、兼容性检查、生成代码同步或任何需要先改协议再改实现的任务。
---

# Protocol Contract Maintainer

## 使用时机

在需要修改、冻结或审查协议契约时使用本 skill。

适用于以下场景：
- 新增或修改 `proto/*.proto`
- 统一服务端和客户端共享消息结构
- 调整错误码、枚举、状态字段
- 处理协议兼容性和生成产物同步
- 协议变更会影响 `auth`、`gate`、`logic` 或 `client`

## 工作方式

先理解需求中的字段语义，再决定是否改协议。
先冻结契约，再通知依赖方。
先改协议源，再检查生成产物和消费方。

## 写入范围

允许写入：
- `F:\work\wuziqi\proto`
- `F:\work\wuziqi\codex\agents\protocol`
- `F:\work\wuziqi\codex\agents\protocol\memory`
- `F:\work\wuziqi\codex\skills\protocol`

## 禁止范围

禁止直接修改：
- `F:\work\wuziqi\server\src\service\auth`
- `F:\work\wuziqi\server\src\service\gate`
- `F:\work\wuziqi\server\src\service\logic`
- `F:\work\wuziqi\client`

不要把业务逻辑写进协议文件。
不要在未确认语义时随意添加字段。
不要绕过协议源直接改消费方来掩盖契约问题。

## 协作方式

先和 `planning` 对齐策划字段是否需要入协议。
再和 `auth`、`gate`、`logic`、`client` 对齐字段语义和兼容性。
如果协议变更会影响多个代理，先把冻结结果写入各自的接收文件，再继续实现。

## 输出要求

完成任务后，写清楚：
- 改了哪些协议文件
- 哪些生成产物需要同步
- 哪些代理会被影响
- 还有哪些兼容性风险

## 风格约束

保持简洁、直接、工程化。
用明确字段名和明确影响面说话。
优先稳定接口，而不是追求一次性大改。

---
name: client-godot-integration
description: 用于 Godot 客户端接入、网络连接、协议映射、本地状态同步、UI 驱动和配置消费。适用于需要修改 client/wuziqi 下脚本或场景、对接已冻结协议、读取 generated/config、接入 autoload/net/data/core 的场景。用户提到 client 子代理、客户端框架理解、Godot 接入、状态同步、消息分发、配置读取或 UI 绑定时触发。
---

# Client Skill

## 目标

专注处理 Godot 客户端的接入层，不越过协议契约，不碰服务端内部实现。

## 什么时候使用

在以下任务中使用此 skill：
- 需要修改 `client/wuziqi` 下的脚本或场景
- 需要把已冻结的协议接到客户端
- 需要补全网络连接、消息收发、状态同步或 UI 驱动
- 需要让客户端读取 `generated/config` 下的配置产物

## 工作方式

先读清客户端现有分层，再确认输入输出边界。
优先处理：
1. `scripts/autoload/` 的全局状态
2. `scripts/net/` 的连接和消息处理
3. `scripts/data/` 的配置读取适配
4. `scenes/` 的启动和 UI 接线

实现时保持改动最小化，沿现有结构补齐，不重写无关代码。

## 写入范围

允许修改：
- `F:\work\wuziqi\client\wuziqi\scripts`
- `F:\work\wuziqi\client\wuziqi\scenes`
- `F:\work\wuziqi\client\wuziqi\generated`
- `F:\work\wuziqi\codex\agents\client`
- `F:\work\wuziqi\codex\skills\client`

## 禁止范围

不要修改：
- `F:\work\wuziqi\server`
- `F:\work\wuziqi\proto`
- `F:\work\wuziqi\excel`
- 未冻结的协议字段定义

## 协作方式

和协议子代理协作时，先确认消息字段和错误码，再写客户端映射。
和策划子代理协作时，先确认配置读取约定和生成路径，再写数据访问层。
和网关子代理协作时，先确认连接和登录流程，再写网络状态机。
和逻辑子代理协作时，先确认状态事件和同步粒度，再写 UI 驱动。

## 风格约束

保持简洁、直接、工程化。
不要发明新的协议字段。
不要把服务端逻辑搬到客户端。
不要把配置定义和配置消费混在一起。
最终输出时说明改动文件、运行影响和残留风险。

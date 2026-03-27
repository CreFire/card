---
name: gate-ingress-workflow
description: 处理 gate 子代理相关的接入、连接生命周期、鉴权绑定、TCP/gRPC/HTTP 编排与连接状态管理。用于需要修改或理解 F:\work\wuziqi\server\src\service\gate 的场景，尤其是连接接入、ticket/session 绑定、网关控制面、连接清理、转发入口和 gate 侧协作边界梳理时触发。
---

# Gate Workflow

用 gate 子代理模式处理接入层任务。

## 何时使用

当任务涉及以下内容时使用本 skill：
- gate 框架理解
- 连接生命周期
- ticket 或 session 绑定
- TCP 长连接接入
- HTTP 管理接口
- gRPC 控制面
- 网关状态统计
- 与 auth 的绑定链路

## 工作方式

先读 gate 代理定义，再读 gate 模块实现。
先确认输入来自哪里、输出流向哪里、状态保存在哪里。
先区分接入层逻辑和业务层逻辑，再开始修改代码。

始终按以下顺序工作：
1. 理清连接入口和生命周期
2. 理清 auth 依赖和会话绑定方式
3. 理清本地连接状态管理
4. 理清 HTTP 与 gRPC 控制面
5. 在明确边界后再修改实现

## 可写范围

只在以下范围内工作：
- `F:\work\wuziqi\server\src\service\gate`
- `F:\work\wuziqi\codex\agents\gate`
- `F:\work\wuziqi\codex\skills\gate`

如需向外扩展，必须先确认接口冻结和任务授权。

## 禁止范围

不要主动修改：
- `F:\work\wuziqi\server\src\service\auth`
- `F:\work\wuziqi\server\src\service\logic`
- `F:\work\wuziqi\client`
- `F:\work\wuziqi\proto`

不要把以下职责混入 gate：
- 账号体系
- 登录业务规则
- 对局规则
- 客户端 UI 逻辑

## 与其他代理协作

与 auth 协作时，只关心验证结果和会话绑定结果，不关心账号内部实现。

与 proto 协作时，只关心网关控制面和快照结构的契约，不自行扩展字段语义。

与 client 协作时，只关心接入协议和连接状态，不介入 UI 与本地交互流程。

与 logic 协作时，只把已认证连接转交给业务层，不实现业务规则本身。

## 结果要求

完成任务后，说明：
- 改了哪些文件
- 连接或绑定链路是否变化
- 依赖了哪些外部服务
- 还有什么风险没有消除

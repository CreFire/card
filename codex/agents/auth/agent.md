# 认证子代理

角色：
- 登录与会话维护者

职责：
- 处理登录
- 处理 ticket、session、token
- 处理账户验证

默认可写：
- `F:\work\wuziqi\server\src\service\auth`
- `F:\work\wuziqi\codex\agents\auth`

默认禁止：
- `F:\work\wuziqi\server\src\service\gate`
- `F:\work\wuziqi\server\src\service\logic`
- `F:\work\wuziqi\client`
- `F:\work\wuziqi\proto`

风格约束：
- 不处理连接生命周期
- 不扩展游戏规则
- 关注身份、凭证、过期和校验链路

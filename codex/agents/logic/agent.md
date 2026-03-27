# 逻辑子代理

角色：
- 房间与对局规则维护者

职责：
- 处理匹配
- 处理房间
- 处理回合和落子
- 处理胜负和状态流转

默认可写：
- `F:\work\wuziqi\server\src\service\logic`
- `F:\work\wuziqi\server\src\service\battle`
- `F:\work\wuziqi\codex\agents\logic`

默认禁止：
- `F:\work\wuziqi\server\src\service\auth`
- `F:\work\wuziqi\server\src\service\gate`
- `F:\work\wuziqi\client`
- `F:\work\wuziqi\proto`

风格约束：
- 不处理底层连接细节
- 不处理登录鉴权
- 先保证领域规则闭环，再考虑接入细节

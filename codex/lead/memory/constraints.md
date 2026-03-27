# 全局约束

## 文件定位链接输出格式

- 新增日期：2026-03-27
- 适用范围：总代理与所有子代理的最终回复、评审说明、代码解释

统一使用如下格式输出文件定位链接：

`[ResAfkCollectRewards (line 34)](/d:/remoteWork/LcuGame/Product/LcuGameClient/Assets/Scripts/Net/Modules/AFK/AFKNet.cs#L34)`

要求：

1. 链接文字包含“符号名 + (line 行号)”。
2. 链接目标使用绝对路径。
3. 行号锚点统一使用 `#L<line>`。
4. 不使用 `file://`、`vscode://`、相对路径或纯反引号路径代替链接。

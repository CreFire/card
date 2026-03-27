# Excel

此目录提供一套最小可用的 Luban 配置表示例：

- `global.xlsx`：全局单例配置表
- `enum.xlsx`：枚举定义表
- `__tables__.xlsx`：预留的表定义文件示例

约定：

- `global.xlsx` 使用 `##var#column` / `##type` 的单列表头格式
- `enum.xlsx` 使用 Luban 的枚举定义格式
- 当前 `server/tools/gen_conf.ps1` 会将业务表临时转换为 Luban 自动导入格式后再执行导出
- `__tables__.xlsx` 当前不参与脚本导出，后续如果要切回显式表定义模式可继续扩展

后续如果要接入正式导表流程，可将 `excel/` 作为 Luban 的输入目录。

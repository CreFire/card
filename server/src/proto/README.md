# proto

此目录用于存放 Luban 导出的 `schema.proto` 以及 `protoc` 生成后的 `schema.pb.go`。

生成方式：

```powershell
cd f:\work\wuziqi
.\server\tools\gen_conf.bat
```

脚本会：

- 调用 Luban 从 `excel/` 读取配置表
- 生成 `schema.proto` 到当前目录
- 生成 `schema.pb.go` 到当前目录

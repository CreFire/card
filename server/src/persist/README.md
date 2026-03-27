# persist

此目录用于持久化相关封装。

当前已提供：

- `mongo/client.go`：MongoDB 客户端构造、默认数据库获取、`Ping` 检查与关闭封装
- `redis/client.go`：Redis 通用客户端构造、`Ping` 检查与关闭封装

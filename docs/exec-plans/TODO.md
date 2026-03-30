# TODO Index

Generated at: 2026-03-30T00:42:39+08:00
Source pattern: TODO: description
Repository root: C:/Users/guzem/Desktop/Core/毕设/MineDock

## Summary

- Total TODO items: 9

## Items

- [ ] 抽取 start/stop/delete 的公共处理流程。 (backend/internal/api/handlers.go:87)
- [ ] 增加统一的编码错误日志，提升可观测性。 (backend/internal/api/handlers.go:154)
- [ ] 让 Docker 创建与 SQLite 保存具备原子性。 (backend/internal/service/docker_service.go:47)
- [ ] 将逐条 Save 改为批量或事务化同步路径。 (backend/internal/service/docker_service.go:111)
- [ ] 增加并发写保护，避免最后写入覆盖前写入。 (backend/internal/service/docker_service.go:112)
- [ ] 在不影响列表返回的前提下上报同步失败。 (backend/internal/service/docker_service.go:139)
- [ ] 将该函数拆分为存储读取与 Docker 对账两个辅助函数。 (backend/internal/service/docker_service.go:168)
- [ ] 当写入吞吐成为瓶颈时，重新评估连接池策略。 (backend/internal/store/sqlite.go:39)
- [ ] 用 SQLite 错误码替代文本匹配判断。 (backend/internal/store/sqlite.go:162)

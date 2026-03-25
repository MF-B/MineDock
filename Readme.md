# MineDock

轻量级游戏服务器容器化管理平台。

## 运行指南
```bash
go run main.go
```

## 待办清单
- [x] 实现本地调用 Docker API 创建和销毁容器
- [x] 接入 SQLite 数据库，实现容器实例状态与配置的持久化存储
- [ ] 编写 Makefile，实现前后端服务的一键启动、统一编译与清理
- [ ] 引入 GitHub Actions，构建 CI/CD 流水线实现代码自动化检查与跨平台编译发布
- [ ] 引入 Cgroups 技术，实现游戏容器的 CPU 与内存资源配额动态限制
- [ ] 设计基于 Docker Volume 的挂载方案，实现核心游戏存档的持久化分离
- [ ] 基于 WebSocket 建立全双工通道，实现控制台日志的毫秒级无阻塞推流与指令下发
- [ ] 开发自动化定时灾备模块，支持定时对游戏存档进行增量/全量快照压缩

## 注意事项
默认 SQLite 数据库路径为 `backend/data/minedock.db`（在 `backend` 目录启动时对应 `data/minedock.db`）。
可通过环境变量 `MINEDOCK_DB_PATH` 覆盖。
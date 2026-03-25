# MineDock

轻量级游戏服务器容器化管理平台。

## 运行指南

### Task 命令
```bash
task dev               # 一键启动前后端开发服务
task fmt               # 执行全局格式检查（当前依赖 backend:fmt）
task vet               # 执行全局静态检查（当前依赖 backend:vet）
task test              # 执行全局测试（当前依赖 backend:test）
task build             # 统一编译前后端
task clean             # 清理构建产物
task frontend:install  # 安装前端依赖
```

## 待办清单

### 实例生命周期与资源调度
- [x] 开服与停服
- [x] 实例的创建与删除
- [ ] 实例重启与强制结束进程
- [ ] 基于 Cgroups 的 CPU 与内存资源配额可视化配置

### 实时交互与性能监控
- [ ] Web 控制台标准输出日志实时推流
- [ ] 网页端在线交互指令无缝下发
- [ ] 容器 CPU、内存、网络 I/O 运行态数据实时图表展示

### 数据灾备与持久化
- [ ] 游戏容器 Volume 持久化目录挂载映射
- [ ] 游戏存档/数据一键手动快照备份
- [ ] 基于 Cron 表达式的自动化定时备份任务
- [ ] 一键回档功能

### 在线文件管理与差异化配置
- [ ] 可视化 Web 文件浏览器
- [ ] 游戏基础配置文件的在线文本编辑器
- [ ] 大文件 (Mod/插件) 上传及在线解压缩

## 注意事项
默认 SQLite 数据库路径为 `backend/data/minedock.db`（在 `backend` 目录启动时对应 `data/minedock.db`）。
可通过环境变量 `MINEDOCK_DB_PATH` 覆盖。
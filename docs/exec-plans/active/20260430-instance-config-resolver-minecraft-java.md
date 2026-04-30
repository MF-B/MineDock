# 实例配置文件与 Minecraft Java 创建向导

## 背景

当前实例配置主要从 Docker Inspect 反推，缺少 MineDock 自己的期望配置事实来源。后续要支持 Minecraft Java 版本/加载器/Java 自动适配、配置重建换镜像、ZIP 服务端包导入，需要先建立统一实例配置模型。

## 方案

- SQLite 继续保存轻量索引：`container_id`、`name`、`game_id`、`status`、`config_path`。
- 每个实例目录保存 `minedock.instance.json`，作为 desired config。
- 创建与重建流程通过 resolver 生成 `InstanceConfig`，再生成 Docker 容器配置。
- 通用游戏继续走模板 resolver。
- Minecraft Java 走专用 resolver：
  - 用户选择 Minecraft 版本。
  - 用户可选 Paper/Fabric/Forge/NeoForge，不选时按 Vanilla。
  - Fabric/Forge/NeoForge 可填写加载器版本。
  - Java 版本默认推导，也允许用户手动覆盖。
- 创建/重建前执行宿主机端口预检。
- 配置文件写入使用同实例互斥锁与临时文件原子替换。

## 安全约束

- ZIP 导入后续实现时必须防 Zip Slip：清理 ZIP 内路径，并校验最终绝对路径仍位于临时导入目录内。
- 配置文件不能直接覆盖写入，必须写 `.tmp` 后 rename。
- Docker 实际状态可能漂移；面板保存并重建时以 `minedock.instance.json` 覆盖实际状态。

## 当前进展

- [x] 新增 `StoredInstanceConfig` 模型。
- [x] SQLite 新增 `config_path` 与 `updated_at`。
- [x] 新增 `InstanceConfigFileStore`，支持原子写入。
- [x] 创建/重建流程接入 desired config。
- [x] 新增端口预检。
- [x] 新增 Minecraft Java resolver，支持 Java 自动推荐和手动覆盖。
- [x] Minecraft Java 创建页改为专用向导。
- [x] 配置页显示 Minecraft Java 的 Java 版本选择。
- [ ] ZIP 服务端包导入。
- [x] 远程版本源与缓存。
  - Minecraft 版本：Mojang version manifest。
  - Fabric：Fabric Meta API。
  - Forge：Forge Maven metadata。
  - NeoForge：NeoForge Maven metadata。
- [ ] 其他游戏专用 resolver 增强。

## 验证

- `go test ./...`
- `npm run build`
- `npm run lint`

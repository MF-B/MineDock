# 容器实例生命周期

## 数据结构

### 后端

```go
type Instance struct {
    // 映射的 Docker 容器唯一标识
    ContainerID string `json:"container_id"`
    // 服务端名称
    Name        string `json:"name"`
    // 来源游戏模板 ID（持久化在 instances 表，用于配置编辑时回查模板）
    GameID      string `json:"game_id"`
    // 当前运行态
    Status      string `json:"status"`
}
```

### 数据库

- `instances` 表字段：
- `container_id`（主键）
- `name`（唯一）
- `status`
- `game_id`
- `created_at`

## 接口

[../api/contracts.md](../api/contracts.md)

## 状态流转

```mermaid
stateDiagram-v2
    [*] --> Stopped: Create
    Stopped --> Running: Start
    Running --> Stopped: Stop
    Stopped --> [*]: Delete
```

## 创建配置注入

- 端口映射来源：模板 `container.ports`，创建容器时写入 Docker `ExposedPorts` 与 `PortBindings`。
- 卷挂载来源：模板 `container.volumes`，采用 bind mount 映射到 `MINEDOCK_DATA_DIR/{instanceName}/volumes/{volumeName}`。
- 启动命令来源：若模板配置了 `container.command` 则覆盖镜像命令；否则使用镜像默认 `ENTRYPOINT/CMD`。

## 在线配置修改

- 修改范围：允许编辑模板 `params` 定义的用户参数，以及模板端口映射对应的宿主机端口；模板固定环境变量 `container.env` 不可直接修改。
- 变更方式：通过重建容器应用新配置，流程为 `Inspect -> Remove(old) -> Create(new env + port bindings)`。
- 状态约束：仅允许在 Stopped 状态下执行配置更新，运行中返回冲突错误。
- 生效方式：更新完成后容器保持 Stopped，需用户手动启动使配置生效。
- 保留策略：按实例名称复用 bind mount 宿主机目录，并应用最新模板卷配置。
- 标识变化：重建后 `container_id` 会变化，前端需跳转到新的详情路由。

## 一致性策略

- 事实来源：Docker Daemon。
- 缓存来源：SQLite（用于实例名、状态缓存和恢复）。
- 收敛机制（主路径）：监听 Docker Events，在容器状态变化后通过 WebSocket 推送最新实例快照。
- 收敛机制（降级路径）：`ListInstances` 按需对账 Docker -> SQLite；当前端 WebSocket 不可用时回退到轮询接口。

## 控制台交互

- 路由：前端通过 `/instances/:id` 进入容器详情页，页面建立 `GET /api/ws/console/:id` WebSocket。
- 输入链路：浏览器 WebSocket 输入 -> 后端 ConsoleHandler -> Docker Attach stdin。
- 输出链路：Docker Attach stdout/stderr -> 后端 ConsoleHandler -> 浏览器 WebSocket -> xterm.js。
- 运行态约束：仅 Running 容器允许 Attach；容器停止后连接会断开，页面提示用户重新启动后再连接。
- TTY 差异：TTY 容器输出直接透传；非 TTY 容器需先做 stdout/stderr 解复用再推送。

## 失败与回滚策略

- 创建流程：若数据库保存失败，立即强制删除刚创建容器。
- 启停流程：Docker 操作成功但 DB 更新失败时，接口返回错误；后续列表刷新会将状态重新收敛。
- 删除流程：Docker 删除成功后若 DB 删除失败，后续列表会因 Docker 不存在而清理状态。
- 数据卷策略：删除实例默认保留宿主机数据目录；传入 `purge_data=true` 时同步删除实例数据目录。

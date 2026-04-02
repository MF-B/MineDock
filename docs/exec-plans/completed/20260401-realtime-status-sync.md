# 容器运行状态实时同步（WebSocket + Docker Events）

## 背景

当前 MineDock 的容器状态同步依赖请求驱动的拉取模式：前端每次执行操作（create / start / stop / delete）后主动调用 `fetchInstances()` 刷新列表，后端 `ListInstances` 遍历 Docker API 并逐条同步到 SQLite。这意味着：

- **外部状态变更不可见**：若容器被 Docker CLI / Portainer 等外部工具操作（或容器自行崩溃退出），前端无法感知，直到用户手动触发刷新
- **状态延迟**：start / stop 操作返回 HTTP 200 时，Docker 可能尚未完成状态切换，前端显示的状态可能短暂不一致
- **轮询浪费**：若改为定时轮询弥补，会产生大量无意义的 HTTP 请求

本计划引入 **WebSocket + Docker Events** 方案：后端启动一个常驻 Goroutine 监听 Docker Events（`container start`、`stop`、`die` 等），当托管容器状态变化时，通过 WebSocket 将增量事件推送给所有已连接的前端客户端，实现实时状态同步。

## 需要评审的内容

> [!IMPORTANT]
> **WebSocket 库选型**
>
> Go 标准库不内置 WebSocket 支持。推荐使用 `github.com/coder/websocket`（`nhooyr.io/websocket` 的社区延续版本，API 兼容、`net/http` 原生兼容、支持 context 取消）。也可选择 `github.com/gorilla/websocket`（社区更成熟但已归档维护模式）。
>
> 本计划默认使用 `github.com/coder/websocket`。

> [!IMPORTANT]
> **推送消息格式**
>
> WebSocket 推送的消息为 JSON，采用最简的「全量快照」模式：当任一托管容器状态变化时，后端重新查询完整实例列表并推送给所有客户端。这与当前 `GET /api/instances` 返回格式一致。
>
> ```json
> {
>   "type": "instances_updated",
>   "data": [
>     { "container_id": "xxx", "name": "xxx", "status": "Running" },
>     { "container_id": "yyy", "name": "yyy", "status": "Stopped" }
>   ]
> }
> ```
>
> 选择全量快照而非增量事件的原因：
>
> - 前端无需维护复杂的增量合并逻辑，直接替换 `instances` 数组
> - 容器数量有限（游戏服务器场景通常 < 50），全量传输开销可忽略
> - 避免增量事件丢失或乱序导致的状态不一致

> [!IMPORTANT]
> **前端降级策略**
>
> WebSocket 连接失败或断线时，前端自动降级为定时轮询（如每 5 秒一次 `GET /api/instances`），恢复连接后停止轮询。这保证即使 WebSocket 不可用，功能依然正常。

## 拟定更改

### 后端 Service 层

#### [NEW] event_hub.go (`backend/internal/service/event_hub.go`)

新增 `EventHub`，职责：管理 WebSocket 客户端连接、监听 Docker Events、广播状态变更。

```go
// EventHub 管理 WebSocket 连接并将 Docker 事件广播给所有客户端。
type EventHub struct {
    cli           *client.Client
    listFn        func(ctx context.Context) ([]model.Instance, error)
    mu            sync.RWMutex
    clients       map[*websocket.Conn]struct{}
    lastSnapshot  []byte // 上一次广播的 JSON 快照，用于去重比对
}

// NewEventHub 创建 EventHub。
// listFn 是获取完整实例列表的回调（注入 DockerService.ListInstances）。
func NewEventHub(cli *client.Client, listFn func(ctx context.Context) ([]model.Instance, error)) *EventHub { ... }

// Run 启动 Docker Events 监听循环，ctx 取消时退出。
func (h *EventHub) Run(ctx context.Context) { ... }

// AddClient 注册一个 WebSocket 客户端连接。
func (h *EventHub) AddClient(conn *websocket.Conn) { ... }

// RemoveClient 移除一个 WebSocket 客户端连接。
func (h *EventHub) RemoveClient(conn *websocket.Conn) { ... }
```

核心逻辑：

- **`Run` 方法**：
  - 调用 `cli.Events(ctx, events.ListOptions{...})` 获取 Docker Events 流
  - 通过 `filters` 仅监听 `container` 类型、`start / stop / die / destroy / kill` 动作、带有 `minedock.managed=true` 标签的容器
  - 收到事件后，调用 `listFn(ctx)` 获取最新实例列表
  - **快照去重**：将列表序列化为 JSON 后与上一次广播的 `lastSnapshot []byte` 对比，内容完全一致则跳过本次推送，避免无意义的全量传输
  - 内容变化时，将列表封装为 `{ "type": "instances_updated", "data": [...] }` JSON
  - 遍历所有 `clients`，写入 WebSocket 消息；写入失败则移除该连接
  - **防抖**：短时间内收到多个事件时（如 stop 后紧接 die），合并为一次推送（100~200ms 窗口）

- **退出机制**：`ctx` 取消时，Events 流自动关闭，`Run` 退出。关闭所有客户端连接。

- **重连**：Docker Events 流断开时（网络闪断、Docker 重启），自动重连（指数退避，最大间隔 30s），并在重连后立即推送一次全量快照。

---

### 后端 API 层

#### [NEW] ws_handler.go (`backend/internal/api/ws_handler.go`)

新增 WebSocket Handler：

```go
// EventBroadcaster 定义 WebSocket Handler 依赖的事件广播操作。
type EventBroadcaster interface {
    AddClient(conn *websocket.Conn)
    RemoveClient(conn *websocket.Conn)
}

// WsHandler 暴露 WebSocket 相关 HTTP 处理器。
type WsHandler struct {
    hub EventBroadcaster
}

// NewWsHandler 创建 WsHandler。
func NewWsHandler(hub EventBroadcaster) *WsHandler { ... }

// HandleEvents 处理 GET /api/ws/events，将 HTTP 连接升级为 WebSocket。
func (h *WsHandler) HandleEvents(w http.ResponseWriter, r *http.Request) { ... }
```

`HandleEvents` 逻辑：

1. 使用 `websocket.Accept(w, r, &websocket.AcceptOptions{...})` 升级连接
2. 调用 `hub.AddClient(conn)` 注册连接
3. 阻塞读循环（等待客户端关闭或 ctx 取消）
4. `defer hub.RemoveClient(conn)` 清理

#### [MODIFY] router.go (`backend/internal/api/router.go`)

- `NewRouter` 签名变更：增加 `*WsHandler` 参数
- 注册新路由：`GET /api/ws/events`（不受 `withCORS` 中间件影响，WebSocket 有自身的 Origin 校验）

---

### 后端入口

#### [MODIFY] main.go

- 创建 `EventHub`，注入 Docker client 和 `DockerService.ListInstances`
- 启动 `EventHub.Run` Goroutine（使用 `context.WithCancel` 控制生命周期）
- 创建 `WsHandler` 并注入 `EventHub`
- 更新 `NewRouter` 调用

```go
// main.go 新增部分（伪代码）
ctx, cancel := context.WithCancel(context.Background())
defer cancel()

hub := service.NewEventHub(cli, svc.ListInstances)
go hub.Run(ctx)

wsHandler := api.NewWsHandler(hub)
router := api.NewRouter(h, registryHandler, wsHandler)
```

---

### 后端依赖

#### [MODIFY] go.mod

- 新增依赖：`github.com/coder/websocket`

---

### 前端 Composable 层

#### [NEW] useInstanceSync.ts (`frontend/src/composables/useInstanceSync.ts`)

新增 Composable，封装原生 WebSocket 连接管理和降级轮询逻辑。

> [!NOTE]
> 本计划使用原生 `new WebSocket()` API 自实现，**不引入 VueUse 的 `useWebSocket`**。原因：
>
> - 当前项目未依赖 VueUse，仅为一个 Composable 引入整个库不合算
> - 自实现可以精确控制降级轮询、重连退避、store 集成等业务逻辑
> - 原生 WebSocket API 本身已足够简洁，封装量很小

```typescript
// useInstanceSync 管理与后端的 WebSocket 实时连接，
// 收到 instances_updated 消息时自动更新 container store。
// 连接失败或断开时降级为定时轮询。
export function useInstanceSync(): {
  /** WebSocket 连接状态 */
  connected: Ref<boolean>;
  /** 启动同步（在 onMounted 中调用） */
  start: () => void;
  /** 停止同步（在 onUnmounted 中调用） */
  stop: () => void;
};
```

核心逻辑：

- **WebSocket URL 构建**：根据当前页面协议自动选择 `ws://` 或 `wss://`，路径为 `/api/ws/events`
- **连接成功**：设置 `connected = true`，停止轮询定时器
- **收到消息**：解析 JSON，若 `type === "instances_updated"`，直接将 `data` 写入 `useContainerStore().instances`
- **连接断开 / 失败**：设置 `connected = false`，启动降级轮询（每 5 秒调用 `fetchInstances()`），并启动重连定时器（指数退避 1s → 2s → 4s → ... → 最大 30s）
- **重连成功**：停止轮询，重新进入 WebSocket 模式
- **`stop`**：关闭 WebSocket 连接，清除所有定时器

#### [MODIFY] containers.ts (`frontend/src/stores/containers.ts`)

- 新增 `applySnapshot(instances: Instance[])` 方法：直接替换 `instances` 数组（供 WebSocket 消息使用，绕过 `fetchInstances` 的 HTTP 调用）
- 现有 `create` / `remove` / `toggle` action 中的 `await fetchInstances()` 保留不变（WebSocket 推送会自然覆盖，但 HTTP 回调提供了及时的首次状态更新）

---

### 前端视图层

#### [MODIFY] ContainerList.vue (`frontend/src/views/ContainerList.vue`)

- 引入 `useInstanceSync` composable
- `onMounted` 中调用 `start()`
- `onUnmounted` 中调用 `stop()`

#### [MODIFY] TopBar.vue (`frontend/src/components/TopBar.vue`)

- 在语言切换按钮左侧新增 WebSocket 连接状态指示器：
  - 绿色圆点（`connected === true`）+ tooltip「实时同步已连接」
  - 灰色圆点（`connected === false`）+ tooltip「实时同步已断开，轮询模式」
  - 圆点使用 CSS 变量 `--success` / `--text-muted`
- `useInstanceSync` 的 `connected` ref 需要在 store 或 composable 中暴露为全局可访问（因为 TopBar 和 ContainerList 是不同组件）
  - 方案：在 `useContainerStore` 中新增 `wsConnected` ref，由 `useInstanceSync` 写入，TopBar 读取

---

### 前端 API 层

#### [MODIFY] index.ts (`frontend/src/api/index.ts`)

- 新增 `WS_BASE_URL` 常量：根据 `VITE_API_BASE_URL` 和当前协议构建 WebSocket URL
- 新增 `WsMessage` 类型定义：

```typescript
export interface WsInstancesUpdated {
  type: "instances_updated";
  data: Instance[];
}

export type WsMessage = WsInstancesUpdated;
```

---

### 文档

#### [MODIFY] contracts.md (`docs/api/contracts.md`)

新增 WebSocket 接口文档：

````markdown
### GET /api/ws/events (WebSocket)

- 说明：建立 WebSocket 连接，实时接收容器状态变更推送
- 协议：WebSocket（HTTP Upgrade）
- 消息格式（服务端 → 客户端）：

\```json
{
"type": "instances_updated",
"data": [{ "container_id": "xxx", "name": "xxx", "status": "Running" }]
}
\```

- 触发时机：任一托管容器状态发生变化（start / stop / die / destroy）
- 降级方案：客户端连接失败时应回退到轮询 GET /api/instances
````

#### [MODIFY] instance_lifecycle.md (`docs/design-docs/instance_lifecycle.md`)

- 更新一致性策略：新增 Docker Events 实时推送机制说明
- 收敛机制更新为：Docker Events 实时推送（主路径）+ `ListInstances` 按需对账（降级路径）

---

## 执行步骤

- [ ] 后端依赖
  - [ ] 执行 `go get github.com/coder/websocket`，更新 `go.mod` 和 `go.sum`
- [ ] 后端 Service 层
  - [ ] 新建 `backend/internal/service/event_hub.go`，实现 `EventHub`
  - [ ] 实现 Docker Events 监听循环（过滤托管容器、防抖合并、重连退避）
  - [ ] 实现快照去重（`lastSnapshot` 对比，内容一致时跳过推送）
  - [ ] 实现客户端连接管理（AddClient / RemoveClient / 广播）
  - [ ] 编写 `event_hub_test.go` 单元测试（广播逻辑、客户端增删）
- [ ] 后端 API 层
  - [ ] 新建 `backend/internal/api/ws_handler.go`，实现 `WsHandler`
  - [ ] 修改 `backend/internal/api/router.go`：注册 `GET /api/ws/events` 路由
- [ ] 后端入口
  - [ ] 修改 `backend/main.go`：创建 `EventHub`、启动 Run Goroutine、创建 `WsHandler`、更新 Router
- [ ] 前端 API 层
  - [ ] 修改 `frontend/src/api/index.ts`：新增 `WS_BASE_URL` 和 `WsMessage` 类型
- [ ] 前端 Composable 层
  - [ ] 新建 `frontend/src/composables/useInstanceSync.ts`，实现 WebSocket 连接管理和降级轮询
- [ ] 前端 Store 层
  - [ ] 修改 `frontend/src/stores/containers.ts`：新增 `applySnapshot` 方法和 `wsConnected` ref
- [ ] 前端视图层
  - [ ] 修改 `frontend/src/views/ContainerList.vue`：接入 `useInstanceSync`
  - [ ] 修改 `frontend/src/components/TopBar.vue`：新增 WebSocket 连接状态圆点指示器
- [ ] 文档更新
  - [ ] 修改 `docs/api/contracts.md`：新增 WebSocket 接口文档
  - [ ] 修改 `docs/design-docs/instance_lifecycle.md`：更新一致性策略

## 已确认的决策

- ✅ WebSocket 库：使用 `github.com/coder/websocket`
- ✅ 连接状态 UI：在 TopBar 语言按钮左侧添加绿色/灰色圆点指示器
- ✅ 快照去重：EventHub 缓存上一次广播的 JSON，内容一致时跳过推送
- ✅ 前端 WebSocket：使用原生 `new WebSocket()` 自实现 composable，不引入 VueUse

## 验证计划

### 自动化测试

- `task backend:test` — 覆盖：
  - `EventHub`：客户端注册/移除、广播消息格式、并发安全
  - `WsHandler`：HTTP → WebSocket 升级
- `task backend:vet && task backend:lint`
- 前端：`npm run lint`

### 手动验证

- 启动前后端，打开容器列表页，验证 WebSocket 连接建立（DevTools → Network → WS）
- 通过 Docker CLI 执行 `docker stop <container>`，验证前端状态**自动**更新为 Stopped（无需手动刷新）
- 通过 Docker CLI 执行 `docker start <container>`，验证前端状态自动更新为 Running
- 断开 WebSocket（如关闭后端），验证前端降级为轮询模式
- 重启后端，验证前端自动重连 WebSocket 并恢复实时推送
- 同时打开两个浏览器标签页，验证状态变更在两个页面同步更新

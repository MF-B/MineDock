# MineDock API Contracts

## 命名规则

## 约定

- Base URL: `/api`
- CORS: 允许任意来源,允许方法 `GET,POST,PUT,DELETE,OPTIONS`

## HTTP接口

### GET /api/instances

- 说明：获取当前所有容器的列表
- 状态码：
  - 成功：`200`
  - 失败：`500`
- 请求参数：无
- 返回结果：

```json
[{ "container_id": "xxx", "name": "xxx", "status": "xxx" }]
```

### GET /api/games

- 说明：获取游戏目录轻量列表（用于市场页快速加载）
- 状态码：
  - 成功：`200`
  - 失败：`500`
- 请求参数：无
- 返回结果：

```json
[
  {
    "id": "minecraft-java",
    "name": "Minecraft Java Edition",
    "description": "...",
    "category": "minecraft",
    "icon": "minecraft-java"
  }
]
```

### GET /api/games/:id/template

- 说明：按游戏 ID 加载完整模板详情（YAML 解析结果）
- 状态码：
  - 成功：`200`
  - 失败：`400`（ID 非法）、`404`（game 不存在）、`500`（模板不存在/模板非法）
- 请求参数：
  - 路径参数：`id`（game ID）
- 返回结果：

```json
{
  "image": {
    "name": "itzg/minecraft-server",
    "tag": "latest"
  },
  "container": {
    "ports": [{ "host": 25565, "container": 25565, "protocol": "tcp" }],
    "env": { "EULA": "TRUE", "TYPE": "PAPER" },
    "volumes": [
      { "name": "server-data", "container_path": "/data", "readonly": false }
    ],
    "resources": { "memory": "2g", "cpu": 2 },
    "health_check": {
      "test": ["CMD-SHELL", "mc-health"],
      "interval": "30s",
      "timeout": "10s",
      "retries": 3,
      "start_period": "120s"
    }
  },
  "params": [
    {
      "key": "SERVER_TYPE",
      "label": "服务器类型",
      "description": "选择 Minecraft 服务器内核",
      "type": "select",
      "default": "PAPER",
      "options": [
        { "value": "PAPER", "label": "Paper" },
        { "value": "FABRIC", "label": "Fabric" }
      ],
      "env_var": "TYPE"
    }
  ]
}
```

### POST /api/instances

- 说明：创建一个新容器（初始为 Stopped），并应用模板中的端口映射与卷挂载配置
- 状态码：
  - 成功：`200`
  - 失败：`400`（JSON非法/空名称/缺失 game_id/game_id 不合法/params 非法/resources 非法）、`409`（名称冲突）、`500`（模板不存在或模板非法/容器创建失败）
- 行为说明：
  - 端口映射来源：模板 `container.ports`，可在请求体 `ports` 中覆盖 host 端口
  - 卷挂载来源：模板 `container.volumes`，卷名规则为 `minedock-{instanceName}-{volumeName}`
  - 资源限制来源：默认使用模板 `container.resources`，可在请求体 `resources` 中覆盖
  - 若宿主机端口冲突，返回 Docker 原生错误并映射为 `500`
- 请求参数：

```json
{
  "name": "容器1号",
  "game_id": "minecraft-java",
  "ports": [{ "host": 25575, "container": 25565, "protocol": "tcp" }],
  "params": {
    "SERVER_TYPE": "PAPER",
    "ONLINE_MODE": "true"
  },
  "resources": {
    "memory": "2g",
    "cpu": 2
  }
}
```

- 返回结果：

```json
{ "status": "success", "container_id": "xxx" }
```

### POST /api/instances/:id/start

- 说明：启动指定容器实例
- 状态码：
  - 成功：`200`
  - 失败：`400`（ID非法）、`500`
- 请求参数：无（ID 在路径中）
- 返回结果：

```json
{ "status": "success" }
```

### POST /api/instances/:id/stop

- 说明：停止指定容器实例
- 状态码：
  - 成功：`200`
  - 失败：`400`（ID非法）、`500`
- 请求参数：无（ID 在路径中）
- 返回结果：

```json
{ "status": "success" }
```

### DELETE /api/instances/:id

- 说明：彻底删除指定容器实例
- 状态码：
  - 成功：`200`
  - 失败：`400`（ID非法）、`409`（实例运行中）、`500`
- 请求参数：无（ID 在路径中）
- 返回结果：

```json
{ "status": "success" }
```

### GET /api/instances/:id/config

- 说明：获取容器当前生效的可编辑配置（包含模板 `params` 定义参数与可编辑端口映射）
- 状态码：
  - 成功：`200`
  - 失败：`400`（ID非法）、`500`
- 请求参数：无（ID 在路径中）
- 返回结果：

```json
{
  "game_id": "minecraft-java",
  "status": "Stopped",
  "ports": [{ "host": 25565, "container": 25565, "protocol": "tcp" }],
  "resources": { "memory": "2g", "cpu": 2 },
  "params": {
    "SERVER_TYPE": "PAPER",
    "MAX_PLAYERS": "20"
  }
}
```

### PUT /api/instances/:id/config

- 说明：更新容器配置（参数 + 端口映射 + 资源限制，通过重建容器实现，容器必须处于 Stopped）
- 状态码：
  - 成功：`200`
  - 失败：`400`（ID非法/参数非法）、`409`（容器未停止）、`500`
- 请求参数：

```json
{
  "ports": [{ "host": 25575, "container": 25565, "protocol": "tcp" }],
  "resources": { "memory": "1.5g", "cpu": 1.5 },
  "params": {
    "SERVER_TYPE": "FABRIC",
    "MAX_PLAYERS": "50"
  }
}
```

- 返回结果：

```json
{ "status": "success", "container_id": "new_container_id" }
```

### GET /api/ws/events (WebSocket)

- 说明：建立 WebSocket 连接，实时接收容器状态变更推送
- 协议：WebSocket（HTTP Upgrade）
- 同源限制：仅支持同源连接（`Origin` 必须与请求 `Host` 一致），当前版本不支持跨域 WebSocket
- 消息格式（服务端 -> 客户端）：

```json
{
  "type": "instances_updated",
  "data": [{ "container_id": "xxx", "name": "xxx", "status": "Running" }]
}
```

- 触发时机：任一托管容器状态发生变化（`start` / `stop` / `die` / `destroy` / `kill`）
- 降级方案：客户端连接失败时应回退到轮询 `GET /api/instances`

### GET /api/ws/console/:id (WebSocket)

- 说明：建立 WebSocket 连接，双向桥接容器主进程的 stdin/stdout/stderr
- 协议：WebSocket（HTTP Upgrade）
- 前置条件：容器必须处于 Running 状态
- 路径参数：`id`（容器 ID）
- 数据流向：
  - 服务端 -> 客户端：容器 stdout/stderr 输出（Binary 帧）
  - 客户端 -> 服务端：用户输入命令（Text/Binary 帧，原样写入容器 stdin）
- TTY 自适应：
  - TTY 容器：直接转发输出流
  - 非 TTY 容器：服务端使用 Docker 多路复用解复用后再转发
- 连接关闭时机：客户端断开、容器退出、服务端关闭连接
- 失败行为：容器不存在或未运行时，服务端在升级后主动关闭连接并返回原因

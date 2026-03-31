# MineDock API Contracts

## 命名规则

## 约定

- Base URL: `/api`
- CORS: 允许任意来源,允许方法 `GET,POST,DELETE,OPTIONS`

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

### GET /api/registry/images

- 说明：获取注册表中所有可用的容器镜像列表
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
    "image": "itzg/minecraft-server:latest",
    "description": "...",
    "category": "minecraft",
    "icon": "minecraft-java",
    "default_env": { "EULA": "TRUE" },
    "default_ports": ["25565:25565"]
  }
]
```

### POST /api/instances

- 说明：创建一个新容器（初始为 Stopped）
- 状态码：
  - 成功：`200`
  - 失败：`400`（JSON非法/空名称/缺失 image_id/image_id 不合法）、`409`（名称冲突）、`500`
- 请求参数：

```json
{ "name": "容器1号", "image_id": "minecraft-java" }
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

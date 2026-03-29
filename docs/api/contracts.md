# MineDock API Contracts

## 命名规则


## 约定
- Base URL: `/api`
- CORS: 允许任意来源,允许方法 `GET,POST,DELETE,OPTIONS`

## HTTP接口

| 方法 | 路径 | 说明 | 请求参数 | 返回结果 |
| --- | --- | --- | --- | --- |
| GET | `/api/instances` | 获取当前所有容器的列表 | 无 | `[{"container_id":"xxx", "name":"xxx", "status":"xxx"}]` |
| POST | `/api/instances` | 创建一个新容器（初始为 Stopped） | `{"name": "测试服1号"}` | `{"status": "success", "container_id": "xxx"}` |
| POST | `/api/instances/:id/start` | 启动指定容器实例 | 无（ID 在路径中） | `{"status": "success"}` |
| POST | `/api/instances/:id/stop` | 停止指定容器实例 | 无（ID 在路径中） | `{"status": "success"}` |
| DELETE | `/api/instances/:id` | 彻底删除指定容器实例 | 无（ID 在路径中） | `{"status": "success"}` |

## JSON格式

```go
type createRequest struct {
	Name string `json:"name"`
}

type statusResponse struct {
	Status string `json:"status"`
	Error  string `json:"error,omitempty"`
}

type createResponse struct {
	Status      string `json:"status"`
	ContainerID string `json:"container_id"`
}
```

## 状态码约定

| 接口 | 成功 | 失败 |
| --- | --- | --- |
| `GET /api/instances` | `200` | `500` |
| `POST /api/instances` | `200` | `400`(JSON非法/空名称), `409`(名称冲突), `500` |
| `POST /api/instances/:id/start` | `200` | `400`(ID非法), `500` |
| `POST /api/instances/:id/stop` | `200` | `400`(ID非法), `500` |
| `DELETE /api/instances/:id` | `200` | `400`(ID非法), `409`(实例运行中), `500` |

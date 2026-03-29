# MineDock API Contracts

前后端接口定义,后端与Docker Daemon的交互流等

## 1. 容器实例生命周期接口 (Instance Lifecycle)

| 方法 | 路径 | 说明 | 请求参数 | 返回结果 |
| --- | --- | --- | --- | --- |
| GET | `/api/instances` | 获取当前所有容器的列表 | 无 | `[{"container_id":"xxx", "name":"xxx", "status":"xxx"}]` |
| POST | `/api/instances` | 创建一个新容器（初始为 Stopped） | `{"name": "测试服1号"}` | `{"status": "success", "container_id": "xxx"}` |
| POST | `/api/instances/:id/start` | 启动指定容器实例 | 无（ID 在路径中） | `{"status": "success"}` |
| POST | `/api/instances/:id/stop` | 停止指定容器实例 | 无（ID 在路径中） | `{"status": "success"}` |
| DELETE | `/api/instances/:id` | 彻底删除指定容器实例 | 无（ID 在路径中） | `{"status": "success"}` |

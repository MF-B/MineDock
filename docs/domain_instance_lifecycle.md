# 领域切片：容器实例生命周期（Instance Lifecycle）

## 1. 领域说明
该领域负责容器实例的完整生命周期管理：
- 查询当前实例列表
- 创建并启动新实例
- 停止并销毁指定实例

该领域是当前系统的核心业务能力。

## 2. 领域数据结构

### 2.1 Instance（后端）
```go
// 容器相关
// Instances 表
type Instance struct {
    ContainerID string `json:"container_id"` // 映射的 Docker 容器唯一标识
    Name        string `json:"name"`         // 服务端名称 (比如 "测试服1号")
    Status      string `json:"status"`       // 当前运行态 (Running, Stopped)
}
```

### 2.2 字段语义与约束
- `container_id`：Docker 容器唯一标识。
- `name`：实例名称（例如“测试服1号”）。
- `status`：实例运行状态，当前约定值为 `Running` 或 `Stopped`。

## 3. 领域 HTTP 接口契约

| 方法 | 路径 | 说明 | 请求参数 | 返回结果 |
| --- | --- | --- | --- | --- |
| GET | `/api/instances` | 获取当前所有容器的列表 | 无 | `[{"container_id":"xxx", "name":"xxx", "status":"xxx"}]` |
| POST | `/api/instances` | 创建并启动一个新容器 | `{"name": "测试服1号"}` | `{"status": "success", "container_id": "xxx"}` |
| DELETE | `/api/instances/:id` | 停止并销毁指定容器 | 无（ID 在路径中） | `{"status": "success"}` |

## 4. 领域状态流转（当前实现语义）
- 创建成功后：实例进入 `Running`。
- 销毁成功后：实例从当前内存状态中移除（不做持久化保留）。
- 列表查询返回的是当前内存视图中的实例集合。

## 5. 领域内外依赖关系
- 领域内部依赖后端模块：
  - API 路由层
  - 业务服务层
  - 内存状态管理层
- 领域对外通过 HTTP 提供统一契约，前端仅调用该契约，不直接触达 Docker 引擎。

# 容器实例生命周期

## 1. 说明
该领域负责容器实例的完整生命周期管理：
- 查询当前实例列表
- 创建新实例（不自动启动）
- 启动指定实例
- 停止指定实例
- 彻底删除指定实例（要求先停止）

该领域是当前系统的核心业务能力。

## 2. 数据结构

### Instance（后端）
```go
type Instance struct {
    // 映射的 Docker 容器唯一标识
    ContainerID string `json:"container_id"`
    // 服务端名称 (比如 "测试服1号")
    Name        string `json:"name"`
    // 当前运行态 (Running, Stopped)
    Status      string `json:"status"`
}
```

## 3. HTTP 接口
具体的接口定义请参考 [02_API_Contracts.md](02_API_Contracts.md#1-容器实例生命周期接口-instance-lifecycle)。

## 4. 状态流转（当前实现语义）
- 创建成功后：实例进入 `Stopped`，并写入 SQLite 持久化存储。
- 启动成功后：实例状态更新为 `Running`。
- 停止成功后：实例状态更新为 `Stopped`。
- 删除成功后：销毁 Docker 容器，同时从 SQLite 中删除该实例记录；若实例仍为 `Running`，删除请求会被拒绝（需先停止）。
- 列表查询：以后端从 Docker 引擎读取的受管容器为准，并同步更新 SQLite 中的实例状态信息。

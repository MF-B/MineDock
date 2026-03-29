# 后端规范

## 1. 基础规范

### 技术栈
- 开发语言：Go
- 持久化存储：SQLite
- 核心依赖：Docker SDK
- HTTP 框架：标准库 `net/http` + `http.ServeMux`
- 数据库驱动：`modernc.org/sqlite`

### 格式化

- 所有 Go 文件必须通过 `gofmt`。
- 本地执行：`task backend:fmt`，仓库级执行：`task fmt`。

### 注释(Go Doc)

- 导出符号（大写开头）**必须**以符号名开头的完整句子注释。
- TODO 规范：`// TODO(username): 描述内容`。

## 2. 核心机制与控制流

### 上下文与并发
- Context 传递：
  - `ctx` 必须作为函数第一个参数。
  - Handler 层通过 `r.Context()` 获取生命周期。
  - 禁止将 `context.Context` 存储在结构体中。
- Goroutine 管理：
    - 启动 Goroutine 时必须明确其**退出机制**（通过 ctx 或 channel）。
    - 禁止在 `init()` 函数中启动后台任务。

### 错误处理

- 底层错误统一使用 `fmt.Errorf("...: %w", err)` 包装。
- 对于可预见的业务逻辑错误，在包级别定义 `ErrNotFound` 等变量，便于 `errors.Is()` 判断。
- 优先处理错误流，减少 `else` 嵌套。

## 3. 架构与设计模式

### 依赖注入

- 接口原则：Service 层定义的依赖接口应尽量“小”。
- 构造函数：组件通过 `New...` 函数初始化，并在此处注入依赖（如 `func NewService(s Store) *Service`）。
- 解耦：Handler 仅负责解析请求、调用 Service、返回 Response。禁止在 Handler 直接操作 Docker SDK 或 SQL。

## 4. 数据与存储

### 数据库

- 并发控制：SQLite 必须限制 `SetMaxOpenConns(1)` 以避免 `database is locked` 错误。
- 事务处理：涉及多表更新的操作必须在 Store 层封装事务，并确保 `defer tx.Rollback()`。
- Upsert 模式：优先使用 `ON CONFLICT` 处理幂等写入。

### API

[../api/contracts.md](../api/contracts.md)

## 5. 可观测性

### 日志规范

- 启动与致命错误使用标准库日志输出。
- 禁止记录无意义的 `err != nil`。日志应包含：**动作+对象+关键ID+原始错误**。

## 6. 质量保障

### 测试

- 后端测试入口：`task backend:test`（`go test ./...`）。
- Lint 检查：`task backend:lint`（`golangci-lint run ./...`）。
- 静态检查：`task backend:vet`（`go vet ./...`）。
- 提交前最低门禁建议：`task fmt && task lint && task vet && task test && task build`。

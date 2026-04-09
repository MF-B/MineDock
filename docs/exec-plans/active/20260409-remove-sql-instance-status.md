# 删除 SQLite 中的实例状态持久化

## 背景

当前实例状态同时存在于 Docker 运行态、Go 内存模型和 SQLite 持久化表中。随着实时状态同步改为以 Docker API + WebSocket 为准，SQLite 里的 `status` 字段已经不再适合作为状态来源，继续保留只会增加重复写入和语义歧义。

本计划的目标是**物理删除 SQLite 中的实例状态列**，让持久化层只保存实例身份与配置类信息，运行态状态统一由 Docker 查询结果提供。

## 需要评审的内容 (User Review Required)

> [!IMPORTANT]
> **范围确认**
>
> 这里的“删除状态信息”仅指 SQLite 持久化层，不删除 API 返回的 `Instance.Status` 字段，也不删除运行时的状态计算逻辑。前端仍然需要 `status` 用于展示，只是这个字段不再从 SQLite 读取或写入。

> [!IMPORTANT]
> **迁移策略**
>
> 现有 SQLite 数据库里可能已经存在 `instances.status` 列。优先使用 `ALTER TABLE ... DROP COLUMN` 直接移除该列；如果实际运行环境或迁移验证不支持，再回退到表重建迁移方案。

## 拟定更改 (Proposed Changes)

### 后端存储层

#### [MODIFY] `backend/internal/store/sqlite.go`

- 从 `instances` 表结构中移除 `status` 列。
- 调整 `Save` 的写入 SQL，不再写入 `status`。
- 调整 `Get` / `List` 的查询列，不再读取 `status`。
- 新增或调整 schema 迁移逻辑，优先使用 `ALTER TABLE ... DROP COLUMN`，确保已有数据库可以平滑升级。

#### [MODIFY] `backend/internal/store/sqlite_test.go`

- 更新建表与 CRUD 测试，去掉对 `status` 持久化字段的断言。
- 增加迁移回归测试，覆盖旧库升级到新 schema 的场景。

### 后端服务层

#### [MODIFY] `backend/internal/service/docker_service.go`

- 保持运行态 `model.Instance.Status` 的计算逻辑不变。
- 清理或更新与“持久化状态同步”相关的注释，避免继续暗示 SQLite 是状态源。
- 收敛 `Save` 调用处的状态参数依赖，让业务语义与存储职责一致。

### 文档层

#### [MODIFY] `docs/design-docs/instance_lifecycle.md`

- 更新一致性说明：SQLite 只存实例元数据，状态以 Docker 查询结果为准。
- 说明历史状态记录不再由实例表承担。

#### [MODIFY] `docs/api/contracts.md`

- 如文档里仍明确提到状态落库，需要同步改成“状态由运行时计算/查询得到”。

## 执行步骤 (Execution Steps)

- [x] 存储层改造
  - [x] 移除 `instances.status` 的建表与查询逻辑
  - [x] 调整 `Save` 语句与入参使用方式
  - [x] 设计并实现旧库迁移路径，优先采用 `DROP COLUMN`
- [x] 测试更新
  - [x] 修改 `sqlite_test.go` 中所有状态持久化断言
  - [x] 增加迁移与兼容性测试
- [x] 服务层收敛
  - [x] 检查 `DockerService` 中对存储状态的依赖并清理注释
  - [x] 确认运行态 `status` 仍然正常由 Docker API 计算
- [ ] 文档同步
  - [ ] 更新实例生命周期设计说明
  - [ ] 校正文档中关于状态持久化的描述

## 待确认的疑问 (Open Questions)

> [!NOTE]
> **迁移实现方式**
>
> 迁移优先按 `ALTER TABLE instances DROP COLUMN status;` 处理；需要在实现时用旧数据库样本实际验证一次，确认 `modernc.org/sqlite` 当前版本下该语法可直接执行。

## 验证计划 (Verification Plan)

### 自动化测试 (Automated Tests)

- 执行后端单测，重点覆盖 `backend/internal/store/sqlite_test.go`。
- 执行后端整体测试，确认 `DockerService` 读取与写入路径没有回归。

### 手动验证 (Manual Verification)

- 启动带有旧 schema 的数据库文件，确认服务能够正常迁移并启动。
- 创建、启动、停止、删除实例，确认前端仍然能看到正确状态，但数据库里不再保存状态列。
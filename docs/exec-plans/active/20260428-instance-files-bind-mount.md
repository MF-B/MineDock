# Bind Mount 文件管理能力

提供基于 Bind Mount 的实例文件管理能力，后端落盘到宿主机目录并对外提供列表/上传/下载/删除/新建目录等 API，前端将现有 Mock 替换为真实接口调用。

## 需要评审的内容 (User Review Required)

> [!IMPORTANT]
> **Bind Mount 根目录与命名规则**
>
> 建议将实例卷映射到 `backend/data/instances/{instanceName}/volumes/{volumeName}`，其中 `instanceName` 与 `volumeName` 使用现有 `sanitizeVolumeNameToken` 规则归一化。该目录将承载文件管理的实际读写目标。

> [!IMPORTANT]
> **旧实例兼容策略**
>
> 决定：仅支持新建实例，旧实例不提供 Volume -> Bind Mount 迁移工具与文件管理入口。

> [!IMPORTANT]
> **删除实例时的数据策略**
>
> 决定：提供“删除实例并清理数据”的可选开关，默认仍保留宿主机目录。

> [!WARNING]
> **运行态文件写入的风险**
>
> 决定：允许运行态写入，风险由用户自行承担；只读卷仍禁止写操作。

## 拟定更改 (Proposed Changes)

### 后端：Bind Mount 目录与挂载构建

#### [MODIFY] backend/internal/service/docker_volume.go

- 将 `buildVolumeBinds` 从命名卷转换为 Bind Mount：
  - 新增 `buildBindMounts(baseDir, instanceName string, volumes []model.VolumeMount)`。
  - 通过 `sanitizeVolumeNameToken` 生成安全目录名。
  - 生成 Bind 格式：`{hostPath}:{containerPath}[:ro]`。

#### [MODIFY] backend/internal/service/docker.go

- `CreateInstance` / `UpdateInstanceConfig` 使用 Bind Mount 版 `buildBindMounts`。
- 统一由 `DockerService` 持有 `dataDir`（宿主机根目录），避免散落常量。
- 删除实例并清理数据：
  - `DeleteInstance(ctx, containerID, purgeData)`：容器删除成功后，若 `purgeData=true`，清理对应宿主机目录。
  - 目录路径复用 Bind Mount 规则：`{dataDir}/{instanceName}/volumes/{volumeName}`。
  - 删除逻辑需做路径安全校验，避免误删其他目录。

#### [MODIFY] backend/main.go

- 新增配置 `MINEDOCK_DATA_DIR`（默认 `data/instances`），初始化时 `os.MkdirAll`。
- 构建 `DockerService` 时注入 `dataDir`。

---

### 后端：文件管理服务与 API

#### [NEW] backend/internal/model/files.go

- 定义 `FileEntry`：`name`, `is_dir`, `size`, `modified_at`。
- 新增错误类型：`ErrMountNotFound`, `ErrPathInvalid`, `ErrReadOnlyMount`, `ErrFileNotFound` 等。

#### [NEW] backend/internal/service/files.go

- 新增 `FileService`：
  - `List(ctx, containerID, mountName, path)`
  - `CreateDir(ctx, containerID, mountName, path)`
  - `Delete(ctx, containerID, mountName, path, recursive)`
  - `OpenDownload(ctx, containerID, mountName, path)`
  - `SaveUpload(ctx, containerID, mountName, path, reader)`
- 通过 `InstanceStore` 获取实例 `name` + `game_id`，结合 `GameRegistry` 获取模板卷列表，确定可操作的挂载根目录。
- 路径安全：
  - API 层路径使用 POSIX 语义（`/`）解析，拒绝包含 `..` 或 `\`。
  - 通过 `filepath.Join` + `filepath.Rel` 确保目标在挂载根目录内。
  - 禁止跟随符号链接逃逸（可选：拒绝 symlink 或在 `EvalSymlinks` 后校验前缀）。
- 只读卷：仅允许 `List` / `Download`，写操作直接返回 `ErrReadOnlyMount`。
- 上传限制：默认限制单文件大小（如 128MB），并提供常量便于后续配置。

#### [NEW] backend/internal/api/files_handler.go

- 新增 HTTP 处理器：
  - `GET /api/instances/{id}/files/mounts`：返回可用挂载列表
  - `GET /api/instances/{id}/files`：列表（query: `mount`, `path`）
  - `POST /api/instances/{id}/files/dir`：新建目录（JSON body）
  - `POST /api/instances/{id}/files/upload`：上传文件（multipart/form-data）
  - `GET /api/instances/{id}/files/download`：下载文件（query: `mount`, `path`）
  - `DELETE /api/instances/{id}/files`：删除文件/目录（query: `mount`, `path`, `recursive`）
- 在 handler 层使用 `http.MaxBytesReader` 做上传大小限制。
- 错误映射：`ErrMountNotFound` -> 404，`ErrPathInvalid` -> 400，`ErrReadOnlyMount` -> 409。

#### [MODIFY] backend/internal/api/router.go

- 注册上述文件管理路由。

#### [MODIFY] backend/internal/api/handlers.go

- 文件管理错误映射：扩展 `mapErrorCode` 覆盖文件管理相关错误。
- 删除实例清理开关：
  - `DELETE /api/instances/{id}` 增加可选 query：`purge_data=true|false`。
  - `DeleteInstance` 透传清理开关给 Service。

---

### 前端：替换 Mock 文件管理

#### [MODIFY] frontend/src/api/index.ts

- 新增接口：
  - `listInstanceFileMounts(containerId)`
  - `listInstanceFiles(containerId, mount, path)`
  - `createInstanceDir(containerId, mount, path)`
  - `uploadInstanceFile(containerId, mount, path, file)`
  - `downloadInstanceFileUrl(containerId, mount, path)`
  - `deleteInstanceFile(containerId, mount, path, recursive)`
- 新增 `FileEntry` 与 `FileMount` 类型定义。
- `deleteInstance(containerId, purgeData?: boolean)` 支持清理开关

#### [MODIFY] frontend/src/views/InstanceFiles.vue

- 替换 Mock 状态为真实请求：
  - 初次进入时加载挂载列表并选择默认卷。
  - `breadcrumbs` 与 `files` 基于 API 返回更新。
  - `upload` 使用 `FormData`。
  - `download` 直接打开下载 URL。
- 异常处理使用 `ApiRequestError` 映射到 i18n。

#### [MODIFY] frontend/src/views/ContainerList.vue

- 删除确认弹窗增加“同时清理数据”开关，透传到删除接口。

#### [MODIFY] frontend/src/locales/en-US.json / zh-CN.json

- 增加文件管理错误提示（如 `files.errors.mountNotFound`, `files.errors.readOnly`, `files.errors.pathInvalid`）。

---

### 文档

#### [MODIFY] docs/api/contracts.md

- 新增文件管理 API 契约与返回结构说明。
- 说明 `mount` / `path` 语义与安全约束。
- `DELETE /api/instances/{id}` 增加 `purge_data` 可选参数说明。

#### [MODIFY] docs/design-docs/instance_lifecycle.md

- 在创建配置注入中补充：卷挂载采用 Bind Mount，并指向 `data/instances/...` 路径。

## 执行步骤 (Execution Steps)

- [x] 目录与配置
  - [x] 确认 Bind Mount 根目录规则（`MINEDOCK_DATA_DIR` 默认值与结构）
  - [x] 创建目录初始化逻辑（`MkdirAll`）
- [x] 后端挂载逻辑
  - [x] 在 `docker_volume.go` 实现 Bind Mount 构建
  - [x] `DockerService` 注入 `dataDir` 并在创建/重建时使用
- [x] 后端文件服务
  - [x] 新增 `FileService` 与路径安全校验
  - [x] 补齐只读卷限制与上传大小限制
- [x] 后端 API
  - [x] 新增 `files_handler.go` 与路由注册
  - [x] 完成错误映射与响应结构
- [x] 删除清理开关
  - [x] `DELETE /api/instances/{id}` 支持 `purge_data` 并透传到 Service
  - [x] 删除实例时按需清理宿主机目录
- [x] 前端接入
  - [x] 新增 API 调用与类型
  - [x] `InstanceFiles.vue` 接入真实接口
  - [x] i18n 文案补齐
- [x] 文档更新
  - [x] 更新 API 契约与实例生命周期说明
- [x] 测试与验证
  - [x] 新增后端单测与 handler 测试
  - [x] 执行 `task backend:test` / `task backend:lint` / `task backend:vet`

## 待确认的疑问 (Open Questions)

- 多挂载卷在前端如何呈现（下拉选择/默认首个可写卷）？已处理：后端支持多个，前端默认选择第一个；只有多个卷时显示选择框。
- 上传单文件大小限制是否需要可配置（环境变量）？

## 验证计划 (Verification Plan)

### 自动化测试 (Automated Tests)

- `task backend:test`
- `task backend:lint`
- `task backend:vet`

### 手动验证 (Manual Verification)

- 创建新实例后检查宿主机目录：`data/instances/{instanceName}/volumes/{volumeName}` 存在。
- 在文件页进入根目录，确认列表与面包屑正常。
- 新建文件夹、上传文件、下载文件、删除文件/目录均返回成功。
- 在只读卷上尝试上传/删除，返回明确错误提示。
- 若容器运行中允许写操作，验证服务端正常反映文件变化。
- 删除实例时勾选“清理数据”，确认宿主机目录被删除。

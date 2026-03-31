# 静态镜像注册表（后端 JSON 配置）

## 背景

当前 MineDock 创建容器时使用硬编码的 `alpine:latest` 镜像，用户无法选择容器镜像。本计划实现**静态镜像注册表**：后端通过 `registry.json` 配置文件维护一份受控的镜像列表，并暴露 API 供前端查询；同时改造创建容器接口，要求前端必须指定镜像 ID 进行创建。

本计划仅覆盖**后端部分**和**必要的前端 API/Store 适配**，不包含镜像市场 UI 页面（留作后续计划）。

## 需要评审的内容

> [!IMPORTANT]
> **API 变更（Breaking Change）**
>
> `POST /api/instances` 请求体从 `{ "name": "xxx" }` 变更为 `{ "name": "xxx", "image_id": "minecraft-java" }`。
>
> - `image_id` 为**必填**字段，未提供时返回 `400`。
> - `image_id` 不合法（即不在注册表中）时返回 `400`。
> - 同时移除 `MINEDOCK_IMAGE` 环境变量，镜像来源完全由注册表管控。

> [!IMPORTANT]
> **镜像校验逻辑**
>
> 创建容器时只允许使用注册表中存在的镜像，拒绝任意 Docker 镜像名称。这保证了安全可控性。

## 拟定更改

### 镜像注册表数据（新增）

#### [NEW] registry.json (`backend/registry.json`)

静态镜像目录配置文件，启动时由后端加载。初始包含以下镜像条目：

```json
[
  {
    "id": "minecraft-java",
    "name": "Minecraft Java Edition",
    "image": "itzg/minecraft-server:latest",
    "description": "最流行的 Minecraft Java 版服务器，支持原版/Forge/Fabric/Paper 等多种模组加载器。",
    "category": "minecraft",
    "icon": "minecraft-java",
    "default_env": {
      "EULA": "TRUE",
      "TYPE": "PAPER"
    },
    "default_ports": ["25565:25565"]
  },
  {
    "id": "minecraft-bedrock",
    "name": "Minecraft Bedrock Edition",
    "image": "itzg/minecraft-bedrock-server:latest",
    "description": "Minecraft 基岩版服务器，支持手机/主机/Win10 跨平台联机。",
    "category": "minecraft",
    "icon": "minecraft-bedrock",
    "default_env": {
      "EULA": "TRUE"
    },
    "default_ports": ["19132:19132/udp"]
  },
  {
    "id": "terraria",
    "name": "Terraria",
    "image": "ryshe/terraria:latest",
    "description": "Terraria 专用服务器，支持 tShock 管理。",
    "category": "sandbox",
    "icon": "terraria",
    "default_env": {},
    "default_ports": ["7777:7777"]
  }
]
```

> [!NOTE]
> `default_env` 和 `default_ports` 本期**不会**在创建容器时生效（当前 `ContainerCreate` 尚未支持环境变量和端口映射）。它们作为元数据保留，为后续功能做准备。本期仅使用 `image` 字段来决定创建容器时使用的 Docker 镜像。

---

### 后端 Model 层

#### [NEW] registry.go (`backend/internal/model/registry.go`)

新增 `RegistryImage` 结构体，映射 `registry.json` 中的条目：

```go
// RegistryImage 描述注册表中的一个可用镜像条目。
type RegistryImage struct {
    ID           string            `json:"id"`
    Name         string            `json:"name"`
    Image        string            `json:"image"`
    Description  string            `json:"description"`
    Category     string            `json:"category"`
    Icon         string            `json:"icon"`
    DefaultEnv   map[string]string `json:"default_env"`
    DefaultPorts []string          `json:"default_ports"`
}
```

#### [MODIFY] errors.go (`backend/internal/model/errors.go`)

新增错误变量：

```go
// ErrImageNotFound 表示请求的镜像 ID 不在注册表中。
var ErrImageNotFound = errors.New("image not found in registry")
```

---

### 后端 Service 层

#### [NEW] registry_service.go (`backend/internal/service/registry_service.go`)

新增 `RegistryService`，职责：

- 从指定路径读取 `registry.json`，解码后在内存中缓存
- 启动时校验：JSON 合法性、ID 非空且唯一、`image` 非空
- 暴露以下方法：
  - `ListImages(ctx) -> []RegistryImage` — 返回完整列表
  - `GetImage(ctx, id) -> (RegistryImage, error)` — 按 ID 查找，不存在时返回 `model.ErrImageNotFound`

```go
// RegistryService 提供可用镜像注册表的查询能力。
type RegistryService struct {
    images   []model.RegistryImage
    imageMap map[string]model.RegistryImage
}

// NewRegistryService 从 JSON 文件加载镜像数据并构建查找索引。
func NewRegistryService(filePath string) (*RegistryService, error) { ... }

// ListImages 返回注册表中全部可用镜像。
func (s *RegistryService) ListImages(_ context.Context) []model.RegistryImage { ... }

// GetImage 按 ID 查找镜像，未找到时返回 model.ErrImageNotFound。
func (s *RegistryService) GetImage(_ context.Context, id string) (model.RegistryImage, error) { ... }
```

#### [MODIFY] docker_service.go (`backend/internal/service/docker_service.go`)

- **`DockerService` 结构体变更**：移除 `image string` 字段，新增 `registry` 依赖（消费方接口）
- **`NewDockerService` 签名变更**：用 `ImageRegistry` 接口替代 `imageName string` 参数
- **`CreateInstance` 签名变更**：`CreateInstance(ctx, name, imageID string)` — 增加 `imageID` 参数
  - 通过 `registry.GetImage(ctx, imageID)` 校验镜像 ID，不存在则返回 `model.ErrImageNotFound`
  - 使用查到的 `RegistryImage.Image` 作为 Docker 镜像名调用 `ensureImage` 和 `ContainerCreate`
- **移除 `defaultImage` 常量**和相关回退逻辑

在 service 包内定义消费方接口：

```go
// ImageRegistry 定义 DockerService 依赖的镜像注册表查询能力。
type ImageRegistry interface {
    GetImage(ctx context.Context, id string) (model.RegistryImage, error)
}
```

---

### 后端 API 层

#### [NEW] registry_handlers.go (`backend/internal/api/registry_handlers.go`)

新增 `RegistryHandler`：

```go
// RegistryLister 定义注册表 Handler 依赖的查询操作。
type RegistryLister interface {
    ListImages(ctx context.Context) []model.RegistryImage
}

// RegistryHandler 暴露镜像注册表相关 HTTP 处理器。
type RegistryHandler struct {
    registry RegistryLister
}

// NewRegistryHandler 创建 RegistryHandler。
func NewRegistryHandler(r RegistryLister) *RegistryHandler { ... }

// GetImages 处理 GET /api/registry/images，返回可用镜像列表。
func (h *RegistryHandler) GetImages(w http.ResponseWriter, r *http.Request) { ... }
```

#### [MODIFY] handlers.go (`backend/internal/api/handlers.go`)

- `createRequest` 增加 `ImageID string \`json:"image_id"\`` 字段
- `InstanceService` 接口的 `CreateInstance` 签名更新为 `CreateInstance(ctx context.Context, name string, imageID string) (string, error)`
- `CreateInstance` Handler 增加 `image_id` 非空校验，为空时返回 `400`
- `CreateInstance` Handler 调用 Service 时传入 `req.ImageID`
- `mapErrorCode` 增加 `model.ErrImageNotFound -> 400` 的映射

#### [MODIFY] router.go (`backend/internal/api/router.go`)

- `NewRouter` 签名变更：接受 `*Handler` 和 `*RegistryHandler` 两个参数
- 注册新路由：`GET /api/registry/images`

---

### 后端入口

#### [MODIFY] main.go

- 移除 `MINEDOCK_IMAGE` 环境变量读取逻辑
- 新增 `RegistryService` 初始化：从 `registry.json` 加载（路径可通过 `MINEDOCK_REGISTRY_PATH` 环境变量覆盖，默认 `registry.json`）
- 将 `RegistryService` 注入 `DockerService`（替代原有的 `imageName` 参数）
- 创建 `RegistryHandler` 并注入 `RegistryService`
- 更新 `NewRouter` 调用

---

### 前端 API 层

#### [MODIFY] index.ts (`frontend/src/api/index.ts`)

- 新增 `RegistryImage` 接口类型
- 新增 `listRegistryImages()` API 函数
- `createInstance` 函数签名变更：接受 `name` 和 `imageId` 两个必填参数

---

### 前端状态管理

#### [NEW] registry.ts (`frontend/src/stores/registry.ts`)

新增 Pinia store：

- `images: RegistryImage[]` — 可用镜像列表
- `loading: boolean` — 加载状态
- `fetchImages()` — 从后端加载镜像列表
- `getById(id)` — 按 ID 查找镜像

#### [MODIFY] containers.ts (`frontend/src/stores/containers.ts`)

- `create` action 签名变更：`create(name: string, imageId: string)`
- 调用 `apiCreate(name, imageId)` 时传入镜像参数

---

### 文档更新

#### [MODIFY] contracts.md (`docs/api/contracts.md`)

新增 API 文档：

````markdown
### GET /api/registry/images

- 说明：获取注册表中所有可用的容器镜像列表
- 状态码：
  - 成功：`200`
  - 失败：`500`
- 请求参数：无
- 返回结果：

\```json
[{
"id": "minecraft-java",
"name": "Minecraft Java Edition",
"image": "itzg/minecraft-server:latest",
"description": "...",
"category": "minecraft",
"icon": "minecraft-java",
"default_env": { "EULA": "TRUE" },
"default_ports": ["25565:25565"]
}]
\```
````

更新 `POST /api/instances` 请求参数说明，增加 `image_id` 字段。

#### [MODIFY] instance_lifecycle.md (`docs/design-docs/instance_lifecycle.md`)

- Instance 结构体新增 `ImageID` 字段说明（用于标识容器基于的注册表镜像）

---

## 执行步骤

- [ ] 后端 Model 层
  - [ ] 新建 `backend/internal/model/registry.go`，定义 `RegistryImage` 结构体
  - [ ] 修改 `backend/internal/model/errors.go`，新增 `ErrImageNotFound`
- [ ] 后端注册表数据
  - [ ] 新建 `backend/registry.json`，写入初始镜像条目
- [ ] 后端 Service 层
  - [ ] 新建 `backend/internal/service/registry_service.go`，实现 `RegistryService`
  - [ ] 编写 `registry_service_test.go` 单元测试（加载合法/非法 JSON、按 ID 查询）
  - [ ] 修改 `backend/internal/service/docker_service.go`：移除 `image` 字段和 `defaultImage` 常量、新增 `ImageRegistry` 接口依赖、更新 `CreateInstance` 签名
- [ ] 后端 API 层
  - [ ] 新建 `backend/internal/api/registry_handlers.go`，实现 `RegistryHandler`
  - [ ] 修改 `backend/internal/api/handlers.go`：`createRequest` 增加 `ImageID`、`InstanceService` 接口同步更新
  - [ ] 修改 `backend/internal/api/router.go`：注册 `GET /api/registry/images`
  - [ ] 更新 `handlers_test.go`：适配新的 `CreateInstance` 签名
- [ ] 后端入口
  - [ ] 修改 `backend/main.go`：移除 `MINEDOCK_IMAGE`、初始化 `RegistryService`、注入 `DockerService`、创建 `RegistryHandler`
- [ ] 前端适配
  - [ ] 修改 `frontend/src/api/index.ts`：新增 `RegistryImage` 类型和 `listRegistryImages()` 函数、更新 `createInstance` 签名
  - [ ] 新建 `frontend/src/stores/registry.ts`：镜像注册表 Pinia store
  - [ ] 修改 `frontend/src/stores/containers.ts`：`create` action 增加 `imageId` 参数
- [ ] 文档更新
  - [ ] 修改 `docs/api/contracts.md`：新增 `GET /api/registry/images`、更新 `POST /api/instances`
  - [ ] 修改 `docs/design-docs/instance_lifecycle.md`：补充 `ImageID` 字段

## 已确认的决策

- ✅ 初始镜像列表：Minecraft Java、Minecraft Bedrock、Terraria 三个条目
- ✅ 字段命名：使用 `image_id` 引用注册表 ID
- ✅ `image_id` 为必填字段，不提供时返回 `400`
- ✅ 移除 `MINEDOCK_IMAGE` 环境变量
- ✅ 移除 `recommended` 和 `tags` 字段

## 验证计划

### 自动化测试

- `task backend:test` — 覆盖：
  - `RegistryService`：加载合法 JSON / 格式错误 JSON / ID 重复校验
  - `RegistryService.GetImage`：存在的 ID / 不存在的 ID
  - `DockerService.CreateInstance`（mock）：有效 imageID、无效 imageID 返回 400
  - `Handler` 层：更新后的请求体解析与错误映射
- `task backend:vet && task backend:lint`
- 前端：`npm run lint`

### 手动验证

- 启动后端，调用 `GET /api/registry/images`，验证返回完整镜像列表
- 调用 `POST /api/instances` 传入合法 `image_id`，验证容器使用了正确镜像
- 调用 `POST /api/instances` 不传 `image_id`，验证返回 `400`
- 调用 `POST /api/instances` 传入非法 `image_id`，验证返回 `400`

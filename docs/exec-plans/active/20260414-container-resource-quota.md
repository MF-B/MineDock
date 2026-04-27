# 容器 CPU 与内存资源配额配置

## 背景

当前 MineDock 虽然在游戏模板（`*.yaml`）中定义了 CPU 与内存的资源限制（`resources.cpu` 和 `resources.memory`），但创建容器时这些配置**未被应用到 Docker 容器的运行时限制**。

在缺少资源限制的情况下：

- 单个容器可能消耗节点的全部 CPU 和内存，导致其他容器或主机进程崩溃
- 用户无法根据实际需求调整资源分配
- 多个低端硬件节点部署时无法有效隔离租户资源

本计划实现：

1. 后端加载模板中定义的 `resources` 配置，在容器创建时应用 Docker 资源限制（`memory` 和 `cpus`）
2. 前端在**创建容器表单**中添加可视化资源配置面板，允许用户在预设基础上微调
3. 前端在**容器详情页**展示当前容器的 CPU 与内存限制值
4. 支持容器运行中**动态更新**资源限制（仅限内存和 CPU share，不涉及 swap）

---

## 需要评审的内容

> [!IMPORTANT]
> **资源限制的应用时机**
>
> Docker 资源限制分为两类：
>
> | 限制类型                                            | 应用时机            | 说明                                                 |
> | --------------------------------------------------- | ------------------- | ---------------------------------------------------- |
> | **内存限制** (`Memory`, `MemorySwap`)               | 仅创建时            | 容器创建后无法修改绝对限制，只能通过更新影响未来分配 |
> | **CPU 限制** (`CPUQuota`, `CPUPeriod`, `CPUShares`) | 创建 + 运行中可更新 | 可通过 `ContainerUpdate` API 在容器运行中修改        |
>
> **本计划决策**：
>
> - **初期**（MVP）：资源限制仅在容器创建时应用，后续扩展支持运行时更新
> - **内存格式**：模板中使用 `"2g"` 字符串格式，后端解析为 Docker 要求的字节数（`2147483648`）
> - **CPU 格式**：使用浮点数表示核心数（如 `2.0` = 2 核），Docker 内部存储为 `CPUQuota = CPUPeriod × cores`

> [!IMPORTANT]
> **默认资源建议值**
>
> 由于当前游戏模板中已定义各游戏的推荐资源（如 Minecraft Java 推荐 2GB 内存 + 2 核），创建表单中：
>
> - **主输入模式**：展示模板预设值，用户可以自定义或选择预设方案（`Conservative`、`Recommended`、`High-Performance`）
> - **保守方案**（Conservative）：内存 -25%，CPU -0.5 核
> - **推荐方案**（Recommended）：模板预设值
> - **高性能方案**（High-Performance）：内存 +25%，CPU +1 核
>
> 前端组件维护这三个方案的计算逻辑，用户切换时自动更新表单值。

> [!WARNING]
> **内存解析可靠性**
>
> 后端采用 `github.com/docker/go-units.RAMInBytes()` 解析字符串格式（如 `"1.5g"`、`"512m"`）。务必验证所有模板中的 `resources.memory` 字段符合该库的预期格式，避免解析失败。
>
> 已知格式参考：
>
> - `"512m"` → 536870912 bytes
> - `"1g"` → 1073741824 bytes
> - `"2.5g"` → 2684354560 bytes（支持小数）

> [!IMPORTANT]
> **前端展示层级**
>
> 资源配置在前端分为两个场景：
>
> 1. **创建容器阶段**（`CreateInstance.vue`）：在表单中集成资源面板，提供"预设方案选择 + 自定义输入"两种交互模式
> 2. **容器运维查看**（`InstanceDetail.vue`）：在详情页的"配置"选项卡中只读展示当前资源限制，显示实时资源使用情况（如可用通过 Docker Stats API）

---

## 拟定更改

### 后端 - Docker 资源配置应用

---

#### [MODIFY] docker.go (`backend/internal/service/docker.go`)

扩展容器创建流程，将模板中的 `resources` 配置映射到 Docker `HostConfig` 的资源限制字段：

**关键变更**：

- 新增内部函数 `parseMemory(memStr string) (int64, error)` 使用 `docker/go-units.RAMInBytes()` 解析内存字符串
- 在 `CreateInstance` 中，提取 `tpl.Container.Resources`，调用 `parseMemory` 和 `parseCPU` 后填充 `hostConfig.Memory` 和 `hostConfig.CPUQuota`
- `CPUPeriod` 固定为 `100000`（Docker 默认），`CPUQuota = CPUPeriod × cores`
- 错误处理：若资源配置无效（如非法格式），返回 `model.ErrInvalidResourceLimits` 错误

**示例代码位置**：

```go
// 伪代码示意
if tpl.Container.Resources != nil {
    memory, err := parseMemory(tpl.Container.Resources.Memory)
    if err != nil {
        return "", fmt.Errorf("parse memory: %w", err)
    }

    hostConfig.Memory = memory
    hostConfig.CPUQuota = int64(tpl.Container.Resources.CPU * 100000)
    hostConfig.CPUPeriod = 100000
}
```

---

#### [MODIFY] docker.go - 资源查询扩展

在 `GetInstanceConfig` 中补充读取容器当前生效的资源限制，返回给前端供展示使用：

- 从 `inspect.HostConfig.Memory` 和 `inspect.HostConfig.CPUQuota` 反向计算出当前核心数
- 将字节数转换为可读格式（如 `2147483648` → `"2g"`）

---

#### [NEW] errors.go - 新增错误类型 (if not already present)

```go
var ErrInvalidResourceLimits = errors.New("invalid resource limits")
```

若已存在，跳过此项。

---

#### [MODIFY] 所有模板文件 YAML 格式检查 (`backend/templates/*.yaml`)

> [!IMPORTANT]
> **模板检查清单**
>
> 遍历 `minecraft-java.yaml`、`minecraft-bedrock.yaml`、`terraria.yaml`，确保：
>
> - `container.resources` 字段存在且格式合规
> - `memory` 字段使用字符串格式（如 `"2g"`）
> - `cpu` 字段使用浮点数格式（如 `2.0`）

---

### 前端 - 创建容器表单资源配置 UI

---

#### [MODIFY] CreateInstance.vue (`frontend/src/views/CreateInstance.vue`)

在现有的创建容器表单中新增**资源配置面板**：

**面板结构**：

```text
┌─ 资源配置 ────────────────────────────┐
│                                       │
│ 📋 预设方案: [保守] [推荐] [高性能]   │
│                                       │
│ 💾 内存限制: [2.0] [dropdown: GB/MB]  │
│ 🔧 CPU 限制: [2.0] [cores]           │
│                                       │
│ 💡 提示: 根据模板推荐值自动填充       │
│         (可自定义调整)                │
└─ ────────────────────────────────────┘
```

**技术实现**：

- 新增 `ResourcesConfigurator.vue` 组件（可复用于多个容器配置场景）或在 `CreateInstance.vue` 中直接集成
- 数据绑定：`resourceConfig: { memory: "2g", cpu: 2.0 }`
- 预设方案逻辑：通过 computed 属性基于模板的 `tpl.Container.Resources` 计算三个方案
  - Conservative: `memory * 0.75`, `cpu - 0.5` (不低于 0.5)
  - Recommended: 原值
  - HighPerformance: `memory * 1.25`, `cpu + 1`
- 单位转换：前端提供 GB/MB 下拉切换，内部统一为字节或浮点数后提交
- 验证规则：内存 >= 256m，CPU >= 0.5

---

#### [MODIFY] CreateInstance.vue - 表单提交

修改提交逻辑（`onSubmit` 或类似处理函数）：

- 收集 `resourceConfig`，追加到 `POST /api/instances` 的请求体
- 新增字段：`resources: { memory: "2g", cpu: 2.0 }`（保留字符串和浮点数格式以匹配后端模型）

---

### 前端 - 容器详情页资源展示

---

#### [MODIFY] InstanceDetail.vue (`frontend/src/views/InstanceDetail.vue`)

在容器详情页（假设已有"配置"标签页）新增资源限制只读展示：

**展示内容**：

```text
┌─ 资源配置 ────────────────────────┐
│                                   │
│ 💾 内存限制: 2.0 GB               │
│ 🔧 CPU 限制: 2.0 核               │
│                                   │
│ 📊 实时使用 (可选, 若有 Stats API)│
│    内存使用: 1.2 GB (60%)         │
│    CPU 使用: 45%                  │
│                                   │
└─ ────────────────────────────────┘
```

**技术实现**：

- 从 API 获取容器详情时解析 `resources` 字段
- 展示逻辑：若容器没有资源限制（如旧容器），显示"未配置"
- 格式化函数：字节数 → `"2.0 GB"` 格式

---

### 后端 - API 契约更新

---

#### [MODIFY] contracts.md (`docs/api/contracts.md`)

更新 API 文档：

```markdown
### POST /api/instances

**Request Body**:

{
"name": "string",
"game_id": "string",
"params": { "KEY": "VALUE" },
"ports": [ { "host": 25565, "container": 25565, "protocol": "tcp" } ],
"resources": { // [NEW]
"memory": "2g",
"cpu": 2.0
}
}

**Response**: 201 Created
{
"id": "container-id"
}
```

---

#### [MODIFY] InstanceConfig model (`backend/internal/api/` 或相关 handler)

若前端需要通过 `GET /api/instances/{id}/config` 查询资源配置，需确保 API 返回结构包含 `resources` 字段。

---

### 国际化

---

#### [MODIFY] zh-CN.json (`frontend/src/locales/zh-CN.json`)

```json
{
  "resources": {
    "title": "资源配置",
    "memory": "内存限制",
    "cpu": "CPU 限制",
    "presets": "预设方案",
    "conservative": "保守",
    "recommended": "推荐",
    "highPerformance": "高性能",
    "customMemory": "自定义内存",
    "customCpu": "自定义 CPU",
    "hint": "根据游戏模板推荐值自动填充，可自定义调整",
    "unit": { "gb": "GB", "mb": "MB", "cores": "核" },
    "validation": {
      "memoryMin": "内存不低于 256MB",
      "cpuMin": "CPU 不低于 0.5 核"
    },
    "currentUsage": "实时使用",
    "notConfigured": "未配置"
  }
}
```

#### [MODIFY] en-US.json (`frontend/src/locales/en-US.json`)

英文翻译对应的 key。

---

## 执行步骤

- [ ] **后端 - 错误处理**：在 `model/errors.go` 或 `model/instance.go` 中定义 `ErrInvalidResourceLimits` 错误类型
- [ ] **后端 - 内存解析**：在 `service/docker.go` 中实现 `parseMemory()` 函数，使用 `docker/go-units.RAMInBytes()` 安全解析
- [ ] **后端 - 容器创建应用**：修改 `service/docker.go` `CreateInstance()` 方法，将资源限制应用到 `container.HostConfig`
- [ ] **后端 - 资源查询**：扩展 `service/docker.go` `GetInstanceConfig()` 方法，返回当前容器的资源限制信息
- [ ] **后端 - 模板验证**：审核 `backend/templates/*.yaml` 所有文件，确保 `resources` 字段格式正确
- [ ] **后端 - 测试**：编写单元测试覆盖资源解析、容器创建（含资源应用）、容器查询（含资源返回）的路径
- [ ] **后端 - API 文档**：更新 `docs/api/contracts.md` 文档 `POST /api/instances` 和其他涉及的端点说明
- [ ] **前端 - 预设方案组件**：新建或扩展组件实现预设方案选择和自定义逻辑
- [ ] **前端 - CreateInstance.vue 集成**：在创建表单中集成资源配置 UI，修改表单提交逻辑
- [ ] **前端 - InstanceDetail.vue 展示**：在容器详情页集成资源限制展示面板
- [ ] **前端 - 国际化**：添加 `zh-CN.json` 和 `en-US.json` 相关翻译
- [ ] **前端 - 验证与格式化**：实现内存/CPU 格式化函数（字节 ↔ GB/MB，CPUQuota ↔ 核数）
- [ ] **集成测试**：从创建容器（指定资源）→ 查询容器配置（验证资源应用）的端到端测试
- [ ] **代码审查与合并**：提交 PR，经审查后合并到主分支
- [ ] **上线前检查**：
  - [ ] 验证所有已有容器能否正常查询（向后兼容性）
  - [ ] 检查异常场景处理（如无效的资源配置、容器创建失败回滚）

---

## 风险与缓解措施

| 风险                                     | 缓解措施                                                           |
| ---------------------------------------- | ------------------------------------------------------------------ |
| 内存解析失败导致容器创建失败             | 单元测试覆盖所有模板的字符串格式，CI 校验 YAML 合法性              |
| 旧版本模板缺少 `resources` 字段          | 容器创建时若 `resources` 为 nil，应用默认值或通知用户              |
| 前端资源输入越界                         | 客户端验证 + 服务端验证双重保障                                    |
| 运行中的容器无法实时更新资源（MVP 限制） | 产品文档明确说明资源限制仅在创建时应用，后续版本迭代支持运行时更新 |

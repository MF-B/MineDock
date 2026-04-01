# 镜像市场前端页面

## 背景

静态镜像注册表后端已实现（`GET /api/registry/images`、`POST /api/instances` 需要 `image_id`）。前端 API 层（`listRegistryImages()`）和 Pinia store（`useRegistryStore`）也已就绪。当前创建容器时通过 `ContainerList.vue` 弹窗内的下拉框选择镜像，但缺少一个独立的**镜像市场页面**让用户直观浏览、筛选、选用镜像。

本计划实现 `/registry` 路由下的镜像市场页面，作为发现和选用镜像的主入口。

## 需要评审的内容

> [!IMPORTANT]
> **镜像市场 → 创建容器的交互流程**
>
> 用户在镜像市场**直接点击整张游戏卡片**，使用 Vue Router 跳转到容器列表页 `/`，并通过 query 参数 `?imageId=minecraft-java` 传递预选镜像。`ContainerList.vue` 检测到该参数后自动弹出创建弹窗并预填镜像。
>
> 卡片上不设独立的「选用」按钮，卡片本身即为点击入口，交互更简洁直接。

> [!IMPORTANT]
> **按 category 筛选**
>
> 镜像卡片支持按 `category` 筛选。筛选栏提供「全部」及各 `category`（从后端数据动态提取）选项卡。当前 registry.json 中有 `minecraft` 和 `sandbox` 两个分类。

## 拟定更改

### 路由

#### [MODIFY] index.ts (`frontend/src/router/index.ts`)

- 新增路由 `/registry`，路由名 `ImageRegistry`，懒加载 `views/ImageRegistry.vue`

---

### 视图层

#### [NEW] ImageRegistry.vue (`frontend/src/views/ImageRegistry.vue`)

**镜像市场页面**，页面结构：

```text
┌─────────────────────────────────────────┐
│ page-header: "镜像市场" 标题            │
├─────────────────────────────────────────┤
│ 筛选栏: [全部] [minecraft] [sandbox]    │
├─────────────────────────────────────────┤
│ ┌─────────┐ ┌─────────┐ ┌─────────┐   │
│ │  ⛏️     │ │  ⛏️     │ │  🌳    │   │
│ │ MC Java │ │ MC 基岩 │ │ 泰拉瑞亚│   │
│ │ 描述... │ │ 描述... │ │ 描述... │   │
│ │ minecraft│ │ minecraft│ │ sandbox │   │
│ └─────────┘ └─────────┘ └─────────┘   │
│   (整张卡片可点击 → 跳转创建容器)       │
│                                         │
│ 空状态 / 加载中 提示                    │
└─────────────────────────────────────────┘
```

关键技术细节：

- **数据来源**：使用 `useRegistryStore`，`onMounted` 时调用 `fetchImages()`
- **分类筛选**：`computed` 动态提取所有 category，`selectedCategory` ref 控制当前筛选项，`filteredImages` computed 过滤结果
- **图标方案**：前端维护 `icon → emoji` 映射表（如 `minecraft-java → ⛏️`，`minecraft-bedrock → ⛏️`，`terraria → 🌳`），未匹配时回退到名称首字母
- **卡片展示**：每张卡片作为一个完整的游戏入口，展示 emoji 图标、`name`（游戏名称）、`description`（简介，可截断为 2~3 行）、`category` 分类徽标。**不显示**端口列表、环境变量、选用按钮
- **卡片点击**：整张卡片可点击，`cursor: pointer` + hover 反馈，点击执行 `router.push({ name: 'ContainerList', query: { imageId: image.id } })`
- **空状态**：无镜像时显示空状态提示
- **加载态**：`registryStore.loading` 为 `true` 时展示加载占位
- **样式**：遵循 Create 主题 CSS 变量体系，卡片复用 `--card-*` 系列变量，响应式网格布局（桌面 3 列，平板 2 列，手机 1 列）

#### [MODIFY] ContainerList.vue (`frontend/src/views/ContainerList.vue`)

- `onMounted` 中检测 `route.query.imageId`，若存在：
  - 等待 `registryStore.fetchImages()` 完成
  - 校验 imageId 在注册表中存在
  - 设置 `selectedImageId` 为该值
  - 自动打开创建弹窗
  - 清除 URL query 参数（`router.replace({ query: {} })`）防止刷新重触发

---

### 侧边栏

#### [MODIFY] Sidebar.vue (`frontend/src/components/Sidebar.vue`)

- 在「容器列表」菜单项下方新增「镜像市场」菜单项
- 路由指向 `/registry`
- 图标使用网格/市场风格 SVG（4 格方块或拼图图标）
- 移动端和桌面端菜单同步新增

---

### 国际化

#### [MODIFY] zh-CN.json (`frontend/src/locales/zh-CN.json`)

新增翻译 key：

```json
{
  "registry": {
    "title": "镜像市场",
    "filterAll": "全部",
    "emptyState": "暂无可用镜像。",
    "loading": "正在加载镜像列表...",
    "loadError": "镜像列表加载失败，请稍后重试。"
  },
  "sidebar": {
    "imageRegistry": "镜像市场"
  }
}
```

#### [MODIFY] en-US.json (`frontend/src/locales/en-US.json`)

```json
{
  "registry": {
    "title": "Image Marketplace",
    "filterAll": "All",
    "emptyState": "No images available.",
    "loading": "Loading image list...",
    "loadError": "Failed to load image list. Please try again later."
  },
  "sidebar": {
    "imageRegistry": "Images"
  }
}
```

---

### Store 层（可选增强）

#### [MODIFY] registry.ts (`frontend/src/stores/registry.ts`)

- 新增 `categories` computed getter：从 `images` 中提取去重的 category 列表
- 新增 `error` ref：`fetchImages` 失败时记录错误状态，视图层据此渲染错误提示

---

### 文档

#### [MODIFY] frontend.md (`docs/standards/frontend.md`)

- 更新路由表：新增 `/registry` → `ImageRegistry.vue` 说明

---

## 执行步骤

- [x] Store 层增强
  - [x] 修改 `frontend/src/stores/registry.ts`：新增 `categories` computed 和 `error` ref
- [x] 路由注册
  - [x] 修改 `frontend/src/router/index.ts`：新增 `/registry` 路由（懒加载）
- [x] 镜像市场页面
  - [x] 新建 `frontend/src/views/ImageRegistry.vue`
  - [x] 实现页面标题区（复用 `.page-header` / `.page-title` 样式模式）
  - [x] 实现分类筛选栏（category tabs）
  - [x] 实现镜像卡片网格（响应式 grid）
  - [x] 实现卡片内容：emoji 图标、名称、描述、分类徽标
  - [x] 实现加载态和空状态
  - [x] 实现整张卡片点击 → 跳转到容器列表页并传入 `imageId` query
- [x] 容器列表页适配
  - [x] 修改 `frontend/src/views/ContainerList.vue`：检测 `route.query.imageId`，自动预填镜像并弹出创建弹窗
- [x] 侧边栏更新
  - [x] 修改 `frontend/src/components/Sidebar.vue`：添加「镜像市场」导航项（桌面端 + 移动端）
- [x] 国际化
  - [x] 修改 `frontend/src/locales/zh-CN.json`：新增 `registry.*` 和 `sidebar.imageRegistry`
  - [x] 修改 `frontend/src/locales/en-US.json`：同步英文翻译
- [x] 文档
  - [x] 修改 `docs/standards/frontend.md`：更新路由说明

## 已确认的决策

- ✅ 镜像图标方案：Emoji / Unicode 映射（`minecraft-java → ⛏️`，`minecraft-bedrock → ⛏️`，`terraria → 🌳`），未匹配时回退首字母
- ✅ 卡片只展示：emoji 图标 + 游戏名称 + 描述 + 分类徽标
- ✅ 不显示：端口列表、环境变量、独立选用按钮
- ✅ 整张卡片可点击，直接跳转到创建容器流程

## 验证计划

### 自动化测试

- 前端：`npm run lint`（ESLint + Prettier + TypeScript 检查）

### 手动验证

- 启动前后端，在侧边栏点击「镜像市场」，验证页面正常渲染
- 验证分类筛选栏：点击各分类 tab，卡片正确过滤
- 验证卡片点击：点击整张卡片后跳转到容器列表页，创建弹窗自动弹出且镜像已预填
- 创建弹窗内完成创建，验证容器使用了预选的镜像
- 手动刷新 `/registry` 页面，验证数据正常重新加载
- 验证响应式布局：桌面端 3 列 → 平板 2 列 → 手机 1 列
- 验证 i18n：切换中英文，所有新增文案正确显示
- 验证空状态：后端返回空数组时，页面展示友好提示

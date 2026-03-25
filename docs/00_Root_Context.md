# MineDock Root Context

## 1. 项目核心定义与最终目标
MineDock 是一个用于管理本地 Docker 容器实例的最小可行系统。
当前阶段目标是：跑通一个测试容器的“创建”与“销毁”完整流程，并提供最基本的列表查询能力。

## 2. 全局系统边界
- 后端仅实现对本地 Docker 引擎的调用。
- 当前阶段不涉及 SQLite 数据库读写，所有状态数据仅存储在内存中。
- 前端仅提供三个交互动作：
  - 获取列表
  - 开服（创建并启动容器）
  - 停服（停止并销毁容器）
- 暂不处理 Cgroups 内存限制与跨环境一致性问题。
- 直接使用现成公开镜像进行流程验证。

## 3. 全局命名与契约约定
- HTTP 接口统一挂载在 `/api` 前缀下。
- 资源命名采用复数形式：`/api/instances`。
- JSON 字段命名采用 snake_case（如 `container_id`）。
- 实例状态字段 `status` 的业务值遵循：
  - `Running`
  - `Stopped`

## 4. 顶层目录架构规范
MineDock/
├── docs/                # 架构与文档
├── backend/             # Go 后端代码
│   ├── main.go          #   程序入口
│   └── internal/        #   内部包
│       ├── api/         #     路由
│       ├── model/       #     数据结构定义
│       ├── service/     #     业务逻辑
│       ├── store/       #     内存状态管理
│       └── utils/       #     工具函数
└── frontend/            # 前端代码
    └── src/
        ├── api/         #   HTTP 请求封装
        ├── components/  #   UI 组件
        ├── composables/ #   组合式函数
        ├── stores/      #   状态管理
        ├── locales/     #   i18n 语言文件
        └── assets/      #   静态资源

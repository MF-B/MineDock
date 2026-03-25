# Ops & Infrastructure

本文档定义了 MineDock 项目的本地开发工作流、自动化构建与 CI/CD 规范。

## 1. 核心构建工具
项目根目录维护一个统一的 `Makefile`，屏蔽底层各语言组件的启动差异。
**当前支持的指令：**

| 指令 | 作用 | 说明 |
| --- | --- | --- |
| `make` / `make help` | 查看可用命令 | 输出当前支持的目标列表 |
| `make dev` | 一键启动前后端开发服务 | 并行执行后端与前端开发进程 |
| `make build` | 统一编译前后端 | 生成后端二进制与前端静态资源 |
| `make clean` | 清理构建产物 | 清理 `backend/bin` 与 `frontend/dist` |
| `make install-frontend` | 安装前端依赖 | 在 `frontend` 目录执行 `npm install` |

## 2. 环境依赖约束

本地开发与构建至少需要以下工具：

| 组件 | 用途 |
| --- | --- |
| Go | 后端编译与运行 |
| Node.js + npm | 前端依赖安装、开发与构建 |
| Docker Engine | 后端容器生命周期能力依赖 |
| make | 统一任务入口 |

## 3. CI/CD 对接规范（预留）

后续接入 GitHub Actions 时，建议直接复用根目录 Makefile 目标作为流水线步骤，保持本地与 CI 行为一致：

1. 依赖准备（Node/Go 环境）
2. 执行 `make install-frontend`
3. 执行 `make build`
4. 按需执行 `make clean`

## 4. 本地开发工作流

推荐执行顺序如下：

1. 安装前端依赖：`make install-frontend`
2. 启动开发环境：`make dev`
3. 发布前构建验证：`make build`
4. 收尾清理：`make clean`

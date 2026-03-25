# Ops & Infrastructure

本文档定义了 MineDock 项目的本地开发工作流、自动化构建与 CI/CD 规范。

## 1. 核心构建工具
项目根目录维护一个统一的 `Taskfile.yml`，屏蔽底层各语言组件的启动差异。
**当前支持的指令：**

| 指令 | 作用 | 说明 |
| --- | --- | --- |
| `task --list-all` | 查看可用任务 | 输出当前支持的任务列表 |
| `task dev` | 一键启动前后端开发服务 | 并行执行 `backend:dev` 与 `frontend:dev` |
| `task backend:fmt` | 检查后端 Go 代码格式 | 若存在未格式化文件则失败 |
| `task backend:vet` | 执行后端静态检查 | 在 `backend` 目录执行 `go vet ./...` |
| `task backend:test` | 执行后端测试 | 在 `backend` 目录执行 `go test ./...` |
| `task build` | 统一编译前后端 | 依赖 `backend:build` 与 `frontend:build` |
| `task clean` | 清理构建产物 | 清理 `backend/bin` 与 `frontend/dist` |
| `task frontend:install` | 安装前端依赖 | 在 `frontend` 目录执行 `npm install` |
| `task fmt` | 执行全局格式检查 | 当前依赖 `backend:fmt` |
| `task vet` | 执行全局静态检查 | 当前依赖 `backend:vet` |
| `task test` | 执行全局测试 | 当前依赖 `backend:test` |

## 2. 环境依赖约束

本地开发与构建至少需要以下工具：

| 组件 | 用途 |
| --- | --- |
| Go | 后端编译与运行 |
| Node.js + npm | 前端依赖安装、开发与构建 |
| Docker Engine | 后端容器生命周期能力依赖 |
| task | 统一任务入口 |

## 3. CI/CD 对接规范

项目已在仓库内新增 GitHub Actions 工作流：

- `.github/workflows/ci.yml`
- `.github/workflows/release.yml`

### 3.1 CI（代码自动化检查）

触发条件：`push`、`pull_request`

执行顺序：

1. 准备 Go 与 Node.js 环境
2. 安装 `task`
3. 执行 `task frontend:install`
4. 执行 `task fmt`
5. 执行 `task test`
6. 执行 `task vet`
7. 执行 `task build`

### 3.2 Release（跨平台编译发布）

触发条件：推送标签 `v*`（例如 `v0.1.0`）

执行顺序：

1. 准备 Go 与 Node.js 环境
2. 安装 `task`
3. 执行 `task frontend:install`
4. 执行 `task frontend:build`（仅构建一次）
5. 交叉编译后端二进制（`linux/amd64`、`windows/amd64`、`darwin/amd64`、`darwin/arm64`）
6. 打包发布产物（包含后端二进制、前端 `dist`、`Readme.md`）
7. 上传到 GitHub Release

## 4. 本地开发工作流

推荐执行顺序如下：

1. 安装前端依赖：`task frontend:install`
2. 启动开发环境：`task dev`
3. 发布前构建验证：`task build`
4. 收尾清理：`task clean`

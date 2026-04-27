# 运维规范

## 构建

- `task frontend:install`：安装前端依赖
- `task dev`：并行启动前后端开发服务
- `task build`：构建前后端产物
- `task clean`：清理构建产物
- `task fmt:check` / `task lint` / `task test` / `task vet`：质量检查

构建产物约定：

- 后端：`backend/bin/minedock-backend(.exe)`
- 前端：`frontend/dist`

环境变量约定：

- 后端：
  - `MINEDOCK_DB_PATH`（默认 `data/minedock.db`）
  - `MINEDOCK_GAMES_PATH`（默认 `games.json`）
  - `MINEDOCK_TEMPLATES_DIR`（默认 `templates`）
- 前端：
  - `VITE_API_BASE_URL`（默认 `/api`）

## CI/CD

CI（`.github/workflows/ci.yml`）规则：

- 触发条件：`push`、`pull_request`
- 运行环境：`ubuntu-latest`
- 校验顺序：
  1. 安装前端依赖
  2. `task fmt:check`
  3. `task lint`
  4. `task test`
  5. `task vet`
  6. `task build`

Release（`.github/workflows/release.yml`）规则：

- 触发条件：tag push（`v*`）
- 发布前检查：前端格式检查、前端 lint、前端构建
- 打包目标：
  - `linux/amd64` (`tar.gz`)
  - `windows/amd64` (`zip`)
  - `darwin/amd64` (`tar.gz`)
  - `darwin/arm64` (`tar.gz`)
- 归档内容：
  - 后端可执行文件
  - 前端 `dist`
  - `Readme.md`

### 命名规范

- 正式版本标签：`vX.Y.Z`
- 预发布标签：`vX.Y.Z-alpha.N` / `vX.Y.Z-beta.N` / `vX.Y.Z-rc.N`
- 不符合上述规则的 tag 禁止进入发布流程

### 必要检查

- 合并到主线前必须通过：`fmt:check`、`lint`、`test`、`vet`、`build`
- 发布前必须确保：
  - 前端已完成一次独立构建
  - Release 产物可在目标平台解压并运行

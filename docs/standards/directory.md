# 目录规范

```text
MineDock/
├── .github/                # 存放 CI/CD
├── backend/                # 后端 Go 服务
│   ├── data/               # 数据存储目录
│   ├── games.json          # 游戏目录索引（轻量展示信息）
│   ├── templates/          # 游戏模板目录（YAML，按游戏 ID 命名）
│   ├── internal/           # 内部私有代码
│   │   ├── api/            # 路由与 HTTP 处理层
│   │   ├── model/          # 领域数据模型定义
│   │   ├── service/        # 核心业务逻辑层
│   │   └── store/          # 数据持久化/存储交互层
│   ├── main.go             # 后端程序入口
│   └── go.mod              # Go 依赖配置
├── frontend/               # 前端 Vue 项目
│   ├── src/
│   │   ├── api/            # 统一管理后端接口定义与请求封装
│   │   ├── components/     # 全局复用组件
│   │   ├── composables/    # 复用的组合式API
│   │   ├── locales/        # 多语言 i18n 配置文件
│   │   ├── router/         # 路由配置与全局路由守卫
│   │   ├── stores/         # 全局状态管理
│   │   └── views/          # 路由级别的业务页面组件
│   ├── package.json        # Npm 依赖配置
│   └── vite.config.js      # Vite 构建配置
├── docs/                   # 文档
├── Taskfile.yml            # 构建配置
├── AGENTS.md               # 文档导航
└── Readme.md               # 项目介绍
```

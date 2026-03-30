# 执行计划目录说明

本目录用于存放计划文档
- `completed` 存放已经完成的计划
- `active` 存放当前正在进行的计划

## 命名规范

`YYYYMMDD-主题关键字.md`

示例：

- `20260329-instance-restart.md`
- `20260330-backup-snapshot.md`

## 内容格式

执行计划内容建议遵循以下标准结构模块：

````markdown
# [目标描述 / Goal Description]

提供问题的简要描述，相关背景上下文，以及此更改要实现的目标。

## 需要评审的内容 (User Review Required)

记录任何需要用户审查或反馈的内容，例如破坏性更改或重大且重要的设计决策。可以使用 GitHub 提示块（如 `> [!IMPORTANT]` 或 `> [!WARNING]`）高亮强调。

## 拟定更改 (Proposed Changes)

按组件名称（如具体的 package、功能区或依赖层）将将要修改的文件分组归类，并按逻辑顺序（例如先依赖后实现）展开描述。建议使用水平分割线来区分不同组件。

### [组件名称 / Component Name]

简述该组件的变化，并按具体文件拆分说明。
对于具体的变更文件，使用 `[NEW]` 和 `[DELETE]` 标签标明文件的新旧状态，例如：

#### [MODIFY] 修改的文件名.go (及文件相对路径)
#### [NEW] 新增的文件名.go
#### [DELETE] 删除的文件名.go

## 执行步骤 (Execution Steps)

将完整的计划拆分为可执行的、细化的步骤列表，使用复选框来追踪进度状态：
- `[ ]` 待办 (Uncompleted)
- `[/]` 进行中 (In Progress)
- `[x]` 已完成 (Completed)

推荐使用嵌套列表来拆分复杂的步骤，例如：
- [ ] 核心模型层设计
  - [ ] 定义 `User` 结构体
  - [ ] 编写对应的单元测试
- [ ] API 路由注册
- [/] 当前正在开发的功能
- [x] 已经处理完毕的事项

## 待确认的疑问 (Open Questions)

记录任何需要澄清的业务逻辑或者架构设计疑问，这些疑问通常会直接影响执行计划的方案。可以使用 GitHub 提示块强调。

## 验证计划 (Verification Plan)

简述你将如何验证上述更改达到了预期效果。

### 自动化测试 (Automated Tests)
- 将要执行的确切测试命令 (如 `go test ./...`)。

### 手动验证 (Manual Verification)
- 如果涉及无法自动化测试的部分（如 UI 调整），提供手动的步骤和期望结果的说明。
````

## TODO 索引生成

- 运行 `task docs:todo` 可从仓库注释中的 `TODO: 描述` 自动生成 `docs/exec-plans/TODO.md`。
- 生成文件用于集中追踪技术债与改进事项，建议在提交前执行一次。
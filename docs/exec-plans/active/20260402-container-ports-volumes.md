# 创建容器时支持端口映射与卷挂载

## 背景

当前 MineDock 的 `CreateInstance` 虽然已经从 YAML 模板中加载了 `ports` 和 `volumes` 配置数据（数据结构 `PortMapping` / `VolumeMount` 已就绪），但在实际调用 Docker SDK `ContainerCreate` 时仅传入了 `Image`、`Env`、`Labels`（参见 `docker_service.go:81-90`），**端口映射和卷挂载均未生效**。

这意味着：

- 创建的容器没有暴露端口，宿主机无法访问游戏服务器
- 容器数据保存在匿名层，删除容器后世界存档等数据丢失
- 同时 `Cmd` 字段硬编码为 `["sleep", "3600"]`（开发阶段占位），真正的游戏服务器镜像有自己的 `ENTRYPOINT`，不需要覆盖 `Cmd`

本计划将模板中已定义的 `ports` 和 `volumes` 配置实际传入 Docker SDK，使创建的容器真正具备网络可达和数据持久化能力。

## 需要评审的内容

> [!IMPORTANT]
> **卷命名策略**
>
> 模板 YAML 中 `volumes[].name` 定义的是语义标识（如 `server-data`），需要转换为 Docker 卷名。
> 采用的命名规则为：`minedock-{instanceName}-{volumeName}`。
>
> 例如：实例名 `my-server`，模板卷名 `server-data` → Docker 卷名 `minedock-my-server-server-data`。
>
> 这确保每个实例拥有独立卷，卷名可读且可追溯到实例。

> [!IMPORTANT]
> **移除硬编码 Cmd**
>
> 当前 `ContainerCreate` 传入 `Cmd: []string{"sleep", "3600"}`，这会覆盖镜像自身的 `ENTRYPOINT` / `CMD`，导致游戏服务器无法正常启动。本计划将移除该硬编码，让容器使用镜像默认的启动命令。
>
> 如果模板中定义了 `container.command`，则使用模板的命令覆盖；否则不传 `Cmd`。

> [!WARNING]
> **端口冲突**
>
> 当宿主机端口已被占用时，Docker 会返回创建/启动错误。当前版本不做端口冲突的预检查，依赖 Docker 的原生错误返回。后续可考虑增加端口可用性预检和用户自定义端口覆盖。

## 拟定更改

### 后端 Service 层

#### [MODIFY] docker_service.go (`backend/internal/service/docker_service.go`)

这是本次变更的核心文件。需要修改 `CreateInstance` 方法，将模板中的 `ports` 和 `volumes` 转换为 Docker SDK 的数据结构并传入 `ContainerCreate`。

##### 1. 端口映射

将模板 `[]PortMapping` 转换为 Docker SDK 所需的两个结构：

- `container.Config.ExposedPorts`（`nat.PortSet`）— 声明容器暴露的端口
- `container.HostConfig.PortBindings`（`nat.PortMap`）— 映射宿主机端口

```go
// buildPortBindings 将模板端口映射转换为 Docker 端口配置。
func buildPortBindings(ports []model.PortMapping) (nat.PortSet, nat.PortMap) {
    exposedPorts := nat.PortSet{}
    portBindings := nat.PortMap{}

    for _, p := range ports {
        protocol := strings.ToLower(strings.TrimSpace(p.Protocol))
        if protocol == "" {
            protocol = "tcp"
        }
        containerPort, _ := nat.NewPort(protocol, strconv.Itoa(p.Container))
        exposedPorts[containerPort] = struct{}{}
        portBindings[containerPort] = []nat.PortBinding{
            {HostPort: strconv.Itoa(p.Host)},
        }
    }

    return exposedPorts, portBindings
}
```

##### 2. 卷挂载

将模板 `[]VolumeMount` 转换为 Docker SDK 所需的结构：

- `container.HostConfig.Binds`（`[]string`）— 格式为 `volumeName:containerPath[:ro]`

卷名使用 `minedock-{instanceName}-{volumeName}` 格式，确保每个实例的卷相互隔离。

```go
// buildVolumeBinds 将模板卷配置转换为 Docker Binds 列表。
// 卷名格式：minedock-{instanceName}-{volumeName}
func buildVolumeBinds(instanceName string, volumes []model.VolumeMount) []string {
    if len(volumes) == 0 {
        return nil
    }

    binds := make([]string, 0, len(volumes))
    for _, v := range volumes {
        dockerVolName := fmt.Sprintf("minedock-%s-%s", instanceName, v.Name)
        bind := fmt.Sprintf("%s:%s", dockerVolName, v.ContainerPath)
        if v.ReadOnly {
            bind += ":ro"
        }
        binds = append(binds, bind)
    }
    return binds
}
```

##### 3. 修改 `CreateInstance` 方法

```go
// 变更前
resp, err := s.cli.ContainerCreate(ctx, &container.Config{
    Image: imageRef,
    Cmd:   []string{"sleep", "3600"},
    Env:   mapToDockerEnv(env),
    Labels: map[string]string{...},
}, nil, nil, nil, "")

// 变更后
exposedPorts, portBindings := buildPortBindings(tpl.Container.Ports)

var cmd []string
if len(tpl.Container.Command) > 0 {
    cmd = tpl.Container.Command
}

resp, err := s.cli.ContainerCreate(ctx, &container.Config{
    Image:        imageRef,
    Cmd:          cmd,
    Env:          mapToDockerEnv(env),
    ExposedPorts: exposedPorts,
    Labels: map[string]string{...},
}, &container.HostConfig{
    PortBindings: portBindings,
    Binds:        buildVolumeBinds(name, tpl.Container.Volumes),
}, nil, nil, "")
```

##### 4. 新增 import

```go
import (
    "github.com/docker/go-connections/nat"
)
```

> [!NOTE]
> `github.com/docker/go-connections/nat` 是 Docker SDK 的已有传递依赖，无需额外 `go get`。可通过 `go list -m all | grep go-connections` 确认。

---

### 后端 Service 层测试

#### [MODIFY] docker_service_test.go（如存在）或新增端口/卷相关测试

新增以下单元测试覆盖新增的辅助函数：

- `TestBuildPortBindings`：
  - 空端口列表 → 返回空 PortSet/PortMap
  - TCP 端口映射 → 验证 ExposedPorts 键和 PortBindings 映射正确
  - UDP 端口映射 → 验证 protocol 正确拼接
  - 协议为空时默认 tcp
- `TestBuildVolumeBinds`：
  - 空卷列表 → 返回 nil
  - 正常卷 → 验证格式 `minedock-{name}-{volName}:/path`
  - 只读卷 → 验证 `:ro` 后缀
  - 实例名含特殊字符时的卷名安全性

---

### 文档

#### [MODIFY] contracts.md (`docs/api/contracts.md`)

无 API 签名变更。但 `POST /api/instances` 的行为语义增强：创建的容器现在会应用模板中定义的端口映射和卷挂载。可在说明中补充端口冲突时的 `500` 错误行为。

#### [MODIFY] instance_lifecycle.md (`docs/design-docs/instance_lifecycle.md`)

在创建流程部分补充：

- 端口映射来源：模板 `container.ports`
- 卷命名规则：`minedock-{instanceName}-{volumeName}`
- 删除实例时 Docker 卷**不会自动清理**（需用户手动或后续计划实现卷清理功能）

---

## 执行步骤

- [ ] 确认依赖
  - [ ] 运行 `go list -m all | grep go-connections` 确认 `nat` 包已在依赖树中
- [ ] 后端 Service 层
  - [ ] 新增 `buildPortBindings` 辅助函数，将 `[]PortMapping` → `nat.PortSet` + `nat.PortMap`
  - [ ] 新增 `buildVolumeBinds` 辅助函数，将 `[]VolumeMount` → `[]string`（Docker Binds 格式）
  - [ ] 修改 `CreateInstance`：
    - [ ] 移除 `Cmd: []string{"sleep", "3600"}` 硬编码
    - [ ] 传入 `ExposedPorts` 到 `container.Config`
    - [ ] 构建 `container.HostConfig`，传入 `PortBindings` 和 `Binds`
    - [ ] 支持模板 `container.command` 覆盖（有值时传入 `Cmd`）
  - [ ] 添加 `nat` 包的 import
- [ ] 后端测试
  - [ ] 编写 `TestBuildPortBindings`（空/TCP/UDP/默认协议）
  - [ ] 编写 `TestBuildVolumeBinds`（空/正常/只读）
  - [ ] 运行 `task backend:test` 确认全部通过
  - [ ] 运行 `task backend:vet && task backend:lint`
- [ ] 文档更新
  - [ ] 更新 `docs/design-docs/instance_lifecycle.md`：补充端口映射和卷挂载说明
  - [ ] 更新 `docs/api/contracts.md`：补充端口冲突错误说明（可选）

## 已确认的决策

- ✅ **删除实例时不清理卷**：本期 `DeleteInstance` 不处理关联 Docker Volume 的清理，卷数据在删除实例后保留。用户可通过 `docker volume rm` 手动回收。后续计划提供"删除实例及数据"选项。
- ✅ **端口冲突直接透传 Docker 错误**：当宿主机端口被占用时，Docker 返回的错误直接传递给前端。端口可用性预检查和用户自定义端口覆盖留作后续增强。

## 验证计划

### 自动化测试

- `task backend:test` — 覆盖：
  - `buildPortBindings`：空列表、TCP 映射、UDP 映射、默认协议
  - `buildVolumeBinds`：空列表、正常挂载、只读挂载
  - 现有 `CreateInstance` 相关测试（确保不被破坏）
- `task backend:vet && task backend:lint`

### 手动验证

- 启动后端，创建一个 Minecraft Java 实例
- 验证容器端口映射：`docker inspect <container_id>` 查看 `PortBindings` 包含 `25565/tcp -> 25565`
- 验证卷挂载：`docker inspect <container_id>` 查看 `Binds` 包含 `minedock-<name>-server-data:/data`
- 验证 `docker volume ls` 中出现 `minedock-<name>-server-data` 卷
- 启动容器后，验证 `docker port <container_id>` 输出正确端口映射
- 停止并删除实例后，验证 Docker 卷仍然存在（本期不清理卷）
- 创建另一个实例使用相同端口，验证 Docker 返回端口冲突错误并正确传递给前端

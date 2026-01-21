package api

import (
	"MineDock/model"
	"bufio"
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"

	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/image"
	"github.com/docker/docker/client"
	"github.com/docker/go-connections/nat"
	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
)

type Handler struct {
	Cli *client.Client
}

func NewHandler(c *client.Client) *Handler {
	return &Handler{Cli: c}
}

// 获取列表
func (h *Handler) GetContainers(c *gin.Context) {
	containers, err := h.Cli.ContainerList(context.Background(), container.ListOptions{All: true})
	if err != nil {
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}

	var viewList []model.ContainerView
	for _, ctn := range containers {
		name := "未知"
		if len(ctn.Names) > 0 {
			name = ctn.Names[0][1:]
		}
		viewList = append(viewList, model.ContainerView{
			ID:     ctn.ID[:10],
			Name:   name,
			Image:  ctn.Image,
			State:  ctn.State,
			Status: ctn.Status,
		})
	}
	c.JSON(200, viewList)
}

// 启动容器
func (h *Handler) StartContainer(c *gin.Context) {
	id := c.Param("id")
	if err := h.Cli.ContainerStart(context.Background(), id, container.StartOptions{}); err != nil {
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}
	c.JSON(200, gin.H{"message": "✅ 容器已启动！"})
}

// 停止容器
func (h *Handler) StopContainer(c *gin.Context) {
	id := c.Param("id")
	if err := h.Cli.ContainerStop(context.Background(), id, container.StopOptions{}); err != nil {
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}
	c.JSON(200, gin.H{"message": "🛑 容器已停止！"})
}

// WebSocket 升级器配置
var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool { return true },
}

// 实时日志
func (h *Handler) StreamLogs(c *gin.Context) {
	id := c.Param("id")
	ws, err := upgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		return
	}
	defer ws.Close()

	reader, err := h.Cli.ContainerLogs(context.Background(), id, container.LogsOptions{
		ShowStdout: true, ShowStderr: true, Follow: true, Tail: "50",
	})
	if err != nil {
		ws.WriteMessage(websocket.TextMessage, []byte(`{"type":"error","content":"无法获取日志"}`))
		return
	}
	defer reader.Close()

	scanner := bufio.NewScanner(reader)
	for scanner.Scan() {
		payload := scanner.Bytes()
		if len(payload) > 8 {
			streamType := payload[0]
			line := string(payload[8:])
			msgType := "info"
			if streamType == 2 {
				msgType = "error"
			}
			// 简单的 JSON 清洗逻辑可以直接写这，或者封装成私有函数
			cleanLine := strings.ReplaceAll(line, "\\", "\\\\")
			cleanLine = strings.ReplaceAll(cleanLine, "\"", "\\\"")
			cleanLine = strings.ReplaceAll(cleanLine, "\r", "")
			cleanLine = strings.ReplaceAll(cleanLine, "\n", "")

			jsonMsg := fmt.Sprintf(`{"type": "%s", "content": "%s"}`, msgType, cleanLine)
			if err := ws.WriteMessage(websocket.TextMessage, []byte(jsonMsg)); err != nil {
				break
			}
		}
	}
}

// CreateContainer 创建并启动一个新服务器
func (h *Handler) CreateContainer(c *gin.Context) {
	var req model.CreateRequest
	// 1. 解析前端发来的 JSON
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(400, gin.H{"error": "参数格式不对: " + err.Error()})
		return
	}

	// 默认值处理（防止前端没传炸掉）
	if req.Image == "" {
		req.Image = "itzg/minecraft-server"
	}
	reader, err := h.Cli.ImagePull(context.Background(), req.Image, image.PullOptions{})
	if err != nil {
		// 如果拉取失败（比如没网，或者镜像名写错）
		c.JSON(500, gin.H{"error": "拉取镜像失败: " + err.Error()})
		return
	}
	io.Copy(os.Stdout, reader)
	reader.Close()

	envList := []string{
		"EULA=TRUE",
		"UID=1000",
		"GID=1000",
	}

	for key, value := range req.Env {
		envList = append(envList, key+"="+value)
	}

	// 2. 配置容器环境 (Config)
	config := &container.Config{
		Image:     req.Image,
		Tty:       true,
		OpenStdin: true,
		Env:       envList, // 把拼好的列表塞进去
	}

	// 3. 配置宿主机挂载 (HostConfig)
	// 3.1 端口映射: 把宿主机的 req.Port 映射到容器的 25565
	hostBinding := nat.PortBinding{
		HostIP:   "0.0.0.0",
		HostPort: req.Port,
	}
	containerPort, _ := nat.NewPort("tcp", "25565")
	portBinding := nat.PortMap{containerPort: []nat.PortBinding{hostBinding}}

	// 3.2 目录挂载: 把你电脑上的 DataPath 挂载到容器里的 /data
	// 如果 DataPath 为空，Docker 会自动创建一个匿名卷（不推荐）
	binds := []string{}
	if req.DataPath != "" {
		binds = append(binds, req.DataPath+":/data")
	}

	hostConfig := &container.HostConfig{
		PortBindings: portBinding,
		Binds:        binds,
		Resources:    container.Resources{
			// 这里其实可以限制 CPU，暂时先不做
		},
		RestartPolicy: container.RestartPolicy{Name: "unless-stopped"}, // 除非手动停，否则崩了自动重启
	}

	// 4. 调用 Docker API 创建容器
	resp, err := h.Cli.ContainerCreate(context.Background(), config, hostConfig, nil, nil, req.Name)
	if err != nil {
		c.JSON(500, gin.H{"error": "创建失败: " + err.Error()})
		return
	}

	// 5. 顺手把它启动了
	if err := h.Cli.ContainerStart(context.Background(), resp.ID, container.StartOptions{}); err != nil {
		c.JSON(500, gin.H{"error": "创建成功但启动失败: " + err.Error()})
		return
	}

	c.JSON(200, gin.H{"message": "✅ 服务器创建并启动成功！", "id": resp.ID})
}

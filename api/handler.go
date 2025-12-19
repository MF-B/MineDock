package api

import (
	"MineDock/model"
	"bufio"
	"context"
	"fmt"
	"net/http"
	"strings"

	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/client"
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

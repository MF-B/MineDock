package main

import (
	"context"
	"fmt"
	"net/http"
	"strings"

	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/client"
	"github.com/gin-gonic/gin"

	"bufio"

	"github.com/gorilla/websocket"
)

func main() {
	var upgrader = websocket.Upgrader{
		// 允许跨域（为了方便开发，生产环境通常要限制）
		CheckOrigin: func(r *http.Request) bool {
			return true
		},
	}

	// 1. 初始化 Docker 客户端
	cli, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	if err != nil {
		panic(err)
	}
	defer cli.Close()

	// 2. 初始化 Web 服务器 (Gin)
	r := gin.Default()

	// 定义一个简单的结构体，只返回前端需要的数据，保持清爽
	type ContainerView struct {
		ID     string `json:"id"`
		Name   string `json:"names"` // 容器通常有多个名字，我们取第一个
		Image  string `json:"image"`
		State  string `json:"state"`  // running, exited...
		Status string `json:"status"` // "Up 2 hours", "Exited (0) 5 seconds ago"
	}

	// 获取列表接口
	r.GET("/containers", func(c *gin.Context) {
		// ListOptions{All: true} 表示列出所有容器，包括停止运行的
		containers, err := cli.ContainerList(context.Background(), container.ListOptions{All: true})
		if err != nil {
			c.JSON(500, gin.H{"error": err.Error()})
			return
		}

		// 把 Docker 的原始数据转换成我们定义的简单结构体
		var viewList []ContainerView
		for _, ctn := range containers {
			name := "未知"
			if len(ctn.Names) > 0 {
				// Docker 的名字通常以 "/" 开头，去掉它才好看
				name = ctn.Names[0][1:]
			}

			viewList = append(viewList, ContainerView{
				ID:     ctn.ID[:10], // ID 截取前10位就够了
				Name:   name,
				Image:  ctn.Image,
				State:  ctn.State,
				Status: ctn.Status,
			})
		}

		c.JSON(200, viewList)
	})

	// 启动容器
	r.POST("/containers/:id/start", func(c *gin.Context) {
		id := c.Param("id")

		if err := cli.ContainerStart(context.Background(), id, container.StartOptions{}); err != nil {
			c.JSON(500, gin.H{"error": "启动失败: " + err.Error()})
			return
		}
		c.JSON(200, gin.H{"message": "✅ 容器已启动！"})
	})

	// 停止容器
	r.POST("/containers/:id/stop", func(c *gin.Context) {
		id := c.Param("id")

		if err := cli.ContainerStop(context.Background(), id, container.StopOptions{}); err != nil {
			c.JSON(500, gin.H{"error": "停止失败: " + err.Error()})
			return
		}
		c.JSON(200, gin.H{"message": "🛑 容器已停止！"})
	})

	// 实时日志接口 (WebSocket)
	r.GET("/containers/:id/logs", func(c *gin.Context) {
		id := c.Param("id")

		// 1. 升级连接：从 HTTP 变成 WebSocket
		ws, err := upgrader.Upgrade(c.Writer, c.Request, nil)
		if err != nil {
			return
		}
		defer ws.Close() // 结束时记得挂电话

		// 2. 调用 Docker 获取日志流
		// Follow: true 表示持续监听，ShowStdout/Stderr 表示标准输出和错误都要
		reader, err := cli.ContainerLogs(context.Background(), id, container.LogsOptions{
			ShowStdout: true,
			ShowStderr: true,
			Follow:     true,
			Tail:       "50", // 刚打开时先看最后 50 行
		})
		if err != nil {
			ws.WriteMessage(websocket.TextMessage, []byte("无法获取日志: "+err.Error()))
			return
		}
		defer reader.Close()

		// 3. 搬运工：不断从 Docker 读一行，往 WebSocket 写一行
		scanner := bufio.NewScanner(reader)
		for scanner.Scan() {
			payload := scanner.Bytes()

			// Docker 日志头有 8 个字节，只有长于 8 字节的才是有效内容
			if len(payload) > 8 {
				// 第 1 个字节是类型：1=stdout(正常), 2=stderr(错误)
				streamType := payload[0]

				// 切掉前 8 个字节的头，剩下的才是真正的日志文本
				line := string(payload[8:])

				// 我们构造一个简单的 JSON 发给前端，带上颜色信息
				// 1=绿色/白色，2=红色
				msgType := "info"
				if streamType == 2 {
					msgType = "error"
				}

				// 这里偷个懒，直接拼 JSON 字符串（或者你可以定义结构体用 json.Marshal）
				// 注意：如果日志里有引号可能需要转义，但作为简单控制台先这样跑
				jsonMsg := fmt.Sprintf(`{"type": "%s", "content": "%s"}`, msgType, cleanJsonString(line))

				err := ws.WriteMessage(websocket.TextMessage, []byte(jsonMsg))
				if err != nil {
					break
				}
			}
		}
	})

	// 4. 启动服务器，监听 8080 端口
	r.StaticFile("/", "./static/index.html")
	r.Run(":8080")
}

// 简单的字符串清洗，防止 JSON 格式错误
func cleanJsonString(s string) string {
	// 把双引号转义，把换行符去掉
	s = strings.ReplaceAll(s, "\\", "\\\\")
	s = strings.ReplaceAll(s, "\"", "\\\"")
	s = strings.ReplaceAll(s, "\r", "")
	s = strings.ReplaceAll(s, "\n", "")
	return s
}

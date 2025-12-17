package main

import (
	"context"

	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/client"
	"github.com/gin-gonic/gin"
)

func main() {
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

	// 4. 启动服务器，监听 8080 端口
	r.StaticFile("/", "./static/index.html")
	r.Run(":8080")
}

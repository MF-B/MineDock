package main

import (
	"context"
	"net/http"

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

	// 3. 定义一个接口：GET /containers
	r.GET("/containers", func(c *gin.Context) {
		// 获取容器列表
		containers, err := cli.ContainerList(context.Background(), container.ListOptions{All: true})
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}

		// 把复杂的 Docker 数据简化一下，只返回我们关心的
		var simpleList []gin.H
		for _, ctr := range containers {
			name := "无名氏"
			if len(ctr.Names) > 0 {
				name = ctr.Names[0][1:] // 去掉开头的 /
			}
			simpleList = append(simpleList, gin.H{
				"id":     ctr.ID[:10],
				"name":   name,
				"status": ctr.State, // running 或 exited
				"image":  ctr.Image,
			})
		}

		// 返回 JSON 数据给浏览器
		c.JSON(http.StatusOK, gin.H{
			"code": 200,
			"data": simpleList,
		})
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
	r.Run(":8080")
}

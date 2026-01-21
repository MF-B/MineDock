package main

import (
	"MineDock/api"
	"MineDock/core"

	"github.com/gin-gonic/gin"
)

func main() {
	// 初始化Core Docker 客户端
	cli, err := core.InitDockerClient()
	if err != nil {
		panic("Docker 连接失败: " + err.Error())
	}
	defer cli.Close()

	// 初始化API Handler
	handler := api.NewHandler(cli)

	// 设置路由Gin
	r := gin.Default()

	// 静态资源
	r.StaticFile("/", "./static/index.html")
	r.Static("/js", "./static/js")
	r.Static("/css", "./static/css")

	// 注册接口
	r.GET("/containers", handler.GetContainers)
	r.POST("/containers/:id/start", handler.StartContainer)
	r.POST("/containers/:id/stop", handler.StopContainer)
	r.GET("/containers/:id/logs", handler.StreamLogs)
	r.POST("/containers/create", handler.CreateContainer)

	// 启动
	r.Run(":8080")
}

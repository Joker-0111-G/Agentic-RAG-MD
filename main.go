package main

import (
	"fmt"

	"Agentic-RAG-MD/global"
	"Agentic-RAG-MD/initialize"
	"Agentic-RAG-MD/router"
)

func main() {
	//初始化配置与数据库
	initialize.InitApp()

	//Gin
	r := router.SetupRouter()

	//run
	port := fmt.Sprintf(":%d", global.Config.Server.Port)
	fmt.Printf(" Agentic RAG 知识库服务启动，监听端口 %s\n", port)
	
	if err := r.Run(port); err != nil {
		panic(fmt.Sprintf("服务启动失败: %v", err))
	}
}
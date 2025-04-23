package main

import (
	"hacker-news/config"
	"hacker-news/internal/api"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/robfig/cron/v3"
)

func main() {
	// 设置日志格式
	log.SetFlags(log.LstdFlags | log.Lshortfile)
	log.Println("启动 Hacker News 服务")

	// 加载配置
	cfg := config.LoadConfig()

	// 创建API服务器
	server, err := api.NewServer(cfg)
	if err != nil {
		log.Fatalf("创建服务器失败: %v", err)
	}

	// 创建定时任务
	c := cron.New(cron.WithSeconds())

	// 每天凌晨1点执行
	_, err = c.AddFunc("0 0 1 * * *", func() {
		// 获取当前日期
		date := time.Now().Format("2006-01-02")
		log.Printf("定时任务触发：处理 %s 的 Hacker News", date)

		// 发送HTTP请求到自己的API端点
		// 这里使用内部函数直接处理，避免额外的HTTP请求
		go server.ProcessHackerNews(date, cfg.HackerNews.MaxItems)
	})

	if err != nil {
		log.Printf("添加定时任务失败: %v", err)
	} else {
		c.Start()
		defer c.Stop()
		log.Println("定时任务已启动")
	}

	// 创建通道接收系统信号
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

	// 启动服务器（非阻塞）
	go func() {
		log.Printf("服务器正在监听端口 %s", cfg.Server.Port)
		if err := server.Run(); err != nil {
			log.Fatalf("服务器运行失败: %v", err)
		}
	}()

	// 等待退出信号
	<-quit
	log.Println("收到退出信号，正在关闭服务")
}

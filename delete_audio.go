package main

import (
	"context"
	"hacker-news/config"
	"hacker-news/internal/storage"
	"log"
	"time"
)

func main() {
	// 设置日志格式
	log.SetFlags(log.LstdFlags | log.Lshortfile)
	log.Println("开始删除音频文件")

	// 加载配置
	cfg := config.LoadConfig()

	// 创建MinIO客户端
	minioClient, err := storage.NewMinioClient(&cfg.MinIO)
	if err != nil {
		log.Fatalf("创建MinIO客户端失败: %v", err)
	}

	// 要删除的文件名
	objectName := "audio/hacker-news-2025-04-23.mp3"

	// 创建上下文
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// 检查文件是否存在
	exists, err := minioClient.ObjectExists(ctx, objectName)
	if err != nil {
		log.Fatalf("检查文件是否存在失败: %v", err)
	}

	if !exists {
		log.Printf("文件 %s 不存在", objectName)
		return
	}

	// 删除文件
	err = minioClient.DeleteFile(ctx, objectName)
	if err != nil {
		log.Fatalf("删除文件失败: %v", err)
	}

	log.Printf("文件 %s 已成功删除", objectName)
}

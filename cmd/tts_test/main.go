package main

import (
	"context"
	"fmt"
	"hacker-news/internal/tts"
	"log"
	"os"
	"time"

	"hacker-news/config"
)

func main() {
	// 加载配置
	cfg, err := config.LoadConfig()
	if err != nil {
		log.Fatalf("加载配置失败: %v", err)
	}

	// 创建Edge TTS服务
	edgeTTS := &tts.EdgeTTS{
		OutputFormat: "audio-16khz-32kbitrate-mono-mp3",
	}

	// 测试文本
	text := "这是一段测试文本，用于验证Edge TTS服务是否正常工作。"
	
	// 使用男声和女声各测试一次
	testSpeakers := []string{"男", "女"}
	
	log.Println("开始测试Edge TTS...")
	
	for _, speaker := range testSpeakers {
		log.Printf("测试 %s 声音...", speaker)
		
		// 设置超时上下文
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		
		// 调用TTS服务
		startTime := time.Now()
		audio, err := edgeTTS.SynthesizeSpeech(ctx, text, speaker)
		duration := time.Since(startTime)
		
		if err != nil {
			log.Printf("❌ %s声转换失败: %v", speaker, err)
			continue
		}
		
		// 保存音频文件
		filename := fmt.Sprintf("test_%s.mp3", speaker)
		err = os.WriteFile(filename, audio, 0644)
		if err != nil {
			log.Printf("❌ 保存音频文件失败: %v", err)
			continue
		}
		
		// 输出结果
		fileInfo, _ := os.Stat(filename)
		log.Printf("✅ %s声转换成功! 文件: %s, 大小: %d 字节, 耗时: %v", 
			speaker, filename, fileInfo.Size(), duration)
	}
	
	// 显示测试信息
	log.Println("测试完成！请检查当前目录下的test_男.mp3和test_女.mp3文件")
	absPath, _ := os.Getwd()
	log.Printf("当前工作目录: %s", absPath)
}

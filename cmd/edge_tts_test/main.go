package main

import (
	"context"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"strings"
	"time"
)

// 直接进行TTS测试，不依赖项目的其他部分
func main() {
	log.Println("开始测试Edge TTS...")

	// 测试文本和发声人
	text := "这是一段测试文本，用于验证Edge TTS服务是否正常工作。"
	voiceID := "zh-CN-YunxiNeural" // 男声

	// 调用TTS接口
	audio, err := directEdgeTTS(text, voiceID)
	if err != nil {
		log.Fatalf("❌ TTS转换失败: %v", err)
	}

	// 保存音频文件
	filename := "edge_tts_test.mp3"
	err = os.WriteFile(filename, audio, 0644)
	if err != nil {
		log.Fatalf("❌ 保存音频文件失败: %v", err)
	}

	// 输出结果
	fileInfo, _ := os.Stat(filename)
	log.Printf("✅ TTS转换成功! 文件: %s, 大小: %d 字节", filename, fileInfo.Size())
	absPath, _ := os.Getwd()
	log.Printf("音频文件保存在: %s\\%s", absPath, filename)
}

// 直接调用Edge TTS API
func directEdgeTTS(text string, voiceID string) ([]byte, error) {
	log.Printf("开始调用Edge TTS API，语音ID: %s", voiceID)

	// 设置上下文和超时
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// 使用微软官方Edge TTS服务
	baseURL := "https://speech.platform.bing.com/consumer/speech/synthesize/readaloud/edge/v1"

	// 构建SSML文本
	ssml := fmt.Sprintf(`
<speak version="1.0" xmlns="http://www.w3.org/2001/10/synthesis" xml:lang="zh-CN">
	<voice name="%s">
		<prosody rate="0%%" pitch="0%%">%s</prosody>
	</voice>
</speak>`, voiceID, escapeXML(text))

	// 创建HTTP请求
	req, err := http.NewRequestWithContext(ctx, "POST", baseURL, strings.NewReader(ssml))
	if err != nil {
		return nil, fmt.Errorf("创建请求失败: %w", err)
	}

	// 设置请求头
	req.Header.Set("Content-Type", "application/ssml+xml")
	req.Header.Set("X-Microsoft-OutputFormat", "audio-16khz-32kbitrate-mono-mp3")
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/93.0.4577.63 Safari/537.36 Edg/93.0.961.47")
	req.Header.Set("Origin", "https://speech.platform.bing.com")

	// 发送请求
	client := &http.Client{Timeout: 30 * time.Second}
	log.Println("正在发送请求到Edge TTS服务...")
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("发送请求失败: %w", err)
	}
	defer resp.Body.Close()

	// 检查响应状态
	log.Printf("收到响应，状态码: %d", resp.StatusCode)
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("TTS请求失败，状态码: %d", resp.StatusCode)
	}

	// 读取响应内容
	audio, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("读取响应内容失败: %w", err)
	}

	log.Printf("TTS转换成功，音频大小: %d 字节", len(audio))
	return audio, nil
}

// escapeXML 转义XML特殊字符
func escapeXML(text string) string {
	text = strings.ReplaceAll(text, "&", "&amp;")
	text = strings.ReplaceAll(text, "<", "&lt;")
	text = strings.ReplaceAll(text, ">", "&gt;")
	text = strings.ReplaceAll(text, "'", "&apos;")
	text = strings.ReplaceAll(text, "\"", "&quot;")
	return text
}

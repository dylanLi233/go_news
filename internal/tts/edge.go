package tts

import (
	"context"
	"fmt"
	"hacker-news/config"
	"io"
	"log"
	"net/http"
	"strings"
	"time"
)

// EdgeTTS 实现Edge TTS服务
type EdgeTTS struct {
	outputFormat string
}

// NewEdgeTTS 创建一个新的Edge TTS服务
func NewEdgeTTS(cfg config.EdgeTTSConfig) (*EdgeTTS, error) {
	return &EdgeTTS{
		outputFormat: cfg.OutputFormat,
	}, nil
}

// SynthesizeSpeech 将文本转换为语音
func (e *EdgeTTS) SynthesizeSpeech(ctx context.Context, text string, speaker string) ([]byte, error) {
	// 根据角色获取语音ID
	voiceID, err := GetSpeakerVoice("edge", speaker)
	if err != nil {
		return nil, err
	}

	log.Printf("使用Edge TTS转换文本，语音ID: %s", voiceID)

	// 直接使用Edge TTS API
	return e.directEdgeTTS(ctx, text, voiceID)
}

// Provider 返回TTS提供商名称
func (e *EdgeTTS) Provider() string {
	return "edge"
}

// directEdgeTTS 直接使用Edge TTS API
func (e *EdgeTTS) directEdgeTTS(ctx context.Context, text string, voiceID string) ([]byte, error) {
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
	req.Header.Set("X-Microsoft-OutputFormat", e.outputFormat)
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/93.0.4577.63 Safari/537.36 Edg/93.0.961.47")
	req.Header.Set("Origin", "https://speech.platform.bing.com")

	// 发送请求
	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("发送请求失败: %w", err)
	}
	defer resp.Body.Close()

	// 检查响应状态
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
	text = strings.ReplaceAll(text, "\"", "&quot;")
	text = strings.ReplaceAll(text, "'", "&apos;")
	return text
}

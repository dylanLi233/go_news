package tts

import (
	"context"
	"encoding/json"
	"fmt"
	"hacker-news/config"
	"io"
	"log"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"time"
)

// EdgeTTS 实现Edge TTS服务
type EdgeTTS struct {
	outputFormat string
}

// VoiceSetting 语音设置
type VoiceSetting struct {
	VoiceID string  `json:"voice_id"`
	Speed   float64 `json:"speed"`
	Volume  float64 `json:"vol"`
	Pitch   float64 `json:"pitch"`
}

// AudioSetting 音频设置
type AudioSetting struct {
	Format     string `json:"format"`
	SampleRate int    `json:"sample_rate"`
	Bitrate    int    `json:"bitrate"`
}

// TTSRequest TTS请求参数
type TTSRequest struct {
	Model        string       `json:"model"`
	Text         string       `json:"text"`
	Stream       bool         `json:"stream"`
	GetSRT       bool         `json:"get_srt"`
	VoiceSetting VoiceSetting `json:"voice_setting"`
	AudioSetting AudioSetting `json:"audio_setting"`
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

	// 默认请求参数
	request := TTSRequest{
		Model:  "edge-tts",
		Text:   text,
		Stream: false,
		GetSRT: false,
		VoiceSetting: VoiceSetting{
			VoiceID: voiceID,
			Speed:   1.0,
			Volume:  1.0,
			Pitch:   0.0,
		},
		AudioSetting: AudioSetting{
			Format:     e.outputFormat,
			SampleRate: 48000,
			Bitrate:    128000,
		},
	}

	// 直接使用Edge TTS API
	return e.directEdgeTTS(ctx, request)
}

// SynthesizeSpeechWithOptions 将文本转换为语音（带高级选项）
func (e *EdgeTTS) SynthesizeSpeechWithOptions(ctx context.Context, request TTSRequest) ([]byte, []byte, error) {
	audio, srtBytes, err := e.directEdgeTTSWithSRT(ctx, request)
	if err != nil {
		return nil, nil, err
	}
	return audio, srtBytes, nil
}

// Provider 返回TTS提供商名称
func (e *EdgeTTS) Provider() string {
	return "edge"
}

// directEdgeTTS 直接使用Edge TTS API
func (e *EdgeTTS) directEdgeTTS(ctx context.Context, request TTSRequest) ([]byte, error) {
	audio, _, err := e.directEdgeTTSWithSRT(ctx, request)
	return audio, err
}

// directEdgeTTSWithSRT 直接使用Edge TTS API并生成SRT字幕
func (e *EdgeTTS) directEdgeTTSWithSRT(ctx context.Context, request TTSRequest) ([]byte, []byte, error) {
	// 使用微软官方Edge TTS服务
	baseURL := "https://speech.platform.bing.com/consumer/speech/synthesize/readaloud/edge/v1"

	// 构建SSML文本
	ssml := fmt.Sprintf(`
<speak version="1.0" xmlns="http://www.w3.org/2001/10/synthesis" xml:lang="zh-CN">
	<voice name="%s">
		<prosody rate="%d%%" pitch="%d%%">%s</prosody>
	</voice>
</speak>`, 
	request.VoiceSetting.VoiceID, 
	int((request.VoiceSetting.Speed-1.0)*100), 
	int(request.VoiceSetting.Pitch*100), 
	escapeXML(request.Text))

	// 创建HTTP请求
	req, err := http.NewRequestWithContext(ctx, "POST", baseURL, strings.NewReader(ssml))
	if err != nil {
		return nil, nil, fmt.Errorf("创建请求失败: %w", err)
	}

	// 设置请求头
	outputFormat := e.outputFormat
	if request.AudioSetting.Format != "" {
		outputFormat = request.AudioSetting.Format
	}
	req.Header.Set("Content-Type", "application/ssml+xml")
	req.Header.Set("X-Microsoft-OutputFormat", outputFormat)
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/93.0.4577.63 Safari/537.36 Edg/93.0.961.47")
	req.Header.Set("Origin", "https://speech.platform.bing.com")

	// 发送请求
	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, nil, fmt.Errorf("发送请求失败: %w", err)
	}
	defer resp.Body.Close()

	// 检查响应状态
	if resp.StatusCode != http.StatusOK {
		return nil, nil, fmt.Errorf("TTS请求失败，状态码: %d", resp.StatusCode)
	}

	// 读取响应内容
	audio, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, nil, fmt.Errorf("读取响应内容失败: %w", err)
	}

	log.Printf("TTS转换成功，音频大小: %d 字节", len(audio))
	
	// 如果需要生成SRT字幕
	var srtBytes []byte
	if request.GetSRT {
		srt, err := generateSRT(request.Text)
		if err != nil {
			log.Printf("生成SRT字幕失败: %v", err)
		} else {
			srtBytes = []byte(srt)
			log.Printf("SRT字幕生成成功，大小: %d 字节", len(srtBytes))
		}
	}
	
	return audio, srtBytes, nil
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

// generateSRT 生成SRT字幕
func generateSRT(text string) (string, error) {
	// 简单的字幕生成逻辑
	// 这里使用一个简单的算法：每个汉字约0.3秒，每个标点符号约0.5秒
	var srtBuilder strings.Builder
	
	// 分割文本为句子
	sentences := splitSentences(text)
	
	startTime := 0.0
	for i, sentence := range sentences {
		// 计算句子持续时间（秒）
		duration := calculateDuration(sentence)
		endTime := startTime + duration
		
		// 格式化时间
		startTimeStr := formatSRTTime(startTime)
		endTimeStr := formatSRTTime(endTime)
		
		// 写入SRT条目
		srtBuilder.WriteString(fmt.Sprintf("%d\n", i+1))
		srtBuilder.WriteString(fmt.Sprintf("%s --> %s\n", startTimeStr, endTimeStr))
		srtBuilder.WriteString(fmt.Sprintf("%s\n\n", sentence))
		
		startTime = endTime
	}
	
	return srtBuilder.String(), nil
}

// splitSentences 将文本分割为句子
func splitSentences(text string) []string {
	// 使用标点符号分割句子
	re := regexp.MustCompile(`[，。！？；,\.!?;]+`)
	parts := re.Split(text, -1)
	
	// 过滤空字符串
	var sentences []string
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part != "" {
			sentences = append(sentences, part)
		}
	}
	
	// 如果没有句子，则将整个文本作为一个句子
	if len(sentences) == 0 {
		return []string{text}
	}
	
	return sentences
}

// calculateDuration 计算文本的持续时间（秒）
func calculateDuration(text string) float64 {
	// 简单估算：每个汉字约0.3秒
	// 实际应用中可能需要更复杂的算法
	charCount := len([]rune(text))
	return float64(charCount) * 0.3
}

// formatSRTTime 格式化SRT时间
func formatSRTTime(seconds float64) string {
	hours := int(seconds) / 3600
	minutes := (int(seconds) % 3600) / 60
	secs := int(seconds) % 60
	milliseconds := int((seconds - float64(int(seconds))) * 1000)
	
	return fmt.Sprintf("%02d:%02d:%02d,%03d", hours, minutes, secs, milliseconds)
}

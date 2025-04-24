package tts

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"hacker-news/config"
	"io"
	"log"
	"net/http"
	"regexp"
	"strings"
	"time"
)

// EdgeTTS 实现Edge TTS服务
type EdgeTTS struct {
	outputFormat string
	apiURL       string
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

// TTSResponse API响应结构
type TTSResponse struct {
	Audio string `json:"audio"` // Base64编码的音频
	SRT   string `json:"srt"`   // SRT字幕
}

// NewEdgeTTS 创建一个新的Edge TTS服务
func NewEdgeTTS(cfg config.EdgeTTSConfig) (*EdgeTTS, error) {
	return &EdgeTTS{
		outputFormat: cfg.OutputFormat,
		apiURL:       cfg.APIURL, // 从配置中读取API URL
	}, nil
}

// SynthesizeSpeech 将文本转换为语音
func (e *EdgeTTS) SynthesizeSpeech(ctx context.Context, text string, speaker string) ([]byte, error) {
	// 根据角色获取语音ID
	processedText := ProcessDialogueText(text)

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

	// 使用API服务
	return e.callTTSAPI(ctx, request)
}

// SynthesizeSpeechWithOptions 将文本转换为语音（带高级选项）
func (e *EdgeTTS) SynthesizeSpeechWithOptions(ctx context.Context, request TTSRequest) ([]byte, []byte, error) {
	audio, srtBytes, err := e.callTTSAPIWithSRT(ctx, request)
	if err != nil {
		return nil, nil, err
	}
	return audio, srtBytes, nil
}

// Provider 返回TTS提供商名称
func (e *EdgeTTS) Provider() string {
	return "edge"
}

// callTTSAPI 调用TTS API
func (e *EdgeTTS) callTTSAPI(ctx context.Context, request TTSRequest) ([]byte, error) {
	audio, _, err := e.callTTSAPIWithSRT(ctx, request)
	return audio, err
}

// callTTSAPIWithSRT 调用TTS API并获取SRT字幕
func (e *EdgeTTS) callTTSAPIWithSRT(ctx context.Context, request TTSRequest) ([]byte, []byte, error) {
	// 确保请求中包含获取SRT的参数
	request.GetSRT = true
	
	// 准备请求JSON
	requestBody, err := json.Marshal(request)
	if err != nil {
		return nil, nil, fmt.Errorf("序列化请求失败: %w", err)
	}
	
	log.Printf("发送TTS请求到API: %s", e.apiURL)
	
	// 创建HTTP请求
	req, err := http.NewRequestWithContext(ctx, "POST", e.apiURL, strings.NewReader(string(requestBody)))
	if err != nil {
		return nil, nil, fmt.Errorf("创建请求失败: %w", err)
	}
	
	// 设置请求头
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	
	// 添加URL参数
	q := req.URL.Query()
	q.Add("group_id", "hacker-news")
	q.Add("return_srt", "true")
	req.URL.RawQuery = q.Encode()
	
	// 发送请求
	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, nil, fmt.Errorf("发送请求失败: %w", err)
	}
	defer resp.Body.Close()
	
	// 检查响应状态
	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		log.Printf("TTS API响应错误: %s", string(respBody))
		return nil, nil, fmt.Errorf("TTS请求失败，状态码: %d", resp.StatusCode)
	}
	
	// 读取响应内容
	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, nil, fmt.Errorf("读取响应内容失败: %w", err)
	}
	
	// 解析JSON响应
	var ttsResponse TTSResponse
	if err := json.Unmarshal(respBody, &ttsResponse); err != nil {
		return nil, nil, fmt.Errorf("解析响应失败: %w", err)
	}
	
	// 解码Base64音频
	audio, err := base64.StdEncoding.DecodeString(ttsResponse.Audio)
	if err != nil {
		return nil, nil, fmt.Errorf("解码音频失败: %w", err)
	}
	
	log.Printf("TTS转换成功，音频大小: %d 字节", len(audio))
	
	// 处理SRT字幕
	var srtBytes []byte
	if ttsResponse.SRT != "" {
		srtBytes = []byte(ttsResponse.SRT)
		log.Printf("SRT字幕获取成功，大小: %d 字节", len(srtBytes))
	} else if request.GetSRT {
		// 如果API没有返回SRT但请求中要求了SRT，则本地生成
		srt, err := generateSRT(request.Text)
		if err != nil {
			log.Printf("本地生成SRT字幕失败: %v", err)
		} else {
			srtBytes = []byte(srt)
			log.Printf("本地生成SRT字幕成功，大小: %d 字节", len(srtBytes))
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

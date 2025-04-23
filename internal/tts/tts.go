package tts

import (
	"context"
	"fmt"
	"hacker-news/config"
)

// Service 定义TTS服务接口
type Service interface {
	// SynthesizeSpeech 将文本转换为语音
	SynthesizeSpeech(ctx context.Context, text string, speaker string) ([]byte, error)
	
	// Provider 返回TTS提供商名称
	Provider() string
}

// Factory 创建TTS服务
func Factory(cfg *config.TTSConfig) (Service, error) {
	// 根据配置选择TTS服务
	switch cfg.Provider {
	case "edge":
		return NewEdgeTTS(cfg.EdgeTTS)
	case "aliyun":
		return NewAliyunTTS(cfg.AliyunTTS)
	default:
		// 默认使用Edge TTS
		return NewEdgeTTS(cfg.EdgeTTS)
	}
}

// GetSpeakerVoice 根据角色返回语音ID
func GetSpeakerVoice(provider string, speaker string) (string, error) {
	// 确保speaker是"男"或"女"
	if speaker != "男" && speaker != "女" {
		return "", fmt.Errorf("无效的角色，必须是'男'或'女'")
	}

	// 根据不同的TTS提供商返回不同的语音ID
	switch provider {
	case "edge":
		if speaker == "男" {
			return "zh-CN-YunxiNeural", nil
		}
		return "zh-CN-XiaoxiaoNeural", nil
	case "aliyun":
		if speaker == "男" {
			return "aixia", nil // 阿里云男声
		}
		return "xiaoyun", nil // 阿里云女声
	default:
		// 默认使用Edge TTS
		if speaker == "男" {
			return "zh-CN-YunxiNeural", nil
		}
		return "zh-CN-XiaoxiaoNeural", nil
	}
}

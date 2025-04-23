package config

import (
	"log"
	"os"
	"strconv"

	"github.com/joho/godotenv"
)

func init() {
	// 加载.env文件
	if err := godotenv.Load(); err != nil {
		log.Printf("警告: 无法加载.env文件: %v", err)
	}
}

// Config 应用配置
type Config struct {
	Server     ServerConfig
	OpenAI     OpenAIConfig
	MinIO      MinIOConfig
	HackerNews HackerNewsConfig
	TTS        TTSConfig
}

// ServerConfig 服务器配置
type ServerConfig struct {
	Port string
	Env  string
}

// OpenAIConfig OpenAI/Deepseek配置
type OpenAIConfig struct {
	BaseURL        string
	APIKey         string
	Model          string
	ThinkingModel  string
	MaxTokens      int
	DefaultAPIKey  string
	DefaultBaseURL string
}

// MinIOConfig MinIO存储配置
type MinIOConfig struct {
	Endpoint        string
	BucketName      string
	AccessKeyID     string
	SecretAccessKey string
}

// HackerNewsConfig Hacker News相关配置
type HackerNewsConfig struct {
	JinaKey  string
	MaxItems int
}

// TTSConfig 文本转语音配置
type TTSConfig struct {
	Provider  string // "edge", "aliyun", 等
	EdgeTTS   EdgeTTSConfig
	AliyunTTS AliyunTTSConfig
}

// EdgeTTSConfig Edge TTS配置
type EdgeTTSConfig struct {
	OutputFormat string
}

// AliyunTTSConfig 阿里云TTS配置
type AliyunTTSConfig struct {
	AccessKeyID     string
	AccessKeySecret string
	Region          string
	VoiceID         string
}

// LoadConfig 从环境变量加载配置
func LoadConfig() *Config {
	return &Config{
		Server: ServerConfig{
			Port: getEnvOrDefault("APP_PORT", "3001"),
			Env:  getEnvOrDefault("WORKER_ENV", "production"),
		},
		OpenAI: OpenAIConfig{
			BaseURL:        getEnvOrDefault("OPENAI_BASE_URL", "https://api.deepseek.com/v1"),
			APIKey:         getEnvOrDefault("OPENAI_API_KEY", ""),
			Model:          getEnvOrDefault("OPENAI_MODEL", "deepseek-chat"),
			ThinkingModel:  getEnvOrDefault("OPENAI_THINKING_MODEL", ""),
			MaxTokens:      getEnvIntOrDefault("OPENAI_MAX_TOKENS", 4096),
			DefaultAPIKey:  "sk-b5195ce322244754b2c87d901473070e", // 测试用，生产环境应使用环境变量
			DefaultBaseURL: "https://api.deepseek.com/v1",
		},
		MinIO: MinIOConfig{
			Endpoint:        getEnvOrDefault("HACKER_NEWS_R2_BUCKET_URL", "http://localhost:9000"),
			BucketName:      getEnvOrDefault("HACKER_NEWS_BUCKET_NAME", "hacker-news"),
			AccessKeyID:     getEnvOrDefault("MINIO_ACCESS_KEY", "minioadmin"),
			SecretAccessKey: getEnvOrDefault("MINIO_SECRET_KEY", "minioadmin"),
		},
		HackerNews: HackerNewsConfig{
			JinaKey:  getEnvOrDefault("JINA_KEY", ""),
			MaxItems: getEnvIntOrDefault("MAX_ITEMS", 10),
		},
		TTS: TTSConfig{
			Provider: getEnvOrDefault("TTS_PROVIDER", "edge"),
			EdgeTTS: EdgeTTSConfig{
				OutputFormat: getEnvOrDefault("EDGE_TTS_FORMAT", "mp3"),
			},
			AliyunTTS: AliyunTTSConfig{
				AccessKeyID:     getEnvOrDefault("ALIYUN_ACCESS_KEY_ID", ""),
				AccessKeySecret: getEnvOrDefault("ALIYUN_ACCESS_KEY_SECRET", ""),
				Region:          getEnvOrDefault("ALIYUN_REGION", "cn-shanghai"),
				VoiceID:         getEnvOrDefault("ALIYUN_VOICE_ID", "xiaoyun"),
			},
		},
	}
}

// getEnvOrDefault 获取环境变量或默认值
func getEnvOrDefault(key, defaultValue string) string {
	value := os.Getenv(key)
	if value == "" {
		return defaultValue
	}
	return value
}

// getEnvIntOrDefault 获取环境变量(整数)或默认值
func getEnvIntOrDefault(key string, defaultValue int) int {
	value := os.Getenv(key)
	if value == "" {
		return defaultValue
	}
	intValue, err := strconv.Atoi(value)
	if err != nil {
		return defaultValue
	}
	return intValue
}

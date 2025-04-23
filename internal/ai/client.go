package ai

import (
	"context"
	"fmt"
	"hacker-news/config"
	"log"
	"time"

	"github.com/sashabaranov/go-openai"
)

// Client 是AI接口的客户端
type Client struct {
	client    *openai.Client
	config    *config.OpenAIConfig
	maxTokens int
}

// NewClient 创建一个新的AI客户端
func NewClient(cfg *config.OpenAIConfig) *Client {
	// 使用提供的配置创建客户端
	apiKey := cfg.APIKey
	baseURL := cfg.BaseURL

	// 如果配置中没有提供API密钥，使用默认值
	if apiKey == "" {
		log.Println("警告: 未设置OPENAI_API_KEY环境变量，使用默认API密钥")
		apiKey = cfg.DefaultAPIKey
	}

	// 创建OpenAI配置
	clientConfig := openai.DefaultConfig(apiKey)
	clientConfig.BaseURL = baseURL

	// 创建客户端
	client := openai.NewClientWithConfig(clientConfig)

	return &Client{
		client:    client,
		config:    cfg,
		maxTokens: cfg.MaxTokens,
	}
}

// GenerateStoryText 生成单个文章的摘要
func (c *Client) GenerateStoryText(ctx context.Context, storyContent string) (string, error) {
	// 限制内容长度，防止超过token限制
	maxLength := c.maxTokens * 4
	if len(storyContent) > maxLength {
		storyContent = storyContent[:maxLength]
	}

	// 创建聊天请求
	req := openai.ChatCompletionRequest{
		Model: c.config.Model,
		Messages: []openai.ChatCompletionMessage{
			{
				Role:    openai.ChatMessageRoleSystem,
				Content: SummarizeStoryPrompt,
			},
			{
				Role:    openai.ChatMessageRoleUser,
				Content: storyContent,
			},
		},
		MaxTokens: c.maxTokens,
	}

	// 发送请求
	return c.generateText(ctx, req)
}

// GeneratePodcastContent 生成播客内容
func (c *Client) GeneratePodcastContent(ctx context.Context, summaries []string) (string, error) {
	// 合并所有摘要
	content := JoinContents(summaries)

	// 创建聊天请求
	req := openai.ChatCompletionRequest{
		Model: c.config.Model,
		Messages: []openai.ChatCompletionMessage{
			{
				Role:    openai.ChatMessageRoleSystem,
				Content: SummarizePodcastPrompt(),
			},
			{
				Role:    openai.ChatMessageRoleUser,
				Content: content,
			},
		},
		MaxTokens: c.maxTokens,
	}

	// 发送请求
	return c.generateText(ctx, req)
}

// GenerateBlogContent 生成博客内容
func (c *Client) GenerateBlogContent(ctx context.Context, summaries []string) (string, error) {
	// 合并所有摘要
	content := JoinContents(summaries)

	// 创建聊天请求
	req := openai.ChatCompletionRequest{
		Model: c.config.Model,
		Messages: []openai.ChatCompletionMessage{
			{
				Role:    openai.ChatMessageRoleSystem,
				Content: SummarizeBlogPrompt,
			},
			{
				Role:    openai.ChatMessageRoleUser,
				Content: content,
			},
		},
		MaxTokens: c.maxTokens,
	}

	// 发送请求
	return c.generateText(ctx, req)
}

// GenerateIntroContent 生成简介内容
func (c *Client) GenerateIntroContent(ctx context.Context, podcastContent string) (string, error) {
	// 创建聊天请求
	req := openai.ChatCompletionRequest{
		Model: c.config.Model,
		Messages: []openai.ChatCompletionMessage{
			{
				Role:    openai.ChatMessageRoleSystem,
				Content: IntroPrompt,
			},
			{
				Role:    openai.ChatMessageRoleUser,
				Content: podcastContent,
			},
		},
		MaxTokens: 300, // 简介较短，不需要太多token
	}

	// 发送请求
	return c.generateText(ctx, req)
}

// generateText 发送AI请求并获取生成的文本
func (c *Client) generateText(ctx context.Context, req openai.ChatCompletionRequest) (string, error) {
	log.Printf("生成AI内容，模型: %s", req.Model)

	// 添加重试逻辑
	maxRetries := 3
	for i := 0; i < maxRetries; i++ {
		// 添加超时
		timeoutCtx, cancel := context.WithTimeout(ctx, 2*time.Minute)
		defer cancel()

		// 发送请求
		resp, err := c.client.CreateChatCompletion(timeoutCtx, req)
		if err != nil {
			// 检查是否是可重试的错误
			if i < maxRetries-1 {
				log.Printf("AI请求失败，正在重试 (%d/%d): %v", i+1, maxRetries, err)
				time.Sleep(time.Duration(i+1) * 2 * time.Second) // 指数退避
				continue
			}
			return "", fmt.Errorf("生成AI内容失败: %w", err)
		}

		// 检查响应是否有效
		if len(resp.Choices) == 0 {
			if i < maxRetries-1 {
				log.Printf("AI响应无效，正在重试 (%d/%d)", i+1, maxRetries)
				time.Sleep(time.Duration(i+1) * 2 * time.Second)
				continue
			}
			return "", fmt.Errorf("AI响应中没有内容")
		}

		log.Printf("AI内容生成成功，使用tokens: %d", resp.Usage.TotalTokens)
		return resp.Choices[0].Message.Content, nil
	}

	return "", fmt.Errorf("超过最大重试次数")
}

// JoinContents 将多个内容合并为一个字符串，以分隔符分隔
func JoinContents(contents []string) string {
	if len(contents) == 0 {
		return ""
	}

	separator := "\n\n---\n\n"
	result := contents[0]
	for i := 1; i < len(contents); i++ {
		result += separator + contents[i]
	}

	return result
}

package models

// Story 表示一个Hacker News文章
type Story struct {
	ID           string `json:"id"`
	Title        string `json:"title"`
	URL          string `json:"url"`
	HackerNewsURL string `json:"hackerNewsUrl"`
	Content      string `json:"content,omitempty"`
	Comments     string `json:"comments,omitempty"`
}

// StoryContent 表示文章内容（包括原文和评论）
type StoryContent struct {
	Title    string `json:"title"`
	Article  string `json:"article"`
	Comments string `json:"comments"`
}

// GeneratedContent 表示AI生成的内容
type GeneratedContent struct {
	Text        string `json:"text"`
	Usage       Usage  `json:"usage"`
	FinishReason string `json:"finishReason"`
}

// Usage 表示API使用情况
type Usage struct {
	PromptTokens     int `json:"promptTokens"`
	CompletionTokens int `json:"completionTokens"`
	TotalTokens      int `json:"totalTokens"`
}

// PodcastInfo 表示播客信息
type PodcastInfo struct {
	Title    string `json:"title"`
	Date     string `json:"date"`
	Content  string `json:"content"`
	AudioURL string `json:"audioUrl"`
}

// BlogInfo 表示博客信息
type BlogInfo struct {
	Title   string `json:"title"`
	Date    string `json:"date"`
	Content string `json:"content"`
}

// AudioSegment 表示音频片段
type AudioSegment struct {
	Index     int    `json:"index"`
	Text      string `json:"text"`
	Speaker   string `json:"speaker"`
	AudioData []byte `json:"audioData,omitempty"`
	AudioURL  string `json:"audioUrl,omitempty"`
}

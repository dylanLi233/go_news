package crawler

import (
	"fmt"
	"hacker-news/internal/models"
	"io"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/PuerkitoBio/goquery"
)

// HackerNewsClient 用于与Hacker News交互的客户端
type HackerNewsClient struct {
	jinaKey string
}

// NewHackerNewsClient 创建一个新的HackerNews客户端
func NewHackerNewsClient(jinaKey string) *HackerNewsClient {
	return &HackerNewsClient{
		jinaKey: jinaKey,
	}
}

// GetTopStories 获取指定日期的热门文章
func (c *HackerNewsClient) GetTopStories(date string, maxItems int) ([]models.Story, error) {
	// 如果未指定日期，使用今天的日期
	if date == "" {
		date = time.Now().Format("2006-01-02")
	}

	// 构建请求URL - 直接访问Hacker News，不使用Jina代理
	url := fmt.Sprintf("https://news.ycombinator.com/front?day=%s", date)
	log.Printf("获取热门文章 %s 从 %s", date, url)

	// 直接发送HTTP请求，不使用Jina代理
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("创建请求失败: %w", err)
	}

	// 设置请求头 - 模拟浏览器请求
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/91.0.4472.124 Safari/537.36")
	req.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml")
	req.Header.Set("Accept-Language", "en-US,en;q=0.9")

	// 发送请求
	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("获取热门文章失败: %w", err)
	}
	defer resp.Body.Close()

	log.Printf("获取热门文章结果: %s", resp.Status)

	// 读取响应
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("读取响应失败: %w", err)
	}

	// 解析HTML
	doc, err := goquery.NewDocumentFromReader(strings.NewReader(string(body)))
	if err != nil {
		return nil, fmt.Errorf("解析HTML失败: %w", err)
	}

	// 解析文章列表
	var stories []models.Story
	doc.Find(".athing").Each(func(i int, s *goquery.Selection) {
		id, _ := s.Attr("id")
		title := s.Find(".titleline > a").Text()
		url, _ := s.Find(".titleline > a").Attr("href")
		hackerNewsURL := fmt.Sprintf("https://news.ycombinator.com/item?id=%s", id)

		if id != "" && url != "" {
			stories = append(stories, models.Story{
				ID:            id,
				Title:         title,
				URL:           url,
				HackerNewsURL: hackerNewsURL,
			})
		}
	})

	// 限制文章数量
	if len(stories) > maxItems {
		stories = stories[:maxItems]
	}

	log.Printf("找到 %d 篇文章", len(stories))
	return stories, nil
}

// GetStoryContent 获取文章内容和评论
func (c *HackerNewsClient) GetStoryContent(story models.Story, maxTokens int) (string, error) {
	// 获取文章内容和评论
	articleCh := make(chan string, 1)
	commentsCh := make(chan string, 1)
	errCh := make(chan error, 2)

	// 并行获取文章和评论
	go func() {
		article, err := c.fetchArticle(story.URL)
		if err != nil {
			errCh <- fmt.Errorf("获取文章失败: %w", err)
			articleCh <- ""
			return
		}
		articleCh <- article
	}()

	go func() {
		comments, err := c.fetchComments(story.ID)
		if err != nil {
			errCh <- fmt.Errorf("获取评论失败: %w", err)
			commentsCh <- ""
			return
		}
		commentsCh <- comments
	}()

	// 获取结果
	article := <-articleCh
	comments := <-commentsCh

	// 检查是否有错误
	select {
	case err := <-errCh:
		return "", err
	default:
		// 继续处理
	}

	// 构建结果
	var result []string

	// 添加标题
	if story.Title != "" {
		result = append(result, fmt.Sprintf("<title>\n%s\n</title>", story.Title))
	}

	// 添加文章内容
	if article != "" {
		// 限制文章大小以避免token过多
		if len(article) > maxTokens*4 {
			article = article[:maxTokens*4]
		}
		result = append(result, fmt.Sprintf("<article>\n%s\n</article>", article))
	}

	// 添加评论
	if comments != "" {
		// 限制评论大小以避免token过多
		if len(comments) > maxTokens*4 {
			comments = comments[:maxTokens*4]
		}
		result = append(result, fmt.Sprintf("<comments>\n%s\n</comments>", comments))
	}

	return strings.Join(result, "\n\n---\n\n"), nil
}

// fetchArticle 获取文章内容
func (c *HackerNewsClient) fetchArticle(url string) (string, error) {
	// 直接访问URL，不使用Jina代理
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return "", fmt.Errorf("创建请求失败: %w", err)
	}

	// 设置请求头 - 模拟浏览器请求
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/91.0.4472.124 Safari/537.36")
	req.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml")
	req.Header.Set("Accept-Language", "en-US,en;q=0.9")

	// 发送请求
	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		log.Printf("获取文章失败: %v %s", err, url)
		return "", nil
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		log.Printf("获取文章失败: %s %s", resp.Status, url)
		return "", nil
	}

	// 读取响应
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("读取响应失败: %w", err)
	}

	return string(body), nil
}

// fetchComments 获取文章评论
func (c *HackerNewsClient) fetchComments(storyID string) (string, error) {
	// 直接访问Hacker News评论页面
	commentURL := fmt.Sprintf("https://news.ycombinator.com/item?id=%s", storyID)
	req, err := http.NewRequest("GET", commentURL, nil)
	if err != nil {
		return "", fmt.Errorf("创建请求失败: %w", err)
	}

	// 设置请求头 - 模拟浏览器请求
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/91.0.4472.124 Safari/537.36")
	req.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml")
	req.Header.Set("Accept-Language", "en-US,en;q=0.9")

	// 发送请求
	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		log.Printf("获取评论失败: %v https://news.ycombinator.com/item?id=%s", err, storyID)
		return "", nil
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		log.Printf("获取评论失败: %s https://news.ycombinator.com/item?id=%s", resp.Status, storyID)
		return "", nil
	}

	// 读取响应
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("读取响应失败: %w", err)
	}

	// 使用goquery提取评论内容，模拟Jina的X-Target-Selector功能
	doc, err := goquery.NewDocumentFromReader(strings.NewReader(string(body)))
	if err != nil {
		return "", fmt.Errorf("解析HTML失败: %w", err)
	}

	// 提取评论区域（会因网站结构变化而需要调整）
	commentsHtml, err := doc.Find(".comment-tree").Html()
	if err != nil {
		return "", fmt.Errorf("提取评论区域失败: %w", err)
	}

	return commentsHtml, nil
}

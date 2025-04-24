package api

import (
	"context"
	"encoding/json"
	"fmt"
	"hacker-news/config"
	"hacker-news/internal/ai"
	"hacker-news/internal/crawler"
	"hacker-news/internal/storage"
	"hacker-news/internal/tts"
	"log"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	// "github.com/sirupsen/logrus"
	// "runtime/debug"

	// "bufio"
	// "bytes"
	// "encoding/binary"
	// "io"
	// "io/ioutil"
	"os/exec"
	"path/filepath"
)

// Server 是API服务器结构
type Server struct {
	config        *config.Config
	router        *gin.Engine
	aiClient      *ai.Client
	minioClient   *storage.MinioClient
	hnClient      *crawler.HackerNewsClient
	ttsService    tts.Service
	isProcessing  bool
	lastProcessed time.Time
}

// NewServer 创建一个新的API服务器
func NewServer(cfg *config.Config) (*Server, error) {
	// 创建AI客户端
	aiClient := ai.NewClient(&cfg.OpenAI)

	// 创建MinIO客户端
	minioClient, err := storage.NewMinioClient(&cfg.MinIO)
	if err != nil {
		return nil, err
	}

	// 创建Hacker News客户端
	hnClient := crawler.NewHackerNewsClient(cfg.HackerNews.JinaKey)

	// 创建TTS服务
	ttsService, err := tts.Factory(&cfg.TTS)
	if err != nil {
		return nil, err
	}

	// 创建Gin路由
	router := gin.Default()

	// 启用CORS
	router.Use(func(c *gin.Context) {
		c.Writer.Header().Set("Access-Control-Allow-Origin", "*")
		c.Writer.Header().Set("Access-Control-Allow-Credentials", "true")
		c.Writer.Header().Set("Access-Control-Allow-Headers", "Content-Type, Content-Length, Accept-Encoding, X-CSRF-Token, Authorization")
		c.Writer.Header().Set("Access-Control-Allow-Methods", "POST, GET, OPTIONS, PUT, DELETE")

		if c.Request.Method == "OPTIONS" {
			c.AbortWithStatus(204)
			return
		}

		c.Next()
	})

	// 创建服务器
	server := &Server{
		config:        cfg,
		router:        router,
		aiClient:      aiClient,
		minioClient:   minioClient,
		hnClient:      hnClient,
		ttsService:    ttsService,
		isProcessing:  false,
		lastProcessed: time.Time{},
	}

	// 注册路由
	server.registerRoutes()

	return server, nil
}

// registerRoutes 注册API路由
func (s *Server) registerRoutes() {
	// 健康检查
	s.router.GET("/health", s.healthHandler)

	// API v1
	v1 := s.router.Group("/api/v1")
	{
		// 处理Hacker News文章
		v1.POST("/process", s.processHandler)

		// 获取播客
		v1.GET("/podcast", s.getPodcastHandler)

		// 获取博客
		v1.GET("/blog", s.getBlogHandler)

		// 获取处理状态
		v1.GET("/status", s.getStatusHandler)

		// 文本转语音
		v1.POST("/tts", s.ttsHandler)

		// 音频合并
		v1.POST("/audio/concat", s.concatAudioHandler)
		
		// 删除内容
		v1.DELETE("/content", s.deleteContentHandler)
	}

	// 提供音频文件
	s.router.GET("/audio/:filename", s.serveAudioHandler)
}

// Run 启动API服务器
func (s *Server) Run() error {
	return s.router.Run(":" + s.config.Server.Port)
}

// ProcessHackerNews 导出处理Hacker News的方法，供外部调用
func (s *Server) ProcessHackerNews(date string, maxItems int) {
	s.processHackerNews(date, maxItems, false, false)
}

// healthHandler 健康检查处理程序
func (s *Server) healthHandler(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"status": "ok",
		"time":   time.Now().Format(time.RFC3339),
	})
}

// processHandler 处理Hacker News文章
func (s *Server) processHandler(c *gin.Context) {
	// 获取请求参数
	var req struct {
		Date       string `json:"date"`
		MaxItems   int    `json:"maxItems"`
		Force      bool   `json:"force"`      // 强制重新处理内容
		ForceAudio bool   `json:"force_audio"` // 强制重新生成音频
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "无效的请求参数",
		})
		return
	}

	// 如果未指定日期，使用今天的日期
	if req.Date == "" {
		req.Date = time.Now().Format("2006-01-02")
	}

	// 如果未指定最大项数，使用配置中的值
	if req.MaxItems <= 0 {
		req.MaxItems = s.config.HackerNews.MaxItems
	}

	// 标记为正在处理
	s.isProcessing = true

	// 在后台处理
	go func() {
		defer func() {
			s.isProcessing = false
		}()
		s.processHackerNews(req.Date, req.MaxItems, req.Force, req.ForceAudio)
	}()

	c.JSON(http.StatusOK, gin.H{
		"date":    req.Date,
		"message": "处理已开始",
	})
}

// getPodcastHandler 获取最新的播客
func (s *Server) getPodcastHandler(c *gin.Context) {
	// 获取日期参数
	date := c.Query("date")
	if date == "" {
		date = time.Now().Format("2006-01-02")
	}

	// 构建日期路径格式
	datePath := strings.ReplaceAll(date, "-", "/")
	env := s.config.Server.Env
	podcastObjectName := datePath + "/" + env + "/hacker-news-" + date + ".mp3"

	// 获取预签名URL
	ctx := c.Request.Context()
	presignedURL, err := s.minioClient.GetPresignedURL(ctx, podcastObjectName, 24*time.Hour)
	if err != nil {
		log.Printf("获取预签名URL失败: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "获取播客失败",
		})
		return
	}

	// 获取内容
	contentObjectName := "content:" + env + ":hacker-news:" + date
	contentData, err := s.minioClient.DownloadFile(ctx, contentObjectName)
	if err != nil {
		log.Printf("获取内容失败: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "获取播客内容失败",
		})
		return
	}

	var content struct {
		Intro    string `json:"intro"`
		Podcast  string `json:"podcast"`
		Blog     string `json:"blog"`
		AudioURL string `json:"audioUrl"`
	}

	// 解析内容
	if err := json.Unmarshal(contentData, &content); err != nil {
		log.Printf("解析内容失败: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "解析内容失败",
		})
		return
	}

	// 返回播客信息
	c.JSON(http.StatusOK, gin.H{
		"date":     date,
		"title":    "Hacker News 每日播客 " + date,
		"intro":    content.Intro,
		"content":  content.Podcast,
		"audioUrl": presignedURL,
	})
}

// getBlogHandler 获取最新的博客
func (s *Server) getBlogHandler(c *gin.Context) {
	// 获取日期参数
	date := c.Query("date")
	if date == "" {
		date = time.Now().Format("2006-01-02")
	}

	// 构建内容对象名
	env := s.config.Server.Env
	contentObjectName := "content:" + env + ":hacker-news:" + date

	// 获取内容
	ctx := c.Request.Context()
	contentData, err := s.minioClient.DownloadFile(ctx, contentObjectName)
	if err != nil {
		log.Printf("获取内容失败: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "获取博客内容失败",
		})
		return
	}

	var content struct {
		Intro    string `json:"intro"`
		Podcast  string `json:"podcast"`
		Blog     string `json:"blog"`
		AudioURL string `json:"audioUrl"`
	}

	// 解析内容
	if err := json.Unmarshal(contentData, &content); err != nil {
		log.Printf("解析内容失败: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "解析内容失败",
		})
		return
	}

	// 返回博客信息
	c.JSON(http.StatusOK, gin.H{
		"date":    date,
		"title":   "Hacker News 每日博客 " + date,
		"content": content.Blog,
	})
}

// getStatusHandler 获取处理状态
func (s *Server) getStatusHandler(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"isProcessing":  s.isProcessing,
		"lastProcessed": s.lastProcessed.Format(time.RFC3339),
	})
}

// ttsHandler 文本转语音处理
func (s *Server) ttsHandler(c *gin.Context) {
	// 获取请求参数
	var req struct {
		Text    string `json:"text" binding:"required"`
		Speaker string `json:"speaker" binding:"required"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "无效的请求参数",
		})
		return
	}

	// 转换文本为语音
	ctx := c.Request.Context()
	audio, err := s.ttsService.SynthesizeSpeech(ctx, req.Text, req.Speaker)
	if err != nil {
		log.Printf("文本转语音失败: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "文本转语音失败",
		})
		return
	}

	// 生成文件名
	filename := "tts-" + time.Now().Format("20060102-150405") + ".mp3"

	// 上传到MinIO
	contentType := "audio/mpeg"
	audioURL, err := s.minioClient.UploadFile(ctx, filename, audio, contentType)
	if err != nil {
		log.Printf("上传音频失败: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "上传音频失败",
		})
		return
	}

	// 返回音频URL
	c.JSON(http.StatusOK, gin.H{
		"audioUrl": audioURL,
	})
}

// concatAudioHandler 合并音频处理
func (s *Server) concatAudioHandler(c *gin.Context) {
	// 获取请求参数
	var req struct {
		AudioURLs []string `json:"audioUrls" binding:"required"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "无效的请求参数",
		})
		return
	}

	// 这里简单演示，实际合并逻辑需要使用FFmpeg或其他音频处理库
	c.JSON(http.StatusNotImplemented, gin.H{
		"error": "音频合并功能尚未实现",
	})
}

// serveAudioHandler 提供音频文件
func (s *Server) serveAudioHandler(c *gin.Context) {
	// 获取文件名
	filename := c.Param("filename")
	if filename == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "文件名不能为空",
		})
		return
	}

	// 使用统一的路径方式：audio/文件名
	audioPath := "audio/" + filename
	ctx := c.Request.Context()
	data, err := s.minioClient.DownloadFile(ctx, audioPath)
	if err != nil {
		log.Printf("获取音频文件失败: %v", err)
		c.JSON(http.StatusNotFound, gin.H{
			"error": "文件不存在",
		})
		return
	}

	// 设置响应头
	c.Writer.Header().Set("Content-Type", "audio/mpeg")
	c.Writer.Header().Set("Content-Disposition", "inline")
	c.Writer.Write(data)
}

// deleteContentHandler 删除已有内容
func (s *Server) deleteContentHandler(c *gin.Context) {
	// 获取日期参数
	date := c.Query("date")
	if date == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "日期不能为空",
		})
		return
	}

	// 构建内容对象名
	env := s.config.Server.Env
	contentObjectName := "content:" + env + ":hacker-news:" + date
	
	// 构建音频对象名(新格式)
	audioObjectName := "audio/hacker-news-" + date + ".mp3"
	
	// 构建音频对象名(旧格式)
	oldAudioObjectName := strings.ReplaceAll(date, "-", "/") + "/" + env + "/hacker-news-" + date + ".mp3"

	// 删除内容
	ctx := c.Request.Context()
	err := s.minioClient.DeleteFile(ctx, contentObjectName)
	if err != nil {
		log.Printf("删除内容失败: %v", err)
		// 继续执行，因为有可能内容不存在
	}

	// 尝试删除新格式音频
	err = s.minioClient.DeleteFile(ctx, audioObjectName)
	if err != nil {
		log.Printf("删除新格式音频失败: %v", err)
		// 继续执行，尝试删除旧格式
	}
	
	// 尝试删除旧格式音频
	err = s.minioClient.DeleteFile(ctx, oldAudioObjectName)
	if err != nil {
		log.Printf("删除旧格式音频失败: %v", err)
		// 继续执行，因为有可能音频不存在
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "内容删除成功",
		"date": date,
	})
}

// processHackerNews 处理Hacker News文章的核心逻辑
func (s *Server) processHackerNews(date string, maxItems int, force bool, forceAudio bool) {
	log.Printf("开始处理 Hacker News 文章，日期: %s", date)

	// 标记处理开始
	s.isProcessing = true
	defer func() {
		s.isProcessing = false
		s.lastProcessed = time.Now()
	}()

	// 创建上下文
	ctx := context.Background()

	// 构建内容对象的键名
	contentKey := fmt.Sprintf("content:%s:hacker-news:%s", s.config.Server.Env, date)
	
	// 定义变量用于存储内容
	var podcastContent, blogContent, introContent string
	var content struct {
		Intro      string   `json:"intro"`
		Podcast    string   `json:"podcast"`
		Blog       string   `json:"blog"`
		AudioURL   string   `json:"audioUrl"`
		AudioFiles []string `json:"audioFiles"`
	}
	
	// 检查该日期内容是否已存在
	contentExists, err := s.minioClient.ObjectExists(ctx, contentKey)
	if err != nil {
		log.Printf("检查内容是否存在失败: %v", err)
	}
	
	// 如果内容不存在，生成内容
	if !contentExists || force {
		log.Printf("日期 %s 的内容不存在或强制重新生成，开始生成", date)
		
		// 步骤1: 获取热门文章
		stories, err := s.hnClient.GetTopStories(date, maxItems)
		if err != nil {
			log.Printf("获取热门文章失败: %v", err)
			return
		}
		log.Printf("获取了 %d 篇热门文章", len(stories))

		// 步骤2: 获取文章内容并生成摘要
		var storyTexts []string
		for i, story := range stories {
			log.Printf("处理文章 %d/%d: %s", i+1, len(stories), story.Title)

			// 获取文章内容
			storyContent, err := s.hnClient.GetStoryContent(story, s.config.OpenAI.MaxTokens)
			if err != nil {
				log.Printf("获取文章内容失败: %v", err)
				continue
			}

			// 生成文章摘要
			text, err := s.aiClient.GenerateStoryText(ctx, storyContent)
			if err != nil {
				log.Printf("生成文章摘要失败: %v", err)
				continue
			}

			storyTexts = append(storyTexts, text)

			// 稍作暂停，避免API限制
			time.Sleep(2 * time.Second)
		}

		if len(storyTexts) == 0 {
			log.Printf("没有成功处理任何文章")
			return
		}

		// 步骤3: 生成播客内容
		log.Printf("开始生成播客内容")
		podcastContent, err = s.aiClient.GeneratePodcastContent(ctx, storyTexts)
		if err != nil {
			log.Printf("生成播客内容失败: %v", err)
			return
		}

		// 步骤4: 生成博客内容
		log.Printf("开始生成博客内容")
		blogContent, err = s.aiClient.GenerateBlogContent(ctx, storyTexts)
		if err != nil {
			log.Printf("生成博客内容失败: %v", err)
			return
		}

		// 步骤5: 生成简介
		log.Printf("开始生成简介")
		introContent, err = s.aiClient.GenerateIntroContent(ctx, podcastContent)
		if err != nil {
			log.Printf("生成简介失败: %v", err)
			return
		}
		
		// 构建内容对象
		content = struct {
			Intro      string   `json:"intro"`
			Podcast    string   `json:"podcast"`
			Blog       string   `json:"blog"`
			AudioURL   string   `json:"audioUrl"`
			AudioFiles []string `json:"audioFiles"`
		}{
			Intro:      introContent,
			Podcast:    podcastContent,
			Blog:       blogContent,
			AudioURL:   "",
			AudioFiles: []string{},
		}
		
		// 序列化并保存内容
		contentData, err := json.Marshal(content)
		if err != nil {
			log.Printf("序列化内容失败: %v", err)
			return
		}

		// 上传内容
		_, err = s.minioClient.UploadFile(ctx, contentKey, contentData, "application/json")
		if err != nil {
			log.Printf("上传内容失败: %v", err)
			return
		}
		
		log.Printf("内容生成完成，已保存到存储")
	} else {
		// 如果内容已存在，获取现有内容
		log.Printf("日期 %s 的内容已存在，获取现有内容", date)
		contentData, err := s.minioClient.DownloadFile(ctx, contentKey)
		if err != nil {
			log.Printf("获取现有内容失败: %v", err)
			return
		}
		
		// 解析内容
		err = json.Unmarshal(contentData, &content)
		if err != nil {
			log.Printf("解析内容失败: %v", err)
			return
		}
		
		podcastContent = content.Podcast
		introContent = content.Intro
		
		// 检查是否已有音频URL
		if content.AudioURL != "" && !forceAudio {
			log.Printf("音频已存在，URL: %s", content.AudioURL)
			return
		}
		
		log.Printf("音频不存在或强制重新生成，开始生成")
	}

	// 步骤6: 生成播客音频
	log.Printf("开始生成播客音频")

	// 生成播客主内容音频片段并收集
	var audioSegments [][]byte


	for _, conversation := range strings.Split(podcastContent, "\n") {
		if strings.TrimSpace(conversation) == "" {
			continue
		}
		// 根据前缀确定说话者
		speaker := "女"
		if strings.HasPrefix(conversation, "男:") || strings.HasPrefix(conversation, "男：") {
			speaker = "男"
		}
		// 日志：原始内容
		log.Printf("原始对话: %q", conversation)
	
		// 优化前缀移除逻辑，兼容中英文冒号
		text := conversation
		if idx := strings.IndexAny(conversation, ":："); idx != -1 {
			text = strings.TrimSpace(conversation[idx+1:])
		}
		// 日志：处理后内容
		log.Printf("去前缀后: %q", text)
	
		// 生成语音
		audio, err := s.ttsService.SynthesizeSpeech(ctx, text, speaker)
		if err != nil {
			log.Printf("生成音频失败: %v", err)
			continue
		}
		audioSegments = append(audioSegments, audio)
	}






	// 合并所有音频片段并上传
	if len(audioSegments) > 0 {
		mergedAudio, err := mergeAudioBytes(ctx, audioSegments)
		if err != nil {
			log.Printf("合并音频失败: %v", err)
		} else {
			mergedAudioKey := fmt.Sprintf("audio/%s-complete.mp3", date)
			mergedAudioURL, err := s.minioClient.UploadFile(ctx, mergedAudioKey, mergedAudio, "audio/mpeg")
			if err != nil {
				log.Printf("上传合并音频失败: %v", err)
			} else {
				content.AudioURL = mergedAudioURL
			}
		}
	}

	// 生成并上传简介音频（intro）
	_, err = s.ttsService.SynthesizeSpeech(ctx, introContent, "男")
	if err != nil {
		log.Printf("生成简介音频失败: %v", err)
	} 
	// else {
	// 	introKey := fmt.Sprintf("audio/%s-intro.mp3", date)
	// 	introURL, err := s.minioClient.UploadFile(ctx, introKey, introAudio, "audio/mpeg")
	// 	if err != nil {
	// 		log.Printf("上传简介音频失败: %v", err)
	// 	} else {
	// 		content.AudioFiles = []string{introURL}
	// 	}
	// }

	// 序列化并上传内容对象
	contentData, err := json.Marshal(content)
	if err != nil {
		log.Printf("序列化内容失败: %v", err)
		return
	}
	_, err = s.minioClient.UploadFile(ctx, contentKey, contentData, "application/json")
	if err != nil {
		log.Printf("上传内容失败: %v", err)
		return
	}

	log.Printf("处理完成，日期: %s", date)
}

func mergeAudioBytes(ctx context.Context, audioSegments [][]byte) ([]byte, error) {
    tempDir, err := os.MkdirTemp("", "audio-merge")
    if err != nil {
        return nil, err
    }
    defer os.RemoveAll(tempDir)

    var fileListPath = filepath.Join(tempDir, "filelist.txt")
    fileList, err := os.Create(fileListPath)
    if err != nil {
        return nil, err
    }
    defer fileList.Close()

    var segmentFiles []string
    for i, segment := range audioSegments {
        segPath := filepath.Join(tempDir, fmt.Sprintf("seg%d.mp3", i))
        if err := os.WriteFile(segPath, segment, 0644); err != nil {
            return nil, err
        }
        fmt.Fprintf(fileList, "file '%s'\n", segPath)
        segmentFiles = append(segmentFiles, segPath)
    }
    fileList.Sync()

    outPath := filepath.Join(tempDir, "merged.mp3")
    cmd := exec.CommandContext(ctx, "ffmpeg", "-y", "-f", "concat", "-safe", "0", "-i", fileListPath, "-c", "copy", outPath)
    if err := cmd.Run(); err != nil {
        return nil, err
    }
    return os.ReadFile(outPath)
}
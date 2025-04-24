# Hacker News 内容聚合服务 - Go 版本

这是 Hacker News 内容聚合服务的 Go 实现版本，用于替代原有的基于 Cloudflare Workers 和 Workflows 的实现。本项目可完全在本地或任何支持 Docker 的环境中部署运行。

## 功能特性

- **文章爬取**：自动获取 Hacker News 热门文章及评论
- **AI摘要生成**：使用OpenAI/Deepseek API生成文章摘要
- **播客内容生成**：生成结构化的播客脚本
- **博客内容生成**：生成针对搜索引擎优化的博客内容
- **文本转语音**：支持多种TTS服务，包括Edge TTS和阿里云TTS
- **定时更新**：通过内置cron任务定时更新内容
- **本地存储**：使用MinIO对象存储代替Cloudflare R2

## 系统架构

项目采用模块化设计，主要组件包括：

- **API服务**：提供HTTP接口用于触发任务和获取内容
- **爬虫模块**：负责从Hacker News抓取内容
- **AI模块**：与AI服务交互生成内容
- **存储模块**：负责内容和音频的存储管理
- **TTS模块**：处理文本到语音的转换

## 目录结构

```
.
├── cmd/
│   └── server/         # 主服务入口
├── config/             # 配置管理
├── internal/
│   ├── api/            # HTTP API实现
│   ├── crawler/        # 网页爬虫
│   ├── models/         # 数据模型
│   ├── storage/        # 存储接口
│   ├── tts/            # 文本转语音
│   └── ai/             # AI服务接口
├── scripts/            # 实用脚本
├── Dockerfile          # Docker构建文件
├── docker-compose.yml  # Docker Compose配置
├── go.mod              # Go模块定义
└── README.md           # 项目文档
```

## 环境要求

- Go 1.24 或更高版本
- Docker 和 Docker Compose (用于容器化部署)
- MinIO (作为对象存储)
- OpenAI/Deepseek API账户
- (可选) 阿里云TTS账户

## 安装与运行

### 本地开发环境

1. 克隆仓库：

```bash
git clone <仓库URL>
cd hacker-news
```

2. 安装依赖：

```bash
go mod tidy
```

3. 设置环境变量：

创建`.env`文件并填入：

```
OPENAI_BASE_URL=https://api.deepseek.com/v1
OPENAI_API_KEY=你的API密钥
OPENAI_MODEL=deepseek-chat
TTS_PROVIDER=edge
```

4. 运行服务：

```bash
go run cmd/server/main.go
```

### Docker部署（推荐用于生产环境）

1. 使用Docker Compose启动所有服务：

```bash
docker-compose up -d
```

这将启动应用服务和MinIO存储服务。

2. 查看服务状态：

```bash
docker-compose ps
```
1. 健康检查接口
curl http://localhost:3001/health

# 获取今天的播客
curl http://localhost:3001/api/v1/podcast

# 获取指定日期的播客
curl http://localhost:3001/api/v1/podcast?date=2025-04-20

# 处理当天文章
curl -X POST http://localhost:3001/api/v1/process \
  -H "Content-Type: application/json" \
  -d '{}'


# 处理指定日期的文章，限制数量
curl -X POST http://localhost:3001/api/v1/process \
  -H "Content-Type: application/json" \
  -d '{"date": "2025-04-20", "maxItems": 5}'


curl http://localhost:6005/api/v1/status

curl -X POST http://localhost:6005/api/v1/audio/concat \
  -H "Content-Type: application/json" \
  -d '{"audioUrls": ["http://localhost:6005/audio/file1.mp3", "http://localhost:6005/audio/file2.mp3"]}'

curl -X POST http://localhost:6005/api/v1/tts \
  -H "Content-Type: application/json" \
  -d '{"text": "这是一段测试文本", "speaker": "男"}'
curl http://localhost:6005/audio/some-audio-file.mp3 --output audio.mp3



# 仅强制重新生成音频，保留现有内容
curl -X POST http://localhost:3001/api/v1/process -H "Content-Type: application/json" -d '{"force_audio": true}'

# 强制重新生成所有内容（包括文章和音频）
curl -X POST http://localhost:3001/api/v1/process -H "Content-Type: application/json" -d '{"force": true}'

# 处理特定日期的内容并强制重新生成
curl -X POST http://localhost:3001/api/v1/process -H "Content-Type: application/json" -d '{"date": "2025-04-19", "force": true}'






3. 访问服务：
   - API服务: http://localhost:8080
   - MinIO控制台: http://localhost:9001 (用户名/密码: minioadmin/minioadmin)

## API文档

### 手动触发内容处理

```
POST /api/v1/process
Content-Type: application/json

{
  "date": "2025-04-20",  // 可选，默认为当天
  "maxItems": 10         // 可选，处理的文章数量
}
```

### 获取播客内容

```
GET /api/v1/podcast?date=2025-04-20
```

### 获取博客内容

```
GET /api/v1/blog?date=2025-04-20
```

### 文本转语音

```
POST /api/v1/tts
Content-Type: application/json

{
  "text": "要转换的文本",
  "speaker": "男"  // 或 "女"
}
```

## 配置选项

所有配置都可以通过环境变量设置：

| 环境变量 | 描述 | 默认值 |
|---------|------|-------|
| APP_PORT | 服务监听端口 | 8080 |
| WORKER_ENV | 环境名称 | production |
| OPENAI_BASE_URL | OpenAI API URL | https://api.deepseek.com/v1 |
| OPENAI_API_KEY | OpenAI API密钥 | - |
| OPENAI_MODEL | 使用的模型名称 | deepseek-chat |
| OPENAI_MAX_TOKENS | 最大输出token数 | 4096 |
| TTS_PROVIDER | TTS服务提供商 | edge |
| MINIO_ACCESS_KEY | MinIO访问密钥 | minioadmin |
| MINIO_SECRET_KEY | MinIO私钥 | minioadmin |
| HACKER_NEWS_BUCKET_NAME | MinIO存储桶名称 | hacker-news |
| JINA_KEY | Jina.ai API密钥 | - |

## Mac部署指南

在Mac上部署时，由于是ARM64架构，可以按照以下步骤进行：

1. 安装Docker Desktop for Mac

2. 克隆代码仓库：
```bash
git clone <仓库URL>
cd hacker-news
```

3. 创建`.env`文件设置必要的环境变量：
```bash
OPENAI_API_KEY=你的API密钥
```

4. 使用Docker Compose启动服务：
```bash
docker-compose up -d
```

5. 查看日志：
```bash
docker-compose logs -f app
```

## 问题排查

常见问题及解决方案：

1. **MinIO连接失败**：
   - 检查MinIO是否已启动
   - 验证访问凭证是否正确

2. **AI API调用失败**：
   - 检查API密钥是否设置
   - 确认请求URL配置是否正确
   - 查看API使用限制是否已达到

3. **TTS服务故障**：
   - 检查服务提供商配置
   - 尝试切换到备用TTS服务

## 贡献指南

欢迎贡献代码或提出改进建议：

1. Fork项目仓库
2. 创建特性分支 (`git checkout -b feature/amazing-feature`)
3. 提交更改 (`git commit -m 'Add some amazing feature'`)
4. 推送分支 (`git push origin feature/amazing-feature`)
5. 创建Pull Request

## 许可证

[MIT License](LICENSE)


础接口
GET /health - 健康检查接口，用于监控服务是否正常运行
核心功能接口
POST /api/v1/process - 触发Hacker News文章处理的接口，可以指定日期参数
GET /api/v1/podcast - 获取生成的播客内容，包括音频URL和文字稿
GET /api/v1/blog - 获取生成的博客文章内容
GET /api/v1/status - 查询当前处理状态的接口，显示是否有任务正在执行
语音相关接口
POST /api/v1/tts - 文本转语音接口，可以将文本转换为音频
POST /api/v1/audio/concat - 音频合并接口，用于将多个音频片段合并为一个
GET /audio/:filename - 音频文件访问接口，根据文件名获取音频文件
这些接口构成了一个完整的系统，可以获取Hacker News文章、生成AI摘要、创建播客和博客内容，并提供文本转语音服务。

服务已经在8080端口启动，您可以使用浏览器或API测试工具(如curl、Postman)访问这些接口。例如，访问http://localhost:8080/health可以检查服务健康状态。

现在您可以使用POST /api/v1/process接口触发文章处理，然后通过GET /api/v1/podcast或GET /api/v1/blog获取内容。
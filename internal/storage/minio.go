package storage

import (
	"bytes"
	"context"
	"fmt"
	"hacker-news/config"
	"io"
	"log"
	"net/url"
	"time"

	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
)

// MinioClient 是MinIO存储客户端的封装
type MinioClient struct {
	client     *minio.Client
	bucketName string
}

// NewMinioClient 创建一个新的MinIO客户端
func NewMinioClient(cfg *config.MinIOConfig) (*MinioClient, error) {
	// 解析endpoint
	u, err := url.Parse(cfg.Endpoint)
	if err != nil {
		return nil, fmt.Errorf("解析MinIO endpoint失败: %w", err)
	}

	// 创建MinIO客户端
	secure := u.Scheme == "https"
	endpoint := u.Host

	// 如果endpoint为空，使用localhost:9000
	if endpoint == "" {
		endpoint = "localhost:9000"
	}

	client, err := minio.New(endpoint, &minio.Options{
		Creds:  credentials.NewStaticV4(cfg.AccessKeyID, cfg.SecretAccessKey, ""),
		Secure: secure,
	})
	if err != nil {
		return nil, fmt.Errorf("创建MinIO客户端失败: %w", err)
	}

	// 确保bucket存在
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	exists, err := client.BucketExists(ctx, cfg.BucketName)
	if err != nil {
		return nil, fmt.Errorf("检查bucket是否存在失败: %w", err)
	}

	if !exists {
		log.Printf("Bucket %s 不存在，正在创建...", cfg.BucketName)
		err = client.MakeBucket(ctx, cfg.BucketName, minio.MakeBucketOptions{})
		if err != nil {
			return nil, fmt.Errorf("创建bucket失败: %w", err)
		}
		log.Printf("Bucket %s 创建成功", cfg.BucketName)
	}

	return &MinioClient{
		client:     client,
		bucketName: cfg.BucketName,
	}, nil
}

// UploadFile 上传文件到MinIO
func (c *MinioClient) UploadFile(ctx context.Context, objectName string, data []byte, contentType string) (string, error) {
	// 创建reader
	reader := bytes.NewReader(data)

	// 上传文件
	info, err := c.client.PutObject(ctx, c.bucketName, objectName, reader, int64(len(data)), minio.PutObjectOptions{
		ContentType: contentType,
	})
	if err != nil {
		return "", fmt.Errorf("上传文件失败: %w", err)
	}

	log.Printf("文件 %s 上传成功，大小: %d", objectName, info.Size)

	// 生成预签名URL
	presignedURL, err := c.GetPresignedURL(ctx, objectName, 7*24*time.Hour) // 7天有效期
	if err != nil {
		log.Printf("生成预签名URL失败: %v", err)
		// 返回相对路径
		return fmt.Sprintf("/%s/%s", c.bucketName, objectName), nil
	}

	return presignedURL, nil
}

// DownloadFile 从MinIO下载文件
func (c *MinioClient) DownloadFile(ctx context.Context, objectName string) ([]byte, error) {
	// 获取对象
	obj, err := c.client.GetObject(ctx, c.bucketName, objectName, minio.GetObjectOptions{})
	if err != nil {
		return nil, fmt.Errorf("获取对象失败: %w", err)
	}
	defer obj.Close()

	// 读取数据
	data, err := io.ReadAll(obj)
	if err != nil {
		return nil, fmt.Errorf("读取对象数据失败: %w", err)
	}

	return data, nil
}

// GetPresignedURL 生成预签名URL
func (c *MinioClient) GetPresignedURL(ctx context.Context, objectName string, expiry time.Duration) (string, error) {
	// 生成预签名URL
	presignedURL, err := c.client.PresignedGetObject(ctx, c.bucketName, objectName, expiry, nil)
	if err != nil {
		return "", fmt.Errorf("生成预签名URL失败: %w", err)
	}

	return presignedURL.String(), nil
}

// DeleteFile 从MinIO删除文件
func (c *MinioClient) DeleteFile(ctx context.Context, objectName string) error {
	return c.client.RemoveObject(ctx, c.bucketName, objectName, minio.RemoveObjectOptions{})
}

// ListFiles 列出指定前缀的所有文件
func (c *MinioClient) ListFiles(ctx context.Context, prefix string) ([]string, error) {
	// 列出对象
	objectCh := c.client.ListObjects(ctx, c.bucketName, minio.ListObjectsOptions{
		Prefix:    prefix,
		Recursive: true,
	})

	var objects []string
	for object := range objectCh {
		if object.Err != nil {
			return nil, fmt.Errorf("列出对象失败: %w", object.Err)
		}
		objects = append(objects, object.Key)
	}

	return objects, nil
}

// ObjectExists 检查对象是否存在
func (c *MinioClient) ObjectExists(ctx context.Context, objectName string) (bool, error) {
	// 尝试使用GetObject方法获取对象头信息，如果成功则对象存在
	obj, err := c.client.GetObject(ctx, c.bucketName, objectName, minio.GetObjectOptions{})
	if err != nil {
		return false, fmt.Errorf("获取对象失败: %w", err)
	}
	
	// 尝试获取对象状态，不读取内容
	stat, err := obj.Stat()
	if err != nil {
		// 对象不存在
		return false, nil
	}
	
	// 对象存在且有效
	if stat.Size > 0 {
		return true, nil
	}
	
	return false, nil
}

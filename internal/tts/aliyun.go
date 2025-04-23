package tts

import (
	"context"
	"crypto/hmac"
	"crypto/sha1"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"hacker-news/config"
	"io"
	"log"
	"net/http"
	"net/url"
	"sort"
	"strings"
	"time"
)

// AliyunTTS 实现阿里云TTS服务
type AliyunTTS struct {
	config config.AliyunTTSConfig
}

// NewAliyunTTS 创建一个新的阿里云TTS服务
func NewAliyunTTS(cfg config.AliyunTTSConfig) (*AliyunTTS, error) {
	return &AliyunTTS{
		config: cfg,
	}, nil
}

// SynthesizeSpeech 将文本转换为语音
func (a *AliyunTTS) SynthesizeSpeech(ctx context.Context, text string, speaker string) ([]byte, error) {
	// 根据角色获取语音ID
	voiceID, err := GetSpeakerVoice("aliyun", speaker)
	if err != nil {
		return nil, err
	}

	log.Printf("使用阿里云TTS转换文本，语音ID: %s", voiceID)

	// 构建请求参数
	params := map[string]string{
		"Action":           "SpeechSynthesis",
		"Format":           "mp3",
		"Voice":            voiceID,
		"Volume":           "50",
		"SpeechRate":       "0",
		"PitchRate":        "0",
		"Text":             text,
		"Version":          "2019-08-10",
		"RegionId":         a.config.Region,
		"Timestamp":        time.Now().UTC().Format("2006-01-02T15:04:05Z"),
		"SignatureMethod":  "HMAC-SHA1",
		"SignatureVersion": "1.0",
		"SignatureNonce":   fmt.Sprintf("%d", time.Now().UnixNano()),
		"AccessKeyId":      a.config.AccessKeyID,
	}

	// 生成签名
	signature, err := a.computeSignature(params, a.config.AccessKeySecret)
	if err != nil {
		return nil, fmt.Errorf("计算签名失败: %w", err)
	}
	params["Signature"] = signature

	// 构建请求URL
	requestURL := fmt.Sprintf("https://nls-gateway-%s.aliyuncs.com/", a.config.Region)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, requestURL, nil)
	if err != nil {
		return nil, fmt.Errorf("创建请求失败: %w", err)
	}

	// 添加查询参数
	q := req.URL.Query()
	for k, v := range params {
		q.Add(k, v)
	}
	req.URL.RawQuery = q.Encode()

	// 发送请求
	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("发送请求失败: %w", err)
	}
	defer resp.Body.Close()

	// 检查响应状态
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("TTS请求失败，状态码: %d，响应: %s", resp.StatusCode, string(body))
	}

	// 解析JSON响应
	var result struct {
		RequestId string `json:"RequestId"`
		Code      string `json:"Code"`
		Message   string `json:"Message"`
		Data      struct {
			AudioAddress string `json:"AudioAddress"`
		} `json:"Data"`
	}

	err = json.NewDecoder(resp.Body).Decode(&result)
	if err != nil {
		return nil, fmt.Errorf("解析响应失败: %w", err)
	}

	// 检查返回码
	if result.Code != "Success" {
		return nil, fmt.Errorf("TTS请求失败: %s", result.Message)
	}

	// 下载音频文件
	audioResp, err := http.Get(result.Data.AudioAddress)
	if err != nil {
		return nil, fmt.Errorf("下载音频失败: %w", err)
	}
	defer audioResp.Body.Close()

	// 读取音频数据
	audio, err := io.ReadAll(audioResp.Body)
	if err != nil {
		return nil, fmt.Errorf("读取音频数据失败: %w", err)
	}

	log.Printf("TTS转换成功，音频大小: %d 字节", len(audio))
	return audio, nil
}

// Provider 返回TTS提供商名称
func (a *AliyunTTS) Provider() string {
	return "aliyun"
}

// computeSignature 计算阿里云API签名
func (a *AliyunTTS) computeSignature(params map[string]string, secretKey string) (string, error) {
	// 步骤1：按照参数名称的字典顺序排列参数
	var keys []string
	for k := range params {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	// 步骤2：构造规范化请求字符串
	var canonicalizedQueryString strings.Builder
	for _, k := range keys {
		canonicalizedQueryString.WriteString("&")
		canonicalizedQueryString.WriteString(url.QueryEscape(k))
		canonicalizedQueryString.WriteString("=")
		canonicalizedQueryString.WriteString(url.QueryEscape(params[k]))
	}

	// 步骤3：构造待签名字符串
	stringToSign := "GET&" + url.QueryEscape("/") + "&" + url.QueryEscape(canonicalizedQueryString.String()[1:])

	// 步骤4：计算签名
	key := secretKey + "&"
	mac := hmac.New(sha1.New, []byte(key))
	mac.Write([]byte(stringToSign))
	signature := base64.StdEncoding.EncodeToString(mac.Sum(nil))

	return signature, nil
}

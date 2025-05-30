package service

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"time"

	"gopkg.in/yaml.v2"
)

// API接口路径常量
const (
	// 创建笔记接口
	APICreateNote = "/api/open/api/v1/note/create"
	// 编辑笔记接口
	APIEditNote = "/api/open/api/v1/note/edit"
	// 设置笔记接口
	APISetNote = "/api/open/api/v1/note/set"
)

// 基础URL常量
const (
	// 墨问API基础URL
	BaseURL = "https://open.mowen.cn"
)

// Config 配置文件结构
type Config struct {
	APIKey string `yaml:"api_key"`
}

// MowenClient 墨问API客户端
type MowenClient struct {
	APIKey  string
	BaseURL string
	Client  *http.Client
}

// NewMowenClient 创建新的墨问客户端
// 从配置文件中读取API密钥
func NewMowenClient() (client *MowenClient, err error) {
	// 捕获panic并转换为error
	defer func() {
		if r := recover(); r != nil {
			client = nil
			err = fmt.Errorf("创建墨问客户端时发生panic: %v", r)
		}
	}()

	// 读取配置文件
	apiKey, err := loadAPIKeyFromConfig()
	if err != nil {
		return nil, fmt.Errorf("加载API密钥失败: %w", err)
	}

	return &MowenClient{
		APIKey:  apiKey,
		BaseURL: BaseURL,
		Client: &http.Client{
			Timeout: 30 * time.Second,
		},
	}, nil
}

// loadAPIKeyFromConfig 从配置文件加载API密钥
func loadAPIKeyFromConfig() (apiKey string, err error) {
	// 捕获panic并转换为error
	defer func() {
		if r := recover(); r != nil {
			apiKey = ""
			err = fmt.Errorf("加载API密钥时发生panic: %v", r)
		}
	}()

	// 尝试读取config.yml文件
	wd, err := os.Getwd()
	if err != nil {
		return "", fmt.Errorf("获取当前工作目录失败: %w", err)
	}
	configFile := fmt.Sprintf("%s/config.yml", wd)
	data, err := os.ReadFile(configFile)
	if err != nil {
		return "", fmt.Errorf("读取配置文件失败: %w", err)
	}

	var config Config
	if err := yaml.Unmarshal(data, &config); err != nil {
		return "", fmt.Errorf("解析配置文件失败: %w", err)
	}

	if config.APIKey == "" {
		return "", fmt.Errorf("配置文件中未找到api_key")
	}

	return config.APIKey, nil
}

// APIResponse 通用API响应结构
type APIResponse struct {
	StatusCode int                    `json:"status_code"`
	Body       map[string]interface{} `json:"body"`
	RawBody    string                 `json:"raw_body"`
}

// PostRequest 发送POST请求到指定路径
// 参数:
// - path: API路径（相对于BaseURL）
// - payload: 请求体数据
// 返回:
// - APIResponse: 包含状态码和响应体的结构
// - error: 错误信息
func (c *MowenClient) PostRequest(path string, payload interface{}) (*APIResponse, error) {
	// 构建完整的请求URL
	apiURL, err := url.JoinPath(c.BaseURL, path)
	if err != nil {
		return nil, fmt.Errorf("构建URL失败: %w", err)
	}

	// 序列化请求体
	var jsonData []byte
	if payload != nil {
		jsonData, err = json.Marshal(payload)
		if err != nil {
			return nil, fmt.Errorf("序列化请求体失败: %w", err)
		}
	}

	// 创建HTTP请求
	req, err := http.NewRequest("POST", apiURL, bytes.NewBuffer(jsonData))
	if err != nil {
		return nil, fmt.Errorf("创建请求失败: %w", err)
	}

	// 设置请求头
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", c.APIKey))
	req.Header.Set("Content-Type", "application/json")

	// 发送请求
	resp, err := c.Client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("发送请求失败: %w", err)
	}
	defer resp.Body.Close()

	// 读取响应体
	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("读取响应失败: %w", err)
	}

	// 构建响应结构
	apiResponse := &APIResponse{
		StatusCode: resp.StatusCode,
		RawBody:    string(respBody),
	}

	// 尝试解析JSON响应体
	if len(respBody) > 0 {
		var jsonBody map[string]interface{}
		if err := json.Unmarshal(respBody, &jsonBody); err == nil {
			apiResponse.Body = jsonBody
		}
	}

	return apiResponse, nil
}

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
)

// API接口路径常量
const (
	// 创建笔记接口
	APICreateNote = "/api/open/api/v1/note/create"
	// 编辑笔记接口
	APIEditNote = "/api/open/api/v1/note/edit"
	// 设置笔记接口
	APISetNote = "/api/open/api/v1/note/set"
	// 获取上传授权信息接口
	APIUploadPrepare = "/api/open/api/v1/upload/prepare"
)

// 基础URL常量
const (
	// 墨问API基础URL
	BaseURL = "https://open.mowen.cn"
	// 环境变量名称
	APIKeyEnvVar = "MOWEN_API_KEY"
)

// MowenClient 墨问API客户端
type MowenClient struct {
	APIKey  string
	BaseURL string
	Client  *http.Client
}

// NewMowenClient 创建新的墨问客户端
// 从环境变量中读取API密钥
func NewMowenClient() (client *MowenClient, err error) {
	// 捕获panic并转换为error
	defer func() {
		if r := recover(); r != nil {
			client = nil
			err = fmt.Errorf("创建墨问客户端时发生panic: %v", r)
		}
	}()

	// 从环境变量读取API密钥
	apiKey, err := loadAPIKeyFromEnv()
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

// loadAPIKeyFromEnv 从环境变量加载API密钥
func loadAPIKeyFromEnv() (apiKey string, err error) {
	// 捕获panic并转换为error
	defer func() {
		if r := recover(); r != nil {
			apiKey = ""
			err = fmt.Errorf("加载API密钥时发生panic: %v", r)
		}
	}()

	// 从环境变量获取API密钥
	apiKey = os.Getenv(APIKeyEnvVar)
	if apiKey == "" {
		return "", fmt.Errorf("环境变量 %s 未设置或为空", APIKeyEnvVar)
	}

	return apiKey, nil
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
		// 打印请求体用于调试
		fmt.Printf("发送的请求体: %s\n", string(jsonData))
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

// UploadPrepareRequest 获取上传授权信息请求结构
type UploadPrepareRequest struct {
	FileType int    `json:"fileType"`           // 文件类型：1-图片 2-音频 3-PDF
	FileName string `json:"fileName,omitempty"` // 文件名称：可选（未填时，系统生成）
}

// UploadPrepareResponseForm 获取上传授权信息响应中的表单结构
// 根据用户提供的截图，form是一个map[string]string
type UploadPrepareResponseForm map[string]string

// UploadPrepareResponseData 获取上传授权信息响应中的数据结构
// 假设响应直接是 { "form": { ... } }
// 如果有其他层级，比如 { "data": { "form": { ... } } }，则需要调整
type UploadPrepareResponseData struct {
	Form UploadPrepareResponseForm `json:"form"`
}

// UploadPrepareResponse 获取上传授权信息响应结构
type UploadPrepareResponse struct {
	Form UploadPrepareResponseForm `json:"form"`
}

// UploadPrepare 获取上传授权信息
// 参数:
// - payload: 请求体数据，类型为 UploadPrepareRequest
// 返回:
// - *UploadPrepareResponse: 获取上传授权信息的响应体
// - error: 错误信息
func (c *MowenClient) UploadPrepare(payload *UploadPrepareRequest) (*UploadPrepareResponse, error) {
	apiResponse, err := c.PostRequest(APIUploadPrepare, payload)
	if err != nil {
		return nil, fmt.Errorf("获取上传授权信息失败: %w", err)
	}

	if apiResponse.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("获取上传授权信息API请求失败，状态码: %d, 响应: %s", apiResponse.StatusCode, apiResponse.RawBody)
	}

	var uploadPrepareResponse UploadPrepareResponse
	// 直接从RawBody解析，因为APIResponse.Body是 map[string]interface{}
	// 并且根据截图，响应体直接是 {"form": {...map...}}
	if err := json.Unmarshal([]byte(apiResponse.RawBody), &uploadPrepareResponse); err != nil {
		return nil, fmt.Errorf("解析上传授权信息响应失败: %w. 原始响应: %s", err, apiResponse.RawBody)
	}

	return &uploadPrepareResponse, nil
}

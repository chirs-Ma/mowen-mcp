package service

import (
	"fmt"
	"path/filepath"
	"strings"
)

// ContentBlock 表示输入的内容块结构
type ContentBlock struct {
	Type       string                 `json:"type,omitempty"`        // 段落类型：quote, note, file
	Texts      []TextNode             `json:"texts,omitempty"`       // 文本节点列表
	NoteID     string                 `json:"note_id,omitempty"`     // 内链笔记ID
	FileType   string                 `json:"file_type,omitempty"`   // 文件类型：image, audio, pdf
	SourceType string                 `json:"source_type,omitempty"` // 来源类型：local, url
	SourcePath string                 `json:"source_path,omitempty"` // 文件路径
	Metadata   map[string]interface{} `json:"metadata,omitempty"`    // 元数据
}

// TextNode 表示文本节点结构
type TextNode struct {
	Text      string `json:"text"`                // 文本内容
	Bold      bool   `json:"bold,omitempty"`      // 是否加粗
	Highlight bool   `json:"highlight,omitempty"` // 是否高亮
	Link      string `json:"link,omitempty"`      // 链接地址
}

// MowenContentNode 表示墨问API标准格式的内容节点
type MowenContentNode struct {
	Type    string                 `json:"type"`              // 节点类型
	Content []MowenTextNode        `json:"content,omitempty"` // 文本内容（用于paragraph和quote）
	Attrs   map[string]interface{} `json:"attrs,omitempty"`   // 属性（用于image、audio、pdf、note）
}

// MowenTextNode 表示墨问API标准格式的文本节点
type MowenTextNode struct {
	Type  string     `json:"type"`            // 固定为"text"
	Text  string     `json:"text"`            // 文本内容
	Marks []MarkNode `json:"marks,omitempty"` // 标记列表（加粗、高亮、链接等）
}

// MarkNode 表示文本标记
type MarkNode struct {
	Type  string                 `json:"type"`            // 标记类型：bold, highlight, link
	Attrs map[string]interface{} `json:"attrs,omitempty"` // 标记属性（如链接的href）
}

// MowenDocument 表示墨问API标准格式的文档结构
type MowenDocument struct {
	Type    string             `json:"type"`    // 固定为"doc"
	Content []MowenContentNode `json:"content"` // 内容节点列表
}

// ConvertToMowenFormat 将简化格式转换为墨问API标准格式
// 参数:
// - blocks: 输入的内容块列表
// 返回:
// - MowenDocument: 墨问API标准格式的文档
func ConvertToMowenFormat(client *MowenClient, blocks []ContentBlock) (MowenDocument, error) {
	doc := MowenDocument{
		Type:    "doc",
		Content: make([]MowenContentNode, 0),
	}

	for i, block := range blocks {
		// 在每个内容块之间添加空段落（除了第一个）
		if i > 0 {
			doc.Content = append(doc.Content, MowenContentNode{
				Type: "paragraph",
			})
		}

		switch block.Type {
		case "quote":
			// 引用段落
			doc.Content = append(doc.Content, MowenContentNode{
				Type:    "quote",
				Content: convertTextsToMowenFormat(block.Texts),
			})

		case "note":
			// 内链笔记
			doc.Content = append(doc.Content, MowenContentNode{
				Type: "note",
				Attrs: map[string]interface{}{
					"uuid": block.NoteID,
				},
			})

		case "file":
			// 文件段落
			switch block.FileType {
			case "image":
				var fileUUID string
				var err error
				if block.SourceType == "url" {
					fileUUID, err = uploadFileFromURL(client, block.SourcePath, block.FileType, block.SourcePath)
					if err != nil {
						return doc, fmt.Errorf("通过 URL 上传图片文件失败: %w", err)
					}
				} else {
					fileUUID, err = generateFileUUID(client, block.SourcePath)
					if err != nil {
						return doc, fmt.Errorf("上传本地图片文件失败: %w", err)
					}
				}
				attrs := map[string]interface{}{
					"uuid": fileUUID,
				}
				// 添加元数据
				for key, value := range block.Metadata {
					attrs[key] = value
				}
				doc.Content = append(doc.Content, MowenContentNode{
					Type:  "image",
					Attrs: attrs,
				})

			case "audio":
				var fileUUID string
				var err error
				if block.SourceType == "url" {
					fileUUID, err = uploadFileFromURL(client, block.SourcePath, block.FileType, block.SourcePath)
					if err != nil {
						return doc, fmt.Errorf("通过 URL 上传音频文件失败: %w", err)
					}
				} else {
					fileUUID, err = generateFileUUID(client, block.SourcePath)
					if err != nil {
						return doc, fmt.Errorf("上传本地音频文件失败: %w", err)
					}
				}
				attrs := map[string]interface{}{
					"audio-uuid": fileUUID,
				}
				// 添加元数据
				for key, value := range block.Metadata {
					if key == "show_note" {
						attrs["show-note"] = value
					} else {
						attrs[key] = value
					}
				}
				doc.Content = append(doc.Content, MowenContentNode{
					Type:  "audio",
					Attrs: attrs,
				})

			case "pdf":
				var fileUUID string
				var err error
				if block.SourceType == "url" {
					fileUUID, err = uploadFileFromURL(client, block.SourcePath, block.FileType, filepath.Base(block.SourcePath))
					if err != nil {
						return doc, fmt.Errorf("通过 URL 上传PDF文件失败: %w", err)
					}
				} else {
					fileUUID, err = generateFileUUID(client, block.SourcePath)
					if err != nil {
						return doc, fmt.Errorf("上传本地PDF文件失败: %w", err)
					}
				}
				attrs := map[string]interface{}{
					"uuid": fileUUID,
				}
				// 添加元数据
				for key, value := range block.Metadata {
					attrs[key] = value
				}
				doc.Content = append(doc.Content, MowenContentNode{
					Type:  "pdf",
					Attrs: attrs,
				})
			}

		default:
			// 普通段落（默认）
			doc.Content = append(doc.Content, MowenContentNode{
				Type:    "paragraph",
				Content: convertTextsToMowenFormat(block.Texts),
			})
		}
	}

	return doc, nil
}

// uploadFileFromURL 通过 URL 上传文件并返回文件 UUID
func uploadFileFromURL(client *MowenClient, fileURL string, fileTypeStr string, fileName string) (string, error) {
	var apiFileType int
	switch fileTypeStr {
	case "image":
		apiFileType = 1 // 代表图片
	case "audio":
		apiFileType = 2 // 代表音频
	case "pdf":
		apiFileType = 3 // 代表PDF
	default:
		return "", fmt.Errorf("不支持的文件类型: %s", fileTypeStr)
	}

	payload := map[string]interface{}{
		"fileType": apiFileType,
		"url":      fileURL,
		"fileName": fileName,
	}

	resp, err := client.PostRequest(APIUploadFileByURL, payload)
	if err != nil {
		return "", fmt.Errorf("通过 URL 上传文件失败: %w", err)
	}
	if resp.StatusCode != 200 {
		return "", fmt.Errorf("上传文件失败，状态码: %d", resp.StatusCode)
	}

	// 从响应体中提取文件ID
	uploadResp := resp.Body

	fileID, ok := uploadResp["file"].(map[string]interface{})["fileId"].(string)
	if !ok {
		return "", fmt.Errorf("上传文件响应中缺少 'fileId' 字段")
	}

	return fileID, nil
}

// convertTextsToMowenFormat 将文本节点列表转换为墨问格式
func convertTextsToMowenFormat(texts []TextNode) []MowenTextNode {
	result := make([]MowenTextNode, 0, len(texts))

	for _, text := range texts {
		mowenText := MowenTextNode{
			Type:  "text",
			Text:  text.Text,
			Marks: make([]MarkNode, 0),
		}

		// 添加加粗标记
		if text.Bold {
			mowenText.Marks = append(mowenText.Marks, MarkNode{
				Type: "bold",
			})
		}

		// 添加高亮标记
		if text.Highlight {
			mowenText.Marks = append(mowenText.Marks, MarkNode{
				Type: "highlight",
			})
		}

		// 添加链接标记
		if text.Link != "" {
			mowenText.Marks = append(mowenText.Marks, MarkNode{
				Type: "link",
				Attrs: map[string]interface{}{
					"href": text.Link,
				},
			})
		}

		result = append(result, mowenText)
	}

	return result
}

// generateFileUUID 上传文件并获取真实的UUID
func generateFileUUID(client *MowenClient, filePath string) (string, error) {
	// 根据文件扩展名确定文件类型
	fileType, err := getFileTypeFromPath(filePath)
	if err != nil {
		return "", fmt.Errorf("无法确定文件类型: %w", err)
	}

	// 获取上传授权信息
	uploadPrepareReq := &UploadPrepareRequest{
		FileType: fileType,
		FileName: filepath.Base(filePath),
	}

	uploadPrepareResp, err := client.UploadPrepare(uploadPrepareReq)
	if err != nil {
		return "", fmt.Errorf("获取上传授权失败: %w", err)
	}

	// 上传文件
	uploadResp, err := client.UploadFile(uploadPrepareResp.Form, filePath)
	if err != nil {
		return "", fmt.Errorf("文件上传失败: %w", err)
	}

	// 检查上传是否成功
	if uploadResp.StatusCode != 200 && uploadResp.StatusCode != 204 {
		return "", fmt.Errorf("文件上传失败，状态码: %d，响应: %s", uploadResp.StatusCode, uploadResp.RawBody)
	}

	// 从上传响应中提取文件UUID
	var fileUUID string
	if uploadResp.Body != nil {
		fileUUID = uploadResp.Body["file"].(map[string]interface{})["fileId"].(string)
	}

	// 如果仍然没有UUID，返回错误
	if fileUUID == "" {
		return "", fmt.Errorf("无法从上传响应中获取文件UUID，响应: %s", uploadResp.RawBody)
	}

	return fileUUID, nil
}

// getFileTypeFromPath 根据文件路径确定文件类型
func getFileTypeFromPath(filePath string) (int, error) {
	ext := strings.ToLower(filepath.Ext(filePath))

	switch ext {
	case ".jpg", ".jpeg", ".png", ".gif", ".bmp", ".webp":
		return 1, nil // 图片
	case ".mp3", ".wav", ".aac", ".flac", ".ogg", ".m4a":
		return 2, nil // 音频
	case ".pdf":
		return 3, nil // PDF
	default:
		return 0, fmt.Errorf("不支持的文件类型: %s", ext)
	}
}

package service

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

// TextNode 表示富文本中的一个文本节点
type TextNode struct {
	Text      string `json:"text"`
	Bold      bool   `json:"bold,omitempty"`
	Highlight bool   `json:"highlight,omitempty"`
	Link      string `json:"link,omitempty"`
}

// Paragraph 表示一个段落
type Paragraph struct {
	Texts []TextNode `json:"texts"`
}

// PrivacyRule 表示隐私规则
type PrivacyRule struct {
	NoShare  bool   `json:"noShare"`
	ExpireAt string `json:"expireAt"`
}

// CreateNoteParams 创建笔记的参数
type CreateNoteParams struct {
	Paragraphs  []Paragraph `json:"paragraphs"`
	AutoPublish bool        `json:"auto_publish"`
	Tags        []string    `json:"tags"`
}

// EditNoteParams 编辑笔记的参数
type EditNoteParams struct {
	NoteID     string      `json:"note_id"`
	Paragraphs []Paragraph `json:"paragraphs"`
}

// SetNotePrivacyParams 设置笔记隐私的参数
type SetNotePrivacyParams struct {
	NoteID      string `json:"note_id"`
	PrivacyType string `json:"privacy_type"`
	NoShare     bool   `json:"no_share"`
	ExpireAt    int64  `json:"expire_at"`
}

// validateRichNoteParagraphs 验证富文本笔记段落格式
func validateRichNoteParagraphs(paragraphs []Paragraph) error {
	if len(paragraphs) == 0 {
		return fmt.Errorf("段落列表不能为空")
	}

	for i, para := range paragraphs {
		if len(para.Texts) == 0 {
			return fmt.Errorf("第%d个段落的texts字段不能为空", i+1)
		}

		for j, text := range para.Texts {
			if text.Text == "" {
				return fmt.Errorf("第%d个段落第%d个文本节点的text字段不能为空", i+1, j+1)
			}
			if text.Link != "" && !strings.HasPrefix(text.Link, "http") {
				return fmt.Errorf("第%d个段落第%d个文本节点的link必须是有效的URL", i+1, j+1)
			}
		}
	}
	return nil
}

// 创建一篇新的墨问笔记
func CreateNote(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	// 创建墨问客户端
	client, err := NewMowenClient()
	if err != nil {
		return mcp.NewToolResultText(fmt.Sprintf("❌ 创建客户端失败: %v", err)), nil
	}

	// 解析paragraphs参数
	args := request.Params.Arguments
	paragraphsStr, ok := args["paragraphs"].(string)
	if !ok {
		return mcp.NewToolResultText("❌ paragraphs参数必须是JSON字符串"), nil
	}

	var paragraphs []Paragraph
	if err := json.Unmarshal([]byte(paragraphsStr), &paragraphs); err != nil {
		return mcp.NewToolResultText(fmt.Sprintf("❌ paragraphs JSON解析错误: %v", err)), nil
	}

	// 解析其他参数
	autoPublish, _ := args["auto_publish"].(bool)
	tagsStr, _ := args["tags"].(string)
	var tags []string
	if tagsStr != "" {
		if err := json.Unmarshal([]byte(tagsStr), &tags); err != nil {
			tags = []string{} // 如果解析失败，使用空数组
		}
	}

	// 参数验证
	if err := validateRichNoteParagraphs(paragraphs); err != nil {
		errorMsg := fmt.Sprintf(`❌ 参数格式错误！

正确的paragraphs格式示例：
[
    {
        "texts": [
            {"text": "普通文本"},
            {"text": "加粗文本", "bold": true},
            {"text": "高亮文本", "highlight": true},
            {"text": "链接文本", "link": "https://example.com"}
        ]
    }
]

错误详情: %v`, err)
		return mcp.NewToolResultText(errorMsg), nil
	}

	// 构建请求参数
	payload := CreateNoteParams{
		Paragraphs:  paragraphs,
		AutoPublish: autoPublish,
		Tags:        tags,
	}

	// 调用API创建笔记
	resp, err := client.PostRequest(APICreateNote, payload)
	if err != nil {
		return mcp.NewToolResultText(fmt.Sprintf("❌ API请求失败: %v", err)), nil
	}

	// 处理响应
	if resp.StatusCode != 200 {
		return mcp.NewToolResultText(fmt.Sprintf("❌ API请求失败，状态码: %d，响应: %s", resp.StatusCode, resp.RawBody)), nil
	}

	// 解析响应获取笔记ID
	var noteID string
	if resp.Body != nil {
		if data, ok := resp.Body["data"].(map[string]interface{}); ok {
			if id, ok := data["note_id"].(string); ok {
				noteID = id
			}
		}
	}

	if noteID == "" {
		noteID = "未知ID"
	}

	resultText := fmt.Sprintf("✅ 笔记创建成功！\n\n笔记ID: %s\n段落数: %d\n自动发布: %t\n标签: %s",
		noteID, len(paragraphs), autoPublish, strings.Join(tags, ", "))

	return mcp.NewToolResultText(resultText), nil
}

// 编辑已存在的笔记内容
func EditNote(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	// 创建墨问客户端
	client, err := NewMowenClient()
	if err != nil {
		return mcp.NewToolResultText(fmt.Sprintf("❌ 创建客户端失败: %v", err)), nil
	}

	// 解析参数
	args := request.Params.Arguments
	noteID, ok := args["note_id"].(string)
	if !ok || noteID == "" {
		return mcp.NewToolResultText("❌ 笔记ID不能为空"), nil
	}

	paragraphsStr, ok := args["paragraphs"].(string)
	if !ok {
		return mcp.NewToolResultText("❌ paragraphs参数必须是JSON字符串"), nil
	}

	var paragraphs []Paragraph
	if err := json.Unmarshal([]byte(paragraphsStr), &paragraphs); err != nil {
		return mcp.NewToolResultText(fmt.Sprintf("❌ paragraphs JSON解析错误: %v", err)), nil
	}

	// 参数验证
	if err := validateRichNoteParagraphs(paragraphs); err != nil {
		errorMsg := fmt.Sprintf(`❌ 参数格式错误！

正确的paragraphs格式示例：
[
    {
        "texts": [
            {"text": "普通文本"},
            {"text": "加粗文本", "bold": true},
            {"text": "高亮文本", "highlight": true},
            {"text": "链接文本", "link": "https://example.com"}
        ]
    }
]

错误详情: %v`, err)
		return mcp.NewToolResultText(errorMsg), nil
	}

	// 构建请求参数
	payload := EditNoteParams{
		NoteID:     noteID,
		Paragraphs: paragraphs,
	}

	// 调用API编辑笔记
	resp, err := client.PostRequest(APIEditNote, payload)
	if err != nil {
		return mcp.NewToolResultText(fmt.Sprintf("❌ API请求失败: %v", err)), nil
	}

	// 处理响应
	if resp.StatusCode != 200 {
		return mcp.NewToolResultText(fmt.Sprintf("❌ API请求失败，状态码: %d，响应: %s", resp.StatusCode, resp.RawBody)), nil
	}

	resultText := fmt.Sprintf("✅ 笔记编辑成功！\n\n笔记ID: %s\n段落数: %d",
		noteID, len(paragraphs))

	return mcp.NewToolResultText(resultText), nil
}

// 设置笔记的隐私权限
func SetNotePrivacy(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	// 创建墨问客户端
	client, err := NewMowenClient()
	if err != nil {
		return mcp.NewToolResultText(fmt.Sprintf("❌ 创建客户端失败: %v", err)), nil
	}

	// 解析参数
	args := request.Params.Arguments
	noteID, ok := args["note_id"].(string)
	if !ok || noteID == "" {
		return mcp.NewToolResultText("❌ 笔记ID不能为空"), nil
	}

	privacyType, ok := args["privacy_type"].(string)
	if !ok {
		return mcp.NewToolResultText("❌ 隐私类型不能为空"), nil
	}

	noShare, _ := args["no_share"].(bool)
	expireAt, _ := args["expire_at"].(float64) // JSON数字默认为float64

	// 参数验证
	validPrivacyTypes := map[string]string{
		"public":  "完全公开",
		"private": "私有",
		"rule":    "规则公开",
	}

	privacyDesc, valid := validPrivacyTypes[privacyType]
	if !valid {
		return mcp.NewToolResultText("❌ 隐私类型必须是 'public', 'private' 或 'rule'"), nil
	}

	// 构建请求参数
	payload := SetNotePrivacyParams{
		NoteID:      noteID,
		PrivacyType: privacyType,
		NoShare:     noShare,
		ExpireAt:    int64(expireAt),
	}

	// 调用API设置笔记隐私
	resp, err := client.PostRequest(APISetNote, payload)
	if err != nil {
		return mcp.NewToolResultText(fmt.Sprintf("❌ API请求失败: %v", err)), nil
	}

	// 处理响应
	if resp.StatusCode != 200 {
		return mcp.NewToolResultText(fmt.Sprintf("❌ API请求失败，状态码: %d，响应: %s", resp.StatusCode, resp.RawBody)), nil
	}

	responseText := fmt.Sprintf("✅ 笔记隐私设置成功！\n\n笔记ID: %s\n隐私类型: %s",
		noteID, privacyDesc)

	if privacyType == "rule" {
		responseText += fmt.Sprintf("\n禁止分享: %s", map[bool]string{true: "是", false: "否"}[noShare])
		if expireAt == 0 {
			responseText += "\n有效期: 永久"
		} else {
			responseText += fmt.Sprintf("\n过期时间戳: %.0f", expireAt)
		}
	}

	return mcp.NewToolResultText(responseText), nil
}

// 所有墨问相关的MCP工具
// 创建笔记工具
var CreateNoteTool = mcp.NewTool("create_note",
	mcp.WithDescription("创建一篇新的墨问笔记。支持富文本格式，包括加粗、高亮、链接等格式。可以设置自动发布和标签。"),
	mcp.WithString("paragraphs",
		mcp.Required(),
		mcp.Description("富文本段落列表JSON字符串，每个段落包含多个文本节点。格式：[{\"texts\": [{\"text\": \"内容\", \"bold\": true, \"highlight\": true, \"link\": \"https://example.com\"}]}]"),
	),
	mcp.WithBoolean("auto_publish",
		mcp.Description("是否自动发布笔记。true表示立即发布，false表示保存为草稿"),
	),
	mcp.WithString("tags",
		mcp.Description("笔记标签列表JSON字符串，例如：['工作', '学习', '重要']"),
	),
)

// 编辑笔记工具
var EditNoteTool = mcp.NewTool("edit_note",
	mcp.WithDescription("编辑已存在的笔记内容。此操作会完全替换笔记的原有内容，而不是追加内容。"),
	mcp.WithString("note_id",
		mcp.Required(),
		mcp.Description("要编辑的笔记ID，通常是创建笔记时返回的ID"),
	),
	mcp.WithString("paragraphs",
		mcp.Required(),
		mcp.Description("富文本段落列表JSON字符串，每个段落包含多个文本节点。将完全替换原有笔记内容。"),
	),
)

// 设置笔记隐私工具
var SetNotePrivacyTool = mcp.NewTool("set_note_privacy",
	mcp.WithDescription("设置笔记的隐私权限。支持三种模式：完全公开(public)、私有(private)、规则公开(rule)。"),
	mcp.WithString("note_id",
		mcp.Required(),
		mcp.Description("笔记ID"),
	),
	mcp.WithString("privacy_type",
		mcp.Required(),
		mcp.Description("隐私类型：'public'(完全公开)、'private'(私有)、'rule'(规则公开)"),
	),
	mcp.WithBoolean("no_share",
		mcp.Description("当privacy_type为'rule'时，是否禁止分享。true表示禁止分享，false表示允许分享"),
	),
	mcp.WithNumber("expire_at",
		mcp.Description("当privacy_type为'rule'时，过期时间戳（Unix时间戳）。0表示永不过期"),
	),
)

// 适配器函数，将我们的函数签名转换为 ToolHandlerFunc 期望的签名
func createNoteHandler(arguments map[string]interface{}) (*mcp.CallToolResult, error) {
	request := mcp.CallToolRequest{}
	request.Params.Arguments = arguments
	return CreateNote(context.Background(), request)
}

func editNoteHandler(arguments map[string]interface{}) (*mcp.CallToolResult, error) {
	request := mcp.CallToolRequest{}
	request.Params.Arguments = arguments
	return EditNote(context.Background(), request)
}

func setNotePrivacyHandler(arguments map[string]interface{}) (*mcp.CallToolResult, error) {
	request := mcp.CallToolRequest{}
	request.Params.Arguments = arguments
	return SetNotePrivacy(context.Background(), request)
}

func RegisterAllTools(s *server.MCPServer) {
	s.AddTool(CreateNoteTool, createNoteHandler)
	s.AddTool(EditNoteTool, editNoteHandler)
	s.AddTool(SetNotePrivacyTool, setNotePrivacyHandler)
}

package service

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/bytedance/gopkg/util/logger"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

// TextNode 表示富文本中的一个文本节点

// CreateNoteParams 创建笔记的参数
type CreateNoteParams struct {
	Body     *MowenDocument `json:"body,omitempty"`
	Settings *Settings      `json:"settings,omitempty"`
}

// Settings 笔记设置
type Settings struct {
	AutoPublish *bool    `json:"autoPublish,omitempty"`
	Tags        []string `json:"tags,omitempty"`
	Privacy     *string  `json:"privacy,omitempty"`
	NoShare     *bool    `json:"noShare,omitempty"`
	ExpireAt    *int64   `json:"expireAt,omitempty"`
}

// EditNoteParams 编辑笔记的参数
type EditNoteParams struct {
	NoteID     string          `json:"note_id"`
	Paragraphs []MowenDocument `json:"paragraphs"`
}

// PrivacyRule 隐私规则
type PrivacyRule struct {
	NoShare  bool   `json:"noShare"`
	ExpireAt string `json:"expireAt"`
}

// Privacy 隐私设置
type Privacy struct {
	Type string       `json:"type"`
	Rule *PrivacyRule `json:"rule,omitempty"`
}

// SettingsForPrivacy 用于设置隐私的Settings结构
type SetNotePrivacyParams struct {
	NoteID   string `json:"noteId"`
	Section  int    `json:"section"`
	Settings struct {
		Privacy Privacy `json:"privacy"`
	} `json:"settings"`
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

	var blocks []ContentBlock
	if err = json.Unmarshal([]byte(paragraphsStr), &blocks); err != nil {
		return mcp.NewToolResultText(fmt.Sprintf("❌ paragraphs JSON解析错误: %v", err)), nil
	}

	// 解析其他参数
	autoPublish, _ := args["auto_publish"].(bool)
	tagsStr, _ := args["tags"].(string)
	var tags []string
	if tagsStr != "" {
		if err = json.Unmarshal([]byte(tagsStr), &tags); err != nil {
			tags = []string{} // 如果解析失败，使用空数组
		}
	}

	// 参数验证
	if len(blocks) == 0 {
		return mcp.NewToolResultText("❌ 段落列表不能为空"), nil
	}

	// 使用ConvertToMowenFormat函数进行数据转换
	mowenDoc, err := ConvertToMowenFormat(client, blocks)
	if err != nil {
		return mcp.NewToolResultText(fmt.Sprintf("❌ 转换文档格式失败: %v", err)), nil
	}

	// 构建设置
	settings := &Settings{
		AutoPublish: &autoPublish,
		Tags:        tags,
	}

	payload := CreateNoteParams{
		Body:     &mowenDoc,
		Settings: settings,
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
		if id, ok := resp.Body["noteId"].(string); ok {
			noteID = id
		}
	}

	if noteID == "" {
		noteID = "未知ID"
	}
	go func() {
		// 存入数据库
		summary := ""
		if success, err := SaveNoteToSQLite(noteID, paragraphsStr, summary); !success {
			logger.Info("保存笔记到数据库失败", "error", err, "noteID", noteID)
		} else {
			logger.Info("笔记已成功保存到数据库", "noteID", noteID)
		}
	}()

	resultText := fmt.Sprintf("✅ 笔记创建成功！\n\n笔记ID: %s\n段落数: %d\n自动发布: %t\n标签: %s",
		noteID, len(blocks), autoPublish, strings.Join(tags, ", "))

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

	var blocks []ContentBlock
	if err = json.Unmarshal([]byte(paragraphsStr), &blocks); err != nil {
		return mcp.NewToolResultText(fmt.Sprintf("❌ paragraphs JSON解析错误: %v", err)), nil
	}

	// 参数验证
	if len(blocks) == 0 {
		return mcp.NewToolResultText("❌ 段落列表不能为空"), nil
	}

	// 使用ConvertToMowenFormat函数进行数据转换
	mowenDoc, err := ConvertToMowenFormat(client, blocks)
	if err != nil {
		return mcp.NewToolResultText(fmt.Sprintf("❌ 转换文档格式失败: %v", err)), nil
	}

	// 构建请求参数
	payload := EditNoteParams{
		NoteID:     noteID,
		Paragraphs: []MowenDocument{mowenDoc},
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
		noteID, len(blocks))

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
	privacy := Privacy{
		Type: privacyType,
	}

	// 如果是规则公开，添加规则设置
	if privacyType == "rule" {
		expireAtStr := fmt.Sprintf("%d", int64(expireAt))
		privacy.Rule = &PrivacyRule{
			NoShare:  noShare,
			ExpireAt: expireAtStr,
		}
	}

	settings := struct {
		Privacy Privacy `json:"privacy"`
	}{
		Privacy: privacy,
	}

	payload := SetNotePrivacyParams{
		NoteID:   noteID,
		Section:  1,
		Settings: settings,
	}

	// 调用API设置笔记隐私
	resp, err := client.PostRequest(APISetNote, payload)
	if err != nil {
		return mcp.NewToolResultText(fmt.Sprintf("❌ API请求失败: %v", err)), nil
	}

	// 处理响应
	if resp.StatusCode != 200 {
		requestStr, _ := json.Marshal(payload)
		return mcp.NewToolResultText(fmt.Sprintf("❌ API请求失败，状态码: %d，响应: %s，请求参数：%s", resp.StatusCode, resp.RawBody, requestStr)), nil
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

// 分析笔记内容
// SearchNote 查询笔记功能
func SearchNote(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	// 解析请求参数
	var queryType string
	var startDate, endDate string
	var specificDate string

	if queryTypeArg, exists := request.Params.Arguments["query_type"]; exists {
		if qt, ok := queryTypeArg.(string); ok {
			queryType = qt
		}
	}

	if startDateArg, exists := request.Params.Arguments["start_date"]; exists {
		if sd, ok := startDateArg.(string); ok {
			startDate = sd
		}
	}

	if endDateArg, exists := request.Params.Arguments["end_date"]; exists {
		if ed, ok := endDateArg.(string); ok {
			endDate = ed
		}
	}

	if specificDateArg, exists := request.Params.Arguments["specific_date"]; exists {
		if sd, ok := specificDateArg.(string); ok {
			specificDate = sd
		}
	}

	nowDate := time.Now()
	var results []NoteRecord
	var err error

	// 根据查询类型执行不同的查询
	switch queryType {
	case "specific_date":
		// 查询特定日期的笔记
		if specificDate == "" {
			specificDate = nowDate.Format("2006-01-02")
		}
		results, err = SearchByDate(specificDate)

	case "date_range":
		// 查询日期范围内的笔记
		if startDate == "" || endDate == "" {
			return mcp.NewToolResultError("日期范围查询需要提供开始日期和结束日期"), nil
		}
		results, err = SearchByDateRange(startDate, endDate)

	case "this_week":
		// 查询本周的笔记
		weekday := int(nowDate.Weekday())
		if weekday == 0 { // Sunday
			weekday = 7
		}
		startOfWeek := nowDate.AddDate(0, 0, -(weekday - 1))
		endOfWeek := startOfWeek.AddDate(0, 0, 6)
		results, err = SearchByDateRange(
			startOfWeek.Format("2006-01-02"),
			endOfWeek.Format("2006-01-02"),
		)

	case "this_month":
		// 查询本月的笔记
		startOfMonth := time.Date(nowDate.Year(), nowDate.Month(), 1, 0, 0, 0, 0, nowDate.Location())
		endOfMonth := startOfMonth.AddDate(0, 1, -1)
		results, err = SearchByDateRange(
			startOfMonth.Format("2006-01-02"),
			endOfMonth.Format("2006-01-02"),
		)

	case "last_week":
		// 查询上周的笔记
		weekday := int(nowDate.Weekday())
		if weekday == 0 {
			weekday = 7
		}
		startOfLastWeek := nowDate.AddDate(0, 0, -(weekday - 1 + 7))
		endOfLastWeek := startOfLastWeek.AddDate(0, 0, 6)
		results, err = SearchByDateRange(
			startOfLastWeek.Format("2006-01-02"),
			endOfLastWeek.Format("2006-01-02"),
		)

	case "last_month":
		// 查询上月的笔记
		startOfLastMonth := time.Date(nowDate.Year(), nowDate.Month()-1, 1, 0, 0, 0, 0, nowDate.Location())
		endOfLastMonth := startOfLastMonth.AddDate(0, 1, -1)
		results, err = SearchByDateRange(
			startOfLastMonth.Format("2006-01-02"),
			endOfLastMonth.Format("2006-01-02"),
		)

	case "today":
		// 查询今天的笔记
		results, err = SearchByDate(nowDate.Format("2006-01-02"))

	case "yesterday":
		// 查询昨天的笔记
		yesterday := nowDate.AddDate(0, 0, -1)
		results, err = SearchByDate(yesterday.Format("2006-01-02"))

	default:
		// 默认查询今天的笔记
		results, err = SearchByDate(nowDate.Format("2006-01-02"))
	}

	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("查询笔记失败: %v", err)), nil
	}

	// 格式化查询结果
	if len(results) == 0 {
		return mcp.NewToolResultText("📝 未找到符合条件的笔记"), nil
	}

	var resultText strings.Builder
	resultText.WriteString(fmt.Sprintf("📝 找到 %d 条笔记:\n\n", len(results)))

	for i, note := range results {
		resultText.WriteString(fmt.Sprintf("**%d. 笔记 %s**\n", i+1, note.NoteID))
		resultText.WriteString(fmt.Sprintf("创建时间: %s\n", note.CreatedAt))

		// 显示内容摘要（前100个字符）
		content := note.Content
		if len(content) > 100 {
			content = content[:100] + "..."
		}
		resultText.WriteString(fmt.Sprintf("内容摘要: %s\n", content))

		if note.Summary != "" {
			resultText.WriteString(fmt.Sprintf("总结: %s\n", note.Summary))
		}

		resultText.WriteString("\n")
	}

	return mcp.NewToolResultText(resultText.String()), nil
}

// 所有墨问相关的MCP工具
// 创建笔记工具
var CreateNoteTool = mcp.NewTool("create_note",
	mcp.WithDescription("创建一篇新的墨问笔记。支持多种内容块，包括段落、引用、图片、音频、PDF和内嵌笔记。可以设置自动发布和标签。"),
	mcp.WithString("paragraphs",
		mcp.Required(),
		mcp.Description(`
		富文本段落列表，每个段落包含多个文本节点。支持文本、引用、内链笔记和文件。
        
        段落类型：
        1. 普通段落（默认）：{"texts": [...]}
        2. 引用段落：{"type": "quote", "texts": [...]}
        3. 内链笔记：{"type": "note", "note_id": "笔记ID"}
        4. 文件段落：{"type": "file", "file_type": "image|audio|pdf", "source_type": "local|url", "source_path": "路径", "metadata": {...}}
        
        格式示例：
        [
            {
                "texts": [
                    {"text": "这是普通文本"},
                    {"text": "这是加粗文本", "bold": true},
                    {"text": "这是高亮文本", "highlight": true},
                    {"text": "这是链接", "link": "https://example.com"}
                ]
            },
            {
                "type": "quote",
                "texts": [
                    {"text": "这是引用段落"},
                    {"text": "支持富文本", "bold": true}
                ]
            },
            {
                "type": "note",
                "note_id": "VPrWsE_-P0qwrFUOygGs8"
            },
            {
                "type": "file",
                "file_type": "image",
                "source_type": "local",
                "source_path": "/path/to/image.jpg",
                "metadata": {
                    "alt": "图片描述",
                    "align": "center"
                }
            },
            {
                "type": "file",
                "file_type": "audio",
                "source_type": "url",
                "source_path": "https://example.com/audio.mp3",
                "metadata": {
                    "show_note": "00:00 开场\\n01:30 主要内容"
                }
            },
            {
                "texts": [
                    {"text": "第二段内容"}
                ]
            }
        ]
		`),
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
	mcp.WithDescription("编辑已存在的笔记内容。此操作会完全替换笔记的原有内容。支持多种内容块。"),
	mcp.WithString("note_id",
		mcp.Required(),
		mcp.Description("要编辑的笔记ID"),
	),
	mcp.WithString("paragraphs",
		mcp.Required(),
		mcp.Description("新的内容块列表JSON字符串。将完全替换原有笔记内容。"),
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

// 搜索笔记工具
var SearchNoteTool = mcp.NewTool("search_note",
	mcp.WithDescription("查询笔记功能，支持多种时间查询模式：特定日期、日期范围、今天、昨天、本周、本月、上周、上月等"),
	mcp.WithString("query_type",
		mcp.Description("查询类型：specific_date(特定日期)、date_range(日期范围)、 today(今天)、yesterday(昨天)、this_week(本周)、this_month(本月)、last_week(上周)、last_month(上月)"),
	),
	mcp.WithString("specific_date",
		mcp.Description("特定日期，格式：YYYY-MM-DD，用于specific_date查询类型"),
	),
	mcp.WithString("start_date",
		mcp.Description("开始日期，格式：YYYY-MM-DD，用于date_range查询类型"),
	),
	mcp.WithString("end_date",
		mcp.Description("结束日期，格式：YYYY-MM-DD，用于date_range查询类型"),
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

func searchNoteHandler(arguments map[string]interface{}) (*mcp.CallToolResult, error) {
	request := mcp.CallToolRequest{}
	request.Params.Arguments = arguments
	return SearchNote(context.Background(), request)
}

func RegisterAllTools(s *server.MCPServer) {
	s.AddTool(CreateNoteTool, createNoteHandler)
	s.AddTool(EditNoteTool, editNoteHandler)
	s.AddTool(SetNotePrivacyTool, setNotePrivacyHandler)
	s.AddTool(SearchNoteTool, searchNoteHandler)
}

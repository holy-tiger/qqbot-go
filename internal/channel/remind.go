package channel

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strings"

	"github.com/mark3labs/mcp-go/mcp"
)

// remindRequest is the JSON body for the POST /reminders API.
type remindRequest struct {
	Content       string `json:"content"`
	TargetType    string `json:"target_type"`
	TargetAddress string `json:"target_address"`
	Schedule      string `json:"schedule,omitempty"`
}

// remindResponse is the JSON response from the POST /reminders API.
type remindResponse struct {
	JobID    string `json:"job_id"`
	NextRun  string `json:"next_run"`
	Schedule string `json:"schedule"`
}

// apiResponse wraps the qqbot API standard response envelope.
type apiResponse struct {
	OK   bool            `json:"ok"`
	Data *remindResponse `json:"data"`
}

// handleRemind creates a scheduled reminder via the qqbot HTTP API.
func (cs *ChannelServer) handleRemind(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	chatID, err := request.RequireString("chat_id")
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}
	text, err := request.RequireString("text")
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	chatType, targetID, err := parseChatID(chatID)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}
	if chatType != "c2c" && chatType != "group" {
		return mcp.NewToolResultError(fmt.Sprintf("定时提醒不支持 %s 类型，仅支持 c2c 和 group", chatType)), nil
	}

	args := request.GetArguments()
	schedule, _ := args["schedule"].(string)

	body, err := json.Marshal(remindRequest{
		Content:       text,
		TargetType:    chatType,
		TargetAddress: targetID,
		Schedule:      schedule,
	})
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("构建请求失败: %v", err)), nil
	}

	url := fmt.Sprintf("%s/api/v1/accounts/%s/reminders", cs.config.QQBotAPI, cs.config.Account)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, strings.NewReader(string(body)))
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("创建请求失败: %v", err)), nil
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		log.Printf("[channel] remind error: %v", err)
		return mcp.NewToolResultError(fmt.Sprintf("创建提醒失败: %v", err)), nil
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		log.Printf("[channel] remind HTTP %d", resp.StatusCode)
		return mcp.NewToolResultError(fmt.Sprintf("创建提醒失败: HTTP %d", resp.StatusCode)), nil
	}

	var result apiResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		log.Printf("[channel] remind decode error: %v", err)
		return mcp.NewToolResultError(fmt.Sprintf("解析响应失败: %v", err)), nil
	}

	if result.Data == nil {
		log.Printf("[channel] remind response missing data field")
		return mcp.NewToolResultError("创建提醒失败: 响应缺少数据"), nil
	}

	log.Printf("[channel] reminded %s (job_id=%s, schedule=%s)", chatID, result.Data.JobID, schedule)
	return mcp.NewToolResultText(fmt.Sprintf("reminded (job_id=%s)", result.Data.JobID)), nil
}

// handleCancelReminder cancels a scheduled reminder via the qqbot HTTP API.
func (cs *ChannelServer) handleCancelReminder(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	jobID, err := request.RequireString("job_id")
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	url := fmt.Sprintf("%s/api/v1/accounts/%s/reminders/%s", cs.config.QQBotAPI, cs.config.Account, jobID)
	req, err := http.NewRequestWithContext(ctx, http.MethodDelete, url, nil)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("创建请求失败: %v", err)), nil
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		log.Printf("[channel] cancel_reminder error: %v", err)
		return mcp.NewToolResultError(fmt.Sprintf("取消提醒失败: %v", err)), nil
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		log.Printf("[channel] cancel_reminder HTTP %d", resp.StatusCode)
		return mcp.NewToolResultError(fmt.Sprintf("取消提醒失败: HTTP %d", resp.StatusCode)), nil
	}

	log.Printf("[channel] cancelled reminder %s", jobID)
	return mcp.NewToolResultText("cancelled"), nil
}

// registerRemindTools registers the remind and cancel_reminder tools on the MCP server.
func (cs *ChannelServer) registerRemindTools() {
	if cs.mcp == nil {
		return
	}
	remindTool := mcp.NewTool("remind",
		mcp.WithDescription("设置定时提醒。到时间后通过 qqbot 向指定会话发送文本消息。不设置 schedule 则立即发送一次。仅支持 c2c 和 group 类型会话。"),
		mcp.WithString("chat_id",
			mcp.Required(),
			mcp.Description("会话 ID，格式: c2c:user_openid (私聊) 或 group:group_openid (群聊)"),
		),
		mcp.WithString("text",
			mcp.Required(),
			mcp.Description("提醒内容文本"),
		),
		mcp.WithString("schedule",
			mcp.Description("定时规则。支持 @every 30m (间隔) 或 5 字段 cron 表达式。不设置则立即发送一次"),
		),
	)
	cs.mcp.AddTool(remindTool, cs.handleRemind)

	cancelTool := mcp.NewTool("cancel_reminder",
		mcp.WithDescription("取消一个定时提醒"),
		mcp.WithString("job_id",
			mcp.Required(),
			mcp.Description("提醒任务 ID（由 remind 工具返回）"),
		),
	)
	cs.mcp.AddTool(cancelTool, cs.handleCancelReminder)
}

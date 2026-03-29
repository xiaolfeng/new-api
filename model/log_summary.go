package model

import (
	"strings"

	"github.com/QuantumNous/new-api/common"
)

const (
	LogOtherClientSourceKey    = "client_source"
	LogOtherInteractionTypeKey = "interaction_type"
)

type responsesPromptInputItem struct {
	Type string
	Role string
	Text string
}

func AppendLogDetailSummaries(other map[string]interface{}, record string) map[string]interface{} {
	if other == nil {
		other = make(map[string]interface{})
	}

	source, interactionType := ExtractLogDetailSummaries(record)
	if source != "" {
		other[LogOtherClientSourceKey] = source
	}
	if interactionType != "" {
		other[LogOtherInteractionTypeKey] = interactionType
	}
	return other
}

func ExtractLogDetailSummaries(record string) (string, string) {
	if strings.TrimSpace(record) == "" {
		return "", ""
	}

	var detailRecord LogDetailRecord
	if err := common.UnmarshalJsonStr(record, &detailRecord); err != nil {
		return "", ""
	}

	return parseClientSourceFromHeaders(detailRecord.Headers), parseInteractionTypeFromDetailRecord(&detailRecord)
}

func IsDeveloperToolLogSource(source string) bool {
	switch strings.TrimSpace(source) {
	case "Claude Code", "Codex", "OpenCode":
		return true
	default:
		return false
	}
}

func CanViewDeveloperToolLogDetail(userRole int) bool {
	return userRole == common.RoleCodeUser || userRole >= common.RoleAdminUser
}

func parseClientSourceFromHeaders(headers map[string]string) string {
	if len(headers) == 0 {
		return ""
	}

	userAgent := getHeaderIgnoreCase(headers, "user-agent")
	if userAgent == "" {
		userAgent = getHeaderIgnoreCase(headers, "originator")
	}

	return parseClientSource(userAgent)
}

func getHeaderIgnoreCase(headers map[string]string, target string) string {
	target = strings.ToLower(strings.TrimSpace(target))
	for key, value := range headers {
		if strings.ToLower(strings.TrimSpace(key)) == target {
			return value
		}
	}
	return ""
}

func parseClientSource(userAgent string) string {
	if strings.TrimSpace(userAgent) == "" {
		return ""
	}

	ua := strings.ToLower(userAgent)

	switch {
	case strings.Contains(ua, "claude-cli"), strings.Contains(ua, "claudecode"):
		return "Claude Code"
	case strings.Contains(ua, "codex_cli_rs"), strings.Contains(ua, "codex-cli-rs"):
		return "Codex"
	case strings.Contains(ua, "cherrystudio/"):
		return "Cherry Studio"
	case strings.Contains(ua, "cursor/"):
		return "Cursor"
	case strings.Contains(ua, "windsurf/"), strings.Contains(ua, "codeium/"):
		return "Windsurf"
	case strings.Contains(ua, "continue/"):
		return "Continue"
	case strings.Contains(ua, "github-copilot"), strings.Contains(ua, "copilot/"):
		return "Copilot"
	case strings.Contains(ua, "cline/"), strings.Contains(ua, "cline-vscode"):
		return "Cline"
	case strings.Contains(ua, "roo-cline"), strings.Contains(ua, "roocode"), strings.Contains(ua, "roo code"):
		return "Roo Code"
	case strings.Contains(ua, "opencode/"), strings.Contains(ua, "crush/"):
		return "OpenCode"
	case strings.Contains(ua, "aider/"), strings.Contains(ua, "litellm/"):
		return "Aider"
	case strings.Contains(ua, "amazon-q"), strings.Contains(ua, "amazonq"), strings.Contains(ua, "q-developer"):
		return "Amazon Q"
	case strings.Contains(ua, "tabnine/"):
		return "Tabnine"
	case strings.Contains(ua, "codeium"):
		return "Codeium"
	case strings.Contains(ua, "cody/"), strings.Contains(ua, "sourcegraph"):
		return "Cody"
	case strings.Contains(ua, "supermaven/"):
		return "Supermaven"
	case strings.Contains(ua, "goose/"), strings.Contains(ua, "block-goose"):
		return "Goose"
	case strings.Contains(ua, "augment/"), strings.Contains(ua, "augmentcode"):
		return "Augment"
	case strings.Contains(ua, "perplexity-user"), strings.Contains(ua, "perplexity/"):
		return "Perplexity"
	case strings.Contains(ua, "mistralai-user"), strings.Contains(ua, "mistral/"):
		return "Mistral"
	case strings.Contains(ua, "poe/"):
		return "Poe"
	case strings.Contains(ua, "langchain"):
		return "LangChain"
	case strings.Contains(ua, "openai/"), strings.Contains(ua, "openai-api"):
		return "OpenAI API"
	case strings.Contains(ua, "anthropic/"), strings.Contains(ua, "anthropic-api"):
		return "Anthropic API"
	case strings.Contains(ua, "postmanruntime/"):
		return "Postman"
	case strings.Contains(ua, "insomnia/"):
		return "Insomnia"
	case strings.Contains(ua, "curl/"):
		return "cURL"
	case strings.Contains(ua, "wget/"):
		return "Wget"
	case strings.Contains(ua, "python-requests/"), strings.Contains(ua, "python-urllib/"):
		return "Python"
	case strings.Contains(ua, "go-http-client/"), strings.Contains(ua, "go-resty/"):
		return "Go"
	case strings.Contains(ua, "node-fetch/"), strings.Contains(ua, "axios/"):
		return "Node.js"
	case strings.Contains(ua, "java/"):
		return "Java"
	case strings.Contains(ua, "firefox/"):
		return "Firefox"
	case strings.Contains(ua, "edg/"):
		return "Edge"
	case strings.Contains(ua, "chrome/"):
		return "Chrome"
	case strings.Contains(ua, "safari/") && !strings.Contains(ua, "chrome"):
		return "Safari"
	}

	if index := strings.Index(ua, "/"); index > 0 {
		name := ua[:index]
		switch name {
		case "mozilla", "applewebkit", "khtml", "gecko", "like":
			return ""
		default:
			if name == "" {
				return ""
			}
			return strings.ToUpper(name[:1]) + name[1:]
		}
	}

	return ""
}

func parseInteractionTypeFromDetailRecord(detailRecord *LogDetailRecord) string {
	if detailRecord == nil {
		return ""
	}

	if interactionType := inferResponsesStructuredInteractionType(
		detailRecord.ResponsesRequestBlocks,
		detailRecord.ResponsesToolResponses,
		detailRecord.ResponsesResponseBlocks,
	); interactionType != "" {
		return interactionType
	}

	if interactionType := inferResponsesInteractionType(
		flattenResponsesPromptInputItems(detailRecord.Prompt["input"]),
	); interactionType != "" {
		return interactionType
	}

	lastUserMessageContent := getPromptNestedString(detailRecord.Prompt, "lastUserMessage", "content")
	hasPromptObjectContent := len(detailRecord.Prompt) > 0
	if len(detailRecord.Prompt) == 1 {
		if _, ok := detailRecord.Prompt["input"]; ok {
			hasPromptObjectContent = false
		}
	}

	hasNonToolInput := strings.TrimSpace(lastUserMessageContent) != "" ||
		len(detailRecord.ClaudeRequestBlocks) > 0 ||
		len(detailRecord.ResponsesRequestBlocks) > 0 ||
		hasPromptObjectContent
	hasToolInput := len(detailRecord.ClaudeToolResponses) > 0 || len(detailRecord.ResponsesToolResponses) > 0
	hasTextOutput := strings.TrimSpace(detailRecord.Completion) != "" ||
		hasClaudeTextResponseBlocks(detailRecord.ClaudeResponseBlocks) ||
		hasResponsesTextOutputBlocks(detailRecord.ResponsesResponseBlocks) ||
		hasOpenAITextResponseBlocks(detailRecord.OpenAIResponseBlocks)
	hasToolUse := hasClaudeToolUseBlocks(detailRecord.ClaudeResponseBlocks) ||
		hasResponsesFunctionCallBlocks(detailRecord.ResponsesResponseBlocks) ||
		hasOpenAIToolCallBlocks(detailRecord.OpenAIResponseBlocks) ||
		len(detailRecord.ToolInvokes) > 0
	hasAnyOutput := hasTextOutput ||
		len(detailRecord.ClaudeResponseBlocks) > 0 ||
		len(detailRecord.ResponsesResponseBlocks) > 0 ||
		len(detailRecord.OpenAIResponseBlocks) > 0

	switch {
	case hasNonToolInput:
		return "输入"
	case !hasNonToolInput && hasTextOutput && !hasToolUse:
		return "输出"
	case hasToolInput || hasToolUse || hasAnyOutput:
		return "回调"
	default:
		return ""
	}
}

func getPromptNestedString(prompt map[string]interface{}, parentKey string, childKey string) string {
	if len(prompt) == 0 {
		return ""
	}
	parent, ok := prompt[parentKey].(map[string]interface{})
	if !ok {
		return ""
	}
	return strings.TrimSpace(common.Interface2String(parent[childKey]))
}

func flattenResponsesPromptInputItems(input interface{}) []responsesPromptInputItem {
	inputItems, ok := input.([]interface{})
	if !ok {
		return nil
	}

	items := make([]responsesPromptInputItem, 0, len(inputItems))
	for _, rawItem := range inputItems {
		item, ok := rawItem.(map[string]interface{})
		if !ok {
			continue
		}

		itemType := strings.TrimSpace(common.Interface2String(item["type"]))
		if itemType == "message" {
			role := strings.TrimSpace(common.Interface2String(item["role"]))
			content, _ := item["content"].([]interface{})
			for _, rawPart := range content {
				part, ok := rawPart.(map[string]interface{})
				if !ok {
					continue
				}
				partType := strings.TrimSpace(common.Interface2String(part["type"]))
				if partType != "input_text" && partType != "text" && partType != "output_text" {
					continue
				}
				items = append(items, responsesPromptInputItem{
					Type: partType,
					Role: role,
					Text: common.Interface2String(part["text"]),
				})
			}
			continue
		}

		if itemType == "function_call" || itemType == "function_call_output" || itemType == "input_text" || itemType == "text" || itemType == "output_text" {
			items = append(items, responsesPromptInputItem{
				Type: itemType,
				Role: strings.TrimSpace(common.Interface2String(item["role"])),
				Text: common.Interface2String(item["text"]),
			})
		}
	}

	return items
}

func inferResponsesInteractionType(items []responsesPromptInputItem) string {
	if len(items) == 0 {
		return ""
	}

	lastItem := items[len(items)-1]
	switch lastItem.Type {
	case "input_text", "text":
		return "输入"
	case "function_call_output", "function_call":
		return "回调"
	case "output_text":
		for index := len(items) - 2; index >= 0; index-- {
			if items[index].Type == "function_call_output" {
				return "输出"
			}
		}
	}

	return ""
}

func inferResponsesStructuredInteractionType(
	requestBlocks []ResponsesRequestBlock,
	toolResponses []ResponsesToolResponseBlock,
	responseBlocks []ResponsesResponseBlock,
) string {
	hasRequestInput := false
	for _, block := range requestBlocks {
		if strings.TrimSpace(block.Text) != "" {
			hasRequestInput = true
			break
		}
	}

	hasToolResponse := len(toolResponses) > 0
	hasTextOutput := hasResponsesTextOutputBlocks(responseBlocks)
	hasToolUse := hasResponsesFunctionCallBlocks(responseBlocks)

	switch {
	case hasRequestInput:
		return "输入"
	case !hasRequestInput && hasTextOutput && !hasToolUse:
		return "输出"
	case hasToolResponse || hasToolUse || len(responseBlocks) > 0:
		return "回调"
	default:
		return ""
	}
}

func hasClaudeTextResponseBlocks(blocks []ClaudeResponseBlock) bool {
	for _, block := range blocks {
		if block.Type == "text" && strings.TrimSpace(block.Content) != "" {
			return true
		}
	}
	return false
}

func hasClaudeToolUseBlocks(blocks []ClaudeResponseBlock) bool {
	for _, block := range blocks {
		if block.Type == "tool_use" {
			return true
		}
	}
	return false
}

func hasResponsesTextOutputBlocks(blocks []ResponsesResponseBlock) bool {
	for _, block := range blocks {
		if block.Type == "output_text" && strings.TrimSpace(block.Content) != "" {
			return true
		}
	}
	return false
}

func hasResponsesFunctionCallBlocks(blocks []ResponsesResponseBlock) bool {
	for _, block := range blocks {
		if block.Type == "function_call" {
			return true
		}
	}
	return false
}

func hasOpenAITextResponseBlocks(blocks []OpenAIResponseBlock) bool {
	for _, block := range blocks {
		if (block.Type == "content" || block.Type == "reasoning") && strings.TrimSpace(block.Content) != "" {
			return true
		}
	}
	return false
}

func hasOpenAIToolCallBlocks(blocks []OpenAIResponseBlock) bool {
	for _, block := range blocks {
		if block.Type == "tool_call" {
			return true
		}
	}
	return false
}

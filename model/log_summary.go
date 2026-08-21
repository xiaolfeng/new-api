package model

import (
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/pkg/naming"
)

const (
	LogOtherClientSourceKey      = "client_source"
	LogOtherInteractionTypeKey   = "interaction_type"
	LogOtherAgentIdKey           = "agent_id"
	LogOtherSessionIdKey         = "session_id"
	LogOtherAgentNameKey         = "agent_name"
	LogOtherSessionNameKey       = "session_name"
	LogOtherParentSessionIdKey   = "parent_session_id"
	LogOtherParentSessionNameKey = "parent_session_name"
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

	source, interactionType, agentId, sessionId, parentSessionId := ExtractLogDetailSummaries(record)
	if source != "" {
		other[LogOtherClientSourceKey] = source
	}
	if interactionType != "" {
		other[LogOtherInteractionTypeKey] = interactionType
	}
	if agentId != "" {
		other[LogOtherAgentIdKey] = agentId
		other[LogOtherAgentNameKey] = naming.AgentName(agentId)
	}
	if sessionId != "" {
		other[LogOtherSessionIdKey] = sessionId
		other[LogOtherSessionNameKey] = naming.SessionName(sessionId)
	}
	if parentSessionId != "" {
		other[LogOtherParentSessionIdKey] = parentSessionId
		other[LogOtherParentSessionNameKey] = naming.SessionName(parentSessionId)
	}
	return other
}

func ExtractLogDetailSummaries(record string) (string, string, string, string, string) {
	if strings.TrimSpace(record) == "" {
		return "", "", "", "", ""
	}

	var detailRecord LogDetailRecord
	if err := common.UnmarshalJsonStr(record, &detailRecord); err != nil {
		return "", "", "", "", ""
	}

	source := parseClientSourceFromHeaders(detailRecord.Headers)
	interactionType := parseInteractionTypeFromDetailRecord(&detailRecord)
	agentId, sessionId, parentSessionId := parseAgentSessionFromHeaders(detailRecord.Headers, source)
	return source, interactionType, agentId, sessionId, parentSessionId
}

func IsDeveloperToolLogSource(source string) bool {
	switch strings.TrimSpace(source) {
	case "Claude Code", "Codex", "OpenCode", "ZCode", "Grok Build":
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
	identity := common.MatchClientProfile(userAgent, headers)
	if identity.Profile != common.ClientProfileGeneric && identity.Source != "" {
		return identity.Source
	}

	if userAgent == "" {
		userAgent = getHeaderIgnoreCase(headers, "originator")
	}
	return parseClientSource(userAgent)
}

func parseAgentSessionFromHeaders(headers map[string]string, source string) (agentId, sessionId, parentSessionId string) {
	if len(headers) == 0 {
		return "", "", ""
	}
	// ZCode: X-Zcode-Trace-Id 是主线程（主会话），X-Session-Id 是当前运行 Agent 的会话。
	if source == "ZCode" {
		parentSessionId = getHeaderIgnoreCase(headers, "X-Zcode-Trace-Id")
		sessionId = getHeaderIgnoreCase(headers, "X-Session-Id")
		return "", sessionId, parentSessionId
	}
	// Codex: X-Codex-Window-Id 是 Codex CLI/Desktop 的窗口级会话标识，
	// 形如 "<thread_id>:<window_number>"；缺失时从 X-Codex-Turn-Metadata 兜底。
	if source == "Codex" {
		sessionId = getHeaderIgnoreCase(headers, "X-Codex-Window-Id")
		if sessionId == "" {
			sessionId = parseCodexWindowIdFromTurnMetadata(headers)
		}
		parentSessionId = getHeaderIgnoreCase(headers, "X-Codex-Parent-Thread-Id")
		return "", sessionId, parentSessionId
	}
	// Grok Build: X-Grok-Agent-Id 是 Agent 父会话，X-Grok-Session-Id 是当前子对话。
	if source == "Grok Build" {
		parentSessionId = getHeaderIgnoreCase(headers, "X-Grok-Agent-Id")
		sessionId = getHeaderIgnoreCase(headers, "X-Grok-Session-Id")
		return "", sessionId, parentSessionId
	}
	// OpenCode headers (highest priority)
	sessionId = getHeaderIgnoreCase(headers, "X-Session-Affinity")
	parentSessionId = getHeaderIgnoreCase(headers, "X-Parent-Session-Id")
	// ClaudeCode headers (fallback)
	if sessionId == "" {
		sessionId = getHeaderIgnoreCase(headers, "X-Claude-Code-Session-Id")
	}
	// Generic session header (lowest priority, any client can opt in)
	if sessionId == "" {
		sessionId = getHeaderIgnoreCase(headers, "X-Session-Id")
	}
	agentId = getHeaderIgnoreCase(headers, "X-Claude-Code-Agent-Id")
	return agentId, sessionId, parentSessionId
}

// parseCodexWindowIdFromTurnMetadata 从 X-Codex-Turn-Metadata JSON 中提取 window_id。
func parseCodexWindowIdFromTurnMetadata(headers map[string]string) string {
	raw := getHeaderIgnoreCase(headers, "X-Codex-Turn-Metadata")
	if strings.TrimSpace(raw) == "" {
		return ""
	}
	var metadata map[string]interface{}
	if err := common.UnmarshalJsonStr(raw, &metadata); err != nil {
		return ""
	}
	windowId, ok := metadata["window_id"].(string)
	if !ok {
		return ""
	}
	return strings.TrimSpace(windowId)
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

	if interactionType := inferOpenAIStructuredInteractionType(
		detailRecord.OpenAIRequestBlocks,
		detailRecord.OpenAIToolResponses,
		detailRecord.OpenAIResponseBlocks,
	); interactionType != "" {
		return interactionType
	}

	if interactionType := inferClaudeStructuredInteractionType(
		detailRecord.ClaudeRequestBlocks,
		detailRecord.ClaudeToolResponses,
		detailRecord.ClaudeResponseBlocks,
	); interactionType != "" {
		return interactionType
	}

	if interactionType := inferBambooStructuredInteractionType(
		detailRecord.BambooRequestBlocks,
		detailRecord.BambooToolResponses,
		detailRecord.BambooResponseBlocks,
	); interactionType != "" {
		return interactionType
	}

	if interactionType := inferResponsesInteractionType(
		flattenResponsesPromptInputItems(detailRecord.Prompt["input"]),
	); interactionType != "" {
		return interactionType
	}

	isBambooData := len(detailRecord.BambooRequestBlocks) > 0 ||
		len(detailRecord.BambooToolResponses) > 0 ||
		len(detailRecord.BambooResponseBlocks) > 0

	lastUserMessageContent := getPromptNestedString(detailRecord.Prompt, "lastUserMessage", "content")
	hasPromptObjectContent := len(detailRecord.Prompt) > 0
	if len(detailRecord.Prompt) == 1 {
		if _, ok := detailRecord.Prompt["input"]; ok {
			hasPromptObjectContent = false
		}
	}

	// In the bamboo path, lastUserMessageContent is unconditionally populated by
	// buildBambooStructuredRecord, so it must not drive hasNonToolInput alone.
	hasNonToolInput := (!isBambooData && strings.TrimSpace(lastUserMessageContent) != "") ||
		len(detailRecord.ClaudeRequestBlocks) > 0 ||
		len(detailRecord.ResponsesRequestBlocks) > 0 ||
		len(detailRecord.OpenAIRequestBlocks) > 0 ||
		hasBambooTextInputBlocks(detailRecord.BambooRequestBlocks) ||
		hasPromptObjectContent
	hasToolInput := len(detailRecord.ClaudeToolResponses) > 0 ||
		len(detailRecord.ResponsesToolResponses) > 0 ||
		len(detailRecord.OpenAIToolResponses) > 0 ||
		len(detailRecord.BambooToolResponses) > 0
	hasTextOutput := strings.TrimSpace(detailRecord.Completion) != "" ||
		hasClaudeTextResponseBlocks(detailRecord.ClaudeResponseBlocks) ||
		hasResponsesTextOutputBlocks(detailRecord.ResponsesResponseBlocks) ||
		hasOpenAITextResponseBlocks(detailRecord.OpenAIResponseBlocks) ||
		hasBambooTextResponseBlocks(detailRecord.BambooResponseBlocks)
	hasToolUse := hasClaudeToolUseBlocks(detailRecord.ClaudeResponseBlocks) ||
		hasResponsesFunctionCallBlocks(detailRecord.ResponsesResponseBlocks) ||
		hasOpenAIToolCallBlocks(detailRecord.OpenAIResponseBlocks) ||
		hasBambooToolUseBlocks(detailRecord.BambooResponseBlocks) ||
		len(detailRecord.ToolInvokes) > 0
	hasAnyOutput := hasTextOutput ||
		len(detailRecord.ClaudeResponseBlocks) > 0 ||
		len(detailRecord.ResponsesResponseBlocks) > 0 ||
		len(detailRecord.OpenAIResponseBlocks) > 0 ||
		len(detailRecord.BambooResponseBlocks) > 0

	switch {
	case hasNonToolInput && !hasToolInput:
		return "输入"
	case hasTextOutput && !hasToolUse:
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
	case hasRequestInput && !hasToolResponse:
		return "输入"
	case hasToolUse:
		return "回调"
	case hasTextOutput:
		return "输出"
	case hasToolResponse || len(responseBlocks) > 0:
		return "回调"
	default:
		return ""
	}
}

func inferOpenAIStructuredInteractionType(
	requestBlocks []OpenAIRequestBlock,
	toolResponses []OpenAIToolResponseBlock,
	responseBlocks []OpenAIResponseBlock,
) string {
	hasToolResponse := len(toolResponses) > 0
	hasTextOutput := hasOpenAITextResponseBlocks(responseBlocks)
	hasToolUse := hasOpenAIToolCallBlocks(responseBlocks)

	hasRequestInput := false
	for _, block := range requestBlocks {
		if strings.TrimSpace(block.Text) != "" {
			hasRequestInput = true
			break
		}
	}

	switch {
	case hasRequestInput && !hasToolResponse:
		return "输入"
	case hasToolUse:
		return "回调"
	case hasTextOutput:
		return "输出"
	case hasToolResponse:
		return "回调"
	case len(responseBlocks) > 0:
		return "回调"
	default:
		return ""
	}
}

func inferClaudeStructuredInteractionType(
	requestBlocks []ClaudeRequestBlock,
	toolResponses []ClaudeToolResponseBlock,
	responseBlocks []ClaudeResponseBlock,
) string {
	hasToolResponse := len(toolResponses) > 0
	hasTextOutput := hasClaudeTextResponseBlocks(responseBlocks)
	hasToolUse := hasClaudeToolUseBlocks(responseBlocks)

	hasRequestInput := false
	for _, block := range requestBlocks {
		if strings.TrimSpace(block.Text) != "" {
			hasRequestInput = true
			break
		}
	}

	switch {
	case hasRequestInput && !hasToolResponse:
		return "输入"
	case hasToolUse:
		return "回调"
	case hasTextOutput:
		return "输出"
	case hasToolResponse:
		return "回调"
	case len(responseBlocks) > 0:
		return "回调"
	default:
		return ""
	}
}

func inferBambooStructuredInteractionType(
	requestBlocks []BambooRequestBlock,
	toolResponses []BambooToolResponseBlock,
	responseBlocks []BambooResponseBlock,
) string {
	hasToolUse := hasBambooToolUseBlocks(responseBlocks)
	hasTextOutput := hasBambooTextResponseBlocks(responseBlocks)
	hasToolResponse := len(toolResponses) > 0

	hasRequestInput := false
	for _, block := range requestBlocks {
		if strings.TrimSpace(block.Text) != "" {
			hasRequestInput = true
			break
		}
	}

	switch {
	case hasRequestInput && !hasToolResponse:
		return "输入"
	case hasToolUse:
		return "回调"
	case hasTextOutput:
		return "输出"
	case len(toolResponses) > 0:
		return "回调"
	case len(responseBlocks) > 0:
		return "回调"
	default:
		return ""
	}
}

func hasBambooTextResponseBlocks(blocks []BambooResponseBlock) bool {
	for _, block := range blocks {
		if block.Type == "text" && strings.TrimSpace(block.Text) != "" {
			return true
		}
	}
	return false
}

func hasBambooToolUseBlocks(blocks []BambooResponseBlock) bool {
	for _, block := range blocks {
		if block.Type == "tool_use" {
			return true
		}
	}
	return false
}

func hasBambooTextInputBlocks(blocks []BambooRequestBlock) bool {
	for _, block := range blocks {
		if strings.TrimSpace(block.Text) != "" {
			return true
		}
	}
	return false
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

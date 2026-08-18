package model_setting

import (
	"strings"

	"github.com/QuantumNous/new-api/setting/config"
)

// BambooSettings 控制 bamboo 中继桥的灰度开关。
//
// bamboo 中继桥（relay/bamboo）用 bamboo-messages 的协议无关中间表示，
// 替代 new-api 四个对话类 Helper 内部的 Convert→DoRequest→DoResponse 三段式，
// 将入口协议 × 上游协议的 N×M 转换矩阵降为 N+M。
//
// 灰度策略：开关默认关闭，关闭时所有 Helper 走原生三段式（零影响）；
// 开启后仅对 bamboo 覆盖的 ApiType（OpenAI 兼容/Anthropic/Gemini/Responses）生效，
// 未覆盖渠道（AWS/讯飞/腾讯等）自动 fallback 原生链路。
type BambooSettings struct {
	// EnableBambooRelay 全局开关，默认关闭。
	// 关闭时 TextHelper/ClaudeHelper/GeminiHelper/ResponsesHelper 走原生三段式。
	EnableBambooRelay bool `json:"enable_bamboo_relay"`

	// EnableBambooDebugLog 控制 bamboo-messages 的 debug 信息收集。
	// 开启后，bridge.go 会用 FormatRelayParsed/FormatRelayInput/FormatDebugRequest
	// 以及 FormatRelayResponse/FormatRelayResponseFrame 收集分块 debug 信息，
	// 写入 RelayInfo.BambooDebug（*BambooDebugInfo 结构），
	// 最终在消费日志详情的 "Bamboo Debug" 板块分块展示。
	// SDK v0.8.9 的 Format 系列函数为纯函数（调用即返回），由本开关控制是否收集。
	EnableBambooDebugLog bool `json:"enable_bamboo_debug_log"`

	// DegradedReason 流式中断降级时补发的完成原因策略，全局生效。
	// 空字符串/"stop"（默认）：中断时补发 finish_reason=stop，普通对话场景适用。
	// "tool_use"：带 tools 请求中断时补发 finish_reason=tool_calls，
	// 让 ReAct Agent 继续轮询而非终止长时任务。无 tools 时回退为 stop。
	DegradedReason string `json:"degraded_reason"`

	// EnableHostTools 仅在 EnableBambooRelay=true 时生效。默认关闭。
	EnableHostTools bool `json:"enable_host_tools"`

	// HostToolMode: "loop" = Mode A-thin；"return" = Mode B。非法值当 loop。
	HostToolMode string `json:"host_tool_mode"`

	// SearchBackend: off | exa | parallel | searxng
	SearchBackend string `json:"search_backend"`

	// AllowThirdPartySearchEgress 为 false 时 exa/parallel 视为 off。
	AllowThirdPartySearchEgress bool `json:"allow_third_party_search_egress"`

	SearchFallback []string `json:"search_fallback"`
	SearxngBaseURL string   `json:"searxng_base_url"`
	ExaMCPURL      string   `json:"exa_mcp_url"`
	ExaAPIKey      string   `json:"exa_api_key"`
	ParallelMCPURL string   `json:"parallel_mcp_url"`
	ParallelAPIKey string   `json:"parallel_api_key"`

	// MaxHostToolRounds v1 语义 = 额外 hop 数，硬顶 1。
	MaxHostToolRounds int `json:"max_host_tool_rounds"`
	MaxSearchResults  int `json:"max_search_results"`
	MaxFetchBytes     int `json:"max_fetch_bytes"`
	MaxResultRunes    int `json:"max_result_runes"`
	HostToolTimeoutMs int `json:"host_tool_timeout_ms"`

	// EnableImageRecognize 在 Bamboo 对话路径上预识别最新 user 消息里的图。
	// 不是 host tool，不走 tool_call。默认关闭。
	EnableImageRecognize bool `json:"enable_image_recognize"`
	// ImageRecognizeChannelId 指定识别渠道；0 表示未配置。
	ImageRecognizeChannelId int `json:"image_recognize_channel_id"`
	// ImageRecognizeModel 指定识别模型（渠道模型列表中的名字）。
	ImageRecognizeModel string `json:"image_recognize_model"`
	// ImageRecognizePrompt 识别 hop 的系统提示。空则用内置默认。
	ImageRecognizePrompt         string `json:"image_recognize_prompt"`
	ImageRecognizeMaxImages      int    `json:"image_recognize_max_images"`
	ImageRecognizeTimeoutMs      int    `json:"image_recognize_timeout_ms"`
	ImageRecognizeMaxOutputRunes int    `json:"image_recognize_max_output_runes"`
}

const (
	DefaultExaMCPURL      = "https://mcp.exa.ai/mcp"
	DefaultParallelMCPURL = "https://search.parallel.ai/mcp"
)

// 默认配置
var defaultBambooSettings = BambooSettings{
	EnableBambooRelay:            false,
	EnableBambooDebugLog:         false,
	DegradedReason:               "stop",
	EnableHostTools:              false,
	HostToolMode:                 "loop",
	SearchBackend:                "off",
	AllowThirdPartySearchEgress:  false,
	SearchFallback:               []string{},
	ExaMCPURL:                    DefaultExaMCPURL,
	ParallelMCPURL:               DefaultParallelMCPURL,
	MaxHostToolRounds:            1,
	MaxSearchResults:             8,
	MaxFetchBytes:                1048576,
	MaxResultRunes:               65536,
	HostToolTimeoutMs:            15000,
	EnableImageRecognize:         false,
	ImageRecognizeMaxImages:      4,
	ImageRecognizeTimeoutMs:      20000,
	ImageRecognizeMaxOutputRunes: 4096,
}

// 全局实例
var bambooSettings = defaultBambooSettings

func init() {
	// 注册到全局配置管理器，对应 options 表 key 前缀 "bamboo."
	config.GlobalConfig.Register("bamboo", &bambooSettings)
}

// GetBambooSettings 返回 bamboo 中继设置的当前实例（指针，运行时可热更新）。
func GetBambooSettings() *BambooSettings {
	return &bambooSettings
}

// ResolvedHostToolMode 返回规范化后的执行模式。非法值按 loop。
func (s *BambooSettings) ResolvedHostToolMode() string {
	if s == nil {
		return "loop"
	}
	if s.HostToolMode == "return" {
		return "return"
	}
	return "loop"
}

// ResolvedSearchBackend 返回实际可发起请求的搜索后端。
// allow_third_party_search_egress=false 时 exa/parallel 视为 off。
func (s *BambooSettings) ResolvedSearchBackend() string {
	if s == nil {
		return "off"
	}
	backend := s.SearchBackend
	if backend == "" {
		backend = "off"
	}
	switch backend {
	case "exa", "parallel":
		if !s.AllowThirdPartySearchEgress {
			return "off"
		}
		return backend
	case "searxng":
		return "searxng"
	default:
		return "off"
	}
}

func (s *BambooSettings) ClampMaxSearchResults() int {
	n := 8
	if s != nil && s.MaxSearchResults > 0 {
		n = s.MaxSearchResults
	}
	if n > 20 {
		return 20
	}
	if n < 1 {
		return 1
	}
	return n
}

func (s *BambooSettings) ClampMaxFetchBytes() int {
	n := 1048576
	if s != nil && s.MaxFetchBytes > 0 {
		n = s.MaxFetchBytes
	}
	if n > 5*1024*1024 {
		return 5 * 1024 * 1024
	}
	return n
}

func (s *BambooSettings) ClampMaxResultRunes() int {
	n := 65536
	if s != nil && s.MaxResultRunes > 0 {
		n = s.MaxResultRunes
	}
	if n > 65536 {
		return 65536
	}
	return n
}

func (s *BambooSettings) ClampTimeout() int {
	n := 15000
	if s != nil && s.HostToolTimeoutMs > 0 {
		n = s.HostToolTimeoutMs
	}
	if n > 30000 {
		return 30000
	}
	if n < 1 {
		return 15000
	}
	return n
}

func (s *BambooSettings) ResolvedExaMCPURL() string {
	if s != nil && s.ExaMCPURL != "" {
		return s.ExaMCPURL
	}
	return DefaultExaMCPURL
}

func (s *BambooSettings) ResolvedParallelMCPURL() string {
	if s != nil && s.ParallelMCPURL != "" {
		return s.ParallelMCPURL
	}
	return DefaultParallelMCPURL
}

func (s *BambooSettings) ClampImageRecognizeMaxImages() int {
	n := 4
	if s != nil && s.ImageRecognizeMaxImages > 0 {
		n = s.ImageRecognizeMaxImages
	}
	if n > 8 {
		return 8
	}
	if n < 1 {
		return 1
	}
	return n
}

func (s *BambooSettings) ClampImageRecognizeTimeout() int {
	n := 20000
	if s != nil && s.ImageRecognizeTimeoutMs > 0 {
		n = s.ImageRecognizeTimeoutMs
	}
	if n > 60000 {
		return 60000
	}
	if n < 1 {
		return 20000
	}
	return n
}

func (s *BambooSettings) ClampImageRecognizeMaxOutputRunes() int {
	n := 4096
	if s != nil && s.ImageRecognizeMaxOutputRunes > 0 {
		n = s.ImageRecognizeMaxOutputRunes
	}
	if n > 16384 {
		return 16384
	}
	if n < 1 {
		return 4096
	}
	return n
}

func (s *BambooSettings) ResolvedImageRecognizePrompt() string {
	if s != nil && strings.TrimSpace(s.ImageRecognizePrompt) != "" {
		return strings.TrimSpace(s.ImageRecognizePrompt)
	}
	return "Describe each attached image. List visible text (OCR) and key objects. Do not invent details. Reply as [Image N] then the description."
}

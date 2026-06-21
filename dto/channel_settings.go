package dto

type ChannelSettings struct {
	ForceFormat            bool   `json:"force_format,omitempty"`
	ThinkingToContent      bool   `json:"thinking_to_content,omitempty"`
	Proxy                  string `json:"proxy"`
	PassThroughBodyEnabled bool   `json:"pass_through_body_enabled,omitempty"`
	SystemPrompt           string `json:"system_prompt,omitempty"`
	SystemPromptOverride   bool   `json:"system_prompt_override,omitempty"`
}

type VertexKeyType string

const (
	VertexKeyTypeJSON   VertexKeyType = "json"
	VertexKeyTypeAPIKey VertexKeyType = "api_key"
)

type AwsKeyType string

const (
	AwsKeyTypeAKSK   AwsKeyType = "ak_sk" // 默认
	AwsKeyTypeApiKey AwsKeyType = "api_key"
)

// BambooUpstreamFormatType 定义 bamboo 灰度模式下渠道可手动指定的上游协议格式。
type BambooUpstreamFormatType string

const (
	BambooUpstreamFormatAuto      BambooUpstreamFormatType = ""          // 默认：根据渠道 ApiType 自动选择
	BambooUpstreamFormatOpenAI    BambooUpstreamFormatType = "openai"    // 强制使用 OpenAI Chat Completions 协议
	BambooUpstreamFormatAnthropic BambooUpstreamFormatType = "anthropic" // 强制使用 Anthropic Messages 协议
	BambooUpstreamFormatGemini    BambooUpstreamFormatType = "gemini"    // 强制使用 Google Gemini 协议
	BambooUpstreamFormatResponses BambooUpstreamFormatType = "responses" // 强制使用 OpenAI Responses 协议
)

// BambooSmoothLevelType 定义 bamboo 流式平滑缓冲策略档位。
//
// 平滑缓冲器（SmoothPacer）在 codec 序列化后、写入 HTTP Response 前，
// 将文本 delta 切分为微帧并按 EMA 自适应间隔匀速释放，
// 消除上游突发批量到达导致的"打字机跳动"现象。
type BambooSmoothLevelType string

const (
	BambooSmoothLevelOff        BambooSmoothLevelType = ""            // 默认：关闭平滑缓冲，直接透传上游事件
	BambooSmoothLevelGentle     BambooSmoothLevelType = "gentle"     // 轻柔：2 token/帧，间隔 20-100ms，适合大多数场景
	BambooSmoothLevelSmooth     BambooSmoothLevelType = "smooth"     // 平滑：1 token/帧，间隔 15-80ms，打字机效果更明显
	BambooSmoothLevelTypewriter BambooSmoothLevelType = "typewriter" // 打字机：1 token/帧，间隔 30-120ms，经典老式终端体验
)

type ChannelOtherSettings struct {
	AzureResponsesVersion                 string               `json:"azure_responses_version,omitempty"`
	VertexKeyType                         VertexKeyType        `json:"vertex_key_type,omitempty"` // "json" or "api_key"
	OpenRouterEnterprise                  *bool                `json:"openrouter_enterprise,omitempty"`
	ClaudeBetaQuery                       bool                 `json:"claude_beta_query,omitempty"`         // Claude 渠道是否强制追加 ?beta=true
	AllowServiceTier                      bool                 `json:"allow_service_tier,omitempty"`        // 是否允许 service_tier 透传（默认过滤以避免额外计费）
	AllowInferenceGeo                     bool                 `json:"allow_inference_geo,omitempty"`       // 是否允许 inference_geo 透传（仅 Claude，默认过滤以满足数据驻留合规
	AllowSpeed                            bool                 `json:"allow_speed,omitempty"`               // 是否允许 speed 透传（仅 Claude，默认过滤以避免意外切换推理速度模式）
	AllowSafetyIdentifier                 bool                 `json:"allow_safety_identifier,omitempty"`   // 是否允许 safety_identifier 透传（默认过滤以保护用户隐私）
	DisableStore                          bool                 `json:"disable_store,omitempty"`             // 是否禁用 store 透传（默认允许透传，禁用后可能导致 Codex 无法使用）
	AllowIncludeObfuscation               bool                 `json:"allow_include_obfuscation,omitempty"` // 是否允许 stream_options.include_obfuscation 透传（默认过滤以避免关闭流混淆保护）
	AwsKeyType                            AwsKeyType           `json:"aws_key_type,omitempty"`
	BambooUpstreamFormat                  string               `json:"bamboo_upstream_format,omitempty"`    // bamboo 灰度模式下手动指定上游协议格式："" / "auto"(默认) / "openai" / "anthropic" / "gemini" / "responses"
	UpstreamModelUpdateCheckEnabled       bool          `json:"upstream_model_update_check_enabled,omitempty"`        // 是否检测上游模型更新
	UpstreamModelUpdateAutoSyncEnabled    bool          `json:"upstream_model_update_auto_sync_enabled,omitempty"`    // 是否自动同步上游模型更新
	UpstreamModelUpdateLastCheckTime      int64         `json:"upstream_model_update_last_check_time,omitempty"`      // 上次检测时间
	UpstreamModelUpdateLastDetectedModels []string      `json:"upstream_model_update_last_detected_models,omitempty"` // 上次检测到的可加入模型
	UpstreamModelUpdateLastRemovedModels  []string      `json:"upstream_model_update_last_removed_models,omitempty"`  // 上次检测到的可删除模型
	UpstreamModelUpdateIgnoredModels      []string      `json:"upstream_model_update_ignored_models,omitempty"`       // 手动忽略的模型
}

func (s *ChannelOtherSettings) IsOpenRouterEnterprise() bool {
	if s == nil || s.OpenRouterEnterprise == nil {
		return false
	}
	return *s.OpenRouterEnterprise
}

package common

// ImageRecognizePlan 是 Bamboo 图片预识别的本地结果。
// 放在 relay/common，避免 RelayInfo 导入 relay/bamboo 造成循环依赖。
type ImageRecognizePlan struct {
	Enabled       bool
	SkippedReason string // has_vision | no_latest_user_image | not_configured | inner_hop
	ChannelId     int
	Model         string
	ImageCount    int
	VisibleBox    string
	DurationMs    int64
	Error         string
	PromptTokens  int
	CompletionTok int
}

const (
	ImageRecognizeSkipHasVision         = "has_vision"
	ImageRecognizeSkipNoLatestUserImage = "no_latest_user_image"
	ImageRecognizeSkipNotConfigured     = "not_configured"
	ImageRecognizeSkipInnerHop          = "inner_hop"
	ImageRecognizeSkipDisabled          = "disabled"
)

const (
	ImageRecognizeFenceStart  = "<<<image_recognition>>>"
	ImageRecognizeFenceEnd    = "<<<end_image_recognition>>>"
	ImageRecognizeHistoryMark = "[image]"
	ImageRecognizeToolKind    = "recognize"
	ImageRecognizeCanonical   = "host.image_recognize"
	ImageRecognizeOriginal    = "image_recognize"
)

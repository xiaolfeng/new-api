package model_setting

import (
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
	// 开启后，bridge.go 会用 FormatRelayInput/FormatRelayParsed/FormatDebugRequest
	// 收集格式化 debug 字符串，写入 RelayInfo.BambooDebug，
	// 最终在消费日志详情的 "Bamboo" 板块展示。
	// 不再调用 provider.SetDebug(true)，避免 log.Printf 刷屏。
	EnableBambooDebugLog bool `json:"enable_bamboo_debug_log"`

	// SmoothLevel 流式平滑缓冲档位，全局生效。
	// 空字符串/"off" 关闭（直接透传）；gentle/smooth/typewriter 启用 SmoothPacer。
	SmoothLevel string `json:"smooth_level"`
}

// 默认配置
var defaultBambooSettings = BambooSettings{
	EnableBambooRelay:    false,
	EnableBambooDebugLog: false,
	SmoothLevel:          "off",
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

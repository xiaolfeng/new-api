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
}

// 默认配置
var defaultBambooSettings = BambooSettings{
	EnableBambooRelay:    false,
	EnableBambooDebugLog: false,
	DegradedReason:       "stop",
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

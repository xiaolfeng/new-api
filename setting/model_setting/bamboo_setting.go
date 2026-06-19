package model_setting

import (
	"github.com/bamboo-services/bamboo-messages/provider"

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

	// EnableBambooDebugLog 控制 bamboo-messages provider 层的 debug 日志输出。
	// 开启后，provider.SetDebug(true) 会让 bamboo 在每次上游请求时打印
	// provider 类型、目标端点、请求头和请求体（截断至 MaxDebugBodyLen），
	// 用于开发/调试阶段排查"参数有误""协议不兼容"等问题。
	// 仅在 EnableBambooRelay 开启时有意义；生产环境建议关闭。
	EnableBambooDebugLog bool `json:"enable_bamboo_debug_log"`
}

// 默认配置
var defaultBambooSettings = BambooSettings{
	EnableBambooRelay:    false,
	EnableBambooDebugLog: false,
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

// SyncDebugToProvider 将当前 EnableBambooDebugLog 的值同步到
// bamboo-messages provider 包的全局 debug 开关。
//
// 该方法应在应用启动初始化阶段调用一次，以及在 options 配置热更新
// 回调中调用，确保运行时修改开关立即生效。
func (s *BambooSettings) SyncDebugToProvider() {
	provider.SetDebug(s.EnableBambooDebugLog)
}

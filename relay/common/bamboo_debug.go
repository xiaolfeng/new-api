package common

// BambooDebugInfo 存储 bamboo relay 的分块 debug 信息。
// 每个字段对应一个独立的 debug 块，前端可分块展示和单独复制。
//
// 仅在 EnableBambooDebugLog 开启且走 bamboo relay 路径时非 nil。
// 由 relay/bamboo/bridge.go 在各阶段分别写入：
//   - RelayParsed:     FormatRelayParsed 输出（ParseRequest 后）
//   - RelayInput:      FormatRelayInput 输出（provider 创建后）
//   - ProviderRequest: provider.FormatDebugRequest 输出（与 RelayInput 同阶段）
//   - RelayResponse:   FormatRelayResponse / FormatRelayResponseFrame 输出（响应阶段）
type BambooDebugInfo struct {
	RelayParsed     string `json:"relayParsed,omitempty"`
	RelayInput      string `json:"relayInput,omitempty"`
	ProviderRequest string `json:"providerRequest,omitempty"`
	RelayResponse   string `json:"relayResponse,omitempty"`
}

// HasContent 判断是否有任何 debug 内容。
// 用于日志记录层决定是否写入 BambooDebug 字段。
func (d *BambooDebugInfo) HasContent() bool {
	if d == nil {
		return false
	}
	return d.RelayParsed != "" || d.RelayInput != "" || d.ProviderRequest != "" || d.RelayResponse != ""
}

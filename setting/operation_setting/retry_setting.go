/*
Copyright (C) 2025 QuantumNous

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as
published by the Free Software Foundation, either version 3 of the
License, or (at your option) any later version.

This program is distributed in the hope that it will be useful,
but WITHOUT ANY WARRANTY; without even the implied warranty of
MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
GNU Affero General Public License for more details.

You should have received a copy of the GNU Affero General Public License
along with this program. If not, see <https://www.gnu.org/licenses/>.

For commercial licensing, please contact support@quantumnous.com
*/

package operation_setting

import "github.com/QuantumNous/new-api/setting/config"

// RetrySetting 空响应重试设置
type RetrySetting struct {
	// EmptyResponseRetryEnabled 启用空响应重试
	// 当上游返回 HTTP 2xx 但响应内容为空（completion_tokens=0）时自动重试
	EmptyResponseRetryEnabled bool `json:"empty_response_retry_enabled"`
	// EmptyResponseRetryDelaySeconds 空响应重试延迟秒数
	// 0 表示立即重试
	EmptyResponseRetryDelaySeconds int `json:"empty_response_retry_delay_seconds"`
	// RecordConsumeLogDetailEnabled 启用消费日志详细记录
	// 记录消费日志的请求内容、响应内容和 HTTP 头（排除敏感信息）
	RecordConsumeLogDetailEnabled bool `json:"record_consume_log_detail_enabled"`
}

// 默认配置
var retrySetting = RetrySetting{
	EmptyResponseRetryEnabled:      false,
	EmptyResponseRetryDelaySeconds: 0,
	RecordConsumeLogDetailEnabled:  false,
}

func init() {
	// 注册到全局配置管理器
	config.GlobalConfig.Register("retry_setting", &retrySetting)
}

// GetRetrySetting 获取空响应重试设置
func GetRetrySetting() *RetrySetting {
	return &retrySetting
}

// IsEmptyResponseRetryEnabled 是否启用空响应重试
func IsEmptyResponseRetryEnabled() bool {
	return retrySetting.EmptyResponseRetryEnabled
}

// GetEmptyResponseRetryDelaySeconds 获取空响应重试延迟秒数
func GetEmptyResponseRetryDelaySeconds() int {
	return retrySetting.EmptyResponseRetryDelaySeconds
}

// IsRecordConsumeLogDetailEnabled 是否启用消费日志详细记录
func IsRecordConsumeLogDetailEnabled() bool {
	return retrySetting.RecordConsumeLogDetailEnabled
}

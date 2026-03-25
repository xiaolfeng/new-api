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

import (
	"time"

	"github.com/QuantumNous/new-api/setting/config"
)

const recordConsumeLogDetailDurationSeconds = 5 * 60

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
	// FullLogConsumeEnabled 启用完整消费日志记录
	// 记录完整 request/response，仅允许短时间开启
	FullLogConsumeEnabled bool `json:"full_log_consume_enabled"`
	// FullLogConsumeExpiresAt 完整消费日志记录过期时间（Unix 秒）
	FullLogConsumeExpiresAt int64 `json:"full_log_consume_expires_at"`
}

// 默认配置
var retrySetting = RetrySetting{
	EmptyResponseRetryEnabled:      false,
	EmptyResponseRetryDelaySeconds: 0,
	RecordConsumeLogDetailEnabled:  false,
	FullLogConsumeEnabled:          false,
	FullLogConsumeExpiresAt:        0,
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

func IsFullLogConsumeEnabled() bool {
	if !retrySetting.FullLogConsumeEnabled {
		return false
	}
	if retrySetting.FullLogConsumeExpiresAt <= 0 {
		return false
	}
	return retrySetting.FullLogConsumeExpiresAt > time.Now().Unix()
}

func GetFullLogConsumeExpiresAt() int64 {
	if !IsFullLogConsumeEnabled() {
		return 0
	}
	return retrySetting.FullLogConsumeExpiresAt
}

func GetFullLogConsumeRemainingSeconds() int64 {
	expiresAt := GetFullLogConsumeExpiresAt()
	if expiresAt <= 0 {
		return 0
	}
	remaining := expiresAt - time.Now().Unix()
	if remaining < 0 {
		return 0
	}
	return remaining
}

func GetRecordConsumeLogDetailDurationSeconds() int64 {
	return recordConsumeLogDetailDurationSeconds
}

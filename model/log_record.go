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

package model

// LogDetailRecord 消费日志详细记录结构
type LogDetailRecord struct {
	Prompt      map[string]interface{} `json:"prompt,omitempty"`
	Completion  string                 `json:"completion,omitempty"`
	Headers     map[string]string      `json:"headers,omitempty"`
	ToolInvokes []LogToolInvokeRecord  `json:"toolInvokes,omitempty"`
}

type LogToolInvokeRecord struct {
	ID           string      `json:"id,omitempty"`
	Name         string      `json:"name,omitempty"`
	Input        interface{} `json:"input,omitempty"`
	Result       interface{} `json:"result,omitempty"`
	ResultText   string      `json:"resultText,omitempty"`
	IsError      *bool       `json:"isError,omitempty"`
	StopReason   string      `json:"stopReason,omitempty"`
	ResponseRole string      `json:"responseRole,omitempty"`
}

type FullLogRecord struct {
	Request  *FullLogRequest  `json:"request,omitempty"`
	Response *FullLogResponse `json:"response,omitempty"`
	Meta     *FullLogMeta     `json:"meta,omitempty"`
}

type FullLogRequest struct {
	Headers map[string]string `json:"headers,omitempty"`
	Body    interface{}       `json:"body,omitempty"`
}

type FullLogResponse struct {
	Body interface{} `json:"body,omitempty"`
}

type FullLogMeta struct {
	RequestID          string `json:"requestId,omitempty"`
	RequestPath        string `json:"requestPath,omitempty"`
	IsStream           bool   `json:"isStream,omitempty"`
	RelayFormat        string `json:"relayFormat,omitempty"`
	FinalRequestFormat string `json:"finalRequestFormat,omitempty"`
	RetryIndex         int    `json:"retryIndex,omitempty"`
}

// SensitiveHeaders 敏感请求头列表（这些头信息不会被记录）
var SensitiveHeaders = map[string]bool{
	"authorization":       true,
	"x-api-key":           true,
	"x-auth-token":        true,
	"cookie":              true,
	"set-cookie":          true,
	"proxy-authorization": true,
	"cf-authorization":    true,
	"fastly-key":          true,
	"fastly-token":        true,
	"x-amz-target":        true,
	"x-ms-authorization":  true,
}

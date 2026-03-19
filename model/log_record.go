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
	Prompt     map[string]interface{} `json:"prompt,omitempty"`
	Completion string                 `json:"completion,omitempty"`
	Headers    map[string]string      `json:"headers,omitempty"`
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

package model

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/logger"
	"github.com/QuantumNous/new-api/pkg/naming"
	"github.com/QuantumNous/new-api/types"

	"github.com/gin-gonic/gin"

	"github.com/bytedance/gopkg/util/gopool"
	"gorm.io/gorm"
)

func applyExplicitLogTextFilter(tx *gorm.DB, column string, value string) (*gorm.DB, error) {
	if value == "" {
		return tx, nil
	}
	if strings.Contains(value, "%") {
		pattern, err := sanitizeLikePattern(value)
		if err != nil {
			return nil, err
		}
		return tx.Where(column+" LIKE ? ESCAPE '!'", pattern), nil
	}
	return tx.Where(column+" = ?", value), nil
}

type Log struct {
	Id                int     `json:"id" gorm:"index:idx_created_at_id,priority:2;index:idx_user_id_id,priority:2"`
	UserId            int     `json:"user_id" gorm:"index;index:idx_user_id_id,priority:1"`
	CreatedAt         int64   `json:"created_at" gorm:"bigint;index:idx_created_at_id,priority:1;index:idx_created_at_type"`
	Type              int     `json:"type" gorm:"index:idx_created_at_type"`
	Content           string  `json:"content"`
	Username          string  `json:"username" gorm:"index;index:index_username_model_name,priority:2;default:''"`
	TokenName         string  `json:"token_name" gorm:"index;default:''"`
	ModelName         string  `json:"model_name" gorm:"index;index:index_username_model_name,priority:1;default:''"`
	Quota             int     `json:"quota" gorm:"default:0"`
	PromptTokens      int     `json:"prompt_tokens" gorm:"default:0"`
	CompletionTokens  int     `json:"completion_tokens" gorm:"default:0"`
	UseTime           int     `json:"use_time" gorm:"default:0"`
	IsStream          bool    `json:"is_stream"`
	ChannelId         int     `json:"channel" gorm:"index"`
	ChannelName       string  `json:"channel_name" gorm:"->"`
	TokenId           int     `json:"token_id" gorm:"default:0;index"`
	Group             string  `json:"group" gorm:"index"`
	Ip                string  `json:"ip" gorm:"index;default:''"`
	RequestId         string  `json:"request_id,omitempty" gorm:"type:varchar(64);index:idx_logs_request_id;default:''"`
	UpstreamRequestId string  `json:"upstream_request_id,omitempty" gorm:"type:varchar(128);index:idx_logs_upstream_request_id;default:''"`
	Other             string  `json:"other"`
	Record            string  `json:"record" gorm:"type:text"`                 // 消费日志详细记录（管理员/代码用户可见，仍受来源限制）
	FullLog           string  `json:"full_log" gorm:"type:text"`               // 完整消费日志记录（管理员/代码用户可见，仍受来源限制）
	Tps               float64 `json:"tps" gorm:"type:decimal(10,2);default:0"` // Tokens Per Second
}

// don't use iota, avoid change log type value
const (
	LogTypeUnknown = 0
	LogTypeTopup   = 1
	LogTypeConsume = 2
	LogTypeManage  = 3
	LogTypeSystem  = 4
	LogTypeError   = 5
	LogTypeRefund  = 6
	LogTypeLogin   = 7
)

func formatUserLogs(logs []*Log, startIdx int, viewer *User) {
	for i := range logs {
		logs[i].ChannelName = ""
		sourceFromRecord, interactionFromRecord, agentIdFromRecord, sessionIdFromRecord, parentSessionIdFromRecord := ExtractLogDetailSummaries(logs[i].Record)

		otherMap := map[string]interface{}{}
		otherParsed := false
		originalOther := logs[i].Other
		if logs[i].Other != "" {
			parsedOtherMap, err := common.StrToMap(logs[i].Other)
			if err != nil {
				logger.LogWarn(context.TODO(), fmt.Sprintf("formatUserLogs: failed to parse other field: %v", err))
			} else {
				otherMap = parsedOtherMap
				otherParsed = true
			}
		}

		if sourceFromRecord != "" && strings.TrimSpace(common.Interface2String(otherMap[LogOtherClientSourceKey])) == "" {
			otherMap[LogOtherClientSourceKey] = sourceFromRecord
		}
		// interaction_type: always overwrite (see appendAdminLogSummaries for rationale).
		if interactionFromRecord != "" {
			otherMap[LogOtherInteractionTypeKey] = interactionFromRecord
		}
		if agentIdFromRecord != "" && strings.TrimSpace(common.Interface2String(otherMap[LogOtherAgentIdKey])) == "" {
			otherMap[LogOtherAgentIdKey] = agentIdFromRecord
			otherMap[LogOtherAgentNameKey] = naming.AgentName(agentIdFromRecord)
		}
		if sessionIdFromRecord != "" && strings.TrimSpace(common.Interface2String(otherMap[LogOtherSessionIdKey])) == "" {
			otherMap[LogOtherSessionIdKey] = sessionIdFromRecord
			otherMap[LogOtherSessionNameKey] = naming.SessionName(sessionIdFromRecord)
		}
		if parentSessionIdFromRecord != "" && strings.TrimSpace(common.Interface2String(otherMap[LogOtherParentSessionIdKey])) == "" {
			otherMap[LogOtherParentSessionIdKey] = parentSessionIdFromRecord
			otherMap[LogOtherParentSessionNameKey] = naming.SessionName(parentSessionIdFromRecord)
		}
		if otherParsed || len(otherMap) > 0 {
			delete(otherMap, "admin_info")
			// Remove operation-audit details (operator/route info), admin-only.
			delete(otherMap, "audit_info")
			// delete(otherMap, "reject_reason")
			delete(otherMap, "stream_status")
			logs[i].Other = common.MapToJsonStr(otherMap)
		} else {
			logs[i].Other = originalOther
		}

		if viewer != nil {
			summarySource := strings.TrimSpace(common.Interface2String(otherMap[LogOtherClientSourceKey]))
			if summarySource == "" {
				summarySource = sourceFromRecord
			}
			if !CanViewDeveloperToolLogDetail(viewer.Role) || !IsDeveloperToolLogSource(summarySource) {
				logs[i].Record = ""
				logs[i].FullLog = ""
			}
		}

		logs[i].Other = common.MapToJsonStr(otherMap)
		logs[i].Id = startIdx + i + 1
	}
}

func GetLogByTokenId(tokenId int) (logs []*Log, err error) {
	err = LOG_DB.Model(&Log{}).Where("token_id = ?", tokenId).Order("id desc").Limit(common.MaxRecentItems).Find(&logs).Error
	if err != nil {
		return logs, err
	}
	viewer := &User{Role: common.RoleCommonUser}
	if token, tokenErr := GetTokenById(tokenId); tokenErr == nil {
		if user, userErr := GetUserById(token.UserId, false); userErr == nil {
			viewer = user
		}
	}
	formatUserLogs(logs, 0, viewer)
	return logs, err
}

func RecordLog(userId int, logType int, content string) {
	if logType == LogTypeConsume && !common.LogConsumeEnabled {
		return
	}
	username, _ := GetUsernameById(userId, false)
	log := &Log{
		UserId:    userId,
		Username:  username,
		CreatedAt: common.GetTimestamp(),
		Type:      logType,
		Content:   content,
	}
	err := LOG_DB.Create(log).Error
	if err != nil {
		common.SysLog("failed to record log: " + err.Error())
	}
}

// RecordLogWithAdminInfo 记录操作日志，并将管理员相关信息存入 Other.admin_info，
func RecordLogWithAdminInfo(userId int, logType int, content string, adminInfo map[string]interface{}) {
	if logType == LogTypeConsume && !common.LogConsumeEnabled {
		return
	}
	username, _ := GetUsernameById(userId, false)
	log := &Log{
		UserId:    userId,
		Username:  username,
		CreatedAt: common.GetTimestamp(),
		Type:      logType,
		Content:   content,
	}
	if len(adminInfo) > 0 {
		other := map[string]interface{}{
			"admin_info": adminInfo,
		}
		log.Other = common.MapToJsonStr(other)
	}
	if err := LOG_DB.Create(log).Error; err != nil {
		common.SysLog("failed to record log: " + err.Error())
	}
}

// buildOpField 构建语言无关的操作描述（写入 Other.op）。
// 前端依据 action(稳定操作标识) + params(结构化参数) 在渲染期用 i18n 本地化展示，
// 因此不在数据库中存储自然语言句子。
func buildOpField(action string, params map[string]interface{}) map[string]interface{} {
	op := map[string]interface{}{
		"action": action,
	}
	if len(params) > 0 {
		op["params"] = params
	}
	return op
}

// RecordLoginLog 记录用户登录成功的审计日志（type=LogTypeLogin）。
// username 由调用方传入（登录流程已持有用户对象），避免额外的数据库查询。
// content 为英文兜底文本（用于导出/经典前端）；action+params 供前端本地化渲染。
// extra 可携带 login_method、user_agent 等附加信息（普通用户可见）。
func RecordLoginLog(userId int, username string, content string, ip string, action string, params map[string]interface{}, extra map[string]interface{}) {
	other := map[string]interface{}{}
	for k, v := range extra {
		other[k] = v
	}
	other["op"] = buildOpField(action, params)
	log := &Log{
		UserId:    userId,
		Username:  username,
		CreatedAt: common.GetTimestamp(),
		Type:      LogTypeLogin,
		Content:   content,
		Ip:        ip,
		Other:     common.MapToJsonStr(other),
	}
	if err := LOG_DB.Create(log).Error; err != nil {
		common.SysLog("failed to record login log: " + err.Error())
	}
}

// RecordOperationAuditLog 记录管理/高危操作审计日志（type=LogTypeManage）。
// logUserId 为日志归属者（面向用户的操作如额度调整归属目标用户，资源类操作如渠道/系统设置归属操作者），
// username 内部按 logUserId 查询。content 为英文兜底文本（导出/经典前端用）。
// action+params 写入 Other.op，供前端本地化渲染（普通用户可见，不含敏感信息）。
// adminInfo 存放操作者身份（写入 Other.admin_info，普通用户查询时剥离）；
// auditInfo 存放路由/方法/结果等中间件兜底信息（写入 Other.audit_info，普通用户查询时剥离）。
func RecordOperationAuditLog(logUserId int, content string, ip string, action string, params map[string]interface{}, adminInfo map[string]interface{}, auditInfo map[string]interface{}) {
	username, _ := GetUsernameById(logUserId, false)
	other := map[string]interface{}{
		"op": buildOpField(action, params),
	}
	if len(adminInfo) > 0 {
		other["admin_info"] = adminInfo
	}
	if len(auditInfo) > 0 {
		other["audit_info"] = auditInfo
	}
	log := &Log{
		UserId:    logUserId,
		Username:  username,
		CreatedAt: common.GetTimestamp(),
		Type:      LogTypeManage,
		Content:   content,
		Ip:        ip,
		Other:     common.MapToJsonStr(other),
	}
	if err := LOG_DB.Create(log).Error; err != nil {
		common.SysLog("failed to record operation audit log: " + err.Error())
	}
}

func RecordTopupLog(userId int, content string, callerIp string, paymentMethod string, callbackPaymentMethod string) {
	username, _ := GetUsernameById(userId, false)
	adminInfo := map[string]interface{}{
		"server_ip":               common.GetIp(),
		"node_name":               common.NodeName,
		"caller_ip":               callerIp,
		"payment_method":          paymentMethod,
		"callback_payment_method": callbackPaymentMethod,
		"version":                 common.Version,
	}
	other := map[string]interface{}{
		"admin_info": adminInfo,
	}
	log := &Log{
		UserId:    userId,
		Username:  username,
		CreatedAt: common.GetTimestamp(),
		Type:      LogTypeTopup,
		Content:   content,
		Ip:        callerIp,
		Other:     common.MapToJsonStr(other),
	}
	err := LOG_DB.Create(log).Error
	if err != nil {
		common.SysLog("failed to record topup log: " + err.Error())
	}
}

func RecordErrorLog(c *gin.Context, userId int, channelId int, modelName string, tokenName string, content string, tokenId int, useTimeSeconds int,
	isStream bool, group string, other map[string]interface{}) {
	logger.LogInfo(c, fmt.Sprintf("record error log: userId=%d, channelId=%d, modelName=%s, tokenName=%s, content=%s", userId, channelId, modelName, tokenName, common.LocalLogPreview(content)))
	username := c.GetString("username")
	requestId := c.GetString(common.RequestIdKey)
	upstreamRequestId := c.GetString(common.UpstreamRequestIdKey)
	otherStr := common.MapToJsonStr(other)
	// IP 记录永久开启，忽略用户设置
	needRecordIp := true
	log := &Log{
		UserId:           userId,
		Username:         username,
		CreatedAt:        common.GetTimestamp(),
		Type:             LogTypeError,
		Content:          content,
		PromptTokens:     0,
		CompletionTokens: 0,
		TokenName:        tokenName,
		ModelName:        modelName,
		Quota:            0,
		ChannelId:        channelId,
		TokenId:          tokenId,
		UseTime:          useTimeSeconds,
		IsStream:         isStream,
		Group:            group,
		Ip: func() string {
			if needRecordIp {
				return c.ClientIP()
			}
			return ""
		}(),
		RequestId:         requestId,
		UpstreamRequestId: upstreamRequestId,
		Other:             otherStr,
	}
	err := LOG_DB.Create(log).Error
	if err != nil {
		logger.LogError(c, "failed to record log: "+err.Error())
	}

	statusCode := 0
	if other != nil {
		if sc, ok := other["status_code"]; ok {
			switch v := sc.(type) {
			case int:
				statusCode = v
			case float64:
				statusCode = int(v)
			}
		}
	}
	if modelName != "" && statusCode > 0 {
		RecordFailedTokenRecord(modelName, statusCode, log.CreatedAt)
	}
}

type RecordConsumeLogParams struct {
	ChannelId        int                    `json:"channel_id"`
	PromptTokens     int                    `json:"prompt_tokens"`
	CompletionTokens int                    `json:"completion_tokens"`
	ModelName        string                 `json:"model_name"`
	TokenName        string                 `json:"token_name"`
	Quota            int                    `json:"quota"`
	Content          string                 `json:"content"`
	TokenId          int                    `json:"token_id"`
	UseTimeSeconds   int                    `json:"use_time_seconds"`
	IsStream         bool                   `json:"is_stream"`
	Group            string                 `json:"group"`
	Other            map[string]interface{} `json:"other"`
	Record           string                 `json:"record"` // 消费日志详细记录
	FullLog          string                 `json:"full_log"`
	Tps              float64                `json:"tps"` // Tokens Per Second
}

func resolveTokenRecordModelName(recordModelName string, other map[string]interface{}) string {
	if other == nil {
		return recordModelName
	}
	upstreamModelName := strings.TrimSpace(common.Interface2String(other["upstream_model_name"]))
	if upstreamModelName == "" {
		return recordModelName
	}
	return upstreamModelName
}

func RecordConsumeLog(c *gin.Context, userId int, params RecordConsumeLogParams) {
	if !common.LogConsumeEnabled {
		return
	}
	logger.LogInfo(c, fmt.Sprintf("record consume log: userId=%d, params=%s", userId, common.GetJsonString(params)))
	username := c.GetString("username")
	requestId := c.GetString(common.RequestIdKey)
	createdAt := common.GetTimestamp()
	upstreamRequestId := c.GetString(common.UpstreamRequestIdKey)
	params.Other = AppendLogDetailSummaries(params.Other, params.Record)
	otherStr := common.MapToJsonStr(params.Other)
	// IP 记录永久开启，忽略用户设置
	needRecordIp := true
	log := &Log{
		UserId:           userId,
		Username:         username,
		CreatedAt:        createdAt,
		Type:             LogTypeConsume,
		Content:          params.Content,
		PromptTokens:     params.PromptTokens,
		CompletionTokens: params.CompletionTokens,
		TokenName:        params.TokenName,
		ModelName:        params.ModelName,
		Quota:            params.Quota,
		ChannelId:        params.ChannelId,
		TokenId:          params.TokenId,
		UseTime:          params.UseTimeSeconds,
		IsStream:         params.IsStream,
		Group:            params.Group,
		Ip: func() string {
			if needRecordIp {
				return c.ClientIP()
			}
			return ""
		}(),
		RequestId:         requestId,
		UpstreamRequestId: upstreamRequestId,
		Other:             otherStr,
		Record:            params.Record,
		FullLog:           params.FullLog,
		Tps:               params.Tps,
	}
	err := LOG_DB.Create(log).Error
	if err != nil {
		logger.LogError(c, "failed to record log: "+err.Error())
		return
	}
	tokenRecordModelName := resolveTokenRecordModelName(params.ModelName, params.Other)
	err = RecordTokenRecord(tokenRecordModelName, params.PromptTokens, params.CompletionTokens, params.UseTimeSeconds, createdAt)
	if err != nil {
		logger.LogError(c, "failed to record token record: "+err.Error())
	}
	if common.DataExportEnabled {
		gopool.Go(func() {
			LogQuotaData(userId, username, params.ModelName, params.Quota, createdAt, params.PromptTokens+params.CompletionTokens)
		})
	}
}

type RecordTaskBillingLogParams struct {
	UserId    int
	LogType   int
	Content   string
	ChannelId int
	ModelName string
	Quota     int
	TokenId   int
	Group     string
	Other     map[string]interface{}
}

func RecordTaskBillingLog(params RecordTaskBillingLogParams) {
	if params.LogType == LogTypeConsume && !common.LogConsumeEnabled {
		return
	}
	username, _ := GetUsernameById(params.UserId, false)
	tokenName := ""
	if params.TokenId > 0 {
		if token, err := GetTokenById(params.TokenId); err == nil {
			tokenName = token.Name
		}
	}
	log := &Log{
		UserId:    params.UserId,
		Username:  username,
		CreatedAt: common.GetTimestamp(),
		Type:      params.LogType,
		Content:   params.Content,
		TokenName: tokenName,
		ModelName: params.ModelName,
		Quota:     params.Quota,
		ChannelId: params.ChannelId,
		TokenId:   params.TokenId,
		Group:     params.Group,
		Other:     common.MapToJsonStr(params.Other),
	}
	err := LOG_DB.Create(log).Error
	if err != nil {
		common.SysLog("failed to record task billing log: " + err.Error())
	}
}

func GetAllLogs(logType int, startTimestamp int64, endTimestamp int64, modelName string, username string, tokenName string, startIdx int, num int, channel int, group string, requestId string, upstreamRequestId string) (logs []*Log, total int64, err error) {
	var tx *gorm.DB
	if logType == LogTypeUnknown {
		tx = LOG_DB
	} else {
		tx = LOG_DB.Where("logs.type = ?", logType)
	}

	if tx, err = applyExplicitLogTextFilter(tx, "logs.model_name", modelName); err != nil {
		return nil, 0, err
	}
	if tx, err = applyExplicitLogTextFilter(tx, "logs.username", username); err != nil {
		return nil, 0, err
	}
	if tokenName != "" {
		tx = tx.Where("logs.token_name = ?", tokenName)
	}
	if requestId != "" {
		tx = tx.Where("logs.request_id = ?", requestId)
	}
	if upstreamRequestId != "" {
		tx = tx.Where("logs.upstream_request_id = ?", upstreamRequestId)
	}
	if startTimestamp != 0 {
		tx = tx.Where("logs.created_at >= ?", startTimestamp)
	}
	if endTimestamp != 0 {
		tx = tx.Where("logs.created_at <= ?", endTimestamp)
	}
	if channel != 0 {
		tx = tx.Where("logs.channel_id = ?", channel)
	}
	if group != "" {
		tx = tx.Where("logs."+logGroupCol+" = ?", group)
	}
	err = tx.Model(&Log{}).Count(&total).Error
	if err != nil {
		return nil, 0, err
	}
	err = tx.Order("logs.created_at desc, logs.id desc").Limit(num).Offset(startIdx).Find(&logs).Error
	if err != nil {
		return nil, 0, err
	}

	channelIds := types.NewSet[int]()
	for _, log := range logs {
		if log.ChannelId != 0 {
			channelIds.Add(log.ChannelId)
		}
	}

	if channelIds.Len() > 0 {
		var channels []struct {
			Id   int    `gorm:"column:id"`
			Name string `gorm:"column:name"`
		}
		if common.MemoryCacheEnabled {
			// Cache get channel
			for _, channelId := range channelIds.Items() {
				if cacheChannel, err := CacheGetChannel(channelId); err == nil {
					channels = append(channels, struct {
						Id   int    `gorm:"column:id"`
						Name string `gorm:"column:name"`
					}{
						Id:   channelId,
						Name: cacheChannel.Name,
					})
				}
			}
		} else {
			// Bulk query channels from DB
			if err = DB.Table("channels").Select("id, name").Where("id IN ?", channelIds.Items()).Find(&channels).Error; err != nil {
				return logs, total, err
			}
		}
		channelMap := make(map[int]string, len(channels))
		for _, channel := range channels {
			channelMap[channel.Id] = channel.Name
		}
		for i := range logs {
			logs[i].ChannelName = channelMap[logs[i].ChannelId]
		}
	}

	appendAdminLogSummaries(logs)
	return logs, total, err
}

func appendAdminLogSummaries(logs []*Log) {
	for i := range logs {
		sourceFromRecord, interactionFromRecord, agentIdFromRecord, sessionIdFromRecord, parentSessionIdFromRecord := ExtractLogDetailSummaries(logs[i].Record)
		if sourceFromRecord == "" && interactionFromRecord == "" && agentIdFromRecord == "" && sessionIdFromRecord == "" && parentSessionIdFromRecord == "" {
			continue
		}

		otherMap := map[string]interface{}{}
		if logs[i].Other != "" {
			parsedOtherMap, err := common.StrToMap(logs[i].Other)
			if err != nil {
				logger.LogWarn(context.TODO(), fmt.Sprintf("appendAdminLogSummaries: failed to parse other field: %v", err))
			} else {
				otherMap = parsedOtherMap
			}
		}

		if sourceFromRecord != "" && strings.TrimSpace(common.Interface2String(otherMap[LogOtherClientSourceKey])) == "" {
			otherMap[LogOtherClientSourceKey] = sourceFromRecord
		}
		// interaction_type: always overwrite from record inference.
		// Earlier versions stored stale values computed by a buggy inference path
		// (bamboo logs were misclassified as "输入"). Overwriting on read ensures
		// historical data is corrected once the running binary has the fix.
		if interactionFromRecord != "" {
			otherMap[LogOtherInteractionTypeKey] = interactionFromRecord
		}
		if agentIdFromRecord != "" && strings.TrimSpace(common.Interface2String(otherMap[LogOtherAgentIdKey])) == "" {
			otherMap[LogOtherAgentIdKey] = agentIdFromRecord
			otherMap[LogOtherAgentNameKey] = naming.AgentName(agentIdFromRecord)
		}
		if sessionIdFromRecord != "" && strings.TrimSpace(common.Interface2String(otherMap[LogOtherSessionIdKey])) == "" {
			otherMap[LogOtherSessionIdKey] = sessionIdFromRecord
			otherMap[LogOtherSessionNameKey] = naming.SessionName(sessionIdFromRecord)
		}
		if parentSessionIdFromRecord != "" && strings.TrimSpace(common.Interface2String(otherMap[LogOtherParentSessionIdKey])) == "" {
			otherMap[LogOtherParentSessionIdKey] = parentSessionIdFromRecord
			otherMap[LogOtherParentSessionNameKey] = naming.SessionName(parentSessionIdFromRecord)
		}
		if len(otherMap) > 0 {
			logs[i].Other = common.MapToJsonStr(otherMap)
		}
	}
}

const logSearchCountLimit = 10000

func GetUserLogs(userId int, logType int, startTimestamp int64, endTimestamp int64, modelName string, tokenName string, startIdx int, num int, group string, requestId string, upstreamRequestId string) (logs []*Log, total int64, err error) {
	var tx *gorm.DB
	if logType == LogTypeUnknown {
		tx = LOG_DB.Where("logs.user_id = ?", userId)
	} else {
		tx = LOG_DB.Where("logs.user_id = ? and logs.type = ?", userId, logType)
	}

	if tx, err = applyExplicitLogTextFilter(tx, "logs.model_name", modelName); err != nil {
		return nil, 0, err
	}
	if tokenName != "" {
		tx = tx.Where("logs.token_name = ?", tokenName)
	}
	if requestId != "" {
		tx = tx.Where("logs.request_id = ?", requestId)
	}
	if upstreamRequestId != "" {
		tx = tx.Where("logs.upstream_request_id = ?", upstreamRequestId)
	}
	if startTimestamp != 0 {
		tx = tx.Where("logs.created_at >= ?", startTimestamp)
	}
	if endTimestamp != 0 {
		tx = tx.Where("logs.created_at <= ?", endTimestamp)
	}
	if group != "" {
		tx = tx.Where("logs."+logGroupCol+" = ?", group)
	}
	err = tx.Model(&Log{}).Limit(logSearchCountLimit).Count(&total).Error
	if err != nil {
		common.SysError("failed to count user logs: " + err.Error())
		return nil, 0, errors.New("查询日志失败")
	}
	err = tx.Order("logs.id desc").Limit(num).Offset(startIdx).Find(&logs).Error
	if err != nil {
		common.SysError("failed to search user logs: " + err.Error())
		return nil, 0, errors.New("查询日志失败")
	}

	viewer := &User{Role: common.RoleCommonUser}
	if user, userErr := GetUserById(userId, false); userErr == nil {
		viewer = user
	} else {
		common.SysError("failed to load user when formatting logs: " + userErr.Error())
	}
	formatUserLogs(logs, startIdx, viewer)
	return logs, total, err
}

type Stat struct {
	Quota int `json:"quota"`
	Rpm   int `json:"rpm"`
	Tpm   int `json:"tpm"`
}

func SumUsedQuota(logType int, startTimestamp int64, endTimestamp int64, modelName string, username string, tokenName string, channel int, group string) (stat Stat, err error) {
	tx := LOG_DB.Table("logs").Select("sum(quota) quota")

	// 为rpm和tpm创建单独的查询
	rpmTpmQuery := LOG_DB.Table("logs").Select("count(*) rpm, sum(prompt_tokens) + sum(completion_tokens) tpm")

	if tx, err = applyExplicitLogTextFilter(tx, "username", username); err != nil {
		return stat, err
	}
	if rpmTpmQuery, err = applyExplicitLogTextFilter(rpmTpmQuery, "username", username); err != nil {
		return stat, err
	}
	if tokenName != "" {
		tx = tx.Where("token_name = ?", tokenName)
		rpmTpmQuery = rpmTpmQuery.Where("token_name = ?", tokenName)
	}
	if startTimestamp != 0 {
		tx = tx.Where("created_at >= ?", startTimestamp)
	}
	if endTimestamp != 0 {
		tx = tx.Where("created_at <= ?", endTimestamp)
	}
	if tx, err = applyExplicitLogTextFilter(tx, "model_name", modelName); err != nil {
		return stat, err
	}
	if rpmTpmQuery, err = applyExplicitLogTextFilter(rpmTpmQuery, "model_name", modelName); err != nil {
		return stat, err
	}
	if channel != 0 {
		tx = tx.Where("channel_id = ?", channel)
		rpmTpmQuery = rpmTpmQuery.Where("channel_id = ?", channel)
	}
	if group != "" {
		tx = tx.Where(logGroupCol+" = ?", group)
		rpmTpmQuery = rpmTpmQuery.Where(logGroupCol+" = ?", group)
	}

	tx = tx.Where("type = ?", LogTypeConsume)
	rpmTpmQuery = rpmTpmQuery.Where("type = ?", LogTypeConsume)

	// 只统计最近60秒的rpm和tpm
	rpmTpmQuery = rpmTpmQuery.Where("created_at >= ?", time.Now().Add(-60*time.Second).Unix())

	// 执行查询
	if err := tx.Scan(&stat).Error; err != nil {
		common.SysError("failed to query log stat: " + err.Error())
		return stat, errors.New("查询统计数据失败")
	}
	if err := rpmTpmQuery.Scan(&stat).Error; err != nil {
		common.SysError("failed to query rpm/tpm stat: " + err.Error())
		return stat, errors.New("查询统计数据失败")
	}

	return stat, nil
}

func SumUsedToken(logType int, startTimestamp int64, endTimestamp int64, modelName string, username string, tokenName string) (token int) {
	tx := LOG_DB.Table("logs").Select("ifnull(sum(prompt_tokens),0) + ifnull(sum(completion_tokens),0)")
	if username != "" {
		tx = tx.Where("username = ?", username)
	}
	if tokenName != "" {
		tx = tx.Where("token_name = ?", tokenName)
	}
	if startTimestamp != 0 {
		tx = tx.Where("created_at >= ?", startTimestamp)
	}
	if endTimestamp != 0 {
		tx = tx.Where("created_at <= ?", endTimestamp)
	}
	if modelName != "" {
		tx = tx.Where("model_name = ?", modelName)
	}
	tx.Where("type = ?", LogTypeConsume).Scan(&token)
	return token
}

func DeleteOldLog(ctx context.Context, targetTimestamp int64, limit int) (int64, error) {
	var total int64 = 0

	for {
		if nil != ctx.Err() {
			return total, ctx.Err()
		}

		result := LOG_DB.Where("created_at < ?", targetTimestamp).Limit(limit).Delete(&Log{})
		if nil != result.Error {
			return total, result.Error
		}

		total += result.RowsAffected

		if result.RowsAffected < int64(limit) {
			break
		}
	}

	return total, nil
}

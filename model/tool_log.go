package model

import (
	"context"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/types"

	"gorm.io/gorm"
)

type ToolLog struct {
	Id           int    `json:"id" gorm:"index:idx_tool_logs_created_at_id,priority:2;index:idx_tool_logs_user_id_id,priority:2"`
	CreatedAt    int64  `json:"created_at" gorm:"bigint;index:idx_tool_logs_created_at_id,priority:1"`
	UserId       int    `json:"user_id" gorm:"index;index:idx_tool_logs_user_id_id,priority:1"`
	Username     string `json:"username" gorm:"index;default:''"`
	TokenId      int    `json:"token_id" gorm:"index;default:0"`
	TokenName    string `json:"token_name" gorm:"index;default:''"`
	ChannelId    int    `json:"channel" gorm:"column:channel_id;index"`
	ChannelName  string `json:"channel_name" gorm:"-"`
	Group        string `json:"group" gorm:"index"`
	ModelName    string `json:"model_name" gorm:"index;default:''"`
	RequestId    string `json:"request_id" gorm:"type:varchar(64);index;default:''"`
	Ip           string `json:"ip" gorm:"index;default:''"`
	OriginalName string `json:"original_name" gorm:"index;default:''"`
	Canonical    string `json:"canonical" gorm:"index;default:''"`
	Kind         string `json:"kind" gorm:"index;default:''"`
	Mode         string `json:"mode" gorm:"default:''"`
	Backend      string `json:"backend" gorm:"default:''"`
	Query        string `json:"query" gorm:"type:text"`
	URL          string `json:"url" gorm:"type:text"`
	ErrorCode    string `json:"error_code" gorm:"index;default:''"`
	DurationMs   int64  `json:"duration_ms" gorm:"default:0"`
	Truncated    bool   `json:"truncated"`
	Result       string `json:"result" gorm:"type:text"`
}

func (ToolLog) TableName() string {
	return "tool_logs"
}

type ToolLogQuery struct {
	UserId      int
	StartAt     int64
	EndAt       int64
	Kind        string
	Canonical   string
	ErrorFilter string // "", "ok", "error", or a specific error_code
	ModelName   string
	TokenName   string
	Username    string
	Channel     int
	Group       string
	RequestId   string
	Q           string
	StartIdx    int
	Num         int
}

func RecordToolLogs(logs []*ToolLog) {
	if len(logs) == 0 || LOG_DB == nil {
		return
	}
	now := common.GetTimestamp()
	for _, log := range logs {
		if log == nil {
			continue
		}
		if log.CreatedAt == 0 {
			log.CreatedAt = now
		}
		if log.RequestId == "" {
			log.RequestId = common.NewRequestId()
		}
	}
	if err := LOG_DB.Create(logs).Error; err != nil {
		common.SysError("failed to record tool logs: " + err.Error())
	}
}

func GetAllToolLogs(q ToolLogQuery) (logs []*ToolLog, total int64, err error) {
	return queryToolLogs(LOG_DB, q)
}

func GetUserToolLogs(userId int, q ToolLogQuery) (logs []*ToolLog, total int64, err error) {
	q.UserId = userId
	q.Username = ""
	q.Channel = 0
	return queryToolLogs(LOG_DB.Where("tool_logs.user_id = ?", userId), q)
}

func queryToolLogs(tx *gorm.DB, q ToolLogQuery) (logs []*ToolLog, total int64, err error) {
	if q.Kind != "" {
		tx = tx.Where("tool_logs.kind = ?", q.Kind)
	}
	if q.Canonical != "" {
		tx = tx.Where("tool_logs.canonical = ?", q.Canonical)
	}
	switch q.ErrorFilter {
	case "ok":
		tx = tx.Where("tool_logs.error_code = ?", "")
	case "error":
		tx = tx.Where("tool_logs.error_code <> ?", "")
	case "":
	default:
		tx = tx.Where("tool_logs.error_code = ?", q.ErrorFilter)
	}
	if tx, err = applyExplicitLogTextFilter(tx, "tool_logs.model_name", q.ModelName); err != nil {
		return nil, 0, err
	}
	if tx, err = applyExplicitLogTextFilter(tx, "tool_logs.username", q.Username); err != nil {
		return nil, 0, err
	}
	if q.TokenName != "" {
		tx = tx.Where("tool_logs.token_name = ?", q.TokenName)
	}
	if q.RequestId != "" {
		tx = tx.Where("tool_logs.request_id = ?", q.RequestId)
	}
	if q.StartAt != 0 {
		tx = tx.Where("tool_logs.created_at >= ?", q.StartAt)
	}
	if q.EndAt != 0 {
		tx = tx.Where("tool_logs.created_at <= ?", q.EndAt)
	}
	if q.Channel != 0 {
		tx = tx.Where("tool_logs.channel_id = ?", q.Channel)
	}
	if q.Group != "" {
		tx = tx.Where("tool_logs."+logGroupCol+" = ?", q.Group)
	}
	if strings.TrimSpace(q.Q) != "" {
		needle := strings.TrimSpace(q.Q)
		if strings.Contains(needle, "%") {
			condition, pattern, ferr := buildLogLikeCondition("tool_logs.query", needle)
			if ferr != nil {
				return nil, 0, ferr
			}
			urlCond, urlPattern, uerr := buildLogLikeCondition("tool_logs.url", needle)
			if uerr != nil {
				return nil, 0, uerr
			}
			tx = tx.Where("("+condition+" OR "+urlCond+")", pattern, urlPattern)
		} else {
			tx = tx.Where("(tool_logs.query = ? OR tool_logs.url = ?)", needle, needle)
		}
	}

	err = tx.Model(&ToolLog{}).Count(&total).Error
	if err != nil {
		return nil, 0, err
	}
	order := "tool_logs.created_at desc, tool_logs.id desc"
	if common.UsingLogDatabase(common.DatabaseTypeClickHouse) {
		order = "tool_logs.created_at desc, tool_logs.request_id desc"
	}
	err = tx.Order(order).Limit(q.Num).Offset(q.StartIdx).Find(&logs).Error
	if err != nil {
		return nil, 0, err
	}
	if common.UsingLogDatabase(common.DatabaseTypeClickHouse) {
		for i := range logs {
			logs[i].Id = q.StartIdx + i + 1
		}
	}
	attachToolLogChannelNames(logs)
	return logs, total, nil
}

func attachToolLogChannelNames(logs []*ToolLog) {
	channelIds := types.NewSet[int]()
	for _, log := range logs {
		if log != nil && log.ChannelId != 0 {
			channelIds.Add(log.ChannelId)
		}
	}
	if channelIds.Len() == 0 {
		return
	}
	channelMap := make(map[int]string, channelIds.Len())
	if common.MemoryCacheEnabled {
		for _, channelId := range channelIds.Items() {
			if cacheChannel, err := CacheGetChannel(channelId); err == nil && cacheChannel != nil {
				channelMap[channelId] = cacheChannel.Name
			}
		}
	} else {
		var channels []struct {
			Id   int    `gorm:"column:id"`
			Name string `gorm:"column:name"`
		}
		if err := DB.Table("channels").Select("id, name").Where("id IN ?", channelIds.Items()).Find(&channels).Error; err != nil {
			return
		}
		for _, channel := range channels {
			channelMap[channel.Id] = channel.Name
		}
	}
	for i := range logs {
		logs[i].ChannelName = channelMap[logs[i].ChannelId]
	}
}

func CountOldToolLog(ctx context.Context, targetTimestamp int64) (int64, error) {
	var total int64
	if err := LOG_DB.WithContext(ctx).Model(&ToolLog{}).Where("created_at < ?", targetTimestamp).Count(&total).Error; err != nil {
		return 0, err
	}
	return total, nil
}

func deleteOldToolLogBatch(ctx context.Context, targetTimestamp int64, limit int) (int64, error) {
	if common.UsingLogDatabase(common.DatabaseTypeClickHouse) {
		total, err := CountOldToolLog(ctx, targetTimestamp)
		if err != nil {
			return 0, err
		}
		if total == 0 {
			return 0, nil
		}
		if err := LOG_DB.WithContext(ctx).Exec(
			"ALTER TABLE tool_logs DELETE WHERE created_at < ? SETTINGS mutations_sync = 1",
			targetTimestamp,
		).Error; err != nil {
			return 0, err
		}
		return total, nil
	}
	result := LOG_DB.WithContext(ctx).Where("created_at < ?", targetTimestamp).Limit(limit).Delete(&ToolLog{})
	if result.Error != nil {
		return 0, result.Error
	}
	return result.RowsAffected, nil
}

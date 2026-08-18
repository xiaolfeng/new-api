package controller

import (
	"strconv"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"

	"github.com/gin-gonic/gin"
)

func parseToolLogQuery(c *gin.Context) model.ToolLogQuery {
	pageInfo := common.GetPageQuery(c)
	channel, _ := strconv.Atoi(c.Query("channel"))
	startTimestamp, _ := strconv.ParseInt(c.Query("start_timestamp"), 10, 64)
	endTimestamp, _ := strconv.ParseInt(c.Query("end_timestamp"), 10, 64)
	return model.ToolLogQuery{
		StartAt:     startTimestamp,
		EndAt:       endTimestamp,
		Kind:        c.Query("kind"),
		Canonical:   c.Query("canonical"),
		ErrorFilter: c.Query("error"),
		ModelName:   c.Query("model_name"),
		TokenName:   c.Query("token_name"),
		Username:    c.Query("username"),
		Channel:     channel,
		Group:       c.Query("group"),
		RequestId:   c.Query("request_id"),
		Q:           c.Query("q"),
		StartIdx:    pageInfo.GetStartIdx(),
		Num:         pageInfo.GetPageSize(),
	}
}

func GetAllToolLogs(c *gin.Context) {
	pageInfo := common.GetPageQuery(c)
	logs, total, err := model.GetAllToolLogs(parseToolLogQuery(c))
	if err != nil {
		common.ApiError(c, err)
		return
	}
	pageInfo.SetTotal(int(total))
	pageInfo.SetItems(logs)
	common.ApiSuccess(c, pageInfo)
}

func GetUserToolLogs(c *gin.Context) {
	pageInfo := common.GetPageQuery(c)
	userId := c.GetInt("id")
	q := parseToolLogQuery(c)
	logs, total, err := model.GetUserToolLogs(userId, q)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	pageInfo.SetTotal(int(total))
	pageInfo.SetItems(logs)
	common.ApiSuccess(c, pageInfo)
}

package controller

import (
	"strconv"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/gin-gonic/gin"
)

func GetRecentTokenRecords(c *gin.Context) {
	hoursStr := c.DefaultQuery("hours", "24")
	hours, _ := strconv.ParseInt(hoursStr, 10, 64)
	if hours <= 0 {
		hours = 24
	}

	snapshot, err := model.GetRecentTokenRecordSnapshot(common.GetTimestamp(), hours)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, snapshot)
}

func GetDailyTokenRecords(c *gin.Context) {
	now := common.GetTimestamp()
	startTime := now - 365*24*3600
	endTime := now

	items, err := model.GetDailyTokenSummary(startTime, endTime)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, items)
}

func GetSelfDailyTokenRecords(c *gin.Context) {
	userId := c.GetInt("id")

	now := common.GetTimestamp()
	startTime := now - 365*24*3600
	endTime := now

	items, err := model.GetUserDailyTokenSummary(userId, startTime, endTime)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, items)
}

package controller

import (
	"errors"
	"net/http/httptest"
	"testing"

	"github.com/QuantumNous/new-api/relaykit/types"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
)

func newShouldRetryTestContext() *gin.Context {
	gin.SetMode(gin.TestMode)
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	return c
}

func newWrittenShouldRetryTestContext() *gin.Context {
	c := newShouldRetryTestContext()
	c.Writer.WriteHeaderNow()
	return c
}

// shouldRetry 是「失败已按部分交付结算」后防止重复计费与响应流损坏的闸门：
// 响应一旦开始写出就不得再换渠道重试，empty_response 是唯一例外。
func TestShouldRetry(t *testing.T) {
	badResponseErr := types.NewError(errors.New("stream broken"), types.ErrorCodeBadResponseBody)
	upstreamErr := types.NewError(errors.New("upstream 500"), types.ErrorCode("upstream:test"))
	channelErr := types.NewError(errors.New("channel down"), types.ErrorCode("channel:test"))
	skipRetryErr := types.NewError(errors.New("no retry"), types.ErrorCodeInvalidRequest, types.ErrOptionWithSkipRetry())
	emptyResponseErr := types.NewError(errors.New("empty response"), types.ErrorCodeEmptyResponse)

	tests := []struct {
		name       string
		ctx        func() *gin.Context
		err        *types.NewAPIError
		retryTimes int
		want       bool
	}{
		{
			name: "nil error never retries",
			ctx:  newShouldRetryTestContext,
			err:  nil,
			want: false,
		},
		{
			name:       "retriable upstream error retries when response not written",
			ctx:        newShouldRetryTestContext,
			err:        upstreamErr,
			retryTimes: 1,
			want:       true,
		},
		{
			name:       "written response blocks retry even on retriable status",
			ctx:        newWrittenShouldRetryTestContext,
			err:        upstreamErr,
			retryTimes: 1,
			want:       false,
		},
		{
			name:       "bad response body never retries by existing invariant",
			ctx:        newShouldRetryTestContext,
			err:        badResponseErr,
			retryTimes: 1,
			want:       false,
		},
		{
			name: "empty response retry does not bypass pinned channel",
			ctx: func() *gin.Context {
				c := newWrittenShouldRetryTestContext()
				c.Set("specific_channel_id", 42)
				return c
			},
			err:        emptyResponseErr,
			retryTimes: 1,
			want:       false,
		},
		{
			name:       "channel error retries when response not written",
			ctx:        newShouldRetryTestContext,
			err:        channelErr,
			retryTimes: 1,
			want:       true,
		},
		{
			name:       "skip retry error honored",
			ctx:        newShouldRetryTestContext,
			err:        skipRetryErr,
			retryTimes: 1,
			want:       false,
		},
		{
			name:       "no retry budget left",
			ctx:        newShouldRetryTestContext,
			err:        upstreamErr,
			retryTimes: 0,
			want:       false,
		},
		{
			name:       "empty response retries even if written",
			ctx:        newWrittenShouldRetryTestContext,
			err:        emptyResponseErr,
			retryTimes: 1,
			want:       true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, shouldRetry(tt.ctx(), tt.err, tt.retryTimes))
		})
	}
}

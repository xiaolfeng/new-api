package types

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
)

// MarkSkipRetry 是失败结算路径阻止换渠道重试的契约入口：
// 结算过部分交付的错误必须能就地标记不可重试。
func TestMarkSkipRetry(t *testing.T) {
	t.Run("nil receiver is safe", func(t *testing.T) {
		var e *NewAPIError
		assert.NotPanics(t, func() { e.MarkSkipRetry() })
	})

	t.Run("marks error as skip retry", func(t *testing.T) {
		e := NewError(errors.New("boom"), ErrorCodeBadResponseBody)
		assert.False(t, IsSkipRetryError(e))
		e.MarkSkipRetry()
		assert.True(t, IsSkipRetryError(e))
	})

	t.Run("keeps existing skip retry flag", func(t *testing.T) {
		e := NewError(errors.New("boom"), ErrorCodeInvalidRequest, ErrOptionWithSkipRetry())
		e.MarkSkipRetry()
		assert.True(t, IsSkipRetryError(e))
	})
}

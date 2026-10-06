package common

import (
	"sync"
	"time"
)

// DeliveryTimingResult 表示服务端响应写出的请求级计时，不表示客户端接收时间。
type DeliveryTimingResult struct {
	Version int    `json:"version"`
	Source  string `json:"source"`
	TotalMs int64  `json:"total_ms"`
	TTFTMs  *int64 `json:"ttft_ms"`
	Status  string `json:"status"`
}

// DeliveryTiming 跨跳保留同一请求起点，收尾幂等，零首字和缺失首字分离。
type DeliveryTiming struct {
	// WriteMu 串行化业务帧、心跳、响应头和收尾，避免写出与计时顺序不一致。
	WriteMu   sync.Mutex
	mu        sync.Mutex
	start     time.Time
	first     time.Time
	end       time.Time
	status    string
	committed bool
}

func NewDeliveryTiming(start time.Time) *DeliveryTiming {
	return &DeliveryTiming{start: start}
}

// RecordContent 仅在完整有效内容成功写出后调用。
func (t *DeliveryTiming) RecordContent(at time.Time) bool {
	t.mu.Lock()
	defer t.mu.Unlock()
	if !t.end.IsZero() || !t.first.IsZero() {
		return false
	}
	t.first = at
	return true
}

// Fail 保留最早失败，写出失败优先于上游错误和取消。
func (t *DeliveryTiming) Fail(status string) {
	t.mu.Lock()
	defer t.mu.Unlock()
	if !t.end.IsZero() {
		return
	}
	if t.status == "" || status == "write_error" {
		t.status = status
	}
}

func (t *DeliveryTiming) Finish(at time.Time, status string) {
	t.mu.Lock()
	defer t.mu.Unlock()
	if !t.end.IsZero() {
		return
	}
	t.end = at
	if t.status == "" {
		t.status = status
	}
}

func (t *DeliveryTiming) Finished() bool {
	t.mu.Lock()
	defer t.mu.Unlock()
	return !t.end.IsZero()
}

// Result 不钳制异常数据，已收尾但不满足不变量时返回无效标记供日志诊断。
func (t *DeliveryTiming) Result() (*DeliveryTimingResult, bool) {
	if t == nil {
		return nil, true
	}
	t.mu.Lock()
	defer t.mu.Unlock()
	if t.end.IsZero() {
		return nil, true
	}
	if t.start.IsZero() || t.end.Before(t.start) || (!t.first.IsZero() && (t.first.Before(t.start) || t.first.After(t.end))) {
		return nil, false
	}
	r := &DeliveryTimingResult{Version: 1, Source: "server_delivery", TotalMs: t.end.Sub(t.start).Milliseconds(), Status: t.status}
	if !t.first.IsZero() {
		ms := t.first.Sub(t.start).Milliseconds()
		r.TTFTMs = &ms
	}
	return r, true
}

// AverageTPS 和前端使用同一毫秒分母；短于 100ms 的流式吞吐不可可靠测量。
func (r *DeliveryTimingResult) AverageTPS(tokens int, stream bool) (float64, bool) {
	if r == nil || tokens <= 0 || r.TotalMs <= 0 || r.TTFTMs == nil {
		return 0, false
	}
	ms := r.TotalMs
	if stream {
		ms -= *r.TTFTMs
		if ms < 100 {
			return 0, false
		}
	}
	return float64(tokens) * 1000 / float64(ms), true
}

// MarkCommitted 区分已发送信封/响应头与首个有效内容，避免已交付响应被重试。
func (t *DeliveryTiming) MarkCommitted() {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.committed = true
}

func (t *DeliveryTiming) Committed() bool {
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.committed
}

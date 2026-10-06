package common

import (
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDeliveryTimingPrecisionAndFirstContent(t *testing.T) {
	for _, offset := range []time.Duration{50 * time.Millisecond, 950 * time.Millisecond} {
		t.Run(offset.String(), func(t *testing.T) {
			start := time.Unix(100, 0).Add(offset)
			timing := NewDeliveryTiming(start)
			require.True(t, timing.RecordContent(start.Add(7079*time.Millisecond)))
			assert.False(t, timing.RecordContent(start.Add(7090*time.Millisecond)))
			timing.Finish(start.Add(7109*time.Millisecond), "completed")
			timing.Finish(start.Add(9*time.Second), "upstream_error")
			result, valid := timing.Result()
			require.True(t, valid)
			require.NotNil(t, result)
			require.NotNil(t, result.TTFTMs)
			assert.Equal(t, int64(7079), *result.TTFTMs)
			assert.Equal(t, int64(7109), result.TotalMs)
			assert.Equal(t, "completed", result.Status)
			_, reliable := result.AverageTPS(100, true)
			assert.False(t, reliable)
		})
	}
}

func TestDeliveryTimingMissingZeroAndInvalid(t *testing.T) {
	start := time.Unix(100, 0)
	for _, tc := range []struct {
		name  string
		first time.Time
		end   time.Time
		valid bool
	}{
		{"missing", time.Time{}, start.Add(time.Second), true},
		{"zero", start, start.Add(time.Second), true},
		{"before_start", start.Add(-time.Millisecond), start.Add(time.Second), false},
		{"after_end", start.Add(2 * time.Second), start.Add(time.Second), false},
		{"negative_total", time.Time{}, start.Add(-time.Second), false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			timing := NewDeliveryTiming(start)
			if !tc.first.IsZero() {
				timing.RecordContent(tc.first)
			}
			timing.Finish(tc.end, "completed")
			result, valid := timing.Result()
			require.Equal(t, tc.valid, valid)
			if !valid {
				assert.Nil(t, result)
				return
			}
			require.NotNil(t, result)
			if tc.first.IsZero() {
				assert.Nil(t, result.TTFTMs)
			} else {
				require.NotNil(t, result.TTFTMs)
				assert.Zero(t, *result.TTFTMs)
			}
		})
	}
}

func TestDeliveryTimingConcurrentFailureAndContent(t *testing.T) {
	start := time.Unix(100, 0)
	timing := NewDeliveryTiming(start)
	var wg sync.WaitGroup
	wg.Add(2)
	go func() { defer wg.Done(); timing.RecordContent(start.Add(time.Second)) }()
	go func() { defer wg.Done(); timing.Fail("write_error") }()
	wg.Wait()
	timing.Finish(start.Add(2*time.Second), "cancelled")
	require.False(t, timing.RecordContent(start.Add(3*time.Second)))
	r, valid := timing.Result()
	require.True(t, valid)
	assert.Equal(t, "write_error", r.Status)
	assert.Equal(t, int64(2000), r.TotalMs)
}

func TestDeliveryAverageTPS(t *testing.T) {
	first := int64(7079)
	for _, tc := range []struct {
		name   string
		total  int64
		first  *int64
		stream bool
		tokens int
		want   float64
		valid  bool
	}{
		{"slow_delivery", 8079, &first, true, 100, 100, true},
		{"threshold", 7179, &first, true, 10, 100, true},
		{"short_tail", 7109, &first, true, 100, 0, false},
		{"zero_tail", 7079, &first, true, 100, 0, false},
		{"no_content", 8079, nil, true, 100, 0, false},
		{"nonstream", 8000, &first, false, 100, 12.5, true},
		{"no_tokens", 8079, &first, true, 0, 0, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			r := &DeliveryTimingResult{TotalMs: tc.total, TTFTMs: tc.first}
			got, valid := r.AverageTPS(tc.tokens, tc.stream)
			assert.Equal(t, tc.valid, valid)
			assert.Equal(t, tc.want, got)
		})
	}
}

package bamboo

import (
	"context"
	"time"

	bamboocodec "github.com/bamboo-services/bamboo-messages/bamboo/codec"
	bamboorelay "github.com/bamboo-services/bamboo-messages/bamboo/relay"

	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/setting/model_setting"
)

// smoothPresetParams 是各档位的预设参数，与 bamboo-messages SDK 的 presetParams 保持一致。
//
// SDK v0.4.9 的 presetParams 为包私有，无法直接引用，
// 这里以值副本形式内联，参数值来自 SDK smooth.go:34-65。
var smoothPresetParams = map[dto.BambooSmoothLevelType]bamboorelay.SmoothParams{
	dto.BambooSmoothLevelGentle: {
		TokensPerFrame:  2,
		MinInterval:     20 * time.Millisecond,
		MaxInterval:     100 * time.Millisecond,
		EMAAlpha:        0.3,
		DrainTier1Ratio: 0.5,
		DrainTier1Mult:  0.6,
		DrainTier2Ratio: 0.2,
		DrainTier2Mult:  0.3,
	},
	dto.BambooSmoothLevelSmooth: {
		TokensPerFrame:  1,
		MinInterval:     15 * time.Millisecond,
		MaxInterval:     80 * time.Millisecond,
		EMAAlpha:        0.25,
		DrainTier1Ratio: 0.5,
		DrainTier1Mult:  0.5,
		DrainTier2Ratio: 0.2,
		DrainTier2Mult:  0.25,
	},
	dto.BambooSmoothLevelTypewriter: {
		TokensPerFrame:  1,
		MinInterval:     30 * time.Millisecond,
		MaxInterval:     120 * time.Millisecond,
		EMAAlpha:        0.2,
		DrainTier1Ratio: 0.5,
		DrainTier1Mult:  0.7,
		DrainTier2Ratio: 0.2,
		DrainTier2Mult:  0.4,
	},
}

// resolveSmoothLevel 从全局 BambooSettings 解析平滑缓冲档位。
//
// 读取 model_setting.GetBambooSettings().SmoothLevel，仅接受有效枚举值，
// 空字符串/"off"/未知值返回 BambooSmoothLevelOff（关闭平滑缓冲）。
func resolveSmoothLevel() dto.BambooSmoothLevelType {
	level := dto.BambooSmoothLevelType(model_setting.GetBambooSettings().SmoothLevel)
	switch level {
	case dto.BambooSmoothLevelGentle,
		dto.BambooSmoothLevelSmooth,
		dto.BambooSmoothLevelTypewriter:
		return level
	default:
		return dto.BambooSmoothLevelOff
	}
}

// smoothBufferWriter 适配 SmoothPacer 的输出 channel 到 HTTP ResponseWriter。
//
// 启用平滑缓冲时，doStreamRelay 不再直接 c.Writer.Write(data)，
// 而是将序列化后的 SSE 帧推入 pacer（pacer.Push），
// pacer 按自适应间隔将微帧释放到 out channel，
// 此 goroutine 消费 out channel 写入 HTTP Response。
//
// 返回 push/SignalEnd/wait 三个操作句柄，调用方按以下顺序使用：
//  1. push(data)  — 每次序列化后调用
//  2. signalEnd() — 上游事件流结束后调用
//  3. wait()      — 等待 pacer 排空所有缓冲帧
type smoothBufferWriter struct {
	pacer     *bamboorelay.SmoothPacer
	out       chan []byte
	done      chan struct{}
	writeFunc func([]byte) bool
}

// startSmoothBuffer 启动平滑缓冲 goroutine，返回写入句柄。
//
// writeFunc 返回 false 表示写入失败（客户端断开），pacer 应停止。
// outFmt 用于初始化 FrameParser（决定 SSE 帧切分策略）。
func startSmoothBuffer(
	ctx context.Context,
	outFmt bamboocodec.FormatType,
	level dto.BambooSmoothLevelType,
	writeFunc func([]byte) bool,
) *smoothBufferWriter {
	out := make(chan []byte, 128)
	params, ok := smoothPresetParams[level]
	if !ok {
		params = smoothPresetParams[dto.BambooSmoothLevelGentle]
	}

	w := &smoothBufferWriter{
		out:       out,
		done:      make(chan struct{}),
		writeFunc: writeFunc,
	}
	w.pacer = bamboorelay.NewSmoothPacer(outFmt, params, out, ctx)

	go w.run(ctx)

	return w
}

func (w *smoothBufferWriter) run(ctx context.Context) {
	defer close(w.done)
	for {
		select {
		case data, ok := <-w.out:
			if !ok {
				return
			}
			if !w.writeFunc(data) {
				return
			}
		case <-ctx.Done():
			return
		}
	}
}

func (w *smoothBufferWriter) push(data []byte) {
	w.pacer.Push(data)
}

func (w *smoothBufferWriter) signalEnd() {
	w.pacer.SignalEnd()
}

func (w *smoothBufferWriter) wait() {
	w.pacer.Wait()
	close(w.out)
	<-w.done
}

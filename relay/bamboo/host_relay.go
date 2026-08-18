package bamboo

import (
	"context"
	"strings"
	"sync"
	"time"

	bamboosdk "github.com/bamboo-services/bamboo-messages/bamboo"
	bamboocodec "github.com/bamboo-services/bamboo-messages/bamboo/codec"
	bamboorelay "github.com/bamboo-services/bamboo-messages/bamboo/relay"
	"github.com/gin-gonic/gin"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/relay/bamboo/hosttool"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/relaykit/dto"
	"github.com/QuantumNous/new-api/relaykit/types"
	"github.com/QuantumNous/new-api/setting/model_setting"
)

func writeStreamHeaders(c *gin.Context) {
	c.Writer.Header().Set("Content-Type", "text/event-stream")
	c.Writer.Header().Set("Cache-Control", "no-cache")
	c.Writer.Header().Set("X-Accel-Buffering", "no")
	c.Writer.Flush()
}

func startHostPing(ctx context.Context, write func([]byte) bool, pingSer bamboocodec.StreamSerializer) (stop func()) {
	done := make(chan struct{})
	var once sync.Once
	go func() {
		ticker := time.NewTicker(10 * time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-done:
				return
			case <-ticker.C:
				data, err := pingSer.Serialize(bamboosdk.StreamEvent{Type: bamboosdk.EventPing})
				if err != nil || data == nil {
					continue
				}
				for _, frame := range bamboorelay.SplitSSEFrames(data) {
					if !write(frame) {
						return
					}
				}
			}
		}
	}()
	return func() { once.Do(func() { close(done) }) }
}

func doHostCompleteRelay(c *gin.Context, info *relaycommon.RelayInfo, client bamboosdk.BambooClient,
	entryCodec bamboocodec.Codec, codecFmt bamboocodec.FormatType, req *bamboocodec.RelayRequest) (*dto.Usage, *types.NewAPIError) {

	resp, err := client.Complete(c.Request.Context(), req.Messages, req.System, req.Config)
	if err != nil {
		if isClientCancel(c, err) {
			return nil, nil
		}
		return nil, translateSDKError(err)
	}
	info.SetFirstResponseTime()
	usage := usageFromBamboo(resp)
	blocks := hosttool.BlocksFromResponse(resp)
	recordResponseBlocks(info, blocks)

	uses := hosttool.CollectToolUses(blocks)
	action := hosttool.DecideAction(info.HostToolPlan, uses)
	if action == hosttool.ActionPassthrough {
		return writeCompleteResponse(c, info, entryCodec, codecFmt, resp, usage)
	}

	st := model_setting.GetBambooSettings()
	results := hosttool.ExecuteCalls(c.Request.Context(), info, st, uses)
	if action == hosttool.ActionFoldB {
		folded := hosttool.FoldResponse(resp.ID, resp.Model, blocks, results, resp.Usage)
		return writeCompleteResponse(c, info, entryCodec, codecFmt, folded, usage)
	}

	hop2Req := hosttool.BuildHop2Request(req, blocks, results, uses)
	if ok, reason := hosttool.AllowHop2(c, info, hop2Req); !ok {
		if info.HostToolPlan != nil {
			info.HostToolPlan.StoppedReason = reason
		}
		folded := hosttool.FoldResponse(resp.ID, resp.Model, blocks, results, resp.Usage)
		return writeCompleteResponse(c, info, entryCodec, codecFmt, folded, usage)
	}

	resp2, err := client.Complete(c.Request.Context(), hop2Req.Messages, hop2Req.System, hop2Req.Config)
	if err != nil {
		if isClientCancel(c, err) {
			return usage, nil
		}
		return usage, translateSDKError(err)
	}
	usage2 := usageFromBamboo(resp2)
	usage = hosttool.AddUsage(usage, usage2)
	blocks2 := hosttool.BlocksFromResponse(resp2)
	recordResponseBlocks(info, blocks2)
	uses2 := hosttool.CollectToolUses(blocks2)
	if hosttool.AllHostToolUses(info.HostToolPlan, uses2) && len(uses2) > 0 {
		results2 := hosttool.ExecuteCalls(c.Request.Context(), info, st, uses2)
		folded := hosttool.FoldResponse(resp2.ID, resp2.Model, blocks2, results2, resp2.Usage)
		return writeCompleteResponse(c, info, entryCodec, codecFmt, folded, usage)
	}
	return writeCompleteResponse(c, info, entryCodec, codecFmt, resp2, usage)
}

func writeCompleteResponse(c *gin.Context, info *relaycommon.RelayInfo, entryCodec bamboocodec.Codec,
	codecFmt bamboocodec.FormatType, resp *bamboosdk.Response, usage *dto.Usage) (*dto.Usage, *types.NewAPIError) {
	if info.HostToolExecuted {
		common.SetContextKey(c, constant.ContextKeyEmptyResponse, false)
	} else if resp != nil && len(resp.Content) == 0 && resp.Usage.OutputTokens == 0 {
		common.SetContextKey(c, constant.ContextKeyEmptyResponse, true)
	}
	prependVisibleBox(resp, info)
	body, serr := entryCodec.SerializeResponse(resp)
	if serr != nil {
		return usage, translateCodecError(serr)
	}
	if info.BambooDebug != nil {
		info.BambooDebug.RelayResponse = bamboorelay.FormatRelayResponse("CompleteRelay", codecFmt, codecFmt, body)
	}
	c.Writer.Header().Set("Content-Type", "application/json")
	_, _ = c.Writer.Write(body)
	info.ResponseBody = truncateResponseBody(string(body))
	return usage, nil
}

func usageFromBamboo(resp *bamboosdk.Response) *dto.Usage {
	if resp == nil {
		return &dto.Usage{UsageSemantic: "anthropic"}
	}
	nonCached := resp.Usage.InputTokens - resp.Usage.CacheReadInputTokens - resp.Usage.CacheCreationInputTokens
	if nonCached < 0 {
		nonCached = 0
	}
	return &dto.Usage{
		PromptTokens:     int(nonCached),
		CompletionTokens: int(resp.Usage.OutputTokens),
		TotalTokens:      int(nonCached + resp.Usage.OutputTokens),
		UsageSemantic:    "anthropic",
		PromptTokensDetails: dto.InputTokenDetails{
			CachedTokens:         int(resp.Usage.CacheReadInputTokens),
			CachedCreationTokens: int(resp.Usage.CacheCreationInputTokens),
		},
	}
}

func doHostStreamRelay(c *gin.Context, info *relaycommon.RelayInfo, client bamboosdk.BambooClient,
	entryCodec bamboocodec.Codec, outFmt bamboocodec.FormatType, req *bamboocodec.RelayRequest) (*dto.Usage, *types.NewAPIError) {

	ctx, cancel := context.WithCancel(c.Request.Context())
	defer cancel()
	go func() {
		<-c.Request.Context().Done()
		cancel()
	}()

	writeStreamHeaders(c)
	writeSSE := func(data []byte) bool {
		if _, werr := c.Writer.Write(data); werr != nil {
			return false
		}
		c.Writer.Flush()
		return true
	}

	modelName := ""
	if req.Config != nil {
		modelName = req.Config.Model
	}
	pingSer := entryCodec.NewSerializer(modelName)
	stopPing := startHostPing(ctx, writeSSE, pingSer)

	hop1, hopErr := collectStreamHop(ctx, c, info, client, entryCodec, outFmt, req, false, writeSSE)
	if hopErr != nil {
		stopPing()
		return hop1.usage, hopErr
	}
	if hop1.cancelled {
		stopPing()
		return hop1.usage, nil
	}

	uses := hosttool.CollectToolUses(hop1.blocks)
	action := hosttool.DecideAction(info.HostToolPlan, uses)
	if action == hosttool.ActionPassthrough {
		stopPing()
		return replayStreamHop(c, info, hop1, writeSSE)
	}

	st := model_setting.GetBambooSettings()
	results := hosttool.ExecuteCalls(ctx, info, st, uses)
	if action == hosttool.ActionFoldB {
		stopPing()
		return emitFoldedStream(c, info, entryCodec, modelName, hop1, results, writeSSE)
	}

	hop2Req := hosttool.BuildHop2Request(req, hop1.blocks, results, uses)
	if ok, reason := hosttool.AllowHop2(c, info, hop2Req); !ok {
		if info.HostToolPlan != nil {
			info.HostToolPlan.StoppedReason = reason
		}
		stopPing()
		return emitFoldedStream(c, info, entryCodec, modelName, hop1, results, writeSSE)
	}

	stopPing()
	hop2, hop2Err := collectStreamHop(ctx, c, info, client, entryCodec, outFmt, hop2Req, true, writeSSE)
	if hop2Err != nil {
		return hosttool.AddUsage(hop1.usage, hop2.usage), hop2Err
	}
	usage := hosttool.AddUsage(hop1.usage, hop2.usage)
	uses2 := hosttool.CollectToolUses(hop2.blocks)
	if hosttool.AllHostToolUses(info.HostToolPlan, uses2) && len(uses2) > 0 {
		results2 := hosttool.ExecuteCalls(ctx, info, st, uses2)
		return emitFoldedStream(c, info, entryCodec, modelName, hop2, results2, writeSSE)
	}
	if c.Request.Context().Err() != nil {
		ensureStreamStatus(info).SetEndReason(relaycommon.StreamEndReasonClientGone, c.Request.Context().Err())
	} else {
		ensureStreamStatus(info).SetEndReason(relaycommon.StreamEndReasonDone, nil)
	}
	return usage, nil
}

type collectedHop struct {
	blocks    []bamboosdk.ContentBlock
	frames    [][]byte
	usage     *dto.Usage
	id        string
	model     string
	bambooU   bamboosdk.Usage
	cancelled bool
	flush     []byte
}

func collectStreamHop(ctx context.Context, c *gin.Context, info *relaycommon.RelayInfo, client bamboosdk.BambooClient,
	entryCodec bamboocodec.Codec, outFmt bamboocodec.FormatType, req *bamboocodec.RelayRequest,
	writeLive bool, writeSSE func([]byte) bool) (collectedHop, *types.NewAPIError) {

	out := collectedHop{usage: &dto.Usage{UsageSemantic: "anthropic"}}
	eventCh, err := client.Chat(ctx, req.Messages, req.System, req.Config)
	if err != nil {
		if isClientCancel(c, err) {
			ensureStreamStatus(info).SetEndReason(relaycommon.StreamEndReasonClientGone, err)
			out.cancelled = true
			return out, nil
		}
		return out, translateSDKError(err)
	}

	modelName := ""
	if req.Config != nil {
		modelName = req.Config.Model
	}
	out.model = modelName
	serializer := entryCodec.NewSerializer(modelName)
	collector := newBambooTimingCollector()

	type streamBlockAccum struct {
		blockType    string
		textBuf      strings.Builder
		thinkingBuf  strings.Builder
		toolID       string
		toolName     string
		toolInputBuf strings.Builder
	}
	streamBlocks := make(map[int]*streamBlockAccum)
	var orderedIndices []int
	var usage dto.Usage
	usage.UsageSemantic = "anthropic"

	for event := range eventCh {
		if event.Type == bamboosdk.EventError {
			cause := errStreamErrorNoDetail
			if event.Error != nil {
				cause = event.Error
			}
			cancelled := isClientCancel(c, cause)
			if cancelled {
				ensureStreamStatus(info).SetEndReason(relaycommon.StreamEndReasonClientGone, cause)
				out.cancelled = true
				out.usage = &usage
				return out, nil
			}
			ensureStreamStatus(info).SetEndReason(relaycommon.StreamEndReasonScannerErr, cause)
			return out, types.NewError(cause, types.ErrorCodeBadResponseBody)
		}
		collector.observe(event)
		accumulateReasoningFromEvent(info.OriginModelName, &usage, &event)
		if event.Type == bamboosdk.EventMessageStart && event.Message != nil {
			// ID 不在 BambooMessage 上时用空；fold 会兜底。
		}
		if event.Usage != nil && event.Type == bamboosdk.EventMessageStart {
			out.bambooU = *event.Usage
		}
		if event.Type == bamboosdk.EventMessageDelta && event.Usage != nil {
			out.bambooU = *event.Usage
		}

		switch event.Type {
		case bamboosdk.EventContentBlockStart:
			if event.ContentBlock != nil {
				accum := &streamBlockAccum{blockType: string(event.ContentBlock.BlockType())}
				switch b := event.ContentBlock.(type) {
				case *bamboosdk.TextBlock:
					accum.textBuf.WriteString(b.Text)
				case *bamboosdk.ThinkingBlock:
					accum.thinkingBuf.WriteString(b.Thinking)
				case *bamboosdk.ToolUseBlock:
					accum.toolID = b.ID
					accum.toolName = b.Name
					if seed := hosttool.SeedToolInput(b.Input); len(seed) > 0 {
						accum.toolInputBuf.Write(seed)
					}
				}
				streamBlocks[event.Index] = accum
				orderedIndices = append(orderedIndices, event.Index)
			}
		case bamboosdk.EventContentBlockDelta:
			if event.Delta != nil {
				if delta, ok := event.Delta.(*bamboosdk.StreamDelta); ok && delta != nil {
					accum, exists := streamBlocks[event.Index]
					if !exists {
						accum = &streamBlockAccum{blockType: "text"}
						streamBlocks[event.Index] = accum
						orderedIndices = append(orderedIndices, event.Index)
					}
					switch delta.Type {
					case bamboosdk.DeltaTextDelta:
						accum.textBuf.WriteString(delta.Text)
					case bamboosdk.DeltaThinkingDelta:
						accum.blockType = "thinking"
						accum.thinkingBuf.WriteString(delta.Thinking)
					case bamboosdk.DeltaInputJSON:
						accum.toolInputBuf.WriteString(delta.PartialJSON)
					}
				}
			}
		}

		if event.Type == bamboosdk.EventMessageStart ||
			event.Type == bamboosdk.EventMessageDelta ||
			event.Type == bamboosdk.EventPing {
			extractStreamUsage(&usage, &event)
		}

		data, serr := serializer.Serialize(event)
		if serr != nil {
			return out, translateCodecError(serr)
		}
		if writeLive {
			info.SetFirstResponseTime()
		}
		if data == nil {
			continue
		}
		for _, frame := range bamboorelay.SplitSSEFrames(data) {
			out.frames = append(out.frames, frame)
			if writeLive {
				if info.BambooDebug != nil && len(info.BambooDebug.RelayResponse) < maxBambooDebugStreamLen {
					debugFrame := bamboorelay.FormatRelayResponseFrame("StreamRelay", outFmt, outFmt, frame)
					if info.BambooDebug.RelayResponse == "" {
						info.BambooDebug.RelayResponse = debugFrame
					} else {
						info.BambooDebug.RelayResponse += "\n" + debugFrame
					}
				}
				if !writeSSE(frame) {
					out.cancelled = true
					out.usage = &usage
					return out, nil
				}
			}
		}
	}
	tail, _ := serializer.Flush()
	out.flush = tail
	if writeLive && len(tail) > 0 {
		for _, frame := range bamboorelay.SplitSSEFrames(tail) {
			out.frames = append(out.frames, frame)
			_ = writeSSE(frame)
		}
	}

	blocks := make([]bamboosdk.ContentBlock, 0, len(orderedIndices))
	for _, idx := range orderedIndices {
		accum := streamBlocks[idx]
		if accum == nil {
			continue
		}
		switch accum.blockType {
		case "thinking":
			blocks = append(blocks, bamboosdk.NewThinkingBlock(accum.thinkingBuf.String(), ""))
		case "tool_use":
			blocks = append(blocks, bamboosdk.NewToolUseBlockWithRawInput(accum.toolID, accum.toolName, accum.toolInputBuf.String()))
		default:
			if accum.textBuf.Len() > 0 {
				blocks = append(blocks, bamboosdk.NewTextBlock(accum.textBuf.String()))
			}
		}
	}
	out.blocks = blocks
	out.usage = &usage
	recordResponseBlocks(info, blocks)
	if timingResult := collector.result(); !timingResult.IsZero() {
		result := timingResult
		info.BambooTiming = &result
	}
	if usage.PromptTokens == 0 {
		usage.PromptTokens = info.GetEstimatePromptTokens()
		out.usage = &usage
	}
	return out, nil
}

func replayStreamHop(c *gin.Context, info *relaycommon.RelayInfo, hop collectedHop, writeSSE func([]byte) bool) (*dto.Usage, *types.NewAPIError) {
	info.SetFirstResponseTime()
	var items []string
	for _, frame := range hop.frames {
		items = append(items, string(frame))
		if !writeSSE(frame) {
			break
		}
	}
	if len(hop.flush) > 0 {
		for _, frame := range bamboorelay.SplitSSEFrames(hop.flush) {
			items = append(items, string(frame))
			_ = writeSSE(frame)
		}
	}
	if len(items) > 0 {
		info.ResponseBody = truncateResponseBody(strings.Join(items, "\n"))
	}
	ensureStreamStatus(info).SetEndReason(relaycommon.StreamEndReasonDone, nil)
	return hop.usage, nil
}

func emitFoldedStream(c *gin.Context, info *relaycommon.RelayInfo, entryCodec bamboocodec.Codec, model string,
	hop collectedHop, results []hosttool.ExecResult, writeSSE func([]byte) bool) (*dto.Usage, *types.NewAPIError) {
	info.SetFirstResponseTime()
	folded := hosttool.FoldResponse(hop.id, firstNonEmptyStr(hop.model, model), hop.blocks, results, hop.bambooU)
	prependVisibleBox(folded, info)
	ser := entryCodec.NewSerializer(firstNonEmptyStr(hop.model, model))
	var items []string
	for _, ev := range hosttool.FoldToStreamEvents(folded) {
		data, err := ser.Serialize(ev)
		if err != nil {
			return hop.usage, translateCodecError(err)
		}
		if data == nil {
			continue
		}
		for _, frame := range bamboorelay.SplitSSEFrames(data) {
			items = append(items, string(frame))
			if !writeSSE(frame) {
				break
			}
		}
	}
	tail, _ := ser.Flush()
	if len(tail) > 0 {
		for _, frame := range bamboorelay.SplitSSEFrames(tail) {
			items = append(items, string(frame))
			_ = writeSSE(frame)
		}
	}
	if len(items) > 0 {
		info.ResponseBody = truncateResponseBody(strings.Join(items, "\n"))
	}
	ensureStreamStatus(info).SetEndReason(relaycommon.StreamEndReasonDone, nil)
	return hop.usage, nil
}

func recordResponseBlocks(info *relaycommon.RelayInfo, blocks []bamboosdk.ContentBlock) {
	if info == nil || info.BambooRelayData == nil || len(blocks) == 0 {
		return
	}
	responseBlocks := make([]relaycommon.BambooBlockExtract, 0, len(blocks))
	for _, block := range blocks {
		ext := relaycommon.BambooBlockExtract{Type: string(block.BlockType())}
		switch b := block.(type) {
		case *bamboosdk.TextBlock:
			ext.Text = b.Text
		case *bamboosdk.ThinkingBlock:
			ext.Thinking = b.Thinking
		case *bamboosdk.ToolUseBlock:
			ext.ToolID = b.ID
			ext.ToolName = b.Name
			ext.ToolInput = b.Input
		case *bamboosdk.ToolResultBlock:
			ext.ToolID = b.ToolUseID
			ext.ToolName = b.ToolName
			ext.ToolResult = b.Content
			ext.IsError = b.IsError
		}
		responseBlocks = append(responseBlocks, ext)
	}
	info.BambooRelayData.ResponseBlocks = responseBlocks
}

func firstNonEmptyStr(vals ...string) string {
	for _, v := range vals {
		if v != "" {
			return v
		}
	}
	return ""
}

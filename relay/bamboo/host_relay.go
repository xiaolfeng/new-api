package bamboo

import (
	"context"
	"fmt"
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
	"github.com/QuantumNous/new-api/relay/clientprofile"
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
	action := hosttool.DecideActionForClient(info.HostToolPlan, uses, info)
	if action == hosttool.ActionPassthrough {
		return writeCompleteResponse(c, info, entryCodec, codecFmt, resp, usage, nil)
	}

	st := model_setting.GetBambooSettings()
	results := hosttool.ExecuteCalls(c.Request.Context(), info, st, uses)
	if action == hosttool.ActionFoldB {
		folded := hosttool.FoldResponse(resp.ID, resp.Model, blocks, results, resp.Usage)
		return writeCompleteResponse(c, info, entryCodec, codecFmt, folded, usage, nil)
	}

	hop2Req := hosttool.BuildHop2Request(req, blocks, results, uses)
	if ok, reason := hosttool.AllowHop2(c, info, hop2Req); !ok {
		if info.HostToolPlan != nil {
			info.HostToolPlan.StoppedReason = reason
		}
		folded := hosttool.FoldResponse(resp.ID, resp.Model, blocks, results, resp.Usage)
		return writeCompleteResponse(c, info, entryCodec, codecFmt, folded, usage, nil)
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
		return writeCompleteResponse(c, info, entryCodec, codecFmt, folded, usage, nil)
	}
	prependHostToolCalls(resp2, info, results)
	return writeCompleteResponse(c, info, entryCodec, codecFmt, resp2, usage, results)
}

func prependHostToolCalls(resp *bamboosdk.Response, info *relaycommon.RelayInfo, results []hosttool.ExecResult) {
	if resp == nil || len(results) == 0 {
		return
	}
	grok := info != nil && info.ClientProfile == common.ClientProfileGrokBuild
	calls := make([]bamboosdk.ContentBlock, 0, len(results))
	for i, r := range results {
		if grok && r.Kind == "search" {
			continue
		}
		id := r.CallID
		if id == "" {
			id = fmt.Sprintf("call_host_%d", i+1)
		}
		name := r.OriginalName
		if name == "" {
			name = r.Kind
		}
		calls = append(calls, bamboosdk.NewToolUseBlockWithRawInput(id, name, hosttool.CallInputJSON(r)))
	}
	if len(calls) == 0 {
		return
	}
	resp.Content = append(calls, resp.Content...)
}

func writeCompleteResponse(c *gin.Context, info *relaycommon.RelayInfo, entryCodec bamboocodec.Codec,
	codecFmt bamboocodec.FormatType, resp *bamboosdk.Response, usage *dto.Usage, results []hosttool.ExecResult) (*dto.Usage, *types.NewAPIError) {
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
	if entryCodec.Format() == bamboocodec.FormatOpenAI {
		body = hosttool.InjectOpenAIToolOutputs(body, results)
	}
	if info.BambooDebug != nil {
		info.BambooDebug.RelayResponse = bamboorelay.FormatRelayResponse("CompleteRelay", codecFmt, codecFmt, body)
	}
	body = clientprofile.NormalizeResponsesPayload(info, body)
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
	entryCodec bamboocodec.Codec, outFmt bamboocodec.FormatType, req *bamboocodec.RelayRequest, cs *clientStream) (*dto.Usage, *types.NewAPIError) {

	ctx, cancel := context.WithCancel(c.Request.Context())
	defer cancel()
	go func() {
		<-c.Request.Context().Done()
		cancel()
	}()

	modelName := ""
	if req.Config != nil {
		modelName = req.Config.Model
	}
	if cs == nil {
		cs = newClientStream(c, entryCodec, info, modelName)
	}
	if cs.startPing == nil {
		cs.startPing = func() func() {
			return startHostPing(ctx, cs.writeSSE, entryCodec.NewSerializer(modelName))
		}
	}
	cs.ensureHeaders()
	cs.resumePing()

	// 第一跳 tee：工具调用出现前的 thinking/text 实时透传，使首字延迟等于上游真实首字；
	// 首个 tool_use 是 commit point，其后的 tool_use 帧不示人，交折叠/hop2 决策统一收尾。
	hop1, hopErr := collectStreamHop(ctx, c, info, client, req, true, cs)
	if hopErr != nil {
		cs.pausePing()
		if cs.HasHeaders() {
			cs.emitError(hopErr)
		}
		return hop1.usage, hopErr
	}
	if hop1.cancelled {
		cs.pausePing()
		cs.finish()
		return hop1.usage, nil
	}

	uses := hosttool.CollectToolUses(hop1.blocks)
	action := hosttool.DecideActionForClient(info.HostToolPlan, uses, info)
	if action == hosttool.ActionPassthrough {
		// tee 已实时写完全部内容，只需补终止帧。
		cs.pausePing()
		cs.finish()
		finishHostStream(c, info, cs)
		return hop1.usage, nil
	}

	st := model_setting.GetBambooSettings()
	results := hosttool.ExecuteCalls(ctx, info, st, uses)
	if action == hosttool.ActionFoldB {
		// 折叠保留 thinking/text，tee 已实时透传同质内容，只追加工具结果 dump。
		cs.pausePing()
		emitPendingVisibleBox(cs, info)
		appendToolDump(cs, results)
		cs.finish()
		finishHostStream(c, info, cs)
		return hop1.usage, nil
	}

	hop2Req := hosttool.BuildHop2Request(req, hop1.blocks, results, uses)
	if ok, reason := hosttool.AllowHop2(c, info, hop2Req); !ok {
		if info.HostToolPlan != nil {
			info.HostToolPlan.StoppedReason = reason
		}
		cs.pausePing()
		emitPendingVisibleBox(cs, info)
		appendToolDump(cs, results)
		cs.finish()
		finishHostStream(c, info, cs)
		return hop1.usage, nil
	}

	cs.pausePing()
	if !hop1.liveEmitted {
		// tee 未透传任何思考内容（模型直接发起工具调用），用 visible box 兜底反馈。
		emitPendingVisibleBox(cs, info)
	}
	emitHostToolCalls(cs, info, results)
	hop2, hop2Err := collectStreamHop(ctx, c, info, client, hop2Req, false, cs)
	if hop2Err != nil {
		if cs.HasHeaders() {
			cs.emitError(hop2Err)
		}
		return hosttool.AddUsage(hop1.usage, hop2.usage), hop2Err
	}
	usage := hosttool.AddUsage(hop1.usage, hop2.usage)
	uses2 := hosttool.CollectToolUses(hop2.blocks)
	if hosttool.AllHostToolUses(info.HostToolPlan, uses2) && len(uses2) > 0 {
		// hop2 只调工具未成文：实时内容已透传，补工具结果 dump 收尾。
		appendToolDump(cs, hosttool.ExecuteCalls(ctx, info, st, uses2))
	}
	cs.finish()
	finishHostStream(c, info, cs)
	return usage, nil
}

type collectedHop struct {
	blocks    []bamboosdk.ContentBlock
	usage     *dto.Usage
	cancelled bool
	// liveEmitted 表示 tee 模式已实时透传过 thinking/text 内容，
	// 供 hop2 路径决定是否还需要 visible box 兜底反馈。
	liveEmitted bool
}

// teeRelay 按 commit-point 规则实时透传 hop1 的 thinking/text 内容。
// 首个 tool_use（start 或 partial_json delta）出现后进入 committed 状态，
// 只允许补发已打开内容块的 stop，避免客户端看到会被折叠/续写掉的工具调用帧。
type teeRelay struct {
	committed      bool
	openBlocks     map[int]bool
	emittedContent bool
}

func (t *teeRelay) forward(cs *clientStream, ev bamboosdk.StreamEvent) bool {
	if t == nil || cs == nil {
		return true
	}
	if t.committed {
		if ev.Type == bamboosdk.EventContentBlockStop && t.openBlocks[ev.Index] {
			delete(t.openBlocks, ev.Index)
			return cs.forward(ev)
		}
		return true
	}
	switch ev.Type {
	case bamboosdk.EventContentBlockStart:
		if _, isTool := ev.ContentBlock.(*bamboosdk.ToolUseBlock); isTool {
			t.committed = true
			return true
		}
		t.emittedContent = true
		t.openBlocks[ev.Index] = true
		return cs.forward(ev)
	case bamboosdk.EventContentBlockDelta:
		if d, ok := ev.Delta.(*bamboosdk.StreamDelta); ok && d.Type == bamboosdk.DeltaInputJSON {
			t.committed = true
			return true
		}
		t.emittedContent = true
		return cs.forward(ev)
	case bamboosdk.EventContentBlockStop:
		if t.openBlocks[ev.Index] {
			delete(t.openBlocks, ev.Index)
			return cs.forward(ev)
		}
		return true
	case bamboosdk.EventMessageStart:
		return cs.forward(ev)
	default:
		// message_delta / message_stop / ping：终止语义由收尾路径统一补发，不实时透传。
		return true
	}
}

func collectStreamHop(ctx context.Context, c *gin.Context, info *relaycommon.RelayInfo, client bamboosdk.BambooClient,
	req *bamboocodec.RelayRequest, tee bool, live *clientStream) (collectedHop, *types.NewAPIError) {

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

	var relay *teeRelay
	if tee {
		relay = &teeRelay{openBlocks: make(map[int]bool)}
	}

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

		if live != nil {
			ok := true
			if relay != nil {
				// tee：tool_use 前的 thinking/text 实时透传，commit 后交由决策收尾。
				ok = relay.forward(live, event)
			} else {
				// live：hop2 全量透传。
				ok = live.forward(event)
			}
			if !ok {
				out.cancelled = true
				out.usage = &usage
				return out, nil
			}
		}
	}
	if relay != nil {
		out.liveEmitted = relay.emittedContent
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

// finishHostStream 统一记录流式收尾的响应体与结束原因。
func finishHostStream(c *gin.Context, info *relaycommon.RelayInfo, cs *clientStream) {
	if frames := cs.Frames(); len(frames) > 0 {
		info.ResponseBody = truncateResponseBody(strings.Join(frames, "\n"))
	}
	if c.Request.Context().Err() != nil {
		ensureStreamStatus(info).SetEndReason(relaycommon.StreamEndReasonClientGone, c.Request.Context().Err())
	} else {
		ensureStreamStatus(info).SetEndReason(relaycommon.StreamEndReasonDone, nil)
	}
}

// appendToolDump 在 tee 已实时透传的流末尾追加工具结果摘要文本块。
func appendToolDump(cs *clientStream, results []hosttool.ExecResult) {
	if cs == nil {
		return
	}
	dump := hosttool.JoinFormattedResults(results)
	if dump != "" {
		cs.emitClosedText(dump)
	}
}

func emitPendingVisibleBox(cs *clientStream, info *relaycommon.RelayInfo) {
	box := visibleBoxText(info)
	if cs == nil || box == "" {
		return
	}
	cs.emitClosedThinking(box)
	if info.ImageRecognizePlan != nil {
		info.ImageRecognizePlan.VisibleBox = ""
	}
}

func emitHostToolCalls(cs *clientStream, info *relaycommon.RelayInfo, results []hosttool.ExecResult) {
	if cs == nil {
		return
	}
	grok := info != nil && info.ClientProfile == common.ClientProfileGrokBuild
	for i, r := range results {
		if grok && r.Kind == "search" {
			// 主路径已对 grok search 透传；若仍走到 hop2，禁止再抛
			// function_call name=web_search，否则客户端 helper 会再搜一轮。
			continue
		}
		id := r.CallID
		if id == "" {
			id = fmt.Sprintf("call_host_%d", i+1)
		}
		name := r.OriginalName
		if name == "" {
			name = r.Kind
		}
		cs.emitToolUse(id, name, hosttool.CallInputJSON(r))
		cs.emitOpenAIToolOutput(i, id, hosttool.MCPResultJSON(r))
	}
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

func doHostBuiltinResponses(c *gin.Context, info *relaycommon.RelayInfo, req *bamboocodec.RelayRequest) (*dto.Usage, *types.NewAPIError) {
	st := model_setting.GetBambooSettings()
	result := hosttool.ExecuteBuiltinRequest(c.Request.Context(), info, st, req)

	modelName := info.OriginModelName
	if req != nil && req.Config != nil && req.Config.Model != "" {
		modelName = req.Config.Model
	}
	createdAt := time.Now().Unix()
	usage := &dto.Usage{UsageSemantic: "anthropic"}
	common.SetContextKey(c, constant.ContextKeyEmptyResponse, false)
	info.SetFirstResponseTime()

	if req != nil && req.IsStream {
		writeStreamHeaders(c)
		frames, err := hosttool.BuiltinStreamFrames(modelName, info.RequestId, createdAt, result)
		if err != nil {
			return usage, types.NewError(err, types.ErrorCodeBadResponseBody)
		}
		var items []string
		for _, frame := range frames {
			frame = clientprofile.NormalizeResponsesSSEFrame(info, frame)
			if len(frame) == 0 {
				continue
			}
			items = append(items, string(frame))
			if _, werr := c.Writer.Write(frame); werr != nil {
				break
			}
			c.Writer.Flush()
		}
		if len(items) > 0 {
			info.ResponseBody = truncateResponseBody(strings.Join(items, "\n"))
		}
		ensureStreamStatus(info).SetEndReason(relaycommon.StreamEndReasonDone, nil)
		return usage, nil
	}

	body, err := hosttool.MarshalBuiltinComplete(modelName, info.RequestId, createdAt, result)
	if err != nil {
		return usage, types.NewError(err, types.ErrorCodeBadResponseBody)
	}
	body = clientprofile.NormalizeResponsesPayload(info, body)
	c.Writer.Header().Set("Content-Type", "application/json")
	_, _ = c.Writer.Write(body)
	info.ResponseBody = truncateResponseBody(string(body))
	return usage, nil
}

func doHostClaudeSearch(c *gin.Context, info *relaycommon.RelayInfo, req *bamboocodec.RelayRequest) (*dto.Usage, *types.NewAPIError) {
	st := model_setting.GetBambooSettings()
	result := hosttool.ExecuteBuiltinRequest(c.Request.Context(), info, st, req)

	modelName := info.OriginModelName
	if req != nil && req.Config != nil && req.Config.Model != "" {
		modelName = req.Config.Model
	}
	usage := &dto.Usage{UsageSemantic: "anthropic"}
	common.SetContextKey(c, constant.ContextKeyEmptyResponse, false)
	info.SetFirstResponseTime()

	if req != nil && req.IsStream {
		writeStreamHeaders(c)
		var frames [][]byte
		var err error
		if result.Kind == "fetch" {
			frames, err = hosttool.ClaudeServerFetchStreamFrames(modelName, info.RequestId, result)
		} else {
			frames, err = hosttool.ClaudeServerSearchStreamFrames(modelName, info.RequestId, result)
		}
		if err != nil {
			return usage, types.NewError(err, types.ErrorCodeBadResponseBody)
		}
		var items []string
		for _, frame := range frames {
			frame = clientprofile.NormalizeClaudeSSEFrame(info, frame)
			if len(frame) == 0 {
				continue
			}
			items = append(items, string(frame))
			if _, werr := c.Writer.Write(frame); werr != nil {
				break
			}
			c.Writer.Flush()
		}
		if len(items) > 0 {
			info.ResponseBody = truncateResponseBody(strings.Join(items, "\n"))
		}
		ensureStreamStatus(info).SetEndReason(relaycommon.StreamEndReasonDone, nil)
		return usage, nil
	}

	var body []byte
	var err error
	if result.Kind == "fetch" {
		body, err = hosttool.MarshalClaudeServerFetch(modelName, info.RequestId, result)
	} else {
		body, err = hosttool.MarshalClaudeServerSearch(modelName, info.RequestId, result)
	}
	if err != nil {
		return usage, types.NewError(err, types.ErrorCodeBadResponseBody)
	}
	c.Writer.Header().Set("Content-Type", "application/json")
	_, _ = c.Writer.Write(body)
	info.ResponseBody = truncateResponseBody(string(body))
	return usage, nil
}

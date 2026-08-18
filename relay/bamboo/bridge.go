package bamboo

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"unicode/utf8"

	"github.com/gin-gonic/gin"

	bamboosdk "github.com/bamboo-services/bamboo-messages/bamboo"
	bamboocodec "github.com/bamboo-services/bamboo-messages/bamboo/codec"
	bamboorelay "github.com/bamboo-services/bamboo-messages/bamboo/relay"
	"github.com/bamboo-services/bamboo-messages/provider"
	// 空白 import 触发各 codec 子包的 init() 注册。
	// codec.Get 依赖包级变量（registry.go:9-21）由子包 init() 赋值，
	// 不显式 import 子包会导致 codec.Get 返回 nil。
	_ "github.com/bamboo-services/bamboo-messages/bamboo/codec/anthropic"
	_ "github.com/bamboo-services/bamboo-messages/bamboo/codec/gemini"
	_ "github.com/bamboo-services/bamboo-messages/bamboo/codec/openai"
	_ "github.com/bamboo-services/bamboo-messages/bamboo/codec/responses"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/relay/bamboo/hosttool"
	"github.com/QuantumNous/new-api/relay/bamboo/imagerec"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/relaykit/dto"
	"github.com/QuantumNous/new-api/relaykit/types"
	"github.com/QuantumNous/new-api/service"
	"github.com/QuantumNous/new-api/setting/model_setting"
)

// errStreamErrorNoDetail 用于 event.Error 为 nil 但事件类型为 EventError 的兜底。
var errStreamErrorNoDetail = errors.New("bamboo stream error event without detail")

// maxResponseBodyLen 限制 info.ResponseBody 的最大长度，防止超大日志撑爆数据库。
const maxResponseBodyLen = 50000

// maxBambooDebugStreamLen 限制流式 debug 帧收集的总字符数上限。
// 达到上限后停止追加新帧，避免超长流式响应撑爆 debug 字段。
const maxBambooDebugStreamLen = 50000

// truncateResponseBody 按字节截断超长响应体，回退到最后一个合法 UTF-8 边界并追加截断标记。
func truncateResponseBody(body string) string {
	if len(body) <= maxResponseBodyLen {
		return body
	}
	cut := maxResponseBodyLen
	// 回退到最后一个合法的 UTF-8 起始字节，避免截断多字节字符中间
	for cut > 0 && !utf8.RuneStart(body[cut]) {
		cut--
	}
	return body[:cut] + "\n...[truncated]"
}

// ChatRelay 对话中继统一内核。
//
// 替代 TextHelper/ClaudeHelper/GeminiHelper/ResponsesHelper 内部的
// Convert→DoRequest→DoResponse 三段式，用 bamboo 中间表示做协议归一化：
//
//   - 入口侧：codec.ParseRequest 把入口协议请求体解析为协议无关的 RelayRequest
//     （替代 adaptor.Convert*Request 的入口→上游格式转换）
//   - 上游侧：provider.Chat/Complete 发起上游请求（替代 adaptor.DoRequest）
//   - 出口侧：codec.NewSerializer/SerializeResponse 把上游事件转回入口协议格式
//     （替代 adaptor.DoResponse 的上游→入口格式转换 + reply 生成）
//
// 调用方只需传 new-api 侧的 types.RelayFormat，格式映射在 bridge 内部完成。
// info.ApiType（经 ChannelMeta 嵌入）决定上游用哪个 bamboo provider。
//
// 返回 (usage, nil) 成功；(nil, err) 失败。
// 当 errors.Is(err, ErrUnsupportedProvider) 时，调用方应 fallback 原生链路。
func ChatRelay(c *gin.Context, info *relaycommon.RelayInfo,
	entryFormat types.RelayFormat, requestBody []byte) (*dto.Usage, *types.NewAPIError) {

	// ① 入口格式映射：RelayFormat → codec FormatType
	codecFmt, ok := relayFormatToCodec(entryFormat)
	if !ok {
		// 非对话格式（Audio/Image/Task/Realtime/Rerank/Embedding）不应进入 bridge
		return nil, types.NewError(ErrUnsupportedProvider, types.ErrorCodeInvalidApiType)
	}

	entryCodec, gerr := bamboocodec.Get(codecFmt)
	if gerr != nil || entryCodec == nil {
		return nil, types.NewError(fmt.Errorf("bamboo codec not registered: %s", codecFmt), types.ErrorCodeInvalidRequest)
	}

	// debug 收集：当 EnableBambooDebugLog 开启时，用 FormatRelayInput/FormatRelayParsed
	// 等纯函数收集格式化 debug 字符串，写入 info.BambooDebug 分块结构供日志详情展示。
	// SDK v0.8.9 已移除 provider.SetDebug()，Format 系列函数为纯函数（调用即返回），
	// 由 newapi 自身的 EnableBambooDebugLog 开关控制是否收集。
	debugEnabled := model_setting.GetBambooSettings().EnableBambooDebugLog
	if debugEnabled {
		info.BambooDebug = &relaycommon.BambooDebugInfo{}
	}

	relayReq, parseErr := entryCodec.ParseRequest(requestBody)
	if parseErr != nil {
		return nil, translateCodecError(parseErr) // 内部 errors.As 断言 *CodecError
	}

	info.BambooRelayData = extractBambooRelayData(relayReq)

	if recErr := imagerec.MaybeRewrite(c, info, relayReq); recErr != nil {
		return nil, recErr
	}

	bambooSettings := model_setting.GetBambooSettings()
	if bambooSettings.EnableHostTools {
		plan, herr := hosttool.InspectAndRewrite(entryFormat, requestBody, relayReq, bambooSettings)
		if herr != nil {
			return nil, types.NewError(herr, types.ErrorCodeInvalidRequest)
		}
		info.HostToolPlan = plan
	}

	if debugEnabled {
		info.BambooDebug.RelayParsed = bamboorelay.FormatRelayParsed("ChatRelay", codecFmt, relayReq)
	}

	if info.HostToolPlan != nil && info.HostToolPlan.Enabled &&
		hosttool.IsResponsesBuiltinOnly(entryFormat, requestBody, info.HostToolPlan, relayReq) {
		info.HostToolPlan.BuiltinResponses = true
		return doHostBuiltinResponses(c, info, relayReq)
	}

	// ② 上游侧：根据 ApiType 构造 bamboo provider
	p, upstreamRelayFormat, provErr := newProvider(c, info)
	if provErr != nil {
		return nil, provErr // 含 ErrUnsupportedProvider，调用方判 errors.Is 做 fallback
	}

	// 确定有效的上游格式：provider 解析结果优先，为空时回退到入口格式
	effectiveUpstreamFormat := upstreamRelayFormat
	if effectiveUpstreamFormat == "" {
		effectiveUpstreamFormat = entryFormat
	}

	// 更新 RelayInfo 格式链路，供 billing/log/relay-output 下游消费
	info.AppendRequestConversion(effectiveUpstreamFormat)
	info.FinalRequestRelayFormat = effectiveUpstreamFormat

	if debugEnabled {
		// 使用真实上游格式作为 out 参数（而非入口格式）
		outCodecFmt, _ := relayFormatToCodec(effectiveUpstreamFormat)
		info.BambooDebug.RelayInput = bamboorelay.FormatRelayInput("ChatRelay", codecFmt, outCodecFmt, requestBody)
		info.BambooDebug.ProviderRequest = provider.FormatDebugRequest(
			"bamboo-bridge",
			fmt.Sprintf("upstream provider=%T model=%s relayFormat=%s", p, relayReq.Config.Model, effectiveUpstreamFormat),
			nil, relayReq.Config,
		)
	}

	client := bamboosdk.NewClient(p)

	// ③ 出口侧：按入口 codec 序列化响应
	if info.HostToolPlan != nil && info.HostToolPlan.Enabled {
		if relayReq.IsStream {
			return doHostStreamRelay(c, info, client, entryCodec, codecFmt, relayReq)
		}
		return doHostCompleteRelay(c, info, client, entryCodec, codecFmt, relayReq)
	}
	if relayReq.IsStream {
		return doStreamRelay(c, info, client, entryCodec, codecFmt, relayReq)
	}
	return doCompleteRelay(c, info, client, entryCodec, codecFmt, relayReq)
}

// isClientCancel 判断错误或请求上下文是否表明客户端已断开（取消）。
//
// 优先通过 errors.Is 识别 SDK 的 BambooError.Unwrap 链（v0.9.6 起
// "对话已取消"错误携带 context.Canceled 作为 cause）；兜底检查请求
// 上下文是否已结束，避免 SDK 未显式标记取消时漏判。
func isClientCancel(c *gin.Context, err error) bool {
	if err != nil && errors.Is(err, context.Canceled) {
		return true
	}
	return c != nil && c.Request != nil && c.Request.Context().Err() != nil
}

// ensureStreamStatus 惰性初始化 relayInfo.StreamStatus 并返回非 nil 实例。
//
// bamboo 链路平时不初始化 StreamStatus（官方 stream_scanner 无条件新建），
// 这里在需要标记流结束状态（取消/正常结束/流层错误）时按需初始化，
// 使 service/log_info_generate.go 的 appendStreamStatus 能写入日志详情。
func ensureStreamStatus(info *relaycommon.RelayInfo) *relaycommon.StreamStatus {
	if info.StreamStatus == nil {
		info.StreamStatus = relaycommon.NewStreamStatus()
	}
	return info.StreamStatus
}

// doStreamRelay 消费 bamboo StreamEvent，按入口 codec 序列化为出口 SSE。
//
// SDK v0.8.15 起移除 SmoothPacer 子系统，流式输出改为纯透传：
// 序列化后的 SSE 帧直接写入 HTTP Response，不再做平滑缓冲。
func doStreamRelay(c *gin.Context, info *relaycommon.RelayInfo, client bamboosdk.BambooClient,
	entryCodec bamboocodec.Codec, outFmt bamboocodec.FormatType, req *bamboocodec.RelayRequest) (*dto.Usage, *types.NewAPIError) {

	ctx, cancel := context.WithCancel(c.Request.Context())
	defer cancel()

	go func() {
		<-c.Request.Context().Done()
		cancel()
	}()

	eventCh, err := client.Chat(ctx, req.Messages, req.System, req.Config)
	if err != nil {
		if isClientCancel(c, err) {
			// 客户端在首个事件前断开：标记流状态为 client_gone 并正常结束，
			// 与官方 stream_scanner 在 c.Request.Context().Done() 时的行为对齐，
			// 不产生 status_code=500 错误日志。
			ensureStreamStatus(info).SetEndReason(relaycommon.StreamEndReasonClientGone, err)
			return nil, nil
		}
		return nil, translateSDKError(err)
	}

	c.Writer.Header().Set("Content-Type", "text/event-stream")
	c.Writer.Header().Set("Cache-Control", "no-cache")
	c.Writer.Header().Set("X-Accel-Buffering", "no")
	c.Writer.Flush()

	serializer := entryCodec.NewSerializer(req.Config.Model)
	var usage dto.Usage

	collector := newBambooTimingCollector()

	var streamItems []string

	writeSSE := func(data []byte) bool {
		if _, werr := c.Writer.Write(data); werr != nil {
			return false
		}
		c.Writer.Flush()
		return true
	}

	// 流式内容块累加器：从 StreamEvent 直接累积 thinking / text / tool_use，
	// 避免后续从格式化 SSE 字符串反向解析（可能丢失 thinking 块或被截断）。
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
	boxInjected := visibleBoxText(info) == ""

	for event := range eventCh {
		if event.Type == bamboosdk.EventError {
			cause := errStreamErrorNoDetail
			if event.Error != nil {
				cause = event.Error
			}
			// 客户端取消（SDK 在 ctx 取消时注入"对话已取消"错误事件）：
			// 按已收到内容结算并标记 client_gone，不当作错误日志；
			// 真实流层错误则标记 ScannerErr 后走错误路径。
			cancelled := isClientCancel(c, cause)
			if cancelled {
				ensureStreamStatus(info).SetEndReason(relaycommon.StreamEndReasonClientGone, cause)
			} else {
				ensureStreamStatus(info).SetEndReason(relaycommon.StreamEndReasonScannerErr, cause)
			}
			tail, _ := serializer.Flush()
			if len(tail) > 0 {
				for _, frame := range bamboorelay.SplitSSEFrames(tail) {
					streamItems = append(streamItems, string(frame))
					writeSSE(frame)
				}
			}
			if len(streamItems) > 0 {
				info.ResponseBody = truncateResponseBody(strings.Join(streamItems, "\n"))
			}
			usage.TotalTokens = usage.PromptTokens + usage.CompletionTokens
			if cancelled {
				return &usage, nil
			}
			return &usage, types.NewError(cause, types.ErrorCodeBadResponseBody)
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
						if accum.blockType != "thinking" {
							accum.blockType = "thinking"
						}
						accum.thinkingBuf.WriteString(delta.Thinking)
					case bamboosdk.DeltaInputJSON:
						accum.toolInputBuf.WriteString(delta.PartialJSON)
					}
				}
			}
		}

		data, serr := serializer.Serialize(event)
		if serr != nil {
			return nil, translateCodecError(serr)
		}

		// TTFT：首个有效事件序列化成功后记录首次响应时间（幂等，仅首次生效）。
		info.SetFirstResponseTime()

		// 先提取 usage，再写 SSE — 即使客户端断开（writeSSE 返回 false）也能拿到 usage。
		if event.Type == bamboosdk.EventMessageStart ||
			event.Type == bamboosdk.EventMessageDelta ||
			event.Type == bamboosdk.EventPing {
			extractStreamUsage(&usage, &event)
		}

		// 过滤 nil 数据（EventContentBlockStop / EventMessageStop 等返回 nil）
		// 并拆分合并帧（handleMessageDelta 的 marshalChunks 将 finish_reason + usage
		// 合并为单个 []byte，需要 SplitSSEFrames 按 \n\n 边界拆分为独立 SSE 事件）
		if data == nil {
			continue
		}
		frames := bamboorelay.SplitSSEFrames(data)
		for _, frame := range frames {
			streamItems = append(streamItems, string(frame))

			if info.BambooDebug != nil && len(info.BambooDebug.RelayResponse) < maxBambooDebugStreamLen {
				debugFrame := bamboorelay.FormatRelayResponseFrame("StreamRelay", outFmt, outFmt, frame)
				if info.BambooDebug.RelayResponse == "" {
					info.BambooDebug.RelayResponse = debugFrame
				} else {
					info.BambooDebug.RelayResponse += "\n" + debugFrame
				}
			}

			if !writeSSE(frame) {
				break
			}
		}
		if !boxInjected && event.Type == bamboosdk.EventMessageStart {
			if writeVisibleBoxFrames(serializer, visibleBoxText(info), func(frame []byte) bool {
				streamItems = append(streamItems, string(frame))
				return writeSSE(frame)
			}) {
				boxInjected = true
			}
		}
	}

	// flush 剩余缓冲（如 OpenAI codec 的 [DONE] 终止符）
	tail, _ := serializer.Flush()
	if len(tail) > 0 {
		for _, frame := range bamboorelay.SplitSSEFrames(tail) {
			streamItems = append(streamItems, string(frame))
			if !writeSSE(frame) {
				break
			}
		}
	}

	if len(streamItems) > 0 {
		info.ResponseBody = truncateResponseBody(strings.Join(streamItems, "\n"))
	}

	// 将累加的流式内容块转换为格式无关的中间表示，供日志记录消费。
	// 优先于 ResponseBody 反向解析——保留了完整的 thinking / tool_use 结构。
	if info.BambooRelayData != nil && len(orderedIndices) > 0 {
		responseBlocks := make([]relaycommon.BambooBlockExtract, 0, len(orderedIndices))
		for _, idx := range orderedIndices {
			accum := streamBlocks[idx]
			if accum == nil {
				continue
			}
			ext := relaycommon.BambooBlockExtract{Type: accum.blockType}
			switch accum.blockType {
			case "text":
				ext.Text = accum.textBuf.String()
			case "thinking":
				ext.Thinking = accum.thinkingBuf.String()
			case "tool_use":
				ext.ToolID = accum.toolID
				ext.ToolName = accum.toolName
				ext.ToolInput = json.RawMessage(accum.toolInputBuf.String())
			}
			responseBlocks = append(responseBlocks, ext)
		}
		info.BambooRelayData.ResponseBlocks = responseBlocks
	}

	if timingResult := collector.result(); !timingResult.IsZero() {
		result := timingResult
		info.BambooTiming = &result
	}

	// 空响应检测：流式输出无任何 content_block_delta 且 output_tokens 为 0 时标记，
	// 供 controller 触发空响应重试（与非流式 doCompleteRelay 对齐）。
	// GLM 等端点在限流（429）时可能返回 200 + 空流而非标准错误码，
	// 不检测则客户端收到"正常结束但无内容"的空响应。
	hasContent := false
	for _, idx := range orderedIndices {
		if accum := streamBlocks[idx]; accum != nil {
			if accum.textBuf.Len() > 0 || accum.thinkingBuf.Len() > 0 || accum.toolInputBuf.Len() > 0 {
				hasContent = true
				break
			}
		}
	}

	// Fallback：当上游未返回 usage 或 SDK StreamConverter 覆盖导致 usage 丢失时，
	// 用估算值兜底，与原生 claude 路径（relay-claude.go:963-979）对齐。
	if usage.PromptTokens == 0 {
		usage.PromptTokens = info.GetEstimatePromptTokens()
	}
	if usage.CompletionTokens == 0 && hasContent {
		var allText strings.Builder
		for _, idx := range orderedIndices {
			if accum := streamBlocks[idx]; accum != nil {
				allText.WriteString(accum.textBuf.String())
				allText.WriteString(accum.thinkingBuf.String())
				allText.WriteString(accum.toolInputBuf.String())
			}
		}
		if allText.Len() > 0 {
			usage.CompletionTokens = service.CountTextToken(allText.String(), info.OriginModelName)
		}
	}

	usage.TotalTokens = usage.PromptTokens + usage.CompletionTokens
	usage.UsageSemantic = "anthropic"

	if !hasContent && usage.CompletionTokens == 0 {
		common.SetContextKey(c, constant.ContextKeyEmptyResponse, true)
	}

	// 标记流结束状态：writeSSE 失败（客户端断开）标记 client_gone，
	// 否则自然结束标记 done，供日志详情展示 stream_status。
	if c.Request.Context().Err() != nil {
		ensureStreamStatus(info).SetEndReason(relaycommon.StreamEndReasonClientGone, c.Request.Context().Err())
	} else {
		ensureStreamStatus(info).SetEndReason(relaycommon.StreamEndReasonDone, nil)
	}

	return &usage, nil
}

// accumulateReasoningFromEvent 从 StreamEvent 的 thinking delta 提取 reasoning token。
//
// bamboo 的 StreamEvent.Delta 是 any，content_block_delta 事件携带 *StreamDelta，
// 其中 Type==DeltaThinkingDelta 的增量代表思考过程。用 new-api 的 CountTextToken
// 精确计数（按 modelName 选 tiktoken/分词器），累计到 CompletionTokenDetails。
//
// 注意：thinking 文本分多次 delta 增量到达，这里逐段计数后求和，
// 与 tiktoken 对完整文本计数可能有微小差异（分段边界），可接受。
func accumulateReasoningFromEvent(modelName string, usage *dto.Usage, event *bamboosdk.StreamEvent) {
	if event.Type != bamboosdk.EventContentBlockDelta || event.Delta == nil {
		return
	}
	delta, ok := event.Delta.(*bamboosdk.StreamDelta)
	if !ok || delta == nil {
		return
	}
	if delta.Type == bamboosdk.DeltaThinkingDelta && delta.Thinking != "" {
		tokens := service.CountTextToken(delta.Thinking, modelName)
		accumulateReasoning(usage, tokens)
	}
}

// extractStreamUsage updates usage from StreamEvent's Usage field.
//
// Anthropic streaming protocol:
//   - message_start carries input_tokens, cache_read_input_tokens, cache_creation_input_tokens
//   - message_delta carries output_tokens (and sometimes updated input_tokens)
//
// bamboo SDK's StreamConverter sends intermediate usage via EventPing (not EventMessageDelta)
// to avoid carrying usage in a terminal-semantic chunk that could mislead clients like Vercel AI SDK.
// Therefore EventPing must also be processed here.
//
// bamboo SDK 的 Usage.InputTokens 是总输入（含 cache_read + cache_creation），
// 而 new-api 的 Claude 语义计费要求 PromptTokens 仅含非缓存部分。
// 若直接赋值，text_quota.go 的 input_tokens_total（= PromptTokens + CacheTokens +
// CacheCreationTokens）会双重计算缓存 token，导致缓存率被稀释（最高 ≈ 50%）。
// 此处减去缓存部分，与 bamboo codec/anthropic 的序列化语义（commit 3e1aea4）保持一致。
//
// Only overwrites non-zero values to avoid clobbering start data with delta zeros.
func extractStreamUsage(usage *dto.Usage, event *bamboosdk.StreamEvent) {
	if event.Usage == nil {
		return
	}
	if event.Usage.InputTokens > 0 {
		nonCached := event.Usage.InputTokens -
			event.Usage.CacheReadInputTokens -
			event.Usage.CacheCreationInputTokens
		if nonCached < 0 {
			nonCached = 0
		}
		usage.PromptTokens = int(nonCached)
	}
	if event.Usage.OutputTokens > 0 {
		usage.CompletionTokens = int(event.Usage.OutputTokens)
	}
	if event.Usage.CacheReadInputTokens > 0 {
		usage.PromptTokensDetails.CachedTokens = int(event.Usage.CacheReadInputTokens)
	}
	if event.Usage.CacheCreationInputTokens > 0 {
		usage.PromptTokensDetails.CachedCreationTokens = int(event.Usage.CacheCreationInputTokens)
	}
}

// doCompleteRelay 非流式中继。
func doCompleteRelay(c *gin.Context, info *relaycommon.RelayInfo, client bamboosdk.BambooClient,
	entryCodec bamboocodec.Codec, codecFmt bamboocodec.FormatType, req *bamboocodec.RelayRequest) (*dto.Usage, *types.NewAPIError) {

	resp, err := client.Complete(c.Request.Context(), req.Messages, req.System, req.Config)
	if err != nil {
		if isClientCancel(c, err) {
			// 客户端断开：正常结束（不记错误日志）。非流式无 StreamStatus 标记，
			// 与 appendStreamStatus 仅对流式生效的官方行为一致。
			return nil, nil
		}
		return nil, translateSDKError(err)
	}

	info.SetFirstResponseTime()
	prependVisibleBox(resp, info)

	// 空响应检测：content 为空且 output_tokens 为 0 时标记，供 controller 触发重试。
	// 参考 service/text_quota.go 的 CompletionTokens==0 && PromptTokens>0 判断，
	// 这里在 bamboo 非流式结果上通过 Response.Content 长度 + Usage.OutputTokens 检测。
	if len(resp.Content) == 0 && resp.Usage.OutputTokens == 0 {
		common.SetContextKey(c, constant.ContextKeyEmptyResponse, true)
	}

	if info.BambooRelayData != nil && len(resp.Content) > 0 {
		responseBlocks := make([]relaycommon.BambooBlockExtract, 0, len(resp.Content))
		for _, block := range resp.Content {
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

	body, serr := entryCodec.SerializeResponse(resp)
	if serr != nil {
		return nil, translateCodecError(serr)
	}

	if info.BambooDebug != nil {
		info.BambooDebug.RelayResponse = bamboorelay.FormatRelayResponse("CompleteRelay", codecFmt, codecFmt, body)
	}

	c.Writer.Header().Set("Content-Type", "application/json")
	c.Writer.Write(body)

	info.ResponseBody = truncateResponseBody(string(body))

	// bamboo.Usage.InputTokens 含缓存，Claude 语义计费要求 PromptTokens 仅含非缓存部分。
	nonCachedInput := resp.Usage.InputTokens -
		resp.Usage.CacheReadInputTokens -
		resp.Usage.CacheCreationInputTokens
	if nonCachedInput < 0 {
		nonCachedInput = 0
	}

	return &dto.Usage{
		PromptTokens:     int(nonCachedInput),
		CompletionTokens: int(resp.Usage.OutputTokens),
		TotalTokens:      int(nonCachedInput + resp.Usage.OutputTokens),
		UsageSemantic:    "anthropic",
		PromptTokensDetails: dto.InputTokenDetails{
			CachedTokens:         int(resp.Usage.CacheReadInputTokens),
			CachedCreationTokens: int(resp.Usage.CacheCreationInputTokens),
		},
	}, nil
}

func extractBambooRelayData(req *bamboocodec.RelayRequest) *relaycommon.BambooRelayExtract {
	if req == nil {
		return nil
	}
	extract := &relaycommon.BambooRelayExtract{
		System: req.System,
	}
	for _, msg := range req.Messages {
		msgExt := relaycommon.BambooMessageExtract{
			Role: string(msg.Role),
		}
		for _, block := range msg.Content {
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
			msgExt.Blocks = append(msgExt.Blocks, ext)
		}
		extract.Messages = append(extract.Messages, msgExt)
	}
	return extract
}

package bamboo

import (
	"context"
	"fmt"
	"net/http"
	"strings"

	bamboosdk "github.com/bamboo-services/bamboo-messages/bamboo"
	bamboocodec "github.com/bamboo-services/bamboo-messages/bamboo/codec"
	bamboorelay "github.com/bamboo-services/bamboo-messages/bamboo/relay"
	"github.com/bamboo-services/bamboo-messages/provider"
	"github.com/gin-gonic/gin"

	"github.com/QuantumNous/new-api/common"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/relaykit/types"
	"github.com/QuantumNous/new-api/setting/model_setting"
)

// ResponsesWSMemory 保存一条 Responses WebSocket 连接上的多轮快照。
// 上游不是原生 Responses（Anthropic / Gemini / Chat Completions）时，
// previous_response_id 靠这里续接工具调用和思维签名。
type ResponsesWSMemory struct {
	store *bamboorelay.ResponsesSessionStore
}

// NewResponsesWSMemory 创建连接级会话存储。
func NewResponsesWSMemory() *ResponsesWSMemory {
	return &ResponsesWSMemory{store: bamboorelay.NewResponsesSessionStore()}
}

// ResponsesWSTurnResult 是一轮 response.create 的写出结果。
// Delivered 为 true 表示至少一帧已经写给客户端，调用方应按实际用量结算而不是整单退款。
type ResponsesWSTurnResult struct {
	Delivered bool
}

// StreamResponsesWebSocket 把一条 Responses 请求经 bamboo 转成任意已支持的上游，
// 再把出口事件写成 Responses WebSocket 的纯 JSON 帧。
//
// 原生 Responses WebSocket 透传不走这里。这里只服务「客户端是 Responses WS、
// 上游是 bamboo 能对话的协议」这一侧，并复用 SDK v1.1.0 的会话快照和 SSE 拆帧。
// skipGenerate 对应官方 generate:false 预热：不打上游，只记住上下文并回空 completed。
func StreamResponsesWebSocket(
	ctx context.Context,
	c *gin.Context,
	info *relaycommon.RelayInfo,
	requestBody []byte,
	streamID string,
	skipGenerate bool,
	memory *ResponsesWSMemory,
	write func([]byte) error,
) (ResponsesWSTurnResult, *types.NewAPIError) {
	var result ResponsesWSTurnResult
	if ctx == nil {
		ctx = context.Background()
	}
	if memory == nil || memory.store == nil {
		memory = NewResponsesWSMemory()
	}
	entryCodec, gerr := bamboocodec.Get(bamboocodec.FormatResponses)
	if gerr != nil || entryCodec == nil {
		return result, types.NewError(fmt.Errorf("bamboo codec not registered: responses"), types.ErrorCodeInvalidRequest, types.ErrOptionWithSkipRetry())
	}
	relayReq, parseErr := entryCodec.ParseRequest(requestBody)
	if parseErr != nil {
		apiErr := translateCodecError(parseErr)
		if apiErr != nil && (apiErr.StatusCode == 0 || apiErr.StatusCode == http.StatusInternalServerError) {
			apiErr.StatusCode = http.StatusBadRequest
		}
		return result, types.NewError(apiErr, types.ErrorCodeInvalidRequest, types.ErrOptionWithSkipRetry(), types.ErrOptionWithStatusCode(http.StatusBadRequest))
	}
	if relayReq.Config == nil {
		relayReq.Config = &bamboosdk.RequestConfig{}
	}
	if info != nil && info.IsModelMapped && info.GetUpstreamModelName() != "" {
		relayReq.Config.Model = info.GetUpstreamModelName()
	} else if relayReq.Config.Model == "" && info != nil {
		if model := info.GetUpstreamModelName(); model != "" {
			relayReq.Config.Model = model
		} else {
			relayReq.Config.Model = info.GetOriginModelName()
		}
	}
	relayReq.IsStream = true
	if info != nil && relayReq.Config.MaxTokens <= 0 && resolveUpstreamRelayFormat(info) == types.RelayFormatClaude {
		if def := model_setting.GetClaudeSettings().GetDefaultMaxTokens(relayReq.Config.Model); def > 0 {
			relayReq.Config.MaxTokens = int64(def)
		}
	}

	streamIDFromReq, _ := provider.GetExtraString(relayReq.Config.ProviderExtra, "stream_id")
	if streamID == "" {
		streamID = streamIDFromReq
	}
	prevID, _ := provider.GetExtraString(relayReq.Config.ProviderExtra, "previous_response_id")

	p, upstreamFormat, provErr := newProvider(c, info)
	if provErr != nil {
		return result, provErr
	}
	effective := upstreamFormat
	if effective == "" && info != nil {
		effective = info.RelayFormat
	}
	if info != nil && effective != "" {
		info.AppendRequestConversion(effective)
		info.FinalRequestRelayFormat = effective
	}

	if prevID != "" {
		prevSys, prevMsgs, ok := memory.store.Load(prevID)
		if !ok {
			if p.GetProviderType() != provider.ProviderOpenAIResponses {
				return result, types.NewErrorWithStatusCode(
					fmt.Errorf("previous_response_id %q not found", prevID),
					types.ErrorCodeInvalidRequest,
					http.StatusBadRequest,
					types.ErrOptionWithSkipRetry(),
				)
			}
		} else {
			if relayReq.System == "" {
				relayReq.System = prevSys
			}
			combined := make([]bamboosdk.BambooMessage, 0, len(prevMsgs)+len(relayReq.Messages))
			combined = append(combined, prevMsgs...)
			combined = append(combined, relayReq.Messages...)
			relayReq.Messages = combined
		}
	}

	if skipGenerate {
		warmupID := fmt.Sprintf("resp_warmup_%d", len(relayReq.Messages)+1)
		memory.store.Save(warmupID, relayReq.System, relayReq.Messages)
		if err := writeResponsesWSWarmup(write, streamID, warmupID, relayReq.Config.Model, &result); err != nil {
			return result, types.NewError(err, types.ErrorCodeBadResponse, types.ErrOptionWithSkipRetry())
		}
		if info != nil {
			ensureStreamStatus(info).SetEndReason(relaycommon.StreamEndReasonDone, nil)
		}
		return result, nil
	}

	client := bamboosdk.NewClient(p)
	eventCh, err := client.Chat(ctx, relayReq.Messages, relayReq.System, relayReq.Config)
	if err != nil {
		if ctx.Err() != nil || isClientCancel(c, err) {
			if info != nil {
				ensureStreamStatus(info).SetEndReason(relaycommon.StreamEndReasonClientGone, err)
			}
			return result, nil
		}
		return result, withBambooStatus(translateSDKError(err), err)
	}

	serializer := entryCodec.NewSerializer(relayReq.Config.Model)
	collector := newResponsesWSCollector()
	var responseID string
	var frames []string

	emit := func(sseBytes []byte) error {
		for _, payload := range bamboorelay.ExtractSSEDataPayloads(sseBytes) {
			if id := responsesWSResponseID(payload); id != "" {
				responseID = id
			}
			out := injectResponsesStreamID(payload, streamID)
			if write != nil {
				if werr := write(out); werr != nil {
					return werr
				}
			}
			result.Delivered = true
			if len(frames) < 32 {
				frames = append(frames, string(out))
			}
		}
		return nil
	}

	for event := range eventCh {
		if event.Type == bamboosdk.EventError && event.Error != nil {
			collector.failed = true
			if info != nil {
				cause := error(event.Error)
				if ctx.Err() != nil || isClientCancel(c, cause) {
					ensureStreamStatus(info).SetEndReason(relaycommon.StreamEndReasonClientGone, cause)
				} else {
					ensureStreamStatus(info).SetEndReason(relaycommon.StreamEndReasonScannerErr, cause)
				}
			}
		}
		collector.observe(event)
		sseBytes, sErr := serializer.Serialize(event)
		if sErr != nil {
			continue
		}
		if werr := emit(sseBytes); werr != nil {
			return result, types.NewError(werr, types.ErrorCodeBadResponse, types.ErrOptionWithSkipRetry())
		}
		if event.Type == bamboosdk.EventError && event.Error != nil && ctx.Err() == nil && !isClientCancel(c, event.Error) {
			if info != nil && len(frames) > 0 {
				info.ResponseBody = truncateResponseBody(strings.Join(frames, "\n"))
			}
			return result, withBambooStatus(types.NewError(event.Error, types.ErrorCodeBadResponseBody), event.Error)
		}
	}
	if flushBytes, fErr := serializer.Flush(); fErr == nil && len(flushBytes) > 0 {
		if werr := emit(flushBytes); werr != nil {
			return result, types.NewError(werr, types.ErrorCodeBadResponse, types.ErrOptionWithSkipRetry())
		}
	}
	if info != nil && len(frames) > 0 {
		info.ResponseBody = truncateResponseBody(strings.Join(frames, "\n"))
	}
	if responseID != "" && !collector.failed {
		updated := make([]bamboosdk.BambooMessage, 0, len(relayReq.Messages)+1)
		updated = append(updated, relayReq.Messages...)
		if assistant, ok := collector.buildMessage(); ok {
			updated = append(updated, assistant)
		}
		memory.store.Save(responseID, relayReq.System, updated)
	}
	if info != nil && ensureStreamStatus(info).EndReason == "" {
		if ctx.Err() != nil {
			ensureStreamStatus(info).SetEndReason(relaycommon.StreamEndReasonClientGone, ctx.Err())
		} else {
			ensureStreamStatus(info).SetEndReason(relaycommon.StreamEndReasonDone, nil)
		}
	}
	return result, nil
}

func writeResponsesWSWarmup(write func([]byte) error, streamID, responseID, model string, result *ResponsesWSTurnResult) error {
	for i, status := range []string{"in_progress", "completed"} {
		evType := "response.created"
		if i == 1 {
			evType = "response.completed"
		}
		payload := map[string]any{
			"type":            evType,
			"sequence_number": i + 1,
			"response_id":     responseID,
			"response": map[string]any{
				"id":     responseID,
				"object": "response",
				"status": status,
				"model":  model,
				"output": []any{},
			},
		}
		if streamID != "" {
			payload["stream_id"] = streamID
		}
		data, err := common.Marshal(payload)
		if err != nil {
			return err
		}
		if write != nil {
			if err := write(data); err != nil {
				return err
			}
		}
		result.Delivered = true
	}
	return nil
}

func responsesWSResponseID(payload []byte) string {
	var env struct {
		ResponseID string `json:"response_id"`
		Response   *struct {
			ID string `json:"id"`
		} `json:"response"`
	}
	if common.Unmarshal(payload, &env) != nil {
		return ""
	}
	if env.Response != nil && env.Response.ID != "" {
		return env.Response.ID
	}
	return env.ResponseID
}

func injectResponsesStreamID(payload []byte, streamID string) []byte {
	if streamID == "" {
		return payload
	}
	var obj map[string]any
	if common.Unmarshal(payload, &obj) != nil {
		return payload
	}
	obj["stream_id"] = streamID
	out, err := common.Marshal(obj)
	if err != nil {
		return payload
	}
	return out
}

// responsesWSCollector 收集一轮 assistant 输出，供 previous_response_id 续接。
// 语义与 bamboo-messages v1.1.0 的连接内收集器一致，但结算和鉴权留在网关侧。
type responsesWSCollector struct {
	blocks      map[int]bamboosdk.ContentBlock
	inputJSON   map[int]*strings.Builder
	order       []int
	reasoningID string
	failed      bool
}

func newResponsesWSCollector() *responsesWSCollector {
	return &responsesWSCollector{
		blocks:    make(map[int]bamboosdk.ContentBlock),
		inputJSON: make(map[int]*strings.Builder),
	}
}

func (c *responsesWSCollector) observe(ev bamboosdk.StreamEvent) {
	switch ev.Type {
	case bamboosdk.EventError:
		c.failed = true
	case bamboosdk.EventMessageStart, bamboosdk.EventMessageDelta:
		if md, ok := ev.Delta.(*bamboosdk.MessageDelta); ok && md.ReasoningID != "" {
			c.reasoningID = md.ReasoningID
		}
	case bamboosdk.EventContentBlockStart:
		if ev.ContentBlock == nil {
			return
		}
		c.order = append(c.order, ev.Index)
		switch b := ev.ContentBlock.(type) {
		case *bamboosdk.TextBlock:
			cp := *b
			c.blocks[ev.Index] = &cp
		case *bamboosdk.ThinkingBlock:
			cp := *b
			c.blocks[ev.Index] = &cp
		case *bamboosdk.ToolUseBlock:
			cp := *b
			c.blocks[ev.Index] = &cp
			c.inputJSON[ev.Index] = &strings.Builder{}
		}
	case bamboosdk.EventContentBlockDelta:
		delta, ok := ev.Delta.(*bamboosdk.StreamDelta)
		if !ok {
			return
		}
		switch delta.Type {
		case bamboosdk.DeltaTextDelta:
			if tb, ok := c.blocks[ev.Index].(*bamboosdk.TextBlock); ok {
				tb.Text += delta.Text
			}
		case bamboosdk.DeltaThinkingDelta:
			if th, ok := c.blocks[ev.Index].(*bamboosdk.ThinkingBlock); ok {
				th.Thinking += delta.Thinking
			}
		case bamboosdk.DeltaSignature:
			if th, ok := c.blocks[ev.Index].(*bamboosdk.ThinkingBlock); ok {
				th.Signature = delta.Signature
				if delta.SignatureProvider != "" {
					th.SignatureProvider = delta.SignatureProvider
				}
			}
		case bamboosdk.DeltaInputJSON:
			if sb, ok := c.inputJSON[ev.Index]; ok {
				sb.WriteString(delta.PartialJSON)
			}
		}
	case bamboosdk.EventContentBlockStop:
		if tu, ok := c.blocks[ev.Index].(*bamboosdk.ToolUseBlock); ok {
			if sb, ok := c.inputJSON[ev.Index]; ok {
				raw := strings.TrimSpace(sb.String())
				if raw == "" {
					raw = "{}"
				}
				tu.Input = []byte(raw)
			}
		}
	}
}

func (c *responsesWSCollector) buildMessage() (bamboosdk.BambooMessage, bool) {
	if len(c.order) == 0 {
		return bamboosdk.BambooMessage{}, false
	}
	var content []bamboosdk.ContentBlock
	for _, idx := range c.order {
		if b := c.blocks[idx]; b != nil {
			content = append(content, b)
		}
	}
	if len(content) == 0 {
		return bamboosdk.BambooMessage{}, false
	}
	msg := bamboosdk.NewAssistantMessageBlocks(content...)
	msg.ReasoningID = c.reasoningID
	return msg, true
}

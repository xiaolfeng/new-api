package hosttool

import (
	"strings"

	bamboosdk "github.com/bamboo-services/bamboo-messages/bamboo"
	bamboocodec "github.com/bamboo-services/bamboo-messages/bamboo/codec"
	"github.com/gin-gonic/gin"
	"github.com/shopspring/decimal"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/relaykit/dto"
	"github.com/QuantumNous/new-api/service"
	"github.com/QuantumNous/new-api/setting/model_setting"
)

type Action string

const (
	ActionPassthrough Action = "passthrough"
	ActionFoldB       Action = "fold_b"
	ActionHop2        Action = "hop2"
)

func DecideAction(plan *relaycommon.HostToolPlan, uses []toolUseCall) Action {
	return DecideActionForClient(plan, uses, nil)
}

func DecideActionForClient(plan *relaycommon.HostToolPlan, uses []toolUseCall, info *relaycommon.RelayInfo) Action {
	if passthroughClientOwnedTools(info, uses) {
		return ActionPassthrough
	}
	if len(uses) == 0 {
		return ActionPassthrough
	}
	if !AllHostToolUses(plan, uses) {
		return ActionPassthrough
	}
	if plan != nil && plan.Mode == ModeReturn {
		return ActionFoldB
	}
	return ActionHop2
}

// clientOwnedHostTool 是「客户端工具所有权」表：这些 Agent 客户端自带工具主循环，
// 收到网关代跑后抛回的同名 tool_call 会再执行一轮，形成无限搜索循环。命中该表的
// canonical 工具一律透传首个 tool_call 由客户端本地执行，echo 阶段也不再回抛。
// 规则只允许维护在本表；决策（DecideActionForClient）与回抛抑制
// （SuppressHostToolEcho）必须查表，禁止按模型名或客户端散写 if 分支。
type clientOwnedHostTool struct {
	profile   common.ClientProfile
	canonical string // CanonicalWebSearch / CanonicalWebFetch
}

var clientOwnedHostTools = []clientOwnedHostTool{
	{profile: common.ClientProfileClaudeCode, canonical: CanonicalWebSearch},
}

// agentLocalSearchLoopClients 是自带搜索主循环、但没有可用本地搜索代理的 Agent 客户端：
// 回抛 web_search 会被其本地主循环当函数再次执行（如 Grok Build 硬编码直连
// cli-chat-proxy.grok.com），非官方网络环境必然超时并重试成死循环。这类客户端由网关
// 代跑（A-thin），且 hop2 后抑制回抛，直接交付最终答案。
var agentLocalSearchLoopClients = []common.ClientProfile{
	common.ClientProfileGrokBuild,
	common.ClientProfileCodex,
}

// clientOwnedTool 查所有权表。EnableClientStrictEgress 关闭时全部视为网关所有。
func clientOwnedTool(info *relaycommon.RelayInfo, canonical string) bool {
	if info == nil || canonical == "" {
		return false
	}
	if !model_setting.GetBambooSettings().ClientStrictEgressEnabled() {
		return false
	}
	for _, entry := range clientOwnedHostTools {
		if entry.profile == info.ClientProfile && entry.canonical == canonical {
			return true
		}
	}
	return false
}

func passthroughClientOwnedTools(info *relaycommon.RelayInfo, uses []toolUseCall) bool {
	if len(uses) == 0 {
		return false
	}
	for _, use := range uses {
		if !clientOwnedTool(info, CanonicalFromName(use.Name)) {
			return false
		}
	}
	return true
}

// SuppressHostToolEcho 报告某条网关代跑结果是否不应向客户端回抛 tool_call。
// ① 命中客户端所有权表的工具（如 Claude Code 的 WebSearch）——回抛会双跑；
// ② agentLocalSearchLoopClients 表内的 Agent 客户端——回抛会被其本地主循环直连官方代理重试。
func SuppressHostToolEcho(info *relaycommon.RelayInfo, r ExecResult) bool {
	if r.Kind != "search" {
		return false
	}
	if info == nil {
		return false
	}
	if !model_setting.GetBambooSettings().ClientStrictEgressEnabled() {
		return false
	}
	for _, profile := range agentLocalSearchLoopClients {
		if info.ClientProfile == profile {
			return true
		}
	}
	return clientOwnedTool(info, CanonicalFromName(r.OriginalName))
}

func BuildHop2Request(req *bamboocodec.RelayRequest, hop1 []bamboosdk.ContentBlock, results []ExecResult, uses []toolUseCall) *bamboocodec.RelayRequest {
	if req == nil {
		return nil
	}
	assistantBlocks := make([]bamboosdk.ContentBlock, 0, len(hop1))
	for _, block := range hop1 {
		switch block.(type) {
		case *bamboosdk.ToolResultBlock:
			continue
		default:
			assistantBlocks = append(assistantBlocks, block)
		}
	}
	userBlocks := make([]bamboosdk.ContentBlock, 0, len(results))
	for i, r := range results {
		id := ""
		if i < len(uses) {
			id = uses[i].ID
		}
		content, isErr := FormatToolResult(r)
		userBlocks = append(userBlocks, bamboosdk.NewToolResultBlock(id, content, isErr))
	}
	out := *req
	msgs := make([]bamboosdk.BambooMessage, 0, len(req.Messages)+2)
	msgs = append(msgs, req.Messages...)
	msgs = append(msgs, bamboosdk.NewAssistantMessageBlocks(assistantBlocks...))
	msgs = append(msgs, bamboosdk.NewUserMessageBlocks(userBlocks...))
	out.Messages = msgs
	return &out
}

func AllowHop2(c *gin.Context, info *relaycommon.RelayInfo, hop2Req *bamboocodec.RelayRequest) (bool, string) {
	if info == nil || info.Billing == nil || info.PriceData.FreeModel {
		return true, ""
	}
	est := estimateHop2Quota(info, hop2Req)
	walletTrusted := isWalletTrusted(c, info)

	if info.BillingSource == service.BillingSourceSubscription {
		err := info.Billing.Reserve(info.Billing.GetPreConsumedQuota() + est)
		if err != nil {
			return false, "insufficient_subscription"
		}
		return true, ""
	}

	left, err := model.GetUserQuota(info.UserId, true)
	if err != nil || left < est {
		return false, "insufficient_quota"
	}
	if walletTrusted {
		return true, ""
	}
	if err := info.Billing.Reserve(info.Billing.GetPreConsumedQuota() + est); err != nil {
		return false, "reserve_error"
	}
	return true, ""
}

func estimateHop2Quota(info *relaycommon.RelayInfo, hop2Req *bamboocodec.RelayRequest) int {
	if info == nil || info.PriceData.FreeModel {
		return 0
	}
	pd := info.PriceData
	if pd.UsePrice {
		return 0
	}
	promptTokens := 0
	if hop2Req != nil {
		var b strings.Builder
		b.WriteString(hop2Req.System)
		for _, msg := range hop2Req.Messages {
			for _, block := range msg.Content {
				switch blk := block.(type) {
				case *bamboosdk.TextBlock:
					b.WriteString(blk.Text)
				case *bamboosdk.ThinkingBlock:
					b.WriteString(blk.Thinking)
				case *bamboosdk.ToolUseBlock:
					b.Write(blk.Input)
				case *bamboosdk.ToolResultBlock:
					b.WriteString(blk.Content)
				}
			}
		}
		promptTokens = service.CountTextToken(b.String(), info.OriginModelName)
	}
	maxOut := int64(0)
	if hop2Req != nil && hop2Req.Config != nil {
		maxOut = hop2Req.Config.MaxTokens
	}
	group := pd.GroupRatioInfo.GroupRatio
	sum := decimal.NewFromInt(int64(promptTokens)).
		Mul(decimal.NewFromFloat(pd.ModelRatio)).
		Mul(decimal.NewFromFloat(group))
	if maxOut > 0 {
		sum = sum.Add(decimal.NewFromInt(maxOut).
			Mul(decimal.NewFromFloat(pd.ModelRatio)).
			Mul(decimal.NewFromFloat(pd.CompletionRatio)).
			Mul(decimal.NewFromFloat(group)))
	}
	est, clamp := common.QuotaFromDecimalChecked(sum)
	if info != nil {
		if clamp != nil && info.QuotaClamp == nil {
			info.QuotaClamp = clamp
		}
	}
	return est
}

func isWalletTrusted(c *gin.Context, info *relaycommon.RelayInfo) bool {
	if info == nil || info.Billing == nil {
		return false
	}
	if info.BillingSource == service.BillingSourceSubscription {
		return false
	}
	if info.ForcePreConsume {
		return false
	}
	trust := common.GetTrustQuota()
	if trust <= 0 {
		return false
	}
	tokenQuota := 0
	if c != nil {
		tokenQuota = c.GetInt("token_quota")
	}
	if !info.TokenUnlimited && tokenQuota <= trust {
		return false
	}
	return info.UserQuota > trust
}

func AddUsage(total, hop *dto.Usage) *dto.Usage {
	if total == nil {
		if hop == nil {
			return &dto.Usage{}
		}
		cp := *hop
		return &cp
	}
	if hop == nil {
		return total
	}
	total.PromptTokens += hop.PromptTokens
	total.CompletionTokens += hop.CompletionTokens
	total.PromptTokensDetails.CachedTokens += hop.PromptTokensDetails.CachedTokens
	total.PromptTokensDetails.CachedCreationTokens += hop.PromptTokensDetails.CachedCreationTokens
	total.TotalTokens = total.PromptTokens + total.CompletionTokens
	if total.UsageSemantic == "" {
		total.UsageSemantic = hop.UsageSemantic
	}
	return total
}

func BlocksFromResponse(resp *bamboosdk.Response) []bamboosdk.ContentBlock {
	if resp == nil {
		return nil
	}
	return resp.Content
}

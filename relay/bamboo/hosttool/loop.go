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
	if shouldPassthroughClaudeClientTools(info, uses) {
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

func shouldPassthroughClaudeClientTools(info *relaycommon.RelayInfo, uses []toolUseCall) bool {
	if info == nil || info.ClientProfile != common.ClientProfileClaudeCode {
		return false
	}
	if !model_setting.GetBambooSettings().ClaudeStrictEgressEnabled() {
		return false
	}
	if len(uses) == 0 {
		return false
	}
	for _, use := range uses {
		if use.Name != "WebSearch" && use.Name != "WebFetch" {
			return false
		}
	}
	return true
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

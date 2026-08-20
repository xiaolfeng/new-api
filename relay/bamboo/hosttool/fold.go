package hosttool

import (
	bamboosdk "github.com/bamboo-services/bamboo-messages/bamboo"
)

func FoldResponse(id, model string, hop1 []bamboosdk.ContentBlock, results []ExecResult, usage bamboosdk.Usage) *bamboosdk.Response {
	blocks := make([]bamboosdk.ContentBlock, 0, len(hop1)+1)
	for _, block := range hop1 {
		switch b := block.(type) {
		case *bamboosdk.ThinkingBlock:
			blocks = append(blocks, b)
		case *bamboosdk.TextBlock:
			if b != nil && b.Text != "" {
				blocks = append(blocks, b)
			}
		case *bamboosdk.RedactedThinkingBlock:
			blocks = append(blocks, b)
		}
	}
	dump := JoinFormattedResults(results)
	if dump != "" {
		blocks = append(blocks, bamboosdk.NewTextBlock(dump))
	}
	if id == "" {
		id = "msg_host_fold"
	}
	return &bamboosdk.Response{
		ID:         id,
		Type:       "message",
		Role:       bamboosdk.RoleAssistant,
		Content:    blocks,
		Model:      model,
		StopReason: bamboosdk.FinishReasonEndTurn,
		Usage:      usage,
	}
}

func hop1UsageFromDTO(prompt, completion, cacheRead, cacheCreate int) bamboosdk.Usage {
	return bamboosdk.Usage{
		InputTokens:              int64(prompt + cacheRead + cacheCreate),
		OutputTokens:             int64(completion),
		CacheReadInputTokens:     int64(cacheRead),
		CacheCreationInputTokens: int64(cacheCreate),
	}
}

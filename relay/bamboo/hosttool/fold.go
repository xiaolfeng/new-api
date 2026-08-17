package hosttool

import (
	bamboosdk "github.com/bamboo-services/bamboo-messages/bamboo"
)

const foldDeltaRunes = 512

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

func FoldToStreamEvents(resp *bamboosdk.Response) []bamboosdk.StreamEvent {
	if resp == nil {
		return nil
	}
	usage := resp.Usage
	events := []bamboosdk.StreamEvent{
		{
			Type:    bamboosdk.EventMessageStart,
			Message: &bamboosdk.BambooMessage{Role: bamboosdk.RoleAssistant, Content: nil},
			Usage:   &usage,
		},
	}
	for i, block := range resp.Content {
		switch b := block.(type) {
		case *bamboosdk.ThinkingBlock:
			events = append(events, bamboosdk.StreamEvent{
				Type:         bamboosdk.EventContentBlockStart,
				Index:        i,
				ContentBlock: &bamboosdk.ThinkingBlock{Type: bamboosdk.ContentBlockThinking},
			})
			for _, part := range splitRunes(b.Thinking, foldDeltaRunes) {
				events = append(events, bamboosdk.StreamEvent{
					Type:  bamboosdk.EventContentBlockDelta,
					Index: i,
					Delta: &bamboosdk.StreamDelta{Type: bamboosdk.DeltaThinkingDelta, Thinking: part},
				})
			}
		case *bamboosdk.TextBlock:
			events = append(events, bamboosdk.StreamEvent{
				Type:         bamboosdk.EventContentBlockStart,
				Index:        i,
				ContentBlock: &bamboosdk.TextBlock{Type: bamboosdk.ContentBlockText},
			})
			for _, part := range splitRunes(b.Text, foldDeltaRunes) {
				events = append(events, bamboosdk.StreamEvent{
					Type:  bamboosdk.EventContentBlockDelta,
					Index: i,
					Delta: &bamboosdk.StreamDelta{Type: bamboosdk.DeltaTextDelta, Text: part},
				})
			}
		default:
			continue
		}
		events = append(events, bamboosdk.StreamEvent{Type: bamboosdk.EventContentBlockStop, Index: i})
	}
	events = append(events,
		bamboosdk.StreamEvent{
			Type:  bamboosdk.EventMessageDelta,
			Delta: &bamboosdk.MessageDelta{StopReason: bamboosdk.FinishReasonEndTurn},
			Usage: &usage,
		},
		bamboosdk.StreamEvent{Type: bamboosdk.EventMessageStop},
	)
	return events
}

func splitRunes(s string, size int) []string {
	if s == "" {
		return nil
	}
	if size <= 0 {
		size = foldDeltaRunes
	}
	runes := []rune(s)
	if len(runes) <= size {
		return []string{s}
	}
	out := make([]string, 0, (len(runes)+size-1)/size)
	for i := 0; i < len(runes); i += size {
		end := i + size
		if end > len(runes) {
			end = len(runes)
		}
		out = append(out, string(runes[i:end]))
	}
	return out
}

func hop1UsageFromDTO(prompt, completion, cacheRead, cacheCreate int) bamboosdk.Usage {
	return bamboosdk.Usage{
		InputTokens:              int64(prompt + cacheRead + cacheCreate),
		OutputTokens:             int64(completion),
		CacheReadInputTokens:     int64(cacheRead),
		CacheCreationInputTokens: int64(cacheCreate),
	}
}

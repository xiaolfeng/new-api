package bamboo

import (
	"testing"

	bamboosdk "github.com/bamboo-services/bamboo-messages/bamboo"
	"github.com/QuantumNous/new-api/dto"
)

func TestExtractStreamUsage_MessageStart(t *testing.T) {
	usage := &dto.Usage{}
	event := bamboosdk.StreamEvent{
		Type: bamboosdk.EventMessageStart,
		Usage: &bamboosdk.Usage{
			InputTokens:              100,
			CacheReadInputTokens:     50,
			CacheCreationInputTokens: 30,
		},
	}

	extractStreamUsage(usage, &event)

	if usage.PromptTokens != 100 {
		t.Fatalf("expected PromptTokens=100, got %d", usage.PromptTokens)
	}
	if usage.PromptTokensDetails.CachedTokens != 50 {
		t.Fatalf("expected CachedTokens=50, got %d", usage.PromptTokensDetails.CachedTokens)
	}
	if usage.PromptTokensDetails.CachedCreationTokens != 30 {
		t.Fatalf("expected CachedCreationTokens=30, got %d", usage.PromptTokensDetails.CachedCreationTokens)
	}
	if usage.CompletionTokens != 0 {
		t.Fatalf("expected CompletionTokens=0, got %d", usage.CompletionTokens)
	}
}

func TestExtractStreamUsage_MessageDeltaOutputOnly(t *testing.T) {
	usage := &dto.Usage{
		PromptTokens:         100,
		CompletionTokens:     0,
		PromptTokensDetails: dto.InputTokenDetails{
			CachedTokens:         50,
			CachedCreationTokens: 30,
		},
	}
	event := bamboosdk.StreamEvent{
		Type: bamboosdk.EventMessageDelta,
		Usage: &bamboosdk.Usage{
			OutputTokens: 200,
		},
	}

	extractStreamUsage(usage, &event)

	if usage.CompletionTokens != 200 {
		t.Fatalf("expected CompletionTokens=200, got %d", usage.CompletionTokens)
	}
	if usage.PromptTokens != 100 {
		t.Fatalf("expected PromptTokens unchanged (100), got %d", usage.PromptTokens)
	}
	if usage.PromptTokensDetails.CachedTokens != 50 {
		t.Fatalf("expected CachedTokens unchanged (50), got %d", usage.PromptTokensDetails.CachedTokens)
	}
	if usage.PromptTokensDetails.CachedCreationTokens != 30 {
		t.Fatalf("expected CachedCreationTokens unchanged (30), got %d", usage.PromptTokensDetails.CachedCreationTokens)
	}
}

func TestExtractStreamUsage_MessageDeltaZerosDoNotOverwrite(t *testing.T) {
	usage := &dto.Usage{
		PromptTokens:         100,
		CompletionTokens:     200,
		PromptTokensDetails: dto.InputTokenDetails{
			CachedTokens:         50,
			CachedCreationTokens: 30,
		},
	}
	event := bamboosdk.StreamEvent{
		Type: bamboosdk.EventMessageDelta,
		Usage: &bamboosdk.Usage{
			InputTokens:              0,
			OutputTokens:             0,
			CacheReadInputTokens:     0,
			CacheCreationInputTokens: 0,
		},
	}

	extractStreamUsage(usage, &event)

	if usage.PromptTokens != 100 {
		t.Fatalf("expected PromptTokens unchanged (100), got %d", usage.PromptTokens)
	}
	if usage.CompletionTokens != 200 {
		t.Fatalf("expected CompletionTokens unchanged (200), got %d", usage.CompletionTokens)
	}
	if usage.PromptTokensDetails.CachedTokens != 50 {
		t.Fatalf("expected CachedTokens unchanged (50), got %d", usage.PromptTokensDetails.CachedTokens)
	}
	if usage.PromptTokensDetails.CachedCreationTokens != 30 {
		t.Fatalf("expected CachedCreationTokens unchanged (30), got %d", usage.PromptTokensDetails.CachedCreationTokens)
	}
}

func TestExtractStreamUsage_MessageStartFollowedByMessageDelta(t *testing.T) {
	usage := &dto.Usage{}

	startEvent := &bamboosdk.StreamEvent{
		Type: bamboosdk.EventMessageStart,
		Usage: &bamboosdk.Usage{
			InputTokens:              100,
			CacheReadInputTokens:     50,
			CacheCreationInputTokens: 30,
		},
	}
	deltaEvent := &bamboosdk.StreamEvent{
		Type: bamboosdk.EventMessageDelta,
		Usage: &bamboosdk.Usage{
			OutputTokens: 200,
		},
	}

	extractStreamUsage(usage, startEvent)
	extractStreamUsage(usage, deltaEvent)

	if usage.PromptTokens != 100 {
		t.Fatalf("expected PromptTokens=100, got %d", usage.PromptTokens)
	}
	if usage.CompletionTokens != 200 {
		t.Fatalf("expected CompletionTokens=200, got %d", usage.CompletionTokens)
	}
	if usage.PromptTokensDetails.CachedTokens != 50 {
		t.Fatalf("expected CachedTokens=50, got %d", usage.PromptTokensDetails.CachedTokens)
	}
	if usage.PromptTokensDetails.CachedCreationTokens != 30 {
		t.Fatalf("expected CachedCreationTokens=30, got %d", usage.PromptTokensDetails.CachedCreationTokens)
	}
}

func TestExtractStreamUsage_PingCarriesInputTokens(t *testing.T) {
	usage := &dto.Usage{}
	event := bamboosdk.StreamEvent{
		Type: bamboosdk.EventPing,
		Usage: &bamboosdk.Usage{
			InputTokens:              100,
			CacheReadInputTokens:     50,
			CacheCreationInputTokens: 30,
		},
	}

	extractStreamUsage(usage, &event)

	if usage.PromptTokens != 100 {
		t.Fatalf("expected PromptTokens=100, got %d", usage.PromptTokens)
	}
	if usage.PromptTokensDetails.CachedTokens != 50 {
		t.Fatalf("expected CachedTokens=50, got %d", usage.PromptTokensDetails.CachedTokens)
	}
	if usage.PromptTokensDetails.CachedCreationTokens != 30 {
		t.Fatalf("expected CachedCreationTokens=30, got %d", usage.PromptTokensDetails.CachedCreationTokens)
	}
}

func TestExtractStreamUsage_PingCarriesOutputTokens(t *testing.T) {
	usage := &dto.Usage{
		PromptTokens: 100,
	}
	event := bamboosdk.StreamEvent{
		Type: bamboosdk.EventPing,
		Usage: &bamboosdk.Usage{
			OutputTokens: 200,
		},
	}

	extractStreamUsage(usage, &event)

	if usage.CompletionTokens != 200 {
		t.Fatalf("expected CompletionTokens=200, got %d", usage.CompletionTokens)
	}
	if usage.PromptTokens != 100 {
		t.Fatalf("expected PromptTokens unchanged (100), got %d", usage.PromptTokens)
	}
}

func TestExtractStreamUsage_PingFollowedByMessageDelta(t *testing.T) {
	usage := &dto.Usage{}

	pingEvent := &bamboosdk.StreamEvent{
		Type: bamboosdk.EventPing,
		Usage: &bamboosdk.Usage{
			InputTokens:              100,
			CacheReadInputTokens:     50,
			CacheCreationInputTokens: 30,
		},
	}
	deltaEvent := &bamboosdk.StreamEvent{
		Type: bamboosdk.EventMessageDelta,
		Usage: &bamboosdk.Usage{
			OutputTokens: 200,
		},
	}

	extractStreamUsage(usage, pingEvent)
	extractStreamUsage(usage, deltaEvent)

	if usage.PromptTokens != 100 {
		t.Fatalf("expected PromptTokens=100, got %d", usage.PromptTokens)
	}
	if usage.CompletionTokens != 200 {
		t.Fatalf("expected CompletionTokens=200, got %d", usage.CompletionTokens)
	}
	if usage.PromptTokensDetails.CachedTokens != 50 {
		t.Fatalf("expected CachedTokens=50, got %d", usage.PromptTokensDetails.CachedTokens)
	}
	if usage.PromptTokensDetails.CachedCreationTokens != 30 {
		t.Fatalf("expected CachedCreationTokens=30, got %d", usage.PromptTokensDetails.CachedCreationTokens)
	}
}

package bamboo

import (
	"testing"

	"github.com/QuantumNous/new-api/dto"
)

func TestAccumulateReasoning(t *testing.T) {
	usage := &dto.Usage{}
	accumulateReasoning(usage, 10)
	accumulateReasoning(usage, 5)
	if usage.CompletionTokenDetails.ReasoningTokens != 15 {
		t.Fatalf("expected ReasoningTokens=15, got %d", usage.CompletionTokenDetails.ReasoningTokens)
	}
}

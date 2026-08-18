package service

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/relaykit/dto"
)

func TestAppendUsageActivityTagsFromSuccessfulHostToolsAndImageRecognize(t *testing.T) {
	info := &relaycommon.RelayInfo{
		HostToolPlan: &relaycommon.HostToolPlan{
			Execs: []relaycommon.HostToolExecRecord{
				{Canonical: relaycommon.HostToolCanonicalSearch},
				{Canonical: relaycommon.HostToolCanonicalFetch},
				{Canonical: relaycommon.HostToolCanonicalFetch, ErrorCode: "ssrf_blocked"},
			},
		},
		ImageRecognizePlan: &relaycommon.ImageRecognizePlan{
			Enabled:    true,
			ImageCount: 2,
		},
	}
	other := map[string]interface{}{}
	appendUsageActivityTags(info, other)

	require.Equal(t, []string{"web_search", "web_fetch", "image_recognize"}, other["usage_tags"])
	assert.Equal(t, true, other["web_search"])
	assert.Equal(t, 1, other["web_search_call_count"])
	assert.Equal(t, true, other["web_fetch"])
	assert.Equal(t, 1, other["web_fetch_call_count"])
	assert.Equal(t, true, other["image_recognize"])
	assert.Equal(t, 2, other["image_recognize_image_count"])
}

func TestAppendUsageActivityTagsUsesBuiltInSearchCount(t *testing.T) {
	info := &relaycommon.RelayInfo{
		ResponsesUsageInfo: &relaycommon.ResponsesUsageInfo{
			BuiltInTools: map[string]*relaycommon.BuildInToolInfo{
				dto.BuildInToolWebSearch: {CallCount: 3},
			},
		},
	}
	other := map[string]interface{}{}
	appendUsageActivityTags(info, other)
	assert.Equal(t, []string{"web_search"}, other["usage_tags"])
	assert.Equal(t, 3, other["web_search_call_count"])
	assert.NotContains(t, other, "web_fetch")
	assert.NotContains(t, other, "image_recognize")
}

func TestAppendUsageActivityTagsSkipsFailedOrDisabled(t *testing.T) {
	info := &relaycommon.RelayInfo{
		HostToolPlan: &relaycommon.HostToolPlan{
			Execs: []relaycommon.HostToolExecRecord{
				{Canonical: relaycommon.HostToolCanonicalFetch, ErrorCode: "timeout"},
			},
		},
		ImageRecognizePlan: &relaycommon.ImageRecognizePlan{
			Enabled:       false,
			SkippedReason: relaycommon.ImageRecognizeSkipNoLatestUserImage,
		},
	}
	other := map[string]interface{}{}
	appendUsageActivityTags(info, other)
	assert.NotContains(t, other, "usage_tags")
	assert.NotContains(t, other, "web_fetch")
	assert.NotContains(t, other, "image_recognize")
}

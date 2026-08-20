package model_setting

import (
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/setting/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestResolvedImageRecognizePromptAlwaysKeepsParserRole(t *testing.T) {
	st := &BambooSettings{}
	got := st.ResolvedImageRecognizePrompt()
	require.Contains(t, got, ImageRecognizeRolePrompt)
	assert.Contains(t, got, "你只是图片解析模块")
	assert.Contains(t, got, "回答用户问题")

	st.ImageRecognizePrompt = "请重点读出图里的表格数字"
	got = st.ResolvedImageRecognizePrompt()
	assert.Contains(t, got, ImageRecognizeRolePrompt)
	assert.Contains(t, got, "请重点读出图里的表格数字")
}

func TestBambooClientProfileFlagDefaultsMissingKeys(t *testing.T) {
	st := cloneBambooBools(defaultBambooSettings)
	cm := config.NewConfigManager()
	cm.Register("bamboo", &st)

	require.NoError(t, cm.LoadFromDB(map[string]string{}))
	assert.True(t, st.GrokStrictEgressEnabled())
	assert.True(t, st.ClaudeStrictEgressEnabled())
	assert.False(t, st.ClientReturnProfilesEnabled())

	require.NoError(t, cm.LoadFromDB(map[string]string{
		"bamboo.enable_bamboo_relay": "true",
	}))
	assert.True(t, st.GrokStrictEgressEnabled())
	assert.True(t, st.ClaudeStrictEgressEnabled())
	assert.False(t, st.ClientReturnProfilesEnabled())
}

func TestBambooClientProfileFlagDefaultsWholeJSONOmitsKeys(t *testing.T) {
	st := cloneBambooBools(defaultBambooSettings)
	require.NoError(t, common.Unmarshal([]byte(`{"enable_bamboo_relay":true}`), &st))
	assert.True(t, st.GrokStrictEgressEnabled())
	assert.True(t, st.ClaudeStrictEgressEnabled())
	assert.False(t, st.ClientReturnProfilesEnabled())
}

func TestBambooClientProfileFlagJSONOverrides(t *testing.T) {
	st := cloneBambooBools(defaultBambooSettings)
	require.NoError(t, common.Unmarshal([]byte(`{"enable_grok_strict_egress":false}`), &st))
	assert.False(t, st.GrokStrictEgressEnabled())
	assert.True(t, st.ClaudeStrictEgressEnabled())
	assert.False(t, st.ClientReturnProfilesEnabled())

	st = cloneBambooBools(defaultBambooSettings)
	require.NoError(t, common.Unmarshal([]byte(`{"enable_client_return_profiles":true}`), &st))
	assert.True(t, st.ClientReturnProfilesEnabled())
	assert.True(t, st.GrokStrictEgressEnabled())
}

func TestBambooClientProfileFlagPointerIsolation(t *testing.T) {
	live := cloneBambooBools(defaultBambooSettings)
	require.NotNil(t, live.EnableGrokStrictEgress)
	require.NotSame(t, defaultBambooSettings.EnableGrokStrictEgress, live.EnableGrokStrictEgress)
	require.NotSame(t, defaultBambooSettings.EnableClaudeStrictEgress, live.EnableClaudeStrictEgress)
	require.NotSame(t, defaultBambooSettings.EnableClientReturnProfiles, live.EnableClientReturnProfiles)

	*live.EnableGrokStrictEgress = false
	*live.EnableClaudeStrictEgress = false
	cloned := cloneBambooBools(defaultBambooSettings)
	assert.True(t, cloned.GrokStrictEgressEnabled())
	assert.True(t, cloned.ClaudeStrictEgressEnabled())
	assert.True(t, defaultBambooSettings.GrokStrictEgressEnabled())
	assert.True(t, defaultBambooSettings.ClaudeStrictEgressEnabled())
}

func TestBambooClientProfileFlagNilHelpers(t *testing.T) {
	var st *BambooSettings
	assert.True(t, st.GrokStrictEgressEnabled())
	assert.True(t, st.ClaudeStrictEgressEnabled())
	assert.False(t, st.ClientReturnProfilesEnabled())

	empty := &BambooSettings{}
	assert.True(t, empty.GrokStrictEgressEnabled())
	assert.True(t, empty.ClaudeStrictEgressEnabled())
	assert.False(t, empty.ClientReturnProfilesEnabled())
}

func TestClampImageRecognizeRetryTimes(t *testing.T) {
	st := &BambooSettings{}
	assert.Equal(t, 0, st.ClampImageRecognizeRetryTimes(), "zero value means no retry")
	st.ImageRecognizeRetryTimes = 0
	assert.Equal(t, 0, st.ClampImageRecognizeRetryTimes())
	st.ImageRecognizeRetryTimes = -1
	assert.Equal(t, 0, st.ClampImageRecognizeRetryTimes())
	st.ImageRecognizeRetryTimes = 8
	assert.Equal(t, 5, st.ClampImageRecognizeRetryTimes())
	st.ImageRecognizeRetryTimes = 2
	assert.Equal(t, 2, st.ClampImageRecognizeRetryTimes())
}

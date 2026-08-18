package model_setting

import (
	"testing"

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

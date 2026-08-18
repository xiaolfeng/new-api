package model

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestTagListHas(t *testing.T) {
	assert.True(t, tagListHas("Reasoning,Tools,Vision,128K", "vision"))
	assert.True(t, tagListHas("Vision", "VISION"))
	assert.False(t, tagListHas("Tools,Files,16.4K", "vision"))
	assert.False(t, tagListHas("", "vision"))
}

package bamboo

import (
	bamboosdk "github.com/bamboo-services/bamboo-messages/bamboo"

	relaycommon "github.com/QuantumNous/new-api/relay/common"
)

func visibleBoxText(info *relaycommon.RelayInfo) string {
	if info == nil || info.ImageRecognizePlan == nil {
		return ""
	}
	return info.ImageRecognizePlan.VisibleBox
}

func prependVisibleBox(resp *bamboosdk.Response, info *relaycommon.RelayInfo) {
	box := visibleBoxText(info)
	if resp == nil || box == "" {
		return
	}
	resp.Content = append([]bamboosdk.ContentBlock{bamboosdk.NewTextBlock(box)}, resp.Content...)
}

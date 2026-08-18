package bamboo

import (
	bamboosdk "github.com/bamboo-services/bamboo-messages/bamboo"
	bamboocodec "github.com/bamboo-services/bamboo-messages/bamboo/codec"
	bamboorelay "github.com/bamboo-services/bamboo-messages/bamboo/relay"

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

func visibleBoxStreamEvents(box string) []bamboosdk.StreamEvent {
	if box == "" {
		return nil
	}
	return []bamboosdk.StreamEvent{
		{
			Type:         bamboosdk.EventContentBlockStart,
			Index:        0,
			ContentBlock: bamboosdk.NewTextBlock(""),
		},
		{
			Type:  bamboosdk.EventContentBlockDelta,
			Index: 0,
			Delta: &bamboosdk.StreamDelta{Type: bamboosdk.DeltaTextDelta, Text: box},
		},
		{
			Type:  bamboosdk.EventContentBlockStop,
			Index: 0,
		},
	}
}

func writeVisibleBoxFrames(serializer bamboocodec.StreamSerializer, box string, write func([]byte) bool) bool {
	for _, event := range visibleBoxStreamEvents(box) {
		data, err := serializer.Serialize(event)
		if err != nil || data == nil {
			continue
		}
		for _, frame := range bamboorelay.SplitSSEFrames(data) {
			if !write(frame) {
				return false
			}
		}
	}
	return true
}

package perfmetrics

import (
	"testing"
	"time"

	relaycommon "github.com/QuantumNous/new-api/relay/common"
)

// TestRecordRelaySampleSkipsHostToolInternal 验证 host 内部工具合成请求
// （HostToolInternal=true，如 Claude server tool helper / Responses
// builtin-only）不产生 perf 样本，避免 0-token 内部请求稀释 TPS 统计。
func TestRecordRelaySampleSkipsHostToolInternal(t *testing.T) {
	internalModel := "host-internal-" + t.Name()

	RecordRelaySample(&relaycommon.RelayInfo{
		OriginModelName:  internalModel,
		UsingGroup:       "default",
		StartTime:        time.Now(),
		HostToolInternal: true,
	}, true, 100)

	hotBuckets.Range(func(key, _ any) bool {
		k := key.(bucketKey)
		if k.model == internalModel {
			t.Fatalf("host tool internal sample must not be recorded, found bucket for model %s", k.model)
		}
		return true
	})
}

// TestRecordRelaySampleRecordsNormal 对照验证：普通请求必须正常记录样本，
// 防止跳过条件误伤主路径（hop1+hop2 真过模型的 host-tool 请求）。
func TestRecordRelaySampleRecordsNormal(t *testing.T) {
	normalModel := "host-normal-" + t.Name()

	RecordRelaySample(&relaycommon.RelayInfo{
		OriginModelName: normalModel,
		UsingGroup:      "default",
		StartTime:       time.Now(),
	}, true, 100)

	found := false
	hotBuckets.Range(func(key, _ any) bool {
		if key.(bucketKey).model == normalModel {
			found = true
		}
		return true
	})
	if !found {
		t.Fatalf("normal sample should be recorded for model %s", normalModel)
	}
}
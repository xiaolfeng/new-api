/*
Copyright (C) 2023-2026 QuantumNous

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as
published by the Free Software Foundation, either version 3 of the
License, or (at your option) any later version.

This program is distributed in the hope that it will be useful,
but WITHOUT ANY WARRANTY; without even the implied warranty of
MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
GNU Affero General Public License for more details.

You should have received a copy of the GNU Affero General Public License
along with this program. If not, see <https://www.gnu.org/licenses/>.

For commercial licensing, please contact support@quantumnous.com
*/
import { describe, expect, it } from "vitest";

import type {
  TokenRecordHourCell,
  TokenRecordRecentItem,
  TokenRecordSummary,
} from "../../types";
import type { TokenPhase } from "../metrics";
import { buildModelRanking } from "../ranking";

function makeCell(
  bucketStartAt: number,
  values: Partial<TokenRecordHourCell>,
): TokenRecordHourCell {
  return {
    bucket_start_at: bucketStartAt,
    bucket_end_at: bucketStartAt + 3599,
    request_count: 0,
    prompt_tokens: 0,
    completion_tokens: 0,
    total_tokens: 0,
    total_use_time: 0,
    avg_tps: null,
    tps_sample_count: 0,
    thinking_tokens: 0,
    thinking_duration_ms: 0,
    avg_thinking_tps: null,
    thinking_sample_count: 0,
    output_tokens: 0,
    output_duration_ms: 0,
    avg_output_tps: null,
    output_sample_count: 0,
    tool_tokens: 0,
    tool_duration_ms: 0,
    avg_tool_tps: null,
    tool_sample_count: 0,
    failed_count: 0,
    failed_detail: {},
    is_current: false,
    ...values,
  };
}

function buildSummary(cells: TokenRecordHourCell[]): TokenRecordSummary {
  const requestCount = cells.reduce((sum, cell) => sum + cell.request_count, 0);
  const failedCount = cells.reduce((sum, cell) => sum + cell.failed_count, 0);
  const completionTokens = cells.reduce(
    (sum, cell) => sum + cell.completion_tokens,
    0,
  );
  const totalUseTime = cells.reduce(
    (sum, cell) => sum + cell.total_use_time,
    0,
  );
  const tpsSampleCount = cells.reduce(
    (sum, cell) => sum + (cell.tps_sample_count ?? 0),
    0,
  );
  const weightedTps = cells.reduce((sum, cell) => {
    if (cell.avg_tps === null) {
      return sum;
    }
    return sum + cell.avg_tps * (cell.tps_sample_count ?? 0);
  }, 0);
  const totalRequests = requestCount + failedCount;

  const phaseSummary = (
    phase: "thinking" | "output" | "tool",
  ): {
    tokens: number;
    avgTps: number | null;
    sampleCount: number;
  } => {
    const tokensField = `${phase}_tokens` as const;
    const avgField = `avg_${phase}_tps` as const;
    const sampleField = `${phase}_sample_count` as const;
    const tokens = cells.reduce((sum, cell) => sum + cell[tokensField], 0);
    const sampleCount = cells.reduce(
      (sum, cell) => sum + (cell[sampleField] ?? 0),
      0,
    );
    const weighted = cells.reduce((sum, cell) => {
      const avg = cell[avgField];
      if (avg === null || avg === undefined) {
        return sum;
      }
      return sum + avg * (cell[sampleField] ?? 0);
    }, 0);
    return {
      tokens,
      avgTps: sampleCount > 0 ? weighted / sampleCount : null,
      sampleCount,
    };
  };

  const thinking = phaseSummary("thinking");
  const output = phaseSummary("output");
  const tool = phaseSummary("tool");

  return {
    request_count: requestCount,
    prompt_tokens: 0,
    completion_tokens: completionTokens,
    total_tokens: completionTokens,
    total_use_time: totalUseTime,
    avg_tps: tpsSampleCount > 0 ? weightedTps / tpsSampleCount : null,
    tps_sample_count: tpsSampleCount,
    thinking_tokens: thinking.tokens,
    thinking_duration_ms: 0,
    avg_thinking_tps: thinking.avgTps,
    thinking_sample_count: thinking.sampleCount,
    output_tokens: output.tokens,
    output_duration_ms: 0,
    avg_output_tps: output.avgTps,
    output_sample_count: output.sampleCount,
    tool_tokens: tool.tokens,
    tool_duration_ms: 0,
    avg_tool_tps: tool.avgTps,
    tool_sample_count: tool.sampleCount,
    failed_count: failedCount,
    failed_rate: totalRequests > 0 ? (failedCount / totalRequests) * 100 : 0,
    failed_detail: {},
  };
}

function makeItem(
  modelName: string,
  cells: TokenRecordHourCell[],
): TokenRecordRecentItem {
  return {
    model_name: modelName,
    cells,
    summary: buildSummary(cells),
  };
}

const BUCKETS = [1711454400, 1711458000, 1711461600];
const SOLO_BUCKET = 1711465200;

/**
 * 构造一个小时桶样本：
 * requests 请求数 / completionTokens 输出 token / trimmedTps 去首字吞吐 /
 * ttftSeconds 每请求首字延迟 / failed 失败请求数。
 */
function makeBucketCell(
  bucketStartAt: number,
  options: {
    requests: number;
    completionTokens: number;
    trimmedTps: number;
    ttftSeconds: number;
    failed?: number;
    phases?: Partial<Record<TokenPhase, { tps: number; tokens: number }>>;
  },
): TokenRecordHourCell {
  const generationPerRequest =
    options.completionTokens / (options.requests * options.trimmedTps);
  const totalPerRequest = generationPerRequest + options.ttftSeconds;

  return makeCell(bucketStartAt, {
    request_count: options.requests,
    failed_count: options.failed ?? 0,
    completion_tokens: options.completionTokens,
    total_use_time:
      Math.round(totalPerRequest * options.requests * 1000) / 1000,
    avg_tps: options.trimmedTps,
    tps_sample_count: options.requests,
    thinking_tokens: options.phases?.thinking?.tokens ?? 0,
    avg_thinking_tps: options.phases?.thinking?.tps ?? null,
    thinking_sample_count: options.phases?.thinking ? options.requests : 0,
    output_tokens: options.phases?.output?.tokens ?? 0,
    avg_output_tps: options.phases?.output?.tps ?? null,
    output_sample_count: options.phases?.output ? options.requests : 0,
    tool_tokens: options.phases?.tool?.tokens ?? 0,
    avg_tool_tps: options.phases?.tool?.tps ?? null,
    tool_sample_count: options.phases?.tool ? options.requests : 0,
  });
}

describe("model ranking", () => {
  it("ranks the faster model first with the weighted composite score", () => {
    const items = [
      makeItem(
        "model-fast",
        BUCKETS.map((bucket) =>
          makeBucketCell(bucket, {
            requests: 10,
            completionTokens: 1000,
            trimmedTps: 100,
            ttftSeconds: 0.2,
          }),
        ),
      ),
      makeItem(
        "model-slow",
        BUCKETS.map((bucket) =>
          makeBucketCell(bucket, {
            requests: 10,
            completionTokens: 500,
            trimmedTps: 50,
            ttftSeconds: 1,
          }),
        ),
      ),
    ];

    const result = buildModelRanking(items);

    expect(result.entries.map((entry) => entry.modelName)).toEqual([
      "model-fast",
      "model-slow",
    ]);
    expect(result.entries[0].rank).toBe(1);
    expect(result.entries[1].rank).toBe(2);
    expect(result.entries[0].score).toBeCloseTo(96.21, 2);
    expect(result.entries[1].score).toBeCloseTo(56.82, 2);
    expect(result.entries[0].sufficiency).toBe("low");
    expect(result.entries[0].requests).toBe(30);
    expect(result.entries[0].comparableBuckets).toBe(3);
    expect(result.insufficient).toHaveLength(0);
  });

  it("keeps models without enough comparable hours out of the main board", () => {
    const items = [
      makeItem(
        "model-stable",
        BUCKETS.map((bucket) =>
          makeBucketCell(bucket, {
            requests: 10,
            completionTokens: 1000,
            trimmedTps: 100,
            ttftSeconds: 0.2,
          }),
        ),
      ),
      makeItem(
        "model-peer",
        BUCKETS.map((bucket) =>
          makeBucketCell(bucket, {
            requests: 10,
            completionTokens: 900,
            trimmedTps: 90,
            ttftSeconds: 0.3,
          }),
        ),
      ),
      makeItem("model-rare", [
        makeBucketCell(BUCKETS[0], {
          requests: 5,
          completionTokens: 500,
          trimmedTps: 100,
          ttftSeconds: 0.2,
        }),
      ]),
    ];

    const result = buildModelRanking(items);

    expect(result.entries.map((entry) => entry.modelName)).toEqual([
      "model-stable",
      "model-peer",
    ]);
    expect(result.entries[0].comparableBuckets).toBe(3);
    expect(result.insufficient.map((entry) => entry.modelName)).toEqual([
      "model-rare",
    ]);
    expect(result.insufficient[0].sufficiency).toBe("insufficient");
    expect(result.insufficient[0].comparableBuckets).toBe(1);
    expect(result.insufficient[0].rank).toBeNull();
  });

  it("ignores hours where a model is the only participant", () => {
    const items = [
      makeItem(
        "model-a",
        BUCKETS.map((bucket) =>
          makeBucketCell(bucket, {
            requests: 10,
            completionTokens: 1000,
            trimmedTps: 100,
            ttftSeconds: 0.2,
          }),
        ),
      ),
      makeItem(
        "model-b",
        BUCKETS.map((bucket) =>
          makeBucketCell(bucket, {
            requests: 10,
            completionTokens: 900,
            trimmedTps: 90,
            ttftSeconds: 0.3,
          }),
        ),
      ),
      makeItem("model-solo", [
        makeBucketCell(SOLO_BUCKET, {
          requests: 10,
          completionTokens: 1000,
          trimmedTps: 100,
          ttftSeconds: 0.2,
        }),
      ]),
    ];

    const result = buildModelRanking(items);

    const solo = result.insufficient.find(
      (entry) => entry.modelName === "model-solo",
    );
    expect(solo).toBeDefined();
    expect(solo?.comparableBuckets).toBe(0);
    expect(solo?.score).toBeNull();
  });

  it("shares the same rank for models with equal scores", () => {
    const items = [
      makeItem(
        "model-a",
        BUCKETS.map((bucket) =>
          makeBucketCell(bucket, {
            requests: 10,
            completionTokens: 1000,
            trimmedTps: 100,
            ttftSeconds: 0.2,
          }),
        ),
      ),
      makeItem(
        "model-b",
        BUCKETS.map((bucket) =>
          makeBucketCell(bucket, {
            requests: 10,
            completionTokens: 1000,
            trimmedTps: 100,
            ttftSeconds: 0.2,
          }),
        ),
      ),
    ];

    const result = buildModelRanking(items);

    expect(result.entries.map((entry) => entry.rank)).toEqual([1, 1]);
    expect(result.entries[0].score).toBeCloseTo(
      result.entries[1].score ?? 0,
      6,
    );
  });

  it("lets first-token latency outweigh a 50% throughput gap", () => {
    const items = [
      makeItem(
        "model-throughput",
        BUCKETS.map((bucket) =>
          makeBucketCell(bucket, {
            requests: 10,
            completionTokens: 1000,
            trimmedTps: 100,
            ttftSeconds: 1,
          }),
        ),
      ),
      makeItem(
        "model-latency",
        BUCKETS.map((bucket) =>
          makeBucketCell(bucket, {
            requests: 10,
            completionTokens: 500,
            trimmedTps: 50,
            ttftSeconds: 0.2,
          }),
        ),
      ),
    ];

    const result = buildModelRanking(items);

    expect(result.entries[0].modelName).toBe("model-latency");
    expect(result.entries[0].score).toBeCloseTo(82.58, 2);
    expect(result.entries[1].score).toBeCloseTo(70.45, 2);
  });

  it("adds a small advantage to the busier model through the usage weight", () => {
    const items = [
      makeItem(
        "model-busy",
        BUCKETS.map((bucket) =>
          makeBucketCell(bucket, {
            requests: 20,
            completionTokens: 2000,
            trimmedTps: 100,
            ttftSeconds: 0.2,
          }),
        ),
      ),
      makeItem(
        "model-quiet",
        BUCKETS.map((bucket) =>
          makeBucketCell(bucket, {
            requests: 8,
            completionTokens: 800,
            trimmedTps: 100,
            ttftSeconds: 0.2,
          }),
        ),
      ),
    ];

    const result = buildModelRanking(items);

    expect(result.entries.map((entry) => entry.modelName)).toEqual([
      "model-busy",
      "model-quiet",
    ]);
    expect(result.entries[0].score).toBeCloseTo(96.21, 2);
    expect(result.entries[1].score).toBeCloseTo(93.68, 2);
  });

  it("adds the phase TPS dimension when every model in the hour has phase data", () => {
    const items = [
      makeItem(
        "model-phase-fast",
        BUCKETS.map((bucket) =>
          makeBucketCell(bucket, {
            requests: 10,
            completionTokens: 1000,
            trimmedTps: 100,
            ttftSeconds: 0.2,
            phases: {
              thinking: { tps: 60, tokens: 400 },
              output: { tps: 100, tokens: 600 },
            },
          }),
        ),
      ),
      makeItem(
        "model-phase-slow",
        BUCKETS.map((bucket) =>
          makeBucketCell(bucket, {
            requests: 10,
            completionTokens: 1000,
            trimmedTps: 100,
            ttftSeconds: 0.2,
            phases: {
              thinking: { tps: 20, tokens: 400 },
              output: { tps: 50, tokens: 600 },
            },
          }),
        ),
      ),
    ];

    const result = buildModelRanking(items);

    expect(result.entries[0].modelName).toBe("model-phase-fast");
    expect(result.entries[0].score).toBeCloseTo(96.53, 2);
    expect(result.entries[1].score).toBeCloseTo(91.96, 2);
    expect(result.entries[0].phaseTps).toBeCloseTo(84, 2);
    expect(result.entries[0].phaseEntries).toHaveLength(2);
  });

  it("drops the phase dimension when a model in the hour lacks phase data", () => {
    const items = [
      makeItem(
        "model-with-phase",
        BUCKETS.map((bucket) =>
          makeBucketCell(bucket, {
            requests: 10,
            completionTokens: 1000,
            trimmedTps: 100,
            ttftSeconds: 0.2,
            phases: { output: { tps: 100, tokens: 1000 } },
          }),
        ),
      ),
      makeItem(
        "model-without-phase",
        BUCKETS.map((bucket) =>
          makeBucketCell(bucket, {
            requests: 10,
            completionTokens: 1000,
            trimmedTps: 100,
            ttftSeconds: 0.2,
          }),
        ),
      ),
    ];

    const result = buildModelRanking(items);

    expect(result.entries[0].score).toBeCloseTo(96.21, 2);
    expect(result.entries[1].score).toBeCloseTo(96.21, 2);
    expect(result.entries.map((entry) => entry.rank)).toEqual([1, 1]);
    expect(result.entries[0].phaseTps).toBeCloseTo(100, 2);
    expect(result.entries[1].phaseTps).toBeNull();
  });

  it("penalises failed requests through the reliability dimension", () => {
    const items = [
      makeItem(
        "model-stable",
        BUCKETS.map((bucket) =>
          makeBucketCell(bucket, {
            requests: 10,
            completionTokens: 1000,
            trimmedTps: 100,
            ttftSeconds: 0.2,
          }),
        ),
      ),
      makeItem(
        "model-flaky",
        BUCKETS.map((bucket) =>
          makeBucketCell(bucket, {
            requests: 10,
            completionTokens: 1000,
            trimmedTps: 100,
            ttftSeconds: 0.2,
            failed: 5,
          }),
        ),
      ),
    ];

    const result = buildModelRanking(items);

    expect(result.entries.map((entry) => entry.modelName)).toEqual([
      "model-stable",
      "model-flaky",
    ]);
    expect(result.entries[1].reliability).toBeCloseTo(0.6667, 3);
    expect(result.entries[1].score).toBeLessThan(result.entries[0].score ?? 0);
  });
});

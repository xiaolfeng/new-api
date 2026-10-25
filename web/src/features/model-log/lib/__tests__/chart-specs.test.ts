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

import { formatTokenUnits } from "@/lib/format";

import type { TokenRecordHourCell, TokenRecordRecentItem } from "../../types";
import {
  buildStableModelColorMap,
  getChartColors,
  getPhaseColorVariant,
  getPhaseLineDash,
} from "../chart-colors";
import {
  buildEstimatedTpsChartSpec,
  buildLineChartSpec,
  formatModelTpsSummary,
  sortModelLogItems,
  transformChartData,
  transformEstimatedTpsData,
} from "../chart-specs";

function makeCell(
  bucketStartAt: number,
  values: Partial<TokenRecordHourCell>,
): TokenRecordHourCell {
  return {
    bucket_start_at: bucketStartAt,
    bucket_end_at: bucketStartAt + 3599,
    request_count: 1,
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

function makeItem(
  modelName: string,
  cells: TokenRecordHourCell[],
  summaryOverrides: Partial<TokenRecordRecentItem["summary"]> = {},
): TokenRecordRecentItem {
  return {
    model_name: modelName,
    cells,
    summary: {
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
      failed_rate: 0,
      failed_detail: {},
      ...summaryOverrides,
    },
  };
}

const labels = {
  thinking: "Thinking",
  output: "Output",
  tool: "Tool",
};

describe("formatTokenUnits", () => {
  it("formats decimal k/M/B units, trims trailing zeros, and promotes on cross-tier rounding", () => {
    expect(formatTokenUnits(0)).toBe("0");
    expect(formatTokenUnits(-10)).toBe("0");
    expect(formatTokenUnits(null)).toBe("0");
    expect(formatTokenUnits(Number.NaN)).toBe("0");
    expect(formatTokenUnits(999)).toBe("999");
    expect(formatTokenUnits(999.6)).toBe("1k");
    expect(formatTokenUnits(1000)).toBe("1k");
    expect(formatTokenUnits(100000)).toBe("100k");
    expect(formatTokenUnits(100500)).toBe("100.5k");
    expect(formatTokenUnits(999950)).toBe("1M");
    expect(formatTokenUnits(10450000)).toBe("10.45M");
    expect(formatTokenUnits(999995000)).toBe("1B");
    expect(formatTokenUnits(2345000000)).toBe("2.345B");
    expect(formatTokenUnits(1500000000000)).toBe("1500B");
  });
});

describe("model log TPS chart data and sorting", () => {
  it("keeps average TPS grouped by model and preserves null breakpoints instead of 0", () => {
    const items = [
      makeItem("model-a", [
        makeCell(1711454400, {
          completion_tokens: 40,
          avg_tps: 12.5,
          tps_sample_count: 2,
        }),
        makeCell(1711458000, {
          completion_tokens: 0,
          avg_tps: null,
          tps_sample_count: 0,
        }),
      ]),
      makeItem("model-b", [
        makeCell(1711454400, {
          completion_tokens: 20,
          avg_tps: 7.25,
          tps_sample_count: 1,
        }),
      ]),
    ];

    const result = transformChartData(items, new Set(["model-a", "model-b"]));

    expect(result.averageTpsData).toEqual([
      expect.objectContaining({
        Model: "model-a",
        Value: 12.5,
        SampleCount: 2,
      }),
      expect.objectContaining({
        Model: "model-a",
        Value: null,
        SampleCount: 0,
      }),
      expect.objectContaining({
        Model: "model-b",
        Value: 7.25,
        SampleCount: 1,
      }),
    ]);

    const avgSpec = buildLineChartSpec(
      "averageTpsData",
      result.averageTpsData,
      ["model-a", "model-b"],
      ["#0ea5e9", "#8b5cf6"],
      { valueFormat: "tps" },
    ) as Record<string, unknown>;
    expect(avgSpec.invalidType).toBe("break");

    expect(result.requestCountData).toEqual([
      expect.objectContaining({
        Model: "model-a",
        Value: 1,
        SampleCount: 1,
      }),
      expect.objectContaining({
        Model: "model-a",
        Value: 1,
        SampleCount: 1,
      }),
      expect.objectContaining({
        Model: "model-b",
        Value: 1,
        SampleCount: 1,
      }),
    ]);

    expect(result.trimmedTpsData).toEqual(result.averageTpsData);
  });

  it("formats missing summary TPS as N/A and sorts missing avg_tps last in both asc and desc", () => {
    expect(formatModelTpsSummary(null, 0)).toBe("N/A");
    expect(formatModelTpsSummary(18.4, 0)).toBe("N/A");
    expect(formatModelTpsSummary(18.456, 3)).toBe("18.46");

    const items = [
      makeItem("model-missing", [], { avg_tps: null, tps_sample_count: 0 }),
      makeItem("model-fast", [], { avg_tps: 45.2, tps_sample_count: 4 }),
      makeItem("model-slow", [], { avg_tps: 12.1, tps_sample_count: 2 }),
    ];

    expect(
      sortModelLogItems(items, "avg_tps", "desc").map((i) => i.model_name),
    ).toEqual(["model-fast", "model-slow", "model-missing"]);
    expect(
      sortModelLogItems(items, "avg_tps", "asc").map((i) => i.model_name),
    ).toEqual(["model-slow", "model-fast", "model-missing"]);
  });

  it("builds up to 3 phase series per model (6 series for 2 models) using backend verified phase TPS", () => {
    const items = [
      makeItem("model-a", [
        makeCell(1711454400, {
          avg_thinking_tps: 32.5,
          thinking_sample_count: 2,
          avg_output_tps: 48.2,
          output_sample_count: 2,
          avg_tool_tps: 19.8,
          tool_sample_count: 1,
        }),
        makeCell(1711458000, {
          // Unreliable raw tokens/duration with null verified TPS must stay a null breakpoint
          thinking_tokens: 50,
          thinking_duration_ms: 40,
          avg_thinking_tps: null,
          thinking_sample_count: 0,
          avg_output_tps: 50.0,
          output_sample_count: 1,
          avg_tool_tps: null,
          tool_sample_count: 0,
        }),
      ]),
      makeItem("model-b", [
        makeCell(1711454400, {
          avg_thinking_tps: 21.0,
          thinking_sample_count: 1,
          avg_output_tps: 35.4,
          output_sample_count: 3,
          avg_tool_tps: 14.2,
          tool_sample_count: 1,
        }),
      ]),
    ];

    const result = transformEstimatedTpsData(
      items,
      new Set(["model-a", "model-b"]),
      labels,
    );

    const uniqueSeries = new Set(result.map((point) => point.SeriesKey));
    expect(uniqueSeries.size).toBe(6);
    expect([...uniqueSeries]).toEqual([
      "model-a (Thinking)",
      "model-a (Output)",
      "model-a (Tool)",
      "model-b (Thinking)",
      "model-b (Output)",
      "model-b (Tool)",
    ]);

    // Hour 2 for model-a thinking has raw tokens/duration but null avg_thinking_tps -> breakpoint null
    const modelAThinkingPoints = result.filter(
      (p) => p.Model === "model-a" && p.Phase === "thinking",
    );
    expect(modelAThinkingPoints.map((p) => p.Value)).toEqual([32.5, null]);

    const colorMap = buildStableModelColorMap(["model-a", "model-b"]);
    const spec = buildEstimatedTpsChartSpec(
      "estimatedTpsData",
      result,
      colorMap,
    ) as unknown as {
      seriesField: string;
      color: { domain: string[]; range: string[] };
    };
    expect(spec.seriesField).toBe("SeriesKey");
    expect(spec.color.domain).toHaveLength(6);
    expect(spec.color.range).toHaveLength(6);
    expect(getPhaseLineDash("output")).toEqual([]);
    expect(getPhaseLineDash("thinking")).toEqual([6, 4]);
    expect(getPhaseLineDash("tool")).toEqual([2, 3]);
    expect(getPhaseColorVariant("#0ea5e9", "output")).toBe("#0ea5e9");
    expect(getPhaseColorVariant("#0ea5e9", "thinking")).not.toBe("#0ea5e9");
    expect(getPhaseColorVariant("#0ea5e9", "tool")).not.toBe("#0ea5e9");
  });

  it("keeps model colors stable across sorting, filtering, and new model additions", () => {
    const initialMap = buildStableModelColorMap([
      "gpt-4o",
      "claude-3-5-sonnet",
    ]);
    const gptColor = initialMap.get("gpt-4o");
    const claudeColor = initialMap.get("claude-3-5-sonnet");

    const reorderedMap = buildStableModelColorMap(
      ["claude-3-5-sonnet", "gemini-2.5-pro", "gpt-4o"],
      initialMap,
    );
    expect(reorderedMap.get("gpt-4o")).toBe(gptColor);
    expect(reorderedMap.get("claude-3-5-sonnet")).toBe(claudeColor);
    expect(reorderedMap.get("gemini-2.5-pro")).toBeDefined();

    const extendedPalette = getChartColors(8);
    expect(new Set(extendedPalette).size).toBe(8);
  });
});

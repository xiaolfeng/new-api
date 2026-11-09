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
import { fireEvent, render, screen, within } from "@testing-library/react";
import { createInstance } from "i18next";
import { I18nextProvider } from "react-i18next";
import { beforeEach, describe, expect, it } from "vitest";

import en from "@/i18n/locales/en.json";

import type { TokenPhase } from "../../lib/metrics";
import type { TokenRecordHourCell, TokenRecordRecentItem } from "../../types";
import { ModelRanking } from "../model-ranking";

const i18n = createInstance();

beforeEach(async () => {
  await i18n.init({
    lng: "en",
    resources: { en },
    interpolation: { escapeValue: false },
  });
});

const BUCKETS = [1711454400, 1711458000, 1711461600];

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

function makeItem(
  modelName: string,
  options: {
    requests: number;
    completionTokens: number;
    trimmedTps: number;
    ttftSeconds: number;
    phases?: Partial<Record<TokenPhase, { tps: number; tokens: number }>>;
  },
): TokenRecordRecentItem {
  const generationPerRequest =
    options.completionTokens / (options.requests * options.trimmedTps);
  const totalPerRequest = generationPerRequest + options.ttftSeconds;
  const totalUseTime = totalPerRequest * options.requests;

  const cells = BUCKETS.map((bucket) =>
    makeCell(bucket, {
      request_count: options.requests,
      completion_tokens: options.completionTokens,
      total_use_time: totalUseTime,
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
    }),
  );

  const requestCount = options.requests * BUCKETS.length;
  return {
    model_name: modelName,
    cells,
    summary: {
      request_count: requestCount,
      prompt_tokens: 0,
      completion_tokens: options.completionTokens * BUCKETS.length,
      total_tokens: options.completionTokens * BUCKETS.length,
      total_use_time: totalUseTime * BUCKETS.length,
      avg_tps: options.trimmedTps,
      tps_sample_count: requestCount,
      thinking_tokens: (options.phases?.thinking?.tokens ?? 0) * BUCKETS.length,
      thinking_duration_ms: 0,
      avg_thinking_tps: options.phases?.thinking?.tps ?? null,
      thinking_sample_count: options.phases?.thinking ? requestCount : 0,
      output_tokens: (options.phases?.output?.tokens ?? 0) * BUCKETS.length,
      output_duration_ms: 0,
      avg_output_tps: options.phases?.output?.tps ?? null,
      output_sample_count: options.phases?.output ? requestCount : 0,
      tool_tokens: (options.phases?.tool?.tokens ?? 0) * BUCKETS.length,
      tool_duration_ms: 0,
      avg_tool_tps: options.phases?.tool?.tps ?? null,
      tool_sample_count: options.phases?.tool ? requestCount : 0,
      failed_count: 0,
      failed_rate: 0,
      failed_detail: {},
    },
  };
}

function renderRanking(items: TokenRecordRecentItem[]) {
  return render(
    <I18nextProvider i18n={i18n}>
      <ModelRanking
        items={items}
        selectedModels={new Set(items.map((item) => item.model_name))}
        locale="en-US"
      />
    </I18nextProvider>,
  );
}

function modelRows(): HTMLElement[] {
  return screen.getAllByRole("listitem");
}

describe("ModelRanking", () => {
  it("renders the ranking board with rank, model, composite score and weights", async () => {
    renderRanking([
      makeItem("model-slow", {
        requests: 10,
        completionTokens: 500,
        trimmedTps: 50,
        ttftSeconds: 1,
      }),
      makeItem("model-fast", {
        requests: 10,
        completionTokens: 1000,
        trimmedTps: 100,
        ttftSeconds: 0.2,
      }),
    ]);

    const panel = await screen.findByRole("region", { name: "Model Ranking" });
    expect(within(panel).getByText("Weighted Average")).toBeVisible();
    expect(within(panel).getByText("Weight composition")).toBeVisible();

    const rows = modelRows();
    expect(rows).toHaveLength(2);
    expect(within(rows[0]).getByText("model-fast")).toBeVisible();
    expect(within(rows[0]).getByText("1")).toBeVisible();
    expect(within(rows[0]).getByText("96.2")).toBeVisible();
    expect(within(rows[1]).getByText("model-slow")).toBeVisible();
    expect(within(rows[1]).getByText("56.8")).toBeVisible();
  });

  it("explains the score bar with per-dimension weighted contributions", async () => {
    renderRanking([
      makeItem("model-fast", {
        requests: 10,
        completionTokens: 1000,
        trimmedTps: 100,
        ttftSeconds: 0.2,
      }),
      makeItem("model-slow", {
        requests: 10,
        completionTokens: 500,
        trimmedTps: 50,
        ttftSeconds: 1,
      }),
    ]);

    await screen.findByRole("region", { name: "Model Ranking" });

    const throughputSegment = screen.getByTitle(
      /^Throughput: 30 × 1\.00 = 30\.0$/,
    );
    expect(throughputSegment).toBeVisible();
    expect(
      screen.getByTitle(/^First Token: 25 × 1\.00 = 25\.0$/),
    ).toBeVisible();
  });

  it("shows the phase TPS with a per-phase breakdown tooltip", async () => {
    renderRanking([
      makeItem("model-fast", {
        requests: 10,
        completionTokens: 1000,
        trimmedTps: 100,
        ttftSeconds: 0.2,
        phases: {
          thinking: { tps: 60, tokens: 400 },
          output: { tps: 100, tokens: 600 },
        },
      }),
      makeItem("model-slow", {
        requests: 10,
        completionTokens: 500,
        trimmedTps: 50,
        ttftSeconds: 1,
      }),
    ]);

    const rows = modelRows();
    const phaseLine = within(rows[0]).getByTitle(
      "Thinking: 60 tok/s · 1,200 tokens · Output: 100 tok/s · 1,800 tokens",
    );
    expect(phaseLine).toBeVisible();
    expect(phaseLine).toHaveTextContent("Thinking 60");
  });

  it("re-sorts the board when a sort control is clicked", async () => {
    renderRanking([
      makeItem("model-slow", {
        requests: 10,
        completionTokens: 500,
        trimmedTps: 50,
        ttftSeconds: 1,
      }),
      makeItem("model-fast", {
        requests: 10,
        completionTokens: 1000,
        trimmedTps: 100,
        ttftSeconds: 0.2,
      }),
    ]);

    await screen.findByRole("region", { name: "Model Ranking" });
    const firstTokenSort = screen.getByRole("button", { name: "First Token" });

    fireEvent.click(firstTokenSort);
    let rows = modelRows();
    expect(within(rows[0]).getByText("model-fast")).toBeVisible();
    expect(within(rows[1]).getByText("model-slow")).toBeVisible();

    fireEvent.click(firstTokenSort);
    rows = modelRows();
    expect(within(rows[0]).getByText("model-slow")).toBeVisible();
    expect(within(rows[1]).getByText("model-fast")).toBeVisible();
  });

  it("shows a friendly empty state when no model data is available", async () => {
    renderRanking([]);

    expect(
      await screen.findByText(
        "No model ranking data in the selected time range.",
      ),
    ).toBeVisible();
    expect(screen.queryAllByRole("listitem")).toHaveLength(0);
  });
});

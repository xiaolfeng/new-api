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
import { calculateInfraEfficiency } from "@/lib/infra-efficiency";

/**
 * 可派生观测指标的最小字段集合：小时桶与区间汇总都满足该结构，
 * 保证「图表序列」与「模型排名」共用同一套口径。
 */
export interface TokenMetricSource {
  request_count: number;
  failed_count: number;
  completion_tokens: number;
  total_use_time: number;
  avg_tps: number | null;
  tps_sample_count?: number;
  thinking_tokens?: number;
  avg_thinking_tps?: number | null;
  thinking_sample_count?: number;
  output_tokens?: number;
  avg_output_tps?: number | null;
  output_sample_count?: number;
  tool_tokens?: number;
  avg_tool_tps?: number | null;
  tool_sample_count?: number;
}

/** 阶段（思考 / 输出 / 工具）字段映射：阶段 TPS 只采信后端校验过的样本。 */
const PHASE_SOURCES: {
  phase: TokenPhase;
  tokensField: "thinking_tokens" | "output_tokens" | "tool_tokens";
  avgField: "avg_thinking_tps" | "avg_output_tps" | "avg_tool_tps";
  sampleCountField:
    | "thinking_sample_count"
    | "output_sample_count"
    | "tool_sample_count";
}[] = [
  {
    phase: "thinking",
    tokensField: "thinking_tokens",
    avgField: "avg_thinking_tps",
    sampleCountField: "thinking_sample_count",
  },
  {
    phase: "output",
    tokensField: "output_tokens",
    avgField: "avg_output_tps",
    sampleCountField: "output_sample_count",
  },
  {
    phase: "tool",
    tokensField: "tool_tokens",
    avgField: "avg_tool_tps",
    sampleCountField: "tool_sample_count",
  },
];
export type TokenPhase = "thinking" | "output" | "tool";

export interface PhaseTpsEntry {
  phase: TokenPhase;
  /** 该阶段的输出吞吐，tok/s */
  tps: number;
  /** 该阶段产出的 token 数 */
  tokens: number;
}

export interface TokenMetrics {
  /** 成功请求数 */
  requests: number;
  /** 失败请求数 */
  failedRequests: number;
  /** 成功 + 失败请求数 */
  totalRequests: number;
  /** 全生命周期吞吐（含首字等待），tok/s */
  overallTps: number | null;
  /** 去除首字延迟的输出吞吐，tok/s */
  trimmedTps: number | null;
  /** 首字延迟，秒 */
  ttft: number | null;
  /** INFRA 吞吐保留率，0 - 100 */
  infraEfficiency: number | null;
  /** 失败率，0 - 100 */
  failureRate: number | null;
  /** 稳定性（1 - 失败率），0 - 1 */
  reliability: number | null;
  /** 使用量权重（对数刻度，避免头部流量线性放大） */
  scale: number;
  /** 阶段吞吐综合值（按阶段 token 量加权），tok/s */
  phaseTps: number | null;
  /** 各阶段吞吐明细 */
  phaseEntries: PhaseTpsEntry[];
}

export function hasVerifiedAvgTps(
  avgTps: number | null | undefined,
  sampleCount?: number,
): avgTps is number {
  if (typeof avgTps !== "number" || !Number.isFinite(avgTps) || avgTps <= 0) {
    return false;
  }
  if (sampleCount !== undefined && sampleCount <= 0) {
    return false;
  }
  return true;
}

/**
 * 汇总一次观测样本（小时桶或区间汇总）的派生指标：
 * 总吞吐、去首字吞吐、首字延迟、INFRA 保留率、失败率与稳定性。
 */
export function computeTokenMetrics(source: TokenMetricSource): TokenMetrics {
  const requests = source.request_count || 0;
  const failedRequests = source.failed_count || 0;
  const totalRequests = requests + failedRequests;

  const hasOverallTps =
    (source.total_use_time || 0) > 0 && (source.completion_tokens || 0) > 0;
  const overallTps = hasOverallTps
    ? source.completion_tokens / source.total_use_time
    : null;

  const trimmedTps = hasVerifiedAvgTps(source.avg_tps, source.tps_sample_count)
    ? source.avg_tps
    : null;

  let ttft: number | null = null;
  if (
    trimmedTps !== null &&
    requests > 0 &&
    (source.completion_tokens || 0) > 0 &&
    (source.total_use_time || 0) > 0
  ) {
    const totalPerRequest = source.total_use_time / requests;
    const generationPerRequest =
      source.completion_tokens / (requests * trimmedTps);
    ttft = Math.max(0, totalPerRequest - generationPerRequest);
  }

  const infraEfficiency = calculateInfraEfficiency(overallTps, trimmedTps);
  const failureRate =
    totalRequests > 0 ? (failedRequests / totalRequests) * 100 : null;

  const phaseEntries: PhaseTpsEntry[] = [];
  for (const phase of PHASE_SOURCES) {
    const tps = source[phase.avgField];
    const sampleCount = source[phase.sampleCountField];
    const tokens = source[phase.tokensField] ?? 0;
    if (!hasVerifiedAvgTps(tps, sampleCount) || tokens <= 0) {
      continue;
    }
    phaseEntries.push({ phase: phase.phase, tps, tokens });
  }
  const phaseTokens = phaseEntries.reduce((sum, entry) => sum + entry.tokens, 0);
  const phaseTps =
    phaseEntries.length > 0 && phaseTokens > 0
      ? phaseEntries.reduce((sum, entry) => sum + entry.tokens * entry.tps, 0) /
        phaseTokens
      : null;

  return {
    requests,
    failedRequests,
    totalRequests,
    overallTps,
    trimmedTps,
    ttft,
    infraEfficiency: infraEfficiency ? infraEfficiency.efficiencyRate : null,
    failureRate,
    reliability: failureRate === null ? null : 1 - failureRate / 100,
    scale: Math.log1p(totalRequests),
    phaseTps,
    phaseEntries,
  };
}

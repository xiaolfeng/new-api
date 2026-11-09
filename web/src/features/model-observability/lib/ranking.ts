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
import type { TokenRecordRecentItem } from "../types";
import {
  computeTokenMetrics,
  type PhaseTpsEntry,
  type TokenMetrics,
} from "./metrics";

/**
 * 排名维度权重（相对分值，合成时按参与维度归一）。
 * throughput 吞吐能力 / ttft 首字延迟 / infra 基础设施效率 /
 * reliability 稳定性 / scale 使用量 / phase 阶段吞吐。
 */
export const RANKING_WEIGHTS = {
  throughput: 30,
  ttft: 25,
  infra: 25,
  reliability: 20,
  scale: 10,
  phase: 10,
} as const;

/** 进入主榜所需的最少可比小时数（与其它模型同小时对比过的桶数） */
export const RANKING_MIN_COMPARABLE_BUCKETS = 3;
/** 进入主榜所需的最少请求量 */
export const RANKING_MIN_REQUESTS = 20;
/** 并列容差：分差小于该值视为同一名次 */
export const RANKING_TIE_TOLERANCE = 0.1;

export type RankingSufficiency = "high" | "medium" | "low" | "insufficient";

/** 排名维度键，与 RANKING_WEIGHTS 一一对应 */
export type RankingDimensionKey =
  | "throughput"
  | "ttft"
  | "infra"
  | "reliability"
  | "scale"
  | "phase";

/** 单个维度对综合分的贡献：权重 × 归一值 */
export interface RankingDimension {
  key: RankingDimensionKey;
  /** 维度权重 */
  weight: number;
  /** 跨小时加权平均后的归一值 0 - 1 */
  norm: number;
  /** 对综合分的贡献分值（weight × norm） */
  contribution: number;
}

export interface RankingEntry {
  modelName: string;
  /** 综合得分 0 - 100，无可比小时时为 null */
  score: number | null;
  /** 主榜名次，样本不足分组为 null */
  rank: number | null;
  /** 区间口径的去首字吞吐，tok/s */
  throughputTps: number | null;
  /** 区间口径的首字延迟，秒 */
  ttft: number | null;
  /** 区间口径的 INFRA 吞吐保留率，0 - 100 */
  infraEfficiency: number | null;
  /** 区间口径的稳定性，0 - 1 */
  reliability: number | null;
  /** 区间口径的阶段吞吐综合值，tok/s */
  phaseTps: number | null;
  /** 区间口径的各阶段吞吐明细 */
  phaseEntries: PhaseTpsEntry[];
  /** 区间口径的总请求数（成功 + 失败） */
  requests: number;
  /** 与其它模型同小时对比过的小时数 */
  comparableBuckets: number;
  /** 样本充分度 */
  sufficiency: RankingSufficiency;
  /** 各维度对综合分的贡献，用于解释「为什么排这么前」 */
  dimensions: RankingDimension[];
}

export interface RankingResult {
  /** 样本充分的主榜 */
  entries: RankingEntry[];
  /** 样本不足的模型 */
  insufficient: RankingEntry[];
}

interface BucketBaselines {
  maxTrimmedTps: number;
  minTtft: number;
  maxTtft: number;
  maxScale: number;
  /** 该桶内所有参与模型都有阶段吞吐时，阶段维度才可横向比较 */
  maxPhaseTps: number;
  phaseComparable: boolean;
}

interface BucketDimension {
  key: RankingDimensionKey;
  weight: number;
  normalized: number;
}

interface BucketScore {
  score: number;
  dimensions: BucketDimension[];
}

interface ScoreAccumulator {
  weightedScore: number;
  weight: number;
  comparableBuckets: number;
  /** 各维度归一值的加权累计与权重累计 */
  dimensionWeighted: Record<RankingDimensionKey, number>;
  dimensionWeight: Record<RankingDimensionKey, number>;
}

const DIMENSION_ORDER: RankingDimensionKey[] = [
  "throughput",
  "ttft",
  "infra",
  "reliability",
  "scale",
  "phase",
];

function createAccumulator(): ScoreAccumulator {
  const dimensionWeighted = {} as Record<RankingDimensionKey, number>;
  const dimensionWeight = {} as Record<RankingDimensionKey, number>;
  for (const key of DIMENSION_ORDER) {
    dimensionWeighted[key] = 0;
    dimensionWeight[key] = 0;
  }
  return {
    weightedScore: 0,
    weight: 0,
    comparableBuckets: 0,
    dimensionWeighted,
    dimensionWeight,
  };
}

function computeBaselines(observations: TokenMetrics[]): BucketBaselines {
  let maxTrimmedTps = 0;
  let minTtft = Number.POSITIVE_INFINITY;
  let maxTtft = 0;
  let maxScale = 0;
  let maxPhaseTps = 0;
  let phaseComparable = observations.length > 0;

  for (const metrics of observations) {
    if (metrics.trimmedTps !== null) {
      maxTrimmedTps = Math.max(maxTrimmedTps, metrics.trimmedTps);
    }
    if (metrics.ttft !== null && metrics.ttft > 0) {
      minTtft = Math.min(minTtft, metrics.ttft);
      maxTtft = Math.max(maxTtft, metrics.ttft);
    }
    if (metrics.phaseTps === null) {
      phaseComparable = false;
    } else {
      maxPhaseTps = Math.max(maxPhaseTps, metrics.phaseTps);
    }
    maxScale = Math.max(maxScale, metrics.scale);
  }

  return {
    maxTrimmedTps,
    minTtft: Number.isFinite(minTtft) ? minTtft : 0,
    maxTtft,
    maxScale,
    maxPhaseTps,
    phaseComparable,
  };
}

/**
 * 首字延迟归一化：默认用「同小时最优比值」，最优为 0 时退化为区间线性归一，
 * 避免最优延迟为 0 时整维度塌陷为 0 分。
 */
function normalizeLatency(ttft: number, baselines: BucketBaselines): number {
  if (baselines.minTtft > 0) {
    return ttft > 0 ? baselines.minTtft / ttft : 1;
  }
  const span = baselines.maxTtft - baselines.minTtft;
  if (span <= 0) {
    return 1;
  }
  return (baselines.maxTtft - ttft) / span;
}

/**
 * 单个小时桶内把派生指标折算成 0 - 1 的相对值再加权合成 0 - 100 分。
 * 阶段吞吐维度仅在该桶所有参与模型都有阶段数据时才参与，
 * 保证同一桶内各模型的评分维度集合一致。
 */
function composeBucketScore(
  metrics: TokenMetrics,
  baselines: BucketBaselines,
): BucketScore | null {
  const trimmedTps = metrics.trimmedTps;
  const ttft = metrics.ttft;
  if (trimmedTps === null || ttft === null) {
    return null;
  }

  const dimensions: BucketDimension[] = [
    {
      key: "throughput",
      weight: RANKING_WEIGHTS.throughput,
      normalized:
        baselines.maxTrimmedTps > 0 ? trimmedTps / baselines.maxTrimmedTps : 0,
    },
    {
      key: "ttft",
      weight: RANKING_WEIGHTS.ttft,
      normalized: normalizeLatency(ttft, baselines),
    },
    {
      key: "infra",
      weight: RANKING_WEIGHTS.infra,
      normalized:
        metrics.infraEfficiency === null ? 0 : metrics.infraEfficiency / 100,
    },
    {
      key: "reliability",
      weight: RANKING_WEIGHTS.reliability,
      normalized: metrics.reliability === null ? 0 : metrics.reliability,
    },
    {
      key: "scale",
      weight: RANKING_WEIGHTS.scale,
      normalized:
        baselines.maxScale > 0 ? metrics.scale / baselines.maxScale : 0,
    },
  ];

  if (
    baselines.phaseComparable &&
    baselines.maxPhaseTps > 0 &&
    metrics.phaseTps !== null
  ) {
    dimensions.push({
      key: "phase",
      weight: RANKING_WEIGHTS.phase,
      normalized: metrics.phaseTps / baselines.maxPhaseTps,
    });
  }

  const totalWeight = dimensions.reduce((sum, dim) => sum + dim.weight, 0);
  const weighted = dimensions.reduce(
    (sum, dim) => sum + dim.weight * dim.normalized,
    0,
  );
  return { score: (weighted / totalWeight) * 100, dimensions };
}

function resolveSufficiency(
  comparableBuckets: number,
  requests: number,
): RankingSufficiency {
  if (
    comparableBuckets < RANKING_MIN_COMPARABLE_BUCKETS ||
    requests < RANKING_MIN_REQUESTS
  ) {
    return "insufficient";
  }
  if (comparableBuckets >= 12 && requests >= 500) {
    return "high";
  }
  if (comparableBuckets >= 6 && requests >= 100) {
    return "medium";
  }
  return "low";
}

/**
 * 模型排名：同一小时桶内横向比出相对分，再按请求量做加权平均。
 * 只有与其它模型同小时对比过（可比小时）且样本量达标的模型才进入主榜。
 */
export function buildModelRanking(
  items: TokenRecordRecentItem[],
): RankingResult {
  const buckets = new Map<
    number,
    { modelName: string; metrics: TokenMetrics }[]
  >();

  for (const item of items) {
    for (const cell of item.cells ?? []) {
      const metrics = computeTokenMetrics(cell);
      if (metrics.trimmedTps === null || metrics.totalRequests <= 0) {
        continue;
      }
      const observation = { modelName: item.model_name, metrics };
      const bucket = buckets.get(cell.bucket_start_at);
      if (bucket) {
        bucket.push(observation);
      } else {
        buckets.set(cell.bucket_start_at, [observation]);
      }
    }
  }

  const accumulators = new Map<string, ScoreAccumulator>();
  for (const observations of buckets.values()) {
    if (observations.length < 2) {
      continue;
    }
    const baselines = computeBaselines(observations.map((o) => o.metrics));
    for (const observation of observations) {
      const bucketScore = composeBucketScore(observation.metrics, baselines);
      if (bucketScore === null) {
        continue;
      }
      const accumulator =
        accumulators.get(observation.modelName) ?? createAccumulator();
      accumulator.weightedScore += bucketScore.score * observation.metrics.scale;
      accumulator.weight += observation.metrics.scale;
      accumulator.comparableBuckets += 1;
      for (const dimension of bucketScore.dimensions) {
        accumulator.dimensionWeighted[dimension.key] +=
          dimension.normalized * observation.metrics.scale;
        accumulator.dimensionWeight[dimension.key] += observation.metrics.scale;
      }
      accumulators.set(observation.modelName, accumulator);
    }
  }

  const entries: RankingEntry[] = [];
  const insufficient: RankingEntry[] = [];

  for (const item of items) {
    const summary = computeTokenMetrics(item.summary);
    const accumulator = accumulators.get(item.model_name);
    const comparableBuckets = accumulator?.comparableBuckets ?? 0;
    const score =
      accumulator && accumulator.weight > 0
        ? accumulator.weightedScore / accumulator.weight
        : null;
    const sufficiency = resolveSufficiency(
      comparableBuckets,
      summary.totalRequests,
    );

    const dimensions: RankingDimension[] = [];
    for (const key of DIMENSION_ORDER) {
      const dimensionWeight = accumulator?.dimensionWeight[key] ?? 0;
      if (dimensionWeight <= 0) {
        continue;
      }
      const weight = RANKING_WEIGHTS[key];
      const norm =
        (accumulator?.dimensionWeighted[key] ?? 0) / dimensionWeight;
      dimensions.push({ key, weight, norm, contribution: weight * norm });
    }

    const entry: RankingEntry = {
      modelName: item.model_name,
      score,
      rank: null,
      throughputTps: summary.trimmedTps,
      ttft: summary.ttft,
      infraEfficiency: summary.infraEfficiency,
      phaseTps: summary.phaseTps,
      phaseEntries: summary.phaseEntries,
      reliability: summary.reliability,
      requests: summary.totalRequests,
      comparableBuckets,
      sufficiency,
      dimensions,
    };

    if (sufficiency === "insufficient") {
      insufficient.push(entry);
    } else {
      entries.push(entry);
    }
  }

  entries.sort((left, right) => {
    const scoreDelta = (right.score ?? 0) - (left.score ?? 0);
    if (Math.abs(scoreDelta) > 1e-9) {
      return scoreDelta;
    }
    return right.requests - left.requests;
  });

  let rank = 0;
  let previousScore: number | null = null;
  entries.forEach((entry, index) => {
    if (
      previousScore === null ||
      entry.score === null ||
      Math.abs(entry.score - previousScore) >= RANKING_TIE_TOLERANCE
    ) {
      rank = index + 1;
    }
    entry.rank = rank;
    previousScore = entry.score;
  });

  insufficient.sort((left, right) => {
    const scoreDelta = (right.score ?? 0) - (left.score ?? 0);
    if (Math.abs(scoreDelta) > 1e-9) {
      return scoreDelta;
    }
    return right.requests - left.requests;
  });

  return { entries, insufficient };
}

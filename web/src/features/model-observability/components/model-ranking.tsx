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
import { ArrowDown, ArrowUp, Trophy } from "lucide-react";
import { useMemo, useState } from "react";
import { useTranslation } from "react-i18next";

import { Badge } from "@/components/ui/badge";
import { formatCompactNumber, formatNumber } from "@/lib/format";

import type { TokenPhase } from "../lib/metrics";
import {
  buildModelRanking,
  RANKING_WEIGHTS,
  type RankingDimensionKey,
  type RankingEntry,
} from "../lib/ranking";
import type { TokenRecordRecentItem } from "../types";
import { MetricChartPanel } from "./metric-chart-panel";

type RankingSortKey = "score" | RankingDimensionKey | "requests";

interface ModelRankingProps {
  items: TokenRecordRecentItem[];
  selectedModels: Set<string>;
  modelColorMap?: ReadonlyMap<string, string>;
  locale?: string;
}

/** 维度展示元数据：颜色沿用编辑式配色，深浅主题各一套 */
const DIMENSION_META: Record<
  RankingDimensionKey,
  { labelKey: string; swatch: string }
> = {
  throughput: {
    labelKey: "Throughput",
    swatch: "bg-[oklch(0.58_0.19_255)] dark:bg-[oklch(0.72_0.15_255)]",
  },
  ttft: {
    labelKey: "First Token",
    swatch: "bg-[oklch(0.55_0.2_295)] dark:bg-[oklch(0.7_0.16_295)]",
  },
  infra: {
    labelKey: "INFRA",
    swatch: "bg-[oklch(0.6_0.15_215)] dark:bg-[oklch(0.76_0.12_215)]",
  },
  reliability: {
    labelKey: "Reliability",
    swatch: "bg-[oklch(0.58_0.14_155)] dark:bg-[oklch(0.74_0.13_155)]",
  },
  scale: {
    labelKey: "Usage Volume",
    swatch: "bg-[oklch(0.62_0.13_75)] dark:bg-[oklch(0.8_0.13_78)]",
  },
  phase: {
    labelKey: "Phase TPS",
    swatch: "bg-[oklch(0.6_0.19_25)] dark:bg-[oklch(0.72_0.16_25)]",
  },
};

const DIMENSION_ORDER: RankingDimensionKey[] = [
  "throughput",
  "ttft",
  "infra",
  "reliability",
  "scale",
  "phase",
];

const ASCENDING_BY_DEFAULT = new Set<RankingSortKey>(["ttft"]);

function entryValue(entry: RankingEntry, key: RankingSortKey): number | null {
  switch (key) {
    case "score":
      return entry.score;
    case "throughput":
      return entry.throughputTps;
    case "ttft":
      return entry.ttft;
    case "infra":
      return entry.infraEfficiency;
    case "reliability":
      return entry.reliability;
    case "scale":
      return entry.requests;
    case "phase":
      return entry.phaseTps;
    case "requests":
      return entry.requests;
  }
}

function sortEntries(
  entries: RankingEntry[],
  key: RankingSortKey,
  ascending: boolean,
): RankingEntry[] {
  return [...entries].sort((left, right) => {
    const leftValue = entryValue(left, key);
    const rightValue = entryValue(right, key);
    if (leftValue === null && rightValue === null) {
      return 0;
    }
    if (leftValue === null) {
      return 1;
    }
    if (rightValue === null) {
      return -1;
    }
    if (leftValue === rightValue) {
      return (left.rank ?? 0) - (right.rank ?? 0);
    }
    return ascending ? leftValue - rightValue : rightValue - leftValue;
  });
}

export function ModelRanking(props: ModelRankingProps) {
  const { t } = useTranslation();
  const [sortKey, setSortKey] = useState<RankingSortKey>("score");
  const [ascending, setAscending] = useState(false);

  const ranking = useMemo(() => buildModelRanking(props.items), [props.items]);

  const { entries, insufficient } = useMemo(() => {
    const visible = (entry: RankingEntry) =>
      props.selectedModels.has(entry.modelName);
    return {
      entries: sortEntries(ranking.entries.filter(visible), sortKey, ascending),
      insufficient: ranking.insufficient.filter(visible),
    };
  }, [ranking, props.selectedModels, sortKey, ascending]);

  const handleSort = (key: RankingSortKey) => {
    if (key === sortKey) {
      setAscending((prev) => !prev);
      return;
    }
    setSortKey(key);
    setAscending(ASCENDING_BY_DEFAULT.has(key));
  };

  const sortOptions: { key: RankingSortKey; label: string }[] = [
    { key: "score", label: t("Composite Score") },
    ...DIMENSION_ORDER.map((key) => ({
      key: key as RankingSortKey,
      label: t(DIMENSION_META[key].labelKey),
    })),
  ];

  const renderSufficiency = (entry: RankingEntry) => {
    switch (entry.sufficiency) {
      case "high":
        return (
          <Badge
            variant="outline"
            className="border-emerald-500/30 bg-emerald-500/10 text-emerald-700 dark:text-emerald-300"
          >
            {t("Rich samples")}
          </Badge>
        );
      case "medium":
        return (
          <Badge
            variant="outline"
            className="border-sky-500/30 bg-sky-500/10 text-sky-700 dark:text-sky-300"
          >
            {t("Adequate samples")}
          </Badge>
        );
      case "low":
        return (
          <Badge
            variant="outline"
            className="border-amber-500/30 bg-amber-500/10 text-amber-700 dark:text-amber-300"
          >
            {t("Thin samples")}
          </Badge>
        );
      default:
        return (
          <Badge variant="outline" className="bg-muted/60">
            {t("Insufficient")}
          </Badge>
        );
    }
  };

  const phaseLabels: Record<TokenPhase, string> = {
    thinking: t("Thinking"),
    output: t("Output"),
    tool: t("Tool"),
  };

  const phaseLabel = (phase: TokenPhase) => phaseLabels[phase];

  const phaseBreakdownTitle = (entry: RankingEntry) => {
    if (entry.phaseEntries.length === 0) {
      return entry.modelName;
    }
    return entry.phaseEntries
      .map(
        (phase) =>
          `${phaseLabels[phase.phase]}: ${formatNumber(phase.tps, props.locale)} tok/s · ${formatNumber(phase.tokens, props.locale)} tokens`,
      )
      .join(" · ");
  };

  const renderDimensionLine = (entry: RankingEntry) => {
    const parts = [
      `${t("Throughput")} ${formatNumber(entry.throughputTps, props.locale)} tok/s`,
      entry.ttft === null
        ? null
        : `${t("First Token")} ${formatNumber(entry.ttft, props.locale)} s`,
      entry.infraEfficiency === null
        ? null
        : `${t("INFRA")} ${formatNumber(entry.infraEfficiency, props.locale)}%`,
      entry.reliability === null
        ? null
        : `${t("Reliability")} ${formatNumber(entry.reliability * 100, props.locale)}%`,
      entry.phaseTps === null
        ? null
        : `${t("Phase TPS")} ${formatNumber(entry.phaseTps, props.locale)}`,
      `${t("Requests")} ${formatCompactNumber(entry.requests, props.locale)}`,
    ].filter((part): part is string => part !== null);
    return parts.join(" · ");
  };

  const renderScoreBar = (entry: RankingEntry) => {
    if (entry.score === null || entry.dimensions.length === 0) {
      return (
        <span className="text-muted-foreground block h-2 rounded-full bg-muted" />
      );
    }
    const total = entry.dimensions.reduce(
      (sum, dimension) => sum + dimension.contribution,
      0,
    );
    if (total <= 0) {
      return (
        <span className="text-muted-foreground block h-2 rounded-full bg-muted" />
      );
    }
    return (
      <span className="bg-muted block h-2 w-full overflow-hidden rounded-full">
        <span
          className="flex h-full"
          style={{
            width: `${Math.max(0, Math.min(100, entry.score))}%`,
          }}
        >
          {entry.dimensions.map((dimension) => (
            <span
              key={dimension.key}
              className={`block h-full ${DIMENSION_META[dimension.key].swatch}`}
              style={{
                width: `${(dimension.contribution / total) * 100}%`,
              }}
              title={`${t(DIMENSION_META[dimension.key].labelKey)}: ${dimension.weight} × ${dimension.norm.toFixed(2)} = ${dimension.contribution.toFixed(1)}`}
            />
          ))}
        </span>
      </span>
    );
  };

  const renderRow = (entry: RankingEntry, showRank: boolean) => {
    const color = props.modelColorMap?.get(entry.modelName);
    return (
      <div
        key={entry.modelName}
        role="listitem"
        className="grid grid-cols-[2.25rem_minmax(0,1fr)_minmax(0,1.1fr)_3.75rem] items-center gap-4 border-b py-2.5 last:border-b-0"
      >
        <span className="text-foreground text-xl font-semibold tabular-nums">
          {showRank && entry.rank !== null ? entry.rank : "—"}
        </span>
        <span className="min-w-0">
          <span className="flex items-center gap-2">
            {color && (
              <span
                className="size-2 shrink-0 rounded-full"
                style={{ backgroundColor: color }}
                aria-hidden="true"
              />
            )}
            <span className="truncate font-medium">{entry.modelName}</span>
            {entry.sufficiency !== "insufficient" && renderSufficiency(entry)}
          </span>
          <span className="text-muted-foreground mt-0.5 block truncate text-xs tabular-nums">
            {renderDimensionLine(entry)}
          </span>
        </span>
        <span className="min-w-0">
          {renderScoreBar(entry)}
          {entry.phaseTps !== null && (
            <span
              className="text-muted-foreground mt-1 block truncate text-xs"
              title={phaseBreakdownTitle(entry)}
            >
              {entry.phaseEntries
                .map(
                  (phase) =>
                    `${phaseLabel(phase.phase)} ${formatNumber(phase.tps, props.locale)}`,
                )
                .join(" · ")}
            </span>
          )}
        </span>
        <span className="text-right">
          <span className="block text-lg font-semibold tabular-nums">
            {entry.score === null ? "—" : entry.score.toFixed(1)}
          </span>
          <span className="text-muted-foreground block text-[0.65rem] tracking-wide uppercase">
            {t("Composite Score")}
          </span>
        </span>
      </div>
    );
  };

  const hasRows = entries.length > 0 || insufficient.length > 0;

  return (
    <MetricChartPanel
      title={t("Model Ranking")}
      description={t(
        "Models are scored against their peers in each hour, then averaged by request volume; only models with comparable hours and enough samples enter the main board.",
      )}
      icon={<Trophy className="size-4" aria-hidden="true" />}
      badge={t("Weighted Average")}
      bodyClassName="p-0 sm:p-0"
      emptyMessage={
        hasRows
          ? undefined
          : t("No model ranking data in the selected time range.")
      }
    >
      <div className="border-b px-4 py-2">
        <div className="flex flex-wrap items-center gap-x-1 gap-y-1">
          <span className="text-muted-foreground mr-1 text-[0.65rem] tracking-wider uppercase">
            {t("Sort by")}
          </span>
          {sortOptions.map((option) => {
            const active = option.key === sortKey;
            return (
              <button
                key={option.key}
                type="button"
                aria-label={option.label}
                onClick={() => handleSort(option.key)}
                className={`hover:text-foreground inline-flex items-center gap-1 rounded-md px-2 py-1 text-xs transition-colors ${
                  active
                    ? "bg-muted text-foreground font-medium"
                    : "text-muted-foreground"
                }`}
              >
                {option.label}
                {active &&
                  (ascending ? (
                    <ArrowUp className="size-3" aria-hidden="true" />
                  ) : (
                    <ArrowDown className="size-3" aria-hidden="true" />
                  ))}
              </button>
            );
          })}
        </div>
      </div>

      <div role="list" className="px-4">
        {entries.map((entry) => renderRow(entry, true))}

        {insufficient.length > 0 && (
          <div className="text-muted-foreground bg-muted/40 mt-2 px-2 py-1 text-xs font-medium">
            {t("Insufficient Samples")}
          </div>
        )}
        {insufficient.map((entry) => (
          <div key={entry.modelName} className="opacity-70">
            {renderRow(entry, false)}
          </div>
        ))}
      </div>

      <div className="border-t px-4 py-3">
        <div className="text-muted-foreground flex flex-wrap items-center gap-x-4 gap-y-1 text-xs">
          <span className="tracking-wider uppercase">
            {t("Weight composition")}
          </span>
          {DIMENSION_ORDER.map((key) => (
            <span key={key} className="inline-flex items-center gap-1.5">
              <span
                className={`size-2 rounded-[2px] ${DIMENSION_META[key].swatch}`}
                aria-hidden="true"
              />
              {t(DIMENSION_META[key].labelKey)}
              <span className="tabular-nums">{RANKING_WEIGHTS[key]}</span>
            </span>
          ))}
        </div>
        <p className="text-muted-foreground mt-2 text-xs leading-relaxed">
          {t(
            "The bar length is the composite score, and each segment is one dimension's weighted contribution, so the ranking shows why a model ranks where it does.",
          )}
        </p>
      </div>
    </MetricChartPanel>
  );
}

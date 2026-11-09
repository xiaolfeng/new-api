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
import { ArrowDown, ArrowUp, ChevronsUpDown, Trophy } from "lucide-react";
import { useMemo, useState } from "react";
import { useTranslation } from "react-i18next";

import { Badge } from "@/components/ui/badge";
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from "@/components/ui/table";
import { formatCompactNumber, formatNumber } from "@/lib/format";

import { buildModelRanking, type RankingEntry } from "../lib/ranking";
import type { TokenRecordRecentItem } from "../types";
import { MetricChartPanel } from "./metric-chart-panel";

type RankingSortKey =
  "score" | "throughput" | "ttft" | "infra" | "reliability" | "requests";

interface ModelRankingProps {
  items: TokenRecordRecentItem[];
  selectedModels: Set<string>;
  modelColorMap?: ReadonlyMap<string, string>;
  locale?: string;
}

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

  const renderSortIcon = (key: RankingSortKey) => {
    if (key !== sortKey) {
      return (
        <ChevronsUpDown className="size-3 opacity-50" aria-hidden="true" />
      );
    }
    return ascending ? (
      <ArrowUp className="size-3" aria-hidden="true" />
    ) : (
      <ArrowDown className="size-3" aria-hidden="true" />
    );
  };

  const renderHeader = (
    key: RankingSortKey,
    label: string,
    className?: string,
  ) => (
    <TableHead className={className}>
      <button
        type="button"
        aria-label={label}
        onClick={() => handleSort(key)}
        className="hover:text-foreground inline-flex items-center gap-1 text-xs font-medium"
      >
        {label}
        {renderSortIcon(key)}
      </button>
    </TableHead>
  );

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

  const renderRow = (entry: RankingEntry, showRank: boolean) => {
    const color = props.modelColorMap?.get(entry.modelName);
    const score = entry.score;
    return (
      <TableRow key={entry.modelName}>
        <TableCell className="text-muted-foreground tabular-nums">
          {showRank && entry.rank !== null ? entry.rank : "—"}
        </TableCell>
        <TableCell className="max-w-[220px]">
          <span className="flex items-center gap-2">
            {color && (
              <span
                className="size-2 shrink-0 rounded-full"
                style={{ backgroundColor: color }}
                aria-hidden="true"
              />
            )}
            <span className="truncate">{entry.modelName}</span>
          </span>
        </TableCell>
        <TableCell className="text-right">
          {score === null ? (
            <span className="text-muted-foreground">—</span>
          ) : (
            <span className="flex items-center justify-end gap-2">
              <span className="bg-muted h-1.5 w-16 overflow-hidden rounded-full">
                <span
                  className="bg-primary block h-full rounded-full"
                  style={{ width: `${Math.max(0, Math.min(100, score))}%` }}
                />
              </span>
              <span className="w-10 text-right tabular-nums">
                {score.toFixed(1)}
              </span>
            </span>
          )}
        </TableCell>
        <TableCell className="text-right tabular-nums">
          {entry.throughputTps === null
            ? "—"
            : formatNumber(entry.throughputTps, props.locale)}
        </TableCell>
        <TableCell className="text-right tabular-nums">
          {entry.ttft === null
            ? "—"
            : `${formatNumber(entry.ttft, props.locale)} s`}
        </TableCell>
        <TableCell className="text-right tabular-nums">
          {entry.infraEfficiency === null
            ? "—"
            : `${formatNumber(entry.infraEfficiency, props.locale)}%`}
        </TableCell>
        <TableCell className="text-right tabular-nums">
          {entry.reliability === null
            ? "—"
            : `${formatNumber(entry.reliability * 100, props.locale)}%`}
        </TableCell>
        <TableCell className="text-right tabular-nums">
          {formatCompactNumber(entry.requests, props.locale)}
        </TableCell>
        <TableCell>{renderSufficiency(entry)}</TableCell>
      </TableRow>
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
      bodyClassName="max-h-[420px] overflow-auto p-0 sm:p-0"
      emptyMessage={
        hasRows
          ? undefined
          : t("No model ranking data in the selected time range.")
      }
    >
      <Table>
        <TableHeader>
          <TableRow>
            <TableHead className="w-12">{t("Rank")}</TableHead>
            <TableHead>{t("Model")}</TableHead>
            {renderHeader("score", t("Composite Score"), "text-right")}
            {renderHeader("throughput", t("Throughput"), "text-right")}
            {renderHeader("ttft", t("First Token"), "text-right")}
            {renderHeader("infra", t("INFRA"), "text-right")}
            {renderHeader("reliability", t("Reliability"), "text-right")}
            {renderHeader("requests", t("Requests"), "text-right")}
            <TableHead>{t("Data Sufficiency")}</TableHead>
          </TableRow>
        </TableHeader>
        <TableBody>
          {entries.map((entry) => renderRow(entry, true))}
          {insufficient.length > 0 && (
            <TableRow className="hover:bg-transparent">
              <TableCell
                colSpan={9}
                className="text-muted-foreground bg-muted/40 text-xs font-medium"
              >
                {t("Insufficient Samples")}
              </TableCell>
            </TableRow>
          )}
          {insufficient.map((entry) => renderRow(entry, false))}
        </TableBody>
      </Table>
    </MetricChartPanel>
  );
}

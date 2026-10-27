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
import { useTranslation } from "react-i18next";

import { cn } from "@/lib/utils";

import type { InfraEfficiencyResult } from "../lib/timing";

const efficiencyBadgeColorMap: Record<InfraEfficiencyResult["level"], string> =
  {
    excellent: "text-emerald-600 dark:text-emerald-400 bg-emerald-500/10",
    good: "text-lime-600 dark:text-lime-400 bg-lime-500/10",
    fair: "text-amber-600 dark:text-amber-400 bg-amber-500/10",
    poor: "text-red-600 dark:text-red-400 bg-red-500/10",
  };

interface TpsInfraTooltipContentProps {
  overallTps: number | null | undefined;
  trimmedTps: number | null | undefined;
  efficiency: InfraEfficiencyResult | null;
  firstLabel?: string | null;
  totalLabel?: string | null;
}

export function TpsInfraTooltipContent(props: TpsInfraTooltipContentProps) {
  const { t } = useTranslation();
  const { overallTps, trimmedTps, efficiency, firstLabel, totalLabel } = props;

  if (overallTps == null && trimmedTps == null) {
    return (
      <div className="p-2 text-xs text-muted-foreground">
        {t("No TPS or timing data available for this request.")}
      </div>
    );
  }

  return (
    <div className="max-w-[240px] space-y-1.5 p-2 text-xs">
      <div className="flex items-center justify-between gap-2 border-b pb-1 font-semibold">
        <span className="text-foreground">{t("INFRA Efficiency")}</span>
        {efficiency != null ? (
          <span
            className={cn(
              "rounded px-1.5 py-0.5 font-mono text-[11px] font-bold tabular-nums",
              efficiencyBadgeColorMap[efficiency.level],
            )}
          >
            {efficiency.efficiencyRate.toFixed(1)}%
          </span>
        ) : (
          <span className="text-muted-foreground font-mono text-[11px]">~</span>
        )}
      </div>

      <div className="space-y-1 pt-0.5">
        <div className="flex items-center justify-between gap-3">
          <span className="text-muted-foreground">
            {t("Average TPS (incl. TTFT)")}
          </span>
          <span className="font-mono tabular-nums text-foreground">
            {overallTps != null ? `${overallTps.toFixed(1)} t/s` : "~"}
          </span>
        </div>

        <div className="flex items-center justify-between gap-3">
          <span className="text-muted-foreground">
            {t("Generation TPS (excl. TTFT)")}
          </span>
          <span className="font-mono tabular-nums text-foreground">
            {trimmedTps != null ? `${trimmedTps.toFixed(1)} t/s` : "~"}
          </span>
        </div>

        {efficiency != null && efficiency.tpsDrop > 0 && (
          <div className="flex items-center justify-between gap-3 border-t border-dashed pt-1 text-[11px]">
            <span className="text-muted-foreground">{t("TTFT Penalty")}</span>
            <span className="font-mono tabular-nums text-red-600 dark:text-red-400">
              -{efficiency.tpsDrop.toFixed(1)} t/s (
              {efficiency.tpsDropRate.toFixed(1)}%)
            </span>
          </div>
        )}

        {firstLabel && totalLabel && (
          <div className="flex items-center justify-between gap-3 border-t pt-1 text-[11px]">
            <span className="text-muted-foreground">
              {t("First token")} / {t("Duration")}
            </span>
            <span className="font-mono tabular-nums text-muted-foreground">
              {firstLabel} / {totalLabel}
            </span>
          </div>
        )}
      </div>

      <p className="border-t pt-1 text-[10px] leading-tight text-muted-foreground/80">
        {efficiency != null
          ? t(
              "Throughput ratio reflecting infrastructure and TTFT responsiveness",
            )
          : t(
              "Non-stream or single-phase request, INFRA efficiency is not applicable.",
            )}
      </p>
    </div>
  );
}

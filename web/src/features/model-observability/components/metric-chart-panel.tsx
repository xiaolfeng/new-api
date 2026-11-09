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
import { type ReactNode, useId } from "react";

import { Badge } from "@/components/ui/badge";
import {
  Empty,
  EmptyDescription,
  EmptyHeader,
  EmptyMedia,
} from "@/components/ui/empty";
import { cn } from "@/lib/utils";

interface MetricChartPanelProps {
  title: string;
  description: string;
  icon: ReactNode;
  badge?: string;
  badgeClassName?: string;
  emptyMessage?: string;
  bodyClassName?: string;
  children: ReactNode;
}

/** 观测指标面板：统一标题、说明、徽标与空态外壳。 */
export function MetricChartPanel(props: MetricChartPanelProps) {
  const titleId = useId();

  return (
    <section
      aria-labelledby={titleId}
      className="bg-card/70 overflow-hidden rounded-xl border shadow-xs"
    >
      <div className="border-border/60 flex flex-col gap-2 border-b px-4 py-3 sm:flex-row sm:items-start sm:justify-between sm:px-5">
        <div className="flex min-w-0 items-start gap-2.5">
          <span className="bg-muted text-muted-foreground mt-0.5 flex size-7 shrink-0 items-center justify-center rounded-lg">
            {props.icon}
          </span>
          <div className="min-w-0">
            <h3 id={titleId} className="text-sm font-semibold">
              {props.title}
            </h3>
            <p className="text-muted-foreground mt-0.5 max-w-3xl text-xs leading-relaxed">
              {props.description}
            </p>
          </div>
        </div>
        {props.badge && (
          <Badge
            variant="outline"
            className={cn("w-fit shrink-0", props.badgeClassName)}
          >
            {props.badge}
          </Badge>
        )}
      </div>
      <div
        className={cn(
          "p-1.5 sm:p-2",
          props.bodyClassName ?? "h-[280px] sm:h-96",
        )}
      >
        {props.emptyMessage ? (
          <Empty className="h-full">
            <EmptyHeader>
              <EmptyMedia variant="icon">{props.icon}</EmptyMedia>
              <EmptyDescription>{props.emptyMessage}</EmptyDescription>
            </EmptyHeader>
          </Empty>
        ) : (
          props.children
        )}
      </div>
    </section>
  );
}

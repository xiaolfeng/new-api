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
import { render, screen } from "@testing-library/react";
import { describe, expect, it } from "vitest";

import { calculateInfraEfficiency } from "../../lib/timing";
import { TpsInfraTooltipContent } from "../tps-infra-tooltip";

describe("TpsInfraTooltipContent", () => {
  it("renders empty state message when both TPS are null", () => {
    render(
      <TpsInfraTooltipContent
        overallTps={null}
        trimmedTps={null}
        efficiency={null}
      />,
    );
    expect(
      screen.getByText("No TPS or timing data available for this request."),
    ).toBeVisible();
  });

  it("renders non-stream message when trimmed TPS is missing", () => {
    render(
      <TpsInfraTooltipContent
        overallTps={45.2}
        trimmedTps={null}
        efficiency={null}
      />,
    );
    expect(screen.getByText("45.2 t/s")).toBeVisible();
    expect(
      screen.getByText(
        "Non-stream or single-phase request, INFRA efficiency is not applicable.",
      ),
    ).toBeVisible();
  });

  it("renders complete INFRA efficiency breakdown when dual TPS are present", () => {
    const efficiency = calculateInfraEfficiency(50, 75);
    render(
      <TpsInfraTooltipContent
        overallTps={50}
        trimmedTps={75}
        efficiency={efficiency}
        firstLabel="0.333s"
        totalLabel="1.000s"
      />,
    );
    expect(screen.getByText("INFRA Efficiency")).toBeVisible();
    expect(screen.getByText("66.7%")).toBeVisible();
    expect(screen.getByText("50.0 t/s")).toBeVisible();
    expect(screen.getByText("75.0 t/s")).toBeVisible();
    expect(screen.getByText("-25.0 t/s (33.3%)")).toBeVisible();
    expect(screen.getByText("0.333s / 1.000s")).toBeVisible();
  });
});

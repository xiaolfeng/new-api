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

export interface InfraEfficiencyResult {
  /** 吞吐保留率 / 效率比率 (0 - 100 范围) */
  efficiencyRate: number;
  /** 首字延迟带来的吞吐降低值 (t/s) */
  tpsDrop: number;
  /** 首字吞吐损耗比例 (0 - 100 范围) */
  tpsDropRate: number;
  /** 效率评级 */
  level: "excellent" | "good" | "fair" | "poor";
}

/**
 * INFRA 效率：全生命周期吞吐（含首字等待）相对纯输出吞吐（去除首字等待）的保留率。
 *
 * overallTps 为包含首字延迟的总吞吐，trimmedTps 为去除首字延迟后的输出吞吐；
 * 保留率越接近 100%，说明首字等待被摊薄得越充分，基础设施能力越强。
 */
export function calculateInfraEfficiency(
  overallTps: number | null | undefined,
  trimmedTps: number | null | undefined,
): InfraEfficiencyResult | null {
  if (
    overallTps == null ||
    trimmedTps == null ||
    !Number.isFinite(overallTps) ||
    !Number.isFinite(trimmedTps) ||
    trimmedTps <= 0 ||
    overallTps <= 0
  ) {
    return null;
  }

  // overallTps 代表包含 TTFT 在内的全生命周期吞吐；trimmedTps 代表去除 TTFT 的纯输出吞吐。
  // 理论上 overallTps <= trimmedTps，做 clamp 保护微小抖动误差
  const safeOverall = Math.min(overallTps, trimmedTps);
  const efficiencyRate = Math.min(
    100,
    Math.max(0, (safeOverall / trimmedTps) * 100),
  );
  const tpsDrop = Math.max(0, trimmedTps - safeOverall);
  const tpsDropRate = Math.min(100, Math.max(0, (tpsDrop / trimmedTps) * 100));

  let level: "excellent" | "good" | "fair" | "poor" = "poor";
  if (efficiencyRate >= 80) {
    level = "excellent";
  } else if (efficiencyRate >= 60) {
    level = "good";
  } else if (efficiencyRate >= 40) {
    level = "fair";
  }

  return {
    efficiencyRate,
    tpsDrop,
    tpsDropRate,
    level,
  };
}

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
const THEME_CHART_COLOR_VARIABLES = [
  '--chart-1',
  '--chart-2',
  '--chart-3',
  '--chart-4',
  '--chart-5',
] as const

function getThemeChartColors(): string[] {
  if (typeof document === 'undefined') return []
  const bodyStyle = window.getComputedStyle(document.body)
  const rootStyle = window.getComputedStyle(document.documentElement)
  return THEME_CHART_COLOR_VARIABLES.map((name) =>
    (
      bodyStyle.getPropertyValue(name) || rootStyle.getPropertyValue(name)
    ).trim()
  ).filter(Boolean)
}

export function getChartColors(count: number): string[] {
  const themeColors = getThemeChartColors()
  if (themeColors.length > 0) {
    return Array.from(
      { length: count },
      (_, index) => themeColors[index % themeColors.length]
    )
  }
  return Array.from({ length: count }, (_, index) => {
    const hue = (index * 360) / count
    return `hsl(${hue}, 65%, 55%)`
  })
}

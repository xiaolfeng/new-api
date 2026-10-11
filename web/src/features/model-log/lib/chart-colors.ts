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
import type { EstimatedTpsPhase } from '../types'

const THEME_CHART_COLOR_VARIABLES = [
  '--chart-1',
  '--chart-2',
  '--chart-3',
  '--chart-4',
  '--chart-5',
] as const

const FALLBACK_PALETTE = [
  '#0ea5e9',
  '#8b5cf6',
  '#10b981',
  '#f59e0b',
  '#ec4899',
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

function getExtendedHueColor(extraIndex: number): string {
  const hue = Math.round((extraIndex * 137.508 + 25) % 360)
  return `hsl(${hue}, 68%, 52%)`
}

export function getChartColors(count: number): string[] {
  if (count <= 0) return []
  const themeColors = getThemeChartColors()
  const basePalette =
    themeColors.length > 0 ? themeColors : [...FALLBACK_PALETTE]
  return Array.from({ length: count }, (_, index) => {
    if (index < basePalette.length) {
      return basePalette[index]
    }
    return getExtendedHueColor(index - basePalette.length)
  })
}

export function buildStableModelColorMap(
  modelNames: string[],
  previousMap?: ReadonlyMap<string, string>
): Map<string, string> {
  const nextMap = new Map<string, string>()
  if (previousMap) {
    for (const [model, color] of previousMap.entries()) {
      nextMap.set(model, color)
    }
  }

  const unseenModels = [...new Set(modelNames)]
    .filter((name) => !nextMap.has(name))
    .sort((a, b) => a.localeCompare(b))

  if (unseenModels.length === 0) {
    return nextMap
  }

  const totalNeeded = nextMap.size + unseenModels.length
  const palette = getChartColors(totalNeeded)
  const usedColors = new Set(nextMap.values())
  let paletteCursor = 0

  for (const model of unseenModels) {
    while (
      paletteCursor < palette.length &&
      usedColors.has(palette[paletteCursor])
    ) {
      paletteCursor++
    }
    const color =
      palette[paletteCursor] ??
      getExtendedHueColor(paletteCursor + nextMap.size)
    nextMap.set(model, color)
    usedColors.add(color)
    paletteCursor++
  }

  return nextMap
}

function clamp(val: number, minVal: number, maxVal: number): number {
  return Math.min(maxVal, Math.max(minVal, val))
}

export function getPhaseColorVariant(
  baseColor: string,
  phase: EstimatedTpsPhase
): string {
  if (phase === 'output') {
    return baseColor
  }

  const oklchMatch = baseColor.match(
    /^oklch\(\s*([\d.]+)\s+([\d.]+)\s+([\d.]+)\s*\)$/i
  )
  if (oklchMatch) {
    const l = Number(oklchMatch[1])
    const c = Number(oklchMatch[2])
    const h = Number(oklchMatch[3])
    const nextL =
      phase === 'thinking'
        ? clamp(l + 0.09, 0.25, 0.88)
        : clamp(l - 0.09, 0.22, 0.85)
    const nextH = phase === 'thinking' ? (h + 10) % 360 : (h + 350) % 360
    return `oklch(${nextL.toFixed(4)} ${c} ${nextH.toFixed(2)})`
  }

  const hslMatch = baseColor.match(
    /^hsl\(\s*([\d.]+)\s*,\s*([\d.]+)%\s*,\s*([\d.]+)%\s*\)$/i
  )
  if (hslMatch) {
    const h = Number(hslMatch[1])
    const s = Number(hslMatch[2])
    const l = Number(hslMatch[3])
    const nextL =
      phase === 'thinking' ? clamp(l + 10, 25, 82) : clamp(l - 10, 20, 78)
    const nextH = phase === 'thinking' ? (h + 10) % 360 : (h + 350) % 360
    return `hsl(${Math.round(nextH)}, ${Math.round(s)}%, ${Math.round(nextL)}%)`
  }

  const hexMatch = baseColor.match(/^#([0-9a-f]{6})$/i)
  if (hexMatch) {
    const raw = hexMatch[1]
    const r = Number.parseInt(raw.slice(0, 2), 16)
    const g = Number.parseInt(raw.slice(2, 4), 16)
    const b = Number.parseInt(raw.slice(4, 6), 16)
    const adjust = (channel: number) =>
      phase === 'thinking'
        ? Math.round(channel + (255 - channel) * 0.25)
        : Math.round(channel * 0.78)
    const toHex = (n: number) => clamp(n, 0, 255).toString(16).padStart(2, '0')
    return `#${toHex(adjust(r))}${toHex(adjust(g))}${toHex(adjust(b))}`
  }

  return baseColor
}

export function getPhaseLineDash(phase: EstimatedTpsPhase): number[] {
  switch (phase) {
    case 'output':
      return []
    case 'thinking':
      return [6, 4]
    case 'tool':
      return [2, 3]
  }
}

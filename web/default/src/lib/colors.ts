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
export type SemanticColor =
  | 'blue'
  | 'green'
  | 'cyan'
  | 'purple'
  | 'pink'
  | 'red'
  | 'orange'
  | 'amber'
  | 'yellow'
  | 'lime'
  | 'light-green'
  | 'teal'
  | 'light-blue'
  | 'indigo'
  | 'violet'
  | 'grey'
  | 'slate'

export const colorToBgClass: Record<SemanticColor, string> = {
  blue: 'bg-blue-500',
  green: 'bg-green-500',
  cyan: 'bg-cyan-500',
  purple: 'bg-purple-500',
  pink: 'bg-pink-500',
  red: 'bg-red-500',
  orange: 'bg-orange-500',
  amber: 'bg-amber-500',
  yellow: 'bg-yellow-500',
  lime: 'bg-lime-500',
  'light-green': 'bg-green-400',
  teal: 'bg-teal-500',
  'light-blue': 'bg-sky-400',
  indigo: 'bg-indigo-500',
  violet: 'bg-violet-500',
  grey: 'bg-gray-400',
  slate: 'bg-slate-500',
}

export const avatarColorMap: Record<SemanticColor, string> = {
  blue: 'bg-chart-1/10 text-chart-1',
  green: 'bg-success/10 text-success',
  cyan: 'bg-chart-2/10 text-chart-2',
  purple: 'bg-chart-4/10 text-chart-4',
  pink: 'bg-chart-5/10 text-chart-5',
  red: 'bg-destructive/10 text-destructive',
  orange: 'bg-warning/10 text-warning',
  amber: 'bg-warning/10 text-warning',
  yellow: 'bg-warning/10 text-warning',
  lime: 'bg-chart-3/10 text-chart-3',
  'light-green': 'bg-success/10 text-success',
  teal: 'bg-chart-2/10 text-chart-2',
  'light-blue': 'bg-info/10 text-info',
  indigo: 'bg-chart-1/10 text-chart-1',
  violet: 'bg-chart-4/10 text-chart-4',
  grey: 'bg-muted text-muted-foreground',
  slate: 'bg-muted text-muted-foreground',
}

export function getAvatarColorClass(name: string): string {
  return avatarColorMap[stringToColor(name)]
}

export function getBgColorClass(color?: string): string {
  if (!color) return colorToBgClass.blue
  return (
    (colorToBgClass as Record<string, string>)[color] || colorToBgClass.blue
  )
}

/**
 * Chart color palette - Modern gradient colors compatible with light/dark themes
 * Uses HSL format for better theme adaptation
 */
export const CHART_COLORS = [
  'hsl(217, 91%, 60%)', // blue
  'hsl(142, 76%, 36%)', // green
  'hsl(38, 92%, 50%)', // amber
  'hsl(258, 90%, 66%)', // violet
  'hsl(330, 81%, 60%)', // pink
  'hsl(189, 94%, 43%)', // cyan
  'hsl(25, 95%, 53%)', // orange
  'hsl(239, 84%, 67%)', // indigo
  'hsl(173, 80%, 40%)', // teal
  'hsl(271, 91%, 65%)', // purple
  'hsl(199, 89%, 48%)', // sky
  'hsl(280, 65%, 60%)', // fuchsia
] as const

/**
 * Get a chart color by index (cycles through the palette)
 */
export function getChartColor(index: number): string {
  return CHART_COLORS[index % CHART_COLORS.length]
}

/**
 * Announcement status types
 */
export type AnnouncementType =
  | 'default'
  | 'ongoing'
  | 'success'
  | 'warning'
  | 'error'

/**
 * Announcement status color mapping
 */
export const ANNOUNCEMENT_TYPE_COLORS: Record<AnnouncementType, string> = {
  default: 'bg-neutral',
  ongoing: 'bg-info',
  success: 'bg-success',
  warning: 'bg-warning',
  error: 'bg-destructive',
}

/**
 * Get announcement status color class
 */
export function getAnnouncementColorClass(type?: string): string {
  const validType = (type || 'default') as AnnouncementType
  return ANNOUNCEMENT_TYPE_COLORS[validType] || ANNOUNCEMENT_TYPE_COLORS.default
}

/**
 * Semantic colors for tags and badges
 */
const TAG_COLORS = [
  'amber',
  'blue',
  'cyan',
  'green',
  'grey',
  'indigo',
  'light-blue',
  'lime',
  'orange',
  'pink',
  'purple',
  'red',
  'teal',
  'violet',
  'yellow',
] as const

/**
 * Convert string to a stable semantic color
 * Used for model tags, group badges, user avatars, etc.
 * Same string always returns the same color
 *
 * @param str - Input string (model name, group name, username, etc.)
 * @returns Semantic color name from TAG_COLORS
 *
 * @example
 * stringToColor('gpt-4') // 'blue'
 * stringToColor('claude-3') // 'purple'
 * stringToColor('default') // 'green'
 */
export function stringToColor(str: string): SemanticColor {
  let sum = 0
  for (let i = 0; i < str.length; i++) {
    sum += str.charCodeAt(i)
  }
  const index = sum % TAG_COLORS.length
  return TAG_COLORS[index]
}

/**
 * Distinct color palette for badges - each color is visually distinguishable
 * 15 unique colors with good light/dark mode support
 */
export const BADGE_PALETTE = [
  { dot: 'bg-blue-500', text: 'text-blue-600 dark:text-blue-400', bg: 'bg-blue-100 dark:bg-blue-900/30' },
  { dot: 'bg-rose-500', text: 'text-rose-600 dark:text-rose-400', bg: 'bg-rose-100 dark:bg-rose-900/30' },
  { dot: 'bg-emerald-500', text: 'text-emerald-600 dark:text-emerald-400', bg: 'bg-emerald-100 dark:bg-emerald-900/30' },
  { dot: 'bg-amber-500', text: 'text-amber-600 dark:text-amber-400', bg: 'bg-amber-100 dark:bg-amber-900/30' },
  { dot: 'bg-violet-500', text: 'text-violet-600 dark:text-violet-400', bg: 'bg-violet-100 dark:bg-violet-900/30' },
  { dot: 'bg-cyan-500', text: 'text-cyan-600 dark:text-cyan-400', bg: 'bg-cyan-100 dark:bg-cyan-900/30' },
  { dot: 'bg-orange-500', text: 'text-orange-600 dark:text-orange-400', bg: 'bg-orange-100 dark:bg-orange-900/30' },
  { dot: 'bg-teal-500', text: 'text-teal-600 dark:text-teal-400', bg: 'bg-teal-100 dark:bg-teal-900/30' },
  { dot: 'bg-pink-500', text: 'text-pink-600 dark:text-pink-400', bg: 'bg-pink-100 dark:bg-pink-900/30' },
  { dot: 'bg-indigo-500', text: 'text-indigo-600 dark:text-indigo-400', bg: 'bg-indigo-100 dark:bg-indigo-900/30' },
  { dot: 'bg-lime-500', text: 'text-lime-600 dark:text-lime-400', bg: 'bg-lime-100 dark:bg-lime-900/30' },
  { dot: 'bg-sky-400', text: 'text-sky-600 dark:text-sky-400', bg: 'bg-sky-100 dark:bg-sky-900/30' },
  { dot: 'bg-fuchsia-500', text: 'text-fuchsia-600 dark:text-fuchsia-400', bg: 'bg-fuchsia-100 dark:bg-fuchsia-900/30' },
  { dot: 'bg-red-500', text: 'text-red-600 dark:text-red-400', bg: 'bg-red-100 dark:bg-red-900/30' },
  { dot: 'bg-slate-500', text: 'text-slate-600 dark:text-slate-400', bg: 'bg-slate-100 dark:bg-slate-900/30' },
] as const

/**
 * Generate a consistent HSL color from a string.
 * Uses bit-shift hash for better distribution than simple charCode sum.
 */
export function stringToHslColor(str: string): string {
  if (!str) return 'hsl(0, 0%, 60%)'
  let hash = 0
  for (let i = 0; i < str.length; i++) {
    hash = str.charCodeAt(i) + ((hash << 5) - hash)
    hash = hash & hash
  }
  const hue = Math.abs(hash % 360)
  const saturation = 65 + (Math.abs(hash) % 10) // 65-75%
  const lightness = 50 + (Math.abs(hash >> 8) % 8) // 50-58%
  return `hsl(${hue}, ${saturation}%, ${lightness}%)`
}

/**
 * Deterministic color index for badge palette
 */
export function stringToBadgeIndex(str: string): number {
  let hash = 0
  for (let i = 0; i < str.length; i++) {
    hash = str.charCodeAt(i) + ((hash << 5) - hash)
    hash = hash & hash
  }
  return Math.abs(hash) % BADGE_PALETTE.length
}

/**
 * Get badge style classes (dot, text, bg) for a given string
 * Returns an object with dot, text, bg Tailwind classes
 */
export function getBadgeStyle(str: string): { dot: string; text: string; bg: string } {
  const index = stringToBadgeIndex(str)
  return BADGE_PALETTE[index]
}

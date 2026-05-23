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
/* eslint-disable react-refresh/only-export-components */
import * as React from 'react'
import { type LucideIcon } from 'lucide-react'
import { stringToColor } from '@/lib/colors'
import { cn } from '@/lib/utils'
import { useCopyToClipboard } from '@/hooks/use-copy-to-clipboard'

export const dotColorMap = {
  success: 'bg-emerald-500',
  warning: 'bg-amber-500',
  danger: 'bg-red-500',
  info: 'bg-sky-500',
  neutral: 'bg-slate-400',
  blue: 'bg-blue-500',
  green: 'bg-emerald-500',
  cyan: 'bg-cyan-500',
  purple: 'bg-violet-500',
  pink: 'bg-pink-500',
  red: 'bg-rose-500',
  orange: 'bg-orange-500',
  amber: 'bg-amber-500',
  yellow: 'bg-yellow-500',
  lime: 'bg-lime-500',
  'light-green': 'bg-green-400',
  teal: 'bg-teal-500',
  'light-blue': 'bg-sky-400',
  indigo: 'bg-indigo-500',
  violet: 'bg-purple-500',
  grey: 'bg-gray-400',
  slate: 'bg-slate-500',
} as const

export const textColorMap = {
  success: 'text-emerald-600 dark:text-emerald-400',
  warning: 'text-amber-600 dark:text-amber-400',
  danger: 'text-red-600 dark:text-red-400',
  info: 'text-sky-600 dark:text-sky-400',
  neutral: 'text-slate-500 dark:text-slate-400',
  blue: 'text-blue-600 dark:text-blue-400',
  green: 'text-emerald-600 dark:text-emerald-400',
  cyan: 'text-cyan-600 dark:text-cyan-400',
  purple: 'text-violet-600 dark:text-violet-400',
  pink: 'text-pink-600 dark:text-pink-400',
  red: 'text-rose-600 dark:text-rose-400',
  orange: 'text-orange-600 dark:text-orange-400',
  amber: 'text-amber-600 dark:text-amber-400',
  yellow: 'text-yellow-600 dark:text-yellow-400',
  lime: 'text-lime-600 dark:text-lime-400',
  'light-green': 'text-green-600 dark:text-green-400',
  teal: 'text-teal-600 dark:text-teal-400',
  'light-blue': 'text-sky-600 dark:text-sky-400',
  indigo: 'text-indigo-600 dark:text-indigo-400',
  violet: 'text-purple-600 dark:text-purple-400',
  grey: 'text-gray-500 dark:text-gray-400',
  slate: 'text-slate-600 dark:text-slate-400',
} as const

export type StatusVariant = keyof typeof dotColorMap

const sizeMap = {
  sm: 'text-xs gap-1.5',
  md: 'text-xs gap-1.5',
  lg: 'text-sm gap-2',
} as const

export interface StatusBadgeProps extends Omit<
  React.HTMLAttributes<HTMLSpanElement>,
  'children'
> {
  label?: string
  children?: React.ReactNode
  icon?: LucideIcon
  pulse?: boolean
  /** When false, hides the leading dot */
  showDot?: boolean
  variant?: StatusVariant | null
  size?: 'sm' | 'md' | 'lg' | null
  copyable?: boolean
  copyText?: string
  autoColor?: string
}

export function StatusBadge({
  label,
  children,
  icon: Icon,
  variant,
  size = 'sm',
  pulse = false,
  showDot = true,
  copyable = true,
  copyText,
  autoColor,
  className,
  onClick,
  ...props
}: StatusBadgeProps) {
  const { copyToClipboard } = useCopyToClipboard()

  const computedVariant: StatusVariant = autoColor
    ? (stringToColor(autoColor) as StatusVariant)
    : (variant ?? 'neutral')

  const handleClick = (e: React.MouseEvent<HTMLSpanElement>) => {
    if (copyable) {
      e.stopPropagation()
      copyToClipboard(copyText || label || '')
    }
    onClick?.(e)
  }

  const content =
    children ?? (label ? <span className='truncate'>{label}</span> : null)

  return (
    <span
      className={cn(
        'inline-flex w-fit shrink-0 items-center font-medium whitespace-nowrap',
        sizeMap[size ?? 'sm'],
        textColorMap[computedVariant],
        pulse && 'animate-pulse',
        copyable &&
          'cursor-pointer transition-opacity hover:opacity-70 active:scale-95',
        className
      )}
      onClick={handleClick}
      title={copyable ? `Click to copy: ${copyText || label || ''}` : undefined}
      {...props}
    >
      {showDot && (
        <span
          className={cn(
            'inline-block size-1.5 shrink-0 rounded-full',
            dotColorMap[computedVariant]
          )}
          aria-hidden='true'
        />
      )}
      {Icon && <Icon className='size-3 shrink-0' />}
      {content}
    </span>
  )
}

export const statusPresets = {
  active: {
    variant: 'success' as const,
    label: 'Active',
    showDot: true,
  },
  inactive: {
    variant: 'neutral' as const,
    label: 'Inactive',
    showDot: true,
  },
  invited: {
    variant: 'info' as const,
    label: 'Invited',
    showDot: true,
  },
  suspended: {
    variant: 'danger' as const,
    label: 'Suspended',
    showDot: true,
  },
  pending: {
    variant: 'warning' as const,
    label: 'Pending',
    showDot: true,
    pulse: true,
  },
} as const

export type StatusPreset = keyof typeof statusPresets

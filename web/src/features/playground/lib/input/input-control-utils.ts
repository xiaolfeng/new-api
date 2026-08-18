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
import type { GroupOption, ModelOption } from '../../types'

type InputControlStateOptions = {
  disabled?: boolean
  groups: GroupOption[]
  hasStopHandler: boolean
  isGenerating?: boolean
  isModelLoading?: boolean
  models: ModelOption[]
  text: string
  hasAttachments?: boolean
}

type InputControlState = {
  canSubmit: boolean
  isSelectorDisabled: boolean
  shouldShowStop: boolean
}

type SubmittableInputMessage = {
  text?: string | null
  files?: Array<{ url?: string; mediaType?: string }>
}

export type SubmittablePlaygroundInput = {
  text: string
  imageUrls: string[]
}

export function getImageUrlsFromFiles(
  files?: Array<{ url?: string; mediaType?: string }>
): string[] {
  if (!files?.length) {
    return []
  }

  return files
    .filter((file) => {
      const url = file.url?.trim()
      if (!url) return false
      if (file.mediaType?.startsWith('image/')) return true
      return url.startsWith('data:image/')
    })
    .map((file) => file.url!.trim())
}

export function getSubmittablePlaygroundInput(
  message: SubmittableInputMessage,
  disabled?: boolean
): SubmittablePlaygroundInput | null {
  if (disabled) {
    return null
  }

  const text = message.text?.trim() ?? ''
  const imageUrls = getImageUrlsFromFiles(message.files)
  if (!text && imageUrls.length === 0) {
    return null
  }

  return { text, imageUrls }
}

export function getSubmittableInputText(
  message: SubmittableInputMessage,
  disabled?: boolean
): string | null {
  return getSubmittablePlaygroundInput(message, disabled)?.text || null
}

export function getInputControlState({
  disabled,
  groups,
  hasStopHandler,
  isGenerating,
  isModelLoading,
  models,
  text,
  hasAttachments = false,
}: InputControlStateOptions): InputControlState {
  const hasModels = models.length > 0

  return {
    canSubmit:
      !disabled && hasModels && (text.trim().length > 0 || hasAttachments),
    isSelectorDisabled: disabled || isModelLoading || groups.length === 0,
    shouldShowStop: Boolean(isGenerating && hasStopHandler),
  }
}

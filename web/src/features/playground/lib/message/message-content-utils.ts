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
import { MESSAGE_ROLES, MESSAGE_STATUS } from '../../constants'
import type { Message } from '../../types'
import { splitHostToolContent, type HostToolSplitItem } from './host-tool-utils'
import { splitImageRecognitionContent } from './image-recognize-utils'
import { hasMessageImages } from './message-utils'
import { parseThinkTags } from './message-reasoning-utils'

type MessageContentStateBase = {
  displayContent: string
  isReasoningStreaming: boolean
  hostTools: HostToolSplitItem[]
  hasSources: boolean
  isAssistant: boolean
  showLoader: boolean
  showMessageContent: boolean
  sources: NonNullable<Message['sources']>
}

type MessageContentState = MessageContentStateBase &
  (
    | {
        hasReasoning: true
        reasoningContent: string
      }
    | {
        hasReasoning: false
        reasoningContent: undefined
      }
  )

function shouldShowMessageLoader(
  message: Message,
  isAssistant: boolean,
  versionContent: string
): boolean {
  return (
    isAssistant &&
    !message.isReasoningStreaming &&
    (message.status === MESSAGE_STATUS.LOADING ||
      (message.status === MESSAGE_STATUS.STREAMING && !versionContent))
  )
}

function shouldShowMessageContent(
  message: Message,
  versionContent: string
): boolean {
  const hasVisibleText =
    splitHostToolContent(
      splitImageRecognitionContent(getRawDisplayContent(message, versionContent))
        .rest
    ).rest.length > 0
  return (
    (message.from === MESSAGE_ROLES.USER || !message.isReasoningStreaming) &&
    (hasVisibleText || hasMessageImages(message))
  )
}

function getRawDisplayContent(message: Message, versionContent: string): string {
  if (message.from !== MESSAGE_ROLES.ASSISTANT) {
    return versionContent
  }

  if (!versionContent.includes('<think>')) {
    return versionContent
  }

  return parseThinkTags(versionContent).visibleContent
}

function getDisplayContent(message: Message, versionContent: string): string {
  return splitHostToolContent(
    splitImageRecognitionContent(
      getRawDisplayContent(message, versionContent)
    ).rest
  ).rest
}

function foldRecognitionIntoReasoning(
  reasoning: string | undefined,
  recognitionFromContent: string | undefined
): string | undefined {
  const split = reasoning
    ? splitImageRecognitionContent(reasoning)
    : { rest: '', recognition: undefined, isStreaming: false }
  const recognition = split.recognition || recognitionFromContent
  const rest = split.recognition ? split.rest : (reasoning ?? '')
  const parts = [recognition, rest].filter(
    (part) => typeof part === 'string' && part.trim() !== ''
  )
  if (parts.length === 0) return undefined
  return parts.join('\n\n')
}

export function getMessageContentState(
  message: Message,
  versionContent: string
): MessageContentState {
  const isAssistant = message.from === MESSAGE_ROLES.ASSISTANT
  const sources = message.sources ?? []
  const rawDisplay = getRawDisplayContent(message, versionContent)
  const recognition = splitImageRecognitionContent(rawDisplay)
  const foldedReasoning = isAssistant
    ? foldRecognitionIntoReasoning(
        message.reasoning?.content,
        recognition.recognition
      )
    : undefined
  const showLoader = shouldShowMessageLoader(
    message,
    isAssistant,
    versionContent
  )
  const showMessageContent = shouldShowMessageContent(message, versionContent)
  const hostTools = splitHostToolContent(recognition.rest)

  const baseState: MessageContentStateBase = {
    displayContent: getDisplayContent(message, versionContent),
    isReasoningStreaming:
      Boolean(message.isReasoningStreaming) || recognition.isStreaming,
    hostTools: hostTools.tools,
    hasSources: sources.length > 0,
    isAssistant,
    showLoader,
    showMessageContent,
    sources,
  }

  if (foldedReasoning) {
    return {
      ...baseState,
      hasReasoning: true,
      reasoningContent: foldedReasoning,
    }
  }

  return {
    ...baseState,
    hasReasoning: false,
    reasoningContent: undefined,
  }
}

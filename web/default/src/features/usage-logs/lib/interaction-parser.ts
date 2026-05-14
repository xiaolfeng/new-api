export type InteractionType = 'input' | 'output' | 'callback'

export interface FlattenedItem {
  type: string
  role?: string
  text?: string
}

export interface StructuredInteractionData {
  responsesRequestBlocks: unknown[]
  responsesToolResponses: unknown[]
  responsesResponseBlocks: unknown[]
}

export interface ClaudeBlock {
  type: string
  content?: string
}

export interface ResponseBlock {
  type: string
  content?: string
}

export interface OpenAIBlock {
  type: string
  content?: string
}

export interface OpenAIStructuredInteractionData {
  openAIRequestBlocks: unknown[]
  openAIToolResponses: unknown[]
  openAIResponseBlocks: OpenAIBlock[]
}

interface PromptItem {
  type: string
  content?: unknown
  role?: string
  text?: string
}

interface ContentPart {
  type: string
  text?: string
}

function flattenResponsesPromptInputItems(input: unknown): FlattenedItem[] {
  if (!Array.isArray(input)) return []

  const items: FlattenedItem[] = []
  for (const item of input) {
    if (!item || typeof item !== 'object') continue

    const typed = item as PromptItem

    if (typed.type === 'message') {
      const content: ContentPart[] = Array.isArray(typed.content)
        ? typed.content
        : []
      for (const part of content) {
        if (!part || typeof part !== 'object') continue
        if (
          part.type !== 'input_text' &&
          part.type !== 'text' &&
          part.type !== 'output_text'
        )
          continue
        items.push({ type: part.type, role: typed.role, text: part.text })
      }
      continue
    }

    if (
      typed.type === 'function_call' ||
      typed.type === 'function_call_output' ||
      typed.type === 'input_text' ||
      typed.type === 'text' ||
      typed.type === 'output_text'
    ) {
      items.push(typed as FlattenedItem)
    }
  }

  return items
}

function inferResponsesInteractionType(
  items: FlattenedItem[]
): InteractionType | null {
  if (!items.length) return null

  const meaningful = items.filter(
    (item) => item && typeof item === 'object' && item.type
  )
  if (!meaningful.length) return null

  const last = meaningful[meaningful.length - 1]
  if (last.type === 'input_text' || last.type === 'text') return 'input'
  if (last.type === 'function_call_output') return 'callback'
  if (last.type === 'function_call') return 'callback'

  if (last.type === 'output_text') {
    for (let i = meaningful.length - 2; i >= 0; i--) {
      if (meaningful[i].type === 'function_call_output') return 'output'
    }
  }

  return null
}

function inferResponsesStructuredInteractionType(
  data: StructuredInteractionData
): InteractionType | null {
  const requestBlocks = (
    Array.isArray(data.responsesRequestBlocks)
      ? data.responsesRequestBlocks
      : []
  ) as Array<{ text?: string }>
  const toolResponses = Array.isArray(data.responsesToolResponses)
    ? data.responsesToolResponses
    : []
  const responseBlocks = (
    Array.isArray(data.responsesResponseBlocks)
      ? data.responsesResponseBlocks
      : []
  ) as Array<{ type: string; content?: string }>

  const hasRequestInput = requestBlocks.some(
    (block) => typeof block.text === 'string' && block.text.trim() !== ''
  )
  const hasToolResponse = toolResponses.length > 0
  const hasTextOutput = responseBlocks.some(
    (block) =>
      block.type === 'output_text' &&
      typeof block.content === 'string' &&
      block.content.trim() !== ''
  )
  const hasToolUse = responseBlocks.some(
    (block) => block.type === 'function_call'
  )

  if (hasRequestInput) return 'input'
  if (!hasRequestInput && hasTextOutput && !hasToolUse) return 'output'
  if (hasToolResponse || hasToolUse || responseBlocks.length > 0)
    return 'callback'

  return null
}

function inferOpenAIStructuredInteractionType(
  data: OpenAIStructuredInteractionData
): InteractionType | null {
  const requestBlocks = (
    Array.isArray(data.openAIRequestBlocks) ? data.openAIRequestBlocks : []
  ) as Array<{ text?: string }>
  const toolResponses = Array.isArray(data.openAIToolResponses)
    ? data.openAIToolResponses
    : []
  const responseBlocks = Array.isArray(data.openAIResponseBlocks)
    ? data.openAIResponseBlocks
    : []

  const hasRequestInput = requestBlocks.some(
    (block) => typeof block.text === 'string' && block.text.trim() !== ''
  )
  const hasToolResponse = toolResponses.length > 0
  const hasTextOutput = responseBlocks.some(
    (block) =>
      (block.type === 'content' || block.type === 'reasoning') &&
      typeof block.content === 'string' &&
      block.content.trim() !== ''
  )
  const hasToolUse = responseBlocks.some((block) => block.type === 'tool_call')

  if (hasToolResponse) return 'callback'
  if (hasToolUse) return 'callback'
  if (!hasRequestInput && hasTextOutput) return 'output'
  if (hasRequestInput) return 'input'
  if (responseBlocks.length > 0) return 'callback'

  return null
}

export function parseInteractionType(record: unknown): InteractionType | null {
  if (!record) return null

  try {
    const data =
      typeof record === 'string'
        ? (JSON.parse(record) as Record<string, unknown>)
        : (record as Record<string, unknown>)

    const request = (data.request as Record<string, unknown>) ?? {}
    const promptObj = ((request.body as Record<string, unknown>) ??
      (data.prompt as Record<string, unknown> | string) ??
      {}) as Record<string, unknown>
    const completion =
      (data.response as Record<string, unknown>)?.body ??
      (data.completion as unknown) ??
      ''

    const claudeRequestBlocks: ClaudeBlock[] = Array.isArray(
      data.claudeRequestBlocks
    )
      ? data.claudeRequestBlocks
      : Array.isArray(promptObj.claudeRequestBlocks)
        ? (promptObj.claudeRequestBlocks as ClaudeBlock[])
        : []
    const claudeToolResponses = Array.isArray(data.claudeToolResponses)
      ? (data.claudeToolResponses as unknown[])
      : []
    const claudeResponseBlocks: ClaudeBlock[] = Array.isArray(
      data.claudeResponseBlocks
    )
      ? (data.claudeResponseBlocks as ClaudeBlock[])
      : []

    const responsesRequestBlocks = Array.isArray(data.responsesRequestBlocks)
      ? (data.responsesRequestBlocks as unknown[])
      : []
    const responsesToolResponses = Array.isArray(data.responsesToolResponses)
      ? (data.responsesToolResponses as unknown[])
      : []
    const responsesResponseBlocks: ResponseBlock[] = Array.isArray(
      data.responsesResponseBlocks
    )
      ? (data.responsesResponseBlocks as ResponseBlock[])
      : []

    const openAIResponseBlocks: OpenAIBlock[] = Array.isArray(
      data.openaiResponseBlocks
    )
      ? (data.openaiResponseBlocks as OpenAIBlock[])
      : []
    const openAIRequestBlocks = Array.isArray(data.openaiRequestBlocks)
      ? (data.openaiRequestBlocks as unknown[])
      : []
    const openaiToolResponses = Array.isArray(data.openaiToolResponses)
      ? (data.openaiToolResponses as unknown[])
      : []

    const responsesPromptItems = flattenResponsesPromptInputItems(
      promptObj.input
    )

    // Try structured responses format first
    const responsesStructuredType = inferResponsesStructuredInteractionType({
      responsesRequestBlocks,
      responsesToolResponses,
      responsesResponseBlocks,
    })
    if (responsesStructuredType) return responsesStructuredType

    // Try structured OpenAI Chat Completions format before legacy prompt fallback.
    const openAIStructuredType = inferOpenAIStructuredInteractionType({
      openAIRequestBlocks,
      openAIToolResponses: openaiToolResponses,
      openAIResponseBlocks,
    })
    if (openAIStructuredType) return openAIStructuredType

    // Try flattened prompt items
    const responsesType = inferResponsesInteractionType(responsesPromptItems)
    if (responsesType) return responsesType

    // Fallback: analyze prompt/completion fields
    const lastUserMessage =
      (promptObj.lastUserMessage as Record<string, unknown>) ?? {}
    const legacyToolInvokes = Array.isArray(data.toolInvokes)
      ? (data.toolInvokes as unknown[])
      : []

    const hasPromptObjectContent =
      promptObj !== null &&
      !Array.isArray(promptObj) &&
      Object.keys(promptObj).some((key) => key !== 'input')

    const hasNonToolInput =
      (typeof data.prompt === 'string' &&
        (data.prompt as string).trim() !== '') ||
      (typeof lastUserMessage.content === 'string' &&
        lastUserMessage.content.trim() !== '') ||
      claudeRequestBlocks.length > 0 ||
      responsesRequestBlocks.length > 0 ||
      openAIRequestBlocks.length > 0 ||
      hasPromptObjectContent ||
      (Array.isArray(data.prompt) && data.prompt.length > 0)

    const hasToolInput =
      claudeToolResponses.length > 0 ||
      responsesToolResponses.length > 0 ||
      openaiToolResponses.length > 0

    const hasTextOutput =
      (typeof completion === 'string' && completion.trim() !== '') ||
      claudeResponseBlocks.some(
        (block) =>
          block.type === 'text' &&
          typeof block.content === 'string' &&
          block.content.trim() !== ''
      ) ||
      responsesResponseBlocks.some(
        (block) =>
          block.type === 'output_text' &&
          typeof block.content === 'string' &&
          block.content.trim() !== ''
      ) ||
      openAIResponseBlocks.some(
        (block) =>
          (block.type === 'content' || block.type === 'reasoning') &&
          typeof block.content === 'string' &&
          block.content.trim() !== ''
      )

    const hasAnyOutput =
      hasTextOutput ||
      (typeof completion === 'object' &&
        completion !== null &&
        ((Array.isArray(completion) && completion.length > 0) ||
          (!Array.isArray(completion) &&
            Object.keys(completion as Record<string, unknown>).length > 0))) ||
      claudeResponseBlocks.length > 0 ||
      responsesResponseBlocks.length > 0 ||
      openAIResponseBlocks.length > 0

    const hasToolUse =
      claudeResponseBlocks.some((block) => block.type === 'tool_use') ||
      responsesResponseBlocks.some((block) => block.type === 'function_call') ||
      openAIResponseBlocks.some((block) => block.type === 'tool_call') ||
      legacyToolInvokes.length > 0

    if (hasNonToolInput) return 'input'
    if (!hasNonToolInput && hasTextOutput && !hasToolUse) return 'output'
    if (hasToolInput || hasToolUse || hasAnyOutput) return 'callback'

    return null
  } catch {
    return null
  }
}

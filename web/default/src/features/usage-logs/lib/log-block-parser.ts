/**
 * Structured log block parser for usage log detail records.
 *
 * Parses the JSON `content` field of a usage log into typed sections
 * (input, thinking, answer, tool calls, tool responses) for three
 * upstream formats: OpenAI, Claude, and Responses API.
 */

// ---------------------------------------------------------------------------
// Type definitions — mirrors Go `model/log_record.go`
// ---------------------------------------------------------------------------

export interface ClaudeRequestBlock {
  type?: string
  text?: string
}

export interface ClaudeToolResponseBlock {
  toolUseId?: string
  name?: string
  type?: string
  role?: string
}

export interface ClaudeResponseBlock {
  id?: string
  type?: string
  content?: string
  name?: string
  input?: unknown
}

export interface OpenAIRequestBlock {
  type?: string
  role?: string
  text?: string
}

export interface OpenAIToolResponseBlock {
  toolCallId?: string
  name?: string
  type?: string
  role?: string
}

export interface OpenAIResponseBlock {
  id?: string
  type?: string
  role?: string
  content?: string
  name?: string
  callIndex?: number | null
  arguments?: unknown
}

export interface ResponsesRequestBlock {
  type?: string
  role?: string
  text?: string
}

export interface ResponsesToolResponseBlock {
  callId?: string
  name?: string
  type?: string
}

export interface ResponsesResponseBlock {
  id?: string
  type?: string
  content?: string
  name?: string
  callId?: string
  arguments?: unknown
}

export interface ToolInvokeRecord {
  id?: string
  name?: string
  input?: unknown
  result?: unknown
  resultText?: string
  isError?: boolean | null
  stopReason?: string
  responseRole?: string
}

export interface LogDetailRecord {
  prompt?: Record<string, unknown>
  completion?: string
  headers?: Record<string, string>
  toolInvokes?: ToolInvokeRecord[]
  openaiResponseBlocks?: OpenAIResponseBlock[]
  claudeRequestBlocks?: ClaudeRequestBlock[]
  claudeToolResponses?: ClaudeToolResponseBlock[]
  claudeResponseBlocks?: ClaudeResponseBlock[]
  responsesRequestBlocks?: ResponsesRequestBlock[]
  responsesToolResponses?: ResponsesToolResponseBlock[]
  responsesResponseBlocks?: ResponsesResponseBlock[]
  openaiRequestBlocks?: OpenAIRequestBlock[]
  openaiToolResponses?: OpenAIToolResponseBlock[]
}

// ---------------------------------------------------------------------------
// Parsed tool-use row (format-agnostic)
// ---------------------------------------------------------------------------

export interface ToolUseRow {
  order: number
  id: string
  callId?: string
  callIndex?: number | null
  name: string
  arguments?: unknown
  input?: unknown
}

export interface ToolResponseRow {
  order: number
  name: string
  callId: string
  type?: string
}

// ---------------------------------------------------------------------------
// Parsed sections — the final shape consumed by the UI
// ---------------------------------------------------------------------------

export interface ParsedSections {
  /** Which upstream format produced these sections */
  format: 'openai' | 'claude' | 'responses' | 'none'

  /** User text input blocks */
  requestBlocks: Array<{
    type: string
    role: string
    text: string
  }>

  /** Tool response rows (from previous turns) */
  toolResponses: ToolResponseRow[]

  /** Thinking / reasoning content (may be empty) */
  thinking: string

  /** Final answer text */
  answer: string

  /** Tool calls issued by the model */
  toolUses: ToolUseRow[]
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

function ensureArray<T>(value: unknown): T[] {
  return Array.isArray(value) ? value : []
}

/**
 * When `responsesRequestBlocks` is empty, try to derive request data
 * from `prompt.input` (which may contain message / input_text items).
 */
function deriveResponsesRequestDataFromPromptInput(
  promptInput: unknown,
): {
  requestBlocks: Array<{ type: string; role: string; text: string }>
  toolResponses: ToolResponseRow[]
} {
  if (!Array.isArray(promptInput)) {
    return { requestBlocks: [], toolResponses: [] }
  }

  const requestBlocks: Array<{ type: string; role: string; text: string }> = []
  const toolResponses: ToolResponseRow[] = []

  for (const item of promptInput) {
    if (!item || typeof item !== 'object') continue

    if ((item as Record<string, unknown>).type === 'message') {
      const role =
        typeof (item as Record<string, unknown>).role === 'string'
          ? ((item as Record<string, unknown>).role as string)
          : ''
      const content = Array.isArray((item as Record<string, unknown>).content)
        ? ((item as Record<string, unknown>).content as unknown[])
        : []
      for (const part of content) {
        if (!part || typeof part !== 'object') continue
        const p = part as Record<string, unknown>
        if (!['input_text', 'text'].includes(p.type as string)) continue
        if (typeof p.text !== 'string' || (p.text as string).trim() === '') continue
        requestBlocks.push({ type: p.type as string, role, text: p.text as string })
      }
      continue
    }

    if (['input_text', 'text'].includes((item as Record<string, unknown>).type as string)) {
      const p = item as Record<string, unknown>
      if (typeof p.text !== 'string' || (p.text as string).trim() === '') continue
      requestBlocks.push({
        type: p.type as string,
        role: typeof p.role === 'string' ? (p.role as string) : '',
        text: p.text as string,
      })
      continue
    }

    if ((item as Record<string, unknown>).type === 'function_call_output') {
      const p = item as Record<string, unknown>
      toolResponses.push({
        order: toolResponses.length + 1,
        callId: (p.call_id || p.callId || '') as string,
        name: (p.name || '') as string,
        type: 'function_call_output',
      })
    }
  }

  return { requestBlocks, toolResponses }
}

// ---------------------------------------------------------------------------
// Main parser
// ---------------------------------------------------------------------------

export function parseLogDetailRecord(
  record: LogDetailRecord | null,
): ParsedSections {
  const empty: ParsedSections = {
    format: 'none',
    requestBlocks: [],
    toolResponses: [],
    thinking: '',
    answer: '',
    toolUses: [],
  }

  if (!record) return empty

  // Claude format
  const claudeRequestBlocks = ensureArray<ClaudeRequestBlock>(record.claudeRequestBlocks)
  const claudeToolResponses = ensureArray<ClaudeToolResponseBlock>(record.claudeToolResponses)
  const claudeResponseBlocks = ensureArray<ClaudeResponseBlock>(record.claudeResponseBlocks)
  const hasClaude =
    claudeRequestBlocks.length > 0 ||
    claudeToolResponses.length > 0 ||
    claudeResponseBlocks.length > 0

  if (hasClaude) {
    const thinkingParts: string[] = []
    const answerParts: string[] = []
    const toolUses: ToolUseRow[] = []

    claudeResponseBlocks.forEach((block, index) => {
      if (!block || typeof block !== 'object') return
      if (block.type === 'thinking' && block.content) {
        thinkingParts.push(block.content)
        return
      }
      if (block.type === 'text' && block.content) {
        answerParts.push(block.content)
        return
      }
      if (block.type === 'tool_use') {
        toolUses.push({
          order: toolUses.length + 1,
          id: block.id || `claude-tool-${index}`,
          name: block.name || '',
          input: block.input,
        })
      }
    })

    return {
      format: 'claude',
      requestBlocks: claudeRequestBlocks
        .filter((b) => b.text && b.text.trim() !== '')
        .map((b) => ({ type: b.type || 'text', role: '', text: b.text! })),
      toolResponses: claudeToolResponses.map((item, index) => ({
        order: index + 1,
        name: item.name || '',
        callId: item.toolUseId || '',
        type: item.type,
      })),
      thinking: thinkingParts.join('\n\n'),
      answer: answerParts.join('\n\n'),
      toolUses,
    }
  }

  // OpenAI format
  const openaiRequestBlocks = ensureArray<OpenAIRequestBlock>(record.openaiRequestBlocks)
  const openaiToolResponses = ensureArray<OpenAIToolResponseBlock>(record.openaiToolResponses)
  const openaiResponseBlocks = ensureArray<OpenAIResponseBlock>(record.openaiResponseBlocks)
  const hasOpenAI =
    openaiRequestBlocks.length > 0 ||
    openaiToolResponses.length > 0 ||
    openaiResponseBlocks.length > 0

  if (hasOpenAI) {
    const thinkingParts: string[] = []
    const answerParts: string[] = []
    const toolUses: ToolUseRow[] = []

    openaiResponseBlocks.forEach((block, index) => {
      if (!block || typeof block !== 'object') return
      if (block.type === 'reasoning' && block.content) {
        thinkingParts.push(block.content)
        return
      }
      if (block.type === 'content' && block.content) {
        answerParts.push(block.content)
        return
      }
      if (block.type === 'tool_call') {
        toolUses.push({
          order: toolUses.length + 1,
          id: block.id || `openai-tool-${index}`,
          callId: block.id,
          callIndex: block.callIndex ?? null,
          name: block.name || '',
          arguments: block.arguments,
        })
      }
    })

    return {
      format: 'openai',
      requestBlocks: openaiRequestBlocks
        .filter((b) => b.text && b.text.trim() !== '')
        .map((b) => ({ type: b.type || 'text', role: b.role || '', text: b.text! })),
      toolResponses: openaiToolResponses.map((item, index) => ({
        order: index + 1,
        name: item.name || '',
        callId: item.toolCallId || '',
        type: item.type,
      })),
      thinking: thinkingParts.join('\n\n'),
      answer: answerParts.join('\n\n'),
      toolUses,
    }
  }

  // Responses API format
  const prompt = record.prompt
  const promptInput = prompt?.input
  const fallbackData = deriveResponsesRequestDataFromPromptInput(promptInput)

  let responsesRequestBlocks = ensureArray<ResponsesRequestBlock>(record.responsesRequestBlocks)
  let responsesToolResponses = ensureArray<ResponsesToolResponseBlock>(record.responsesToolResponses)
  if (responsesRequestBlocks.length === 0) {
    responsesRequestBlocks = fallbackData.requestBlocks as unknown as ResponsesRequestBlock[]
  }
  if (responsesToolResponses.length === 0) {
    responsesToolResponses =
      fallbackData.toolResponses as unknown as ResponsesToolResponseBlock[]
  }

  const responsesResponseBlocks = ensureArray<ResponsesResponseBlock>(record.responsesResponseBlocks)
  const hasResponses =
    responsesRequestBlocks.length > 0 ||
    responsesToolResponses.length > 0 ||
    responsesResponseBlocks.length > 0

  if (hasResponses) {
    const answerParts: string[] = []
    const toolUses: ToolUseRow[] = []

    responsesResponseBlocks.forEach((block, index) => {
      if (!block || typeof block !== 'object') return
      if (block.type === 'output_text' && block.content) {
        answerParts.push(block.content)
        return
      }
      if (block.type === 'function_call') {
        toolUses.push({
          order: toolUses.length + 1,
          id: block.id || block.callId || `responses-tool-${index}`,
          callId: block.callId || block.id,
          name: block.name || '',
          arguments: block.arguments,
        })
      }
    })

    return {
      format: 'responses',
      requestBlocks: responsesRequestBlocks
        .filter((b) => b.text && b.text.trim() !== '')
        .map((b) => ({ type: b.type || 'input_text', role: b.role || '', text: b.text! })),
      toolResponses: responsesToolResponses.map((item, index) => ({
        order: index + 1,
        name: item.name || '',
        callId: item.callId || '',
        type: item.type,
      })),
      thinking: '',
      answer: answerParts.join('\n\n'),
      toolUses,
    }
  }

  return empty
}

/**
 * Returns true when the parsed sections contain any meaningful structured data.
 */
export function hasStructuredData(sections: ParsedSections): boolean {
  return (
    sections.format !== 'none' &&
    (sections.requestBlocks.length > 0 ||
      sections.toolResponses.length > 0 ||
      sections.thinking.trim() !== '' ||
      sections.answer.trim() !== '' ||
      sections.toolUses.length > 0)
  )
}

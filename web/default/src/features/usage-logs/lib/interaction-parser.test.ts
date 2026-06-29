import { describe, it, expect } from 'bun:test'
import { parseInteractionType } from './interaction-parser'

describe('parseInteractionType', () => {
  it('returns null for null input', () => {
    expect(parseInteractionType(null)).toBeNull()
  })

  it('returns null for undefined input', () => {
    expect(parseInteractionType(undefined)).toBeNull()
  })

  it('returns null for empty object', () => {
    expect(parseInteractionType({})).toBeNull()
  })

  it('detects input type from claude request blocks', () => {
    const record = {
      claudeRequestBlocks: [{ type: 'text', text: 'Hello' }],
      claudeToolResponses: [],
      claudeResponseBlocks: [],
    }
    expect(parseInteractionType(record)).toBe('input')
  })

  it('detects output type from claude response blocks', () => {
    const record = {
      claudeRequestBlocks: [],
      claudeToolResponses: [],
      claudeResponseBlocks: [{ type: 'text', content: 'Response text here' }],
    }
    expect(parseInteractionType(record)).toBe('output')
  })

  it('detects callback type from tool responses', () => {
    const record = {
      claudeRequestBlocks: [],
      claudeToolResponses: [
        { toolUseId: '1', name: 'tool1', type: 'tool_result' },
      ],
      claudeResponseBlocks: [],
    }
    expect(parseInteractionType(record)).toBe('callback')
  })

  it('detects callback from tool_use in response blocks', () => {
    const record = {
      claudeRequestBlocks: [],
      claudeToolResponses: [],
      claudeResponseBlocks: [
        { type: 'tool_use', id: '1', name: 'tool1', input: {} },
      ],
    }
    expect(parseInteractionType(record)).toBe('callback')
  })

  it('detects input from openai request blocks', () => {
    const record = {
      openaiRequestBlocks: [{ type: 'text', role: 'user', text: 'Hello' }],
      openaiToolResponses: [],
      openaiResponseBlocks: [],
    }
    expect(parseInteractionType(record)).toBe('input')
  })

  it('handles JSON string input', () => {
    const record = JSON.stringify({
      claudeRequestBlocks: [{ type: 'text', text: 'Hello' }],
    })
    expect(parseInteractionType(record)).toBe('input')
  })

  it('handles responses format with request blocks', () => {
    const record = {
      responsesRequestBlocks: [
        { type: 'input_text', text: 'Hello', role: 'user' },
      ],
      responsesToolResponses: [],
      responsesResponseBlocks: [],
    }
    expect(parseInteractionType(record)).toBe('input')
  })

  it('handles prompt field fallback', () => {
    const record = {
      prompt: { lastUserMessage: { content: 'Hello' } },
    }
    expect(parseInteractionType(record)).toBe('input')
  })

  it('detects output from responses format', () => {
    const record = {
      responsesRequestBlocks: [],
      responsesToolResponses: [],
      responsesResponseBlocks: [
        { type: 'output_text', content: 'AI response' },
      ],
    }
    expect(parseInteractionType(record)).toBe('output')
  })

  it('detects callback from responses function_call', () => {
    const record = {
      responsesRequestBlocks: [],
      responsesToolResponses: [],
      responsesResponseBlocks: [
        { type: 'function_call', name: 'tool1', arguments: '{}' },
      ],
    }
    expect(parseInteractionType(record)).toBe('callback')
  })

  it('detects input from openai response blocks with reasoning', () => {
    const record = {
      openaiRequestBlocks: [{ type: 'text', role: 'user', text: 'Hello' }],
      openaiToolResponses: [],
      openaiResponseBlocks: [{ type: 'reasoning', content: 'Thinking...' }],
    }
    expect(parseInteractionType(record)).toBe('input')
  })

  it('detects output from openai structured response blocks', () => {
    const record = {
      openaiRequestBlocks: [],
      openaiToolResponses: [],
      openaiResponseBlocks: [{ type: 'content', content: 'Done' }],
    }
    expect(parseInteractionType(record)).toBe('output')
  })

  it('prioritizes request input over openai tool calls', () => {
    const record = {
      openaiRequestBlocks: [
        { type: 'text', role: 'user', text: 'Run command' },
      ],
      openaiToolResponses: [],
      openaiResponseBlocks: [
        { type: 'content', content: 'I will run it.' },
        { type: 'tool_call', id: 'call_1', name: 'exec_command' },
      ],
    }
    expect(parseInteractionType(record)).toBe('input')
  })

  it('detects openai tool response with text output as output', () => {
    const record = {
      openaiRequestBlocks: [],
      openaiToolResponses: [
        { type: 'tool', role: 'tool', toolCallId: 'call_1' },
      ],
      openaiResponseBlocks: [{ type: 'content', content: 'Task done' }],
    }
    expect(parseInteractionType(record)).toBe('output')
  })

  it('detects openai tool responses as callback', () => {
    const record = {
      openaiRequestBlocks: [],
      openaiToolResponses: [
        { type: 'tool', role: 'tool', toolCallId: 'call_1' },
      ],
      openaiResponseBlocks: [],
    }
    expect(parseInteractionType(record)).toBe('callback')
  })

  it('returns null for invalid JSON string', () => {
    expect(parseInteractionType('not-json')).toBeNull()
  })

  it('handles prompt as string', () => {
    const record = {
      prompt: 'Hello world',
    }
    expect(parseInteractionType(record)).toBe('input')
  })

  it('handles responses format with tool responses', () => {
    const record = {
      responsesRequestBlocks: [],
      responsesToolResponses: [
        { type: 'function_call_output', output: 'result' },
      ],
      responsesResponseBlocks: [],
    }
    expect(parseInteractionType(record)).toBe('callback')
  })

  it('detects bamboo input from request blocks', () => {
    const record = {
      bambooRequestBlocks: [{ text: 'Hello bamboo' }],
      bambooToolResponses: [],
      bambooResponseBlocks: [],
    }
    expect(parseInteractionType(record)).toBe('input')
  })

  it('detects bamboo output from response text blocks', () => {
    const record = {
      bambooRequestBlocks: [],
      bambooToolResponses: [],
      bambooResponseBlocks: [{ type: 'text', text: 'Bamboo response' }],
    }
    expect(parseInteractionType(record)).toBe('output')
  })

  it('detects bamboo callback from bare tool responses (no text, no tool_use)', () => {
    const record = {
      bambooRequestBlocks: [],
      bambooToolResponses: [{ toolUseId: '1', output: 'result' }],
      bambooResponseBlocks: [],
    }
    expect(parseInteractionType(record)).toBe('callback')
  })

  it('detects bamboo output when tool response exists but AI gives final text', () => {
    const record = {
      bambooRequestBlocks: [],
      bambooToolResponses: [{ toolUseId: '1', output: 'result' }],
      bambooResponseBlocks: [{ type: 'text', text: 'Final answer' }],
    }
    expect(parseInteractionType(record)).toBe('output')
  })

  it('detects bamboo callback when tool_use follows tool response', () => {
    const record = {
      bambooRequestBlocks: [],
      bambooToolResponses: [{ toolUseId: '1', output: 'result' }],
      bambooResponseBlocks: [
        { type: 'text', text: 'Let me check' },
        { type: 'tool_use', id: '2', name: 'tool2', input: {} },
      ],
    }
    expect(parseInteractionType(record)).toBe('callback')
  })

  it('detects bamboo callback from tool_use in response blocks', () => {
    const record = {
      bambooRequestBlocks: [],
      bambooToolResponses: [],
      bambooResponseBlocks: [
        { type: 'tool_use', id: '1', name: 'tool1', input: {} },
      ],
    }
    expect(parseInteractionType(record)).toBe('callback')
  })

  it('does not let prompt fallback short-circuit bamboo output', () => {
    const record = {
      prompt: { lastUserMessage: { content: 'User message' } },
      bambooRequestBlocks: [],
      bambooToolResponses: [],
      bambooResponseBlocks: [{ type: 'text', text: 'Bamboo output' }],
    }
    expect(parseInteractionType(record)).toBe('output')
  })
})

import { describe, expect, it } from 'vitest'

import { MESSAGE_ROLES, MESSAGE_STATUS } from '../../constants'
import type { Message } from '../../types'
import { applyStreamingToolCalls } from './message-streaming-utils'

function emptyAssistant(): Message {
  return {
    key: 'a1',
    from: MESSAGE_ROLES.ASSISTANT,
    versions: [{ id: 'v1', content: '' }],
    status: MESSAGE_STATUS.STREAMING,
  }
}

describe('applyStreamingToolCalls', () => {
  it('merges incremental arguments and MCP output by id', () => {
    let message = applyStreamingToolCalls(
      emptyAssistant(),
      JSON.stringify([
        {
          index: 0,
          id: 'call_1',
          type: 'function',
          function: { name: 'WebSearch', arguments: '{"que' },
        },
      ])
    )
    message = applyStreamingToolCalls(
      message,
      JSON.stringify([
        { index: 0, id: 'call_1', function: { arguments: 'ry":"cats"}' } },
      ])
    )
    message = applyStreamingToolCalls(
      message,
      JSON.stringify([
        {
          index: 0,
          id: 'call_1',
          function: {
            output: '{"content":[{"type":"text","text":"ok"}],"isError":false}',
          },
        },
      ])
    )

    expect(message.toolCalls).toHaveLength(1)
    expect(message.toolCalls?.[0].function.name).toBe('WebSearch')
    expect(message.toolCalls?.[0].function.arguments).toBe('{"query":"cats"}')
    expect(message.toolCalls?.[0].function.output).toContain('"isError":false')
  })
})

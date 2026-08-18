import { describe, expect, it } from 'vitest'

import { splitHostToolContent } from './host-tool-utils'

describe('splitHostToolContent', () => {
  it('returns original text when there is no fence', () => {
    expect(splitHostToolContent('hello')).toEqual({
      rest: 'hello',
      tools: [],
    })
  })

  it('splits search and fetch fences from the answer', () => {
    const content = `<<<host_web_search>>>
[WebSearch] query="cats"
1. Cat
   https://c
<<<end_host_web_search>>>
<<<host_web_fetch>>>
[WebFetch] url="https://c"
body
<<<end_host_web_fetch>>>
Final answer.`
    expect(splitHostToolContent(content)).toEqual({
      rest: 'Final answer.',
      tools: [
        {
          kind: 'search',
          body: '[WebSearch] query="cats"\n1. Cat\n   https://c',
          isStreaming: false,
        },
        {
          kind: 'fetch',
          body: '[WebFetch] url="https://c"\nbody',
          isStreaming: false,
        },
      ],
    })
  })

  it('treats an unclosed fence as streaming', () => {
    expect(
      splitHostToolContent('<<<host_web_search>>>\n[WebSearch] query="c"')
    ).toEqual({
      rest: '',
      tools: [
        {
          kind: 'search',
          body: '[WebSearch] query="c"',
          isStreaming: true,
        },
      ],
    })
  })

  it('keeps image recognition rest intact when mixed', () => {
    const content = `<<<host_web_search>>>
hits
<<<end_host_web_search>>>
The weather is fine.`
    expect(splitHostToolContent(content).rest).toBe('The weather is fine.')
  })
})

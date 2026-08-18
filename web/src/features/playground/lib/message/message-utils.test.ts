import { describe, expect, it } from 'vitest'

import { MESSAGE_ROLES } from '../../constants'
import { createUserMessage, formatMessageForAPI } from './message-utils'

describe('formatMessageForAPI', () => {
  it('sends pasted images as image_url parts', () => {
    const message = createUserMessage('what is this', Date.now(), [
      'data:image/png;base64,aaa',
    ])
    expect(formatMessageForAPI(message)).toEqual({
      role: MESSAGE_ROLES.USER,
      content: [
        { type: 'text', text: 'what is this' },
        {
          type: 'image_url',
          image_url: { url: 'data:image/png;base64,aaa' },
        },
      ],
    })
  })

  it('sends assistant reasoning so the next turn can see the thinking chain', () => {
    const message = {
      key: 'a1',
      from: MESSAGE_ROLES.ASSISTANT,
      versions: [{ id: 'v1', content: 'It is a cat.' }],
      reasoning: {
        content: '<<<image_recognition>>>\n[Image 1]\nA cat\n<<<end_image_recognition>>>',
        duration: 1,
      },
    }
    expect(formatMessageForAPI(message)).toEqual({
      role: MESSAGE_ROLES.ASSISTANT,
      content: 'It is a cat.',
      reasoning_content:
        '<<<image_recognition>>>\n[Image 1]\nA cat\n<<<end_image_recognition>>>',
    })
  })
})

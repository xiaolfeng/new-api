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
})

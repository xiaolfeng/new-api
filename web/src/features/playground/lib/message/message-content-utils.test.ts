import { describe, expect, it } from 'vitest'

import { MESSAGE_ROLES } from '../../constants'
import { getMessageContentState } from './message-content-utils'

describe('getMessageContentState', () => {
  it('folds image recognition fences into the thinking chain', () => {
    const state = getMessageContentState(
      {
        key: 'a1',
        from: MESSAGE_ROLES.ASSISTANT,
        versions: [{ id: 'v1', content: 'It is a cat.' }],
        reasoning: {
          content:
            '<<<image_recognition>>>\n[Image 1]\nA cat\n<<<end_image_recognition>>>\nLet me answer.',
          duration: 1,
        },
      },
      'It is a cat.'
    )

    expect(state.hasReasoning).toBe(true)
    if (state.hasReasoning) {
      expect(state.reasoningContent).toContain('[Image 1]\nA cat')
      expect(state.reasoningContent).toContain('Let me answer.')
      expect(state.reasoningContent).not.toContain('<<<image_recognition>>>')
    }
    expect(state.displayContent).toBe('It is a cat.')
  })

  it('moves a legacy content fence into thinking instead of a tool card', () => {
    const content = `<<<image_recognition>>>
[Image 1]
A cat
<<<end_image_recognition>>>
It is a cat.`
    const state = getMessageContentState(
      {
        key: 'a1',
        from: MESSAGE_ROLES.ASSISTANT,
        versions: [{ id: 'v1', content }],
      },
      content
    )

    expect(state.hasReasoning).toBe(true)
    if (state.hasReasoning) {
      expect(state.reasoningContent).toContain('[Image 1]\nA cat')
    }
    expect(state.displayContent).toBe('It is a cat.')
  })
})

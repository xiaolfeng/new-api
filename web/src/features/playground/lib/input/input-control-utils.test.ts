import { describe, expect, it } from 'vitest'

import {
  getImageUrlsFromFiles,
  getInputControlState,
  getSubmittablePlaygroundInput,
} from './input-control-utils'

describe('getImageUrlsFromFiles', () => {
  it('keeps image files and data urls', () => {
    expect(
      getImageUrlsFromFiles([
        { url: 'data:image/png;base64,aaa', mediaType: 'image/png' },
        { url: 'https://example.com/a.png', mediaType: 'image/png' },
        { url: 'https://example.com/a.txt', mediaType: 'text/plain' },
      ])
    ).toEqual([
      'data:image/png;base64,aaa',
      'https://example.com/a.png',
    ])
  })
})

describe('getSubmittablePlaygroundInput', () => {
  it('rejects empty text without images', () => {
    expect(getSubmittablePlaygroundInput({ text: '   ' })).toBeNull()
  })

  it('accepts image-only input', () => {
    expect(
      getSubmittablePlaygroundInput({
        text: '',
        files: [{ url: 'data:image/png;base64,aaa', mediaType: 'image/png' }],
      })
    ).toEqual({
      text: '',
      imageUrls: ['data:image/png;base64,aaa'],
    })
  })
})

describe('getInputControlState', () => {
  const base = {
    groups: [{ label: 'default', value: 'default', ratio: 1 }],
    hasStopHandler: false,
    models: [{ label: 'm', value: 'm' }],
    text: '',
  }

  it('allows submit when only attachments exist', () => {
    expect(
      getInputControlState({
        ...base,
        hasAttachments: true,
      }).canSubmit
    ).toBe(true)
  })

  it('blocks submit when both text and attachments are empty', () => {
    expect(getInputControlState(base).canSubmit).toBe(false)
  })
})

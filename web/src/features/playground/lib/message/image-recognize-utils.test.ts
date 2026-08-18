import { describe, expect, it } from 'vitest'

import { splitImageRecognitionContent } from './image-recognize-utils'

describe('splitImageRecognitionContent', () => {
  it('returns original text when there is no fence', () => {
    expect(splitImageRecognitionContent('hello')).toEqual({
      rest: 'hello',
      isStreaming: false,
    })
  })

  it('splits a completed recognition block', () => {
    const content = `<<<image_recognition>>>
[Image 1]
A cat
<<<end_image_recognition>>>
The cat is sitting.`
    expect(splitImageRecognitionContent(content)).toEqual({
      recognition: '[Image 1]\nA cat',
      rest: 'The cat is sitting.',
      isStreaming: false,
    })
  })

  it('treats an unclosed fence as streaming', () => {
    expect(
      splitImageRecognitionContent('<<<image_recognition>>>\n[Image 1]\nA')
    ).toEqual({
      recognition: '[Image 1]\nA',
      rest: '',
      isStreaming: true,
    })
  })
})

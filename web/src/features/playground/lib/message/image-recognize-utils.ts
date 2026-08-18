/*
Copyright (C) 2023-2026 QuantumNous

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as
published by the Free Software Foundation, either version 3 of the
License, or (at your option) any later version.

This program is distributed in the hope that it will be useful,
but WITHOUT ANY WARRANTY; without even the implied warranty of
MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
GNU Affero General Public License for more details.

You should have received a copy of the GNU Affero General Public License
along with this program. If not, see <https://www.gnu.org/licenses/>.

For commercial licensing, please contact support@quantumnous.com
*/
export const IMAGE_RECOGNITION_FENCE_START = '<<<image_recognition>>>'
export const IMAGE_RECOGNITION_FENCE_END = '<<<end_image_recognition>>>'

export type ImageRecognitionSplit = {
  recognition?: string
  rest: string
  isStreaming: boolean
}

export function splitImageRecognitionContent(
  content: string
): ImageRecognitionSplit {
  const startIdx = content.indexOf(IMAGE_RECOGNITION_FENCE_START)
  if (startIdx === -1) {
    return { rest: content, isStreaming: false }
  }

  const afterStart = startIdx + IMAGE_RECOGNITION_FENCE_START.length
  const endIdx = content.indexOf(IMAGE_RECOGNITION_FENCE_END, afterStart)
  if (endIdx === -1) {
    return {
      recognition: content.slice(afterStart).trim(),
      rest: content.slice(0, startIdx).trim(),
      isStreaming: true,
    }
  }

  return {
    recognition: content.slice(afterStart, endIdx).trim(),
    rest: (
      content.slice(0, startIdx) +
      content.slice(endIdx + IMAGE_RECOGNITION_FENCE_END.length)
    ).trim(),
    isStreaming: false,
  }
}

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
export const HOST_WEB_SEARCH_FENCE_START = '<<<host_web_search>>>'
export const HOST_WEB_SEARCH_FENCE_END = '<<<end_host_web_search>>>'
export const HOST_WEB_FETCH_FENCE_START = '<<<host_web_fetch>>>'
export const HOST_WEB_FETCH_FENCE_END = '<<<end_host_web_fetch>>>'

export type HostToolKind = 'search' | 'fetch'

export type HostToolSplitItem = {
  body: string
  isStreaming: boolean
  kind: HostToolKind
}

export type HostToolSplit = {
  rest: string
  tools: HostToolSplitItem[]
}

const FENCES: { end: string; kind: HostToolKind; start: string }[] = [
  {
    kind: 'search',
    start: HOST_WEB_SEARCH_FENCE_START,
    end: HOST_WEB_SEARCH_FENCE_END,
  },
  {
    kind: 'fetch',
    start: HOST_WEB_FETCH_FENCE_START,
    end: HOST_WEB_FETCH_FENCE_END,
  },
]

export function splitHostToolContent(content: string): HostToolSplit {
  if (!content) {
    return { rest: content, tools: [] }
  }

  const tools: HostToolSplitItem[] = []
  let rest = content

  for (const fence of FENCES) {
    while (rest.includes(fence.start)) {
      const startIdx = rest.indexOf(fence.start)
      const afterStart = startIdx + fence.start.length
      const endIdx = rest.indexOf(fence.end, afterStart)
      const before = rest.slice(0, startIdx)
      if (endIdx === -1) {
        tools.push({
          kind: fence.kind,
          body: rest.slice(afterStart).trim(),
          isStreaming: true,
        })
        rest = before
        break
      }
      tools.push({
        kind: fence.kind,
        body: rest.slice(afterStart, endIdx).trim(),
        isStreaming: false,
      })
      rest = before + rest.slice(endIdx + fence.end.length)
    }
  }

  return { rest: rest.trim(), tools }
}

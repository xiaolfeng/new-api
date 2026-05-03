import { useMemo } from 'react'
import { cn } from '@/lib/utils'

interface MarkdownSourceHighlighterProps {
  content: string
  fontSize?: number
  className?: string
}

/**
 * Token-level inline highlighting — priority order:
 * 1. Inline code `code`
 * 2. Bold **text** or __text__
 * 3. Strikethrough ~~text~~
 * 4. Links [text](url)
 * 5. Images ![alt](url)
 * 6. Italic *text* or _text_
 */
function highlightSegment(text: string, key: string): React.ReactNode[] {
  const elements: React.ReactNode[] = []
  let remaining = text
  let elementKey = 0

  while (remaining.length > 0) {
    // Inline code `code`
    const inlineCodeMatch = remaining.match(/^`([^`]+)`/)
    if (inlineCodeMatch) {
      elements.push(
        <code
          key={`${key}-code-${elementKey++}`}
          className="rounded bg-orange-500/15 px-1 py-px font-mono text-[0.9em] text-orange-600"
        >
          {inlineCodeMatch[0]}
        </code>
      )
      remaining = remaining.slice(inlineCodeMatch[0].length)
      continue
    }

    // Bold **text** or __text__
    const boldMatch = remaining.match(/^(\*\*|__)(.+?)\1/)
    if (boldMatch) {
      elements.push(
        <span key={`${key}-bold-${elementKey++}`}>
          <span className="text-gray-500">{boldMatch[1]}</span>
          <strong className="text-foreground">{boldMatch[2]}</strong>
          <span className="text-gray-500">{boldMatch[1]}</span>
        </span>
      )
      remaining = remaining.slice(boldMatch[0].length)
      continue
    }

    // Strikethrough ~~text~~
    const strikeMatch = remaining.match(/^~~(.+?)~~/)
    if (strikeMatch) {
      elements.push(
        <span key={`${key}-strike-${elementKey++}`}>
          <span className="text-gray-500">~~</span>
          <del className="text-foreground line-through">{strikeMatch[1]}</del>
          <span className="text-gray-500">~~</span>
        </span>
      )
      remaining = remaining.slice(strikeMatch[0].length)
      continue
    }

    // Links [text](url)
    const linkMatch = remaining.match(/^\[([^\]]+)\]\(([^)]+)\)/)
    if (linkMatch) {
      elements.push(
        <span key={`${key}-link-${elementKey++}`}>
          <span className="text-gray-500">[</span>
          <span className="text-blue-500 underline">{linkMatch[1]}</span>
          <span className="text-gray-500">](</span>
          <span className="text-gray-500">{linkMatch[2]}</span>
          <span className="text-gray-500">)</span>
        </span>
      )
      remaining = remaining.slice(linkMatch[0].length)
      continue
    }

    // Images ![alt](url)
    const imgMatch = remaining.match(/^!\[([^\]]*)\]\(([^)]+)\)/)
    if (imgMatch) {
      elements.push(
        <span key={`${key}-img-${elementKey++}`}>
          <span className="text-gray-500">![</span>
          <span className="text-blue-500">{imgMatch[1] || 'image'}</span>
          <span className="text-gray-500">](</span>
          <span className="text-gray-500">{imgMatch[2]}</span>
          <span className="text-gray-500">)</span>
        </span>
      )
      remaining = remaining.slice(imgMatch[0].length)
      continue
    }

    // Italic *text* or _text_ (avoid matching bold)
    const italicMatch = remaining.match(/^(\*|_)(.+?)\1(?!\1)/)
    if (italicMatch && !italicMatch[2].startsWith(italicMatch[1])) {
      elements.push(
        <span key={`${key}-italic-${elementKey++}`}>
          <span className="text-gray-500">{italicMatch[1]}</span>
          <em className="text-foreground italic">{italicMatch[2]}</em>
          <span className="text-gray-500">{italicMatch[1]}</span>
        </span>
      )
      remaining = remaining.slice(italicMatch[0].length)
      continue
    }

    // No match — advance to next special character or consume rest
    const nextSpecial = remaining.search(/[`*_~\[\]!]/)
    if (nextSpecial === -1) {
      elements.push(
        <span key={`${key}-text-${elementKey++}`}>{remaining}</span>
      )
      break
    } else if (nextSpecial === 0) {
      elements.push(
        <span key={`${key}-char-${elementKey++}`}>{remaining[0]}</span>
      )
      remaining = remaining.slice(1)
    } else {
      elements.push(
        <span key={`${key}-text-${elementKey++}`}>
          {remaining.slice(0, nextSpecial)}
        </span>
      )
      remaining = remaining.slice(nextSpecial)
    }
  }

  return elements
}

/**
 * Line-level block detection for headings, lists, blockquotes, hr, etc.
 */
function highlightLine(line: string, lineIndex: number): React.ReactNode[] {
  const elements: React.ReactNode[] = []

  // Heading (# to ######)
  const headingMatch = line.match(/^(#{1,6})\s+/)
  if (headingMatch) {
    elements.push(
      <span key={`heading-mark-${lineIndex}`} className="text-purple-500">
        {headingMatch[1]}
      </span>
    )
    elements.push(<span key={`heading-space-${lineIndex}`}> </span>)
    elements.push(
      <strong key={`heading-text-${lineIndex}`} className="text-foreground font-bold">
        {highlightSegment(line.slice(headingMatch[0].length), `h${headingMatch[1].length}-${lineIndex}`)}
      </strong>
    )
    return elements
  }

  // Unordered list (-, *, +)
  const ulMatch = line.match(/^(\s*)([-*+])\s+/)
  if (ulMatch) {
    elements.push(<span key={`ul-indent-${lineIndex}`}>{ulMatch[1]}</span>)
    elements.push(
      <span key={`ul-mark-${lineIndex}`} className="text-purple-500">
        {ulMatch[2]}
      </span>
    )
    elements.push(<span key={`ul-space-${lineIndex}`}> </span>)
    elements.push(...highlightSegment(line.slice(ulMatch[0].length), `ul-${lineIndex}`))
    return elements
  }

  // Ordered list (1.)
  const olMatch = line.match(/^(\s*)(\d+)\.\s+/)
  if (olMatch) {
    elements.push(<span key={`ol-indent-${lineIndex}`}>{olMatch[1]}</span>)
    elements.push(
      <span key={`ol-num-${lineIndex}`} className="text-purple-500">
        {olMatch[2]}
      </span>
    )
    elements.push(
      <span key={`ol-dot-${lineIndex}`} className="text-purple-500">
        .
      </span>
    )
    elements.push(<span key={`ol-space-${lineIndex}`}> </span>)
    elements.push(...highlightSegment(line.slice(olMatch[0].length), `ol-${lineIndex}`))
    return elements
  }

  // Blockquote (>)
  const quoteMatch = line.match(/^(\s*)(>)\s?/)
  if (quoteMatch) {
    elements.push(<span key={`quote-indent-${lineIndex}`}>{quoteMatch[1]}</span>)
    elements.push(
      <span key={`quote-mark-${lineIndex}`} className="text-purple-500">
        {quoteMatch[2]}
      </span>
    )
    if (quoteMatch[0].endsWith(' ')) {
      elements.push(<span key={`quote-space-${lineIndex}`}> </span>)
    }
    elements.push(
      <span key={`quote-text-${lineIndex}`} className="text-gray-500 italic">
        {highlightSegment(line.slice(quoteMatch[0].length), `quote-${lineIndex}`)}
      </span>
    )
    return elements
  }

  // Horizontal rule (---, ***, ___)
  const hrMatch = line.match(/^(\*{3,}|-{3,}|_{3,})$/)
  if (hrMatch) {
    elements.push(
      <span key={`hr-${lineIndex}`} className="text-gray-500">
        {hrMatch[1]}
      </span>
    )
    return elements
  }

  // Regular text — inline highlighting only
  return highlightSegment(line, `line-${lineIndex}`)
}

/**
 * Markdown source syntax highlighter.
 * Highlights the RAW markdown source text — does NOT render to HTML.
 */
export function MarkdownSourceHighlighter({
  content,
  fontSize = 13,
  className,
}: MarkdownSourceHighlighterProps) {
  const highlighted = useMemo(() => {
    if (!content) return null

    const lines = content.split('\n')
    const result: React.ReactNode[] = []
    let inCodeBlock = false
    let codeBlockLang = ''
    let codeBlockLines: string[] = []
    let codeBlockStartLine = 0

    for (let lineNum = 0; lineNum < lines.length; lineNum++) {
      const line = lines[lineNum]

      // Code block fence start/end
      const codeBlockMatch = line.match(/^```(\w*)$/)
      if (codeBlockMatch) {
        if (!inCodeBlock) {
          inCodeBlock = true
          codeBlockLang = codeBlockMatch[1] || ''
          codeBlockLines = []
          codeBlockStartLine = lineNum

          result.push(
            <div key={`cb-open-${lineNum}`}>
              <span className="text-gray-500">```</span>
              <span className="ml-1 text-orange-600">{codeBlockLang}</span>
            </div>
          )
        } else {
          inCodeBlock = false

          // Render code block content
          for (let ci = 0; ci < codeBlockLines.length; ci++) {
            result.push(
              <div key={`cb${codeBlockStartLine}-cl${ci}`} className="font-mono">
                {codeBlockLines[ci] || '\u00A0'}
              </div>
            )
          }

          // Closing fence
          result.push(
            <div key={`cb${codeBlockStartLine}-end${lineNum}`}>
              <span className="text-gray-500">```</span>
            </div>
          )

          codeBlockLang = ''
          codeBlockLines = []
        }
        continue
      }

      if (inCodeBlock) {
        codeBlockLines.push(line)
        continue
      }

      result.push(
        <div key={`mdl-${lineNum}`}>
          {highlightLine(line, lineNum)}
        </div>
      )
    }

    // Handle unclosed code blocks
    if (inCodeBlock && codeBlockLines.length > 0) {
      for (let ci = 0; ci < codeBlockLines.length; ci++) {
        result.push(
          <div key={`ucb${codeBlockStartLine}-cl${ci}`} className="font-mono">
            {codeBlockLines[ci] || '\u00A0'}
          </div>
        )
      }
    }

    return result
  }, [content])

  return (
    <div
      className={cn(
        'font-[inherit] whitespace-pre-wrap break-words leading-relaxed',
        className
      )}
      style={{ fontSize }}
    >
      {highlighted}
    </div>
  )
}

/*
Copyright (C) 2025 QuantumNous

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

import React, { useMemo } from 'react';

// 颜色配置
const COLORS = {
  heading: '#a855f7', // 紫色 - 标题符号
  headingText: 'var(--semi-color-text-0)', // 标题文字
  bold: 'inherit', // 粗体文字
  italic: 'inherit', // 斜体文字
  marker: '#6b7280', // 灰色 - 语法标记
  code: '#ea580c', // 橙色 - 内联代码
  codeBlock: '#1f2937', // 代码块背景
  link: '#3b82f6', // 蓝色 - 链接文字
  url: '#6b7280', // 灰色 - URL
  list: '#a855f7', // 紫色 - 列表符号
  quote: '#a855f7', // 紫色 - 引用符号
  quoteText: '#6b7280', // 灰色 - 引用文字
  hr: '#6b7280', // 灰色 - 分隔线
  strikethrough: 'inherit', // 删除线文字
};

/**
 * 高亮单个文本片段
 */
function highlightSegment(text, key) {
  const elements = [];
  let remaining = text;
  let elementKey = 0;

  // 匹配优先级：
  // 1. 内联代码 `code`
  // 2. 粗体 **text** 或 __text__
  // 3. 斜体 *text* 或 _text_（需要避免匹配粗体）
  // 4. 链接 [text](url)
  // 5. 删除线 ~~text~~

  while (remaining.length > 0) {
    let matched = false;

    // 内联代码 `code`
    const inlineCodeMatch = remaining.match(/^`([^`]+)`/);
    if (inlineCodeMatch) {
      elements.push(
        <code
          key={`${key}-code-${elementKey++}`}
          style={{
            backgroundColor: 'rgba(234, 88, 12, 0.15)',
            color: COLORS.code,
            padding: '1px 4px',
            borderRadius: '3px',
            fontFamily: 'monospace',
            fontSize: '0.9em',
          }}
        >
          {inlineCodeMatch[0]}
        </code>
      );
      remaining = remaining.slice(inlineCodeMatch[0].length);
      matched = true;
      continue;
    }

    // 粗体 **text** 或 __text__
    const boldMatch = remaining.match(/^(\*\*|__)(.+?)\1/);
    if (boldMatch) {
      elements.push(
        <span key={`${key}-bold-${elementKey++}`}>
          <span style={{ color: COLORS.marker }}>{boldMatch[1]}</span>
          <strong>{boldMatch[2]}</strong>
          <span style={{ color: COLORS.marker }}>{boldMatch[1]}</span>
        </span>
      );
      remaining = remaining.slice(boldMatch[0].length);
      matched = true;
      continue;
    }

    // 删除线 ~~text~~
    const strikeMatch = remaining.match(/^~~(.+?)~~/);
    if (strikeMatch) {
      elements.push(
        <span key={`${key}-strike-${elementKey++}`}>
          <span style={{ color: COLORS.marker }}>~~</span>
          <del>{strikeMatch[1]}</del>
          <span style={{ color: COLORS.marker }}>~~</span>
        </span>
      );
      remaining = remaining.slice(strikeMatch[0].length);
      matched = true;
      continue;
    }

    // 链接 [text](url)
    const linkMatch = remaining.match(/^\[([^\]]+)\]\(([^)]+)\)/);
    if (linkMatch) {
      elements.push(
        <span key={`${key}-link-${elementKey++}`}>
          <span style={{ color: COLORS.marker }}>[</span>
          <span style={{ color: COLORS.link, textDecoration: 'underline' }}>{linkMatch[1]}</span>
          <span style={{ color: COLORS.marker }}>](</span>
          <span style={{ color: COLORS.url }}>{linkMatch[2]}</span>
          <span style={{ color: COLORS.marker }}>)</span>
        </span>
      );
      remaining = remaining.slice(linkMatch[0].length);
      matched = true;
      continue;
    }

    // 图片 ![alt](url)
    const imgMatch = remaining.match(/^!\[([^\]]*)\]\(([^)]+)\)/);
    if (imgMatch) {
      elements.push(
        <span key={`${key}-img-${elementKey++}`}>
          <span style={{ color: COLORS.marker }}>![</span>
          <span style={{ color: COLORS.link }}>{imgMatch[1] || 'image'}</span>
          <span style={{ color: COLORS.marker }}>](</span>
          <span style={{ color: COLORS.url }}>{imgMatch[2]}</span>
          <span style={{ color: COLORS.marker }}>)</span>
        </span>
      );
      remaining = remaining.slice(imgMatch[0].length);
      matched = true;
      continue;
    }

    // 斜体 *text* 或 _text_（排除已被粗体匹配的情况）
    const italicMatch = remaining.match(/^(\*|_)(.+?)\1(?!\1)/);
    if (italicMatch && !italicMatch[2].startsWith(italicMatch[1])) {
      elements.push(
        <span key={`${key}-italic-${elementKey++}`}>
          <span style={{ color: COLORS.marker }}>{italicMatch[1]}</span>
          <em>{italicMatch[2]}</em>
          <span style={{ color: COLORS.marker }}>{italicMatch[1]}</span>
        </span>
      );
      remaining = remaining.slice(italicMatch[0].length);
      matched = true;
      continue;
    }

    // 如果没有匹配，取下一个字符
    if (!matched) {
      // 找到下一个可能匹配的位置
      const nextSpecial = remaining.search(/[`*_~\[\]!]/);
      if (nextSpecial === -1) {
        elements.push(
          <span key={`${key}-text-${elementKey++}`}>{remaining}</span>
        );
        break;
      } else if (nextSpecial === 0) {
        // 第一个字符就是特殊字符但没有匹配，直接输出
        elements.push(
          <span key={`${key}-char-${elementKey++}`}>{remaining[0]}</span>
        );
        remaining = remaining.slice(1);
      } else {
        elements.push(
          <span key={`${key}-text-${elementKey++}`}>{remaining.slice(0, nextSpecial)}</span>
        );
        remaining = remaining.slice(nextSpecial);
      }
    }
  }

  return elements;
}

/**
 * 高亮单行文本
 */
function highlightLine(line, lineIndex) {
  const elements = [];
  let remaining = line;
  let elementKey = 0;

  // 检查标题 (行首)
  const headingMatch = remaining.match(/^(#{1,6})\s+/);
  if (headingMatch) {
    const level = headingMatch[1].length;
    elements.push(
      <span key={`heading-mark-${lineIndex}`} style={{ color: COLORS.heading }}>
        {headingMatch[1]}
      </span>
    );
    elements.push(<span key={`heading-space-${lineIndex}`}> </span>);
    remaining = remaining.slice(headingMatch[0].length);

    // 标题文字加粗
    elements.push(
      <strong key={`heading-text-${lineIndex}`}>
        {highlightSegment(remaining, `h${level}-${lineIndex}`)}
      </strong>
    );
    return elements;
  }

  // 检查无序列表 (行首)
  const ulMatch = remaining.match(/^(\s*)([-*+])\s+/);
  if (ulMatch) {
    elements.push(<span key={`ul-indent-${lineIndex}`}>{ulMatch[1]}</span>);
    elements.push(
      <span key={`ul-mark-${lineIndex}`} style={{ color: COLORS.list }}>
        {ulMatch[2]}
      </span>
    );
    elements.push(<span key={`ul-space-${lineIndex}`}> </span>);
    remaining = remaining.slice(ulMatch[0].length);
    elements.push(...highlightSegment(remaining, `ul-${lineIndex}`));
    return elements;
  }

  // 检查有序列表 (行首)
  const olMatch = remaining.match(/^(\s*)(\d+)\.\s+/);
  if (olMatch) {
    elements.push(<span key={`ol-indent-${lineIndex}`}>{olMatch[1]}</span>);
    elements.push(
      <span key={`ol-num-${lineIndex}`} style={{ color: COLORS.list }}>
        {olMatch[2]}
      </span>
    );
    elements.push(<span key={`ol-dot-${lineIndex}`} style={{ color: COLORS.list }}>.</span>);
    elements.push(<span key={`ol-space-${lineIndex}`}> </span>);
    remaining = remaining.slice(olMatch[0].length);
    elements.push(...highlightSegment(remaining, `ol-${lineIndex}`));
    return elements;
  }

  // 检查引用 (行首)
  const quoteMatch = remaining.match(/^(\s*)(>)\s?/);
  if (quoteMatch) {
    elements.push(<span key={`quote-indent-${lineIndex}`}>{quoteMatch[1]}</span>);
    elements.push(
      <span key={`quote-mark-${lineIndex}`} style={{ color: COLORS.quote }}>
        {quoteMatch[2]}
      </span>
    );
    if (quoteMatch[0].endsWith(' ')) {
      elements.push(<span key={`quote-space-${lineIndex}`}> </span>);
    }
    remaining = remaining.slice(quoteMatch[0].length);
    elements.push(
      <span key={`quote-text-${lineIndex}`} style={{ color: COLORS.quoteText, fontStyle: 'italic' }}>
        {highlightSegment(remaining, `quote-${lineIndex}`)}
      </span>
    );
    return elements;
  }

  // 检查分隔线 (单独一行)
  const hrMatch = remaining.match(/^(\*{3,}|-{3,}|_{3,})$/);
  if (hrMatch) {
    elements.push(
      <span key={`hr-${lineIndex}`} style={{ color: COLORS.hr }}>
        {hrMatch[1]}
      </span>
    );
    return elements;
  }

  // 普通文本，处理内联元素
  return highlightSegment(remaining, `line-${lineIndex}`);
}

/**
 * Markdown 源码语法高亮组件
 * 不渲染为 HTML，仅对语法元素添加颜色
 */
export function MarkdownSourceHighlighter({ content, fontSize = 13, style }) {
  const highlighted = useMemo(() => {
    if (!content) return null;

    const lines = content.split('\n');
    const result = [];
    let inCodeBlock = false;
    let codeBlockLang = '';
    let codeBlockLines = [];
    let codeBlockStartLine = 0;

    lines.forEach((line, lineIndex) => {
      // 检查代码块开始/结束
      const codeBlockMatch = line.match(/^```(\w*)$/);
      if (codeBlockMatch) {
        if (!inCodeBlock) {
          // 开始代码块
          inCodeBlock = true;
          codeBlockLang = codeBlockMatch[1] || '';
          codeBlockLines = [];
          codeBlockStartLine = lineIndex;

          // 渲染代码块开始标记
          result.push(
            <div key={`line-${lineIndex}`}>
              <span style={{ color: COLORS.marker }}>```</span>
              <span style={{ color: COLORS.code, marginLeft: 4 }}>{codeBlockLang}</span>
            </div>
          );
        } else {
          // 结束代码块
          inCodeBlock = false;

          // 渲染代码块内容
          codeBlockLines.forEach((codeLine, idx) => {
            result.push(
              <div key={`code-${codeBlockStartLine}-${idx}`} style={{ fontFamily: 'monospace' }}>
                {codeLine || '\u00A0'}
              </div>
            );
          });

          // 渲染代码块结束标记
          result.push(
            <div key={`line-${lineIndex}-end`}>
              <span style={{ color: COLORS.marker }}>```</span>
            </div>
          );

          codeBlockLang = '';
          codeBlockLines = [];
        }
        return;
      }

      if (inCodeBlock) {
        // 在代码块内，收集行
        codeBlockLines.push(line);
        return;
      }

      // 普通行，进行高亮
      result.push(
        <div key={`line-${lineIndex}`}>
          {highlightLine(line, lineIndex)}
        </div>
      );
    });

    // 如果代码块未闭合，输出剩余内容
    if (inCodeBlock && codeBlockLines.length > 0) {
      codeBlockLines.forEach((codeLine, idx) => {
        result.push(
          <div key={`code-unclosed-${idx}`} style={{ fontFamily: 'monospace' }}>
            {codeLine || '\u00A0'}
          </div>
        );
      });
    }

    return result;
  }, [content]);

  return (
    <div
      style={{
        fontSize,
        lineHeight: 1.6,
        fontFamily: 'inherit',
        whiteSpace: 'pre-wrap',
        wordBreak: 'break-word',
        margin: 0,
        ...style,
      }}
    >
      {highlighted}
    </div>
  );
}

export default MarkdownSourceHighlighter;

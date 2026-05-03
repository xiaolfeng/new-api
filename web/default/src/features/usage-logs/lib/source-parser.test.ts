import { describe, it, expect } from 'bun:test'
import { parseClientSource, getSourceColor } from './source-parser'

describe('parseClientSource', () => {
  it('returns dash for empty input', () => {
    expect(parseClientSource('').name).toBe('-')
  })

  it('returns dash for null-ish input', () => {
    expect(parseClientSource(null as unknown as string).name).toBe('-')
  })

  // AI coding assistants
  it('detects Claude Code', () => {
    expect(parseClientSource('Claude-CLI/1.0').name).toBe('Claude Code')
  })
  it('detects Cursor', () => {
    expect(parseClientSource('Cursor/0.45.0').name).toBe('Cursor')
  })
  it('detects Copilot', () => {
    expect(parseClientSource('github-copilot/1.0').name).toBe('Copilot')
  })
  it('detects Cline', () => {
    expect(parseClientSource('cline/3.0.0').name).toBe('Cline')
  })
  it('detects Windsurf', () => {
    expect(parseClientSource('windsurf/1.0').name).toBe('Windsurf')
  })
  it('detects Continue', () => {
    expect(parseClientSource('continue/0.8.0').name).toBe('Continue')
  })
  it('detects Aider via litellm', () => {
    expect(parseClientSource('litellm/1.0').name).toBe('Aider')
  })
  it('detects Codex', () => {
    expect(parseClientSource('codex_cli_rs/0.1').name).toBe('Codex')
  })
  it('detects Cherry Studio', () => {
    expect(parseClientSource('cherrystudio/1.0').name).toBe('Cherry Studio')
  })
  it('detects Roo Code', () => {
    expect(parseClientSource('roocode/1.0').name).toBe('Roo Code')
  })
  it('detects OpenCode', () => {
    expect(parseClientSource('opencode/1.0').name).toBe('OpenCode')
  })
  it('detects Amazon Q', () => {
    expect(parseClientSource('amazon-q/1.0').name).toBe('Amazon Q')
  })
  it('detects Tabnine', () => {
    expect(parseClientSource('tabnine/1.0').name).toBe('Tabnine')
  })
  it('detects Codeium via Windsurf alias', () => {
    expect(parseClientSource('codeium/1.0').name).toBe('Windsurf')
  })
  it('detects Cody', () => {
    expect(parseClientSource('cody/1.0').name).toBe('Cody')
  })
  it('detects Supermaven', () => {
    expect(parseClientSource('supermaven/1.0').name).toBe('Supermaven')
  })
  it('detects Goose', () => {
    expect(parseClientSource('goose/1.0').name).toBe('Goose')
  })
  it('detects Augment', () => {
    expect(parseClientSource('augment/1.0').name).toBe('Augment')
  })

  // AI chat clients
  it('detects Perplexity', () => {
    expect(parseClientSource('perplexity-user/1.0').name).toBe('Perplexity')
  })
  it('detects Mistral', () => {
    expect(parseClientSource('mistralai-user/1.0').name).toBe('Mistral')
  })
  it('detects Poe', () => {
    expect(parseClientSource('poe/1.0').name).toBe('Poe')
  })

  // Other AI tools
  it('detects LangChain', () => {
    expect(parseClientSource('langchain/1.0').name).toBe('LangChain')
  })
  it('detects OpenAI API', () => {
    expect(parseClientSource('openai/1.0').name).toBe('OpenAI API')
  })
  it('detects Anthropic API', () => {
    expect(parseClientSource('anthropic/1.0').name).toBe('Anthropic API')
  })

  // API testing tools
  it('detects Postman', () => {
    expect(parseClientSource('postmanruntime/1.0').name).toBe('Postman')
  })
  it('detects Insomnia', () => {
    expect(parseClientSource('insomnia/1.0').name).toBe('Insomnia')
  })

  // CLI tools
  it('detects cURL', () => {
    expect(parseClientSource('curl/8.1.0').name).toBe('cURL')
  })
  it('detects Wget', () => {
    expect(parseClientSource('wget/1.21').name).toBe('Wget')
  })
  it('detects Python', () => {
    expect(parseClientSource('python-requests/2.31.0').name).toBe('Python')
  })
  it('detects Go', () => {
    expect(parseClientSource('go-http-client/1.1').name).toBe('Go')
  })
  it('detects Node.js', () => {
    expect(parseClientSource('node-fetch/1.0').name).toBe('Node.js')
  })
  it('detects Java', () => {
    expect(parseClientSource('java/11.0').name).toBe('Java')
  })

  // Browsers (fallback)
  it('detects Chrome', () => {
    expect(parseClientSource('Mozilla/5.0 Chrome/120.0').name).toBe('Chrome')
  })
  it('detects Firefox', () => {
    expect(parseClientSource('Mozilla/5.0 Firefox/120.0').name).toBe('Firefox')
  })
  it('detects Edge', () => {
    expect(parseClientSource('Mozilla/5.0 Edg/120.0').name).toBe('Edge')
  })

  // Generic extraction
  it('extracts name from unknown tool/version pattern', () => {
    expect(parseClientSource('mytool/1.0').name).toBe('Mytool')
  })

  // Returns color
  it('returns a color string', () => {
    const result = parseClientSource('cursor/1.0')
    expect(result.color).toBeTruthy()
    expect(typeof result.color).toBe('string')
  })
})

describe('getSourceColor', () => {
  it('returns gray for empty input', () => {
    expect(getSourceColor('')).toBe('gray')
  })
  it('returns gray for dash', () => {
    expect(getSourceColor('-')).toBe('gray')
  })
  it('returns consistent color for same input', () => {
    expect(getSourceColor('Cursor')).toBe(getSourceColor('Cursor'))
  })
  it('returns valid color strings', () => {
    const c1 = getSourceColor('Cursor')
    const c2 = getSourceColor('Claude Code')
    expect(typeof c1).toBe('string')
    expect(typeof c2).toBe('string')
    expect(c1.length).toBeGreaterThan(0)
    expect(c2.length).toBeGreaterThan(0)
  })
})

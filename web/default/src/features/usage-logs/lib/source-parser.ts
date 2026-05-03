const COLORS = [
  'amber',
  'blue',
  'cyan',
  'green',
  'gray',
  'indigo',
  'sky',
  'lime',
  'orange',
  'pink',
  'purple',
  'red',
  'teal',
  'violet',
  'yellow',
]

export interface ParsedSource {
  name: string
  color: string
}

export function parseClientSource(userAgent: string): ParsedSource {
  if (!userAgent) return { name: '-', color: 'gray' }

  const ua = userAgent.toLowerCase()

  // AI coding assistants
  if (ua.includes('claude-cli')) return { name: 'Claude Code', color: getSourceColor('Claude Code') }
  if (ua.includes('codex_cli_rs') || ua.includes('codex-cli-rs')) return { name: 'Codex', color: getSourceColor('Codex') }
  if (ua.includes('cherrystudio/')) return { name: 'Cherry Studio', color: getSourceColor('Cherry Studio') }
  if (ua.includes('cursor/')) return { name: 'Cursor', color: getSourceColor('Cursor') }
  if (ua.includes('windsurf/') || ua.includes('codeium/')) return { name: 'Windsurf', color: getSourceColor('Windsurf') }
  if (ua.includes('continue/')) return { name: 'Continue', color: getSourceColor('Continue') }
  if (ua.includes('github-copilot') || ua.includes('copilot/')) return { name: 'Copilot', color: getSourceColor('Copilot') }
  if (ua.includes('cline/') || ua.includes('cline-vscode')) return { name: 'Cline', color: getSourceColor('Cline') }
  if (ua.includes('roo-cline') || ua.includes('roocode') || ua.includes('roo code')) return { name: 'Roo Code', color: getSourceColor('Roo Code') }
  if (ua.includes('opencode/') || ua.includes('crush/')) return { name: 'OpenCode', color: getSourceColor('OpenCode') }
  if (ua.includes('aider/') || ua.includes('litellm/')) return { name: 'Aider', color: getSourceColor('Aider') }
  if (ua.includes('amazon-q') || ua.includes('amazonq') || ua.includes('q-developer')) return { name: 'Amazon Q', color: getSourceColor('Amazon Q') }
  if (ua.includes('tabnine/')) return { name: 'Tabnine', color: getSourceColor('Tabnine') }
  if (ua.includes('codeium')) return { name: 'Codeium', color: getSourceColor('Codeium') }
  if (ua.includes('cody/') || ua.includes('sourcegraph')) return { name: 'Cody', color: getSourceColor('Cody') }
  if (ua.includes('supermaven/')) return { name: 'Supermaven', color: getSourceColor('Supermaven') }
  if (ua.includes('goose/') || ua.includes('block-goose')) return { name: 'Goose', color: getSourceColor('Goose') }
  if (ua.includes('augment/') || ua.includes('augmentcode')) return { name: 'Augment', color: getSourceColor('Augment') }

  // AI chat clients
  if (ua.includes('perplexity-user') || ua.includes('perplexity/')) return { name: 'Perplexity', color: getSourceColor('Perplexity') }
  if (ua.includes('mistralai-user') || ua.includes('mistral/')) return { name: 'Mistral', color: getSourceColor('Mistral') }
  if (ua.includes('poe/')) return { name: 'Poe', color: getSourceColor('Poe') }

  // Other AI tools
  if (ua.includes('langchain')) return { name: 'LangChain', color: getSourceColor('LangChain') }
  if (ua.includes('openai/') || ua.includes('openai-api')) return { name: 'OpenAI API', color: getSourceColor('OpenAI API') }
  if (ua.includes('anthropic/') || ua.includes('anthropic-api')) return { name: 'Anthropic API', color: getSourceColor('Anthropic API') }

  // API testing tools
  if (ua.includes('postmanruntime/')) return { name: 'Postman', color: getSourceColor('Postman') }
  if (ua.includes('insomnia/')) return { name: 'Insomnia', color: getSourceColor('Insomnia') }

  // CLI tools
  if (ua.includes('curl/')) return { name: 'cURL', color: getSourceColor('cURL') }
  if (ua.includes('wget/')) return { name: 'Wget', color: getSourceColor('Wget') }
  if (ua.includes('python-requests/') || ua.includes('python-urllib/')) return { name: 'Python', color: getSourceColor('Python') }
  if (ua.includes('go-http-client/') || ua.includes('go-resty/')) return { name: 'Go', color: getSourceColor('Go') }
  if (ua.includes('node-fetch/') || ua.includes('axios/')) return { name: 'Node.js', color: getSourceColor('Node.js') }
  if (ua.includes('java/')) return { name: 'Java', color: getSourceColor('Java') }

  // Browsers (fallback)
  if (ua.includes('firefox/')) return { name: 'Firefox', color: getSourceColor('Firefox') }
  if (ua.includes('edg/')) return { name: 'Edge', color: getSourceColor('Edge') }
  if (ua.includes('chrome/')) return { name: 'Chrome', color: getSourceColor('Chrome') }
  if (ua.includes('safari/') && !ua.includes('chrome')) return { name: 'Safari', color: getSourceColor('Safari') }

  // Generic extraction for name/version pattern
  const match = ua.match(/^([a-z0-9_-]+)\//)
  if (match) {
    const name = match[1]
    if (!['mozilla', 'applewebkit', 'khtml', 'gecko', 'like'].includes(name)) {
      const displayName = name.charAt(0).toUpperCase() + name.slice(1)
      return { name: displayName, color: getSourceColor(displayName) }
    }
  }

  return { name: '-', color: 'gray' }
}

export function getSourceColor(source: string): string {
  if (!source || source === '-') return 'gray'
  let hash = 0
  for (let i = 0; i < source.length; i++) {
    hash = source.charCodeAt(i) + ((hash << 5) - hash)
  }
  return COLORS[Math.abs(hash) % COLORS.length]
}

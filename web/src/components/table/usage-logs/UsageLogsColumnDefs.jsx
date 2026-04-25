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

import React from 'react';
import {
  Avatar,
  Space,
  Tag,
  Tooltip,
  Popover,
  Typography,
} from '@douyinfe/semi-ui';
import {
  renderGroup,
  renderQuota,
  stringToColor,
  getLogOther,
  renderModelTag,
  renderModelPriceSimple,
  renderTieredModelPriceSimple,
} from '../../../helpers';
import { IconHelpCircle } from '@douyinfe/semi-icons';
import { CircleAlert, Route, Sparkles } from 'lucide-react';

const colors = [
  'amber',
  'blue',
  'cyan',
  'green',
  'grey',
  'indigo',
  'light-blue',
  'lime',
  'orange',
  'pink',
  'purple',
  'red',
  'teal',
  'violet',
  'yellow',
];

function formatRatio(ratio) {
  if (ratio === undefined || ratio === null) {
    return '-';
  }
  if (typeof ratio === 'number') {
    return ratio.toFixed(4);
  }
  return String(ratio);
}

function buildChannelAffinityTooltip(affinity, t) {
  if (!affinity) {
    return null;
  }

  const keySource = affinity.key_source || '-';
  const keyPath = affinity.key_path || affinity.key_key || '-';
  const keyHint = affinity.key_hint || '';
  const keyFp = affinity.key_fp ? `#${affinity.key_fp}` : '';
  const keyText = `${keySource}:${keyPath}${keyFp}`;

  const lines = [
    t('渠道亲和性'),
    `${t('规则')}：${affinity.rule_name || '-'}`,
    `${t('分组')}：${affinity.selected_group || '-'}`,
    `${t('Key')}：${keyText}`,
    ...(keyHint ? [`${t('Key 摘要')}：${keyHint}`] : []),
  ];

  return (
    <div style={{ lineHeight: 1.6, display: 'flex', flexDirection: 'column' }}>
      {lines.map((line, i) => (
        <div key={i}>{line}</div>
      ))}
    </div>
  );
}

// Render functions
function renderType(type, t) {
  switch (type) {
    case 1:
      return (
        <Tag color='cyan' shape='circle'>
          {t('充值')}
        </Tag>
      );
    case 2:
      return (
        <Tag color='lime' shape='circle'>
          {t('消费')}
        </Tag>
      );
    case 3:
      return (
        <Tag color='orange' shape='circle'>
          {t('管理')}
        </Tag>
      );
    case 4:
      return (
        <Tag color='purple' shape='circle'>
          {t('系统')}
        </Tag>
      );
    case 5:
      return (
        <Tag color='red' shape='circle'>
          {t('错误')}
        </Tag>
      );
    case 6:
      return (
        <Tag color='teal' shape='circle'>
          {t('退款')}
        </Tag>
      );
    default:
      return (
        <Tag color='grey' shape='circle'>
          {t('未知')}
        </Tag>
      );
  }
}

function buildStreamStatusTooltip(ss, t) {
  if (!ss) return null;
  const lines = [
    t('流状态') + '：' + t('异常'),
    (ss.end_reason || 'unknown'),
  ];
  if (ss.error_count > 0) {
    lines.push(`${t('软错误')}: ${ss.error_count}`);
  }
  if (ss.end_error) {
    lines.push(ss.end_error);
  }
  return (
    <div style={{ lineHeight: 1.6, display: 'flex', flexDirection: 'column' }}>
      {lines.map((line, i) => (
        <div key={i}>{line}</div>
      ))}
    </div>
  );
}

function renderIsStream(bool, t, streamStatus) {
  const isError = streamStatus && streamStatus.status !== 'ok';

  if (bool) {
    return (
      <span style={{ position: 'relative', display: 'inline-block' }}>
        <Tag color='blue' shape='circle'>
          {t('流')}
        </Tag>
        {isError && (
          <Tooltip content={buildStreamStatusTooltip(streamStatus, t)}>
            <span
              style={{
                position: 'absolute',
                right: -4,
                top: -4,
                lineHeight: 1,
                color: '#ef4444',
                cursor: 'pointer',
                userSelect: 'none',
              }}
            >
              <CircleAlert
                size={14}
                strokeWidth={2.5}
                color='currentColor'
              />
            </span>
          </Tooltip>
        )}
      </span>
    );
  } else {
    return (
      <Tag color='purple' shape='circle'>
        {t('非流')}
      </Tag>
    );
  }
}

function renderUseTime(type, t) {
  const time = parseInt(type);
  if (time < 101) {
    return (
      <Tag color='green' shape='circle'>
        {' '}
        {time} s{' '}
      </Tag>
    );
  } else if (time < 300) {
    return (
      <Tag color='orange' shape='circle'>
        {' '}
        {time} s{' '}
      </Tag>
    );
  } else {
    return (
      <Tag color='red' shape='circle'>
        {' '}
        {time} s{' '}
      </Tag>
    );
  }
}

function renderFirstUseTime(type, t) {
  let time = parseFloat(type) / 1000.0;
  time = time.toFixed(1);
  if (time < 3) {
    return (
      <Tag color='green' shape='circle'>
        {' '}
        {time} s{' '}
      </Tag>
    );
  } else if (time < 10) {
    return (
      <Tag color='orange' shape='circle'>
        {' '}
        {time} s{' '}
      </Tag>
    );
  } else {
    return (
      <Tag color='red' shape='circle'>
        {' '}
        {time} s{' '}
      </Tag>
    );
  }
}

function renderTPS(tps, t) {
  if (!tps || tps <= 0) {
    return null;
  }
  const tpsValue = tps.toFixed(2);
  if (tps >= 81) {
    return (
      <Tag color='green' shape='circle'>
        {tpsValue} t/s
      </Tag>
    );
  } else if (tps >= 51) {
    return (
      <Tag color='lime' shape='circle'>
        {tpsValue} t/s
      </Tag>
    );
  } else if (tps >= 11) {
    return (
      <Tag color='yellow' shape='circle'>
        {tpsValue} t/s
      </Tag>
    );
  } else {
    return (
      <Tag color='red' shape='circle'>
        {tpsValue} t/s
      </Tag>
    );
  }
}

function renderBillingTag(record, t) {
  const other = getLogOther(record.other);
  if (other?.billing_source === 'subscription') {
    return (
      <Tag color='green' shape='circle'>
        {t('订阅抵扣')}
      </Tag>
    );
  }
  return null;
}

function renderModelName(record, copyText, t) {
  let other = getLogOther(record.other);
  let modelMapped =
    other?.is_model_mapped &&
    other?.upstream_model_name &&
    other?.upstream_model_name !== '';
  if (!modelMapped) {
    return renderModelTag(record.model_name, {
      onClick: (event) => {
        copyText(event, record.model_name).then((r) => {});
      },
    });
  } else {
    return (
      <>
        <Space vertical align={'start'}>
          <Popover
            content={
              <div style={{ padding: 10 }}>
                <Space vertical align={'start'}>
                  <div className='flex items-center'>
                    <Typography.Text strong style={{ marginRight: 8 }}>
                      {t('请求并计费模型')}:
                    </Typography.Text>
                    {renderModelTag(record.model_name, {
                      onClick: (event) => {
                        copyText(event, record.model_name).then((r) => {});
                      },
                    })}
                  </div>
                  <div className='flex items-center'>
                    <Typography.Text strong style={{ marginRight: 8 }}>
                      {t('实际模型')}:
                    </Typography.Text>
                    {renderModelTag(other.upstream_model_name, {
                      onClick: (event) => {
                        copyText(event, other.upstream_model_name).then(
                          (r) => {},
                        );
                      },
                    })}
                  </div>
                </Space>
              </div>
            }
          >
            {renderModelTag(record.model_name, {
              onClick: (event) => {
                copyText(event, record.model_name).then((r) => {});
              },
              suffixIcon: (
                <Route
                  style={{ width: '0.9em', height: '0.9em', opacity: 0.75 }}
                />
              ),
            })}
          </Popover>
        </Space>
      </>
    );
  }
}

function toTokenNumber(value) {
  const parsed = Number(value);
  if (!Number.isFinite(parsed) || parsed <= 0) {
    return 0;
  }
  return parsed;
}

function formatTokenCount(value) {
  return toTokenNumber(value).toLocaleString();
}

function getPromptCacheSummary(other) {
  if (!other || typeof other !== 'object') {
    return null;
  }

  const cacheReadTokens = toTokenNumber(other.cache_tokens);
  const cacheCreationTokens = toTokenNumber(other.cache_creation_tokens);
  const cacheCreationTokens5m = toTokenNumber(other.cache_creation_tokens_5m);
  const cacheCreationTokens1h = toTokenNumber(other.cache_creation_tokens_1h);

  const hasSplitCacheCreation =
    cacheCreationTokens5m > 0 || cacheCreationTokens1h > 0;
  const cacheWriteTokens = hasSplitCacheCreation
    ? cacheCreationTokens5m + cacheCreationTokens1h
    : cacheCreationTokens;

  if (cacheReadTokens <= 0 && cacheWriteTokens <= 0) {
    return null;
  }

  return {
    cacheReadTokens,
    cacheWriteTokens,
  };
}

function normalizeDetailText(detail) {
  return String(detail || '')
    .replace(/\n\r/g, '\n')
    .replace(/\r\n/g, '\n');
}

function getUsageLogGroupSummary(groupRatio, userGroupRatio, t) {
  const parsedUserGroupRatio = Number(userGroupRatio);
  const useUserGroupRatio =
    Number.isFinite(parsedUserGroupRatio) && parsedUserGroupRatio !== -1;
  const ratio = useUserGroupRatio ? userGroupRatio : groupRatio;
  if (ratio === undefined || ratio === null || ratio === '') {
    return '';
  }
  return `${useUserGroupRatio ? t('专属倍率') : t('分组')} ${formatRatio(ratio)}x`;
}

/**
 * 从 User-Agent 解析客户端来源
 * @param {string} userAgent - User-Agent 字符串
 * @returns {string} 来源名称
 */
function parseClientSource(userAgent) {
  if (!userAgent) return '-';

  const ua = userAgent.toLowerCase();

  // ===== AI 编程助手 =====
  // Claude Code CLI
  if (ua.includes('claude-cli')) {
    return 'Claude Code';
  }

  // OpenAI Codex CLI
  if (ua.includes('codex_cli_rs') || ua.includes('codex-cli-rs')) {
    return 'Codex';
  }

  // CherryStudio (需要在 Chrome 匹配之前)
  if (ua.includes('cherrystudio/')) {
    return 'Cherry Studio';
  }

  // Cursor IDE (Electron 应用，可能包含 cursor 关键字)
  if (ua.includes('cursor/')) {
    return 'Cursor';
  }

  // Windsurf/Codeium
  if (ua.includes('windsurf/') || ua.includes('codeium/')) {
    return 'Windsurf';
  }

  // Continue (VS Code Extension)
  if (ua.includes('continue/')) {
    return 'Continue';
  }

  // Copilot (GitHub Copilot)
  if (ua.includes('github-copilot') || ua.includes('copilot/')) {
    return 'Copilot';
  }

  // Cline (VS Code Extension) - 需要在通用 cline 匹配之前
  if (ua.includes('cline/') || ua.includes('cline-vscode')) {
    return 'Cline';
  }

  // Roo Code / Roo-Cline (VS Code Extension)
  if (ua.includes('roo-cline') || ua.includes('roocode') || ua.includes('roo code')) {
    return 'Roo Code';
  }

  // OpenCode / Crush (Terminal-based AI assistant)
  if (ua.includes('opencode/') || ua.includes('crush/')) {
    return 'OpenCode';
  }

  // Aider (Terminal-based pair programming, uses litellm)
  if (ua.includes('aider/') || ua.includes('litellm/')) {
    return 'Aider';
  }

  // Amazon Q Developer
  if (ua.includes('amazon-q') || ua.includes('amazonq') || ua.includes('q-developer')) {
    return 'Amazon Q';
  }

  // Tabnine
  if (ua.includes('tabnine/')) {
    return 'Tabnine';
  }

  // Codeium
  if (ua.includes('codeium')) {
    return 'Codeium';
  }

  // Sourcegraph Cody
  if (ua.includes('cody/') || ua.includes('sourcegraph')) {
    return 'Cody';
  }

  // Supermaven
  if (ua.includes('supermaven/')) {
    return 'Supermaven';
  }

  // Goose (AI coding agent)
  if (ua.includes('goose/') || ua.includes('block-goose')) {
    return 'Goose';
  }

  // Augment Code
  if (ua.includes('augment/') || ua.includes('augmentcode')) {
    return 'Augment';
  }

  // ===== AI 搜索/聊天客户端 =====
  // Perplexity
  if (ua.includes('perplexity-user') || ua.includes('perplexity/')) {
    return 'Perplexity';
  }

  // Mistral AI
  if (ua.includes('mistralai-user') || ua.includes('mistral/')) {
    return 'Mistral';
  }

  // Poe (Quora 的 AI 聚合平台)
  if (ua.includes('poe/')) {
    return 'Poe';
  }

  // ===== 其他 AI 工具 =====
  // LangChain
  if (ua.includes('langchain')) {
    return 'LangChain';
  }

  // OpenAI API (直接调用)
  if (ua.includes('openai/') || ua.includes('openai-api')) {
    return 'OpenAI API';
  }

  // Anthropic API (直接调用)
  if (ua.includes('anthropic/') || ua.includes('anthropic-api')) {
    return 'Anthropic API';
  }

  // ===== Postman/API 测试工具 =====
  if (ua.includes('postmanruntime/')) {
    return 'Postman';
  }

  if (ua.includes('insomnia/')) {
    return 'Insomnia';
  }

  // ===== 命令行工具 =====
  if (ua.includes('curl/')) {
    return 'cURL';
  }

  if (ua.includes('wget/')) {
    return 'Wget';
  }

  if (ua.includes('python-requests/') || ua.includes('python-urllib/')) {
    return 'Python';
  }

  if (ua.includes('go-http-client/') || ua.includes('go-resty/')) {
    return 'Go';
  }

  if (ua.includes('node-fetch/') || ua.includes('axios/')) {
    return 'Node.js';
  }

  if (ua.includes('java/')) {
    return 'Java';
  }

  // ===== 通用浏览器匹配（最后匹配，因为很多应用基于浏览器） =====
  if (ua.includes('firefox/')) return 'Firefox';
  if (ua.includes('edg/')) return 'Edge';
  if (ua.includes('chrome/')) return 'Chrome';
  if (ua.includes('safari/') && !ua.includes('chrome')) return 'Safari';

  // 其他工具：尝试提取名称（格式为 name/version）
  const match = ua.match(/^([a-z0-9_-]+)\//);
  if (match) {
    const name = match[1];
    // 过滤掉一些常见的无关标识
    if (!['mozilla', 'applewebkit', 'khtml', 'gecko', 'like'].includes(name)) {
      return name.charAt(0).toUpperCase() + name.slice(1);
    }
  }

  return '-';
}

/**
 * 根据来源名称获取颜色
 * @param {string} source - 来源名称
 * @returns {string} Semi Design 颜色名称
 */
function getSourceColor(source) {
  if (!source || source === '-') {
    return 'grey';
  }
  // 使用字符串计算颜色索引
  let hash = 0;
  for (let i = 0; i < source.length; i++) {
    hash = source.charCodeAt(i) + ((hash << 5) - hash);
  }
  const index = Math.abs(hash) % colors.length;
  return colors[index];
}

function getLogOtherSummaryValue(other, key) {
  if (!other || typeof other !== 'object') {
    return '';
  }
  const value = other[key];
  return typeof value === 'string' ? value.trim() : '';
}

/**
 * 判断交互类型
 * @param {object|string} record - 日志记录对象
 * @returns {string|null} 类型名称
 */
function flattenResponsesPromptInputItems(input) {
  if (!Array.isArray(input)) {
    return [];
  }

  const items = [];
  input.forEach((item) => {
    if (!item || typeof item !== 'object') {
      return;
    }

    if (item.type === 'message') {
      const content = Array.isArray(item.content) ? item.content : [];
      content.forEach((part) => {
        if (!part || typeof part !== 'object') {
          return;
        }
        if (!['input_text', 'text', 'output_text'].includes(part.type)) {
          return;
        }
        items.push({
          type: part.type,
          role: item.role,
          text: part.text,
        });
      });
      return;
    }

    if (['function_call', 'function_call_output', 'input_text', 'text', 'output_text'].includes(item.type)) {
      items.push(item);
    }
  });

  return items;
}

function inferResponsesInteractionType(items) {
  if (!Array.isArray(items) || items.length === 0) {
    return null;
  }

  const meaningfulItems = items.filter((item) => item && typeof item === 'object' && item.type);
  if (meaningfulItems.length === 0) {
    return null;
  }

  const lastItem = meaningfulItems[meaningfulItems.length - 1];
  if (lastItem.type === 'input_text' || lastItem.type === 'text') {
    return '输入';
  }

  if (lastItem.type === 'function_call_output') {
    return '回调';
  }

  if (lastItem.type === 'function_call') {
    return '回调';
  }

  if (lastItem.type === 'output_text') {
    for (let i = meaningfulItems.length - 2; i >= 0; i -= 1) {
      if (meaningfulItems[i].type === 'function_call_output') {
        return '输出';
      }
    }
  }

  return null;
}

function inferResponsesStructuredInteractionType({
  responsesRequestBlocks,
  responsesToolResponses,
  responsesResponseBlocks,
}) {
  const requestBlocks = Array.isArray(responsesRequestBlocks) ? responsesRequestBlocks : [];
  const toolResponses = Array.isArray(responsesToolResponses) ? responsesToolResponses : [];
  const responseBlocks = Array.isArray(responsesResponseBlocks) ? responsesResponseBlocks : [];

  const hasRequestInput = requestBlocks.some(
    (block) => block && typeof block.text === 'string' && block.text.trim() !== '',
  );
  const hasToolResponse = toolResponses.length > 0;
  const hasTextOutput = responseBlocks.some(
    (block) => block?.type === 'output_text' && typeof block.content === 'string' && block.content.trim() !== '',
  );
  const hasToolUse = responseBlocks.some((block) => block?.type === 'function_call');

  if (hasRequestInput) {
    return '输入';
  }

  if (!hasRequestInput && hasTextOutput && !hasToolUse) {
    return '输出';
  }

  if (hasToolResponse || hasToolUse || responseBlocks.length > 0) {
    return '回调';
  }

  return null;
}

function parseInteractionType(record) {
  if (!record) return null;

  try {
    const recordData = typeof record === 'string' ? JSON.parse(record) : record;
    const headers = recordData?.request?.headers || recordData?.headers || {};
    const prompt = recordData?.request?.body || recordData?.prompt || {};
    const completion = recordData?.response?.body || recordData?.completion || '';
    const claudeRequestBlocks = Array.isArray(recordData?.claudeRequestBlocks)
      ? recordData.claudeRequestBlocks
      : Array.isArray(prompt?.claudeRequestBlocks)
        ? prompt.claudeRequestBlocks
        : [];
    const claudeToolResponses = Array.isArray(recordData?.claudeToolResponses)
      ? recordData.claudeToolResponses
      : [];
    const claudeResponseBlocks = Array.isArray(recordData?.claudeResponseBlocks)
      ? recordData.claudeResponseBlocks
      : [];
    const responsesRequestBlocks = Array.isArray(recordData?.responsesRequestBlocks)
      ? recordData.responsesRequestBlocks
      : [];
    const responsesToolResponses = Array.isArray(recordData?.responsesToolResponses)
      ? recordData.responsesToolResponses
      : [];
    const responsesResponseBlocks = Array.isArray(recordData?.responsesResponseBlocks)
      ? recordData.responsesResponseBlocks
      : [];
    const openAIResponseBlocks = Array.isArray(recordData?.openaiResponseBlocks)
      ? recordData.openaiResponseBlocks
      : [];
    const responsesPromptItems = flattenResponsesPromptInputItems(prompt?.input);

    const responsesStructuredType = inferResponsesStructuredInteractionType({
      responsesRequestBlocks,
      responsesToolResponses,
      responsesResponseBlocks,
    });
    if (responsesStructuredType) {
      return responsesStructuredType;
    }

    const responsesType = inferResponsesInteractionType(responsesPromptItems);
    if (responsesType) {
      return responsesType;
    }

    const lastUserMessage = prompt?.lastUserMessage || {};
    const legacyToolInvokes = Array.isArray(recordData?.toolInvokes)
      ? recordData.toolInvokes
      : [];
    const hasPromptObjectContent =
      typeof prompt === 'object' &&
      prompt !== null &&
      !Array.isArray(prompt) &&
      Object.keys(prompt).some((key) => key !== 'input');

    const hasNonToolInput =
      (typeof prompt === 'string' && prompt.trim() !== '') ||
      (lastUserMessage.content && lastUserMessage.content.trim() !== '') ||
      claudeRequestBlocks.length > 0 ||
      responsesRequestBlocks.length > 0 ||
      hasPromptObjectContent ||
      (Array.isArray(prompt) && prompt.length > 0);
    const hasToolInput =
      claudeToolResponses.length > 0 ||
      responsesToolResponses.length > 0;
    const hasTextOutput =
      (typeof completion === 'string' && completion.trim() !== '') ||
      claudeResponseBlocks.some(
        (block) => block?.type === 'text' && typeof block.content === 'string' && block.content.trim() !== '',
      ) ||
      responsesResponseBlocks.some(
        (block) => block?.type === 'output_text' && typeof block.content === 'string' && block.content.trim() !== '',
      ) ||
      openAIResponseBlocks.some(
        (block) =>
          (block?.type === 'content' || block?.type === 'reasoning') &&
          typeof block.content === 'string' &&
          block.content.trim() !== '',
      );
    const hasAnyOutput =
      hasTextOutput ||
      (typeof completion === 'object' && completion !== null &&
        ((Array.isArray(completion) && completion.length > 0) ||
          (!Array.isArray(completion) && Object.keys(completion).length > 0))) ||
      claudeResponseBlocks.length > 0 ||
      responsesResponseBlocks.length > 0 ||
      openAIResponseBlocks.length > 0;
    const hasToolUse =
      claudeResponseBlocks.some((block) => block?.type === 'tool_use') ||
      responsesResponseBlocks.some((block) => block?.type === 'function_call') ||
      openAIResponseBlocks.some((block) => block?.type === 'tool_call') ||
      legacyToolInvokes.length > 0;

    if (hasNonToolInput) {
      return '输入';
    }

    if (!hasNonToolInput && hasTextOutput && !hasToolUse) {
      return '输出';
    }

    if (hasToolInput || hasToolUse || hasAnyOutput) {
      return '回调';
    }

    return null;
  } catch (e) {
    return null;
  }
}

function renderCompactDetailSummary(summarySegments) {
  const segments = Array.isArray(summarySegments)
    ? summarySegments.filter((segment) => segment?.text)
    : [];
  if (!segments.length) {
    return null;
  }

  return (
    <div
      style={{
        maxWidth: 180,
        lineHeight: 1.35,
      }}
    >
      {segments.map((segment, index) => (
        <Typography.Text
          key={`${segment.text}-${index}`}
          type={segment.tone === 'secondary' ? 'tertiary' : undefined}
          size={segment.tone === 'secondary' ? 'small' : undefined}
          style={{
            display: 'block',
            maxWidth: '100%',
            fontSize: 12,
            marginTop: index === 0 ? 0 : 2,
            whiteSpace: 'nowrap',
            overflow: 'hidden',
            textOverflow: 'ellipsis',
          }}
        >
          {segment.text}
        </Typography.Text>
      ))}
    </div>
  );
}

function getUsageLogDetailSummary(record, text, billingDisplayMode, t) {
  const other = getLogOther(record.other);

  if (record.type === 6) {
    return {
      segments: [{ text: t('异步任务退款'), tone: 'primary' }],
    };
  }

  if (other == null || record.type !== 2) {
    return null;
  }

  if (
    other?.violation_fee === true ||
    Boolean(other?.violation_fee_code) ||
    Boolean(other?.violation_fee_marker)
  ) {
    const feeQuota = other?.fee_quota ?? record?.quota;
    const groupText = getUsageLogGroupSummary(
      other?.group_ratio,
      other?.user_group_ratio,
      t,
    );
    return {
      segments: [
        groupText ? { text: groupText, tone: 'primary' } : null,
        { text: t('违规扣费'), tone: 'primary' },
        {
          text: `${t('扣费')}：${renderQuota(feeQuota, 6)}`,
          tone: 'secondary',
        },
        text ? { text: `${t('详情')}：${text}`, tone: 'secondary' } : null,
      ].filter(Boolean),
    };
  }

  const summaryOpts = { ...other, displayMode: billingDisplayMode, outputMode: 'segments' };

  if (other?.billing_mode === 'tiered_expr') {
    return { segments: renderTieredModelPriceSimple(summaryOpts) };
  }

  return {
    segments: other?.claude
      ? renderModelPriceSimple({ ...summaryOpts, provider: 'claude' })
      : renderModelPriceSimple({ ...summaryOpts, provider: 'openai' }),
  };
}

export const getLogsColumns = ({
  t,
  COLUMN_KEYS,
  copyText,
  showUserInfoFunc,
  openChannelAffinityUsageCacheModal,
  isAdminUser,
  billingDisplayMode = 'price',
}) => {
  return [
    {
      key: COLUMN_KEYS.TIME,
      title: t('时间'),
      dataIndex: 'timestamp2string',
    },
    {
      key: COLUMN_KEYS.CHANNEL,
      title: t('渠道'),
      dataIndex: 'channel',
      render: (text, record, index) => {
        let isMultiKey = false;
        let multiKeyIndex = -1;
        let content = t('渠道') + `：${record.channel}`;
        let affinity = null;
        let showMarker = false;
        let other = getLogOther(record.other);
        if (other?.admin_info) {
          let adminInfo = other.admin_info;
          if (adminInfo?.is_multi_key) {
            isMultiKey = true;
            multiKeyIndex = adminInfo.multi_key_index;
          }
          if (
            Array.isArray(adminInfo.use_channel) &&
            adminInfo.use_channel.length > 0
          ) {
            content = t('渠道') + `：${adminInfo.use_channel.join('->')}`;
          }
          if (adminInfo.channel_affinity) {
            affinity = adminInfo.channel_affinity;
            showMarker = true;
          }
        }

        return isAdminUser &&
          (record.type === 0 ||
            record.type === 2 ||
            record.type === 5 ||
            record.type === 6) ? (
          <Space>
            <span style={{ position: 'relative', display: 'inline-block' }}>
              <Tooltip content={record.channel_name || t('未知渠道')}>
                <span>
                  <Tag
                    color={colors[parseInt(text) % colors.length]}
                    shape='circle'
                  >
                    {text}
                  </Tag>
                </span>
              </Tooltip>
              {showMarker && (
                <Tooltip
                  content={
                    <div style={{ lineHeight: 1.6 }}>
                      <div>{content}</div>
                      {affinity ? (
                        <div style={{ marginTop: 6 }}>
                          {buildChannelAffinityTooltip(affinity, t)}
                        </div>
                      ) : null}
                    </div>
                  }
                >
                  <span
                    style={{
                      position: 'absolute',
                      right: -4,
                      top: -4,
                      lineHeight: 1,
                      fontWeight: 600,
                      color: '#f59e0b',
                      cursor: 'pointer',
                      userSelect: 'none',
                    }}
                    onClick={(e) => {
                      e.stopPropagation();
                      openChannelAffinityUsageCacheModal?.(affinity);
                    }}
                  >
                    <Sparkles
                      size={14}
                      strokeWidth={2}
                      color='currentColor'
                      fill='currentColor'
                    />
                  </span>
                </Tooltip>
              )}
            </span>
            {isMultiKey && (
              <Tag color='white' shape='circle'>
                {multiKeyIndex}
              </Tag>
            )}
          </Space>
        ) : null;
      },
    },
    {
      key: COLUMN_KEYS.USERNAME,
      title: t('用户'),
      dataIndex: 'username',
      render: (text, record, index) => {
        return isAdminUser ? (
          <div>
            <Avatar
              size='extra-small'
              color={stringToColor(text)}
              style={{ marginRight: 4 }}
              onClick={(event) => {
                event.stopPropagation();
                showUserInfoFunc(record.user_id);
              }}
            >
              {typeof text === 'string' && text.slice(0, 1)}
            </Avatar>
            {text}
          </div>
        ) : (
          <></>
        );
      },
    },
    {
      key: COLUMN_KEYS.TOKEN,
      title: t('令牌'),
      dataIndex: 'token_name',
      render: (text, record, index) => {
        return record.type === 0 ||
          record.type === 2 ||
          record.type === 5 ||
          record.type === 6 ? (
          <div>
            <Tag
              color='grey'
              shape='circle'
              onClick={(event) => {
                copyText(event, text);
              }}
            >
              {' '}
              {t(text)}{' '}
            </Tag>
          </div>
        ) : (
          <></>
        );
      },
    },
    {
      key: COLUMN_KEYS.GROUP,
      title: t('分组'),
      dataIndex: 'group',
      render: (text, record, index) => {
        if (
          record.type === 0 ||
          record.type === 2 ||
          record.type === 5 ||
          record.type === 6
        ) {
          if (record.group) {
            return <>{renderGroup(record.group)}</>;
          } else {
            let other = null;
            try {
              other = JSON.parse(record.other);
            } catch (e) {
              console.error(
                `Failed to parse record.other: "${record.other}".`,
                e,
              );
            }
            if (other === null) {
              return <></>;
            }
            if (other.group !== undefined) {
              return <>{renderGroup(other.group)}</>;
            } else {
              return <></>;
            }
          }
        } else {
          return <></>;
        }
      },
    },
    {
      key: COLUMN_KEYS.TYPE,
      title: t('类型'),
      dataIndex: 'type',
      render: (text, record, index) => {
        return <>{renderType(text, t)}</>;
      },
    },
    {
      key: COLUMN_KEYS.MODEL,
      title: t('模型'),
      dataIndex: 'model_name',
      render: (text, record, index) => {
        return record.type === 0 ||
          record.type === 2 ||
          record.type === 5 ||
          record.type === 6 ? (
          <>{renderModelName(record, copyText, t)}</>
        ) : (
          <></>
        );
      },
    },
    {
      key: COLUMN_KEYS.SOURCE,
      title: t('来源'),
      render: (text, record, index) => {
        if (!(record.type === 2 || record.type === 5)) {
          return null;
        }

        try {
          const other = getLogOther(record.other);
          const sourceSummary = getLogOtherSummaryValue(other, 'client_source');
          if (sourceSummary) {
            return (
              <Tag color={getSourceColor(sourceSummary)} shape="circle">
                {sourceSummary}
              </Tag>
            );
          }

          const detailData = record.record;
          const recordData = detailData
            ? typeof detailData === 'string'
              ? JSON.parse(detailData)
              : detailData
            : null;
          const headers = recordData?.request?.headers || recordData?.headers || {};

          // 查找 User-Agent（不区分大小写）
          const uaKey = Object.keys(headers).find(
            k => k.toLowerCase() === 'user-agent',
          );
          const userAgent = uaKey ? headers[uaKey] : '';

          const source = parseClientSource(userAgent);
          return source !== '-' ? (
            <Tag color={getSourceColor(source)} shape="circle">
              {source}
            </Tag>
          ) : null;
        } catch (e) {
          return null;
        }
      },
    },
    {
      key: COLUMN_KEYS.INTERACTION_TYPE,
      title: t('类型'),
      render: (text, record, index) => {
        if (!(record.type === 2 || record.type === 5)) {
          return null;
        }

        const other = getLogOther(record.other);
        const interactionTypeSummary = getLogOtherSummaryValue(
          other,
          'interaction_type',
        );
        const interactionType =
          interactionTypeSummary || parseInteractionType(record.record);
        if (!interactionType) return null;

        const colorMap = {
          回调: 'purple',
          输入: 'cyan',
          输出: 'green',
        };

        return (
          <Tag color={colorMap[interactionType] || 'grey'} shape="circle">
            {interactionType}
          </Tag>
        );
      },
    },
    {
      key: COLUMN_KEYS.USE_TIME,
      title: t('用时/首字'),
      dataIndex: 'use_time',
      render: (text, record, index) => {
        if (!(record.type === 2 || record.type === 5)) {
          return <></>;
        }
        if (record.is_stream) {
          let other = getLogOther(record.other);
          return (
            <>
              <Space>
                {renderUseTime(text, t)}
                {renderFirstUseTime(other?.frt, t)}
                {renderIsStream(record.is_stream, t, other?.stream_status)}
              </Space>
            </>
          );
        } else {
          return (
            <>
              <Space>
                {renderUseTime(text, t)}
                {renderIsStream(record.is_stream, t)}
              </Space>
            </>
          );
        }
      },
    },
    {
      key: COLUMN_KEYS.TPS,
      title: t('TPS'),
      dataIndex: 'other',
      render: (text, record, index) => {
        if (!(record.type === 2 || record.type === 5)) {
          return <></>;
        }
        const other = getLogOther(text);
        return renderTPS(other?.tps, t);
      },
    },
    {
      key: COLUMN_KEYS.PROMPT,
      title: (
        <div className='flex items-center gap-1'>
          {t('输入')}
          <Tooltip
            content={t(
              '根据 Anthropic 协定，/v1/messages 的输入 tokens 仅统计非缓存输入，不包含缓存读取与缓存写入 tokens。',
            )}
          >
            <IconHelpCircle className='text-gray-400 cursor-help' />
          </Tooltip>
        </div>
      ),
      dataIndex: 'prompt_tokens',
      render: (text, record, index) => {
        const other = getLogOther(record.other);
        const cacheSummary = getPromptCacheSummary(other);
        const hasCacheRead = (cacheSummary?.cacheReadTokens || 0) > 0;
        const hasCacheWrite = (cacheSummary?.cacheWriteTokens || 0) > 0;
        let cacheText = '';
        if (hasCacheRead && hasCacheWrite) {
          cacheText = `${t('缓存读')} ${formatTokenCount(cacheSummary.cacheReadTokens)} · ${t('写')} ${formatTokenCount(cacheSummary.cacheWriteTokens)}`;
        } else if (hasCacheRead) {
          cacheText = `${t('缓存读')} ${formatTokenCount(cacheSummary.cacheReadTokens)}`;
        } else if (hasCacheWrite) {
          cacheText = `${t('缓存写')} ${formatTokenCount(cacheSummary.cacheWriteTokens)}`;
        }

        return record.type === 0 ||
          record.type === 2 ||
          record.type === 5 ||
          record.type === 6 ? (
          <div
            style={{
              display: 'inline-flex',
              flexDirection: 'column',
              alignItems: 'flex-start',
              lineHeight: 1.2,
            }}
          >
            <span>{text}</span>
            {cacheText ? (
              <span
                style={{
                  marginTop: 2,
                  fontSize: 11,
                  color: 'var(--semi-color-text-2)',
                  whiteSpace: 'nowrap',
                }}
              >
                {cacheText}
              </span>
            ) : null}
          </div>
        ) : (
          <></>
        );
      },
    },
    {
      key: COLUMN_KEYS.COMPLETION,
      title: t('输出'),
      dataIndex: 'completion_tokens',
      render: (text, record, index) => {
        return parseInt(text) > 0 &&
          (record.type === 0 ||
            record.type === 2 ||
            record.type === 5 ||
            record.type === 6) ? (
          <>{<span> {text} </span>}</>
        ) : (
          <></>
        );
      },
    },
    {
      key: COLUMN_KEYS.COST,
      title: t('花费'),
      dataIndex: 'quota',
      render: (text, record, index) => {
        if (
          !(
            record.type === 0 ||
            record.type === 2 ||
            record.type === 5 ||
            record.type === 6
          )
        ) {
          return <></>;
        }
        const other = getLogOther(record.other);
        const isSubscription = other?.billing_source === 'subscription';
        if (isSubscription) {
          // Subscription billed: show only tag (no $0), but keep tooltip for equivalent cost.
          return (
            <Tooltip content={`${t('由订阅抵扣')}：${renderQuota(text, 6)}`}>
              <span>{renderBillingTag(record, t)}</span>
            </Tooltip>
          );
        }
        return <>{renderQuota(text, 6)}</>;
      },
    },
    {
      key: COLUMN_KEYS.IP,
      title: (
        <div className='flex items-center gap-1'>
          {t('IP')}
          <Tooltip
            content={t(
              '只有当用户设置开启IP记录时，才会进行请求和错误类型日志的IP记录',
            )}
          >
            <IconHelpCircle className='text-gray-400 cursor-help' />
          </Tooltip>
        </div>
      ),
      dataIndex: 'ip',
      render: (text, record, index) => {
        const showIp =
          (record.type === 2 ||
            record.type === 5 ||
            (isAdminUser && record.type === 1)) &&
          text;
        return showIp ? (
          <Tooltip content={text}>
            <span>
              <Tag
                color='orange'
                shape='circle'
                onClick={(event) => {
                  copyText(event, text);
                }}
              >
                {text}
              </Tag>
            </span>
          </Tooltip>
        ) : (
          <></>
        );
      },
    },
    {
      key: COLUMN_KEYS.RETRY,
      title: t('重试'),
      dataIndex: 'retry',
      render: (text, record, index) => {
        if (!(record.type === 2 || record.type === 5)) {
          return <></>;
        }
        let content = t('渠道') + `：${record.channel}`;
        const other = getLogOther(record.other);
        if (other && Object.keys(other).length > 0) {
          if (other.admin_info !== undefined) {
            if (
              other.admin_info.use_channel !== null &&
              other.admin_info.use_channel !== undefined &&
              other.admin_info.use_channel !== ''
            ) {
              const useChannel = other.admin_info.use_channel;
              const useChannelStr = useChannel.join('->');
              content = t('渠道') + `：${useChannelStr}`;
            }
          }
        }
        return isAdminUser ? <div>{content}</div> : <></>;
      },
    },
    {
      key: COLUMN_KEYS.DETAILS,
      title: t('详情'),
      dataIndex: 'content',
      fixed: 'right',
      width: 200,
      render: (text, record, index) => {
        const detailSummary = getUsageLogDetailSummary(
          record,
          text,
          billingDisplayMode,
          t,
        );

        if (!detailSummary) {
          return (
            <Typography.Paragraph
              ellipsis={{
                rows: 2,
                showTooltip: {
                  type: 'popover',
                  opts: { style: { width: 240 } },
                },
              }}
              style={{ maxWidth: 200, marginBottom: 0 }}
            >
              {text}
            </Typography.Paragraph>
          );
        }

        return renderCompactDetailSummary(detailSummary.segments);
      },
    },
  ];
};

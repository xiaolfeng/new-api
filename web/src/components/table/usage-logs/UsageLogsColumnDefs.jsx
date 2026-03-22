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
} from '../../../helpers';
import { IconHelpCircle } from '@douyinfe/semi-icons';
import { Route, Sparkles } from 'lucide-react';

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

function renderIsStream(bool, t) {
  if (bool) {
    return (
      <Tag color='blue' shape='circle'>
        {t('流')}
      </Tag>
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

/**
 * 判断交互类型
 * @param {object|string} record - 日志记录对象
 * @returns {string|null} 类型名称，Codex 返回 null
 */
function parseInteractionType(record) {
  if (!record) return null;

  try {
    const recordData = typeof record === 'string' ? JSON.parse(record) : record;
    const headers = recordData?.headers || {};
    const prompt = recordData?.prompt || {};
    const completion = recordData?.completion || '';

    // 检查 User-Agent 是否为 Codex
    const userAgent = Object.keys(headers).find(
      key => key.toLowerCase() === 'user-agent'
    );
    if (userAgent) {
      const ua = headers[userAgent]?.toLowerCase() || '';
      if (ua.includes('codex_cli_rs') || ua.includes('codex-cli-rs')) {
        return null; // Codex 不显示类型
      }
    }

    // 获取最后一个用户消息的内容
    const lastUserMessage = prompt?.lastUserMessage || {};
    const hasContent = lastUserMessage.content && lastUserMessage.content.trim() !== '';
    const hasCompletion = completion && completion.trim() !== '';

    // 判断逻辑：
    // 1. 有请求内容 → "输入"（不管是否有响应）
    // 2. 无请求内容 + 有响应 → "输出"
    // 3. 无请求内容 + 无响应 → "工具"
    if (hasContent) {
      return '输入';
    }
    if (hasCompletion) {
      return '输出';
    }
    return '工具';
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

  return {
    segments: other?.claude
      ? renderModelPriceSimple(
          other.model_ratio,
          other.model_price,
          other.group_ratio,
          other?.user_group_ratio,
          other.cache_tokens || 0,
          other.cache_ratio || 1.0,
          other.cache_creation_tokens || 0,
          other.cache_creation_ratio || 1.0,
          other.cache_creation_tokens_5m || 0,
          other.cache_creation_ratio_5m || other.cache_creation_ratio || 1.0,
          other.cache_creation_tokens_1h || 0,
          other.cache_creation_ratio_1h || other.cache_creation_ratio || 1.0,
          false,
          1.0,
          other?.is_system_prompt_overwritten,
          'claude',
          billingDisplayMode,
          'segments',
        )
      : renderModelPriceSimple(
          other.model_ratio,
          other.model_price,
          other.group_ratio,
          other?.user_group_ratio,
          other.cache_tokens || 0,
          other.cache_ratio || 1.0,
          0,
          1.0,
          0,
          1.0,
          0,
          1.0,
          false,
          1.0,
          other?.is_system_prompt_overwritten,
          'openai',
          billingDisplayMode,
          'segments',
        ),
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
          const recordData = record.record
            ? typeof record.record === 'string'
              ? JSON.parse(record.record)
              : record.record
            : null;
          const headers = recordData?.headers || {};

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

        const interactionType = parseInteractionType(record.record);
        if (!interactionType) return null;

        const colorMap = {
          工具: 'purple',
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
                {renderIsStream(record.is_stream, t)}
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
        return (record.type === 2 || record.type === 5) && text ? (
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
        if (record.other !== '') {
          let other = JSON.parse(record.other);
          if (other === null) {
            return <></>;
          }
          if (other.admin_info !== undefined) {
            if (
              other.admin_info.use_channel !== null &&
              other.admin_info.use_channel !== undefined &&
              other.admin_info.use_channel !== ''
            ) {
              let useChannel = other.admin_info.use_channel;
              let useChannelStr = useChannel.join('->');
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

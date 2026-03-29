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
import {
  Modal,
  Button,
  Collapse,
  Typography,
  Empty,
  Table,
} from '@douyinfe/semi-ui';
import { IconCopy } from '@douyinfe/semi-icons';
import { copy, showError, showSuccess } from '../../../../helpers';
import { MarkdownSourceHighlighter } from '../../../common/markdown/MarkdownSourceHighlighter';

const { Text } = Typography;

const deriveResponsesRequestDataFromPromptInput = (promptInput) => {
  if (!Array.isArray(promptInput)) {
    return {
      requestBlocks: [],
      toolResponses: [],
    };
  }

  const requestBlocks = [];
  const toolResponses = [];

  promptInput.forEach((item) => {
    if (!item || typeof item !== 'object') {
      return;
    }

    if (item.type === 'message') {
      const role = typeof item.role === 'string' ? item.role : '';
      const content = Array.isArray(item.content) ? item.content : [];
      content.forEach((part) => {
        if (!part || typeof part !== 'object') {
          return;
        }
        if (!['input_text', 'text'].includes(part.type)) {
          return;
        }
        if (typeof part.text !== 'string' || part.text.trim() === '') {
          return;
        }
        requestBlocks.push({
          type: part.type,
          role,
          text: part.text,
        });
      });
      return;
    }

    if (['input_text', 'text'].includes(item.type)) {
      if (typeof item.text !== 'string' || item.text.trim() === '') {
        return;
      }
      requestBlocks.push({
        type: item.type,
        role: typeof item.role === 'string' ? item.role : '',
        text: item.text,
      });
      return;
    }

    if (item.type === 'function_call_output') {
      toolResponses.push({
        callId: item.call_id || item.callId || '',
        name: item.name || '',
        type: 'function_call_output',
      });
    }
  });

  return {
    requestBlocks,
    toolResponses,
  };
};

const LogDetailModal = ({
  showLogDetailModal,
  setShowLogDetailModal,
  logDetailTarget,
  logDetailMode,
  t,
}) => {
  const parseJson = (value) => {
    if (!value) return null;
    try {
      return JSON.parse(value);
    } catch {
      return null;
    }
  };

  const legacyRecord = useMemo(
    () => parseJson(logDetailTarget?.record),
    [logDetailTarget?.record],
  );
  const fullLogRecord = useMemo(
    () => parseJson(logDetailTarget?.full_log),
    [logDetailTarget?.full_log],
  );
  const isFullLogRecord = logDetailMode === 'full_log';
  const record = isFullLogRecord ? fullLogRecord : legacyRecord;
  const prompt = record?.prompt || {};
  const fallbackResponsesRequestData = useMemo(
    () => deriveResponsesRequestDataFromPromptInput(prompt?.input),
    [prompt?.input],
  );
  const claudeRequestBlocks = Array.isArray(record?.claudeRequestBlocks)
    ? record.claudeRequestBlocks
    : Array.isArray(prompt?.claudeRequestBlocks)
      ? prompt.claudeRequestBlocks
      : [];
  const claudeToolResponses = Array.isArray(record?.claudeToolResponses)
    ? record.claudeToolResponses
    : [];
  const claudeResponseBlocks = Array.isArray(record?.claudeResponseBlocks)
    ? record.claudeResponseBlocks
    : [];
  const responsesRequestBlocks =
    Array.isArray(record?.responsesRequestBlocks) && record.responsesRequestBlocks.length > 0
      ? record.responsesRequestBlocks
      : fallbackResponsesRequestData.requestBlocks;
  const responsesToolResponses =
    Array.isArray(record?.responsesToolResponses) && record.responsesToolResponses.length > 0
      ? record.responsesToolResponses
      : fallbackResponsesRequestData.toolResponses;
  const responsesResponseBlocks = Array.isArray(record?.responsesResponseBlocks)
    ? record.responsesResponseBlocks
    : [];
  const openAIResponseBlocks = Array.isArray(record?.openaiResponseBlocks)
    ? record.openaiResponseBlocks
    : [];
  const hasClaudeStructuredRecord =
    !isFullLogRecord &&
    (claudeRequestBlocks.length > 0 ||
      claudeToolResponses.length > 0 ||
      claudeResponseBlocks.length > 0);
  const hasResponsesStructuredRecord =
    !isFullLogRecord &&
    (responsesRequestBlocks.length > 0 ||
      responsesToolResponses.length > 0 ||
      responsesResponseBlocks.length > 0);
  const hasOpenAIStructuredRecord =
    !isFullLogRecord &&
    openAIResponseBlocks.length > 0;
  const claudeResponseSections = useMemo(() => {
    const thinkingParts = [];
    const answerParts = [];
    const toolUses = [];

    claudeResponseBlocks.forEach((block, index) => {
      if (!block || typeof block !== 'object') {
        return;
      }

      if (block.type === 'thinking' && block.content) {
        thinkingParts.push(block.content);
        return;
      }

      if (block.type === 'text' && block.content) {
        answerParts.push(block.content);
        return;
      }

      if (block.type === 'tool_use') {
        toolUses.push({
          order: toolUses.length + 1,
          id: block.id || `claude-tool-${index}`,
          name: block.name,
          input: block.input,
        });
      }
    });

    return {
      thinking: thinkingParts.join('\n\n'),
      answer: answerParts.join('\n\n'),
      toolUses,
    };
  }, [claudeResponseBlocks]);
  const responsesResponseSections = useMemo(() => {
    const answerParts = [];
    const toolUses = [];

    responsesResponseBlocks.forEach((block, index) => {
      if (!block || typeof block !== 'object') {
        return;
      }

      if (block.type === 'output_text' && block.content) {
        answerParts.push(block.content);
        return;
      }

      if (block.type === 'function_call') {
        toolUses.push({
          order: toolUses.length + 1,
          id: block.id || block.callId || `responses-tool-${index}`,
          callId: block.callId || block.id,
          name: block.name,
          arguments: block.arguments,
        });
      }
    });

    return {
      answer: answerParts.join('\n\n'),
      toolUses,
    };
  }, [responsesResponseBlocks]);
  const openAIResponseSections = useMemo(() => {
    const thinkingParts = [];
    const answerParts = [];
    const toolUses = [];

    openAIResponseBlocks.forEach((block, index) => {
      if (!block || typeof block !== 'object') {
        return;
      }

      if (block.type === 'reasoning' && block.content) {
        thinkingParts.push(block.content);
        return;
      }

      if (block.type === 'content' && block.content) {
        answerParts.push(block.content);
        return;
      }

      if (block.type === 'tool_call') {
        toolUses.push({
          order: toolUses.length + 1,
          id: block.id || `openai-tool-${index}`,
          callId: block.id,
          callIndex: block.callIndex,
          name: block.name,
          arguments: block.arguments,
        });
      }
    });

    return {
      thinking: thinkingParts.join('\n\n'),
      answer: answerParts.join('\n\n'),
      toolUses,
    };
  }, [openAIResponseBlocks]);

  const renderSimpleTableValue = (value) => (
    <pre style={{
      margin: 0,
      whiteSpace: 'pre-wrap',
      wordBreak: 'break-word',
      fontSize: 12,
    }}>
      {value == null ? '-' : JSON.stringify(value, null, 2)}
    </pre>
  );

  const renderHorizontalScrollableTableValue = (value) => (
    <div
      style={{
        maxWidth: '100%',
        overflowX: 'auto',
        overflowY: 'hidden',
      }}
    >
      <pre style={{
        margin: 0,
        whiteSpace: 'pre',
        wordBreak: 'normal',
        fontSize: 12,
        minWidth: 'max-content',
      }}>
        {value == null ? '-' : JSON.stringify(value, null, 2)}
      </pre>
    </div>
  );

  const renderClaudeToolResponsesTable = () => {
    if (claudeToolResponses.length === 0) {
      return <Empty description={t('无工具响应记录')} style={{ padding: '20px 0' }} />;
    }

    const columns = [
      {
        title: t('顺序'),
        dataIndex: 'order',
        key: 'order',
        width: 80,
      },
      {
        title: t('工具'),
        dataIndex: 'name',
        key: 'name',
        render: (text, row) => <Text strong>{text || row.toolUseId || '-'}</Text>,
      },
      {
        title: t('调用 ID'),
        dataIndex: 'toolUseId',
        key: 'toolUseId',
        render: (text) => (
          <Text
            style={{
              wordBreak: 'break-all',
              maxWidth: 500,
            }}
          >
            {text || '-'}
          </Text>
        ),
      },
    ];

    const dataSource = claudeToolResponses.map((item, index) => ({
      ...item,
      order: index + 1,
      rowKey: item.toolUseId || `${item.name || 'tool-response'}-${index}`,
    }));

    return (
      <Table
        columns={columns}
        dataSource={dataSource}
        pagination={false}
        size='small'
        bordered
        rowKey='rowKey'
        style={{ fontSize: 12 }}
      />
    );
  };

  const renderClaudeToolUsesTable = (toolUses) => {
    if (toolUses.length === 0) {
      return <Empty description={t('无工具调用记录')} style={{ padding: '20px 0' }} />;
    }

    const columns = [
      {
        title: t('顺序'),
        dataIndex: 'order',
        key: 'order',
        width: 80,
      },
      {
        title: t('工具'),
        dataIndex: 'name',
        key: 'name',
        width: 180,
        render: (text, row) => <Text strong>{text || row.id || '-'}</Text>,
      },
      {
        title: t('调用 ID'),
        dataIndex: 'id',
        key: 'id',
        width: 220,
        render: (text) => (
          <Text
            style={{
              wordBreak: 'break-all',
              maxWidth: 500,
            }}
          >
            {text || '-'}
          </Text>
        ),
      },
      {
        title: t('参数'),
        dataIndex: 'input',
        key: 'input',
        render: renderHorizontalScrollableTableValue,
      },
    ];

    const dataSource = toolUses.map((item, index) => ({
      ...item,
      rowKey: item.id || `${item.name || 'tool-use'}-${index}`,
    }));

    return (
      <Table
        columns={columns}
        dataSource={dataSource}
        pagination={false}
        size='small'
        bordered
        rowKey='rowKey'
        style={{ fontSize: 12 }}
      />
    );
  };

  const renderResponsesToolResponsesTable = () => {
    if (responsesToolResponses.length === 0) {
      return <Empty description={t('无工具响应记录')} style={{ padding: '20px 0' }} />;
    }

    const columns = [
      {
        title: t('顺序'),
        dataIndex: 'order',
        key: 'order',
        width: 80,
      },
      {
        title: t('工具'),
        dataIndex: 'name',
        key: 'name',
        render: (text, row) => <Text strong>{text || row.callId || '-'}</Text>,
      },
      {
        title: t('调用 ID'),
        dataIndex: 'callId',
        key: 'callId',
        render: (text) => (
          <Text
            style={{
              wordBreak: 'break-all',
              maxWidth: 500,
            }}
          >
            {text || '-'}
          </Text>
        ),
      },
    ];

    const dataSource = responsesToolResponses.map((item, index) => ({
      ...item,
      order: index + 1,
      rowKey: item.callId || `${item.name || 'responses-tool-response'}-${index}`,
    }));

    return (
      <Table
        columns={columns}
        dataSource={dataSource}
        pagination={false}
        size='small'
        bordered
        rowKey='rowKey'
        style={{ fontSize: 12 }}
      />
    );
  };

  const renderResponsesToolUsesTable = (toolUses) => {
    if (toolUses.length === 0) {
      return <Empty description={t('无工具调用记录')} style={{ padding: '20px 0' }} />;
    }

    const columns = [
      {
        title: t('顺序'),
        dataIndex: 'order',
        key: 'order',
        width: 80,
      },
      {
        title: t('工具'),
        dataIndex: 'name',
        key: 'name',
        width: 180,
        render: (text, row) => <Text strong>{text || row.callId || row.id || '-'}</Text>,
      },
      {
        title: t('调用 ID'),
        dataIndex: 'callId',
        key: 'callId',
        width: 220,
        render: (text, row) => (
          <Text
            style={{
              wordBreak: 'break-all',
              maxWidth: 500,
            }}
          >
            {text || row.id || '-'}
          </Text>
        ),
      },
      {
        title: t('参数'),
        dataIndex: 'arguments',
        key: 'arguments',
        render: renderHorizontalScrollableTableValue,
      },
    ];

    const dataSource = toolUses.map((item, index) => ({
      ...item,
      rowKey: item.callId || item.id || `${item.name || 'responses-tool-use'}-${index}`,
    }));

    return (
      <Table
        columns={columns}
        dataSource={dataSource}
        pagination={false}
        size='small'
        bordered
        rowKey='rowKey'
        style={{ fontSize: 12 }}
      />
    );
  };

  const renderOpenAIToolUsesTable = (toolUses) => {
    if (toolUses.length === 0) {
      return <Empty description={t('无工具调用记录')} style={{ padding: '20px 0' }} />;
    }

    const columns = [
      {
        title: t('顺序'),
        dataIndex: 'order',
        key: 'order',
        width: 80,
      },
      {
        title: t('工具'),
        dataIndex: 'name',
        key: 'name',
        width: 180,
        render: (text, row) => <Text strong>{text || row.callId || row.id || '-'}</Text>,
      },
      {
        title: t('调用 ID'),
        dataIndex: 'callId',
        key: 'callId',
        width: 220,
        render: (text, row) => (
          <Text
            style={{
              wordBreak: 'break-all',
              maxWidth: 500,
            }}
          >
            {text || row.id || '-'}
          </Text>
        ),
      },
      {
        title: t('调用序号'),
        dataIndex: 'callIndex',
        key: 'callIndex',
        width: 100,
        render: (value) => (typeof value === 'number' ? value : '-'),
      },
      {
        title: t('参数'),
        dataIndex: 'arguments',
        key: 'arguments',
        render: renderHorizontalScrollableTableValue,
      },
    ];

    const dataSource = toolUses.map((item, index) => ({
      ...item,
      rowKey: item.callId || item.id || `${item.name || 'openai-tool-use'}-${index}`,
    }));

    return (
      <Table
        columns={columns}
        dataSource={dataSource}
        pagination={false}
        size='small'
        bordered
        rowKey='rowKey'
        style={{ fontSize: 12 }}
      />
    );
  };

  const copySection = async (section, content) => {
    const text = typeof content === 'object' ? JSON.stringify(content, null, 2) : String(content);
    if (await copy(text)) {
      showSuccess(t('{{section}} 已复制', { section }));
    } else {
      showError(t('无法复制到剪贴板，请手动复制'));
    }
  };

  const parseCollapsibleTaggedContent = (content) => {
    if (typeof content !== 'string') {
      return null;
    }

    const lines = content.split('\n');
    let startIndex = 0;
    let endIndex = lines.length - 1;

    while (startIndex <= endIndex && lines[startIndex].trim() === '') {
      startIndex += 1;
    }
    while (endIndex >= startIndex && lines[endIndex].trim() === '') {
      endIndex -= 1;
    }

    if (startIndex >= endIndex) {
      return null;
    }

    const startLine = lines[startIndex].trim();
    const endLine = lines[endIndex].trim();
    const startMatch = startLine.match(/^<([A-Za-z][\w-]*)(?:\s[^>]*)?>$/);
    const endMatch = endLine.match(/^<\/([A-Za-z][\w-]*)>$/);

    if (!startMatch || !endMatch || startMatch[1] !== endMatch[1]) {
      return null;
    }

    return {
      label: startMatch[1],
      content: lines.slice(startIndex + 1, endIndex).join('\n').trim(),
    };
  };

  const renderContentBlock = (content) => {
    if (content === null || content === undefined || content === '') {
      return null;
    }

    if (typeof content === 'string') {
      return (
        <div
          style={{
            background: 'var(--semi-color-fill-0)',
            padding: 12,
            borderRadius: 8,
            maxHeight: 400,
            overflow: 'auto',
          }}
        >
          <MarkdownSourceHighlighter content={content} fontSize={13} />
        </div>
      );
    }

    return (
      <pre
        style={{
          background: 'var(--semi-color-fill-0)',
          padding: 12,
          borderRadius: 8,
          overflow: 'auto',
          maxHeight: 400,
          fontSize: 12,
          margin: 0,
        }}
      >
        {JSON.stringify(content, null, 2)}
      </pre>
    );
  };

  const renderClaudeRequestBlockContent = (content) => {
    const collapsibleContent = parseCollapsibleTaggedContent(content);
    if (!collapsibleContent) {
      return renderContentBlock(content);
    }

    return (
      <details
        style={{
          background: 'var(--semi-color-fill-0)',
          borderRadius: 8,
          padding: 12,
        }}
      >
        <summary
          style={{
            cursor: 'pointer',
            fontSize: 13,
            fontWeight: 600,
            userSelect: 'none',
          }}
        >
          {`<${collapsibleContent.label}>`}
        </summary>
        <div style={{ marginTop: 12 }}>
          {collapsibleContent.content
            ? renderContentBlock(collapsibleContent.content)
            : <Text type='tertiary'>{t('该折叠片段无可展示内容')}</Text>}
        </div>
      </details>
    );
  };

  const renderHeaders = () => {
    const headers = record?.request?.headers || record?.headers || {};
    const entries = Object.entries(headers);
    if (entries.length === 0) {
      return <Empty description={t('无请求头记录')} style={{ padding: '20px 0' }} />;
    }

    const columns = [
      {
        title: t('请求头名称'),
        dataIndex: 'key',
        key: 'key',
        width: 200,
        render: (text) => <Text strong>{text}</Text>,
      },
      {
        title: t('值'),
        dataIndex: 'value',
        key: 'value',
        render: (text) => (
          <Text
            style={{
              wordBreak: 'break-all',
              maxWidth: 500,
            }}
          >
            {text}
          </Text>
        ),
      },
    ];

    const dataSource = entries.map(([key, value], index) => ({
      key,
      value,
      rowKey: index,
    }));

    return (
      <div style={{ padding: '8px 0' }}>
        <div style={{ display: 'flex', justifyContent: 'flex-end', marginBottom: 8 }}>
          <Button
            icon={<IconCopy />}
            size='small'
            theme='borderless'
            onClick={() => copySection(t('请求头'), headers)}
          >
            {t('复制')}
          </Button>
        </div>
        <Table
          columns={columns}
          dataSource={dataSource}
          pagination={false}
          size='small'
          bordered
          rowKey='rowKey'
          style={{
            fontSize: 12,
          }}
        />
      </div>
    );
  };

  const renderPrompt = () => {
    if (isFullLogRecord) {
      const prompt = record?.request?.body;
      if (prompt === null || prompt === undefined || prompt === '') {
        return <Empty description={t('无请求内容记录')} style={{ padding: '20px 0' }} />;
      }

      return (
        <div style={{ padding: '8px 0' }}>
          <div style={{ display: 'flex', justifyContent: 'flex-end', marginBottom: 8 }}>
            <Button
              icon={<IconCopy />}
              size='small'
              theme='borderless'
              onClick={() => copySection(t('请求内容'), prompt)}
            >
              {t('复制')}
            </Button>
          </div>
          {renderContentBlock(prompt)}
        </div>
      );
    }

    if (hasResponsesStructuredRecord) {
      if (responsesRequestBlocks.length === 0 && responsesToolResponses.length === 0) {
        return <Empty description={t('无请求内容记录')} style={{ padding: '20px 0' }} />;
      }

      return (
        <div style={{ padding: '8px 0' }}>
          <div style={{ display: 'flex', justifyContent: 'flex-end', marginBottom: 8 }}>
            <Button
              icon={<IconCopy />}
              size='small'
              theme='borderless'
              onClick={() => copySection(t('请求内容'), {
                requestBlocks: responsesRequestBlocks,
                toolResponses: responsesToolResponses,
              })}
            >
              {t('复制')}
            </Button>
          </div>
          <div style={{ display: 'flex', flexDirection: 'column', gap: 16 }}>
            <div>
              <Text strong>{t('输入内容')}</Text>
              <div style={{ marginTop: 8, display: 'flex', flexDirection: 'column', gap: 12 }}>
                {responsesRequestBlocks.length > 0 ? (
                  responsesRequestBlocks.map((block, index) => (
                    <div
                      key={`${block.type || 'block'}-${block.role || 'role'}-${index}`}
                      style={{
                        border: '1px solid var(--semi-color-border)',
                        borderRadius: 10,
                        background: 'var(--semi-color-bg-1)',
                        padding: 12,
                      }}
                    >
                      <div style={{ display: 'flex', justifyContent: 'space-between', marginBottom: 8, gap: 12 }}>
                        <Text strong>{t('输入片段')} #{index + 1}</Text>
                        <Text type='tertiary' size='small'>
                          {[block.role, block.type].filter(Boolean).join(' · ') || 'input_text'}
                        </Text>
                      </div>
                      {block.text ? (
                        renderClaudeRequestBlockContent(block.text)
                      ) : (
                        <Text type='tertiary'>{t('该输入片段无可展示文本')}</Text>
                      )}
                    </div>
                  ))
                ) : (
                  <Empty description={t('无输入片段记录')} style={{ padding: '20px 0' }} />
                )}
              </div>
            </div>
            <div>
              <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', marginBottom: 8 }}>
                <Text strong>{t('工具响应')}</Text>
                <Button
                  icon={<IconCopy />}
                  size='small'
                  theme='borderless'
                  onClick={() => copySection(t('工具响应'), responsesToolResponses)}
                >
                  {t('复制')}
                </Button>
              </div>
              {renderResponsesToolResponsesTable()}
            </div>
          </div>
        </div>
      );
    }

    if (hasClaudeStructuredRecord) {
      if (claudeRequestBlocks.length === 0 && claudeToolResponses.length === 0) {
        return <Empty description={t('无请求内容记录')} style={{ padding: '20px 0' }} />;
      }

      return (
        <div style={{ padding: '8px 0' }}>
          <div style={{ display: 'flex', justifyContent: 'flex-end', marginBottom: 8 }}>
            <Button
              icon={<IconCopy />}
              size='small'
              theme='borderless'
              onClick={() => copySection(t('请求内容'), {
                requestBlocks: claudeRequestBlocks,
                toolResponses: claudeToolResponses,
              })}
            >
              {t('复制')}
            </Button>
          </div>
          <div style={{ display: 'flex', flexDirection: 'column', gap: 16 }}>
            <div>
              <Text strong>{t('输入内容')}</Text>
              <div style={{ marginTop: 8, display: 'flex', flexDirection: 'column', gap: 12 }}>
                {claudeRequestBlocks.length > 0 ? (
                  claudeRequestBlocks.map((block, index) => (
                    <div
                      key={`${block.type || 'block'}-${index}`}
                      style={{
                        border: '1px solid var(--semi-color-border)',
                        borderRadius: 10,
                        background: 'var(--semi-color-bg-1)',
                        padding: 12,
                      }}
                    >
                      <div style={{ display: 'flex', justifyContent: 'space-between', marginBottom: 8 }}>
                        <Text strong>{t('输入片段')} #{index + 1}</Text>
                        <Text type='tertiary' size='small'>{block.type || 'text'}</Text>
                      </div>
                      {block.text ? (
                        renderClaudeRequestBlockContent(block.text)
                      ) : (
                        <Text type='tertiary'>{t('该输入片段无可展示文本')}</Text>
                      )}
                    </div>
                  ))
                ) : (
                  <Empty description={t('无输入片段记录')} style={{ padding: '20px 0' }} />
                )}
              </div>
            </div>
            <div>
              <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', marginBottom: 8 }}>
                <Text strong>{t('工具响应')}</Text>
                <Button
                  icon={<IconCopy />}
                  size='small'
                  theme='borderless'
                  onClick={() => copySection(t('工具响应'), claudeToolResponses)}
                >
                  {t('复制')}
                </Button>
              </div>
              {renderClaudeToolResponsesTable()}
            </div>
          </div>
        </div>
      );
    }

    const lastUserMessage = prompt?.lastUserMessage;
    const input = prompt?.input;
    const instructions = prompt?.instructions;
    const promptText = prompt?.prompt;

    // 提取要显示的内容
    let displayContent = '';
    let displayLabel = '';

    if (lastUserMessage?.content) {
      displayContent = lastUserMessage.content;
      displayLabel = t('用户消息');
    } else if (input) {
      displayContent = typeof input === 'string' ? input : JSON.stringify(input, null, 2);
      displayLabel = t('输入内容');
    } else if (promptText) {
      displayContent = typeof promptText === 'string' ? promptText : JSON.stringify(promptText, null, 2);
      displayLabel = t('Prompt');
    }

    if (!displayContent && Object.keys(prompt).length === 0) {
      return <Empty description={t('无请求内容记录')} style={{ padding: '20px 0' }} />;
    }

    return (
      <div style={{ padding: '8px 0' }}>
        <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', marginBottom: 8 }}>
          {displayLabel && <Text type='tertiary' size='small'>{displayLabel}</Text>}
          <Button
            icon={<IconCopy />}
            size='small'
            theme='borderless'
            onClick={() => copySection(t('请求内容'), prompt)}
          >
            {t('复制')}
          </Button>
        </div>
        {displayContent ? renderContentBlock(displayContent) : renderContentBlock(prompt)}
        {instructions && (
          <div style={{ marginTop: 12 }}>
            <Text type='tertiary' size='small'>{t('指令')}:</Text>
            <div
              style={{
                background: 'var(--semi-color-fill-0)',
                padding: 8,
                borderRadius: 6,
                marginTop: 4,
                fontSize: 12,
              }}
            >
              {typeof instructions === 'string' ? instructions : JSON.stringify(instructions)}
            </div>
          </div>
        )}
      </div>
    );
  };

  const renderCompletion = () => {
    if (hasOpenAIStructuredRecord) {
      const { thinking, answer, toolUses } = openAIResponseSections;
      const hasThinking = thinking.trim() !== '';
      const hasAnswer = answer.trim() !== '';
      const hasToolUses = toolUses.length > 0;

      if (!hasThinking && !hasAnswer && !hasToolUses) {
        return <Empty description={t('无响应内容记录')} style={{ padding: '20px 0' }} />;
      }

      const cardStyle = {
        flex: '1 1 320px',
        minWidth: 0,
        border: '1px solid var(--semi-color-border)',
        borderRadius: 10,
        background: 'var(--semi-color-bg-1)',
        padding: 12,
      };

      return (
        <div style={{ padding: '8px 0' }}>
          <div style={{ display: 'flex', justifyContent: 'flex-end', marginBottom: 8 }}>
            <Button
              icon={<IconCopy />}
              size='small'
              theme='borderless'
              onClick={() =>
                copySection(t('响应内容'), {
                  thinking,
                  answer,
                  toolUses,
                })
              }
            >
              {t('复制')}
            </Button>
          </div>
          <div style={{ display: 'flex', gap: 16, flexWrap: 'wrap' }}>
            <div style={cardStyle}>
              <Text strong>{t('thinking')}</Text>
              <div style={{ marginTop: 8 }}>
                {hasThinking ? (
                  renderContentBlock(thinking)
                ) : (
                  <Empty description={t('无 thinking 记录')} style={{ padding: '28px 0' }} />
                )}
              </div>
            </div>
            <div style={cardStyle}>
              <Text strong>{t('回答内容')}</Text>
              <div style={{ marginTop: 8 }}>
                {hasAnswer ? (
                  renderContentBlock(answer)
                ) : (
                  <Empty description={t('无回答内容记录')} style={{ padding: '28px 0' }} />
                )}
              </div>
            </div>
          </div>
          <div style={{ marginTop: 16 }}>
            <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', marginBottom: 8 }}>
              <Text strong>{t('工具调用顺序')}</Text>
              <Button
                icon={<IconCopy />}
                size='small'
                theme='borderless'
                onClick={() => copySection(t('工具调用顺序'), toolUses)}
              >
                {t('复制')}
              </Button>
            </div>
            {renderOpenAIToolUsesTable(toolUses)}
          </div>
        </div>
      );
    }

    if (hasResponsesStructuredRecord) {
      const { answer, toolUses } = responsesResponseSections;
      const hasAnswer = answer.trim() !== '';
      const hasToolUses = toolUses.length > 0;

      if (!hasAnswer && !hasToolUses) {
        return <Empty description={t('无响应内容记录')} style={{ padding: '20px 0' }} />;
      }

      return (
        <div style={{ padding: '8px 0' }}>
          <div style={{ display: 'flex', justifyContent: 'flex-end', marginBottom: 8 }}>
            <Button
              icon={<IconCopy />}
              size='small'
              theme='borderless'
              onClick={() =>
                copySection(t('响应内容'), {
                  answer,
                  toolUses,
                })
              }
            >
              {t('复制')}
            </Button>
          </div>
          <div
            style={{
              border: '1px solid var(--semi-color-border)',
              borderRadius: 10,
              background: 'var(--semi-color-bg-1)',
              padding: 12,
            }}
          >
            <Text strong>{t('回答内容')}</Text>
            <div style={{ marginTop: 8 }}>
              {hasAnswer ? (
                renderContentBlock(answer)
              ) : (
                <Empty description={t('无回答内容记录')} style={{ padding: '28px 0' }} />
              )}
            </div>
          </div>
          <div style={{ marginTop: 16 }}>
            <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', marginBottom: 8 }}>
              <Text strong>{t('工具调用顺序')}</Text>
              <Button
                icon={<IconCopy />}
                size='small'
                theme='borderless'
                onClick={() => copySection(t('工具调用顺序'), toolUses)}
              >
                {t('复制')}
              </Button>
            </div>
            {renderResponsesToolUsesTable(toolUses)}
          </div>
        </div>
      );
    }

    if (claudeResponseBlocks.length > 0) {
      const { thinking, answer, toolUses } = claudeResponseSections;
      const hasThinking = thinking.trim() !== '';
      const hasAnswer = answer.trim() !== '';
      const hasToolUses = toolUses.length > 0;

      if (!hasThinking && !hasAnswer && !hasToolUses) {
        return <Empty description={t('无响应内容记录')} style={{ padding: '20px 0' }} />;
      }

      const cardStyle = {
        flex: '1 1 320px',
        minWidth: 0,
        border: '1px solid var(--semi-color-border)',
        borderRadius: 10,
        background: 'var(--semi-color-bg-1)',
        padding: 12,
      };

      return (
        <div style={{ padding: '8px 0' }}>
          <div style={{ display: 'flex', justifyContent: 'flex-end', marginBottom: 8 }}>
            <Button
              icon={<IconCopy />}
              size='small'
              theme='borderless'
              onClick={() =>
                copySection(t('响应内容'), {
                  thinking,
                  answer,
                  toolUses,
                })
              }
            >
              {t('复制')}
            </Button>
          </div>
          <div style={{ display: 'flex', gap: 16, flexWrap: 'wrap' }}>
            <div style={cardStyle}>
              <Text strong>{t('thinking')}</Text>
              <div style={{ marginTop: 8 }}>
                {hasThinking ? (
                  renderContentBlock(thinking)
                ) : (
                  <Empty description={t('无 thinking 记录')} style={{ padding: '28px 0' }} />
                )}
              </div>
            </div>
            <div style={cardStyle}>
              <Text strong>{t('回答内容')}</Text>
              <div style={{ marginTop: 8 }}>
                {hasAnswer ? (
                  renderContentBlock(answer)
                ) : (
                  <Empty description={t('无回答内容记录')} style={{ padding: '28px 0' }} />
                )}
              </div>
            </div>
          </div>
          <div style={{ marginTop: 16 }}>
            <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', marginBottom: 8 }}>
              <Text strong>{t('工具调用顺序')}</Text>
              <Button
                icon={<IconCopy />}
                size='small'
                theme='borderless'
                onClick={() => copySection(t('工具调用顺序'), toolUses)}
              >
                {t('复制')}
              </Button>
            </div>
            {renderClaudeToolUsesTable(toolUses)}
          </div>
        </div>
      );
    }

    const completion = record?.response?.body ?? record?.completion ?? '';
    const isEmptyObject =
      typeof completion === 'object' &&
      completion !== null &&
      !Array.isArray(completion) &&
      Object.keys(completion).length === 0;
    if (completion === '' || completion === null || completion === undefined || isEmptyObject) {
      return <Empty description={t('无响应内容记录')} style={{ padding: '20px 0' }} />;
    }
    return (
      <div style={{ padding: '8px 0' }}>
        <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', marginBottom: 8 }}>
          {typeof completion === 'string' ? (
            <Text type='tertiary' size='small'>
              {t('长度')}: {completion.length} {t('字符')}
            </Text>
          ) : (
            <span />
          )}
          <Button
            icon={<IconCopy />}
            size='small'
            theme='borderless'
            onClick={() => copySection(t('响应内容'), completion)}
          >
            {t('复制')}
          </Button>
        </div>
        {renderContentBlock(completion)}
      </div>
    );
  };

  const renderMeta = () => {
    const meta = record?.meta || {};
    if (!isFullLogRecord || Object.keys(meta).length === 0) {
      return <Empty description={t('无元信息记录')} style={{ padding: '20px 0' }} />;
    }

    return (
      <div style={{ padding: '8px 0' }}>
        <div style={{ display: 'flex', justifyContent: 'flex-end', marginBottom: 8 }}>
          <Button
            icon={<IconCopy />}
            size='small'
            theme='borderless'
            onClick={() => copySection(t('元信息'), meta)}
          >
            {t('复制')}
          </Button>
        </div>
        {renderContentBlock(meta)}
      </div>
    );
  };

  const renderToolInvokes = () => {
    const toolInvokes = Array.isArray(record?.toolInvokes)
      ? record.toolInvokes
      : [];
    if (toolInvokes.length === 0) {
      return <Empty description={t('无工具调用记录')} style={{ padding: '20px 0' }} />;
    }

    const baseColumns = [
      {
        title: t('工具'),
        dataIndex: 'name',
        key: 'name',
        width: 180,
        render: (text, row) => <Text strong>{text || row.id || '-'}</Text>,
      },
      {
        title: t('参数'),
        dataIndex: 'input',
        key: 'input',
        render: renderSimpleTableValue,
      },
    ];

    const legacyColumns = [
      ...baseColumns,
      {
        title: t('结果'),
        dataIndex: 'resultDisplay',
        key: 'resultDisplay',
        render: (value, row) => (
          <div>
            <pre style={{
              margin: 0,
              whiteSpace: 'pre-wrap',
              wordBreak: 'break-word',
              fontSize: 12,
            }}>
              {value || '-'}
            </pre>
            {typeof row.isError === 'boolean' && (
              <Text type={row.isError ? 'danger' : 'success'} size='small'>
                {row.isError ? t('工具执行失败') : t('工具执行成功')}
              </Text>
            )}
          </div>
        ),
      },
    ];

    const dataSource = toolInvokes.map((item, index) => ({
      ...item,
      rowKey: item.id || `${item.name || 'tool'}-${index}`,
      resultDisplay: item.resultText || (item.result == null ? '' : JSON.stringify(item.result, null, 2)),
    }));

    return (
      <div style={{ padding: '8px 0' }}>
        <div style={{ display: 'flex', justifyContent: 'flex-end', marginBottom: 8 }}>
          <Button
            icon={<IconCopy />}
            size='small'
            theme='borderless'
            onClick={() => copySection(t('工具调用'), toolInvokes)}
          >
            {t('复制')}
          </Button>
        </div>
        <Table
          columns={legacyColumns}
          dataSource={dataSource}
          pagination={false}
          size='small'
          bordered
          rowKey='rowKey'
          style={{ fontSize: 12 }}
        />
      </div>
    );
  };

  const panelData = [
    {
      header: t('请求头'),
      content: renderHeaders(),
      key: 'headers',
    },
    {
      header: isFullLogRecord ? t('完整请求') : t('请求内容'),
      content: renderPrompt(),
      key: 'prompt',
    },
    {
      header: isFullLogRecord ? t('完整响应') : t('响应内容'),
      content: renderCompletion(),
      key: 'completion',
    },
    ...(!(hasClaudeStructuredRecord || hasResponsesStructuredRecord || hasOpenAIStructuredRecord) ? [{
      header: t('工具调用'),
      content: renderToolInvokes(),
      key: 'toolInvokes',
    }] : []),
    {
      header: t('元信息'),
      content: renderMeta(),
      key: 'meta',
    },
  ];

  return (
    <Modal
      title={t('日志详情')}
      visible={showLogDetailModal}
      onCancel={() => setShowLogDetailModal(false)}
      footer={null}
      centered
      closable
      maskClosable
      width={1100}
      bodyStyle={{ maxHeight: '70vh', overflow: 'auto' }}
    >
      <div style={{ padding: '8px 0 16px' }}>
        {!record ? (
          <Empty description={t('无详细记录')} style={{ padding: '40px 0' }} />
        ) : (
          <Collapse accordion defaultActiveKey='prompt'>
            {panelData.map((panel) => (
              <Collapse.Panel header={panel.header} itemKey={panel.key} key={panel.key}>
                {panel.content}
              </Collapse.Panel>
            ))}
          </Collapse>
        )}
      </div>
    </Modal>
  );
};

export default LogDetailModal;

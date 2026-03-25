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

  const copySection = async (section, content) => {
    const text = typeof content === 'object' ? JSON.stringify(content, null, 2) : String(content);
    if (await copy(text)) {
      showSuccess(t('{{section}} 已复制', { section }));
    } else {
      showError(t('无法复制到剪贴板，请手动复制'));
    }
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

    const prompt = record?.prompt || {};
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
    const toolInvokes = Array.isArray(record?.toolInvokes) ? record.toolInvokes : [];
    if (toolInvokes.length === 0) {
      return <Empty description={t('无工具调用记录')} style={{ padding: '20px 0' }} />;
    }

    const columns = [
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
        render: (value) => (
          <pre style={{
            margin: 0,
            whiteSpace: 'pre-wrap',
            wordBreak: 'break-word',
            fontSize: 12,
          }}>
            {value == null ? '-' : JSON.stringify(value, null, 2)}
          </pre>
        ),
      },
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
          columns={columns}
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
    {
      header: t('工具调用'),
      content: renderToolInvokes(),
      key: 'toolInvokes',
    },
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
      width={900}
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

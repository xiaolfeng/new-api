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
} from '@douyinfe/semi-ui';
import { IconCopy } from '@douyinfe/semi-icons';
import { copy, showError, showSuccess } from '../../../../helpers';

const { Text } = Typography;

const LogDetailModal = ({
  showLogDetailModal,
  setShowLogDetailModal,
  logDetailTarget,
  t,
}) => {
  const record = useMemo(() => {
    if (!logDetailTarget?.record) return null;
    try {
      return JSON.parse(logDetailTarget.record);
    } catch {
      return null;
    }
  }, [logDetailTarget]);

  const copySection = async (section, content) => {
    const text = typeof content === 'object' ? JSON.stringify(content, null, 2) : String(content);
    if (await copy(text)) {
      showSuccess(t('{{section}} 已复制', { section }));
    } else {
      showError(t('无法复制到剪贴板，请手动复制'));
    }
  };

  const renderHeaders = () => {
    const headers = record?.headers || {};
    const entries = Object.entries(headers);
    if (entries.length === 0) {
      return <Empty description={t('无请求头记录')} style={{ padding: '20px 0' }} />;
    }
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
        <pre style={{
          background: 'var(--semi-color-fill-0)',
          padding: 12,
          borderRadius: 8,
          overflow: 'auto',
          maxHeight: 300,
          fontSize: 12,
          margin: 0,
        }}>
          {entries.map(([key, value]) => (
            <div key={key} style={{ marginBottom: 4 }}>
              <Text strong>{key}:</Text> {value}
            </div>
          ))}
        </pre>
      </div>
    );
  };

  const renderPrompt = () => {
    const prompt = record?.prompt || {};
    const messages = prompt?.messages || [];
    if (messages.length === 0 && Object.keys(prompt).length === 0) {
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
        <pre style={{
          background: 'var(--semi-color-fill-0)',
          padding: 12,
          borderRadius: 8,
          overflow: 'auto',
          maxHeight: 400,
          fontSize: 12,
          margin: 0,
        }}>
          {JSON.stringify(prompt, null, 2)}
        </pre>
      </div>
    );
  };

  const renderCompletion = () => {
    const completion = record?.completion || '';
    if (!completion) {
      return <Empty description={t('无响应内容记录')} style={{ padding: '20px 0' }} />;
    }
    return (
      <div style={{ padding: '8px 0' }}>
        <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', marginBottom: 8 }}>
          <Text type='tertiary' size='small'>
            {t('长度')}: {completion.length} {t('字符')}
          </Text>
          <Button
            icon={<IconCopy />}
            size='small'
            theme='borderless'
            onClick={() => copySection(t('响应内容'), completion)}
          >
            {t('复制')}
          </Button>
        </div>
        <pre style={{
          background: 'var(--semi-color-fill-0)',
          padding: 12,
          borderRadius: 8,
          overflow: 'auto',
          maxHeight: 400,
          fontSize: 12,
          margin: 0,
          whiteSpace: 'pre-wrap',
          wordBreak: 'break-word',
        }}>
          {completion}
        </pre>
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
      header: t('请求内容'),
      content: renderPrompt(),
      key: 'prompt',
    },
    {
      header: t('响应内容'),
      content: renderCompletion(),
      key: 'completion',
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
      width={720}
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

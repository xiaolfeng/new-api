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

import React, { useEffect, useRef, useState } from 'react';
import { Button, Col, Form, Row, Spin, Typography } from '@douyinfe/semi-ui';
import { compareObjects, API, showError, showSuccess, showWarning } from '../../../helpers';
import { useTranslation } from 'react-i18next';

const { Text } = Typography;

export default function SettingsEmptyResponseRetry(props) {
  const { t } = useTranslation();
  const [loading, setLoading] = useState(false);
  const [inputs, setInputs] = useState({
    'retry_setting.empty_response_retry_enabled': false,
    'retry_setting.empty_response_retry_delay_seconds': 0,
    'retry_setting.record_consume_log_detail_enabled': false,
    'retry_setting.full_log_consume_enabled': false,
    'retry_setting.full_log_consume_expires_at': 0,
    'retry_setting.full_log_consume_remaining_seconds': 0,
    'global.responses_to_chat_completions_enabled': false,
  });
  const refForm = useRef();
  const [inputsRow, setInputsRow] = useState(inputs);

  const remainingSeconds = inputs['retry_setting.full_log_consume_remaining_seconds'] || 0;
  const expiresAt = inputs['retry_setting.full_log_consume_expires_at'] || 0;
  const isFullLogActive =
    inputs['retry_setting.full_log_consume_enabled'] && remainingSeconds > 0;

  const formatExpireTime = (timestamp) => {
    if (!timestamp) return '-';
    return new Date(timestamp * 1000).toLocaleString();
  };

  function onSubmit() {
    const updateArray = compareObjects(inputs, inputsRow).filter(
      (item) =>
        item.key !== 'retry_setting.full_log_consume_expires_at' &&
        item.key !== 'retry_setting.full_log_consume_remaining_seconds',
    );
    if (!updateArray.length) return showWarning(t('你似乎并没有修改什么'));

    const requestQueue = updateArray.map((item) => {
      let value = '';
      if (typeof inputs[item.key] === 'boolean') {
        value = String(inputs[item.key]);
      } else {
        value = inputs[item.key];
      }
      return API.put('/api/option/', { key: item.key, value });
    });

    setLoading(true);
    Promise.all(requestQueue)
      .then((res) => {
        if (res.includes(undefined)) return showError(t('部分保存失败，请重试'));
        showSuccess(t('保存成功'));
        props.refresh();
      })
      .catch(() => showError(t('保存失败，请重试')))
      .finally(() => setLoading(false));
  }

  useEffect(() => {
    const currentInputs = {};
    for (let key in props.options) {
      if (Object.keys(inputs).includes(key)) {
        currentInputs[key] = props.options[key];
      }
    }
    setInputs(currentInputs);
    setInputsRow(structuredClone(currentInputs));
    refForm.current?.setValues(currentInputs);
  }, [props.options]);

  useEffect(() => {
    if (!inputsRow['retry_setting.full_log_consume_enabled'] || remainingSeconds <= 0) {
      return undefined;
    }

    const timer = window.setInterval(() => {
      setInputs((prev) => {
        const currentRemaining = prev['retry_setting.full_log_consume_remaining_seconds'] || 0;
        const nextRemaining = currentRemaining - 1;
        if (nextRemaining <= 0) {
          return {
            ...prev,
            'retry_setting.full_log_consume_enabled': false,
            'retry_setting.full_log_consume_expires_at': 0,
            'retry_setting.full_log_consume_remaining_seconds': 0,
          };
        }
        return {
          ...prev,
          'retry_setting.full_log_consume_remaining_seconds': nextRemaining,
        };
      });
    }, 1000);

    return () => window.clearInterval(timer);
  }, [remainingSeconds, inputsRow]);

  return (
    <Spin spinning={loading}>
      <Form values={inputs} getFormApi={(formAPI) => (refForm.current = formAPI)}>
        <Form.Section text={t('空响应重试设置')}>
          <Row gutter={16}>
            <Col xs={24} sm={12} md={8} lg={8} xl={8}>
              <Form.Switch
                field={'retry_setting.empty_response_retry_enabled'}
                label={t('启用空响应重试')}
                size='default'
                checkedText='｜'
                uncheckedText='〇'
                extraText={t(
                  '当上游返回 HTTP 2xx 但响应内容为空（completion_tokens=0）时自动重试'
                )}
                onChange={(value) =>
                  setInputs({
                    ...inputs,
                    'retry_setting.empty_response_retry_enabled': value,
                  })
                }
              />
            </Col>
            <Col xs={24} sm={12} md={8} lg={8} xl={8}>
              <Form.InputNumber
                label={t('重试延迟')}
                step={1}
                min={0}
                suffix={t('秒')}
                extraText={t('空响应重试前等待的秒数，0 表示立即重试')}
                placeholder={''}
                field={'retry_setting.empty_response_retry_delay_seconds'}
                onChange={(value) =>
                  setInputs({
                    ...inputs,
                    'retry_setting.empty_response_retry_delay_seconds': parseInt(value),
                  })
                }
              />
            </Col>
          </Row>
          <Row>
            <Button size='default' onClick={onSubmit}>
              {t('保存空响应重试设置')}
            </Button>
          </Row>
        </Form.Section>
        <Form.Section text={t('日志详细记录设置')}>
          <Row gutter={16}>
            <Col xs={24} sm={12} md={8} lg={8} xl={8}>
              <Form.Switch
                field={'retry_setting.record_consume_log_detail_enabled'}
                label={t('启用 record 日志记录')}
                size='default'
                checkedText='｜'
                uncheckedText='〇'
                extraText={t(
                  '记录摘要请求内容、响应内容、工具调用和过滤后的 HTTP 头'
                )}
                onChange={(value) =>
                  setInputs({
                    ...inputs,
                    'retry_setting.record_consume_log_detail_enabled': value,
                  })
                }
              />
            </Col>
            <Col xs={24} sm={12} md={8} lg={8} xl={8}>
              <Form.Switch
                field={'retry_setting.full_log_consume_enabled'}
                label={t('启用 5 分钟完整日志记录')}
                size='default'
                checkedText='｜'
                uncheckedText='〇'
                extraText={t(
                  '完整记录请求内容、响应内容和 HTTP 头（排除敏感信息），仅允许开启 5 分钟'
                )}
                onChange={(value) =>
                  setInputs({
                    ...inputs,
                    'retry_setting.full_log_consume_enabled': value,
                  })
                }
              />
              <div style={{ marginTop: 8 }}>
                <Text type={isFullLogActive ? 'success' : 'tertiary'} size='small'>
                  {isFullLogActive
                    ? t('完整日志记录剩余 {{count}} 秒', { count: remainingSeconds })
                    : t('完整日志记录已关闭')}
                </Text>
                <br />
                <Text type='tertiary' size='small'>
                  {t('到期时间')}：{formatExpireTime(expiresAt)}
                </Text>
              </div>
            </Col>
          </Row>
          <Row>
            <Button size='default' onClick={onSubmit}>
              {t('保存客制化设置')}
            </Button>
          </Row>
        </Form.Section>
        <Form.Section text={t('Responses 转换设置')}>
          <Row gutter={16}>
            <Col xs={24} sm={12} md={8} lg={8} xl={8}>
              <Form.Switch
                field={'global.responses_to_chat_completions_enabled'}
                label={t('将 Responses 转换为 Chat Completions')}
                size='default'
                checkedText='｜'
                uncheckedText='〇'
                extraText={t(
                  '启用后，对 Responses API 的请求将自动转换为 Chat Completions 格式，适用于不支持 Responses API 的上游提供商'
                )}
                onChange={(value) =>
                  setInputs({
                    ...inputs,
                    'global.responses_to_chat_completions_enabled': value,
                  })
                }
              />
            </Col>
          </Row>
          <Row>
            <Button size='default' onClick={onSubmit}>
              {t('保存 Responses 转换设置')}
            </Button>
          </Row>
        </Form.Section>
      </Form>
    </Spin>
  );
}

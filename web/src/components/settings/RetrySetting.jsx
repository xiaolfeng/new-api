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

import React, { useEffect, useState } from 'react';
import { Card, Spin } from '@douyinfe/semi-ui';
import SettingsEmptyResponseRetry from '../../pages/Setting/Retry/SettingsEmptyResponseRetry';
import { API, showError, toBoolean } from '../../helpers';

const RetrySetting = () => {
  const [inputs, setInputs] = useState({
    'retry_setting.empty_response_retry_enabled': false,
    'retry_setting.empty_response_retry_delay_seconds': 0,
    'retry_setting.record_consume_log_detail_enabled': false,
    'retry_setting.record_consume_log_detail_expires_at': 0,
    'retry_setting.record_consume_log_detail_remaining_seconds': 0,
  });
  const [loading, setLoading] = useState(false);

  const getOptions = async () => {
    const res = await API.get('/api/option/');
    const { success, message, data } = res.data;
    if (success) {
      setInputs((prev) => {
        const newInputs = { ...prev };
        data.forEach((item) => {
          if (typeof prev[item.key] === 'boolean') {
            newInputs[item.key] = toBoolean(item.value);
          } else if (
            item.key === 'retry_setting.empty_response_retry_delay_seconds' ||
            item.key === 'retry_setting.record_consume_log_detail_expires_at' ||
            item.key === 'retry_setting.record_consume_log_detail_remaining_seconds'
          ) {
            newInputs[item.key] = parseInt(item.value) || 0;
          } else {
            newInputs[item.key] = item.value;
          }
        });
        return newInputs;
      });
    } else {
      showError(message);
    }
  };

  async function onRefresh() {
    try {
      setLoading(true);
      await getOptions();
    } catch (error) {
      showError('刷新失败');
    } finally {
      setLoading(false);
    }
  }

  useEffect(() => {
    onRefresh();
  }, []);

  return (
    <Spin spinning={loading} size='large'>
      <Card style={{ marginTop: '10px' }}>
        <SettingsEmptyResponseRetry options={inputs} refresh={onRefresh} />
      </Card>
    </Spin>
  );
};

export default RetrySetting;

import { useCallback, useEffect, useState } from 'react';
import { useTranslation } from 'react-i18next';
import dayjs from 'dayjs';
import { API, showError } from '../../helpers';

export const useModelLogData = () => {
  const { t } = useTranslation();
  const [loading, setLoading] = useState(true);
  const [refreshing, setRefreshing] = useState(false);
  const [hours, setHours] = useState([]);
  const [items, setItems] = useState([]);
  const [summary, setSummary] = useState(null);
  const [lastUpdatedAt, setLastUpdatedAt] = useState(0);

  const fetchRecentTokenRecords = useCallback(async (withLoading = true) => {
    if (withLoading) {
      setLoading(true);
    } else {
      setRefreshing(true);
    }

    try {
      const res = await API.get('/api/token_record/recent', {
        disableDuplicate: true,
      });
      const { success, message, data } = res.data;
      if (!success) {
        showError(message);
        return;
      }

      setHours(Array.isArray(data?.hours) ? data.hours : []);
      setItems(Array.isArray(data?.items) ? data.items : []);
      setSummary(data?.summary || null);
      setLastUpdatedAt(dayjs().unix());
    } catch (error) {
      showError(error);
    } finally {
      if (withLoading) {
        setLoading(false);
      } else {
        setRefreshing(false);
      }
    }
  }, []);

  useEffect(() => {
    fetchRecentTokenRecords(true);
  }, [fetchRecentTokenRecords]);

  return {
    t,
    loading,
    refreshing,
    hours,
    items,
    summary,
    lastUpdatedAt,
    refreshData: () => fetchRecentTokenRecords(false),
  };
};

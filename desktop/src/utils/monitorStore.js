// 监控数据存储和管理

const MONITOR_CONFIG_KEY = 'monitor_configs';
const MONITOR_HISTORY_PREFIX = 'monitor_history_';
const DEFAULT_MONITOR_INTERVAL = 10; // 默认10分钟

/**
 * 加载监控配置
 * @returns {Object} groupName -> config
 */
export function loadMonitorConfigs() {
  try {
    const raw = localStorage.getItem(MONITOR_CONFIG_KEY);
    return raw ? JSON.parse(raw) : {};
  } catch {
    return {};
  }
}

/**
 * 保存监控配置
 * @param {Object} configs
 */
export function saveMonitorConfigs(configs) {
  try {
    localStorage.setItem(MONITOR_CONFIG_KEY, JSON.stringify(configs));
  } catch (error) {
    console.error('Failed to save monitor configs:', error);
  }
}

/**
 * 获取单个分组的监控配置
 * @param {string} groupName
 * @returns {Object|null}
 */
export function getMonitorConfig(groupName) {
  const configs = loadMonitorConfigs();
  return configs[groupName] || null;
}

/**
 * 设置单个分组的监控配置
 * @param {string} groupName
 * @param {Object} config
 */
export function setMonitorConfig(groupName, config) {
  const configs = loadMonitorConfigs();
  configs[groupName] = {
    enabled: Boolean(config.enabled),
    interval: Number(config.interval) || DEFAULT_MONITOR_INTERVAL,
    lastCheck: Number(config.lastCheck) || 0,
    nextCheck: Number(config.nextCheck) || 0,
    autoOptimizeEnabled: Boolean(config.autoOptimizeEnabled),
  };
  saveMonitorConfigs(configs);
}

/**
 * 删除单个分组的监控配置
 * @param {string} groupName
 */
export function deleteMonitorConfig(groupName) {
  const configs = loadMonitorConfigs();
  delete configs[groupName];
  saveMonitorConfigs(configs);

  // 同时删除历史记录
  deleteMonitorHistory(groupName);
}

/**
 * 加载监控历史记录
 * @param {string} groupName
 * @returns {Array}
 */
export function loadMonitorHistory(groupName) {
  try {
    const key = `${MONITOR_HISTORY_PREFIX}${groupName}`;
    const raw = localStorage.getItem(key);
    if (!raw) return [];

    const history = JSON.parse(raw);

    // 过滤掉超过24小时的记录
    const cutoff = Date.now() - 24 * 60 * 60 * 1000;
    return history.filter(entry => entry.timestamp > cutoff);
  } catch {
    return [];
  }
}

/**
 * 保存监控历史记录
 * @param {string} groupName
 * @param {Object} entry - { timestamp, results }
 */
export function saveMonitorHistoryEntry(groupName, entry) {
  try {
    const key = `${MONITOR_HISTORY_PREFIX}${groupName}`;
    let history = loadMonitorHistory(groupName);

    // 添加新记录
    history.push({
      timestamp: entry.timestamp || Date.now(),
      results: entry.results || [],
    });

    // 只保留24小时内的数据
    const cutoff = Date.now() - 24 * 60 * 60 * 1000;
    history = history.filter(h => h.timestamp > cutoff);

    // 限制最大记录数（防止内存溢出）
    const maxEntries = 200; // 大约每7分钟一条，24小时约200条
    if (history.length > maxEntries) {
      history = history.slice(-maxEntries);
    }

    localStorage.setItem(key, JSON.stringify(history));
  } catch (error) {
    console.error('Failed to save monitor history:', error);
  }
}

/**
 * 删除监控历史记录
 * @param {string} groupName
 */
export function deleteMonitorHistory(groupName) {
  try {
    const key = `${MONITOR_HISTORY_PREFIX}${groupName}`;
    localStorage.removeItem(key);
  } catch (error) {
    console.error('Failed to delete monitor history:', error);
  }
}

/**
 * 清空所有监控历史记录
 */
export function clearAllMonitorHistory() {
  try {
    const keys = Object.keys(localStorage);
    keys.forEach(key => {
      if (key.startsWith(MONITOR_HISTORY_PREFIX)) {
        localStorage.removeItem(key);
      }
    });
  } catch (error) {
    console.error('Failed to clear all monitor history:', error);
  }
}

/**
 * 生成健康槽位数据
 * @param {Array} history
 * @param {number} interval - 监控间隔（分钟）
 * @param {string|null} channelKey - 可选，指定渠道 `${siteUrl}||${model}`，只统计该渠道
 * @returns {Array}
 */
export function generateHealthSlots(history, interval = 10, channelKey = null) {
  const now = Date.now();
  const slotDuration = interval * 60 * 1000;
  const totalSlots = Math.floor((24 * 60) / interval); // 默认144格（10分钟间隔）

  // 优化：减少槽位数量以降低 GPU 负载
  // 如果间隔小于20分钟，强制使用20分钟间隔（72格）
  const optimizedInterval = Math.max(interval, 20);
  const optimizedSlotDuration = optimizedInterval * 60 * 1000;
  const optimizedTotalSlots = Math.floor((24 * 60) / optimizedInterval);

  const slots = [];

  for (let i = 0; i < optimizedTotalSlots; i++) {
    const slotEnd = now - (i * optimizedSlotDuration);
    const slotStart = slotEnd - optimizedSlotDuration;

    // 查找该时间段内的检测记录
    const records = history.filter(h =>
      h.timestamp >= slotStart && h.timestamp < slotEnd
    );

    let status = 'empty';
    let tooltip = '';

    if (records.length > 0) {
      // 使用最新的一条记录
      const latest = records[records.length - 1];

      // 如果指定了渠道，只统计该渠道的结果
      let results = latest.results || [];
      if (channelKey) {
        results = results.filter(r => `${r.siteUrl}||${r.model}` === channelKey);
      }

      const successCount = results.filter(r => r.status === 'success').length;
      const totalCount = results.length;

      // 只有当该时间段确实有该渠道的数据时才标记状态
      if (totalCount > 0) {
        const successRate = (successCount / totalCount) * 100;

        if (successRate >= 95) {
          status = 'success';
        } else if (successRate >= 70) {
          status = 'warning';
        } else {
          status = 'error';
        }

        const timeStr = new Date(latest.timestamp).toLocaleString('zh-CN', {
          month: '2-digit',
          day: '2-digit',
          hour: '2-digit',
          minute: '2-digit',
        });

        tooltip = `${timeStr}\n总请求: ${totalCount}\n成功数: ${successCount}\n成功率: ${successRate.toFixed(2)}%`;
      }
    }

    slots.unshift({
      id: `slot_${slotStart}_${slotEnd}`,
      timestamp: slotStart,
      status,
      tooltip,
    });
  }

  return slots;
}

/**
 * 计算分组统计数据
 * @param {Array} history
 * @returns {Object}
 */
export function calculateGroupStats(history) {
  if (history.length === 0) {
    return {
      totalRequests: 0,
      successCount: 0,
      successRate: 0,
      averageResponseTime: 0,
      status: 'unknown',
      lastCheckTime: null,
    };
  }

  let totalRequests = 0;
  let successCount = 0;
  let totalResponseTime = 0;
  let responseTimeCount = 0;

  history.forEach(entry => {
    entry.results.forEach(result => {
      totalRequests++;
      if (result.status === 'success') {
        successCount++;
        if (result.responseTime > 0) {
          totalResponseTime += result.responseTime;
          responseTimeCount++;
        }
      }
    });
  });

  const successRate = totalRequests > 0 ? (successCount / totalRequests) * 100 : 0;
  const averageResponseTime = responseTimeCount > 0 ? totalResponseTime / responseTimeCount : 0;

  let status = 'normal';
  if (successRate < 70) {
    status = 'error';
  } else if (successRate < 95) {
    status = 'warning';
  }

  const lastCheckTime = history.length > 0 ? history[history.length - 1].timestamp : null;

  return {
    totalRequests,
    successCount,
    successRate,
    averageResponseTime,
    status,
    lastCheckTime,
  };
}

// 监控调度器 - 负责定时执行监控任务

import { runRecordQuickTest } from './keyPanelStore.js';
import {
  loadMonitorConfigs,
  setMonitorConfig,
  saveMonitorHistoryEntry,
  getMonitorConfig,
} from './monitorStore.js';

class MonitorScheduler {
  constructor() {
    this.timers = new Map(); // groupName -> timerId
    this.running = new Map(); // groupName -> boolean (是否正在执行)
    this.getGroupRecordsFn = null; // 获取分组记录的函数
    this.onHistoryUpdate = null; // 历史更新回调（用于触发UI刷新）
    this.onCheckComplete = null; // 检测完成回调（用于自动优选队列）
  }

  /**
   * 设置历史更新回调
   * @param {Function} fn
   */
  setOnHistoryUpdate(fn) {
    this.onHistoryUpdate = fn;
  }

  /**
   * 设置检测完成回调（用于自动优选队列）
   * @param {Function} fn - (groupName, results) => void
   */
  setOnCheckComplete(fn) {
    this.onCheckComplete = fn;
  }

  /**
   * 设置获取分组记录的回调函数
   * @param {Function} fn - (groupName) => records[]
   */
  setGetGroupRecordsFn(fn) {
    this.getGroupRecordsFn = fn;
  }

  /**
   * 启动监控
   * @param {string} groupName
   * @param {Object} config
   */
  start(groupName, config) {
    if (!groupName) return;

    // 停止现有的监控
    this.stop(groupName);

    const intervalMs = (config.interval || 10) * 60 * 1000;

    console.log(`[MonitorScheduler] Starting monitor for group: ${groupName}, interval: ${config.interval}m`);

    // 立即执行一次
    this.runCheck(groupName);

    // 设置定时器
    const timerId = setInterval(() => {
      this.runCheck(groupName);
    }, intervalMs);

    this.timers.set(groupName, timerId);

    // 更新配置
    const nextCheck = Date.now() + intervalMs;
    setMonitorConfig(groupName, {
      ...config,
      enabled: true,
      nextCheck,
    });
  }

  /**
   * 停止监控
   * @param {string} groupName
   */
  stop(groupName) {
    const timerId = this.timers.get(groupName);
    if (timerId) {
      clearInterval(timerId);
      this.timers.delete(groupName);
      this.running.delete(groupName);
      console.log(`[MonitorScheduler] Stopped monitor for group: ${groupName}`);
    }

    // 更新配置
    const config = getMonitorConfig(groupName);
    if (config) {
      setMonitorConfig(groupName, {
        ...config,
        enabled: false,
      });
    }
  }

  /**
   * 停止所有监控
   */
  stopAll() {
    console.log('[MonitorScheduler] Stopping all monitors');
    this.timers.forEach((timerId, groupName) => {
      clearInterval(timerId);
      console.log(`[MonitorScheduler] Stopped monitor for group: ${groupName}`);
    });
    this.timers.clear();
    this.running.clear();
  }

  /**
   * 执行检测
   * @param {string} groupName
   */
  async runCheck(groupName) {
    // 防止重复执行
    if (this.running.get(groupName)) {
      console.log(`[MonitorScheduler] Check already running for group: ${groupName}, skipping`);
      return;
    }

    if (!this.getGroupRecordsFn) {
      console.warn('[MonitorScheduler] getGroupRecordsFn not set');
      return;
    }

    // 不设置 running 状态，避免 loading 转圈
    // this.running.set(groupName, true);

    try {
      const records = this.getGroupRecordsFn(groupName);
      if (!records || records.length === 0) {
        console.log(`[MonitorScheduler] No records found for group: ${groupName}`);
        return;
      }

      console.log(`[MonitorScheduler] Running check for group: ${groupName}, ${records.length} records`);

      const results = [];
      const startTime = Date.now();

      // 批量并发执行，每批5个，避免过载
      const batchSize = 5;
      for (let i = 0; i < records.length; i += batchSize) {
        const batch = records.slice(i, i + batchSize);

        // 并发执行当前批次
        const batchResults = await Promise.all(
          batch.map(async (record) => {
            try {
              const result = await runRecordQuickTest(record, new Map());
              // result.quickTestStatus: 'success' | 'warning' | 'error'
              const status = result.quickTestStatus || 'error';
              return {
                siteUrl: record.siteUrl || record.site_url || '',
                model: record.model || record.selectedModel || '',
                status: status === 'success' ? 'success' : 'error', // warning 也算失败
                responseTime: result.quickTestResponseTime || 0,
                errorMessage: status !== 'success' ? result.quickTestRemark : null,
                errorDetail: status !== 'success' ? result.quickTestRemark : null,
              };
            } catch (error) {
              return {
                siteUrl: record.siteUrl || record.site_url || '',
                model: record.model || record.selectedModel || '',
                status: 'error',
                errorMessage: error.message || 'Unknown error',
                errorDetail: error.detail || null, // 保存完整的尝试日志
                responseTime: 0,
              };
            }
          })
        );

        results.push(...batchResults);

        // 批次之间让出 100ms，避免阻塞 UI
        if (i + batchSize < records.length) {
          await new Promise(resolve => setTimeout(resolve, 100));
        }
      }

      const duration = Date.now() - startTime;

      // 保存历史记录
      saveMonitorHistoryEntry(groupName, {
        timestamp: Date.now(),
        results,
      });

      // 触发UI更新
      if (this.onHistoryUpdate) {
        this.onHistoryUpdate(groupName);
      }

      // 触发检测完成回调（用于自动优选队列）
      if (this.onCheckComplete) {
        this.onCheckComplete(groupName, results);
      }

      const successCount = results.filter(r => r.status === 'success').length;
      const successRate = results.length > 0 ? (successCount / results.length) * 100 : 0;

      console.log(
        `[MonitorScheduler] Check completed for group: ${groupName}, ` +
        `${successCount}/${results.length} succeeded (${successRate.toFixed(2)}%), ` +
        `duration: ${(duration / 1000).toFixed(2)}s`
      );

      // 更新配置
      const config = getMonitorConfig(groupName);
      if (config) {
        const intervalMs = (config.interval || 10) * 60 * 1000;
        setMonitorConfig(groupName, {
          ...config,
          lastCheck: Date.now(),
          nextCheck: Date.now() + intervalMs,
        });
      }
    } catch (error) {
      console.error(`[MonitorScheduler] Error running check for group: ${groupName}`, error);
    } finally {
      // 不使用 running 状态
      // this.running.set(groupName, false);
    }
  }

  /**
   * 获取所有活跃的监控
   * @returns {Array} - [{ groupName, config }]
   */
  getActiveMonitors() {
    const configs = loadMonitorConfigs();
    return Object.entries(configs)
      .filter(([_, config]) => config.enabled)
      .map(([groupName, config]) => ({ groupName, config }));
  }

  /**
   * 检查分组是否正在监控
   * @param {string} groupName
   * @returns {boolean}
   */
  isMonitoring(groupName) {
    return this.timers.has(groupName);
  }

  /**
   * 检查分组是否正在执行检测
   * @param {string} groupName
   * @returns {boolean}
   */
  isRunning(groupName) {
    return Boolean(this.running.get(groupName));
  }
}

// 单例
const monitorScheduler = new MonitorScheduler();

// 应用启动时清空所有监控的 enabled 状态（不保存监控状态）
// 这段代码只在模块首次加载时执行一次
(() => {
  const configs = loadMonitorConfigs();
  let hasEnabledMonitor = false;

  Object.entries(configs).forEach(([groupName, config]) => {
    if (config.enabled) {
      hasEnabledMonitor = true;
      setMonitorConfig(groupName, {
        ...config,
        enabled: false,
        nextCheck: 0,
      });
    }
  });

  if (hasEnabledMonitor) {
    console.log('[MonitorScheduler] Cleared all enabled monitors on app startup');
  }
})();

export default monitorScheduler;

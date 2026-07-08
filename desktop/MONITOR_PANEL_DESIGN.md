# 监控面板功能设计文档

## 功能概述

在密钥管理页面新增"监控"标签页，对自定义分组进行持续可用性监控，展示 24 小时滑动窗口内的健康状况。

## UI 设计

### 整体布局

```
┌────────────────────────────────────────────────────────────┐
│  KEY  |  DISPATCH  |  [监控]  ← 新增标签                    │
├────────────────────────────────────────────────────────────┤
│                                                              │
│  ┌─────────────────────┐  ┌─────────────────────┐          │
│  │ 主流聊天模型 ✓      │  │ 推理模型      ☐    │          │
│  │ claude-opus-4-8     │  │ deepseek-r1        │          │
│  │ ●●●●●○○●●●●●●●●●   │  │ ○○○○○○○○○○○○○○○   │          │
│  │ 正常 100.00% 8请求  │  │ 未监控              │          │
│  │ 1小时前  30分钟前 现在│  │                    │          │
│  └─────────────────────┘  └─────────────────────┘          │
│                                                              │
│  ┌─────────────────────┐                                    │
│  │ API测试组    ✓      │                                    │
│  │ api.example.com/gpt │                                    │
│  │ ●●●○●●●●●●●●●●●●    │                                    │
│  │ 97.5% 成功率 40请求 │                                    │
│  └─────────────────────┘                                    │
└────────────────────────────────────────────────────────────┘
```

### 悬浮卡片设计（参考图片）

```vue
<div class="monitor-card">
  <!-- 顶部：分组名 + 开关 -->
  <div class="monitor-card-header">
    <h3>主流聊天模型</h3>
    <a-switch v-model="group.monitorEnabled" />
  </div>

  <!-- 内容：每个站点/模型一行 -->
  <div class="monitor-card-body">
    <div class="monitor-item">
      <div class="monitor-item-label">
        <span class="monitor-icon">●</span>
        <span>claude-opus-4-8</span>
        <a-tag color="success" size="small">正常</a-tag>
      </div>
      <div class="monitor-item-stats">
        <span class="monitor-success-rate">100.00%</span>
        <span class="monitor-request-count">成功率 · 8 请求</span>
      </div>
    </div>

    <!-- 健康条 -->
    <div class="monitor-health-bar">
      <div class="health-bar-grid">
        <!-- 144格 = 24小时 * 6次/小时（10分钟间隔） -->
        <div v-for="slot in healthSlots" :key="slot.id" 
             :class="['health-slot', slot.status]"
             @mouseenter="showTooltip(slot)">
        </div>
      </div>
      <div class="health-bar-timeline">
        <span>1 小时前</span>
        <span>30 分钟前</span>
        <span>现在</span>
      </div>
    </div>
  </div>
</div>
```

## 数据结构

### 监控配置

```typescript
interface MonitorConfig {
  groupName: string;          // 分组名称
  enabled: boolean;           // 是否启用监控
  interval: number;           // 监控间隔（分钟，默认10）
  lastCheck: number;          // 上次检测时间戳
  nextCheck: number;          // 下次检测时间戳
}

interface MonitorHistory {
  groupName: string;
  timestamp: number;          // 检测时间戳
  results: SiteModelResult[]; // 每个站点/模型的结果
}

interface SiteModelResult {
  siteUrl: string;
  model: string;
  status: 'success' | 'warning' | 'error';  // 成功/警告/失败
  responseTime: number;       // 响应时间（毫秒）
  errorMessage?: string;      // 错误消息
}

interface HealthSlot {
  id: string;
  timestamp: number;
  status: 'success' | 'warning' | 'error' | 'empty';  // 绿/黄/红/灰
  tooltip: string;            // 悬浮提示内容
}
```

### 数据持久化

```
localStorage 结构:
{
  "monitor_configs": {
    "主流聊天模型": {
      "enabled": true,
      "interval": 10,
      "lastCheck": 1720412340000
    }
  },
  "monitor_history": {
    "主流聊天模型": [
      {
        "timestamp": 1720412340000,
        "results": [
          {
            "siteUrl": "https://api.example.com",
            "model": "gpt-4",
            "status": "success",
            "responseTime": 1234
          }
        ]
      }
    ]
  }
}
```

## 核心逻辑

### 1. 监控调度器

```javascript
class MonitorScheduler {
  constructor() {
    this.timers = new Map();  // groupName -> timerId
    this.configs = new Map(); // groupName -> MonitorConfig
  }

  // 启动监控
  start(groupName, config) {
    if (this.timers.has(groupName)) {
      this.stop(groupName);
    }

    const intervalMs = config.interval * 60 * 1000;
    
    // 立即执行一次
    this.runCheck(groupName);

    // 设置定时器
    const timerId = setInterval(() => {
      this.runCheck(groupName);
    }, intervalMs);

    this.timers.set(groupName, timerId);
    this.configs.set(groupName, config);
  }

  // 停止监控
  stop(groupName) {
    const timerId = this.timers.get(groupName);
    if (timerId) {
      clearInterval(timerId);
      this.timers.delete(groupName);
    }
  }

  // 执行检测
  async runCheck(groupName) {
    const group = this.getGroupRecords(groupName);
    const results = [];

    for (const record of group.records) {
      try {
        const result = await runRecordQuickTest(record);
        results.push({
          siteUrl: record.siteUrl,
          model: record.model,
          status: result.status,
          responseTime: result.responseTime
        });
      } catch (error) {
        results.push({
          siteUrl: record.siteUrl,
          model: record.model,
          status: 'error',
          errorMessage: error.message
        });
      }
    }

    // 保存历史记录
    this.saveHistory(groupName, {
      timestamp: Date.now(),
      results
    });
  }

  // 获取分组记录
  getGroupRecords(groupName) {
    // 从现有的 filteredRecords 和 customGroups 中获取
    const records = window.getAllRecords();
    return records.filter(r => r.customGroup === groupName);
  }

  // 保存历史记录
  saveHistory(groupName, historyEntry) {
    const key = `monitor_history_${groupName}`;
    let history = JSON.parse(localStorage.getItem(key) || '[]');
    
    // 添加新记录
    history.push(historyEntry);

    // 只保留 24 小时内的数据
    const cutoff = Date.now() - 24 * 60 * 60 * 1000;
    history = history.filter(h => h.timestamp > cutoff);

    localStorage.setItem(key, JSON.stringify(history));
  }
}
```

### 2. 健康条生成

```javascript
function generateHealthSlots(history, interval = 10) {
  const now = Date.now();
  const oneDayAgo = now - 24 * 60 * 60 * 1000;
  const slotDuration = interval * 60 * 1000;  // 10分钟 = 600000ms
  const totalSlots = Math.floor((24 * 60) / interval);  // 144个格子

  const slots = [];
  
  for (let i = 0; i < totalSlots; i++) {
    const slotEnd = now - (i * slotDuration);
    const slotStart = slotEnd - slotDuration;

    // 查找该时间段内的检测记录
    const records = history.filter(h => 
      h.timestamp >= slotStart && h.timestamp < slotEnd
    );

    let status = 'empty';
    let tooltip = '';

    if (records.length > 0) {
      const latest = records[records.length - 1];
      const successCount = latest.results.filter(r => r.status === 'success').length;
      const totalCount = latest.results.length;
      const successRate = totalCount > 0 ? (successCount / totalCount) * 100 : 0;

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
        minute: '2-digit'
      });

      tooltip = `${timeStr}\n总请求: ${totalCount}\n成功数: ${successCount}\n成功率: ${successRate.toFixed(2)}%`;
    }

    slots.unshift({
      id: `slot_${i}`,
      timestamp: slotStart,
      status,
      tooltip
    });
  }

  return slots;
}
```

### 3. 统计计算

```javascript
function calculateGroupStats(history) {
  if (history.length === 0) {
    return {
      totalRequests: 0,
      successRate: 0,
      averageResponseTime: 0,
      status: 'unknown'
    };
  }

  let totalRequests = 0;
  let successRequests = 0;
  let totalResponseTime = 0;

  history.forEach(entry => {
    entry.results.forEach(result => {
      totalRequests++;
      if (result.status === 'success') {
        successRequests++;
        totalResponseTime += result.responseTime || 0;
      }
    });
  });

  const successRate = totalRequests > 0 ? (successRequests / totalRequests) * 100 : 0;
  const averageResponseTime = successRequests > 0 ? totalResponseTime / successRequests : 0;

  let status = 'normal';
  if (successRate < 70) {
    status = 'error';
  } else if (successRate < 95) {
    status = 'warning';
  }

  return {
    totalRequests,
    successRate,
    averageResponseTime,
    status
  };
}
```

## 样式设计

### 主题色

```scss
// 状态颜色
$status-success: #52c41a;    // 绿色 - 正常
$status-warning: #faad14;    // 黄色 - 警告
$status-error: #ff4d4f;      // 红色 - 错误
$status-empty: #d9d9d9;      // 灰色 - 无数据

// 背景和边框
$card-bg: rgba(255, 255, 255, 0.72);
$card-border: rgba(90, 117, 79, 0.12);
$card-shadow: 0 16px 36px rgba(98, 119, 84, 0.08);
```

### 监控卡片样式

```scss
.monitor-card {
  background: $card-bg;
  border: 1px solid $card-border;
  border-radius: 20px;
  padding: 20px;
  margin-bottom: 20px;
  box-shadow: $card-shadow;
  backdrop-filter: blur(8px);
  transition: all 0.3s ease;

  &:hover {
    transform: translateY(-2px);
    box-shadow: 0 20px 40px rgba(98, 119, 84, 0.12);
  }

  .monitor-card-header {
    display: flex;
    justify-content: space-between;
    align-items: center;
    margin-bottom: 16px;

    h3 {
      font-size: 18px;
      font-weight: 600;
      color: #2c3e50;
      margin: 0;
    }
  }

  .monitor-item {
    display: flex;
    justify-content: space-between;
    align-items: center;
    padding: 12px 0;
    border-bottom: 1px solid rgba(0, 0, 0, 0.06);

    &:last-child {
      border-bottom: none;
    }

    .monitor-item-label {
      display: flex;
      align-items: center;
      gap: 8px;

      .monitor-icon {
        font-size: 20px;
        color: $status-success;
      }
    }

    .monitor-item-stats {
      display: flex;
      flex-direction: column;
      align-items: flex-end;

      .monitor-success-rate {
        font-size: 16px;
        font-weight: 600;
        color: #2c3e50;
      }

      .monitor-request-count {
        font-size: 12px;
        color: #8c8c8c;
      }
    }
  }

  .monitor-health-bar {
    margin-top: 16px;

    .health-bar-grid {
      display: grid;
      grid-template-columns: repeat(144, 1fr);  // 144格
      gap: 2px;
      margin-bottom: 8px;
      height: 32px;

      .health-slot {
        border-radius: 3px;
        transition: all 0.2s ease;
        cursor: pointer;

        &.success {
          background: $status-success;
        }

        &.warning {
          background: $status-warning;
        }

        &.error {
          background: $status-error;
        }

        &.empty {
          background: $status-empty;
        }

        &:hover {
          transform: scaleY(1.2);
          z-index: 1;
        }
      }
    }

    .health-bar-timeline {
      display: flex;
      justify-content: space-between;
      font-size: 11px;
      color: #8c8c8c;
    }
  }
}

// 暗色模式
.dark-mode .monitor-card {
  background: rgba(30, 30, 30, 0.8);
  border-color: rgba(255, 255, 255, 0.1);

  h3 {
    color: #e6e6e6;
  }

  .monitor-item {
    border-bottom-color: rgba(255, 255, 255, 0.08);

    .monitor-item-stats .monitor-success-rate {
      color: #e6e6e6;
    }
  }
}
```

## 实现步骤

### 阶段 1: 基础结构（1-2小时）

1. **创建 MonitorPanel 组件**
   - 文件: `src/components/MonitorPanel.vue`
   - 实现标签页切换
   - 基本布局和样式

2. **添加到路由/标签**
   - 在 KeyManagement.vue 中添加"监控"标签
   - 实现标签切换逻辑

3. **数据存储工具**
   - 文件: `src/utils/monitorStore.js`
   - 实现配置和历史记录的读写

### 阶段 2: 核心功能（2-3小时）

1. **监控调度器**
   - 实现 `MonitorScheduler` 类
   - 定时检测逻辑
   - 集成 `runRecordQuickTest`

2. **健康条组件**
   - 文件: `src/components/MonitorHealthBar.vue`
   - 生成 144 格健康条
   - 实现悬浮提示

3. **统计计算**
   - 成功率计算
   - 请求数统计
   - 响应时间平均值

### 阶段 3: UI 完善（1-2小时）

1. **监控卡片**
   - 优雅的卡片设计
   - 动画和过渡效果
   - 响应式布局

2. **设置面板**
   - 监控间隔设置（可调整 5/10/15/30 分钟）
   - 全局开关
   - 数据清理

3. **空状态和加载状态**
   - 无监控分组提示
   - 加载动画
   - 错误处理

### 阶段 4: 优化和测试（1小时）

1. **性能优化**
   - 防抖节流
   - 虚拟滚动（如果分组很多）
   - 内存管理

2. **用户体验**
   - 暗色模式适配
   - 动画效果
   - 键盘快捷键

3. **测试**
   - 边界情况
   - 长时间运行
   - 数据迁移

## 关键文件清单

### 新增文件

```
src/components/MonitorPanel.vue           # 监控面板主组件
src/components/MonitorCard.vue            # 监控卡片组件
src/components/MonitorHealthBar.vue       # 健康条组件
src/utils/monitorStore.js                 # 监控数据存储
src/utils/monitorScheduler.js             # 监控调度器
```

### 修改文件

```
src/components/KeyManagement.vue          # 添加"监控"标签
src/i18n/catalog.js                       # 添加国际化文本
```

## 国际化文本

```javascript
{
  "monitor": "监控",
  "monitor.enable": "启用监控",
  "monitor.disable": "停止监控",
  "monitor.interval": "监控间隔",
  "monitor.interval.5min": "5 分钟",
  "monitor.interval.10min": "10 分钟",
  "monitor.interval.15min": "15 分钟",
  "monitor.interval.30min": "30 分钟",
  "monitor.status.normal": "正常",
  "monitor.status.warning": "警告",
  "monitor.status.error": "异常",
  "monitor.stats.successRate": "成功率",
  "monitor.stats.requests": "请求",
  "monitor.empty": "暂无监控分组",
  "monitor.empty.desc": "请在密钥管理中创建自定义分组",
  "monitor.clearHistory": "清空历史",
  "monitor.clearHistory.confirm": "确认清空所有监控历史记录？",
  "monitor.tooltip.timestamp": "检测时间",
  "monitor.tooltip.total": "总请求",
  "monitor.tooltip.success": "成功数",
  "monitor.tooltip.successRate": "成功率"
}
```

## 注意事项

1. **性能考虑**
   - 不要同时监控过多分组（建议最多 10 个）
   - 监控间隔不要设置太短（最小 5 分钟）
   - 历史记录只保留 24 小时

2. **用户体验**
   - 监控默认不启用，需要用户手动开启
   - 在后台默默执行，不弹框打扰
   - 提供清晰的状态反馈

3. **错误处理**
   - 网络错误不应该中断监控
   - 记录错误但继续下次检测
   - 提供重试机制

4. **数据迁移**
   - 考虑未来可能的数据格式变更
   - 提供数据导出/导入功能

## 后续扩展

1. **通知功能**
   - 成功率低于阈值时通知
   - 桌面通知集成

2. **报表功能**
   - 导出 CSV
   - 生成周报/月报

3. **对比功能**
   - 多个分组对比
   - 历史趋势图表

4. **告警规则**
   - 自定义告警条件
   - 告警历史记录

---

**预计总开发时间**: 6-8 小时

**优先级**: P1（核心功能）

**复杂度**: 高

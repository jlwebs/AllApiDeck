# 监控面板集成指南

## 已完成的组件

✅ `src/utils/monitorStore.js` - 数据存储和统计计算
✅ `src/utils/monitorScheduler.js` - 监控调度器
✅ `src/components/MonitorHealthBar.vue` - 健康条组件
✅ `src/components/MonitorCard.vue` - 监控卡片组件
✅ `src/components/MonitorPanel.vue` - 监控面板主组件

## 集成步骤

### 1. 在 KeyManagement.vue 中添加监控标签

在现有的分组标签栏后面添加"监控"标签：

```vue
<!-- 在 line 260 附近，全部密钥和分组标签后添加 -->
<button
  type="button"
  class="key-group-tab key-group-tab-monitor"
  :class="{ 'key-group-tab-active': activeKeyGroupId === MONITOR_GROUP_ID }"
  @click="setActiveKeyGroup(MONITOR_GROUP_ID)"
>
  <FundProjectionScreenOutlined />
  <span>监控</span>
</button>
```

### 2. 添加常量和引入

在 `<script setup>` 部分添加：

```javascript
import MonitorPanel from './MonitorPanel.vue';
import { FundProjectionScreenOutlined } from '@ant-design/icons-vue';

// 添加监控组ID常量
const MONITOR_GROUP_ID = '__monitor__';
```

### 3. 修改内容区域渲染逻辑

在密钥表格渲染的地方添加条件判断：

```vue
<!-- 在现有的密钥表格div外层包裹一个条件渲染 -->
<div v-if="activeKeyGroupId === MONITOR_GROUP_ID" class="monitor-panel-wrapper">
  <MonitorPanel
    :key-groups="keyGroups"
    :get-group-records="getGroupRecordsForMonitor"
  />
</div>

<div v-else class="key-table-wrapper">
  <!-- 现有的密钥表格内容 -->
  <a-table ... />
</div>
```

### 4. 添加获取分组记录的方法

```javascript
// 为监控面板提供获取分组记录的函数
function getGroupRecordsForMonitor(groupName) {
  if (!groupName) return [];
  
  const group = keyGroups.value.find(g => g.name === groupName);
  if (!group) return [];

  // 返回该分组下的所有记录
  return allSortedRows.value.filter(record => {
    const recordGroupIds = normalizeRecordGroupIds(record?.groupIds);
    return recordGroupIds.includes(group.id);
  });
}
```

### 5. 添加样式

```scss
.key-group-tab-monitor {
  display: flex;
  align-items: center;
  gap: 6px;
  margin-left: auto; // 推到右边
  
  &:hover {
    background: rgba(24, 144, 255, 0.1);
  }
}

.monitor-panel-wrapper {
  width: 100%;
  min-height: 400px;
}
```

## 完整的修改位置参考

### KeyManagement.vue 修改清单

1. **Import 部分** (line ~100)
```javascript
import MonitorPanel from './MonitorPanel.vue';
import { FundProjectionScreenOutlined } from '@ant-design/icons-vue';
```

2. **常量定义** (line ~1400)
```javascript
const MONITOR_GROUP_ID = '__monitor__';
```

3. **模板部分** (line ~260)
```vue
<!-- 在分组标签后添加 -->
<button
  type="button"
  class="key-group-tab key-group-tab-monitor"
  :class="{ 'key-group-tab-active': activeKeyGroupId === MONITOR_GROUP_ID }"
  @click="setActiveKeyGroup(MONITOR_GROUP_ID)"
>
  <FundProjectionScreenOutlined />
  监控
</button>
```

4. **内容区域** (line ~800, 密钥表格位置)
```vue
<!-- 根据 activeKeyGroupId 条件渲染 -->
<MonitorPanel v-if="activeKeyGroupId === MONITOR_GROUP_ID" ... />
<div v-else><!-- 现有密钥表格 --></div>
```

5. **方法定义** (line ~4000, 在其他方法附近)
```javascript
function getGroupRecordsForMonitor(groupName) {
  // 实现如上
}
```

## 测试步骤

1. **创建测试分组**
   - 在密钥管理中创建一个自定义分组
   - 添加几个测试密钥到该分组

2. **打开监控面板**
   - 点击"监控"标签
   - 应该看到刚创建的分组卡片

3. **启动监控**
   - 点击分组卡片右上角的开关
   - 应该立即执行一次检测
   - 查看健康条是否显示绿色格子

4. **等待定时检测**
   - 等待10分钟（或设置的间隔）
   - 观察是否自动执行新的检测
   - 查看健康条是否新增格子

5. **调整间隔**
   - 修改全局监控间隔
   - 观察是否重新调度

6. **测试暗色模式**
   - 切换到暗色模式
   - 检查所有组件的显示效果

## 注意事项

1. **性能**
   - 不要同时监控太多分组（建议 ≤10 个）
   - 监控间隔不要太短（最小 5 分钟）
   - 每次检测会串行执行，避免过载

2. **数据清理**
   - 历史记录自动保留 24 小时
   - 提供"清空历史"按钮手动清理
   - localStorage 有容量限制（通常 5MB）

3. **用户体验**
   - 监控默认关闭，需手动启用
   - 后台静默执行，不弹框
   - 实时显示下次检测时间

4. **错误处理**
   - 单个密钥失败不影响其他密钥
   - 网络错误会标记为红色格子
   - 控制台输出详细日志便于调试

## 后续优化建议

1. **通知功能**
   - 成功率低于阈值时发送通知
   - 集成桌面通知 API

2. **导出功能**
   - 导出监控报告为 CSV
   - 生成可视化图表

3. **对比功能**
   - 多个分组横向对比
   - 历史趋势曲线图

4. **告警规则**
   - 自定义告警条件
   - 告警历史记录

## 故障排查

### 监控不执行
- 检查控制台是否有错误
- 确认 `getGroupRecordsForMonitor` 返回正确数据
- 检查 localStorage 是否被禁用

### 健康条不显示
- 检查历史记录是否保存成功
- 查看浏览器控制台的 localStorage
- 确认时间戳计算正确

### 样式错乱
- 检查暗色模式类名是否正确
- 确认 CSS scoped 作用域
- 验证父容器的宽度

---

**预计集成时间**: 30-60 分钟
**难度**: 中等

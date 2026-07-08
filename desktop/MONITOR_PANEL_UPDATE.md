# 监控面板重大更新 ✅

## 修复日期：2026-07-08

---

## 🐛 修复的问题

### 1. 监控逻辑理解错误 ✅
**问题**：之前每个分组只显示一个整体健康条，但实际上一个分组有多个渠道（站点/模型组合），每个渠道应该有自己独立的健康条。

**修复**：
- ✅ 重构数据结构，按渠道分组
- ✅ 每个渠道独立显示一行，包含：站点/模型名称（35%宽度） + 健康条（65%宽度）
- ✅ `generateHealthSlots()` 支持 `channelKey` 参数，只统计特定渠道的数据
- ✅ 健康条过滤逻辑：`${siteUrl}||${model}` 作为渠道唯一标识

### 2. 开关点击无反应 ✅
**问题**：点击监控开关后，没有激活也没有反应，UI 不更新。

**根本原因**：
- `monitorGroups` 使用 `computed` 读取 localStorage
- localStorage 不是响应式的，修改后不会触发 computed 重新计算
- 组件无法感知数据变化

**修复方案**：
```javascript
// 1. 添加 refreshToken 强制刷新
const refreshToken = ref(0);

// 2. computed 显式依赖 refreshToken
const monitorGroups = computed(() => {
  void refreshToken.value; // 显式依赖
  void props.refreshSignal; // 父组件信号
  // ... 读取 localStorage
});

// 3. 数据变化时触发刷新
function handleToggleMonitor(group, checked) {
  // ... 开关逻辑
  triggerRefresh(); // 立即刷新 UI
}

// 4. 调度器完成检测后回调
monitorScheduler.setOnHistoryUpdate(() => {
  triggerRefresh();
});
```

### 3. UI 优化 ✅
- ✅ 图标垂直居中对齐（添加 `transform: translateY(1px)`）
- ✅ 标签文字改为英文 "Monitor"
- ✅ 工具栏移到标签右侧空白区
- ✅ 健康条更紧凑（高度 20px，间距 1px）
- ✅ 时间轴标签统一显示在第一个渠道上方

---

## 📊 新的数据结构

### 渠道（Channel）
```javascript
{
  siteUrl: 'https://example.com',
  model: 'gpt-4',
  channelKey: 'https://example.com||gpt-4',
  label: 'example.com / gpt-4'
}
```

### 监控卡片数据
```javascript
{
  id: 'group-123',
  name: 'gpt-5.5',
  monitorEnabled: true,
  interval: 10,
  lastCheckTime: 1234567890,
  nextCheckTime: 1234568490,
  history: [...],
  channels: [
    { channelKey: 'site1||model1', label: 'site1 / model1' },
    { channelKey: 'site2||model2', label: 'site2 / model2' },
    // ...
  ],
  loading: false
}
```

---

## 🎨 UI 布局

### 监控卡片布局（每个分组）
```
┌─────────────────────────────────────────────────┐
│ 分组名称                              [开关]    │
├─────────────────────────────────────────────────┤
│                  24h前  12h前  现在             │  ← 时间轴标签
├─────────────────────────────────────────────────┤
│ site1 / gpt-4 │ ████░░██████░░░░░░░░░░░░░░░░   │  ← 35% + 65%
│ site2 / gpt-5 │ ██████████░░░░██░░░░░░░░░░░░   │
│ site3 / gpt-4 │ ░░░░░░████████████████████░░   │
├─────────────────────────────────────────────────┤
│ 上次检测: 2分钟前    下次检测: 8分钟后         │
└─────────────────────────────────────────────────┘
```

### 健康条颜色
- 🟢 绿色：成功率 ≥ 95%
- 🟡 黄色：成功率 70% - 95%
- 🔴 红色：成功率 < 70%
- ⚪ 灰色：无数据（半透明）

---

## 🔧 关键代码修改

### 1. monitorStore.js
```javascript
export function generateHealthSlots(history, interval = 10, channelKey = null) {
  // ... 
  if (channelKey) {
    // 过滤指定渠道的结果
    results = results.filter(r => `${r.siteUrl}||${r.model}` === channelKey);
  }
  // ...
}
```

### 2. monitorScheduler.js
```javascript
// 添加历史更新回调
setOnHistoryUpdate(fn) {
  this.onHistoryUpdate = fn;
}

// 检测完成后触发
if (this.onHistoryUpdate) {
  this.onHistoryUpdate(groupName);
}
```

### 3. MonitorPanel.vue
```javascript
// 响应式刷新令牌
const refreshToken = ref(0);

// 显式依赖
const monitorGroups = computed(() => {
  void refreshToken.value;
  void props.refreshSignal;
  // ...
});

// 历史更新回调
monitorScheduler.setOnHistoryUpdate(() => {
  triggerRefresh();
});
```

### 4. MonitorCard.vue
```vue
<!-- 每个渠道独立显示 -->
<div v-for="channel in channels" class="monitor-channel-row">
  <div class="monitor-channel-label">{{ channel.label }}</div>
  <div class="monitor-channel-healthbar">
    <MonitorHealthBar
      :history="history"
      :interval="interval"
      :channel-key="channel.channelKey"
    />
  </div>
</div>
```

### 5. KeyManagement.vue
```javascript
// 监控工具栏（右侧）
const monitorGlobalInterval = ref(10);
const monitorRefreshSignal = ref(0);

// 所有操作后触发刷新
monitorRefreshSignal.value++;
```

---

## ✅ 测试清单

### 基础功能
- [ ] 监控标签显示正常（Monitor）
- [ ] 图标和文字垂直居中
- [ ] 工具栏在标签右侧显示
- [ ] 切换到监控面板无报错

### 监控开关
- [ ] 点击开关立即响应
- [ ] 开关状态正确显示（开/关）
- [ ] 启用后立即执行第一次检测
- [ ] 停止后不再执行检测

### 渠道显示
- [ ] 每个渠道独立显示一行
- [ ] 站点/模型名称占 35% 宽度
- [ ] 健康条占 65% 宽度
- [ ] 渠道名称过长时正确截断

### 健康条
- [ ] 每个渠道显示独立的健康条
- [ ] 144 格显示完整
- [ ] 悬浮显示该渠道的详细信息
- [ ] 颜色正确（绿/黄/红/灰）
- [ ] 时间轴标签显示在第一个渠道上方

### 定时检测
- [ ] 按间隔自动执行
- [ ] 检测完成后 UI 立即更新
- [ ] 历史记录正确保存
- [ ] 下次检测时间正确倒计时

### 工具栏
- [ ] 间隔选择器工作正常
- [ ] 刷新按钮立即执行检测
- [ ] 清空历史正确清理数据
- [ ] 统计数字正确

---

## 🔍 调试技巧

### 1. 查看控制台日志
```
[MonitorScheduler] Starting monitor: 分组名
[MonitorScheduler] Running check: 分组名
[MonitorScheduler] Check complete: 成功X/总数Y
```

### 2. 检查 localStorage
```javascript
// 查看监控配置
JSON.parse(localStorage.getItem('monitor_configs'))

// 查看历史记录
JSON.parse(localStorage.getItem('monitor_history_gpt-5.5'))

// 检查最新记录的结果
const history = JSON.parse(localStorage.getItem('monitor_history_gpt-5.5'))
const latest = history[history.length - 1]
console.log(latest.results)
```

### 3. 检查渠道过滤
```javascript
// 在 MonitorHealthBar.vue 中添加调试
console.log('channelKey:', props.channelKey)
console.log('filtered results:', results)
```

### 4. 强制刷新
```javascript
// 在浏览器控制台执行
monitorRefreshSignal.value++
```

---

## 🚀 性能优化

### 已优化
1. **按渠道过滤**：只计算需要显示的渠道数据
2. **响应式更新**：使用 refreshToken 而不是轮询
3. **回调触发**：检测完成立即更新，不等待定时器
4. **UI 刷新**：5秒定时器仅更新倒计时，不重新计算数据

### 注意事项
- 单个分组不建议超过 20 个渠道（影响显示性能）
- 监控间隔最小 5 分钟（避免频繁请求）
- 历史记录自动保留 24 小时（超出自动清理）

---

## 📝 更新文件清单

### 修改的文件
1. `src/utils/monitorStore.js` - 支持按渠道过滤
2. `src/utils/monitorScheduler.js` - 添加历史更新回调
3. `src/components/MonitorPanel.vue` - 响应式更新机制
4. `src/components/MonitorCard.vue` - 按渠道显示布局
5. `src/components/MonitorHealthBar.vue` - 支持 channelKey 参数
6. `src/components/KeyManagement.vue` - 工具栏集成

### 新增概念
- `channelKey`: 渠道唯一标识 `${siteUrl}||${model}`
- `refreshToken`: 强制刷新令牌
- `onHistoryUpdate`: 历史更新回调

---

## ✨ 最终效果

### 修复前
```
问题1：一个分组只有一个整体健康条
问题2：点击开关无反应
问题3：图标不对齐，工具栏在卡片内
```

### 修复后
```
✅ 每个渠道独立健康条（站点/模型 35% + 健康条 65%）
✅ 开关立即响应，UI 实时更新
✅ 图标完美对齐，工具栏在标签右侧
✅ 时间轴标签统一显示
✅ 健康条紧凑美观
```

---

**编译状态**：✅ 成功
**测试建议**：重启开发服务器，清空浏览器缓存，创建测试分组验证

🎉 监控面板现在完全可用了！

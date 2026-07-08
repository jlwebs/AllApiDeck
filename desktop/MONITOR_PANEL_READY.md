# 监控面板集成完成 ✅

## 已完成的集成

✅ 添加"监控"标签按钮（在 KEY/DISPATCH 后）
✅ 引入 MonitorPanel 组件
✅ 引入 FundProjectionScreenOutlined 图标
✅ 添加条件渲染逻辑
✅ 实现 getGroupRecordsForMonitor 方法
✅ 更新 setActiveInventoryPanel 函数
✅ 添加样式定义
✅ 编译测试通过

## 如何使用

### 1. 启动应用

```bash
npm run dev
# 或者如果有桌面应用
npm run wails:dev
```

### 2. 打开监控面板

1. 进入密钥管理页面
2. 点击顶部的 **KEY** | **DISPATCH** | **监控** 标签
3. 选择"监控"标签

### 3. 创建测试分组（如果还没有）

在 KEY 标签页中：
1. 点击"快捷分组"按钮
2. 创建一个新的分组，比如"测试分组"
3. 添加几个密钥到该分组

### 4. 启用监控

在监控面板中：
1. 找到刚创建的分组卡片
2. 点击右上角的开关启用监控
3. 立即执行第一次检测
4. 观察健康条显示绿色格子

### 5. 调整设置

- **监控间隔**：在顶部工具栏选择 5/10/15/30 分钟
- **刷新全部**：手动触发所有监控任务
- **清空历史**：删除所有历史记录

## 功能验证清单

### 基础功能
- [ ] 监控标签显示正常
- [ ] 切换到监控面板无报错
- [ ] 显示所有自定义分组
- [ ] 空状态提示正常

### 监控功能
- [ ] 启用监控开关工作正常
- [ ] 立即执行第一次检测
- [ ] 健康条显示正确（绿色=成功）
- [ ] 统计信息正确（成功率、请求数）
- [ ] 状态标签正确（正常/警告/异常）

### 定时检测
- [ ] 按设定间隔自动执行（默认10分钟）
- [ ] 下次检测时间显示正确
- [ ] 后台静默执行，无弹框
- [ ] 历史记录正确保存

### 健康条
- [ ] 144格显示完整
- [ ] 悬浮显示详细信息（时间、请求数、成功率）
- [ ] 颜色正确（绿=正常、黄=警告、红=异常、灰=无数据）
- [ ] 时间轴标签正确

### UI/UX
- [ ] 卡片悬浮效果正常
- [ ] 暗色模式适配正常
- [ ] 响应式布局正常
- [ ] 动画过渡流畅

### 工具栏
- [ ] 全局间隔选择器工作正常
- [ ] 刷新全部按钮工作正常
- [ ] 清空历史按钮工作正常（有确认框）
- [ ] 统计数字正确（X/Y 个分组已监控）

## 常见问题排查

### 1. 监控标签不显示
- 检查浏览器控制台是否有错误
- 确认 MonitorPanel.vue 文件存在
- 检查导入路径是否正确

### 2. 监控不执行
- 检查是否有自定义分组
- 确认分组下有密钥记录
- 查看控制台日志 `[MonitorScheduler]`

### 3. 健康条不显示
- 等待第一次检测完成
- 检查 localStorage 是否有数据
- 查看浏览器控制台错误

### 4. 样式错乱
- 清空浏览器缓存
- 重新编译前端 `npm run build`
- 检查暗色模式是否正确应用

## 数据存储

### localStorage 键名

```
monitor_configs - 监控配置
monitor_history_<分组名> - 每个分组的历史记录
```

### 查看数据

在浏览器控制台执行：

```javascript
// 查看所有监控配置
JSON.parse(localStorage.getItem('monitor_configs'))

// 查看特定分组的历史
JSON.parse(localStorage.getItem('monitor_history_测试分组'))

// 清空所有监控数据
Object.keys(localStorage).forEach(key => {
  if (key.startsWith('monitor_')) {
    localStorage.removeItem(key);
  }
})
```

## 性能注意事项

### 建议的使用限制

- **最多监控分组**：10 个
- **最小监控间隔**：5 分钟
- **每个分组最多密钥**：50 个

### 为什么有这些限制？

1. **浏览器性能**：过多的定时任务会影响页面性能
2. **网络负载**：频繁的检测会产生大量网络请求
3. **存储限制**：localStorage 通常只有 5MB 空间

### 如果需要监控更多？

考虑：
1. 减少监控间隔（使用 15 或 30 分钟）
2. 合并相似的分组
3. 只监控关键的分组

## 下一步优化方向

### 短期（1-2天）

1. **通知功能**
   - 成功率低于阈值时发送通知
   - 桌面通知集成

2. **导出功能**
   - 导出为 CSV
   - 生成可视化报告

### 中期（1周）

1. **图表功能**
   - 趋势曲线图
   - 多分组对比

2. **告警规则**
   - 自定义告警条件
   - 告警历史记录

### 长期（1个月+）

1. **服务端监控**
   - 将监控任务移到后端
   - 支持更长时间的历史记录

2. **高级分析**
   - 响应时间分析
   - 故障模式识别
   - 预测性维护

## 技术架构

```
MonitorPanel.vue (主面板)
  ├── MonitorCard.vue (卡片组件)
  │   └── MonitorHealthBar.vue (健康条)
  ├── monitorStore.js (数据存储)
  └── monitorScheduler.js (定时调度)
```

### 调用流程

```
1. 用户点击开关 → MonitorCard.emit('toggle')
2. MonitorPanel.handleToggleMonitor()
3. monitorScheduler.start(groupName, config)
4. monitorScheduler.runCheck(groupName)
5. runRecordQuickTest(record) × N
6. saveMonitorHistoryEntry(groupName, results)
7. 更新 UI（每5秒自动刷新）
```

## 文件清单

### 新增文件
- `src/utils/monitorStore.js`
- `src/utils/monitorScheduler.js`
- `src/components/MonitorHealthBar.vue`
- `src/components/MonitorCard.vue`
- `src/components/MonitorPanel.vue`

### 修改文件
- `src/components/KeyManagement.vue`
  - 添加监控标签
  - 引入组件
  - 添加方法和样式

## 成功标志

当你看到以下情况时，说明监控面板已经成功运行：

1. ✅ 监控标签可见且可点击
2. ✅ 监控面板显示分组卡片
3. ✅ 启用监控后健康条显示绿色格子
4. ✅ 控制台输出 `[MonitorScheduler]` 日志
5. ✅ localStorage 中有 `monitor_` 开头的数据

---

**恭喜！监控面板已完全集成并可以使用了！** 🎉

如果遇到任何问题，请检查：
1. 浏览器控制台的错误信息
2. localStorage 的数据
3. 网络请求是否成功

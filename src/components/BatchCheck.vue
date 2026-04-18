<template>
  <ConfigProvider :theme="configProviderTheme">
    <div class="wrapper batch-wrapper">
      <div class="batch-shell" :class="{ 'batch-shell-motion-active': step === 1 || step === -1 }">
        <div class="batch-forest-scene" aria-hidden="true">
          <div class="forest-mist forest-mist-left"></div>
          <div class="forest-mist forest-mist-right"></div>
          <div class="forest-path-glow"></div>
          <div class="forest-firegrass firegrass-left"></div>
          <div class="forest-firegrass firegrass-right"></div>
          <div class="forest-slime slime-a"></div>
          <div class="forest-slime slime-b"></div>
          <div class="forest-slime slime-c"></div>
        </div>
        <div class="page-content batch-page-content">
          <div class="container batch-page-container">
            <!-- Header section, similar to Check.vue for consistency -->
            <AppHeader
              current-page="batch"
              :is-dark-mode="isDarkMode"
              @experimental="showExperimentalFeatures = true"
              @settings="openSettingsModal"
            />

            <section class="batch-hero" :class="{ 'batch-hero-compact': step !== 1 }">
              <div class="batch-hero-motion" aria-hidden="true">
                <span class="leaf leaf-a"></span>
                <span class="leaf leaf-b"></span>
                <span class="leaf leaf-c"></span>
                <span class="leaf leaf-d"></span>
                <span class="grass grass-a"></span>
                <span class="grass grass-b"></span>
                <span class="grass grass-c"></span>
              </div>

              <div class="batch-hero-head">
                <div class="batch-hero-copy">
                  <p class="batch-hero-kicker">Batch Workspace</p>
                  <div class="page-title-row">
                    <div class="page-title-block">
                      <h1 class="page-title">
                        批量导入中转站 
                      </h1>
                      <p class="page-subtitle">
                        推荐从扩展一键导入/浏览器桥识别导入，或者从扩展备份文件恢复。
                      </p>
                    </div>
                    <a-tooltip v-if="showBackendHealth" :title="backendHealthTooltip">
                      <div
                        class="backend-health-pill"
                        :class="{
                          'backend-health-ok': backendHealth.ok,
                          'backend-health-down': backendHealth.checked && !backendHealth.ok,
                        }"
                      >
                        <span class="backend-health-dot"></span>
                        <span class="backend-health-label">
                          {{ backendHealth.ok ? '本地后端正常' : (backendHealth.checked ? '本地后端异常' : '本地后端检测中') }}
                        </span>
                      </div>
                    </a-tooltip>
                  </div>
                  <div class="batch-hero-meta">
                    <span class="batch-hero-tag">扩展导入优先</span>
                    <span class="batch-hero-tag">JSON 备用导入</span>
                  </div>
                </div>
              </div>

              <div v-show="step === 1" class="step-container step-container-hero">
                <div class="hero-stage-grid">
                  <div class="hero-left-stack">
                    <div class="hero-primary-pair">
                      <div class="hero-action-card hero-action-card-bridge hero-action-card-large hero-action-card-recommend">
                        <span class="hero-card-watermark">RECOMMEND</span>
                        <div class="hero-action-copy">
                          <h3>当前浏览器标签直接导入</h3>
                          <p class="hero-copy-half-gap">基于浏览器拓展桥直接导入你正在浏览的中转站。 【兼容性最高】</p>
                        </div>
                        <a-button type="dashed" class="hero-secondary-button hero-bridge-button" @click="handleDirectTabImport">
                          <TagsOutlined /> 当前标签导入
                        </a-button>
                      </div>

                      <div class="hero-action-card hero-action-card-primary hero-action-card-compact">
                        <div class="hero-action-copy">
                          <h3>从浏览器扩展导入</h3>
                          <p class="hero-copy-half-gap">直接从本机扩展插件“ALL API HUB”文件数据完成一键导入。</p>
                        </div>
                        <div class="hero-primary-inline">
                          <a-button
                            v-if="isWailsRuntime"
                            type="primary"
                            size="large"
                            @click="importFromExtension"
                            :disabled="isImportingExtension"
                            class="hero-primary-button"
                          >
                            <AppstoreOutlined /> {{ isImportingExtension ? '正在读取扩展数据...' : '从浏览器扩展导入' }}
                          </a-button>
                          <p v-if="!isWailsRuntime" class="hero-action-note">当前环境非桌面模式，可改用右侧 JSON 导入。</p>
                        </div>
                      </div>
                    </div>

                    <div class="hero-action-card hero-action-card-secondary hero-action-card-compact">
                      <div class="hero-action-copy">
                        <h3>查看上一次检测结果</h3>
                        <p>直接回到最近一次结果树。</p>
                      </div>
                      <a-button v-if="hasHistory" @click="loadHistory" type="dashed" class="hero-secondary-button">
                        <HistoryOutlined /> 查看上一次检测结果
                      </a-button>
                      <p v-else class="hero-action-note">当前还没有历史记录。</p>
                    </div>
                  </div>

                  <div class="hero-upload-card hero-upload-card-right">
                    <div class="hero-upload-copy">
                      <h3>导入备份 JSON</h3>
                      <p>备用入口，从All-API-Hub备份文件导入`。</p>
                    </div>
                    <a-upload-dragger
                      name="file"
                      :multiple="false"
                      :before-upload="beforeUpload"
                      :show-upload-list="false"
                      accept=".json"
                      class="hero-upload-dragger"
                    >
                      <p class="ant-upload-drag-icon">
                        <FileTextOutlined />
                      </p>
                      <p class="ant-upload-text">点击或拖入 accounts-backup.json</p>
                      <p class="ant-upload-hint">解析后自动拉起模型列表读取</p>
                    </a-upload-dragger>
                  </div>
                </div>

                <div
                  v-if="isWailsRuntime && importExtensionStatusText"
                  class="extension-import-status-line"
                >
                  <a-tag :color="importExtensionStatusColor">{{ importExtensionStatusText }}</a-tag>
                </div>
              </div>
            </section>

            <!-- 加载状态 -->
            <div v-show="isLoadingModels && step === -1" class="step-container loading-container">
              <a-spin size="large" />
              <p style="margin-top: 20px;">{{ loadingStageTitle }}</p>
              <p style="margin-top: 8px; color: #8c8c8c;">{{ loadingStageDescription }}</p>
              <p v-if="loadingStageMeta" style="margin-top: 4px; color: #bfbfbf; font-size: 12px;">{{ loadingStageMeta }}</p>
              <div v-if="loadingStageStatusText" class="loading-stage-status-line">
                <a-tag :color="loadingStageStatusColor">{{ loadingStageStatusText }}</a-tag>
              </div>
            </div>

            <!-- 步骤 2：树形选择器选择想要检查的模型 -->
            <div v-show="step === 2" class="step-container">
              <div class="selection-topbar">
                <div class="selection-header-row">
                  <h3 class="selection-title">请勾选需要测试的网站与模型</h3>
                  <a-space wrap class="selection-action-group">
                    <a-button @click="selectAllNodes" size="small">全部全选</a-button>
                    <a-button @click="unselectAllNodes" size="small">全部反选</a-button>
                    <a-button @click="selectChatModelsOnly" size="small">仅选主流聊天</a-button>
                  </a-space>
                </div>
                <div class="selection-quick-filters">
                  <div class="quick-filter-toolbar">
                    <div class="quick-filter-strip" v-if="quickFilters.length">
                      <a-popover
                        v-for="family in quickFilters"
                        :key="family.key"
                        trigger="hover"
                        placement="bottomLeft"
                        overlayClassName="quick-filter-family-popover"
                      >
                        <template #content>
                          <div class="quick-filter-family-panel">
                            <div class="quick-filter-family-panel-title">{{ family.label }}</div>
                            <div class="quick-filter-option-list">
                              <a-button
                                v-for="option in family.options"
                                :key="option.key"
                                size="small"
                                :type="activeQuickFilters.includes(option.key) ? 'primary' : 'default'"
                                @click="toggleQuickFilter(option.key)"
                              >
                                {{ option.label }}
                              </a-button>
                              <a-button
                                size="small"
                                class="quick-filter-family-select-all"
                                @click="selectQuickFilterFamily(family)"
                              >
                                {{ isQuickFilterFamilyFullySelected(family) ? '取消' : '全选' }}
                              </a-button>
                            </div>
                          </div>
                        </template>
                        <a-button
                          class="quick-filter-family-trigger"
                          :type="isQuickFilterFamilyActive(family) ? 'primary' : 'default'"
                          @click="selectQuickFilterFamily(family)"
                        >
                          {{ family.label }}
                          <span v-if="getQuickFilterFamilyActiveCount(family)" class="quick-filter-family-count">
                            {{ getQuickFilterFamilyActiveCount(family) }}
                          </span>
                        </a-button>
                      </a-popover>
                      <a-button
                        class="quick-filter-clear-trigger"
                        @click="clearQuickFilters"
                        :disabled="!activeQuickFilters.length"
                      >
                        清空
                      </a-button>
                    </div>
                    <div v-else class="quick-filter-empty-inline">暂无可用快捷分组</div>
                    <span v-if="activeQuickFilterSummary" class="quick-filter-summary">{{ activeQuickFilterSummary }}</span>
                  </div>
                </div>
              </div>

              <div
                v-if="isDiscoveringModels || browserSessionPolling.active"
                style="display:flex; align-items:center; gap:8px; margin-bottom: 12px; color:#1677ff;"
              >
                <a-spin size="small" />
                <span v-if="isDiscoveringModels">模型发现进行中（{{ loadedSitesCount }} / {{ totalAccountsCount }}）</span>
                <span v-if="isDiscoveringModels && browserSessionPolling.active">，</span>
                <span v-if="browserSessionPolling.active">受控浏览器后台检测中（{{ browserSessionPolling.round }} / {{ browserSessionPolling.totalRounds }}），剩余 {{ browserSessionPolling.pending }} 个站点</span>
              </div>

              <div class="tree-wrapper">
                <a-tree
                  v-model:checkedKeys="checkedKeys"
                  :expanded-keys="selectionExpandedKeys"
                  :tree-data="treeData"
                  checkable
                  @expand="handleSelectionTreeExpand"
                >
                  <template #title="node">
                    <div class="custom-tree-node-wrapper tree-provider-node-wrapper" style="display: flex; align-items: center;">
                      <div class="provider-tree-label">
                        <button
                          v-if="canOpenProviderSiteFromTreeNode(node) && getProviderTreeTitle(node)"
                          type="button"
                          :class="['provider-tree-link', { 'is-grey': node.isProviderDiagnostic || node.titleClass === 'tree-node-grey' || node.siteDisabled }]"
                          @click.stop="openProviderSiteFromTreeNode(node)"
                        >
                          {{ getProviderTreeTitle(node) }}
                        </button>
                        <span
                          v-if="canOpenProviderSiteFromTreeNode(node) && getProviderTreeSuffix(node)"
                          :class="['custom-tree-node', node.titleClass]"
                        >
                          {{ getProviderTreeSuffix(node) }}
                        </span>
                        <span
                          v-else
                          :class="['custom-tree-node', node.titleClass]"
                        >
                          {{ node.title }}
                        </span>
                        <span v-if="node.isManualToken" class="site-tree-inline-tag">手动添加</span>
                        <a-popconfirm
                          v-if="node.isManualToken"
                          title="确认删除这个手动添加的 key？"
                          @confirm="handleTreeManualTokenDelete(node)"
                        >
                          <button type="button" class="site-tree-inline-delete-btn" @click.stop>
                            <DeleteOutlined />
                          </button>
                        </a-popconfirm>
                        <span v-if="node.isSiteRoot && node.siteNote" class="site-tree-note-badge">
                          {{ node.siteNote }}
                        </span>
                      </div>
                      <span v-if="node.isModelDiscovering || node.isBrowserPending" class="tree-node-pending-hint">
                        <a-spin size="small" />
                        <span>{{ node.isModelDiscovering ? (node.modelDiscoveringHint || '模型检测中') : node.pendingHint }}</span>
                      </span>
                      <div v-if="node.isSiteRoot" class="site-tree-actions">
                        <a-tooltip title="重新加载">
                          <button type="button" class="site-tree-action-btn" @click.stop="handleTreeSiteRefresh(node)">
                            <ReloadOutlined />
                          </button>
                        </a-tooltip>
                        <a-tooltip title="手动追加自定义 sk">
                          <button type="button" class="site-tree-action-btn" @click.stop="handleTreeSiteCustomSk(node)">
                            <LockOutlined />
                          </button>
                        </a-tooltip>
                        <a-tooltip :title="node.siteDisabled ? '激活该站点' : '禁用该站点'">
                          <button type="button" class="site-tree-action-btn" @click.stop="handleTreeSiteToggleDisabled(node)">
                            <CheckCircleOutlined v-if="node.siteDisabled" />
                            <StopOutlined v-else />
                          </button>
                        </a-tooltip>
                        <a-tooltip title="设置 10 字以内备注">
                          <button type="button" class="site-tree-action-btn" @click.stop="handleTreeSiteEditNote(node)">
                            <MessageOutlined />
                          </button>
                        </a-tooltip>
                        <a-popconfirm title="确认删除该站点缓存？" @confirm="handleTreeSiteDelete(node)">
                          <button type="button" class="site-tree-action-btn is-danger" @click.stop>
                            <DeleteOutlined />
                          </button>
                        </a-popconfirm>
                      </div>
                      <div v-if="isProviderDiagnosticTreeNode(node)" class="provider-tree-actions">
                        <a-popover trigger="hover" placement="rightTop" overlayClassName="provider-diagnostic-popover">
                          <template #content>
                            <div class="provider-diagnostic-menu">
                              <a-button size="small" @click.stop="copyProviderFetchReplay(node)">复制 fetch 复现</a-button>
                              <a-button size="small" @click.stop="copyProviderTraceLog(node)">复制调研 trace 日志</a-button>
                            </div>
                          </template>
                          <span class="provider-diagnostic-trigger" @click.stop>调试</span>
                        </a-popover>
                      </div>
                    </div>
                  </template>
                </a-tree>
              </div>

                <div class="settings-action-bar">
                  <div class="batch-settings">
                    <span class="batch-settings-label">并发数</span>
                    <a-input-number v-model:value="batchConcurrency" :min="1" :max="100" class="batch-setting-input" />
                    <span class="batch-settings-label">超时(秒)</span>
                    <a-input-number v-model:value="modelTimeout" :min="1" class="batch-setting-input" />
                  </div>
                  <div class="actions">
                    <a-button class="batch-reset-button" @click="resetStep1">重新导入</a-button>
                    <a-button class="batch-start-button" type="primary" size="large" @click="startBatchCheck" :disabled="isDiscoveringModels">
                    <PlayCircleOutlined /> 开始检测
                    </a-button>
                  </div>
                </div>
            </div>

            <!-- 步骤 3：显示检测结果 -->
            <div v-show="step === 3" class="step-container result-container">
              <div style="display: flex; justify-content: space-between; align-items: center; margin-bottom: 10px;">
                <h3 style="margin: 0; cursor: pointer; user-select: none;" @click="isTableExpanded = !isTableExpanded">
                  <DownOutlined v-if="isTableExpanded" style="margin-right: 8px;" />
                  <RightOutlined v-else style="margin-right: 8px;" />
                  批量检测结果
                </h3>
              </div>
              <div
                v-if="browserSessionPolling.active"
                style="display:flex; align-items:center; gap:8px; margin-bottom: 10px; color:#1677ff;"
              >
                <a-spin size="small" />
                <span>受控浏览器后台检测中（{{ browserSessionPolling.round }} / {{ browserSessionPolling.totalRounds }}），剩余 {{ browserSessionPolling.pending }} 个站点...</span>
              </div>
              <div v-show="isTableExpanded">
                <div class="result-topbar">
                  <div class="quick-filter-toolbar">
                    <div class="quick-filter-strip" v-if="quickFilters.length">
                      <a-popover
                        v-for="family in quickFilters"
                        :key="family.key"
                        trigger="hover"
                        placement="bottomLeft"
                        overlayClassName="quick-filter-family-popover"
                      >
                        <template #content>
                          <div class="quick-filter-family-panel">
                            <div class="quick-filter-family-panel-title">{{ family.label }}</div>
                            <div class="quick-filter-option-list">
                              <a-button
                                v-for="option in family.options"
                                :key="option.key"
                                size="small"
                                :type="activeQuickFilters.includes(option.key) ? 'primary' : 'default'"
                                @click="toggleQuickFilter(option.key)"
                              >
                                {{ option.label }}
                              </a-button>
                              <a-button
                                size="small"
                                class="quick-filter-family-select-all"
                                @click="selectQuickFilterFamily(family)"
                              >
                                {{ isQuickFilterFamilyFullySelected(family) ? '取消' : '全选' }}
                              </a-button>
                            </div>
                          </div>
                        </template>
                        <a-button
                          class="quick-filter-family-trigger"
                          :type="isQuickFilterFamilyActive(family) ? 'primary' : 'default'"
                          @click="selectQuickFilterFamily(family)"
                        >
                          {{ family.label }}
                          <span v-if="getQuickFilterFamilyActiveCount(family)" class="quick-filter-family-count">
                            {{ getQuickFilterFamilyActiveCount(family) }}
                          </span>
                        </a-button>
                      </a-popover>
                      <a-button
                        class="quick-filter-clear-trigger"
                        @click="clearQuickFilters"
                        :disabled="!activeQuickFilters.length"
                      >
                        清空
                      </a-button>
                    </div>
                    <div v-else class="quick-filter-empty-inline">暂无可用快捷分组</div>
                    <span v-if="activeQuickFilterSummary" class="quick-filter-summary">{{ activeQuickFilterSummary }}</span>
                  </div>

                  <div class="result-side-controls">
                    <a-input-search
                      v-model:value="resultModelFilter"
                      placeholder="模型过滤：空格分隔关键字（如 gpt-5.2 codex）"
                      allow-clear
                    >
                      <template #prefix><SearchOutlined /></template>
                    </a-input-search>

                    <a-space wrap class="result-action-group">
                      <a-dropdown-button @click="copyOrganizedResults" :disabled="testing || !testResults.length">
                        <CopyOutlined /> 整理有效配置
                        <template #overlay>
                          <a-menu>
                            <a-menu-item key="2" @click="copyAllConfigs">
                              <CopyOutlined /> 复制全表配置
                            </a-menu-item>
                          </a-menu>
                        </template>
                      </a-dropdown-button>
                      <a-button @click="retestAllFromResults" :disabled="testing || !testResults.length">
                        <RedoOutlined /> 再测一次
                      </a-button>
                      <a-button v-if="hasHistory && !testing" @click="loadHistory">
                        <HistoryOutlined /> 恢复历史
                      </a-button>
                      <a-button danger v-if="testing" @click="stopTesting">停止检测</a-button>
                      <a-button v-else @click="resetStep2">返回选择面板</a-button>
                    </a-space>
                  </div>
                </div>
                <a-progress :percent="testProgress" show-info style="margin-bottom: 15px" />

                <a-table
                  :columns="resultColumns"
                  :data-source="currentResultData"
                  :pagination="tablePagination"
                  @change="handleTableChange"
                  :row-class-name="record => record.id === highlightedTaskId ? 'highlighted-row' : ''"
                  size="small"
                  row-key="id"
                >
                  <!-- ... table slots ... -->
                  <template #bodyCell="{ column, record }">
                  <template v-if="column.dataIndex === 'siteName'">
                    <a-tooltip :title="record.quota" placement="top">
                      <a href="" @click.prevent="openUrlInSystemBrowser(record.siteUrl)" @mouseenter="hoverQuota(record)">
                        {{ record.siteName }}
                      </a>
                    </a-tooltip>
                  </template>
                  <template v-else-if="column.dataIndex === 'payload'">
                    <a-tooltip placement="top">
                      <template #title>
                        <pre style="max-width:300px; white-space:pre-wrap; margin:0; font-size:12px;">{{ getPayloadJson(record) }}</pre>
                      </template>
                      <div style="cursor: pointer; user-select: none;" @dblclick="openPayloadEditor(record)">
                        {{ getMaskedKey(record.apiKey) }}
                      </div>
                    </a-tooltip>
                  </template>
                  <template v-else-if="column.dataIndex === 'status'">
                    <a-tooltip placement="topLeft">
                      <template #title>
                        <pre style="max-width:560px; max-height:420px; overflow:auto; white-space:pre-wrap; margin:0; font-size:12px;">{{ getStatusTooltip(record) }}</pre>
                      </template>
                      <a-tag :color="getStatusColor(record.status)" style="cursor: pointer;">
                        {{ record.statusText }}
                      </a-tag>
                    </a-tooltip>
                  </template>
                  <template v-else-if="column.dataIndex === 'responseTime'">
                    <div class="result-performance-cell">
                      <span>{{ record.responseTime && record.responseTime !== '-' ? `${record.responseTime}s` : '-' }}</span>
                      <a-tooltip v-if="hasPerformanceMetrics(record)">
                        <template #title>
                          <div class="performance-tooltip-list">
                            <div v-for="line in getPerformanceTooltipLines(record)" :key="line">{{ line }}</div>
                          </div>
                        </template>
                        <span class="performance-badge performance-badge-inline" aria-label="性能指标">
                          <ThunderboltOutlined />
                        </span>
                      </a-tooltip>
                    </div>
                  </template>
                  <template v-else-if="column.dataIndex === 'remark'">
                    <a-tooltip :title="record.remark">
                      <span :style="{ color: record.status === 'error' ? '#ff4d4f' : 'inherit', fontWeight: record.status === 'error' ? 'bold' : 'normal' }">
                        {{ record.remark }}
                      </span>
                    </a-tooltip>
                  </template>
                </template>
              </a-table>
            </div>

              <!-- NEW ORGANIZED AREA -->
              <div v-if="testResults.length > 0" class="organized-section" style="margin-top: 25px; padding-top: 15px; border-top: 2px dashed var(--border-color);">
                <div style="display: flex; align-items: center; justify-content: space-between; margin-bottom: 15px;">
                  <h3 style="margin: 0; cursor: pointer; user-select: none;" @click="isTreeExpanded = !isTreeExpanded">
                    <DownOutlined v-if="isTreeExpanded" style="margin-right: 8px;" />
                  <RightOutlined v-else style="margin-right: 8px;" />
                    <ShareAltOutlined /> 整理与概览
                  </h3>
                  <a-space>
                    <a-button 
                      size="small" 
                      type="link"
                      @click="toggleExpandAll"
                      style="margin-right: 2px"
                    >
                      <template v-if="expandedKeys.length > 0">
                        <MenuFoldOutlined /> 全部折叠
                      </template>
                      <template v-else>
                        <MenuUnfoldOutlined /> 全部展开
                      </template>
                    </a-button>
                    <a-button 
                      size="small" 
                      type="link"
                      :loading="isRefreshingBalances" 
                      @click="refreshAllBalances"
                      style="margin-right: 5px; color: #1677ff;"
                    >
                      <ReloadOutlined v-if="!isRefreshingBalances" /> 更新余额
                    </a-button>
                    <a-checkbox v-model:checked="filterOnlySuccess" style="margin-right: 15px;">
                      仅有效(过滤红色/失败)
                    </a-checkbox>
                    <a-tooltip title="自动同步sk密钥到本地存储">
                      <a-button
                        size="small"
                        type="link"
                        :loading="isSyncingLocalKeys"
                        @click="syncDetectedKeysToLocalStorage()"
                        style="margin-right: 6px; color: #1677ff;"
                      >
                        <CloudSyncOutlined v-if="!isSyncingLocalKeys" /> 同步本地
                      </a-button>
                    </a-tooltip>
                    <a-input-search
                      v-model:value="searchQuery"
                      placeholder="关键字过滤 (空格分隔多词，如 gpt4 claude)"
                      style="width: 400px"
                      allow-clear
                    >
                      <template #prefix><SearchOutlined /></template>
                    </a-input-search>
                  </a-space>
                </div>

                <div v-show="isTreeExpanded" class="organized-tree-wrapper">
                  <div v-if="organizedTreeData.length === 0" style="text-align: center; padding: 40px; color: #999;">
                    没有匹配当前过滤条件的配置
                  </div>
                  <a-tree
                    v-else
                    :tree-data="organizedTreeData"
                    v-model:expanded-keys="expandedKeys"
                    @select="onTreeSelect"
                    class="result-summary-tree"
                    block-node
                  >
                    <template #title="node">
                       <div class="custom-tree-node-wrapper" style="display: flex; align-items: center;">
                         <span :class="['custom-tree-node', node.class]">{{ node.title }}</span>
                         <span v-if="node.isBrowserPending" class="tree-node-pending-hint">
                           <a-spin size="small" />
                           <span>{{ node.pendingHint }}</span>
                         </span>
                         
                         <!-- 仅在叶子节点（模型项）显示快捷拉起图标，空两格紧跟 -->
                         <div v-if="node.isLeaf" class="shortcut-actions" style="margin-left: 12px; display: flex; gap: 8px;">
                           <a-tooltip title="一键添加到 Cherry Studio">
                             <span class="app-icon cherry-icon" @click.stop="launchCherryStudio(node)">
                               🍒
                             </span>
                           </a-tooltip>
                           <a-tooltip title="一键添加到 CC-Switch">
                             <span class="app-icon switch-icon" @click.stop="launchCCSwitch(node)">
                               🔄
                             </span>
                           </a-tooltip>
                         </div>
                       </div>
                    </template>
                  </a-tree>
                </div>
              </div>
            </div>

            <!-- Payload Editor Modal -->
            <a-modal
              v-model:open="isEditorOpen"
              title="修改并重发请求 Payload"
              @ok="resendPayload"
              ok-text="重发"
              cancel-text="取消"
              destroy-on-close
              width="600px"
            >
              <div style="margin-bottom: 10px; color: #666;">
                在此处修改您想重新测试的 JSON Payload (请确保格式准确)。点击重新发送将直接用此 Payload 请求后端。
              </div>
              <a-textarea v-model:value="editingPayload" :rows="12" style="font-family: monospace;" />
            </a-modal>

            <AdvancedProxyModal v-model:open="showExperimentalFeatures" />

            <TextPromptModal
              v-model:open="textPromptOpen"
              v-model:value="textPromptValue"
              :title="textPromptTitle"
              :placeholder="textPromptPlaceholder"
              :ok-text="textPromptOkText"
              :multiline="textPromptMode === 'sk'"
              :rows="textPromptMode === 'sk' ? 5 : 1"
              :max-length="textPromptMode === 'note' ? SITE_NOTE_MAX_LENGTH : 0"
              :show-count="textPromptMode === 'note'"
              @ok="submitTextPromptModal"
              @cancel="closeTextPromptModal"
            />

            <BridgeImportWizardModal
              :open="bridgeImportModalOpen"
              :opening="bridgeImportSessionOpening"
              :opening-install="bridgeImportOpeningInstall"
              :install-opened="bridgeImportInstallOpened"
              :polling="bridgeImportPolling"
              :importing="bridgeImportImporting"
              :records="bridgeImportRecords"
              :ready-count="bridgeImportReadyCount"
              :last-received-at="bridgeImportLastReceivedAt"
              :session-active="bridgeImportSessionActive"
              :client-ready="bridgeImportClientReady"
              :last-client-ping="bridgeImportLastClientPing"
              :server-url="bridgeImportServerUrl"
              :log-path="bridgeImportLogPath"
              :last-logs="bridgeImportLastLogs"
              @cancel="closeBridgeImportModal"
              @open-install="openBridgeScriptInstallPage"
              @finish-import="finalizeBridgeImportSession"
            />

            <a-modal
              v-model:open="showKeySyncStrategyModal"
              title="请选择密钥更新策略"
              :mask-closable="false"
              :keyboard="false"
              :closable="false"
              @cancel="resolveKeySyncStrategy('keep')"
            >
              <div class="key-sync-strategy-modal">
                <p class="key-sync-strategy-summary">
                  本次检测获取到 {{ pendingKeySyncIncomingCount }} 条密钥，当前密钥管理中已有 {{ pendingKeySyncExistingCount }} 条自动同步记录。
                </p>
                <div class="key-sync-strategy-option">
                  <span class="key-sync-strategy-index">1.</span>
                  <div>
                    <div class="key-sync-strategy-title">增量更新</div>
                    <div class="key-sync-strategy-desc">保留现有自动记录，仅覆盖相同网站 + API Key 的重复项，并追加新的密钥。</div>
                  </div>
                </div>
                <div class="key-sync-strategy-option">
                  <span class="key-sync-strategy-index">2.</span>
                  <div>
                    <div class="key-sync-strategy-title">清空覆盖</div>
                    <div class="key-sync-strategy-desc">清空当前自动同步记录，再用本次检测结果整体重建。</div>
                  </div>
                </div>
                <div class="key-sync-strategy-option">
                  <span class="key-sync-strategy-index">3.</span>
                  <div>
                    <div class="key-sync-strategy-title">不改变</div>
                    <div class="key-sync-strategy-desc">保留现有密钥管理数据，不写入本次自动同步结果。</div>
                  </div>
                </div>
              </div>
              <template #footer>
                <a-space>
                  <a-button type="primary" @click="resolveKeySyncStrategy('merge')">增量更新</a-button>
                  <a-button danger @click="resolveKeySyncStrategy('replace')">清空覆盖</a-button>
                  <a-button @click="resolveKeySyncStrategy('keep')">不改变</a-button>
                </a-space>
              </template>
            </a-modal>

            <SystemSettingsModal
              v-model:open="showAppSettingsModal"
              v-model:tree-expanded="isTreeExpanded"
              v-model:desktop-token-source-mode="desktopTokenSourceMode"
              :is-chrome-profile-auth-available="isChromeProfileAuthAvailable"
              :app-name="appInfo.name"
              :app-version="appInfo.version"
            />
          </div>
        </div>
      </div>
    </div>
  </ConfigProvider>
</template>

<script setup>
import { ref, reactive, computed, onMounted, onBeforeUnmount, watch, nextTick, h } from 'vue';
import { useI18n } from 'vue-i18n';
import { useRouter } from 'vue-router';
import { ConfigProvider, message, theme, Modal } from 'ant-design-vue';
import { HomeOutlined, ReloadOutlined, MenuUnfoldOutlined, MenuFoldOutlined, InboxOutlined, PlayCircleOutlined, SearchOutlined, CopyOutlined, FilterOutlined, HistoryOutlined, ShareAltOutlined, DownOutlined, RightOutlined, UserOutlined, LockOutlined, MessageOutlined, CopyFilled, SmileOutlined, RedoOutlined, CloudSyncOutlined, StopOutlined, CheckCircleOutlined, DeleteOutlined, ThunderboltOutlined } from '@ant-design/icons-vue';
import AppHeader from './AppHeader.vue';
import AdvancedProxyModal from './AdvancedProxyModal.vue';
import BridgeImportWizardModal from './BridgeImportWizardModal.vue';
import TextPromptModal from './TextPromptModal.vue';
import SystemSettingsModal from './SystemSettingsModal.vue';
import { fetchModelList } from '../utils/api.js';
import { listDesktopLogFiles, readDesktopLogFile, isDesktopLogBridgeAvailable } from '../utils/desktopLogBridge.js';
import { apiFetch, isProbablyWailsRuntime, openUrlInSystemBrowser } from '../utils/runtimeApi.js';
import { extractChromeProfileTokens, isChromeProfileAuthBridgeAvailable } from '../utils/profileAuthBridge.js';
import { maximiseMainWindow } from '../utils/windowSizing.js';
import { loadTreeExpandedSetting } from '../utils/systemSettings.js';
import { fetchQuotaLabelWithBatchLogic, isDisplayableQuotaLabel } from '../utils/balance.js';
import { logClientDiagnostic } from '../utils/clientDiagnostics.js';
import { buildQuickTestMessages } from '../utils/quickTestPrompts.js';
import {
  buildRowKey as buildKeyPanelRowKey,
  loadPanelRecords,
  normalizeModels as normalizeKeyPanelModels,
  persistPanelRecords,
} from '../utils/keyPanelStore.js';
import { buildPerformanceTooltipLines, derivePerformanceMetricsFromResponse, hasPerformanceMetrics } from '../utils/performanceMetrics.js';
import {
  appendCustomKeysToSiteCache,
  buildSiteCacheKey,
  buildBatchSitesFromCache,
  consumePendingBatchStart,
  consumePendingSiteRestore,
  deleteSiteCacheRecord,
  findAnySiteCacheRecord,
  loadAllSiteCacheRecords,
  mergeExtractedSitesIntoTempCache,
  mergeExtractedSitesIntoCache,
  normalizeSiteUrl,
  removeCustomKeyFromSiteCache,
  setSiteCacheDisabled,
  updateSiteCacheTreeNodes,
  updateSiteCacheNote,
  writePendingBatchStart,
  writePendingSiteRestore,
} from '../utils/siteCacheStore.js';

const isWailsRuntime = isProbablyWailsRuntime();
const { t } = useI18n();
const router = useRouter();
const isDarkMode = ref(false);
const configProviderTheme = computed(() => ({
  algorithm: isDarkMode.value ? theme.darkAlgorithm : theme.defaultAlgorithm,
}));

// State logic
const step = ref(1); // 1: upload, 2: select tree, 3: result table
const isLoadingModels = ref(false);
const isDiscoveringModels = ref(false);
const isImportingExtension = ref(false);
const importExtensionStatus = ref('');
const importExtensionStatusColor = ref('default');
const importExtensionElapsedSeconds = ref(0);
const showBackendHealth = computed(() => isWailsRuntime);
const backendHealth = reactive({
  ok: false,
  checked: false,
  detail: '等待首次检测',
  debug: '',
});
const totalAccountsCount = ref(0);
const showExperimentalFeatures = ref(false);
const textPromptOpen = ref(false);
const textPromptMode = ref('sk');
const textPromptValue = ref('');
const textPromptSiteCacheKey = ref('');
const bridgeImportModalOpen = ref(false);
const bridgeImportSessionOpening = ref(false);
const bridgeImportOpeningInstall = ref(false);
const bridgeImportPolling = ref(false);
const bridgeImportImporting = ref(false);
const bridgeImportInstallOpened = ref(false);
const bridgeImportRecords = ref([]);
const bridgeImportLastReceivedAt = ref('');
const bridgeImportReadyCount = ref(0);
const bridgeImportServerUrl = ref('');
const bridgeImportLogPath = ref('');
const bridgeImportLastLogs = ref([]);
const bridgeImportSessionActive = ref(false);
const bridgeImportClientReady = ref(false);
const bridgeImportLastClientPing = ref('');
let bridgeImportPollTimer = null;
const showAppSettingsModal = ref(false);
const settingsApiUrl = ref('');
const settingsApiKey = ref('');
const localCacheList = ref([]);
const portablePacking = ref(false);
const portableUnpacking = ref(false);
const portableSettingsMeta = ref('');
const desktopTokenSourceMode = ref('profile_file');
const activeExtractionMode = ref('');
const desktopLogsLoading = ref(false);
const desktopLogContentLoading = ref(false);
const desktopLogFiles = ref([]);
const selectedDesktopLogGroup = ref('');
const selectedDesktopLogPath = ref('');
const selectedDesktopLogContent = ref('');
const isCloudLoggedIn = ref(false);
const cloudUrl = ref('');
const cloudPassword = ref('');
const cloudDataList = ref([]);
const isSyncingLocalKeys = ref(false);
const showKeySyncStrategyModal = ref(false);
const pendingKeySyncExistingCount = ref(0);
const pendingKeySyncIncomingCount = ref(0);

const KEY_MANAGEMENT_STORAGE_KEY = 'api_check_key_management_records_v1';
const KEY_MANAGEMENT_META_STORAGE_KEY = 'api_check_key_management_meta_v1';
const KEY_MANAGEMENT_SYNC_EVENT = 'batch-api-check:key-management-sync';
const SITE_NOTE_MAX_LENGTH = 10;
const textPromptTitle = computed(() =>
  textPromptMode.value === 'sk' ? '手动追加自定义 sk' : '设置 10 字以内备注'
);
const textPromptPlaceholder = computed(() =>
  textPromptMode.value === 'sk'
    ? '请输入一个或多个 sk，支持换行、空格、逗号分隔'
    : `请输入 ${SITE_NOTE_MAX_LENGTH} 个字以内备注`
);
const textPromptOkText = computed(() => (textPromptMode.value === 'sk' ? '追加' : '保存'));
// Temporary kill switch:
// Only disable the built-in WebView2/Profile Assist window fallback.
// Keep the profile-file manual recovery flow alive:
// users may still see the login confirmation prompt, open failed sites,
// and trigger "re-read Profile file" retry rounds in the normal browser.
const PROFILE_FILE_WEBVIEW_FALLBACK_ENABLED = false;
const PROFILE_FILE_MANUAL_RECOVERY_ENABLED = true;
let keySyncStrategyResolver = null;
const activeSiteTreeSession = {
  replaceSites: null,
  requestDiscoveryRefresh: null,
  syncCacheSnapshot: null,
  currentSites: [],
};

const appInfo = reactive({
  name: 'API Checker',
  subtitle: '批量 API 检测工具',
  version: '2.5.0',
  author: { url: 'https://github.com/jlwebs' }
});
const appDescription = ref(['支持 OpenAI / Claude / Gemini / NewAPI 等多种格式接口的批量并发检测与账号管理。']);

const openSettingsModal = () => {
  showAppSettingsModal.value = true;
};

const closeSettingsModal = () => {
  showAppSettingsModal.value = false;
};

const isDesktopLogAvailable = computed(() => isDesktopLogBridgeAvailable());

const desktopLogGroups = computed(() => {
  const groupMap = new Map();
  (Array.isArray(desktopLogFiles.value) ? desktopLogFiles.value : []).forEach(file => {
    const key = String(file?.groupKey || 'other').trim() || 'other';
    const label = String(file?.groupLabel || '其他日志').trim() || '其他日志';
    if (!groupMap.has(key)) {
      groupMap.set(key, { key, label, files: [] });
    }
    groupMap.get(key).files.push(file);
  });
  return Array.from(groupMap.values());
});

const currentDesktopLogGroupFiles = computed(() => {
  const targetGroup = String(selectedDesktopLogGroup.value || '').trim();
  const group = desktopLogGroups.value.find(item => item.key === targetGroup);
  return Array.isArray(group?.files) ? group.files : [];
});

const currentDesktopLogFileMeta = computed(() => {
  const targetPath = String(selectedDesktopLogPath.value || '').trim();
  return currentDesktopLogGroupFiles.value.find(file => String(file?.path || '').trim() === targetPath) || null;
});

const backendHealthTooltip = computed(() => {
  const statusText = backendHealth.ok
    ? '状态：正常'
    : (backendHealth.checked ? '状态：异常' : '状态：检测中');
  const detailText = `详情：${backendHealth.detail || '无'}`;
  const debugText = backendHealth.debug ? `诊断：${backendHealth.debug}` : '';
  return [statusText, detailText, debugText].filter(Boolean).join('\n');
});

const formatLogTimestamp = (ts) => {
  const num = Number(ts || 0);
  if (!num) return '-';
  const date = new Date(num);
  if (Number.isNaN(date.getTime())) return '-';
  return date.toLocaleString();
};

const formatLogSize = (size) => {
  const value = Number(size || 0);
  if (!Number.isFinite(value) || value <= 0) return '0 B';
  if (value < 1024) return `${value} B`;
  if (value < 1024 * 1024) return `${(value / 1024).toFixed(1)} KB`;
  return `${(value / 1024 / 1024).toFixed(1)} MB`;
};

const loadDesktopLogContent = async (path) => {
  const targetPath = String(path || '').trim();
  if (!targetPath || !isDesktopLogAvailable.value) {
    selectedDesktopLogContent.value = '';
    return;
  }

  desktopLogContentLoading.value = true;
  try {
    const result = await readDesktopLogFile(targetPath);
    selectedDesktopLogPath.value = targetPath;
    selectedDesktopLogContent.value = String(result?.content || '');
  } catch (err) {
    selectedDesktopLogContent.value = '';
    message.error(err?.message || '读取日志失败');
  } finally {
    desktopLogContentLoading.value = false;
  }
};

const loadDesktopLogs = async () => {
  if (!isDesktopLogAvailable.value) {
    desktopLogFiles.value = [];
    selectedDesktopLogGroup.value = '';
    selectedDesktopLogPath.value = '';
    selectedDesktopLogContent.value = '';
    return;
  }

  desktopLogsLoading.value = true;
  try {
    const snapshot = await listDesktopLogFiles();
    desktopLogFiles.value = Array.isArray(snapshot?.files) ? snapshot.files : [];

    const nextGroup = desktopLogGroups.value.find(group => group.key === selectedDesktopLogGroup.value)?.key
      || desktopLogGroups.value[0]?.key
      || '';
    selectedDesktopLogGroup.value = nextGroup;

    const nextPath = currentDesktopLogGroupFiles.value.find(file => String(file?.path || '') === selectedDesktopLogPath.value)?.path
      || currentDesktopLogGroupFiles.value[0]?.path
      || '';
    selectedDesktopLogPath.value = nextPath;

    if (nextPath) {
      await loadDesktopLogContent(nextPath);
    } else {
      selectedDesktopLogContent.value = '';
    }
  } catch (err) {
    desktopLogFiles.value = [];
    selectedDesktopLogGroup.value = '';
    selectedDesktopLogPath.value = '';
    selectedDesktopLogContent.value = '';
    message.error(err?.message || '加载日志列表失败');
  } finally {
    desktopLogsLoading.value = false;
  }
};

const loadLocalCache = () => {
  const cache = localStorage.getItem('api_check_local_cache');
  if (cache) {
    try {
      localCacheList.value = JSON.parse(cache);
    } catch (e) {
      localCacheList.value = [];
    }
  }
};

const snapshotPortableLocalStorage = () => {
  const snapshot = {};
  for (let index = 0; index < localStorage.length; index += 1) {
    const key = localStorage.key(index);
    if (!key) continue;
    snapshot[key] = localStorage.getItem(key);
  }
  return snapshot;
};

const applyPortableLocalStorageSnapshot = (snapshot) => {
  if (!snapshot || typeof snapshot !== 'object' || Array.isArray(snapshot)) {
    throw new Error('invalid_localstorage_snapshot');
  }
  localStorage.clear();
  Object.entries(snapshot).forEach(([key, value]) => {
    localStorage.setItem(key, value == null ? '' : String(value));
  });
};

const getPortableErrorMessage = (error, fallback) => {
  if (!error) return fallback;
  if (typeof error === 'string') return error.trim() || fallback;
  const direct = String(error?.message || error?.error || '').trim();
  if (direct) return direct;
  try {
    const serialized = JSON.stringify(error);
    if (serialized && serialized !== '{}') return serialized;
  } catch {
    // noop
  }
  return String(error).trim() || fallback;
};

const packagePortableData = async () => {
  const packer = window?.go?.main?.App?.PackagePortableData;
  if (typeof packer !== 'function') {
    message.error('当前环境不支持本地绿色化封包');
    return;
  }
  portablePacking.value = true;
  try {
    const snapshotJson = JSON.stringify(snapshotPortableLocalStorage());
    const result = await packer(snapshotJson);
    portableSettingsMeta.value = `封包完成：${result?.backupDir || 'backup'}，localStorage ${Number(result?.localStorageKeyCount || 0)} 项`;
    message.success('已完成本地绿色化封包');
  } catch (error) {
    message.error(`封包失败：${getPortableErrorMessage(error, '未知错误，请查看 logs/portable-data.log')}`);
  } finally {
    portablePacking.value = false;
  }
};

const unpackPortableData = async () => {
  const unpacker = window?.go?.main?.App?.UnpackPortableData;
  if (typeof unpacker !== 'function') {
    message.error('当前环境不支持本地绿色化解包');
    return;
  }
  portableUnpacking.value = true;
  try {
    const result = await unpacker();
    const parsedSnapshot = JSON.parse(String(result?.localStorageJson || '{}'));
    applyPortableLocalStorageSnapshot(parsedSnapshot);
    portableSettingsMeta.value = `解包完成：${result?.backupDir || 'backup'}，已恢复 ${Number(result?.localStorageKeyCount || 0)} 项本地数据`;
    message.success('已从 backup 解包恢复本程序数据，页面即将刷新');
    setTimeout(() => {
      window.location.reload();
    }, 600);
  } catch (error) {
    message.error(`解包失败：${getPortableErrorMessage(error, '未知错误，请查看 logs/portable-data.log')}`);
  } finally {
    portableUnpacking.value = false;
  }
};

const saveToLocal = () => {
  if (!settingsApiUrl.value || !settingsApiKey.value) {
    message.warning('请输入完整的 API URL 和 Key');
    return;
  }
  const newRecord = {
    id: Date.now(),
    name: new URL(settingsApiUrl.value).hostname,
    url: settingsApiUrl.value,
    apiKey: settingsApiKey.value
  };
  localCacheList.value.push(newRecord);
  localStorage.setItem('api_check_local_cache', JSON.stringify(localCacheList.value));
  message.success('保存成功');
};

const deleteLocalRecord = (id) => {
  localCacheList.value = localCacheList.value.filter(r => r.id !== id);
  localStorage.setItem('api_check_local_cache', JSON.stringify(localCacheList.value));
};

const loadLocalRecord = (id) => {
  const record = localCacheList.value.find(r => r.id === id);
  if (record) {
    // 批量模式通常是通过文件导入，这里加载到设置仅做展示或备用
    settingsApiUrl.value = record.url;
    settingsApiKey.value = record.apiKey;
    message.success('已加载到配置表单');
  }
};

const maskApiKey = (key) => {
  if (!key) return '';
  return key.slice(0, 8) + '***' + key.slice(-4);
};

const maskTokenPreview = (token) => {
  const text = String(token || '').trim();
  if (!text) return '';
  if (text.length <= 12) return text;
  return `${text.slice(0, 8)}...${text.slice(-4)}`;
};

const saveLastResultsSnapshot = (results = testResults.value) => {
  try {
    const snapshot = Array.isArray(results) ? results : [];
    localStorage.setItem('api_check_last_results', JSON.stringify(snapshot));
    hasHistory.value = snapshot.length > 0;
  } catch (error) {
    console.warn('[BatchCheck] save history snapshot failed:', error?.message || String(error));
  }
};

const stringifyPreview = (value, maxLength = 280) => {
  if (value == null) return '';
  let text = '';
  if (typeof value === 'string') {
    text = value;
  } else {
    try {
      text = JSON.stringify(value);
    } catch {
      text = String(value);
    }
  }
  text = text.replace(/\s+/g, ' ').trim();
  if (!text) return '';
  return text.length > maxLength ? `${text.slice(0, maxLength)}...` : text;
};

const buildCompatHeadersForUid = (uid) => {
  const normalizedUid = String(uid || '').trim();
  if (!/^\d+$/.test(normalizedUid)) return {};
  return {
    'one-api-user': normalizedUid,
    'New-API-User': normalizedUid,
    'Veloera-User': normalizedUid,
    'voapi-user': normalizedUid,
    'User-id': normalizedUid,
    'Rix-Api-User': normalizedUid,
    'neo-api-user': normalizedUid,
  };
};

const getTokenListEndpointCandidates = (siteType) => {
  if (siteType === 'anyrouter') {
    return [
      '/api/token/?p=0&size=100',
      '/api/token?p=0&size=100',
    ];
  }
  return siteType === 'sub2api'
    ? [
      '/api/v1/keys?page=1&page_size=100',
      '/api/v1/keys?p=0&size=100',
      '/api/token/?p=0&size=100',
      '/api/token?p=0&size=100',
    ]
    : [
      '/api/token/?p=0&size=100',
      '/api/token?p=0&size=100',
      '/api/v1/keys?page=1&page_size=100',
      '/api/v1/keys?p=0&size=100',
    ];
};

const buildProviderReplayHeaders = ({ tokenKey, uid, siteUrl }) => {
  const normalizedSiteUrl = String(siteUrl || '').replace(/\/+$/, '').trim();
  const headers = {
    Authorization: `Bearer ${String(tokenKey || '').trim()}`,
    Accept: 'application/json, text/plain, */*',
    'Accept-Language': 'zh-CN,zh;q=0.9,en;q=0.8',
    'X-Requested-With': 'XMLHttpRequest',
    'Cache-Control': 'no-cache',
    Pragma: 'no-cache',
  };
  if (normalizedSiteUrl) {
    headers.Referer = `${normalizedSiteUrl}/`;
  }
  return {
    ...headers,
    ...buildCompatHeadersForUid(uid),
  };
};

const getProviderTreeParts = (node) => {
  if (!node) {
    return { title: '', suffix: '' };
  }
  const explicitTitle = String(node?.providerTitleText || '').trim();
  const explicitSuffix = String(node?.providerStatusText || '').trim();
  if (explicitTitle || explicitSuffix) {
    return {
      title: explicitTitle,
      suffix: explicitSuffix,
    };
  }

  const rawTitle = String(node?.title || '').trim();
  const match = rawTitle.match(/^(\d+\.\s*\[[^\]]+\])(\s*.*)$/);
  if (match) {
    return {
      title: String(match[1] || '').trim(),
      suffix: String(match[2] || '').trim(),
    };
  }
  return {
    title: '',
    suffix: rawTitle,
  };
};

const getProviderTreeTitle = (node) => getProviderTreeParts(node).title;
const getProviderTreeSuffix = (node) => getProviderTreeParts(node).suffix;
const isProviderDiagnosticTreeNode = (node) => Boolean(node?.isProviderDiagnostic);
const canOpenProviderSiteFromTreeNode = (node) => /^https?:\/\//i.test(String(node?.providerSiteUrl || '').trim());

const openProviderSiteFromTreeNode = (node) => {
  const url = String(node?.providerSiteUrl || '').replace(/\/+$/, '').trim();
  if (!canOpenProviderSiteFromTreeNode(node)) {
    message.warning('当前节点没有可打开的站点地址');
    return;
  }
  openUrlInSystemBrowser(url);
};

const buildProviderFetchReplayText = (node) => {
  const meta = node?.providerDiagnostic || {};
  const request = meta?.replayRequest || null;
  const replayCandidates = Array.isArray(meta?.replayCandidates) ? meta.replayCandidates.filter(Boolean) : [];
  if ((!request?.url || !request?.headers?.Authorization) && replayCandidates.length > 0) {
    const candidateUrls = replayCandidates.map(item => JSON.stringify(item.url)).join(',\n  ');
    const headersText = JSON.stringify(replayCandidates[0]?.headers || {}, null, 2);
    return [
      `// ${meta.siteName || 'provider'} Token 列表抓取复现`,
      `const targets = [`,
      `  ${candidateUrls}`,
      `];`,
      `const headers = ${headersText};`,
      `for (const url of targets) {`,
      `  const res = await fetch(url, { method: 'GET', headers, credentials: 'include' });`,
      `  console.log('url=', url, 'status=', res.status);`,
      `  console.log(await res.text());`,
      `}`,
    ].join('\n');
  }
  if (!request?.url || !request?.headers?.Authorization) {
    return [
      `// ${meta.siteName || node?.providerTitleText || '当前节点'} 暂无可复现的探测请求`,
      `// 原因: ${meta.userFacingError || meta.rawError || 'unknown'}`,
      `// 建议复制“调研 trace 日志”查看本轮完整回溯`,
    ].join('\n');
  }

  return [
    `// ${meta.siteName || 'provider'} 模型发现复现`,
    `const res = await fetch(${JSON.stringify(request.url)}, {`,
    `  method: 'GET',`,
    `  headers: ${JSON.stringify(request.headers, null, 2)}`,
    `});`,
    `console.log('status=', res.status);`,
    `console.log(await res.text());`,
  ].join('\n');
};

const buildProviderTraceLogText = (node) => {
  const meta = node?.providerDiagnostic || {};
  const storageFields = Array.isArray(meta?.storageFields) ? meta.storageFields.filter(Boolean) : [];
  const traceLines = Array.isArray(meta?.traceLines) && meta.traceLines.length
    ? meta.traceLines
    : ['(empty)'];
  return [
    `[Provider] ${meta.siteName || node?.providerTitleText || '-'}`,
    `[SiteURL] ${meta.siteUrl || node?.providerSiteUrl || '-'}`,
    `[Stage] ${meta.stage || '-'}`,
    `[ExtractionMode] ${meta.extractionMode || '-'}`,
    `[UID] ${meta.uid || '-'}`,
    `[Tokens] total=${Number(meta.totalTokens || 0)} usable=${Number(meta.usableTokens || 0)}`,
    `[TokenEndpoint] ${meta.tokenEndpoint || '-'}`,
    `[StorageOrigin] ${meta.storageOrigin || '-'}`,
    `[StorageFields] ${storageFields.length ? storageFields.join(', ') : '-'}`,
    `[ReasonRaw] ${meta.rawError || '-'}`,
    `[ReasonDisplay] ${meta.userFacingError || '-'}`,
    '',
    '[Trace]',
    ...traceLines,
  ].join('\n');
};

const writeTextToClipboard = async (text) => {
  const content = String(text || '');
  if (!content) throw new Error('empty_text');
  if (navigator?.clipboard?.writeText) {
    await navigator.clipboard.writeText(content);
    return;
  }

  const textarea = document.createElement('textarea');
  textarea.value = content;
  textarea.setAttribute('readonly', 'readonly');
  textarea.style.position = 'fixed';
  textarea.style.top = '-9999px';
  document.body.appendChild(textarea);
  textarea.select();
  document.execCommand('copy');
  document.body.removeChild(textarea);
};

const copyProviderFetchReplay = async (node) => {
  try {
    await writeTextToClipboard(buildProviderFetchReplayText(node));
    message.success('已复制 fetch 复现语句');
  } catch (error) {
    message.error(error?.message || '复制 fetch 复现失败');
  }
};

const copyProviderTraceLog = async (node) => {
  try {
    await writeTextToClipboard(buildProviderTraceLogText(node));
    message.success('已复制调研 trace 日志');
  } catch (error) {
    message.error(error?.message || '复制 trace 日志失败');
  }
};

const attachSiteRuntimeMeta = (site, importSource = '') => {
  if (!site || typeof site !== 'object') return site;
  const siteCacheKey = String(site?._siteCacheKey || site?.siteCacheKey || buildSiteCacheKey(site)).trim();
  const nextImportSource = String(site?._lastImportSource || site?.lastImportSource || importSource || '').trim();
  return {
    ...site,
    _siteCacheKey: siteCacheKey,
    _lastImportSource: nextImportSource,
  };
};

const updateActiveSiteSessionSnapshot = (sites, importSource = '') => {
  activeSiteTreeSession.currentSites = (Array.isArray(sites) ? sites : []).map(site => attachSiteRuntimeMeta(site, importSource));
  return activeSiteTreeSession.currentSites;
};

const getActiveSessionSiteRecord = siteCacheKey => {
  const key = String(siteCacheKey || '').trim();
  if (!key) return null;
  return activeSiteTreeSession.currentSites.find(site => String(site?._siteCacheKey || '').trim() === key) || null;
};

const syncSiteCacheSnapshot = (sites, options = {}) => {
  const importSource = String(options?.importSource || '').trim();
  const normalizedSites = updateActiveSiteSessionSnapshot(sites, importSource);
  mergeExtractedSitesIntoTempCache(normalizedSites, options);
  if (!Array.isArray(sites) || sites.length === 0) return [];
  try {
    return mergeExtractedSitesIntoCache(normalizedSites, options);
  } catch (error) {
    console.warn('[SiteCache] sync failed:', error?.message || String(error));
    return [];
  }
};

const buildBatchSiteFromCachedRecord = (siteCacheKey) => {
  const record = findAnySiteCacheRecord(siteCacheKey);
  const restored = buildBatchSitesFromCache(record ? [record] : [], { includeDisabled: true });
  return restored[0] || null;
};

const replaceActiveSiteFromCacheRecord = async (siteCacheKey, reason = 'site-cache-update') => {
  if (typeof activeSiteTreeSession.replaceSites !== 'function') return;
  const restoredSite = buildBatchSiteFromCachedRecord(siteCacheKey);
  if (!restoredSite) return;

  await activeSiteTreeSession.replaceSites(currentSites => currentSites.map(site => {
    const currentKey = String(site?._siteCacheKey || '').trim();
    if (currentKey !== siteCacheKey) return site;
    return {
      ...site,
      ...restoredSite,
      _siteCacheKey: siteCacheKey,
      _localDisabled: restoredSite._localDisabled === true,
      _localNote: String(restoredSite._localNote || '').trim(),
    };
  }), reason, { syncCache: false });
};

const closeTextPromptModal = () => {
  textPromptOpen.value = false;
  textPromptSiteCacheKey.value = '';
  textPromptValue.value = '';
};

const openCustomSkPrompt = record => {
  const siteCacheKey = String(record?.siteCacheKey || '').trim();
  if (!siteCacheKey) return;
  textPromptMode.value = 'sk';
  textPromptSiteCacheKey.value = siteCacheKey;
  textPromptValue.value = '';
  textPromptOpen.value = true;
};

const openSiteNotePrompt = record => {
  const siteCacheKey = String(record?.siteCacheKey || '').trim();
  if (!siteCacheKey) return;
  textPromptMode.value = 'note';
  textPromptSiteCacheKey.value = siteCacheKey;
  textPromptValue.value = String(record?.siteNote || '').trim().slice(0, SITE_NOTE_MAX_LENGTH);
  textPromptOpen.value = true;
};

const submitTextPromptModal = async () => {
  const siteCacheKey = String(textPromptSiteCacheKey.value || '').trim();
  if (!siteCacheKey) {
    message.warning('当前节点缺少站点缓存标识');
    return;
  }

  if (textPromptMode.value === 'sk') {
    const raw = String(textPromptValue.value || '').trim();
    if (!raw) {
      message.warning('请输入一个或多个 sk');
      return;
    }
    appendCustomKeysToSiteCache(siteCacheKey, raw);
    syncSiteCacheSnapshot(loadAllSiteCacheRecords(), {
      importSource: 'site_tree_custom_sk',
      refreshedAt: Date.now(),
    });
    await replaceActiveSiteFromCacheRecord(siteCacheKey, 'site-tree-custom-sk');
    message.success('自定义 SK 已追加');
    closeTextPromptModal();
    return;
  }

  const nextNote = String(textPromptValue.value || '').trim().slice(0, SITE_NOTE_MAX_LENGTH);
  updateSiteCacheNote(siteCacheKey, nextNote);
  syncSiteCacheSnapshot(loadAllSiteCacheRecords(), {
    importSource: 'site_tree_note',
    refreshedAt: Date.now(),
  });
  await replaceActiveSiteFromCacheRecord(siteCacheKey, 'site-tree-note');
  message.success('备注已更新');
  closeTextPromptModal();
};

const getManualTokenKeyFromTreeNode = node => {
  const key = String(node?.key || '').trim();
  if (!key.startsWith('token|')) return '';
  const parts = key.split('|');
  return String(parts[2] || '').trim();
};

const handleTreeSiteRefresh = async node => {
  const siteCacheKey = String(node?.siteCacheKey || '').trim();
  if (!siteCacheKey) {
    message.warning('当前节点缺少站点缓存标识');
    return;
  }

  try {
    const record = getActiveSessionSiteRecord(siteCacheKey) || findAnySiteCacheRecord(siteCacheKey);
    if (!record) {
      message.warning('当前未找到该站点的中间态缓存');
      return;
    }

    let refreshSeed = attachSiteRuntimeMeta({
      ...record,
      site_name: record?.site_name || record?.siteName,
      site_url: record?.site_url || record?.siteUrl,
      site_type: record?.site_type || record?.siteType,
      api_key: record?.api_key || record?.apiBaseUrl,
      account_info: record?.account_info || record?.accountInfo || {},
      resolved_access_token: record?.resolved_access_token || record?.resolvedAccessToken,
      resolved_user_id: record?.resolved_user_id || record?.resolvedUserId,
    }, record?._lastImportSource || record?.lastImportSource || '');
    const importSource = String(refreshSeed?._lastImportSource || refreshSeed?.lastImportSource || '').trim();

    if (isWailsRuntime && /extension_import/i.test(importSource)) {
      const importer = window?.go?.main?.App?.ImportExtensionAccounts;
      if (typeof importer === 'function') {
        try {
          const extensionResult = await importer();
          const extensionAccounts = extensionResult?.payload?.accounts?.accounts;
          const matchedAccount = (Array.isArray(extensionAccounts) ? extensionAccounts : []).find(account =>
            normalizeSiteUrl(account?.site_url) === normalizeSiteUrl(refreshSeed?.siteUrl || refreshSeed?.site_url)
          );
          if (matchedAccount?.account_info?.access_token) {
            refreshSeed = attachSiteRuntimeMeta({
              ...refreshSeed,
              ...matchedAccount,
              account_info: {
                ...(refreshSeed?.account_info || refreshSeed?.accountInfo || {}),
                ...(matchedAccount?.account_info || {}),
              },
            }, 'extension_import_refresh');
          }
        } catch (error) {
          console.warn('[SiteRefresh] extension reimport failed:', error?.message || String(error));
        }
      }
    }

    let refreshed = await fetchTokensForAccountFromBrowserV2(refreshSeed);
    if ((!Array.isArray(refreshed?.tokens) || refreshed.tokens.length === 0) && refreshed?._needServerFallback !== false) {
      const serverResults = await fetchTokensForAccountsViaServer([refreshSeed]).catch(() => []);
      const serverResult = Array.isArray(serverResults) ? serverResults[0] : null;
      if (serverResult) {
        refreshed = {
          ...refreshSeed,
          ...serverResult,
          account_info: {
            ...(refreshSeed?.account_info || {}),
            ...(serverResult?.account_info || {}),
          },
        };
      }
    }

    if ((!Array.isArray(refreshed?.tokens) || refreshed.tokens.length === 0) && isWailsRuntime && isChromeProfileAuthAvailable.value) {
      try {
        const profileResponse = await extractChromeProfileTokens([refreshSeed]);
        const profileResult = Array.isArray(profileResponse?.results) ? profileResponse.results[0] : null;
        if (profileResult) {
          refreshed = attachSiteRuntimeMeta({
            ...refreshSeed,
            ...profileResult,
            account_info: {
              ...(refreshSeed?.account_info || {}),
              ...(profileResult?.account_info || {}),
            },
            resolved_access_token: profileResult?.resolved_access_token || refreshSeed?.account_info?.access_token,
            resolved_user_id: profileResult?.resolved_user_id || refreshSeed?.account_info?.id,
          }, importSource || 'profile_refresh');
        }
      } catch (error) {
        console.warn('[SiteRefresh] profile fallback failed:', error?.message || String(error));
      }
    }

    refreshed = attachSiteRuntimeMeta({
      ...refreshSeed,
      ...refreshed,
      _localDisabled: refreshSeed?._localDisabled,
      _localNote: refreshSeed?._localNote,
    }, importSource);
    syncSiteCacheSnapshot([refreshed], {
      importSource: 'site_tree_refresh',
      refreshedAt: Date.now(),
    });
    await replaceActiveSiteFromCacheRecord(siteCacheKey, 'site-tree-refresh');
    message.success(`已刷新 ${record.siteName || record.site_name || '站点'}`);
  } catch (error) {
    message.error(error?.message || '站点刷新失败');
  }
};

const handleTreeSiteCustomSk = async node => {
  const siteCacheKey = String(node?.siteCacheKey || '').trim();
  if (!siteCacheKey) return;
  openCustomSkPrompt(getRecordBySiteCacheKey(siteCacheKey));
};

const handleTreeManualTokenDelete = async node => {
  const siteCacheKey = String(node?.siteCacheKey || '').trim();
  const tokenKey = getManualTokenKeyFromTreeNode(node);
  if (!siteCacheKey || !tokenKey) return;
  removeCustomKeyFromSiteCache(siteCacheKey, tokenKey);
  await replaceActiveSiteFromCacheRecord(siteCacheKey, 'site-tree-manual-sk-delete');
  message.success('手动添加的 key 已删除');
};

const handleTreeSiteToggleDisabled = async node => {
  const siteCacheKey = String(node?.siteCacheKey || '').trim();
  if (!siteCacheKey) return;
  const currentDisabled = node?.siteDisabled === true;
  setSiteCacheDisabled(siteCacheKey, !currentDisabled);
  await replaceActiveSiteFromCacheRecord(siteCacheKey, 'site-tree-toggle-disabled');
  message.success(currentDisabled ? '站点已激活' : '站点已禁用');
};

const handleTreeSiteEditNote = async node => {
  const siteCacheKey = String(node?.siteCacheKey || '').trim();
  if (!siteCacheKey) return;
  openSiteNotePrompt(getRecordBySiteCacheKey(siteCacheKey));
};

const handleTreeSiteDelete = async node => {
  const siteCacheKey = String(node?.siteCacheKey || '').trim();
  if (!siteCacheKey) return;
  deleteSiteCacheRecord(siteCacheKey);
  if (typeof activeSiteTreeSession.replaceSites === 'function') {
    await activeSiteTreeSession.replaceSites(
      currentSites => currentSites.filter(site => String(site?._siteCacheKey || '').trim() !== siteCacheKey),
      'site-tree-delete',
      { syncCache: false }
    );
  }
  message.success('站点缓存已删除');
};

const isTableExpanded = ref(true);
const isTreeExpanded = ref(loadTreeExpandedSetting(true));
const highlightedTaskId = ref(null);
const tablePagination = ref({
  current: 1,
  pageSize: 15,
  showSizeChanger: true,
  pageSizeOptions: ['15', '30', '50', '100', '300', '500'],
});

const handleTableChange = (pagination) => {
  tablePagination.value = pagination;
};

const onTreeSelect = (selectedKeys, e) => {
  if (e.node.isLeaf) {
    const taskId = e.node.key;
    const idx = currentResultData.value.findIndex(item => item.id === taskId);
    if (idx !== -1) {
      isTableExpanded.value = true;
      highlightedTaskId.value = taskId;
      const targetPage = Math.floor(idx / tablePagination.value.pageSize) + 1;
      tablePagination.value.current = targetPage;
      setTimeout(() => {
        const row = document.querySelector(`[data-row-key="${taskId}"]`);
        if (row) {
          row.scrollIntoView({ behavior: 'smooth', block: 'center' });
        }
      }, 100);
      
      setTimeout(() => {
        if (highlightedTaskId.value === taskId) {
          highlightedTaskId.value = null;
        }
      }, 3000);
    }
  }
};

const validAccounts = ref([]);
const treeData = ref([]);
const checkedKeys = ref([]);
const allKeys = ref([]); // Store all keys for easy 'Select All'
const selectionExpandedKeys = ref([]);
const getSelectionRootKeys = (nodes = treeData.value) => (
  Array.isArray(nodes)
    ? nodes
      .map(node => String(node?.key || '').trim())
      .filter(key => key.startsWith('site-root|'))
    : []
);
const ensureSelectionRootExpanded = (keys = [], nodes = treeData.value) => {
  const merged = new Set(getSelectionRootKeys(nodes));
  if (Array.isArray(keys)) {
    keys.forEach(key => {
      const normalizedKey = String(key || '').trim();
      if (normalizedKey) merged.add(normalizedKey);
    });
  }
  return [...merged];
};

const loadedSitesCount = ref(0);
const fetchKeysProgress = reactive({
  active: false,
  stage: '',
  detail: '',
  total: 0,
  completed: 0,
  successSites: 0,
  lastSiteName: '',
  startedAt: 0,
  lastUpdatedAt: 0,
});
const browserSessionPolling = reactive({
  active: false,
  round: 0,
  totalRounds: 0,
  pending: 0,
});
const browserSessionPendingSiteNames = ref([]);
let fetchKeysProgressTimer = null;

// 按 siteUrl 缓存余额，确保其为响应式对象
const siteQuotaCache = reactive({});
const siteQuotaPendingMap = new Map();

const batchConcurrency = ref(25);
const modelTimeout = ref(15);

const testing = ref(false);
const isRefreshingBalances = ref(false); // NEW: 刷新余额状态
const expandedKeys = ref([]); // NEW: 受控展开状态
const cancelTokens = ref([]); // to allow stopping

// ── NEW: 提取树形数据中所有的 Key 并展开/折叠 ──
const toggleExpandAll = () => {
  if (expandedKeys.value.length > 0) {
    // 当前有展开的，则执行“全部折叠”
    expandedKeys.value = [];
  } else {
    // 当前全部折叠，提取所有节点的 Key 执行“全部展开”
    const allKeys = [];
    const collectKeys = (nodes) => {
      nodes.forEach(node => {
        allKeys.push(node.key);
        if (node.children && node.children.length > 0) {
          collectKeys(node.children);
        }
      });
    };
    collectKeys(organizedTreeData.value);
    expandedKeys.value = allKeys;
  }
};
const handleSelectionTreeExpand = (keys) => {
  selectionExpandedKeys.value = ensureSelectionRootExpanded(keys);
};
const testResults = ref([]); // all tasks
const totalTasks = ref(0);
const completedTasks = ref(0);
const resultModelFilter = ref('');
const organizedGroupIndex = ref([]);
const organizedModelUniverse = ref([]);

const ORGANIZED_REFRESH_INTERVAL_MS = 220;
const organizedSourceResults = ref([]);
let organizedRefreshTimer = null;

const refreshOrganizedSourceNow = () => {
  organizedSourceResults.value = [...testResults.value];
};

const scheduleOrganizedSourceRefresh = (force = false) => {
  if (force) {
    if (organizedRefreshTimer) {
      clearTimeout(organizedRefreshTimer);
      organizedRefreshTimer = null;
    }
    refreshOrganizedSourceNow();
    return;
  }
  if (organizedRefreshTimer) return;
  organizedRefreshTimer = setTimeout(() => {
    organizedRefreshTimer = null;
    refreshOrganizedSourceNow();
  }, ORGANIZED_REFRESH_INTERVAL_MS);
};

// Search & Filter State (Default no filter, no memory)
const searchQuery = ref('');
const filterOnlySuccess = ref(false);

// 快捷筛选：按系列分组，悬浮展开版本/子类
const activeQuickFilters = ref([]);
const quickFilterSelectionMode = ref(false);

const normalizeQuickFilterName = (name) => {
  const normalized = String(name || '').trim();
  if (!normalized) return '';
  const withoutVendor = normalized.includes('/') ? normalized.split('/').pop() : normalized;
  return String(withoutVendor || '').trim();
};

const extractQuickFilterCategory = (name) => {
  const normalized = normalizeQuickFilterName(name);
  if (!normalized) return '';
  const match = normalized.match(/gpt|[a-zA-Z]{3,}/i);
  return match ? match[0].toLowerCase() : '';
};

const extractQuickFilterVersion = (name) => {
  const normalized = normalizeQuickFilterName(name);
  if (!normalized) return '';
  const match = normalized.match(/\d+(?:\.\d+)?/);
  return match ? match[0] : '';
};

const buildQuickFilterOptionLabel = (category, version, sampleName) => {
  if (version) return `${category}-${version}`;
  return normalizeQuickFilterName(sampleName || category);
};

const TASK_STATUS_ORDER = { success: 0, warning: 1, error: 2, testing: 3, pending: 4 };

const quickFilterSourceModels = computed(() => {
  if (step.value === 2) {
    return Array.from(new Set(
      allKeys.value
        .filter(isSelectableModelKey)
        .map(getModelNameFromSelectableKey)
        .map(model => String(model || '').trim())
        .filter(Boolean)
    ));
  }
  return organizedModelUniverse.value;
});

const quickFilters = computed(() => {
  const models = quickFilterSourceModels.value;
  const familyMap = new Map();

  models.forEach(model => {
    const category = extractQuickFilterCategory(model);
    if (!category) return;
    const version = extractQuickFilterVersion(model);
    const familyKey = category;
    const optionKey = `${familyKey}:${version || normalizeQuickFilterName(model).toLowerCase()}`;
    if (!familyMap.has(familyKey)) {
      familyMap.set(familyKey, {
        key: familyKey,
        label: familyKey.toUpperCase(),
        category: familyKey,
        optionsMap: new Map(),
      });
    }

    const family = familyMap.get(familyKey);
    if (!family.optionsMap.has(optionKey)) {
      family.optionsMap.set(optionKey, {
        key: optionKey,
        label: buildQuickFilterOptionLabel(familyKey, version, model),
        version,
        models: [],
      });
    }

    family.optionsMap.get(optionKey).models.push(model);
  });

  const regularFamilies = [];
  const rareOptions = [];
  familyMap.forEach(family => {
    const options = Array.from(family.optionsMap.values()).sort((a, b) => {
      const versionDiff = (parseFloat(b.version) || 0) - (parseFloat(a.version) || 0);
      if (versionDiff !== 0) return versionDiff;
      return a.label.localeCompare(b.label);
    });

    const nextFamily = {
      key: family.key,
      label: family.label,
      category: family.category,
      options,
    };

    if (options.length <= 1) {
      rareOptions.push(...options);
      return;
    }

    regularFamilies.push(nextFamily);
  });

  if (rareOptions.length > 0) {
    rareOptions.sort((a, b) => a.label.localeCompare(b.label));
    regularFamilies.push({
      key: 'rare',
      label: '冷门组模型',
      category: 'rare',
      options: rareOptions,
    });
  }

  const priority = ['gpt', 'claude', 'gemini', 'deepseek', 'llama', 'minimax', 'grok', 'kimi', 'glm'];
  regularFamilies.sort((a, b) => {
    const idxA = priority.indexOf(a.category);
    const idxB = priority.indexOf(b.category);
    if (idxA !== -1 && idxB !== -1) return idxA - idxB;
    if (idxA !== -1) return -1;
    if (idxB !== -1) return 1;
    if (a.options.length !== b.options.length) return b.options.length - a.options.length;
    return a.label.localeCompare(b.label);
  });

  return regularFamilies;
});

watch(quickFilters, (families) => {
  const validOptionKeys = new Set();
  (Array.isArray(families) ? families : []).forEach(family => {
    (Array.isArray(family?.options) ? family.options : []).forEach(option => {
      validOptionKeys.add(option.key);
    });
  });

  if (activeQuickFilters.value.length === 0) return;
  activeQuickFilters.value = activeQuickFilters.value.filter(key => validOptionKeys.has(key));
});

const applyActiveQuickFilters = (nextOptionKeys) => {
  const normalized = Array.from(new Set(
    (Array.isArray(nextOptionKeys) ? nextOptionKeys : []).filter(Boolean)
  ));

  if (step.value === 2 && normalized.length > 0 && !quickFilterSelectionMode.value) {
    // 第一次点击快捷筛选，先清空默认全选/手工勾选，再交给快捷筛选接管。
    checkedKeys.value = [];
    quickFilterSelectionMode.value = true;
  }

  activeQuickFilters.value = normalized;

  if (step.value === 2 && normalized.length === 0 && quickFilterSelectionMode.value) {
    checkedKeys.value = [];
    quickFilterSelectionMode.value = false;
  }
};

const toggleQuickFilter = (optionKey) => {
  const current = new Set(activeQuickFilters.value);
  if (current.has(optionKey)) current.delete(optionKey);
  else current.add(optionKey);
  applyActiveQuickFilters(Array.from(current));
};

const clearQuickFilters = () => {
  applyActiveQuickFilters([]);
};

const isQuickFilterFamilyFullySelected = (family) => {
  return family.options.length > 0
    && family.options.every(option => activeQuickFilters.value.includes(option.key));
};

const selectQuickFilterFamily = (family) => {
  const current = new Set(activeQuickFilters.value);
  if (isQuickFilterFamilyFullySelected(family)) {
    family.options.forEach(option => current.delete(option.key));
  } else {
    family.options.forEach(option => current.add(option.key));
  }
  applyActiveQuickFilters(Array.from(current));
};

const isQuickFilterFamilyActive = (family) => {
  return family.options.some(option => activeQuickFilters.value.includes(option.key));
};

const getQuickFilterFamilyActiveCount = (family) => {
  return family.options.filter(option => activeQuickFilters.value.includes(option.key)).length;
};

const activeQuickFilterModelSet = computed(() => {
  const selectedModels = new Set();
  quickFilters.value.forEach(family => {
    family.options.forEach(option => {
      if (!activeQuickFilters.value.includes(option.key)) return;
      option.models.forEach(model => selectedModels.add(model));
    });
  });
  return selectedModels;
});

watch([activeQuickFilterModelSet, allKeys], ([currentModelSet], [previousModelSet]) => {
  if (step.value !== 2) return;
  if (!(currentModelSet instanceof Set) || currentModelSet.size === 0) {
    if (quickFilterSelectionMode.value) {
      checkedKeys.value = [];
    }
    return;
  }

  const selectableKeys = allKeys.value.filter(isSelectableModelKey);
  const matchedKeys = selectableKeys.filter(key => currentModelSet.has(getModelNameFromSelectableKey(key)));

  // 第一次点击快捷筛选时，直接切换为“快捷筛选驱动勾选”。
  const previousSize = previousModelSet instanceof Set ? previousModelSet.size : 0;
  if (previousSize === 0) {
    checkedKeys.value = [...matchedKeys];
    return;
  }

  checkedKeys.value = [...matchedKeys];
});

const activeQuickFilterSummary = computed(() => {
  const labels = [];
  quickFilters.value.forEach(family => {
    family.options.forEach(option => {
      if (activeQuickFilters.value.includes(option.key)) labels.push(option.label);
    });
  });
  if (labels.length === 0) return '';
  if (labels.length <= 3) return `已选: ${labels.join(' / ')}`;
  return `已选: ${labels.slice(0, 3).join(' / ')} +${labels.length - 3}`;
});

const loadingStagePercent = computed(() => {
  if (step.value !== -1 || !isLoadingModels.value) return 0;
  const total = fetchKeysProgress.total || totalAccountsCount.value;
  const completed = fetchKeysProgress.completed || 0;
  if (total <= 0) return 0;
  return Math.max(0, Math.min(100, Math.floor((completed / total) * 100)));
});

const resolveLoadingMode = () => (
  activeExtractionMode.value ||
  (isWailsRuntime ? normalizeDesktopTokenSourceMode(desktopTokenSourceMode.value) : 'browser_direct')
);

const loadingStageTitle = computed(() => {
  const loadingMode = resolveLoadingMode();
  if (isWailsRuntime && loadingMode === 'profile_file' && fetchKeysProgress.total > 0) {
    if (fetchKeysProgress.stage === 'profile_copy') return '正在复制 Chrome Local Storage';
    if (fetchKeysProgress.stage === 'profile_scan') return '正在扫描 Chrome Local Storage';
    if (fetchKeysProgress.stage === 'extract_site') return '正在逐站点提取 Token';
    if (fetchKeysProgress.stage === 'done') return '正在整理 Profile 提取结果';
    return '正在读取 Chrome Profile 文件';
  }
  if (isWailsRuntime && fetchKeysProgress.total > 0) return '正在提取站点 Token';
  return '正在准备可检测站点';
});

const loadingStageDescription = computed(() => {
  const total = fetchKeysProgress.total || totalAccountsCount.value;
  const completed = fetchKeysProgress.completed || 0;
  const loadingMode = resolveLoadingMode();
  if (isWailsRuntime && loadingMode === 'profile_file' && total > 0) {
    if (fetchKeysProgress.stage === 'profile_copy' || fetchKeysProgress.stage === 'profile_scan') {
      return `${fetchKeysProgress.detail || '正在读取本地 Chrome Profile 数据'}，完成后将开始逐站点提取`;
    }
    return `已处理 ${completed} / ${total} 个站点，已从本地 Profile 提取 ${fetchKeysProgress.successSites} 个站点的 Token`;
  }
  if (total > 0) {
    return `已完成 ${completed} / ${total} 个站点，成功拉取 ${fetchKeysProgress.successSites} 个站点的 Token`;
  }
  return '正在建立批量检测所需的站点数据，请稍候。';
});

const loadingStageMeta = computed(() => {
  const meta = [];
  const refreshedAt = fetchKeysProgress.lastUpdatedAt || Date.now();
  const loadingMode = resolveLoadingMode();
  if (fetchKeysProgress.detail && !(isWailsRuntime && loadingMode === 'profile_file' && (fetchKeysProgress.stage === 'profile_copy' || fetchKeysProgress.stage === 'profile_scan'))) {
    meta.push(fetchKeysProgress.detail);
  }
  if (fetchKeysProgress.lastSiteName) {
    meta.push(`当前站点：${fetchKeysProgress.lastSiteName}`);
  }
  if (fetchKeysProgress.startedAt) {
    meta.push(`耗时 ${Math.max(1, Math.floor((refreshedAt - fetchKeysProgress.startedAt) / 1000))} 秒`);
  }
  return meta.join(' · ');
});

const loadingStageStatusText = computed(() => {
  if (step.value !== -1 || !isLoadingModels.value) return '';

  const loadingMode = resolveLoadingMode();
  if (isWailsRuntime && loadingMode === 'profile_file' && fetchKeysProgress.total > 0) {
    const total = fetchKeysProgress.total || totalAccountsCount.value;
    const completed = fetchKeysProgress.completed || 0;
    if (fetchKeysProgress.stage === 'profile_copy') {
      return 'Profile 文件准备中：复制 Local Storage';
    }
    if (fetchKeysProgress.stage === 'profile_scan') {
      return 'Profile 文件提取中：扫描 Local Storage';
    }
    if (completed < total) {
      return `Profile 文件提取中：${completed}/${total}`;
    }
    return 'Profile 文件提取完成，正在整理站点结果';
  }

  if (isWailsRuntime && fetchKeysProgress.total > 0) {
    const total = fetchKeysProgress.total || totalAccountsCount.value;
    const completed = fetchKeysProgress.completed || 0;
    const currentSite = String(fetchKeysProgress.lastSiteName || '').trim();
    if (completed < total) {
      return currentSite
        ? `Token 提取中：${completed}/${total}，当前站点 ${currentSite}`
        : `Token 提取中：${completed}/${total}`;
    }
    return 'Token 提取完成，正在整理站点结果';
  }

  if (loadedSitesCount.value > 0 && loadedSitesCount.value < totalAccountsCount.value) {
    return `模型发现中：${loadedSitesCount.value}/${totalAccountsCount.value}`;
  }

  if (loadedSitesCount.value >= totalAccountsCount.value && totalAccountsCount.value > 0) {
    return '模型发现完成，正在生成可选树';
  }

  return '正在初始化批量检测任务';
});

const loadingStageStatusColor = computed(() => {
  if (!loadingStageStatusText.value) return 'default';
  if (loadedSitesCount.value >= totalAccountsCount.value && totalAccountsCount.value > 0) return 'success';
  return 'processing';
});

const importExtensionStatusText = computed(() => {
  const text = String(importExtensionStatus.value || '').trim();
  if (!text) return '';
  if (!isImportingExtension.value) return text;
  return `${text}（${importExtensionElapsedSeconds.value}s）`;
});

const testProgress = computed(() => {
  if (totalTasks.value === 0) return 0;
  return Math.floor((completedTasks.value / totalTasks.value) * 100);
});
const browserSessionPendingSiteNameSet = computed(() => new Set(browserSessionPendingSiteNames.value));
const organizedSearchKeywords = computed(() => String(searchQuery.value || '').trim().toLowerCase().split(/\s+/).filter(Boolean));
const organizedModelKeywords = computed(() => String(resultModelFilter.value || '').trim().toLowerCase().split(/\s+/).filter(Boolean));

const rebuildOrganizedGroupIndex = (results) => {
  const sourceResults = Array.isArray(results) ? results : [];
  if (!sourceResults.length) {
    organizedGroupIndex.value = [];
    organizedModelUniverse.value = [];
    return;
  }

  const groups = new Map();
  const modelUniverse = new Set();

  sourceResults.forEach(task => {
    const siteName = String(task?.siteName || '').trim();
    const apiKey = String(task?.apiKey || '').trim();
    const siteUrl = String(task?.siteUrl || '').trim();
    const modelName = String(task?.modelName || '').trim();
    const siteNameLower = siteName.toLowerCase();
    const modelNameLower = modelName.toLowerCase();
    if (modelName) {
      modelUniverse.add(modelName);
    }

    const groupKey = `${siteName}|${apiKey}`;
    if (!groups.has(groupKey)) {
      groups.set(groupKey, {
        key: groupKey,
        siteName,
        siteNameLower,
        apiKey,
        siteUrl,
        tasks: [],
      });
    }

    groups.get(groupKey).tasks.push({
      task,
      modelNameLower,
    });
  });

  const nextGroups = Array.from(groups.values()).map(group => {
    const sortedTasks = [...group.tasks].sort((left, right) => {
      const leftOrder = TASK_STATUS_ORDER[left.task?.status] ?? 99;
      const rightOrder = TASK_STATUS_ORDER[right.task?.status] ?? 99;
      if (leftOrder !== rightOrder) return leftOrder - rightOrder;
      return String(left.task?.modelName || '').localeCompare(String(right.task?.modelName || ''));
    });

    const hasSuccess = sortedTasks.some(item => item.task?.status === 'success');
    const hasWarning = sortedTasks.some(item => item.task?.status === 'warning');

    return {
      ...group,
      tasks: sortedTasks,
      hasSuccess,
      hasWarning,
    };
  }).sort((left, right) => {
    if (left.hasSuccess && !right.hasSuccess) return -1;
    if (!left.hasSuccess && right.hasSuccess) return 1;
    if (left.hasWarning && !right.hasWarning) return -1;
    if (!left.hasWarning && right.hasWarning) return 1;
    return String(left.siteName || '').localeCompare(String(right.siteName || ''));
  });

  organizedGroupIndex.value = nextGroups;
  organizedModelUniverse.value = Array.from(modelUniverse).sort((left, right) => left.localeCompare(right));
};

watch(organizedSourceResults, (results) => {
  rebuildOrganizedGroupIndex(results);
}, { immediate: true });

const matchesResultTaskByFilters = (task, options = {}) => {
  const { includeSearch = false, includeSuccessFilter = false } = options;
  const modelName = String(task?.modelName || '').trim();
  const modelNameLower = modelName.toLowerCase();
  const siteNameLower = String(task?.siteName || '').toLowerCase();

  if (includeSuccessFilter && filterOnlySuccess.value && task?.status === 'error') {
    return false;
  }

  if (activeQuickFilterModelSet.value.size > 0 && !activeQuickFilterModelSet.value.has(modelName)) {
    return false;
  }

  if (organizedModelKeywords.value.length > 0 && !organizedModelKeywords.value.some(keyword => modelNameLower.includes(keyword))) {
    return false;
  }

  if (includeSearch && organizedSearchKeywords.value.length > 0) {
    const matched = organizedSearchKeywords.value.some(keyword =>
      siteNameLower.includes(keyword) || modelNameLower.includes(keyword)
    );
    if (!matched) return false;
  }

  return true;
};

// --- NEW Core Computed: Organized & Filtered Tree Data ---
const organizedTreeData = computed(() => {
  if (!organizedGroupIndex.value.length) return [];

  return organizedGroupIndex.value.reduce((bucket, group) => {
    const filteredTasks = group.tasks
      .filter(item => matchesResultTaskByFilters(item.task, { includeSearch: true, includeSuccessFilter: true }))
      .map(item => item.task);

    if (!filteredTasks.length) {
      return bucket;
    }

    const hasSuccess = filteredTasks.some(task => task.status === 'success');
    const hasWarning = filteredTasks.some(task => task.status === 'warning');
    const siteKey = group.siteUrl?.replace(/\/+$/, '') || '';
    const quota = siteQuotaCache[siteKey];
    const quotaStr = (quota && !['获取中...', '无授权', '请求超时', '网络错误'].includes(quota)) 
      ? ` (剩余: ${quota.replace('$', '')} $)` 
      : '';

    let titleClass = 'tree-node-grey';
    if (hasSuccess) titleClass = 'tree-node-green';
    else if (hasWarning) titleClass = 'tree-node-orange';
    const isBrowserPending = browserSessionPolling.active && browserSessionPendingSiteNameSet.value.has(group.siteName);
    const pendingHint = isBrowserPending
      ? `后台检测中（第 ${Math.max(browserSessionPolling.round, 1)}/${Math.max(browserSessionPolling.totalRounds, 1)} 轮）`
      : '';
    const groupSiteName = String(group?.siteName || '未命名站点').trim() || '未命名站点';
    const groupApiKey = String(group?.apiKey || '').trim();
    const groupApiKeyPreview = groupApiKey
      ? `${groupApiKey.slice(0, 15)}...`
      : '(缺少Key)';

    bucket.push({
      title: `[${groupSiteName}] ${groupApiKeyPreview}${quotaStr}`,
      key: group.key,
      class: titleClass,
      isBrowserPending,
      pendingHint,
      children: filteredTasks.map(t => ({
        title: `${t.modelName}${t.modelSuffix || ''} - ${t.statusText} (${t.responseTime}s)`,
        displayTitle: t.displaySuffixHtml ? `${t.modelName}${t.displaySuffixHtml} - ${t.statusText} (${t.responseTime}s)` : null,
        key: t.id,
        isLeaf: true,
        class: `status-${t.status}`,
        siteName: t.siteName,
        siteUrl: t.siteUrl,
        apiKey: t.apiKey,
        model: t.modelName
      })),
      hasSuccess,
      hasWarning,
    });
    return bucket;
  }, []);
});

const currentResultData = computed(() => {
  return organizedSourceResults.value.filter(item => matchesResultTaskByFilters(item));
});

const resultColumns = [
  { title: '平台名称', dataIndex: 'siteName', width: 120 },
  { title: '请求Payload', dataIndex: 'payload', width: 150 },
  { title: '模型名称', dataIndex: 'modelName', width: 150 },
  { title: '状态', dataIndex: 'status', width: 100 },
  { title: '响应(s)', dataIndex: 'responseTime', width: 112 },
  { title: '备注信息', dataIndex: 'remark', ellipsis: true },
];

const hasHistory = ref(false);

const isEditorOpen = ref(false);
const editingRecord = ref(null);
const editingPayload = ref('');
let importExtensionResetTimer = null;
let importExtensionTickTimer = null;
let backendHealthTimer = null;

const getMaskedKey = (key) => {
  if (!key) return '';
  if (key.length <= 10) return key;
  return key.slice(0, 5) + '...' + key.slice(-4);
};

const getPayloadJson = (record) => {
  return JSON.stringify({
    url: record.siteUrl ? record.siteUrl.replace(/\/+$/, '') : '',
    key: record.apiKey,
    model: record.modelName,
    messages: buildQuickTestMessages()
  }, null, 2);
};

const truncateText = (input, max = 1200) => {
  const text = String(input || '');
  if (text.length <= max) return text;
  return `${text.slice(0, max)}\n...(truncated ${text.length - max} chars)`;
};

const tryParseJson = (input) => {
  try {
    return JSON.parse(String(input || ''));
  } catch {
    return null;
  }
};

const normalizeNestedErrorText = (raw) => {
  let cursor = raw;
  for (let i = 0; i < 2; i += 1) {
    if (!cursor || typeof cursor !== 'string') break;
    const trimmed = cursor.trim();
    if (!trimmed.startsWith('{') && !trimmed.startsWith('[')) break;
    const parsed = tryParseJson(trimmed);
    if (!parsed || typeof parsed !== 'object') break;
    const next = parsed?.error?.message || parsed?.message || parsed?.error;
    if (typeof next === 'string' && next.trim()) {
      cursor = next;
      continue;
    }
    return trimmed;
  }
  return String(cursor || '');
};

const toReadableError = (rawData, fallback = '请求失败') => {
  if (!rawData) return fallback;
  const candidate = rawData?.error?.message || rawData?.message || rawData?.error || fallback;
  const normalized = normalizeNestedErrorText(candidate);
  const parsed = tryParseJson(normalized);
  if (parsed && typeof parsed === 'object') {
    return parsed?.error?.message || parsed?.message || fallback;
  }
  return normalized || fallback;
};

const toStatusTextByError = (messageText) => {
  const msg = String(messageText || '').toLowerCase();
  if (!msg) return '调用失败';
  if (msg.includes('html') || msg.includes('cloudflare')) return '静态页/风控';
  if (msg.includes('overloaded') || msg.includes('繁忙')) return '系统繁忙';
  if (msg.includes('余额不足') || msg.includes('insufficient')) return '余额不足';
  if (msg.includes('unauthorized') || msg.includes('401') || msg.includes('forbidden') || msg.includes('403')) return '鉴权失败';
  if (msg.includes('timeout') || msg.includes('超时')) return '请求超时';
  return '调用失败';
};

const getStatusTooltip = (record) => {
  const raw = String(record?.fullResponse || '').trim();
  if (!raw) return '无原始响应数据';
  const parsed = tryParseJson(raw);
  if (parsed && typeof parsed === 'object') {
    return truncateText(JSON.stringify(parsed, null, 2), 20000);
  }
  return truncateText(raw, 20000);
};

const getPerformanceTooltipLines = (record) => buildPerformanceTooltipLines(record);

const formatBalance = (amount) => {
  if (amount == null) return '0.000';
  return (amount / 500000).toFixed(3);
};

const hoverQuota = (record) => {
  // 已有有效的缓存直接跳过
  if (record.quota !== undefined) return;

  const siteKey = record.siteUrl?.replace(/\/+$/, '') || '';

  // 命中缓存：同一 siteUrl 已算过
  if (siteQuotaCache[siteKey] !== undefined) {
    record.quota = siteQuotaCache[siteKey];
    return;
  }

  record.quota = '获取中...';

  void loadQuotaForRecord(record);
};

const loadQuotaForRecord = async (record, { force = false } = {}) => {
  const siteKey = record?.siteUrl?.replace(/\/+$/, '') || '';
  if (!siteKey) return '';

  if (!force && siteQuotaCache[siteKey] !== undefined) {
    record.quota = siteQuotaCache[siteKey];
    return siteQuotaCache[siteKey];
  }

  if (!force && siteQuotaPendingMap.has(siteKey)) {
    return siteQuotaPendingMap.get(siteKey);
  }

  const pending = (async () => {
    const label = await fetchQuotaLabelWithBatchLogic({
      apiFetch,
      site: record.accountData,
      siteUrl: siteKey,
    });
    siteQuotaCache[siteKey] = label;
    testResults.value.forEach(r => {
      if (r.siteUrl?.replace(/\/+$/, '') === siteKey) {
        r.quota = label;
      }
    });
    return label;
  })();

  siteQuotaPendingMap.set(siteKey, pending);
  try {
    return await pending;
  } finally {
    siteQuotaPendingMap.delete(siteKey);
  }
};

// ── NEW: 导入文件后直接预取所有额度 ──
const preloadAllQuotas = async (extractedSites) => {
  if (!Array.isArray(extractedSites) || extractedSites.length === 0) return;

  const uniqueSiteRecords = Array.from(new Map(
    extractedSites
      .filter(site => site?.site_url && !site?.error)
      .map(site => [String(site.site_url).replace(/\/+$/, ''), {
        siteUrl: site.site_url,
        accountData: site,
      }])
  ).values());

  const concurrency = 4;
  let cursor = 0;
  const worker = async () => {
    while (cursor < uniqueSiteRecords.length) {
      const currentIndex = cursor;
      cursor += 1;
      const record = uniqueSiteRecords[currentIndex];
      if (!record) continue;
      try {
        await loadQuotaForRecord(record);
      } catch {}
    }
  };

  await Promise.allSettled(
    Array.from({ length: Math.min(concurrency, uniqueSiteRecords.length) }, () => worker())
  );
};

watch(step, (value) => {
  if (value !== 2) {
    selectionExpandedKeys.value = [];
    return;
  }
  selectionExpandedKeys.value = ensureSelectionRootExpanded(selectionExpandedKeys.value);
});

watch(treeData, (nodes) => {
  if (step.value !== 2) return;
  selectionExpandedKeys.value = ensureSelectionRootExpanded(selectionExpandedKeys.value, nodes);
});

watch(searchQuery, (value) => {
  const hasKeyword = Boolean(String(value || '').trim());
  if (hasKeyword && treeData.value.length > 0 && selectionExpandedKeys.value.length === 0) {
    selectionExpandedKeys.value = ensureSelectionRootExpanded(
      treeData.value
        .map(node => node?.key)
        .filter(Boolean)
        .slice(0, 24),
    );
  }
});

// ── NEW: 批量异步强制刷新所有已选站点的余额 ──
const refreshAllBalances = async () => {
  if (isRefreshingBalances.value) return;
  
  const results = testResults.value;
  if (results.length === 0) {
    message.warning('当前暂无检测结果，无法刷新余额');
    return;
  }

  isRefreshingBalances.value = true;
  
  // 1. 清空所有 siteQuotaCache 缓存
  Object.keys(siteQuotaCache).forEach(key => delete siteQuotaCache[key]);
  
  // 2. 找到所有唯一的站点 URL
  const uniqueSites = new Map();
  results.forEach(r => {
    const siteKey = r.siteUrl?.replace(/\/+$/, '') || '';
    if (siteKey && !uniqueSites.has(siteKey)) {
      uniqueSites.set(siteKey, r);
    }
  });

  // 3. 异步并发刷新
  const promises = Array.from(uniqueSites.values()).map(record => {
    // 强制重置当前记录的 quota 状态，触发 hoverQuota 的重新获取
    delete record.quota; 
    return loadQuotaForRecord(record, { force: true });
  });

  await Promise.allSettled(promises);
  isRefreshingBalances.value = false;
  message.success('余额刷新请求已全部发出');
};

// ── NEW: 一键拉起 Cherry Studio ──
const launchCherryStudio = (node) => {
  if (!node.apiKey || !node.siteUrl) {
    message.warning('配置信息不完整，无法导出');
    return;
  }
  
  const payload = {
    id: `batch-${node.key}`,
    baseUrl: node.siteUrl.replace(/\/+$/, ''),
    apiKey: node.apiKey,
    name: `${node.siteName} (${node.model})`
  };
  
  try {
    const jsonString = JSON.stringify(payload);
    // 使用 TextEncoder 处理 UTF-8 字符，确保中文字符名不乱码
    const bytes = new TextEncoder().encode(jsonString);
    const base64String = btoa(String.fromCharCode(...bytes));
    const url = `cherrystudio://providers/api-keys?v=1&data=${base64String}`;
    window.open(url, '_blank');
    message.success('正在尝试唤起 Cherry Studio...');
  } catch (err) {
    message.error('生成配置失败: ' + err.message);
  }
};

// ── NEW: 一键拉起 CC-Switch ──
const launchCCSwitch = (node) => {
  if (!node.apiKey || !node.siteUrl) {
    message.warning('配置信息不完整，无法导出');
    return;
  }

  const params = new URLSearchParams();
  params.set('resource', 'provider');
  params.set('app', 'claude'); // 默认映射为 claude 类型
  params.set('name', `${node.siteName} - ${node.model}`);
  params.set('homepage', node.siteUrl);
  params.set('endpoint', node.siteUrl);
  params.set('apiKey', node.apiKey);
  params.set('model', node.model);

  const url = `ccswitch://v1/import?${params.toString()}`;
  window.open(url, '_blank');
  message.success('正在尝试唤起 CC-Switch...');
};

const openPayloadEditor = (record) => {
  editingRecord.value = record;
  editingPayload.value = getPayloadJson(record);
  isEditorOpen.value = true;
};

const resendPayload = async () => {
  let custom;
  try {
    custom = JSON.parse(editingPayload.value);
  } catch(e) {
    message.error('JSON格式不正确，请检查！');
    return;
  }
  isEditorOpen.value = false;
  
    // Update task temporarily
  editingRecord.value.status = 'testing';
  editingRecord.value.statusText = '重测中';
  // If user changed the model or key in payload, do NOT change the table's display fields immediately unless we want to, but running with custom payload is fine.
  
  await runSingleTest(editingRecord.value, custom);
  
  // Also update history immediately
  saveLastResultsSnapshot();
};

onMounted(() => {
  logClientDiagnostic('batch.lifecycle', 'BatchCheck mounted');
  logClientDiagnostic(
    'batch.lifecycle',
    `PerformHttpRequest typeof=${typeof window?.go?.main?.App?.PerformHttpRequest} PerformHttpRequestRaw typeof=${typeof window?.go?.main?.App?.PerformHttpRequestRaw} AppendClientLog typeof=${typeof window?.go?.main?.App?.AppendClientLog}`
  );
  resetImportExtensionState();
  isDarkMode.value = document.body.classList.contains('dark-mode');
  loadDesktopTokenSourceMode();
  void probeBackendHealth();
  setTimeout(() => {
    if (!backendHealth.checked) {
      void probeBackendHealth();
    }
  }, 1200);
  if (showBackendHealth.value) {
    backendHealthTimer = setInterval(() => {
      void probeBackendHealth();
    }, 10000);
  }
  const hist = localStorage.getItem('api_check_last_results');
  if (hist) {
    try {
      const parsed = JSON.parse(hist);
      if (Array.isArray(parsed) && parsed.length > 0) {
        hasHistory.value = true;
      }
  } catch(e) {}
  }
  const pendingBatchStart = consumePendingBatchStart();
  const pendingRestoreKeys = consumePendingSiteRestore();
  if (pendingRestoreKeys.length > 0) {
    const cachedSites = buildBatchSitesFromCache(loadAllSiteCacheRecords(), {
      siteCacheKeys: pendingRestoreKeys,
      includeDisabled: true,
    });
    if (cachedSites.length > 0) {
      void maximiseMainWindow();
      void processAccountsV2([], {
        importSource: 'site_cache_restore',
        prefetchedSites: cachedSites,
      }).then(async () => {
        if (!pendingBatchStart?.autoStart) return;
        batchConcurrency.value = Number(pendingBatchStart?.batchConcurrency || batchConcurrency.value || 25);
        modelTimeout.value = Number(pendingBatchStart?.modelTimeout || modelTimeout.value || 15);
        if (Array.isArray(pendingBatchStart?.checkedKeys) && pendingBatchStart.checkedKeys.length > 0) {
          checkedKeys.value = pendingBatchStart.checkedKeys.map(item => String(item || '').trim()).filter(Boolean);
        }
        await nextTick();
        await startBatchCheck();
      });
    }
  }
});

watch(desktopTokenSourceMode, (value) => {
  try {
    localStorage.setItem(
      DESKTOP_TOKEN_SOURCE_MODE_STORAGE_KEY,
      normalizeDesktopTokenSourceMode(value)
    );
  } catch {}
});

watch(selectedDesktopLogGroup, (groupKey) => {
  const files = desktopLogGroups.value.find(group => group.key === groupKey)?.files || [];
  const nextPath = files.find(file => String(file?.path || '') === selectedDesktopLogPath.value)?.path
    || files[0]?.path
    || '';
  selectedDesktopLogPath.value = nextPath;
  if (nextPath) {
    void loadDesktopLogContent(nextPath);
  } else {
    selectedDesktopLogContent.value = '';
  }
});

onBeforeUnmount(() => {
  resetImportExtensionState();
  stopFetchKeysProgressPolling();
  stopBridgeImportPolling();
  activeSiteTreeSession.replaceSites = null;
  activeSiteTreeSession.requestDiscoveryRefresh = null;
  activeSiteTreeSession.syncCacheSnapshot = null;
  if (backendHealthTimer) {
    clearInterval(backendHealthTimer);
    backendHealthTimer = null;
  }
});

const loadHistory = async () => {
  const hist = localStorage.getItem('api_check_last_results');
  if (hist) {
    try {
      await maximiseMainWindow();
      const parsed = JSON.parse(hist);
      testResults.value = (Array.isArray(parsed) ? parsed : []).map((task, index) => ({
        ...task,
        id: String(task?.id || `history_task_${index}`),
        siteId: String(task?.siteId || '').trim(),
        siteName: String(task?.siteName || '未命名站点').trim() || '未命名站点',
        siteUrl: String(task?.siteUrl || '').trim(),
        apiKey: String(task?.apiKey || '').trim(),
        modelName: String(task?.modelName || '').trim(),
        status: String(task?.status || 'pending').trim() || 'pending',
        statusText: String(task?.statusText || '').trim() || '等待重测',
        responseTime: String(task?.responseTime || '-').trim() || '-',
        remark: String(task?.remark || '-').trim() || '-',
      })).filter(task => task.siteUrl && task.apiKey && task.modelName);
      organizedSourceResults.value = [...testResults.value];
      totalTasks.value = testResults.value.length;
      completedTasks.value = testResults.value.filter(task => !['pending', 'testing'].includes(String(task?.status || ''))).length;
      step.value = 3;
      message.success('历史检测结果已恢复');
    } catch (e) {
      message.error('解析历史数据失败');
    }
  }
};

const resetStep1 = () => {
  stopFetchKeysProgressPolling();
  resetFetchKeysProgress();
  step.value = 1;
  treeData.value = [];
  checkedKeys.value = [];
  validAccounts.value = [];
  testResults.value = [];
  organizedSourceResults.value = [];
  selectionExpandedKeys.value = [];
};

const resetStep2 = () => {
  stopFetchKeysProgressPolling();
  step.value = 2;
  testResults.value = [];
  organizedSourceResults.value = [];
  completedTasks.value = 0;
  totalTasks.value = 0;
};

const FALLBACK_BROWSER_STORAGE_KEY = 'batch_api_check_fallback_browser';
const DESKTOP_TOKEN_SOURCE_MODE_STORAGE_KEY = 'batch_api_check_desktop_token_source_mode';

const normalizeDesktopTokenSourceMode = (value) => {
  const normalized = String(value || '').trim();
  if (normalized === 'cdp_restart') return 'cdp_restart';
  if (normalized === 'server_proxy') return 'cdp_restart';
  return 'profile_file';
};

const loadDesktopTokenSourceMode = () => {
  try {
    desktopTokenSourceMode.value = normalizeDesktopTokenSourceMode(
      localStorage.getItem(DESKTOP_TOKEN_SOURCE_MODE_STORAGE_KEY)
    );
  } catch {
    desktopTokenSourceMode.value = 'profile_file';
  }
};

const isChromeProfileAuthAvailable = computed(() => isChromeProfileAuthBridgeAvailable());

const isUsableToken = (token) => {
  const key = String(token?.key || token?.access_token || '').trim();
  if (!key) return false;
  if (token?.unresolved === true) return false;
  return !key.includes('*');
};

const countUsableTokensForSite = (site) => {
  const tokens = Array.isArray(site?.tokens) ? site.tokens : [];
  return tokens.filter(isUsableToken).length;
};

const DESKTOP_PROFILE_ASSIST_HOSTS = new Set([
  'anyrouter.top',
  'elysiver.h-e.top',
]);

const PROFILE_ASSIST_ENABLED = true;

const isDesktopProfileAssistSite = (site) => {
  if (!PROFILE_ASSIST_ENABLED) return false;
  if (isAnyrouterSite(site)) return true;
  const rawUrl = String(site?.site_url || site?.siteUrl || '').trim();
  if (!rawUrl) return false;
  try {
    const parsed = new URL(rawUrl);
    const host = String(parsed.hostname || '').toLowerCase();
    return DESKTOP_PROFILE_ASSIST_HOSTS.has(host);
  } catch {
    return false;
  }
};

const shouldUseDesktopProfileAssist = (site) => (
  PROFILE_ASSIST_ENABLED &&
  isDesktopProfileAssistSite(site) &&
  (
    Boolean(site?.error) ||
    countUsableTokensForSite(site) <= 0
  )
);

const markLocalProfileExtractionStart = (total) => {
  resetFetchKeysProgress();
  fetchKeysProgress.active = true;
  fetchKeysProgress.stage = 'profile_prepare';
  fetchKeysProgress.detail = '准备读取 Chrome Default Profile';
  fetchKeysProgress.total = Number(total || 0);
  fetchKeysProgress.completed = 0;
  fetchKeysProgress.successSites = 0;
  fetchKeysProgress.lastSiteName = 'Chrome Default Profile';
  fetchKeysProgress.startedAt = Date.now();
  fetchKeysProgress.lastUpdatedAt = Date.now();
};

const markLocalProfileExtractionDone = (sites) => {
  const safeSites = Array.isArray(sites) ? sites : [];
  fetchKeysProgress.active = false;
  fetchKeysProgress.stage = 'done';
  fetchKeysProgress.detail = '提取完成，正在整理结果';
  fetchKeysProgress.completed = safeSites.length;
  fetchKeysProgress.successSites = safeSites.filter(site => Array.isArray(site?.tokens) && site.tokens.length > 0).length;
  fetchKeysProgress.lastSiteName = 'Chrome Default Profile';
  fetchKeysProgress.lastUpdatedAt = Date.now();
};

const markBrowserExtractionStart = (total) => {
  resetFetchKeysProgress();
  fetchKeysProgress.active = true;
  fetchKeysProgress.stage = 'extract_site';
  fetchKeysProgress.detail = '正在提取站点 Token';
  fetchKeysProgress.total = Number(total || 0);
  fetchKeysProgress.completed = 0;
  fetchKeysProgress.successSites = 0;
  fetchKeysProgress.lastSiteName = '';
  fetchKeysProgress.startedAt = Date.now();
  fetchKeysProgress.lastUpdatedAt = Date.now();
};

const markBrowserExtractionProgress = (siteName, succeeded) => {
  fetchKeysProgress.completed = Math.min(fetchKeysProgress.completed + 1, fetchKeysProgress.total || 0);
  if (succeeded) {
    fetchKeysProgress.successSites += 1;
  }
  fetchKeysProgress.lastSiteName = String(siteName || '').trim();
  fetchKeysProgress.lastUpdatedAt = Date.now();
};

const markBrowserExtractionDone = (sites) => {
  const safeSites = Array.isArray(sites) ? sites : [];
  fetchKeysProgress.active = false;
  fetchKeysProgress.stage = 'done';
  fetchKeysProgress.detail = '提取完成，正在整理结果';
  fetchKeysProgress.completed = safeSites.length;
  fetchKeysProgress.successSites = safeSites.filter(site => Array.isArray(site?.tokens) && site.tokens.length > 0).length;
  fetchKeysProgress.lastSiteName = '';
  fetchKeysProgress.lastUpdatedAt = Date.now();
};

const mergeChromeProfileExtractedSites = (accounts, response) => {
  const resultMap = new Map(
    (Array.isArray(response?.results) ? response.results : []).map(item => [String(item?.id || ''), item])
  );

  return (Array.isArray(accounts) ? accounts : []).map(account => {
    const extracted = resultMap.get(String(account?.id || ''));
    if (!extracted) {
      return {
        ...account,
        tokens: [],
        error: 'profile_result_missing',
      };
    }

    const resolvedAccessToken = String(
      extracted?.resolved_access_token || account?.account_info?.access_token || ''
    ).trim();
    const resolvedUserId = String(
      extracted?.resolved_user_id || account?.account_info?.id || ''
    ).trim();

    return {
      ...account,
      site_name: extracted?.site_name || account?.site_name,
      site_url: extracted?.site_url || account?.site_url,
      tokens: Array.isArray(extracted?.tokens) ? extracted.tokens : [],
      error: String(extracted?.error || '').trim(),
      _profileStorageFields: Array.isArray(extracted?.storage_fields) ? extracted.storage_fields : [],
      _profileStorageOrigin: String(extracted?.storage_origin || '').trim(),
      account_info: {
        ...(account?.account_info || {}),
        ...(resolvedUserId ? { id: resolvedUserId } : {}),
        ...(resolvedAccessToken ? { access_token: resolvedAccessToken } : {}),
      },
    };
  });
};

const isSelectableModelKey = (key) => {
  const text = String(key || '');
  if (!text.includes('|')) return false;
  if (text.startsWith('token|')) return false;
  if (text.startsWith('fail-site|')) return false;
  if (text.startsWith('no-model-site|')) return false;
  if (text.startsWith('no-usable-token-site|')) return false;
  if (text.startsWith('discover-loading|')) return false;
  const parts = text.split('|');
  return parts.length >= 3;
};

const getModelNameFromSelectableKey = (key) => {
  const parts = String(key || '').split('|');
  if (parts.length < 3) return '';
  return String(parts.slice(2).join('|') || '').trim();
};

const collectSelectableModelKeysFromTreeNodes = (nodes, bucket = []) => {
  (Array.isArray(nodes) ? nodes : []).forEach(node => {
    const key = String(node?.key || '');
    if (node?.isLeaf === true && isSelectableModelKey(key)) {
      bucket.push(key);
      return;
    }

    const children = Array.isArray(node?.children) ? node.children : [];
    if (children.length > 0) {
      collectSelectableModelKeysFromTreeNodes(children, bucket);
    }
  });
  return bucket;
};

const normalizeSelectionTreeNode = (node) => {
  if (!node || typeof node !== 'object') return node;

  const key = String(node?.key || '').trim();
  const children = Array.isArray(node?.children)
    ? node.children.map(child => normalizeSelectionTreeNode(child)).filter(Boolean)
    : [];
  const isLoadingNode = key.startsWith('discover-loading|');
  const isDiagnosticNode =
    isLoadingNode ||
    key.startsWith('fail-site|') ||
    key.startsWith('no-model-site|') ||
    key.startsWith('no-usable-token-site|') ||
    node?.isProviderDiagnostic === true;
  const isSiteRootNode = key.startsWith('site-root|');
  const isTokenNode = key.startsWith('token|');
  const isModelLeafNode = node?.isLeaf === true && isSelectableModelKey(key);

  const normalized = {
    ...node,
    children,
  };

  if (isDiagnosticNode) {
    return normalized;
  }
  if (isSiteRootNode || node?.isSiteRoot === true) {
    return {
      ...normalized,
      checkable: false,
      disabled: normalized?.disabled === true ? true : false,
      disableCheckbox: false,
      class: [String(normalized?.class || '').trim(), 'site-root-summary-node'].filter(Boolean).join(' '),
    };
  }
  if (isTokenNode && children.length > 0) {
    return {
      ...normalized,
      disabled: false,
      disableCheckbox: false,
    };
  }
  if (isModelLeafNode) {
    return {
      ...normalized,
      disabled: false,
      disableCheckbox: false,
    };
  }
  return normalized;
};

const normalizeSelectionTreeNodes = (nodes) => (
  (Array.isArray(nodes) ? nodes : []).map(node => normalizeSelectionTreeNode(node)).filter(Boolean)
);

const mergeExtractedSiteResults = (baseResults, retryResults) => {
  const merged = Array.isArray(baseResults) ? baseResults : [];
  const stats = {
    mergedSites: 0,
    recoveredSites: 0,
    gainedTokens: 0,
    gainedUsableTokens: 0,
    changedSiteIds: [],
  };
  const changedSiteIdSet = new Set();

  retryResults.forEach(retryResult => {
    const idx = merged.findIndex(item => item?.id === retryResult?.id);
    if (idx === -1) return;
    const prev = merged[idx];
    const prevTokenCount = Array.isArray(prev?.tokens) ? prev.tokens.length : 0;
    const prevUsableCount = countUsableTokensForSite(prev);
    const prevInvalid = !prev || prev.error || !Array.isArray(prev.tokens) || prev.tokens.length === 0;
    const nextTokenCount = Array.isArray(retryResult?.tokens) ? retryResult.tokens.length : 0;
    const shouldReplace = nextTokenCount > 0 || prevInvalid;
    if (!shouldReplace) return;

    merged[idx] = retryResult;
    const nextUsableCount = countUsableTokensForSite(retryResult);
    stats.mergedSites += 1;
    if (prevInvalid && nextTokenCount > 0) {
      stats.recoveredSites += 1;
    }
    stats.gainedTokens += Math.max(0, nextTokenCount - prevTokenCount);
    stats.gainedUsableTokens += Math.max(0, nextUsableCount - prevUsableCount);
    const changedId = String(retryResult?.id ?? '').trim();
    if (changedId) changedSiteIdSet.add(changedId);
  });

  stats.changedSiteIds = Array.from(changedSiteIdSet);
  return stats;
};

const normalizeFallbackBrowserType = (value) => {
  return value === 'edge' ? 'edge' : 'chrome';
};

const getDetectedFallbackBrowser = async () => {
  const res = await apiFetch('/api/browser-session/browsers');
  if (!res.ok) {
    const text = await res.text().catch(() => '');
    throw new Error(text || `探测系统浏览器失败(${res.status})`);
  }

  const data = await res.json().catch(() => ({}));
  const browsers = Array.isArray(data?.browsers) ? data.browsers : [];
  const availableTypes = browsers
    .map(item => item?.type)
    .filter(type => type === 'chrome' || type === 'edge');

  if (!availableTypes.length) {
    throw new Error('系统未检测到可用的 Chrome 或 Edge');
  }

  const saved = normalizeFallbackBrowserType(localStorage.getItem(FALLBACK_BROWSER_STORAGE_KEY) || '');
  const browserType = availableTypes.includes(saved)
    ? saved
    : (availableTypes.includes(data?.defaultBrowser) ? data.defaultBrowser : availableTypes[0]);

  localStorage.setItem(FALLBACK_BROWSER_STORAGE_KEY, browserType);
  return {
    browserType,
    availableTypes,
  };
};

const getFallbackBrowserStatus = async (browserType = 'chrome') => {
  const normalizedType = normalizeFallbackBrowserType(browserType);
  const res = await apiFetch(`/api/browser-session/status?browserType=${normalizedType}`);
  if (!res.ok) {
    return { running: false, attached: false, browserType: normalizedType };
  }

  const data = await res.json().catch(() => ({}));
  return {
    running: Boolean(data?.running),
    attached: Boolean(data?.attached),
    launching: Boolean(data?.launching),
    managed: Boolean(data?.managed),
    browserType: data?.browserType || normalizedType,
  };
};

const chooseDetectedFallbackBrowserType = ({ browserType, availableTypes }) => {
  return new Promise(resolve => {
    if (!Array.isArray(availableTypes) || availableTypes.length <= 1) {
      localStorage.setItem(FALLBACK_BROWSER_STORAGE_KEY, browserType);
      resolve(browserType);
      return;
    }

    Modal.confirm({
      title: '选择兜底浏览器',
      content: `检测到多个浏览器可用：${availableTypes.map(type => type === 'edge' ? 'Edge' : 'Chrome').join(' / ')}。请选择要用于兜底抓取的浏览器。`,
      okText: availableTypes.includes('chrome') ? '使用 Chrome' : '继续',
      cancelText: availableTypes.includes('edge') ? '使用 Edge' : '取消',
      closable: false,
      maskClosable: false,
      onOk: () => {
        const finalBrowserType = availableTypes.includes('chrome') ? 'chrome' : browserType;
        localStorage.setItem(FALLBACK_BROWSER_STORAGE_KEY, finalBrowserType);
        resolve(finalBrowserType);
      },
      onCancel: () => {
        if (availableTypes.includes('edge')) {
          localStorage.setItem(FALLBACK_BROWSER_STORAGE_KEY, 'edge');
          resolve('edge');
          return;
        }
        resolve(null);
      },
    });
  });
};

const confirmWithModal = ({ title, content, okText = '确定', cancelText = '取消', okType = 'primary' }) => {
  return new Promise(resolve => {
    Modal.confirm({
      title,
      content,
      okText,
      cancelText,
      okType,
      onOk: () => resolve(true),
      onCancel: () => resolve(false),
    });
  });
};

const openFailedSitesForManualLogin = async (sites) => {
  const urls = Array.from(new Set(
    (Array.isArray(sites) ? sites : [])
      .map(site => String(site?.site_url || site?.siteUrl || '').replace(/\/+$/, '').trim())
      .filter(url => /^https?:\/\//i.test(url))
  ));

  urls.forEach(url => openUrlInSystemBrowser(url));
  return urls.length;
};

const getProfileSiteErrorCode = (site) => normalizeErrorCodeForDisplay(site?.error || '');

const shouldRetryProfileFileSite = (site) => (
  !site ||
  site.error ||
  !Array.isArray(site.tokens) ||
  site.tokens.length === 0 ||
  shouldUseDesktopProfileAssist(site)
);

const collectProfileFileRetrySites = (sites) => (
  (Array.isArray(sites) ? sites : []).filter(site => shouldRetryProfileFileSite(site))
);

const autoOpenExpiredTokenSitesForRelogin = async (sites, reason = 'unknown') => {
  const targets = (Array.isArray(sites) ? sites : [])
    .filter(site => ['TOKEN_EXPIRED', 'TOKEN_EXPIRED_LOCAL'].includes(getProfileSiteErrorCode(site)) && !site?._profileReloginOpened);
  if (!targets.length) return 0;

  const openedCount = await openFailedSitesForManualLogin(targets);
  if (openedCount > 0) {
    targets.forEach(site => {
      site._profileReloginOpened = true;
    });
    message.warning(`检测到 ${openedCount} 个站点 Token 已失效，已在本机浏览器打开对应页面，请重新登录后再继续读取。`, 6);
    console.log(`[ProfileRelogin] auto open (${reason}) opened=${openedCount}`);
  }
  return openedCount;
};

const isAnyrouterSite = (site) => {
  const siteType = String(site?.site_type || site?.siteType || '').trim().toLowerCase();
  if (siteType === 'anyrouter') return true;
  const rawUrl = String(site?.site_url || site?.siteUrl || '').trim();
  if (!rawUrl) return false;
  try {
    const parsed = new URL(rawUrl);
    const host = String(parsed.hostname || '').toLowerCase();
    return host === 'anyrouter.top' || host.endsWith('.anyrouter.top');
  } catch {
    return false;
  }
};

const getNormalizedUrlHost = (rawUrl) => {
  const text = String(rawUrl || '').trim();
  if (!text) return '';
  try {
    return String(new URL(text).hostname || '').toLowerCase();
  } catch {
    return '';
  }
};

const getDesktopProfileProgressDirect = async () => {
  const getter = window?.go?.main?.App?.GetChromeProfileExtractProgress;
  if (typeof getter !== 'function') {
    return null;
  }
  return await getter();
};

const openDesktopProfileAssistSites = async (sites) => {
  const payload = (Array.isArray(sites) ? sites : [])
    .map(site => ({
      siteName: String(site?.site_name || site?.siteName || '').trim() || '站点',
      siteUrl: String(site?.site_url || site?.siteUrl || '').replace(/\/+$/, '').trim(),
      siteType: String(site?.site_type || site?.siteType || '').trim(),
    }))
    .filter(site => /^https?:\/\//i.test(site.siteUrl));

  if (!payload.length) {
    return { opened: 0, results: [], errors: [] };
  }

  const opener = window?.go?.main?.App?.OpenDesktopProfileAssist;
  if (typeof opener === 'function') {
    const directResult = await opener(payload);
    return {
      opened: Number(directResult?.opened || 0),
      results: Array.isArray(directResult?.results) ? directResult.results : [],
      errors: Array.isArray(directResult?.errors) ? directResult.errors : [],
    };
  }

  const res = await apiFetch('/api/profile-assist/open', {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ sites: payload }),
  });

  const data = await res.json().catch(() => ({}));
  if (!res.ok) {
    const error = new Error(
      Array.isArray(data?.errors) && data.errors.length
        ? data.errors.join(' | ')
        : (data?.message || `打开桌面内置登录窗口失败(${res.status})`)
    );
    error.code = data?.code || `HTTP_${res.status}`;
    throw error;
  }

  return {
    opened: Number(data?.opened || 0),
    results: Array.isArray(data?.results) ? data.results : [],
    errors: Array.isArray(data?.errors) ? data.errors : [],
  };
};

const closeDesktopProfileAssistSites = async (sites, reason = 'unknown') => {
  if (!isWailsRuntime) return 0;
  const hostSet = new Set(
    (Array.isArray(sites) ? sites : [])
      .map(site => getNormalizedUrlHost(site?.site_url || site?.siteUrl || ''))
      .filter(Boolean)
  );
  const hosts = Array.from(hostSet);
  if (!hosts.length) return 0;

  try {
    const closer = window?.go?.main?.App?.CloseDesktopProfileAssist;
    if (typeof closer === 'function') {
      const res = await closer(hosts);
      const closed = Number(res?.closed || 0);
      if (closed > 0) {
        console.log(`[ProfileAssist] auto close (${reason}) closed=${closed}`);
      }
      return closed;
    }

    const res = await apiFetch('/api/profile-assist/close', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ hosts }),
    });
    const data = await res.json().catch(() => ({}));
    const closed = Number(data?.closed || 0);
    if (closed > 0) {
      console.log(`[ProfileAssist] auto close (${reason}) closed=${closed}`);
    }
    return closed;
  } catch (err) {
    console.warn('[ProfileAssist] auto close failed:', err?.message || String(err));
    return 0;
  }
};

const closeDesktopProfileAssistForRecoveredSites = async (sites, reason = 'unknown') => {
  const targets = (Array.isArray(sites) ? sites : [])
    .filter(site => isDesktopProfileAssistSite(site) && site?._profileAssistOpened)
    .filter(site => !isSiteFailed(site) && Array.isArray(site?.tokens) && site.tokens.length > 0);
  if (!targets.length) return 0;
  return await closeDesktopProfileAssistSites(targets, reason);
};

const autoOpenDesktopProfileAssist = async (sites, reason = 'unknown') => {
  const targets = (Array.isArray(sites) ? sites : [])
    .filter(site => shouldUseDesktopProfileAssist(site) && !site?._profileAssistOpened);
  if (!targets.length) return { opened: 0, results: [], errors: [] };

  const assistResult = await openDesktopProfileAssistSites(targets);
  if (Number(assistResult?.opened || 0) > 0) {
    const openedHostSet = new Set(
      (assistResult?.results || [])
        .map(item => getNormalizedUrlHost(item?.siteUrl || ''))
        .filter(Boolean)
    );
    targets.forEach(site => {
      const siteHost = getNormalizedUrlHost(site?.site_url || site?.siteUrl || '');
      if (siteHost && openedHostSet.has(siteHost)) {
        site._profileAssistOpened = true;
      }
    });
    message.info(`已自动打开内置 WebView2 (${assistResult.opened} 个站点)，请在窗口内完成登录，系统检测到 Token 后会自动关闭。`, 5);
    console.log(`[ProfileAssist] auto open (${reason}) opened=${assistResult.opened}`);
  }
  if (Array.isArray(assistResult?.errors) && assistResult.errors.length > 0) {
    console.warn(`[ProfileAssist] auto open warnings(${reason}):`, assistResult.errors.join(' | '));
  }
  return assistResult;
};

const confirmShadowLoginReadiness = async (sites, browserType = 'chrome') => {
  const normalizedSites = Array.isArray(sites) ? sites.filter(Boolean) : [];
  const previewNames = normalizedSites
    .map(site => String(site?.site_name || site?.siteName || '').trim())
    .filter(Boolean)
    .slice(0, 6);
  const remaining = normalizedSites.length - previewNames.length;
  const siteSummary = previewNames.join('、') + (remaining > 0 ? ` 等 ${normalizedSites.length} 个站点` : '');
  const browserLabel = browserType === 'edge' ? 'Edge' : 'Chrome';
  const action = await new Promise(resolve => {
    Modal.confirm({
      title: '非 CDP 模式登录确认',
      content: `接下来会使用 ${browserLabel} 的 shadow 模式继续老流程抓取。请先确认当前失效站点都已经在普通浏览器里登录完成；Google 关联登录在此模式下可能失效。${siteSummary ? `本轮待处理站点：${siteSummary}。` : ''}`,
      okText: '打开并确认',
      cancelText: '跳过',
      okType: 'primary',
      onOk: () => resolve('open'),
      onCancel: () => resolve('skip'),
    });
  });

  if (action === 'skip') {
    return true;
  }

  const openedCount = await openFailedSitesForManualLogin(normalizedSites);
  if (openedCount <= 0) {
    message.warning('没有可打开的失败站点 URL，将直接继续 shadow 抓取。');
    return true;
  }

  return await confirmWithModal({
    title: '站点已打开',
    content: `已在普通浏览器中打开 ${openedCount} 个失败站点。请先完成登录确认，再继续拉起 shadow 浏览器执行老流程；后续受控浏览器会最小化启动。`,
    okText: '登录完成，继续',
    cancelText: '取消本次抓取',
    okType: 'primary',
  });
};

const confirmProfileFileLoginRecovery = async (sites) => {
  const normalizedSites = Array.isArray(sites) ? sites.filter(Boolean) : [];
  const previewNames = normalizedSites
    .map(site => String(site?.site_name || site?.siteName || '').trim())
    .filter(Boolean)
    .slice(0, 6);
  const remaining = normalizedSites.length - previewNames.length;
  const siteSummary = previewNames.join('、') + (remaining > 0 ? ` 等 ${normalizedSites.length} 个站点` : '');

  const action = await new Promise(resolve => {
    Modal.confirm({
      title: 'Profile 文件模式登录确认',
      content: `以下站点暂未能从本地 Chrome Profile 读取到可用 Token。通常是这些站点尚未在默认 Chrome Profile 中登录，或登录态还没刷新到本地存储。${siteSummary ? `本轮待处理站点：${siteSummary}。` : ''}是否现在为你打开这些站点，让你手动登录后再重新读取？`,
      okText: '打开站点',
      cancelText: '稍后处理',
      okType: 'primary',
      onOk: () => resolve('open'),
      onCancel: () => resolve('skip'),
    });
  });

  if (action !== 'open') {
    return { shouldRetry: false, openedCount: 0 };
  }

  const desktopAssistSites = (
    PROFILE_FILE_WEBVIEW_FALLBACK_ENABLED &&
    isWailsRuntime
  )
    ? normalizedSites.filter(site => isDesktopProfileAssistSite(site) && !site?._profileAssistOpened)
    : [];
  const browserSites = normalizedSites.filter(site => !desktopAssistSites.includes(site));

  let desktopAssistOpened = 0;
  if (desktopAssistSites.length > 0) {
    try {
      const assistResult = await openDesktopProfileAssistSites(desktopAssistSites);
      desktopAssistOpened = Number(assistResult?.opened || 0);
      if (desktopAssistOpened > 0) {
        message.info(`已为 ${desktopAssistOpened} 个站点打开内置 WebView2，请在窗口内完成登录。`, 5);
      }
      if (Array.isArray(assistResult?.errors) && assistResult.errors.length > 0) {
        console.warn('[ProfileAssist] desktop assist warnings:', assistResult.errors.join(' | '));
      }
    } catch (assistError) {
      console.warn('[ProfileAssist] open desktop profile assist failed:', assistError?.message || String(assistError));
      message.warning(`内置登录窗口打开失败，将回退到系统浏览器：${assistError?.message || String(assistError)}`, 5);
      browserSites.push(...desktopAssistSites);
      desktopAssistOpened = 0;
    }
  }

  const browserOpenedCount = await openFailedSitesForManualLogin(browserSites);
  const openedCount = desktopAssistOpened + browserOpenedCount;
  if (openedCount <= 0) {
    message.warning('没有可打开的失败站点 URL，当前无法执行手动登录引导。');
    return { shouldRetry: false, openedCount: 0 };
  }

  const shouldRetry = await confirmWithModal({
    title: '站点已打开',
    content: desktopAssistOpened > 0
      ? `已打开 ${openedCount} 个失败站点，其中 ${desktopAssistOpened} 个站点使用内置 WebView2 直接打开登录页。请在这些窗口中手动登录并完成刷新后，再点击“重新读取 Profile 文件”。系统随后会最多尝试重新读取 3 轮。`
      : `已在默认浏览器打开 ${openedCount} 个失败站点。请先在这些页面手动登录并完成刷新，然后点击“重新读取 Profile 文件”。此模式不会拉起受控浏览器，也不会切到 CDP 模式。`,
    okText: '我已登录，重新读取',
    cancelText: '稍后处理',
    okType: 'primary',
  });

  return { shouldRetry, openedCount };
};

const openSitesInBrowserSession = async (sites, browserType = 'chrome') => {
  const payload = sites
    .map(site => ({
      name: site?.site_name || '未知站点',
      url: String(site?.site_url || '').replace(/\/+$/, ''),
    }))
    .filter(site => /^https?:\/\//i.test(site.url));

  if (!payload.length) return 0;

  const res = await apiFetch('/api/browser-session/open', {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ sites: payload, browserType }),
  });

  if (!res.ok) {
    const data = await res.json().catch(() => null);
    const error = new Error(data?.message || `打开受控浏览器失败(${res.status})`);
    error.code = data?.code || `HTTP_${res.status}`;
    throw error;
  }

  const data = await res.json().catch(() => ({}));
  return Number(data?.opened || payload.length);
};

const restartBrowserSessionProcessAndOpen = async (sites, browserType = 'chrome') => {
  const payload = sites
    .map(site => ({
      name: site?.site_name || '未知站点',
      url: String(site?.site_url || '').replace(/\/+$/, ''),
    }))
    .filter(site => /^https?:\/\//i.test(site.url));

  const res = await apiFetch('/api/browser-session/restart-open', {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ browserType, sites: payload }),
  });

  if (!res.ok) {
    const data = await res.json().catch(() => null);
    const error = new Error(data?.message || `重启浏览器并打开站点失败(${res.status})`);
    error.code = data?.code || `HTTP_${res.status}`;
    throw error;
  }

  return await res.json().catch(() => ({}));
};

const browserSessionFetchForAccounts = async (accounts, browserType = 'chrome', round = 1, totalRounds = 1) => {
  if (!accounts.length) return [];

  const res = await apiFetch('/api/browser-session/fetch-keys', {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ accounts, browserType, round, totalRounds }),
  });

  if (!res.ok) {
    const text = await res.text().catch(() => '');
    throw new Error(text || `浏览器会话抓取失败(${res.status})`);
  }

  const data = await res.json().catch(() => ({}));
  return Array.isArray(data?.results) ? data.results : [];
};

const sleep = (ms) => new Promise(resolve => setTimeout(resolve, ms));

const createCdpPendingSites = (accounts) => (
  (Array.isArray(accounts) ? accounts : []).map(account => ({
    ...account,
    tokens: [],
    error: '等待 CDP 抓取',
  }))
);

const openSitesForCdpRestartMode = async (sites, browserType) => {
  let openedCount = 0;
  try {
    openedCount = await openSitesInBrowserSession(sites, browserType);
  } catch (openErr) {
    const fallbackStatus = await getFallbackBrowserStatus(browserType).catch(() => ({
      running: false,
      attached: false,
      launching: false,
      managed: false,
      browserType,
    }));
    const shouldHandleAsProfileInUse =
      openErr?.code === 'BROWSER_PROFILE_IN_USE' ||
      (fallbackStatus.running && !fallbackStatus.attached && !fallbackStatus.launching && !fallbackStatus.managed);

    if (!shouldHandleAsProfileInUse) {
      throw openErr;
    }

    const shouldKill = await confirmWithModal({
      title: '浏览器已占用',
      content: `${browserType === 'edge' ? 'Edge' : 'Chrome'} 当前已在普通模式运行，默认 profile 被占用。结束后会关闭该浏览器所有窗口，是否继续？`,
      okText: '结束并继续',
      cancelText: '取消',
      okType: 'danger',
    });
    if (!shouldKill) {
      return { cancelled: true, openedCount: 0 };
    }

    const restartResult = await restartBrowserSessionProcessAndOpen(sites, browserType);
    if (!restartResult?.stopped) {
      throw new Error(`${browserType === 'edge' ? 'Edge' : 'Chrome'} 进程结束后仍未完全退出，请手动关闭后重试。`);
    }
    openedCount = Number(restartResult?.opened || sites.length);
  }

  return { cancelled: false, openedCount };
};

const startCdpRestartMode = async (accounts, isSiteFailed) => {
  const detected = await getDetectedFallbackBrowser();
  const browserType = await chooseDetectedFallbackBrowserType(detected);
  if (!browserType) {
    return { cancelled: true, reason: 'browser_not_selected' };
  }

  const readyForShadow = await confirmShadowLoginReadiness(accounts, browserType);
  if (!readyForShadow) {
    return { cancelled: true, reason: 'login_not_confirmed' };
  }

  const openResult = await openSitesForCdpRestartMode(accounts, browserType);
  if (openResult.cancelled) {
    return { cancelled: true, reason: 'browser_kill_cancelled' };
  }
  if (openResult.openedCount <= 0) {
    return { cancelled: true, reason: 'no_site_opened' };
  }

  const availableText = detected.availableTypes.map(type => (type === 'edge' ? 'Edge' : 'Chrome')).join(' / ');
  message.info(`已探测到 ${availableText}，当前使用 ${browserType === 'edge' ? 'Edge' : 'Chrome'} 打开 ${openResult.openedCount} 个站点，开始执行 CDP 抓取。`, 6);

  const maxRetryRounds = 3;
  const retryIntervalMs = 15000;
  let extractedSites = createCdpPendingSites(accounts);
  let pendingSites = extractedSites.filter(isSiteFailed);

  browserSessionPolling.active = true;
  browserSessionPolling.totalRounds = maxRetryRounds;
  browserSessionPolling.round = 1;
  browserSessionPolling.pending = pendingSites.length;
  updateBrowserSessionPendingSites(pendingSites);

  const firstRoundResults = await browserSessionFetchForAccounts(accounts, browserType, 1, maxRetryRounds);
  mergeExtractedSiteResults(extractedSites, firstRoundResults);
  pendingSites = extractedSites.filter(isSiteFailed);
  browserSessionPolling.pending = pendingSites.length;
  updateBrowserSessionPendingSites(pendingSites);

  return {
    cancelled: false,
    extractedSites,
    pendingSites,
    browserType,
    maxRetryRounds,
    retryIntervalMs,
  };
};

// --- 浏览器端直接提取Token（绕过Cloudflare WAF服务端拦截）---
// 核心原理：Cloudflare Bot Protection会拦截无TLS指纹的服务器请求，
// 但放行真实浏览器发出的请求（有JA3 TLS指纹+clearance cookie）
const fetchTokensForAccountFromBrowser = async (acc) => {
  const { id, site_name, site_url, site_type, account_info } = acc;
  const apiKey = account_info?.access_token;
  const baseUrl = (site_url || '').replace(/\/+$/, '');
  const uid = account_info?.id;

  if (!apiKey || !baseUrl) {
    return { id, site_name, site_url, tokens: [], error: '缺少 access_token 或 site_url', account_info };
  }

  // 优先级端点列表：参考all-api-hub的实现策略
  let endpoints;
  if (site_type === 'sub2api') {
    // sub2api使用JWT token，对应不同的API路径
    endpoints = [
      `/api/v1/keys?page=1&page_size=100`,
      `/api/v1/keys?p=0&size=100`,
      `/api/token/?p=0&size=100`,
    ];
  } else {
    // oneAPI / newAPI / anyrouter 等
    endpoints = site_type === 'anyrouter'
      ? [
        `/api/token/?p=0&size=100`,
        `/api/token?p=0&size=100`,
      ]
      : [
        `/api/token/?p=0&size=100`,
        `/api/token?p=0&size=100`,
        `/api/v1/keys?page=1&page_size=100`,
      ];
  }

  const headers = {
    'Authorization': `Bearer ${apiKey}`,
    'Accept': 'application/json, text/plain, */*',
    'Accept-Language': 'zh-CN,zh;q=0.9,en;q=0.8',
    'X-Requested-With': 'XMLHttpRequest',
  };
  // 如果uid是纯数字，加入兼容头（参考all-api-hub的compat headers）
  if (uid && /^\d+$/.test(String(uid))) {
    headers['one-api-user'] = String(uid);
    headers['New-API-User'] = String(uid);
    headers['Veloera-User'] = String(uid);
    headers['voapi-user'] = String(uid);
    headers['User-id'] = String(uid);
    headers['Rix-Api-User'] = String(uid);
    headers['neo-api-user'] = String(uid);
  }

  const isMaskedKey = (value) => {
    const key = String(value || '').trim();
    if (!key) return false;
    return key.includes('*') || key.includes('***');
  };

  const extractSecretKeyFromPayload = (payload) => {
    if (!payload) return '';
    if (typeof payload === 'string') return payload.trim();
    if (typeof payload !== 'object') return '';
    const candidates = [
      payload?.key,
      payload?.data?.key,
      payload?.data,
      payload?.result?.key,
      payload?.result?.data?.key,
      payload?.token,
    ];
    for (const candidate of candidates) {
      if (typeof candidate === 'string' && candidate.trim()) {
        return candidate.trim();
      }
    }
    return '';
  };

  for (const endpoint of endpoints) {
    try {
      const url = `${baseUrl}${endpoint}`;
      const controller = new AbortController();
      const timeout = setTimeout(() => controller.abort(), 10000);

      const response = await fetch(url, {
        method: 'GET',
        headers,
        signal: controller.signal,
        credentials: 'include',
        mode: 'cors',
        referrer: `${baseUrl}/`,
      });
      clearTimeout(timeout);

      if (!response.ok) {
        // 403被CF拦截：检查是否返回了HTML（CF页面）
        if (response.status === 403) {
          const ct = response.headers.get('content-type') || '';
          if (/html/i.test(ct)) {
            // CF Bot Protection，浏览器也无法直接绕（需要challenge）
            // 继续试其他端点
            continue;
          }
        }
        continue;
      }

      // 检查Content-Type，CF挡截页也可能是200但返回HTML
      const ct = response.headers.get('content-type') || '';
      if (/html/i.test(ct)) continue;

      let body;
      try {
        body = await response.json();
      } catch (e) {
        continue; // 非JSON，跳过
      }

      // 解析不同格式的响应
      let items = [];
      if (body && body.data !== undefined) {
        const data = body.data;
        if (Array.isArray(data)) items = data;
        else if (data && Array.isArray(data.items)) items = data.items;
      } else if (body && Array.isArray(body.items)) {
        items = body.items;
      } else if (Array.isArray(body)) {
        items = body;
      }

      const resolvedItems = [];
      for (const t of items) {
        const rawKey = t.key || t.access_token || t.token || t.api_key || t.apikey || (typeof t === 'string' ? t : '');
        resolvedItems.push({ ...t, key: rawKey || '未知格式Token' });
      }

      // 二次处理：掩码 key 尝试补全，避免“提取数量很多但最终可用很少”
      if (resolvedItems.length > 0) {
        const normalizedResolvedItems = [];
        for (const t of items) {
          const rawKey = t.key || t.access_token || t.token || t.api_key || t.apikey || (typeof t === 'string' ? t : '');
          let resolvedKey = rawKey || '';
          let unresolved = false;

          if (isMaskedKey(rawKey) && t?.id) {
            const secretEndpointCandidates = [
              { path: `/api/token/${t.id}/key`, method: 'POST' },
              { path: `/api/token/${t.id}/key`, method: 'GET' },
              { path: `/api/token/${t.id}`, method: 'GET' },
              { path: `/api/v1/keys/${t.id}`, method: 'GET' },
            ];
            for (const secretEp of secretEndpointCandidates) {
              try {
                const secretRes = await fetch(`${baseUrl}${secretEp.path}`, {
                  method: secretEp.method,
                  headers: {
                    ...headers,
                    ...(secretEp.method !== 'GET' ? { 'Content-Type': 'application/json' } : {}),
                  },
                  credentials: 'include',
                  mode: 'cors',
                  referrer: `${baseUrl}/`,
                });
                if (!secretRes.ok) continue;
                const secretBody = await secretRes.json().catch(() => null);
                const fullKey = extractSecretKeyFromPayload(secretBody);
                if (fullKey) {
                  resolvedKey = fullKey;
                  break;
                }
              } catch {}
            }
            unresolved = isMaskedKey(resolvedKey);
          }

          normalizedResolvedItems.push({
            ...t,
            key: resolvedKey || '未知格式Token',
            unresolved,
          });
        }
        resolvedItems.length = 0;
        resolvedItems.push(...normalizedResolvedItems);
      }

      if (resolvedItems && resolvedItems.length > 0) {
        console.log(`[BrowserFetch] ${site_name} | ${endpoint} => ${resolvedItems.length}个token`);
        return { id, site_name, site_url, tokens: resolvedItems, endpoint, account_info, _browserFetched: true };
      }
    } catch (err) {
      if (err.name === 'AbortError') continue;
      // CORS错误或网络错误，继续
      console.debug(`[BrowserFetch] ${site_name} | ${endpoint} CORS/网络错误:`, err.message);
      continue;
    }
  }

  // 所有浏览器端端点均失败，返回失败标记（由processAccounts fallback到服务端）
  return {
    id,
    site_name,
    site_url,
    tokens: [],
    error: '浏览器端所有端点均失败，将尝试服务端代理',
    account_info,
    _needServerFallback: true,
    _browserFetchFailed: true,
  };
};

const extractBrowserListItems = (body) => {
  if (Array.isArray(body)) return body;
  if (!body || typeof body !== 'object') return [];
  if (Array.isArray(body.items)) return body.items;
  if (Array.isArray(body.data)) return body.data;
  if (body.data && typeof body.data === 'object') {
    if (Array.isArray(body.data.items)) return body.data.items;
    if (Array.isArray(body.data.data)) return body.data.data;
  }
  return [];
};

const extractSecretKeyFromPayloadForBrowser = (payload) => {
  if (!payload) return '';
  if (typeof payload === 'string') return payload.trim();
  if (typeof payload !== 'object') return '';
  const candidates = [
    payload?.key,
    payload?.data?.key,
    payload?.data,
    payload?.result?.key,
    payload?.result?.data?.key,
    payload?.token,
  ];
  for (const candidate of candidates) {
    if (typeof candidate === 'string' && candidate.trim()) {
      return candidate.trim();
    }
  }
  return '';
};

const fetchTokensForAccountFromBrowserV2 = async (acc) => {
  const { id, site_name, site_url, site_type, account_info } = acc;
  const apiKey = String(account_info?.access_token || '').trim();
  const baseUrl = String(site_url || '').replace(/\/+$/, '');
  const uid = String(account_info?.id || '').trim();

  if (!apiKey || !baseUrl) {
    return { id, site_name, site_url, tokens: [], error: '缺少 access_token 或 site_url', account_info };
  }

  const endpoints = site_type === 'anyrouter'
    ? [
      '/api/token/?p=0&size=100',
      '/api/token?p=0&size=100',
    ]
    : site_type === 'sub2api'
      ? [
        '/api/v1/keys?page=1&page_size=100',
        '/api/v1/keys?p=0&size=100',
        '/api/token/?p=0&size=100',
        '/api/token?p=0&size=100',
      ]
      : [
        '/api/token/?p=0&size=100',
        '/api/token?p=0&size=100',
        '/api/v1/keys?page=1&page_size=100',
        '/api/v1/keys?p=0&size=100',
      ];

  const headers = {
    Authorization: `Bearer ${apiKey}`,
    Accept: 'application/json, text/plain, */*',
    'Accept-Language': 'zh-CN,zh;q=0.9,en;q=0.8',
    'X-Requested-With': 'XMLHttpRequest',
  };

  if (/^\d+$/.test(uid)) {
    headers['one-api-user'] = uid;
    headers['New-API-User'] = uid;
    headers['Veloera-User'] = uid;
    headers['voapi-user'] = uid;
    headers['User-id'] = uid;
    headers['Rix-Api-User'] = uid;
    headers['neo-api-user'] = uid;
  }

  const isMaskedKey = (value) => {
    const key = String(value || '').trim();
    return Boolean(key) && key.includes('*');
  };

  const resolveMaskedKey = async (tokenId) => {
    const endpointCandidates = [
      { path: `/api/token/${tokenId}/key`, method: 'POST' },
      { path: `/api/token/${tokenId}/key`, method: 'GET' },
      { path: `/api/token/${tokenId}`, method: 'GET' },
      { path: `/api/v1/keys/${tokenId}`, method: 'GET' },
    ];

    for (const endpoint of endpointCandidates) {
      try {
        const res = await fetch(`${baseUrl}${endpoint.path}`, {
          method: endpoint.method,
          headers: {
            ...headers,
            ...(endpoint.method !== 'GET' ? { 'Content-Type': 'application/json' } : {}),
          },
          credentials: 'include',
          mode: 'cors',
          referrer: `${baseUrl}/`,
        });
        if (!res.ok) continue;
        const payload = await res.json().catch(() => null);
        const key = extractSecretKeyFromPayloadForBrowser(payload);
        if (key) return key;
      } catch {}
    }
    return '';
  };

  const ENDPOINT_TIMEOUT_MS = 8000;
  const ENDPOINT_MAX_RETRIES = 2; // total attempts = 1 + retries
  const maxAttempts = ENDPOINT_MAX_RETRIES + 1;
  const activeControllers = new Set();
  let resolved = false;

  const abortAll = () => {
    activeControllers.forEach(controller => controller.abort());
    activeControllers.clear();
  };

  const attemptEndpoint = async (endpoint) => {
    for (let attempt = 1; attempt <= maxAttempts; attempt += 1) {
      if (resolved) throw new Error('resolved_by_other');
      const url = `${baseUrl}${endpoint}`;
      const controller = new AbortController();
      activeControllers.add(controller);
      const timeout = setTimeout(() => controller.abort(), ENDPOINT_TIMEOUT_MS);
      try {
        const response = await fetch(url, {
          method: 'GET',
          headers,
          signal: controller.signal,
          credentials: 'include',
          mode: 'cors',
          referrer: `${baseUrl}/`,
        });
        clearTimeout(timeout);
        activeControllers.delete(controller);

        if (!response.ok) {
          if (response.status === 403) {
            const ct = response.headers.get('content-type') || '';
            if (/html/i.test(ct)) continue;
          }
          continue;
        }

        const ct = response.headers.get('content-type') || '';
        if (/html/i.test(ct)) continue;

        const body = await response.json().catch(() => null);
        if (!body) continue;

        const items = extractBrowserListItems(body);
        if (!items.length) continue;

        const resolvedItems = [];
        for (const item of items) {
          const rawKey = item?.key || item?.access_token || item?.token || item?.api_key || item?.apikey || (typeof item === 'string' ? item : '');
          let key = String(rawKey || '').trim();
          if (isMaskedKey(key) && item?.id) {
            const fullKey = await resolveMaskedKey(item.id);
            if (fullKey) key = fullKey;
          }
          resolvedItems.push({
            ...item,
            key: key || '未知格式Token',
            unresolved: isMaskedKey(key),
          });
        }

        if (resolvedItems.length > 0) {
          const usableCount = resolvedItems.filter(isUsableToken).length;
          const unresolvedCount = resolvedItems.length - usableCount;
          const detailPreview = resolvedItems
            .slice(0, 5)
            .map((token, idx) => {
              const tokenId = token?.id ?? token?.token_id ?? `idx${idx + 1}`;
              const tokenKey = String(token?.key || '').trim();
              const keyPreview = tokenKey ? `${tokenKey.slice(0, 12)}...${tokenKey.slice(-4)}` : '(empty-key)';
              const tokenName = String(token?.name || token?.token_name || '').trim();
              return `#${tokenId}${tokenName ? `(${tokenName})` : ''}:${keyPreview}`;
            })
            .join(' | ');
          console.log(`[BrowserFetch] [${site_name}] ${endpoint} 获取成功: count=${resolvedItems.length}, usable=${usableCount}, unresolved=${unresolvedCount}, 明细=${detailPreview || '(no-preview)'}`);
          return { id, site_name, site_url, tokens: resolvedItems, endpoint, account_info, _browserFetched: true };
        }
      } catch (err) {
        clearTimeout(timeout);
        activeControllers.delete(controller);
        if (err?.name === 'AbortError') continue;
        if (resolved) throw new Error('resolved_by_other');
        continue;
      }
    }
    throw new Error('endpoint_failed');
  };

  const endpointTasks = endpoints.map(endpoint => attemptEndpoint(endpoint));
  try {
    let result = null;
    if (typeof Promise.any === 'function') {
      result = await Promise.any(endpointTasks);
    } else {
      result = await new Promise((resolve, reject) => {
        let pending = endpointTasks.length;
        let lastError = null;
        endpointTasks.forEach(task => {
          task
            .then(resolve)
            .catch((err) => {
              lastError = err;
              pending -= 1;
              if (pending <= 0) reject(lastError);
            });
        });
      });
    }
    resolved = true;
    abortAll();
    return result;
  } catch {
    resolved = true;
    abortAll();
  }

  return {
    id,
    site_name,
    site_url,
    tokens: [],
    error: '浏览器端所有端点均失败，将尝试服务端代理',
    account_info,
    _needServerFallback: true,
    _browserFetchFailed: true,
  };
};

// --- Upload and Parse ---
const beforeUpload = (file) => {
  const reader = new FileReader();
  reader.onload = async (e) => {
    try {
      const data = JSON.parse(e.target.result);
      if (data && data.accounts && Array.isArray(data.accounts.accounts)) {
        await processAccountsV2(data.accounts.accounts, {
          importSource: 'json_backup',
          forceExtractionMode: 'browser_direct',
        });
        await router.push('/sites');
      } else {
        message.error('无效的文件格式: 缺少 accounts 数组');
      }
    } catch (err) {
      message.error('解析 JSON 文件出错');
    }
  };
  reader.readAsText(file);
  return false; // prevent automatic upload
};

const stopImportExtensionTicking = () => {
  if (importExtensionTickTimer) {
    clearInterval(importExtensionTickTimer);
    importExtensionTickTimer = null;
  }
};

const waitForUiPaint = async () => {
  await nextTick();
  await new Promise(resolve => setTimeout(resolve, 0));
};

const probeBackendHealth = async () => {
  if (!showBackendHealth.value) {
    logClientDiagnostic('batch.health', 'skip probe: not wails runtime');
    return;
  }

  const performType = typeof window?.go?.main?.App?.PerformHttpRequest;
  const performRawType = typeof window?.go?.main?.App?.PerformHttpRequestRaw;
  const appendType = typeof window?.go?.main?.App?.AppendClientLog;
  backendHealth.debug = `isWailsRuntime=${String(isWailsRuntime)}; PerformHttpRequest=${performType}; PerformHttpRequestRaw=${performRawType}; AppendClientLog=${appendType}`;

  try {
    logClientDiagnostic('batch.health', 'probe start');
    const response = await apiFetch('/api/alive');
    if (!response.ok) {
      throw new Error(`HTTP ${response.status}`);
    }
    const payload = await response.json().catch(() => ({}));
    backendHealth.ok = Boolean(payload?.ok ?? true);
    backendHealth.checked = true;
    backendHealth.detail = backendHealth.ok
      ? `桥接模式在线 · ${payload?.mode || 'local'}`
      : '保活响应异常';
    backendHealth.debug = `${backendHealth.debug}; status=${response.status}; payload=${JSON.stringify(payload).slice(0, 240)}`;
    logClientDiagnostic('batch.health', `probe success ok=${backendHealth.ok} detail=${backendHealth.detail}`);
  } catch (error) {
    backendHealth.ok = false;
    backendHealth.checked = true;
    backendHealth.detail = error?.message || '请求失败';
    backendHealth.debug = `${backendHealth.debug}; error=${error?.stack || error?.message || String(error)}`;
    logClientDiagnostic('batch.health', `probe failed: ${backendHealth.detail}`);
  }
};

const setImportExtensionStatus = (text, color = 'processing') => {
  importExtensionStatus.value = String(text || '').trim();
  importExtensionStatusColor.value = color;
};

const resetImportExtensionState = (options = {}) => {
  const { preserveStatus = false } = options;
  if (importExtensionResetTimer) {
    clearTimeout(importExtensionResetTimer);
    importExtensionResetTimer = null;
  }
  stopImportExtensionTicking();
  isImportingExtension.value = false;
  importExtensionElapsedSeconds.value = 0;
  if (!preserveStatus) {
    importExtensionStatus.value = '';
    importExtensionStatusColor.value = 'default';
  }
};

const markImportExtensionBusy = () => {
  resetImportExtensionState();
  isImportingExtension.value = true;
  importExtensionElapsedSeconds.value = 0;
  stopImportExtensionTicking();
  importExtensionTickTimer = setInterval(() => {
    importExtensionElapsedSeconds.value += 1;
  }, 1000);
  setImportExtensionStatus('等待桌面端读取浏览器扩展存储', 'processing');
  importExtensionResetTimer = setTimeout(() => {
    console.warn('[ExtensionImport] loading state watchdog reset');
    setImportExtensionStatus('导入状态已自动复位，请重试', 'warning');
    resetImportExtensionState({ preserveStatus: true });
  }, 20000);
};

const getExtensionImportHintLines = () => ([
  '自动扫描失败后，你可以手动选择任一层级目录继续：',
  '1. 浏览器 User Data 根目录',
  '2. 某个 Profile 目录，例如 Default / Profile 1',
  '3. Local Extension Settings 目录',
  `4. 直接选扩展目录 ${defaultExtensionImportId}`,
  '',
  '常见位置示例：',
  'Windows: %LOCALAPPDATA%\\Google\\Chrome\\User Data\\Default',
  'macOS: ~/Library/Application Support/Google/Chrome/Default',
  'Linux: ~/.config/google-chrome/Default',
]).join('\n');

const defaultExtensionImportId = 'lapnciffpekdengooeolaienkeoilfeo';

const buildExtensionImportFallbackContent = (errorText) => h('div', {
  style: 'display:flex;flex-direction:column;gap:10px;line-height:1.7;',
}, [
  h('div', {
    style: 'color:#4f5f49;font-weight:600;',
  }, `自动导入失败：${errorText || '未知错误'}`),
  h('div', {
    style: 'white-space:pre-wrap;color:#5f6f59;font-size:13px;',
  }, [
    '程序已经把搜索路径与失败原因写入运行日志：\n',
    'runtime/logs/extension-import.log\n\n',
    getExtensionImportHintLines(),
  ].join('')),
  h('div', {
    style: 'color:#7a8675;font-size:12px;',
  }, '点击“选择目录重试”后，可直接选择 User Data、某个 Profile、Local Extension Settings，或扩展 ID 目录。'),
]);

const importAccountsFromExtensionResult = async (result, importSource = 'extension_import') => {
  setImportExtensionStatus('桌面端已返回扩展数据，正在解析账号', 'processing');
  await waitForUiPaint();
  const accounts = result?.payload?.accounts?.accounts;
  if (!Array.isArray(accounts) || accounts.length === 0) {
    setImportExtensionStatus('扩展存储中未找到可用账号数据', 'warning');
    message.warning('扩展存储中未找到可用账号数据');
    return false;
  }
  setImportExtensionStatus(`已读取 ${accounts.length} 个账号，正在构建检测任务`, 'success');
  await waitForUiPaint();
  message.success(`已从扩展导入 ${accounts.length} 个账号`);
  await processAccountsV2(accounts, {
    importSource,
    fallbackOnProfileFailure: true,
  });
  await router.push('/sites');
  return true;
};

const offerManualExtensionImportFallback = async (error) => {
  const picker = window?.go?.main?.App?.PickExtensionImportDirectory;
  const importer = window?.go?.main?.App?.ImportExtensionAccountsFromDir;
  if (typeof picker !== 'function' || typeof importer !== 'function') {
    return false;
  }

  const shouldRetry = await new Promise(resolve => {
    Modal.confirm({
      title: '扩展自动导入失败',
      width: 720,
      okText: '选择目录重试',
      cancelText: '取消',
      content: buildExtensionImportFallbackContent(error?.message || '扩展导入失败'),
      onOk: () => resolve(true),
      onCancel: () => resolve(false),
    });
  });

  if (!shouldRetry) {
    return false;
  }

  let selectedDir = '';
  try {
    selectedDir = await picker();
  } catch (pickError) {
    message.error(`目录选择失败：${pickError?.message || '未知错误'}`);
    return false;
  }

  if (!selectedDir) {
    message.info('已取消目录选择');
    return false;
  }

  setImportExtensionStatus(`已选择目录：${selectedDir}，正在重试解析`, 'processing');
  await waitForUiPaint();
  const result = await importer(selectedDir);
  return await importAccountsFromExtensionResult(result, 'extension_import_manual_dir');
};

const importFromExtension = async () => {
  if (isImportingExtension.value) return;

  const importer = window?.go?.main?.App?.ImportExtensionAccounts;
  if (typeof importer !== 'function') {
    message.error('当前仅 Wails 桌面端支持扩展导入');
    return;
  }

  markImportExtensionBusy();
  try {
    setImportExtensionStatus('正在扫描浏览器扩展数据库', 'processing');
    await waitForUiPaint();
    const result = await Promise.race([
      importer(),
      new Promise((_, reject) => {
        setTimeout(() => reject(new Error('扩展导入超时，请重试')), 15000);
      }),
    ]);
    await importAccountsFromExtensionResult(result, 'extension_import');
  } catch (err) {
    stopFetchKeysProgressPolling();
    setImportExtensionStatus(err?.message || '扩展导入失败', 'error');
    const handled = await offerManualExtensionImportFallback(err);
    if (!handled) {
      message.error(err?.message || '扩展导入失败');
    }
  } finally {
    resetImportExtensionState({ preserveStatus: true });
  }
};

const getBridgeImportSnapshot = async () => {
  const getter = window?.go?.main?.App?.GetBridgeImportSnapshot;
  if (typeof getter !== 'function') {
    throw new Error('当前环境不支持浏览器桥接导入');
  }
  return await getter();
};

const syncBridgeImportSnapshot = async () => {
  const snapshot = await getBridgeImportSnapshot();
  bridgeImportRecords.value = Array.isArray(snapshot?.records) ? snapshot.records : [];
  bridgeImportReadyCount.value = Number(snapshot?.readyCount || bridgeImportRecords.value.length || 0);
  bridgeImportLastReceivedAt.value = String(snapshot?.lastReceivedAt || '').trim();
  bridgeImportServerUrl.value = String(snapshot?.serverUrl || '').trim();
  bridgeImportLogPath.value = String(snapshot?.logPath || '').trim();
  bridgeImportLastLogs.value = Array.isArray(snapshot?.lastLogs) ? snapshot.lastLogs : [];
  bridgeImportSessionActive.value = snapshot?.sessionActive === true;
  bridgeImportClientReady.value = snapshot?.clientReady === true;
  bridgeImportLastClientPing.value = String(snapshot?.lastClientPing || '').trim();
  return snapshot;
};

const closeBridgeImportSession = async () => {
  const closer = window?.go?.main?.App?.CloseBridgeImportSession;
  if (typeof closer !== 'function') return;
  try {
    const snapshot = await closer();
    bridgeImportSessionActive.value = snapshot?.sessionActive === true;
    bridgeImportClientReady.value = snapshot?.clientReady === true;
    bridgeImportLastClientPing.value = String(snapshot?.lastClientPing || '').trim();
  } catch (error) {
    console.warn('[BridgeImport] close session failed:', error?.message || String(error));
  }
};

const stopBridgeImportPolling = () => {
  bridgeImportPolling.value = false;
  if (bridgeImportPollTimer) {
    clearInterval(bridgeImportPollTimer);
    bridgeImportPollTimer = null;
  }
};

const startBridgeImportPolling = async () => {
  stopBridgeImportPolling();
  bridgeImportPolling.value = true;
  await syncBridgeImportSnapshot();
  bridgeImportPollTimer = setInterval(() => {
    void syncBridgeImportSnapshot().catch(error => {
      console.warn('[BridgeImport] snapshot sync failed:', error?.message || String(error));
    });
  }, 1200);
};

const handleDirectTabImport = async () => {
  if (!isWailsRuntime) {
    message.warning('当前浏览器标签直接导入仅支持桌面端 EXE 环境');
    return;
  }

  const starter = window?.go?.main?.App?.StartBridgeImportSession;
  if (typeof starter !== 'function') {
    message.error('当前环境缺少浏览器桥接会话启动能力');
    return;
  }

  bridgeImportModalOpen.value = true;
  bridgeImportSessionOpening.value = true;
  bridgeImportInstallOpened.value = false;
  bridgeImportRecords.value = [];
  bridgeImportReadyCount.value = 0;
  bridgeImportLastReceivedAt.value = '';
  bridgeImportServerUrl.value = '';
  bridgeImportLogPath.value = '';
  bridgeImportLastLogs.value = [];
  bridgeImportSessionActive.value = false;
  bridgeImportClientReady.value = false;
  bridgeImportLastClientPing.value = '';

  try {
    await starter();
    await startBridgeImportPolling();
    console.info('[BridgeImport] session initialized');
  } catch (error) {
    message.error(error?.message || '浏览器桥接会话初始化失败');
  } finally {
    bridgeImportSessionOpening.value = false;
  }
};

const closeBridgeImportModal = () => {
  bridgeImportModalOpen.value = false;
  bridgeImportSessionOpening.value = false;
  stopBridgeImportPolling();
  void closeBridgeImportSession();
};

const openBridgeScriptInstallPage = async () => {
  const opener = window?.go?.main?.App?.OpenBridgeScriptInstallPage;
  if (typeof opener !== 'function') {
    message.error('当前环境缺少脚本发布页打开能力');
    return;
  }

  bridgeImportOpeningInstall.value = true;
  try {
    await opener();
    bridgeImportInstallOpened.value = true;
    message.success('已打开桥接脚本发布页');
    console.info('[BridgeImport] install page opened');
  } catch (error) {
    message.error(error?.message || '打开桥接脚本发布页失败');
  } finally {
    bridgeImportOpeningInstall.value = false;
  }
};

const looksLikeJwtToken = value => {
  const text = String(value || '').trim();
  if (!text) return false;
  const parts = text.split('.');
  return parts.length >= 3 && parts.every(part => /^[A-Za-z0-9_-]+$/.test(part));
};

const normalizeBridgeImportedTokens = (tokens) => {
  const dedupe = new Map();
  (Array.isArray(tokens) ? tokens : []).forEach((token, index) => {
    const key = String(
      token?.key ||
      token?.access_token ||
      token?.token ||
      token?.api_key ||
      token?.apikey ||
      (typeof token === 'string' ? token : '')
    ).trim();
    if (!key) return;
    dedupe.set(key, {
      ...token,
      key,
      access_token: key,
      name: String(token?.name || token?.token_name || `Bridge Token ${index + 1}`).trim() || `Bridge Token ${index + 1}`,
      status: token?.status ?? 1,
      source: String(token?.source || 'bridge').trim() || 'bridge',
    });
  });
  return Array.from(dedupe.values());
};

const inferBridgeImportedSiteType = ({ siteType, siteUrl, endpoint, accessToken }) => {
  const explicit = String(siteType || '').trim().toLowerCase();
  if (explicit) return explicit;
  const endpointText = String(endpoint || '').trim();
  if (endpointText.startsWith('/api/v1/keys')) return 'sub2api';
  try {
    const host = String(new URL(siteUrl).hostname || '').toLowerCase();
    if (host === 'anyrouter.top' || host.endsWith('.anyrouter.top')) return 'anyrouter';
  } catch {}
  if (looksLikeJwtToken(accessToken)) return 'sub2api';
  return '';
};

const buildBridgeImportedPreparedPayload = (records) => {
  const prefetchedSites = [];
  const accounts = [];
  const skipped = [];
  const blockedReasons = {
    token_expired: '登录态已过期，请重新登录站点后再试',
    token_expired_local: '登录态已过期，请重新登录站点后再试',
    not_logged_in: '当前页面未登录，请先登录站点后再试',
    weak_access_token: '未捕获到可复用的真实登录态，请在站点主界面重新触发',
  };

  (Array.isArray(records) ? records : []).forEach((record, index) => {
    const payload = record?.payload && typeof record.payload === 'object' ? record.payload : {};
    const extracted = payload?.extracted && typeof payload.extracted === 'object' ? payload.extracted : payload;
    const readyReason = String(record?.readyReason || '').trim();
    const extractedError = String(extracted?.error || '').trim();
    const sourceUrl = normalizeSiteUrl(
      extracted?.site_url ||
      extracted?.source_origin ||
      payload?.source_origin ||
      record?.sourceOrigin ||
      record?.sourceUrl ||
      ''
    );
    if (!sourceUrl) {
      skipped.push({
        title: String(record?.title || `桥接记录 ${index + 1}`).trim() || `桥接记录 ${index + 1}`,
        reason: '缺少站点地址',
      });
      return;
    }

    let hostname = '';
    try {
      hostname = new URL(sourceUrl).hostname;
    } catch {}

    const accountInfo = extracted?.account_info && typeof extracted.account_info === 'object'
      ? extracted.account_info
      : {};
    const accessToken = String(
      extracted?.resolved_access_token ||
      extracted?.access_token ||
      accountInfo?.access_token ||
      ''
    ).trim();
    const userId = String(
      extracted?.resolved_user_id ||
      extracted?.user_id ||
      accountInfo?.id ||
      ''
    ).trim();
    const endpoint = String(extracted?.endpoint || payload?.endpoint || '').trim();
    const siteType = inferBridgeImportedSiteType({
      siteType: extracted?.site_type || payload?.site_type,
      siteUrl: sourceUrl,
      endpoint,
      accessToken,
    });
    const siteName = String(
      extracted?.site_name ||
      record?.title ||
      hostname ||
      `桥接站点 ${index + 1}`
    ).trim() || `桥接站点 ${index + 1}`;

    const blockedReason = blockedReasons[readyReason] || blockedReasons[extractedError] || '';
    if (blockedReason) {
      skipped.push({
        title: siteName,
        reason: blockedReason,
      });
      return;
    }

    const prefetchedTokens = normalizeBridgeImportedTokens(extracted?.tokens || payload?.tokens);
    const storageFields = Array.isArray(extracted?.storage_fields)
      ? extracted.storage_fields
      : Array.isArray(payload?.storage_fields)
        ? payload.storage_fields
        : [];
    const storageOrigin = String(
      extracted?.storage_origin ||
      payload?.storage_origin ||
      sourceUrl
    ).trim();
    const baseSite = {
      site_name: siteName,
      site_url: sourceUrl,
      site_type: siteType,
      api_key: normalizeSiteUrl(extracted?.api_base_url || payload?.api_base_url || sourceUrl),
      account_info: {
        ...(accountInfo || {}),
        ...(userId ? { id: userId } : {}),
        ...(accessToken ? { access_token: accessToken } : {}),
      },
      resolved_access_token: accessToken,
      resolved_user_id: userId,
      tokens: prefetchedTokens,
      endpoint,
      error: String(extracted?.error || '').trim(),
      _profileStorageFields: storageFields,
      _profileStorageOrigin: storageOrigin,
    };

    if (prefetchedTokens.length > 0) {
      prefetchedSites.push(baseSite);
      return;
    }

    if (accessToken) {
      accounts.push({
        id: String(userId || sourceUrl || `bridge-account-${index + 1}`).trim(),
        site_name: siteName,
        site_url: sourceUrl,
        site_type: siteType,
        api_key: normalizeSiteUrl(extracted?.api_base_url || payload?.api_base_url || sourceUrl),
        account_info: {
          ...(accountInfo || {}),
          ...(userId ? { id: userId } : {}),
          access_token: accessToken,
        },
      });
      return;
    }

    skipped.push({
      title: siteName,
      reason: '未提取到 access_token 且未预取到账号内 key',
    });
  });

  return {
    prefetchedSites,
    accounts,
    skipped,
  };
};

const collectSiteCacheModelsByToken = (nodes, bucket = new Map()) => {
  (Array.isArray(nodes) ? nodes : []).forEach(node => {
    const key = String(node?.key || '').trim();
    if (key.startsWith('token|')) {
      const parts = key.split('|');
      const tokenKey = String(parts[2] || '').trim();
      if (tokenKey) {
        const models = (Array.isArray(node?.children) ? node.children : [])
          .map(child => {
            const childKey = String(child?.key || '').trim();
            const childTitle = String(child?.title || '').trim();
            if (childTitle) return childTitle;
            if (!childKey.includes('|')) return '';
            return String(childKey.split('|').slice(2).join('|') || '').trim();
          })
          .filter(Boolean);
        if (models.length > 0) {
          bucket.set(tokenKey, normalizeKeyPanelModels([
            ...(bucket.get(tokenKey) || []),
            ...models,
          ]));
        }
      }
    }
    if (Array.isArray(node?.children) && node.children.length > 0) {
      collectSiteCacheModelsByToken(node.children, bucket);
    }
  });
  return bucket;
};

const syncBridgeSitesToKeyPanel = (sites) => {
  const targetSites = Array.isArray(sites) ? sites : [];
  if (targetSites.length === 0) return 0;

  const { records: existingRecords } = loadPanelRecords();
  const mergedRecords = new Map(
    existingRecords.map(record => [
      String(record?.rowKey || buildKeyPanelRowKey(record?.siteUrl, record?.apiKey)).trim(),
      { ...record },
    ])
  );

  const now = Date.now();
  let importedCount = 0;

  targetSites.forEach((site, siteIndex) => {
    const siteUrl = normalizeSiteUrl(site?.siteUrl || site?.site_url);
    if (!siteUrl) return;

    const siteName = String(site?.siteName || site?.site_name || `桥接站点 ${siteIndex + 1}`).trim() || `桥接站点 ${siteIndex + 1}`;
    const modelsByToken = collectSiteCacheModelsByToken(site?.cachedTreeNodes || site?._cachedTreeNodes);
    const tokenMap = new Map();

    [...(Array.isArray(site?.tokens) ? site.tokens : []), ...(Array.isArray(site?.customTokens) ? site.customTokens : [])]
      .forEach((token, tokenIndex) => {
        const apiKey = String(token?.key || token?.access_token || token?.token || '').trim();
        if (!apiKey) return;
        tokenMap.set(apiKey, {
          ...token,
          apiKey,
          tokenName: String(token?.name || `Bridge Token ${tokenIndex + 1}`).trim() || `Bridge Token ${tokenIndex + 1}`,
        });
      });

    tokenMap.forEach(token => {
      const rowKey = buildKeyPanelRowKey(siteUrl, token.apiKey);
      const existing = mergedRecords.get(rowKey) || null;
      const modelsList = normalizeKeyPanelModels([
        ...(Array.isArray(existing?.modelsList) ? existing.modelsList : []),
        ...(Array.isArray(token?.models) ? token.models : []),
        ...(modelsByToken.get(token.apiKey) || []),
        token?.model || '',
        existing?.selectedModel || '',
      ]);
      const statusValue = Number(token?.status ?? existing?.status ?? 1);
      const nextRecord = {
        ...existing,
        rowKey,
        sourceType: 'auto',
        siteName,
        tokenName: token.tokenName || existing?.tokenName || '',
        siteUrl,
        apiKey: token.apiKey,
        modelsList,
        modelsText: modelsList.join(', ') || existing?.modelsText || '未提供模型信息',
        selectedModel: (
          (existing?.selectedModel && modelsList.includes(String(existing.selectedModel).trim()) && String(existing.selectedModel).trim())
          || (String(token?.selectedModel || '').trim() && modelsList.includes(String(token.selectedModel).trim()) && String(token.selectedModel).trim())
          || modelsList[0]
          || ''
        ),
        status: statusValue === 2 ? 2 : 1,
        createdAt: existing?.createdAt || now,
        updatedAt: now,
        quickTestStatus: existing?.quickTestStatus || '',
        quickTestLabel: existing?.quickTestLabel || '',
        quickTestModel: existing?.quickTestModel || '',
        quickTestRemark: existing?.quickTestRemark || '',
        quickTestAt: existing?.quickTestAt || null,
        quickTestResponseTime: existing?.quickTestResponseTime || '',
        quickTestTtftMs: existing?.quickTestTtftMs || '',
        quickTestTps: existing?.quickTestTps || '',
        quickTestResponseContent: existing?.quickTestResponseContent || '',
        balanceLabel: existing?.balanceLabel || '',
        balanceUpdatedAt: existing?.balanceUpdatedAt || null,
        balanceError: existing?.balanceError || '',
        remainQuota: token?.remain_quota ?? existing?.remainQuota ?? null,
        usedQuota: token?.used_quota ?? existing?.usedQuota ?? null,
        unlimitedQuota: token?.unlimited_quota === true || existing?.unlimitedQuota === true,
      };
      mergedRecords.set(rowKey, nextRecord);
      importedCount += 1;
    });
  });

  if (importedCount > 0) {
    persistPanelRecords(Array.from(mergedRecords.values()));
  }

  return importedCount;
};

const finalizeBridgeImportSession = async () => {
  const prepared = buildBridgeImportedPreparedPayload(bridgeImportRecords.value);
  const importableCount = prepared.prefetchedSites.length + prepared.accounts.length;
  if (importableCount === 0) {
    message.warning('当前没有可导入的桥接记录');
    return;
  }

  bridgeImportImporting.value = true;
  try {
    const processedSites = [];
    if (prepared.accounts.length > 0) {
      const extractedAccountSites = await processAccountsV2(prepared.accounts, {
        importSource: 'browser_tag_bridge',
        forceExtractionMode: 'browser_direct',
      });
      if (Array.isArray(extractedAccountSites) && extractedAccountSites.length > 0) {
        processedSites.push(...extractedAccountSites);
      }
    }
    if (prepared.prefetchedSites.length > 0) {
      const extractedPrefetchedSites = await processAccountsV2([], {
        prefetchedSites: prepared.prefetchedSites,
        importSource: 'browser_tag_bridge_prefetched',
      });
      if (Array.isArray(extractedPrefetchedSites) && extractedPrefetchedSites.length > 0) {
        processedSites.push(...extractedPrefetchedSites);
      }
    }
    const processedSiteCacheKeys = new Set(
      processedSites
        .map(site => String(site?._siteCacheKey || site?.siteCacheKey || buildSiteCacheKey(site)).trim())
        .filter(Boolean)
    );
    const bridgeSiteSignatures = [...prepared.accounts, ...prepared.prefetchedSites]
      .map(site => ({
        siteCacheKey: String(site?._siteCacheKey || site?.siteCacheKey || buildSiteCacheKey(site) || '').trim(),
        siteUrl: normalizeSiteUrl(site?.site_url || site?.siteUrl),
        userId: String(
          site?.resolved_user_id ||
          site?.resolvedUserId ||
          site?.account_info?.id ||
          site?.accountInfo?.id ||
          ''
        ).trim(),
      }))
      .filter(signature => signature.siteCacheKey || signature.siteUrl);
    const bridgeSiteCacheKeys = new Set(
      [
        ...Array.from(processedSiteCacheKeys),
        ...bridgeSiteSignatures
        .map(signature => signature.siteCacheKey)
        .filter(Boolean)
      ]
    );
    const importedSiteRecords = loadAllSiteCacheRecords().filter(record => {
        const recordSiteCacheKey = String(record?.siteCacheKey || '').trim();
        if (recordSiteCacheKey && bridgeSiteCacheKeys.has(recordSiteCacheKey)) {
          return true;
        }
        const recordSiteUrl = normalizeSiteUrl(record?.siteUrl || record?.site_url);
        const recordUserId = String(record?.resolvedUserId || record?.resolved_user_id || record?.accountInfo?.id || record?.account_info?.id || '').trim();
        return bridgeSiteSignatures.some(signature =>
          signature.siteUrl &&
          signature.siteUrl === recordSiteUrl &&
          (!signature.userId || !recordUserId || signature.userId === recordUserId)
        );
      });
    const syncedKeyCount = syncBridgeSitesToKeyPanel(importedSiteRecords);
    const importedSiteCacheKeys = Array.from(new Set([
      ...Array.from(processedSiteCacheKeys),
      ...importedSiteRecords
        .map(record => String(record?.siteCacheKey || '').trim())
        .filter(Boolean)
    ]));
    bridgeImportSessionOpening.value = false;
    bridgeImportModalOpen.value = false;
    stopBridgeImportPolling();
    await closeBridgeImportSession();
    if (prepared.skipped.length > 0) {
      const preview = prepared.skipped
        .slice(0, 3)
        .map(item => `${item.title}(${item.reason})`)
        .join('，');
      message.warning(`有 ${prepared.skipped.length} 条桥接记录未导入：${preview}${prepared.skipped.length > 3 ? '…' : ''}`, 6);
    }
    message.success(`已导入 ${importableCount} 条桥接记录到站点管理`);
    console.info('[BridgeImport] finalize success', {
      importedCount: importableCount,
      prefetchedSiteCount: prepared.prefetchedSites.length,
      accountCount: prepared.accounts.length,
      processedSiteCount: processedSites.length,
      syncedKeyCount,
      importedSiteCacheKeyCount: importedSiteCacheKeys.length,
      skippedCount: prepared.skipped.length,
      serverUrl: bridgeImportServerUrl.value,
      logPath: bridgeImportLogPath.value,
    });
    writePendingSiteRestore(importedSiteCacheKeys);
    writePendingBatchStart({
      autoStart: false,
    });
    await router.push('/');
    message.info(`已切换到本次导入列表，共 ${importedSiteCacheKeys.length} 个站点`);
  } catch (error) {
    message.error(error?.message || '桥接记录导入失败');
  } finally {
    bridgeImportImporting.value = false;
  }
};

const fetchTokensForAccountsViaServer = async (accounts) => {
  const response = await apiFetch('/api/fetch-keys', {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ accounts }),
  });
  if (!response.ok) {
    const text = await response.text().catch(() => '');
    throw new Error(text || `server fetch failed (${response.status})`);
  }
  const data = await response.json().catch(() => ({}));
  return Array.isArray(data?.results) ? data.results : [];
};

const resetFetchKeysProgress = () => {
  fetchKeysProgress.active = false;
  fetchKeysProgress.stage = '';
  fetchKeysProgress.detail = '';
  fetchKeysProgress.total = 0;
  fetchKeysProgress.completed = 0;
  fetchKeysProgress.successSites = 0;
  fetchKeysProgress.lastSiteName = '';
  fetchKeysProgress.startedAt = 0;
  fetchKeysProgress.lastUpdatedAt = 0;
};

const stopFetchKeysProgressPolling = () => {
  if (fetchKeysProgressTimer) {
    clearInterval(fetchKeysProgressTimer);
    fetchKeysProgressTimer = null;
  }
};

const syncFetchKeysProgress = async () => {
  try {
    let snapshot = null;
    if (isWailsRuntime && desktopTokenSourceMode.value === 'profile_file') {
      snapshot = await getDesktopProfileProgressDirect();
    }
    if (!snapshot) {
      const response = await apiFetch('/api/fetch-keys/progress');
      if (!response.ok) return;
      snapshot = await response.json().catch(() => null);
    }
    if (!snapshot || typeof snapshot !== 'object') return;
    fetchKeysProgress.active = Boolean(snapshot.active);
    fetchKeysProgress.stage = String(snapshot.stage || '');
    fetchKeysProgress.detail = String(snapshot.detail || '');
    fetchKeysProgress.total = Number(snapshot.total || 0);
    fetchKeysProgress.completed = Number(snapshot.completed || 0);
    fetchKeysProgress.successSites = Number(snapshot.successSites || 0);
    fetchKeysProgress.lastSiteName = String(snapshot.lastSiteName || '');
    fetchKeysProgress.startedAt = Number(snapshot.startedAt || 0);
    fetchKeysProgress.lastUpdatedAt = Number(snapshot.lastUpdatedAt || 0);
  } catch {}
};

const startFetchKeysProgressPolling = () => {
  stopFetchKeysProgressPolling();
  void syncFetchKeysProgress();
  fetchKeysProgressTimer = setInterval(() => {
    void syncFetchKeysProgress();
  }, 700);
};

const updateBrowserSessionPendingSites = (sites) => {
  browserSessionPendingSiteNames.value = (Array.isArray(sites) ? sites : [])
    .map(site => String(site?.site_name || '').trim())
    .filter(Boolean);
};

const processAccounts = async (accounts) => {
  const accountsToFetch = accounts.filter(acc => 
    !acc.disabled && 
    acc.site_url && 
    acc.account_info && 
    acc.account_info.access_token
  );
  
  if (accountsToFetch.length === 0) {
    message.warning('备份文件中未找到可用账号配置！');
    return;
  }
  
  // ── 第 0 步：清空后端日志 ──
  try {
    await apiFetch('/api/clear-logs?type=fetch', { method: 'POST' });
    await apiFetch('/api/clear-logs?type=check', { method: 'POST' });
  } catch (e) {
    console.warn('Clear logs fail, ignoring...', e);
  }

  totalAccountsCount.value = accountsToFetch.length;
  isLoadingModels.value = true;
  step.value = -1; // 显示提取中的中间状态
  loadedSitesCount.value = 0;
  browserSessionPolling.active = false;
  browserSessionPolling.round = 0;
  browserSessionPolling.totalRounds = 0;
  browserSessionPolling.pending = 0;
  browserSessionPendingSiteNames.value = [];
  
  // ── 第 1 步：先用浏览器端直接并发提取（绕过Cloudflare WAF服务端拦截）──
  let extractedSites = [];
  try {
    const BROWSER_FETCH_CONCURRENCY = 25;
    const browserResults = new Array(accountsToFetch.length);
    let currentIdx = 0;

    const browserFetchWorker = async () => {
      while (currentIdx < accountsToFetch.length) {
        const idx = currentIdx++;
        browserResults[idx] = await fetchTokensForAccountFromBrowserV2(accountsToFetch[idx]);
      }
    };

    const browserWorkers = Array.from(
      { length: Math.min(BROWSER_FETCH_CONCURRENCY, accountsToFetch.length) },
      () => browserFetchWorker()
    );
    await Promise.all(browserWorkers);

    // 将浏览器端成功的结果同步
    extractedSites = browserResults;

    // 对于浏览器端提取失败的（_needServerFallback=true），Fallback到服务端代理
    const failedAccounts = accountsToFetch.filter((acc, i) => 
      browserResults[i]?._needServerFallback === true
    );

    if (failedAccounts.length > 0) {
      console.log(`[FetchKeys] 浏览器端失败 ${failedAccounts.length} 个，尝试服务端代理墙跑...`);
      try {
        const serverResponse = await apiFetch('/api/fetch-keys', {
          method: 'POST',
          headers: { 'Content-Type': 'application/json' },
          body: JSON.stringify({ accounts: failedAccounts }),
        });
        if (serverResponse.ok) {
          const serverData = await serverResponse.json();
          const serverResults = serverData.results || [];
          // 将服务端成功的结果写回 extractedSites
          serverResults.forEach(serverResult => {
            const idx = accountsToFetch.findIndex(a => a.id === serverResult.id);
            if (idx !== -1) {
              // 强制将服务端获取到的结果覆盖浏览器的初始错误态，不管服务端有没有取到token
              extractedSites[idx] = serverResult;
            }
          });
        }
      } catch (e) {
        console.warn('[FetchKeys] 服务端墙跑失败:', e.message);
      }
    }

    let stillFailedAccounts = extractedSites.filter(site =>
      !site || site.error || !site.tokens || site.tokens.length === 0
    );
    updateBrowserSessionPendingSites(stillFailedAccounts);

    // 先展示当前可得结果，不阻塞后续流程
    validAccounts.value = extractedSites;
    preloadAllQuotas(extractedSites);
    const nowSuccessSites = extractedSites.filter(site => site && !site.error && Array.isArray(site.tokens) && site.tokens.length > 0).length;
    const nowTokenCount = extractedSites.reduce((sum, site) => sum + (Array.isArray(site?.tokens) ? site.tokens.length : 0), 0);
    console.log(`[FetchKeys] 当前阶段完成: successSites=${nowSuccessSites}/${extractedSites.length}, totalTokens=${nowTokenCount}, pendingSites=${stillFailedAccounts.length}`);

    if (stillFailedAccounts.length > 0) {
      void (async () => {
        try {
          const detected = await getDetectedFallbackBrowser();
          const browserType = await chooseDetectedFallbackBrowserType(detected);
          if (!browserType) {
            message.warning('你取消了浏览器选择，当前保留已有结果并继续后续流程。');
            return;
          }

          const readyForShadow = await confirmShadowLoginReadiness(stillFailedAccounts, browserType);
          if (!readyForShadow) {
            message.warning('你取消了 shadow 模式抓取，请先在浏览器中完成失效站点登录后再重试。');
            return;
          }

          let openedCount = 0;
          try {
            openedCount = await openSitesInBrowserSession(stillFailedAccounts, browserType);
          } catch (openErr) {
            const fallbackStatus = await getFallbackBrowserStatus(browserType).catch(() => ({
              running: false,
              attached: false,
              launching: false,
              managed: false,
              browserType,
            }));
            const shouldHandleAsProfileInUse =
              openErr?.code === 'BROWSER_PROFILE_IN_USE' ||
              (fallbackStatus.running && !fallbackStatus.attached && !fallbackStatus.launching && !fallbackStatus.managed);

            if (shouldHandleAsProfileInUse) {
              const shouldKill = await confirmWithModal({
                title: '浏览器已占用',
                content: `${browserType === 'edge' ? 'Edge' : 'Chrome'} 当前已在普通模式运行，默认 profile 被占用。结束后会关闭该浏览器的所有窗口。是否结束并立即以受控模式重新打开目标站点？`,
                okText: '结束并继续',
                cancelText: '取消',
                okType: 'danger',
              });
              if (!shouldKill) {
                message.warning('你取消了结束浏览器进程，当前保留已有结果并继续后续流程。');
                return;
              }

              const restartResult = await restartBrowserSessionProcessAndOpen(stillFailedAccounts, browserType);
              if (!restartResult?.stopped) {
                message.error(`${browserType === 'edge' ? 'Edge' : 'Chrome'} 进程结束后仍未完全退出，请手动关闭后再重试。`);
                return;
              }
              openedCount = Number(restartResult?.opened || stillFailedAccounts.length);
            } else {
              throw openErr;
            }
          }

          if (openedCount <= 0) return;

          const availableText = detected.availableTypes.map(type => type === 'edge' ? 'Edge' : 'Chrome').join(' / ');
          message.info(`已智能探测到 ${availableText}，当前使用 ${browserType === 'edge' ? 'Edge' : 'Chrome'} 打开 ${openedCount} 个失败站点，后台自动轮询抓取中。`, 6);

          const maxRetryRounds = 3;
          const retryIntervalMs = 15000;
          browserSessionPolling.active = true;
          browserSessionPolling.totalRounds = maxRetryRounds;
          browserSessionPolling.pending = stillFailedAccounts.length;
          updateBrowserSessionPendingSites(stillFailedAccounts);

          try {
            for (let round = 1; round <= maxRetryRounds && stillFailedAccounts.length > 0; round += 1) {
              browserSessionPolling.round = round;
              browserSessionPolling.pending = stillFailedAccounts.length;
              updateBrowserSessionPendingSites(stillFailedAccounts);
              console.log(`[FetchKeys] 受控浏览器(${browserType})自动抓取，第 ${round}/${maxRetryRounds} 轮，当前失败站点 ${stillFailedAccounts.length} 个`);

              const browserSessionResults = await browserSessionFetchForAccounts(stillFailedAccounts, browserType, round, maxRetryRounds);
              extractedSites = mergeExtractedSiteResults(extractedSites, browserSessionResults);
              validAccounts.value = extractedSites;
              preloadAllQuotas(extractedSites);

              stillFailedAccounts = extractedSites.filter(site =>
                !site || site.error || !site.tokens || site.tokens.length === 0
              );
              browserSessionPolling.pending = stillFailedAccounts.length;
              updateBrowserSessionPendingSites(stillFailedAccounts);
              const roundSuccessSites = extractedSites.filter(site => site && !site.error && Array.isArray(site.tokens) && site.tokens.length > 0).length;
              const roundTokenCount = extractedSites.reduce((sum, site) => sum + (Array.isArray(site?.tokens) ? site.tokens.length : 0), 0);
              console.log(`[FetchKeys] 受控浏览器(${browserType})第 ${round}/${maxRetryRounds} 轮结束: successSites=${roundSuccessSites}/${extractedSites.length}, totalTokens=${roundTokenCount}, pendingSites=${stillFailedAccounts.length}`);

              if (stillFailedAccounts.length === 0) break;
              if (round < maxRetryRounds) {
                await sleep(retryIntervalMs);
              }
            }
          } finally {
            browserSessionPolling.active = false;
            browserSessionPolling.round = 0;
            browserSessionPolling.totalRounds = 0;
            browserSessionPolling.pending = 0;
            browserSessionPendingSiteNames.value = [];
          }

          if (stillFailedAccounts.length > 0) {
            message.warning(`受控浏览器自动轮询完成，仍有 ${stillFailedAccounts.length} 个站点未抓取成功。`);
          }
        } catch (e) {
          console.warn('[FetchKeys] 受控浏览器兜底失败:', e.message);
          message.warning(`失败站点受控浏览器兜底未执行成功: ${e.message}`);
        }
      })();
    }
  } catch (err) {
    message.error(`批量提取 Token 失败: ${err.message}`);
    isLoadingModels.value = false;
    step.value = 1;
    return;
  }

    const discoverySites = [...extractedSites];
    const siteNodes = new Array(discoverySites.length);
    const fullCheckedKeys = [];
    const fullAllKeys = [];

    // ── 第 2 步：探测模型 (采用分流多进程) ──
    const discoveryLimit = 25; 
    let currentIndex = 0;

    const discoverWorker = async () => {
      while (currentIndex < discoverySites.length) {
        const globalIdx = currentIndex++;
        const site = discoverySites[globalIdx];
        
        const siteIdx = globalIdx + 1;
        const siteDisplayTitle = `${siteIdx}. [${site.site_name}]`;
        const currentSiteNodes = [];

        // ── 情况 A: 令牌提取报错 ──
        if (!site || site.error || !site.tokens || site.tokens.length === 0) {
          const errorMsg = formatUserFacingErrorText(site?.error || '获取令牌失败');
          currentSiteNodes.push({
            title: `${siteDisplayTitle} - ❌ ${errorMsg}`,
            key: `fail-site|${site.id || globalIdx}`,
            disabled: true,
            checkable: false,
            selectable: false,
            class: 'site-root-summary-node',
            children: []
          });
          siteNodes[globalIdx] = currentSiteNodes;
          loadedSitesCount.value++;
          continue;
        }

        // 探测模型
        let effectiveBaseUrl = site.site_url.replace(/\/+$/, '');
        const rawApiKey = String(site.api_key || '').trim();
        if (rawApiKey.startsWith('http')) effectiveBaseUrl = rawApiKey.replace(/\/+$/, '');
        
        const baseUrl = effectiveBaseUrl;
        const firstToken = site.tokens[0];
        const testApiKey = firstToken.key || firstToken.access_token;
        
        let supportedModels = [];
        const endpointsToTry = [
          { url: `${baseUrl}/v1/models`, type: 'openai' },
          { url: `${baseUrl}/api/models`, type: 'newapi_public' },
          { url: `${baseUrl}/api/user/models`, type: 'newapi_user' }
        ];

        for (const ep of endpointsToTry) {
          try {
            const rawDiscoveryId = site?.account_info?.id || site?.id || site?.uid || site?.user_id || '';
            const discoveryUid = /^\d+$/.test(String(rawDiscoveryId)) ? String(rawDiscoveryId) : '';
            const res = await apiFetch(`/api/proxy-get?url=${encodeURIComponent(ep.url)}&uid=${discoveryUid}`, {
              headers: { Authorization: `Bearer ${testApiKey}` }
            });
            if (res.ok) {
              const result = await res.json();
              let rawData = Array.isArray(result) ? result : (result.data?.data || result.data?.items || result.data || []);
              if (rawData.length > 0) {
                supportedModels = rawData.map(m => (typeof m === 'string' ? m : (m.id || m.name || m))).filter(m => typeof m === 'string').sort();
                if (supportedModels.length > 0) break;
              }
            }
          } catch (e) {}
        }

        // ── 情况 B: 探测不到模型 ──
        if (supportedModels.length === 0) {
          console.log(`[FetchKeys] 模型发现失败: [${site.site_name}] tokenCount=${site.tokens?.length || 0}, firstToken=${String(testApiKey || '').slice(0, 12)}...`);
          currentSiteNodes.push({
            title: `${siteDisplayTitle} - ⚠️ 未能探测到可用模型列表`,
            key: `no-model-site|${site.id}`,
            disabled: true,
            checkable: false,
            selectable: false,
            class: 'site-root-summary-node',
            children: []
          });
        } else {
          // ── 情况 C: 正常 ──
          site.tokens.forEach((token, idx) => {
            const tKey = token.key || token.access_token;
            const tName = token.name || `Token ${idx + 1}`;
            const tokenNodeKey = `token|${site.id}|${tKey}`;
            const children = supportedModels.map(model => {
              const itemKey = `${site.id}|${tKey}|${model}`;
              fullAllKeys.push(itemKey);
              fullCheckedKeys.push(itemKey);
              return { title: model, key: itemKey, isLeaf: true };
            });
            fullAllKeys.push(tokenNodeKey);
            fullCheckedKeys.push(tokenNodeKey);
            currentSiteNodes.push({
              title: `${siteDisplayTitle} ${tName} (${tKey.slice(0, 15)}...)`,
              key: tokenNodeKey,
              children: children,
            });
          });
        }

        siteNodes[globalIdx] = currentSiteNodes;
        loadedSitesCount.value++;
      }
    };

    const discoveryWorkers = Array.from({ length: Math.min(discoveryLimit, discoverySites.length) }, () => discoverWorker());
    await Promise.all(discoveryWorkers);

    const discoveredSiteCount = discoverySites.filter(site => site && !site.error && Array.isArray(site.tokens) && site.tokens.length > 0).length;
    const failedSiteCount = discoverySites.length - discoveredSiteCount;
    const selectableModelCount = fullAllKeys.filter(key => key.includes('|')).length;
    console.log(`[FetchKeys] 模型发现阶段完成: tokenSites=${discoveredSiteCount}, failedSites=${failedSiteCount}, selectableModels=${selectableModelCount}`);

    treeData.value = siteNodes.flat().filter(Boolean);
    allKeys.value = fullAllKeys;
    checkedKeys.value = fullCheckedKeys;
    isLoadingModels.value = false;
    step.value = 2; // 进入树形选择器
  };
  
const processAccountsV2 = async (accounts, options = {}) => {
  const prefetchedSites = Array.isArray(options?.prefetchedSites) ? options.prefetchedSites : null;
  const accountsToFetch = prefetchedSites
    ? prefetchedSites.filter(site => !site?.disabled && site?.site_url)
    : (Array.isArray(accounts) ? accounts : []).filter(acc =>
      !acc?.disabled &&
      acc?.site_url &&
      acc?.account_info &&
      acc?.account_info?.access_token
    );
  const importSource = String(options?.importSource || '').trim();
  const forcedExtractionMode = String(options?.forceExtractionMode || '').trim();
  const fallbackOnProfileFailure = options?.fallbackOnProfileFailure === true;

  if (accountsToFetch.length === 0) {
    message.warning(prefetchedSites ? '站点缓存中没有可恢复的站点' : '备份文件中未找到可用账号配置');
    return [];
  }

  totalAccountsCount.value = accountsToFetch.length;
  loadedSitesCount.value = 0;
  isLoadingModels.value = true;
  isDiscoveringModels.value = false;
  step.value = -1;
  treeData.value = [];
  checkedKeys.value = [];
  allKeys.value = [];
  selectionExpandedKeys.value = [];

  browserSessionPolling.active = false;
  browserSessionPolling.round = 0;
  browserSessionPolling.totalRounds = 0;
  browserSessionPolling.pending = 0;
  browserSessionPendingSiteNames.value = [];
  resetFetchKeysProgress();
  if (isWailsRuntime) {
    fetchKeysProgress.total = accountsToFetch.length;
  }

  try {
    await Promise.allSettled([
      apiFetch('/api/clear-logs?type=fetch', { method: 'POST' }),
      apiFetch('/api/clear-logs?type=check', { method: 'POST' }),
    ]);
  } catch (e) {
    console.warn('Clear logs fail, ignoring...', e);
  }

  const isSiteFailed = (site) => !site || site.error || !Array.isArray(site.tokens) || site.tokens.length === 0;
  const getPendingHint = () => `后台检测中（第 ${Math.max(browserSessionPolling.round, 1)}/${Math.max(browserSessionPolling.totalRounds, 1)} 轮）`;
  const withPendingMeta = (siteName, node) => {
    const normalizedSiteName = String(siteName || '').trim();
    const pending = browserSessionPolling.active && browserSessionPendingSiteNameSet.value.has(normalizedSiteName);
    return {
      ...node,
      siteName: normalizedSiteName,
      isBrowserPending: pending,
      pendingHint: pending ? getPendingHint() : '',
    };
  };
  const summarizeStage = (tag, sites, pendingSites = []) => {
    const safeSites = Array.isArray(sites) ? sites : [];
    const successSites = safeSites.filter(site => !isSiteFailed(site)).length;
    const totalTokens = safeSites.reduce((sum, site) => sum + (Array.isArray(site?.tokens) ? site.tokens.length : 0), 0);
    const usableTokens = safeSites.reduce((sum, site) => sum + countUsableTokensForSite(site), 0);
    const unresolvedTokens = Math.max(0, totalTokens - usableTokens);
    const pendingCount = Array.isArray(pendingSites) ? pendingSites.length : 0;
    console.log(
      `[FetchKeys] ${tag}: successSites=${successSites}/${safeSites.length}, totalTokens=${totalTokens}, usableTokens=${usableTokens}, unresolvedTokens=${unresolvedTokens}, pendingSites=${pendingCount}`
    );
  };
  const refreshTreePendingHints = () => {
    if (!Array.isArray(treeData.value) || treeData.value.length === 0) return;
    treeData.value = treeData.value.map(node => withPendingMeta(node?.siteName || '', node));
  };
  const updateActiveSiteSession = (replaceSites, requestDiscoveryRefreshFn) => {
    activeSiteTreeSession.replaceSites = replaceSites;
    activeSiteTreeSession.requestDiscoveryRefresh = requestDiscoveryRefreshFn;
    activeSiteTreeSession.syncCacheSnapshot = syncSiteCacheSnapshot;
  };
  const normalizeErrorCodeForDisplay = (rawError) => {
    const text = String(rawError || '').trim();
    if (!text) return 'UNKNOWN';
    if (text === 'token_expired_local') return 'TOKEN_EXPIRED_LOCAL';
    if (text === 'token_expired') return 'TOKEN_EXPIRED';
    if (text === 'user_banned' || /封禁|banned/i.test(text)) return 'USER_BANNED';
    if (/^http_\d+$/i.test(text)) return text.toUpperCase();
    if (/^business_code_/i.test(text)) return text.toUpperCase();
    if (/^exception_/i.test(text)) return 'EXCEPTION';
    if (text === 'profile_storage_not_found') return 'PROFILE_STORAGE_NOT_FOUND';
    if (text === 'profile_token_not_found') return 'PROFILE_TOKEN_NOT_FOUND';
    if (text === 'profile_fetch_no_tokens') return 'PROFILE_FETCH_NO_TOKENS';
    if (text === 'token_list_empty' || text === 'empty_models') return 'EMPTY_RESULT';
    if (text === 'html_response') return 'HTML_RESPONSE';
    if (text === 'json_decode_failed') return 'JSON_DECODE_FAILED';
    if (/tls:\s*handshake\s*failure/i.test(text)) return 'TLS_HANDSHAKE_FAILURE';
    if (/no such host/i.test(text)) return 'DNS_NOT_FOUND';
    if (/context deadline exceeded|Client\.Timeout exceeded|This operation was aborted/i.test(text)) return 'TIMEOUT';
    if (/ECONNREFUSED|actively refused/i.test(text)) return 'CONNECTION_REFUSED';
    if (/insufficient account balance/i.test(text)) return 'INSUFFICIENT_BALANCE';
    if (/service temporarily unavailable/i.test(text)) return 'SERVICE_UNAVAILABLE';
    if (/system disk overloaded/i.test(text)) return 'SYSTEM_OVERLOADED';
    if (/bot|cloudflare/i.test(text)) return 'BOT_PROTECTION';
    return text.toUpperCase().slice(0, 64);
  };
  const getCommonErrorHint = (rawError) => {
    const code = normalizeErrorCodeForDisplay(rawError);
    switch (code) {
      case 'TOKEN_EXPIRED_LOCAL':
        return '常见原因：本地解析到当前保存的 JWT access token 已过期；该结论基于 token exp 字段预判，未依赖远程接口返回。请重新登录后刷新登录态再重试。';
      case 'TOKEN_EXPIRED':
        return '常见原因：当前保存的 access token 已失效。请在打开的站点页面重新登录，刷新登录态后再重试。';
      case 'USER_BANNED':
        return '常见原因：该站点账号已被封禁，接口仍可访问但业务返回 success=false。';
      case 'PROFILE_STORAGE_NOT_FOUND':
        return '常见原因：当前 Chrome Default Profile 未找到该站点登录态，或登录信息存放在 Cookie/SessionStorage/其他域名。';
      case 'PROFILE_TOKEN_NOT_FOUND':
        return '常见原因：已找到站点本地存储，但未发现 access_token/auth_token 等关键字段。';
      case 'PROFILE_FETCH_NO_TOKENS':
      case 'EMPTY_RESULT':
        return '常见原因：账号可用但当前列表为空，或接口返回了空数据。';
      case 'HTTP_401':
        return '常见原因：登录态失效、Token 过期，或用户标识头不匹配。';
      case 'HTTP_403':
        return '常见原因：权限不足、风控拦截，或站点要求额外验证。';
      case 'HTTP_404':
        return '常见原因：Token 列表接口路由不存在，通常是非标 API 路由或站点已改版。';
      case 'HTTP_429':
        return '常见原因：站点限流，当前请求过多。';
      case 'HTTP_500':
      case 'HTTP_502':
      case 'HTTP_503':
      case 'HTTP_504':
      case 'SERVICE_UNAVAILABLE':
      case 'SYSTEM_OVERLOADED':
        return '常见原因：站点服务异常、网关故障，或上游暂时不可用。';
      case 'TLS_HANDSHAKE_FAILURE':
        return '常见原因：TLS 握手失败，通常是证书链异常、代理干扰或服务端配置问题。';
      case 'DNS_NOT_FOUND':
        return '常见原因：域名已失效、已迁移，或本机 DNS 无法解析。';
      case 'TIMEOUT':
        return '常见原因：站点响应过慢、网络不稳定，或服务端长时间无响应。';
      case 'CONNECTION_REFUSED':
        return '常见原因：目标端口未监听，或服务未启动。';
      case 'HTML_RESPONSE':
        return '常见原因：返回的是页面而不是 JSON，通常被登录页、跳转页或挑战页拦截。';
      case 'JSON_DECODE_FAILED':
        return '常见原因：接口返回结构异常，不是预期的 JSON。';
      case 'INSUFFICIENT_BALANCE':
        return '常见原因：账号余额不足，无法继续调用模型接口。';
      case 'BOT_PROTECTION':
        return '常见原因：站点触发了 Bot/Cloudflare 防护。';
      default:
        return '常见原因：站点接口异常、字段结构变更，或当前环境登录态与备份不一致。';
    }
  };
  const formatUserFacingErrorText = (rawError) => {
    const text = String(rawError || '').trim();
    const code = normalizeErrorCodeForDisplay(text);
    const hint = getCommonErrorHint(text);
    const compactRaw = text && code !== text.toUpperCase().slice(0, 64)
      ? ` 原始：${text.slice(0, 96)}${text.length > 96 ? '...' : ''}`
      : '';
    return `${code} · ${hint}${compactRaw}`;
  };
  const clearBrowserSessionState = () => {
    browserSessionPolling.active = false;
    browserSessionPolling.round = 0;
    browserSessionPolling.totalRounds = 0;
    browserSessionPolling.pending = 0;
    browserSessionPendingSiteNames.value = [];
    refreshTreePendingHints();
  };

  let extractedSites = [];
  let extractionMode = forcedExtractionMode || (isWailsRuntime
    ? normalizeDesktopTokenSourceMode(desktopTokenSourceMode.value)
    : 'browser_direct');
  activeExtractionMode.value = extractionMode;
  let cdpModeContext = null;
  let initialDiscoveryCompleted = false;
  let discoveryInFlight = false;
  let discoveryQueued = false;
  let discoveryQueuedReason = '';
  let discoveryVersion = 0;

  const runModelDiscoveryOnce = async (reason = 'initial') => {
    const runVersion = ++discoveryVersion;
    const snapshot = [...extractedSites];
    const siteNodes = new Array(snapshot.length);
    const fullAllKeys = [];
    const prevSelectableKeys = allKeys.value.filter(isSelectableModelKey);
    const prevSelectableSet = new Set(prevSelectableKeys);
    const prevCheckedSelectableSet = new Set(
      checkedKeys.value.filter(key => prevSelectableSet.has(String(key)))
    );
    const prevAllSelected = prevSelectableKeys.length > 0 && prevCheckedSelectableSet.size === prevSelectableKeys.length;
    const discoveryLimit = 20;
    let currentIndex = 0;
    let noModelSiteCount = 0;
    const isInitialDiscovery = reason === 'initial' || !initialDiscoveryCompleted;
    const persistSiteNodeSnapshot = (siteCacheKey, nodes) => {
      const key = String(siteCacheKey || '').trim();
      if (!key) return;
      try {
        updateSiteCacheTreeNodes(key, normalizeSelectionTreeNodes(nodes));
      } catch (error) {
        console.warn('[SiteCache] tree snapshot sync failed:', error?.message || String(error));
      }
    };
    const existingNodesBySiteCacheKey = new Map();
    if (!isInitialDiscovery && Array.isArray(treeData.value)) {
      treeData.value.forEach(node => {
        const key = String(node?.siteCacheKey || node?.siteName || '').trim();
        if (!key) return;
        if (!existingNodesBySiteCacheKey.has(key)) existingNodesBySiteCacheKey.set(key, []);
        existingNodesBySiteCacheKey.get(key).push(node);
      });
    }

    isDiscoveringModels.value = true;
    loadedSitesCount.value = 0;

    snapshot.forEach((site, idx) => {
      const siteName = String(site?.site_name || `站点${idx + 1}`);
      const siteCacheKey = String(site?._siteCacheKey || site?.siteCacheKey || buildSiteCacheKey(site)).trim();
      if (isInitialDiscovery) {
        siteNodes[idx] = [
          withPendingMeta(siteName, {
            title: `${idx + 1}. [${siteName}] - 模型检测中...`,
            key: `discover-loading|${site?.id || idx}|${runVersion}`,
            disabled: true,
            checkable: false,
            selectable: false,
            isModelDiscovering: true,
            modelDiscoveringHint: '模型检测中',
            siteCacheKey,
            class: 'site-root-summary-node',
            switcherIcon: false,
            children: [],
          }),
        ];
        persistSiteNodeSnapshot(siteCacheKey, siteNodes[idx]);
      } else {
        const existing = existingNodesBySiteCacheKey.get(siteCacheKey);
        siteNodes[idx] = Array.isArray(existing) && existing.length
          ? normalizeSelectionTreeNodes(existing.map(node => withPendingMeta(siteName, { ...node, isModelDiscovering: false })))
          : [];
      }
    });
        if (isInitialDiscovery) {
      treeData.value = normalizeSelectionTreeNodes(siteNodes.flat().filter(Boolean));
      selectionExpandedKeys.value = ensureSelectionRootExpanded(selectionExpandedKeys.value, treeData.value);
    }

    const discoverOne = async (globalIdx) => {
      const site = extractedSites[globalIdx] || snapshot[globalIdx];
      const siteName = String(site?.site_name || `站点${globalIdx + 1}`);
      const siteUrl = String(site?.site_url || '').replace(/\/+$/, '').trim();
      const siteDisplayTitle = `${globalIdx + 1}. [${siteName}]`;
      const siteCacheKey = String(site?._siteCacheKey || site?.siteCacheKey || buildSiteCacheKey(site)).trim();
      const siteDisabled = site?._localDisabled === true;
      const siteNote = String(site?._localNote || '').trim();
      const existingSiteNodes = Array.isArray(siteNodes[globalIdx]) ? siteNodes[globalIdx] : [];
      const hasExistingModelNodes = existingSiteNodes.some(node => {
        if (String(node?.key || '').startsWith('token|')) return true;
        return Array.isArray(node?.children) && node.children.some(child => String(child?.key || '').startsWith('token|'));
      });
      const rawDiscoveryId = site?.account_info?.id || site?.id || site?.uid || site?.user_id || '';
      const discoveryUid = /^\d+$/.test(String(rawDiscoveryId)) ? String(rawDiscoveryId) : '';
      const extractionToken = String(site?.resolved_access_token || site?.account_info?.access_token || '').trim();
      const tokenReplayCandidates = extractionToken && siteUrl
        ? getTokenListEndpointCandidates(site?.site_type).map(path => ({
          url: `${siteUrl}${path}`,
          headers: buildProviderReplayHeaders({
            tokenKey: extractionToken,
            uid: discoveryUid,
            siteUrl,
          }),
        }))
        : [];
      const createSiteRootNode = (statusText, children = [], extra = {}) => withPendingMeta(siteName, {
        title: siteDisplayTitle,
        key: `site-root|${siteCacheKey}`,
        checkable: false,
        disableCheckbox: false,
        selectable: false,
        switcherIcon: false,
        isModelDiscovering: false,
        isSiteRoot: true,
        siteCacheKey,
        siteDisabled,
        siteNote,
        class: [String(extra?.class || '').trim(), 'site-root-summary-node'].filter(Boolean).join(' '),
        titleClass: siteDisabled ? 'tree-site-disabled' : (extra?.titleClass || ''),
        providerTitleText: siteDisplayTitle,
        providerStatusText: statusText,
        providerSiteUrl: siteUrl,
        children,
        ...extra,
      });

      if (siteDisabled) {
        return [
          createSiteRootNode('- 已禁用', [], {
            isProviderDiagnostic: false,
          }),
        ];
      }

      if (isSiteFailed(site)) {
        if (!isInitialDiscovery && hasExistingModelNodes) {
          console.log(`[FetchKeys] 模型刷新跳过: [${siteName}] 当前提取失败，保留上次成功模型节点`);
          return existingSiteNodes.map(node => withPendingMeta(siteName, { ...node, isModelDiscovering: false }));
        }
        const rawError = String(site?.error || '获取令牌失败').trim();
        const errorMsg = formatUserFacingErrorText(rawError);
        return [createSiteRootNode(`- ❌ ${errorMsg}`, [], {
          key: `fail-site|${siteCacheKey}|${globalIdx}`,
          titleClass: 'tree-node-grey',
          isProviderDiagnostic: true,
          providerDiagnostic: {
            stage: 'token_extract',
            siteName,
            siteUrl,
            extractionMode,
            uid: discoveryUid,
            totalTokens: Array.isArray(site?.tokens) ? site.tokens.length : 0,
            usableTokens: countUsableTokensForSite(site),
            tokenEndpoint: String(site?.endpoint || '').trim(),
            storageOrigin: String(site?._profileStorageOrigin || '').trim(),
            storageFields: Array.isArray(site?._profileStorageFields) ? site._profileStorageFields : [],
            replayCandidates: tokenReplayCandidates,
            rawError,
            userFacingError: errorMsg,
            traceLines: [
              `[EXTRACT_FAIL] site=${siteName}`,
              `[ERROR] ${rawError || '获取令牌失败'}`,
              extractionToken ? `[TOKEN] ${maskTokenPreview(extractionToken)}` : '',
              site?.endpoint ? `[TOKEN_ENDPOINT] ${site.endpoint}` : '',
              tokenReplayCandidates.length
                ? `[TOKEN_CANDIDATES] ${tokenReplayCandidates.map(item => item.url).join(' | ')}`
                : '',
              site?._profileStorageOrigin ? `[PROFILE_ORIGIN] ${site._profileStorageOrigin}` : '',
              Array.isArray(site?._profileStorageFields) && site._profileStorageFields.length
                ? `[PROFILE_FIELDS] ${site._profileStorageFields.join(', ')}`
                : '',
            ].filter(Boolean),
          },
        })];
      }

      const usableTokens = (site.tokens || []).filter(isUsableToken);
      if (usableTokens.length === 0) {
        if (!isInitialDiscovery && hasExistingModelNodes) {
          console.log(`[FetchKeys] 模型刷新跳过: [${siteName}] usableTokens=0，保留上次成功模型节点`);
          return existingSiteNodes.map(node => withPendingMeta(siteName, { ...node, isModelDiscovering: false }));
        }
        noModelSiteCount += 1;
        return [createSiteRootNode('- ⏳ Token 已取到，但可用 Key 为 0（等待后台补全）', [], {
          key: `no-usable-token-site|${siteCacheKey}|${globalIdx}`,
          titleClass: 'tree-node-grey',
          isProviderDiagnostic: true,
          providerDiagnostic: {
            stage: 'token_extract',
            siteName,
            siteUrl,
            extractionMode,
            uid: discoveryUid,
            totalTokens: Array.isArray(site?.tokens) ? site.tokens.length : 0,
            usableTokens: 0,
            tokenEndpoint: String(site?.endpoint || '').trim(),
            storageOrigin: String(site?._profileStorageOrigin || '').trim(),
            storageFields: Array.isArray(site?._profileStorageFields) ? site._profileStorageFields : [],
            replayCandidates: tokenReplayCandidates,
            rawError: 'usable_token_empty',
            userFacingError: 'Token 已取到，但当前没有可直接使用的明文 Key',
            traceLines: [
              `[TOKEN_EMPTY] site=${siteName}`,
              `[TOKENS] total=${Array.isArray(site?.tokens) ? site.tokens.length : 0} usable=0`,
              extractionToken ? `[TOKEN] ${maskTokenPreview(extractionToken)}` : '',
              site?.endpoint ? `[TOKEN_ENDPOINT] ${site.endpoint}` : '',
              tokenReplayCandidates.length
                ? `[TOKEN_CANDIDATES] ${tokenReplayCandidates.map(item => item.url).join(' | ')}`
                : '',
              site?._profileStorageOrigin ? `[PROFILE_ORIGIN] ${site._profileStorageOrigin}` : '',
              Array.isArray(site?._profileStorageFields) && site._profileStorageFields.length
                ? `[PROFILE_FIELDS] ${site._profileStorageFields.join(', ')}`
                : '',
            ].filter(Boolean),
          },
        })];
      }

      let effectiveBaseUrl = String(site.site_url || '').replace(/\/+$/, '');
      const rawApiKey = String(site.api_key || '').trim();
      if (rawApiKey.startsWith('http')) {
        effectiveBaseUrl = rawApiKey.replace(/\/+$/, '');
      }
      const endpointsToTry = [
        { url: `${effectiveBaseUrl}/v1/models`, type: 'openai' },
        { url: `${effectiveBaseUrl}/api/models`, type: 'newapi_public' },
        { url: `${effectiveBaseUrl}/api/user/models`, type: 'newapi_user' },
      ];

      let supportedModels = [];
      let discoveryReason = 'unknown';
      let tokenUsed = '';
      let replayRequest = null;
      const traceLines = [
        `[DISCOVERY_START] site=${siteName} usableTokens=${usableTokens.length}`,
      ];
      for (const token of usableTokens) {
        const tokenKey = String(token?.key || token?.access_token || '').trim();
        if (!tokenKey) continue;
        const tokenPreview = maskTokenPreview(tokenKey);
        for (const ep of endpointsToTry) {
          const requestHeaders = buildProviderReplayHeaders({
            tokenKey,
            uid: discoveryUid,
            siteUrl: effectiveBaseUrl || siteUrl,
          });
          replayRequest = {
            url: ep.url,
            headers: requestHeaders,
          };
          traceLines.push(`[TRY] token=${tokenPreview || '(empty)'} endpoint=${ep.type} url=${ep.url}`);
          try {
            const res = await apiFetch(`/api/proxy-get?url=${encodeURIComponent(ep.url)}&uid=${discoveryUid}`, {
              headers: { Authorization: `Bearer ${tokenKey}` },
            });
            if (!res.ok) {
              let errorPayload = '';
              try {
                errorPayload = stringifyPreview(await res.clone().json());
              } catch {
                errorPayload = stringifyPreview(await res.text().catch(() => ''));
              }
              traceLines.push(`[HTTP_${res.status}] ${ep.url}${errorPayload ? ` payload=${errorPayload}` : ''}`);
              discoveryReason = `http_${res.status}`;
              continue;
            }
            let result = null;
            try {
              result = await res.json();
            } catch (parseError) {
              discoveryReason = 'json_decode_failed';
              traceLines.push(`[PARSE_FAIL] ${ep.url} error=${parseError?.message || 'unknown'}`);
              continue;
            }
            const rawData = Array.isArray(result)
              ? result
              : (result.data?.data || result.data?.items || result.data || []);
            if (Array.isArray(rawData) && rawData.length > 0) {
              supportedModels = rawData
                .map(m => (typeof m === 'string' ? m : (m.id || m.name || m)))
                .filter(m => typeof m === 'string')
                .sort();
              if (supportedModels.length > 0) {
                tokenUsed = tokenKey;
                discoveryReason = `ok_${ep.type}`;
                traceLines.push(`[SUCCESS] ${ep.url} models=${supportedModels.length}`);
                break;
              }
            } else {
              discoveryReason = 'empty_models';
              traceLines.push(`[EMPTY_MODELS] ${ep.url} payload=${stringifyPreview(result) || '(empty)'}`);
            }
          } catch (e) {
            discoveryReason = `exception_${e?.message || 'unknown'}`;
            traceLines.push(`[EXCEPTION] ${ep.url} error=${e?.message || 'unknown'}`);
          }
        }
        if (supportedModels.length > 0) break;
      }

      if (supportedModels.length === 0) {
        if (!isInitialDiscovery && hasExistingModelNodes) {
          console.log(`[FetchKeys] 模型刷新跳过: [${siteName}] 本轮模型探测为空，保留上次成功模型节点`);
          return existingSiteNodes.map(node => withPendingMeta(siteName, { ...node, isModelDiscovering: false }));
        }
        noModelSiteCount += 1;
        console.log(`[FetchKeys] 模型发现失败: [${siteName}] usableTokens=${usableTokens.length}, reason=${discoveryReason}`);
        const discoveryReasonText = formatUserFacingErrorText(discoveryReason);
        return [createSiteRootNode(`- ⚠️ 未能探测到可用模型列表（usable=${usableTokens.length}，${discoveryReasonText}）`, [], {
          key: `no-model-site|${siteCacheKey}|${globalIdx}`,
          titleClass: 'tree-node-grey',
          isProviderDiagnostic: true,
          providerDiagnostic: {
            stage: 'model_discovery',
            siteName,
            siteUrl,
            extractionMode,
            uid: discoveryUid,
            totalTokens: Array.isArray(site?.tokens) ? site.tokens.length : 0,
            usableTokens: usableTokens.length,
            rawError: discoveryReason,
            userFacingError: discoveryReasonText,
            replayRequest,
            traceLines,
          },
        })];
      }

      console.log(`[FetchKeys] 模型发现成功: [${siteName}] models=${supportedModels.length}, usableTokens=${usableTokens.length}, token=${tokenUsed.slice(0, 12)}...`);
      const tokenChildren = [];
      usableTokens.forEach((token, idx) => {
        const tokenKey = String(token.key || token.access_token || '').trim();
        if (!tokenKey) return;
        const tokenName = String(token.name || `Token ${idx + 1}`).trim();
        const tokenNodeKey = `token|${siteCacheKey}|${tokenKey}`;
        const children = supportedModels.map(model => {
          const itemKey = `${siteCacheKey}|${tokenKey}|${model}`;
          fullAllKeys.push(itemKey);
          return { title: model, key: itemKey, isLeaf: true };
        });
        fullAllKeys.push(tokenNodeKey);
        tokenChildren.push(withPendingMeta(siteName, {
          title: `${siteDisplayTitle} ${tokenName} (${tokenKey.slice(0, 15)}...)`,
          key: tokenNodeKey,
          siteCacheKey,
          isModelDiscovering: false,
          disableCheckbox: false,
          isManualToken: String(token?.source || '').trim() === 'manual',
          providerTitleText: siteDisplayTitle,
          providerStatusText: `${tokenName} (${tokenKey.slice(0, 15)}...)`,
          providerSiteUrl: siteUrl,
          children,
        }));
      });

      return [createSiteRootNode(`- ${usableTokens.length} 个可用 Key / ${supportedModels.length} 个模型`, tokenChildren, {
        disableCheckbox: false,
      })];
    };

    const worker = async () => {
      while (currentIndex < snapshot.length) {
        const idx = currentIndex++;
        if (runVersion !== discoveryVersion) return;
        const nodes = await discoverOne(idx);
        if (runVersion !== discoveryVersion) return;
        siteNodes[idx] = nodes;
        const snapshotSite = snapshot[idx] || extractedSites[idx];
        persistSiteNodeSnapshot(snapshotSite?._siteCacheKey || snapshotSite?.siteCacheKey || buildSiteCacheKey(snapshotSite), nodes);
        loadedSitesCount.value += 1;
        treeData.value = normalizeSelectionTreeNodes(siteNodes.flat().filter(Boolean));
        selectionExpandedKeys.value = ensureSelectionRootExpanded(selectionExpandedKeys.value, treeData.value);
      }
    };

    await Promise.all(
      Array.from({ length: Math.min(discoveryLimit, Math.max(snapshot.length, 1)) }, () => worker())
    );

    if (runVersion !== discoveryVersion) return;

    treeData.value = normalizeSelectionTreeNodes(siteNodes.flat().filter(Boolean));
    selectionExpandedKeys.value = ensureSelectionRootExpanded(selectionExpandedKeys.value, treeData.value);
    const nextSelectableKeys = fullAllKeys.filter(isSelectableModelKey);
    let nextCheckedKeys = [];
    if (!initialDiscoveryCompleted || prevAllSelected) {
      // 首次默认全选；若上次是“全选”，增量刷新后继续保持全选（避免新模型漏选）
      nextCheckedKeys = [...nextSelectableKeys];
    } else {
      // 保留用户已勾选项，同时清理不存在的脏 key（避免勾选状态残留）
      nextCheckedKeys = nextSelectableKeys.filter(key => prevCheckedSelectableSet.has(key));
    }
    allKeys.value = [...nextSelectableKeys];
    checkedKeys.value = [...new Set(nextCheckedKeys)];
    initialDiscoveryCompleted = true;
    isDiscoveringModels.value = false;

    const tokenSites = extractedSites.filter(site => !isSiteFailed(site)).length;
    const usableTokenSites = extractedSites.filter(site => countUsableTokensForSite(site) > 0).length;
    const selectableModelCount = fullAllKeys.filter(key => key.includes('|') && !key.startsWith('token|')).length;
    console.log(`[FetchKeys] 模型发现阶段完成(${reason}): tokenSites=${tokenSites}, usableTokenSites=${usableTokenSites}, noModelSites=${noModelSiteCount}, selectableModels=${selectableModelCount}`);
  };

  const requestDiscoveryRefresh = async (reason = 'unknown') => {
    if (discoveryInFlight) {
      discoveryQueued = true;
      discoveryQueuedReason = reason;
      return;
    }
    discoveryInFlight = true;
    try {
      let currentReason = reason;
      do {
        discoveryQueued = false;
        try {
          await runModelDiscoveryOnce(currentReason);
        } catch (err) {
          console.warn(`[FetchKeys] 模型发现异常(${currentReason}):`, err?.message || String(err));
          isDiscoveringModels.value = false;
          break;
        }
        currentReason = discoveryQueuedReason || 'queued-refresh';
      } while (discoveryQueued);
    } finally {
      discoveryInFlight = false;
    }
  };
  const replaceExtractedSites = async (updater, reason = 'site-cache-update', options = {}) => {
    const nextSites = typeof updater === 'function' ? updater([...extractedSites]) : updater;
    extractedSites = Array.isArray(nextSites) ? nextSites : [];
    validAccounts.value = extractedSites;
    if (options?.syncCache !== false) {
      syncSiteCacheSnapshot(extractedSites, {
        importSource: importSource || 'site_tree_runtime',
        refreshedAt: Date.now(),
      });
    }
    await requestDiscoveryRefresh(reason);
  };
  updateActiveSiteSession(replaceExtractedSites, requestDiscoveryRefresh);

  if (prefetchedSites) {
    extractedSites = [...prefetchedSites].map(site => ({
      ...site,
      id: String(site?.id || site?._siteCacheKey || site?.siteCacheKey || buildSiteCacheKey(site)).trim(),
      site_name: String(site?.site_name || site?.siteName || '').trim(),
      site_url: normalizeSiteUrl(site?.site_url || site?.siteUrl || ''),
      api_key: normalizeSiteUrl(site?.api_key || site?.apiBaseUrl || site?.site_url || site?.siteUrl || ''),
      _siteCacheKey: String(site?._siteCacheKey || site?.siteCacheKey || buildSiteCacheKey(site)).trim(),
      _localDisabled: site?._localDisabled === true,
      _localNote: String(site?._localNote || '').trim(),
    }));
    validAccounts.value = extractedSites;
    syncSiteCacheSnapshot(extractedSites, {
      importSource: importSource || 'site_cache_restore',
      refreshedAt: Date.now(),
    });
    void preloadAllQuotas(extractedSites);
    summarizeStage('站点缓存恢复', extractedSites, []);
    step.value = 2;
    isLoadingModels.value = false;
    await requestDiscoveryRefresh('site-cache-restore');
    activeExtractionMode.value = '';
    return extractedSites;
  }

  try {
    if (isWailsRuntime && extractionMode !== 'browser_direct') {
      if (extractionMode === 'profile_file' && isChromeProfileAuthAvailable.value) {
        console.log(`[FetchKeys] Wails WebView detected, use Chrome Profile extraction for ${accountsToFetch.length} sites`);
        markLocalProfileExtractionStart(accountsToFetch.length);
        startFetchKeysProgressPolling();
        let profileExtractError = null;
        try {
          const response = await extractChromeProfileTokens(accountsToFetch);
          if (Array.isArray(response?.warnings) && response.warnings.length > 0) {
            console.warn('[FetchKeys] Chrome Profile extraction warnings:', response.warnings.join(' | '));
          }
          extractedSites = mergeChromeProfileExtractedSites(accountsToFetch, response);
          syncSiteCacheSnapshot(extractedSites, {
            importSource: importSource || 'profile_file',
            refreshedAt: Date.now(),
          });
        } catch (err) {
          profileExtractError = err;
        } finally {
          stopFetchKeysProgressPolling();
          markLocalProfileExtractionDone(extractedSites);
        }
        if (profileExtractError) {
          if (fallbackOnProfileFailure) {
            console.warn(`[FetchKeys] Profile extraction failed for ${importSource || 'unknown_source'}, fallback to browser_direct:`, profileExtractError?.message || String(profileExtractError));
            message.warning(`Profile 文件提取失败，已自动改用导入登录态继续提取：${profileExtractError?.message || String(profileExtractError)}`);
            extractionMode = 'browser_direct';
          } else {
            throw profileExtractError;
          }
        }
      } else if (extractionMode === 'profile_file') {
        throw new Error('当前桌面端尚未暴露 Profile 文件提取接口，无法使用 Profile 文件模式');
      } else if (extractionMode === 'cdp_restart') {
        console.log(`[FetchKeys] Wails WebView detected, use CDP restart mode for ${accountsToFetch.length} sites`);
        const cdpStart = await startCdpRestartMode(accountsToFetch, isSiteFailed);
        if (cdpStart?.cancelled) {
          if (cdpStart.reason === 'browser_not_selected') {
            message.warning('你取消了浏览器选择，本次未执行 CDP 重开模式。');
          } else if (cdpStart.reason === 'login_not_confirmed') {
            message.warning('你取消了登录确认，本次未执行 CDP 重开模式。');
          } else if (cdpStart.reason === 'browser_kill_cancelled') {
            message.warning('你取消了结束浏览器进程，本次未执行 CDP 重开模式。');
          } else if (cdpStart.reason === 'no_site_opened') {
            message.warning('未成功打开任何站点，本次未执行 CDP 重开模式。');
          }
          isLoadingModels.value = false;
          step.value = 1;
          browserSessionPolling.active = false;
          browserSessionPolling.round = 0;
          browserSessionPolling.totalRounds = 0;
          browserSessionPolling.pending = 0;
          browserSessionPendingSiteNames.value = [];
          return;
        }
        cdpModeContext = cdpStart;
        extractedSites = cdpStart.extractedSites;
        syncSiteCacheSnapshot(extractedSites, {
          importSource: importSource || 'cdp_restart',
          refreshedAt: Date.now(),
        });
      } else {
        throw new Error(`未知桌面端提取模式: ${extractionMode}`);
      }
    }
    if (!isWailsRuntime || extractionMode === 'browser_direct') {
      if (extractionMode === 'browser_direct') {
        markBrowserExtractionStart(accountsToFetch.length);
      }
      if (isWailsRuntime && importSource === 'json_backup') {
        console.log(`[FetchKeys] JSON backup import detected, skip profile_file and use browser_direct extraction for ${accountsToFetch.length} sites`);
      } else if (isWailsRuntime && importSource === 'extension_import') {
        console.log(`[FetchKeys] Extension import extraction via browser_direct for ${accountsToFetch.length} sites`);
      }
      const BROWSER_FETCH_CONCURRENCY = 25;
      const browserResults = new Array(accountsToFetch.length);
      let currentIdx = 0;

      const browserFetchWorker = async () => {
        while (currentIdx < accountsToFetch.length) {
          const idx = currentIdx++;
          const result = await fetchTokensForAccountFromBrowserV2(accountsToFetch[idx]);
          browserResults[idx] = result;
          if (extractionMode === 'browser_direct') {
            const siteName = String(accountsToFetch[idx]?.site_name || '').trim();
            const succeeded = Array.isArray(result?.tokens) && result.tokens.length > 0;
            markBrowserExtractionProgress(siteName, succeeded);
          }
        }
      };

      await Promise.all(
        Array.from(
          { length: Math.min(BROWSER_FETCH_CONCURRENCY, Math.max(accountsToFetch.length, 1)) },
          () => browserFetchWorker()
        )
      );

      extractedSites = browserResults;
      syncSiteCacheSnapshot(extractedSites, {
        importSource: importSource || extractionMode || 'browser_direct',
        refreshedAt: Date.now(),
      });
      if (extractionMode === 'browser_direct') {
        markBrowserExtractionDone(extractedSites);
      }

      const failedAccounts = accountsToFetch.filter((acc, i) => browserResults[i]?._needServerFallback === true);
      if (failedAccounts.length > 0) {
        console.log(`[FetchKeys] 浏览器端失败 ${failedAccounts.length} 个，尝试服务端代理兜底...`);
        try {
          const serverResponse = await apiFetch('/api/fetch-keys', {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify({ accounts: failedAccounts }),
          });
          if (serverResponse.ok) {
            const serverData = await serverResponse.json();
            const serverResults = Array.isArray(serverData?.results) ? serverData.results : [];
            const mergeStats = mergeExtractedSiteResults(extractedSites, serverResults);
            syncSiteCacheSnapshot(extractedSites, {
              importSource: `${importSource || 'browser_direct'}_server_fallback`,
              refreshedAt: Date.now(),
            });
            console.log(`[FetchKeys] 服务端兜底合并: mergedSites=${mergeStats.mergedSites}, recoveredSites=${mergeStats.recoveredSites}, gainedTokens=${mergeStats.gainedTokens}, gainedUsableTokens=${mergeStats.gainedUsableTokens}`);
          }
        } catch (e) {
          console.warn('[FetchKeys] 服务端兜底失败:', e?.message || String(e));
        }
      }
      if (extractionMode === 'browser_direct') {
        markBrowserExtractionDone(extractedSites);
      }
    }

    let stillFailedAccounts = extractionMode === 'profile_file'
      ? collectProfileFileRetrySites(extractedSites)
      : extractedSites.filter(site => isSiteFailed(site));
    if (PROFILE_FILE_WEBVIEW_FALLBACK_ENABLED && isWailsRuntime && extractionMode === 'profile_file') {
      try {
        await autoOpenDesktopProfileAssist(stillFailedAccounts, 'initial-extract');
      } catch (assistError) {
        console.warn('[ProfileAssist] initial auto open failed:', assistError?.message || String(assistError));
      }
    }
    if (isWailsRuntime && extractionMode === 'profile_file') {
      try {
        await autoOpenExpiredTokenSitesForRelogin(stillFailedAccounts, 'initial-extract');
      } catch (reloginError) {
        console.warn('[ProfileRelogin] initial auto open failed:', reloginError?.message || String(reloginError));
      }
    }
    updateBrowserSessionPendingSites(stillFailedAccounts);
    validAccounts.value = extractedSites;
    syncSiteCacheSnapshot(extractedSites, {
      importSource: importSource || extractionMode || 'initial_extract',
      refreshedAt: Date.now(),
    });
    void preloadAllQuotas(extractedSites);
    if (PROFILE_FILE_WEBVIEW_FALLBACK_ENABLED && isWailsRuntime && extractionMode === 'profile_file') {
      void closeDesktopProfileAssistForRecoveredSites(extractedSites, 'initial-extract');
    }

    summarizeStage('提取阶段完成', extractedSites, stillFailedAccounts);

    // 先展示结果页，再后台异步更新
    step.value = 2;
    isLoadingModels.value = false;
    void requestDiscoveryRefresh('initial');

    if (extractionMode === 'cdp_restart' && cdpModeContext && stillFailedAccounts.length === 0) {
      clearBrowserSessionState();
    }

    if (PROFILE_FILE_MANUAL_RECOVERY_ENABLED && isWailsRuntime && extractionMode === 'profile_file' && stillFailedAccounts.length > 0) {
      void (async () => {
        const profileRetryMessageKey = 'profile-file-retry';
        try {
          const recoveryAction = await confirmProfileFileLoginRecovery(stillFailedAccounts);
          if (!recoveryAction?.shouldRetry) {
            if (recoveryAction?.openedCount > 0) {
              message.info('失败站点已打开。你完成登录后，可重新执行 Profile 文件模式提取。', 5);
            }
            return;
          }

          const maxProfileRetryRounds = 3;
          let totalMergeStats = {
            mergedSites: 0,
            recoveredSites: 0,
            gainedTokens: 0,
            gainedUsableTokens: 0,
          };

          for (let round = 1; round <= maxProfileRetryRounds && stillFailedAccounts.length > 0; round += 1) {
            message.loading({
              key: profileRetryMessageKey,
              content: `正在重新读取 Chrome Profile 文件（第 ${round}/${maxProfileRetryRounds} 轮）...`,
              duration: 0,
            });

            const retryResponse = await extractChromeProfileTokens(stillFailedAccounts);
            if (Array.isArray(retryResponse?.warnings) && retryResponse.warnings.length > 0) {
              console.warn('[FetchKeys] Chrome Profile retry warnings:', retryResponse.warnings.join(' | '));
            }

            const retrySites = mergeChromeProfileExtractedSites(stillFailedAccounts, retryResponse);
            const mergeStats = mergeExtractedSiteResults(extractedSites, retrySites);
            syncSiteCacheSnapshot(extractedSites, {
              importSource: 'profile_file_retry',
              refreshedAt: Date.now(),
            });
            totalMergeStats = {
              mergedSites: totalMergeStats.mergedSites + mergeStats.mergedSites,
              recoveredSites: totalMergeStats.recoveredSites + mergeStats.recoveredSites,
              gainedTokens: totalMergeStats.gainedTokens + mergeStats.gainedTokens,
              gainedUsableTokens: totalMergeStats.gainedUsableTokens + mergeStats.gainedUsableTokens,
            };
            validAccounts.value = extractedSites;
            void preloadAllQuotas(extractedSites);
            if (PROFILE_FILE_WEBVIEW_FALLBACK_ENABLED && isWailsRuntime && extractionMode === 'profile_file') {
              void closeDesktopProfileAssistForRecoveredSites(extractedSites, `profile-retry-${round}`);
            }

            stillFailedAccounts = collectProfileFileRetrySites(extractedSites);
            if (PROFILE_FILE_WEBVIEW_FALLBACK_ENABLED) {
              try {
                await autoOpenDesktopProfileAssist(stillFailedAccounts, `profile-retry-${round}`);
              } catch (assistError) {
                console.warn('[ProfileAssist] retry auto open failed:', assistError?.message || String(assistError));
              }
            }
            try {
              await autoOpenExpiredTokenSitesForRelogin(stillFailedAccounts, `profile-retry-${round}`);
            } catch (reloginError) {
              console.warn('[ProfileRelogin] retry auto open failed:', reloginError?.message || String(reloginError));
            }
            summarizeStage(`Profile 文件模式手动登录后重试 round=${round}`, extractedSites, stillFailedAccounts);
            console.log(`[FetchKeys] Profile 文件模式重试合并 round=${round}: mergedSites=${mergeStats.mergedSites}, recoveredSites=${mergeStats.recoveredSites}, gainedTokens=${mergeStats.gainedTokens}, gainedUsableTokens=${mergeStats.gainedUsableTokens}`);

            if (stillFailedAccounts.length === 0) break;
            if (round < maxProfileRetryRounds) {
              await sleep(3000);
            }
          }

          if (totalMergeStats.gainedUsableTokens > 0) {
            void requestDiscoveryRefresh('profile-file-manual-retry');
          }

          if (stillFailedAccounts.length > 0) {
            message.warning({
              key: profileRetryMessageKey,
              content: `重新读取完成，仍有 ${stillFailedAccounts.length} 个站点未获取成功。`,
              duration: 5,
            });
          } else {
            message.success({
              key: profileRetryMessageKey,
              content: 'Profile 文件重新读取完成，失败站点已恢复。',
              duration: 3,
            });
          }
        } catch (e) {
          console.warn('[FetchKeys] Profile 文件模式手动重试失败:', e?.message || String(e));
          message.error({
            key: profileRetryMessageKey,
            content: `Profile 文件重新读取失败: ${e?.message || String(e)}`,
            duration: 5,
          });
        }
      })();
    } else if (extractionMode === 'cdp_restart' && cdpModeContext && stillFailedAccounts.length > 0) {
      void (async () => {
        try {
        const { browserType, maxRetryRounds, retryIntervalMs } = cdpModeContext;
          browserSessionPolling.active = true;
          browserSessionPolling.totalRounds = maxRetryRounds;
          browserSessionPolling.round = 1;
          browserSessionPolling.pending = stillFailedAccounts.length;
          updateBrowserSessionPendingSites(stillFailedAccounts);
          refreshTreePendingHints();

          try {
            for (let round = 2; round <= maxRetryRounds && stillFailedAccounts.length > 0; round += 1) {
              browserSessionPolling.round = round;
              browserSessionPolling.pending = stillFailedAccounts.length;
              updateBrowserSessionPendingSites(stillFailedAccounts);
              refreshTreePendingHints();
              console.log(`[FetchKeys] CDP 重开模式继续抓取: ${browserType} round=${round}/${maxRetryRounds}, pendingSites=${stillFailedAccounts.length}`);

              const browserSessionResults = await browserSessionFetchForAccounts(stillFailedAccounts, browserType, round, maxRetryRounds);
              const mergeStats = mergeExtractedSiteResults(extractedSites, browserSessionResults);
              syncSiteCacheSnapshot(extractedSites, {
                importSource: 'cdp_restart_polling',
                refreshedAt: Date.now(),
              });
              validAccounts.value = extractedSites;
              void preloadAllQuotas(extractedSites);

              stillFailedAccounts = extractedSites.filter(isSiteFailed);
              browserSessionPolling.pending = stillFailedAccounts.length;
              updateBrowserSessionPendingSites(stillFailedAccounts);
              refreshTreePendingHints();
              summarizeStage(`受控浏览器第 ${round} 轮`, extractedSites, stillFailedAccounts);
              console.log(`[FetchKeys] 受控浏览器第 ${round} 轮合并: mergedSites=${mergeStats.mergedSites}, recoveredSites=${mergeStats.recoveredSites}, gainedTokens=${mergeStats.gainedTokens}, gainedUsableTokens=${mergeStats.gainedUsableTokens}`);

              if (mergeStats.gainedUsableTokens > 0) {
                void requestDiscoveryRefresh(`cdp-round-${round}`);
              }

              if (stillFailedAccounts.length === 0) break;
              if (round < maxRetryRounds) {
                await sleep(retryIntervalMs);
              }
            }
          } finally {
            clearBrowserSessionState();
          }

          if (stillFailedAccounts.length > 0) {
            message.warning(`CDP 重开模式轮询完成，仍有 ${stillFailedAccounts.length} 个站点未抓取成功。`);
          }
          summarizeStage('CDP 重开模式轮询结束', extractedSites, stillFailedAccounts);
          void requestDiscoveryRefresh('cdp-polling-finished');
        } catch (e) {
          clearBrowserSessionState();
          console.warn('[FetchKeys] CDP 重开模式轮询失败:', e?.message || String(e));
          message.warning(`CDP 重开模式轮询未执行成功: ${e?.message || String(e)}`);
        }
      })();
    } else if (!isWailsRuntime && stillFailedAccounts.length > 0) {
      void (async () => {
        try {
          const detected = await getDetectedFallbackBrowser();
          const browserType = await chooseDetectedFallbackBrowserType(detected);
          if (!browserType) {
            message.warning('你取消了浏览器选择，当前保留已提取结果并继续后续流程。');
            return;
          }

          const readyForShadow = await confirmShadowLoginReadiness(stillFailedAccounts, browserType);
          if (!readyForShadow) {
            message.warning('你取消了 shadow 模式抓取，请先在浏览器中完成失效站点登录后再重试。');
            return;
          }

          let openedCount = 0;
          try {
            openedCount = await openSitesInBrowserSession(stillFailedAccounts, browserType);
          } catch (openErr) {
            const fallbackStatus = await getFallbackBrowserStatus(browserType).catch(() => ({
              running: false,
              attached: false,
              launching: false,
              managed: false,
              browserType,
            }));
            const shouldHandleAsProfileInUse =
              openErr?.code === 'BROWSER_PROFILE_IN_USE' ||
              (fallbackStatus.running && !fallbackStatus.attached && !fallbackStatus.launching && !fallbackStatus.managed);

            if (shouldHandleAsProfileInUse) {
              const shouldKill = await confirmWithModal({
                title: '浏览器已占用',
                content: `${browserType === 'edge' ? 'Edge' : 'Chrome'} 当前已在普通模式运行，默认 profile 被占用。结束后会关闭该浏览器所有窗口，是否继续？`,
                okText: '结束并继续',
                cancelText: '取消',
                okType: 'danger',
              });
              if (!shouldKill) {
                message.warning('你取消了结束浏览器进程，当前保留已有结果并继续后续流程。');
                return;
              }

              const restartResult = await restartBrowserSessionProcessAndOpen(stillFailedAccounts, browserType);
              if (!restartResult?.stopped) {
                message.error(`${browserType === 'edge' ? 'Edge' : 'Chrome'} 进程结束后仍未完全退出，请手动关闭后重试。`);
                return;
              }
              openedCount = Number(restartResult?.opened || stillFailedAccounts.length);
            } else {
              throw openErr;
            }
          }

          if (openedCount <= 0) return;

          const availableText = detected.availableTypes.map(type => (type === 'edge' ? 'Edge' : 'Chrome')).join(' / ');
          message.info(`已探测到 ${availableText}，当前使用 ${browserType === 'edge' ? 'Edge' : 'Chrome'} 打开 ${openedCount} 个失败站点，后台自动轮询抓取中。`, 6);

          const maxRetryRounds = 3;
          const retryIntervalMs = 15000;
          browserSessionPolling.active = true;
          browserSessionPolling.totalRounds = maxRetryRounds;
          browserSessionPolling.pending = stillFailedAccounts.length;
          updateBrowserSessionPendingSites(stillFailedAccounts);
          refreshTreePendingHints();

          try {
            for (let round = 1; round <= maxRetryRounds && stillFailedAccounts.length > 0; round += 1) {
              browserSessionPolling.round = round;
              browserSessionPolling.pending = stillFailedAccounts.length;
              updateBrowserSessionPendingSites(stillFailedAccounts);
              refreshTreePendingHints();
              console.log(`[FetchKeys] 受控浏览器自动抓取: ${browserType} round=${round}/${maxRetryRounds}, pendingSites=${stillFailedAccounts.length}`);

              const browserSessionResults = await browserSessionFetchForAccounts(stillFailedAccounts, browserType, round, maxRetryRounds);
              const mergeStats = mergeExtractedSiteResults(extractedSites, browserSessionResults);
              syncSiteCacheSnapshot(extractedSites, {
                importSource: 'browser_session_polling',
                refreshedAt: Date.now(),
              });
              validAccounts.value = extractedSites;
              void preloadAllQuotas(extractedSites);

              stillFailedAccounts = extractedSites.filter(site => isSiteFailed(site) || shouldUseDesktopProfileAssist(site));
              browserSessionPolling.pending = stillFailedAccounts.length;
              updateBrowserSessionPendingSites(stillFailedAccounts);
              refreshTreePendingHints();
              summarizeStage(`受控浏览器第 ${round} 轮`, extractedSites, stillFailedAccounts);
              console.log(`[FetchKeys] 受控浏览器第 ${round} 轮合并: mergedSites=${mergeStats.mergedSites}, recoveredSites=${mergeStats.recoveredSites}, gainedTokens=${mergeStats.gainedTokens}, gainedUsableTokens=${mergeStats.gainedUsableTokens}`);

              if (mergeStats.gainedUsableTokens > 0) {
                void requestDiscoveryRefresh(`browser-round-${round}`);
              }

              if (stillFailedAccounts.length === 0) break;
              if (round < maxRetryRounds) {
                await sleep(retryIntervalMs);
              }
            }
          } finally {
            clearBrowserSessionState();
          }

          if (stillFailedAccounts.length > 0) {
            message.warning(`受控浏览器自动轮询完成，仍有 ${stillFailedAccounts.length} 个站点未抓取成功。`);
          }
          summarizeStage('受控浏览器轮询结束', extractedSites, stillFailedAccounts);
          void requestDiscoveryRefresh('browser-polling-finished');
        } catch (e) {
          clearBrowserSessionState();
          console.warn('[FetchKeys] 受控浏览器兜底失败:', e?.message || String(e));
          message.warning(`失败站点受控浏览器兜底未执行成功: ${e?.message || String(e)}`);
        }
      })();
    }
  } catch (err) {
    clearBrowserSessionState();
    message.error(`批量提取 Token 失败: ${err?.message || String(err)}`);
    isLoadingModels.value = false;
    isDiscoveringModels.value = false;
    step.value = 1;
    return [];
  } finally {
    activeExtractionMode.value = '';
  }
  return extractedSites;
};

// --- Tree Actions ---
const selectAllNodes = () => {
  checkedKeys.value = allKeys.value.filter(isSelectableModelKey);
};

const unselectAllNodes = () => {
  checkedKeys.value = [];
};

const selectChatModelsOnly = () => {
  const notChatPattern = /(bge|stabilityai|dall|mj|stable|flux|video|midjourney|stable-diffusion|playground|swap_face|tts|whisper|text|emb|luma|vidu|pdf|suno|pika|chirp|domo|runway|cogvideo|babbage|davinci|gpt-4o-realtime)/i;
  
  const filteredKeys = [];
  const childKeys = allKeys.value.filter(isSelectableModelKey);
  childKeys.forEach(k => {
    const parts = k.split('|');
    const model = parts[2]; 
    if (!notChatPattern.test(model) && !/(image|audio|video|music|pdf|flux|suno|embed)/i.test(model)) {
      filteredKeys.push(k);
    }
  });
  
  checkedKeys.value = filteredKeys;
};

// --- Testing Logic ---
const startBatchCheck = async () => {
  // Extract selected tasks
  const selectedModelKeys = checkedKeys.value.filter(k =>
    k.includes('|') &&
    !k.startsWith('token|') &&
    !k.startsWith('fail-site|') &&
    !k.startsWith('no-model-site|') &&
    !k.startsWith('no-usable-token-site|') &&
    !k.startsWith('discover-loading|')
  );
  if (selectedModelKeys.length === 0) {
    message.warning('请至少勾选一个模型进行测试');
    return;
  }

  step.value = 3;
  testing.value = true;
  cancelTokens.value = [];
  testResults.value = [];
  organizedSourceResults.value = [];
  
  // Build task queue
  const tasksQueue = [];
  selectedModelKeys.forEach((keyStr, idx) => {
    // 格式: siteId|tokenKey|modelName
    const parts = keyStr.split('|');
    if (parts.length < 3) return; // 忽略不符合新格式的
    
    const [siteId, tokenKey, modelName] = parts;
    const site = validAccounts.value.find(s => s.id === siteId);
    
    if (site) {
      // 增强逻辑：对 api_key 进行清洗，优先从中提取 API 基址
      let effectiveUrl = site.site_url;
      const rawApiKey = String(site.api_key || '').trim();
      if (rawApiKey.startsWith('http')) {
        effectiveUrl = rawApiKey;
      }
      
      const task = {
        id: `task_${idx}`,
        siteId,
        siteName: site.site_name,
        siteUrl: effectiveUrl,
        apiKey: tokenKey, // <--- 使用真正的 sk- 密钥!
        modelName: modelName,
        status: 'pending',
        statusText: '排队中',
        responseTime: '-',
        remark: '-',
        accountData: site, // 仅做记录
      };
      tasksQueue.push(task);
      testResults.value.push(task);
    }
  });

  if (tasksQueue.length === 0) {
    step.value = 2;
    testing.value = false;
    const detail = `selected=${selectedModelKeys.length}, validAccounts=${validAccounts.value.length}`;
    console.warn(`[BatchCheck] 未能根据当前勾选构建检测任务: ${detail}`);
    logClientDiagnostic('batch.start.error', `failed to build tasks from selected model keys: ${detail}`);
    message.error('当前勾选项未能构建出有效检测任务，请重新导入或刷新站点缓存后再试');
    return;
  }

  totalTasks.value = tasksQueue.length;
  completedTasks.value = 0;
  saveLastResultsSnapshot();
  scheduleOrganizedSourceRefresh(true);
  console.log(`[BatchCheck] 开始检测: selectedModelKeys=${selectedModelKeys.length}, queuedTasks=${tasksQueue.length}`);

  // Concurrency executor
  let currentIndex = 0;
  
  const worker = async () => {
    while (currentIndex < tasksQueue.length && testing.value) {
      const taskIndex = currentIndex++;
      const task = tasksQueue[taskIndex];
      task.status = 'testing';
      task.statusText = '测试中';
      
      await runSingleTest(task);
      
      completedTasks.value++;
      scheduleOrganizedSourceRefresh();
    }
  };

  const workers = [];
  const actualConcurrency = Math.min(batchConcurrency.value, tasksQueue.length);
  for (let i = 0; i < actualConcurrency; i++) {
    workers.push(worker());
  }

  await Promise.all(workers);
  
  if (testing.value) {
    testing.value = false;
    scheduleOrganizedSourceRefresh(true);
    await syncDetectedKeysToLocalStorage({ silent: true });
    message.success('批量检测完成！');
    // Save to history
    saveLastResultsSnapshot();
  }
};

const stopTesting = () => {
  testing.value = false;
  // Trigger abort on controllers
  cancelTokens.value.forEach(controller => controller.abort());
  message.info('已停止检测');
};

function normalizeKeyManagementSiteUrl(rawUrl) {
  return String(rawUrl || '').trim().replace(/\/+$/, '');
}

function buildKeyManagementRowKey(siteUrl, apiKey) {
  return `${normalizeKeyManagementSiteUrl(siteUrl)}::${String(apiKey || '').trim()}`;
}

function loadStoredAutoKeyManagementRecords() {
  try {
    const raw = localStorage.getItem(KEY_MANAGEMENT_STORAGE_KEY);
    const parsed = JSON.parse(raw || '[]');
    return Array.isArray(parsed) ? parsed : [];
  } catch (error) {
    console.error('Load stored key management records failed', error);
    return [];
  }
}

function mergeDetectedKeyManagementRecords(existingRecords, incomingRecords) {
  const mergedMap = new Map();

  (Array.isArray(existingRecords) ? existingRecords : []).forEach(record => {
    const rowKey = String(record?.rowKey || buildKeyManagementRowKey(record?.siteUrl, record?.apiKey)).trim();
    if (!rowKey) return;
    mergedMap.set(rowKey, {
      ...record,
      rowKey,
    });
  });

  (Array.isArray(incomingRecords) ? incomingRecords : []).forEach(record => {
    const rowKey = String(record?.rowKey || buildKeyManagementRowKey(record?.siteUrl, record?.apiKey)).trim();
    if (!rowKey) return;
    const existing = mergedMap.get(rowKey) || {};
    const modelsList = Array.from(new Set([
      ...(Array.isArray(existing?.modelsList) ? existing.modelsList : []),
      ...(Array.isArray(record?.modelsList) ? record.modelsList : []),
    ].map(item => String(item || '').trim()).filter(Boolean))).sort((left, right) => left.localeCompare(right));

    mergedMap.set(rowKey, {
      ...existing,
      ...record,
      rowKey,
      sourceType: 'auto',
      createdAt: existing?.createdAt || record?.createdAt || Date.now(),
      updatedAt: record?.updatedAt || Date.now(),
      modelsList,
      modelsText: modelsList.length ? modelsList.join(', ') : (record?.modelsText || existing?.modelsText || '未提供模型信息'),
      selectedModel: record?.selectedModel || existing?.selectedModel || '',
      quickTestStatus: existing?.quickTestStatus || '',
      quickTestLabel: existing?.quickTestLabel || '',
      quickTestModel: existing?.quickTestModel || '',
      quickTestRemark: existing?.quickTestRemark || '',
      quickTestAt: existing?.quickTestAt || null,
      quickTestResponseTime: existing?.quickTestResponseTime || '',
      quickTestTtftMs: existing?.quickTestTtftMs || '',
      quickTestTps: existing?.quickTestTps || '',
      quickTestResponseContent: existing?.quickTestResponseContent || '',
      balanceLabel: record?.balanceLabel || existing?.balanceLabel || '',
      balanceUpdatedAt: record?.balanceUpdatedAt || existing?.balanceUpdatedAt || null,
      balanceError: existing?.balanceError || '',
      remainQuota: record?.remainQuota ?? existing?.remainQuota ?? null,
      usedQuota: record?.usedQuota ?? existing?.usedQuota ?? null,
      unlimitedQuota: record?.unlimitedQuota === true || existing?.unlimitedQuota === true,
    });
  });

  return Array.from(mergedMap.values());
}

function promptKeySyncStrategy(existingCount, incomingCount) {
  if (existingCount <= 0) {
    return Promise.resolve('replace');
  }
  pendingKeySyncExistingCount.value = existingCount;
  pendingKeySyncIncomingCount.value = incomingCount;
  showKeySyncStrategyModal.value = true;
  return new Promise(resolve => {
    keySyncStrategyResolver = resolve;
  });
}

function resolveKeySyncStrategy(strategy = 'keep') {
  showKeySyncStrategyModal.value = false;
  const resolver = keySyncStrategyResolver;
  keySyncStrategyResolver = null;
  if (typeof resolver === 'function') {
    resolver(strategy);
  }
}

async function syncDetectedKeysToLocalStorage(options = {}) {
  const { silent = false } = options;
  const sourceResults = Array.isArray(testResults.value) ? testResults.value : [];
  const finishedResults = sourceResults.filter(task =>
    task &&
    task.siteUrl &&
    task.apiKey &&
    task.status &&
    !['pending', 'testing'].includes(task.status)
  );

  if (finishedResults.length === 0) {
    if (!silent) {
      message.warning('当前没有可同步的检测结果');
    }
    return false;
  }

  isSyncingLocalKeys.value = true;
  try {
    const grouped = new Map();
    const now = Date.now();
    finishedResults.forEach(task => {
      const siteUrl = String(task.siteUrl || '').replace(/\/+$/, '').trim();
      const apiKey = String(task.apiKey || '').trim();
      if (!siteUrl || !apiKey) return;
      const key = `${siteUrl}::${apiKey}`;
      if (!grouped.has(key)) {
        grouped.set(key, {
          rowKey: key,
          sourceType: 'auto',
          siteName: String(task.siteName || '未命名站点').trim() || '未命名站点',
          tokenName: '批量检测',
          siteUrl,
          apiKey,
          modelsSet: new Set(),
          statuses: [],
          createdAt: now,
          updatedAt: now,
          quickTestStatus: '',
          quickTestLabel: '',
          quickTestModel: '',
          quickTestRemark: '',
          quickTestAt: null,
          quickTestResponseTime: '',
          balanceLabel: '',
          balanceUpdatedAt: null,
        });
      }

      const record = grouped.get(key);
      record.siteName = record.siteName || String(task.siteName || '').trim() || '未命名站点';
      record.updatedAt = now;
      record.statuses.push(String(task.status || ''));
      if (task.modelName) {
        record.modelsSet.add(String(task.modelName).trim());
      }
      if (!record.balanceLabel && isDisplayableQuotaLabel(task.quota)) {
        record.balanceLabel = String(task.quota).trim();
        record.balanceUpdatedAt = now;
      }
    });

    const incomingRecords = Array.from(grouped.values()).map(record => {
      const modelsList = Array.from(record.modelsSet).filter(Boolean).sort();
      const status = record.statuses.some(item => item === 'success' || item === 'warning') ? 1 : 2;
      return {
        rowKey: record.rowKey,
        sourceType: 'auto',
        siteName: record.siteName,
        tokenName: record.tokenName,
        siteUrl: record.siteUrl,
        apiKey: record.apiKey,
        modelsList,
        modelsText: modelsList.length ? modelsList.join(', ') : '未提供模型信息',
        status,
        createdAt: record.createdAt,
        updatedAt: record.updatedAt,
        quickTestStatus: '',
        quickTestLabel: '',
        quickTestModel: '',
        quickTestRemark: '',
        quickTestAt: null,
        quickTestResponseTime: '',
        quickTestTtftMs: '',
        quickTestTps: '',
        balanceLabel: record.balanceLabel || '',
        balanceUpdatedAt: record.balanceUpdatedAt || null,
      };
    });

    const existingAutoRecords = loadStoredAutoKeyManagementRecords();
    const strategy = await promptKeySyncStrategy(existingAutoRecords.length, incomingRecords.length);
    if (strategy === 'keep') {
      if (!silent) {
        message.info('已保留当前密钥管理数据，本次检测结果未写入');
      }
      return false;
    }

    const records = strategy === 'merge'
      ? mergeDetectedKeyManagementRecords(existingAutoRecords, incomingRecords)
      : incomingRecords;

    localStorage.setItem(KEY_MANAGEMENT_STORAGE_KEY, JSON.stringify(records));
    localStorage.setItem(
      KEY_MANAGEMENT_META_STORAGE_KEY,
      JSON.stringify({
        lastBatchSyncAt: now,
        lastBatchSyncCount: incomingRecords.length,
        lastBatchFailedCount: records.filter(item => item.status !== 1).length,
        lastBatchSyncStrategy: strategy,
      })
    );
    window.dispatchEvent(new CustomEvent(KEY_MANAGEMENT_SYNC_EVENT, {
      detail: {
        recordsCount: records.length,
        syncedAt: now,
        strategy,
      },
    }));

    if (!silent) {
      const strategyLabel = strategy === 'merge' ? '增量更新' : '清空覆盖';
      message.success(`已按${strategyLabel}写入 ${incomingRecords.length} 条 sk 密钥`);
    }
    return true;
  } catch (error) {
    console.error('Sync detected keys failed', error);
    if (!silent) {
      message.error(`同步本地存储失败：${error.message || '未知错误'}`);
    }
    return false;
  } finally {
    isSyncingLocalKeys.value = false;
  }
}

const retestAllFromResults = async () => {
  if (testing.value) return;
  if (!testResults.value || testResults.value.length === 0) {
    message.warning('当前没有任务可测试');
    return;
  }

  const tasksQueue = testResults.value
    .map((task, index) => ({
      ...task,
      id: String(task?.id || `history_task_${index}`),
      siteId: String(task?.siteId || '').trim(),
      siteName: String(task?.siteName || '未命名站点').trim() || '未命名站点',
      siteUrl: String(task?.siteUrl || '').trim(),
      apiKey: String(task?.apiKey || '').trim(),
      modelName: String(task?.modelName || '').trim(),
      status: 'pending',
      statusText: '排队中',
      responseTime: '-',
      remark: '-',
    }))
    .filter(task => task.siteUrl && task.apiKey && task.modelName);

  if (tasksQueue.length === 0) {
    message.warning('历史结果缺少可重测的完整请求参数');
    return;
  }
  
  testResults.value = tasksQueue;
  organizedSourceResults.value = [...tasksQueue];
  
  testing.value = true;
  totalTasks.value = tasksQueue.length;
  completedTasks.value = 0;
  cancelTokens.value = [];
  scheduleOrganizedSourceRefresh(true);
  
  message.success('已重新加入队列开始测试！');
  
  let currentIndex = 0;
  const worker = async () => {
    while (currentIndex < tasksQueue.length && testing.value) {
      const taskIndex = currentIndex++;
      const task = tasksQueue[taskIndex];
      task.status = 'testing';
      task.statusText = '测试中';
      await runSingleTest(task);
      completedTasks.value++;
      scheduleOrganizedSourceRefresh();
    }
  };

  const workers = [];
  const actualConcurrency = Math.min(batchConcurrency.value, tasksQueue.length);
  for (let i = 0; i < actualConcurrency; i++) {
    workers.push(worker());
  }

  await Promise.all(workers);
  
  if (testing.value) {
    testing.value = false;
    scheduleOrganizedSourceRefresh(true);
    await syncDetectedKeysToLocalStorage({ silent: true });
    message.success('再次批量检测完成！');
    saveLastResultsSnapshot();
  }
};

const runSingleTest = async (task, customPayload = null) => {
  const apiUrlValue = customPayload ? customPayload.url.replace(/\/+$/, '') : task.siteUrl.replace(/\/+$/, '');
  const modelToTest = customPayload ? customPayload.model : task.modelName;
  const keyToUse = customPayload ? customPayload.key : task.apiKey;
  const messagesToUse = customPayload ? customPayload.messages : buildQuickTestMessages();

  let backendTimeoutMs = modelTimeout.value * 1000;
  if (modelToTest.startsWith('o1-')) {
    backendTimeoutMs *= 6;
  }
  const clientTimeoutMs = Math.max(backendTimeoutMs + 10000, 30000);

  const controller = new AbortController();
  cancelTokens.value.push(controller);
  
  const id = setTimeout(() => controller.abort(), clientTimeoutMs);
  const startTime = Date.now();

  try {
    const isFirst = task.id === 'task_0';
      const payloadBody = {
        url: apiUrlValue,
        key: keyToUse,
        model: modelToTest,
        messages: messagesToUse,
        timeoutMs: backendTimeoutMs,
        _isFirst: isFirst
      };
    
    // 如果是编辑模式重试，同步更新一下任务的属性以便UI显示最新值 (可选，看是否需要覆盖原来的)
    if (customPayload) {
      task.modelName = modelToTest;
      task.apiKey = keyToUse;
      task.siteUrl = customPayload.url;
    }

    const response = await apiFetch('/api/check-key', {
      method: 'POST',
      headers: {
        'Content-Type': 'application/json',
      },
      body: JSON.stringify(payloadBody),
      signal: controller.signal,
    });

    const endTime = Date.now();
    const responseTime = ((endTime - startTime) / 1000).toFixed(2);
    task.responseTime = responseTime;

    if (response.ok) {
      let data = await response.json();
      
      // 有些接口返回不是标准的 JSON 格式，可能带有 htmlSnippet。
      // 我们尝试从中深度提取 JSON，增强解析鲁棒性 (处理 SSE 格式的 data: 前缀)
      if (data && data.htmlSnippet) {
        let snippet = String(data.htmlSnippet).trim();
        if (snippet.startsWith('data:')) {
          snippet = snippet.replace(/^data:\s*/, '').trim();
        }
        if (snippet.startsWith('{') || snippet.startsWith('[')) {
          try { data = JSON.parse(snippet); } catch (e) {}
        }
      }

      const returnedModel = data.model || 'unknown';
      const msgObj = data.choices && data.choices[0]?.message;
      
      // 增强兼容性判定：思维链模型可能使用 reasoning_content
      const hasContent = msgObj && (msgObj.content || msgObj.reasoning_content || msgObj.thinking);
      const isReasoning = msgObj && (msgObj.reasoning_content || msgObj.thinking);
      const isStreamAssembled = data.isStreamAssembled;
      const performance = derivePerformanceMetricsFromResponse(data, responseTime);

      let suffixHtml = '';
      let suffixPlain = '';
      if (isReasoning) {
        suffixHtml = ' <span style="color:#52c41a; font-weight:500; font-size:12px;">(thinking)</span>';
        suffixPlain = ' (thinking)';
      } else if (isStreamAssembled) {
        suffixHtml = ' <span style="color:#52c41a; font-weight:500; font-size:12px;">(strict SSE)</span>';
        suffixPlain = ' (strict SSE)';
      }
      
      task.modelSuffix = suffixPlain;
      task.displaySuffixHtml = suffixHtml;
      task.ttftMs = performance.ttftMs;
      task.tps = performance.tps;
      
      // 保存原始响应
      task.fullResponse = JSON.stringify(data, null, 2);

      if (returnedModel.toLowerCase().includes(task.modelName.toLowerCase()) || task.modelName === 'unknown') {
        task.status = 'success';
        task.statusText = '一致可用';
        task.remark = hasContent ? (msgObj?.content ? '通过' : '思维链模型通过') : '响应成功结构异常';
        if (!hasContent) {
           task.status = 'warning';
        }
      } else {
        task.status = 'warning';
        if (returnedModel === 'unknown') {
          task.statusText = '模型未知';
          task.remark = hasContent ? '✅ 响应成功但未返回模型标识' : '❌ 响应为空且模型未知';
          if (!hasContent) task.status = 'error';
        } else {
          task.statusText = '模型重定向';
          task.remark = `映射由平台处理 -> ${returnedModel}`;
        }
      }
    } else {
      let errText = '';
      let rawData = null;
      try {
        const contentType = response.headers.get('content-type') || '';
        if (contentType.includes('application/json')) {
           rawData = await response.json();
        } else {
           const text = await response.text();
           const titleMatch = text.match(/<title>(.*?)<\/title>/i);
           rawData = { 
             htmlTitle: titleMatch ? titleMatch[1] : 'HTML Payload',
             htmlSnippet: text.substring(0, 500).replace(/<[^>]*>/g, ' ').trim()
           };
        }

        if (rawData.htmlTitle) {
          errText = `(HTML) ${rawData.htmlTitle}`;
        } else {
          errText = toReadableError(rawData, '请求失败');
        }
        task.fullResponse = rawData.htmlSnippet
          ? `HTML 内容摘要: ${rawData.htmlSnippet}\n\n完整响应: ${JSON.stringify(rawData, null, 2)}`
          : JSON.stringify(rawData, null, 2);
      } catch (e) {
        errText = `HTTP ${response.status}`;
        task.fullResponse = `Error: ${errText}`;
      }
      task.status = 'error';
      task.statusText = toStatusTextByError(errText);
      task.remark = truncateText(errText, 200);
      task.ttftMs = '';
      task.tps = '';
    }
  } catch (err) {
    task.status = 'error';
    task.statusText = toStatusTextByError(err?.message || '');
    task.ttftMs = '';
    task.tps = '';
    if (err.name === 'AbortError') {
      task.remark = `前端等待超时 (${Math.round(clientTimeoutMs / 1000)}s)`;
      task.fullResponse = JSON.stringify({
        error: 'client_abort',
        message: task.remark,
        siteUrl: apiUrlValue,
        model: modelToTest,
        backendTimeoutMs,
        clientTimeoutMs,
      }, null, 2);
    } else {
      task.remark = truncateText(err.message, 200);
      task.fullResponse = JSON.stringify({
        error: err?.name || 'request_failed',
        message: err?.message || 'unknown_error',
        siteUrl: apiUrlValue,
        model: modelToTest,
        backendTimeoutMs,
        clientTimeoutMs,
      }, null, 2);
    }
  } finally {
    clearTimeout(id);
    const cIdx = cancelTokens.value.indexOf(controller);
    if (cIdx > -1) cancelTokens.value.splice(cIdx, 1);
    scheduleOrganizedSourceRefresh();
  }
};


const getStatusColor = (status) => {
  switch (status) {
    case 'success': return 'green';
    case 'warning': return 'orange';
    case 'error': return 'red';
    case 'testing': return 'blue';
    case 'pending': return 'default';
    default: return 'default';
  }
};

const copyAllConfigs = () => {
  const validTasks = testResults.value.filter(t => t.status === 'success' || t.status === 'warning');
  if (validTasks.length === 0) {
    message.warning('没有可用的配置组合！');
    return;
  }
  
  const siteMap = new Map();
  validTasks.forEach(task => {
    const key = `${task.siteUrl}|${task.apiKey}`;
    if (!siteMap.has(key)) {
      siteMap.set(key, { name: task.siteName, url: task.siteUrl, key: task.apiKey, models: [] });
    }
    siteMap.get(key).models.push(task.modelName);
  });
  
  const text = Array.from(siteMap.values()).map(s => 
    `====================\n平台名称: ${s.name}\n接口地址: ${s.url}\nAPI 密钥: ${s.key}\n可用模型: ${s.models.join(',')}\n`
  ).join('\n');

  navigator.clipboard.writeText(text).then(() => {
    message.success(`已复制全表 ${siteMap.size} 个站点的有效配置`);
  });
};

const copyOrganizedResults = () => {
  const tree = organizedTreeData.value;
  if (tree.length === 0) {
    message.warning('当前视图没有可复制的配置');
    return;
  }

  const text = tree.map(group => {
    const validModels = group.children
      .filter(c => c.class === 'status-success' || c.class === 'status-warning')
      .map(c => c.title.split(' - ')[0]);
    
    if (validModels.length === 0) return null;

    const [siteName, apiKeyTail] = group.key.split('|'); 
    // Find the original full task to get the correct site URL
    const originalTask = testResults.value.find(t => t.siteName === siteName && t.apiKey === apiKeyTail);
    const url = originalTask ? originalTask.siteUrl : 'unknown';

    return `====================\n平台名称: ${siteName}\n接口地址: ${url}\nAPI 密钥: ${apiKeyTail}\n可用模型: ${validModels.join(',')}\n`;
  }).filter(t => t).join('\n');

  if (!text) {
    message.warning('当前筛选出的站点中没有有效的模型配置');
    return;
  }

  navigator.clipboard.writeText(text).then(() => {
    message.success(`已按当前过滤视图复制配置信息`);
  });
};

</script>

<style scoped>
/* Header & Navigation Style */
.loading-status-card {
  width: min(560px, 92vw);
  margin-top: 18px;
  padding: 18px 20px;
  border-radius: 14px;
  background: rgba(255, 255, 255, 0.92);
  box-shadow: 0 12px 30px rgba(15, 23, 42, 0.08);
}

.loading-status-title {
  font-size: 18px;
  font-weight: 700;
  color: #1f2937;
}

.loading-status-description {
  margin: 0;
  font-size: 14px;
  color: #334155;
}

.loading-status-meta {
  margin: 6px 0 0;
  font-size: 12px;
  color: #64748b;
}

.selection-topbar {
  display: flex;
  flex-direction: column;
  gap: 14px;
  margin-bottom: 18px;
}

.selection-header-row {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 16px;
  flex-wrap: wrap;
}

.selection-title {
  margin: 0;
  font-size: 28px;
  font-weight: 700;
  line-height: 1.2;
  white-space: nowrap;
  color: #ffffff;
}

.batch-settings-label {
  font-size: 12px;
  font-weight: 600;
  letter-spacing: 0.02em;
  color: #5d6d57;
  white-space: nowrap;
}

.settings-action-bar {
  display: flex;
  justify-content: space-between;
  align-items: center;
  gap: 14px;
  flex-wrap: wrap;
  margin-top: 18px;
  padding: 14px 16px;
  border: 1px solid rgba(116, 144, 104, 0.16);
  border-radius: 18px;
  background: linear-gradient(180deg, rgba(255, 255, 255, 0.94), rgba(246, 250, 244, 0.88));
  box-shadow: 0 14px 34px rgba(90, 117, 79, 0.08);
  backdrop-filter: blur(14px);
}

.batch-settings {
  display: flex;
  align-items: center;
  flex-wrap: wrap;
  gap: 10px 12px;
  min-width: 0;
}

.batch-setting-input {
  width: 92px;
  min-width: 92px;
  height: 38px;
  border-radius: 12px;
  border: 1px solid rgba(120, 142, 109, 0.18);
  background: rgba(255, 255, 255, 0.92);
  box-shadow: inset 0 1px 0 rgba(255, 255, 255, 0.72);
  overflow: hidden;
}

.batch-setting-input :deep(.ant-input-number-input) {
  height: 36px;
  padding-left: 12px;
  padding-right: 12px;
  font-size: 13px;
  color: #314032;
}

.batch-setting-input :deep(.ant-input-number-handler-wrap) {
  border-radius: 0 12px 12px 0;
  opacity: 0.92;
}

.actions {
  display: inline-flex;
  align-items: center;
  gap: 10px;
  margin-left: auto;
  flex-wrap: wrap;
  min-width: 0;
}

.batch-reset-button,
.batch-start-button {
  height: 38px;
  border-radius: 999px;
  font-weight: 600;
  padding: 0 16px;
  transition: transform 0.18s ease, box-shadow 0.18s ease, background 0.18s ease, border-color 0.18s ease;
}

.batch-reset-button {
  border-color: rgba(116, 144, 104, 0.18) !important;
  color: #4a5b46 !important;
  background: rgba(255, 255, 255, 0.8) !important;
}

.batch-reset-button:hover,
.batch-start-button:hover {
  transform: translateY(-1px);
}

.batch-start-button {
  min-width: 132px;
  border: 0 !important;
  background: linear-gradient(135deg, #4f6e49, #7b9a5d) !important;
  box-shadow: 0 10px 20px rgba(87, 118, 76, 0.2) !important;
}

.selection-quick-filters {
  width: 100%;
  min-width: 0;
}

.selection-action-group {
  display: flex;
  justify-content: flex-end;
  flex-wrap: wrap;
  margin-left: auto;
}

.result-topbar {
  display: grid;
  grid-template-columns: minmax(0, 1fr) 380px;
  align-items: start;
  gap: 20px;
  margin-bottom: 12px;
}

.result-side-controls {
  width: 100%;
  min-width: 0;
  display: flex;
  flex-direction: column;
  gap: 12px;
}

.result-action-group {
  justify-content: flex-start;
}

.quick-filter-toolbar {
  display: flex;
  align-items: flex-start;
  flex-direction: column;
  gap: 12px;
  min-height: 32px;
  min-width: 0;
  width: 100%;
}

.quick-filter-strip {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(132px, 1fr));
  gap: 0;
  width: 100%;
  max-width: 100%;
  border: 1px solid rgba(15, 23, 42, 0.12);
  border-radius: 12px;
  overflow: hidden;
  background: rgba(255, 255, 255, 0.92);
  box-shadow: 0 10px 24px rgba(15, 23, 42, 0.06);
}

.quick-filter-strip > :not(.quick-filter-clear-trigger) {
  min-width: 0;
}

.quick-filter-empty-inline {
  color: #94a3b8;
  font-size: 13px;
  padding: 6px 0;
}

.quick-filter-family-trigger,
.quick-filter-clear-trigger {
  width: 100%;
  border: 0 !important;
  border-right: 1px solid rgba(15, 23, 42, 0.08) !important;
  border-bottom: 1px solid rgba(15, 23, 42, 0.08) !important;
  border-radius: 0 !important;
  box-shadow: none !important;
  height: 40px;
  justify-content: center;
  padding: 0 20px !important;
}

.quick-filter-family-trigger:hover,
.quick-filter-clear-trigger:hover {
  background: rgba(22, 119, 255, 0.06) !important;
}

.quick-filter-clear-trigger.ant-btn[disabled],
.quick-filter-clear-trigger.ant-btn[disabled]:hover {
  background: rgba(148, 163, 184, 0.08) !important;
  color: rgba(148, 163, 184, 0.9) !important;
}

.quick-filter-family-count {
  margin-left: 6px;
  font-size: 11px;
  opacity: 0.75;
}

.quick-filter-summary {
  color: #64748b;
  font-size: 12px;
  line-height: 1.5;
}

.quick-filter-family-panel {
  width: min(420px, 56vw);
  max-width: 420px;
}

.quick-filter-family-panel-title {
  margin-bottom: 8px;
  font-size: 13px;
  font-weight: 700;
  color: #334155;
}

.quick-filter-option-list {
  display: flex;
  flex-wrap: wrap;
  gap: 8px;
}

.quick-filter-family-select-all {
  border: 2px solid #8b5e3c !important;
  color: #8b5e3c !important;
  background: #fffaf4 !important;
  box-shadow: none !important;
  font-weight: 600;
}

.extension-import-status-line {
  margin-top: 8px;
  min-height: 18px;
  text-align: left;
}

.loading-stage-status-line {
  margin-top: 8px;
  min-height: 18px;
}

.batch-hero {
  position: relative;
  overflow: hidden;
  margin-bottom: 6px;
  padding: 20px 16px;
  border-radius: 18px;
  border: 1px solid rgba(90, 117, 79, 0.1);
  background:
    radial-gradient(circle at top right, rgba(255, 231, 161, 0.38), transparent 36%),
    radial-gradient(circle at left center, rgba(204, 228, 184, 0.34), transparent 32%),
    linear-gradient(145deg, rgba(255, 252, 244, 0.95), rgba(244, 249, 236, 0.9));
  box-shadow:
    0 36px 90px rgba(98, 119, 84, 0.12),
    inset 0 1px 0 rgba(255, 255, 255, 0.84);
}

.batch-hero-compact {
  padding-bottom: 16px;
}

.batch-hero-head {
  position: relative;
  z-index: 1;
}

.batch-hero-copy {
  max-width: 100%;
}

.batch-hero-kicker {
  margin: 0 0 4px;
  color: #8a936f;
  font-size: 8px;
  letter-spacing: 0.14em;
  text-transform: uppercase;
}

.batch-hero-meta {
  display: flex;
  flex-wrap: wrap;
  gap: 5px;
  margin-top: 6px;
}

.batch-hero-tag {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  padding: 4px 8px;
  border-radius: 999px;
  background: rgba(255, 255, 255, 0.55);
  border: 1px solid rgba(90, 117, 79, 0.08);
  color: #6e7c64;
  font-size: 9px;
  font-weight: 600;
}

.page-title-row {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 8px;
  margin-bottom: 0;
  flex-wrap: wrap;
}

.page-title-block {
  display: flex;
  flex-direction: column;
  gap: 3px;
}

.page-title {
  margin: 0;
  text-align: left;
  color: #31422f;
  font: 700 clamp(20px, 2.4vw, 30px)/1 Georgia, 'Times New Roman', serif;
  letter-spacing: -0.03em;
}

.page-subtitle {
  margin: 0;
  color: #72806c;
  font-size: 10px;
  line-height: 1.2;
}

.backend-health-pill {
  display: inline-flex;
  align-items: center;
  gap: 6px;
  padding: 6px 10px;
  border-radius: 999px;
  border: 1px solid rgba(90, 117, 79, 0.1);
  background: rgba(255, 255, 255, 0.72);
  color: #405240;
  font-size: 10px;
  cursor: help;
  box-shadow: 0 10px 22px rgba(98, 119, 84, 0.08);
}

.backend-health-dot {
  width: 8px;
  height: 8px;
  border-radius: 999px;
  background: #faad14;
  box-shadow: 0 0 0 4px rgba(250, 173, 20, 0.16);
}

.backend-health-ok .backend-health-dot {
  background: #52c41a;
  box-shadow: 0 0 0 4px rgba(82, 196, 26, 0.16);
}

.backend-health-down .backend-health-dot {
  background: #ff4d4f;
  box-shadow: 0 0 0 4px rgba(255, 77, 79, 0.16);
}

.backend-health-label {
  font-weight: 600;
}

.batch-wrapper {
  min-height: calc(var(--vh, 1vh) * 100);
  padding: 0;
  overflow: hidden;
}
/* 覆盖 global.css 里 .container 的 max-width: 800px 限制 */
.batch-page-container {
  max-width: 100% !important;
  padding: 8px 8px 0 !important;
  margin: 0 auto !important;
}

.batch-shell {
  width: 100%;
  min-height: calc(var(--vh, 1vh) * 100);
  position: relative;
  isolation: isolate;
  overflow: hidden;
}

.batch-forest-scene {
  position: absolute;
  inset: 0;
  overflow: hidden;
  pointer-events: none;
  z-index: 0;
  background:
    radial-gradient(circle at 16% 18%, rgba(164, 213, 120, 0.14), transparent 24%),
    radial-gradient(circle at 84% 14%, rgba(255, 213, 116, 0.14), transparent 22%),
    linear-gradient(180deg, rgba(8, 18, 12, 0.14) 0%, rgba(8, 20, 13, 0.34) 42%, rgba(6, 16, 10, 0.62) 100%),
    url('/forest-batch-bg-v2.png') center center / cover no-repeat;
  opacity: 0.92;
  animation: none;
}

.batch-forest-scene > * {
  display: block;
}

.forest-mist,
.forest-path-glow,
.forest-firegrass,
.forest-slime {
  position: absolute;
}

.forest-mist {
  top: 8%;
  width: 34%;
  height: 44%;
  border-radius: 999px;
  background: radial-gradient(circle, rgba(210, 255, 232, 0.12) 0%, rgba(210, 255, 232, 0.02) 56%, transparent 74%);
  filter: blur(12px);
  animation: none;
}

.forest-mist-left {
  left: -10%;
}

.forest-mist-right {
  right: -8%;
  top: 12%;
  animation-delay: -8s;
}

.forest-path-glow {
  left: 50%;
  bottom: -12%;
  width: min(460px, 42vw);
  height: 42%;
  transform: translateX(-50%);
  background:
    radial-gradient(ellipse at center bottom, rgba(255, 214, 126, 0.22) 0%, rgba(212, 255, 182, 0.12) 24%, rgba(30, 58, 33, 0) 72%);
  clip-path: polygon(47% 100%, 53% 100%, 65% 76%, 60% 56%, 67% 33%, 57% 0, 43% 0, 33% 33%, 40% 56%, 35% 76%);
  filter: blur(8px);
  opacity: 0.9;
}

.forest-firegrass {
  bottom: -4px;
  width: 188px;
  height: 122px;
  background: url('/forest-firegrass-sprite-v2.png') left bottom / auto 100% no-repeat;
  filter: drop-shadow(0 6px 12px rgba(18, 38, 22, 0.2));
  opacity: 0.98;
  animation: none;
}

.firegrass-left {
  left: 8px;
}

.firegrass-right {
  right: 8px;
  transform: scaleX(-1);
  transform-origin: center bottom;
  animation-delay: 0s, -2.8s;
}

.forest-slime {
  bottom: 26px;
  width: 26px;
  height: 22px;
  border-radius: 58% 58% 46% 46%;
  background:
    radial-gradient(circle at 36% 36%, rgba(255,255,255,0.9) 0 10%, transparent 11%),
    radial-gradient(circle at 64% 36%, rgba(255,255,255,0.9) 0 10%, transparent 11%),
    radial-gradient(circle at 40% 40%, rgba(20,34,21,0.86) 0 3%, transparent 4%),
    radial-gradient(circle at 60% 40%, rgba(20,34,21,0.86) 0 3%, transparent 4%),
    radial-gradient(circle at 50% 72%, rgba(18,72,42,0.44) 0 14%, transparent 15%),
    linear-gradient(180deg, rgba(177, 255, 149, 0.98), rgba(70, 177, 88, 0.94));
  box-shadow:
    inset 0 2px 0 rgba(255,255,255,0.45),
    0 10px 16px rgba(14, 38, 18, 0.24),
    0 0 10px rgba(154, 255, 142, 0.18);
}

.forest-slime::after {
  content: '';
  position: absolute;
  left: 50%;
  bottom: -4px;
  width: 18px;
  height: 7px;
  transform: translateX(-50%);
  border-radius: 999px;
  background: rgba(28, 48, 30, 0.26);
  filter: blur(2px);
}

.slime-a {
  left: 44%;
  animation: none;
}

.slime-b {
  left: 51%;
  width: 20px;
  height: 17px;
  animation: none;
}

.slime-c {
  left: 57%;
  width: 18px;
  height: 15px;
  animation: none;
}

.batch-page-content {
  background: transparent;
  border-radius: 0;
  box-shadow: none;
  padding: 2px;
  min-height: calc(var(--vh, 1vh) * 100);
  position: relative;
  z-index: 1;
}

.step-container {
  margin-top: 6px;
}

.step-container-hero {
  position: relative;
  z-index: 1;
  margin-top: 15px;
}

.hero-stage-grid {
  display: grid;
  grid-template-columns: minmax(0, 1.34fr) minmax(220px, 0.58fr);
  gap: 6px;
  align-items: stretch;
}

.hero-left-stack {
  display: grid;
  grid-template-rows: auto minmax(0, 1fr);
  gap: 6px;
  align-content: stretch;
  min-height: 0;
  height: 100%;
}

.hero-primary-pair {
  display: grid;
  grid-template-columns: minmax(0, 1.92fr) minmax(220px, 1fr);
  gap: 6px;
  align-items: stretch;
}

.hero-action-card,
.hero-upload-card {
  position: relative;
  padding: 9px 10px;
  border-radius: 16px;
  border: 1px solid rgba(90, 117, 79, 0.08);
  background: rgba(255, 255, 255, 0.56);
  box-shadow:
    0 8px 18px rgba(98, 119, 84, 0.08),
    inset 0 1px 0 rgba(255, 255, 255, 0.72);
}

.hero-action-card-primary {
  background:
    linear-gradient(145deg, rgba(235, 244, 212, 0.96), rgba(222, 236, 196, 0.88));
}

.hero-action-card-large {
  min-height: 104px;
  display: flex;
  flex-direction: column;
  justify-content: space-between;
}

.hero-action-card-secondary {
  background:
    linear-gradient(145deg, rgba(255, 253, 248, 0.92), rgba(246, 249, 238, 0.86));
  display: flex;
  flex-direction: column;
  justify-content: space-between;
  min-height: 100%;
}

.hero-action-card-bridge {
  background:
    linear-gradient(112deg, rgba(255, 255, 255, 0.24) 0%, rgba(255, 255, 255, 0.08) 14%, rgba(255, 255, 255, 0) 30%),
    radial-gradient(circle at 18% 20%, rgba(255, 248, 220, 0.84), transparent 24%),
    radial-gradient(circle at 84% 18%, rgba(250, 222, 150, 0.34), transparent 22%),
    linear-gradient(145deg, rgba(249, 228, 166, 0.97), rgba(242, 206, 113, 0.95) 54%, rgba(233, 192, 92, 0.93) 100%);
  border-color: rgba(194, 151, 52, 0.22);
  box-shadow:
    0 10px 24px rgba(179, 138, 34, 0.14),
    0 0 0 1px rgba(255, 245, 205, 0.44) inset,
    inset 0 1px 0 rgba(255, 255, 255, 0.74),
    inset 0 -8px 14px rgba(176, 126, 18, 0.05);
}

.hero-action-card-compact {
  min-height: 48px;
}

.hero-primary-pair .hero-action-card-primary,
.hero-primary-pair .hero-action-card-bridge {
  min-height: 128px;
}

.hero-primary-pair .hero-action-card-primary {
  display: flex;
  flex-direction: column;
  justify-content: space-between;
}

.hero-action-card-recommend {
  padding-bottom: 10px;
}

.hero-card-watermark {
  position: absolute;
  top: 50%;
  right: 14px;
  transform: translateY(-50%);
  font-size: 11px;
  line-height: 1;
  font-weight: 800;
  letter-spacing: 0.22em;
  color: rgba(135, 92, 6, 0.3);
  pointer-events: none;
  user-select: none;
}

.hero-action-copy,
.hero-upload-copy {
  display: flex;
  flex-direction: column;
  gap: 0;
  margin-bottom: 4px;
}

.hero-action-card h3,
.hero-upload-copy h3 {
  margin: 0;
  color: #314230;
  font: 700 13px/1.12 Georgia, 'Times New Roman', serif;
}

.hero-action-card p,
.hero-upload-copy p,
.hero-action-note {
  margin: 0;
  color: #697766;
  font-size: 10px;
  line-height: 1.2;
}

.hero-copy-half-gap {
  margin-top: 5px !important;
}

.hero-primary-inline {
  display: flex;
  flex-direction: column;
  align-items: flex-start;
  gap: 4px;
}

.hero-primary-button {
  min-width: 176px;
  height: 34px;
  border-radius: 999px;
  border: 0 !important;
  background: linear-gradient(135deg, #476847, #6f8f55) !important;
  box-shadow: 0 8px 16px rgba(87, 118, 76, 0.2) !important;
}

.hero-primary-pair .hero-primary-button {
  min-width: 0;
  width: 100%;
  justify-content: center;
}

.hero-secondary-button {
  min-height: 30px;
  border-radius: 999px;
  border-color: rgba(90, 117, 79, 0.18) !important;
  color: #405240 !important;
  background: rgba(255, 255, 255, 0.78) !important;
}

.hero-bridge-button {
  border-color: rgba(146, 108, 18, 0.2) !important;
  color: #6d4f0e !important;
  background: rgba(255, 250, 236, 0.82) !important;
}

.hero-upload-card {
  display: flex;
  flex-direction: column;
  justify-content: center;
  min-height: 156px;
  padding: 9px 10px;
}

.hero-upload-card-right {
  display: grid;
  grid-template-rows: auto minmax(0, 1fr);
  align-self: start;
  min-height: 198px;
  height: auto;
}

.hero-upload-dragger {
  margin-top: 2px;
  min-height: 0;
}

.hero-upload-dragger :deep(.ant-upload.ant-upload-drag) {
  border-radius: 12px;
  border: 1px dashed rgba(90, 117, 79, 0.24);
  background: rgba(255, 255, 255, 0.78);
  min-height: 96px;
  padding: 10px 8px;
}

.hero-upload-card-right .hero-upload-dragger :deep(.ant-upload.ant-upload-drag) {
  min-height: 128px;
  height: auto;
}

@media (max-width: 1700px) {
  .hero-upload-card-right {
    align-self: stretch;
    min-height: 0;
    height: 100%;
    padding: 9px 10px;
  }

  .hero-upload-card-right .hero-upload-copy {
    margin-bottom: 4px;
  }

  .hero-upload-card-right .hero-upload-copy h3 {
    font-size: 13px;
    line-height: 1.12;
  }

  .hero-upload-card-right .hero-upload-copy p {
    font-size: 10px;
    line-height: 1.2;
  }

  .hero-upload-card-right .hero-upload-dragger {
    display: flex;
    min-height: 0;
  }

  .hero-upload-card-right .hero-upload-dragger :deep(.ant-upload.ant-upload-drag) {
    display: flex;
    flex: 1;
    flex-direction: column;
    justify-content: center;
    min-height: 0;
    height: 100%;
    padding: 10px 8px;
  }

  .hero-upload-card-right .hero-upload-dragger :deep(.ant-upload-drag-icon) {
    margin-bottom: 10px;
  }

  .hero-upload-card-right .hero-upload-dragger :deep(.ant-upload-drag-icon .anticon) {
    font-size: 66px;
  }

  .hero-upload-card-right .hero-upload-dragger :deep(.ant-upload-text) {
    font-size: 10px;
    line-height: 1.2;
  }

  .hero-upload-card-right .hero-upload-dragger :deep(.ant-upload-hint) {
    font-size: 9px;
    line-height: 1.14;
  }
}

.hero-upload-dragger :deep(.ant-upload.ant-upload-drag:hover) {
  border-color: rgba(90, 117, 79, 0.42);
}

.hero-upload-dragger :deep(.ant-upload-text) {
  color: #314230;
  font-weight: 700;
  font-size: 10px;
}

.hero-upload-dragger :deep(.ant-upload-hint) {
  color: #7b8776;
  font-size: 9px;
}

.loading-container {
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  padding: 28px 0;
  border-radius: 30px;
  background: rgba(255, 255, 255, 0.62);
  border: 1px solid rgba(90, 117, 79, 0.08);
}
.tree-wrapper {
  background: rgba(255, 255, 255, 0.72);
  border: 1px solid rgba(90, 117, 79, 0.12);
  border-radius: 20px;
  padding: 14px;
  margin-bottom: 20px;
  max-height: 420px;
  overflow-y: auto;
  box-shadow: 0 16px 36px rgba(98, 119, 84, 0.08);
  contain: layout paint;
}
.tree-wrapper :deep(.site-root-summary-node > .ant-tree-switcher),
.tree-wrapper :deep(.site-root-summary-node > .ant-tree-checkbox) {
  display: none;
}
.result-container {
  border: 1px solid rgba(90, 117, 79, 0.12);
  border-radius: 24px;
  padding: 18px;
  background-color: rgba(255, 255, 255, 0.74);
  box-shadow: 0 20px 48px rgba(98, 119, 84, 0.08);
  contain: layout paint;
}

/* Organized Tree Styles */
.organized-tree-wrapper {
  background: rgba(255, 255, 255, 0.68);
  border: 1px solid rgba(90, 117, 79, 0.12);
  border-radius: 20px;
  padding: 12px;
  max-height: 500px;
  overflow-y: auto;
  contain: layout paint;
}

.batch-hero-motion {
  position: absolute;
  inset: 0;
  pointer-events: none;
}

.leaf,
.grass {
  position: absolute;
  opacity: 0.42;
}

.leaf {
  width: 10px;
  height: 20px;
  border-radius: 70% 0 70% 0;
  background: linear-gradient(180deg, rgba(170, 202, 127, 0.7), rgba(96, 131, 75, 0.42));
  filter: blur(0.2px);
  transform-origin: center bottom;
  animation: none;
}

.leaf-a { top: 24%; right: 18%; animation-delay: 0s; }
.leaf-b { top: 42%; right: 8%; width: 9px; height: 18px; animation-delay: 1.4s; }
.leaf-c { bottom: 28%; left: 9%; width: 10px; height: 20px; animation-delay: 2.1s; }
.leaf-d { bottom: 18%; right: 28%; width: 8px; height: 14px; animation-delay: 3.2s; }

.grass {
  bottom: -10px;
  width: 2px;
  border-radius: 999px;
  background: linear-gradient(180deg, rgba(121, 157, 96, 0), rgba(121, 157, 96, 0.58));
  transform-origin: bottom center;
  animation: none;
}

.batch-shell-motion-active .batch-forest-scene {
  animation: forestBackdropShift 36s ease-in-out infinite;
}

.batch-shell-motion-active .forest-mist {
  animation: forestMistDrift 24s ease-in-out infinite;
}

.batch-shell-motion-active .forest-firegrass {
  animation: firegrassFrames 1.35s steps(8) infinite;
}

.batch-shell-motion-active .slime-a {
  animation: slimeHopA 7.4s ease-in-out infinite;
}

.batch-shell-motion-active .slime-b {
  animation: slimeHopB 8.2s ease-in-out infinite;
}

.batch-shell-motion-active .slime-c {
  animation: slimeHopC 9.2s ease-in-out infinite;
}

.batch-shell-motion-active .leaf {
  animation: leafFloat 10s ease-in-out infinite;
}

.batch-shell-motion-active .grass {
  animation: grassSway 7s ease-in-out infinite;
}

.grass-a { left: 8%; height: 38px; animation-delay: 0s; }
.grass-b { left: 11%; height: 30px; animation-delay: 1.2s; }
.grass-c { right: 12%; height: 34px; animation-delay: 2.4s; }

@keyframes leafFloat {
  0%, 100% { transform: translate3d(0, 0, 0) rotate(-8deg); }
  50% { transform: translate3d(0, -8px, 0) rotate(8deg); }
}

@keyframes grassSway {
  0%, 100% { transform: rotate(-7deg) scaleY(1); }
  50% { transform: rotate(7deg) scaleY(1.04); }
}

@keyframes firegrassFrames {
  from { background-position-x: 0; }
  to { background-position-x: -1504px; }
}

@keyframes firegrassDrift {
  0%, 100% { transform: translate3d(0, 0, 0); }
  50% { transform: translate3d(0, -2px, 0); }
}

@keyframes forestMistDrift {
  0%, 100% { transform: translate3d(0, 0, 0); opacity: 0.45; }
  50% { transform: translate3d(18px, -6px, 0); opacity: 0.7; }
}

@keyframes slimeHopA {
  0%, 100% { transform: translate3d(0, 0, 0) scale(1); }
  20% { transform: translate3d(8px, -3px, 0) scale(1.02, 0.94); }
  32% { transform: translate3d(22px, -14px, 0) scale(0.96, 1.06); }
  48% { transform: translate3d(34px, 0, 0) scale(1.02, 0.94); }
  68% { transform: translate3d(18px, -8px, 0) scale(0.98, 1.02); }
}

@keyframes slimeHopB {
  0%, 100% { transform: translate3d(0, 0, 0) scale(1); }
  18% { transform: translate3d(-6px, -2px, 0) scale(1.03, 0.92); }
  34% { transform: translate3d(-18px, -10px, 0) scale(0.95, 1.08); }
  52% { transform: translate3d(-28px, 0, 0) scale(1.02, 0.94); }
  72% { transform: translate3d(-16px, -6px, 0) scale(0.98, 1.02); }
}

@keyframes slimeHopC {
  0%, 100% { transform: translate3d(0, 0, 0) scale(1); }
  24% { transform: translate3d(5px, -2px, 0) scale(1.02, 0.94); }
  38% { transform: translate3d(14px, -8px, 0) scale(0.96, 1.08); }
  56% { transform: translate3d(22px, 0, 0) scale(1.02, 0.96); }
  76% { transform: translate3d(12px, -5px, 0) scale(0.98, 1.02); }
}

@keyframes forestBackdropShift {
  0%, 100% { background-position: center center, center center, center center, center center; }
  50% { background-position: 48% 50%, 52% 50%, center center, 50.8% 49.4%; }
}

:deep(body.dark-mode) .batch-hero {
  border-color: rgba(160, 189, 144, 0.12);
  background:
    radial-gradient(circle at top right, rgba(179, 147, 67, 0.24), transparent 34%),
    radial-gradient(circle at left center, rgba(104, 149, 88, 0.2), transparent 34%),
    linear-gradient(145deg, rgba(24, 38, 27, 0.95), rgba(35, 53, 39, 0.92));
  box-shadow:
    0 34px 90px rgba(0, 0, 0, 0.28),
    inset 0 1px 0 rgba(255, 255, 255, 0.04);
}

:deep(body.dark-mode) .batch-forest-scene {
  background:
    radial-gradient(circle at 18% 18%, rgba(92, 161, 113, 0.14), transparent 24%),
    radial-gradient(circle at 82% 15%, rgba(255, 206, 104, 0.1), transparent 22%),
    linear-gradient(180deg, rgba(4, 10, 7, 0.42) 0%, rgba(4, 10, 7, 0.62) 42%, rgba(2, 6, 4, 0.86) 100%),
    url('/forest-batch-bg-v2.png') center center / cover no-repeat;
}

:deep(body.dark-mode) .page-title,
:deep(body.dark-mode) .hero-action-card h3,
:deep(body.dark-mode) .hero-upload-copy h3 {
  color: #eef5e6;
}

:deep(body.dark-mode) .page-subtitle,
:deep(body.dark-mode) .hero-action-card p,
:deep(body.dark-mode) .hero-upload-copy p,
:deep(body.dark-mode) .hero-action-note,
:deep(body.dark-mode) .backend-health-pill,
:deep(body.dark-mode) .batch-hero-tag {
  color: #b8c8b2;
}

:deep(body.dark-mode) .batch-hero-tag,
:deep(body.dark-mode) .hero-action-card,
:deep(body.dark-mode) .hero-upload-card,
:deep(body.dark-mode) .loading-container,
:deep(body.dark-mode) .tree-wrapper,
:deep(body.dark-mode) .result-container,
:deep(body.dark-mode) .organized-tree-wrapper {
  background: rgba(255, 255, 255, 0.05);
  border-color: rgba(160, 189, 144, 0.12);
}

:deep(body.dark-mode) .hero-action-card-primary {
  background:
    linear-gradient(145deg, rgba(74, 102, 64, 0.44), rgba(53, 76, 48, 0.4));
}

:deep(body.dark-mode) .hero-action-card-secondary {
  background:
    linear-gradient(145deg, rgba(255, 255, 255, 0.06), rgba(160, 189, 144, 0.06));
}

:deep(body.dark-mode) .hero-action-card-bridge {
  background:
    linear-gradient(112deg, rgba(255, 244, 202, 0.08) 0%, rgba(255, 244, 202, 0.02) 18%, rgba(255, 244, 202, 0) 34%),
    radial-gradient(circle at 18% 22%, rgba(255, 214, 112, 0.1), transparent 22%),
    linear-gradient(145deg, rgba(120, 86, 16, 0.88), rgba(154, 109, 25, 0.84));
  border-color: rgba(232, 197, 111, 0.2);
  box-shadow:
    0 10px 22px rgba(0, 0, 0, 0.18),
    0 0 0 1px rgba(255, 222, 142, 0.06) inset,
    inset 0 1px 0 rgba(255, 242, 208, 0.05);
}

:deep(body.dark-mode) .hero-card-watermark {
  color: rgba(255, 236, 184, 0.28);
}

:deep(body.dark-mode) .hero-upload-dragger :deep(.ant-upload.ant-upload-drag) {
  background: rgba(255, 255, 255, 0.04);
  border-color: rgba(160, 189, 144, 0.2);
}

:deep(body.dark-mode) .hero-upload-dragger :deep(.ant-upload-text) {
  color: #eef5e6;
}

:deep(body.dark-mode) .hero-upload-dragger :deep(.ant-upload-hint) {
  color: #aab7a6;
}

.custom-tree-node {
  font-size: 14px;
}

.tree-node-green { color: #52c41a; font-weight: bold; }
.tree-node-orange { color: #faad14; font-weight: bold; }
.tree-node-grey { color: #999; opacity: 0.7; }
.tree-node-pending-hint {
  display: inline-flex;
  align-items: center;
  gap: 6px;
  margin-left: 10px;
  color: #1677ff;
  font-size: 12px;
}

.status-success { color: #52c41a; }
.status-warning { color: #faad14; }
.status-error { color: #ff4d4f; }

:deep(.result-summary-tree .ant-tree-node-content-wrapper) {
  width: 100%;
}

:deep(.highlighted-row) {
  background-color: rgba(24, 144, 255, 0.15) !important;
  transition: background-color 0.5s;
}

:deep(.dark-mode .highlighted-row) {
  background-color: rgba(24, 144, 255, 0.3) !important;
}
.custom-tree-node-wrapper {
  display: flex !important;
  align-items: center;
  width: 100%;
}

.tree-provider-node-wrapper {
  gap: 10px;
}

.provider-tree-label {
  min-width: 0;
  display: inline-flex;
  align-items: center;
  flex-wrap: wrap;
  gap: 6px;
  flex: 1;
}

.provider-tree-link {
  border: none;
  background: transparent;
  padding: 0;
  margin: 0;
  color: #1677ff;
  cursor: pointer;
  font-size: 14px;
  font-weight: 600;
  line-height: 1.4;
}

.provider-tree-link:hover {
  text-decoration: underline;
}

.provider-tree-link.is-grey {
  color: #8c8c8c;
}

.provider-tree-actions {
  margin-left: auto;
  opacity: 0.12;
  transition: opacity 0.2s ease;
}

.tree-provider-node-wrapper:hover .provider-tree-actions {
  opacity: 1;
}

.site-tree-actions {
  display: inline-flex;
  align-items: center;
  gap: 4px;
  opacity: 0.12;
  transition: opacity 0.2s ease;
}

.tree-provider-node-wrapper:hover .site-tree-actions {
  opacity: 1;
}

.result-performance-cell {
  display: inline-flex;
  align-items: center;
  gap: 6px;
  white-space: nowrap;
}

.performance-tooltip-list {
  display: flex;
  flex-direction: column;
  gap: 2px;
}

.performance-badge {
  width: 16px;
  height: 16px;
  display: inline-flex;
  align-items: center;
  justify-content: center;
  border-radius: 999px;
  border: 1px solid rgba(217, 119, 6, 0.24);
  background: rgba(255, 247, 237, 0.92);
  color: #d97706;
  font-size: 10px;
  line-height: 1;
}

.performance-badge-inline {
  flex: 0 0 auto;
  cursor: help;
}

.site-tree-action-btn {
  width: 24px;
  height: 24px;
  padding: 0;
  border: 0;
  border-radius: 999px;
  background: rgba(22, 119, 255, 0.08);
  color: #1677ff;
  display: inline-flex;
  align-items: center;
  justify-content: center;
  cursor: pointer;
}

.site-tree-action-btn:hover {
  background: rgba(22, 119, 255, 0.16);
}

.site-tree-action-btn.is-danger {
  background: rgba(255, 77, 79, 0.08);
  color: #ff4d4f;
}

.site-tree-note-badge {
  display: inline-flex;
  align-items: center;
  max-width: 120px;
  padding: 0 8px;
  border-radius: 999px;
  background: rgba(245, 208, 112, 0.2);
  color: #8a5a00;
  font-size: 11px;
  line-height: 20px;
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
}

.site-tree-inline-tag {
  display: inline-flex;
  align-items: center;
  padding: 0 8px;
  border-radius: 999px;
  background: rgba(22, 119, 255, 0.12);
  color: #1677ff;
  font-size: 11px;
  line-height: 20px;
  user-select: none;
}

.site-tree-inline-delete-btn {
  width: 20px;
  height: 20px;
  padding: 0;
  border: 0;
  border-radius: 999px;
  background: rgba(255, 77, 79, 0.08);
  color: #ff4d4f;
  display: inline-flex;
  align-items: center;
  justify-content: center;
  cursor: pointer;
}

.site-tree-inline-delete-btn:hover {
  background: rgba(255, 77, 79, 0.16);
}

.tree-site-disabled {
  color: #8c8c8c;
  text-decoration: line-through;
  opacity: 0.76;
}

:deep(body.dark-mode) .site-tree-action-btn {
  background: rgba(172, 199, 151, 0.12);
  color: #dfead8;
}

:deep(body.dark-mode) .site-tree-action-btn:hover {
  background: rgba(172, 199, 151, 0.22);
}

:deep(body.dark-mode) .site-tree-action-btn.is-danger {
  background: rgba(255, 77, 79, 0.16);
  color: #ffb6b7;
}

:deep(body.dark-mode) .site-tree-note-badge {
  background: rgba(245, 208, 112, 0.18);
  color: #ffd98b;
}

:deep(body.dark-mode) .site-tree-inline-tag {
  background: rgba(92, 164, 255, 0.18);
  color: #a9d0ff;
}

:deep(body.dark-mode) .site-tree-inline-delete-btn {
  background: rgba(255, 77, 79, 0.16);
  color: #ffb6b7;
}

.provider-diagnostic-trigger {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  min-width: 42px;
  padding: 2px 8px;
  border-radius: 999px;
  background: rgba(140, 140, 140, 0.14);
  color: #8c8c8c;
  font-size: 12px;
  cursor: pointer;
  user-select: none;
}

.provider-diagnostic-menu {
  display: flex;
  flex-direction: column;
  gap: 8px;
  min-width: 180px;
}

.shortcut-actions {
  opacity: 0.1;
  transition: opacity 0.3s ease;
}

.custom-tree-node-wrapper:hover .shortcut-actions {
  opacity: 1;
}

.app-icon {
  cursor: pointer;
  font-size: 14px;
  filter: grayscale(0.8);
  transition: all 0.2s cubic-bezier(0.175, 0.885, 0.32, 1.275);
  display: inline-flex;
  align-items: center;
  justify-content: center;
}

.app-icon:hover {
  filter: grayscale(0);
  transform: scale(1.3);
}

.cherry-icon:hover {
  text-shadow: 0 0 8px rgba(255, 0, 0, 0.4);
}

.switch-icon:hover {
  text-shadow: 0 0 8px rgba(0, 123, 255, 0.4);
}

.key-sync-strategy-modal {
  display: grid;
  gap: 12px;
}

.key-sync-strategy-summary {
  margin: 0;
  color: #4f5f49;
  line-height: 1.7;
}

.key-sync-strategy-option {
  display: grid;
  grid-template-columns: auto minmax(0, 1fr);
  gap: 10px;
  align-items: start;
  padding: 12px 14px;
  border-radius: 14px;
  background: rgba(245, 249, 242, 0.96);
  border: 1px solid rgba(137, 165, 126, 0.18);
}

.key-sync-strategy-index {
  font-weight: 700;
  color: #3f6f35;
}

.key-sync-strategy-title {
  font-weight: 700;
  color: #23311d;
  margin-bottom: 4px;
}

.key-sync-strategy-desc {
  color: #65735d;
  line-height: 1.65;
}

.portable-settings-card {
  display: grid;
  gap: 18px;
  padding: 18px;
  border-radius: 18px;
  border: 1px solid rgba(116, 144, 104, 0.16);
  background: rgba(248, 251, 246, 0.96);
}

.portable-settings-copy {
  display: grid;
  gap: 8px;
}

.portable-settings-title {
  font-size: 18px;
  font-weight: 700;
  color: #20301b;
}

.portable-settings-desc,
.portable-settings-hint,
.portable-settings-meta,
.portable-settings-warning {
  line-height: 1.7;
  color: #5f6f59;
}

.portable-settings-warning {
  color: #b25f00;
}

.portable-settings-actions {
  display: flex;
  gap: 12px;
}

:deep(body.dark-mode) .settings-action-bar {
  border-color: rgba(154, 191, 142, 0.16);
  background: linear-gradient(180deg, rgba(22, 28, 22, 0.94), rgba(18, 24, 18, 0.9));
  box-shadow: 0 16px 34px rgba(0, 0, 0, 0.18);
}

:deep(body.dark-mode) .batch-settings-label {
  color: #d3dfcd;
}

:deep(body.dark-mode) .batch-setting-input {
  border-color: rgba(154, 191, 142, 0.18);
  background: rgba(28, 35, 27, 0.94);
}

:deep(body.dark-mode) .batch-setting-input :deep(.ant-input-number-input) {
  color: #edf6e9;
}

:deep(body.dark-mode) .batch-reset-button {
  border-color: rgba(154, 191, 142, 0.18) !important;
  color: #e4f1df !important;
  background: rgba(28, 35, 27, 0.94) !important;
}

:deep(body.dark-mode) .batch-start-button {
  background: linear-gradient(135deg, #5d8255, #89a864) !important;
  box-shadow: 0 10px 20px rgba(0, 0, 0, 0.22) !important;
}

:deep(body.dark-mode) .key-sync-strategy-summary {
  color: #d5e6cf;
}

:deep(body.dark-mode) .key-sync-strategy-option {
  background: rgba(24, 32, 25, 0.92);
  border-color: rgba(154, 191, 142, 0.2);
}

:deep(body.dark-mode) .key-sync-strategy-index {
  color: #9fcc8a;
}

:deep(body.dark-mode) .key-sync-strategy-title {
  color: #ecf8e7;
}

:deep(body.dark-mode) .key-sync-strategy-desc {
  color: #b8cbb1;
}

:deep(body.dark-mode) .portable-settings-card {
  border-color: rgba(154, 191, 142, 0.18);
  background: rgba(24, 32, 25, 0.92);
}

:deep(body.dark-mode) .portable-settings-title {
  color: #ecf8e7;
}

:deep(body.dark-mode) .portable-settings-desc,
:deep(body.dark-mode) .portable-settings-hint,
:deep(body.dark-mode) .portable-settings-meta {
  color: #b8cbb1;
}

:deep(body.dark-mode) .portable-settings-warning {
  color: #ffcb8a;
}

@media (max-width: 620px) {
  .batch-hero {
    padding: 12px 10px;
  }

  .page-title-row {
    gap: 14px;
  }

  .hero-stage-grid {
    grid-template-columns: minmax(0, 1fr);
  }

  .hero-primary-pair {
    grid-template-columns: minmax(0, 1fr);
  }

  .hero-action-card-large,
  .hero-action-card-compact {
    min-height: unset;
  }

  .forest-firegrass {
    width: 136px;
    height: 88px;
  }

  .hero-primary-button {
    min-width: 0;
    width: 100%;
  }

  .result-topbar {
    grid-template-columns: minmax(0, 1fr);
  }

  .selection-header-row {
    align-items: flex-start;
  }

  .selection-title,
  .selection-action-group {
    white-space: normal;
  }

  .selection-action-group {
    margin-left: 0;
    justify-content: flex-start;
  }

  .quick-filter-strip {
    grid-template-columns: repeat(auto-fit, minmax(112px, 1fr));
  }

  .result-side-controls {
    width: 100%;
    min-width: 0;
  }

  .settings-action-bar {
    padding: 12px 13px;
  }

  .batch-settings {
    width: 100%;
  }

  .batch-setting-input {
    width: min(100%, 112px);
    min-width: 0;
    flex: 1 1 112px;
  }

  .actions {
    width: 100%;
    margin-left: 0;
  }

  .batch-reset-button,
  .batch-start-button {
    flex: 1 1 0;
  }

  .portable-settings-actions {
    flex-direction: column;
  }
}
</style>

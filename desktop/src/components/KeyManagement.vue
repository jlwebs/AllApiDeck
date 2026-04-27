<template>
  <ConfigProvider :theme="configProviderTheme">
    <div class="wrapper batch-wrapper key-management-wrapper" :class="{ 'site-wrapper-gaia key-management-wrapper-gaia': isDarkMode }">
      <div class="batch-shell key-management-shell">
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
        <div class="page-content batch-page-content key-management-page-content">
          <div class="container batch-page-container key-management-page-container">
            <div class="key-management" :class="{ 'key-management-compact': isCompactMode, 'key-management-gaia': isDarkMode }">
              <AppHeader v-if="!isCompactMode" current-page="keys" :is-dark-mode="isDarkMode" @experimental="showExperimentalFeatures = true" @settings="openSettingsModal" />

      <template v-if="!isCompactMode">

      <a-card class="sync-card sync-card-inline-title">
        <div class="sync-toolbar">
          <div class="sync-title-wrap">
            <span class="sync-title-text">同步密钥历史</span>
          </div>
          <div class="sync-meta">
            <span>本地记录：{{ tableData.length }}</span>
            <span>状态正常：{{ healthyKeyCount }}</span>
            <span class="sync-meta-time sync-meta-time-row">上次同步：{{ formatCompactDateTime(syncMeta.lastBatchSyncAt) }}</span>
          </div>
          <div v-if="!loading && failedSites.length === 0" class="sync-summary-slot">
            <a-alert type="info" show-icon class="sync-alert sync-alert-inline" :message="syncSummary" />
          </div>
          <div class="sync-panel-trigger-slot">
            <a-tooltip
              title="进入挂件悬窗模式"
              placement="topRight"
              overlay-class-name="key-management-mini-bar-tooltip"
              :getPopupContainer="getSidebarPopupContainer"
            >
              <button
                type="button"
                class="sync-panel-trigger-button sync-panel-trigger-button-fiery"
                :disabled="openingManualSidebar"
                @click="openManualMiniBar"
              >
                <MenuFoldOutlined />
              </button>
            </a-tooltip>
          </div>
        </div>

        <div v-if="loading" class="sync-loading"><a-spin /><span>正在批量提取 sk key，并写入 localStorage...</span></div>
        <div v-else-if="failedSites.length > 0" class="sync-feedback">
          <a-alert type="warning" show-icon class="sync-alert sync-alert-warning" :message="`${failedSites.length} 个站点本次未获取到 key，详见 logs/fetch-keys.log`" :description="failedSiteNames" />
          <a-alert type="info" show-icon class="sync-alert sync-alert-inline" :message="syncSummary" />
        </div>
      </a-card>
      </template>

      <div v-if="isCompactMode" class="compact-sidebar-summary">
        <div class="compact-sidebar-heading">
          <strong>密钥侧边栏</strong>
          <span class="subtle-text">{{ displayedRows.length }} 条可见 / 正常 {{ healthyKeyCount }}</span>
        </div>
        <a-alert type="info" show-icon class="compact-sidebar-alert" :message="syncSummary" />
      </div>

      <a-card title="本地密钥管理" class="inventory-card">
        <template #extra>
          <a-space wrap>
            <a-checkbox v-model:checked="hideInvalidKeys">隐藏无效密钥</a-checkbox>
            <a-popover
              trigger="hover"
              placement="bottomRight"
              overlay-class-name="key-management-batch-popover"
              :getPopupContainer="getSidebarPopupContainer"
            >
              <template #content>
                <div class="import-export-menu">
                  <button
                    type="button"
                    class="import-export-menu-item"
                    :disabled="batchQuickTestDisabled"
                    @click="runBatchQuickTest"
                  >
                    <ThunderboltOutlined />
                    <span>{{ batchQuickTestRunning ? `批量快测 ${batchQuickTestProgress.completed}/${batchQuickTestProgress.total}` : '批量快测' }}</span>
                  </button>
                  <button
                    type="button"
                    class="import-export-menu-item import-export-menu-item-danger"
                    :disabled="batchDeleteAbnormalDisabled"
                    @click="confirmDeleteAbnormalRecords"
                  >
                    <DeleteOutlined />
                    <span>批量删除异常密钥</span>
                  </button>
                </div>
              </template>
              <a-tooltip :title="batchActionButtonTitle">
                <a-button
                  type="primary"
                  size="small"
                  class="inventory-batch-quick-test-button"
                >
                  <ThunderboltOutlined />
                </a-button>
              </a-tooltip>
            </a-popover>
            <a-tooltip title="手工添加">
              <button type="button" class="inventory-icon-button inventory-icon-button-primary" @click="openManualRecordModal()">
                <PlusOutlined />
              </button>
            </a-tooltip>
            <a-button v-if="isCompactMode" size="small" @click="exitCompactSidebar">展开</a-button>
            <a-popover
              trigger="hover"
              placement="bottomRight"
              overlay-class-name="key-management-import-popover"
              :getPopupContainer="getSidebarPopupContainer"
            >
              <template #content>
                <div class="import-export-menu">
                  <button type="button" class="import-export-menu-item" @click="exportAllValidKeysPackage">
                    <DownloadOutlined />
                    <span>导出全部有效 Key</span>
                  </button>
                  <button type="button" class="import-export-menu-item" :disabled="displayedRows.length === 0" @click="exportCsv">
                    <FileTextOutlined />
                    <span>导出 CSV</span>
                  </button>
                  <button type="button" class="import-export-menu-item" @click="importFromClipboardPackage">
                    <ImportOutlined />
                    <span>从剪贴板导入</span>
                  </button>
                </div>
              </template>
              <a-tooltip title="导入/导出">
                <button type="button" class="inventory-icon-button inventory-icon-button-primary">
                  <SwapOutlined />
                </button>
              </a-tooltip>
            </a-popover>
            <a-tooltip v-if="!isCompactMode" title="清空本地库">
              <a-popconfirm title="确认清空本地密钥库？" ok-text="清空" cancel-text="取消" @confirm="clearLocalRecords">
                <button type="button" class="inventory-icon-button inventory-icon-button-danger" :disabled="tableData.length === 0">
                  <DeleteOutlined />
                </button>
              </a-popconfirm>
            </a-tooltip>
          </a-space>
        </template>

        <a-empty v-if="displayedRows.length === 0" description="暂无本地密钥记录，可从批量检测自动同步、剪贴板导入或手工添加。" />
        <a-table
          v-else
          :columns="activeColumns"
          :data-source="displayedRows"
          :row-key="record => record.rowKey"
          :pagination="tablePagination"
          :scroll="isCompactMode ? { x: 650 } : { x: 800 }"
          size="small"
          class="compact-key-table"
          @change="handleTableChange"
        >
          <template #bodyCell="{ column, record }">
            <template v-if="column.dataIndex === 'siteName'">
              <div class="site-cell">
                <div class="site-top-row">
                  <div class="site-main-block">
                    <div class="site-heading">
                      <button
                        type="button"
                        class="site-title-link"
                        :style="getSiteTitleStyle(record.siteName)"
                        :disabled="!record.siteUrl"
                        @click="openRecordSiteUrl(record)"
                      >
                        <strong class="site-title-text">{{ record.siteName }}</strong>
                      </button>
                      <a-tag v-if="isCompactMode" :color="record.status === 1 ? 'green' : 'red'">{{ record.status === 1 ? '正常' : '异常' }}</a-tag>
                    </div>
                    <div class="site-subline">
                      <a-tag v-if="record.sourceType === 'manual'" color="blue">手工添加</a-tag>
                      <span class="subtle-text">{{ record.tokenName || '未命名 Token' }}</span>
                    </div>
                    <div v-if="isCompactMode" class="compact-site-api-block">
                      <a-typography-text :copyable="{ text: record.apiKey }" :ellipsis="{ tooltip: record.apiKey }" class="cell-copy-text compact-key-text">{{ maskApiKey(record.apiKey) }}</a-typography-text>
                      <a-typography-text :copyable="{ text: record.siteUrl }" :ellipsis="{ tooltip: record.siteUrl }" class="cell-copy-text api-endpoint-text compact-endpoint-text">{{ record.siteUrl }}</a-typography-text>
                    </div>
                  </div>
                  <div v-if="canRefreshBalance(record)" class="site-balance-panel" :title="getRecordBalanceTooltip(record)">
                    <div class="site-balance-meta">
                      <span class="site-balance-time">
                        <ClockCircleOutlined />
                        <span>{{ getBalanceRelativeTime(record) }}</span>
                      </span>
                      <button type="button" class="site-balance-refresh-icon-button" :disabled="record.balanceLoading" @click="refreshRecordBalance(record)">
                        <ReloadOutlined class="site-balance-refresh-icon" :class="{ 'site-balance-refresh-icon-spinning': record.balanceLoading }" />
                      </button>
                    </div>
                    <div class="site-balance-value" :class="{ 'site-balance-value-empty': !getRecordBalanceValue(record) || record.balanceLoading }">
                      <span class="site-balance-label">剩余:</span>
                      <span class="site-balance-text">{{ record.balanceLoading ? '--' : (getRecordBalanceNumericText(record) || '--') }}</span>
                      <span v-if="showBalanceUnit(record)" class="site-balance-unit">USD</span>
                    </div>
                  </div>
                </div>
              </div>
            </template>
            <template v-else-if="column.dataIndex === 'apiKey'">
              <div class="api-combined-cell">
                <a-typography-text :copyable="{ text: record.apiKey }" :ellipsis="{ tooltip: record.apiKey }" class="cell-copy-text">{{ maskApiKey(record.apiKey) }}</a-typography-text>
                <a-typography-text :copyable="{ text: record.siteUrl }" :ellipsis="{ tooltip: record.siteUrl }" class="cell-copy-text api-endpoint-text">{{ record.siteUrl }}</a-typography-text>
                <div v-if="!isCompactMode" class="api-model-row">
                  <a-tooltip :title="getRecordModelTooltip(record)">
                    <a-select
                      size="small"
                      class="record-model-select api-model-select"
                      popup-class-name="record-model-dropdown"
                      :value="record.selectedModel || undefined"
                      :options="getRecordModelOptions(record)"
                      :loading="record.modelLoading"
                      :filter-option="true"
                      option-filter-prop="label"
                      :placeholder="record.modelsList?.length ? '选择模型' : '点击拉取模型列表'"
                      @dropdownVisibleChange="open => handleRecordModelDropdownVisibleChange(record, open)"
                      @change="value => handleRecordModelSelectionChange(record, value)"
                    />
                  </a-tooltip>
                </div>
              </div>
            </template>
            <template v-else-if="column.dataIndex === 'modelsText'">
              <a-tooltip :title="getRecordModelTooltip(record)">
                <a-select
                  size="small"
                  class="record-model-select"
                  popup-class-name="record-model-dropdown"
                  :value="record.selectedModel || undefined"
                  :options="getRecordModelOptions(record)"
                  :loading="record.modelLoading"
                  :filter-option="true"
                  option-filter-prop="label"
                  :placeholder="record.modelsList?.length ? '选择模型' : '点击拉取模型列表'"
                  @dropdownVisibleChange="open => handleRecordModelDropdownVisibleChange(record, open)"
                  @change="value => handleRecordModelSelectionChange(record, value)"
                />
              </a-tooltip>
            </template>
            <template v-else-if="column.dataIndex === 'status'">
              <a-tag :color="record.status === 1 ? 'green' : 'red'">{{ record.status === 1 ? '正常' : '禁用/异常' }}</a-tag>
            </template>
            <template v-else-if="column.dataIndex === 'exportActions'">
              <div class="export-actions-cell">
                <div class="inline-export-actions">
                  <a-tooltip title="便捷一键设置">
                    <button type="button" class="export-icon-button export-desktop" @click="openDesktopConfigWizard(record)">
                      <img :src="quickSetupIcon" alt="便捷一键设置" class="export-icon-image" />
                    </button>
                  </a-tooltip>
                  <a-tooltip title="导出到 Cherry Studio">
                    <button type="button" class="export-icon-button export-cherry" @click="launchCherryStudio(record)">
                      <span class="export-icon-glyph">🍒</span>
                    </button>
                  </a-tooltip>
                  <a-popover trigger="hover" placement="bottom">
                    <template #content>
                      <div class="switch-app-menu">
                        <button
                          v-for="app in CC_SWITCH_TARGET_APPS"
                          :key="app"
                          type="button"
                          class="switch-app-item"
                          @click="launchCCSwitch(record, app)"
                        >
                          {{ app }}
                        </button>
                      </div>
                    </template>
                    <a-tooltip title="导出到 CC Switch">
                      <button type="button" class="export-icon-button export-switch">
                        <img :src="ccSwitchIcon" alt="CC Switch" class="export-icon-image export-icon-image-switch" />
                      </button>
                    </a-tooltip>
                  </a-popover>
                  <a-tooltip title="复制为单个 sk:// 导入命令">
                    <button type="button" class="export-icon-button export-copy" @click="copySingleImportCommand(record)">
                      <span class="export-icon-glyph">⧉</span>
                    </button>
                  </a-tooltip>
                </div>
                <div class="quick-test-cell export-quick-test-row">
                  <a-button type="primary" size="small" class="quick-test-button" :loading="record.quickTestLoading" @click="runQuickTest(record)">快速测</a-button>
                  <div class="quick-test-status-row">
                    <div class="quick-test-status-inline">
                      <a-tooltip :title="getQuickTestTooltip(record)">
                        <a-tag v-if="record.quickTestStatus" :color="getQuickTestColor(record.quickTestStatus)" class="quick-test-tag">{{ record.quickTestLabel || record.quickTestStatus }}</a-tag>
                        <span v-else class="subtle-text">未测速</span>
                      </a-tooltip>
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
                  </div>
                </div>
              </div>
            </template>
            <template v-else-if="column.dataIndex === 'updatedAt'">
              <div class="time-cell"><span>{{ formatDateTime(record.updatedAt) }}</span><span class="subtle-text">{{ record.quickTestAt ? `快测 ${formatDateTime(record.quickTestAt)}` : '暂无快测记录' }}</span></div>
            </template>
            <template v-else-if="column.dataIndex === 'rowActions'">
              <div class="row-actions-stack">
                <a-button size="small" @click="openManualRecordModal(record)">编辑</a-button>
                <a-popconfirm title="确认删除这条记录？" ok-text="删除" cancel-text="取消" @confirm="deleteRecord(record)">
                  <a-button size="small" danger>删除</a-button>
                </a-popconfirm>
              </div>
            </template>
          </template>
        </a-table>
      </a-card>

      <a-modal
        v-model:open="manualRecordModalOpen"
        :title="null"
        :closable="false"
        :footer="null"
        :mask-closable="false"
        :width="'min(96vw, 1120px)'"
        wrap-class-name="manual-record-modal-wrap"
        @cancel="closeManualRecordModal"
      >
        <div class="manual-record-dialog">
          <div class="manual-record-header">
            <div class="manual-record-header-copy">
              <div class="manual-record-kicker">Key Editor</div>
              <div class="manual-record-title">{{ manualRecordEditing ? '编辑密钥' : '手工添加密钥' }}</div>
              <div class="manual-record-subtitle">常用字段两列排布，减少上下滚动和来回切换。</div>
            </div>

            <div class="manual-record-header-actions">
              <a-button
                type="text"
                size="small"
                class="manual-record-close-button"
                aria-label="关闭"
                title="关闭"
                @click="closeManualRecordModal"
              >
                ×
              </a-button>
            </div>
          </div>

          <a-form layout="vertical" class="manual-record-form">
            <div class="manual-record-fields">
              <div class="manual-record-row">
                <a-form-item label="网站名称" class="manual-record-form-item">
                  <a-input v-model:value="manualRecordDraft.siteName" size="small" placeholder="例如 My Site" />
                </a-form-item>
                <a-form-item label="Token 名称" class="manual-record-form-item">
                  <a-input v-model:value="manualRecordDraft.tokenName" size="small" placeholder="可选" />
                </a-form-item>
              </div>

              <div class="manual-record-row">
                <a-form-item label="接口地址" class="manual-record-form-item">
                  <a-input v-model:value="manualRecordDraft.siteUrl" size="small" placeholder="https://example.com" />
                </a-form-item>
                <a-form-item label="API Key" class="manual-record-form-item">
                  <a-input-password v-model:value="manualRecordDraft.apiKey" size="small" placeholder="sk-..." />
                </a-form-item>
              </div>

              <div class="manual-record-row manual-record-row-last">
                <a-form-item label="状态" class="manual-record-form-item manual-record-form-item-tight">
                  <a-select v-model:value="manualRecordDraft.status" size="small">
                    <a-select-option :value="1">正常</a-select-option>
                    <a-select-option :value="2">禁用/异常</a-select-option>
                  </a-select>
                </a-form-item>
                <a-form-item label="模型候选" class="manual-record-form-item manual-record-form-item-tight">
                  <a-select
                    v-model:value="manualRecordDraft.modelsValue"
                    :options="manualModelOptions"
                    :loading="manualModelLoading"
                    size="small"
                    show-search
                    :filter-option="true"
                    option-filter-prop="label"
                    placeholder="切换到这里会自动抓取模型，单选保留一个候选"
                    @dropdownVisibleChange="handleManualModelDropdownVisibleChange"
                    @change="handleManualModelSelectionChange"
                  />
                </a-form-item>
              </div>
            </div>
          </a-form>

          <div class="manual-record-footer">
            <a-button size="small" @click="closeManualRecordModal">取消</a-button>
            <a-button type="primary" size="small" :loading="manualRecordSaving" @click="submitManualRecord">保存</a-button>
          </div>
        </div>
      </a-modal>

      <a-modal v-model:open="desktopConfigModalOpen" title="专属一键配置" :confirm-loading="desktopConfigLoading" ok-text="生成变更预览" cancel-text="取消" width="1120px" @ok="generateDesktopConfigPreview">
        <div v-if="desktopConfigTargetRecord" class="desktop-config-modal">
          <a-alert type="info" show-icon class="desktop-config-alert" :message="`${desktopConfigTargetRecord.siteName} | ${desktopConfigTargetRecord.siteUrl}`" :description="`将读取本机应用配置，生成变更预览，确认后才会真正写入。`" />
          <div class="desktop-config-layout">
            <section class="desktop-app-panel">
              <div class="desktop-panel-title">目标应用</div>
              <div class="desktop-panel-hint">默认不勾选，按需点选后再生成变更预览。</div>
              <div class="desktop-app-grid">
                <button
                  v-for="app in DESKTOP_CONFIG_APPS"
                  :key="app.id"
                  type="button"
                  class="desktop-app-card"
                  :class="[`desktop-app-${app.id}`, { 'desktop-app-card-active': isDesktopAppSelected(app.id) }]"
                  @click="toggleDesktopAppSelection(app.id)"
                >
                  <span class="desktop-app-logo">
                    <img :src="DESKTOP_APP_ICONS[app.id]" :alt="app.label" class="desktop-app-logo-image" />
                  </span>
                  <span class="desktop-app-name">{{ app.label }}</span>
                </button>
              </div>
            </section>

            <section class="desktop-form-panel">
              <a-form layout="vertical">
                <div class="config-grid">
                  <a-form-item label="Provider 名称"><a-input v-model:value="desktopConfigDraft.providerName" placeholder="例如 My Provider" /></a-form-item>
                  <a-form-item label="Provider Key">
                    <a-input
                      v-model:value="desktopConfigDraft.providerKey"
                      :readonly="desktopConfigDraft.forceCustomProviderKey !== false"
                      :placeholder="desktopConfigDraft.forceCustomProviderKey !== false ? 'custom' : '请输入 provider key'"
                    />
                    <a-checkbox :checked="desktopConfigDraft.forceCustomProviderKey !== false" class="desktop-provider-checkbox" @change="handleDesktopProviderKeyModeChange">
                      custom:统一化保证历史会话可见
                    </a-checkbox>
                    <div class="desktop-field-hint">默认勾选会统一写入 `custom`；取消后保持各应用修改前的当前 provider key。</div>
                  </a-form-item>
                  <a-form-item label="API Key"><a-input-password v-model:value="desktopConfigDraft.apiKey" placeholder="sk-..." /></a-form-item>
                  <a-form-item label="默认模型">
                    <a-select
                      v-model:value="desktopConfigDraft.model"
                      :options="desktopConfigModelOptions"
                      show-search
                      :filter-option="true"
                      option-filter-prop="label"
                      placeholder="请选择当前记录模型"
                    />
                  </a-form-item>
                  <a-form-item label="Claude Base URL"><a-input v-model:value="desktopConfigDraft.claudeBaseUrl" /></a-form-item>
                  <a-form-item label="Claude Key 字段"><a-select v-model:value="desktopConfigDraft.claudeApiKeyField"><a-select-option value="ANTHROPIC_AUTH_TOKEN">ANTHROPIC_AUTH_TOKEN</a-select-option><a-select-option value="ANTHROPIC_API_KEY">ANTHROPIC_API_KEY</a-select-option></a-select></a-form-item>
                  <a-form-item label="Claude 高级代理">
                    <a-switch v-model:checked="desktopConfigDraft.claudeUseAdvancedProxy" />
                    <div class="desktop-field-hint">开启后会把 Claude Base URL 改写到本机高级代理地址，并由 All API Deck 负责兼容 OpenAI vendor、故障转移和错误修正。</div>
                  </a-form-item>
                  <a-form-item label="Codex Base URL"><a-input v-model:value="desktopConfigDraft.codexBaseUrl" /></a-form-item>
                  <a-form-item label="Codex 高级代理">
                    <a-switch v-model:checked="desktopConfigDraft.codexUseAdvancedProxy" />
                    <div class="desktop-field-hint">开启后会把 Codex 的 `base_url` 改写到本地代理，并使用占位 Key。</div>
                  </a-form-item>
                  <a-form-item label="OpenCode Base URL"><a-input v-model:value="desktopConfigDraft.opencodeBaseUrl" /></a-form-item>
                  <a-form-item label="OpenCode Adapter"><a-select v-model:value="desktopConfigDraft.opencodeNpm"><a-select-option value="@ai-sdk/openai-compatible">@ai-sdk/openai-compatible</a-select-option><a-select-option value="@openrouter/ai-sdk-provider">@openrouter/ai-sdk-provider</a-select-option></a-select></a-form-item>
                  <a-form-item label="OpenCode 高级代理">
                    <a-switch v-model:checked="desktopConfigDraft.opencodeUseAdvancedProxy" />
                    <div class="desktop-field-hint">开启后会改写到本地 OpenAI 兼容代理入口，并固定使用 openai-compatible 适配器。</div>
                  </a-form-item>
                  <a-form-item label="OpenClaw Base URL"><a-input v-model:value="desktopConfigDraft.openclawBaseUrl" /></a-form-item>
                  <a-form-item label="OpenClaw 高级代理">
                    <a-switch v-model:checked="desktopConfigDraft.openclawUseAdvancedProxy" />
                    <div class="desktop-field-hint">开启后会改写到本地 OpenClaw 代理入口，并切到 openai-completions 协议。</div>
                  </a-form-item>
                  <a-form-item label="OpenClaw API 协议"><a-select v-model:value="desktopConfigDraft.openclawApi"><a-select-option value="openai-completions">openai-completions</a-select-option><a-select-option value="anthropic-messages">anthropic-messages</a-select-option></a-select></a-form-item>
                </div>
              </a-form>
            </section>
          </div>
        </div>
      </a-modal>

              <DesktopConfigDiffModal :open="desktopConfigDiffOpen" :preview="desktopConfigPreview" @cancel="desktopConfigDiffOpen = false" @confirm="applyDesktopConfigPreview" />
              <SystemSettingsModal
                v-model:open="showAppSettingsModal"
                v-model:tree-expanded="globalTreeExpanded"
                v-model:desktop-token-source-mode="desktopTokenSourceMode"
                :app-name="'All API Deck'"
              />
              <AdvancedProxyModal v-model:open="showExperimentalFeatures" />
            </div>
          </div>
        </div>
      </div>
    </div>
  </ConfigProvider>
</template>

<script setup>
import { computed, h, onBeforeUnmount, onMounted, reactive, ref, watch } from 'vue';
import { ClockCircleOutlined, DeleteOutlined, DownloadOutlined, FileTextOutlined, ImportOutlined, MenuFoldOutlined, PlusOutlined, ReloadOutlined, SwapOutlined, ThunderboltOutlined } from '@ant-design/icons-vue';
import { ConfigProvider, message, Modal, theme } from 'ant-design-vue';
import { useRoute } from 'vue-router';
import AppHeader from './AppHeader.vue';
import AdvancedProxyModal from './AdvancedProxyModal.vue';
import DesktopConfigDiffModal from './DesktopConfigDiffModal.vue';
import SystemSettingsModal from './SystemSettingsModal.vue';
import { fetchModelList } from '../utils/api.js';
import { maskApiKey } from '../utils/normal.js';
import { apiFetch, isProbablyWailsRuntime, openUrlInSystemBrowser } from '../utils/runtimeApi.js';
import { applyManagedAppConfigFiles, isDesktopConfigBridgeAvailable, readManagedAppConfigFiles } from '../utils/desktopConfigBridge.js';
import { buildDesktopConfigPreview, createDesktopConfigDraft, DESKTOP_CONFIG_APPS, inferProviderKeyFromSnapshot } from '../utils/desktopConfigTransform.js';
import { fetchQuotaLabelWithBatchLogic, isDisplayableQuotaLabel } from '../utils/balance.js';
import { buildQuickTestMessages } from '../utils/quickTestPrompts.js';
import { normalizeCCSwitchEndpoint } from '../utils/ccSwitch.js';
import { getAppliedThemeMode, isDarkThemeMode, THEME_MODE_CHANGE_EVENT } from '../utils/theme.js';
import { exitSidebarMode, isManualSidebarBridgeAvailable, isSidebarBridgeAvailable, openManualSidebarPanel } from '../utils/windowMode.js';
import { loadDesktopTokenSourceMode, loadTreeExpandedSetting } from '../utils/systemSettings.js';
import { buildPerformanceTooltipLines, derivePerformanceMetricsFromResponse, hasPerformanceMetrics } from '../utils/performanceMetrics.js';
import {
  hydrateLastResultsSnapshotCache,
  HISTORY_SNAPSHOT_INDEX_KEY,
  HISTORY_SNAPSHOT_SYNC_EVENT,
  getCachedLastResultsSnapshotRaw,
} from '../utils/historySnapshotStore.js';
import { ExportTextFile } from '../../wailsjs/go/main/App.js';
import claudeAppIcon from '../assets/app-icons/claude.svg';
import codexAppIcon from '../assets/app-icons/codex.svg';
import geminiAppIcon from '../assets/app-icons/gemini.svg';
import opencodeAppIcon from '../assets/app-icons/opencode.svg';
import openclawAppIcon from '../assets/app-icons/openclaw-fallback.svg';
import quickSetupIcon from '../assets/action-icons/quick-setup-cute.svg';
import ccSwitchIcon from '../assets/action-icons/cc-switch.png';

const STORAGE_KEY = 'api_check_key_management_records_v1';
const MANUAL_STORAGE_KEY = 'api_check_key_management_manual_records_v1';
const META_STORAGE_KEY = 'api_check_key_management_meta_v1';
const LAST_RESULTS_STORAGE_KEY = HISTORY_SNAPSHOT_INDEX_KEY;
const KEY_MANAGEMENT_SYNC_EVENT = 'batch-api-check:key-management-sync';
const DEFAULT_TEST_TIMEOUT_MS = 20000;
const CC_SWITCH_TARGET_APPS = ['claude', 'codex', 'gemini', 'opencode', 'openclaw'];
const DESKTOP_APP_ICONS = {
  claude: claudeAppIcon,
  codex: codexAppIcon,
  gemini: geminiAppIcon,
  opencode: opencodeAppIcon,
  openclaw: openclawAppIcon,
};
const route = useRoute();
const isWailsRuntime = isProbablyWailsRuntime();

function createManualRecordDraft(record = null) {
  const modelsList = normalizeModels(record?.modelsList || record?.modelsText);
  const modelsValue = String(record?.selectedModel || pickPreferredModel(modelsList) || '').trim();
  return {
    rowKey: record?.rowKey || '',
    sourceType: record?.sourceType || 'manual',
    siteName: record?.siteName || '',
    tokenName: record?.tokenName || '',
    siteUrl: record?.siteUrl || '',
    apiKey: record?.apiKey || '',
    selectedModel: modelsValue,
    modelsText: modelsList.join(', '),
    modelsValue,
    status: Number(record?.status || 1),
  };
}

const isDarkMode = ref(false);
const loading = ref(false);
const allResults = ref([]);
const tableData = ref([]);
const showExperimentalFeatures = ref(false);
const syncMeta = ref({ lastBatchSyncAt: null, lastBatchSyncCount: 0, lastBatchFailedCount: 0 });
const keyBalanceRefreshBootstrapped = ref(false);
const batchHistoryContextMap = ref(new Map());
const desktopConfigModalOpen = ref(false);
const desktopConfigDiffOpen = ref(false);
const desktopConfigLoading = ref(false);
const desktopConfigTargetRecord = ref(null);
const desktopConfigPreview = ref({ appGroups: [], writes: [], errors: [] });
const desktopConfigDraft = reactive(createDesktopConfigDraft({}));
const desktopProviderKeyManualValue = ref('');
const manualRecordModalOpen = ref(false);
const manualRecordSaving = ref(false);
const manualRecordEditing = ref(false);
const manualRecordDraft = reactive(createManualRecordDraft());
const manualModelOptions = ref([]);
const manualModelLoading = ref(false);
const manualModelFetchKey = ref('');
const hideInvalidKeys = ref(true);
const BATCH_QUICK_TEST_CONCURRENCY = 10;
const batchQuickTestRunning = ref(false);
const batchQuickTestProgress = reactive({
  completed: 0,
  total: 0,
  active: 0,
});
const currentTablePage = ref(1);
const currentTablePageSize = ref(20);
const showAppSettingsModal = ref(false);
const globalTreeExpanded = ref(loadTreeExpandedSetting(true));
const desktopTokenSourceMode = ref(loadDesktopTokenSourceMode());
const settingsApiUrl = ref('');
const settingsApiKey = ref('');
const localCacheList = ref([]);
const portablePacking = ref(false);
const portableUnpacking = ref(false);
const portableSettingsMeta = ref('');
const openingManualSidebar = ref(false);
const manualSidebarBridgeReady = ref(false);
let manualSidebarBridgeProbeTimer = null;
const isCompactMode = computed(() => route.query?.compact === '1');
const PERSIST_DEBOUNCE_MS = 240;
let persistRecordsTimer = null;
let lastPersistedRecordsSnapshot = '';
const recordRenderMetaCache = new Map();

const configProviderTheme = computed(() => ({
  algorithm: isDarkMode.value ? theme.darkAlgorithm : theme.defaultAlgorithm,
}));

function syncThemeState() {
  isDarkMode.value = isDarkThemeMode(getAppliedThemeMode());
}

function getSidebarPopupContainer(triggerNode) {
  return triggerNode?.ownerDocument?.body || document.body;
}

const refreshManualSidebarBridgeReady = () => {
  manualSidebarBridgeReady.value = isManualSidebarBridgeAvailable();
  if (manualSidebarBridgeReady.value && manualSidebarBridgeProbeTimer) {
    clearInterval(manualSidebarBridgeProbeTimer);
    manualSidebarBridgeProbeTimer = null;
  }
};

const desktopConfigModelOptions = computed(() => {
  const record = desktopConfigTargetRecord.value;
  if (!record) return [];
  const options = getRecordModelOptions(record);
  const currentValue = String(desktopConfigDraft.model || '').trim();
  if (!currentValue) return options;
  return options.some(option => option.value === currentValue)
    ? options
    : [{ label: currentValue, value: currentValue }, ...options];
});
const sortManagedRecords = rows => [...rows].sort(
  (a, b) => Number(b.updatedAt || 0) - Number(a.updatedAt || 0) || String(a.siteName || '').localeCompare(String(b.siteName || ''))
);
const columns = [
  { title: '网站', dataIndex: 'siteName', key: 'siteName', width: 154, sorter: (a, b) => String(a.siteName || '').localeCompare(String(b.siteName || '')) },
  { title: 'API Key', dataIndex: 'apiKey', key: 'apiKey', width: 120, className: 'api-key-column' },
  { title: '状态', dataIndex: 'status', key: 'status', width: 56, sorter: (a, b) => Number(a.status || 0) - Number(b.status || 0) },
  { title: '专属导出', dataIndex: 'exportActions', key: 'exportActions', width: 116 },
  { title: '操作', dataIndex: 'rowActions', key: 'rowActions', width: 56 },
  { title: '最近同步', dataIndex: 'updatedAt', key: 'updatedAt', width: 138, sorter: (a, b) => Number(a.updatedAt || 0) - Number(b.updatedAt || 0), defaultSortOrder: 'descend' },
];
const activeColumns = computed(() => (isCompactMode.value
  ? columns.filter(column => ['siteName', 'exportActions', 'rowActions'].includes(column.dataIndex))
  : columns));
const failedSites = computed(() => allResults.value.filter(result => !Array.isArray(result?.tokens) || result.tokens.length === 0));
const failedSiteNames = computed(() => failedSites.value.map(site => site?.site_name || site?.id || '未命名站点').join('，'));
const allSortedRows = computed(() => sortManagedRecords(tableData.value));
const displayedRows = computed(() => {
  const filteredRows = hideInvalidKeys.value
    ? allSortedRows.value.filter(record => Number(record?.status || 0) === 1)
    : allSortedRows.value;
  return sortManagedRecords(filteredRows);
});
const healthyKeyCount = computed(() => tableData.value.filter(record => record.status === 1).length);
const abnormalKeyCount = computed(() => tableData.value.filter(record => Number(record?.status || 0) !== 1).length);
const syncSummary = computed(() => !syncMeta.value.lastBatchSyncAt ? '导入并批量检测后，会自动把获取到的 sk key 更新到本页。' : `最近一次批量同步写入 ${syncMeta.value.lastBatchSyncCount} 条记录，失败站点 ${syncMeta.value.lastBatchFailedCount} 个。`);
const currentVisiblePageRows = computed(() => {
  if (isCompactMode.value) return displayedRows.value;
  const start = Math.max(0, (currentTablePage.value - 1) * currentTablePageSize.value);
  return displayedRows.value.slice(start, start + currentTablePageSize.value);
});
const batchQuickTestDisabled = computed(() => batchQuickTestRunning.value || tableData.value.length === 0);
const batchDeleteAbnormalDisabled = computed(() => batchQuickTestRunning.value || abnormalKeyCount.value === 0);
const batchQuickTestButtonTitle = computed(() => {
  if (batchQuickTestRunning.value) {
    return `批量快测进行中：已完成 ${batchQuickTestProgress.completed}/${batchQuickTestProgress.total}，并发 ${BATCH_QUICK_TEST_CONCURRENCY}，运行中 ${batchQuickTestProgress.active}`;
  }
  return `按页面优先级批量触发“快速测”，并发 ${BATCH_QUICK_TEST_CONCURRENCY}，只测试每条当前已选择的模型`;
});
const batchActionButtonTitle = computed(() => {
  if (batchQuickTestRunning.value) {
    return batchQuickTestButtonTitle.value;
  }
  if (abnormalKeyCount.value > 0) {
    return `批量操作：可删除 ${abnormalKeyCount.value} 条异常密钥`;
  }
  return '批量操作';
});
const tablePagination = computed(() => {
  if (isCompactMode.value) return false;
  return {
    current: currentTablePage.value,
    pageSize: currentTablePageSize.value,
    showSizeChanger: true,
    pageSizeOptions: ['20', '50', '100'],
    total: displayedRows.value.length,
  };
});

onMounted(() => {
  void (async () => {
    await hydrateLastResultsSnapshotCache();
    syncThemeState();
    refreshManualSidebarBridgeReady();
    if (!manualSidebarBridgeReady.value && typeof window !== 'undefined') {
      manualSidebarBridgeProbeTimer = window.setInterval(refreshManualSidebarBridgeReady, 250);
    }
    refreshManagedRecordsFromStorage();
    if (typeof window !== 'undefined') {
      window.addEventListener(THEME_MODE_CHANGE_EVENT, syncThemeState);
      window.addEventListener(KEY_MANAGEMENT_SYNC_EVENT, handleManagedRecordSyncEvent);
      window.addEventListener(HISTORY_SNAPSHOT_SYNC_EVENT, handleManagedRecordSyncEvent);
      window.addEventListener('storage', handleManagedRecordStorageEvent);
    }
  })();
});

onBeforeUnmount(() => {
  flushPersistRecords();
  if (manualSidebarBridgeProbeTimer) {
    clearInterval(manualSidebarBridgeProbeTimer);
    manualSidebarBridgeProbeTimer = null;
  }
  if (typeof window !== 'undefined') {
    window.removeEventListener(THEME_MODE_CHANGE_EVENT, syncThemeState);
    window.removeEventListener(KEY_MANAGEMENT_SYNC_EVENT, handleManagedRecordSyncEvent);
    window.removeEventListener(HISTORY_SNAPSHOT_SYNC_EVENT, handleManagedRecordSyncEvent);
    window.removeEventListener('storage', handleManagedRecordStorageEvent);
  }
});

watch([displayedRows, currentTablePageSize, isCompactMode], () => {
  if (isCompactMode.value) {
    currentTablePage.value = 1;
    return;
  }
  const total = displayedRows.value.length;
  const maxPage = Math.max(1, Math.ceil(total / currentTablePageSize.value));
  if (currentTablePage.value > maxPage) {
    currentTablePage.value = maxPage;
  }
}, { immediate: true });

const openSettingsModal = () => {
  showAppSettingsModal.value = true;
};

const closeSettingsModal = () => {
  showAppSettingsModal.value = false;
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
    settingsApiUrl.value = record.url;
    settingsApiKey.value = record.apiKey;
    message.success('已加载到配置表单');
  }
};

function refreshManagedRecordsFromStorage() {
  recordRenderMetaCache.clear();
  batchHistoryContextMap.value = loadBatchHistoryContextMap();
  tableData.value = loadStoredRecords();
  syncMeta.value = loadStoredMeta();
  keyBalanceRefreshBootstrapped.value = false;
  void autoRefreshKeyBalancesOnce();
}

function handleManagedRecordSyncEvent() {
  refreshManagedRecordsFromStorage();
}

function handleManagedRecordStorageEvent(event) {
  const watchedKeys = [STORAGE_KEY, MANUAL_STORAGE_KEY, META_STORAGE_KEY, LAST_RESULTS_STORAGE_KEY];
  if (event?.key && !watchedKeys.includes(event.key)) return;
  refreshManagedRecordsFromStorage();
}

function handleTableChange(pagination) {
  if (isCompactMode.value || !pagination) return;
  const nextPage = Number(pagination.current || 1);
  const nextPageSize = Number(pagination.pageSize || currentTablePageSize.value || 20);
  if (nextPageSize !== currentTablePageSize.value) {
    currentTablePageSize.value = nextPageSize;
  }
  currentTablePage.value = nextPage;
}

async function exitCompactSidebar() {
  if (!isSidebarBridgeAvailable()) return;
  try {
    await exitSidebarMode();
  } catch (error) {
    console.error(error);
    message.error(`展开主界面失败：${error.message || '未知错误'}`);
  }
}

async function openManualMiniBar() {
  if (openingManualSidebar.value) return;
  openingManualSidebar.value = true;
  try {
    await openManualSidebarPanel();
  } catch (error) {
    console.error(error);
    message.error(`进入挂件悬窗模式失败：${error?.message || '未知错误'}`);
  } finally {
    openingManualSidebar.value = false;
  }
}

function beforeUpload(file) {
  const reader = new FileReader();
  reader.onload = async event => {
    try {
      const parsed = JSON.parse(String(event.target?.result || ''));
      const accounts = extractAccountsFromBackup(parsed);
      if (!accounts.length) {
        message.error('备份文件中未找到可用账号数据');
        return;
      }
      message.success(`已加载 ${accounts.length} 个账号，开始同步真实 sk key`);
      await processAccounts(accounts);
    } catch (error) {
      console.error(error);
      message.error(`解析备份文件失败：${error.message || '未知错误'}`);
    }
  };
  reader.readAsText(file);
  return false;
}

async function processAccounts(accounts) {
  const accountsToTarget = accounts.filter(account => !account?.disabled && account?.account_info?.access_token);
  if (accountsToTarget.length === 0) {
    message.warning('没有找到包含 access_token 的可用账号');
    return;
  }

  loading.value = true;
  allResults.value = [];
  try {
    const response = await apiFetch('/api/fetch-keys', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ accounts: accountsToTarget }),
    });
    if (!response.ok) {
      const errorPayload = await safeReadJson(response);
      throw new Error(errorPayload?.message || '批量获取真实 key 失败');
    }

    const data = await response.json();
    const results = Array.isArray(data?.results) ? data.results : [];
    const normalizedRows = normalizeFetchedRows(results);
    const mergedRows = mergeStoredRecords(normalizedRows);
    const failedCount = results.filter(result => !Array.isArray(result?.tokens) || result.tokens.length === 0).length;

    allResults.value = results;
    tableData.value = mergedRows;
    syncMeta.value = { lastBatchSyncAt: Date.now(), lastBatchSyncCount: normalizedRows.length, lastBatchFailedCount: failedCount };
    persistRecords();
    persistMeta();
    message.success(`批量同步完成：本次获取 ${normalizedRows.length} 个 sk key，失败站点 ${failedCount} 个。`);
  } catch (error) {
    console.error(error);
    message.error(`同步失败：${error.message || '未知错误'}`);
  } finally {
    loading.value = false;
  }
}

function normalizeFetchedRows(results) {
  const normalized = [];
  results.forEach(result => {
    const tokens = Array.isArray(result?.tokens) ? result.tokens : [];
    tokens.forEach((token, index) => {
      const apiKey = normalizeApiKey(token?.key);
      const siteUrl = resolveSiteUrl(result);
      if (!apiKey || !siteUrl) return;
      const modelsList = normalizeModels(token?.models);
      normalized.push({
        rowKey: buildRowKey(siteUrl, apiKey),
        sourceType: 'auto',
        siteName: result?.site_name || '未命名站点',
        tokenName: token?.name || `未命名 Token ${index + 1}`,
        siteUrl,
        apiKey,
        modelsList,
        modelsText: modelsList.length ? modelsList.join(', ') : '未提供模型信息',
        status: typeof token?.status === 'number' ? token.status : token?.is_disabled ? 2 : 1,
        remainQuota: token?.remain_quota ?? null,
        usedQuota: token?.used_quota ?? null,
        unlimitedQuota: token?.unlimited_quota === true || token?.remain_quota === undefined || token?.remain_quota < 0,
      });
    });
  });
  return normalized;
}

function mergeStoredRecords(incomingRows) {
  const now = Date.now();
  const mergedMap = new Map();
  loadStoredRecords().forEach(record => mergedMap.set(record.rowKey, { ...record, quickTestLoading: false }));
  incomingRows.forEach(row => {
    const previous = mergedMap.get(row.rowKey);
    mergedMap.set(row.rowKey, hydrateRecordModelSelection({
      ...previous,
      ...row,
      createdAt: previous?.createdAt || now,
      updatedAt: now,
      quickTestStatus: previous?.quickTestStatus || '',
      quickTestLabel: previous?.quickTestLabel || '',
      quickTestModel: previous?.quickTestModel || '',
      quickTestRemark: previous?.quickTestRemark || '',
      quickTestAt: previous?.quickTestAt || null,
      quickTestResponseTime: previous?.quickTestResponseTime || '',
      quickTestTtftMs: previous?.quickTestTtftMs || '',
      quickTestTps: previous?.quickTestTps || '',
      quickTestResponseContent: previous?.quickTestResponseContent || '',
      quickTestLoading: false,
    }));
  });
  return mergeBatchHistoryBalances(Array.from(mergedMap.values()));
}

async function runQuickTest(record, options = {}) {
  const silent = options?.silent === true;
  const fixedModel = String(options?.fixedModel || '').trim();
  if (record.quickTestLoading) return;
  record.quickTestLoading = true;
  try {
    const model = fixedModel || await resolveQuickTestModel(record);
    const testResult = await executeQuickTest({ apiKey: record.apiKey, siteUrl: record.siteUrl, model });
    record.quickTestStatus = testResult.status;
    record.quickTestLabel = testResult.label;
    record.quickTestModel = model;
    record.quickTestRemark = testResult.remark;
    record.quickTestAt = Date.now();
    record.quickTestResponseTime = testResult.responseTime;
    record.quickTestTtftMs = testResult.ttftMs || '';
    record.quickTestTps = testResult.tps || '';
    record.quickTestResponseContent = testResult.responseContent || '';
    persistRecords();
    if (!silent) {
      const messageMethod = testResult.status === 'success' ? 'success' : testResult.status === 'warning' ? 'warning' : 'error';
      message[messageMethod](`快测${testResult.label}：${record.siteName} / ${model}${testResult.responseTime ? ` / ${testResult.responseTime}s` : ''}`);
    }
    return {
      status: testResult.status,
      label: testResult.label,
      model,
      responseTime: testResult.responseTime,
    };
  } catch (error) {
    console.error(error);
    record.quickTestStatus = 'error';
    record.quickTestLabel = '失败';
    record.quickTestRemark = error.message || '快速测试失败';
    record.quickTestAt = Date.now();
    record.quickTestResponseTime = '';
    record.quickTestTtftMs = '';
    record.quickTestTps = '';
    record.quickTestResponseContent = '';
    const detail = String(error?.detail || error?.message || '快速测试失败').trim();
    record.quickTestRemark = detail;
    record.quickTestResponseContent = detail;
    persistRecords();
    if (!silent) {
      showQuickTestErrorDialog(detail);
      message.error(`快速测试失败：${error.message || '未知错误'}`);
    }
    return {
      status: 'error',
      label: '失败',
      model: fixedModel || String(record?.selectedModel || '').trim(),
      responseTime: '',
      errorDetail: detail,
    };
  } finally {
    record.quickTestLoading = false;
  }
}

function buildBatchQuickTestQueue() {
  const queue = [];
  const queued = new Set();
  const pushRecords = records => {
    records.forEach(record => {
      if (!record?.rowKey || queued.has(record.rowKey)) return;
      queued.add(record.rowKey);
      queue.push(record);
    });
  };

  const currentPageRows = currentVisiblePageRows.value;
  pushRecords(currentPageRows.filter(record => Number(record?.status || 0) === 1));
  pushRecords(currentPageRows.filter(record => Number(record?.status || 0) !== 1));

  const otherVisibleRows = displayedRows.value.filter(record => !queued.has(record.rowKey));
  pushRecords(otherVisibleRows.filter(record => Number(record?.status || 0) === 1));

  const remainingRows = allSortedRows.value.filter(record => !queued.has(record.rowKey));
  pushRecords(remainingRows);

  return queue;
}

function buildBatchQuickTestSummary(stats) {
  const summaryText = `已执行 ${stats.executed} 条，可用 ${stats.success} 条，告警 ${stats.warning} 条，失败 ${stats.error} 条，跳过 ${stats.skipped} 条。`;
  const detailParts = [];
  if (stats.skippedNoModel > 0) detailParts.push(`未选择模型 ${stats.skippedNoModel} 条`);
  if (stats.skippedInvalidConfig > 0) detailParts.push(`配置不完整 ${stats.skippedInvalidConfig} 条`);
  if (stats.skippedBusy > 0) detailParts.push(`已在测试中 ${stats.skippedBusy} 条`);
  if (stats.executedModels.length > 0) {
    const uniqueModels = Array.from(new Set(stats.executedModels)).slice(0, 8);
    detailParts.push(`本次模型 ${uniqueModels.join('、')}`);
  }
  return {
    type: stats.error > 0 ? 'warning' : 'success',
    message: `批量快测完成：${summaryText}`,
    description: detailParts.join('；') || '已按当前页优先级完成整库快测。',
  };
}

async function runBatchQuickTest() {
  if (batchQuickTestRunning.value) return;

  const queue = buildBatchQuickTestQueue();
  if (!queue.length) {
    message.warning('当前没有可处理的密钥记录');
    return;
  }

  batchQuickTestRunning.value = true;
  batchQuickTestProgress.total = queue.length;
  batchQuickTestProgress.completed = 0;
  batchQuickTestProgress.active = 0;

  const stats = {
    executed: 0,
    success: 0,
    warning: 0,
    error: 0,
    skipped: 0,
    skippedNoModel: 0,
    skippedInvalidConfig: 0,
    skippedBusy: 0,
    executedModels: [],
  };
  let cursor = 0;

  try {
    const worker = async () => {
      while (cursor < queue.length) {
        const index = cursor;
        cursor += 1;
        const record = queue[index];

        if (record.quickTestLoading) {
          stats.skipped += 1;
          stats.skippedBusy += 1;
          batchQuickTestProgress.completed += 1;
          continue;
        }

        const apiKey = normalizeApiKey(record?.apiKey);
        const siteUrl = normalizeSiteUrl(record?.siteUrl);
        const selectedModel = String(record?.selectedModel || '').trim();

        if (!apiKey || !siteUrl) {
          stats.skipped += 1;
          stats.skippedInvalidConfig += 1;
          batchQuickTestProgress.completed += 1;
          continue;
        }
        if (!selectedModel) {
          stats.skipped += 1;
          stats.skippedNoModel += 1;
          batchQuickTestProgress.completed += 1;
          continue;
        }

        batchQuickTestProgress.active += 1;
        try {
          const result = await runQuickTest(record, {
            silent: true,
            fixedModel: selectedModel,
          });

          stats.executed += 1;
          stats.executedModels.push(selectedModel);
          if (result?.status === 'success') stats.success += 1;
          else if (result?.status === 'warning') stats.warning += 1;
          else stats.error += 1;
        } finally {
          batchQuickTestProgress.active = Math.max(0, batchQuickTestProgress.active - 1);
          batchQuickTestProgress.completed += 1;
        }
      }
    };

    await Promise.allSettled(
      Array.from({ length: Math.min(BATCH_QUICK_TEST_CONCURRENCY, queue.length) }, () => worker())
    );
  } finally {
    batchQuickTestProgress.active = 0;
    batchQuickTestRunning.value = false;
  }

  const batchQuickTestNotice = buildBatchQuickTestSummary(stats);
  if (stats.error > 0) {
    message.warning(batchQuickTestNotice.message);
  } else {
    message.success(batchQuickTestNotice.message);
  }
}

async function resolveQuickTestModel(record) {
  const selectedModel = String(record?.selectedModel || '').trim();
  if (selectedModel) return selectedModel;
  const historyPreferred = getBatchHistoryContext(record)?.preferredModel || '';
  if (historyPreferred) {
    record.selectedModel = historyPreferred;
    persistRecords();
    return historyPreferred;
  }
  const fromRecord = pickPreferredModel(record.modelsList);
  if (fromRecord) {
    record.selectedModel = fromRecord;
    persistRecords();
    return fromRecord;
  }
  const modelResponse = await fetchModelList(record.siteUrl, record.apiKey);
  const rawCandidates = modelResponse?.data || modelResponse?.models || [];
  const normalizedCandidates = normalizeModels(rawCandidates);
  if (normalizedCandidates.length === 0) throw new Error('没有获取到可测试模型');
  const preferred = pickPreferredModel(normalizedCandidates);
  if (!preferred) throw new Error('没有找到适合快速对话测试的模型');
  record.modelsList = normalizedCandidates;
  record.modelsText = normalizedCandidates.join(', ');
  record.selectedModel = preferred;
  persistRecords();
  return preferred;
}

async function executeQuickTest({ apiKey, siteUrl, model }) {
  let timeoutMs = DEFAULT_TEST_TIMEOUT_MS;
  if (/^o1-|^o3-/i.test(model)) timeoutMs *= 3;
  const startedAt = Date.now();
  const response = await apiFetch('/api/check-key', {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ url: normalizeSiteUrl(siteUrl), key: apiKey, model, messages: buildQuickTestMessages(), timeoutMs, _isFirst: false }),
  });
  if (!response.ok) {
    const rawError = await safeReadResponsePayload(response);
    throw createQuickTestError(rawError, response.status, {
      siteUrl: normalizeSiteUrl(siteUrl),
      model,
      timeoutMs,
    });
  }

  let data = await response.json();
  if (data?.htmlSnippet) {
    const snippet = String(data.htmlSnippet).replace(/^data:\s*/, '').trim();
    if (snippet.startsWith('{') || snippet.startsWith('[')) {
      try { data = JSON.parse(snippet); } catch (error) { console.warn('Failed to parse htmlSnippet JSON payload', error); }
    }
  }

  const returnedModel = String(data?.model || 'unknown');
  const messageObj = data?.choices?.[0]?.message;
  const hasContent = Boolean(messageObj?.content || messageObj?.reasoning_content || messageObj?.thinking);
  const responseContent = extractQuickTestResponseContent(messageObj);
  const responseTime = ((Date.now() - startedAt) / 1000).toFixed(2);
  const performance = derivePerformanceMetricsFromResponse(data, responseTime);
  if (returnedModel.toLowerCase().includes(model.toLowerCase()) || returnedModel === 'unknown') {
    if (hasContent) {
      return {
        status: returnedModel === 'unknown' ? 'warning' : 'success',
        label: returnedModel === 'unknown' ? '可用待确认' : '可用',
        remark: returnedModel === 'unknown' ? '接口有正常响应，但未返回模型标识' : '接口返回了有效对话内容',
        responseTime,
        ttftMs: performance.ttftMs,
        tps: performance.tps,
        responseContent,
      };
    }
    return { status: 'warning', label: '结构异常', remark: '接口响应成功，但未检测到有效消息内容', responseTime, responseContent };
  }
  return { status: 'warning', label: '模型映射', remark: `平台返回模型 ${returnedModel}，请求模型为 ${model}`, responseTime, responseContent };
}

async function exportAllValidKeysPackage() {
  const validRecords = tableData.value
    .filter(record => record.status === 1 && record.siteUrl && record.apiKey)
    .map(({ quickTestLoading, ...record }) => ({
      ...record,
      modelsList: normalizeModels(record.modelsList || record.modelsText),
      modelsText: record.modelsText || '未提供模型信息',
    }));

  if (validRecords.length === 0) {
    message.warning('当前没有状态正常的 Key');
    return;
  }

  try {
    const payload = {
      format: 'api-check-key-export-v1',
      compressed: 'gzip',
      exportedAt: Date.now(),
      records: validRecords,
    };
    const compressed = await compressClipboardPackage(JSON.stringify(payload));
    const output = `sk://${compressed}`;
    await navigator.clipboard.writeText(output);
    message.success(`已导出 ${validRecords.length} 条有效 Key 到剪贴板`);
  } catch (error) {
    console.error(error);
    message.error(`导出失败：${error.message || '未知错误'}`);
  }
}

async function importFromClipboardPackage() {
  try {
    const text = String(await navigator.clipboard.readText()).trim();
    if (!text) {
      throw new Error('剪贴板为空');
    }

    if (!text.startsWith('sk://')) {
      throw new Error('剪贴板内容不是 sk:// 导入包');
    }

    const payloadText = await decompressClipboardPackage(text.slice('sk://'.length));
    const payload = JSON.parse(payloadText);
    const importedRecords = Array.isArray(payload?.records) ? payload.records : [];
    if (importedRecords.length === 0) {
      throw new Error('导入包中没有记录');
    }

    const merged = new Map(tableData.value.map(record => [record.rowKey, { ...record }]));
    importedRecords.forEach(rawRecord => {
      const modelsList = normalizeModels(rawRecord.modelsList || rawRecord.modelsText);
      const record = hydrateRecordModelSelection({
        ...rawRecord,
        sourceType: rawRecord.sourceType || 'auto',
        siteName: String(rawRecord.siteName || '未命名站点').trim() || '未命名站点',
        tokenName: String(rawRecord.tokenName || '').trim(),
        siteUrl: normalizeSiteUrl(rawRecord.siteUrl),
        apiKey: normalizeApiKey(rawRecord.apiKey),
        modelsList,
        modelsText: modelsList.join(', ') || '未提供模型信息',
        selectedModel: String(rawRecord.selectedModel || '').trim(),
        status: Number(rawRecord.status || 1),
        quickTestStatus: rawRecord.quickTestStatus || '',
        quickTestLabel: rawRecord.quickTestLabel || '',
        quickTestModel: rawRecord.quickTestModel || '',
        quickTestRemark: rawRecord.quickTestRemark || '',
        quickTestAt: rawRecord.quickTestAt || null,
        quickTestResponseTime: rawRecord.quickTestResponseTime || '',
        quickTestTtftMs: rawRecord.quickTestTtftMs || '',
        quickTestTps: rawRecord.quickTestTps || '',
        quickTestResponseContent: rawRecord.quickTestResponseContent || '',
        quickTestLoading: false,
      });
      record.rowKey = rawRecord.rowKey || (record.sourceType === 'manual' ? buildManualRowKey() : buildRowKey(record.siteUrl, record.apiKey));
      if (record.siteUrl && record.apiKey) {
        merged.set(record.rowKey, record);
      }
    });

    tableData.value = Array.from(merged.values());
    syncMeta.value = {
      lastBatchSyncAt: Date.now(),
      lastBatchSyncCount: importedRecords.length,
      lastBatchFailedCount: importedRecords.filter(record => Number(record?.status || 1) !== 1).length,
    };
    persistRecords();
    persistMeta();
    message.success(`已从剪贴板导入 ${importedRecords.length} 条记录`);
  } catch (error) {
    console.error(error);
    message.error(`导入失败：${error.message || '未知错误'}`);
  }
}

async function copySingleImportCommand(record) {
  try {
    const normalizedRecord = {
      ...record,
      sourceType: record.sourceType || 'auto',
      rowKey: record.rowKey || (record.sourceType === 'manual' ? buildManualRowKey() : buildRowKey(record.siteUrl, record.apiKey)),
      siteName: String(record.siteName || '未命名站点').trim() || '未命名站点',
      tokenName: String(record.tokenName || '').trim(),
      siteUrl: normalizeSiteUrl(record.siteUrl),
      apiKey: normalizeApiKey(record.apiKey),
      modelsList: normalizeModels(record.modelsList || record.modelsText),
      modelsText: normalizeModels(record.modelsList || record.modelsText).join(', ') || '未提供模型信息',
      selectedModel: String(record.selectedModel || '').trim(),
      quickTestResponseContent: record.quickTestResponseContent || '',
    };
    const payload = {
      format: 'api-check-key-export-v1',
      compressed: 'gzip',
      exportedAt: Date.now(),
      records: [normalizedRecord],
    };
    const compressed = await compressClipboardPackage(JSON.stringify(payload));
    await navigator.clipboard.writeText(`sk://${compressed}`);
    message.success('已复制单条 sk:// 导入命令；相同记录会覆盖，不会重复追加');
  } catch (error) {
    console.error(error);
    message.error(`复制导入命令失败：${error.message || '未知错误'}`);
  }
}

function openManualRecordModal(record = null) {
  manualRecordEditing.value = Boolean(record);
  overwriteManualRecordDraft(createManualRecordDraft(record));
  manualRecordModalOpen.value = true;
}

function closeManualRecordModal() {
  manualRecordModalOpen.value = false;
}

async function submitManualRecord() {
  const siteName = String(manualRecordDraft.siteName || '').trim();
  const siteUrl = normalizeSiteUrl(manualRecordDraft.siteUrl);
  const apiKey = normalizeApiKey(manualRecordDraft.apiKey);
  manualRecordDraft.modelsText = normalizeModels([manualRecordDraft.modelsValue]).join(', ');
  manualRecordDraft.selectedModel = String(manualRecordDraft.modelsValue || '').trim();
  if (!siteName || !siteUrl || !apiKey) {
    message.warning('请至少填写网站名称、接口地址和 API Key');
    return;
  }

  manualRecordSaving.value = true;
  try {
    const existingRecord = manualRecordDraft.rowKey
      ? tableData.value.find(item => item.rowKey === manualRecordDraft.rowKey)
      : null;
    const nextRecord = createRecordFromDraft(manualRecordDraft, existingRecord);
    tableData.value = [
      ...tableData.value.filter(item => item.rowKey !== manualRecordDraft.rowKey),
      nextRecord,
    ];
    persistRecords();
    closeManualRecordModal();
    message.success(manualRecordEditing.value ? '记录已更新' : '手工记录已添加');
  } finally {
    manualRecordSaving.value = false;
  }
}

function deleteRecord(record) {
  tableData.value = tableData.value.filter(item => item.rowKey !== record.rowKey);
  persistRecords();
  message.success('记录已删除');
}

async function handleManualModelDropdownVisibleChange(open) {
  if (!open) return;
  await loadManualModelOptions();
}

function handleManualModelSelectionChange(values) {
  const normalizedValue = normalizeModels([values])[0] || '';
  manualRecordDraft.modelsValue = normalizedValue;
  manualRecordDraft.selectedModel = normalizedValue;
  manualRecordDraft.modelsText = normalizedValue;
  mergeManualModelOptions(normalizedValue ? [normalizedValue] : []);
}

async function loadManualModelOptions(force = false) {
  const siteUrl = normalizeSiteUrl(manualRecordDraft.siteUrl);
  const apiKey = normalizeApiKey(manualRecordDraft.apiKey);
  if (!siteUrl || !apiKey) {
    message.warning('请先填写接口地址和 API Key，再获取模型列表');
    return;
  }

  const currentFetchKey = `${siteUrl}::${apiKey}`;
  if (!force && manualModelFetchKey.value === currentFetchKey && manualModelOptions.value.length > 0) {
    return;
  }

  manualModelLoading.value = true;
  try {
    const modelResponse = await fetchModelList(siteUrl, apiKey);
    const rawCandidates = modelResponse?.data || modelResponse?.models || [];
    const historyContext = getBatchHistoryContextByKeys(siteUrl, apiKey);
    const normalizedCandidates = normalizeModels([
      ...getContextModelNames(historyContext),
      ...normalizeModels(rawCandidates),
    ]);
    if (!normalizedCandidates.length) {
      throw new Error('没有获取到可用模型');
    }
    manualModelFetchKey.value = currentFetchKey;
    mergeManualModelOptions(normalizedCandidates);
    if (!manualRecordDraft.modelsValue) {
      const preferred = historyContext?.preferredModel || pickPreferredModel(normalizedCandidates) || normalizedCandidates[0];
      handleManualModelSelectionChange(preferred);
    }
  } catch (error) {
    console.error(error);
    message.error(`获取模型列表失败：${error.message || '未知错误'}`);
  } finally {
    manualModelLoading.value = false;
  }
}

function mergeManualModelOptions(values) {
  const merged = normalizeModels([
    ...manualModelOptions.value.map(option => option.value),
    ...values,
  ]);
  manualModelOptions.value = merged.map(value => ({ label: value, value }));
}

async function compressClipboardPackage(text) {
  if (typeof CompressionStream !== 'function') {
    throw new Error('当前环境不支持压缩导出');
  }
  const source = new Blob([new TextEncoder().encode(String(text || ''))]).stream();
  const compressed = source.pipeThrough(new CompressionStream('gzip'));
  const arrayBuffer = await new Response(compressed).arrayBuffer();
  return bytesToBase64Url(new Uint8Array(arrayBuffer));
}

async function decompressClipboardPackage(text) {
  if (typeof DecompressionStream !== 'function') {
    throw new Error('当前环境不支持压缩导入');
  }
  const bytes = base64UrlToBytes(text);
  const decompressed = new Blob([bytes]).stream().pipeThrough(new DecompressionStream('gzip'));
  return await new Response(decompressed).text();
}

function bytesToBase64Url(bytes) {
  let binary = '';
  const chunkSize = 0x8000;
  for (let index = 0; index < bytes.length; index += chunkSize) {
    const chunk = bytes.subarray(index, index + chunkSize);
    binary += String.fromCharCode(...chunk);
  }
  return btoa(binary).replace(/\+/g, '-').replace(/\//g, '_').replace(/=+$/g, '');
}

function base64UrlToBytes(value) {
  const normalized = String(value || '').replace(/-/g, '+').replace(/_/g, '/');
  const padding = normalized.length % 4 === 0 ? '' : '='.repeat(4 - (normalized.length % 4));
  const binary = atob(`${normalized}${padding}`);
  const bytes = new Uint8Array(binary.length);
  for (let index = 0; index < binary.length; index += 1) {
    bytes[index] = binary.charCodeAt(index);
  }
  return bytes;
}

async function exportCsv() {
  if (!displayedRows.value.length) return;
  let csv = '\uFEFF网站,Token名称,API Key,接口地址,模型候选,状态,最近同步,最近快测,快测结果\n';
  displayedRows.value.forEach(record => {
    csv += [`"${record.siteName}"`, `"${record.tokenName || ''}"`, `"${record.apiKey}"`, `"${record.siteUrl}"`, `"${record.modelsText || ''}"`, `"${record.status === 1 ? '正常' : '禁用/异常'}"`, `"${formatDateTime(record.updatedAt)}"`, `"${formatDateTime(record.quickTestAt)}"`, `"${record.quickTestRemark || ''}"`].join(',');
    csv += '\n';
  });
  const filename = `key-management-${Date.now()}.csv`;
  if (isWailsRuntime) {
    const savedPath = await ExportTextFile(csv, filename);
    if (savedPath) {
      message.success(`CSV 已导出：${savedPath}`);
    }
    return;
  }

  const anchor = document.createElement('a');
  anchor.href = `data:text/csv;charset=utf-8,${encodeURIComponent(csv)}`;
  anchor.download = filename;
  anchor.click();
}

function clearLocalRecords() {
  tableData.value = [];
  allResults.value = [];
  syncMeta.value = { lastBatchSyncAt: null, lastBatchSyncCount: 0, lastBatchFailedCount: 0 };
  localStorage.removeItem(STORAGE_KEY);
  localStorage.removeItem(MANUAL_STORAGE_KEY);
  localStorage.removeItem(META_STORAGE_KEY);
  message.success('本地密钥库已清空');
}

function deleteAbnormalRecords() {
  const abnormalCount = abnormalKeyCount.value;
  if (abnormalCount <= 0) {
    message.warning('当前没有异常密钥可删除');
    return;
  }
  tableData.value = tableData.value.filter(record => Number(record?.status || 0) === 1);
  persistRecords();
  message.success(`已删除 ${abnormalCount} 条异常密钥`);
}

function confirmDeleteAbnormalRecords() {
  if (batchDeleteAbnormalDisabled.value) return;
  Modal.confirm({
    title: '确认批量删除异常密钥？',
    content: `将删除 ${abnormalKeyCount.value} 条状态为“禁用/异常”的记录，正常密钥不会受影响。`,
    okText: '删除',
    cancelText: '取消',
    okButtonProps: { danger: true },
    onOk: deleteAbnormalRecords,
  });
}

function launchCherryStudio(record) {
  if (!record.apiKey || !record.siteUrl) {
    message.warning('配置不完整，无法导出');
    return;
  }
  const payload = { id: `key-${record.rowKey}`, baseUrl: normalizeSiteUrl(record.siteUrl), apiKey: record.apiKey, name: `${record.siteName}${record.quickTestModel ? ` (${record.quickTestModel})` : ''}` };
  try {
    const encoded = btoa(String.fromCharCode(...new TextEncoder().encode(JSON.stringify(payload))));
    window.open(`cherrystudio://providers/api-keys?v=1&data=${encoded}`, '_blank');
    message.success('正在尝试唤起 Cherry Studio');
  } catch (error) {
    console.error(error);
    message.error(`导出 Cherry Studio 失败：${error.message || '未知错误'}`);
  }
}

function launchCCSwitch(record, targetApp = 'claude') {
  if (!record.apiKey || !record.siteUrl) {
    message.warning('配置不完整，无法导出');
    return;
  }
  const params = new URLSearchParams();
  params.set('resource', 'provider');
  params.set('app', targetApp);
  params.set('name', `${record.siteName}${record.quickTestModel ? ` - ${record.quickTestModel}` : ''}`);
  params.set('homepage', normalizeSiteUrl(record.siteUrl));
  params.set('endpoint', normalizeCCSwitchEndpoint(record.siteUrl, targetApp));
  params.set('apiKey', record.apiKey);
  if (record.quickTestModel) params.set('model', record.quickTestModel);
  const schemaUrl = `ccswitch://v1/import?${params.toString()}`;
  const platform = String(
    navigator?.userAgentData?.platform ||
    navigator?.platform ||
    navigator?.userAgent ||
    ''
  ).toLowerCase();
  if (platform.includes('mac')) {
    openUrlInSystemBrowser(schemaUrl);
  } else {
    window.open(schemaUrl, '_blank');
  }
  message.success(`正在尝试唤起 CC Switch (${targetApp})`);
}

function openDesktopConfigWizard(record) {
  if (!isDesktopConfigBridgeAvailable()) {
    message.warning('专属一键配置仅支持桌面版 EXE 运行环境');
    return;
  }
  desktopConfigTargetRecord.value = record;
  desktopConfigPreview.value = { appGroups: [], writes: [], errors: [] };
  overwriteDesktopConfigDraft(createDesktopConfigDraft(record));
  desktopConfigModalOpen.value = true;
  void syncDesktopProviderKeyFromSnapshot();
}

async function generateDesktopConfigPreview() {
  if (!desktopConfigDraft.selectedApps.length) {
    message.warning('请至少选择一个目标应用');
    return;
  }
  desktopConfigLoading.value = true;
  try {
    const snapshot = await readManagedAppConfigFiles(desktopConfigDraft.selectedApps);
    const preview = buildDesktopConfigPreview(desktopConfigDraft, snapshot);
    desktopConfigPreview.value = preview;
    if (!preview.appGroups.length && preview.errors.length) throw new Error(preview.errors.join('；'));
    desktopConfigDiffOpen.value = true;
    if (preview.errors.length) message.warning(`部分应用预览生成失败：${preview.errors.join('；')}`);
    else message.success(`已生成 ${preview.writes.length} 个配置文件的变更预览`);
  } catch (error) {
    console.error(error);
    message.error(`生成配置预览失败：${error.message || '未知错误'}`);
  } finally {
    desktopConfigLoading.value = false;
  }
}

async function applyDesktopConfigPreview() {
  if (!desktopConfigPreview.value.writes.length) {
    message.warning('没有可写入的配置变更');
    return;
  }
  desktopConfigLoading.value = true;
  try {
    const result = await applyManagedAppConfigFiles(desktopConfigPreview.value.writes);
    const appliedCount = Array.isArray(result?.applied) ? result.applied.length : 0;
    desktopConfigDiffOpen.value = false;
    desktopConfigModalOpen.value = false;
    message.success(`已写入 ${appliedCount} 个本地配置文件，并自动创建备份`);
  } catch (error) {
    console.error(error);
    message.error(`写入本地配置失败：${error.message || '未知错误'}`);
  } finally {
    desktopConfigLoading.value = false;
  }
}

function overwriteDesktopConfigDraft(nextDraft) {
  Object.keys(desktopConfigDraft).forEach(key => delete desktopConfigDraft[key]);
  Object.assign(desktopConfigDraft, nextDraft);
  desktopProviderKeyManualValue.value = String(nextDraft?.providerKey || '').trim();
}

function isDesktopAppSelected(appId) {
  return Array.isArray(desktopConfigDraft.selectedApps) && desktopConfigDraft.selectedApps.includes(appId);
}

function toggleDesktopAppSelection(appId) {
  const current = Array.isArray(desktopConfigDraft.selectedApps) ? [...desktopConfigDraft.selectedApps] : [];
  if (current.includes(appId)) {
    desktopConfigDraft.selectedApps = current.filter(item => item !== appId);
  } else {
    desktopConfigDraft.selectedApps = [...current, appId];
  }
  if (desktopConfigDraft.forceCustomProviderKey === false) {
    void syncDesktopProviderKeyFromSnapshot();
  }
}

function handleDesktopProviderKeyModeChange(event) {
  const checked = Boolean(event?.target?.checked);
  if (!checked && desktopConfigDraft.forceCustomProviderKey === false) {
    desktopProviderKeyManualValue.value = String(desktopConfigDraft.providerKey || '').trim();
  }
  desktopConfigDraft.forceCustomProviderKey = checked;
  const fallbackManualValue = String(desktopProviderKeyManualValue.value || '').trim();
  desktopConfigDraft.providerKey = checked
    ? 'custom'
    : (fallbackManualValue && fallbackManualValue !== 'custom' ? fallbackManualValue : '');
  if (!checked && !String(desktopConfigDraft.providerKey || '').trim()) {
    void syncDesktopProviderKeyFromSnapshot();
  }
}

async function syncDesktopProviderKeyFromSnapshot() {
  if (!isDesktopConfigBridgeAvailable()) return;
  try {
    const selectedApps = Array.isArray(desktopConfigDraft.selectedApps) && desktopConfigDraft.selectedApps.length
      ? desktopConfigDraft.selectedApps
      : DESKTOP_CONFIG_APPS.map(app => app.id);
    const snapshot = await readManagedAppConfigFiles(selectedApps);
    const inferred = inferProviderKeyFromSnapshot(snapshot, desktopConfigDraft, selectedApps);
    const detected = String(inferred?.providerKey || '').trim();
    if (!detected) return;
    desktopProviderKeyManualValue.value = detected;
    if (desktopConfigDraft.forceCustomProviderKey === false) {
      desktopConfigDraft.providerKey = detected;
    }
  } catch {}
}

function overwriteManualRecordDraft(nextDraft) {
  Object.keys(manualRecordDraft).forEach(key => delete manualRecordDraft[key]);
  Object.assign(manualRecordDraft, nextDraft);
  manualModelFetchKey.value = '';
  mergeManualModelOptions(normalizeModels([nextDraft.modelsValue]));
}

function getQuickTestTooltip(record) {
  if (!record.quickTestStatus) return '尚未执行快速对话测试';
  return [
    `结果：${record.quickTestLabel || record.quickTestStatus}`,
    record.quickTestModel ? `模型：${record.quickTestModel}` : '',
    ...getPerformanceTooltipLines(record),
    record.quickTestRemark ? `说明：${record.quickTestRemark}` : '',
    record.quickTestResponseContent ? `内容：${record.quickTestResponseContent}` : '',
    record.quickTestAt ? `时间：${formatDateTime(record.quickTestAt)}` : '',
  ].filter(Boolean).join('\n');
}

function getPerformanceTooltipLines(record) {
  return buildPerformanceTooltipLines(record);
}

function canRefreshBalance(record) {
  return Boolean(getBatchHistoryContext(record)?.accountData);
}

function getRecordBalanceValue(record) {
  const directLabel = normalizeBalanceLabel(record?.balanceLabel);
  if (directLabel) return formatBalanceDisplay(directLabel);
  if (record?.unlimitedQuota) return '无限';
  const remainQuota = Number(record?.remainQuota);
  if (Number.isFinite(remainQuota)) {
    return formatBalanceAmount(remainQuota);
  }
  return '';
}

function getRecordBalanceNumericText(record) {
  const value = getRecordBalanceValue(record);
  if (!value || value === '无限') return value;
  return value.replace(/\s*USD$/i, '').trim();
}

function showBalanceUnit(record) {
  const value = getRecordBalanceValue(record);
  return Boolean(value && value !== '无限');
}

function getBalanceRelativeTime(record) {
  if (record?.balanceLoading) return '刷新中';
  const timestamp = Number(record?.balanceUpdatedAt || 0);
  if (!timestamp) return '未刷新';
  const diffMs = Math.max(0, Date.now() - timestamp);
  const minute = 60 * 1000;
  const hour = 60 * minute;
  const day = 24 * hour;
  if (diffMs < minute) return '刚刚';
  if (diffMs < hour) return `${Math.floor(diffMs / minute)} 分钟前`;
  if (diffMs < day) return `${Math.floor(diffMs / hour)} 小时前`;
  return `${Math.floor(diffMs / day)} 天前`;
}

function getRecordBalanceTooltip(record) {
  const lines = [];
  const balanceText = getRecordBalanceValue(record);
  if (balanceText) lines.push(`余额 ${balanceText}`);
  const usedQuota = Number(record?.usedQuota);
  if (Number.isFinite(usedQuota)) {
    lines.push(`已用 ${formatBalanceAmount(usedQuota)}`);
  }
  if (record?.balanceUpdatedAt) {
    lines.push(`更新时间 ${formatDateTime(record.balanceUpdatedAt)}`);
  }
  if (record?.balanceError) {
    lines.push(`刷新失败 ${record.balanceError}`);
  }
  return lines.join('\n') || '暂无余额信息';
}

function normalizeBalanceLabel(value) {
  const text = String(value || '').trim();
  if (!text) return '';
  if (/^\$?-?\d/.test(text) || /USD$/i.test(text)) return text;
  if (/^无限/.test(text)) return text;
  return '';
}

function formatBalanceAmount(rawAmount) {
  const amount = Number(rawAmount);
  if (!Number.isFinite(amount)) return '';
  const finalAmount = amount < 100000 ? amount.toFixed(2) : (amount / 500000).toFixed(2);
  return `${finalAmount} USD`;
}

function formatBalanceDisplay(value) {
  const text = String(value || '').trim();
  if (!text) return '';
  if (/^无限/.test(text)) return '无限';
  if (/USD$/i.test(text)) return text.replace(/\s+/g, ' ');
  if (text.startsWith('$')) return `${text.slice(1)} USD`;
  return text;
}

async function refreshRecordBalance(record, { silent = false } = {}) {
  if (!canRefreshBalance(record) || record.balanceLoading) return;
  record.balanceLoading = true;
  record.balanceError = '';
  try {
    const snapshot = await fetchRecordBalanceSnapshot(record);
    record.balanceLabel = snapshot.balanceLabel || '';
    record.remainQuota = snapshot.remainQuota ?? record.remainQuota ?? null;
    record.usedQuota = snapshot.usedQuota ?? record.usedQuota ?? null;
    record.unlimitedQuota = snapshot.unlimitedQuota === true;
    record.balanceUpdatedAt = Date.now();
    persistRecords();
    if (!silent) {
      message.success(`已刷新 ${record.siteName} 余额`);
    }
  } catch (error) {
    record.balanceError = error.message || '未知错误';
    persistRecords();
    if (!silent) {
      message.error(`刷新余额失败：${record.balanceError}`);
    }
  } finally {
    record.balanceLoading = false;
    persistRecords();
  }
}

async function fetchRecordBalanceSnapshot(record) {
  const siteUrl = normalizeSiteUrl(record.siteUrl);
  const apiKey = normalizeApiKey(record.apiKey);
  const batchContext = getBatchHistoryContext(record);
  const controller = new AbortController();
  const timer = setTimeout(() => controller.abort(), 15000);

  try {
    if (!batchContext?.accountData) {
      throw new Error('缺少批量检测上下文，无法复用余额刷新逻辑');
    }

    const batchSnapshot = await tryFetchBatchCheckQuota(batchContext.accountData, siteUrl);
    if (batchSnapshot) return batchSnapshot;

    throw new Error('批量检测同款余额接口未返回可识别字段');
  } catch (error) {
    if (error?.name === 'AbortError') {
      throw new Error('请求超时');
    }
    throw error;
  } finally {
    clearTimeout(timer);
  }
}

async function tryFetchBatchCheckQuota(site, siteUrl) {
  const label = await fetchQuotaLabelWithBatchLogic({
    apiFetch,
    site,
    siteUrl,
  });
  if (!isDisplayableQuotaLabel(label)) return null;
  return {
    balanceLabel: label,
    remainQuota: null,
    usedQuota: null,
    unlimitedQuota: /^无限/.test(String(label || '').trim()),
  };
}

function getQuickTestColor(status) {
  if (status === 'success') return 'green';
  if (status === 'warning') return 'orange';
  if (status === 'error') return 'red';
  return 'default';
}

function extractQuickTestResponseContent(messageObj) {
  const candidates = [
    normalizeQuickTestContent(messageObj?.content),
    normalizeQuickTestContent(messageObj?.reasoning_content),
    normalizeQuickTestContent(messageObj?.thinking),
  ].filter(Boolean);
  return candidates[0] || '';
}

function normalizeQuickTestContent(value) {
  if (!value) return '';
  let text = '';

  if (typeof value === 'string') {
    text = value;
  } else if (Array.isArray(value)) {
    text = value
      .map(item => {
        if (typeof item === 'string') return item;
        if (typeof item?.text === 'string') return item.text;
        if (typeof item?.content === 'string') return item.content;
        if (typeof item?.value === 'string') return item.value;
        return '';
      })
      .filter(Boolean)
      .join('\n');
  } else if (typeof value === 'object') {
    text = String(value?.text || value?.content || value?.value || '');
  }

  text = text.replace(/\s+\n/g, '\n').replace(/\n{3,}/g, '\n\n').trim();
  if (text.length > 500) {
    return `${text.slice(0, 500)}...`;
  }
  return text;
}

const buildRowKey = (siteUrl, apiKey) => `${normalizeSiteUrl(siteUrl)}::${String(apiKey || '').trim()}`;
const buildManualRowKey = () => `manual::${Date.now()}::${Math.random().toString(36).slice(2, 8)}`;

function createRecordFromDraft(draft, existingRecord = null) {
  const modelsList = normalizeModels([draft.modelsValue || draft.selectedModel || draft.modelsText]);
  const now = Date.now();
  const sourceType = existingRecord?.sourceType || draft.sourceType || 'manual';
  const isManual = sourceType === 'manual';
  return {
    ...existingRecord,
    rowKey: isManual ? (existingRecord?.rowKey || draft.rowKey || buildManualRowKey()) : buildRowKey(draft.siteUrl, draft.apiKey),
    sourceType,
    siteName: String(draft.siteName || '').trim() || '未命名站点',
    tokenName: String(draft.tokenName || '').trim(),
    siteUrl: normalizeSiteUrl(draft.siteUrl),
    apiKey: normalizeApiKey(draft.apiKey),
    modelsList,
    modelsText: modelsList.length ? modelsList.join(', ') : '未提供模型信息',
    selectedModel: modelsList[0] || '',
    status: Number(draft.status || 1),
    createdAt: existingRecord?.createdAt || now,
    updatedAt: now,
    quickTestStatus: existingRecord?.quickTestStatus || '',
    quickTestLabel: existingRecord?.quickTestLabel || '',
    quickTestModel: existingRecord?.quickTestModel || '',
    quickTestRemark: existingRecord?.quickTestRemark || '',
    quickTestAt: existingRecord?.quickTestAt || null,
    quickTestResponseTime: existingRecord?.quickTestResponseTime || '',
    quickTestTtftMs: existingRecord?.quickTestTtftMs || '',
    quickTestTps: existingRecord?.quickTestTps || '',
    quickTestResponseContent: existingRecord?.quickTestResponseContent || '',
    quickTestLoading: false,
  };
}
function normalizeApiKey(rawKey) {
  let apiKey = String(rawKey || '').trim();
  if (!apiKey) return '';
  if (!apiKey.startsWith('sk-')) apiKey = `sk-${apiKey}`;
  return apiKey;
}

function normalizeSiteUrl(rawUrl) {
  return String(rawUrl || '').trim().replace(/\/+$/, '');
}

function normalizeModels(rawModels) {
  const list = Array.isArray(rawModels) ? rawModels : String(rawModels || '').split(/[\n,，\s]+/).map(item => item.trim());
  return Array.from(new Set(list.map(item => typeof item === 'string' ? item : item?.id || item?.model || '').map(item => String(item || '').trim()).filter(Boolean)));
}

function resolveSiteUrl(result) {
  const explicitSiteUrl = normalizeSiteUrl(result?.site_url);
  if (explicitSiteUrl) return explicitSiteUrl;
  const apiAddress = normalizeSiteUrl(result?.api_url || result?.api_address);
  if (apiAddress) return apiAddress;
  const rawApiKey = String(result?.api_key || '').trim();
  return rawApiKey.startsWith('http://') || rawApiKey.startsWith('https://') ? normalizeSiteUrl(rawApiKey) : '';
}

function extractAccountsFromBackup(parsed) {
  if (Array.isArray(parsed?.accounts?.accounts)) return parsed.accounts.accounts;
  if (Array.isArray(parsed?.accounts)) return parsed.accounts;
  if (Array.isArray(parsed)) return parsed;
  return [];
}

function pickPreferredModel(candidates) {
  const chatCandidates = normalizeModels(candidates).filter(isLikelyChatModel);
  if (!chatCandidates.length) return '';
  const preferredPatterns = [/gpt-5/i, /gpt-4\.1/i, /gpt-4o/i, /^o3/i, /^o1/i, /claude/i, /gemini/i, /deepseek/i, /qwen/i, /grok/i, /kimi/i, /chat/i];
  return chatCandidates.find(model => preferredPatterns.some(pattern => pattern.test(model))) || chatCandidates[0];
}

function getSiteTitleStyle(siteName) {
  const text = String(siteName || '').trim();
  const chars = Array.from(text);
  const minWidth = 80;
  const maxWidth = 106;
  const width = Math.max(minWidth, Math.min(maxWidth, 58 + Math.min(chars.length, 8) * 6));
  const visualUnits = chars.reduce((sum, ch) => sum + (/[\u4e00-\u9fff]/.test(ch) ? 1 : 0.58), 0) || 1;
  const fontSize = Math.max(10, Math.min(17, (width - 4) / visualUnits));
  return {
    width: `${width}px`,
    maxWidth: '100%',
    fontSize: `${fontSize}px`,
    lineHeight: '1.08',
  };
}

function openRecordSiteUrl(record) {
  const target = String(record?.siteUrl || '').trim();
  if (!target) return;
  openUrlInSystemBrowser(target);
}

function isLikelyChatModel(model) {
  return !/(embedding|tts|whisper|speech|audio|image|video|vision|flux|midjourney|mj|rerank|bge|stability|playground|suno|music|ocr|moderation|asr)/i.test(String(model || ''));
}

async function safeReadJson(response) {
  try {
    return await response.json();
  } catch (error) {
    console.warn('Failed to read JSON response', error);
    return null;
  }
}

async function safeReadResponsePayload(response) {
  const contentType = response.headers.get('content-type') || '';
  if (contentType.includes('application/json')) return safeReadJson(response);
  const text = await response.text();
  const htmlTitle = text.match(/<title>(.*?)<\/title>/i)?.[1];
  return { message: htmlTitle || text.slice(0, 300) };
}

function extractReadableError(payload, statusCode) {
  if (!payload) return `HTTP ${statusCode}`;
  return payload?.error?.message || payload?.message || `HTTP ${statusCode}`;
}

function buildQuickTestDiagnosticText(payload, statusCode, requestMeta = {}) {
  const diagnostics = payload?.error?.diagnostics || payload?.diagnostics || null;
  const lines = [extractReadableError(payload, statusCode)];

  if (requestMeta?.siteUrl) {
    lines.push(`输入地址: ${requestMeta.siteUrl}`);
  }
  if (requestMeta?.model) {
    lines.push(`请求模型: ${requestMeta.model}`);
  }
  if (Number.isFinite(Number(requestMeta?.timeoutMs)) && Number(requestMeta.timeoutMs) > 0) {
    lines.push(`超时设置: ${Math.round(Number(requestMeta.timeoutMs) / 1000)}s`);
  }
  if (diagnostics?.resolvedEndpoint) {
    lines.push(`命中端点: ${diagnostics.resolvedEndpoint}`);
  }

  const attempts = Array.isArray(diagnostics?.attempts) ? diagnostics.attempts : [];
  if (attempts.length) {
    lines.push('尝试日志:');
    attempts.forEach((attempt, index) => {
      const status = Number(attempt?.status || 0);
      const endpoint = String(attempt?.endpoint || '').trim();
      const messageText = String(attempt?.message || '').trim();
      lines.push(`${index + 1}. [${status || '?'}] ${endpoint}${messageText ? ` -> ${messageText}` : ''}`);
    });
  }

  return lines.filter(Boolean).join('\n');
}

function createQuickTestError(payload, statusCode, requestMeta = {}) {
  const error = new Error(extractReadableError(payload, statusCode));
  error.detail = buildQuickTestDiagnosticText(payload, statusCode, requestMeta);
  return error;
}

function showQuickTestErrorDialog(detailText) {
  Modal.error({
    title: '快速测活失败',
    width: 760,
    okText: '关闭',
    content: h('div', {
      style: {
        whiteSpace: 'pre-wrap',
        wordBreak: 'break-word',
        maxHeight: '60vh',
        overflow: 'auto',
        fontSize: '12px',
        lineHeight: '1.6',
        fontFamily: 'ui-monospace, SFMono-Regular, Menlo, Consolas, monospace',
      },
    }, detailText),
  });
}

function formatDateTime(timestamp) {
  if (!timestamp) return '未同步';
  try {
    return new Date(timestamp).toLocaleString();
  } catch (error) {
    console.warn('Failed to format timestamp', error);
    return '时间异常';
  }
}

function formatCompactDateTime(timestamp) {
  if (!timestamp) return '未同步';
  try {
    const date = new Date(timestamp);
    if (Number.isNaN(date.getTime())) return '未同步';
    const month = date.getMonth() + 1;
    const day = date.getDate();
    const hours = String(date.getHours()).padStart(2, '0');
    const minutes = String(date.getMinutes()).padStart(2, '0');
    return `${month}/${day} ${hours}:${minutes}`;
  } catch (error) {
    console.warn('Failed to format compact timestamp', error);
    return '时间异常';
  }
}

function loadStoredRecords() {
  try {
    const autoRaw = localStorage.getItem(STORAGE_KEY);
    const manualRaw = localStorage.getItem(MANUAL_STORAGE_KEY);
    const autoRecords = JSON.parse(autoRaw || '[]');
    const manualRecords = JSON.parse(manualRaw || '[]');
    const parsedRecords = [
      ...(Array.isArray(autoRecords) ? autoRecords : []),
      ...(Array.isArray(manualRecords) ? manualRecords : []),
    ];
    return parsedRecords.map(record => ({
      ...record,
      sourceType: record.sourceType || 'auto',
      siteName: record.siteName || '未命名站点',
      tokenName: record.tokenName || '',
      siteUrl: normalizeSiteUrl(record.siteUrl),
      apiKey: String(record.apiKey || '').trim(),
      modelsList: normalizeModels(record.modelsList || record.modelsText),
      modelsText: record.modelsText || '未提供模型信息',
      selectedModel: String(record.selectedModel || '').trim(),
      quickTestStatus: record.quickTestStatus || '',
      quickTestLabel: record.quickTestLabel || '',
      quickTestModel: record.quickTestModel || '',
      quickTestRemark: record.quickTestRemark || '',
      quickTestAt: record.quickTestAt || null,
      quickTestResponseTime: record.quickTestResponseTime || '',
      quickTestTtftMs: record.quickTestTtftMs || '',
      quickTestTps: record.quickTestTps || '',
      quickTestResponseContent: record.quickTestResponseContent || '',
      balanceLabel: record.balanceLabel || '',
      balanceUpdatedAt: record.balanceUpdatedAt || null,
      balanceError: record.balanceError || '',
      balanceLoading: false,
      quickTestLoading: false,
      rowKey: record.rowKey || (record.sourceType === 'manual' ? buildManualRowKey() : buildRowKey(record.siteUrl, record.apiKey)),
    })).map(hydrateRecordModelSelection).filter(record => record.siteUrl && record.apiKey);

    const raw = localStorage.getItem(STORAGE_KEY);
    const parsed = JSON.parse(raw || '[]');
    if (!Array.isArray(parsed)) return [];
    return parsed.map(record => ({
      ...record,
      siteName: record.siteName || '未命名站点',
      tokenName: record.tokenName || '',
      siteUrl: normalizeSiteUrl(record.siteUrl),
      apiKey: String(record.apiKey || '').trim(),
      modelsList: normalizeModels(record.modelsList || record.modelsText),
      modelsText: record.modelsText || '未提供模型信息',
      selectedModel: String(record.selectedModel || '').trim(),
      quickTestStatus: record.quickTestStatus || '',
      quickTestLabel: record.quickTestLabel || '',
      quickTestModel: record.quickTestModel || '',
      quickTestRemark: record.quickTestRemark || '',
      quickTestAt: record.quickTestAt || null,
      quickTestResponseTime: record.quickTestResponseTime || '',
      quickTestTtftMs: record.quickTestTtftMs || '',
      quickTestTps: record.quickTestTps || '',
      quickTestResponseContent: record.quickTestResponseContent || '',
      balanceLabel: record.balanceLabel || '',
      balanceUpdatedAt: record.balanceUpdatedAt || null,
      balanceError: record.balanceError || '',
      balanceLoading: false,
      quickTestLoading: false,
      rowKey: record.rowKey || buildRowKey(record.siteUrl, record.apiKey),
    })).map(hydrateRecordModelSelection).filter(record => record.siteUrl && record.apiKey);
  } catch (error) {
    console.error(error);
    return [];
  }
}

function loadStoredMeta() {
  try {
    const raw = localStorage.getItem(META_STORAGE_KEY);
    const parsed = JSON.parse(raw || '{}');
    return {
      lastBatchSyncAt: parsed?.lastBatchSyncAt || null,
      lastBatchSyncCount: parsed?.lastBatchSyncCount || 0,
      lastBatchFailedCount: parsed?.lastBatchFailedCount || 0,
    };
  } catch (error) {
    console.error(error);
    return { lastBatchSyncAt: null, lastBatchSyncCount: 0, lastBatchFailedCount: 0 };
  }
}

function loadBatchHistoryBalanceMap() {
  try {
    const raw = getCachedLastResultsSnapshotRaw();
    const parsed = JSON.parse(raw || '[]');
    if (!Array.isArray(parsed)) return new Map();

    const balanceMap = new Map();
    parsed.forEach(item => {
      const siteUrl = normalizeSiteUrl(item?.siteUrl);
      const apiKey = String(item?.apiKey || '').trim();
      const balanceLabel = normalizeBalanceLabel(item?.quota);
      if (!siteUrl || !apiKey || !balanceLabel) return;
      const rowKey = buildRowKey(siteUrl, apiKey);
      const updatedAt = Number(item?.updatedAt || item?.finishedAt || item?.completedAt || item?.timestamp || Date.now());
      const current = balanceMap.get(rowKey);
      if (!current || updatedAt >= current.balanceUpdatedAt) {
        balanceMap.set(rowKey, {
          balanceLabel,
          balanceUpdatedAt: updatedAt,
        });
      }
    });
    return balanceMap;
  } catch (error) {
    console.error(error);
    return new Map();
  }
}

function loadBatchHistoryContextMap() {
  try {
    const raw = getCachedLastResultsSnapshotRaw();
    const parsed = JSON.parse(raw || '[]');
    if (!Array.isArray(parsed)) return new Map();

    const groupedContextMap = new Map();
    parsed.forEach(item => {
      const siteUrl = normalizeSiteUrl(item?.siteUrl);
      const apiKey = String(item?.apiKey || '').trim();
      if (!siteUrl || !apiKey) return;
      const rowKey = buildRowKey(siteUrl, apiKey);
      const updatedAt = Number(item?.updatedAt || item?.finishedAt || item?.completedAt || item?.timestamp || Date.now());
      const modelName = String(item?.modelName || '').trim();
      const current = groupedContextMap.get(rowKey) || {
        updatedAt: 0,
        accountData: null,
        tasksByModel: new Map(),
      };
      if (updatedAt >= current.updatedAt && item?.accountData) {
        current.accountData = item.accountData;
      }
      current.updatedAt = Math.max(current.updatedAt, updatedAt);
      if (modelName) {
        const taskSnapshot = {
          modelName,
          status: String(item?.status || '').trim(),
          statusText: String(item?.statusText || '').trim(),
          responseTime: String(item?.responseTime || '').trim(),
          modelSuffix: String(item?.modelSuffix || '').trim(),
          remark: String(item?.remark || '').trim(),
          updatedAt,
        };
        const previousTask = current.tasksByModel.get(modelName);
        if (!previousTask || compareHistoryTasks(taskSnapshot, previousTask) < 0) {
          current.tasksByModel.set(modelName, taskSnapshot);
        }
      }
      groupedContextMap.set(rowKey, current);
    });

    const contextMap = new Map();
    groupedContextMap.forEach((context, rowKey) => {
      const tasks = Array.from(context.tasksByModel.values()).sort(compareHistoryTasks);
      const preferredTask = tasks.find(isUsableHistoryTask) || tasks[0] || null;
      contextMap.set(rowKey, {
        updatedAt: context.updatedAt,
        accountData: context.accountData,
        tasks,
        preferredTask,
        preferredModel: preferredTask?.modelName || '',
      });
    });
    return contextMap;
  } catch (error) {
    console.error(error);
    return new Map();
  }
}

function getBatchHistoryContextByKeys(siteUrl, apiKey) {
  return batchHistoryContextMap.value.get(buildRowKey(siteUrl, apiKey)) || null;
}

function getBatchHistoryContext(record) {
  return getBatchHistoryContextByKeys(record?.siteUrl, record?.apiKey);
}

function mergeBatchHistoryBalances(records) {
  const balanceMap = loadBatchHistoryBalanceMap();
  if (!balanceMap.size) return records;

  return records.map(record => {
    const snapshot = balanceMap.get(buildRowKey(record.siteUrl, record.apiKey));
    if (!snapshot) return record;
    const currentUpdatedAt = Number(record.balanceUpdatedAt || 0);
    if (currentUpdatedAt && currentUpdatedAt >= snapshot.balanceUpdatedAt) return record;
    return {
      ...record,
      balanceLabel: snapshot.balanceLabel,
      balanceUpdatedAt: snapshot.balanceUpdatedAt,
    };
  });
}

function compareHistoryTasks(left, right) {
  const leftWeight = getHistoryTaskWeight(left);
  const rightWeight = getHistoryTaskWeight(right);
  if (leftWeight !== rightWeight) return rightWeight - leftWeight;
  const leftUpdatedAt = Number(left?.updatedAt || 0);
  const rightUpdatedAt = Number(right?.updatedAt || 0);
  if (leftUpdatedAt !== rightUpdatedAt) return rightUpdatedAt - leftUpdatedAt;
  return String(left?.modelName || '').localeCompare(String(right?.modelName || ''));
}

function getHistoryTaskWeight(task) {
  const status = String(task?.status || '').trim();
  if (status === 'success') return 3;
  if (status === 'warning') return 2;
  if (status === 'pending') return 1;
  return 0;
}

function isUsableHistoryTask(task) {
  const status = String(task?.status || '').trim();
  return Boolean(task?.modelName) && (status === 'success' || status === 'warning');
}

function getContextModelNames(context) {
  return Array.isArray(context?.tasks) ? context.tasks.map(task => task?.modelName).filter(Boolean) : [];
}

function buildHistoryTaskSummary(task) {
  if (!task) return '';
  const suffix = String(task?.modelSuffix || '').replace(/[()]/g, '').trim();
  const statusText = String(task?.statusText || '').trim() || (String(task?.status || '').trim() === 'success' ? '一致可用' : '');
  const responseTime = String(task?.responseTime || '').trim();
  return [suffix, statusText, responseTime ? `${responseTime}s` : ''].filter(Boolean).join(' / ');
}

function buildModelOptionLabel(model, task = null) {
  const summary = buildHistoryTaskSummary(task);
  return summary ? `${model} (${summary})` : model;
}

function buildMergedModelList(record, context = getBatchHistoryContext(record)) {
  return normalizeModels([
    ...getContextModelNames(context),
    ...(Array.isArray(record?.modelsList) ? record.modelsList : []),
    record?.selectedModel || '',
    record?.quickTestModel || '',
  ]);
}

function hydrateRecordModelSelection(record) {
  const context = getBatchHistoryContext(record);
  const modelsList = buildMergedModelList(record, context);
  const selectedModel = String(record?.selectedModel || '').trim();
  const preferredModel = context?.preferredModel || pickPreferredModel(modelsList) || '';
  const nextSelectedModel = modelsList.includes(selectedModel)
    ? selectedModel
    : (modelsList.includes(preferredModel) ? preferredModel : (modelsList[0] || ''));
  return {
    ...record,
    modelsList,
    modelsText: modelsList.join(', ') || '未提供模型信息',
    selectedModel: nextSelectedModel,
    modelLoading: false,
  };
}

function getRecordRenderMeta(record) {
  const context = getBatchHistoryContext(record);
  const baseModels = Array.isArray(record?.modelsList) ? record.modelsList : normalizeModels(record?.modelsText);
  const signature = [
    record?.rowKey || '',
    Number(context?.updatedAt || 0),
    String(record?.selectedModel || '').trim(),
    String(record?.quickTestModel || '').trim(),
    baseModels.join('|'),
  ].join('::');

  const cached = recordRenderMetaCache.get(record?.rowKey || '');
  if (cached?.signature === signature) {
    return cached.value;
  }

  const modelsList = buildMergedModelList(record, context);
  const taskMap = new Map((Array.isArray(context?.tasks) ? context.tasks : []).map(task => [task.modelName, task]));
  const selectedModel = String(record?.selectedModel || '').trim();
  const selectedTask = selectedModel ? (taskMap.get(selectedModel) || null) : null;
  const summary = buildHistoryTaskSummary(selectedTask);
  const value = {
    options: modelsList.map(model => ({
      label: buildModelOptionLabel(model, taskMap.get(model) || null),
      value: model,
    })),
    selectedTask,
    tooltip: selectedModel ? (summary ? `${selectedModel} (${summary})` : selectedModel) : (record?.modelsText || '未提供模型信息'),
  };
  recordRenderMetaCache.set(record?.rowKey || '', { signature, value });
  return value;
}

function getRecordModelOptions(record) {
  return getRecordRenderMeta(record).options;
}

function getRecordSelectedModelTask(record) {
  return getRecordRenderMeta(record).selectedTask || null;
}

function getRecordModelTooltip(record) {
  return getRecordRenderMeta(record).tooltip;
}

async function loadRecordModelOptions(record, force = false) {
  if (!record?.siteUrl || !record?.apiKey) return;
  const currentFetchKey = `${normalizeSiteUrl(record.siteUrl)}::${normalizeApiKey(record.apiKey)}`;
  if (!force && record.modelFetchKey === currentFetchKey && Array.isArray(record.modelsList) && record.modelsList.length > 0) {
    return;
  }

  record.modelLoading = true;
  try {
    const modelResponse = await fetchModelList(record.siteUrl, record.apiKey);
    const rawCandidates = modelResponse?.data || modelResponse?.models || [];
    const normalizedCandidates = normalizeModels(rawCandidates);
    const context = getBatchHistoryContext(record);
    const mergedModels = normalizeModels([
      ...getContextModelNames(context),
      ...normalizedCandidates,
      ...(Array.isArray(record.modelsList) ? record.modelsList : []),
    ]);
    if (!mergedModels.length) {
      throw new Error('没有获取到可用模型');
    }
    record.modelsList = mergedModels;
    record.modelsText = mergedModels.join(', ');
    record.modelFetchKey = currentFetchKey;
    if (!record.selectedModel || !mergedModels.includes(record.selectedModel)) {
      record.selectedModel = context?.preferredModel || pickPreferredModel(mergedModels) || mergedModels[0] || '';
    }
    persistRecords();
  } catch (error) {
    console.error(error);
    message.error(`获取模型列表失败：${error.message || '未知错误'}`);
  } finally {
    record.modelLoading = false;
  }
}

async function handleRecordModelDropdownVisibleChange(record, open) {
  if (!open) return;
  await loadRecordModelOptions(record);
}

function handleRecordModelSelectionChange(record, value) {
  const normalizedValue = normalizeModels([value])[0] || '';
  record.selectedModel = normalizedValue;
  if (normalizedValue && !normalizeModels(record.modelsList).includes(normalizedValue)) {
    record.modelsList = normalizeModels([...(record.modelsList || []), normalizedValue]);
    record.modelsText = record.modelsList.join(', ');
  }
  persistRecords();
}

async function autoRefreshKeyBalancesOnce() {
  if (keyBalanceRefreshBootstrapped.value) return;
  keyBalanceRefreshBootstrapped.value = true;

  const targets = tableData.value.filter(record => canRefreshBalance(record));
  if (!targets.length) return;

  const concurrency = 4;
  let cursor = 0;
  const worker = async () => {
    while (cursor < targets.length) {
      const currentIndex = cursor;
      cursor += 1;
      const record = targets[currentIndex];
      if (!record) continue;
      await refreshRecordBalance(record, { silent: true });
    }
  };

  await Promise.allSettled(
    Array.from({ length: Math.min(concurrency, targets.length) }, () => worker())
  );
}

function persistRecords() {
  schedulePersistRecords();
}

function createPersistRecordsSnapshot() {
  const autoRecords = [];
  const manualRecords = [];
  tableData.value.forEach(({ quickTestLoading, balanceLoading, modelLoading, modelFetchKey, ...record }) => {
    const normalizedRecord = {
      ...record,
      sourceType: record.sourceType || 'auto',
      modelsList: normalizeModels(record.modelsList || record.modelsText),
      selectedModel: String(record.selectedModel || '').trim(),
      quickTestResponseContent: record.quickTestResponseContent || '',
      rowKey: record.rowKey || (record.sourceType === 'manual' ? buildManualRowKey() : buildRowKey(record.siteUrl, record.apiKey)),
    };
    if (normalizedRecord.sourceType === 'manual') manualRecords.push(normalizedRecord);
    else autoRecords.push(normalizedRecord);
  });
  return {
    autoJson: JSON.stringify(autoRecords),
    manualJson: JSON.stringify(manualRecords),
  };
}

function flushPersistRecords() {
  if (persistRecordsTimer) {
    clearTimeout(persistRecordsTimer);
    persistRecordsTimer = null;
  }
  const snapshot = createPersistRecordsSnapshot();
  const signature = `${snapshot.autoJson}\n${snapshot.manualJson}`;
  if (signature === lastPersistedRecordsSnapshot) return;
  localStorage.setItem(STORAGE_KEY, snapshot.autoJson);
  localStorage.setItem(MANUAL_STORAGE_KEY, snapshot.manualJson);
  lastPersistedRecordsSnapshot = signature;
}

function schedulePersistRecords() {
  if (persistRecordsTimer) {
    clearTimeout(persistRecordsTimer);
  }
  persistRecordsTimer = setTimeout(() => {
    persistRecordsTimer = null;
    flushPersistRecords();
  }, PERSIST_DEBOUNCE_MS);
}

function persistMeta() {
  localStorage.setItem(META_STORAGE_KEY, JSON.stringify(syncMeta.value));
}
</script>

<style scoped>
.batch-wrapper.key-management-wrapper{min-height:calc(var(--vh,1vh) * 100);padding:0;overflow:hidden}
.batch-shell.key-management-shell{width:100%;min-height:calc(var(--vh,1vh) * 100);position:relative;isolation:isolate;overflow:hidden}
.batch-page-content.key-management-page-content{background:transparent;border-radius:0;box-shadow:none;padding:2px;min-height:calc(var(--vh,1vh) * 100);position:relative;z-index:1}
.batch-page-container.key-management-page-container{max-width:100% !important;padding:8px 8px 0 !important;margin:0 auto !important;min-height:calc(var(--vh,1vh) * 100 - 4px);display:flex}
.batch-forest-scene{position:absolute;inset:0;overflow:hidden;pointer-events:none;z-index:0;background:radial-gradient(circle at 16% 18%,rgba(164,213,120,.14),transparent 24%),radial-gradient(circle at 84% 14%,rgba(255,213,116,.14),transparent 22%),linear-gradient(180deg,rgba(8,18,12,.14) 0%,rgba(8,20,13,.34) 42%,rgba(6,16,10,.62) 100%),url('/forest-batch-bg-v2.png') center center/cover no-repeat;opacity:.92}
.forest-mist,.forest-path-glow,.forest-firegrass,.forest-slime{position:absolute}
.forest-mist{top:8%;width:34%;height:44%;border-radius:999px;background:radial-gradient(circle,rgba(210,255,232,.12) 0%,rgba(210,255,232,.02) 56%,transparent 74%);filter:blur(12px)}
.forest-mist-left{left:-10%}
.forest-mist-right{right:-8%;top:12%}
.forest-path-glow{left:50%;bottom:-12%;width:min(460px,42vw);height:42%;transform:translateX(-50%);background:radial-gradient(ellipse at center bottom,rgba(255,214,126,.22) 0%,rgba(212,255,182,.12) 24%,rgba(30,58,33,0) 72%);clip-path:polygon(47% 100%,53% 100%,65% 76%,60% 56%,67% 33%,57% 0,43% 0,33% 33%,40% 56%,35% 76%);filter:blur(8px);opacity:.9}
.forest-firegrass{bottom:-4px;width:188px;height:122px;background:url('/forest-firegrass-sprite-v2.png') left bottom/auto 100% no-repeat;filter:drop-shadow(0 6px 12px rgba(18,38,22,.2));opacity:.98}
.firegrass-left{left:8px}
.firegrass-right{right:8px;transform:scaleX(-1);transform-origin:center bottom}
.forest-slime{bottom:26px;width:26px;height:22px;border-radius:58% 58% 46% 46%;background:radial-gradient(circle at 36% 36%,rgba(255,255,255,.9) 0 10%,transparent 11%),radial-gradient(circle at 64% 36%,rgba(255,255,255,.9) 0 10%,transparent 11%),radial-gradient(circle at 40% 40%,rgba(20,34,21,.86) 0 3%,transparent 4%),radial-gradient(circle at 60% 40%,rgba(20,34,21,.86) 0 3%,transparent 4%),radial-gradient(circle at 50% 72%,rgba(18,72,42,.44) 0 14%,transparent 15%),linear-gradient(180deg,rgba(177,255,149,.98),rgba(70,177,88,.94));box-shadow:inset 0 2px 0 rgba(255,255,255,.45),0 10px 16px rgba(14,38,18,.24),0 0 10px rgba(154,255,142,.18)}
.slime-a{left:44%}
.slime-b{left:51%;width:20px;height:17px}
.slime-c{left:57%;width:18px;height:15px}
.key-management{width:100%;padding:0;min-height:100%;display:flex;flex:1 1 auto;flex-direction:column;gap:6px;position:relative;overflow:visible;border-radius:0;background:transparent;box-shadow:none}
.key-management-compact{padding:12px;gap:12px;min-height:100%;background:linear-gradient(180deg,#f8fafc,#eef2ff)}
.key-management>*{position:relative;z-index:1}
.compact-sidebar-summary{display:flex;flex-direction:column;gap:10px}
.compact-sidebar-heading{display:flex;align-items:center;justify-content:space-between;gap:12px;flex-wrap:wrap}
.compact-sidebar-alert{margin:0}
.sync-card,.inventory-card{width:100%}
.inventory-card{flex:1 1 auto;display:flex;flex-direction:column;min-height:0;overflow:hidden;border:0 !important;border-radius:0 !important;background:linear-gradient(180deg,rgba(228,233,226,.96),rgba(214,220,212,.92)) !important;box-shadow:none !important}
.inventory-card :deep(.ant-card-head),.inventory-card :deep(.ant-card-body){background:transparent}
.inventory-card :deep(.ant-card-head){border-bottom-color:rgba(114,132,103,.08)}
.inventory-card :deep(.ant-card-body){display:flex;flex:1 1 auto;flex-direction:column;min-height:0}
.inventory-card :deep(.ant-empty){margin-block:auto}
.sync-card{position:relative;overflow:hidden;border-radius:18px}
.sync-card :deep(.ant-card-body){padding:10px 12px}
.sync-toolbar,.sync-meta,.quick-test-cell,.site-cell,.time-cell{display:flex;gap:12px;flex-wrap:wrap}
.sync-toolbar{display:grid;grid-template-columns:max-content max-content minmax(240px,1fr) auto;align-items:center;column-gap:14px;row-gap:4px}
.sync-meta,.site-cell,.time-cell,.quick-test-cell{flex-direction:column;gap:3px}
.sync-toolbar{margin-bottom:0}
.sync-meta{display:grid;grid-template-columns:repeat(2,max-content);align-items:center;gap:4px 14px}
.sync-meta span{white-space:nowrap}
.sync-meta-time{font-variant-numeric:tabular-nums;letter-spacing:-.01em}
.sync-meta-time-row{grid-column:1 / -1}
.sync-summary-slot{min-width:0;display:flex;justify-content:flex-start}
.sync-panel-trigger-slot{display:flex;align-items:center;justify-content:flex-end}
.sync-panel-trigger-button{position:relative;isolation:isolate;overflow:visible;width:34px;height:34px;border-radius:999px;border:1px solid rgba(90,117,79,.18);background:rgba(255,255,255,.54);color:#55684d;display:inline-flex;align-items:center;justify-content:center;cursor:pointer;transition:transform .18s ease,box-shadow .18s ease,border-color .18s ease,background-color .18s ease,color .18s ease}
.sync-panel-trigger-button .anticon{font-size:11px}
.sync-panel-trigger-button:hover:not(:disabled){transform:translateY(-1px);background:rgba(255,255,255,.82);border-color:rgba(96,128,84,.3);color:#30412f;box-shadow:0 10px 24px rgba(90,117,79,.12)}
.sync-panel-trigger-button:disabled{opacity:.45;cursor:default}
.sync-panel-trigger-button-fiery::before,
.sync-panel-trigger-button-fiery::after{content:"";position:absolute;pointer-events:none;border-radius:999px}
.sync-panel-trigger-button-fiery::before{inset:2px;z-index:0;padding:2px;background:conic-gradient(from 0deg,transparent 0deg 22deg,rgba(255,215,94,.98) 40deg 92deg,transparent 118deg 160deg,rgba(255,189,46,.98) 188deg 242deg,transparent 268deg 318deg,rgba(255,170,0,.94) 334deg 360deg);box-shadow:inset 0 0 6px rgba(255,185,42,.26),0 0 6px rgba(255,191,68,.16);animation:sync-trigger-orbit 1.8s linear infinite;-webkit-mask:linear-gradient(#000 0 0) content-box,linear-gradient(#000 0 0);-webkit-mask-composite:xor;mask-composite:exclude}
.sync-panel-trigger-button-fiery::after{inset:4px;z-index:0;border:1px solid rgba(255,196,72,.78);box-shadow:0 0 0 1px rgba(255,212,125,.12),inset 0 0 6px rgba(255,170,0,.18);animation:sync-trigger-pulse 1.15s ease-in-out infinite alternate}
.sync-panel-trigger-button-fiery .anticon{position:relative;z-index:1}
.sync-panel-trigger-button-fiery:disabled::before,
.sync-panel-trigger-button-fiery:disabled::after{animation:none;opacity:.42}
.sync-title-wrap{display:flex;align-items:center;padding-right:16px;margin-right:2px;border-right:1px solid rgba(90,117,79,.14);min-height:42px}
.sync-title-text{font:700 clamp(18px,2vw,24px)/1 Georgia,'Times New Roman',serif;color:#31422f;letter-spacing:-.03em;white-space:nowrap}
.site-heading{display:flex;align-items:center;gap:6px;flex-wrap:nowrap;min-width:0}
.site-subline{display:flex;align-items:center;gap:6px;min-width:0;flex-wrap:wrap}
.site-title-text{display:block;flex:0 1 auto;min-width:0;overflow:hidden;white-space:nowrap}
.site-title-link{display:block;flex:0 1 auto;min-width:0;padding:0;border:0;background:transparent;text-align:left;cursor:pointer;color:inherit}
.site-title-link:hover .site-title-text,.site-title-link:focus-visible .site-title-text{text-decoration:underline}
.site-title-link:disabled{cursor:default;opacity:1}
.site-cell{width:100%;position:relative}
.site-top-row{display:flex;flex-direction:column;align-items:flex-start;justify-content:flex-start;gap:2px;width:100%}
.site-main-block{min-width:80px;max-width:106px;width:106px;flex:0 0 auto}
.compact-site-api-block{margin-top:6px;display:flex;flex-direction:column;gap:4px;min-width:0}
.compact-key-text,.compact-endpoint-text{max-width:100%}
.site-balance-panel{display:flex;flex-direction:column;align-items:flex-start;gap:3px;min-width:128px;margin-top:0}
.site-balance-meta{display:flex;align-items:center;justify-content:flex-start;gap:10px;width:100%;color:#9ca3af;font-size:11px}
.site-balance-time{display:inline-flex;align-items:center;gap:4px;white-space:nowrap}
.site-balance-refresh-icon-button{border:0;background:transparent;color:#6b7280;padding:0;margin-left:2px;display:inline-flex;align-items:center;justify-content:center;cursor:pointer}
.site-balance-refresh-icon-button:disabled{opacity:.65;cursor:default}
.site-balance-value{display:flex;align-items:baseline;gap:6px;color:#c2410c;font-size:12px;font-weight:500;line-height:1.2;white-space:nowrap;max-width:100%}
.site-balance-value-empty{color:#94a3b8}
.site-balance-label{color:#6b7280}
.site-balance-text{overflow:hidden;text-overflow:ellipsis;font-size:12px;font-weight:600;color:#ea580c}
.site-balance-unit{color:#6b7280;font-size:11px}
.site-balance-refresh-icon{font-size:14px}
.site-balance-refresh-icon-spinning{animation:spin 1s linear infinite}
.sync-meta,.subtle-text{color:#72806c;font-size:11px}
.time-cell{width:120px;min-width:120px}
.time-cell span{display:block;line-height:1.2}
.sync-loading{margin-top:6px;display:flex;align-items:center;gap:8px}
.sync-alert{margin-top:6px}
.sync-feedback{margin-top:6px;display:flex;align-items:flex-start;gap:6px;flex-wrap:wrap}
.sync-card :deep(.ant-alert){padding:6px 10px;border-radius:999px;border-color:rgba(90,117,79,.14);background:rgba(255,255,255,.52)}
.sync-card :deep(.ant-alert-message){font-size:11px;line-height:1.25}
.sync-card :deep(.sync-alert-inline.ant-alert){margin:0;display:inline-flex;width:fit-content;max-width:100%;align-items:center}
.sync-card :deep(.sync-alert-inline.ant-alert .ant-alert-content){min-width:0}
.sync-card :deep(.sync-alert-warning.ant-alert){margin:0;flex:1 1 360px;min-width:min(100%,360px)}
.cell-copy-text{max-width:240px;display:inline-block}
.api-combined-cell{display:flex;flex-direction:column;gap:2px;min-width:0}
.api-model-row{margin-top:8px;min-width:0;width:120%;max-width:none}
.api-endpoint-text{font-size:12px;color:#64748b;line-height:1.1;margin-top:-1px}
.models-text{display:block;width:100%;max-width:200px;overflow:hidden;text-overflow:ellipsis;white-space:nowrap}
.record-model-select{width:100%;min-width:0}
.api-model-select{width:100%;max-width:none}
.inline-record-model-select{flex:1 1 auto;min-width:0}
.record-model-select :deep(.ant-select-selector){border-radius:8px;padding-inline:9px !important;min-height:24px}
.record-model-select :deep(.ant-select-selection-item){overflow:hidden;text-overflow:ellipsis;white-space:nowrap;font-size:11px;line-height:22px}
.record-model-select :deep(.ant-select-arrow){font-size:11px}
.record-model-select :deep(.ant-select-selection-placeholder){font-size:11px;line-height:22px}
:deep(.record-model-dropdown .ant-select-item-option-content){font-size:11px;line-height:1.35}
:deep(.record-model-dropdown .ant-select-item){font-size:11px;min-height:34px;padding-block:6px}
.import-export-menu{width:max-content;min-width:0;max-width:calc(100vw - 24px);display:flex;flex-direction:column;gap:8px}
.import-export-menu-item{border:0;border-radius:12px;display:flex;align-items:center;gap:8px;width:100%;padding:10px 12px;background:#f8fafc;color:#0f172a;text-align:left;cursor:pointer;transition:background .18s ease,color .18s ease,transform .18s ease}
.import-export-menu-item:hover:not(:disabled){background:#e0ecff;color:#1d4ed8;transform:translateY(-1px)}
.import-export-menu-item:disabled{cursor:not-allowed;opacity:.5;box-shadow:none;transform:none}
.import-export-menu-item :deep(.anticon),.import-export-menu-item svg{font-size:14px;flex:0 0 auto}
.import-export-menu-item span{white-space:normal;overflow-wrap:anywhere}
.import-export-menu-item-danger{background:#fff1f2;color:#be123c}
.import-export-menu-item-danger:hover:not(:disabled){background:#ffe4e6;color:#be123c}
.export-actions-cell{display:flex;flex-direction:column;align-items:flex-start;gap:10px;min-width:0}
.row-actions-stack{display:flex;flex-direction:column;align-items:stretch;gap:8px;width:100%}
.row-actions-stack :deep(.ant-btn),.row-actions-stack :deep(.ant-popconfirm){width:100%}
.inline-export-actions{display:flex;align-items:center;gap:8px;flex-wrap:nowrap;min-width:max-content}
.inventory-icon-button{width:34px;height:34px;border:0;border-radius:12px;display:inline-flex;align-items:center;justify-content:center;cursor:pointer;transition:transform .18s ease, box-shadow .18s ease, filter .18s ease, opacity .18s ease;background:linear-gradient(135deg,#f8fafc,#e2e8f0);box-shadow:inset 0 0 0 1px rgba(148,163,184,.28);flex:0 0 auto;color:#0f172a}
.inventory-icon-button:hover:not(:disabled){transform:translateY(-1px) scale(1.06);filter:saturate(1.08)}
.inventory-icon-button:disabled{cursor:not-allowed;opacity:.45;transform:none;filter:none;box-shadow:inset 0 0 0 1px rgba(148,163,184,.18)}
.inventory-icon-button :deep(.anticon),.inventory-icon-button svg{font-size:16px;line-height:1}
.inventory-batch-quick-test-button{width:34px;height:34px;padding:0;border:0;border-radius:12px;background:linear-gradient(135deg,#476847,#6f8f55);box-shadow:0 10px 24px rgba(87,118,76,.18);display:inline-flex;align-items:center;justify-content:center;color:#fff}
.inventory-batch-quick-test-button:disabled{opacity:.55}
.inventory-batch-quick-test-button :deep(.anticon),.inventory-batch-quick-test-button svg{font-size:16px;line-height:1}
.inventory-icon-button-primary{background:linear-gradient(135deg,#eff6ff,#dbeafe);color:#1d4ed8;box-shadow:0 10px 24px rgba(96,165,250,.18),inset 0 0 0 1px rgba(96,165,250,.22)}
.inventory-icon-button-danger{background:linear-gradient(135deg,#fff1f2,#ffe4e6);color:#be123c;box-shadow:0 10px 24px rgba(244,63,94,.12),inset 0 0 0 1px rgba(244,63,94,.18)}
.batch-quick-test-alert{margin-bottom:12px;border-radius:14px}
.export-icon-button{width:32px;height:32px;border:0;border-radius:12px;display:inline-flex;align-items:center;justify-content:center;cursor:pointer;transition:transform .18s ease, box-shadow .18s ease, filter .18s ease;background:linear-gradient(135deg,#f8fafc,#e2e8f0);box-shadow:inset 0 0 0 1px rgba(148,163,184,.28);flex:0 0 auto}
.export-icon-button:hover{transform:translateY(-1px) scale(1.06);filter:saturate(1.08)}
.export-icon-glyph{font-size:16px;line-height:1}
.export-icon-image{width:20px;height:20px;display:block;object-fit:contain}
.export-icon-image-switch{width:18px;height:18px;border-radius:6px}
.export-copy{color:#0f172a}
.export-cherry{background:linear-gradient(135deg,#fff1f2,#ffe4e6);color:#be123c}
.export-switch{background:linear-gradient(135deg,#fff7ed,#ffedd5);color:#1d4ed8}
.export-desktop{background:linear-gradient(135deg,#eff6ff,#dbeafe);color:#fff;box-shadow:0 10px 24px rgba(96,165,250,.22)}
.switch-app-menu{min-width:132px;display:flex;flex-direction:column;gap:6px}
.switch-app-item{border:0;border-radius:10px;background:#f8fafc;color:#0f172a;padding:8px 10px;text-align:left;cursor:pointer;transition:background .18s ease,color .18s ease}
.switch-app-item:hover{background:#e0ecff;color:#1d4ed8}
:global(.key-management-mini-bar-tooltip .ant-tooltip-inner){max-width:calc(100vw - 24px);white-space:normal;overflow-wrap:anywhere}
:global(.key-management-import-popover .ant-popover-inner){max-width:calc(100vw - 24px)}
:global(.key-management-import-popover .ant-popover-inner-content){padding:8px}
.quick-test-tag{width:fit-content}
.quick-test-cell{gap:6px;min-width:0;align-items:flex-start;padding-left:0}
.export-quick-test-row{width:100%;flex-direction:row;align-items:center;gap:8px;flex-wrap:nowrap}
.quick-test-status-row{width:100%;min-width:0}
.quick-test-status-inline{display:inline-flex;align-items:center;gap:6px;min-width:0}
.quick-test-button{align-self:flex-start;padding-inline:18px;border-radius:12px;flex:0 0 auto}
.performance-tooltip-list{display:flex;flex-direction:column;gap:2px}
.performance-badge{width:18px;height:18px;display:inline-flex;align-items:center;justify-content:center;border-radius:999px;border:1px solid rgba(217,119,6,.24);background:rgba(255,247,237,.92);color:#d97706;font-size:11px;line-height:1;box-shadow:0 4px 10px rgba(245,158,11,.14)}
.performance-badge-inline{flex:0 0 auto;cursor:help}
.compact-key-table :deep(table){table-layout:fixed}
.compact-key-table :deep(.ant-table-thead > tr > th){padding:12px 12px}
.compact-key-table :deep(.ant-table-tbody > tr > td){padding:10px 12px 0;vertical-align:top}
@keyframes spin{from{transform:rotate(0deg)}to{transform:rotate(360deg)}}
.compact-key-table :deep(.ant-table-cell){overflow:hidden}
.compact-key-table :deep(.ant-table-tbody > tr > td.api-key-column){overflow:visible;position:relative;z-index:2}
.compact-key-table :deep(.ant-table-tbody > tr > td:first-child){overflow:visible;position:relative;z-index:3}
.desktop-config-modal{display:flex;flex-direction:column;gap:16px}
.desktop-config-alert{margin-bottom:4px}
.desktop-config-layout{display:grid;grid-template-columns:280px minmax(0,1fr);gap:20px;align-items:start}
.desktop-app-panel,.desktop-form-panel{border-radius:24px;background:linear-gradient(180deg,#f8fafc,#eef2ff);padding:18px}
.desktop-panel-title{font-size:16px;font-weight:700;color:#0f172a}
.desktop-panel-hint{margin-top:6px;color:#64748b;font-size:12px;line-height:1.5}
.desktop-app-grid{margin-top:16px;display:grid;grid-template-columns:repeat(2,minmax(0,1fr));gap:14px}
.desktop-app-card{border:0;border-radius:22px;padding:16px 12px;background:#fff;color:#0f172a;box-shadow:0 10px 24px rgba(15,23,42,.08),inset 0 0 0 1px rgba(148,163,184,.16);display:flex;flex-direction:column;align-items:center;gap:10px;cursor:pointer;transition:transform .18s ease,box-shadow .18s ease,background .18s ease}
.desktop-app-card:hover{transform:translateY(-2px)}
.desktop-app-card-active{box-shadow:0 14px 30px rgba(37,99,235,.16),inset 0 0 0 2px rgba(37,99,235,.45);background:linear-gradient(180deg,#ffffff,#eff6ff)}
.desktop-provider-checkbox{margin-top:10px}
.desktop-field-hint{margin-top:8px;color:#64748b;font-size:12px;line-height:1.5}
.desktop-app-logo{width:58px;height:58px;border-radius:18px;display:inline-flex;align-items:center;justify-content:center;background:#f8fafc;padding:10px}
.desktop-app-logo-image{width:100%;height:100%;display:block;object-fit:contain}
.desktop-app-name{font-size:13px;font-weight:600}
.desktop-app-claude .desktop-app-logo{background:linear-gradient(135deg,#fff7ed,#ffedd5)}
.desktop-app-codex .desktop-app-logo{background:linear-gradient(135deg,#ffffff,#f3f4f6)}
.desktop-app-gemini .desktop-app-logo{background:linear-gradient(135deg,#ffffff,#eef4ff)}
.desktop-app-opencode .desktop-app-logo{background:linear-gradient(135deg,#eef2ff,#dbeafe)}
.desktop-app-openclaw .desktop-app-logo{background:linear-gradient(135deg,#fff1f2,#ffe4e6)}
.manual-record-modal-wrap :deep(.ant-modal-content){
  background: transparent;
  box-shadow: none;
  padding: 0;
}
.manual-record-modal-wrap :deep(.ant-modal-body){
  padding: 0;
}
.manual-record-dialog{
  width: 100%;
  box-sizing: border-box;
  padding: 8px;
  border-radius: 24px;
  background: transparent;
}
.manual-record-header{
  display:flex;
  align-items:flex-start;
  justify-content:space-between;
  gap:12px;
  margin-bottom:6px;
}
.manual-record-header-copy{min-width:0}
.manual-record-header-actions{display:flex;align-items:center;gap:8px;flex:0 0 auto}
.manual-record-close-button{
  padding-inline:8px;
  color:#ef4444;
  font-size:20px;
  font-weight:800;
  line-height:1;
}
.manual-record-close-button:hover{
  color:#dc2626 !important;
  background:rgba(239,68,68,.08);
}
.manual-record-kicker{
  margin-bottom:2px;
  color:#2563eb;
  font-size:11px;
  font-weight:700;
  letter-spacing:.12em;
  text-transform:uppercase;
}
.manual-record-title{
  color:#0f172a;
  font-size:18px;
  font-weight:800;
  line-height:1.2;
}
.manual-record-subtitle{
  max-width:48ch;
  margin-top:2px;
  color:#64748b;
  font-size:12px;
  line-height:1.35;
}
.manual-record-form{margin-top:0}
.manual-record-fields{
  display:grid;
  gap:6px;
}
.manual-record-row{
  display:grid;
  grid-template-columns:repeat(auto-fit, minmax(280px, 1fr));
  gap:6px;
  align-items:start;
}
.manual-record-row-last{
  grid-template-columns:repeat(auto-fit, minmax(220px, 1fr));
}
.manual-record-form-item{
  margin-bottom:0;
  min-width:0;
}
.manual-record-form-item :deep(.ant-form-item-label){
  padding-bottom:1px;
}
.manual-record-form-item :deep(.ant-form-item-control){
  min-width:0;
}
.manual-record-form-item :deep(.ant-form-item-label > label){
  color:#334155;
  font-size:10px;
  font-weight:600;
  line-height:14px;
  height:14px;
}
.manual-record-form-item :deep(.ant-input),
.manual-record-form-item :deep(.ant-input-password),
.manual-record-form-item :deep(.ant-select-selector){
  border-radius:11px;
}
.manual-record-form-item :deep(.ant-input),
.manual-record-form-item :deep(.ant-input-password){
  padding:1px 10px;
}
.manual-record-form-item :deep(.ant-select-selection-item),
.manual-record-form-item :deep(.ant-select-selection-placeholder){
  font-size:12px;
  line-height:22px;
}
.manual-record-form-item-tight{
  align-self:end;
}
.manual-record-footer{
  display:flex;
  align-items:center;
  justify-content:flex-end;
  gap:8px;
  margin-top:6px;
  padding-top:6px;
  border-top:1px solid rgba(148,163,184,.18);
}
.config-grid{display:grid;grid-template-columns:repeat(2,minmax(0,1fr));gap:0 16px}
.portable-settings-card{display:grid;gap:18px;padding:18px;border-radius:18px;border:1px solid rgba(116,144,104,.16);background:rgba(248,251,246,.96)}
.portable-settings-copy{display:grid;gap:8px}
.portable-settings-title{font-size:18px;font-weight:700;color:#20301b}
.portable-settings-desc,.portable-settings-hint,.portable-settings-meta,.portable-settings-warning{line-height:1.7;color:#5f6f59}
.portable-settings-warning{color:#b25f00}
.portable-settings-actions{display:flex;gap:12px}
.key-management :deep(.ant-card){border-radius:24px;border:1px solid rgba(90,117,79,.12);background:rgba(255,255,255,.7);box-shadow:0 14px 36px rgba(87,107,73,.1),inset 0 1px 0 rgba(255,255,255,.72);backdrop-filter:blur(6px);contain:layout paint}
.key-management :deep(.ant-card-head){min-height:56px;border-bottom-color:rgba(90,117,79,.1)}
.key-management :deep(.ant-card-head-title){font:700 18px/1.1 Georgia,'Times New Roman',serif;color:#2d432f}
.key-management :deep(.ant-card-extra),.key-management .sync-meta,.key-management .subtle-text,.key-management .api-endpoint-text,.key-management .desktop-panel-hint,.key-management .desktop-field-hint{color:#667760}
.key-management :deep(.ant-table-wrapper),.key-management :deep(.ant-table-container){background:transparent}
.key-management :deep(.ant-table-thead > tr > th){background:rgba(239,246,226,.72);color:#334634;font-weight:700}
.key-management :deep(.ant-table-tbody > tr > td){background:rgba(255,255,255,.18)}
.key-management .desktop-app-panel,.key-management .desktop-form-panel{background:linear-gradient(180deg,rgba(248,252,244,.92),rgba(236,245,226,.84));box-shadow:inset 0 1px 0 rgba(255,255,255,.78)}
.key-management .desktop-panel-title,.key-management .desktop-app-name,.key-management .site-title-text{font-family:Georgia,'Times New Roman',serif;color:#2d432f}
.key-management .quick-test-button{border-radius:999px;background:linear-gradient(135deg,#476847,#6f8f55);border:0;box-shadow:0 8px 16px rgba(87,118,76,.18)}
.key-management :deep(.ant-btn-default),.key-management :deep(.ant-btn-primary),.key-management :deep(.ant-select-selector),.key-management :deep(.ant-input),.key-management :deep(.ant-input-password),.key-management :deep(.ant-input-affix-wrapper){border-radius:12px}
.key-management .inventory-card{width:100%;margin:0;flex:1 1 auto;border:0 !important;border-radius:0 !important;background:linear-gradient(180deg,rgba(228,233,226,.96),rgba(214,220,212,.92)) !important;box-shadow:none !important;backdrop-filter:none !important}
.key-management .inventory-card :deep(.ant-card-head){background:linear-gradient(180deg,rgba(228,233,226,.96),rgba(221,227,218,.94)) !important}
.key-management .inventory-card :deep(.ant-card-body){background:transparent}
.key-management .inventory-card :deep(.ant-card-head){border-bottom-color:rgba(114,132,103,.08)}
.key-management-gaia .inventory-card,.key-management-wrapper-gaia .inventory-card{background:linear-gradient(180deg,rgba(10,18,22,.96),rgba(8,14,18,.92)) !important;box-shadow:none !important}
.key-management-gaia .inventory-card :deep(.ant-card-head),.key-management-wrapper-gaia .inventory-card :deep(.ant-card-head){background:linear-gradient(180deg,rgba(14,24,29,.98),rgba(10,18,22,.96)) !important;border-bottom-color:rgba(101,129,138,.16)}
.key-management .sync-card :deep(.ant-card-head){display:none}
.key-management .sync-card{border-radius:18px;border:1px solid rgba(90,117,79,.1);background:radial-gradient(circle at 84% 14%,rgba(255,214,126,.18),transparent 26%),radial-gradient(circle at 18% 18%,rgba(196,226,163,.16),transparent 24%),linear-gradient(180deg,rgba(255,251,242,.94),rgba(243,246,235,.9));box-shadow:0 22px 52px rgba(87,107,73,.1),inset 0 1px 0 rgba(255,255,255,.78)}
.key-management .sync-card :deep(.ant-card-body){padding:10px 12px}
.key-management .sync-card::before{content:'';position:absolute;inset:0;pointer-events:none;background:linear-gradient(90deg,rgba(255,255,255,.22),transparent 32%,transparent 72%,rgba(255,255,255,.1));opacity:.8}
:deep(body.dark-mode) .key-management .sync-card{border-color:rgba(160,189,144,.12);background:radial-gradient(circle at 84% 14%,rgba(179,147,67,.18),transparent 26%),radial-gradient(circle at 18% 18%,rgba(104,149,88,.16),transparent 24%),linear-gradient(145deg,rgba(24,38,27,.95),rgba(35,53,39,.92));box-shadow:0 24px 54px rgba(0,0,0,.26),inset 0 1px 0 rgba(255,255,255,.04)}
:deep(body.dark-mode) .key-management .sync-title-wrap{border-right-color:rgba(160,189,144,.16)}
:deep(body.dark-mode) .key-management .sync-title-text{color:#eef5e6}
:deep(body.dark-mode) .key-management .sync-meta,:deep(body.dark-mode) .key-management .subtle-text{color:#b8c8b2}
:deep(body.dark-mode) .key-management .sync-card :deep(.ant-alert){border-color:rgba(160,189,144,.14);background:rgba(255,255,255,.05)}
:deep(body.dark-mode) .key-management .sync-panel-trigger-button{border-color:rgba(255,148,77,.9);color:#ffb36b;background:rgba(255,255,255,.03)}
:deep(body.dark-mode) .key-management .sync-panel-trigger-button:hover:not(:disabled){background:rgba(255,179,107,.1);box-shadow:0 10px 22px rgba(0,0,0,.2)}
:deep(body.dark-mode) .batch-forest-scene{background:radial-gradient(circle at 16% 18%,rgba(164,213,120,.14),transparent 24%),radial-gradient(circle at 84% 14%,rgba(255,213,116,.14),transparent 22%),linear-gradient(180deg,rgba(5,12,8,.42) 0%,rgba(5,12,8,.62) 45%,rgba(4,8,6,.82) 100%),url('/forest-batch-bg-v2.png') center center/cover no-repeat}
:deep(body.dark-mode) .key-management :deep(.ant-card){background:rgba(255,255,255,.06);border-color:rgba(160,189,144,.14);box-shadow:0 18px 42px rgba(0,0,0,.18),inset 0 1px 0 rgba(255,255,255,.04)}
:deep(body.dark-mode) .key-management .inventory-card{background:linear-gradient(180deg,rgba(18,26,20,.96),rgba(14,20,16,.92)) !important;box-shadow:none !important}
:deep(body.dark-mode) .key-management .inventory-card :deep(.ant-card-head){border-bottom-color:rgba(160,189,144,.14)}
:deep(body.dark-mode) .key-management :deep(.ant-card-head-title),:deep(body.dark-mode) .key-management .desktop-panel-title,:deep(body.dark-mode) .key-management .desktop-app-name,:deep(body.dark-mode) .key-management .site-title-text{color:#edf5e6}
:deep(body.dark-mode) .key-management :deep(.ant-card-extra),:deep(body.dark-mode) .key-management .sync-meta,:deep(body.dark-mode) .key-management .subtle-text,:deep(body.dark-mode) .key-management .api-endpoint-text,:deep(body.dark-mode) .key-management .desktop-panel-hint,:deep(body.dark-mode) .key-management .desktop-field-hint{color:#b6c7b1}
:deep(body.dark-mode) .key-management :deep(.ant-table-thead > tr > th){background:rgba(255,255,255,.08);color:#edf5e6}
:deep(body.dark-mode) .key-management :deep(.ant-table-tbody > tr > td){background:rgba(255,255,255,.03)}
:deep(body.dark-mode) .key-management .desktop-app-panel,:deep(body.dark-mode) .key-management .desktop-form-panel{background:linear-gradient(180deg,rgba(255,255,255,.05),rgba(160,189,144,.06))}
:deep(body.dark-mode) .portable-settings-card{border-color:rgba(154,191,142,.18);background:rgba(24,32,25,.92)}
:deep(body.dark-mode) .portable-settings-title{color:#ecf8e7}
:deep(body.dark-mode) .portable-settings-desc,:deep(body.dark-mode) .portable-settings-hint,:deep(body.dark-mode) .portable-settings-meta{color:#b8cbb1}
:deep(body.dark-mode) .portable-settings-warning{color:#ffcb8a}
:deep(body.gaia-dark) .key-management .sync-panel-trigger-button{border-color:rgba(105,154,145,.92);color:#9ed4c8;background:rgba(255,255,255,.03)}
:deep(body.gaia-dark) .key-management .sync-panel-trigger-button:hover:not(:disabled){background:rgba(108,166,153,.12);box-shadow:0 10px 22px rgba(0,0,0,.24)}
:deep(body.gaia-dark) .batch-forest-scene{background:linear-gradient(180deg,rgba(6,12,16,.12) 0%,rgba(6,12,16,.3) 42%,rgba(5,10,14,.52) 100%)}
:deep(body.gaia-dark) .key-management :deep(.ant-card){background:linear-gradient(180deg,rgba(255,255,255,.034),rgba(255,255,255,.012)),rgba(8,14,18,.7);border-color:rgba(101,129,138,.16);box-shadow:0 20px 46px rgba(0,0,0,.24),inset 0 1px 0 rgba(181,214,225,.035)}
:deep(body.gaia-dark) .key-management .inventory-card{background:linear-gradient(180deg,rgba(10,18,22,.96),rgba(8,14,18,.92)) !important;box-shadow:none !important}
:deep(body.gaia-dark) .key-management .inventory-card :deep(.ant-card-head){border-bottom-color:rgba(101,129,138,.16)}
:deep(body.gaia-dark) .key-management :deep(.ant-card-head-title),:deep(body.gaia-dark) .key-management .desktop-panel-title,:deep(body.gaia-dark) .key-management .desktop-app-name,:deep(body.gaia-dark) .key-management .site-title-text{color:#e8f3ef}
:deep(body.gaia-dark) .key-management :deep(.ant-card-extra),:deep(body.gaia-dark) .key-management .sync-meta,:deep(body.gaia-dark) .key-management .subtle-text,:deep(body.gaia-dark) .key-management .api-endpoint-text,:deep(body.gaia-dark) .key-management .desktop-panel-hint,:deep(body.gaia-dark) .key-management .desktop-field-hint{color:#abc1bb}
:deep(body.gaia-dark) .key-management :deep(.ant-table-thead > tr > th){background:rgba(255,255,255,.07);color:#e8f3ef}
:deep(body.gaia-dark) .key-management :deep(.ant-table-tbody > tr > td){background:rgba(255,255,255,.025)}
:deep(body.gaia-dark) .key-management .desktop-app-panel,:deep(body.gaia-dark) .key-management .desktop-form-panel{background:linear-gradient(180deg,rgba(255,255,255,.04),rgba(79,102,112,.05))}
:deep(body.gaia-dark) .key-management .quick-test-button{background:linear-gradient(135deg,#405965,#243740);box-shadow:0 10px 18px rgba(0,0,0,.22)}
:deep(body.gaia-dark) .key-management-compact{background:linear-gradient(180deg,#0a1116,#111c22)}
.key-management-compact :deep(.ant-card-head){padding-inline:12px;min-height:52px}
.key-management-compact :deep(.ant-card-head-title){font-size:15px}
.key-management-compact :deep(.ant-card-body){padding:12px}
.key-management-compact :deep(.ant-card-extra){max-width:100%}
.key-management-compact :deep(.ant-space){row-gap:8px}
.key-management-compact .site-main-block{min-width:0;max-width:none;flex:1 1 auto}
.key-management-compact .compact-key-table :deep(.ant-table-thead > tr > th){padding:10px 8px;font-size:12px}
.key-management-compact .compact-key-table :deep(.ant-table-tbody > tr > td){padding:8px 8px 0;vertical-align:top}
.key-management-compact .site-cell{padding-bottom:0}
.key-management-compact .quick-test-cell{padding-left:0}
.key-management-compact .quick-test-button{padding-inline:12px}
.key-management-compact .inline-export-actions{gap:6px}
.key-management-compact .export-actions-cell{gap:8px}
.key-management-compact .export-quick-test-row{gap:6px}
.key-management-compact .record-model-select :deep(.ant-select-selector){min-height:26px;padding-inline:8px !important}
.key-management-compact .record-model-select :deep(.ant-select-selection-item),
.key-management-compact .record-model-select :deep(.ant-select-selection-placeholder){font-size:11px;line-height:24px}
@media (max-width:900px){.key-management-page-container{padding:8px 8px 0 !important}.desktop-config-layout{grid-template-columns:1fr}.desktop-app-grid{grid-template-columns:repeat(4,minmax(0,1fr));overflow:auto}.config-grid{grid-template-columns:1fr}}
.key-management-gaia{background:transparent;box-shadow:none}
.key-management-gaia :deep(.ant-card){background:linear-gradient(180deg,rgba(255,255,255,.034),rgba(255,255,255,.012)),rgba(8,14,18,.7);border-color:rgba(101,129,138,.16);box-shadow:0 20px 46px rgba(0,0,0,.24),inset 0 1px 0 rgba(181,214,225,.035)}
.key-management-gaia :deep(.ant-card-head-title),.key-management-gaia .desktop-panel-title,.key-management-gaia .desktop-app-name,.key-management-gaia .site-title-text{color:#e8f3ef}
.key-management-gaia :deep(.ant-card-extra),.key-management-gaia .sync-meta,.key-management-gaia .subtle-text,.key-management-gaia .api-endpoint-text,.key-management-gaia .desktop-panel-hint,.key-management-gaia .desktop-field-hint{color:#abc1bb}
.key-management-gaia :deep(.ant-table-thead > tr > th){background:rgba(255,255,255,.07);color:#e8f3ef}
.key-management-gaia :deep(.ant-table-tbody > tr > td){background:rgba(255,255,255,.025)}
.key-management-gaia .desktop-app-panel,.key-management-gaia .desktop-form-panel{background:linear-gradient(180deg,rgba(255,255,255,.04),rgba(79,102,112,.05))}
.key-management-gaia .quick-test-button{background:linear-gradient(135deg,#405965,#243740);box-shadow:0 10px 18px rgba(0,0,0,.22)}
.key-management-gaia .sync-card{border-color:rgba(101,129,138,.16);background:radial-gradient(circle at 84% 14%,rgba(139,107,75,.14),transparent 26%),radial-gradient(circle at 18% 18%,rgba(76,106,117,.16),transparent 24%),linear-gradient(145deg,rgba(9,18,23,.96),rgba(16,29,35,.94));box-shadow:0 24px 54px rgba(0,0,0,.28),inset 0 1px 0 rgba(180,214,225,.04)}
.key-management-gaia .sync-title-wrap{border-right-color:rgba(101,129,138,.16)}
.key-management-gaia .sync-title-text{color:#e7f1ef}
.key-management-gaia .sync-meta,.key-management-gaia .subtle-text{color:#a9bcbd}
.key-management-gaia .sync-card :deep(.ant-alert){border-color:rgba(101,129,138,.16);background:rgba(8,14,18,.34)}
.key-management-gaia .sync-panel-trigger-button{border-color:rgba(101,129,138,.2);background:rgba(255,255,255,.05);color:#dce8e7}
.key-management-gaia .sync-panel-trigger-button:hover:not(:disabled){background:rgba(88,116,126,.18);border-color:rgba(122,155,166,.3);color:#f4faf8;box-shadow:0 10px 24px rgba(0,0,0,.24)}
.key-management-gaia.key-management-compact{background:linear-gradient(180deg,#0a1116,#111c22)}
@keyframes sync-trigger-orbit{from{transform:rotate(0deg)}to{transform:rotate(360deg)}}
@keyframes sync-trigger-pulse{0%{transform:scale(.98);filter:saturate(1)}100%{transform:scale(1.05);filter:saturate(1.16)}}
</style>


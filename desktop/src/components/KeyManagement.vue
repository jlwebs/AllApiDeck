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
              <AppHeader
                v-if="!isCompactMode"
                current-page="keys"
                :is-dark-mode="isDarkMode"
                @experimental="showExperimentalFeatures = true"
                @request-records="openRequestRecordsDrawer"
                @settings="openSettingsModal"
              />

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

      <a-card ref="inventoryCardRef" class="inventory-card">
        <div class="inventory-panel-toolbar">
          <div class="inventory-card-title-row">
            <div class="inventory-panel-switcher" aria-label="密钥与调度切换">
              <div class="inventory-panel-tabs" role="tablist" aria-label="密钥管理子面板切换">
                <button
                  type="button"
                  class="inventory-panel-tab"
                  :class="{ 'inventory-panel-tab-active': activeInventoryPanel === 'local' }"
                  :aria-selected="activeInventoryPanel === 'local' ? 'true' : 'false'"
                  aria-label="本地密钥管理"
                  title="本地密钥管理"
                @click.stop="setActiveInventoryPanel('local')"
              >
                <KeyOutlined class="inventory-panel-tab-icon inventory-panel-tab-icon-key" />
                <span class="inventory-panel-tab-label">KEY</span>
              </button>
              <span class="inventory-panel-tab-divider" aria-hidden="true"></span>
              <button
                type="button"
                class="inventory-panel-tab"
                  :class="{ 'inventory-panel-tab-active': activeInventoryPanel === 'console' }"
                  :aria-selected="activeInventoryPanel === 'console' ? 'true' : 'false'"
                  aria-label="调度台"
                  title="调度台"
                @click.stop="setActiveInventoryPanel('console')"
              >
                <QueueOrbitIcon class="inventory-panel-tab-icon inventory-panel-tab-icon-console" />
                <span class="inventory-panel-tab-label">Dispatch</span>
              </button>
            </div>
            </div>
          </div>
          <div v-if="activeInventoryPanel === 'local'" class="inventory-panel-actions">
          <a-space wrap>
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
                  <button
                    type="button"
                    class="import-export-menu-item import-export-menu-item-danger"
                    :disabled="batchDeleteQuickTestFailedDisabled"
                    @click="confirmDeleteQuickTestFailedRecords"
                  >
                    <DeleteOutlined />
                    <span>批量删除快测失败密钥</span>
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
            <a-tooltip v-if="syncCurrentGroupProviderQueueDisabled" :title="syncCurrentGroupProviderQueueTooltip">
              <span class="inventory-popover-trigger">
                <button
                  type="button"
                  class="inventory-icon-button inventory-icon-button-provider-queue"
                  :disabled="true"
                >
                  <QueueOrbitIcon class="provider-queue-icon" />
                </button>
              </span>
            </a-tooltip>
            <a-tooltip v-else :title="syncCurrentGroupProviderQueueTooltip">
              <a-popover
                v-model:open="providerQueueInlineConfirmOpen"
                trigger="click"
                placement="bottom"
                overlay-class-name="provider-queue-inline-popover"
                :getPopupContainer="getSidebarPopupContainer"
              >
                <template #content>
                  <div class="provider-queue-inline-actions">
                    <a-button type="primary" size="small" block class="provider-queue-inline-action-button" @click="handleReplaceCurrentGroupToAdvancedProxyQueue">
                      应用当前组的密钥为Provider队列
                    </a-button>
                    <a-button size="small" block class="provider-queue-inline-action-button" @click="handleAppendCurrentGroupToAdvancedProxyQueue">
                      追加当前组进入Provider队列
                    </a-button>
                    <a-button danger size="small" block class="provider-queue-inline-action-button" @click="handleClearAdvancedProxyQueue">
                      清空全部Provider队列
                    </a-button>
                  </div>
                </template>
                <button
                  type="button"
                  class="inventory-icon-button inventory-icon-button-provider-queue"
                  aria-label="Provider队列操作"
                >
                  <QueueOrbitIcon class="provider-queue-icon" />
                </button>
              </a-popover>
            </a-tooltip>
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
            <a-popconfirm
              v-if="!isCompactMode"
              title="确认清空本地密钥库？"
              ok-text="清空"
              cancel-text="取消"
              placement="bottomRight"
              :getPopupContainer="getSidebarPopupContainer"
              @confirm="clearLocalRecords"
            >
              <button
                type="button"
                class="inventory-icon-button inventory-icon-button-danger"
                :disabled="tableData.length === 0"
                title="清空本地库"
                aria-label="清空本地库"
              >
                <DeleteOutlined />
              </button>
            </a-popconfirm>
          </a-space>
          </div>
        </div>

        <div v-show="activeInventoryPanel === 'local'" class="inventory-local-panel">
        <div class="key-group-strip">
          <div class="key-group-tabs" role="tablist" aria-label="密钥分组">
            <button
              type="button"
              class="key-group-tab"
              :class="{ 'key-group-tab-active': activeKeyGroupId === ALL_KEYS_GROUP_ID }"
              @click="setActiveKeyGroup(ALL_KEYS_GROUP_ID)"
            >
              全部密钥
              <span class="key-group-tab-count">{{ allSortedRows.length }}</span>
            </button>
            <button
              v-for="group in keyGroups"
              :key="group.id"
              type="button"
              class="key-group-tab"
              :class="{ 'key-group-tab-active': activeKeyGroupId === group.id }"
              @click="setActiveKeyGroup(group.id)"
              @contextmenu="event => openKeyGroupContextMenu(group, event)"
            >
              {{ group.name }}
              <span class="key-group-tab-count">{{ getGroupRecordCount(group.id) }}</span>
            </button>
            <a-tooltip title="新增分组，数据独立维护，可从全部密钥克隆导入" overlay-class-name="key-group-create-tooltip">
              <button type="button" class="key-group-tab key-group-tab-create" @click.stop="toggleQuickGroupPopover">
                <PlusOutlined />
                <span>快捷分组</span>
              </button>
            </a-tooltip>
          </div>
          <div class="key-group-site-filter">
            <a-tooltip :title="hideInvalidKeys ? '隐藏无效密钥（点击显示全部）' : '显示全部密钥（点击仅看有效）'">
              <button
                type="button"
                class="key-group-site-filter-toggle"
                :class="{ 'key-group-site-filter-toggle-active': hideInvalidKeys }"
                :aria-pressed="hideInvalidKeys ? 'true' : 'false'"
                :aria-label="hideInvalidKeys ? '隐藏无效密钥已开启' : '隐藏无效密钥已关闭'"
                @click="toggleHideInvalidKeys"
              >
                <EyeInvisibleOutlined v-if="hideInvalidKeys" />
                <EyeOutlined v-else />
              </button>
            </a-tooltip>
            <a-input
              v-model:value="keyGroupSiteFilterDisplayValue"
              size="small"
              allow-clear
              class="key-group-site-filter-input"
              placeholder="输入中转站名字筛选"
              @focus="handleKeyGroupSiteFilterFocus"
              @blur="handleKeyGroupSiteFilterBlur"
            />
          </div>
        </div>

        <teleport to="body">
          <div v-if="quickGroupPopoverOpen" class="key-quick-group-overlay" @click="closeQuickGroupPopover">
            <div class="key-quick-group-floating-panel" :class="{ 'key-quick-group-floating-panel-gaia': isDarkMode }" @click.stop>
              <div class="key-quick-group-composer">
                <div class="key-quick-group-input-row">
                  <a-input
                    v-model:value="createKeyGroupDraftName"
                    size="small"
                    :maxlength="32"
                    placeholder="输入组名；不选快捷项也可直接创建空组"
                    @input="handleQuickGroupDraftNameInput"
                    @pressEnter="submitQuickGroupComposer"
                  />
                  <a-tooltip title="刷新密钥当下最新的模型列表~" :getPopupContainer="getQuickGroupPopupContainer">
                    <button
                      type="button"
                      class="key-quick-group-refresh-button"
                      :disabled="quickGroupModelRefreshDisabled"
                      aria-label="刷新密钥当下最新的模型列表"
                      @click="refreshQuickGroupModelCatalog"
                    >
                      <ReloadOutlined class="key-quick-group-refresh-icon" :class="{ 'site-balance-refresh-icon-spinning': quickGroupModelRefreshRunning }" />
                    </button>
                  </a-tooltip>
                  <a-button type="primary" size="small" class="key-quick-group-create-button" @click="submitQuickGroupComposer">
                    创建
                  </a-button>
                </div>
              </div>
              <div class="quick-filter-toolbar key-quick-filter-toolbar">
                <div class="quick-filter-strip key-quick-filter-strip" v-if="quickGroupFilters.length">
                  <a-popover
                    v-for="family in quickGroupFilters"
                    :key="family.key"
                    trigger="hover"
                    placement="bottomLeft"
                    overlay-class-name="quick-filter-family-popover"
                    :getPopupContainer="getQuickGroupPopupContainer"
                    :z-index="1600"
                  >
                    <template #content>
                      <div class="quick-filter-family-panel">
                        <div class="quick-filter-family-panel-title">{{ family.label }}</div>
                        <div class="quick-filter-option-list">
                          <a-button
                            v-for="option in family.options"
                            :key="option.key"
                            size="small"
                            :type="activeQuickGroupFilters.includes(option.key) ? 'primary' : 'default'"
                            @click="toggleQuickGroupFilter(option.key)"
                          >
                            {{ option.label }}
                          </a-button>
                          <a-button
                            size="small"
                            class="quick-filter-family-select-all"
                            @click="selectQuickGroupFilterFamily(family)"
                          >
                            {{ isQuickGroupFilterFamilyFullySelected(family) ? '取消' : '全选' }}
                          </a-button>
                        </div>
                      </div>
                    </template>
                    <a-button
                      class="quick-filter-family-trigger"
                      :type="isQuickGroupFilterFamilyActive(family) ? 'primary' : 'default'"
                      @click="selectQuickGroupFilterFamily(family)"
                    >
                      {{ family.label }}
                      <span v-if="getQuickGroupFilterFamilyActiveCount(family)" class="quick-filter-family-count">
                        {{ getQuickGroupFilterFamilyActiveCount(family) }}
                      </span>
                    </a-button>
                  </a-popover>
                  <a-button
                    class="quick-filter-clear-trigger"
                    @click="clearQuickGroupFilters"
                    :disabled="!activeQuickGroupFilters.length"
                  >
                    清空
                  </a-button>
                </div>
                <div v-else class="quick-filter-empty-inline">暂无可用快捷分组</div>
              </div>
              <div v-if="activeQuickGroupSummary" class="quick-filter-summary key-quick-group-summary">{{ activeQuickGroupSummary }}</div>
              <div v-if="activeQuickGroupFilters.length" class="key-quick-group-preview">
                <div class="key-quick-group-preview-head">
                  <span class="key-quick-group-preview-title">命中密钥</span>
                  <span class="key-quick-group-preview-count">{{ quickGroupMatchedRecords.length }} 条</span>
                </div>
                <div v-if="quickGroupMatchedRecords.length" class="key-quick-group-preview-list">
                  <div
                    v-for="item in quickGroupMatchedRecords"
                    :key="item.rowKey"
                    class="key-quick-group-preview-item"
                  >
                    <div class="key-quick-group-preview-main">
                      <span class="key-quick-group-preview-site">{{ item.siteName }}</span>
                      <span class="key-quick-group-preview-token">{{ item.tokenName }}</span>
                      <span class="key-quick-group-preview-key">{{ item.maskedApiKey }}</span>
                    </div>
                    <div class="key-quick-group-preview-models">{{ item.matchedModels.join(' / ') }}</div>
                  </div>
                </div>
                <div v-else class="key-quick-group-preview-empty">当前快捷项未命中任何密钥</div>
              </div>
            </div>
          </div>
        </teleport>

        <teleport to="body">
          <div
            v-if="keyGroupContextMenu.open && keyGroupContextMenu.group"
            class="key-group-context-menu"
            :class="{ 'key-group-context-menu-dark': isDarkMode }"
            :style="{ left: `${keyGroupContextMenu.x}px`, top: `${keyGroupContextMenu.y}px` }"
          >
            <button type="button" class="import-export-menu-item key-group-context-action" @click="openRenameKeyGroupModalFromContext">
              <EditOutlined />
              <span>重命名</span>
            </button>
            <button
              type="button"
              class="import-export-menu-item key-group-context-action key-group-context-submenu-trigger"
              :class="{ 'key-group-context-submenu-trigger-active': keyGroupContextMenu.mergeSubmenuOpen }"
              @mouseenter="openKeyGroupMergeSubmenu"
            >
              <span>合并到分组</span>
              <span class="key-row-context-submenu-arrow">›</span>
            </button>
            <div
              v-if="keyGroupContextMenu.mergeSubmenuOpen"
              class="key-group-context-submenu"
              :class="{ 'key-group-context-submenu-dark': isDarkMode }"
              @mouseenter="openKeyGroupMergeSubmenu"
            >
              <div class="key-row-group-heading">
                <span class="key-row-action-label">目标分组</span>
              </div>
              <div v-if="mergeTargetKeyGroups.length" class="key-row-group-list">
                <button
                  v-for="group in mergeTargetKeyGroups"
                  :key="group.id"
                  type="button"
                  class="key-row-group-chip"
                  @click="handleMergeKeyGroupInto(group)"
                >
                  <span class="key-row-group-chip-mark">→</span>
                  <span class="key-row-group-chip-name">{{ group.name }}</span>
                </button>
              </div>
              <div v-else class="key-row-action-empty">暂无可合并的目标分组</div>
            </div>
            <button type="button" class="import-export-menu-item key-group-context-action import-export-menu-item-danger" @click="handleKeyGroupContextDelete">
              <DeleteOutlined />
              <span>删除该组</span>
            </button>
          </div>
        </teleport>

        <a-empty v-if="displayedRows.length === 0" :description="keyManagementEmptyDescription" />
        <a-table
          v-else
          :columns="activeColumns"
          :data-source="displayedRows"
          :row-key="record => record.rowKey"
          :pagination="tablePagination"
          :scroll="isCompactMode ? { x: 560 } : undefined"
          :custom-row="getManagedRecordRowProps"
          :show-sorter-tooltip="false"
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
                      <span class="subtle-text">{{ record.tokenName || '未命名 Token' }}</span>
                      <span v-if="record.sourceType === 'manual'" class="manual-source-chip">手工</span>
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
                    <div v-if="record.balanceLoading || getRecordBalanceValue(record)" class="site-balance-value" :class="{ 'site-balance-value-empty': !getRecordBalanceValue(record) || record.balanceLoading }">
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
                      :value="getRecordSelectedModelValue(record) || undefined"
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
                  :value="getRecordSelectedModelValue(record) || undefined"
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
          </template>
        </a-table>
        </div>
        <div v-if="activeInventoryPanel === 'console'" class="inventory-console-panel" aria-label="调度台面板">
          <div class="console-dispatch-control-rack">
            <div class="console-dispatch-summary console-dispatch-summary-top">
              <div v-for="block in consoleDispatchSummaryBlocks" :key="block.id" class="console-dispatch-summary-block">
                <div v-for="item in block.items" :key="item.label" class="console-dispatch-summary-line">
                  <span>{{ item.label }}</span>
                  <strong>{{ item.value }}</strong>
                </div>
              </div>
            </div>
            <div class="console-dispatch-control-panel">
              <a-tooltip :title="consoleProxyMasterTitle">
                <a-switch
                  size="small"
                  class="console-dispatch-master-switch"
                  :class="{ 'console-dispatch-control-pending': consoleProxyMasterPending }"
                  :checked="consoleProxyMasterEnabled"
                  @change="toggleConsoleProxyMaster"
                />
              </a-tooltip>
              <a-tooltip :title="consoleAntiPoisonTitle">
                <button
                  type="button"
                  class="console-dispatch-icon-button console-dispatch-anti-poison-button"
                  :class="{ 'console-dispatch-icon-button-active': consoleAntiPoisonEnabled, 'console-dispatch-control-pending': consoleAntiPoisonPending }"
                  :aria-pressed="consoleAntiPoisonEnabled ? 'true' : 'false'"
                  :aria-label="consoleAntiPoisonEnabled ? '关闭防投毒' : '开启防投毒'"
                  @click="toggleConsoleAntiPoison"
                >
                  <SafetyCertificateOutlined />
                </button>
              </a-tooltip>
              <div class="console-dispatch-app-buttons" aria-label="客户端高级代理配置">
                <a-tooltip v-for="app in consoleProxyAppCards" :key="app.id" :title="app.tooltip">
                  <button
                    type="button"
                    class="console-dispatch-app-button"
                    :class="{ 'console-dispatch-app-button-active': app.enabled, 'console-dispatch-control-pending': app.pending }"
                    :aria-pressed="app.enabled ? 'true' : 'false'"
                    :aria-label="`${app.enabled ? '关闭' : '开启'} ${app.label} 高级代理`"
                    @click="toggleConsoleProxyApp(app.id)"
                  >
                    <img :src="app.icon" :alt="app.label" />
                  </button>
                </a-tooltip>
              </div>
            </div>
          </div>
          <div class="console-dispatch-top-grid">
            <section class="console-queue-section">
              <div class="console-section-head">
                <div>
                  <h4>Provider 队列</h4>
                  <p>全局队列和可调度密钥统一展示。</p>
                </div>
                <span class="console-section-count">{{ consoleQueueCards.length }} 条</span>
              </div>
              <div v-if="consoleQueueCards.length" class="console-provider-grid">
                <button
                  v-for="item in consoleQueueCards"
                  :key="item.id"
                  type="button"
                  class="console-provider-card"
                  :class="{
                    'console-provider-card-primary': item.queueOrder === 1,
                    'console-provider-card-pending': !item.inQueue,
                    'console-provider-card-draggable': item.inQueue,
                    'console-provider-card-dragging': consoleQueueDragState.sourceId === item.id,
                    'console-provider-card-drop-before': consoleQueueDragState.overId === item.id && !consoleQueueDragState.insertAfter,
                    'console-provider-card-drop-after': consoleQueueDragState.overId === item.id && consoleQueueDragState.insertAfter,
                  }"
                  :data-console-provider-id="item.id"
                  @click="handleConsoleProviderCardClick(item)"
                >
                  <div class="console-provider-card-top">
                    <span
                      v-if="item.inQueue"
                      class="console-provider-drag-handle"
                      title="拖动排序"
                      aria-label="拖动排序"
                      @pointerdown.stop.prevent="startConsoleQueueDrag(item, $event)"
                      @click.stop
                    >
                      <i></i><i></i><i></i><i></i><i></i><i></i>
                    </span>
                    <strong>{{ item.siteName }}</strong>
                    <span v-if="item.queueOrder" class="console-provider-order">{{ `P${item.queueOrder}` }}</span>
                  </div>
                  <div class="console-provider-model">{{ item.modelLabel }}</div>
                  <div class="console-provider-meta">
                    <span v-if="item.skLabel" class="console-provider-chip">{{ item.skLabel }}</span>
                    <span v-if="!item.inQueue" class="console-provider-chip console-provider-chip-muted">未入队</span>
                    <span v-if="!item.enabled" class="console-provider-chip console-provider-chip-muted">已停用</span>
                  </div>
                </button>
              </div>
              <div v-else class="console-empty-panel">暂无可调度 Provider，请先在本地密钥管理写入密钥。</div>
            </section>

            <section class="console-dispatch-section">
              <div class="console-section-head">
                <div>
                  <h4>日志</h4>
                  <p>按接收、路由、Provider 切换和结果持续追加。</p>
                </div>
              </div>
              <div ref="advancedProxyConsoleLogScroller" class="console-dispatch-log-panel">
                <pre class="console-dispatch-log-view">{{ consoleDispatchLogText }}</pre>
              </div>
            </section>
          </div>

          <section class="console-connections-section">
            <div class="console-section-head">
              <div>
                <h4>连接信息</h4>
                <p>保留最近 50 条连接，已完成记录固定排在下方。</p>
              </div>
              <span class="console-section-count">{{ sortedAdvancedProxyActiveConnections.length }} 条</span>
            </div>
            <div class="console-connections-panel">
              <div class="console-connection-table" role="table" aria-label="当前高级代理连接">
                <div class="console-connection-row console-connection-head" role="row">
                  <span role="columnheader">状态</span>
                  <span role="columnheader">会话序号</span>
                  <span role="columnheader">已用时间</span>
                  <span role="columnheader">出站</span>
                  <span role="columnheader">Provider</span>
                  <span role="columnheader">入口</span>
                  <span role="columnheader">模型</span>
                  <span role="columnheader">目标地址</span>
                </div>
                <div v-if="advancedProxyActiveConnectionsLoading && !sortedAdvancedProxyActiveConnections.length" class="console-connection-empty-row" role="row">
                  正在加载当前连接...
                </div>
                <div v-else-if="!sortedAdvancedProxyActiveConnections.length" class="console-connection-empty-row" role="row">
                  暂无高级代理连接。
                </div>
                <template v-else>
                  <button
                    v-for="connection in sortedAdvancedProxyActiveConnections"
                    :key="connection.id"
                    type="button"
                    class="console-connection-row console-connection-item"
                    :class="{ 'console-connection-item-selected': selectedAdvancedProxyConnectionId === connection.id }"
                    role="row"
                    @click="selectAdvancedProxyConnection(connection)"
                    @contextmenu="openAdvancedProxyConnectionContextMenu(connection, $event)"
                  >
                    <span
                      role="cell"
                      class="console-connection-status-cell"
                      :class="{ 'console-connection-status-cell-failed': isAdvancedProxyConnectionFailed(connection) }"
                      :title="formatAdvancedProxyConnectionErrorTitle(connection)"
                    >
                      <span class="console-connection-status-dot" :class="getAdvancedProxyConnectionStatusClass(connection)" aria-hidden="true"></span>
                      <small>{{ formatAdvancedProxyConnectionStage(connection) }}</small>
                    </span>
                    <span role="cell">{{ formatAdvancedProxyConnectionSessionOrdinal(connection) }}</span>
                    <span role="cell">
                      <strong>{{ formatAdvancedProxyConnectionWaitMs(connection) }}</strong>
                      <small>{{ formatAdvancedProxyConnectionTime(connection.startedAt) }}</small>
                    </span>
                    <span role="cell">{{ connection.outboundRoute || '-' }}</span>
                    <span role="cell">{{ formatAdvancedProxyConnectionProvider(connection) }}</span>
                    <span role="cell">{{ formatAdvancedProxyConnectionRoute(connection) }}</span>
                    <span role="cell">{{ connection.model || '-' }}</span>
                    <span role="cell">{{ connection.upstreamEndpoint || connection.upstreamUrl || '等待上游' }}</span>
                  </button>
                </template>
              </div>
            </div>
          </section>
        </div>
      </a-card>

      <div
        v-if="advancedProxyConnectionContextMenu.open && advancedProxyConnectionContextMenu.connection"
        class="key-row-context-menu advanced-proxy-connection-context-menu"
        :class="{ 'key-row-context-menu-dark': isDarkMode }"
        :style="{ left: `${advancedProxyConnectionContextMenu.x}px`, top: `${advancedProxyConnectionContextMenu.y}px` }"
      >
        <button
          type="button"
          class="import-export-menu-item key-row-context-action"
          @click="openAdvancedProxyConnectionDetailFromContext"
        >
          详情
        </button>
        <div class="key-row-action-info">
          <span class="key-row-action-label">连接</span>
          <span class="key-row-action-value">
            {{ formatAdvancedProxyConnectionProvider(advancedProxyConnectionContextMenu.connection) }} / {{ advancedProxyConnectionContextMenu.connection?.model || '-' }}
          </span>
        </div>
      </div>

      <div
        v-if="rowContextMenu.open && (rowContextMenu.record || rowContextMenu.records.length)"
        ref="rowContextMenuRef"
        class="key-row-context-menu"
        :class="{ 'key-row-context-menu-dark': isDarkMode }"
        :style="{ left: `${rowContextMenu.x}px`, top: `${rowContextMenu.y}px` }"
        @mouseleave="closeRowContextGroupSubmenu"
      >
        <button
          v-if="!rowContextMenu.batch"
          type="button"
          class="import-export-menu-item key-row-context-action"
          @click="handleRowContextEdit"
        >
          编辑密钥
        </button>
        <button
          type="button"
          class="import-export-menu-item import-export-menu-item-danger key-row-context-action"
          @click="handleRowContextDelete"
        >
          {{ rowContextMenu.batch ? `批量删除记录（${rowContextMenu.records.length}）` : '删除记录' }}
        </button>
        <button
          type="button"
          class="import-export-menu-item key-row-context-action"
          @click="handleRowContextModelProbe"
        >
          {{ rowContextMenu.batch ? `批量探测可用模型（${rowContextMenu.records.length}）` : '探测可用模型' }}
        </button>
        <button
          v-if="!rowContextMenu.batch"
          type="button"
          class="import-export-menu-item key-row-context-action"
          @click="handleRowContextAIImage"
        >
          发起AI绘图
        </button>
        <button
          type="button"
          class="import-export-menu-item key-row-context-action key-row-context-submenu-trigger"
          :class="{ 'key-row-context-submenu-trigger-active': rowContextMenu.groupSubmenuOpen }"
          @mouseenter="openRowContextGroupSubmenu"
        >
          <span>{{ rowContextMenu.batch ? `批量分配到分组（${rowContextMenu.records.length}）` : '分配到分组' }}</span>
          <span class="key-row-context-submenu-arrow">›</span>
        </button>

        <div
          v-if="rowContextMenu.groupSubmenuOpen"
          class="key-row-context-submenu"
          :class="{ 'key-row-context-submenu-dark': isDarkMode }"
          @mouseenter="openRowContextGroupSubmenu"
        >
          <div class="key-row-group-heading">
            <span class="key-row-action-label">分组</span>
            <button type="button" class="key-row-group-create-button" @click="openCreateKeyGroupModalFromContext">
              <PlusOutlined />
            </button>
          </div>
          <div v-if="keyGroups.length" class="key-row-group-list">
            <button
              v-for="group in keyGroups"
              :key="group.id"
              type="button"
              class="key-row-group-chip"
              :class="{ 'key-row-group-chip-active': isRowContextGroupActive(group.id) }"
              @click="toggleRowContextGroupMembership(group.id)"
            >
              <span class="key-row-group-chip-mark">{{ getRowContextGroupMark(group.id) }}</span>
              <span class="key-row-group-chip-name">{{ group.name }}</span>
            </button>
          </div>
          <div v-else class="key-row-action-empty">暂无自定义分组</div>
        </div>
        <div class="key-row-action-info">
          <span class="key-row-action-label">{{ rowContextMenu.batch ? '批量选中' : '最近同步' }}</span>
          <span class="key-row-action-value">
            {{ rowContextMenu.batch ? `${rowContextMenu.records.length} 条记录` : formatDateTime(rowContextMenu.record?.updatedAt) }}
          </span>
        </div>
      </div>

      <a-modal
        v-model:open="createKeyGroupModalOpen"
        title="新建密钥分组"
        ok-text="创建"
        cancel-text="取消"
        :confirm-loading="createKeyGroupSaving"
        @ok="submitCreateKeyGroup"
        @cancel="closeCreateKeyGroupModal"
      >
        <a-input
          v-model:value="createKeyGroupDraftName"
          :maxlength="18"
          show-count
          placeholder="例如：默认 / 极速 / 链式代理"
          @pressEnter="submitCreateKeyGroup"
        />
      </a-modal>

      <a-modal
        v-model:open="renameKeyGroupModalOpen"
        title="重命名密钥分组"
        ok-text="保存"
        cancel-text="取消"
        :confirm-loading="renameKeyGroupSaving"
        @ok="submitRenameKeyGroup"
        @cancel="closeRenameKeyGroupModal"
      >
        <a-input
          v-model:value="renameKeyGroupDraftName"
          :maxlength="18"
          show-count
          placeholder="输入新的分组名称"
          @pressEnter="submitRenameKeyGroup"
        />
      </a-modal>

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

      <a-modal v-model:open="desktopConfigModalOpen" title="专属一键配置" :confirm-loading="desktopConfigLoading" :footer="null" width="1120px">
        <div v-if="desktopConfigTargetRecord" class="desktop-config-modal">
          <div class="desktop-config-hero">
            <div class="desktop-config-alert">
              <div class="desktop-config-alert-icon" aria-hidden="true">i</div>
              <div class="desktop-config-alert-copy">
                <div class="desktop-config-alert-title">{{ `${desktopConfigTargetRecord.siteName} | ${desktopConfigTargetRecord.siteUrl}` }}</div>
                <div class="desktop-config-alert-desc">将读取本机应用配置，生成变更预览，确认后才会真正写入。</div>
              </div>
            </div>
            <div class="desktop-config-hero-actions">
              <a-button type="primary" :loading="desktopConfigLoading" @click="generateDesktopConfigPreview">生成变更预览</a-button>
            </div>
          </div>
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
              <AdvancedProxyRequestRecordsDrawer
                v-model:open="showRequestRecordsDrawer"
                :is-dark-mode="isDarkMode"
                :initial-panel="requestRecordsInitialPanel"
                :focus-record-id="advancedProxyFocusedRequestRecordId"
                @update:open="handleRequestRecordsDrawerOpenChange"
              />
              <AdvancedProxyModal
                v-model:open="showExperimentalFeatures"
                :initial-queue-scope="advancedProxyFocusQueueScope"
                :focus-queue-token="advancedProxyFocusQueueToken"
              />
              <Teleport to="body">
                <div
                  v-if="consoleQueueDragState.active"
                  ref="consoleQueueDragGhostRef"
                  class="console-provider-drag-ghost"
                  :style="consoleQueueDragGhostStyle"
                >
                  <div class="console-provider-card-top">
                    <span class="console-provider-drag-handle console-provider-drag-handle-ghost" aria-hidden="true">
                      <i></i><i></i><i></i><i></i><i></i><i></i>
                    </span>
                    <strong>{{ consoleQueueDragState.ghostSiteName }}</strong>
                    <span class="console-provider-order">{{ consoleQueueDragState.ghostOrderLabel }}</span>
                  </div>
                  <div class="console-provider-model">{{ consoleQueueDragState.ghostModelLabel }}</div>
                  <div class="console-provider-meta">
                    <span v-if="consoleQueueDragState.ghostSkLabel" class="console-provider-chip">{{ consoleQueueDragState.ghostSkLabel }}</span>
                  </div>
                </div>
              </Teleport>
            </div>
          </div>
        </div>
      </div>
    </div>
  </ConfigProvider>
</template>

<script setup>
import { computed, h, nextTick, onBeforeUnmount, onMounted, reactive, ref, watch } from 'vue';
import { ClockCircleOutlined, DeleteOutlined, DownloadOutlined, EyeInvisibleOutlined, EyeOutlined, FileTextOutlined, ImportOutlined, KeyOutlined, MenuFoldOutlined, PlusOutlined, ReloadOutlined, SafetyCertificateOutlined, SwapOutlined, ThunderboltOutlined } from '@ant-design/icons-vue';
import { ConfigProvider, message, Modal, theme } from 'ant-design-vue';
import { useRoute } from 'vue-router';
import AppHeader from './AppHeader.vue';
import QueueOrbitIcon from './icons/QueueOrbitIcon.vue';
import AdvancedProxyModal from './AdvancedProxyModal.vue';
import AdvancedProxyRequestRecordsDrawer from './AdvancedProxyRequestRecordsDrawer.vue';
import DesktopConfigDiffModal from './DesktopConfigDiffModal.vue';
import SystemSettingsModal from './SystemSettingsModal.vue';
import { fetchModelList } from '../utils/api.js';
import { logClientDiagnostic } from '../utils/clientDiagnostics.js';
import { maskApiKey } from '../utils/normal.js';
import { apiFetch, isProbablyWailsRuntime, openUrlInSystemBrowser } from '../utils/runtimeApi.js';
import { applyManagedAppConfigFiles, isDesktopConfigBridgeAvailable, readManagedAppConfigFiles } from '../utils/desktopConfigBridge.js';
import {
  ADVANCED_PROXY_SYNC_EVENT,
  ADVANCED_PROXY_APPS,
  ADVANCED_PROXY_GLOBAL_QUEUE_SCOPE,
  getAdvancedProxyAppBaseUrl,
  getAdvancedProxyConfig,
  getAdvancedProxyEffectiveProviders,
  getAdvancedProxyQueueProviders,
  isAdvancedProxyActiveConnectionBridgeAvailable,
  isAdvancedProxyRequestRecordBridgeAvailable,
  listAdvancedProxyActiveConnections,
  listAdvancedProxyRequestRecords,
  normalizeAdvancedProxyConfig,
  setAdvancedProxyConfig,
  setAdvancedProxyConfigOptimistic,
  syncAdvancedProxyProvidersFromRecords,
} from '../utils/advancedProxyBridge.js';
import { buildDesktopConfigPreview, createDesktopConfigDraft, DESKTOP_CONFIG_APPS, inferProviderKeyFromSnapshot } from '../utils/desktopConfigTransform.js';
import { fetchQuotaLabelWithBatchLogic, isDisplayableQuotaLabel } from '../utils/balance.js';
import { buildQuickTestMessages } from '../utils/quickTestPrompts.js';
import { normalizeCCSwitchEndpoint } from '../utils/ccSwitch.js';
import { resolveOpenAIExportBaseUrl } from '../utils/exportEndpoint.js';
import { getAppliedThemeMode, isDarkThemeMode, THEME_MODE_CHANGE_EVENT } from '../utils/theme.js';
import { exitSidebarMode, isManualSidebarBridgeAvailable, isSidebarBridgeAvailable, openManualSidebarPanel } from '../utils/windowMode.js';
import { loadDesktopTokenSourceMode, loadTreeExpandedSetting, loadUserAgentMappings } from '../utils/systemSettings.js';
import { buildPerformanceTooltipLines, derivePerformanceMetricsFromResponse, hasPerformanceMetrics } from '../utils/performanceMetrics.js';
import {
  hydrateLastResultsSnapshotCache,
  HISTORY_SNAPSHOT_INDEX_KEY,
  HISTORY_SNAPSHOT_SYNC_EVENT,
  getCachedLastResultsSnapshotRaw,
} from '../utils/historySnapshotStore.js';
import { ExportTextFile, OpenAIImageWindow, OpenModelProbeWindow } from '../../wailsjs/go/main/App.js';
import {
  buildSiteCacheKey,
  mergeExtractedSitesIntoCache,
  mergeExtractedSitesIntoTempCache,
  writeModelProbeContext,
} from '../utils/siteCacheStore.js';
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
const KEY_GROUPS_STORAGE_KEY = 'api_check_key_management_groups_v1';
const DEFAULT_PUBLIC_KEY_SEED_STORAGE_KEY = 'api_check_key_management_default_public_seed_checked_v1';
const LAST_RESULTS_STORAGE_KEY = HISTORY_SNAPSHOT_INDEX_KEY;
const KEY_MANAGEMENT_SYNC_EVENT = 'batch-api-check:key-management-sync';
const DEFAULT_TEST_TIMEOUT_MS = 20000;
const CC_SWITCH_TARGET_APPS = ['claude', 'codex', 'gemini', 'opencode', 'openclaw'];
const ALL_KEYS_GROUP_ID = '__all_keys__';
const DEFAULT_PUBLIC_KEY_MODELS = [
  'claude-fable-5',
  'claude-opus-4-8',
  'claude-opus-4-7',
  'claude-opus-4-6',
  'claude-opus-4-5',
  'claude-opus-4-1',
  'claude-sonnet-4-6',
  'claude-sonnet-4-5',
  'claude-sonnet-4',
  'claude-haiku-4-5',
  'gemini-3.5-flash',
  'gemini-3.1-pro',
  'gemini-3-flash',
  'gpt-5.5',
  'gpt-5.5-pro',
  'gpt-5.4',
  'gpt-5.4-pro',
  'gpt-5.4-mini',
  'gpt-5.4-nano',
  'gpt-5.3-codex-spark',
  'gpt-5.3-codex',
  'gpt-5.2',
  'gpt-5.2-codex',
  'gpt-5.1',
  'gpt-5.1-codex-max',
  'gpt-5.1-codex',
  'gpt-5.1-codex-mini',
  'gpt-5',
  'gpt-5-codex',
  'gpt-5-nano',
  'grok-build-0.1',
  'deepseek-v4-pro',
  'deepseek-v4-flash',
  'glm-5.2',
  'glm-5.1',
  'glm-5',
  'minimax-m2.7',
  'minimax-m2.5',
  'kimi-k2.6',
  'kimi-k2.5',
  'qwen3.6-plus',
  'qwen3.5-plus',
  'big-pickle',
  'deepseek-v4-flash-free',
  'mimo-v2.5-free',
  'qwen3.6-plus-free',
  'minimax-m3-free',
  'nemotron-3-ultra-free',
  'north-mini-code-free',
  'glm-4.6',
];
const DEFAULT_PUBLIC_KEY_RECORD = {
  rowKey: 'manual::default-public-opencode',
  sourceType: 'manual',
  siteName: 'Opencode',
  tokenName: 'public',
  siteUrl: 'https://opencode.ai/zen/v1',
  apiKey: 'public',
  modelsList: DEFAULT_PUBLIC_KEY_MODELS,
  modelsText: DEFAULT_PUBLIC_KEY_MODELS.join(', '),
  selectedModel: 'deepseek-v4-flash-free',
  groupSelectedModels: {},
  status: 1,
};
const DESKTOP_APP_ICONS = {
  claude: claudeAppIcon,
  codex: codexAppIcon,
  gemini: geminiAppIcon,
  opencode: opencodeAppIcon,
  openclaw: openclawAppIcon,
};
const CONSOLE_PROXY_APP_IDS = ['claude', 'codex', 'opencode', 'openclaw'];
const CONSOLE_PROXY_APP_LABELS = {
  claude: 'Claude',
  codex: 'Codex',
  opencode: 'OpenCode',
  openclaw: 'OpenClaw',
};
const PROXY_MANAGED_TOKEN = 'PROXY_MANAGED';
const ADVANCED_PROXY_PROVIDER_NAME = 'AllApiDeck Advanced Proxy';
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
const advancedProxyFocusQueueScope = ref(ADVANCED_PROXY_GLOBAL_QUEUE_SCOPE);
const advancedProxyFocusQueueToken = ref(0);
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
const activeInventoryPanel = ref('local');
const advancedProxyConfigSnapshot = ref(normalizeAdvancedProxyConfig({}));
const advancedProxyConsoleRecords = ref([]);
const advancedProxyConsoleRecordsLoading = ref(false);
const advancedProxyConsoleLogLines = ref([]);
const advancedProxyConsoleRecordIds = ref(new Set());
const advancedProxyActiveConnections = ref([]);
const advancedProxyActiveConnectionsLoading = ref(false);
const consoleProxyConfigApplying = ref(false);
const consoleQueueDragState = reactive({
  active: false,
  sourceId: '',
  overId: '',
  insertAfter: false,
  saving: false,
  moved: false,
  suppressClickUntil: 0,
  ghostX: 0,
  ghostY: 0,
  ghostWidth: 0,
  ghostHeight: 0,
  ghostOffsetX: 0,
  ghostOffsetY: 0,
  ghostSiteName: '',
  ghostModelLabel: '',
  ghostSkLabel: '',
  ghostOrderLabel: '',
});
let consoleQueueDragFrame = 0;
let consoleQueueDragGhostX = 0;
let consoleQueueDragGhostY = 0;
let consoleQueueDragLayouts = [];
const consoleProxyPendingAppIds = ref([]);
const consoleProxyOptimisticApps = reactive({});
const consoleProxyMasterOptimistic = ref(null);
const consoleProxyMasterPending = ref(false);
const consoleAntiPoisonOptimistic = ref(null);
const consoleAntiPoisonPending = ref(false);
const consoleTakeoverReconcileCooldownUntil = reactive({});
const advancedProxyConsoleLogScroller = ref(null);
const inventoryCardRef = ref(null);
const consoleQueueDragGhostRef = ref(null);
const selectedAdvancedProxyConnectionId = ref('');
const advancedProxyConnectionClockTick = ref(Date.now());
const advancedProxyFocusedRequestRecordId = ref('');
const requestRecordsInitialPanel = ref('records');
const advancedProxyConnectionContextMenu = reactive({
  open: false,
  x: 0,
  y: 0,
  connection: null,
});
const hideInvalidKeys = ref(true);
const BATCH_QUICK_TEST_CONCURRENCY = 10;
const QUICK_GROUP_MODEL_REFRESH_CONCURRENCY = 6;
const batchQuickTestRunning = ref(false);
const quickGroupModelRefreshRunning = ref(false);
const batchQuickTestProgress = reactive({
  completed: 0,
  total: 0,
  active: 0,
});
const currentTablePage = ref(1);
const currentTablePageSize = ref(20);
const showAppSettingsModal = ref(false);
const showRequestRecordsDrawer = ref(false);
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
let advancedProxyConsolePollingTimer = null;
let advancedProxyConnectionClockTimer = null;
let advancedProxyTakeoverReconciling = false;
const CONSOLE_TAKEOVER_RECONCILE_COOLDOWN_MS = 8000;
const isCompactMode = computed(() => route.query?.compact === '1');
const keyGroups = ref(loadStoredKeyGroups());
const activeKeyGroupId = ref(ALL_KEYS_GROUP_ID);
const activeQuickGroupFilters = ref([]);
const quickGroupPopoverOpen = ref(false);
const quickGroupDraftNameMode = ref('auto');
const createKeyGroupModalOpen = ref(false);
const createKeyGroupSaving = ref(false);
const createKeyGroupDraftName = ref('');
const renameKeyGroupModalOpen = ref(false);
const renameKeyGroupSaving = ref(false);
const renameKeyGroupDraftName = ref('');
const renameKeyGroupTargetId = ref('');
const rowContextMenuRef = ref(null);
const rowContextMenu = reactive({
  open: false,
  x: 0,
  y: 0,
  record: null,
  records: [],
  batch: false,
  groupSubmenuOpen: false,
});
const selectedRowKeys = ref([]);
const providerQueueInlineConfirmOpen = ref(false);
const keyGroupContextMenu = reactive({
  open: false,
  x: 0,
  y: 0,
  group: null,
  mergeSubmenuOpen: false,
});
const PERSIST_DEBOUNCE_MS = 240;
let persistRecordsTimer = null;
let lastPersistedRecordsSnapshot = '';
const recordRenderMetaCache = new Map();

const configProviderTheme = computed(() => ({
  algorithm: isDarkMode.value ? theme.darkAlgorithm : theme.defaultAlgorithm,
}));

function maskProviderApiKey(value) {
  const normalized = String(value || '').trim();
  if (!normalized) return '';
  if (normalized.length <= 8) return normalized;
  return `${normalized.slice(0, 4)}****${normalized.slice(-4)}`;
}

function formatProviderSkLabel(index, apiKey) {
  const maskedKey = maskProviderApiKey(apiKey);
  if (!Number(index)) {
    return maskedKey ? `SK | ${maskedKey}` : '';
  }
  return maskedKey ? `SK ${index} | ${maskedKey}` : `SK ${index}`;
}

function normalizeConsoleText(value) {
  return String(value || '').trim();
}

function firstNonEmpty(...values) {
  return values.map(value => normalizeConsoleText(value)).find(Boolean) || '';
}

function cloneConsoleFallback(value) {
  return JSON.parse(JSON.stringify(value ?? {}));
}

function isConsolePlainObject(value) {
  return Boolean(value) && Object.prototype.toString.call(value) === '[object Object]';
}

function parseConsoleStrictJsonObjectSafe(text, fallback = {}) {
  if (!String(text || '').trim()) return cloneConsoleFallback(fallback);
  try {
    const parsed = JSON.parse(text);
    return isConsolePlainObject(parsed) ? parsed : cloneConsoleFallback(fallback);
  } catch {
    return cloneConsoleFallback(fallback);
  }
}

function stripConsoleJsonComments(input) {
  let result = '';
  let inSingle = false;
  let inDouble = false;
  let escaping = false;
  for (let index = 0; index < input.length; index += 1) {
    const current = input[index];
    const next = input[index + 1];
    if (!inSingle && !inDouble && current === '/' && next === '/') {
      while (index < input.length && input[index] !== '\n') index += 1;
      if (index < input.length) result += '\n';
      continue;
    }
    if (!inSingle && !inDouble && current === '/' && next === '*') {
      index += 2;
      while (index < input.length && !(input[index] === '*' && input[index + 1] === '/')) index += 1;
      index += 1;
      continue;
    }
    result += current;
    if (escaping) {
      escaping = false;
      continue;
    }
    if ((inSingle || inDouble) && current === '\\') {
      escaping = true;
      continue;
    }
    if (!inDouble && current === '\'') {
      inSingle = !inSingle;
      continue;
    }
    if (!inSingle && current === '"') inDouble = !inDouble;
  }
  return result;
}

function convertConsoleSingleQuotedStrings(input) {
  let result = '';
  let inDouble = false;
  let escaping = false;
  for (let index = 0; index < input.length; index += 1) {
    const current = input[index];
    if (inDouble) {
      result += current;
      if (escaping) escaping = false;
      else if (current === '\\') escaping = true;
      else if (current === '"') inDouble = false;
      continue;
    }
    if (current === '"') {
      inDouble = true;
      result += current;
      continue;
    }
    if (current !== '\'') {
      result += current;
      continue;
    }
    let buffer = '';
    let innerEscaping = false;
    let closed = false;
    for (index += 1; index < input.length; index += 1) {
      const inner = input[index];
      if (innerEscaping) {
        buffer += inner;
        innerEscaping = false;
        continue;
      }
      if (inner === '\\') {
        innerEscaping = true;
        buffer += inner;
        continue;
      }
      if (inner === '\'') {
        closed = true;
        break;
      }
      buffer += inner;
    }
    if (!closed) throw new Error('Single-quoted string is not closed');
    result += JSON.stringify(buffer.replace(/\\'/g, '\'').replace(/\\"/g, '"'));
  }
  return result;
}

function parseConsoleLooseJsonObjectSafe(text, fallback = {}) {
  if (!String(text || '').trim()) return cloneConsoleFallback(fallback);
  try {
    const withoutComments = stripConsoleJsonComments(String(text || ''));
    const withDoubleQuotes = convertConsoleSingleQuotedStrings(withoutComments);
    const quotedKeys = withDoubleQuotes.replace(/([{,]\s*)([A-Za-z_$][\w$-]*)(\s*:)/g, '$1"$2"$3');
    const normalized = quotedKeys.replace(/,(\s*[}\]])/g, '$1');
    const parsed = JSON.parse(normalized);
    return isConsolePlainObject(parsed) ? parsed : cloneConsoleFallback(fallback);
  } catch {
    return cloneConsoleFallback(fallback);
  }
}

function normalizeConsoleComparableUrl(value) {
  const normalized = String(value || '').trim();
  if (!normalized) return '';
  try {
    const url = new URL(normalized);
    const pathname = (url.pathname || '/').replace(/\/+$/, '') || '/';
    return `${url.protocol}//${url.host}${pathname}`.toLowerCase();
  } catch {
    return normalized.replace(/\/+$/, '').toLowerCase();
  }
}

function findConsoleManagedSnapshotFile(snapshotFiles, appId, fileId) {
  return (Array.isArray(snapshotFiles) ? snapshotFiles : []).find(file =>
    String(file?.appId || '').trim() === String(appId || '').trim()
    && String(file?.fileId || '').trim() === String(fileId || '').trim()
  ) || null;
}

function isConsoleManagedProxyToken(value) {
  return String(value || '').trim() === PROXY_MANAGED_TOKEN;
}

function extractConsoleCodexActiveProviderKey(text) {
  const match = String(text || '').match(/^\s*model_provider\s*=\s*(?:"([^"\n]+)"|'([^'\n]+)'|([^\s#]+))/m);
  return String(match?.[1] || match?.[2] || match?.[3] || '').trim();
}

function extractConsoleCodexProviderSectionKey(header) {
  const match = String(header || '').trim().match(/^model_providers\.(?:"([^"]+)"|'([^']+)'|([^\s]+))$/);
  return String(match?.[1] || match?.[2] || match?.[3] || '').trim();
}

function extractConsoleCodexProviderBaseUrl(text, providerKey) {
  const normalizedProviderKey = String(providerKey || '').trim();
  if (!normalizedProviderKey) return '';
  const lines = String(text || '').replace(/\r\n/g, '\n').split('\n');
  let inTargetSection = false;
  const sectionLines = [];
  for (const line of lines) {
    const sectionHeader = line.match(/^\s*\[([^\]]+)\]\s*$/);
    if (sectionHeader) {
      if (inTargetSection) break;
      inTargetSection = extractConsoleCodexProviderSectionKey(sectionHeader[1]) === normalizedProviderKey;
      continue;
    }
    if (inTargetSection) sectionLines.push(line);
  }
  const baseUrlMatch = sectionLines.join('\n').match(/^\s*base_url\s*=\s*["']([^"'\n]+)["']/m);
  return String(baseUrlMatch?.[1] || '').trim();
}

function hasMatchingConsoleOpenCodeProxyProvider(config, expectedBaseUrl) {
  const providers = isConsolePlainObject(config?.provider) ? config.provider : {};
  return Object.values(providers).some(provider =>
    normalizeConsoleComparableUrl(provider?.options?.baseURL) === expectedBaseUrl
    && isConsoleManagedProxyToken(provider?.options?.apiKey)
  );
}

function hasMatchingConsoleOpenClawProxyProvider(config, expectedBaseUrl) {
  const providers = isConsolePlainObject(config?.models?.providers) ? config.models.providers : {};
  const primary = String(config?.agents?.defaults?.model?.primary || '').trim();
  if (primary.includes('/')) {
    const activeProvider = providers[primary.split('/')[0]];
    if (activeProvider) {
      return normalizeConsoleComparableUrl(activeProvider?.baseUrl) === expectedBaseUrl
        && isConsoleManagedProxyToken(activeProvider?.apiKey)
        && String(activeProvider?.api || '').trim() === 'openai-completions';
    }
  }
  return Object.values(providers).some(provider =>
    normalizeConsoleComparableUrl(provider?.baseUrl) === expectedBaseUrl
    && isConsoleManagedProxyToken(provider?.apiKey)
    && String(provider?.api || '').trim() === 'openai-completions'
  );
}

function formatConsoleRecordTime(value) {
  const text = normalizeConsoleText(value);
  if (!text) return '-';
  const date = new Date(text);
  if (Number.isNaN(date.getTime())) return text;
  return date.toLocaleString('zh-CN', {
    hour12: false,
    month: '2-digit',
    day: '2-digit',
    hour: '2-digit',
    minute: '2-digit',
    second: '2-digit',
  });
}

function summarizeConsoleEndpoint(value) {
  const text = normalizeConsoleText(value);
  if (!text) return '-';
  return text
    .replace(/^https?:\/\/[^/]+/i, '')
    .replace(/^\/+/, '')
    .replace(/^advanced-proxy\//i, '') || text;
}

function formatConsoleRouteLabel(value) {
  const normalized = normalizeConsoleText(value).toLowerCase();
  switch (normalized) {
    case 'responses':
      return 'responses';
    case 'responses_compact':
      return 'responses/compact';
    case 'chat':
      return 'chat';
    case 'messages':
      return 'messages';
    default:
      return summarizeConsoleEndpoint(value);
  }
}

function formatConsoleRouteSource(value) {
  const normalized = normalizeConsoleText(value).toLowerCase();
  switch (normalized) {
    case 'fallback':
      return 'fallback 切换';
    case 'fallback_restore':
      return 'fallback 恢复';
    case 'preference':
      return '偏好路由';
    case 'upgrade':
      return '升级路由';
    case 'rectified':
      return '请求修正';
    default:
      return normalized || '-';
  }
}

function formatConsoleRouteStatus(value) {
  const normalized = normalizeConsoleText(value).toLowerCase();
  switch (normalized) {
    case 'success':
      return '成功';
    case 'failed':
    case 'fail':
    case 'error':
      return '失败';
    case 'skipped':
      return '跳过';
    default:
      return normalized || '-';
  }
}

function resolveConsoleRouteTrace(record) {
  const steps = Array.isArray(record?.routeTrace) ? record.routeTrace : [];
  const normalized = steps
    .map(step => ({
      route: normalizeConsoleText(step?.route),
      source: normalizeConsoleText(step?.source),
      status: normalizeConsoleText(step?.status),
    }))
    .filter(step => step.route);
  if (normalized.length) return normalized;
  const route = normalizeConsoleText(record?.outboundRoute);
  if (!route) return [];
  return [{
    route,
    source: normalizeConsoleText(record?.source),
    status: Number(record?.statusCode || 0) >= 200 && Number(record?.statusCode || 0) < 400 ? 'success' : 'failed',
  }];
}

function formatAdvancedProxyConsoleRecordLog(record) {
  const time = formatConsoleRecordTime(record?.recordedAt);
  const appType = normalizeConsoleText(record?.appType).toUpperCase() || 'APP';
  const inbound = summarizeConsoleEndpoint(record?.inboundEndpoint || record?.clientRoute);
  const provider = normalizeConsoleText(record?.providerName) || normalizeConsoleText(record?.providerId) || '未命名 Provider';
  const model = normalizeConsoleText(record?.model) || '未记录模型';
  const upstream = summarizeConsoleEndpoint(record?.upstreamEndpoint || record?.upstreamUrl);
  const statusCode = Number(record?.statusCode || 0);
  const ok = statusCode >= 200 && statusCode < 400 && !normalizeConsoleText(record?.errorDetail);
  const duration = Number(record?.durationMs || 0);
  const metrics = [
    duration > 0 ? `耗时 ${duration}ms` : '',
    Number(record?.ttftMs || 0) > 0 ? `TTFT ${record.ttftMs}ms` : '',
    Number(record?.latencyMs || 0) > 0 ? `延迟 ${record.latencyMs}ms` : '',
  ].filter(Boolean).join(' / ');
  const lines = [
    `[${time}] ${appType} 接收请求: ${inbound}`,
    `  调度 Provider: ${provider} | 模型: ${model} | 出口: ${upstream}`,
  ];
  const routeTrace = resolveConsoleRouteTrace(record);
  if (routeTrace.length) {
    lines.push('  路由轨迹:');
    routeTrace.forEach((step, index) => {
      lines.push(`    ${index + 1}. ${formatConsoleRouteLabel(step.route)} | ${formatConsoleRouteSource(step.source)} | ${formatConsoleRouteStatus(step.status)}`);
    });
  }
  lines.push(`  结果: ${ok ? '成功' : '失败'} | HTTP ${statusCode || '-'}${metrics ? ` | ${metrics}` : ''}`);
  const errorDetail = normalizeConsoleText(record?.errorDetail);
  if (errorDetail) {
    lines.push(`  错误: ${errorDetail}`);
  }
  return lines.join('\n');
}

function formatAdvancedProxyConnectionTime(value) {
  const date = new Date(value || '');
  if (Number.isNaN(date.getTime())) return '--';
  return date.toLocaleTimeString('zh-CN', { hour12: false });
}

function formatAdvancedProxyConnectionSessionOrdinal(connection) {
  const ordinal = Number(connection?.sessionOrdinal || 0);
  return Number.isFinite(ordinal) && ordinal > 0 ? `S${ordinal}` : '-';
}

function formatAdvancedProxyConnectionWaitMs(connection) {
  void advancedProxyConnectionClockTick.value;
  const startedAt = new Date(connection?.startedAt || '').getTime();
  if (!Number.isFinite(startedAt)) return '--';
  const finishedAt = isAdvancedProxyConnectionCompleted(connection)
    ? new Date(connection?.updatedAt || '').getTime()
    : Number.NaN;
  const endAt = Number.isFinite(finishedAt) && finishedAt >= startedAt ? finishedAt : Date.now();
  const elapsed = Math.max(0, endAt - startedAt);
  if (elapsed < 1000) return `${elapsed}ms`;
  if (elapsed < 60000) return `${Math.floor(elapsed / 1000)}s`;
  const minutes = Math.floor(elapsed / 60000);
  const seconds = Math.floor((elapsed % 60000) / 1000);
  return `${minutes}m ${seconds}s`;
}

function getAdvancedProxyConnectionStatusClass(connection) {
  if (isAdvancedProxyConnectionFailed(connection)) return 'console-connection-status-failed';
  const stage = normalizeConsoleText(connection?.stage);
  if (isAdvancedProxyConnectionCompleted(connection)) return 'console-connection-status-completed';
  if (stage === 'waiting_upstream') return 'console-connection-status-waiting';
  if (stage === 'force_probe') return 'console-connection-status-probe';
  return 'console-connection-status-active';
}

function isAdvancedProxyConnectionFailed(connection) {
  const status = normalizeConsoleText(connection?.status);
  const statusCode = Number(connection?.statusCode || 0);
  return status === 'failed' || statusCode >= 400;
}

function isAdvancedProxyConnectionCompleted(connection) {
  const status = normalizeConsoleText(connection?.status);
  const stage = normalizeConsoleText(connection?.stage);
  return isAdvancedProxyConnectionFailed(connection)
    || status === 'completed'
    || stage === 'completed'
    || status === 'done'
    || stage === 'done';
}

function formatAdvancedProxyConnectionErrorCode(connection) {
  const statusCode = Number(connection?.statusCode || 0);
  const errorCode = normalizeConsoleText(connection?.errorCode);
  const parts = [
    statusCode > 0 ? `HTTP ${statusCode}` : '',
    errorCode ? errorCode.toUpperCase() : '',
  ].filter(Boolean);
  return parts.join(' / ') || 'FAILED';
}

function formatAdvancedProxyConnectionErrorTitle(connection) {
  const code = formatAdvancedProxyConnectionErrorCode(connection);
  const detail = normalizeConsoleText(connection?.errorDetail);
  if (!detail) return code;
  return `${code}\n${detail}`;
}

function formatAdvancedProxyConnectionRoute(connection) {
  const app = normalizeConsoleText(connection?.appType).toUpperCase() || 'APP';
  const route = normalizeConsoleText(connection?.clientRoute) || summarizeConsoleEndpoint(connection?.inboundEndpoint) || '-';
  return `${app} / ${route}`;
}

function formatAdvancedProxyConnectionProvider(connection) {
  return normalizeConsoleText(connection?.providerName) || normalizeConsoleText(connection?.providerId) || '等待调度';
}

function formatAdvancedProxyConnectionStage(connection) {
  if (isAdvancedProxyConnectionFailed(connection)) {
    return formatAdvancedProxyConnectionErrorCode(connection);
  }
  if (isAdvancedProxyConnectionCompleted(connection)) return '已完成';
  const stage = normalizeConsoleText(connection?.stage);
  const status = normalizeConsoleText(connection?.status);
  const stageLabels = {
    received: '已接收',
    dispatching: '调度中',
    waiting_upstream: '等待上游',
    force_probe: '强制探测',
  };
  return stageLabels[stage] || status || stage || '进行中';
}

function scrollAdvancedProxyConsoleLogToBottom() {
  if (typeof window === 'undefined') return;
  nextTick(() => {
    const scroller = advancedProxyConsoleLogScroller.value;
    if (!scroller) return;
    scroller.scrollTop = scroller.scrollHeight;
  });
}

function selectAdvancedProxyConnection(connection) {
  selectedAdvancedProxyConnectionId.value = String(connection?.id || '').trim();
}

function closeAdvancedProxyConnectionContextMenu() {
  advancedProxyConnectionContextMenu.open = false;
  advancedProxyConnectionContextMenu.connection = null;
}

async function openAdvancedProxyConnectionContextMenu(connection, event) {
  if (!connection || !event) return;
  event.preventDefault();
  event.stopPropagation();
  closeRowContextMenu();
  closeKeyGroupContextMenu();
  selectedAdvancedProxyConnectionId.value = String(connection?.id || '').trim();
  advancedProxyConnectionContextMenu.connection = connection;
  const anchorX = Number(event.clientX) || 0;
  const anchorY = Number(event.clientY) || 0;
  const position = resolveContextMenuPosition(anchorX, anchorY, 224, 112);
  advancedProxyConnectionContextMenu.x = position.x;
  advancedProxyConnectionContextMenu.y = position.y;
  advancedProxyConnectionContextMenu.open = true;
}

function normalizeAdvancedProxyMatchText(value) {
  return String(value || '').trim().toLowerCase();
}

function scoreAdvancedProxyConnectionRecordMatch(connection, record) {
  if (!connection || !record) return -1;
  let score = 0;
  const connectionStartedAt = new Date(connection?.startedAt || '').getTime();
  const recordAt = new Date(record?.recordedAt || '').getTime();
  if (Number.isFinite(connectionStartedAt) && Number.isFinite(recordAt)) {
    const diff = Math.abs(recordAt - connectionStartedAt);
    if (diff <= 180000) score += 24;
    else if (diff <= 600000) score += 10;
  }
  const pairs = [
    [connection?.appType, record?.appType, 12],
    [connection?.clientRoute, record?.clientRoute, 12],
    [connection?.inboundEndpoint, record?.inboundEndpoint, 10],
    [connection?.outboundRoute, record?.outboundRoute, 10],
    [connection?.providerId, record?.providerId || record?.providerRowKey, 18],
    [connection?.providerName, record?.providerName, 18],
    [connection?.model, record?.model, 16],
    [connection?.upstreamEndpoint || connection?.upstreamUrl, record?.upstreamEndpoint || record?.upstreamUrl, 10],
  ];
  pairs.forEach(([left, right, weight]) => {
    const leftText = normalizeAdvancedProxyMatchText(left);
    const rightText = normalizeAdvancedProxyMatchText(right);
    if (!leftText || !rightText) return;
    if (leftText === rightText || leftText.includes(rightText) || rightText.includes(leftText)) {
      score += weight;
    }
  });
  return score;
}

async function findAdvancedProxyConnectionRequestRecord(connection) {
  const cachedRecords = Array.isArray(advancedProxyConsoleRecords.value) ? advancedProxyConsoleRecords.value : [];
  let bestRecord = null;
  let bestScore = -1;
  cachedRecords.forEach(record => {
    const score = scoreAdvancedProxyConnectionRecordMatch(connection, record);
    if (score > bestScore) {
      bestScore = score;
      bestRecord = record;
    }
  });
  if (bestRecord && bestScore >= 28) return bestRecord;
  try {
    const records = await listAdvancedProxyRequestRecords(120);
    const normalizedRecords = Array.isArray(records) ? records : [];
    advancedProxyConsoleRecords.value = normalizedRecords;
    normalizedRecords.forEach(record => {
      const score = scoreAdvancedProxyConnectionRecordMatch(connection, record);
      if (score > bestScore) {
        bestScore = score;
        bestRecord = record;
      }
    });
  } catch (error) {
    console.warn('[KeyManagement] find advanced proxy connection request record failed:', error);
  }
  return bestScore >= 28 ? bestRecord : null;
}

async function openAdvancedProxyConnectionDetailFromContext() {
  const connection = advancedProxyConnectionContextMenu.connection;
  closeAdvancedProxyConnectionContextMenu();
  const record = await findAdvancedProxyConnectionRequestRecord(connection);
  if (!record?.id) {
    message.warning('未找到这条连接对应的请求详情');
    return;
  }
  advancedProxyFocusedRequestRecordId.value = String(record.id || '').trim();
  requestRecordsInitialPanel.value = 'records';
  showRequestRecordsDrawer.value = true;
}

function openRequestRecordsDrawer(panel = 'records') {
  advancedProxyFocusedRequestRecordId.value = '';
  requestRecordsInitialPanel.value = normalizeRequestRecordsPanel(panel);
  showRequestRecordsDrawer.value = true;
}

function normalizeRequestRecordsPanel(panel) {
  const normalized = String(panel || 'records').trim().toLowerCase();
  return ['sessions', 'records', 'mcp', 'skills'].includes(normalized) ? normalized : 'records';
}

function handleRequestRecordsDrawerOpenChange(open) {
  showRequestRecordsDrawer.value = open === true;
  if (!open) {
    advancedProxyFocusedRequestRecordId.value = '';
  }
}

function getConsoleQueueProviderKey(provider, fallback = '') {
  return String(provider?.id || provider?.rowKey || provider?.baseUrl || provider?.apiKey || fallback || '').trim();
}

function resetConsoleQueueDragState(suppressClick = false) {
  stopConsoleQueueDragListeners();
  consoleQueueDragState.active = false;
  consoleQueueDragState.sourceId = '';
  consoleQueueDragState.overId = '';
  consoleQueueDragState.insertAfter = false;
  consoleQueueDragState.moved = false;
  consoleQueueDragState.ghostX = 0;
  consoleQueueDragState.ghostY = 0;
  consoleQueueDragState.ghostWidth = 0;
  consoleQueueDragState.ghostHeight = 0;
  consoleQueueDragState.ghostOffsetX = 0;
  consoleQueueDragState.ghostOffsetY = 0;
  consoleQueueDragState.ghostSiteName = '';
  consoleQueueDragState.ghostModelLabel = '';
  consoleQueueDragState.ghostSkLabel = '';
  consoleQueueDragState.ghostOrderLabel = '';
  consoleQueueDragGhostX = 0;
  consoleQueueDragGhostY = 0;
  consoleQueueDragLayouts = [];
  if (consoleQueueDragFrame && typeof window !== 'undefined') {
    window.cancelAnimationFrame(consoleQueueDragFrame);
    consoleQueueDragFrame = 0;
  }
  if (suppressClick) {
    consoleQueueDragState.suppressClickUntil = Date.now() + 350;
  }
}

function captureConsoleQueueDragLayouts() {
  if (typeof document === 'undefined') return null;
  consoleQueueDragLayouts = Array.from(document.querySelectorAll('[data-console-provider-id]'))
    .map(card => {
      const id = String(card?.dataset?.consoleProviderId || '').trim();
      if (!id || id === consoleQueueDragState.sourceId || !consoleQueueCards.value.some(item => item.id === id && item.inQueue)) return null;
      const rect = card.getBoundingClientRect();
      return {
        id,
        left: rect.left,
        top: rect.top,
        right: rect.right,
        bottom: rect.bottom,
        width: rect.width,
        height: rect.height,
        centerX: rect.left + rect.width / 2,
        centerY: rect.top + rect.height / 2,
      };
    })
    .filter(Boolean);
}

function findConsoleQueueDragTarget(clientX, clientY) {
  let nearest = null;
  let nearestDistance = Number.POSITIVE_INFINITY;
  consoleQueueDragLayouts.forEach(layout => {
    const insideX = clientX >= layout.left && clientX <= layout.right;
    const insideY = clientY >= layout.top && clientY <= layout.bottom;
    const distance = insideX && insideY
      ? 0
      : ((clientX - layout.centerX) ** 2) + ((clientY - layout.centerY) ** 2);
    if (distance < nearestDistance) {
      nearestDistance = distance;
      nearest = layout;
    }
  });
  return nearest;
}

function setConsoleQueueGhostPosition(clientX, clientY) {
  const x = clientX - consoleQueueDragState.ghostOffsetX;
  const y = clientY - consoleQueueDragState.ghostOffsetY;
  consoleQueueDragGhostX = x;
  consoleQueueDragGhostY = y;
  if (consoleQueueDragFrame) return;
  consoleQueueDragFrame = window.requestAnimationFrame(() => {
    consoleQueueDragFrame = 0;
    const ghost = consoleQueueDragGhostRef.value;
    if (ghost) {
      ghost.style.transform = `translate3d(${consoleQueueDragGhostX}px, ${consoleQueueDragGhostY}px, 0) rotate(-1.5deg)`;
    }
  });
}

function updateConsoleQueueDragTarget(event) {
  if (!consoleQueueDragState.active) return;
  const targetLayout = findConsoleQueueDragTarget(event.clientX, event.clientY);
  const targetId = String(targetLayout?.id || '').trim();
  const targetItem = consoleQueueCards.value.find(item => item.id === targetId && item.inQueue);
  if (!targetItem || targetId === consoleQueueDragState.sourceId) {
    consoleQueueDragState.overId = '';
    consoleQueueDragState.insertAfter = false;
    return;
  }
  const horizontalGrid = targetLayout.width >= targetLayout.height;
  const pointerOffset = horizontalGrid ? event.clientX - targetLayout.left : event.clientY - targetLayout.top;
  const targetSize = horizontalGrid ? targetLayout.width : targetLayout.height;
  const ratio = targetSize > 0 ? pointerOffset / targetSize : 0.5;
  let insertAfter = consoleQueueDragState.insertAfter;
  if (targetId !== consoleQueueDragState.overId) {
    insertAfter = ratio > 0.5;
  } else if (ratio < 0.35) {
    insertAfter = false;
  } else if (ratio > 0.65) {
    insertAfter = true;
  }
  consoleQueueDragState.overId = targetId;
  consoleQueueDragState.insertAfter = insertAfter;
}

function handleConsoleQueueDragMove(event) {
  if (!consoleQueueDragState.active) return;
  consoleQueueDragState.moved = true;
  setConsoleQueueGhostPosition(event.clientX, event.clientY);
  updateConsoleQueueDragTarget(event);
}

async function handleConsoleQueueDragEnd(event) {
  if (!consoleQueueDragState.active) return;
  updateConsoleQueueDragTarget(event);
  const sourceId = consoleQueueDragState.sourceId;
  const targetId = consoleQueueDragState.overId;
  const insertAfter = consoleQueueDragState.insertAfter;
  resetConsoleQueueDragState(true);
  if (!sourceId || !targetId || sourceId === targetId) return;
  void reorderConsoleProviderQueue(sourceId, targetId, insertAfter);
}

function handleConsoleQueueDragCancel() {
  if (!consoleQueueDragState.active) return;
  resetConsoleQueueDragState(true);
}

function startConsoleQueueDrag(item, event) {
  if (!item?.inQueue || consoleQueueDragState.saving || typeof window === 'undefined') return;
  const card = event.currentTarget?.closest?.('[data-console-provider-id]');
  const rect = card?.getBoundingClientRect?.();
  consoleQueueDragState.active = true;
  consoleQueueDragState.sourceId = item.id;
  captureConsoleQueueDragLayouts();
  consoleQueueDragState.overId = '';
  consoleQueueDragState.insertAfter = false;
  consoleQueueDragState.moved = false;
  consoleQueueDragState.suppressClickUntil = Date.now() + 350;
  consoleQueueDragState.ghostWidth = Math.max(120, rect?.width || 150);
  consoleQueueDragState.ghostHeight = Math.max(56, rect?.height || 72);
  consoleQueueDragState.ghostOffsetX = Math.max(0, rect ? event.clientX - rect.left : 18);
  consoleQueueDragState.ghostOffsetY = Math.max(0, rect ? event.clientY - rect.top : 16);
  consoleQueueDragState.ghostX = event.clientX - consoleQueueDragState.ghostOffsetX;
  consoleQueueDragState.ghostY = event.clientY - consoleQueueDragState.ghostOffsetY;
  consoleQueueDragGhostX = consoleQueueDragState.ghostX;
  consoleQueueDragGhostY = consoleQueueDragState.ghostY;
  consoleQueueDragState.ghostSiteName = item.siteName || '';
  consoleQueueDragState.ghostModelLabel = item.modelLabel || '';
  consoleQueueDragState.ghostSkLabel = item.skLabel || '';
  consoleQueueDragState.ghostOrderLabel = item.queueOrder ? `P${item.queueOrder}` : '';
  event.currentTarget?.setPointerCapture?.(event.pointerId);
  void nextTick(() => setConsoleQueueGhostPosition(event.clientX, event.clientY));
  window.addEventListener('pointermove', handleConsoleQueueDragMove, true);
  window.addEventListener('pointerup', handleConsoleQueueDragEnd, true);
  window.addEventListener('pointercancel', handleConsoleQueueDragCancel, true);
}

function stopConsoleQueueDragListeners() {
  if (typeof window === 'undefined') return;
  window.removeEventListener('pointermove', handleConsoleQueueDragMove, true);
  window.removeEventListener('pointerup', handleConsoleQueueDragEnd, true);
  window.removeEventListener('pointercancel', handleConsoleQueueDragCancel, true);
}

function handleConsoleProviderCardClick(item) {
  if (consoleQueueDragState.active || consoleQueueDragState.saving || Date.now() < consoleQueueDragState.suppressClickUntil) {
    return;
  }
  void toggleConsoleProviderQueue(item);
}

function reorderConsoleQueueCardList(cards, sourceId, targetId, insertAfter) {
  const list = Array.isArray(cards) ? cards.map(card => ({ ...card })) : [];
  const sourceIndex = list.findIndex(card => card.id === sourceId);
  const targetIndex = list.findIndex(card => card.id === targetId);
  if (sourceIndex < 0 || targetIndex < 0 || sourceIndex === targetIndex) {
    return list;
  }
  const [movedCard] = list.splice(sourceIndex, 1);
  let nextIndex = list.findIndex(card => card.id === targetId);
  if (nextIndex < 0) nextIndex = list.length;
  if (insertAfter) nextIndex += 1;
  list.splice(nextIndex, 0, movedCard);
  return list.map((card, index) => ({
    ...card,
    queueOrder: index + 1,
    skLabel: card.skLabel ? card.skLabel.replace(/^SK\d+/, `SK${index + 1}`) : card.skLabel,
  }));
}

async function reorderConsoleProviderQueue(sourceId, targetId, insertAfter) {
  if (!sourceId || !targetId || sourceId === targetId) return;
  consoleQueueDragState.saving = true;
  try {
    const nextConfig = normalizeAdvancedProxyConfig(JSON.parse(JSON.stringify(advancedProxyConfigSnapshot.value || {})));
    const queue = ensureAdvancedProxyQueueSection(nextConfig, ADVANCED_PROXY_GLOBAL_QUEUE_SCOPE);
    const providers = Array.isArray(queue.providers) ? queue.providers.map((provider, index) => ({ ...provider, __queueKey: getConsoleQueueProviderKey(provider, `provider-${index + 1}`) })) : [];
    const sourceIndex = providers.findIndex(provider => provider.__queueKey === sourceId);
    const targetIndex = providers.findIndex(provider => provider.__queueKey === targetId);
    if (sourceIndex < 0 || targetIndex < 0) {
      message.warning('队列已变化，刷新后再拖动排序');
      return;
    }
    const [movedProvider] = providers.splice(sourceIndex, 1);
    let nextIndex = providers.findIndex(provider => provider.__queueKey === targetId);
    if (nextIndex < 0) nextIndex = providers.length;
    if (insertAfter) nextIndex += 1;
    providers.splice(nextIndex, 0, movedProvider);
    const reorderedProviders = providers.map(({ __queueKey, ...provider }) => provider);
    replaceAdvancedProxyQueueProviders(nextConfig, ADVANCED_PROXY_GLOBAL_QUEUE_SCOPE, reorderedProviders);
    await saveAdvancedProxyQueueConfigFast(nextConfig);
    message.success('Provider 队列顺序已更新');
  } catch (error) {
    message.error(error?.message || '更新 Provider 队列顺序失败');
  } finally {
    consoleQueueDragState.saving = false;
    stopConsoleQueueDragListeners();
  }
}

async function toggleConsoleProviderQueue(item) {
  const providerId = String(item?.rowKey || item?.id || '').trim();
  if (!providerId) return;
  try {
    const nextConfig = normalizeAdvancedProxyConfig(JSON.parse(JSON.stringify(advancedProxyConfigSnapshot.value || {})));
    const queue = ensureAdvancedProxyQueueSection(nextConfig, ADVANCED_PROXY_GLOBAL_QUEUE_SCOPE);
    const currentProviders = Array.isArray(queue.providers) ? queue.providers.map(provider => ({ ...provider })) : [];
    const existingIndex = currentProviders.findIndex(provider => {
      const id = String(provider?.id || provider?.rowKey || '').trim();
      const apiKey = String(provider?.apiKey || '').trim();
      return id === providerId || (item?.apiKey && apiKey === item.apiKey);
    });
    if (existingIndex >= 0) {
      currentProviders.splice(existingIndex, 1);
    } else {
      const sourceRecord = item?.sourceRecord || tableData.value.find(record => String(record?.rowKey || '').trim() === providerId);
      if (!sourceRecord) {
        message.warning('这条 Provider 已不在本地密钥管理中，无法重新加入队列');
        return;
      }
      currentProviders.push(buildProviderFromManagedRecord(sourceRecord, currentProviders.length + 1));
    }
    replaceAdvancedProxyQueueProviders(nextConfig, ADVANCED_PROXY_GLOBAL_QUEUE_SCOPE, currentProviders);
    await saveAdvancedProxyQueueConfigFast(nextConfig);
    message.success(existingIndex >= 0 ? '已从全局 Provider 队列移出' : '已加入全局 Provider 队列');
  } catch (error) {
    message.error(error?.message || '更新 Provider 队列失败');
  }
}

async function refreshAdvancedProxyConsoleSnapshot(event = null) {
  try {
    const eventConfig = event?.detail?.config;
    const config = eventConfig && typeof eventConfig === 'object'
      ? eventConfig
      : await getAdvancedProxyConfig();
    advancedProxyConfigSnapshot.value = await reconcileConsoleLocalAppTakeoverState(config || {});
  } catch (error) {
    console.warn('[KeyManagement] refresh advanced proxy console failed:', error);
    advancedProxyConfigSnapshot.value = normalizeAdvancedProxyConfig({});
  }
}

async function updateConsoleAdvancedProxyConfig(mutator, successMessage) {
  try {
    const savedConfig = await getAdvancedProxyConfig();
    const nextConfig = normalizeAdvancedProxyConfig(savedConfig || {});
    mutator(nextConfig);
    const syncedConfig = syncAdvancedProxyConfigSnapshotFromCurrentRecords(nextConfig);
    await setAdvancedProxyConfig(syncedConfig);
    advancedProxyConfigSnapshot.value = normalizeAdvancedProxyConfig(syncedConfig);
    if (successMessage) message.success(successMessage);
  } catch (error) {
    message.error(error?.message || '更新高级代理配置失败');
    throw error;
  }
}

function setConsoleProxyAppEnabled(config, appId, enabled) {
  if (!CONSOLE_PROXY_APP_IDS.includes(appId)) return;
  if (!config[appId] || typeof config[appId] !== 'object') {
    config[appId] = {};
  }
  config[appId].enabled = enabled === true;
}

function syncConsoleProxyMasterEnabled(config) {
  config.enabled = CONSOLE_PROXY_APP_IDS.some(appId => config?.[appId]?.enabled === true);
}

function hasConsolePendingApp(appId) {
  return consoleProxyPendingAppIds.value.includes(appId);
}

function setConsolePendingApp(appId, pending) {
  const normalized = String(appId || '').trim();
  if (!normalized) return;
  const next = new Set(consoleProxyPendingAppIds.value);
  if (pending) next.add(normalized);
  else next.delete(normalized);
  consoleProxyPendingAppIds.value = Array.from(next);
}

function getConsoleAppEnabled(appId) {
  if (Object.prototype.hasOwnProperty.call(consoleProxyOptimisticApps, appId)) {
    return consoleProxyOptimisticApps[appId] === true;
  }
  return advancedProxyConfigSnapshot.value?.[appId]?.enabled === true;
}

function resetConsoleOptimisticState() {
  Object.keys(consoleProxyOptimisticApps).forEach(key => delete consoleProxyOptimisticApps[key]);
  consoleProxyMasterOptimistic.value = null;
  consoleAntiPoisonOptimistic.value = null;
}

function markConsoleTakeoverReconcileCooldown(appIds, durationMs = CONSOLE_TAKEOVER_RECONCILE_COOLDOWN_MS) {
  const ids = Array.isArray(appIds) ? appIds : [appIds];
  const until = Date.now() + durationMs;
  ids.forEach(appId => {
    const normalized = String(appId || '').trim();
    if (CONSOLE_PROXY_APP_IDS.includes(normalized)) {
      consoleTakeoverReconcileCooldownUntil[normalized] = until;
    }
  });
}

function isConsoleTakeoverReconcileCoolingDown(appId) {
  const normalized = String(appId || '').trim();
  const until = Number(consoleTakeoverReconcileCooldownUntil[normalized] || 0);
  if (!until) return false;
  if (Date.now() <= until) return true;
  delete consoleTakeoverReconcileCooldownUntil[normalized];
  return false;
}

function waitForConsolePaint() {
  return new Promise(resolve => {
    if (typeof window === 'undefined') {
      resolve();
      return;
    }
    window.requestAnimationFrame(() => window.requestAnimationFrame(resolve));
  });
}

function detectConsoleLocalAdvancedProxyTakeoverState(snapshot, config) {
  const files = Array.isArray(snapshot?.files) ? snapshot.files : [];
  const claudeBaseUrl = normalizeConsoleComparableUrl(getAdvancedProxyAppBaseUrl('claude', config));
  const codexBaseUrl = normalizeConsoleComparableUrl(getAdvancedProxyAppBaseUrl('codex', config));
  const opencodeBaseUrl = normalizeConsoleComparableUrl(getAdvancedProxyAppBaseUrl('opencode', config));
  const openclawBaseUrl = normalizeConsoleComparableUrl(getAdvancedProxyAppBaseUrl('openclaw', config));

  const claudeSettings = parseConsoleStrictJsonObjectSafe(
    findConsoleManagedSnapshotFile(files, 'claude', 'settings')?.content || '',
    {}
  );
  const claudeEnv = isConsolePlainObject(claudeSettings?.env) ? claudeSettings.env : {};

  const codexAuth = parseConsoleStrictJsonObjectSafe(
    findConsoleManagedSnapshotFile(files, 'codex', 'auth')?.content || '',
    {}
  );
  const codexConfigText = String(findConsoleManagedSnapshotFile(files, 'codex', 'config')?.content || '');
  const codexProviderKey = extractConsoleCodexActiveProviderKey(codexConfigText);
  const codexProviderBaseUrl = normalizeConsoleComparableUrl(extractConsoleCodexProviderBaseUrl(codexConfigText, codexProviderKey));

  const opencodeConfig = parseConsoleStrictJsonObjectSafe(
    findConsoleManagedSnapshotFile(files, 'opencode', 'config')?.content || '',
    { $schema: 'https://opencode.ai/config.json' }
  );
  const openclawConfig = parseConsoleLooseJsonObjectSafe(
    findConsoleManagedSnapshotFile(files, 'openclaw', 'config')?.content || '',
    { models: { mode: 'merge', providers: {} } }
  );

  return {
    claude: normalizeConsoleComparableUrl(claudeEnv.ANTHROPIC_BASE_URL) === claudeBaseUrl
      && (isConsoleManagedProxyToken(claudeEnv.ANTHROPIC_AUTH_TOKEN) || isConsoleManagedProxyToken(claudeEnv.ANTHROPIC_API_KEY)),
    codex: isConsoleManagedProxyToken(codexAuth.OPENAI_API_KEY)
      && codexProviderBaseUrl === codexBaseUrl,
    opencode: hasMatchingConsoleOpenCodeProxyProvider(opencodeConfig, opencodeBaseUrl),
    openclaw: hasMatchingConsoleOpenClawProxyProvider(openclawConfig, openclawBaseUrl),
  };
}

async function reconcileConsoleLocalAppTakeoverState(config) {
  if (advancedProxyTakeoverReconciling || !isDesktopConfigBridgeAvailable()) {
    return normalizeAdvancedProxyConfig(config || {});
  }
  advancedProxyTakeoverReconciling = true;
  try {
    const normalizedConfig = normalizeAdvancedProxyConfig(config || {});
    const snapshot = await readManagedAppConfigFiles(CONSOLE_PROXY_APP_IDS);
    const takeoverState = detectConsoleLocalAdvancedProxyTakeoverState(snapshot, normalizedConfig);
    const mismatchedApps = CONSOLE_PROXY_APP_IDS.filter(appId =>
      normalizedConfig?.[appId]?.enabled === true && takeoverState[appId] !== true
      && !isConsoleTakeoverReconcileCoolingDown(appId)
    );
    if (!mismatchedApps.length) return normalizedConfig;

    const nextConfig = normalizeAdvancedProxyConfig(normalizedConfig);
    mismatchedApps.forEach(appId => {
      setConsoleProxyAppEnabled(nextConfig, appId, false);
    });
    syncConsoleProxyMasterEnabled(nextConfig);
    const syncedConfig = syncAdvancedProxyConfigSnapshotFromCurrentRecords(nextConfig);
    const savedConfig = await setAdvancedProxyConfig(syncedConfig);
    const labels = mismatchedApps.map(appId => CONSOLE_PROXY_APP_LABELS[appId] || appId).join(' / ');
    message.warning(`检测到 ${labels} 当前已不处于高级代理接管状态，已自动清空图标开启状态`);
    return normalizeAdvancedProxyConfig(savedConfig || syncedConfig);
  } finally {
    advancedProxyTakeoverReconciling = false;
  }
}

function getConsoleCompatibleProviderForApp(config, appId, enabledOnly = true) {
  const providers = getAdvancedProxyEffectiveProviders(config, appId, { enabledOnly });
  return providers[0] || null;
}

function getConsolePreferredModelForApp(config, appId, provider = null) {
  const directModel = String(provider?.model || '').trim();
  if (directModel) return directModel;
  const defaultModel = String(config?.claude?.defaultModel || '').trim();
  if (defaultModel) return defaultModel;
  const effectiveProviders = getAdvancedProxyEffectiveProviders(config, appId, { enabledOnly: false });
  const providerWithModel = effectiveProviders.find(item => String(item?.model || '').trim());
  if (providerWithModel) return String(providerWithModel.model || '').trim();
  const globalProviders = getAdvancedProxyQueueProviders(config, ADVANCED_PROXY_GLOBAL_QUEUE_SCOPE, { effective: false });
  return String(globalProviders.find(item => String(item?.model || '').trim())?.model || '').trim();
}

function createConsoleTakeoverDesktopDraft(appId, enabled, config) {
  const sourceProvider = getConsoleCompatibleProviderForApp(config, appId, true);
  const model = getConsolePreferredModelForApp(config, appId, sourceProvider);
  if (!model) {
    throw new Error('请先给 Provider 补一个模型，再启用该应用接管');
  }

  if (!enabled && !sourceProvider) {
    throw new Error(appId === 'claude'
      ? '当前没有可回退的 Claude 上游 Provider'
      : '当前没有可回退的 OpenAI 兼容上游 Provider');
  }

  const endpoint = enabled ? getAdvancedProxyAppBaseUrl(appId, config) : String(sourceProvider?.baseUrl || '').trim();
  const apiKey = enabled ? PROXY_MANAGED_TOKEN : String(sourceProvider?.apiKey || '').trim();
  const providerName = enabled ? ADVANCED_PROXY_PROVIDER_NAME : String(sourceProvider?.name || 'Custom Provider').trim();

  if (!endpoint) throw new Error('缺少可写入的目标地址');
  if (!apiKey) throw new Error('缺少可写入的 API Key');

  const nextDraft = createDesktopConfigDraft({
    siteName: providerName,
    siteUrl: endpoint,
    apiKey,
    selectedModel: model,
    quickTestModel: model,
  });

  nextDraft.selectedApps = [appId];
  nextDraft.providerName = providerName;
  nextDraft.providerKey = 'custom';
  nextDraft.forceCustomProviderKey = true;
  nextDraft.endpoint = endpoint;
  nextDraft.apiKey = apiKey;
  nextDraft.model = model;
  nextDraft.claudeBaseUrl = appId === 'claude' ? endpoint : String(sourceProvider?.baseUrl || endpoint).trim();
  nextDraft.claudeApiKeyField = enabled ? 'ANTHROPIC_AUTH_TOKEN' : String(sourceProvider?.apiKeyField || 'ANTHROPIC_AUTH_TOKEN').trim();
  nextDraft.codexBaseUrl = appId === 'codex' ? endpoint : String(sourceProvider?.baseUrl || endpoint).trim();
  nextDraft.opencodeBaseUrl = appId === 'opencode' ? endpoint : String(sourceProvider?.baseUrl || endpoint).trim();
  nextDraft.openclawBaseUrl = appId === 'openclaw' ? endpoint : String(sourceProvider?.baseUrl || endpoint).trim();
  nextDraft.claudeUseAdvancedProxy = false;
  nextDraft.codexUseAdvancedProxy = false;
  nextDraft.opencodeUseAdvancedProxy = false;
  nextDraft.openclawUseAdvancedProxy = false;
  return nextDraft;
}

async function applyConsoleTakeoverConfig(appId, enabled, savedConfig, nextConfig, preview) {
  const writes = Array.isArray(preview?.writes) ? preview.writes : [];
  const syncedConfig = syncAdvancedProxyConfigSnapshotFromCurrentRecords(nextConfig);

  if (!writes.length) {
    const saved = await setAdvancedProxyConfig(syncedConfig);
    advancedProxyConfigSnapshot.value = normalizeAdvancedProxyConfig(saved);
    return 0;
  }

  if (!enabled) {
    await applyManagedAppConfigFiles(writes);
    const saved = await setAdvancedProxyConfig(syncedConfig);
    advancedProxyConfigSnapshot.value = normalizeAdvancedProxyConfig(saved);
    return writes.length;
  }

  const saved = await setAdvancedProxyConfig(syncedConfig);
  advancedProxyConfigSnapshot.value = normalizeAdvancedProxyConfig(saved);
  try {
    await applyManagedAppConfigFiles(writes);
    return writes.length;
  } catch (error) {
    await setAdvancedProxyConfig(savedConfig);
    advancedProxyConfigSnapshot.value = normalizeAdvancedProxyConfig(savedConfig);
    throw error;
  }
}

async function toggleConsoleProxyMaster(value) {
  const enabled = value === true;
  if (consoleProxyMasterPending.value) return;
  if (!enabled) {
    markConsoleTakeoverReconcileCooldown(CONSOLE_PROXY_APP_IDS);
  }
  consoleProxyMasterPending.value = true;
  consoleProxyMasterOptimistic.value = enabled;
  CONSOLE_PROXY_APP_IDS.forEach(appId => {
    consoleProxyOptimisticApps[appId] = enabled;
    setConsolePendingApp(appId, true);
  });
  await waitForConsolePaint();
  try {
    await updateConsoleAdvancedProxyConfig(nextConfig => {
      CONSOLE_PROXY_APP_IDS.forEach(appId => {
        setConsoleProxyAppEnabled(nextConfig, appId, enabled);
      });
      nextConfig.enabled = enabled;
    }, enabled ? '已开启四个客户端高级代理' : '已关闭全部客户端高级代理');
  } catch {
    await refreshAdvancedProxyConsoleSnapshot();
  } finally {
    consoleProxyMasterOptimistic.value = null;
    CONSOLE_PROXY_APP_IDS.forEach(appId => {
      delete consoleProxyOptimisticApps[appId];
      setConsolePendingApp(appId, false);
    });
    consoleProxyMasterPending.value = false;
  }
}

async function toggleConsoleProxyApp(appId) {
  if (hasConsolePendingApp(appId)) return;
  if (!isDesktopConfigBridgeAvailable()) {
    message.warning('客户端高级代理接管仅支持桌面版 EXE 运行环境');
    return;
  }
  const appLabel = CONSOLE_PROXY_APP_LABELS[appId] || appId;
  const beforeEnabled = getConsoleAppEnabled(appId);
  const enabled = !beforeEnabled;
  if (!enabled) {
    markConsoleTakeoverReconcileCooldown(appId);
  }
  consoleProxyOptimisticApps[appId] = enabled;
  setConsolePendingApp(appId, true);
  consoleProxyConfigApplying.value = true;
  await waitForConsolePaint();
  try {
    const savedConfigRaw = await getAdvancedProxyConfig();
    const savedConfig = normalizeAdvancedProxyConfig(savedConfigRaw || {});
    const nextConfig = normalizeAdvancedProxyConfig(savedConfig);
    setConsoleProxyAppEnabled(nextConfig, appId, enabled);
    syncConsoleProxyMasterEnabled(nextConfig);

    const app = ADVANCED_PROXY_APPS.find(item => item.id === appId);
    const desktopDraft = createConsoleTakeoverDesktopDraft(appId, enabled, nextConfig);
    const snapshot = await readManagedAppConfigFiles([appId]);
    const preview = buildDesktopConfigPreview(desktopDraft, snapshot);
    if (!preview.appGroups.length && preview.errors.length) {
      throw new Error(preview.errors.join('；'));
    }

    const writeCount = await applyConsoleTakeoverConfig(appId, enabled, savedConfig, nextConfig, preview);
    const writeText = writeCount ? `，已写入 ${writeCount} 个本地配置文件` : '';
    if (preview.errors.length) {
      message.warning(`部分配置预览失败：${preview.errors.join('；')}`);
    }
    message.success(`${app?.label || appLabel} 高级代理已${enabled ? '开启' : '关闭'}${writeText}`);
  } catch (error) {
    consoleProxyOptimisticApps[appId] = beforeEnabled;
    await refreshAdvancedProxyConsoleSnapshot();
    message.error(error?.message || `${appLabel} 接管配置写入失败`);
  } finally {
    delete consoleProxyOptimisticApps[appId];
    setConsolePendingApp(appId, false);
    consoleProxyConfigApplying.value = false;
  }
}

async function toggleConsoleAntiPoison() {
  if (consoleAntiPoisonPending.value) return;
  const beforeEnabled = consoleAntiPoisonEnabled.value;
  const enabled = !beforeEnabled;
  consoleAntiPoisonOptimistic.value = enabled;
  consoleAntiPoisonPending.value = true;
  await waitForConsolePaint();
  try {
    await updateConsoleAdvancedProxyConfig(nextConfig => {
      if (!nextConfig.antiPoison || typeof nextConfig.antiPoison !== 'object') {
        nextConfig.antiPoison = normalizeAdvancedProxyConfig({}).antiPoison;
      }
      nextConfig.antiPoison.enabled = enabled;
    }, enabled ? '防投毒已开启' : '防投毒已关闭');
  } catch {
    consoleAntiPoisonOptimistic.value = beforeEnabled;
    await refreshAdvancedProxyConsoleSnapshot();
  } finally {
    consoleAntiPoisonOptimistic.value = null;
    consoleAntiPoisonPending.value = false;
  }
}

async function refreshAdvancedProxyConsoleRecords() {
  if (!isAdvancedProxyRequestRecordBridgeAvailable()) {
    advancedProxyConsoleRecords.value = [];
    return;
  }
  advancedProxyConsoleRecordsLoading.value = true;
  try {
    const records = await listAdvancedProxyRequestRecords(80);
    const normalizedRecords = Array.isArray(records) ? records : [];
    advancedProxyConsoleRecords.value = normalizedRecords;
    const nextLines = [];
    [...normalizedRecords].reverse().forEach(record => {
      const recordId = String(record?.id || '').trim();
      if (!recordId || advancedProxyConsoleRecordIds.value.has(recordId)) return;
      advancedProxyConsoleRecordIds.value.add(recordId);
      nextLines.push(formatAdvancedProxyConsoleRecordLog(record));
    });
    if (nextLines.length) {
      advancedProxyConsoleLogLines.value = [
        ...advancedProxyConsoleLogLines.value,
        ...nextLines,
      ].slice(-240);
      scrollAdvancedProxyConsoleLogToBottom();
    }
  } catch (error) {
    console.warn('[KeyManagement] refresh advanced proxy console records failed:', error);
  } finally {
    advancedProxyConsoleRecordsLoading.value = false;
  }
}

async function refreshAdvancedProxyActiveConnections() {
  if (!isAdvancedProxyActiveConnectionBridgeAvailable()) {
    advancedProxyActiveConnections.value = [];
    selectedAdvancedProxyConnectionId.value = '';
    return;
  }
  advancedProxyActiveConnectionsLoading.value = true;
  try {
    const connections = await listAdvancedProxyActiveConnections();
    advancedProxyActiveConnections.value = Array.isArray(connections) ? connections : [];
    if (
      selectedAdvancedProxyConnectionId.value &&
      !advancedProxyActiveConnections.value.some(connection => String(connection?.id || '') === selectedAdvancedProxyConnectionId.value)
    ) {
      selectedAdvancedProxyConnectionId.value = '';
    }
  } catch (error) {
    console.warn('[KeyManagement] refresh advanced proxy active connections failed:', error);
    advancedProxyActiveConnections.value = [];
    selectedAdvancedProxyConnectionId.value = '';
  } finally {
    advancedProxyActiveConnectionsLoading.value = false;
  }
}

function startAdvancedProxyConsolePolling() {
  if (advancedProxyConsolePollingTimer || typeof window === 'undefined') return;
  advancedProxyConsolePollingTimer = window.setInterval(() => {
    if (activeInventoryPanel.value !== 'console') return;
    void refreshAdvancedProxyConsoleRecords();
    void refreshAdvancedProxyActiveConnections();
  }, 2000);
}

function startAdvancedProxyConnectionClock() {
  if (advancedProxyConnectionClockTimer || typeof window === 'undefined') return;
  advancedProxyConnectionClockTimer = window.setInterval(() => {
    advancedProxyConnectionClockTick.value = Date.now();
  }, 1000);
}

function stopAdvancedProxyConsolePolling() {
  if (!advancedProxyConsolePollingTimer) return;
  clearInterval(advancedProxyConsolePollingTimer);
  advancedProxyConsolePollingTimer = null;
}

function stopAdvancedProxyConnectionClock() {
  if (!advancedProxyConnectionClockTimer) return;
  clearInterval(advancedProxyConnectionClockTimer);
  advancedProxyConnectionClockTimer = null;
}

function setActiveInventoryPanel(panel) {
  activeInventoryPanel.value = panel === 'console' ? 'console' : 'local';
  if (activeInventoryPanel.value === 'console') {
    void refreshAdvancedProxyConsoleSnapshot();
    void refreshAdvancedProxyConsoleRecords();
    void refreshAdvancedProxyActiveConnections();
    startAdvancedProxyConsolePolling();
    startAdvancedProxyConnectionClock();
  } else {
    stopAdvancedProxyConsolePolling();
    stopAdvancedProxyConnectionClock();
  }
}

function syncThemeState() {
  isDarkMode.value = isDarkThemeMode(getAppliedThemeMode());
}

function getSidebarPopupContainer(triggerNode) {
  return triggerNode?.ownerDocument?.body || document.body;
}

function getQuickGroupPopupContainer(triggerNode) {
  return triggerNode?.closest?.('.key-quick-group-floating-panel') || triggerNode?.ownerDocument?.body || document.body;
}

function buildKeyGroupId() {
  return `group::${Date.now()}::${Math.random().toString(36).slice(2, 7)}`;
}

function normalizeRecordGroupIds(value) {
  if (!Array.isArray(value)) return [];
  return Array.from(new Set(value.map(item => String(item || '').trim()).filter(Boolean)));
}

function normalizeGroupSelectedModels(value) {
  if (!value || typeof value !== 'object' || Array.isArray(value)) return {};
  return Object.fromEntries(
    Object.entries(value)
      .map(([groupId, model]) => [String(groupId || '').trim(), String(model || '').trim()])
      .filter(([groupId, model]) => groupId && model)
  );
}

function getScopedGroupId(groupId = activeKeyGroupId.value) {
  const normalized = String(groupId || '').trim();
  if (!normalized || normalized === ALL_KEYS_GROUP_ID) return '';
  return normalized;
}

function getActiveKeyGroupName(groupId = activeKeyGroupId.value) {
  const scopedGroupId = getScopedGroupId(groupId);
  if (!scopedGroupId) return '全部密钥';
  return keyGroups.value.find(group => group.id === scopedGroupId)?.name || '当前分组';
}

function getRecordSelectedModelValue(record, groupId = activeKeyGroupId.value) {
  const scopedGroupId = getScopedGroupId(groupId);
  const groupSelectedModels = normalizeGroupSelectedModels(record?.groupSelectedModels);
  const groupModel = scopedGroupId ? String(groupSelectedModels?.[scopedGroupId] || '').trim() : '';
  if (groupModel) return groupModel;
  return String(record?.selectedModel || '').trim();
}

function setRecordSelectedModelValue(record, value, groupId = activeKeyGroupId.value) {
  const normalizedValue = normalizeModels([value])[0] || '';
  const scopedGroupId = getScopedGroupId(groupId);
  if (!scopedGroupId) {
    record.selectedModel = normalizedValue;
    return normalizedValue;
  }
  const nextGroupSelectedModels = normalizeGroupSelectedModels(record?.groupSelectedModels);
  if (normalizedValue) nextGroupSelectedModels[scopedGroupId] = normalizedValue;
  else delete nextGroupSelectedModels[scopedGroupId];
  record.groupSelectedModels = nextGroupSelectedModels;
  return normalizedValue;
}

function pickScopedModelForGroup(record, modelsSet, groupId = '') {
  const availableModels = buildMergedModelList(record);
  const scopedSelectedModel = getRecordSelectedModelValue(record, groupId);
  if (scopedSelectedModel && modelsSet.has(scopedSelectedModel)) return scopedSelectedModel;
  const globalSelectedModel = String(record?.selectedModel || '').trim();
  if (globalSelectedModel && modelsSet.has(globalSelectedModel)) return globalSelectedModel;
  const matchedModels = availableModels.filter(model => modelsSet.has(model));
  return pickPreferredModel(matchedModels) || matchedModels[0] || '';
}

function loadStoredKeyGroups() {
  try {
    const raw = localStorage.getItem(KEY_GROUPS_STORAGE_KEY);
    const parsed = JSON.parse(raw || '[]');
    if (!Array.isArray(parsed)) return [];
    return parsed
      .map(item => ({
        id: String(item?.id || '').trim(),
        name: String(item?.name || '').trim(),
        createdAt: Number(item?.createdAt || Date.now()),
      }))
      .filter(item => item.id && item.name);
  } catch (error) {
    console.error(error);
    return [];
  }
}

function persistKeyGroups() {
  localStorage.setItem(KEY_GROUPS_STORAGE_KEY, JSON.stringify(keyGroups.value));
}

function normalizeQuickFilterName(name) {
  const normalized = String(name || '').trim();
  if (!normalized) return '';
  const withoutVendor = normalized.includes('/') ? normalized.split('/').pop() : normalized;
  return String(withoutVendor || '').trim();
}

function normalizeQuickFilterVersion(version) {
  const normalized = String(version || '').trim();
  if (!normalized) return '';
  return normalized
    .replace(/(\.\d*?[1-9])0+$/u, '$1')
    .replace(/\.0+$/u, '');
}

function extractGptSubfamilyMeta(normalizedName) {
  const normalized = String(normalizedName || '').trim().toLowerCase();
  if (!normalized) return null;
  const match = normalized.match(/^gpt-([a-z][a-z0-9-]*)[:_-](\d+(?:\.\d+)?[a-z]?)(?:$|[-_:])/u);
  if (!match) return null;
  return {
    category: `gpt-${match[1]}`,
    version: normalizeQuickFilterVersion(match[2]),
  };
}

function extractQuickFilterCategory(name) {
  const normalized = normalizeQuickFilterName(name).toLowerCase();
  if (!normalized) return '';
  const gptSubfamily = extractGptSubfamilyMeta(normalized);
  if (gptSubfamily?.category) return gptSubfamily.category;
  const match = normalized.match(/gpt|[a-zA-Z]{3,}/i);
  return match ? match[0].toLowerCase() : '';
}

function extractQuickFilterVersion(name) {
  const normalized = normalizeQuickFilterName(name).toLowerCase();
  if (!normalized) return '';
  const gptSubfamily = extractGptSubfamilyMeta(normalized);
  if (gptSubfamily?.version) return gptSubfamily.version;
  const match = normalized.match(/\d+(?:\.\d+)?/);
  return match ? normalizeQuickFilterVersion(match[0]) : '';
}

function resolveQuickFilterFamilyKey(category) {
  const normalized = String(category || '').trim().toLowerCase();
  if (!normalized) return '';
  if (normalized.startsWith('gpt-')) return 'gpt';
  return normalized;
}

function buildQuickFilterOptionLabel(category, version, sampleName) {
  if (version) return `${category}-${version}`;
  return normalizeQuickFilterName(sampleName || category);
}

function getGroupRecordCount(groupId) {
  if (!groupId) return 0;
  return allSortedRows.value.filter(record => normalizeRecordGroupIds(record?.groupIds).includes(groupId)).length;
}

function resetQuickGroupComposer() {
  activeQuickGroupFilters.value = [];
  createKeyGroupDraftName.value = '';
  quickGroupDraftNameMode.value = 'auto';
}

function handleQuickGroupPopoverOpenChange(open) {
  quickGroupPopoverOpen.value = Boolean(open);
  if (open) {
    resetQuickGroupComposer();
  }
}

function openQuickGroupPopover() {
  if (quickGroupPopoverOpen.value) return;
  quickGroupPopoverOpen.value = true;
  resetQuickGroupComposer();
}

function closeQuickGroupPopover() {
  quickGroupPopoverOpen.value = false;
}

function toggleQuickGroupPopover() {
  if (quickGroupPopoverOpen.value) {
    closeQuickGroupPopover();
    return;
  }
  openQuickGroupPopover();
}

function closeKeyGroupContextMenu() {
  keyGroupContextMenu.open = false;
  keyGroupContextMenu.x = 0;
  keyGroupContextMenu.y = 0;
  keyGroupContextMenu.group = null;
  keyGroupContextMenu.mergeSubmenuOpen = false;
}

function closeAllContextMenus() {
  closeRowContextMenu();
  closeKeyGroupContextMenu();
  closeAdvancedProxyConnectionContextMenu();
}

function openKeyGroupContextMenu(group, event) {
  if (!group?.id || !event) return;
  event.preventDefault();
  event.stopPropagation();
  const viewportWidth = typeof window !== 'undefined' ? window.innerWidth : 0;
  const viewportHeight = typeof window !== 'undefined' ? window.innerHeight : 0;
  const menuWidth = 208;
  const menuHeight = 62;
  const edgePadding = 12;
  const triggerRect = event.currentTarget?.getBoundingClientRect?.() || null;
  const anchorX = triggerRect ? triggerRect.left : event.clientX;
  const anchorY = triggerRect ? (triggerRect.bottom + 6) : event.clientY;
  keyGroupContextMenu.group = group;
  keyGroupContextMenu.mergeSubmenuOpen = false;
  keyGroupContextMenu.x = viewportWidth > 0
    ? Math.max(edgePadding, Math.min(anchorX, viewportWidth - menuWidth - edgePadding))
    : anchorX;
  keyGroupContextMenu.y = viewportHeight > 0
    ? Math.max(edgePadding, Math.min(anchorY, viewportHeight - menuHeight - edgePadding))
    : anchorY;
  keyGroupContextMenu.open = true;
}

function deleteKeyGroup(groupId) {
  if (!groupId) return;
  keyGroups.value = keyGroups.value.filter(group => group.id !== groupId);
  if (activeKeyGroupId.value === groupId) {
    activeKeyGroupId.value = ALL_KEYS_GROUP_ID;
    currentTablePage.value = 1;
  }
  tableData.value.forEach(record => {
    record.groupIds = normalizeRecordGroupIds(record.groupIds).filter(id => id !== groupId);
    const nextGroupSelectedModels = normalizeGroupSelectedModels(record.groupSelectedModels);
    delete nextGroupSelectedModels[groupId];
    record.groupSelectedModels = nextGroupSelectedModels;
  });
  persistKeyGroups();
  persistRecords();
}

function handleKeyGroupContextDelete() {
  const group = keyGroupContextMenu.group;
  closeKeyGroupContextMenu();
  if (!group?.id) return;
  Modal.confirm({
    title: `确认删除分组「${group.name}」？`,
    content: '将移除该分组下的成员关系，并清空该分组的独立模型选择。',
    okText: '删除',
    cancelText: '取消',
    okButtonProps: { danger: true },
    onOk: () => {
      deleteKeyGroup(group.id);
      message.success(`已删除分组：${group.name}`);
    },
  });
}

function openRenameKeyGroupModalFromContext() {
  const group = keyGroupContextMenu.group;
  if (!group?.id) return;
  renameKeyGroupTargetId.value = String(group.id || '').trim();
  renameKeyGroupDraftName.value = String(group.name || '').trim();
  closeKeyGroupContextMenu();
  renameKeyGroupModalOpen.value = true;
}

function closeRenameKeyGroupModal() {
  renameKeyGroupModalOpen.value = false;
  renameKeyGroupSaving.value = false;
  renameKeyGroupDraftName.value = '';
  renameKeyGroupTargetId.value = '';
}

async function submitRenameKeyGroup() {
  const targetId = String(renameKeyGroupTargetId.value || '').trim();
  const nextName = String(renameKeyGroupDraftName.value || '').trim();
  if (!targetId) {
    closeRenameKeyGroupModal();
    return;
  }
  if (!nextName) {
    message.warning('请先输入分组名称');
    return;
  }
  if (nextName === '全部密钥') {
    message.warning('该名称已被默认分组占用');
    return;
  }
  const currentGroup = keyGroups.value.find(group => String(group?.id || '').trim() === targetId);
  if (!currentGroup) {
    closeRenameKeyGroupModal();
    return;
  }
  if (keyGroups.value.some(group => String(group?.id || '').trim() !== targetId && group.name === nextName)) {
    message.warning('分组名称已存在');
    return;
  }
  renameKeyGroupSaving.value = true;
  try {
    currentGroup.name = nextName;
    persistKeyGroups();
    closeRenameKeyGroupModal();
    message.success(`已重命名分组为：${nextName}`);
  } finally {
    renameKeyGroupSaving.value = false;
  }
}

function mergeKeyGroupIntoTarget(sourceGroupId, targetGroupId) {
  const sourceId = String(sourceGroupId || '').trim();
  const targetId = String(targetGroupId || '').trim();
  if (!sourceId || !targetId || sourceId === targetId) return;
  tableData.value.forEach(record => {
    const currentGroupIds = normalizeRecordGroupIds(record.groupIds);
    if (!currentGroupIds.includes(sourceId)) return;
    record.groupIds = currentGroupIds.includes(targetId)
      ? currentGroupIds.filter(id => id !== sourceId)
      : currentGroupIds.map(id => (id === sourceId ? targetId : id));
    const nextGroupSelectedModels = normalizeGroupSelectedModels(record.groupSelectedModels);
    const sourceModel = String(nextGroupSelectedModels[sourceId] || '').trim();
    const targetModel = String(nextGroupSelectedModels[targetId] || '').trim();
    if (!targetModel && sourceModel) {
      nextGroupSelectedModels[targetId] = sourceModel;
    }
    delete nextGroupSelectedModels[sourceId];
    record.groupSelectedModels = nextGroupSelectedModels;
  });
  keyGroups.value = keyGroups.value.filter(group => String(group?.id || '').trim() !== sourceId);
  if (activeKeyGroupId.value === sourceId) {
    activeKeyGroupId.value = targetId;
    currentTablePage.value = 1;
  }
  persistKeyGroups();
  persistRecords();
}

function handleMergeKeyGroupInto(targetGroup) {
  const sourceGroup = keyGroupContextMenu.group;
  const sourceId = String(sourceGroup?.id || '').trim();
  const targetId = String(targetGroup?.id || '').trim();
  if (!sourceId || !targetId || sourceId === targetId) return;
  closeKeyGroupContextMenu();
  Modal.confirm({
    title: `是否确认合并进入目标分组「${targetGroup.name}」？`,
    content: `将把分组「${sourceGroup.name}」的成员关系与独立模型选择并入「${targetGroup.name}」，随后删除原分组。`,
    okText: '确认合并',
    cancelText: '取消',
    onOk: () => {
      mergeKeyGroupIntoTarget(sourceId, targetId);
      message.success(`已将分组「${sourceGroup.name}」合并到「${targetGroup.name}」`);
    },
  });
}

function handleQuickGroupDraftNameInput() {
  quickGroupDraftNameMode.value = 'manual';
}

function setActiveKeyGroup(groupId) {
  activeKeyGroupId.value = groupId || ALL_KEYS_GROUP_ID;
  currentTablePage.value = 1;
}

function openCreateKeyGroupModal() {
  createKeyGroupDraftName.value = '';
  createKeyGroupModalOpen.value = true;
}

function openCreateKeyGroupModalFromContext() {
  openCreateKeyGroupModal();
}

function closeCreateKeyGroupModal() {
  createKeyGroupModalOpen.value = false;
  createKeyGroupDraftName.value = '';
}

async function submitCreateKeyGroup() {
  const name = String(createKeyGroupDraftName.value || '').trim();
  if (!name) {
    message.warning('请先输入分组名称');
    return;
  }
  if (name === '全部密钥') {
    message.warning('该名称已被默认分组占用');
    return;
  }
  if (keyGroups.value.some(group => group.name === name)) {
    message.warning('分组名称已存在');
    return;
  }
  createKeyGroupSaving.value = true;
  try {
    const newGroup = {
      id: buildKeyGroupId(),
      name,
      createdAt: Date.now(),
    };
    keyGroups.value = [...keyGroups.value, newGroup];
    persistKeyGroups();
    activeKeyGroupId.value = newGroup.id;
    closeCreateKeyGroupModal();
    message.success(`已创建分组：${name}`);
  } finally {
    createKeyGroupSaving.value = false;
  }
}

function isRecordInGroup(record, groupId) {
  return normalizeRecordGroupIds(record?.groupIds).includes(groupId);
}

function toggleRecordGroupMembership(record, groupId) {
  if (!record || !groupId) return;
  const current = normalizeRecordGroupIds(record.groupIds);
  record.groupIds = current.includes(groupId)
    ? current.filter(id => id !== groupId)
    : [...current, groupId];
  persistRecords();
}

function isValidProviderQueueSourceRecord(record) {
  if (!record?.rowKey || !record?.siteUrl || !record?.apiKey) return false;
  return Number(record?.status || 0) === 1;
}

function buildProviderFromManagedRecord(record, sortIndex) {
  return {
    id: record.rowKey,
    rowKey: record.rowKey,
    name: record.siteName || record.siteUrl || 'Provider',
    baseUrl: record.siteUrl,
    apiKey: record.apiKey,
    model: getRecordSelectedModelValue(record) || record.quickTestModel || '',
    apiFormat: 'openai_responses',
    apiKeyField: 'ANTHROPIC_AUTH_TOKEN',
    enabled: true,
    sortIndex,
    sourceType: record.sourceType || 'auto',
  };
}

function ensureAdvancedProxyQueueSection(config, scope = ADVANCED_PROXY_GLOBAL_QUEUE_SCOPE) {
  if (!config.queues || typeof config.queues !== 'object') {
    config.queues = {};
  }
  if (!config.queues[scope] || typeof config.queues[scope] !== 'object') {
    config.queues[scope] = {
      inheritGlobal: scope !== ADVANCED_PROXY_GLOBAL_QUEUE_SCOPE,
      providers: [],
    };
  }
  if (!Array.isArray(config.queues[scope].providers)) {
    config.queues[scope].providers = [];
  }
  if (scope === ADVANCED_PROXY_GLOBAL_QUEUE_SCOPE) {
    config.queues[scope].inheritGlobal = false;
  }
  return config.queues[scope];
}

function replaceAdvancedProxyQueueProviders(config, scope, providers) {
  const queue = ensureAdvancedProxyQueueSection(config, scope);
  queue.providers = providers.map((provider, index) => ({
    ...provider,
    enabled: provider?.enabled !== false,
    sortIndex: index + 1,
  }));
  if (scope !== ADVANCED_PROXY_GLOBAL_QUEUE_SCOPE) {
    queue.inheritGlobal = false;
  }
}

async function saveAdvancedProxyQueueConfigFast(nextConfig) {
  nextConfig.claude.providers = [...(nextConfig.queues?.[ADVANCED_PROXY_GLOBAL_QUEUE_SCOPE]?.providers || [])];
  const normalizedConfig = normalizeAdvancedProxyConfig(nextConfig);
  advancedProxyConfigSnapshot.value = normalizedConfig;
  void setAdvancedProxyConfigOptimistic(normalizedConfig, {
    onError: async (error) => {
      message.error(error?.message || '后台保存 Provider 队列失败，已刷新配置');
      try {
        const savedConfig = await getAdvancedProxyConfig();
        advancedProxyConfigSnapshot.value = normalizeAdvancedProxyConfig(savedConfig || {});
      } catch {}
    },
  });
  return normalizedConfig;
}

function appendAdvancedProxyQueueProviders(config, scope, providers) {
  const queue = ensureAdvancedProxyQueueSection(config, scope);
  const existingProviders = Array.isArray(queue.providers) ? queue.providers : [];
  const dedupe = new Map();
  existingProviders.forEach(provider => {
    const id = String(provider?.id || provider?.rowKey || '').trim();
    const apiKey = String(provider?.apiKey || '').trim();
    const model = String(provider?.model || '').trim();
    const dedupeKey = `${id}::${apiKey}::${model}`;
    if (!dedupe.has(dedupeKey)) {
      dedupe.set(dedupeKey, {
        ...provider,
        enabled: provider?.enabled !== false,
      });
    }
  });
  providers.forEach(provider => {
    const id = String(provider?.id || provider?.rowKey || '').trim();
    const apiKey = String(provider?.apiKey || '').trim();
    const model = String(provider?.model || '').trim();
    const dedupeKey = `${id}::${apiKey}::${model}`;
    if (!dedupe.has(dedupeKey)) {
      dedupe.set(dedupeKey, {
        ...provider,
        enabled: provider?.enabled !== false,
      });
    }
  });
  queue.providers = Array.from(dedupe.values()).map((provider, index) => ({
    ...provider,
    sortIndex: index + 1,
  }));
  if (scope !== ADVANCED_PROXY_GLOBAL_QUEUE_SCOPE) {
    queue.inheritGlobal = false;
  }
}

function clearAdvancedProxyQueueProviders(config, scope) {
  const queue = ensureAdvancedProxyQueueSection(config, scope);
  queue.providers = [];
  if (scope !== ADVANCED_PROXY_GLOBAL_QUEUE_SCOPE) {
    queue.inheritGlobal = false;
  }
}

function isQuickGroupFilterFamilyFullySelected(family) {
  return family.options.length > 0 && family.options.every(option => activeQuickGroupFilters.value.includes(option.key));
}

function isQuickGroupFilterFamilyActive(family) {
  return family.options.some(option => activeQuickGroupFilters.value.includes(option.key));
}

function getQuickGroupFilterFamilyActiveCount(family) {
  return family.options.filter(option => activeQuickGroupFilters.value.includes(option.key)).length;
}

function getQuickGroupModelsFromOptionKeys(optionKeys) {
  const selectedModels = new Set();
  quickGroupFilters.value.forEach(family => {
    family.options.forEach(option => {
      if (!optionKeys.includes(option.key)) return;
      option.models.forEach(model => selectedModels.add(model));
    });
  });
  return selectedModels;
}

function getQuickGroupLabelsFromOptionKeys(optionKeys) {
  const labels = [];
  quickGroupFilters.value.forEach(family => {
    family.options.forEach(option => {
      if (optionKeys.includes(option.key)) labels.push(option.label);
    });
  });
  return labels;
}

function syncQuickGroupDraftNameFromFilters() {
  if (quickGroupDraftNameMode.value !== 'auto') return;
  createKeyGroupDraftName.value = getQuickGroupLabelsFromOptionKeys(activeQuickGroupFilters.value).join('+');
}

function applyQuickGroupPresetSelection(optionKeys) {
  const normalized = Array.from(new Set((Array.isArray(optionKeys) ? optionKeys : []).filter(Boolean)));
  activeQuickGroupFilters.value = normalized;
  syncQuickGroupDraftNameFromFilters();
}

function submitQuickGroupComposer() {
  const normalized = Array.from(new Set(activeQuickGroupFilters.value.filter(Boolean)));
  const inferredName = getQuickGroupLabelsFromOptionKeys(normalized).join('+');
  const groupName = String(createKeyGroupDraftName.value || '').trim() || inferredName;
  if (!groupName) {
    message.warning('请先输入组名，或选择下方快捷项');
    return;
  }

  const modelsSet = getQuickGroupModelsFromOptionKeys(normalized);
  let targetGroup = keyGroups.value.find(group => group.name === groupName) || null;
  if (!targetGroup) {
    targetGroup = {
      id: buildKeyGroupId(),
      name: groupName,
      createdAt: Date.now(),
    };
    keyGroups.value = [...keyGroups.value, targetGroup];
    persistKeyGroups();
  }

  let changedCount = 0;
  if (modelsSet.size > 0) {
    tableData.value.forEach(record => {
      const recordModels = buildMergedModelList(record);
      const matched = recordModels.some(model => modelsSet.has(model));
      if (!matched) return;
      const current = normalizeRecordGroupIds(record.groupIds);
      const nextSelectedModel = pickScopedModelForGroup(record, modelsSet, targetGroup.id);
      let changed = false;
      if (!current.includes(targetGroup.id)) {
        record.groupIds = [...current, targetGroup.id];
        changed = true;
      }
      if (nextSelectedModel) {
        const currentScopedModel = getRecordSelectedModelValue(record, targetGroup.id);
        if (currentScopedModel !== nextSelectedModel) {
          setRecordSelectedModelValue(record, nextSelectedModel, targetGroup.id);
          changed = true;
        }
      }
      if (changed) changedCount += 1;
    });
  }
  if (changedCount > 0) {
    persistRecords();
  }
  activeKeyGroupId.value = targetGroup.id;
  currentTablePage.value = 1;
  quickGroupPopoverOpen.value = false;
  message.success(modelsSet.size > 0 ? `已创建快捷分组：${groupName}` : `已创建空分组：${groupName}`);
}

function toggleQuickGroupFilter(optionKey) {
  const current = new Set(activeQuickGroupFilters.value);
  if (current.has(optionKey)) current.delete(optionKey);
  else current.add(optionKey);
  applyQuickGroupPresetSelection(Array.from(current));
}

function clearQuickGroupFilters() {
  activeQuickGroupFilters.value = [];
  if (quickGroupDraftNameMode.value === 'auto') {
    createKeyGroupDraftName.value = '';
  }
}

function selectQuickGroupFilterFamily(family) {
  const current = new Set(activeQuickGroupFilters.value);
  if (isQuickGroupFilterFamilyFullySelected(family)) {
    family.options.forEach(option => current.delete(option.key));
  } else {
    family.options.forEach(option => current.add(option.key));
  }
  applyQuickGroupPresetSelection(Array.from(current));
}

function closeRowContextMenu() {
  rowContextMenu.open = false;
  rowContextMenu.record = null;
  rowContextMenu.records = [];
  rowContextMenu.batch = false;
  rowContextMenu.groupSubmenuOpen = false;
}

function normalizeRowKeyValue(value) {
  return String(value || '').trim();
}

function isRowSelected(record) {
  const rowKey = normalizeRowKeyValue(record?.rowKey);
  if (!rowKey) return false;
  return selectedRowKeys.value.includes(rowKey);
}

function toggleRowSelected(record) {
  const rowKey = normalizeRowKeyValue(record?.rowKey);
  if (!rowKey) return;
  const next = new Set(selectedRowKeys.value.map(item => normalizeRowKeyValue(item)).filter(Boolean));
  if (next.has(rowKey)) next.delete(rowKey);
  else next.add(rowKey);
  selectedRowKeys.value = Array.from(next);
}

function getSelectedRecords() {
  const selectedSet = new Set(selectedRowKeys.value.map(item => normalizeRowKeyValue(item)).filter(Boolean));
  if (!selectedSet.size) return [];
  return displayedRows.value.filter(record => selectedSet.has(normalizeRowKeyValue(record?.rowKey)));
}

function resolveContextMenuPosition(anchorX, anchorY, menuWidth, menuHeight, edgePadding = 12, gap = 8) {
  const viewportWidth = typeof window !== 'undefined' ? window.innerWidth : 0;
  const viewportHeight = typeof window !== 'undefined' ? window.innerHeight : 0;
  const safeWidth = Math.max(0, Number(menuWidth) || 0);
  const safeHeight = Math.max(0, Number(menuHeight) || 0);
  const x = viewportWidth > 0
    ? Math.max(edgePadding, Math.min(anchorX, viewportWidth - safeWidth - edgePadding))
    : anchorX;
  if (viewportHeight <= 0) {
    return { x, y: anchorY + gap };
  }

  const belowY = anchorY + gap;
  const aboveY = anchorY - safeHeight - gap;
  const maxY = viewportHeight - safeHeight - edgePadding;
  const canOpenBelow = belowY <= maxY;
  const canOpenAbove = aboveY >= edgePadding;
  let y = belowY;
  if (!canOpenBelow && canOpenAbove) {
    y = aboveY;
  } else if (!canOpenBelow) {
    y = Math.max(edgePadding, Math.min(belowY, maxY));
  }
  return { x, y };
}

async function openRowContextMenu(record, event) {
  if (!record || !event) return;
  event.preventDefault();
  event.stopPropagation();
  const recordKey = normalizeRowKeyValue(record?.rowKey);
  const selectedRecords = getSelectedRecords();
  const isBatch = selectedRecords.length > 1 && selectedRecords.some(item => normalizeRowKeyValue(item?.rowKey) === recordKey);
  if (isBatch) {
    rowContextMenu.record = selectedRecords[0] || record;
    rowContextMenu.records = selectedRecords;
    rowContextMenu.batch = true;
  } else {
    rowContextMenu.record = record;
    rowContextMenu.records = [record];
    rowContextMenu.batch = false;
  }
  rowContextMenu.groupSubmenuOpen = false;
  const anchorX = Number(event.clientX) || 0;
  const anchorY = Number(event.clientY) || 0;
  const initialPosition = resolveContextMenuPosition(anchorX, anchorY, 224, 220);
  rowContextMenu.x = initialPosition.x;
  rowContextMenu.y = initialPosition.y;
  rowContextMenu.open = true;
  await nextTick();
  if (!rowContextMenu.open || rowContextMenu.record !== record) return;
  const menuElement = rowContextMenuRef.value;
  const measuredWidth = menuElement?.offsetWidth || 224;
  const measuredHeight = menuElement?.offsetHeight || 220;
  const resolvedPosition = resolveContextMenuPosition(anchorX, anchorY, measuredWidth, measuredHeight);
  rowContextMenu.x = resolvedPosition.x;
  rowContextMenu.y = resolvedPosition.y;
}

function handleManagedRecordRowClick(record, event) {
  const target = event?.target;
  if (target?.closest?.('.ant-btn') || target?.closest?.('.ant-select') || target?.closest?.('.ant-dropdown') || target?.closest?.('.ant-popover') || target?.closest?.('.ant-tag') || target?.closest?.('.ant-switch') || target?.closest?.('.ant-checkbox') || target?.closest?.('.ant-typography-copy') || target?.closest?.('.site-title-link') || target?.closest?.('a') || target?.closest?.('input') || target?.closest?.('textarea') || target?.closest?.('button')) {
    return;
  }
  toggleRowSelected(record);
}

function getManagedRecordRowProps(record) {
  const isContextTarget = Boolean(
    rowContextMenu.open &&
    rowContextMenu.record &&
    String(rowContextMenu.record?.rowKey || '').trim() === String(record?.rowKey || '').trim()
  );
  const isSelected = isRowSelected(record);
  const classList = [];
  if (isContextTarget) classList.push('key-row-context-target');
  if (isSelected) classList.push('key-row-selected');
  return {
    class: classList.join(' '),
    onClick: event => handleManagedRecordRowClick(record, event),
    onContextmenu: event => openRowContextMenu(record, event),
  };
}

function getRowContextRecords() {
  if (rowContextMenu.batch) {
    return Array.isArray(rowContextMenu.records) ? rowContextMenu.records.filter(Boolean) : [];
  }
  return rowContextMenu.record ? [rowContextMenu.record] : [];
}

function isTargetWithinKeyManagementContextMenu(target) {
  return Boolean(
    target?.closest?.('.key-row-context-menu') ||
    target?.closest?.('.key-row-context-submenu') ||
    target?.closest?.('.key-group-context-menu') ||
    target?.closest?.('.key-group-context-submenu') ||
    target?.closest?.('.advanced-proxy-connection-context-menu')
  );
}

function isRowContextGroupActive(groupId) {
  const records = getRowContextRecords();
  if (!records.length) return false;
  return records.every(record => isRecordInGroup(record, groupId));
}

function getRowContextGroupMark(groupId) {
  const records = getRowContextRecords();
  if (!records.length) return '';
  const includedCount = records.filter(record => isRecordInGroup(record, groupId)).length;
  if (includedCount <= 0) return '';
  if (includedCount >= records.length) return '✓';
  return '—';
}

function toggleRowContextGroupMembership(groupId) {
  const records = getRowContextRecords();
  if (!records.length || !groupId) return;
  if (rowContextMenu.batch) {
    const shouldAdd = !records.every(record => isRecordInGroup(record, groupId));
    records.forEach(record => {
      const current = normalizeRecordGroupIds(record?.groupIds);
      const hasGroup = current.includes(groupId);
      if (shouldAdd && !hasGroup) {
        record.groupIds = [...current, groupId];
      } else if (!shouldAdd && hasGroup) {
        record.groupIds = current.filter(id => id !== groupId);
      }
    });
    persistRecords();
    return;
  }
  toggleRecordGroupMembership(rowContextMenu.record, groupId);
}

function handleGlobalRowContextMenuDismiss(event) {
  const target = event?.target;
  if (rowContextMenu.open) {
    if (isTargetWithinKeyManagementContextMenu(target)) return;
    closeRowContextMenu();
  }
  if (keyGroupContextMenu.open) {
    if (isTargetWithinKeyManagementContextMenu(target)) return;
    closeKeyGroupContextMenu();
  }
  if (advancedProxyConnectionContextMenu.open) {
    if (isTargetWithinKeyManagementContextMenu(target)) return;
    closeAdvancedProxyConnectionContextMenu();
  }
}

function handleGlobalContextMenuScroll(event) {
  if (!rowContextMenu.open && !keyGroupContextMenu.open && !advancedProxyConnectionContextMenu.open) return;
  if (isTargetWithinKeyManagementContextMenu(event?.target)) return;
  closeAllContextMenus();
}

function openRowContextGroupSubmenu() {
  rowContextMenu.groupSubmenuOpen = true;
}

function closeRowContextGroupSubmenu() {
  rowContextMenu.groupSubmenuOpen = false;
}

function openKeyGroupMergeSubmenu() {
  keyGroupContextMenu.mergeSubmenuOpen = true;
}

function handleGlobalRowContextEscape(event) {
  if (event?.key === 'Escape') {
    if (createKeyGroupModalOpen.value) return;
    closeAllContextMenus();
  }
}

function handleRowContextEdit() {
  const record = rowContextMenu.record;
  closeRowContextMenu();
  if (record) {
    openManualRecordModal(record);
  }
}

function handleRowContextDelete() {
  const batch = rowContextMenu.batch;
  const records = getRowContextRecords();
  const record = rowContextMenu.record;
  closeRowContextMenu();
  if (batch) {
    if (!records.length) return;
    const rowKeySet = new Set(records.map(item => normalizeRowKeyValue(item?.rowKey)).filter(Boolean));
    Modal.confirm({
      title: `确认批量删除 ${rowKeySet.size} 条记录？`,
      okText: '删除',
      cancelText: '取消',
      okButtonProps: { danger: true },
      onOk: () => {
        tableData.value = tableData.value.filter(item => !rowKeySet.has(normalizeRowKeyValue(item?.rowKey)));
        selectedRowKeys.value = selectedRowKeys.value.filter(key => !rowKeySet.has(normalizeRowKeyValue(key)));
        persistRecords();
        message.success(`已批量删除 ${rowKeySet.size} 条记录`);
      },
    });
    return;
  }
  if (!record) return;
  Modal.confirm({
    title: '确认删除这条记录？',
    okText: '删除',
    cancelText: '取消',
    okButtonProps: { danger: true },
    onOk: () => deleteRecord(record),
  });
}

async function handleRowContextAIImage() {
  const record = rowContextMenu.record;
  closeRowContextMenu();
  if (!record?.rowKey) return;

  if (isWailsRuntime && typeof OpenAIImageWindow === 'function') {
    try {
      await OpenAIImageWindow(String(record.rowKey || ''));
      return;
    } catch (error) {
      message.error(error?.message || '打开 AI 绘图窗口失败');
      return;
    }
  }

  const rowKey = encodeURIComponent(String(record.rowKey || '').trim());
  const targetUrl = `${window.location.origin}/ai-image?rowKey=${rowKey}`;
  window.open(targetUrl, '_blank', 'noopener');
}

function buildModelProbeSiteCacheRecord(record) {
  const siteUrl = normalizeSiteUrl(record?.siteUrl);
  const apiKey = normalizeApiKey(record?.apiKey);
  if (!siteUrl || !apiKey) return null;
  const siteName = String(record?.siteName || '未命名站点').trim() || '未命名站点';
  const siteCacheKey = buildSiteCacheKey({
    site_url: siteUrl,
    site_name: siteName,
    resolved_user_id: 'model-probe',
  });
  const modelsList = normalizeModels([
    record?.modelsList,
    record?.modelsText,
    record?.selectedModel,
    record?.quickTestModel,
  ]);
  const now = Date.now();
  return {
    siteCacheKey,
    siteName,
    siteUrl,
    siteType: String(record?.siteType || '').trim(),
    apiBaseUrl: siteUrl,
    accountInfo: { id: 'model-probe', access_token: apiKey },
    resolvedAccessToken: apiKey,
    resolvedUserId: 'model-probe',
    tokens: [{
      key: apiKey,
      access_token: apiKey,
      name: String(record?.tokenName || 'Probe SK').trim() || 'Probe SK',
      status: 1,
      source: 'model_probe',
      models: modelsList,
      updatedAt: now,
    }],
    customTokens: [],
    endpoint: siteUrl,
    cachedTreeNodes: [],
    lastImportSource: 'key_model_probe',
    updatedAt: now,
    lastSyncedAt: now,
  };
}

function getModelProbeComparableUserId(siteLike) {
  return String(
    siteLike?.resolvedUserId ||
    siteLike?.resolved_user_id ||
    siteLike?.accountInfo?.id ||
    siteLike?.account_info?.id ||
    ''
  ).trim();
}

function mergeModelProbeSiteCacheRecords(records) {
  const mergedMap = new Map();
  (Array.isArray(records) ? records : []).forEach(item => {
    const normalized = item && typeof item === 'object' ? { ...item } : null;
    if (!normalized) return;
    const apiKey = normalizeApiKey(normalized?.resolvedAccessToken || normalized?.accountInfo?.access_token || normalized?.tokens?.[0]?.access_token || normalized?.tokens?.[0]?.key);
    if (!apiKey) return;
    const existing = mergedMap.get(apiKey);
    const incomingModels = normalizeModels(
      Array.isArray(normalized?.tokens)
        ? normalized.tokens.flatMap(token => [token?.models, token?.model, token?.selectedModel, token?.modelsText])
        : []
    );
    if (!existing) {
      const now = Date.now();
      const primaryToken = Array.isArray(normalized.tokens) && normalized.tokens.length > 0
        ? { ...normalized.tokens[0] }
        : {
          key: apiKey,
          access_token: apiKey,
          name: 'Probe SK',
          status: 1,
          source: 'model_probe',
          updatedAt: now,
        };
      mergedMap.set(apiKey, {
        ...normalized,
        resolvedAccessToken: apiKey,
        accountInfo: {
          ...(normalized.accountInfo || {}),
          access_token: apiKey,
        },
        tokens: [{
          ...primaryToken,
          key: apiKey,
          access_token: apiKey,
          models: incomingModels,
          updatedAt: now,
        }],
        sourceRowKeys: [String(normalized?.rowKey || '').trim()].filter(Boolean),
        sourceSiteNames: [String(normalized?.siteName || '').trim()].filter(Boolean),
      });
      return;
    }
    const nextSourceRowKeys = Array.from(new Set([
      ...((Array.isArray(existing?.sourceRowKeys) ? existing.sourceRowKeys : []).map(item => String(item || '').trim()).filter(Boolean)),
      String(normalized?.rowKey || '').trim(),
    ].filter(Boolean)));
    const nextSourceSiteNames = Array.from(new Set([
      ...((Array.isArray(existing?.sourceSiteNames) ? existing.sourceSiteNames : []).map(item => String(item || '').trim()).filter(Boolean)),
      String(normalized?.siteName || '').trim(),
    ].filter(Boolean)));
    const mergedModels = Array.from(new Set([
      ...normalizeModels(existing?.tokens?.[0]?.models || []),
      ...incomingModels,
    ]));
    const now = Date.now();
    const primarySiteName = nextSourceSiteNames[0] || String(existing?.siteName || '未命名站点').trim() || '未命名站点';
    const mergedSiteName = nextSourceSiteNames.length > 1
      ? `${primarySiteName} 等${nextSourceSiteNames.length}项`
      : primarySiteName;
    mergedMap.set(apiKey, {
      ...existing,
      siteName: mergedSiteName,
      sourceRowKeys: nextSourceRowKeys,
      sourceSiteNames: nextSourceSiteNames,
      tokens: [{
        ...(existing?.tokens?.[0] || {}),
        key: apiKey,
        access_token: apiKey,
        models: mergedModels,
        updatedAt: now,
      }],
      updatedAt: now,
      lastSyncedAt: now,
      lastRefreshAt: now,
    });
  });
  return Array.from(mergedMap.values());
}

async function handleRowContextModelProbe() {
  const batch = rowContextMenu.batch;
  const records = getRowContextRecords();
  const record = rowContextMenu.record;
  closeRowContextMenu();
  if (batch) {
    if (!records.length) return;
    await handleBatchRowContextModelProbe(records);
    return;
  }
  await handleBatchRowContextModelProbe(record ? [record] : []);
}

async function handleBatchRowContextModelProbe(records) {
  const sourceRecords = Array.isArray(records) ? records.filter(Boolean) : [];
  if (!sourceRecords.length) return;
  const siteCacheRecords = mergeModelProbeSiteCacheRecords(
    sourceRecords
      .map(item => buildModelProbeSiteCacheRecord(item))
      .filter(Boolean)
  );
  if (!siteCacheRecords.length) {
    message.warning('当前选中密钥缺少站点地址或 SK，无法探测模型');
    return;
  }

  const probeMessageKey = `key-model-probe-prefetch::batch::${Date.now()}`;
  const batchLabel = siteCacheRecords.length > 1 ? `${siteCacheRecords.length} 个站点` : '1 个站点';
  message.loading({
    key: probeMessageKey,
    content: `正在初始化批量模型探测（${batchLabel}）...`,
    duration: 0,
  });

  const warmedRecords = [];
  let successCount = 0;
  let failedCount = 0;
  for (const rawRecord of siteCacheRecords) {
    let siteCacheRecord = { ...rawRecord };
    try {
      const modelResponse = await fetchModelList(
        siteCacheRecord.siteUrl,
        siteCacheRecord.resolvedAccessToken,
        { uid: getModelProbeComparableUserId(siteCacheRecord) }
      );
      const rawCandidates = modelResponse?.data || modelResponse?.models || [];
      const normalizedCandidates = normalizeModels(rawCandidates);
      if (normalizedCandidates.length > 0) {
        const now = Date.now();
        const mergedModels = Array.from(new Set([
          ...normalizeModels(siteCacheRecord?.tokens?.[0]?.models || []),
          ...normalizedCandidates,
        ]));
        siteCacheRecord = {
          ...siteCacheRecord,
          tokens: [{
            ...(siteCacheRecord?.tokens?.[0] || {}),
            key: siteCacheRecord.resolvedAccessToken,
            access_token: siteCacheRecord.resolvedAccessToken,
            models: mergedModels,
            updatedAt: now,
          }],
          updatedAt: now,
          lastSyncedAt: now,
          lastRefreshAt: now,
          lastImportSource: 'key_model_probe',
        };
        successCount += 1;
      } else {
        failedCount += 1;
      }
    } catch {
      failedCount += 1;
    }
    warmedRecords.push(siteCacheRecord);
  }

  const refreshedAt = Date.now();
  mergeExtractedSitesIntoTempCache(warmedRecords, {
    importSource: 'key_model_probe',
    refreshedAt,
  });
  mergeExtractedSitesIntoCache(warmedRecords, {
    importSource: 'key_model_probe',
    refreshedAt,
  });

  const primaryRecord = warmedRecords[0];
  const probeId = writeModelProbeContext({
    rowKey: String(sourceRecords[0]?.rowKey || '').trim(),
    siteCacheKey: primaryRecord?.siteCacheKey,
    siteCacheKeys: warmedRecords.map(item => String(item?.siteCacheKey || '').trim()).filter(Boolean),
    siteCacheRecord: primaryRecord,
    siteCacheRecords: warmedRecords,
    siteName: primaryRecord?.siteName,
    siteUrl: primaryRecord?.siteUrl,
    apiKey: primaryRecord?.resolvedAccessToken,
    suggestedGroupName: `${String(primaryRecord?.siteName || '模型探测').trim()} 可用模型`,
  });

  const summary = failedCount > 0
    ? `模型预刷新完成：成功 ${successCount}，失败 ${failedCount}，正在打开探测窗口...`
    : `模型预刷新完成：${successCount} 个站点，正在打开探测窗口...`;
  message.success({
    key: probeMessageKey,
    content: summary,
    duration: 2.2,
  });

  if (isWailsRuntime && typeof OpenModelProbeWindow === 'function') {
    try {
      await OpenModelProbeWindow(probeId || String(sourceRecords[0]?.rowKey || ''));
      return;
    } catch (error) {
      message.error(error?.message || '打开模型探测窗口失败');
      return;
    }
  }
  message.error('当前环境不支持原生模型探测窗口，请在桌面端使用该功能');
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
const safeRecordList = rows => (Array.isArray(rows) ? rows : []).filter(Boolean);
const sortManagedRecords = rows => safeRecordList(rows).sort(
  (a, b) => Number(b?.updatedAt || 0) - Number(a?.updatedAt || 0) || String(a?.siteName || '').localeCompare(String(b?.siteName || ''))
);
const columns = [
  { title: '网站', dataIndex: 'siteName', key: 'siteName', width: 142, sorter: (a, b) => String(a.siteName || '').localeCompare(String(b.siteName || '')) },
  { title: 'API Key', dataIndex: 'apiKey', key: 'apiKey', width: 188, className: 'api-key-column' },
  { title: '状态', dataIndex: 'status', key: 'status', width: 88, className: 'status-column', sorter: (a, b) => Number(a.status || 0) - Number(b.status || 0) },
  { title: '专属导出', dataIndex: 'exportActions', key: 'exportActions', width: 136, className: 'export-actions-column' },
];
const activeColumns = computed(() => (isCompactMode.value
  ? columns.filter(column => ['siteName', 'exportActions'].includes(column.dataIndex))
  : columns));
const failedSites = computed(() => allResults.value.filter(result => !Array.isArray(result?.tokens) || result.tokens.length === 0));
const failedSiteNames = computed(() => failedSites.value.map(site => site?.site_name || site?.id || '未命名站点').join('，'));
const keyGroupSiteFilterQuery = ref('');
const keyGroupSiteFilterDisplayValue = computed({
  get() {
    return keyGroupSiteFilterQuery.value;
  },
  set(value) {
    const nextValue = String(value || '');
    if (!nextValue) {
      keyGroupSiteFilterQuery.value = '';
      return;
    }
    keyGroupSiteFilterQuery.value = nextValue.startsWith(' ')
      ? nextValue
      : ` ${nextValue.trimStart()}`;
  },
});
const allSortedRows = computed(() => sortManagedRecords(tableData.value));
const normalizedKeyGroupSiteFilterQuery = computed(() => String(keyGroupSiteFilterQuery.value || '').trim().toLowerCase());
function handleKeyGroupSiteFilterFocus() {
  if (!keyGroupSiteFilterQuery.value) {
    keyGroupSiteFilterQuery.value = ' ';
  }
}

function handleKeyGroupSiteFilterBlur() {
  if (!String(keyGroupSiteFilterQuery.value || '').trim()) {
    keyGroupSiteFilterQuery.value = '';
  }
}

function toggleHideInvalidKeys() {
  hideInvalidKeys.value = !hideInvalidKeys.value;
}

const displayedRows = computed(() => {
  const filteredRows = hideInvalidKeys.value
    ? allSortedRows.value.filter(record => Number(record?.status || 0) === 1)
    : allSortedRows.value;
  const groupedRows = activeKeyGroupId.value === ALL_KEYS_GROUP_ID
    ? filteredRows
    : filteredRows.filter(record => normalizeRecordGroupIds(record?.groupIds).includes(activeKeyGroupId.value));
  const siteFilteredRows = !normalizedKeyGroupSiteFilterQuery.value
    ? groupedRows
    : groupedRows.filter(record => {
      const siteName = String(record?.siteName || '').trim().toLowerCase();
      return siteName.includes(normalizedKeyGroupSiteFilterQuery.value);
    });
  return sortManagedRecords(siteFilteredRows);
});
const keyManagementEmptyDescription = computed(() => {
  if (allSortedRows.value.length === 0) {
    return '暂无本地密钥记录，可从批量检测自动同步、剪贴板导入或手工添加。';
  }
  if (normalizedKeyGroupSiteFilterQuery.value) {
    return '当前站点筛选条件下暂无可见密钥。';
  }
  if (activeKeyGroupId.value !== ALL_KEYS_GROUP_ID) {
    return '当前分组暂无密钥。';
  }
  if (hideInvalidKeys.value) {
    return '当前筛选条件下暂无可见密钥。';
  }
  return '暂无可显示的密钥记录。';
});
const quickGroupSourceModels = computed(() => Array.from(new Set(
  allSortedRows.value
    .flatMap(record => buildMergedModelList(record))
    .map(model => String(model || '').trim())
    .filter(Boolean)
)));
const quickGroupFilters = computed(() => {
  const models = quickGroupSourceModels.value;
  const familyMap = new Map();

  models.forEach(model => {
    const category = extractQuickFilterCategory(model);
    if (!category) return;
    const version = extractQuickFilterVersion(model);
    const familyKey = resolveQuickFilterFamilyKey(category);
    const optionKey = `${category}:${version || normalizeQuickFilterName(model).toLowerCase()}`;
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
        label: buildQuickFilterOptionLabel(category, version, model),
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

  regularFamilies.sort((a, b) => a.label.localeCompare(b.label));
  if (rareOptions.length) {
    regularFamilies.push({
      key: 'mixed',
      label: 'MIXED',
      category: 'mixed',
      options: rareOptions.sort((a, b) => a.label.localeCompare(b.label)),
    });
  }
  return regularFamilies;
});
const activeQuickGroupSummary = computed(() => {
  const labels = [];
  quickGroupFilters.value.forEach(family => {
    family.options.forEach(option => {
      if (activeQuickGroupFilters.value.includes(option.key)) labels.push(option.label);
    });
  });
  if (labels.length === 0) return '';
  if (labels.length <= 3) return `已按 ${labels.join(' + ')} 快速分组`;
  return `已按 ${labels.slice(0, 3).join(' + ')} +${labels.length - 3} 项快速分组`;
});
const activeQuickGroupModelSet = computed(() => getQuickGroupModelsFromOptionKeys(activeQuickGroupFilters.value));
const quickGroupMatchedRecords = computed(() => {
  const modelSet = activeQuickGroupModelSet.value;
  if (!modelSet.size) return [];
  return allSortedRows.value
    .map(record => {
      const matchedModels = Array.from(new Set(buildMergedModelList(record).filter(model => modelSet.has(model))));
      if (!matchedModels.length) return null;
      return {
        rowKey: record.rowKey,
        siteName: String(record.siteName || '未命名站点').trim() || '未命名站点',
        tokenName: String(record.tokenName || '').trim() || '未命名 Token',
        maskedApiKey: maskApiKey(record.apiKey),
        matchedModels: matchedModels.slice(0, 3),
      };
    })
    .filter(Boolean);
});
const healthyKeyCount = computed(() => tableData.value.filter(record => record.status === 1).length);
const abnormalKeyCount = computed(() => tableData.value.filter(record => Number(record?.status || 0) !== 1).length);
const currentGroupQuickTestRecordCount = computed(() => displayedRows.value.filter(record => String(record?.quickTestStatus || '').trim()).length);
const quickTestFailedKeyCount = computed(() => displayedRows.value.filter(record => String(record?.quickTestStatus || '').trim() === 'error').length);
const quickGroupModelRefreshTargetCount = computed(() => tableData.value.filter(record => normalizeSiteUrl(record?.siteUrl) && normalizeApiKey(record?.apiKey)).length);
const quickGroupModelRefreshDisabled = computed(() => quickGroupModelRefreshRunning.value || quickGroupModelRefreshTargetCount.value === 0);
const syncSummary = computed(() => !syncMeta.value.lastBatchSyncAt ? '导入并批量检测后，会自动把获取到的 sk key 更新到本页。' : `最近一次批量同步写入 ${syncMeta.value.lastBatchSyncCount} 条记录，失败站点 ${syncMeta.value.lastBatchFailedCount} 个。`);
const consoleQueueDragGhostStyle = computed(() => ({
  width: `${Math.max(120, consoleQueueDragState.ghostWidth || 150)}px`,
  minHeight: `${Math.max(56, consoleQueueDragState.ghostHeight || 72)}px`,
  transform: `translate3d(${consoleQueueDragState.ghostX}px, ${consoleQueueDragState.ghostY}px, 0) rotate(-1.5deg)`,
}));
const consoleQueueCards = computed(() => {
  const queueProviders = getAdvancedProxyQueueProviders(
    advancedProxyConfigSnapshot.value,
    ADVANCED_PROXY_GLOBAL_QUEUE_SCOPE,
    { enabledOnly: false }
  );
  const queuedKeys = new Set();
  const queuedCards = queueProviders.map((provider, index) => {
    const providerId = getConsoleQueueProviderKey(provider, `provider-${index + 1}`);
    const rowKey = String(provider?.rowKey || provider?.id || '').trim();
    const apiKey = String(provider?.apiKey || '').trim();
    [providerId, rowKey, apiKey].filter(Boolean).forEach(value => queuedKeys.add(value));
    return {
      id: providerId,
      rowKey,
      siteName: String(provider?.name || provider?.baseUrl || `Provider ${index + 1}`).trim() || `Provider ${index + 1}`,
      modelLabel: String(provider?.model || '未设置模型').trim() || '未设置模型',
      skLabel: formatProviderSkLabel(index + 1, provider?.apiKey),
      queueOrder: index + 1,
      enabled: provider?.enabled !== false,
      inQueue: true,
    };
  });
  const pendingCards = allSortedRows.value
    .filter(record => isValidProviderQueueSourceRecord(record))
    .filter(record => {
      const rowKey = String(record?.rowKey || '').trim();
      const apiKey = String(record?.apiKey || '').trim();
      return rowKey && !queuedKeys.has(rowKey) && !queuedKeys.has(apiKey);
    })
    .map((record, index) => ({
      id: `pending-${String(record?.rowKey || index).trim() || index}`,
      rowKey: String(record?.rowKey || '').trim(),
      siteName: String(record?.siteName || record?.siteUrl || `Provider ${index + 1}`).trim() || `Provider ${index + 1}`,
      modelLabel: String(getRecordSelectedModelValue(record) || record?.quickTestModel || '未设置模型').trim() || '未设置模型',
      skLabel: formatProviderSkLabel(queuedCards.length + index + 1, record?.apiKey),
      queueOrder: 0,
      enabled: true,
      inQueue: false,
    }));
  const visibleQueuedCards = consoleQueueDragState.active && consoleQueueDragState.overId
    ? reorderConsoleQueueCardList(queuedCards, consoleQueueDragState.sourceId, consoleQueueDragState.overId, consoleQueueDragState.insertAfter)
    : queuedCards;
  return [...visibleQueuedCards, ...pendingCards].slice(0, 80);
});
const sortedAdvancedProxyActiveConnections = computed(() => {
  return [...advancedProxyActiveConnections.value].sort((left, right) => {
    const leftCompleted = isAdvancedProxyConnectionCompleted(left);
    const rightCompleted = isAdvancedProxyConnectionCompleted(right);
    if (leftCompleted !== rightCompleted) return leftCompleted ? 1 : -1;
    const leftTime = new Date(left?.startedAt || '').getTime() || 0;
    const rightTime = new Date(right?.startedAt || '').getTime() || 0;
    return rightTime - leftTime;
  });
});
const consoleDispatchModeLabel = computed(() => {
  const dispatchMode = String(advancedProxyConfigSnapshot.value?.highAvailability?.dispatchMode || 'fixed').trim();
  if (dispatchMode === 'random') return '随机调度';
  if (dispatchMode === 'ordered') return '顺序轮询';
  return '固定顺序';
});
const consoleDispatchSummaryBlocks = computed(() => {
  const queueHead = consoleQueueCards.value[0];
  const queuedCount = consoleQueueCards.value.filter(item => item.inQueue).length;
  return [
    {
      id: 'dispatch',
      items: [
        { label: '调度模式', value: consoleDispatchModeLabel.value },
        { label: '全局队列', value: `${queuedCount} 条` },
      ],
    },
    {
      id: 'runtime',
      items: [
        { label: '队列头部', value: queueHead?.inQueue ? `P${queueHead.queueOrder} ${queueHead.siteName} / ${queueHead.modelLabel}` : '空' },
        { label: '连接记录', value: `${sortedAdvancedProxyActiveConnections.value.length} 条` },
      ],
    },
  ];
});
const consoleProxyAppCards = computed(() =>
  CONSOLE_PROXY_APP_IDS.map(id => {
    const label = CONSOLE_PROXY_APP_LABELS[id] || id;
    const enabled = getConsoleAppEnabled(id);
    const pending = hasConsolePendingApp(id);
    return {
      id,
      label,
      enabled,
      pending,
      icon: DESKTOP_APP_ICONS[id],
      tooltip: pending
        ? `${label} 高级代理配置中`
        : `${label} 高级代理${enabled ? '已开启' : '未开启'}`,
    };
  }),
);
const consoleProxyMasterEnabled = computed(() => {
  if (consoleProxyMasterOptimistic.value !== null) return consoleProxyMasterOptimistic.value === true;
  return consoleProxyAppCards.value.some(app => app.enabled);
});
const consoleAntiPoisonEnabled = computed(() => {
  if (consoleAntiPoisonOptimistic.value !== null) return consoleAntiPoisonOptimistic.value === true;
  return advancedProxyConfigSnapshot.value?.antiPoison?.enabled === true;
});
const consoleProxyMasterTitle = computed(() => {
  const enabledLabels = consoleProxyAppCards.value.filter(app => app.enabled).map(app => app.label);
  return enabledLabels.length
    ? `高级代理已开启：${enabledLabels.join(' / ')}`
    : '开启四个客户端高级代理入口';
});
const consoleAntiPoisonTitle = computed(() => consoleAntiPoisonEnabled.value ? '防投毒已开启，点击关闭' : '防投毒未开启，点击开启');
const consoleDispatchLogText = computed(() => {
  return advancedProxyConsoleLogLines.value.join('\n\n');
});

function resolveElementFromVueRef(value) {
  return value?.$el || value || null;
}

function logInventoryLayoutSnapshot(reason = 'check') {
  try {
    const element = resolveElementFromVueRef(inventoryCardRef.value);
    const rect = element?.getBoundingClientRect?.();
    const body = element?.querySelector?.('.ant-card-body');
    const bodyRect = body?.getBoundingClientRect?.();
    const height = Math.round(Number(rect?.height || 0));
    const bodyHeight = Math.round(Number(bodyRect?.height || 0));
    if (height > 80 && bodyHeight > 80 && reason !== 'mounted') return;
    logClientDiagnostic('key_management.layout', JSON.stringify({
      reason,
      panel: activeInventoryPanel.value,
      cardHeight: height,
      bodyHeight,
      rows: safeRecordList(displayedRows.value).length,
      viewportHeight: typeof window !== 'undefined' ? window.innerHeight : 0,
    }));
  } catch (error) {
    logClientDiagnostic('key_management.layout.error', error?.stack || error?.message || String(error || 'unknown error'));
  }
}

watch(consoleDispatchLogText, () => {
  scrollAdvancedProxyConsoleLogToBottom();
});
watch(activeInventoryPanel, panel => {
  if (panel === 'console') scrollAdvancedProxyConsoleLogToBottom();
});
const currentVisiblePageRows = computed(() => {
  if (isCompactMode.value) return safeRecordList(displayedRows.value);
  const start = Math.max(0, (currentTablePage.value - 1) * currentTablePageSize.value);
  return safeRecordList(displayedRows.value).slice(start, start + currentTablePageSize.value);
});
const batchQuickTestDisabled = computed(() => batchQuickTestRunning.value || safeRecordList(displayedRows.value).length === 0);
const batchDeleteAbnormalDisabled = computed(() => batchQuickTestRunning.value || abnormalKeyCount.value === 0);
const batchDeleteQuickTestFailedDisabled = computed(() => batchQuickTestRunning.value || safeRecordList(displayedRows.value).length === 0);
const activeKeyGroupLabel = computed(() => {
  if (activeKeyGroupId.value === ALL_KEYS_GROUP_ID) return '全部密钥';
  return keyGroups.value.find(group => group.id === activeKeyGroupId.value)?.name || '当前分组';
});
const mergeTargetKeyGroups = computed(() => {
  const currentGroupId = String(keyGroupContextMenu.group?.id || '').trim();
  return keyGroups.value.filter(group => String(group?.id || '').trim() && String(group.id).trim() !== currentGroupId);
});
const providerQueueSourceRecords = computed(() => {
  const source = safeRecordList(displayedRows.value);
  const seen = new Set();
  return source.filter(record => {
    const rowKey = String(record?.rowKey || '').trim();
    if (!rowKey || seen.has(rowKey)) return false;
    seen.add(rowKey);
    return true;
  });
});
const currentGroupValidProviderQueueRecords = computed(() =>
  providerQueueSourceRecords.value.filter(record => isValidProviderQueueSourceRecord(record))
);
const providerQueueScopeText = computed(() =>
  normalizedKeyGroupSiteFilterQuery.value
    ? '当前筛选结果中的'
    : `${activeKeyGroupLabel.value}中的`
);
const syncCurrentGroupProviderQueueDisabled = computed(() => currentGroupValidProviderQueueRecords.value.length === 0);
const syncCurrentGroupProviderQueueTooltip = computed(() =>
  syncCurrentGroupProviderQueueDisabled.value
    ? `${normalizedKeyGroupSiteFilterQuery.value ? '当前筛选结果' : '当前列表'}没有可写入 provider 队列的状态正常密钥`
    : `将${providerQueueScopeText.value} ${currentGroupValidProviderQueueRecords.value.length} 条状态正常密钥设置为provider队列（用于本地Live高级代理）`
);
const batchQuickTestButtonTitle = computed(() => {
  if (batchQuickTestRunning.value) {
    return `批量快测进行中：已完成 ${batchQuickTestProgress.completed}/${batchQuickTestProgress.total}，并发 ${BATCH_QUICK_TEST_CONCURRENCY}，运行中 ${batchQuickTestProgress.active}`;
  }
  return `按当前列表顺序批量触发“快速测”，并发 ${BATCH_QUICK_TEST_CONCURRENCY}，只测试每条当前已选择的模型`;
});
const batchActionButtonTitle = computed(() => {
  if (batchQuickTestRunning.value) {
    return batchQuickTestButtonTitle.value;
  }
  if (quickTestFailedKeyCount.value > 0) {
    return getScopedGroupId()
      ? `批量操作：可从当前分组移除 ${quickTestFailedKeyCount.value} 条快测失败密钥`
      : `批量操作：可删除 ${quickTestFailedKeyCount.value} 条快测失败密钥`;
  }
  if (safeRecordList(displayedRows.value).length > 0 && currentGroupQuickTestRecordCount.value === 0) {
    return '批量操作：当前分组暂无快测记录，可先整组批量快测';
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
    total: safeRecordList(displayedRows.value).length,
  };
});

onMounted(() => {
  if (typeof window !== 'undefined') {
    window.addEventListener('pointerdown', handleGlobalRowContextMenuDismiss, true);
    window.addEventListener('resize', closeAllContextMenus);
    window.addEventListener('scroll', handleGlobalContextMenuScroll, true);
    window.addEventListener('keydown', handleGlobalRowContextEscape);
    window.addEventListener('contextmenu', handleGlobalRowContextMenuDismiss, true);
  }
  void (async () => {
    await hydrateLastResultsSnapshotCache();
    syncThemeState();
    refreshManualSidebarBridgeReady();
    if (!manualSidebarBridgeReady.value && typeof window !== 'undefined') {
      manualSidebarBridgeProbeTimer = window.setInterval(refreshManualSidebarBridgeReady, 250);
    }
    refreshManagedRecordsFromStorage();
    ensureDefaultPublicKeySeededOnce();
    await refreshAdvancedProxyConsoleSnapshot();
    await refreshAdvancedProxyConsoleRecords();
    await nextTick();
    logInventoryLayoutSnapshot('mounted');
    if (typeof window !== 'undefined') {
      window.addEventListener(THEME_MODE_CHANGE_EVENT, syncThemeState);
      window.addEventListener(ADVANCED_PROXY_SYNC_EVENT, refreshAdvancedProxyConsoleSnapshot);
      window.addEventListener(ADVANCED_PROXY_SYNC_EVENT, refreshAdvancedProxyConsoleRecords);
      window.addEventListener(KEY_MANAGEMENT_SYNC_EVENT, handleManagedRecordSyncEvent);
      window.addEventListener(HISTORY_SNAPSHOT_SYNC_EVENT, handleManagedRecordSyncEvent);
      window.addEventListener('storage', handleManagedRecordStorageEvent);
    }
  })();
});

onBeforeUnmount(() => {
  flushPersistRecords();
  void syncAdvancedProxyProviderSnapshotsFromKeys();
  if (typeof window !== 'undefined') {
    window.removeEventListener('pointerdown', handleGlobalRowContextMenuDismiss, true);
    window.removeEventListener('resize', closeAllContextMenus);
    window.removeEventListener('scroll', handleGlobalContextMenuScroll, true);
    window.removeEventListener('keydown', handleGlobalRowContextEscape);
    window.removeEventListener('contextmenu', handleGlobalRowContextMenuDismiss, true);
  }
  if (manualSidebarBridgeProbeTimer) {
    clearInterval(manualSidebarBridgeProbeTimer);
    manualSidebarBridgeProbeTimer = null;
  }
  stopAdvancedProxyConsolePolling();
  stopAdvancedProxyConnectionClock();
  stopConsoleQueueDragListeners();
  if (typeof window !== 'undefined') {
    window.removeEventListener(THEME_MODE_CHANGE_EVENT, syncThemeState);
    window.removeEventListener(ADVANCED_PROXY_SYNC_EVENT, refreshAdvancedProxyConsoleSnapshot);
    window.removeEventListener(ADVANCED_PROXY_SYNC_EVENT, refreshAdvancedProxyConsoleRecords);
    window.removeEventListener(KEY_MANAGEMENT_SYNC_EVENT, handleManagedRecordSyncEvent);
    window.removeEventListener(HISTORY_SNAPSHOT_SYNC_EVENT, handleManagedRecordSyncEvent);
    window.removeEventListener('storage', handleManagedRecordStorageEvent);
  }
});

watch(activeInventoryPanel, (panel) => {
  if (panel === 'console') {
    void refreshAdvancedProxyConsoleSnapshot();
    void refreshAdvancedProxyConsoleRecords();
    void refreshAdvancedProxyActiveConnections();
    startAdvancedProxyConsolePolling();
    startAdvancedProxyConnectionClock();
    return;
  }
  stopAdvancedProxyConsolePolling();
  stopAdvancedProxyConnectionClock();
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

watch(syncCurrentGroupProviderQueueDisabled, disabled => {
  if (disabled) providerQueueInlineConfirmOpen.value = false;
});

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
  keyGroups.value = loadStoredKeyGroups();
  if (activeKeyGroupId.value !== ALL_KEYS_GROUP_ID) {
    const exists = keyGroups.value.some(group => String(group?.id || '').trim() === String(activeKeyGroupId.value || '').trim());
    if (!exists) {
      activeKeyGroupId.value = ALL_KEYS_GROUP_ID;
    }
  }
  tableData.value = loadStoredRecords();
  syncMeta.value = loadStoredMeta();
  void autoRefreshKeyBalancesOnce();
}

function ensureDefaultPublicKeySeededOnce() {
  if (typeof window === 'undefined') return;
  try {
    if (localStorage.getItem(DEFAULT_PUBLIC_KEY_SEED_STORAGE_KEY)) return;
    localStorage.setItem(DEFAULT_PUBLIC_KEY_SEED_STORAGE_KEY, String(Date.now()));
    if (loadStoredRecords().length > 0 || tableData.value.length > 0) return;

    const now = Date.now();
    const record = hydrateRecordModelSelection({
      ...DEFAULT_PUBLIC_KEY_RECORD,
      modelsList: [...DEFAULT_PUBLIC_KEY_MODELS],
      modelsText: DEFAULT_PUBLIC_KEY_MODELS.join(', '),
      groupIds: [],
      quickTestStatus: '',
      quickTestLabel: '',
      quickTestModel: '',
      quickTestRemark: '',
      quickTestAt: null,
      quickTestResponseTime: '',
      quickTestTtftMs: '',
      quickTestTps: '',
      quickTestResponseContent: '',
      quickTestResolvedEndpoint: '',
      balanceLabel: '',
      balanceUpdatedAt: null,
      balanceError: '',
      balanceLoading: false,
      quickTestLoading: false,
      createdAt: now,
      updatedAt: now,
    });
    tableData.value = [record];
    persistRecords();
    flushPersistRecords();
  } catch (error) {
    console.warn('[KeyManagement] seed default public key failed:', error);
  }
}

function handleManagedRecordSyncEvent() {
  refreshManagedRecordsFromStorage();
}

function handleManagedRecordStorageEvent(event) {
  const watchedKeys = [STORAGE_KEY, MANUAL_STORAGE_KEY, META_STORAGE_KEY, KEY_GROUPS_STORAGE_KEY, LAST_RESULTS_STORAGE_KEY];
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
      quickTestResolvedEndpoint: previous?.quickTestResolvedEndpoint || '',
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
    const testResult = await executeQuickTest({ apiKey: record.apiKey, siteUrl: record.siteUrl, model, siteType: record.siteType || record.site_type || '' });
    record.quickTestStatus = testResult.status;
    record.quickTestLabel = testResult.label;
    record.quickTestModel = model;
    record.quickTestRemark = testResult.remark;
    record.quickTestAt = Date.now();
    record.quickTestResponseTime = testResult.responseTime;
    record.quickTestTtftMs = testResult.ttftMs || '';
    record.quickTestTps = testResult.tps || '';
    record.quickTestResponseContent = testResult.responseContent || '';
    record.quickTestResolvedEndpoint = String(testResult.resolvedEndpoint || '').trim();
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
    record.quickTestResolvedEndpoint = extractResolvedEndpointFromDiagnosticText(detail);
    persistRecords();
    if (!silent) {
      showQuickTestErrorDialog(detail);
      message.error(`快速测试失败：${error.message || '未知错误'}`);
    }
    return {
      status: 'error',
      label: '失败',
      model: fixedModel || getRecordSelectedModelValue(record),
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
  pushRecords(otherVisibleRows.filter(record => Number(record?.status || 0) !== 1));

  return queue;
}

function buildCurrentGroupBatchQuickTestQueue() {
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
  pushRecords(otherVisibleRows.filter(record => Number(record?.status || 0) !== 1));

  return queue;
}

function buildBatchQuickTestSummary(stats, options = {}) {
  const messagePrefix = String(options?.messagePrefix || '批量快测完成：');
  const fallbackDescription = String(options?.fallbackDescription || '已按当前页优先级完成整库快测。');
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
    message: `${messagePrefix}${summaryText}`,
    description: detailParts.join('；') || fallbackDescription,
  };
}

async function runBatchQuickTestWithQueue(queue, options = {}) {
  if (batchQuickTestRunning.value) return;
  if (!queue.length) {
    message.warning(options.emptyMessage || '当前没有可处理的密钥记录');
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
        const selectedModel = getRecordSelectedModelValue(record);

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

  const batchQuickTestNotice = buildBatchQuickTestSummary(stats, options);
  if (stats.error > 0) {
    message.warning(batchQuickTestNotice.message);
  } else {
    message.success(batchQuickTestNotice.message);
  }
}

async function runBatchQuickTest() {
  const queue = buildBatchQuickTestQueue();
  await runBatchQuickTestWithQueue(queue, {
    emptyMessage: '当前没有可处理的密钥记录',
    messagePrefix: '批量快测完成：',
    fallbackDescription: '已按当前列表顺序完成批量快测。',
  });
}

async function runCurrentGroupBatchQuickTest() {
  const queue = buildCurrentGroupBatchQuickTestQueue();
  await runBatchQuickTestWithQueue(queue, {
    emptyMessage: '当前分组没有可处理的密钥记录',
    messagePrefix: '当前分组批量快测完成：',
    fallbackDescription: '已按当前分组完成批量快测。',
  });
}

async function refreshQuickGroupModelCatalog() {
  if (quickGroupModelRefreshRunning.value) return;
  const targets = tableData.value.filter(record => normalizeSiteUrl(record?.siteUrl) && normalizeApiKey(record?.apiKey));
  if (!targets.length) {
    message.warning('当前没有可刷新模型列表的密钥');
    return;
  }

  quickGroupModelRefreshRunning.value = true;
  const stats = {
    refreshed: 0,
    failed: 0,
    skippedBusy: 0,
  };
  let cursor = 0;

  try {
    const worker = async () => {
      while (cursor < targets.length) {
        const index = cursor;
        cursor += 1;
        const record = targets[index];
        if (!record) continue;
        if (record.modelLoading) {
          stats.skippedBusy += 1;
          continue;
        }
        const ok = await loadRecordModelOptions(record, true, { silent: true });
        if (ok) stats.refreshed += 1;
        else stats.failed += 1;
      }
    };

    await Promise.allSettled(
      Array.from({ length: Math.min(QUICK_GROUP_MODEL_REFRESH_CONCURRENCY, targets.length) }, () => worker())
    );
  } finally {
    quickGroupModelRefreshRunning.value = false;
  }

  const summary = `模型列表刷新完成：成功 ${stats.refreshed} 条，失败 ${stats.failed} 条${stats.skippedBusy > 0 ? `，跳过 ${stats.skippedBusy} 条进行中记录` : ''}。快捷模型列表已同步更新。`;
  if (stats.failed > 0) {
    message.warning(summary);
  } else {
    message.success(summary);
  }
}

async function resolveQuickTestModel(record) {
  const selectedModel = getRecordSelectedModelValue(record);
  if (selectedModel) return selectedModel;
  const historyPreferred = getBatchHistoryContext(record)?.preferredModel || '';
  if (historyPreferred) {
    setRecordSelectedModelValue(record, historyPreferred);
    persistRecords();
    return historyPreferred;
  }
  const fromRecord = pickPreferredModel(record.modelsList);
  if (fromRecord) {
    setRecordSelectedModelValue(record, fromRecord);
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
  setRecordSelectedModelValue(record, preferred);
  persistRecords();
  return preferred;
}

async function executeQuickTest({ apiKey, siteUrl, model, siteType = '' }) {
  let timeoutMs = DEFAULT_TEST_TIMEOUT_MS;
  if (/^o1-|^o3-/i.test(model)) timeoutMs *= 3;
  const startedAt = Date.now();
  const response = await apiFetch('/api/check-key', {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({
      url: normalizeSiteUrl(siteUrl),
      key: apiKey,
      model,
      siteType,
      messages: buildQuickTestMessages(),
      timeoutMs,
      userAgentMappings: loadUserAgentMappings(),
      _isFirst: false,
    }),
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
  const resolvedEndpoint = String(data?.diagnostics?.resolvedEndpoint || '').trim();
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
        resolvedEndpoint,
      };
    }
    return {
      status: 'warning',
      label: '结构异常',
      remark: '接口响应成功，但未检测到有效消息内容',
      responseTime,
      responseContent,
      resolvedEndpoint,
    };
  }
  return {
    status: 'warning',
    label: '模型映射',
    remark: `平台返回模型 ${returnedModel}，请求模型为 ${model}`,
    responseTime,
    responseContent,
    resolvedEndpoint,
  };
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

    const payload = await resolveClipboardPackagePayload(text.slice('sk://'.length));
    const importedRecords = Array.isArray(payload?.records) ? payload.records : [];
    if (importedRecords.length === 0) {
      throw new Error('导入包中没有记录');
    }

    const scopedImportGroupId = getScopedGroupId();
    const scopedImportGroupName = scopedImportGroupId
      ? (keyGroups.value.find(group => group.id === scopedImportGroupId)?.name || '当前分组')
      : '';
    const merged = new Map(tableData.value.map(record => [record.rowKey, { ...record }]));
    importedRecords.forEach(rawRecord => {
      const modelsList = normalizeModels(rawRecord.modelsList || rawRecord.modelsText);
      const siteUrl = normalizeSiteUrl(rawRecord.siteUrl);
      const apiKey = normalizeApiKey(rawRecord.apiKey);
      const rowKey = rawRecord.rowKey || (rawRecord.sourceType === 'manual' ? buildManualRowKey() : buildRowKey(siteUrl, apiKey));
      const existingRecord = merged.get(rowKey) || null;
      const nextGroupIds = normalizeRecordGroupIds([
        ...normalizeRecordGroupIds(existingRecord?.groupIds),
        ...normalizeRecordGroupIds(rawRecord.groupIds),
        ...(scopedImportGroupId ? [scopedImportGroupId] : []),
      ]);
      const nextGroupSelectedModels = {
        ...normalizeGroupSelectedModels(existingRecord?.groupSelectedModels),
        ...normalizeGroupSelectedModels(rawRecord.groupSelectedModels),
      };
      const record = hydrateRecordModelSelection({
        ...existingRecord,
        ...rawRecord,
        sourceType: rawRecord.sourceType || 'auto',
        siteName: String(rawRecord.siteName || '未命名站点').trim() || '未命名站点',
        tokenName: String(rawRecord.tokenName || '').trim(),
        siteUrl,
        apiKey,
        modelsList,
        modelsText: modelsList.join(', ') || '未提供模型信息',
        selectedModel: String(rawRecord.selectedModel || '').trim(),
        groupIds: nextGroupIds,
        groupSelectedModels: nextGroupSelectedModels,
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
        quickTestResolvedEndpoint: String(rawRecord.quickTestResolvedEndpoint || existingRecord?.quickTestResolvedEndpoint || '').trim(),
        quickTestLoading: false,
      });
      record.rowKey = rowKey;
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
    message.success(
      scopedImportGroupName
        ? `已从剪贴板导入 ${importedRecords.length} 条记录，并追加到分组「${scopedImportGroupName}」`
        : `已从剪贴板导入 ${importedRecords.length} 条记录`
    );
  } catch (error) {
    console.error(error);
    message.error(`导入失败：${error.message || '未知错误'}`);
  }
}

async function copySingleImportCommand(record) {
  try {
    const smartOpenAIBaseUrl = resolveOpenAIExportBaseUrl(record, record.siteUrl);
    const normalizedRecord = {
      ...record,
      sourceType: record.sourceType || 'auto',
      rowKey: record.rowKey || (record.sourceType === 'manual' ? buildManualRowKey() : buildRowKey(record.siteUrl, record.apiKey)),
      siteName: String(record.siteName || '未命名站点').trim() || '未命名站点',
      tokenName: String(record.tokenName || '').trim(),
      siteUrl: smartOpenAIBaseUrl || normalizeSiteUrl(record.siteUrl),
      apiKey: normalizeApiKey(record.apiKey),
      modelsList: normalizeModels(record.modelsList || record.modelsText),
      modelsText: normalizeModels(record.modelsList || record.modelsText).join(', ') || '未提供模型信息',
      selectedModel: String(record.selectedModel || '').trim(),
      quickTestResponseContent: record.quickTestResponseContent || '',
      quickTestResolvedEndpoint: String(record.quickTestResolvedEndpoint || '').trim(),
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

async function resolveClipboardPackagePayload(text) {
  const encoded = String(text || '').trim();
  try {
    return await readClipboardPackagePayload(encoded);
  } catch (primaryError) {
    const fallbackEncoded = remapClipboardPackageToken(encoded);
    if (!fallbackEncoded || fallbackEncoded === encoded) {
      throw primaryError;
    }
    try {
      return await readClipboardPackagePayload(fallbackEncoded);
    } catch {
      throw primaryError;
    }
  }
}

async function readClipboardPackagePayload(text) {
  const payloadText = await decompressClipboardPackage(text);
  return JSON.parse(payloadText);
}

function remapClipboardPackageToken(value) {
  return String(value || '').replace(/[A-Za-z]/g, letter => {
    const code = letter.charCodeAt(0);
    if (code >= 65 && code <= 90) {
      return String.fromCharCode(90 - (code - 65));
    }
    if (code >= 97 && code <= 122) {
      return String.fromCharCode(122 - (code - 97));
    }
    return letter;
  });
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

function syncAdvancedProxyConfigSnapshotFromCurrentRecords(config) {
  return syncAdvancedProxyProvidersFromRecords(config, tableData.value, {
    modelResolver: record => getRecordSelectedModelValue(record),
  }).config;
}

async function syncCurrentGroupToAdvancedProxyQueue() {
  const validRecords = currentGroupValidProviderQueueRecords.value;
  if (!validRecords.length) {
    message.warning(`${normalizedKeyGroupSiteFilterQuery.value ? '当前筛选结果' : '当前列表'}暂无可写入 provider 队列的状态正常密钥`);
    return;
  }

  try {
    const savedConfig = await getAdvancedProxyConfig();
    const nextConfig = normalizeAdvancedProxyConfig(savedConfig || {});
    const providers = validRecords.map((record, index) => buildProviderFromManagedRecord(record, index + 1));
    replaceAdvancedProxyQueueProviders(nextConfig, ADVANCED_PROXY_GLOBAL_QUEUE_SCOPE, providers);
    nextConfig.claude.providers = [...(nextConfig.queues?.[ADVANCED_PROXY_GLOBAL_QUEUE_SCOPE]?.providers || [])];
    const syncedConfig = syncAdvancedProxyConfigSnapshotFromCurrentRecords(nextConfig);
    await setAdvancedProxyConfig(syncedConfig);
    advancedProxyConfigSnapshot.value = normalizeAdvancedProxyConfig(syncedConfig);
    advancedProxyFocusQueueScope.value = ADVANCED_PROXY_GLOBAL_QUEUE_SCOPE;
    advancedProxyFocusQueueToken.value = Date.now();
    showExperimentalFeatures.value = true;
    message.success(`已将${providerQueueScopeText.value} ${validRecords.length} 条状态正常密钥写入 provider 队列`);
  } catch (error) {
    message.error(error?.message || '写入全局 Provider 队列失败');
  }
}

async function appendCurrentGroupToAdvancedProxyQueue() {
  const validRecords = currentGroupValidProviderQueueRecords.value;
  if (!validRecords.length) {
    message.warning(`${normalizedKeyGroupSiteFilterQuery.value ? '当前筛选结果' : '当前列表'}暂无可写入 provider 队列的状态正常密钥`);
    return;
  }

  try {
    const savedConfig = await getAdvancedProxyConfig();
    const nextConfig = normalizeAdvancedProxyConfig(savedConfig || {});
    const providers = validRecords.map((record, index) => buildProviderFromManagedRecord(record, index + 1));
    appendAdvancedProxyQueueProviders(nextConfig, ADVANCED_PROXY_GLOBAL_QUEUE_SCOPE, providers);
    nextConfig.claude.providers = [...(nextConfig.queues?.[ADVANCED_PROXY_GLOBAL_QUEUE_SCOPE]?.providers || [])];
    const syncedConfig = syncAdvancedProxyConfigSnapshotFromCurrentRecords(nextConfig);
    await setAdvancedProxyConfig(syncedConfig);
    advancedProxyConfigSnapshot.value = normalizeAdvancedProxyConfig(syncedConfig);
    advancedProxyFocusQueueScope.value = ADVANCED_PROXY_GLOBAL_QUEUE_SCOPE;
    advancedProxyFocusQueueToken.value = Date.now();
    showExperimentalFeatures.value = true;
    message.success(`已追加${providerQueueScopeText.value} ${validRecords.length} 条状态正常密钥到 Provider 队列`);
  } catch (error) {
    message.error(error?.message || '追加全局 Provider 队列失败');
  }
}

async function clearAdvancedProxyQueue() {
  try {
    const savedConfig = await getAdvancedProxyConfig();
    const nextConfig = normalizeAdvancedProxyConfig(savedConfig || {});
    clearAdvancedProxyQueueProviders(nextConfig, ADVANCED_PROXY_GLOBAL_QUEUE_SCOPE);
    nextConfig.claude.providers = [...(nextConfig.queues?.[ADVANCED_PROXY_GLOBAL_QUEUE_SCOPE]?.providers || [])];
    const syncedConfig = syncAdvancedProxyConfigSnapshotFromCurrentRecords(nextConfig);
    await setAdvancedProxyConfig(syncedConfig);
    advancedProxyConfigSnapshot.value = normalizeAdvancedProxyConfig(syncedConfig);
    advancedProxyFocusQueueScope.value = ADVANCED_PROXY_GLOBAL_QUEUE_SCOPE;
    advancedProxyFocusQueueToken.value = Date.now();
    showExperimentalFeatures.value = true;
    message.success('已清空全部 Provider 队列');
  } catch (error) {
    message.error(error?.message || '清空全局 Provider 队列失败');
  }
}

async function handleReplaceCurrentGroupToAdvancedProxyQueue() {
  providerQueueInlineConfirmOpen.value = false;
  await syncCurrentGroupToAdvancedProxyQueue();
}

async function handleAppendCurrentGroupToAdvancedProxyQueue() {
  providerQueueInlineConfirmOpen.value = false;
  await appendCurrentGroupToAdvancedProxyQueue();
}

async function handleClearAdvancedProxyQueue() {
  providerQueueInlineConfirmOpen.value = false;
  await clearAdvancedProxyQueue();
}

function getCurrentGroupFailedQuickTestRecords() {
  return displayedRows.value.filter(record => String(record?.quickTestStatus || '').trim() === 'error');
}

function removeRecordFromGroup(record, groupId) {
  const scopedGroupId = getScopedGroupId(groupId);
  if (!record || !scopedGroupId) return false;
  const currentGroupIds = normalizeRecordGroupIds(record.groupIds);
  if (!currentGroupIds.includes(scopedGroupId)) return false;
  record.groupIds = currentGroupIds.filter(id => id !== scopedGroupId);
  const nextGroupSelectedModels = normalizeGroupSelectedModels(record.groupSelectedModels);
  delete nextGroupSelectedModels[scopedGroupId];
  record.groupSelectedModels = nextGroupSelectedModels;
  return true;
}

function deleteQuickTestFailedRecordsGlobally() {
  const targetRecords = getCurrentGroupFailedQuickTestRecords();
  const failedCount = targetRecords.length;
  if (failedCount <= 0) {
    message.warning('当前没有快测失败密钥可删除');
    return;
  }
  const failedRowKeys = new Set(targetRecords.map(record => record.rowKey).filter(Boolean));
  tableData.value = tableData.value.filter(record => !failedRowKeys.has(record?.rowKey));
  persistRecords();
  message.success(`已删除 ${failedCount} 条快测失败密钥`);
}

function removeQuickTestFailedRecordsFromCurrentGroup() {
  const scopedGroupId = getScopedGroupId();
  if (!scopedGroupId) {
    deleteQuickTestFailedRecordsGlobally();
    return;
  }
  const targetRecords = getCurrentGroupFailedQuickTestRecords();
  const failedCount = targetRecords.length;
  if (failedCount <= 0) {
    message.warning('当前分组没有快测失败密钥可移除');
    return;
  }
  const failedRowKeys = new Set(targetRecords.map(record => record.rowKey).filter(Boolean));
  let removedCount = 0;
  tableData.value.forEach(record => {
    if (!failedRowKeys.has(record?.rowKey)) return;
    if (removeRecordFromGroup(record, scopedGroupId)) removedCount += 1;
  });
  persistRecords();
  message.success(`已从分组「${getActiveKeyGroupName(scopedGroupId)}」移除 ${removedCount} 条快测失败密钥`);
}

function confirmDeleteQuickTestFailedRecords() {
  if (batchDeleteQuickTestFailedDisabled.value) return;
  if (!displayedRows.value.length) {
    message.warning(activeKeyGroupId.value === ALL_KEYS_GROUP_ID ? '当前没有可见密钥记录' : '当前分组暂无密钥');
    return;
  }
  if (currentGroupQuickTestRecordCount.value === 0) {
    Modal.confirm({
      title: '暂无记录',
      content: '当前分组下没有任何快测记录，是否批量对该分组全部快测？',
      okText: '是',
      cancelText: '否',
      onOk: runCurrentGroupBatchQuickTest,
    });
    return;
  }
  if (quickTestFailedKeyCount.value <= 0) {
    message.warning(getScopedGroupId() ? '当前分组没有快测失败密钥' : '当前没有快测失败密钥');
    return;
  }
  const scopedGroupId = getScopedGroupId();
  if (scopedGroupId) {
    Modal.confirm({
      title: '确认从当前分组移除快测失败密钥？',
      content: `将从分组「${getActiveKeyGroupName(scopedGroupId)}」移除 ${quickTestFailedKeyCount.value} 条快测失败密钥，仅影响当前分组，其他分组和“全部密钥”中的同源记录会保留。`,
      okText: '移除',
      cancelText: '取消',
      okButtonProps: { danger: true },
      onOk: removeQuickTestFailedRecordsFromCurrentGroup,
    });
    return;
  }
  const targetRecords = getCurrentGroupFailedQuickTestRecords();
  const impactedRecords = targetRecords.filter(record => normalizeRecordGroupIds(record?.groupIds).length > 0);
  const impactedGroupIds = new Set(
    impactedRecords.flatMap(record => normalizeRecordGroupIds(record?.groupIds))
  );
  const globalDeleteContent = impactedRecords.length > 0
    ? `将全局删除 ${quickTestFailedKeyCount.value} 条快测失败密钥。其中 ${impactedRecords.length} 条仍被 ${impactedGroupIds.size} 个分组引用，继续后会一并从这些分组中清除。是否继续？`
    : `将全局删除 ${quickTestFailedKeyCount.value} 条快测失败密钥，告警和可用密钥不会受影响。`;
  Modal.confirm({
    title: impactedRecords.length > 0 ? '确认全局删除快测失败密钥？' : '确认批量删除快测失败密钥？',
    content: globalDeleteContent,
    okText: '删除',
    cancelText: '取消',
    okButtonProps: { danger: true },
    onOk: deleteQuickTestFailedRecordsGlobally,
  });
}

function launchCherryStudio(record) {
  if (!record.apiKey || !record.siteUrl) {
    message.warning('配置不完整，无法导出');
    return;
  }
  const payload = {
    id: `key-${record.rowKey}`,
    baseUrl: resolveOpenAIExportBaseUrl(record, record.siteUrl) || normalizeSiteUrl(record.siteUrl),
    apiKey: record.apiKey,
    name: `${record.siteName}${record.quickTestModel ? ` (${record.quickTestModel})` : ''}`,
  };
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
  const smartOpenAIBaseUrl = resolveOpenAIExportBaseUrl(record, record.siteUrl);
  const exportSiteUrl = normalizeSiteUrl(record.siteUrl);
  const endpointBaseUrl = String(targetApp || '').toLowerCase() === 'claude'
    ? exportSiteUrl
    : (smartOpenAIBaseUrl || exportSiteUrl);
  const params = new URLSearchParams();
  params.set('resource', 'provider');
  params.set('app', targetApp);
  params.set('name', `${record.siteName}${record.quickTestModel ? ` - ${record.quickTestModel}` : ''}`);
  params.set('homepage', exportSiteUrl);
  params.set('endpoint', normalizeCCSwitchEndpoint(endpointBaseUrl, targetApp));
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
  return Boolean(normalizeSiteUrl(record?.siteUrl) && normalizeApiKey(record?.apiKey));
}

function getRecordBalanceValue(record) {
  const directLabel = normalizeBalanceLabel(record?.balanceLabel);
  if (directLabel) {
    const formatted = formatBalanceDisplay(directLabel);
    return shouldDisplayBalanceValue(record, formatted) ? formatted : '';
  }
  const remainQuota = Number(record?.remainQuota);
  if (Number.isFinite(remainQuota)) {
    const formatted = formatBalanceAmount(remainQuota);
    return shouldDisplayBalanceValue(record, formatted) ? formatted : '';
  }
  return '';
}

function shouldDisplayBalanceValue(record, value) {
  const text = String(value || '').trim();
  if (!text) return false;
  if (/^无限/.test(text)) return false;
  const amount = Number(text.replace(/USD$/i, '').replace(/^\$/, '').replace(/,/g, '').trim());
  if (Number.isFinite(amount) && amount <= 0) return false;
  return true;
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
    const site = batchContext?.accountData || {
      site_name: record?.siteName || '',
      site_url: siteUrl,
      site_type: record?.siteType || record?.site_type || '',
      tokens: [{ key: apiKey }],
    };

    const batchSnapshot = await tryFetchBatchCheckQuota(site, siteUrl);
    if (batchSnapshot) return batchSnapshot;

    throw new Error('余额接口未返回可识别字段');
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
    groupSelectedModels: normalizeGroupSelectedModels(existingRecord?.groupSelectedModels),
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
    quickTestResolvedEndpoint: existingRecord?.quickTestResolvedEndpoint || '',
    groupIds: normalizeRecordGroupIds(existingRecord?.groupIds || draft.groupIds),
    quickTestLoading: false,
  };
}
function normalizeApiKey(rawKey) {
  return String(rawKey || '').trim();
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

function extractResolvedEndpointFromDiagnosticText(text) {
  const source = String(text || '');
  if (!source.trim()) return '';
  const directMatch = source.match(/命中端点:\s*(https?:\/\/[^\s]+)/i);
  if (directMatch?.[1]) {
    return normalizeSiteUrl(directMatch[1]);
  }
  const urlMatch = source.match(/https?:\/\/[^\s]+/i);
  return normalizeSiteUrl(urlMatch?.[0] || '');
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
      groupSelectedModels: normalizeGroupSelectedModels(record.groupSelectedModels),
      quickTestStatus: record.quickTestStatus || '',
      quickTestLabel: record.quickTestLabel || '',
      quickTestModel: record.quickTestModel || '',
      quickTestRemark: record.quickTestRemark || '',
      quickTestAt: record.quickTestAt || null,
      quickTestResponseTime: record.quickTestResponseTime || '',
      quickTestTtftMs: record.quickTestTtftMs || '',
      quickTestTps: record.quickTestTps || '',
      quickTestResponseContent: record.quickTestResponseContent || '',
      quickTestResolvedEndpoint: String(record.quickTestResolvedEndpoint || '').trim(),
      groupIds: normalizeRecordGroupIds(record.groupIds),
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
      groupSelectedModels: normalizeGroupSelectedModels(record.groupSelectedModels),
      quickTestStatus: record.quickTestStatus || '',
      quickTestLabel: record.quickTestLabel || '',
      quickTestModel: record.quickTestModel || '',
      quickTestRemark: record.quickTestRemark || '',
      quickTestAt: record.quickTestAt || null,
      quickTestResponseTime: record.quickTestResponseTime || '',
      quickTestTtftMs: record.quickTestTtftMs || '',
      quickTestTps: record.quickTestTps || '',
      quickTestResponseContent: record.quickTestResponseContent || '',
      quickTestResolvedEndpoint: String(record.quickTestResolvedEndpoint || '').trim(),
      groupIds: normalizeRecordGroupIds(record.groupIds),
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
    ...Object.values(normalizeGroupSelectedModels(record?.groupSelectedModels)),
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
    groupSelectedModels: normalizeGroupSelectedModels(record?.groupSelectedModels),
    modelLoading: false,
  };
}

function getRecordRenderMeta(record) {
  const context = getBatchHistoryContext(record);
  const baseModels = Array.isArray(record?.modelsList) ? record.modelsList : normalizeModels(record?.modelsText);
  const signature = [
    record?.rowKey || '',
    getScopedGroupId(),
    Number(context?.updatedAt || 0),
    getRecordSelectedModelValue(record),
    String(record?.quickTestModel || '').trim(),
    baseModels.join('|'),
  ].join('::');

  const cached = recordRenderMetaCache.get(record?.rowKey || '');
  if (cached?.signature === signature) {
    return cached.value;
  }

  const modelsList = buildMergedModelList(record, context);
  const taskMap = new Map((Array.isArray(context?.tasks) ? context.tasks : []).map(task => [task.modelName, task]));
  const selectedModel = getRecordSelectedModelValue(record);
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

async function loadRecordModelOptions(record, force = false, options = {}) {
  if (!record?.siteUrl || !record?.apiKey) return false;
  const currentFetchKey = `${normalizeSiteUrl(record.siteUrl)}::${normalizeApiKey(record.apiKey)}`;
  if (!force && record.modelFetchKey === currentFetchKey && Array.isArray(record.modelsList) && record.modelsList.length > 0) {
    return true;
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
      ...Object.values(normalizeGroupSelectedModels(record?.groupSelectedModels)),
      record?.selectedModel || '',
      record?.quickTestModel || '',
    ]);
    if (!mergedModels.length) {
      throw new Error('没有获取到可用模型');
    }
    record.modelsList = mergedModels;
    record.modelsText = mergedModels.join(', ');
    record.modelFetchKey = currentFetchKey;
    const currentSelectedModel = getRecordSelectedModelValue(record);
    if (!currentSelectedModel || !mergedModels.includes(currentSelectedModel)) {
      setRecordSelectedModelValue(record, context?.preferredModel || pickPreferredModel(mergedModels) || mergedModels[0] || '');
    }
    persistRecords();
    return true;
  } catch (error) {
    console.error(error);
    if (!options?.silent) {
      message.error(`获取模型列表失败：${error.message || '未知错误'}`);
    }
    return false;
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
  setRecordSelectedModelValue(record, normalizedValue);
  if (normalizedValue && !normalizeModels(record.modelsList).includes(normalizedValue)) {
    record.modelsList = normalizeModels([...(record.modelsList || []), normalizedValue]);
    record.modelsText = record.modelsList.join(', ');
  }
  persistRecords();
}

async function autoRefreshKeyBalancesOnce() {
  if (keyBalanceRefreshBootstrapped.value) return;
  keyBalanceRefreshBootstrapped.value = true;

  const targets = tableData.value.filter(record => {
    if (!canRefreshBalance(record)) return false;
    if (record.balanceLoading) return false;
    return !getRecordBalanceValue(record);
  });
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
      groupIds: normalizeRecordGroupIds(record.groupIds),
      groupSelectedModels: normalizeGroupSelectedModels(record.groupSelectedModels),
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
  void syncAdvancedProxyProviderSnapshotsFromKeys();
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

async function syncAdvancedProxyProviderSnapshotsFromKeys() {
  try {
    const config = await getAdvancedProxyConfig();
    const { config: nextConfig, changed } = syncAdvancedProxyProvidersFromRecords(config, tableData.value, {
      modelResolver: record => getRecordSelectedModelValue(record),
    });
    if (!changed) return;
    await setAdvancedProxyConfig(nextConfig);
  } catch (error) {
    console.warn('[AdvancedProxy] sync provider snapshots from key records failed:', error);
  }
}

function persistMeta() {
  localStorage.setItem(META_STORAGE_KEY, JSON.stringify(syncMeta.value));
}
</script>

<style scoped>
.batch-wrapper.key-management-wrapper{min-height:calc(var(--vh,1vh) * 100);padding:0;overflow:hidden}
.batch-shell.key-management-shell{width:100%;min-height:calc(var(--vh,1vh) * 100);position:relative;isolation:isolate;overflow:hidden}
.batch-page-content.key-management-page-content{background:transparent;border-radius:24px;box-shadow:none;padding:2px;min-height:calc(var(--vh,1vh) * 100);position:relative;z-index:1;overflow:auto}
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
.key-management{width:100%;padding:0;min-height:100%;display:flex;flex:1 1 auto;flex-direction:column;gap:6px;position:relative;overflow:visible;border-radius:24px;background:transparent;box-shadow:none}
.key-management-compact{padding:12px;gap:12px;min-height:100%;background:linear-gradient(180deg,#f8fafc,#eef2ff)}
.key-management>*{position:relative;z-index:1}
.compact-sidebar-summary{display:flex;flex-direction:column;gap:10px}
.compact-sidebar-heading{display:flex;align-items:center;justify-content:space-between;gap:12px;flex-wrap:wrap}
.compact-sidebar-alert{margin:0}
.sync-card,.inventory-card{width:100%}
.inventory-card{flex:1 0 auto;display:flex;flex-direction:column;min-height:max(360px,calc(var(--vh,1vh) * 100 - 176px));overflow:hidden;border:0 !important;border-radius:24px !important;background:linear-gradient(180deg,rgba(228,233,226,.96),rgba(214,220,212,.92)) !important;box-shadow:none !important;position:relative;z-index:2}
.inventory-card :deep(.ant-card-head),.inventory-card :deep(.ant-card-body){background:transparent}
.inventory-card :deep(.ant-card-head){border-bottom-color:rgba(114,132,103,.08);min-height:54px;padding:0 14px 0 12px}
.inventory-card :deep(.ant-card-head-wrapper){position:relative;z-index:2}
.inventory-card :deep(.ant-card-head-title){padding:11px 0 9px;position:relative;z-index:3;flex:0 0 auto;overflow:visible}
.inventory-card :deep(.ant-card-extra){padding:8px 0 8px;position:relative;z-index:2;min-width:0}
.inventory-card :deep(.ant-card-body){display:flex;flex:1 1 auto;flex-direction:column;min-height:320px;padding:0 14px 6px;overflow:auto}
.inventory-card :deep(.ant-empty){margin-block:auto}
.key-group-strip{margin:0 0 1px;display:flex;align-items:flex-start;justify-content:space-between;gap:12px;min-width:0}
.key-group-tabs{display:flex;align-items:flex-start;align-content:flex-start;flex:0 1 80%;max-width:80%;flex-wrap:wrap;gap:6px 4px;min-width:0;overflow:visible;padding-bottom:0}
.key-group-tab{height:28px;padding:0 10px;border:1px solid rgba(124,142,112,.18);border-radius:10px;background:linear-gradient(180deg,rgba(255,255,255,.9),rgba(241,245,239,.9));color:#314437;display:inline-flex;align-items:center;gap:6px;flex:0 0 auto;font-size:11px;font-weight:500;line-height:1;cursor:pointer;transition:transform .18s ease,box-shadow .18s ease,border-color .18s ease,background .18s ease}
.key-group-tab:hover{transform:translateY(-1px);border-color:rgba(91,125,88,.28);box-shadow:0 10px 24px rgba(72,102,70,.1)}
.key-group-tab-active{border-color:rgba(106,144,88,.42);background:linear-gradient(180deg,#ffffff,#edf5df);box-shadow:0 12px 26px rgba(117,156,90,.16),inset 0 0 0 1px rgba(171,205,132,.25);color:#203226}
.key-group-tab-count{min-width:14px;padding:1px 5px;border-radius:999px;background:rgba(69,102,59,.1);color:inherit;font-size:10px;font-weight:600;line-height:1.05}
.key-group-tab-create{padding-inline:10px;color:#466846}
.key-group-tab-create :deep(.anticon),.key-group-tab-create svg{font-size:11px}
.key-group-site-filter{flex:1 1 220px;min-width:180px;max-width:320px;display:flex;align-items:center;gap:5px;height:26px}
.key-group-site-filter-toggle{width:18px;height:18px;min-width:18px;min-height:18px;padding:0;border:0 !important;border-radius:999px;flex:0 0 auto;display:inline-flex;align-items:center;justify-content:center;vertical-align:middle;align-self:center;background:transparent !important;box-shadow:none !important}
.key-group-site-filter-toggle :deep(.anticon),.key-group-site-filter-toggle svg{font-size:12px;line-height:1;display:block;color:rgba(15,23,42,.42)}
.key-group-site-filter-toggle:hover{transform:none !important}
.key-group-site-filter-toggle-active{background:transparent !important;box-shadow:none !important}
.key-group-site-filter-toggle-active :deep(.anticon),.key-group-site-filter-toggle-active svg{color:rgba(15,23,42,.82)}
.key-group-site-filter-input :deep(.ant-input){height:26px;border-radius:10px;background:rgba(255,255,255,.88);border-color:rgba(124,142,112,.18);box-shadow:inset 0 1px 0 rgba(255,255,255,.72);font-size:12px}
.key-group-site-filter-input :deep(.ant-input::placeholder){color:rgba(100,116,94,.72)}
.key-group-site-filter-input :deep(.ant-input-affix-wrapper){height:26px;border-radius:10px;padding:0 10px;background:rgba(255,255,255,.88);border-color:rgba(124,142,112,.18);box-shadow:inset 0 1px 0 rgba(255,255,255,.72);font-size:12px}
.key-group-site-filter-input :deep(.ant-input-affix-wrapper .ant-input){height:auto;padding:0;background:transparent;border:0;box-shadow:none}
.key-group-site-filter-input :deep(.ant-input-affix-wrapper .ant-input::placeholder){color:rgba(100,116,94,.72)}
.key-group-site-filter-input :deep(.ant-input-affix-wrapper-focused){border-color:rgba(106,144,88,.42);box-shadow:0 0 0 2px rgba(171,205,132,.18)}
.key-management-gaia .key-group-tab{border-color:rgba(122,151,125,.18);background:linear-gradient(180deg,rgba(34,40,34,.94),rgba(26,31,27,.96));color:#d8e5d4}
.key-management-gaia .key-group-tab:hover{border-color:rgba(138,176,131,.32);box-shadow:0 12px 28px rgba(0,0,0,.24)}
.key-management-gaia .key-group-tab-active{border-color:rgba(157,208,128,.36);background:linear-gradient(180deg,rgba(58,78,45,.98),rgba(40,56,34,.98));color:#f1f7ea;box-shadow:0 14px 30px rgba(0,0,0,.26),inset 0 0 0 1px rgba(186,228,149,.16)}
.key-management-gaia .key-group-tab-count{background:rgba(220,242,194,.12)}
.key-management-gaia .key-group-site-filter-input :deep(.ant-input),.key-management-gaia .key-group-site-filter-input :deep(.ant-input-affix-wrapper){background:linear-gradient(180deg,rgba(34,40,34,.94),rgba(26,31,27,.96));border-color:rgba(122,151,125,.18);color:#d8e5d4;box-shadow:none}
.key-management-gaia .key-group-site-filter-input :deep(.ant-input::placeholder),.key-management-gaia .key-group-site-filter-input :deep(.ant-input-affix-wrapper .ant-input::placeholder){color:rgba(216,229,212,.54)}
.key-management-gaia .key-group-site-filter-input :deep(.ant-input-affix-wrapper-focused){border-color:rgba(157,208,128,.36);box-shadow:0 0 0 2px rgba(186,228,149,.12)}
.key-management-gaia .key-group-site-filter-toggle :deep(.anticon),.key-management-gaia .key-group-site-filter-toggle svg{color:rgba(216,229,212,.5)}
.key-management-gaia .key-group-site-filter-toggle-active{background:transparent !important;box-shadow:none !important}
.key-management-gaia .key-group-site-filter-toggle-active :deep(.anticon),.key-management-gaia .key-group-site-filter-toggle-active svg{color:rgba(216,229,212,.92)}
.key-group-context-menu{position:fixed;z-index:1405;display:flex;flex-direction:column;gap:8px;width:208px;padding:10px;border-radius:18px;background:rgba(255,255,255,.96);box-shadow:0 18px 48px rgba(15,23,42,.24);backdrop-filter:blur(14px)}
.key-group-context-action{width:100%;padding:8px 12px;font-size:13px;line-height:1.35;border-radius:16px}
.key-group-context-submenu-trigger{justify-content:space-between}
.key-group-context-submenu-trigger-active{background:rgba(224,236,255,.9);color:#1d4ed8}
.key-group-context-submenu{position:absolute;left:calc(100% - 6px);top:54px;z-index:2;display:flex;flex-direction:column;gap:8px;width:196px;padding:10px;border-radius:16px;background:rgba(255,255,255,.98);box-shadow:0 18px 48px rgba(15,23,42,.2);backdrop-filter:blur(14px)}
.key-group-context-menu-dark{background:rgba(25,25,25,.96);box-shadow:0 18px 48px rgba(0,0,0,.4)}
.key-group-context-menu-dark .import-export-menu-item{background:rgba(255,255,255,.08);color:#f8fafc}
.key-group-context-menu-dark .import-export-menu-item:hover:not(:disabled){background:rgba(96,165,250,.2);color:#dbeafe}
.key-group-context-menu-dark .import-export-menu-item-danger{background:rgba(190,24,93,.16);color:#fda4af}
.key-group-context-menu-dark .import-export-menu-item-danger:hover:not(:disabled){background:rgba(190,24,93,.24);color:#fecdd3}
.key-group-context-menu-dark .key-group-context-submenu-trigger-active{background:rgba(186,228,149,.12);color:#e2e8f0}
.key-group-context-submenu-dark{background:rgba(25,25,25,.98);box-shadow:0 18px 48px rgba(0,0,0,.4)}
.key-group-context-submenu-dark .key-row-action-label{color:#94a3b8}
.key-group-context-submenu-dark .key-row-action-empty{color:#94a3b8}
.key-group-context-submenu-dark .key-row-group-chip{border-color:rgba(122,151,125,.18);background:rgba(255,255,255,.06);color:#e2e8f0}
.key-group-context-submenu-dark .key-row-group-chip:hover{border-color:rgba(157,208,128,.28);background:rgba(186,228,149,.08)}
.key-group-context-submenu-dark .key-row-group-chip-mark{color:#cfe8bb}
.key-quick-group-overlay{position:fixed;inset:0;z-index:1400;background:transparent}
.key-quick-group-floating-panel{position:fixed;top:18vh;left:50%;transform:translateX(-50%);width:min(960px,78vw);max-width:min(960px,78vw);padding:16px 18px 18px;border-radius:22px;background:rgba(255,255,255,.98);box-shadow:0 24px 64px rgba(15,23,42,.18),0 10px 28px rgba(15,23,42,.1)}
.key-quick-group-floating-panel-gaia{background:linear-gradient(180deg,rgba(28,35,30,.98),rgba(22,27,24,.98));box-shadow:0 28px 72px rgba(0,0,0,.42),0 12px 32px rgba(0,0,0,.26)}
.key-quick-group-composer{display:grid;gap:8px;margin-bottom:10px}
.key-quick-group-input-row{display:grid;grid-template-columns:minmax(0,1fr) auto auto;gap:8px;align-items:center}
.key-quick-group-refresh-button{width:30px;height:30px;border:1px solid rgba(96,165,250,.22);border-radius:10px;background:linear-gradient(135deg,#eff6ff,#dbeafe);color:#2563eb;display:inline-flex;align-items:center;justify-content:center;cursor:pointer;box-shadow:0 8px 18px rgba(96,165,250,.14),inset 0 0 0 1px rgba(255,255,255,.24);transition:transform .18s ease,box-shadow .18s ease,filter .18s ease}
.key-quick-group-refresh-button:hover:not(:disabled){transform:translateY(-1px);filter:saturate(1.06);box-shadow:0 12px 22px rgba(96,165,250,.18),inset 0 0 0 1px rgba(255,255,255,.28)}
.key-quick-group-refresh-button:disabled{cursor:not-allowed;opacity:.55;transform:none;filter:none;box-shadow:0 6px 14px rgba(148,163,184,.1),inset 0 0 0 1px rgba(255,255,255,.18)}
.key-quick-group-refresh-icon{font-size:14px;line-height:1}
.key-quick-group-create-button{height:30px;padding:0 14px;border-radius:10px}
.key-quick-group-summary{margin-top:-2px}
.key-quick-filter-toolbar{width:min(640px,48vw)}
.key-quick-filter-strip{display:grid;grid-template-columns:repeat(6,minmax(0,1fr));align-items:stretch}
.quick-filter-toolbar{display:flex;align-items:flex-start;flex-direction:column;gap:10px;min-height:32px;min-width:0;width:100%}
.quick-filter-strip{display:grid;grid-template-columns:repeat(6,minmax(0,1fr));gap:6px;width:100%;max-width:100%;padding:4px;border:1px solid rgba(15,23,42,.12);border-radius:12px;overflow:visible;background:rgba(255,255,255,.92);box-shadow:0 10px 24px rgba(15,23,42,.06)}
.quick-filter-strip>:not(.quick-filter-clear-trigger){min-width:0}
.quick-filter-empty-inline{color:#94a3b8;font-size:13px;padding:6px 0}
.quick-filter-family-trigger,.quick-filter-clear-trigger{width:100%;min-width:0;border:0 !important;border-radius:10px !important;box-shadow:none !important;height:34px;justify-content:center;padding:0 10px !important}
.quick-filter-family-trigger:hover,.quick-filter-clear-trigger:hover{background:rgba(22,119,255,.06) !important}
.quick-filter-clear-trigger.ant-btn[disabled],.quick-filter-clear-trigger.ant-btn[disabled]:hover{background:rgba(148,163,184,.08) !important;color:rgba(148,163,184,.9) !important}
.quick-filter-family-count{margin-left:6px;font-size:11px;opacity:.75}
.quick-filter-summary{color:#64748b;font-size:12px;line-height:1.5}
.quick-filter-family-panel{width:min(420px,56vw);max-width:420px}
.quick-filter-family-panel-title{margin-bottom:8px;font-size:13px;font-weight:700;color:#334155}
.quick-filter-option-list{display:flex;flex-wrap:wrap;gap:8px}
.quick-filter-family-select-all{border:2px solid #8b5e3c !important;color:#8b5e3c !important;background:#fffaf4 !important;box-shadow:none !important;font-weight:600}
:deep(.quick-filter-family-popover){z-index:1505 !important}
:deep(.quick-filter-family-popover .ant-popover-inner){position:relative;z-index:1505}
.key-quick-group-preview{display:grid;gap:8px;margin-top:10px;padding:10px 12px;border:1px solid rgba(15,23,42,.08);border-radius:12px;background:linear-gradient(180deg,rgba(248,250,252,.95),rgba(241,245,249,.92))}
.key-quick-group-preview-head{display:flex;align-items:center;justify-content:space-between;gap:12px}
.key-quick-group-preview-title{font-size:12px;font-weight:700;color:#334155}
.key-quick-group-preview-count{font-size:11px;color:#64748b}
.key-quick-group-preview-list{display:grid;gap:6px;max-height:180px;overflow:auto;padding-right:2px}
.key-quick-group-preview-item{display:grid;gap:3px;padding:8px 10px;border-radius:10px;background:rgba(255,255,255,.82);box-shadow:inset 0 0 0 1px rgba(148,163,184,.14)}
.key-quick-group-preview-main{display:flex;align-items:center;gap:8px;min-width:0;flex-wrap:wrap}
.key-quick-group-preview-site{font-size:12px;font-weight:700;color:#203226}
.key-quick-group-preview-token,.key-quick-group-preview-key{font-size:11px;color:#64748b}
.key-quick-group-preview-models{font-size:11px;color:#47664a;line-height:1.4;word-break:break-all}
.key-quick-group-preview-empty{font-size:11px;color:#94a3b8}
.key-management-gaia .quick-filter-strip{border-color:rgba(122,151,125,.18);background:rgba(25,31,27,.92);box-shadow:0 12px 28px rgba(0,0,0,.24)}
.key-management-gaia .quick-filter-family-trigger,.key-management-gaia .quick-filter-clear-trigger{background:rgba(255,255,255,.06) !important;color:#d8e5d4 !important}
.key-management-gaia .quick-filter-family-trigger:hover,.key-management-gaia .quick-filter-clear-trigger:hover{background:rgba(186,228,149,.1) !important}
.key-management-gaia .quick-filter-clear-trigger.ant-btn[disabled],.key-management-gaia .quick-filter-clear-trigger.ant-btn[disabled]:hover{background:rgba(255,255,255,.04) !important;color:rgba(216,229,212,.42) !important}
.key-management-gaia .key-quick-group-preview{border-color:rgba(122,151,125,.16);background:linear-gradient(180deg,rgba(28,35,30,.94),rgba(22,27,24,.96))}
.key-management-gaia .key-quick-group-refresh-button{border-color:rgba(101,129,138,.24);background:linear-gradient(180deg,rgba(37,56,66,.92),rgba(27,40,48,.96));color:#dbeafe;box-shadow:0 10px 24px rgba(0,0,0,.2),inset 0 0 0 1px rgba(181,214,225,.08)}
.key-management-gaia .key-quick-group-refresh-button:hover:not(:disabled){background:linear-gradient(180deg,rgba(44,66,78,.96),rgba(31,46,55,.98));box-shadow:0 14px 28px rgba(0,0,0,.24),inset 0 0 0 1px rgba(181,214,225,.12)}
.key-management-gaia .key-quick-group-preview-title{color:#e2e8f0}
.key-management-gaia .key-quick-group-preview-count,.key-management-gaia .key-quick-group-preview-token,.key-management-gaia .key-quick-group-preview-key{color:#9fb0a4}
.key-management-gaia .key-quick-group-preview-item{background:rgba(255,255,255,.04);box-shadow:inset 0 0 0 1px rgba(122,151,125,.14)}
.key-management-gaia .key-quick-group-preview-site{color:#f1f7ea}
.key-management-gaia .key-quick-group-preview-models{color:#c4d7c0}
.key-management-gaia .key-quick-group-preview-empty{color:#8aa08c}
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
.site-heading{display:flex;align-items:center;gap:6px;flex-wrap:nowrap;min-width:0;overflow:visible}
.site-subline{display:flex;align-items:center;gap:6px;min-width:0;flex-wrap:wrap}
.site-title-text{display:block;flex:0 0 auto;min-width:0;overflow:hidden;white-space:nowrap}
.site-title-link{display:block;flex:0 0 auto;min-width:0;padding:0;border:0;background:transparent;text-align:left;cursor:pointer;color:inherit}
.site-title-link:hover .site-title-text,.site-title-link:focus-visible .site-title-text{text-decoration:underline}
.site-title-link:disabled{cursor:default;opacity:1}
.manual-source-chip{display:inline-flex;align-items:center;height:15px;padding:0 5px;border:1px solid rgba(78,105,148,.35);border-radius:4px;background:rgba(83,115,164,.09);color:#4f6792;font-size:9px;font-weight:700;line-height:1;white-space:nowrap}
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
.site-balance-value{display:flex;align-items:baseline;gap:6px;color:#7a6041;font-size:12px;font-weight:500;line-height:1.2;white-space:nowrap;max-width:100%}
.site-balance-value-empty{color:#94a3b8}
.site-balance-label{color:#6b7280}
.site-balance-text{overflow:hidden;text-overflow:ellipsis;font-size:12px;font-weight:600;color:#8a6841}
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
.api-combined-cell{position:relative;display:flex;flex-direction:column;gap:2px;min-width:0;width:100%}
.api-model-row{margin-top:8px;min-width:0;width:calc(100% + 45px);max-width:calc(100% + 45px)}
.api-combined-cell .cell-copy-text{max-width:176px}
.api-combined-cell .api-endpoint-text{max-width:176px}
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
.inline-export-actions{display:flex;align-items:center;gap:6px;flex-wrap:nowrap;min-width:0}
.key-row-context-menu{position:fixed;z-index:1200;display:flex;flex-direction:column;gap:10px;width:224px;padding:10px;border-radius:18px;background:rgba(255,255,255,.96);box-shadow:0 18px 48px rgba(15,23,42,.24);backdrop-filter:blur(14px)}
.key-row-context-submenu{position:absolute;left:calc(100% - 6px);top:118px;z-index:2;display:flex;flex-direction:column;gap:8px;width:196px;padding:10px;border-radius:16px;background:rgba(255,255,255,.98);box-shadow:0 18px 48px rgba(15,23,42,.2);backdrop-filter:blur(14px)}
.key-management :deep(.compact-key-table .ant-table-tbody > tr.key-row-context-target > td){background:rgba(15,23,42,.085) !important;transition:background .16s ease}
.key-management :deep(.compact-key-table .ant-table-tbody > tr.key-row-context-target:hover > td){background:rgba(15,23,42,.11) !important}
.key-management :deep(.compact-key-table .ant-table-tbody > tr.key-row-selected > td){background:rgba(15,23,42,.085) !important;transition:background .16s ease}
.key-management :deep(.compact-key-table .ant-table-tbody > tr.key-row-selected:hover > td){background:rgba(15,23,42,.11) !important}
.key-row-context-menu-dark{background:rgba(25,25,25,.96);box-shadow:0 18px 48px rgba(0,0,0,.4)}
.key-row-context-submenu-dark{background:rgba(25,25,25,.98);box-shadow:0 18px 48px rgba(0,0,0,.4)}
.key-row-context-menu-dark .import-export-menu-item{background:rgba(255,255,255,.08);color:#f8fafc}
.key-row-context-menu-dark .import-export-menu-item:hover:not(:disabled){background:rgba(96,165,250,.2);color:#dbeafe}
.key-row-context-menu-dark .import-export-menu-item-danger{background:rgba(190,24,93,.16);color:#fda4af}
.key-row-context-menu-dark .import-export-menu-item-danger:hover:not(:disabled){background:rgba(190,24,93,.24);color:#fecdd3}
.key-row-context-action{width:100%;padding:8px 12px;font-size:13px;line-height:1.35;border-radius:16px}
.key-row-context-submenu-trigger{display:flex;align-items:center;justify-content:space-between}
.key-row-context-submenu-trigger-active{background:#edf5df;color:#203226}
.key-row-context-submenu-arrow{font-size:16px;line-height:1;opacity:.72}
.key-row-group-heading{display:flex;align-items:center;justify-content:space-between;gap:8px;padding:0 4px}
.key-row-group-create-button{width:24px;height:24px;border:0;border-radius:999px;background:rgba(69,102,59,.08);color:#45663b;display:inline-flex;align-items:center;justify-content:center;cursor:pointer}
.key-row-group-create-button:hover{background:rgba(69,102,59,.14)}
.key-row-group-list{display:flex;flex-direction:column;gap:6px;max-height:160px;overflow:auto;padding-right:2px}
.key-row-group-chip{border:1px solid rgba(124,142,112,.16);border-radius:12px;background:rgba(248,250,252,.78);color:#203226;display:flex;align-items:center;gap:8px;padding:7px 10px;text-align:left;cursor:pointer;transition:border-color .18s ease,background .18s ease}
.key-row-group-chip:hover{border-color:rgba(103,141,91,.28);background:rgba(240,246,236,.95)}
.key-row-group-chip-active{border-color:rgba(117,156,90,.36);background:rgba(230,242,219,.96)}
.key-row-group-chip-mark{width:14px;display:inline-flex;justify-content:center;color:#3e6c3f;font-size:12px;font-weight:700;line-height:1}
.key-row-group-chip-name{font-size:12px;line-height:1.35;overflow:hidden;text-overflow:ellipsis;white-space:nowrap}
.key-row-action-empty{padding:0 4px;color:#64748b;font-size:11px;line-height:1.35}
.key-row-action-info{display:flex;flex-direction:column;gap:2px;padding:2px 4px}
.key-row-action-label{font-size:10px;line-height:1.2;color:#64748b}
.key-row-action-value{font-size:11px;line-height:1.35;color:#0f172a;word-break:break-word}
.key-row-context-menu-dark .key-row-action-label{color:#94a3b8}
.key-row-context-menu-dark .key-row-action-value{color:#e2e8f0}
.key-row-context-menu-dark .key-row-context-submenu-trigger-active{background:rgba(186,228,149,.12);color:#e2e8f0}
.key-row-context-submenu-dark .key-row-action-label{color:#94a3b8}
.key-row-context-submenu-dark .key-row-action-empty{color:#94a3b8}
.key-row-context-menu-dark .key-row-group-create-button{background:rgba(220,242,194,.08);color:#d8e5d4}
.key-row-context-menu-dark .key-row-group-create-button:hover{background:rgba(220,242,194,.14)}
.key-row-context-submenu-dark .key-row-group-create-button{background:rgba(220,242,194,.08);color:#d8e5d4}
.key-row-context-submenu-dark .key-row-group-create-button:hover{background:rgba(220,242,194,.14)}
.key-row-context-submenu-dark .key-row-group-chip{border-color:rgba(122,151,125,.18);background:rgba(255,255,255,.06);color:#e2e8f0}
.key-row-context-submenu-dark .key-row-group-chip:hover{border-color:rgba(157,208,128,.28);background:rgba(186,228,149,.08)}
.key-row-context-submenu-dark .key-row-group-chip-active{border-color:rgba(157,208,128,.34);background:rgba(186,228,149,.12)}
.key-row-context-submenu-dark .key-row-group-chip-mark{color:#cfe8bb}
.inventory-icon-button{width:34px;height:34px;border:0;border-radius:12px;display:inline-flex;align-items:center;justify-content:center;cursor:pointer;transition:transform .18s ease, box-shadow .18s ease, filter .18s ease, opacity .18s ease;background:linear-gradient(135deg,#f8fafc,#e2e8f0);box-shadow:inset 0 0 0 1px rgba(148,163,184,.28);flex:0 0 auto;color:#0f172a}
.inventory-icon-button:hover:not(:disabled){transform:translateY(-1px) scale(1.06);filter:saturate(1.08)}
.inventory-icon-button:disabled{cursor:not-allowed;opacity:.45;transform:none;filter:none;box-shadow:inset 0 0 0 1px rgba(148,163,184,.18)}
.inventory-icon-button :deep(.anticon),.inventory-icon-button svg{font-size:16px;line-height:1}
.inventory-popover-trigger{display:inline-flex;flex:0 0 auto}
.inventory-batch-quick-test-button{width:34px;height:34px;padding:0;border:0;border-radius:12px;background:linear-gradient(135deg,#476847,#6f8f55);box-shadow:0 10px 24px rgba(87,118,76,.18);display:inline-flex;align-items:center;justify-content:center;color:#fff}
.inventory-batch-quick-test-button:disabled{opacity:.55}
.inventory-batch-quick-test-button :deep(.anticon),.inventory-batch-quick-test-button svg{font-size:16px;line-height:1}
.inventory-icon-button-provider-queue{background:linear-gradient(135deg,#fff8e7,#f6d57d);color:#8a5a12;box-shadow:0 10px 24px rgba(234,179,8,.18),inset 0 0 0 1px rgba(234,179,8,.2)}
.inventory-icon-button-provider-queue .provider-queue-icon{display:block;transition:transform .26s ease,filter .26s ease}
.inventory-icon-button-provider-queue:hover:not(:disabled) .provider-queue-icon{transform:rotate(24deg) scale(1.06);filter:saturate(1.12)}
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
:global(.key-group-create-tooltip .ant-tooltip-inner){font-size:11px;line-height:1.2;white-space:nowrap;max-width:none}
:global(.key-management-import-popover .ant-popover-inner){max-width:calc(100vw - 24px)}
:global(.key-management-import-popover .ant-popover-inner-content){padding:8px}
:global(.provider-queue-inline-popover .ant-popover-inner){border-radius:12px}
:global(.provider-queue-inline-popover .ant-popover-inner-content){padding:10px}
.provider-queue-inline-actions{display:flex;flex-direction:column;gap:8px;min-width:248px}
.provider-queue-inline-action-button{justify-content:center;height:32px;border-radius:10px;white-space:nowrap}
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
.compact-key-table :deep(.ant-table-tbody > tr > td.api-key-column){overflow:visible;position:relative;z-index:4}
.compact-key-table :deep(.ant-table-thead > tr > th.status-column),
.compact-key-table :deep(.ant-table-tbody > tr > td.status-column){position:relative;z-index:1}
.compact-key-table :deep(.ant-table-tbody > tr > td.status-column){padding-left:4px}
.compact-key-table :deep(.ant-table-tbody > tr > td.status-column .ant-tag){margin-inline-end:0}
.compact-key-table :deep(.ant-table-tbody > tr > td:first-child){overflow:visible;position:relative;z-index:3}
.desktop-config-modal{display:flex;flex-direction:column;gap:16px}
.desktop-config-hero{display:grid;grid-template-columns:minmax(0,1fr) auto;align-items:center;gap:0;margin-bottom:4px;padding:16px 18px;border-radius:22px;border:1px solid #8ec5ff;background:linear-gradient(180deg,#dcebfb,#d8eafc);box-shadow:inset 0 1px 0 rgba(255,255,255,.55)}
.desktop-config-alert{min-width:0;display:flex;align-items:center;gap:16px;padding-right:18px}
.desktop-config-alert-icon{width:48px;height:48px;flex:0 0 48px;border-radius:999px;border:4px solid #2473ea;color:#2473ea;display:inline-flex;align-items:center;justify-content:center;font-size:31px;font-weight:500;line-height:1;font-family:Georgia,'Times New Roman',serif;background:rgba(255,255,255,.32)}
.desktop-config-alert-copy{min-width:0;display:grid;gap:6px}
.desktop-config-alert-title{color:#1f2937;font-size:18px;line-height:1.2;white-space:nowrap;overflow:hidden;text-overflow:ellipsis}
.desktop-config-alert-desc{color:#24313f;font-size:13px;line-height:1.4}
.desktop-config-hero-actions{display:flex;align-items:center;justify-content:flex-end;flex:0 0 auto}
.desktop-config-hero-actions :deep(.ant-btn){height:42px;padding:0 18px;border-radius:16px;font-size:15px}
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
.key-management .inventory-card{width:100%;margin:0;flex:1 1 auto;border:0 !important;border-radius:24px !important;background:linear-gradient(180deg,rgba(228,233,226,.96),rgba(214,220,212,.92)) !important;box-shadow:none !important;backdrop-filter:none !important}
.inventory-panel-toolbar{display:flex;align-items:center;justify-content:space-between;gap:12px;min-height:54px;padding:0 0 0 0;border-bottom:1px solid rgba(114,132,103,.08);position:relative;z-index:10}
.inventory-card-title-row{display:flex;align-items:center;gap:14px;min-width:0;flex:0 0 auto;flex-wrap:wrap;position:relative;z-index:11;pointer-events:auto}
.inventory-panel-switcher{display:inline-flex;align-items:center;min-width:0}
.inventory-panel-tabs{height:38px;display:inline-flex;align-items:center;gap:6px;min-width:0;position:relative;z-index:5;pointer-events:auto;padding:3px;border:1px solid rgba(124,142,112,.2);border-radius:16px;background:linear-gradient(180deg,rgba(255,255,255,.62),rgba(238,244,235,.46));box-shadow:inset 0 1px 0 rgba(255,255,255,.72),0 8px 18px rgba(72,102,70,.08)}
.inventory-panel-tab{height:30px;padding:0 10px;border:0;border-radius:12px;background:transparent;color:#818b7a;font:700 12px/1.1 Georgia,'Times New Roman',serif;white-space:nowrap;cursor:pointer;position:relative;z-index:6;display:inline-flex;align-items:center;justify-content:center;gap:6px;transition:color .18s ease,background .18s ease;pointer-events:auto}
.inventory-panel-tab:first-child{padding-left:10px}
.inventory-panel-tab-icon{width:18px;height:18px;display:block}
.inventory-panel-tab-label{display:inline-block;font-size:11px;font-weight:800;line-height:1;letter-spacing:.04em;text-transform:uppercase}
.inventory-panel-tab-icon-key{transform:translateY(1px)}
.inventory-panel-tab-icon-console{transform:translateY(1px)}
.inventory-panel-tab::after{content:'';position:absolute;left:0;right:0;bottom:0;height:2px;border-radius:999px;background:transparent;opacity:0;transition:background .18s ease,opacity .18s ease}
.inventory-panel-tab:hover{color:#3d563f}
.inventory-panel-tab-active{color:#2d432f;background:linear-gradient(180deg,rgba(255,255,255,.58),rgba(255,255,255,.12))}
.inventory-panel-tab-active::after{background:linear-gradient(90deg,rgba(75,108,62,.82),rgba(150,185,92,.54));opacity:1}
.inventory-panel-tab-divider{width:1px;height:18px;background:rgba(104,124,94,.18);display:inline-flex;flex:0 0 auto}
.inventory-panel-actions{display:flex;align-items:center;justify-content:flex-end;min-width:0;position:relative;z-index:10}
.inventory-local-panel{display:flex;flex:1 1 auto;min-height:0;flex-direction:column}
.inventory-console-panel{flex:1 1 auto;min-height:360px;padding:18px 0 12px;display:flex;flex-direction:column;gap:14px}
.console-dispatch-top-grid{display:grid;grid-template-columns:minmax(0,65fr) minmax(300px,35fr);gap:14px;align-items:stretch}
.console-queue-section,.console-dispatch-section,.console-connections-section{min-width:0;display:flex;flex-direction:column;gap:10px}
.console-section-head{display:flex;align-items:flex-end;justify-content:space-between;gap:16px}
.console-section-head h4{margin:0;font:800 16px/1.1 Georgia,'Times New Roman',serif;color:#233923}
.console-section-head p{margin:5px 0 0;color:#60725f;font-size:12px;line-height:1.35}
.console-section-count{border-radius:999px;padding:4px 10px;background:rgba(73,103,62,.1);color:#314a31;font-size:12px;font-weight:700}
.console-provider-grid{height:408px;display:grid;grid-template-columns:repeat(4,minmax(0,1fr));grid-auto-rows:minmax(72px,max-content);align-content:start;gap:8px;overflow:auto;padding-right:2px}
.console-provider-grid{scrollbar-width:none;-ms-overflow-style:none}
.console-provider-grid::-webkit-scrollbar{width:0;height:0}
.console-provider-card{position:relative;width:100%;min-width:0;min-height:72px;padding:8px 9px;border-radius:10px;background:linear-gradient(180deg,rgba(255,255,255,.94),rgba(248,250,246,.92));border:1px solid rgba(90,117,79,.15);box-shadow:inset 0 1px 0 rgba(255,255,255,.7);display:grid;align-content:start;gap:4px;overflow:visible;text-align:left;cursor:pointer;appearance:none;-webkit-appearance:none;transition:border-color .18s ease,box-shadow .18s ease,transform .18s ease}
.console-provider-card:hover{border-color:rgba(88,125,66,.24);box-shadow:0 12px 24px rgba(74,104,58,.08);transform:translateY(-1px)}
.console-provider-card:focus-visible{outline:2px solid rgba(107,146,88,.55);outline-offset:2px}
.console-provider-card-primary{background:linear-gradient(180deg,rgba(252,255,249,.98),rgba(242,248,236,.96));border-color:rgba(75,128,50,.34);box-shadow:0 0 0 1px rgba(102,168,68,.12),0 0 0 4px rgba(147,210,109,.12),0 14px 28px rgba(74,104,58,.12)}
.console-provider-card-pending{background:rgba(255,255,255,.42);border-style:dashed;box-shadow:none;opacity:.84}
.console-provider-card-draggable{cursor:pointer}
.console-provider-card-dragging{opacity:.34;transform:scale(.97)}
.console-provider-card-drop-before{box-shadow:inset 3px 0 0 rgba(64,145,72,.88),0 12px 24px rgba(74,104,58,.1)}
.console-provider-card-drop-after{box-shadow:inset -3px 0 0 rgba(64,145,72,.88),0 12px 24px rgba(74,104,58,.1)}
.console-provider-drag-ghost{position:fixed;left:0;top:0;z-index:9999;pointer-events:none;padding:8px 9px;border-radius:10px;background:linear-gradient(180deg,rgba(252,255,249,.86),rgba(242,248,236,.78));border:1px solid rgba(75,128,50,.34);box-shadow:0 18px 36px rgba(45,70,38,.22),0 0 0 4px rgba(147,210,109,.14);display:grid;align-content:start;gap:4px;overflow:hidden;text-align:left;opacity:.86;backdrop-filter:blur(8px)}
.console-provider-card-top{position:relative;display:flex;align-items:center;justify-content:space-between;gap:5px;flex-wrap:nowrap}
.console-provider-card-top strong{min-width:0;flex:1 1 auto;overflow:hidden;text-overflow:ellipsis;white-space:nowrap;font-size:8.4px;font-weight:700;line-height:1.2;color:#22311c}
.console-provider-drag-handle{position:absolute;left:-2px;top:50%;width:7px;height:9px;display:grid;grid-template-columns:repeat(2,2px);grid-auto-rows:2px;gap:1px;align-content:center;justify-content:center;flex:0 0 auto;border:0;background:transparent;padding:0;opacity:.58;cursor:grab;touch-action:none;transform:translate(-100%,-50%)}
.console-provider-drag-handle-ghost{position:static;transform:none}
.console-provider-drag-handle:active{cursor:grabbing;opacity:.9}
.console-provider-drag-handle i{width:2px;height:2px;border-radius:50%;background:rgba(70,92,62,.55)}
.console-provider-order{min-width:22px;height:16px;padding:0 5px;border-radius:999px;display:inline-flex;align-items:center;justify-content:center;background:rgba(60,103,39,.12);color:#2c4a1f;font-size:7px;font-weight:700;flex:0 0 auto}
.console-provider-model{min-width:0;color:#5f6e5a;font-size:7px;line-height:1.25;word-break:break-word;overflow:hidden;text-overflow:ellipsis;white-space:nowrap}
.console-provider-meta{min-height:14px;display:flex;align-items:center;gap:4px;flex-wrap:wrap}
.console-provider-chip{max-width:100%;min-height:14px;padding:0 5px;border-radius:999px;background:rgba(79,108,62,.08);color:#355029;font-size:6.3px;font-weight:600;display:inline-flex;align-items:center;overflow:hidden;text-overflow:ellipsis;white-space:nowrap}
.console-provider-chip-muted{background:rgba(128,119,102,.1);color:#7a705f}
.console-empty-panel{min-height:160px;border-radius:20px;background:rgba(255,255,255,.28);display:flex;align-items:center;justify-content:center;color:#70806d;font-size:13px}
.console-dispatch-control-rack{display:grid;grid-template-columns:auto minmax(0,1fr);gap:10px;align-items:stretch}
.console-dispatch-summary{display:grid;grid-template-columns:repeat(2,minmax(0,1fr));gap:8px}
.console-dispatch-summary-top{min-width:0;order:2}
.console-dispatch-summary-block{min-width:0;min-height:38px;padding:5px 8px;border-radius:10px;border:1px solid rgba(90,117,79,.12);background:rgba(255,255,255,.34);display:grid;grid-template-columns:repeat(2,minmax(0,1fr));gap:8px;align-content:center;box-shadow:inset 0 1px 0 rgba(255,255,255,.46)}
.console-dispatch-summary-line{min-width:0;display:grid;grid-template-columns:auto minmax(0,1fr);align-items:baseline;gap:6px}
.console-dispatch-summary-line span{color:#6a7865;font-size:9px;font-weight:700;line-height:1.1;white-space:nowrap}
.console-dispatch-summary-line strong{min-width:0;color:#263b2a;font-size:11px;line-height:1.18;overflow:hidden;text-overflow:ellipsis;white-space:nowrap}
.console-dispatch-control-panel{order:1;min-height:38px;padding:5px 8px;border-radius:10px;border:1px solid rgba(90,117,79,.12);background:rgba(255,255,255,.34);display:flex;align-items:center;justify-content:flex-start;gap:7px;box-shadow:inset 0 1px 0 rgba(255,255,255,.46)}
.console-dispatch-icon-button,.console-dispatch-app-button{position:relative;isolation:isolate;overflow:hidden;width:28px;height:28px;border-radius:8px;border:1px solid rgba(90,117,79,.16);background:rgba(255,255,255,.56);color:#354d33;display:inline-flex;align-items:center;justify-content:center;cursor:pointer;appearance:none;-webkit-appearance:none;transition:border-color .18s ease,background .18s ease,box-shadow .18s ease,transform .18s ease}
.console-dispatch-icon-button:hover,.console-dispatch-app-button:hover{border-color:rgba(88,125,66,.3);background:rgba(249,252,245,.9);transform:translateY(-1px)}
.console-dispatch-icon-button:focus-visible,.console-dispatch-app-button:focus-visible{outline:2px solid rgba(107,146,88,.55);outline-offset:2px}
.console-dispatch-icon-button-active,.console-dispatch-app-button-active{border-color:rgba(75,128,50,.36);background:linear-gradient(180deg,rgba(248,255,241,.98),rgba(232,244,220,.92));box-shadow:0 0 0 3px rgba(147,210,109,.13)}
.console-dispatch-icon-button>*,.console-dispatch-app-button>*{position:relative;z-index:1}
.console-dispatch-control-pending{pointer-events:none}
.console-dispatch-icon-button.console-dispatch-control-pending::before,.console-dispatch-app-button.console-dispatch-control-pending::before{content:"";position:absolute;inset:0;border-radius:inherit;padding:2px;background:conic-gradient(from 0deg,transparent 0deg 42deg,rgba(74,130,56,.95) 72deg 118deg,transparent 150deg 208deg,rgba(238,122,86,.95) 238deg 288deg,transparent 318deg 360deg);animation:console-control-border-orbit .82s linear infinite;-webkit-mask:linear-gradient(#000 0 0) content-box,linear-gradient(#000 0 0);-webkit-mask-composite:xor;mask-composite:exclude;z-index:2;pointer-events:none}
.console-dispatch-icon-button.console-dispatch-control-pending::after,.console-dispatch-app-button.console-dispatch-control-pending::after{content:"";position:absolute;inset:3px;border-radius:6px;background:rgba(255,255,255,.18);z-index:0;pointer-events:none}
.console-dispatch-master-switch.console-dispatch-control-pending{position:relative;overflow:hidden}
.console-dispatch-master-switch.console-dispatch-control-pending::before{content:"";position:absolute;inset:0;border-radius:999px;padding:2px;background:conic-gradient(from 0deg,transparent 0deg 42deg,rgba(74,130,56,.95) 72deg 118deg,transparent 150deg 208deg,rgba(238,122,86,.95) 238deg 288deg,transparent 318deg 360deg);animation:console-control-border-orbit .82s linear infinite;-webkit-mask:linear-gradient(#000 0 0) content-box,linear-gradient(#000 0 0);-webkit-mask-composite:xor;mask-composite:exclude;z-index:2;pointer-events:none}
@keyframes console-control-border-orbit{to{transform:rotate(360deg)}}
.console-dispatch-anti-poison-button{font-size:16px}
.console-dispatch-app-buttons{display:flex;align-items:center;gap:5px}
.console-dispatch-app-button img{width:17px;height:17px;object-fit:contain;display:block}
.console-dispatch-log-panel{flex:1 1 auto;min-height:408px;height:408px;border-radius:10px;border:1px solid rgba(90,117,79,.16);background:rgba(255,255,255,.5);box-shadow:inset 0 1px 0 rgba(255,255,255,.46);overflow:auto}
.console-dispatch-log-view{min-height:100%;margin:0;padding:10px 12px;color:#263b2a;font:11px/1.38 ui-monospace,SFMono-Regular,Menlo,Consolas,monospace;white-space:pre-wrap;word-break:break-word}
.console-connections-panel{min-height:260px}
.console-connection-table{width:100%;height:300px;border-radius:4px;border:1px solid rgba(92,101,96,.34);background:rgba(255,255,255,.58);overflow:auto}
.console-connection-row{width:100%;display:grid;grid-template-columns:164px 78px 104px 92px minmax(130px,1fr) minmax(130px,1fr) minmax(120px,.9fr) minmax(180px,1.4fr);gap:0;align-items:center;padding:0;border:0;border-bottom:1px solid rgba(92,101,96,.16);background:transparent;text-align:left;color:#263b2a}
.console-connection-row:last-child{border-bottom:0}
.console-connection-head{height:34px;background:rgba(239,246,226,.64);color:#334634;font-size:12px;font-weight:800}
.console-connection-row > span{min-width:0;height:100%;padding:0 10px;display:flex;align-items:center;border-right:1px solid rgba(92,101,96,.13);overflow:hidden;text-overflow:ellipsis;white-space:nowrap}
.console-connection-row > span:last-child{border-right:0}
.console-connection-item{cursor:pointer;font-size:12px;transition:background .18s ease,color .18s ease}
.console-connection-item:hover{background:rgba(245,250,238,.72)}
.console-connection-item-selected{background:rgba(226,240,210,.82)}
.console-connection-item strong{display:block;color:#22311c;font-size:12px;line-height:1.25}
.console-connection-item small{display:block;color:#71806a;font-size:10px;line-height:1.25}
.console-connection-status-cell{gap:7px}
.console-connection-status-cell small{min-width:0;overflow:hidden;text-overflow:ellipsis;white-space:nowrap}
.console-connection-status-cell small{font-size:11px;color:#5f6f59}
.console-connection-status-cell-failed small{white-space:nowrap;line-height:1.15;color:#8d2f36;font-weight:800}
.console-connection-status-dot{width:10px;height:10px;border-radius:999px;display:inline-flex;flex:0 0 auto;background:#7a8a73;box-shadow:0 0 0 3px rgba(122,138,115,.12)}
.console-connection-status-active{background:#5d7f42;box-shadow:0 0 0 3px rgba(93,127,66,.14)}
.console-connection-status-completed{background:#26a269;box-shadow:0 0 0 3px rgba(38,162,105,.14)}
.console-connection-status-failed{background:#d84f57;box-shadow:0 0 0 3px rgba(216,79,87,.15)}
.console-connection-status-waiting{background:#b07c2b;box-shadow:0 0 0 3px rgba(176,124,43,.16)}
.console-connection-status-probe{background:#4f7c94;box-shadow:0 0 0 3px rgba(79,124,148,.16)}
.console-connection-empty-row{height:264px;display:flex;align-items:center;justify-content:center;color:#70806d;font-size:13px}
.key-management .inventory-card :deep(.ant-card-head){background:linear-gradient(180deg,rgba(228,233,226,.96),rgba(221,227,218,.94)) !important}
.key-management .inventory-card :deep(.ant-card-body){background:transparent}
.key-management .inventory-card :deep(.ant-card-head){border-bottom-color:rgba(114,132,103,.08)}
.key-management-gaia .inventory-card,.key-management-wrapper-gaia .inventory-card{background:linear-gradient(180deg,rgba(10,18,22,.96),rgba(8,14,18,.92)) !important;box-shadow:none !important}
.key-management-gaia .inventory-card :deep(.ant-card-head),.key-management-wrapper-gaia .inventory-card :deep(.ant-card-head){background:linear-gradient(180deg,rgba(14,24,29,.98),rgba(10,18,22,.96)) !important;border-bottom-color:rgba(101,129,138,.16)}
.key-management-gaia .inventory-panel-tabs{border-color:rgba(122,151,125,.22);background:linear-gradient(180deg,rgba(34,40,34,.78),rgba(25,31,27,.62));box-shadow:inset 0 1px 0 rgba(220,242,194,.08),0 10px 24px rgba(0,0,0,.22)}
.key-management-gaia .inventory-panel-tab{color:#879a8d}
.key-management-gaia .inventory-panel-tab:hover{color:#e8f3ef}
.key-management-gaia .inventory-panel-tab-active{color:#e8f3ef;background:linear-gradient(180deg,rgba(180,214,225,.08),rgba(180,214,225,0));text-shadow:none}
.key-management-gaia .inventory-panel-tab-active::after{background:linear-gradient(90deg,rgba(186,228,149,.72),rgba(105,154,145,.52))}
.key-management-gaia .inventory-panel-tab-divider{background:rgba(122,151,125,.22)}
.key-management-gaia .console-section-head h4{color:#e8f3ef}
.key-management-gaia .console-section-head p{color:#9fb3ad}
.key-management-gaia .console-provider-card{background:linear-gradient(180deg,rgba(18,31,36,.92),rgba(11,21,26,.86));border-color:rgba(101,129,138,.18);box-shadow:inset 0 1px 0 rgba(180,214,225,.05)}
.key-management-gaia .console-provider-card:hover{border-color:rgba(156,203,134,.28);box-shadow:0 12px 24px rgba(0,0,0,.22),inset 0 1px 0 rgba(180,214,225,.06)}
.key-management-gaia .console-provider-card-primary{border-color:rgba(156,203,134,.34);box-shadow:0 0 0 4px rgba(91,134,86,.12),inset 0 1px 0 rgba(180,214,225,.06)}
.key-management-gaia .console-provider-card-pending{background:rgba(8,14,18,.24);border-color:rgba(101,129,138,.22);box-shadow:none}
.key-management-gaia .console-provider-card-drop-before{box-shadow:inset 3px 0 0 rgba(116,184,104,.9),0 12px 24px rgba(0,0,0,.22),inset 0 1px 0 rgba(180,214,225,.06)}
.key-management-gaia .console-provider-card-drop-after{box-shadow:inset -3px 0 0 rgba(116,184,104,.9),0 12px 24px rgba(0,0,0,.22),inset 0 1px 0 rgba(180,214,225,.06)}
.key-management-gaia .console-provider-drag-ghost{background:linear-gradient(180deg,rgba(18,31,36,.84),rgba(11,21,26,.76));border-color:rgba(156,203,134,.34);box-shadow:0 18px 36px rgba(0,0,0,.34),0 0 0 4px rgba(91,134,86,.14)}
.key-management-gaia .console-provider-card-top strong{color:#e8f3ef}
.key-management-gaia .console-provider-drag-handle i{background:rgba(190,218,184,.62)}
.key-management-gaia .console-provider-model{color:#aebfba}
.key-management-gaia .console-provider-order,.key-management-gaia .console-section-count,.key-management-gaia .console-provider-chip{background:rgba(180,214,225,.08);color:#dcece8}
.key-management-gaia .console-empty-panel{background:rgba(8,14,18,.28);box-shadow:inset 0 1px 0 rgba(180,214,225,.05);color:#9fb3ad}
.key-management-gaia .console-dispatch-summary-block,.key-management-gaia .console-dispatch-control-panel{border-color:rgba(101,129,138,.18);background:rgba(8,14,18,.28);box-shadow:inset 0 1px 0 rgba(180,214,225,.05)}
.key-management-gaia .console-dispatch-summary-line span{color:#9fb3ad}
.key-management-gaia .console-dispatch-summary-line strong{color:#e8f3ef}
.key-management-gaia .console-dispatch-icon-button,.key-management-gaia .console-dispatch-app-button{border-color:rgba(101,129,138,.2);background:rgba(12,22,27,.72);color:#dcece8}
.key-management-gaia .console-dispatch-icon-button:hover,.key-management-gaia .console-dispatch-app-button:hover{border-color:rgba(156,203,134,.3);background:rgba(20,34,39,.86)}
.key-management-gaia .console-dispatch-icon-button-active,.key-management-gaia .console-dispatch-app-button-active{border-color:rgba(156,203,134,.4);background:linear-gradient(180deg,rgba(41,63,44,.92),rgba(22,42,38,.88));box-shadow:0 0 0 3px rgba(91,134,86,.16)}
.key-management-gaia .console-dispatch-icon-button.console-dispatch-control-pending::after,.key-management-gaia .console-dispatch-app-button.console-dispatch-control-pending::after{background:rgba(8,18,18,.22)}
.key-management-gaia .console-dispatch-log-panel{border-color:rgba(101,129,138,.2);background:rgba(8,14,18,.34);box-shadow:inset 0 1px 0 rgba(180,214,225,.05)}
.key-management-gaia .console-dispatch-log-view{color:#dcece8}
.key-management-gaia .console-connection-table{border-color:rgba(101,129,138,.18);background:rgba(8,14,18,.28)}
.key-management-gaia .console-connection-row{border-bottom-color:rgba(101,129,138,.14);color:#dcece8}
.key-management-gaia .console-connection-row > span{border-right-color:rgba(101,129,138,.14)}
.key-management-gaia .console-connection-head{background:rgba(180,214,225,.06);color:#9fb3ad}
.key-management-gaia .console-connection-item:hover{background:rgba(180,214,225,.06)}
.key-management-gaia .console-connection-item-selected{background:rgba(105,154,145,.14)}
.key-management-gaia .console-connection-item strong{color:#e8f3ef}
.key-management-gaia .console-connection-item small{color:#9fb3ad}
.key-management-gaia .console-connection-status-cell small{color:#9fb3ad}
.key-management-gaia .console-connection-status-active{background:#9cc680;box-shadow:0 0 0 3px rgba(156,198,128,.14)}
.key-management-gaia .console-connection-status-completed{background:#48c78e;box-shadow:0 0 0 3px rgba(72,199,142,.14)}
.key-management-gaia .console-connection-status-failed{background:#ff7b86;box-shadow:0 0 0 3px rgba(255,123,134,.16)}
.key-management-gaia .console-connection-status-waiting{background:#d09b56;box-shadow:0 0 0 3px rgba(208,155,86,.16)}
.key-management-gaia .console-connection-status-probe{background:#75b4c2;box-shadow:0 0 0 3px rgba(117,180,194,.16)}
.key-management-gaia .console-connection-empty-row{color:#9fb3ad}
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
:deep(body.dark-mode) .key-management .key-group-site-filter-input :deep(.ant-input),:deep(body.dark-mode) .key-management .key-group-site-filter-input :deep(.ant-input-affix-wrapper){background:linear-gradient(180deg,rgba(31,42,33,.94),rgba(20,29,23,.96));border-color:rgba(160,189,144,.18);color:#edf5e6;box-shadow:none}
:deep(body.dark-mode) .key-management .key-group-site-filter-input :deep(.ant-input::placeholder),:deep(body.dark-mode) .key-management .key-group-site-filter-input :deep(.ant-input-affix-wrapper .ant-input::placeholder){color:rgba(184,200,178,.56)}
:deep(body.dark-mode) .key-management .key-group-site-filter-input :deep(.ant-input-affix-wrapper-focused){border-color:rgba(160,189,144,.32);box-shadow:0 0 0 2px rgba(160,189,144,.12)}
:deep(body.dark-mode) .key-management .key-group-site-filter-toggle :deep(.anticon),:deep(body.dark-mode) .key-management .key-group-site-filter-toggle svg{color:rgba(237,245,230,.5)}
:deep(body.dark-mode) .key-management .key-group-site-filter-toggle-active{background:transparent !important;box-shadow:none !important}
:deep(body.dark-mode) .key-management .key-group-site-filter-toggle-active :deep(.anticon),:deep(body.dark-mode) .key-management .key-group-site-filter-toggle-active svg{color:rgba(237,245,230,.92)}
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
.key-management-compact .key-group-strip{align-items:stretch;flex-wrap:wrap}
.key-management-compact .key-group-tabs{flex-basis:100%;max-width:100%}
.key-management-compact .key-group-site-filter{min-width:0;max-width:100%;flex-basis:100%}
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
@media (max-width:900px){.key-management-page-container{padding:8px 8px 0 !important}.desktop-config-hero{grid-template-columns:minmax(0,1fr) auto;padding:14px 16px}.desktop-config-alert{gap:12px;padding-right:12px}.desktop-config-alert-icon{width:42px;height:42px;flex-basis:42px;font-size:27px;border-width:3px}.desktop-config-alert-title{font-size:16px}.desktop-config-alert-desc{font-size:12px}.desktop-config-hero-actions{justify-content:flex-end}.desktop-config-hero-actions :deep(.ant-btn){height:40px;padding:0 16px;border-radius:15px;font-size:14px}.desktop-config-layout{grid-template-columns:1fr}.desktop-app-grid{grid-template-columns:repeat(4,minmax(0,1fr));overflow:auto}.config-grid{grid-template-columns:1fr}}
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
.key-management-gaia :deep(.compact-key-table .ant-table-tbody > tr.key-row-context-target > td){background:rgba(186,228,149,.14) !important}
.key-management-gaia :deep(.compact-key-table .ant-table-tbody > tr.key-row-context-target:hover > td){background:rgba(186,228,149,.18) !important}
.key-management-gaia :deep(.compact-key-table .ant-table-tbody > tr.key-row-selected > td){background:rgba(186,228,149,.14) !important}
.key-management-gaia :deep(.compact-key-table .ant-table-tbody > tr.key-row-selected:hover > td){background:rgba(186,228,149,.18) !important}
.key-management-gaia .sync-title-text{color:#e7f1ef}
.key-management-gaia .sync-meta,.key-management-gaia .subtle-text{color:#a9bcbd}
.key-management-gaia .sync-card :deep(.ant-alert){border-color:rgba(101,129,138,.16);background:rgba(8,14,18,.34)}
.key-management-gaia .sync-panel-trigger-button{border-color:rgba(101,129,138,.2);background:rgba(255,255,255,.05);color:#dce8e7}
.key-management-gaia .sync-panel-trigger-button:hover:not(:disabled){background:rgba(88,116,126,.18);border-color:rgba(122,155,166,.3);color:#f4faf8;box-shadow:0 10px 24px rgba(0,0,0,.24)}
.key-management-gaia.key-management-compact{background:linear-gradient(180deg,#0a1116,#111c22)}
@media (max-width:700px){.console-dispatch-control-rack{grid-template-columns:1fr}.console-dispatch-summary-top{order:1}.console-dispatch-control-panel{order:2;justify-content:flex-start;flex-wrap:wrap}}
@media (max-width:520px){.console-dispatch-top-grid{grid-template-columns:1fr}.console-provider-grid{grid-template-columns:repeat(2,minmax(0,1fr));gap:10px}.console-dispatch-summary{grid-template-columns:1fr}.console-dispatch-summary-block{grid-template-columns:1fr}.console-connection-table{overflow:auto}.console-connection-row{min-width:860px}}
@keyframes sync-trigger-orbit{from{transform:rotate(0deg)}to{transform:rotate(360deg)}}
@keyframes sync-trigger-pulse{0%{transform:scale(.98);filter:saturate(1)}100%{transform:scale(1.05);filter:saturate(1.16)}}
</style>



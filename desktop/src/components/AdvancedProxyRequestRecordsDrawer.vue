<template>
  <a-drawer
    :open="open"
    :width="drawerWidth"
    placement="right"
    title="请求记录"
    :class="['advanced-proxy-records-drawer', { 'advanced-proxy-records-drawer-dark': isDarkMode }]"
    @close="handleClose"
  >
    <div class="request-records-scroll-shell">
      <div class="request-records-shell" :class="{ 'request-records-shell-dark': isDarkMode }">
        <div class="request-records-mode-tabs" role="tablist" aria-label="会话和统计">
          <button
            type="button"
            class="request-records-mode-tab"
            :class="{ 'is-active': activePanel === 'sessions' }"
            @click="setActivePanel('sessions')"
          >
            <ProfileOutlined />
            <span>会话</span>
          </button>
          <button
            type="button"
            class="request-records-mode-tab"
            :class="{ 'is-active': activePanel === 'records' }"
            @click="setActivePanel('records')"
          >
            <BarChartOutlined />
            <span>统计</span>
          </button>
          <button
            type="button"
            class="request-records-mode-tab"
            :class="{ 'is-active': activePanel === 'mcp' }"
            @click="setActivePanel('mcp')"
          >
            <InboxOutlined />
            <span>MCP</span>
          </button>
          <button
            type="button"
            class="request-records-mode-tab"
            :class="{ 'is-active': activePanel === 'skills' }"
            @click="setActivePanel('skills')"
          >
            <FireOutlined />
            <span>Skill</span>
          </button>
        </div>

        <template v-if="activePanel === 'sessions'">
          <section v-if="!terminalBridgeAvailable" class="request-records-empty">
            当前环境无法读取终端会话。
          </section>

          <section v-else class="terminal-sessions-board">
            <aside class="terminal-session-list-pane">
              <div class="terminal-provider-tabs" role="tablist" aria-label="终端记录集">
                <a-tooltip
                  v-for="provider in terminalProviderItems"
                  :key="provider.id"
                  :title="provider.total > 0 ? `${provider.label} · ${provider.total} 条` : provider.label"
                >
                  <button
                    type="button"
                    class="terminal-provider-tab"
                    :class="{ 'is-active': terminalProviderId === provider.id }"
                    :aria-label="provider.label"
                    @click="switchTerminalProvider(provider.id)"
                  >
                    <img :src="getTerminalProviderIcon(provider.id)" :alt="provider.label" />
                    <small v-if="provider.total > 0">{{ provider.total }}</small>
                  </button>
                </a-tooltip>
              </div>

              <div class="terminal-session-list-controls">
                <span class="request-records-toolbar-pill" :class="{ 'is-loading': sessionsLoading }">
                  <span class="request-records-toolbar-dot"></span>
                  <span>{{ sessionsLoading ? '扫描中' : `${terminalSessionTotal} 条` }}</span>
                </span>
                <a-button
                  size="small"
                  class="request-records-action-button request-records-action-button-refresh terminal-session-refresh-button"
                  :loading="sessionsLoading"
                  @click="refreshTerminalSessions"
                >
                  <ReloadOutlined />
                  刷新
                </a-button>
              </div>

              <a-spin :spinning="sessionsLoading">
                <div class="terminal-session-list">
                  <article
                    v-for="session in terminalSessions"
                    :key="getTerminalSessionKey(session)"
                    class="terminal-session-item"
                    :class="{ 'is-active': selectedTerminalSessionKey === getTerminalSessionKey(session) }"
                    role="button"
                    tabindex="0"
                    @click="selectTerminalSession(session)"
                    @keydown.enter.prevent="selectTerminalSession(session)"
                  >
                    <div class="terminal-session-main">
                      <strong>{{ formatTerminalSessionTitle(session) }}</strong>
                      <span>{{ session.summary || session.projectDir || session.sessionId }}</span>
                      <small>{{ formatTerminalSessionTime(session) }} · {{ compactTerminalSessionPath(session.projectDir || session.sourcePath) }}</small>
                    </div>
                    <a-tooltip :title="session.resumeCommand ? `打开终端：${session.resumeCommand}` : '该会话没有可恢复命令'">
                      <button
                        type="button"
                        class="terminal-session-open-button"
                        :disabled="!session.resumeCommand || launchingSessionKey === getTerminalSessionKey(session)"
                        @click.stop="launchSessionTerminal(session)"
                      >
                        <LoadingOutlined v-if="launchingSessionKey === getTerminalSessionKey(session)" />
                        <CodeOutlined v-else />
                      </button>
                    </a-tooltip>
                  </article>
                  <div v-if="!terminalSessions.length && !sessionsLoading" class="request-records-empty terminal-sessions-empty">
                    暂无会话记录
                  </div>
                </div>
              </a-spin>

              <div v-if="terminalSessionTotal > TERMINAL_SESSION_PAGE_SIZE" class="request-records-pagination terminal-sessions-pagination">
                <a-pagination
                  size="small"
                  simple
                  :current="terminalSessionPage"
                  :page-size="TERMINAL_SESSION_PAGE_SIZE"
                  :total="terminalSessionTotal"
                  @change="handleTerminalSessionPageChange"
                />
              </div>
            </aside>

            <section class="terminal-session-chat-pane">
              <template v-if="selectedTerminalSession">
                <header class="terminal-session-chat-head">
                  <div>
                    <strong>{{ formatTerminalSessionTitle(selectedTerminalSession) }}</strong>
                    <span>{{ compactTerminalSessionPath(selectedTerminalSession.projectDir || selectedTerminalSession.sourcePath) }}</span>
                  </div>
                  <small>{{ terminalSessionMessages.length }} 条消息</small>
                </header>
                <a-spin :spinning="terminalSessionMessagesLoading">
                  <div v-if="terminalSessionMessages.length" class="terminal-session-message-list">
                    <article
                      v-for="(item, index) in terminalSessionMessages"
                      :key="`${selectedTerminalSessionKey}::${index}`"
                      class="terminal-session-message"
                      :class="`is-${getTerminalMessageRoleClass(item.role)}`"
                    >
                      <div class="terminal-session-message-meta">
                        <span>{{ formatTerminalMessageRole(item.role) }}</span>
                        <small>{{ formatTerminalMessageTime(item.ts) }}</small>
                      </div>
                      <p :class="{ 'is-collapsed': isTerminalMessageCollapsed(item, index) }">{{ item.content }}</p>
                      <button
                        v-if="isTerminalMessageCollapsible(item)"
                        type="button"
                        class="terminal-session-message-toggle"
                        @click="toggleTerminalMessageExpanded(index)"
                      >
                        {{ isTerminalMessageExpanded(index) ? '收起' : '展开' }}
                      </button>
                    </article>
                  </div>
                  <div v-else-if="!terminalSessionMessagesLoading" class="request-records-empty terminal-session-chat-empty">
                    该会话没有可展示的聊天记录
                  </div>
                </a-spin>
              </template>
              <div v-else class="request-records-empty terminal-session-chat-empty">
                选择左侧会话查看聊天记录
              </div>
            </section>
          </section>
        </template>

        <template v-else-if="activePanel === 'mcp' || activePanel === 'skills'">
          <section class="request-records-toolbar mcp-skill-toolbar">
            <div class="request-records-toolbar-meta">
              <span class="request-records-toolbar-pill" :class="{ 'is-loading': mcpSkillLoading }">
                <span class="request-records-toolbar-dot"></span>
                <span>{{ mcpSkillCountLabel }}</span>
              </span>
              <span v-if="mcpSkillConfigPath" class="request-records-toolbar-pill request-records-toolbar-pill-muted">
                {{ compactTerminalSessionPath(mcpSkillConfigPath) }}
              </span>
            </div>
            <div class="mcp-skill-app-tabs" role="tablist" aria-label="MCP Skill app browser">
              <a-tooltip v-for="app in managedAppItems" :key="app.id" :title="app.label">
                <button
                  type="button"
                  class="mcp-skill-app-tab"
                  :class="{ 'is-active': selectedManagedAppId === app.id }"
                  :aria-label="app.label"
                  @click="setSelectedManagedApp(app.id)"
                >
                  <img :src="getManagedAppIcon(app.id)" :alt="app.label" />
                </button>
              </a-tooltip>
            </div>
            <div class="request-records-toolbar-actions">
              <a-button size="small" class="request-records-action-button request-records-action-button-refresh" :loading="mcpSkillLoading" @click="refreshMCPSkillConfig">
                <ReloadOutlined />
                {{ tr('刷新') }}
              </a-button>
              <a-button size="small" class="request-records-action-button request-records-action-button-refresh" :loading="mcpSkillSaving" :disabled="!mcpSkillBridgeAvailable" @click="saveMCPSkillConfig">
                {{ tr('保存') }}
              </a-button>
            </div>
          </section>

          <section v-if="!mcpSkillBridgeAvailable" class="request-records-empty">
            当前环境无法读取 MCP / Skill 配置。
          </section>

          <section v-else class="mcp-skill-board">
            <template v-if="activePanel === 'mcp'">
              <article v-if="!managedMCPServers.length && !mcpSkillLoading" class="request-records-empty mcp-skill-empty">
                暂无 MCP 配置。会自动扫描 Claude、Codex、Gemini、OpenCode、OpenClaw 的常见配置文件。
              </article>
              <article v-if="managedMCPServers.length && !visibleManagedMCPServers.length && !mcpSkillLoading" class="request-records-empty mcp-skill-empty">
                当前类目下暂无已启用 MCP。
              </article>
              <article v-for="server in visibleManagedMCPServers" :key="server.id" class="mcp-skill-card">
                <div class="mcp-skill-card-main">
                  <strong>{{ server.name || server.id }}</strong>
                  <span>{{ formatMCPServerSummary(server) }}</span>
                  <small>{{ server.source || 'managed' }}</small>
                </div>
                <div class="mcp-skill-card-actions">
                  <span class="mcp-skill-state-pill is-on">{{ selectedManagedAppLabel }}</span>
                  <a-button size="small" class="mcp-skill-card-button mcp-skill-card-button-import" @click="applyManagedMCPToSelectedApp(server.id)">
                    <CheckCircleFilled />
                    应用
                  </a-button>
                  <a-button size="small" class="mcp-skill-card-button mcp-skill-card-button-disable" @click="disableManagedMCPForSelectedApp(server.id)">
                    <StopOutlined />
                    禁用
                  </a-button>
                  <a-button size="small" danger class="mcp-skill-card-button" @click="removeManagedMCPFromSelectedApp(server.id)">
                    <DeleteOutlined />
                    删除
                  </a-button>
                </div>
              </article>
            </template>

            <template v-else>
              <article v-if="!managedSkills.length && !mcpSkillLoading" class="request-records-empty mcp-skill-empty">
                暂无 Skill。会扫描 ~/.agents/skills、~/.codex/skills、~/.claude/skills 等常见目录。
              </article>
              <article v-if="managedSkills.length && !visibleManagedSkills.length && !mcpSkillLoading" class="request-records-empty mcp-skill-empty">
                当前类目下暂无已启用 Skill。
              </article>
              <article v-for="skill in visibleManagedSkills" :key="skill.id" class="mcp-skill-card">
                <div class="mcp-skill-card-main">
                  <strong>{{ skill.name || skill.id }}</strong>
                  <span>{{ skill.description || '未填写描述' }}</span>
                  <small>{{ skill.directory || skill.source || 'managed' }}</small>
                </div>
                <div class="mcp-skill-card-actions">
                  <span class="mcp-skill-state-pill is-on">{{ selectedManagedAppLabel }}</span>
                  <a-button size="small" class="mcp-skill-card-button mcp-skill-card-button-import" @click="applyManagedSkillToSelectedApp(skill.id)">
                    <CheckCircleFilled />
                    应用
                  </a-button>
                  <a-button size="small" class="mcp-skill-card-button mcp-skill-card-button-disable" @click="disableManagedSkillForSelectedApp(skill.id)">
                    <StopOutlined />
                    禁用
                  </a-button>
                  <a-button size="small" danger class="mcp-skill-card-button" @click="removeManagedSkillFromSelectedApp(skill.id)">
                    <DeleteOutlined />
                    删除
                  </a-button>
                </div>
              </article>
            </template>
          </section>
        </template>

        <template v-else>
        <header class="request-records-toolbar">
          <div class="request-records-toolbar-meta">
            <span class="request-records-toolbar-pill" :class="{ 'is-loading': loading }">
              <span class="request-records-toolbar-dot"></span>
              <span>{{ loading ? '同步中' : `缓存 ${records.length} 条` }}</span>
            </span>
            <span
              v-for="item in statusSummaryItems"
              :key="item.id"
              class="request-records-toolbar-pill request-records-toolbar-pill-muted"
            >
              {{ item.label }} {{ item.count }}
            </span>
          </div>

          <div class="request-records-toolbar-actions">
            <a-button
              size="small"
              class="request-records-action-button request-records-action-button-refresh"
              :loading="loading"
              @click="refreshRecords"
            >
              <ReloadOutlined />
              刷新
            </a-button>
            <a-button
              size="small"
              class="request-records-action-button request-records-action-button-clear"
              danger
              ghost
              :disabled="records.length === 0"
              @click="handleClear"
            >
              <DeleteOutlined />
              清空
            </a-button>
          </div>
        </header>

        <section class="request-records-activity-card">
          <header class="request-records-activity-head">
            <div class="request-records-activity-tabs" role="tablist" aria-label="activity sections">
              <button
                v-for="item in activitySectionTabs"
                :key="item.id"
                type="button"
                class="request-records-activity-tab"
                :class="{ 'is-active': activityDashboardTab === item.id }"
                @click="activityDashboardTab = item.id"
              >
                {{ item.label }}
              </button>
            </div>
            <div class="request-records-activity-range" aria-label="activity range">
              <button
                v-for="item in dashboardRangeTabs"
                :key="item.id"
                type="button"
                class="request-records-activity-range-button"
                :class="{ 'is-active': dashboardRangeValue === item.id }"
                @click="setDashboardRange(item.id)"
              >
                {{ item.label }}
              </button>
            </div>
          </header>

          <template v-if="activityDashboardTab === 'token'">
            <div class="request-records-activity-title">Token 趋势</div>
            <div class="request-records-token-dashboard">
              <div class="request-records-token-chart">
                <div class="request-records-token-legend">
                  <span><i class="is-window"></i> 时段 Token 用量</span>
                  <span><i class="is-total"></i> 累计 Token 用量</span>
                  <span class="request-records-token-source">{{ tokenTrend.sourceLabel }}</span>
                </div>
                <div class="request-records-token-plot">
                  <svg class="request-records-token-svg" viewBox="0 0 640 220" preserveAspectRatio="none" aria-hidden="true">
                    <path v-for="line in tokenTrend.gridLines" :key="line" class="request-records-token-grid-line" :d="`M0 ${line} H640`" />
                    <path v-for="line in tokenTrend.verticalLines" :key="`v-${line}`" class="request-records-token-grid-line" :d="`M${line} 0 V220`" />
                    <path class="request-records-token-line" :d="tokenTrend.linePath" />
                  </svg>
                  <div
                    v-for="bar in tokenTrend.bars"
                    :key="bar.key"
                    class="request-records-token-bar"
                    :class="{ 'is-empty': bar.height <= 0 }"
                    :style="{ left: `${bar.left}%`, height: `${bar.height}%` }"
                    :title="bar.title"
                    @mouseenter="showTokenTooltip(bar, $event)"
                    @mousemove="moveTokenTooltip($event)"
                    @mouseleave="hideTokenTooltip"
                  ></div>
                  <div
                    v-if="tokenTooltip.visible"
                    class="request-records-chart-tooltip"
                    :style="{ left: `${tokenTooltip.x}px`, top: `${tokenTooltip.y}px` }"
                  >
                    <strong>{{ tokenTooltip.title }}</strong>
                    <span>{{ tokenTooltip.period }}</span>
                    <span>{{ tokenTooltip.cumulative }}</span>
                  </div>
                </div>
                <div class="request-records-token-axis">
                  <span v-for="label in tokenTrend.labels" :key="label.key">{{ label.label }}</span>
                </div>
              </div>

              <div class="request-records-token-side">
                <div class="request-records-token-donut" :style="tokenDonutStyle">
                  <div class="request-records-token-donut-hole">
                    <strong>{{ tokenTrend.totalLabel }}</strong>
                    <span>Total</span>
                  </div>
                </div>
                <div class="request-records-token-breakdown">
                  <span><i class="is-input"></i> 输入 <strong>{{ tokenTrend.inputPercent }}%</strong></span>
                  <span><i class="is-output"></i> 输出 <strong>{{ tokenTrend.outputPercent }}%</strong></span>
                  <span><i class="is-reasoning"></i> 推理 <strong>{{ tokenTrend.reasoningPercent }}%</strong></span>
                </div>
                <div v-if="tokenTrend.sourceItems.length > 0" class="request-records-token-sources">
                  <span v-for="source in tokenTrend.sourceItems" :key="source.label">
                    {{ source.label }} <strong>{{ source.value }}</strong>
                  </span>
                </div>
                <small v-if="!tokenTrend.hasData" class="request-records-token-empty-note">暂无可统计 Token</small>
              </div>
            </div>
          </template>

          <template v-else-if="activityDashboardTab === 'activity'">
            <div class="request-records-activity-title">Codex 活跃趋势</div>
            <div ref="activityViewportRef" class="request-records-activity-viewport">
              <div class="request-records-activity-months" :style="{ '--activity-columns': activityTrend.columnCount }" aria-hidden="true">
                <span v-for="month in activityTrend.months" :key="month.key" :style="{ gridColumn: month.column }">{{ month.label }}</span>
              </div>
              <div
                class="request-records-activity-grid"
                :style="{ '--activity-columns': activityTrend.columnCount }"
                role="img"
                :aria-label="activityTrend.description"
              >
                <span
                  v-for="cell in activityTrend.cells"
                  :key="cell.key"
                  class="request-records-activity-cell"
                  :class="[{ 'is-pad': cell.isPad }, `is-level-${cell.level}`]"
                  :title="cell.title"
                ></span>
              </div>
            </div>
            <div class="request-records-activity-legend">
              <span>Less</span>
              <span class="request-records-activity-cell is-level-0"></span>
              <span class="request-records-activity-cell is-level-1"></span>
              <span class="request-records-activity-cell is-level-2"></span>
              <span class="request-records-activity-cell is-level-3"></span>
              <span class="request-records-activity-cell is-level-4"></span>
              <span>More</span>
            </div>
            <footer class="request-records-activity-summary">
              <div>
                <span>{{ activityTrend.primaryLabel }}</span>
                <strong>{{ activityTrend.primaryCount }} 个</strong>
              </div>
              <div>
                <span>活跃天数</span>
                <strong>{{ activityTrend.activeDays }} 天</strong>
              </div>
              <div>
                <span>总会话</span>
                <strong>{{ activityTrend.totalSessions }} 个</strong>
              </div>
            </footer>
          </template>

          <template v-else-if="activityDashboardTab === 'sessions'">
            <div class="request-records-activity-title">Session Trend</div>
            <div class="request-records-session-trend">
              <div class="request-records-session-bars">
                <div class="request-records-session-y-axis" aria-hidden="true">
                  <span v-for="tick in sessionTrend.yTicks" :key="tick.key" :style="{ bottom: `${tick.bottom}%` }">{{ tick.label }}</span>
                </div>
                <div class="request-records-session-plot">
                  <div
                    v-for="bar in sessionTrend.bars"
                    :key="bar.key"
                    class="request-records-session-bar"
                    :class="{ 'is-empty': bar.height <= 0 }"
                    :style="{ left: `${bar.left}%`, height: `${bar.height}%`, width: bar.width }"
                    :title="bar.title"
                  ></div>
                </div>
              </div>
              <div class="request-records-session-axis">
                <span v-for="label in sessionTrend.labels" :key="label.key">{{ label.label }}</span>
              </div>
              <footer class="request-records-session-summary">
                <div><span>Total Sessions</span><strong>{{ sessionTrend.totalSessions }}</strong></div>
                <div><span>Avg Turns</span><strong>{{ sessionTrend.avgTurns }}</strong></div>
                <div><span>Active Days</span><strong>{{ sessionTrend.activeDays }}</strong></div>
              </footer>
            </div>
          </template>

          <template v-else-if="activityDashboardTab === 'tools'">
            <div class="request-records-activity-title">Tool Call Ranking</div>
            <div class="request-records-tool-ranking">
              <div class="request-records-tool-list">
                <div v-for="item in toolRanking.items" :key="item.name" class="request-records-tool-row">
                  <span class="request-records-tool-name">{{ item.name }}</span>
                  <div class="request-records-tool-track">
                    <i :style="{ width: `${item.percent}%` }"></i>
                  </div>
                  <strong>{{ item.countLabel }}</strong>
                </div>
              </div>
              <div class="request-records-tool-side">
                <div class="request-records-tool-donut" :style="toolRanking.donutStyle">
                  <div class="request-records-tool-donut-hole">
                    <strong>{{ toolRanking.totalLabel }}</strong>
                    <span>Total Calls</span>
                  </div>
                </div>
                <div class="request-records-tool-breakdown">
                  <span><i class="is-edit"></i> Edit Tasks <strong>{{ toolRanking.editPercent }}%</strong></span>
                  <span><i class="is-search"></i> Search Tasks <strong>{{ toolRanking.searchPercent }}%</strong></span>
                </div>
              </div>
            </div>
          </template>
        </section>

        <section class="request-records-overview">
          <article class="request-records-metric">
            <span class="request-records-metric-label">请求数</span>
            <strong class="request-records-metric-value">{{ summary.total }}</strong>
            <small>{{ requestCountSubtext }}</small>
          </article>

          <article class="request-records-metric">
            <span class="request-records-metric-label">成功率</span>
            <strong class="request-records-metric-value">{{ summary.successRate }}</strong>
            <small>{{ summary.successCount }} ok · {{ summary.errorCount }} fail</small>
          </article>

          <article class="request-records-metric">
            <span class="request-records-metric-label">平均耗时</span>
            <strong class="request-records-metric-value">{{ summary.avgDuration }}</strong>
            <small>TTFT {{ summary.avgTtft }}</small>
          </article>

          <article class="request-records-metric">
            <span class="request-records-metric-label">Token</span>
            <strong class="request-records-metric-value">{{ summary.totalTokens }}</strong>
            <small>↑ {{ summary.inputTokens }} · ↓ {{ summary.outputTokens }} · TPS {{ summary.avgTps }}</small>
          </article>
        </section>

        <div v-if="!bridgeAvailable" class="request-records-empty">
          当前环境无法读取请求记录。
        </div>

        <section v-else class="request-records-board">
          <div class="request-records-board-head">
            <div class="request-records-board-title">
              <strong>请求流水</strong>
              <span>{{ filteredRecords.length }} 条</span>
            </div>

            <div class="request-records-board-chips">
              <button
                v-for="item in appSummaryItems"
                :key="`app-${item.id}`"
                type="button"
                class="request-records-board-chip request-records-board-chip-toggle"
                :class="{ 'is-inactive': isAppChipHidden(item.id) }"
                @click="toggleAppFilter(item.id)"
              >
                {{ item.label }} {{ item.count }}
              </button>
              <span
                v-for="item in routeSummaryItems"
                :key="`route-${item.id}`"
                class="request-records-board-chip request-records-board-chip-muted"
              >
                {{ item.label }} {{ item.count }}
              </span>
            </div>
          </div>

          <div class="request-records-table-wrap">
            <a-spin :spinning="loading" class="request-records-table-spin">
              <div
                ref="tableScrollRef"
                class="request-records-table-scroll"
                :class="{
                  'is-draggable': tableDragEnabled,
                  'is-dragging': tableDragging,
                }"
                :style="{ maxHeight: `${tableScrollY}px` }"
                @pointerdown="handleTablePointerDown"
                @click.capture="handleTableClickCapture"
              >
                <div
                  class="request-records-table-stage"
                  :style="{ width: `${tableLayoutWidth}px`, transform: `translateX(-${tableScrollLeft}px)` }"
                >
                  <table
                    ref="tableElementRef"
                    class="request-records-table-native"
                    :style="{ width: `${tableLayoutWidth}px` }"
                  >
                    <colgroup>
                      <col
                        v-for="column in columns"
                        :key="`col-${column.key}`"
                        :style="{ width: resolveColumnWidth(column.width) }"
                      />
                    </colgroup>
                    <thead>
                      <tr>
                        <th
                          v-for="column in columns"
                          :key="`head-${column.key}`"
                          class="request-records-table-head"
                          :class="{
                            'is-center': column.align === 'center',
                            'is-actions': column.key === 'actions',
                          }"
                        >
                          {{ column.title }}
                        </th>
                      </tr>
                    </thead>
                    <tbody v-if="pagedRecords.length > 0">
                      <tr v-for="record in pagedRecords" :key="record.id || record.recordedAt">
                        <td
                          v-for="column in columns"
                          :key="`${record.id || record.recordedAt}-${column.key}`"
                          class="request-records-table-cell"
                          :class="{
                            'is-center': column.align === 'center',
                            'is-actions': column.key === 'actions',
                          }"
                        >
                          <template v-if="column.key === 'time'">
                            <div class="request-records-time">
                              <strong>{{ formatTime(record.recordedAt) }}</strong>
                              <small>{{ formatDate(record.recordedAt) }}</small>
                            </div>
                          </template>

                          <template v-else-if="column.key === 'identity'">
                            <div class="request-records-identity">
                              <div class="request-records-identity-main">
                                <span class="request-records-app-pill">{{ formatAppName(record.appType) }}</span>
                                <strong>{{ record.providerName || '-' }}</strong>
                              </div>
                              <small class="request-records-mono">{{ record.model || '未记录模型' }}</small>
                              <small class="request-records-mono">{{ record.providerKeyPreview || '未记录 key' }}</small>
                            </div>
                          </template>

                          <template v-else-if="column.key === 'link'">
                            <div class="request-records-route">
                              <div class="request-records-route-line">
                                <span class="request-records-route-key">入口</span>
                                <span class="request-records-route-path">{{ summarizeInboundEndpoint(record.inboundEndpoint) }}</span>
                              </div>
                              <div class="request-records-route-line">
                                <span class="request-records-route-key is-meta">协议</span>
                                <span class="request-records-route-pill">{{ summarizeOutboundRoute(record.outboundRoute) }}</span>
                              </div>
                              <div class="request-records-route-line">
                                <span class="request-records-route-key is-out">出口</span>
                                <a-tooltip :title="record.upstreamUrl || record.upstreamEndpoint || '-'">
                                  <span class="request-records-route-path request-records-route-path-out">
                                    {{ summarizeUpstreamTarget(record.upstreamUrl || record.upstreamEndpoint, record.upstreamEndpoint) }}
                                  </span>
                                </a-tooltip>
                              </div>
                            </div>
                          </template>

                          <template v-else-if="column.key === 'route'">
                            <div class="request-records-routing">
                              <div
                                v-for="(step, index) in resolveRouteTraceSteps(record)"
                                :key="`${record.id || record.recordedAt || 'route'}-${index}-${step.route}-${step.status}`"
                                class="request-records-routing-line"
                                :class="resolveRouteTraceLineClass(record, step, index)"
                              >
                                <span class="request-records-routing-icon">{{ resolveRouteTraceIcon(record, step, index) }}</span>
                                <span class="request-records-routing-label">{{ formatRouteTraceLabel(step.route) }}</span>
                                <span v-if="resolveRouteTraceSourceLabel(step.source)" class="request-records-routing-source">
                                  {{ resolveRouteTraceSourceLabel(step.source) }}
                                </span>
                              </div>
                            </div>
                          </template>

                          <template v-else-if="column.key === 'metrics'">
                            <div class="request-records-metrics">
                              <strong>{{ formatDuration(record.durationMs) }}</strong>
                              <small>TTFT {{ formatDuration(record.ttftMs) }} · Gen {{ formatDuration(record.latencyMs) }} · TPS {{ formatTps(record.tps) }}</small>
                              <small>↑ {{ formatTokenValue(record.inputTokens) }} · ↓ {{ formatTokenValue(record.outputTokens) }}</small>
                            </div>
                          </template>

                          <template v-else-if="column.key === 'status'">
                            <div class="request-records-status">
                              <a-tag :color="resolveStatusColor(record.statusCode)">
                                {{ record.statusCode || '-' }}
                              </a-tag>
                              <span
                                class="request-records-source-pill"
                                :class="`is-${resolveSourceTone(record.source)}`"
                              >
                                {{ resolveSourceLabel(record.source) }}
                              </span>
                            </div>
                          </template>

                          <template v-else-if="column.key === 'detail'">
                            <div class="request-records-detail-cell">
                              <a-tooltip :title="resolveDetailText(record)">
                                <div class="request-records-detail-text">
                                  {{ summarizeDetail(record) }}
                                </div>
                              </a-tooltip>
                              <a-button type="text" size="small" class="request-records-detail-button" @click="openRecordDetail(record)">
                                详情
                              </a-button>
                            </div>
                          </template>
                        </td>
                      </tr>
                    </tbody>
                    <tbody v-else>
                      <tr>
                        <td :colspan="columns.length" class="request-records-empty-cell">
                          暂无请求记录
                        </td>
                      </tr>
                    </tbody>
                  </table>
                </div>
              </div>
            </a-spin>
            <div
              class="request-records-table-hscroll"
              :class="{ 'is-active': showTableHorizontalScroll }"
            >
              <input
                ref="tableHorizontalScrollRef"
                class="request-records-table-hscroll-range"
                type="range"
                min="0"
                :max="Math.max(tableHorizontalMaxScroll, 1)"
                :value="tableScrollLeft"
                :disabled="tableHorizontalMaxScroll <= 0"
                @input="handleTableHorizontalRangeInput"
              />
            </div>
            <div v-if="filteredRecords.length > REQUEST_RECORD_PAGE_SIZE" class="request-records-pagination">
              <a-pagination
                size="small"
                simple
                :current="currentPage"
                :page-size="REQUEST_RECORD_PAGE_SIZE"
                :total="filteredRecords.length"
                @change="handlePageChange"
              />
            </div>
          </div>
        </section>
        </template>
      </div>
    </div>

    <a-drawer
      :open="detailOpen"
      :width="detailDrawerWidth"
      placement="right"
      title="请求详情"
      :class="['advanced-proxy-records-detail-drawer', { 'advanced-proxy-records-detail-drawer-dark': isDarkMode }]"
      @close="closeRecordDetail"
    >
      <div v-if="recordDetailLoading" class="request-records-empty">
        正在加载详情...
      </div>
      <div v-else-if="selectedRecord" class="request-record-detail-shell" :class="{ 'request-record-detail-shell-dark': isDarkMode }">
        <header class="request-record-detail-hero">
          <div class="request-record-detail-hero-main">
            <span class="request-records-app-pill">{{ formatAppName(selectedRecord.appType) }}</span>
            <strong>{{ selectedRecord.providerName || '-' }}</strong>
            <small>{{ selectedRecord.model || '未记录模型' }}</small>
          </div>

          <div class="request-record-detail-hero-tags">
            <a-tag :color="resolveStatusColor(selectedRecord.statusCode)">
              {{ selectedRecord.statusCode || '-' }}
            </a-tag>
            <span
              class="request-records-source-pill"
              :class="`is-${resolveSourceTone(selectedRecord.source)}`"
            >
              {{ resolveSourceLabel(selectedRecord.source) }}
            </span>
          </div>
        </header>

        <section class="request-record-detail-section">
          <div class="request-record-detail-section-title">标识</div>
          <div class="request-record-detail-grid">
            <div class="request-record-detail-item">
              <span>时间</span>
              <strong>{{ formatDateTime(selectedRecord.recordedAt) }}</strong>
            </div>
            <div class="request-record-detail-item">
              <span>密钥</span>
              <strong class="request-records-mono">{{ selectedRecord.providerKeyPreview || '-' }}</strong>
            </div>
          </div>
        </section>

        <section class="request-record-detail-section">
          <div class="request-record-detail-section-title">链路</div>
          <div class="request-record-detail-grid">
            <div class="request-record-detail-item">
              <span>入口</span>
              <strong>{{ selectedRecord.inboundEndpoint || '-' }}</strong>
            </div>
            <div class="request-record-detail-item">
              <span>协议</span>
              <strong>{{ selectedRecord.outboundRoute || '-' }}</strong>
            </div>
            <div class="request-record-detail-item request-record-detail-item-full">
              <span>上游 URL</span>
              <strong>{{ selectedRecord.upstreamUrl || selectedRecord.upstreamEndpoint || '-' }}</strong>
            </div>
          </div>
        </section>

        <section class="request-record-detail-section">
          <div class="request-record-detail-section-title">性能</div>
          <div class="request-record-detail-grid">
            <div class="request-record-detail-item">
              <span>耗时</span>
              <strong>{{ formatDuration(selectedRecord.durationMs) }}</strong>
            </div>
            <div class="request-record-detail-item">
              <span>TTFT</span>
              <strong>{{ formatDuration(selectedRecord.ttftMs) }}</strong>
            </div>
            <div class="request-record-detail-item">
              <span>Latency</span>
              <strong>{{ formatDuration(selectedRecord.latencyMs) }}</strong>
            </div>
            <div class="request-record-detail-item">
              <span>Token</span>
              <strong>↑ {{ formatTokenValue(selectedRecord.inputTokens) }} · ↓ {{ formatTokenValue(selectedRecord.outputTokens) }}</strong>
            </div>
            <div class="request-record-detail-item">
              <span>TPS</span>
              <strong>{{ formatTps(selectedRecord.tps) }}</strong>
            </div>
          </div>
        </section>

        <section class="request-record-detail-section">
          <div class="request-record-detail-section-title">返回</div>
          <div class="request-record-detail-item request-record-detail-item-full">
            <span>上游原始响应预览</span>
            <pre>{{ selectedRecord.upstreamResponsePreview || '-' }}</pre>
          </div>
          <div class="request-record-detail-item request-record-detail-item-full">
            <span>客户端响应预览</span>
            <pre>{{ selectedRecord.responsePreview || '-' }}</pre>
          </div>
          <div class="request-record-detail-item request-record-detail-item-full">
            <span>错误详情</span>
            <pre>{{ selectedRecord.errorDetail || '-' }}</pre>
          </div>
        </section>

        <section class="request-record-detail-section">
          <div class="request-record-detail-section-title request-record-detail-section-title-row">
            <span>请求</span>
            <div class="request-record-detail-section-actions">
              <a-button
                size="small"
                class="request-record-debug-button"
                :loading="requestDebugTesting"
                @click="handleRequestDebugTest"
              >
                测试
              </a-button>
              <a-tooltip v-if="requestDebugState !== 'idle'" :title="requestDebugResponse || '-'">
                <span class="request-record-debug-result" :class="`is-${requestDebugState}`">
                  <LoadingOutlined v-if="requestDebugState === 'loading'" />
                  <CheckCircleFilled v-else-if="requestDebugState === 'success'" />
                  <CloseCircleFilled v-else />
                </span>
              </a-tooltip>
            </div>
          </div>
          <a-textarea
            v-model:value="requestDebugBody"
            class="request-record-debug-textarea"
            :auto-size="{ minRows: 12, maxRows: 22 }"
          />
        </section>
      </div>
    </a-drawer>
  </a-drawer>
</template>

<script setup>
import { computed, nextTick, onBeforeUnmount, onMounted, ref, watch } from 'vue';
import { message, Modal } from 'ant-design-vue';
import {
  BarChartOutlined,
  CheckCircleFilled,
  CloseCircleFilled,
  CodeOutlined,
  DeleteOutlined,
  FireOutlined,
  StopOutlined,
  InboxOutlined,
  ProfileOutlined,
  LoadingOutlined,
  ReloadOutlined,
} from '@ant-design/icons-vue';
import {
  clearAdvancedProxyRequestRecords,
  getMCPSkillConfigSnapshot,
  getLocalTokenUsageAnalytics,
  getTerminalSessionMessages,
  getAdvancedProxyConfig,
  getAdvancedProxyEffectiveProviders,
  getAdvancedProxyRequestRecord,
  isAdvancedProxyRequestRecordBridgeAvailable,
  isMCPSkillConfigBridgeAvailable,
  isTerminalSessionBridgeAvailable,
  launchTerminalSession,
  listAdvancedProxyRequestRecords,
  listTerminalSessions,
  saveMCPSkillConfigSnapshot,
} from '../utils/advancedProxyBridge.js';
import claudeAppIcon from '../assets/app-icons/claude.svg';
import codexAppIcon from '../assets/app-icons/codex.svg';
import geminiAppIcon from '../assets/app-icons/gemini.svg';
import opencodeAppIcon from '../assets/app-icons/opencode.svg';
import openclawAppIcon from '../assets/app-icons/openclaw-fallback.svg';
import { tr } from '../i18n/runtime.js';

const props = defineProps({
  open: {
    type: Boolean,
    default: false,
  },
  isDarkMode: {
    type: Boolean,
    default: false,
  },
  focusRecordId: {
    type: String,
    default: '',
  },
  initialPanel: {
    type: String,
    default: 'records',
  },
});

const emit = defineEmits(['update:open']);

const bridgeAvailable = isAdvancedProxyRequestRecordBridgeAvailable();
const terminalBridgeAvailable = isTerminalSessionBridgeAvailable();
const mcpSkillBridgeAvailable = isMCPSkillConfigBridgeAvailable();
const loading = ref(false);
const tokenAnalyticsLoading = ref(false);
const sessionsLoading = ref(false);
const mcpSkillLoading = ref(false);
const mcpSkillSaving = ref(false);
const records = ref([]);
const localTokenAnalytics = ref(null);
const hiddenAppIds = ref([]);
const activePanel = ref('records');
const activityDashboardTab = ref('token');
const activityRange = ref('week');
const activityHeatmapRange = ref('week');
const mcpSkillConfigPath = ref('');
const managedMCPServers = ref([]);
const managedSkills = ref([]);
const selectedManagedAppId = ref('codex');
const terminalProviderId = ref('codex');
const terminalSessionPage = ref(1);
const terminalSessionTotal = ref(0);
const terminalSessions = ref([]);
const terminalProviders = ref([]);
const selectedTerminalSession = ref(null);
const terminalSessionMessages = ref([]);
const terminalSessionMessagesLoading = ref(false);
const expandedTerminalMessageIndexes = ref([]);
const launchingSessionKey = ref('');
const currentPage = ref(1);
const detailOpen = ref(false);
const selectedRecord = ref(null);
const requestDebugBody = ref('');
const requestDebugState = ref('idle');
const requestDebugResponse = ref('');
const recordDetailLoading = ref(false);
const tableScrollRef = ref(null);
const tableElementRef = ref(null);
const tableHorizontalScrollRef = ref(null);
const activityViewportRef = ref(null);
const tableContentWidth = ref(0);
const tableViewportWidth = ref(0);
const tableScrollLeft = ref(0);
const tableVerticalMaxScroll = ref(0);
const tableDragging = ref(false);
const tokenTooltip = ref({
  visible: false,
  x: 0,
  y: 0,
  title: '',
  period: '',
  cumulative: '',
});
const viewportWidth = ref(typeof window === 'undefined' ? 900 : window.innerWidth);
const viewportHeight = ref(typeof window === 'undefined' ? 600 : window.innerHeight);
let tableMetricsFrame = 0;
let tableResizeObserver = null;
let tableDragSession = null;
let tableSuppressClickUntil = 0;
let recordDetailRequestToken = 0;
let terminalSessionMessageRequestToken = 0;
const REQUEST_RECORD_PAGE_SIZE = 50;
const TERMINAL_SESSION_PAGE_SIZE = 15;
const TERMINAL_SESSION_MESSAGE_LIMIT = 80;
const TERMINAL_SESSION_COLLAPSE_LINE_LIMIT = 10;
const DEFAULT_TERMINAL_PROVIDERS = [
  { id: 'codex', label: 'Codex', total: 0 },
  { id: 'claude', label: 'Claude', total: 0 },
  { id: 'opencode', label: 'OpenCode', total: 0 },
  { id: 'openclaw', label: 'OpenClaw', total: 0 },
  { id: 'gemini', label: 'Gemini', total: 0 },
];
const activitySectionTabs = [
  { id: 'token', label: 'Token' },
  { id: 'activity', label: '活跃趋势' },
  { id: 'sessions', label: '会话' },
  { id: 'tools', label: '工具' },
];
const activityRangeTabs = [
  { id: 'today', label: '今日' },
  { id: 'week', label: '本周' },
  { id: 'month', label: '本月' },
];
const activityHeatmapRangeTabs = [
  { id: 'week', label: 'Week' },
  { id: 'month', label: 'Month' },
  { id: 'year', label: 'Year' },
];
const managedAppItems = [
  { id: 'codex', short: 'X', label: 'Codex' },
  { id: 'claude', short: 'C', label: 'Claude' },
  { id: 'opencode', short: 'O', label: 'OpenCode' },
  { id: 'openclaw', short: 'L', label: 'OpenClaw' },
  { id: 'gemini', short: 'G', label: 'Gemini' },
];
const TERMINAL_PROVIDER_ICONS = {
  codex: codexAppIcon,
  claude: claudeAppIcon,
  opencode: opencodeAppIcon,
  openclaw: openclawAppIcon,
  gemini: geminiAppIcon,
};

const isCompactWindow = computed(() => viewportWidth.value <= 860);
const dashboardRangeTabs = computed(() => (
  activityDashboardTab.value === 'activity' ? activityHeatmapRangeTabs : activityRangeTabs
));
const dashboardRangeValue = computed(() => (
  activityDashboardTab.value === 'activity' ? activityHeatmapRange.value : activityRange.value
));
const drawerWidth = computed(() => Math.min(Math.max(viewportWidth.value - 18, 380), 1080));
const detailDrawerWidth = computed(() => Math.min(Math.max(Math.floor(viewportWidth.value * 0.42), 320), 420));
const tableScrollY = computed(() => {
  const reservedHeight = isCompactWindow.value ? 360 : 332;
  return Math.max(280, Math.min(620, viewportHeight.value - reservedHeight));
});
const tableLayoutWidth = computed(() => columns.value.reduce((sum, column) => {
  const width = Number(column?.width || 0);
  return sum + (Number.isFinite(width) && width > 0 ? width : 0);
}, 0));
const tableViewportFallbackWidth = computed(() => {
  const numeric = Number(drawerWidth.value || 0);
  if (!Number.isFinite(numeric) || numeric <= 0) return 0;
  return Math.max(260, numeric - 32);
});
const showTableHorizontalScroll = computed(() => tableLayoutWidth.value - tableViewportWidth.value > 2);
const tableHorizontalMaxScroll = computed(() => Math.max(0, tableContentWidth.value - tableViewportWidth.value));
const tableDragEnabled = computed(() => tableHorizontalMaxScroll.value > 0 || tableVerticalMaxScroll.value > 0);
const requestDebugTesting = computed(() => requestDebugState.value === 'loading');
const terminalProviderItems = computed(() => {
  const providerMap = new Map(DEFAULT_TERMINAL_PROVIDERS.map(item => [item.id, { ...item }]));
  (Array.isArray(terminalProviders.value) ? terminalProviders.value : []).forEach((item) => {
    const id = String(item?.id || '').trim().toLowerCase();
    if (!id) return;
    providerMap.set(id, {
      id,
      label: String(item?.label || providerMap.get(id)?.label || id).trim(),
      total: Number(item?.total || providerMap.get(id)?.total || 0),
    });
  });
  return DEFAULT_TERMINAL_PROVIDERS.map(item => providerMap.get(item.id) || item);
});
const selectedTerminalSessionKey = computed(() => (selectedTerminalSession.value ? getTerminalSessionKey(selectedTerminalSession.value) : ''));
const selectedManagedApp = computed(() => managedAppItems.find(item => item.id === selectedManagedAppId.value) || managedAppItems[0]);
const selectedManagedAppLabel = computed(() => selectedManagedApp.value?.label || 'Codex');
const visibleManagedMCPServers = computed(() => managedMCPServers.value.filter(server => isManagedAppEnabled(server.apps, selectedManagedAppId.value)));
const visibleManagedSkills = computed(() => managedSkills.value.filter(skill => isManagedAppEnabled(skill.apps, selectedManagedAppId.value)));
const mcpSkillCountLabel = computed(() => {
  if (mcpSkillLoading.value) return '扫描中';
  if (activePanel.value === 'mcp') {
    return `MCP ${visibleManagedMCPServers.value.length} / ${managedMCPServers.value.length} 个`;
  }
  return `Skill ${visibleManagedSkills.value.length} / ${managedSkills.value.length} 个`;
});

const columns = computed(() => {
  const compact = isCompactWindow.value;
  return [
    { title: '时间', dataIndex: 'recordedAt', key: 'time', width: compact ? 82 : 90 },
    { title: 'Provider', dataIndex: 'providerName', key: 'identity', width: compact ? 168 : 182 },
    { title: '链路', dataIndex: 'outboundRoute', key: 'link', width: compact ? 220 : 250 },
    { title: '路由', dataIndex: 'routeTrace', key: 'route', width: compact ? 138 : 152 },
    { title: '性能', dataIndex: 'durationMs', key: 'metrics', width: compact ? 146 : 158 },
    { title: '状态', dataIndex: 'statusCode', key: 'status', width: compact ? 88 : 96 },
    { title: '摘要', dataIndex: 'responsePreview', key: 'detail', width: compact ? 276 : 346, ellipsis: true },
  ];
});

const filteredRecords = computed(() => {
  const list = Array.isArray(records.value) ? records.value : [];
  if (hiddenAppIds.value.length === 0) {
    return list;
  }
  const hiddenSet = new Set(hiddenAppIds.value);
  return list.filter((record) => !hiddenSet.has(String(record?.appType || '').trim().toLowerCase()));
});

const pagedRecords = computed(() => {
  const start = (currentPage.value - 1) * REQUEST_RECORD_PAGE_SIZE;
  return filteredRecords.value.slice(start, start + REQUEST_RECORD_PAGE_SIZE);
});

const requestCountSubtext = computed(() => {
  if (hiddenAppIds.value.length === 0) {
    return `最近 ${records.value.length} 条`;
  }
  return `显示 ${filteredRecords.value.length} / ${records.value.length} 条`;
});

const summary = computed(() => {
  const list = filteredRecords.value;
  const total = list.length;
  const successCount = list.filter((record) => {
    const code = Number(record?.statusCode || 0);
    return code >= 200 && code < 300;
  }).length;
  const errorCount = Math.max(0, total - successCount);
  const durationValues = list
    .map(record => Number(record?.durationMs || 0))
    .filter(value => Number.isFinite(value) && value > 0);
  const ttftValues = list
    .map(record => Number(record?.ttftMs || 0))
    .filter(value => Number.isFinite(value) && value > 0);
  const inputTokens = list
    .map(record => Number(record?.inputTokens || 0))
    .filter(value => Number.isFinite(value) && value > 0)
    .reduce((sum, value) => sum + value, 0);
  const outputTokens = list
    .map(record => Number(record?.outputTokens || 0))
    .filter(value => Number.isFinite(value) && value > 0)
    .reduce((sum, value) => sum + value, 0);
  const tpsValues = list
    .map(record => Number(record?.tps))
    .filter(value => Number.isFinite(value) && value > 0);

  return {
    total,
    successCount,
    errorCount,
    successRate: total > 0 ? `${Math.round((successCount / total) * 100)}%` : '-',
    avgDuration: durationValues.length > 0
      ? formatDuration(durationValues.reduce((sum, value) => sum + value, 0) / durationValues.length)
      : '-',
    avgTtft: ttftValues.length > 0
      ? formatDuration(ttftValues.reduce((sum, value) => sum + value, 0) / ttftValues.length)
      : '-',
    avgTps: tpsValues.length > 0
      ? formatTps(tpsValues.reduce((sum, value) => sum + value, 0) / tpsValues.length)
      : '-',
    inputTokens: formatCompactNumber(inputTokens),
    outputTokens: formatCompactNumber(outputTokens),
    totalTokens: formatCompactNumber(inputTokens + outputTokens),
  };
});

const activityTrend = computed(() => {
  const today = startOfLocalDay(new Date());
  const end = new Date(today);
  const range = activityHeatmapRange.value;
  const start = getActivityRangeStart(today, range);
  const firstGridDay = new Date(start);
  firstGridDay.setDate(firstGridDay.getDate() - firstGridDay.getDay());
  const counts = new Map();

  const localSessions = Array.isArray(localTokenAnalytics.value?.sessionSeries)
    ? localTokenAnalytics.value.sessionSeries
    : [];
  if (localSessions.length > 0) {
    localSessions.forEach((item) => {
      const key = String(item?.date || '');
      const date = parseDateKey(key);
      if (!date || date < start || date > end) return;
      counts.set(key, (counts.get(key) || 0) + getPositiveNumber(item?.sessionCount));
    });
  } else {
    filteredRecords.value.forEach((record) => {
      const date = getRecordDate(record);
      if (!date || date < start || date > end) return;
      const key = formatDateKey(date);
      counts.set(key, (counts.get(key) || 0) + 1);
    });
  }

  const maxCount = Math.max(0, ...counts.values());
  const cells = [];
  const cursor = new Date(firstGridDay);
  while (cursor <= end) {
    const key = formatDateKey(cursor);
    const isPad = cursor < start;
    const count = counts.get(key) || 0;
    cells.push({
      key,
      count,
      isPad,
      level: isPad ? -1 : getActivityLevel(count, maxCount),
      title: isPad ? '' : `${formatHeatmapDate(cursor)}: ${count} Sessions`,
    });
    cursor.setDate(cursor.getDate() + 1);
  }

  const columnCount = Math.max(1, Math.ceil(cells.length / 7));
  const months = [];
  let lastMonthKey = '';
  cells.forEach((cell, index) => {
    const date = parseDateKey(cell.key);
    if (!date || cell.isPad) return;
    if (date.getDate() > 7) return;
    const monthKey = `${date.getFullYear()}-${date.getMonth()}`;
    if (monthKey === lastMonthKey) return;
    lastMonthKey = monthKey;
    months.push({
      key: monthKey,
      label: `${date.getMonth() + 1}月`,
      column: `${Math.floor(index / 7) + 1}`,
    });
  });

  const activeDays = countActiveDays(counts, start, today);
  const totalSessions = [...counts.values()].reduce((sum, value) => sum + value, 0);
  const primaryCount = counts.get(formatDateKey(today)) || 0;
  const primaryLabel = range === 'week' ? '今日会话' : range === 'month' ? '本月会话' : '本年会话';

  return {
    cells,
    months,
    columnCount,
    primaryLabel,
    primaryCount: range === 'week' ? primaryCount : totalSessions,
    activeDays,
    totalSessions,
    description: `Codex activity heatmap, ${cells.length} days`,
  };
});

const tokenTrend = computed(() => {
  const buckets = buildTokenBuckets(activityRange.value);
  const bucketMap = new Map(buckets.map(bucket => [bucket.key, bucket]));
  const localEntries = getLocalTokenTrendEntries(activityRange.value);
  const useLocalAnalytics = localEntries.length > 0;
  const sourceCounts = new Map();

  if (useLocalAnalytics) {
    localEntries.forEach((entry) => {
      const date = parseDateKey(entry.date);
      if (!date || Number.isNaN(date.getTime())) return;
      const bucketKey = activityRange.value === 'today'
        ? `${entry.date}-${String(entry.hour || '00').padStart(2, '0')}`
        : entry.date;
      const bucket = bucketMap.get(bucketKey);
      if (!bucket) return;
      bucket.input += getPositiveNumber(entry.inputTokens);
      bucket.output += getPositiveNumber(entry.outputTokens);
      bucket.reasoning += getPositiveNumber(entry.reasoningTokens);
      const source = String(entry.sourceLabel || entry.source || 'Codex').trim() || 'Codex';
      sourceCounts.set(source, (sourceCounts.get(source) || 0) + getPositiveNumber(entry.totalTokens));
    });
  } else {
    filteredRecords.value.forEach((record) => {
      const date = getRecordTimestamp(record);
      if (!date) return;
      const bucketKey = resolveTokenBucketKey(date, activityRange.value);
      const bucket = bucketMap.get(bucketKey);
      if (!bucket) return;
      const usage = getRecordTokenUsage(record);
      const reasoning = getReasoningTokens(record);
      bucket.input += usage.input;
      bucket.output += usage.output;
      bucket.reasoning += reasoning;
      const total = usage.input + usage.output + reasoning;
      if (total > 0) {
        const source = formatAppName(record?.appType || '代理记录');
        sourceCounts.set(source, (sourceCounts.get(source) || 0) + total);
      }
    });
  }

  const totals = buckets.map(bucket => bucket.input + bucket.output + bucket.reasoning);
  const maxTotal = Math.max(1, ...totals);
  const grandTotal = totals.reduce((sum, value) => sum + value, 0);
  const inputTotal = buckets.reduce((sum, bucket) => sum + bucket.input, 0);
  const outputTotal = buckets.reduce((sum, bucket) => sum + bucket.output, 0);
  const reasoningTotal = buckets.reduce((sum, bucket) => sum + bucket.reasoning, 0);
  const cumulativeValues = [];
  totals.reduce((sum, value, index) => {
    const next = sum + value;
    cumulativeValues[index] = next;
    return next;
  }, 0);
  const maxCumulative = Math.max(1, ...cumulativeValues);
  const width = 640;
  const height = 220;
  const chartBottom = 204;
  const chartTop = 16;
  const chartHeight = chartBottom - chartTop;
  const pointStep = buckets.length > 1 ? width / (buckets.length - 1) : width;
  const linePoints = cumulativeValues.map((value, index) => ({
    x: buckets.length === 1 ? width / 2 : index * pointStep,
    y: chartBottom - (value / maxCumulative) * chartHeight,
  }));
  const linePath = buildSmoothLinePath(linePoints);

  return {
    bars: buckets.map((bucket, index) => {
      const total = totals[index] || 0;
      const left = getBucketCenterPercent(index, buckets.length);
      return {
        key: bucket.key,
        label: bucket.label,
        total,
        cumulative: cumulativeValues[index] || 0,
        left,
        height: total > 0 ? Math.max(2, (total / maxTotal) * 82) : 0,
        title: `${bucket.label}: ${formatCompactNumber(total)} Token`,
        tooltipTitle: bucket.key.length > 10 ? bucket.key : formatTokenTooltipDate(bucket.key),
        periodLabel: `Period Token Usage ${formatCompactNumber(total)}`,
        cumulativeLabel: `Cumulative Token Usage ${formatCompactNumber(cumulativeValues[index] || 0)}`,
      };
    }),
    labels: selectAxisLabels(buckets),
    linePath,
    gridLines: [28, 76, 124, 172, 220],
    verticalLines: [128, 256, 384, 512],
    total: grandTotal,
    totalLabel: formatCompactNumber(grandTotal),
    inputPercent: formatPercent(inputTotal, grandTotal),
    outputPercent: formatPercent(outputTotal, grandTotal),
    reasoningPercent: formatPercent(reasoningTotal, grandTotal),
    sourceLabel: useLocalAnalytics ? '本地 Codex' : '代理记录',
    sourceItems: [...sourceCounts.entries()]
      .sort((left, right) => right[1] - left[1])
      .slice(0, 3)
      .map(([label, value]) => ({ label, value: formatCompactNumber(value) })),
    loading: tokenAnalyticsLoading.value,
    hasData: grandTotal > 0,
  };
});

const sessionTrend = computed(() => {
  const buckets = buildTokenBuckets(activityRange.value).map(bucket => ({
    ...bucket,
    sessions: 0,
    turns: 0,
  }));
  const bucketMap = new Map(buckets.map(bucket => [bucket.key, bucket]));
  getLocalSessionTrendEntries(activityRange.value).forEach((entry) => {
    const bucketKey = activityRange.value === 'today'
      ? `${entry.date}-${String(new Date().getHours()).padStart(2, '0')}`
      : entry.date;
    const bucket = bucketMap.get(bucketKey);
    if (!bucket) return;
    bucket.sessions += getPositiveNumber(entry.sessionCount);
    bucket.turns += getPositiveNumber(entry.turnCount);
  });
  const totals = buckets.map(bucket => bucket.sessions);
  const maxTotal = Math.max(1, ...totals);
  const axisMax = getNiceAxisMax(maxTotal);
  const yTicks = buildYAxisTicks(axisMax);
  const totalSessions = totals.reduce((sum, value) => sum + value, 0);
  const totalTurns = buckets.reduce((sum, bucket) => sum + bucket.turns, 0);
  return {
    bars: buckets.map((bucket, index) => {
      const left = getBucketCenterPercent(index, buckets.length);
      const width = activityRange.value === 'month'
        ? 'clamp(5px, 0.9%, 10px)'
        : activityRange.value === 'today'
          ? 'clamp(8px, 1.8%, 18px)'
          : 'clamp(14px, 2.8%, 26px)';
      return {
        key: bucket.key,
        left,
        width,
        height: bucket.sessions > 0 ? Math.max(3, (bucket.sessions / axisMax) * 92) : 0,
        title: `${formatTokenTooltipDate(bucket.key)}: ${bucket.sessions} Sessions`,
      };
    }),
    labels: selectAxisLabels(buckets),
    yTicks,
    totalSessions,
    avgTurns: totalSessions > 0 ? (totalTurns / totalSessions).toFixed(1) : '0',
    activeDays: buckets.filter(bucket => bucket.sessions > 0).length,
  };
});

const toolRanking = computed(() => {
  const analytics = localTokenAnalytics.value || {};
  const items = (Array.isArray(analytics.toolRanking) ? analytics.toolRanking : [])
    .filter(item => getPositiveNumber(item?.count) > 0)
    .slice(0, 8);
  const total = items.reduce((sum, item) => sum + getPositiveNumber(item.count), 0);
  const maxCount = Math.max(1, ...items.map(item => getPositiveNumber(item.count)));
  const editCount = items
    .filter(item => item?.category === 'edit')
    .reduce((sum, item) => sum + getPositiveNumber(item.count), 0);
  const searchCount = items
    .filter(item => item?.category === 'search')
    .reduce((sum, item) => sum + getPositiveNumber(item.count), 0);
  const editPercent = formatPercent(editCount, total);
  const searchPercent = formatPercent(searchCount, total);
  return {
    items: items.map(item => ({
      name: String(item?.name || '').trim(),
      category: String(item?.category || 'other').trim(),
      count: getPositiveNumber(item?.count),
      countLabel: formatCompactNumber(item?.count),
      percent: Math.max(1, (getPositiveNumber(item?.count) / maxCount) * 100),
    })),
    total,
    totalLabel: formatCompactNumber(total),
    editPercent,
    searchPercent,
    donutStyle: {
      background: total > 0
        ? `conic-gradient(#40c463 0 ${editPercent}%, #6fc4ec ${editPercent}% ${editPercent + searchPercent}%, rgba(70, 132, 92, 0.12) ${editPercent + searchPercent}% 100%)`
        : 'conic-gradient(rgba(120, 132, 126, 0.16) 0 100%)',
    },
  };
});

const tokenDonutStyle = computed(() => {
  if (!tokenTrend.value.total) {
    return {
      background: 'conic-gradient(rgba(120, 132, 126, 0.16) 0 100%)',
    };
  }
  const input = Number(tokenTrend.value.inputPercent || 0);
  const output = Number(tokenTrend.value.outputPercent || 0);
  const reasoning = Math.max(0, 100 - input - output);
  return {
    background: `conic-gradient(#6fc4ec 0 ${input}%, #77d99e ${input}% ${input + output}%, #ffd06a ${input + output}% ${input + output + reasoning}%, rgba(120, 132, 126, 0.16) ${input + output + reasoning}% 100%)`,
  };
});

const activeActivityTabLabel = computed(() => {
  const tab = activitySectionTabs.find(item => item.id === activityDashboardTab.value);
  return tab?.label || '统计';
});

const activityDashboardPlaceholder = computed(() => {
  if (activityDashboardTab.value === 'sessions') {
    const total = terminalSessionTotal.value || terminalSessions.value.length || 0;
    return `当前记录 ${formatCompactNumber(total)} 个会话，后续可展开为会话时长和消息趋势。`;
  }
  if (activityDashboardTab.value === 'tools') {
    const total = managedMCPServers.value.length + managedSkills.value.length;
    return `当前管理 ${formatCompactNumber(total)} 个 MCP/Skill 项，后续可展开为调用分布。`;
  }
  return '暂无可展示数据。';
});

const statusSummaryItems = computed(() => {
  const buckets = [
    {
      id: '2xx',
      label: '2xx',
      count: filteredRecords.value.filter((record) => {
        const code = Number(record?.statusCode || 0);
        return code >= 200 && code < 300;
      }).length,
    },
    {
      id: '4xx',
      label: '4xx',
      count: filteredRecords.value.filter((record) => {
        const code = Number(record?.statusCode || 0);
        return code >= 400 && code < 500;
      }).length,
    },
    {
      id: '5xx',
      label: '5xx',
      count: filteredRecords.value.filter((record) => {
        const code = Number(record?.statusCode || 0);
        return code >= 500;
      }).length,
    },
  ];
  return buckets.filter(item => item.count > 0);
});

const appSummaryItems = computed(() => {
  const counts = new Map();
  records.value.forEach((record) => {
    const appId = String(record?.appType || '').trim().toLowerCase();
    if (!appId) return;
    counts.set(appId, (counts.get(appId) || 0) + 1);
  });
  return [...counts.entries()]
    .sort((left, right) => right[1] - left[1])
    .slice(0, 4)
    .map(([id, count]) => ({
      id,
      label: formatAppName(id),
      count,
    }));
});

const routeSummaryItems = computed(() => {
  const counts = new Map();
  filteredRecords.value.forEach((record) => {
    const route = summarizeOutboundRoute(record?.outboundRoute);
    if (!route || route === '-') return;
    counts.set(route, (counts.get(route) || 0) + 1);
  });
  return [...counts.entries()]
    .sort((left, right) => right[1] - left[1])
    .slice(0, 3)
    .map(([id, count]) => ({
      id,
      label: id,
      count,
    }));
});

function syncViewport() {
  viewportWidth.value = typeof window === 'undefined' ? 900 : window.innerWidth;
  viewportHeight.value = typeof window === 'undefined' ? 600 : window.innerHeight;
  queueTableMetricsSync();
}

function detachTableResizeObserver() {
  if (tableResizeObserver) {
    tableResizeObserver.disconnect();
    tableResizeObserver = null;
  }
}

function attachTableResizeObserver() {
  detachTableResizeObserver();
  if (typeof ResizeObserver === 'undefined') return;
  const scrollElement = tableScrollRef.value;
  const tableElement = tableElementRef.value;
  if (!scrollElement || !tableElement) return;
  tableResizeObserver = new ResizeObserver(() => {
    queueTableMetricsSync();
  });
  tableResizeObserver.observe(scrollElement);
  tableResizeObserver.observe(tableElement);
}

function syncTableMetrics() {
  tableMetricsFrame = 0;
  const scrollElement = tableScrollRef.value;
  const tableElement = tableElementRef.value;
  if (!scrollElement || !tableElement) {
    tableContentWidth.value = 0;
    tableViewportWidth.value = tableViewportFallbackWidth.value;
    tableVerticalMaxScroll.value = 0;
    tableScrollLeft.value = 0;
    return;
  }
  const measuredViewportWidth = Math.max(scrollElement.clientWidth, 0);
  const fallbackViewportWidth = tableViewportFallbackWidth.value;
  tableContentWidth.value = Math.max(tableLayoutWidth.value, tableElement.offsetWidth, 0);
  tableVerticalMaxScroll.value = Math.max(0, scrollElement.scrollHeight - scrollElement.clientHeight);
  if (fallbackViewportWidth > 0) {
    if (measuredViewportWidth > 0) {
      tableViewportWidth.value = Math.min(measuredViewportWidth, fallbackViewportWidth);
    } else {
      tableViewportWidth.value = fallbackViewportWidth;
    }
  } else {
    tableViewportWidth.value = measuredViewportWidth;
  }
  const maxScrollLeft = Math.max(0, tableContentWidth.value - tableViewportWidth.value);
  if (tableScrollLeft.value > maxScrollLeft) {
    tableScrollLeft.value = maxScrollLeft;
  }
}

function queueTableMetricsSync() {
  if (tableMetricsFrame) {
    window.cancelAnimationFrame(tableMetricsFrame);
  }
  tableMetricsFrame = window.requestAnimationFrame(() => {
    syncTableMetrics();
  });
}

function setTableScrollLeft(nextScrollLeft) {
  const maxScrollLeft = Math.max(0, tableContentWidth.value - tableViewportWidth.value);
  const clamped = Math.max(0, Math.min(maxScrollLeft, Number(nextScrollLeft) || 0));
  tableScrollLeft.value = clamped;
}

function handleTableHorizontalRangeInput(event) {
  const nextValue = Number(event?.target?.value || 0);
  setTableScrollLeft(nextValue);
}

function isTableInteractiveTarget(target) {
  if (!(target instanceof Element)) return false;
  return Boolean(target.closest([
    'button',
    'a',
    'input',
    'textarea',
    'select',
    'label',
    '[role="button"]',
    '.ant-btn',
    '.ant-pagination',
    '.request-records-detail-button',
  ].join(',')));
}

function detachTableDragListeners() {
  window.removeEventListener('pointermove', handleTablePointerMove);
  window.removeEventListener('pointerup', handleTablePointerEnd);
  window.removeEventListener('pointercancel', handleTablePointerEnd);
}

function clearTableDragSession() {
  const session = tableDragSession;
  if (session?.scrollElement?.releasePointerCapture && session.pointerId != null) {
    try {
      session.scrollElement.releasePointerCapture(session.pointerId);
    } catch {}
  }
  detachTableDragListeners();
  tableDragging.value = false;
  tableDragSession = null;
  if (typeof document !== 'undefined') {
    document.body.style.removeProperty('user-select');
  }
}

function handleTablePointerDown(event) {
  const scrollElement = tableScrollRef.value;
  if (!scrollElement || !tableDragEnabled.value) return;
  if (event.pointerType === 'mouse' && event.button !== 0) return;
  if (event.isPrimary === false) return;
  if (isTableInteractiveTarget(event.target)) return;

  clearTableDragSession();
  tableDragSession = {
    pointerId: event.pointerId,
    startClientX: Number(event.clientX || 0),
    startClientY: Number(event.clientY || 0),
    startScrollLeft: tableScrollLeft.value,
    startScrollTop: scrollElement.scrollTop,
    scrollElement,
    dragging: false,
  };

  if (scrollElement.setPointerCapture) {
    try {
      scrollElement.setPointerCapture(event.pointerId);
    } catch {}
  }

  window.addEventListener('pointermove', handleTablePointerMove, { passive: false });
  window.addEventListener('pointerup', handleTablePointerEnd, { passive: false });
  window.addEventListener('pointercancel', handleTablePointerEnd, { passive: false });
}

function handleTablePointerMove(event) {
  const session = tableDragSession;
  if (!session || session.pointerId !== event.pointerId) return;
  const deltaX = Number(event.clientX || 0) - session.startClientX;
  const deltaY = Number(event.clientY || 0) - session.startClientY;
  if (!session.dragging && Math.abs(deltaX) < 3 && Math.abs(deltaY) < 3) {
    return;
  }

  if (!session.dragging) {
    session.dragging = true;
    tableDragging.value = true;
    tableSuppressClickUntil = Date.now() + 250;
    if (typeof document !== 'undefined') {
      document.body.style.userSelect = 'none';
    }
  }

  event.preventDefault();
  setTableScrollLeft(session.startScrollLeft - deltaX);
  const maxScrollTop = Math.max(0, session.scrollElement.scrollHeight - session.scrollElement.clientHeight);
  session.scrollElement.scrollTop = Math.max(0, Math.min(maxScrollTop, session.startScrollTop - deltaY));
}

function handleTablePointerEnd(event) {
  const session = tableDragSession;
  if (!session || session.pointerId !== event.pointerId) return;
  clearTableDragSession();
}

function handleTableClickCapture(event) {
  if (Date.now() > tableSuppressClickUntil) return;
  tableSuppressClickUntil = 0;
  event.preventDefault();
  event.stopPropagation();
}

function normalizeText(value) {
  return String(value || '').trim();
}

function resolveColumnWidth(value) {
  const numeric = Number(value || 0);
  if (!Number.isFinite(numeric) || numeric <= 0) return 'auto';
  return `${numeric}px`;
}

function formatDateTime(value) {
  const text = normalizeText(value);
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

function formatTime(value) {
  const text = formatDateTime(value);
  if (text === '-') return text;
  const parts = text.split(' ');
  return parts[1] || text;
}

function formatDate(value) {
  const text = formatDateTime(value);
  if (text === '-') return text;
  const parts = text.split(' ');
  return parts[0] || text;
}

function startOfLocalDay(date) {
  return new Date(date.getFullYear(), date.getMonth(), date.getDate());
}

function formatDateKey(date) {
  const year = date.getFullYear();
  const month = String(date.getMonth() + 1).padStart(2, '0');
  const day = String(date.getDate()).padStart(2, '0');
  return `${year}-${month}-${day}`;
}

function parseDateKey(key) {
  const [year, month, day] = String(key || '').split('-').map(Number);
  return new Date(year || 1970, Math.max(0, (month || 1) - 1), day || 1);
}

function formatHeatmapDate(date) {
  return date.toLocaleDateString('zh-CN', {
    month: '2-digit',
    day: '2-digit',
  });
}

function getRecordDate(record) {
  const date = getRecordTimestamp(record);
  return date ? startOfLocalDay(date) : null;
}

function getActivityRangeStart(today, range) {
  const start = new Date(today);
  if (range === 'year') {
    start.setDate(start.getDate() - 364);
    return start;
  }
  if (range === 'month') {
    start.setDate(start.getDate() - 29);
    return start;
  }
  start.setDate(start.getDate() - 6);
  return start;
}

function setDashboardRange(range) {
  if (activityDashboardTab.value === 'activity') {
    activityHeatmapRange.value = activityHeatmapRangeTabs.some(item => item.id === range) ? range : 'week';
    queueActivityViewportSync();
    return;
  }
  activityRange.value = activityRangeTabs.some(item => item.id === range) ? range : 'week';
}

function queueActivityViewportSync() {
  nextTick(() => {
    const element = activityViewportRef.value;
    if (!element) return;
    element.scrollLeft = Math.max(0, element.scrollWidth - element.clientWidth);
  });
}

function getRecordTimestamp(record) {
  const candidates = [
    record?.recordedAt,
    record?.createdAt,
    record?.updatedAt,
    record?.timestamp,
    record?.time,
  ];
  for (const value of candidates) {
    if (value == null || value === '') continue;
    const date = typeof value === 'number' ? new Date(value) : new Date(String(value));
    if (!Number.isNaN(date.getTime())) return date;
  }
  return null;
}

function getNiceAxisMax(value) {
  const number = Number(value || 0);
  if (!Number.isFinite(number) || number <= 0) return 4;
  const magnitude = 10 ** Math.floor(Math.log10(number));
  const normalized = number / magnitude;
  const nice = normalized <= 1 ? 1 : normalized <= 2 ? 2 : normalized <= 5 ? 5 : 10;
  return Math.max(4, nice * magnitude);
}

function buildYAxisTicks(maxValue) {
  const max = getNiceAxisMax(maxValue);
  return [0, 0.25, 0.5, 0.75, 1].map((ratio) => ({
    key: `${max}-${ratio}`,
    label: formatCompactNumber(max * ratio),
    bottom: ratio * 92,
  })).reverse();
}

function getBucketCenterPercent(index, total) {
  const count = Math.max(1, Number(total) || 1);
  return ((Number(index) + 0.5) / count) * 100;
}

function buildTokenBuckets(range) {
  const now = new Date();
  if (range === 'today') {
    return Array.from({ length: 24 }, (_, hour) => ({
      key: `${formatDateKey(now)}-${String(hour).padStart(2, '0')}`,
      label: `${String(hour).padStart(2, '0')}:00`,
      input: 0,
      output: 0,
      reasoning: 0,
    }));
  }

  const days = range === 'month' ? 30 : 7;
  return Array.from({ length: days }, (_, index) => {
    const date = startOfLocalDay(now);
    date.setDate(date.getDate() - (days - 1 - index));
    return {
      key: formatDateKey(date),
      label: range === 'month' ? `${date.getMonth() + 1}/${date.getDate()}` : ['日', '一', '二', '三', '四', '五', '六'][date.getDay()],
      input: 0,
      output: 0,
      reasoning: 0,
    };
  });
}

function resolveTokenBucketKey(date, range) {
  if (range === 'today') {
    return `${formatDateKey(date)}-${String(date.getHours()).padStart(2, '0')}`;
  }
  return formatDateKey(date);
}

function getLocalTokenTrendEntries(range) {
  const analytics = localTokenAnalytics.value;
  const series = Array.isArray(analytics?.series) ? analytics.series : [];
  if (series.length === 0 || getPositiveNumber(analytics?.totalTokens) <= 0) {
    return [];
  }
  const now = new Date();
  const todayKey = formatDateKey(now);
  const start = startOfLocalDay(now);
  if (range === 'today') {
    return series
      .filter(item => String(item?.date || '') === todayKey)
      .map(item => ({
        ...item,
        hour: String(item?.hour || '00').slice(0, 2),
      }));
  }
  const days = range === 'month' ? 30 : 7;
  start.setDate(start.getDate() - (days - 1));
  const startKey = formatDateKey(start);
  const daily = new Map();
  series.forEach((item) => {
    const dateKey = String(item?.date || '');
    if (dateKey < startKey || dateKey > todayKey) return;
    const current = daily.get(dateKey) || {
      date: dateKey,
      source: item?.source || 'codex',
      sourceLabel: item?.sourceLabel || 'Codex',
      sessionCount: 0,
      totalTokens: 0,
      inputTokens: 0,
      outputTokens: 0,
      reasoningTokens: 0,
    };
    current.sessionCount += getPositiveNumber(item?.sessionCount);
    current.totalTokens += getPositiveNumber(item?.totalTokens);
    current.inputTokens += getPositiveNumber(item?.inputTokens);
    current.outputTokens += getPositiveNumber(item?.outputTokens);
    current.reasoningTokens += getPositiveNumber(item?.reasoningTokens);
    daily.set(dateKey, current);
  });
  return [...daily.values()];
}

function getLocalSessionTrendEntries(range) {
  const analytics = localTokenAnalytics.value;
  const series = Array.isArray(analytics?.sessionSeries) ? analytics.sessionSeries : [];
  if (series.length === 0) return [];
  const now = new Date();
  const todayKey = formatDateKey(now);
  const start = startOfLocalDay(now);
  const days = range === 'month' ? 30 : 7;
  start.setDate(start.getDate() - (days - 1));
  const startKey = range === 'today' ? todayKey : formatDateKey(start);
  return series.filter((item) => {
    const dateKey = String(item?.date || '');
    return dateKey >= startKey && dateKey <= todayKey;
  });
}

function formatTokenTooltipDate(key) {
  const text = String(key || '');
  if (/^\d{4}-\d{2}-\d{2}-\d{2}$/.test(text)) {
    return `${text.slice(0, 10)} ${text.slice(11)}:00`;
  }
  return text || '-';
}

function buildSmoothLinePath(points) {
  if (!Array.isArray(points) || points.length === 0) return '';
  if (points.length === 1) {
    const point = points[0];
    return `M${point.x.toFixed(1)} ${point.y.toFixed(1)}`;
  }
  const commands = [`M${points[0].x.toFixed(1)} ${points[0].y.toFixed(1)}`];
  for (let index = 1; index < points.length; index += 1) {
    const previous = points[index - 1];
    const current = points[index];
    const controlOffset = (current.x - previous.x) * 0.5;
    const controlX1 = previous.x + controlOffset;
    const controlY1 = previous.y;
    const controlX2 = current.x - controlOffset;
    const controlY2 = current.y;
    commands.push(`C${controlX1.toFixed(1)} ${controlY1.toFixed(1)} ${controlX2.toFixed(1)} ${controlY2.toFixed(1)} ${current.x.toFixed(1)} ${current.y.toFixed(1)}`);
  }
  return commands.join(' ');
}

function showTokenTooltip(bar, event) {
  tokenTooltip.value = {
    visible: true,
    x: 0,
    y: 0,
    title: bar?.tooltipTitle || bar?.label || '-',
    period: bar?.periodLabel || '',
    cumulative: bar?.cumulativeLabel || '',
  };
  moveTokenTooltip(event);
}

function moveTokenTooltip(event) {
  if (!tokenTooltip.value.visible) return;
  const target = event?.currentTarget?.closest?.('.request-records-token-plot') || event?.currentTarget?.parentElement;
  const rect = target?.getBoundingClientRect?.();
  if (!rect) return;
  tokenTooltip.value = {
    ...tokenTooltip.value,
    x: Math.min(Math.max(120, event.clientX - rect.left + 14), Math.max(120, rect.width - 150)),
    y: Math.max(12, event.clientY - rect.top - 18),
  };
}

function hideTokenTooltip() {
  tokenTooltip.value = {
    ...tokenTooltip.value,
    visible: false,
  };
}

function selectAxisLabels(buckets) {
  if (!Array.isArray(buckets) || buckets.length === 0) return [];
  const indexes = buckets.length <= 7
    ? buckets.map((_, index) => index)
    : [0, Math.floor((buckets.length - 1) * 0.25), Math.floor((buckets.length - 1) * 0.5), Math.floor((buckets.length - 1) * 0.75), buckets.length - 1];
  return [...new Set(indexes)].map(index => ({
    key: buckets[index].key,
    label: buckets[index].label,
  }));
}

function getPositiveNumber(value) {
  const numeric = Number(value || 0);
  return Number.isFinite(numeric) && numeric > 0 ? numeric : 0;
}

function getRecordTokenUsage(record) {
  const directInput = getPositiveNumber(record?.inputTokens);
  const directOutput = getPositiveNumber(record?.outputTokens);
  if (directInput > 0 || directOutput > 0) {
    return { input: directInput, output: directOutput };
  }

  const usageSources = [
    record?.usage,
    extractUsageFromJSONText(record?.upstreamResponseRaw),
    extractUsageFromJSONText(record?.upstreamResponsePreview),
    extractUsageFromJSONText(record?.responsePreview),
  ].filter(Boolean);

  for (const usage of usageSources) {
    const input = firstPositiveNumber(
      usage?.input_tokens,
      usage?.prompt_tokens,
      usage?.inputTokens,
      usage?.promptTokens,
    );
    const output = firstPositiveNumber(
      usage?.output_tokens,
      usage?.completion_tokens,
      usage?.outputTokens,
      usage?.completionTokens,
    );
    if (input > 0 || output > 0) {
      return { input, output };
    }
  }

  return { input: 0, output: 0 };
}

function extractUsageFromJSONText(value) {
  const text = String(value || '').trim();
  if (!text || (!text.includes('"usage"') && !text.includes('usage'))) return null;
  const candidates = [text];
  const firstBrace = text.indexOf('{');
  const lastBrace = text.lastIndexOf('}');
  if (firstBrace >= 0 && lastBrace > firstBrace) {
    candidates.push(text.slice(firstBrace, lastBrace + 1));
  }
  for (const candidate of candidates) {
    try {
      const parsed = JSON.parse(candidate);
      if (parsed?.usage && typeof parsed.usage === 'object') return parsed.usage;
      if (parsed?.response?.usage && typeof parsed.response.usage === 'object') return parsed.response.usage;
    } catch {}
  }
  return null;
}

function firstPositiveNumber(...values) {
  for (const value of values) {
    const numeric = getPositiveNumber(value);
    if (numeric > 0) return numeric;
  }
  return 0;
}

function getReasoningTokens(record) {
  const usageSources = [
    record?.usage,
    extractUsageFromJSONText(record?.upstreamResponseRaw),
    extractUsageFromJSONText(record?.upstreamResponsePreview),
    extractUsageFromJSONText(record?.responsePreview),
  ].filter(Boolean);
  const candidates = [
    record?.reasoningTokens,
    record?.reasoning_tokens,
    record?.outputTokensDetails?.reasoning_tokens,
    record?.completionTokensDetails?.reasoning_tokens,
  ];
  usageSources.forEach((usage) => {
    candidates.push(
      usage?.reasoning_tokens,
      usage?.reasoningTokens,
      usage?.output_tokens_details?.reasoning_tokens,
      usage?.output_tokens_details?.reasoningTokens,
      usage?.completion_tokens_details?.reasoning_tokens,
      usage?.completion_tokens_details?.reasoningTokens,
    );
  });
  return firstPositiveNumber(...candidates);
}

function formatPercent(value, total) {
  const numeric = Number(value || 0);
  const denominator = Number(total || 0);
  if (!Number.isFinite(numeric) || !Number.isFinite(denominator) || denominator <= 0) return 0;
  return Math.round((numeric / denominator) * 100);
}

function getActivityLevel(count, maxCount) {
  if (!count || count <= 0) return 0;
  if (!maxCount || maxCount <= 1) return 1;
  const ratio = count / maxCount;
  if (ratio >= 0.8) return 4;
  if (ratio >= 0.55) return 3;
  if (ratio >= 0.3) return 2;
  return 1;
}

function countActiveDays(counts, start, end) {
  let total = 0;
  const cursor = new Date(startOfLocalDay(start));
  const finalDay = startOfLocalDay(end);
  while (cursor <= finalDay) {
    if ((counts.get(formatDateKey(cursor)) || 0) > 0) total += 1;
    cursor.setDate(cursor.getDate() + 1);
  }
  return total;
}

function formatDuration(value) {
  const numeric = Number(value);
  if (!Number.isFinite(numeric) || numeric <= 0) return '-';
  if (numeric < 1000) return `${Math.round(numeric)}ms`;
  return `${(numeric / 1000).toFixed(numeric >= 10000 ? 1 : 2)}s`;
}

function formatCompactNumber(value) {
  const numeric = Number(value);
  if (!Number.isFinite(numeric) || numeric <= 0) return '0';
  if (numeric >= 1000000000) return `${(numeric / 1000000000).toFixed(2)}B`;
  if (numeric >= 1000000) return `${(numeric / 1000000).toFixed(2)}M`;
  if (numeric >= 100000) return `${Math.round(numeric / 1000)}K`;
  if (numeric >= 10000) return `${(numeric / 1000).toFixed(1)}K`;
  if (numeric >= 1000) return `${Math.round(numeric / 100) / 10}K`;
  return String(Math.round(numeric));
}

function formatTokenValue(value) {
  if (value == null || value === '') return '-';
  const numeric = Number(value);
  if (!Number.isFinite(numeric) || numeric < 0) return '-';
  return formatCompactNumber(numeric);
}

function formatTps(value) {
  if (value == null || value === '') return '-';
  const numeric = Number(value);
  if (!Number.isFinite(numeric) || numeric <= 0) return '-';
  if (numeric >= 100) return numeric.toFixed(0);
  if (numeric >= 10) return numeric.toFixed(1);
  return numeric.toFixed(2);
}

function formatAppName(value) {
  switch (String(value || '').trim().toLowerCase()) {
    case 'claude':
      return 'Claude';
    case 'codex':
      return 'Codex';
    case 'opencode':
      return 'OpenCode';
    case 'openclaw':
      return 'OpenClaw';
    default:
      return normalizeText(value) || '-';
  }
}

function resolveSourceLabel(value) {
  switch (String(value || '').trim().toLowerCase()) {
    case 'original':
      return '原始';
    case 'fallback':
      return '回退';
    case 'preference':
      return '偏好';
    case 'direct':
      return '直连';
    default:
      return normalizeText(value) || '-';
  }
}

function resolveSourceTone(value) {
  switch (String(value || '').trim().toLowerCase()) {
    case 'fallback':
      return 'fallback';
    case 'preference':
      return 'preference';
    case 'direct':
      return 'direct';
    default:
      return 'default';
  }
}

function resolveStatusColor(statusCode) {
  const code = Number(statusCode || 0);
  if (code >= 200 && code < 300) return 'green';
  if (code >= 400 && code < 500) return 'orange';
  return 'red';
}

function handlePageChange(page) {
  const numeric = Number(page || 1);
  currentPage.value = Number.isFinite(numeric) && numeric > 0 ? numeric : 1;
}

function isAppChipHidden(appId) {
  return hiddenAppIds.value.includes(String(appId || '').trim().toLowerCase());
}

function toggleAppFilter(appId) {
  const normalized = String(appId || '').trim().toLowerCase();
  if (!normalized) return;
  if (isAppChipHidden(normalized)) {
    hiddenAppIds.value = hiddenAppIds.value.filter(id => id !== normalized);
    return;
  }
  hiddenAppIds.value = [...hiddenAppIds.value, normalized];
}

function resolveDetailText(record) {
  const delivered = normalizeText(record?.responsePreview);
  const upstream = normalizeText(record?.upstreamResponsePreview);
  const error = normalizeText(record?.errorDetail);
  return delivered || upstream || error || '请求成功';
}

function normalizeComparableUrl(value) {
  return String(value || '').trim().replace(/\/+$/, '').toLowerCase();
}

function formatRequestDebugResponse(value) {
  const text = String(value || '').trim();
  if (!text) return '(empty)';
  try {
    return JSON.stringify(JSON.parse(text), null, 2);
  } catch {
    return text;
  }
}

function resetRequestDebugState() {
  requestDebugState.value = 'idle';
  requestDebugResponse.value = '';
}

function collectRequestDebugProviders(config, appId) {
  const candidates = [];
  const seen = new Set();
  const pushProvider = (provider) => {
    const id = String(provider?.id || '').trim();
    const rowKey = String(provider?.rowKey || '').trim();
    const baseUrl = String(provider?.baseUrl || '').trim();
    const dedupeKey = `${id}::${rowKey}::${baseUrl}`;
    if (!baseUrl || seen.has(dedupeKey)) return;
    seen.add(dedupeKey);
    candidates.push(provider);
  };

  getAdvancedProxyEffectiveProviders(config, appId, { enabledOnly: false }).forEach(pushProvider);
  Object.values(config?.queues || {}).forEach((queueSection) => {
    (Array.isArray(queueSection?.providers) ? queueSection.providers : []).forEach(pushProvider);
  });
  (Array.isArray(config?.claude?.providers) ? config.claude.providers : []).forEach(pushProvider);

  return candidates;
}

function resolveRequestDebugProvider(record, config) {
  const appId = String(record?.appType || '').trim().toLowerCase() || 'claude';
  const candidates = collectRequestDebugProviders(config, appId);
  const providerId = String(record?.providerId || '').trim();
  const providerRowKey = String(record?.providerRowKey || '').trim();
  const providerName = String(record?.providerName || '').trim();
  const providerModel = String(record?.model || '').trim().toLowerCase();
  const normalizedTargetURL = normalizeComparableUrl(record?.upstreamUrl);

  return candidates.find((provider) => {
    if (!provider) return false;
    if (providerId && String(provider?.id || '').trim() === providerId) return true;
    if (providerRowKey && String(provider?.rowKey || '').trim() === providerRowKey) return true;

    const normalizedBaseURL = normalizeComparableUrl(provider?.baseUrl);
    if (normalizedBaseURL && normalizedTargetURL.startsWith(normalizedBaseURL)) {
      if (!providerModel) return true;
      return String(provider?.model || '').trim().toLowerCase() === providerModel;
    }

    if (providerName && String(provider?.name || '').trim() === providerName) {
      if (!providerModel) return true;
      return String(provider?.model || '').trim().toLowerCase() === providerModel;
    }

    return false;
  }) || null;
}

function buildRequestDebugHeaders(record, provider, payload) {
  const stream = payload?.stream === true;
  const route = String(record?.outboundRoute || '').trim().toLowerCase();
  const apiFormat = String(provider?.apiFormat || '').trim().toLowerCase();
  const headers = {
    'Content-Type': 'application/json',
    'Accept': stream ? 'text/event-stream' : 'application/json',
  };

  if (String(record?.appType || '').trim().toLowerCase() === 'claude' && (route === 'messages' || apiFormat === 'anthropic')) {
    headers['x-api-key'] = String(provider?.apiKey || '').trim();
    headers['anthropic-version'] = '2023-06-01';
    return headers;
  }

  headers.Authorization = `Bearer ${String(provider?.apiKey || '').trim()}`;
  return headers;
}

function buildRequestDebugCommand(targetURL, headers, payload) {
  const normalizedURL = String(targetURL || '').trim();
  const normalizedHeaders = headers && typeof headers === 'object' ? headers : {};
  const normalizedPayload = payload && typeof payload === 'object' ? payload : {};
  const headerText = JSON.stringify(normalizedHeaders, null, 2) || '{}';
  const payloadText = JSON.stringify(normalizedPayload, null, 2) || '{}';

  return [
    `fetch(${JSON.stringify(normalizedURL)}, {`,
    `  method: "POST",`,
    `  headers: ${headerText.replace(/\n/g, '\n  ')},`,
    `  body: JSON.stringify(${payloadText.replace(/\n/g, '\n  ')})`,
    `})`,
  ].join('\n');
}

function parseRequestDebugPayload(record) {
  const text = String(record?.requestBody || '').trim();
  if (!text) return {};
  try {
    return JSON.parse(text);
  } catch {
    return {};
  }
}

async function syncRequestDebugEditor(record) {
  resetRequestDebugState();
  if (!record) {
    requestDebugBody.value = '';
    return;
  }

  const targetURL = String(record?.upstreamUrl || '').trim();
  const payload = parseRequestDebugPayload(record);

  try {
    const config = await getAdvancedProxyConfig();
    const provider = resolveRequestDebugProvider(record, config);
    const headers = buildRequestDebugHeaders(record, provider || {}, payload);
    requestDebugBody.value = buildRequestDebugCommand(targetURL, headers, payload);
  } catch {
    requestDebugBody.value = buildRequestDebugCommand(targetURL, {
      'Content-Type': 'application/json',
      'Accept': payload?.stream === true ? 'text/event-stream' : 'application/json',
    }, payload);
  }
}

async function executeRequestDebugCommand(commandText) {
  const trimmed = String(commandText || '').trim().replace(/;+\s*$/, '');
  if (!trimmed) {
    throw new Error('empty fetch command');
  }
  const runner = new Function('fetch', `"use strict"; return (async () => { return ${trimmed}; })();`);
  return runner(fetch);
}

function isRequestDebugResponseLike(value) {
  return Boolean(value) && typeof value === 'object' && typeof value.text === 'function';
}

async function normalizeRequestDebugExecutionResult(result) {
  if (isRequestDebugResponseLike(result)) {
    return {
      ok: typeof result.ok === 'boolean' ? result.ok : true,
      text: await result.text(),
    };
  }
  if (typeof result === 'string') {
    return { ok: true, text: result };
  }
  if (result == null) {
    return { ok: true, text: '(empty)' };
  }
  if (typeof result === 'object') {
    try {
      return { ok: true, text: JSON.stringify(result, null, 2) };
    } catch {
      return { ok: true, text: String(result) };
    }
  }
  return { ok: true, text: String(result) };
}

async function handleRequestDebugTest() {
  const record = selectedRecord.value;
  if (!record) return;

  requestDebugState.value = 'loading';
  requestDebugResponse.value = '';

  try {
    const result = await executeRequestDebugCommand(requestDebugBody.value);
    const normalizedResult = await normalizeRequestDebugExecutionResult(result);
    requestDebugResponse.value = formatRequestDebugResponse(normalizedResult.text);
    requestDebugState.value = normalizedResult.ok ? 'success' : 'error';
  } catch (error) {
    requestDebugState.value = 'error';
    requestDebugResponse.value = error?.message || 'request failed';
  }
}

function summarizeDetail(record) {
  const text = resolveDetailText(record);
  return text.length > 80 ? `${text.slice(0, 80)}...` : text;
}

function summarizeInboundEndpoint(value) {
  const text = normalizeText(value);
  if (!text) return '-';
  return text
    .replace(/^https?:\/\/[^/]+/i, '')
    .replace(/^\/+/, '')
    .replace(/^advanced-proxy\//i, '');
}

function summarizeOutboundRoute(value) {
  const text = normalizeText(value);
  if (!text) return '-';
  return text.replace(/^\/+/, '');
}

function resolveRouteTraceSteps(record) {
  const rawSteps = Array.isArray(record?.routeTrace) ? record.routeTrace : [];
  const normalized = rawSteps
    .map((step) => ({
      route: normalizeText(step?.route),
      source: normalizeText(step?.source).toLowerCase(),
      status: normalizeText(step?.status).toLowerCase(),
    }))
    .filter(step => step.route);
  if (normalized.length > 0) {
    return normalized.slice(-3);
  }
  const fallbackRoute = summarizeOutboundRoute(record?.outboundRoute);
  if (!fallbackRoute || fallbackRoute === '-') {
    return [];
  }
  return [{
    route: fallbackRoute,
    source: normalizeText(record?.source).toLowerCase(),
    status: Number(record?.statusCode || 0) >= 200 && Number(record?.statusCode || 0) < 300 ? 'success' : 'failed',
  }];
}

function formatRouteTraceLabel(value) {
  switch (String(value || '').trim().toLowerCase()) {
    case 'responses':
      return 'responses';
    case 'responses_compact':
      return 'resp/compact';
    case 'chat':
      return 'chat';
    case 'messages':
      return 'messages';
    default:
      return summarizeOutboundRoute(value) || '-';
  }
}

function resolveRouteTraceSourceLabel(value) {
  switch (String(value || '').trim().toLowerCase()) {
    case 'fallback':
      return '回退';
    case 'fallback_restore':
      return '恢复';
    case 'preference':
      return '偏好';
    case 'upgrade':
      return '升级';
    case 'rectified':
      return '修正';
    default:
      return '';
  }
}

function isFinalFallbackRouteStep(record, step, index) {
  const steps = resolveRouteTraceSteps(record);
  if (index !== steps.length - 1 || step?.status !== 'success' || steps.length < 2) {
    return false;
  }
  const previousRoutes = steps.slice(0, -1).map(item => item.route);
  if (previousRoutes.some(route => route !== step.route)) {
    return true;
  }
  return ['fallback', 'fallback_restore', 'preference'].includes(String(step?.source || '').trim().toLowerCase());
}

function resolveRouteTraceLineClass(record, step, index) {
  if (isFinalFallbackRouteStep(record, step, index)) {
    return 'is-fallback-final';
  }
  if (step?.status === 'failed') {
    return 'is-failed';
  }
  return 'is-direct';
}

function resolveRouteTraceIcon(record, step, index) {
  if (isFinalFallbackRouteStep(record, step, index)) {
    return '●';
  }
  if (step?.status === 'failed') {
    return '×';
  }
  return '○';
}

function summarizeUpstreamTarget(rawUrl, rawPath) {
  const host = extractHost(rawUrl);
  const path = normalizeText(rawPath);
  if (host === '-' && !path) return '-';
  if (host === '-') return path;
  if (!path) return host;
  return `${host}${path}`;
}

function extractHost(value) {
  const text = normalizeText(value);
  if (!text) return '-';
  try {
    return new URL(text).host || text;
  } catch {
    return text.replace(/^https?:\/\//i, '').split('/')[0] || text;
  }
}

function normalizePanel(value) {
  const normalized = String(value || '').trim().toLowerCase();
  if (['sessions', 'mcp', 'skills'].includes(normalized)) return normalized;
  return 'records';
}

function setActivePanel(panel) {
  const nextPanel = normalizePanel(panel);
  if (activePanel.value === nextPanel) return;
  activePanel.value = nextPanel;
  if (!props.open) return;
  if (nextPanel === 'sessions') {
    void refreshTerminalSessions();
  } else if (nextPanel === 'records') {
    void refreshRecords();
  } else {
    void refreshMCPSkillConfig();
  }
}

function normalizeManagedApps(apps = {}) {
  return {
    claude: Boolean(apps?.claude || apps?.claudeDesktop),
    claudeDesktop: Boolean(apps?.claudeDesktop),
    codex: Boolean(apps?.codex),
    gemini: Boolean(apps?.gemini),
    opencode: Boolean(apps?.opencode),
    openclaw: Boolean(apps?.openclaw),
  };
}

function normalizeManagedMCPServer(server = {}) {
  const id = normalizeText(server?.id);
  return {
    ...server,
    id,
    name: normalizeText(server?.name) || id,
    type: normalizeText(server?.type) || 'stdio',
    command: normalizeText(server?.command),
    url: normalizeText(server?.url),
    source: normalizeText(server?.source),
    args: Array.isArray(server?.args) ? server.args : [],
    env: server?.env && typeof server.env === 'object' ? server.env : {},
    raw: server?.raw && typeof server.raw === 'object' ? server.raw : {},
    apps: normalizeManagedApps(server?.apps),
  };
}

function normalizeManagedSkill(skill = {}) {
  const id = normalizeText(skill?.id);
  return {
    ...skill,
    id,
    name: normalizeText(skill?.name) || id,
    description: normalizeText(skill?.description),
    directory: normalizeText(skill?.directory),
    readmePath: normalizeText(skill?.readmePath),
    source: normalizeText(skill?.source),
    apps: normalizeManagedApps(skill?.apps),
  };
}

async function refreshMCPSkillConfig() {
  if (!mcpSkillBridgeAvailable) return;
  mcpSkillLoading.value = true;
  try {
    const snapshot = await getMCPSkillConfigSnapshot();
    mcpSkillConfigPath.value = normalizeText(snapshot?.configPath);
    managedMCPServers.value = (Array.isArray(snapshot?.mcp) ? snapshot.mcp : [])
      .map(normalizeManagedMCPServer)
      .filter(item => item.id);
    managedSkills.value = (Array.isArray(snapshot?.skills) ? snapshot.skills : [])
      .map(normalizeManagedSkill)
      .filter(item => item.id);
  } catch (error) {
    message.error(error?.message || '读取 MCP / Skill 配置失败');
  } finally {
    mcpSkillLoading.value = false;
  }
}

async function saveMCPSkillConfig(options = {}) {
  if (!mcpSkillBridgeAvailable) return;
  mcpSkillSaving.value = true;
  try {
    await saveMCPSkillConfigSnapshot({
      configPath: mcpSkillConfigPath.value,
      mcp: managedMCPServers.value.map(normalizeManagedMCPServer),
      skills: managedSkills.value.map(normalizeManagedSkill),
    });
    if (!options?.silent) {
      message.success('MCP / Skill 配置已保存');
    }
  } catch (error) {
    message.error(error?.message || '保存 MCP / Skill 配置失败');
  } finally {
    mcpSkillSaving.value = false;
  }
}

function formatMCPServerSummary(server) {
  const type = normalizeText(server?.type) || 'stdio';
  const target = normalizeText(server?.url) || normalizeText(server?.command);
  if (target) return `${type} · ${target}`;
  const argText = Array.isArray(server?.args) && server.args.length ? server.args.join(' ') : '';
  return argText ? `${type} · ${argText}` : type;
}

function isManagedAppEnabled(apps, appId) {
  return Boolean(normalizeManagedApps(apps)[appId]);
}

function setManagedAppEnabled(apps, appId, enabled) {
  return {
    ...normalizeManagedApps(apps),
    [appId]: Boolean(enabled),
  };
}

function setSelectedManagedApp(appId) {
  const normalized = String(appId || '').trim();
  if (!managedAppItems.some(item => item.id === normalized)) return;
  selectedManagedAppId.value = normalized;
}

function getManagedAppIcon(appId) {
  const id = String(appId || '').trim();
  if (id === 'claude' || id === 'claudeDesktop') return claudeAppIcon;
  if (id === 'gemini') return geminiAppIcon;
  if (id === 'opencode') return opencodeAppIcon;
  if (id === 'openclaw') return openclawAppIcon;
  return codexAppIcon;
}

async function persistMCPSkillConfigChange(successText) {
  await saveMCPSkillConfig({ silent: true });
  if (successText) message.success(successText);
  await refreshMCPSkillConfig();
}

async function setManagedMCPApp(serverId, appId, enabled) {
  const id = normalizeText(serverId);
  managedMCPServers.value = managedMCPServers.value.map((server) => {
    if (normalizeText(server?.id) !== id) return server;
    return normalizeManagedMCPServer({
      ...server,
      apps: setManagedAppEnabled(server.apps, appId, enabled),
    });
  });
  await persistMCPSkillConfigChange(enabled ? '已应用 MCP' : '已禁用 MCP');
}

async function setManagedSkillApp(skillId, appId, enabled) {
  const id = normalizeText(skillId);
  managedSkills.value = managedSkills.value.map((skill) => {
    if (normalizeText(skill?.id) !== id) return skill;
    return normalizeManagedSkill({
      ...skill,
      apps: setManagedAppEnabled(skill.apps, appId, enabled),
    });
  });
  await persistMCPSkillConfigChange(enabled ? '已应用 Skill' : '已禁用 Skill');
}

function applyManagedMCPToSelectedApp(serverId) {
  void setManagedMCPApp(serverId, selectedManagedAppId.value, true);
}

function disableManagedMCPForSelectedApp(serverId) {
  void setManagedMCPApp(serverId, selectedManagedAppId.value, false);
}

function removeManagedMCPFromSelectedApp(serverId) {
  const id = normalizeText(serverId);
  managedMCPServers.value = managedMCPServers.value.filter(server => normalizeText(server?.id) !== id);
  void persistMCPSkillConfigChange('已删除 MCP');
}

function applyManagedSkillToSelectedApp(skillId) {
  void setManagedSkillApp(skillId, selectedManagedAppId.value, true);
}

function disableManagedSkillForSelectedApp(skillId) {
  void setManagedSkillApp(skillId, selectedManagedAppId.value, false);
}

function removeManagedSkillFromSelectedApp(skillId) {
  const id = normalizeText(skillId);
  managedSkills.value = managedSkills.value.filter(skill => normalizeText(skill?.id) !== id);
  void persistMCPSkillConfigChange('已删除 Skill');
}

function getTerminalSessionKey(session) {
  return [
    String(session?.providerId || terminalProviderId.value || '').trim(),
    String(session?.sessionId || '').trim(),
    String(session?.sourcePath || '').trim(),
  ].join('::');
}

function formatTerminalSessionTitle(session) {
  return normalizeText(session?.title) || normalizeText(session?.sessionId) || '未命名会话';
}

function formatTerminalSessionTime(session) {
  const timestamp = Number(session?.lastActiveAt || session?.createdAt || 0);
  if (!Number.isFinite(timestamp) || timestamp <= 0) return '时间未知';
  const date = new Date(timestamp);
  if (Number.isNaN(date.getTime())) return '时间未知';
  return date.toLocaleString('zh-CN', {
    hour12: false,
    month: '2-digit',
    day: '2-digit',
    hour: '2-digit',
    minute: '2-digit',
  });
}

function compactTerminalSessionPath(value) {
  const text = normalizeText(value);
  if (!text) return '路径未知';
  const normalized = text.replace(/\\/g, '/');
  const parts = normalized.split('/').filter(Boolean);
  if (parts.length <= 2) return text;
  return `.../${parts.slice(-2).join('/')}`;
}

function getTerminalProviderIcon(providerId) {
  const id = String(providerId || '').trim().toLowerCase();
  return TERMINAL_PROVIDER_ICONS[id] || codexAppIcon;
}

function clearSelectedTerminalSession() {
  terminalSessionMessageRequestToken += 1;
  selectedTerminalSession.value = null;
  terminalSessionMessages.value = [];
  expandedTerminalMessageIndexes.value = [];
  terminalSessionMessagesLoading.value = false;
}

async function refreshTerminalSessions() {
  if (!terminalBridgeAvailable) return;
  sessionsLoading.value = true;
  try {
    const page = await listTerminalSessions(terminalProviderId.value, terminalSessionPage.value, TERMINAL_SESSION_PAGE_SIZE);
    const normalizedProvider = String(page?.providerId || terminalProviderId.value || 'codex').trim().toLowerCase();
    terminalProviderId.value = normalizedProvider || 'codex';
    terminalSessionPage.value = Math.max(1, Number(page?.page || terminalSessionPage.value || 1));
    terminalSessionTotal.value = Math.max(0, Number(page?.total || 0));
    terminalSessions.value = Array.isArray(page?.sessions) ? page.sessions : [];
    terminalProviders.value = Array.isArray(page?.providers) ? page.providers : [];
    if (selectedTerminalSession.value) {
      const currentKey = selectedTerminalSessionKey.value;
      const matched = terminalSessions.value.find(session => getTerminalSessionKey(session) === currentKey);
      if (matched) {
        selectedTerminalSession.value = matched;
      } else {
        clearSelectedTerminalSession();
      }
    }
  } catch (error) {
    message.error(error?.message || '读取终端会话失败');
  } finally {
    sessionsLoading.value = false;
  }
}

function switchTerminalProvider(providerId) {
  const normalized = String(providerId || '').trim().toLowerCase();
  if (!normalized || normalized === terminalProviderId.value) return;
  terminalProviderId.value = normalized;
  terminalSessionPage.value = 1;
  clearSelectedTerminalSession();
  void refreshTerminalSessions();
}

function handleTerminalSessionPageChange(page) {
  const numeric = Number(page || 1);
  terminalSessionPage.value = Number.isFinite(numeric) && numeric > 0 ? numeric : 1;
  clearSelectedTerminalSession();
  void refreshTerminalSessions();
}

async function selectTerminalSession(session) {
  if (!session) return;
  const nextKey = getTerminalSessionKey(session);
  if (selectedTerminalSessionKey.value === nextKey && terminalSessionMessages.value.length > 0) {
    return;
  }
  selectedTerminalSession.value = session;
  terminalSessionMessages.value = [];
  expandedTerminalMessageIndexes.value = [];
  await loadSelectedTerminalSessionMessages(session, nextKey);
}

async function loadSelectedTerminalSessionMessages(session, expectedKey) {
  const sourcePath = normalizeText(session?.sourcePath);
  if (!sourcePath) {
    terminalSessionMessages.value = [];
    return;
  }
  const requestToken = terminalSessionMessageRequestToken + 1;
  terminalSessionMessageRequestToken = requestToken;
  terminalSessionMessagesLoading.value = true;
  try {
    const list = await getTerminalSessionMessages(
      normalizeText(session?.providerId) || terminalProviderId.value,
      sourcePath,
      TERMINAL_SESSION_MESSAGE_LIMIT,
    );
    if (requestToken !== terminalSessionMessageRequestToken || selectedTerminalSessionKey.value !== expectedKey) {
      return;
    }
    terminalSessionMessages.value = Array.isArray(list) ? list : [];
    expandedTerminalMessageIndexes.value = [];
  } catch (error) {
    if (requestToken === terminalSessionMessageRequestToken) {
      terminalSessionMessages.value = [];
      message.error(error?.message || '读取会话聊天记录失败');
    }
  } finally {
    if (requestToken === terminalSessionMessageRequestToken) {
      terminalSessionMessagesLoading.value = false;
    }
  }
}

function formatTerminalMessageRole(role) {
  const normalized = String(role || '').trim().toLowerCase();
  if (normalized === 'assistant') return 'Assistant';
  if (normalized === 'user') return 'User';
  if (normalized === 'tool') return 'Tool';
  if (normalized === 'system') return 'System';
  return normalized || 'Message';
}

function getTerminalMessageRoleClass(role) {
  const normalized = String(role || '').trim().toLowerCase();
  if (['assistant', 'user', 'tool', 'system'].includes(normalized)) return normalized;
  return 'unknown';
}

function formatTerminalMessageTime(timestamp) {
  const numeric = Number(timestamp || 0);
  if (!Number.isFinite(numeric) || numeric <= 0) return '';
  const date = new Date(numeric);
  if (Number.isNaN(date.getTime())) return '';
  return date.toLocaleString('zh-CN', {
    hour12: false,
    month: '2-digit',
    day: '2-digit',
    hour: '2-digit',
    minute: '2-digit',
  });
}

function getTerminalMessageLineCount(message) {
  const text = String(message?.content || '');
  if (!text) return 0;
  return text.split(/\r\n|\r|\n/).reduce((sum, line) => {
    const visualLines = Math.max(1, Math.ceil(Array.from(line || '').length / 88));
    return sum + visualLines;
  }, 0);
}

function isTerminalMessageCollapsible(message) {
  return getTerminalMessageLineCount(message) > TERMINAL_SESSION_COLLAPSE_LINE_LIMIT;
}

function isTerminalMessageExpanded(index) {
  return expandedTerminalMessageIndexes.value.includes(index);
}

function isTerminalMessageCollapsed(message, index) {
  return isTerminalMessageCollapsible(message) && !isTerminalMessageExpanded(index);
}

function toggleTerminalMessageExpanded(index) {
  const current = new Set(expandedTerminalMessageIndexes.value);
  if (current.has(index)) {
    current.delete(index);
  } else {
    current.add(index);
  }
  expandedTerminalMessageIndexes.value = Array.from(current).sort((left, right) => left - right);
}

async function launchSessionTerminal(session) {
  const command = normalizeText(session?.resumeCommand);
  if (!command) {
    message.warning('该会话没有可恢复命令');
    return;
  }
  const sessionKey = getTerminalSessionKey(session);
  launchingSessionKey.value = sessionKey;
  try {
    await launchTerminalSession(command, normalizeText(session?.projectDir));
    message.success('终端已打开');
  } catch (error) {
    message.error(error?.message || '打开终端失败');
  } finally {
    if (launchingSessionKey.value === sessionKey) {
      launchingSessionKey.value = '';
    }
  }
}

async function refreshRecords() {
  if (!bridgeAvailable) return;
  loading.value = true;
  try {
    const [nextRecords] = await Promise.all([
      listAdvancedProxyRequestRecords(400),
      refreshLocalTokenAnalytics(),
    ]);
    records.value = nextRecords;
    await focusRequestedRecord();
    await nextTick();
    queueTableMetricsSync();
  } catch (error) {
    message.error(error?.message || '读取高级代理请求记录失败');
  } finally {
    loading.value = false;
  }
}

async function refreshLocalTokenAnalytics() {
  tokenAnalyticsLoading.value = true;
  try {
    localTokenAnalytics.value = await getLocalTokenUsageAnalytics();
  } catch {
    localTokenAnalytics.value = null;
  } finally {
    tokenAnalyticsLoading.value = false;
  }
}

function handleClose() {
  emit('update:open', false);
}

function closeRecordDetail() {
  recordDetailRequestToken += 1;
  detailOpen.value = false;
  recordDetailLoading.value = false;
}

function openRecordDetail(record) {
  const requestToken = recordDetailRequestToken + 1;
  recordDetailRequestToken = requestToken;
  selectedRecord.value = record || null;
  detailOpen.value = true;
  recordDetailLoading.value = true;
  void (async () => {
    try {
      const detail = await getAdvancedProxyRequestRecord(record?.id || record?.recordedAt);
      if (recordDetailRequestToken !== requestToken) return;
      selectedRecord.value = detail || record || null;
      await syncRequestDebugEditor(detail || record);
    } catch {
      if (recordDetailRequestToken !== requestToken) return;
      await syncRequestDebugEditor(record);
    } finally {
      if (recordDetailRequestToken === requestToken) {
        recordDetailLoading.value = false;
      }
    }
  })();
}

async function focusRequestedRecord() {
  const focusId = String(props.focusRecordId || '').trim();
  if (!focusId) return;
  const record = records.value.find(item => String(item?.id || '') === focusId);
  if (record) {
    openRecordDetail(record);
    return;
  }
  try {
    const detail = await getAdvancedProxyRequestRecord(focusId);
    if (detail) {
      openRecordDetail(detail);
    }
  } catch {
    // The parent already owns the user-facing "not found" path.
  }
}

function handleClear() {
  Modal.confirm({
    title: '清空请求记录？',
    content: '仅清掉本地高级代理请求记录缓存，不影响运行配置。',
    okText: '清空',
    okButtonProps: { danger: true },
    cancelText: '取消',
    async onOk() {
      try {
        await clearAdvancedProxyRequestRecords();
        records.value = [];
        hiddenAppIds.value = [];
        currentPage.value = 1;
        detailOpen.value = false;
        selectedRecord.value = null;
        void syncRequestDebugEditor(null);
        message.success('请求记录已清空');
      } catch (error) {
        message.error(error?.message || '清空请求记录失败');
      }
    },
  });
}

watch(
  () => props.open,
  async (nextOpen) => {
    if (nextOpen) {
      syncViewport();
      activePanel.value = props.focusRecordId ? 'records' : normalizePanel(props.initialPanel);
      if (activePanel.value === 'sessions') {
        await refreshTerminalSessions();
      } else if (activePanel.value === 'records') {
        await refreshRecords();
      } else {
        await refreshMCPSkillConfig();
      }
      await nextTick();
      attachTableResizeObserver();
      queueTableMetricsSync();
      return;
    }
    closeRecordDetail();
    selectedRecord.value = null;
    void syncRequestDebugEditor(null);
    clearTableDragSession();
    detachTableResizeObserver();
  },
  { immediate: true },
);

watch(
  () => props.focusRecordId,
  async () => {
    if (!props.open) return;
    if (props.focusRecordId) {
      activePanel.value = 'records';
    }
    await focusRequestedRecord();
  },
);

watch(
  () => [activityDashboardTab.value, activityHeatmapRange.value, activityTrend.value.columnCount],
  () => {
    if (activityDashboardTab.value === 'activity') {
      queueActivityViewportSync();
    }
  },
  { flush: 'post' },
);

watch(
  () => props.initialPanel,
  async (panel) => {
    if (!props.open || props.focusRecordId) return;
    const normalized = normalizePanel(panel);
    activePanel.value = normalized;
    if (normalized === 'sessions') {
      await refreshTerminalSessions();
    } else if (normalized === 'records') {
      await refreshRecords();
    } else {
      await refreshMCPSkillConfig();
    }
  },
);

watch(selectedRecord, (record) => {
  void syncRequestDebugEditor(record);
});

watch(appSummaryItems, (items) => {
  const validIds = new Set(items.map(item => item.id));
  hiddenAppIds.value = hiddenAppIds.value.filter(id => validIds.has(id));
}, { immediate: true });

watch(filteredRecords, (list) => {
  const totalPages = Math.max(1, Math.ceil(list.length / REQUEST_RECORD_PAGE_SIZE));
  if (currentPage.value > totalPages) {
    currentPage.value = totalPages;
  }
  if (currentPage.value < 1) {
    currentPage.value = 1;
  }
}, { immediate: true });

watch(
  () => [
    props.open,
    currentPage.value,
    filteredRecords.value.length,
    columns.value.map(column => `${column.key}:${column.width}`).join('|'),
    tableScrollY.value,
  ],
  async ([nextOpen]) => {
    if (!nextOpen) return;
    await nextTick();
    attachTableResizeObserver();
    queueTableMetricsSync();
  },
  { flush: 'post' },
);

onMounted(() => {
  syncViewport();
  nextTick(() => {
    attachTableResizeObserver();
    queueTableMetricsSync();
  });
  window.addEventListener('resize', syncViewport);
});

onBeforeUnmount(() => {
  clearTableDragSession();
  detachTableResizeObserver();
  if (tableMetricsFrame) {
    window.cancelAnimationFrame(tableMetricsFrame);
    tableMetricsFrame = 0;
  }
  window.removeEventListener('resize', syncViewport);
});
</script>

<style scoped>
.advanced-proxy-records-drawer :deep(.ant-drawer-header),
.advanced-proxy-records-detail-drawer :deep(.ant-drawer-header) {
  padding: 12px 14px 10px;
  border-bottom: 1px solid rgba(102, 122, 108, 0.12);
  background:
    linear-gradient(180deg, rgba(252, 251, 248, 0.98), rgba(246, 248, 244, 0.94)),
    rgba(255, 255, 255, 0.96);
}

.advanced-proxy-records-drawer :deep(.ant-drawer-title),
.advanced-proxy-records-detail-drawer :deep(.ant-drawer-title) {
  color: #223128;
  font-size: 14px;
  font-weight: 700;
  letter-spacing: 0.01em;
}

.advanced-proxy-records-drawer :deep(.ant-drawer-content-wrapper),
.advanced-proxy-records-drawer :deep(.ant-drawer-content),
.advanced-proxy-records-drawer :deep(.ant-drawer-wrapper-body),
.advanced-proxy-records-drawer :deep(.ant-drawer-body) {
  scrollbar-width: none;
  -ms-overflow-style: none;
}

.advanced-proxy-records-drawer :deep(.ant-drawer-content-wrapper::-webkit-scrollbar),
.advanced-proxy-records-drawer :deep(.ant-drawer-content::-webkit-scrollbar),
.advanced-proxy-records-drawer :deep(.ant-drawer-wrapper-body::-webkit-scrollbar),
.advanced-proxy-records-drawer :deep(.ant-drawer-body::-webkit-scrollbar) {
  width: 0;
  height: 0;
  display: none;
}

.advanced-proxy-records-drawer :deep(.ant-drawer-wrapper-body) {
  overflow: hidden;
}

.advanced-proxy-records-drawer :deep(.ant-drawer-body) {
  display: flex;
  flex-direction: column;
  min-height: 0;
  padding: 12px 14px;
  overflow: hidden;
}

.advanced-proxy-records-detail-drawer :deep(.ant-drawer-body) {
  padding: 12px 14px;
  overflow-x: hidden;
  overflow-y: auto;
  -ms-overflow-style: none;
  scrollbar-width: none;
}

.advanced-proxy-records-detail-drawer :deep(.ant-drawer-body::-webkit-scrollbar) {
  width: 0;
  height: 0;
  display: none;
}

.advanced-proxy-records-drawer-dark :deep(.ant-drawer-header),
.advanced-proxy-records-detail-drawer-dark :deep(.ant-drawer-header) {
  border-bottom-color: rgba(133, 162, 145, 0.16);
  background:
    linear-gradient(180deg, rgba(22, 30, 26, 0.98), rgba(17, 24, 20, 0.96)),
    rgba(17, 24, 20, 0.96);
}

.advanced-proxy-records-drawer-dark :deep(.ant-drawer-title),
.advanced-proxy-records-detail-drawer-dark :deep(.ant-drawer-title) {
  color: #edf6ee;
}

.request-records-scroll-shell {
  flex: 1 1 auto;
  min-height: 0;
  min-width: 0;
  width: 100%;
  overflow-x: hidden;
  overflow-y: auto;
  -ms-overflow-style: none;
  scrollbar-width: none;
}

.request-records-scroll-shell::-webkit-scrollbar {
  width: 0;
  height: 0;
  display: none;
}

.request-records-shell {
  display: grid;
  align-content: start;
  gap: 10px;
  min-height: max-content;
  min-width: 0;
  width: 100%;
  padding-bottom: 6px;
}

.request-records-mode-tabs {
  display: inline-flex;
  align-items: center;
  justify-self: start;
  gap: 3px;
  padding: 3px;
  border: 1px solid rgba(100, 124, 92, 0.14);
  border-radius: 999px;
  background: rgba(255, 255, 255, 0.58);
  box-shadow: inset 0 1px 0 rgba(255, 255, 255, 0.7);
}

.request-records-mode-tab {
  height: 28px;
  padding: 0 11px;
  border: 0;
  border-radius: 999px;
  background: transparent;
  color: #66745f;
  display: inline-flex;
  align-items: center;
  justify-content: center;
  gap: 5px;
  font-size: 12px;
  font-weight: 800;
  line-height: 1;
  cursor: pointer;
  transition: background 0.18s ease, color 0.18s ease, box-shadow 0.18s ease;
}

.request-records-mode-tab.is-active {
  color: #273b29;
  background: linear-gradient(180deg, rgba(245, 251, 238, 0.98), rgba(225, 239, 207, 0.9));
  box-shadow: 0 8px 18px rgba(83, 112, 63, 0.12);
}

.request-records-toolbar {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 10px;
  min-width: 0;
}

.request-records-toolbar-meta,
.request-records-toolbar-actions,
.request-records-board-chips,
.request-record-detail-hero-tags {
  display: flex;
  align-items: center;
  gap: 6px;
  flex-wrap: wrap;
}

.request-records-toolbar-meta {
  min-width: 0;
  flex: 1 1 auto;
}

.request-records-toolbar-pill,
.request-records-board-chip {
  display: inline-flex;
  align-items: center;
  gap: 6px;
  min-height: 26px;
  padding: 0 10px;
  border-radius: 999px;
  border: 1px solid rgba(110, 133, 118, 0.12);
  background: rgba(255, 255, 255, 0.78);
  color: #536257;
  font-size: 11px;
  font-weight: 600;
  line-height: 1;
  white-space: nowrap;
}

.request-records-board-chip-toggle {
  appearance: none;
  -webkit-appearance: none;
  font: inherit;
  cursor: pointer;
  transition:
    background-color 0.18s ease,
    border-color 0.18s ease,
    color 0.18s ease,
    opacity 0.18s ease;
}

.request-records-board-chip-toggle.is-inactive {
  background: rgba(246, 248, 244, 0.92);
  border-color: rgba(110, 133, 118, 0.08);
  color: #8a988d;
  opacity: 0.72;
}

.request-records-toolbar-pill-muted,
.request-records-board-chip-muted {
  background: rgba(246, 248, 244, 0.92);
  color: #69786d;
}

.request-records-toolbar-dot {
  width: 7px;
  height: 7px;
  border-radius: 999px;
  background: #4e7a45;
  box-shadow: 0 0 0 4px rgba(88, 126, 79, 0.12);
}

.request-records-toolbar-pill.is-loading .request-records-toolbar-dot {
  background: #1677ff;
  box-shadow: 0 0 0 4px rgba(22, 119, 255, 0.12);
  animation: request-records-pulse 1.2s ease-in-out infinite;
}

.request-records-action-button {
  height: 30px;
  padding: 0 12px;
  border-radius: 12px;
  font-size: 12px;
  font-weight: 700;
  box-shadow: 0 6px 16px rgba(72, 95, 81, 0.08);
}

.request-records-action-button-refresh {
  border-color: rgba(111, 143, 121, 0.18);
  background: linear-gradient(180deg, rgba(255, 255, 255, 0.98), rgba(241, 246, 238, 0.94));
  color: #29422f;
}

.request-records-action-button-clear {
  border-color: rgba(191, 106, 98, 0.2);
  background: linear-gradient(180deg, rgba(255, 255, 255, 0.98), rgba(251, 241, 238, 0.94));
  color: #a15147;
}

.mcp-skill-toolbar {
  align-items: flex-start;
}

.mcp-skill-app-tabs {
  display: inline-flex;
  align-items: center;
  gap: 4px;
  padding: 3px;
  border: 1px solid rgba(100, 124, 92, 0.14);
  border-radius: 999px;
  background: rgba(255, 255, 255, 0.62);
}

.mcp-skill-app-tab {
  width: 30px;
  height: 30px;
  border: 0;
  border-radius: 999px;
  background: transparent;
  display: inline-flex;
  align-items: center;
  justify-content: center;
  cursor: pointer;
  transition: background 0.16s ease, box-shadow 0.16s ease, opacity 0.16s ease;
}

.mcp-skill-app-tab img {
  width: 17px;
  height: 17px;
  object-fit: contain;
  opacity: 0.68;
}

.mcp-skill-app-tab.is-active {
  background: linear-gradient(180deg, rgba(236, 249, 228, 0.98), rgba(211, 232, 195, 0.94));
  box-shadow: 0 8px 16px rgba(77, 116, 62, 0.12);
}

.mcp-skill-app-tab.is-active img {
  opacity: 1;
}

.mcp-skill-board {
  display: grid;
  gap: 8px;
  min-width: 0;
  width: 100%;
}

.mcp-skill-card {
  display: grid;
  grid-template-columns: minmax(0, 1fr) auto;
  align-items: center;
  gap: 10px;
  min-width: 0;
  padding: 10px 11px;
  border-radius: 14px;
  border: 1px solid rgba(103, 126, 111, 0.12);
  background:
    linear-gradient(180deg, rgba(255, 255, 255, 0.96), rgba(247, 250, 245, 0.94)),
    rgba(255, 255, 255, 0.92);
  box-shadow: 0 12px 24px rgba(87, 105, 92, 0.05);
}

.mcp-skill-card.is-disabled-source {
  opacity: 0.82;
}

.mcp-skill-subsection-title {
  margin: 4px 2px 0;
  color: #65756a;
  font-size: 12px;
  font-weight: 800;
}

.mcp-skill-card-main {
  display: grid;
  gap: 3px;
  min-width: 0;
}

.mcp-skill-card-main strong,
.mcp-skill-card-main span,
.mcp-skill-card-main small {
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.mcp-skill-card-main strong {
  color: #223128;
  font-size: 13px;
  font-weight: 800;
}

.mcp-skill-card-main span {
  color: #516258;
  font-size: 12px;
  line-height: 1.3;
}

.mcp-skill-card-main small {
  color: #7b897f;
  font-size: 11px;
  line-height: 1.25;
}

.mcp-skill-card-actions {
  display: inline-flex;
  align-items: center;
  justify-content: flex-end;
  gap: 6px;
}

.mcp-skill-state-pill {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  height: 26px;
  min-width: 48px;
  padding: 0 9px;
  border: 1px solid rgba(111, 143, 121, 0.16);
  border-radius: 999px;
  background: rgba(246, 248, 244, 0.86);
  color: #7a887f;
  font-size: 11px;
  font-weight: 800;
  line-height: 1;
}

.mcp-skill-state-pill.is-on {
  border-color: rgba(72, 122, 67, 0.22);
  background: rgba(230, 244, 218, 0.9);
  color: #294c2d;
}

.mcp-skill-card-button {
  height: 28px;
  padding: 0 10px;
  border-radius: 10px;
  font-size: 12px;
  font-weight: 800;
}

.mcp-skill-card-button-import {
  border-color: rgba(72, 122, 67, 0.22);
  background: linear-gradient(180deg, rgba(250, 253, 247, 0.98), rgba(232, 244, 222, 0.94));
  color: #294c2d;
}

.mcp-skill-card-button-disable {
  border-color: rgba(132, 142, 128, 0.2);
  background: linear-gradient(180deg, rgba(255, 255, 255, 0.98), rgba(244, 246, 242, 0.94));
  color: #59695f;
}

.mcp-skill-app-toggle {
  width: 30px;
  height: 26px;
  border: 1px solid rgba(111, 143, 121, 0.16);
  border-radius: 9px;
  background: rgba(246, 248, 244, 0.86);
  color: #7a887f;
  font-size: 10px;
  font-weight: 800;
  line-height: 1;
  cursor: pointer;
  transition:
    background-color 0.16s ease,
    border-color 0.16s ease,
    color 0.16s ease,
    box-shadow 0.16s ease;
}

.mcp-skill-app-toggle.is-on {
  border-color: rgba(72, 122, 67, 0.28);
  background: linear-gradient(180deg, rgba(236, 249, 228, 0.98), rgba(211, 232, 195, 0.94));
  color: #294c2d;
  box-shadow: 0 8px 16px rgba(77, 116, 62, 0.12);
}

.mcp-skill-empty {
  min-height: 140px;
}

.request-records-activity-card {
  display: grid;
  gap: 14px;
  min-width: 0;
  overflow: hidden;
  padding: 14px;
  border-radius: 22px;
  border: 1px solid rgba(109, 126, 116, 0.12);
  background:
    linear-gradient(180deg, rgba(255, 255, 255, 0.98), rgba(247, 250, 245, 0.94)),
    rgba(255, 255, 255, 0.94);
  box-shadow:
    0 18px 36px rgba(86, 102, 92, 0.07),
    inset 0 1px 0 rgba(255, 255, 255, 0.82);
}

.request-records-activity-head {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 10px;
  min-width: 0;
  overflow: hidden;
}

.request-records-activity-tabs,
.request-records-activity-range {
  display: inline-flex;
  align-items: center;
  min-width: 0;
  max-width: 100%;
  gap: 3px;
  padding: 3px;
  border-radius: 999px;
  background: rgba(237, 238, 238, 0.82);
}

.request-records-activity-tabs {
  overflow-x: auto;
  scrollbar-width: none;
}

.request-records-activity-tabs::-webkit-scrollbar {
  display: none;
}

.request-records-activity-range {
  flex: 0 0 auto;
}

.request-records-activity-tab,
.request-records-activity-range-button {
  height: 30px;
  padding: 0 14px;
  border: 0;
  border-radius: 999px;
  background: transparent;
  color: #5f6461;
  font: inherit;
  font-size: 13px;
  font-weight: 700;
  cursor: pointer;
  white-space: nowrap;
  transition:
    background 0.16s ease,
    color 0.16s ease,
    box-shadow 0.16s ease;
}

.request-records-activity-tab.is-active,
.request-records-activity-range-button.is-active {
  color: #101513;
  background: rgba(255, 255, 255, 0.96);
  box-shadow: 0 8px 18px rgba(66, 75, 70, 0.12);
}

.request-records-activity-title {
  color: #101513;
  font-size: 17px;
  font-weight: 850;
}

.request-records-token-dashboard {
  display: grid;
  grid-template-columns: minmax(0, 1fr) 190px;
  gap: 18px;
  align-items: stretch;
  min-height: 268px;
}

.request-records-token-chart {
  display: grid;
  grid-template-rows: auto minmax(0, 1fr) auto;
  gap: 12px;
  min-width: 0;
  padding: 14px;
  border-radius: 18px;
  border: 1px solid rgba(104, 117, 109, 0.12);
  background: rgba(252, 253, 251, 0.78);
}

.request-records-token-legend {
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  gap: 12px;
  color: #68746c;
  font-size: 12px;
  font-weight: 700;
}

.request-records-token-legend span,
.request-records-token-breakdown span {
  display: inline-flex;
  align-items: center;
  gap: 7px;
  min-width: 0;
}

.request-records-token-legend .request-records-token-source {
  min-height: 18px;
  padding: 2px 8px;
  border: 1px solid rgba(117, 140, 126, 0.24);
  border-radius: 999px;
  background: rgba(238, 242, 239, 0.78);
  color: #53685c;
  font-size: 11px;
  line-height: 1;
}

.request-records-token-legend i,
.request-records-token-breakdown i {
  width: 9px;
  height: 9px;
  flex: 0 0 auto;
  border-radius: 999px;
}

.request-records-token-legend .is-window {
  background: #40c463;
}

.request-records-token-legend .is-total {
  background: #ffd06a;
}

.request-records-token-plot {
  position: relative;
  min-height: 190px;
  overflow: hidden;
  border-radius: 14px;
  background:
    linear-gradient(180deg, rgba(248, 251, 247, 0.92), rgba(241, 246, 239, 0.72)),
    rgba(247, 249, 246, 0.86);
}

.request-records-token-svg {
  position: absolute;
  inset: 0;
  width: 100%;
  height: 100%;
  overflow: visible;
}

.request-records-token-grid-line {
  fill: none;
  stroke: rgba(111, 126, 117, 0.16);
  stroke-width: 1;
}

.request-records-token-line {
  fill: none;
  stroke: #ffd06a;
  stroke-width: 3;
  stroke-linecap: round;
  stroke-linejoin: round;
  filter: drop-shadow(0 5px 9px rgba(219, 162, 53, 0.24));
}

.request-records-token-bar {
  position: absolute;
  bottom: 16px;
  width: clamp(5px, 2.1%, 14px);
  min-height: 3px;
  border-radius: 999px 999px 4px 4px;
  background: linear-gradient(180deg, #40c463, #78d88f);
  box-shadow: 0 8px 16px rgba(33, 110, 57, 0.16);
  transform: translateX(-50%);
}

.request-records-token-bar.is-empty {
  display: none;
}

.request-records-token-bar:not(.is-empty) {
  cursor: crosshair;
}

.request-records-token-bar:not(.is-empty):hover {
  background: linear-gradient(180deg, #30a14e, #68cf82);
  box-shadow: 0 10px 18px rgba(33, 110, 57, 0.24);
}

.request-records-chart-tooltip {
  position: absolute;
  z-index: 5;
  display: grid;
  gap: 4px;
  min-width: 210px;
  padding: 12px 14px;
  border: 1px solid rgba(101, 109, 104, 0.16);
  border-radius: 12px;
  background: rgba(255, 255, 255, 0.96);
  box-shadow: 0 16px 30px rgba(49, 58, 52, 0.16);
  color: #5d6460;
  font-size: 12px;
  pointer-events: none;
  transform: translate(-50%, -100%);
}

.request-records-chart-tooltip strong {
  color: #151a17;
  font-size: 13px;
  font-weight: 850;
}

.request-records-token-axis {
  display: flex;
  justify-content: space-between;
  gap: 8px;
  color: #7a857d;
  font-size: 11px;
  font-weight: 650;
}

.request-records-token-side {
  display: grid;
  align-content: center;
  justify-items: center;
  gap: 14px;
  padding: 14px;
  border-radius: 18px;
  border: 1px solid rgba(104, 117, 109, 0.12);
  background: rgba(252, 253, 251, 0.78);
}

.request-records-token-donut {
  position: relative;
  display: grid;
  place-items: center;
  width: 144px;
  height: 144px;
  border-radius: 50%;
  box-shadow:
    inset 0 0 0 1px rgba(50, 65, 56, 0.06),
    0 18px 26px rgba(86, 102, 92, 0.1);
}

.request-records-token-donut-hole {
  display: grid;
  place-items: center;
  align-content: center;
  width: 94px;
  height: 94px;
  border-radius: 50%;
  background: rgba(255, 255, 255, 0.96);
  box-shadow: inset 0 0 0 1px rgba(104, 117, 109, 0.1);
}

.request-records-token-donut-hole strong {
  color: #121915;
  font-size: 22px;
  line-height: 1;
  font-weight: 850;
}

.request-records-token-donut-hole span {
  margin-top: 5px;
  color: #7a857d;
  font-size: 11px;
  font-weight: 750;
  text-transform: uppercase;
}

.request-records-token-breakdown {
  display: grid;
  gap: 8px;
  width: 100%;
  color: #627068;
  font-size: 12px;
  font-weight: 700;
}

.request-records-token-breakdown .is-input {
  background: #6fc4ec;
}

.request-records-token-breakdown .is-output {
  background: #77d99e;
}

.request-records-token-breakdown .is-reasoning {
  background: #ffd06a;
}

.request-records-token-breakdown strong {
  margin-left: auto;
  color: #19211c;
  font-variant-numeric: tabular-nums;
}

.request-records-token-sources {
  display: flex;
  flex-wrap: wrap;
  gap: 6px;
  width: 100%;
}

.request-records-token-sources span {
  display: inline-flex;
  align-items: center;
  gap: 5px;
  min-height: 18px;
  padding: 2px 7px;
  border: 1px solid rgba(94, 153, 150, 0.22);
  border-radius: 999px;
  background: rgba(229, 244, 241, 0.72);
  color: #4d6f6a;
  font-size: 11px;
  font-weight: 700;
  line-height: 1;
}

.request-records-token-sources strong {
  color: #233a36;
  font-variant-numeric: tabular-nums;
}

.request-records-token-empty-note {
  color: #7a857d;
  font-size: 12px;
  font-weight: 700;
}

.request-records-session-trend {
  display: grid;
  gap: 12px;
  min-height: 268px;
}

.request-records-session-bars {
  position: relative;
  display: grid;
  grid-template-columns: 42px minmax(0, 1fr);
  gap: 10px;
  min-height: 210px;
}

.request-records-session-y-axis {
  position: relative;
  min-height: 210px;
  color: #75817a;
  font-size: 10px;
  font-weight: 700;
  font-variant-numeric: tabular-nums;
}

.request-records-session-y-axis span {
  position: absolute;
  right: 0;
  transform: translateY(50%);
}

.request-records-session-plot {
  position: relative;
  min-height: 210px;
  border-bottom: 1px solid rgba(111, 126, 117, 0.18);
  background:
    repeating-linear-gradient(0deg, transparent 0, transparent 49px, rgba(111, 126, 117, 0.11) 50px),
    repeating-linear-gradient(90deg, transparent 0, transparent 20%, rgba(111, 126, 117, 0.09) calc(20% + 1px));
}

.request-records-session-bar {
  position: absolute;
  bottom: 0;
  width: clamp(8px, 2.6%, 34px);
  border-radius: 7px 7px 3px 3px;
  background: linear-gradient(180deg, rgba(64, 196, 99, 0.9), rgba(48, 161, 78, 0.84));
  transform: translateX(-50%);
}

.request-records-session-bar.is-empty {
  display: none;
}

.request-records-session-axis {
  display: flex;
  justify-content: space-between;
  color: #68746c;
  font-size: 12px;
  font-weight: 650;
}

.request-records-session-summary {
  display: grid;
  grid-template-columns: repeat(3, minmax(0, 1fr));
  border-top: 1px solid rgba(111, 126, 117, 0.14);
}

.request-records-session-summary div {
  display: grid;
  gap: 4px;
  padding: 10px 14px 0;
  border-right: 1px solid rgba(111, 126, 117, 0.14);
}

.request-records-session-summary div:last-child {
  border-right: 0;
}

.request-records-session-summary span {
  color: #68746c;
  font-size: 12px;
  font-weight: 650;
}

.request-records-session-summary strong {
  color: #171d19;
  font-size: 20px;
  font-weight: 850;
}

.request-records-tool-ranking {
  display: grid;
  grid-template-columns: minmax(0, 1fr) 210px;
  gap: 22px;
  align-items: center;
  min-height: 268px;
}

.request-records-tool-list {
  display: grid;
  gap: 10px;
}

.request-records-tool-row {
  display: grid;
  grid-template-columns: 150px minmax(0, 1fr) 56px;
  align-items: center;
  gap: 12px;
}

.request-records-tool-name {
  overflow: hidden;
  color: #5f6662;
  font-size: 13px;
  font-weight: 700;
  text-align: right;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.request-records-tool-track {
  height: 24px;
  overflow: hidden;
  border-radius: 7px;
  background: rgba(116, 124, 119, 0.08);
}

.request-records-tool-track i {
  display: block;
  height: 100%;
  border-radius: inherit;
  background: linear-gradient(90deg, #40c463, #30a14e);
}

.request-records-tool-row strong {
  color: #5b625e;
  font-size: 13px;
  font-weight: 850;
  text-align: right;
  font-variant-numeric: tabular-nums;
}

.request-records-tool-side {
  display: grid;
  justify-items: center;
  gap: 12px;
}

.request-records-tool-donut {
  display: grid;
  place-items: center;
  width: 150px;
  height: 150px;
  border-radius: 50%;
}

.request-records-tool-donut-hole {
  display: grid;
  place-items: center;
  align-content: center;
  width: 86px;
  height: 86px;
  border-radius: 50%;
  background: rgba(255, 255, 255, 0.96);
}

.request-records-tool-donut-hole strong {
  color: #121915;
  font-size: 22px;
  font-weight: 850;
}

.request-records-tool-donut-hole span {
  color: #68746c;
  font-size: 12px;
  font-weight: 650;
}

.request-records-tool-breakdown {
  display: grid;
  gap: 8px;
  width: 100%;
  color: #636a66;
  font-size: 12px;
  font-weight: 700;
}

.request-records-tool-breakdown span {
  display: flex;
  align-items: center;
  gap: 8px;
}

.request-records-tool-breakdown i {
  width: 9px;
  height: 9px;
  border-radius: 999px;
}

.request-records-tool-breakdown .is-edit {
  background: #40c463;
}

.request-records-tool-breakdown .is-search {
  background: #6fc4ec;
}

.request-records-tool-breakdown strong {
  margin-left: auto;
  color: #151a17;
  font-variant-numeric: tabular-nums;
}

.request-records-dashboard-placeholder {
  display: grid;
  place-items: center;
  gap: 8px;
  min-height: 226px;
  padding: 24px;
  border-radius: 18px;
  border: 1px dashed rgba(104, 117, 109, 0.2);
  background: rgba(250, 252, 249, 0.68);
  text-align: center;
}

.request-records-dashboard-placeholder strong {
  color: #141b17;
  font-size: 20px;
  font-weight: 850;
}

.request-records-dashboard-placeholder span {
  max-width: 420px;
  color: #68746c;
  font-size: 13px;
  line-height: 1.7;
  font-weight: 650;
}

.request-records-activity-viewport {
  display: grid;
  gap: 9px;
  min-width: 0;
  max-width: 100%;
  overflow-x: auto;
  overflow-y: hidden;
  padding-bottom: 2px;
  scrollbar-width: thin;
}

.request-records-activity-months {
  display: grid;
  grid-template-columns: repeat(var(--activity-columns, 53), 11px);
  justify-content: start;
  gap: 3px;
  padding: 0 2px;
  color: #687069;
  font-size: 11px;
  line-height: 1;
}

.request-records-activity-months span {
  white-space: nowrap;
}

.request-records-activity-grid {
  display: grid;
  grid-auto-flow: column;
  grid-template-rows: repeat(7, 11px);
  grid-template-columns: repeat(var(--activity-columns, 53), 11px);
  grid-auto-columns: 11px;
  justify-content: start;
  gap: 3px;
  overflow: visible;
  padding: 0 2px 2px;
  scrollbar-width: none;
}

.request-records-activity-grid::-webkit-scrollbar {
  display: none;
}

.request-records-activity-cell {
  width: 11px;
  height: 11px;
  border-radius: 2px;
  background: #ebedf0;
  box-shadow: inset 0 0 0 1px rgba(27, 31, 35, 0.035);
}

.request-records-activity-cell.is-pad {
  visibility: hidden;
}

.request-records-activity-cell.is-level-1 { background: #9be9a8; }
.request-records-activity-cell.is-level-2 { background: #40c463; }
.request-records-activity-cell.is-level-3 { background: #30a14e; }
.request-records-activity-cell.is-level-4 { background: #216e39; }

.request-records-activity-legend {
  display: inline-flex;
  align-items: center;
  justify-self: center;
  gap: 7px;
  color: #6c716d;
  font-size: 12px;
}

.request-records-activity-legend .request-records-activity-cell {
  width: 11px;
  height: 11px;
}

.request-records-activity-summary {
  display: grid;
  grid-template-columns: repeat(3, minmax(0, 1fr));
  gap: 0;
  padding-top: 12px;
  border-top: 1px solid rgba(104, 117, 109, 0.14);
}

.request-records-activity-summary div {
  display: grid;
  gap: 5px;
  padding: 0 14px;
  border-left: 1px solid rgba(104, 117, 109, 0.14);
}

.request-records-activity-summary div:first-child {
  border-left: 0;
  padding-left: 0;
}

.request-records-activity-summary span {
  color: #68746c;
  font-size: 12px;
  font-weight: 600;
}

.request-records-activity-summary strong {
  color: #111815;
  font-size: 20px;
  line-height: 1;
  font-weight: 850;
}

.request-records-overview {
  display: grid;
  grid-template-columns: repeat(4, minmax(0, 1fr));
  gap: 8px;
}

.request-records-metric {
  display: grid;
  gap: 5px;
  min-height: 76px;
  padding: 10px 12px;
  border-radius: 18px;
  border: 1px solid rgba(110, 133, 118, 0.12);
  background:
    linear-gradient(180deg, rgba(255, 255, 255, 0.98), rgba(244, 248, 242, 0.94)),
    rgba(255, 255, 255, 0.94);
  box-shadow:
    0 14px 28px rgba(85, 104, 90, 0.06),
    inset 0 1px 0 rgba(255, 255, 255, 0.8);
}

.request-records-metric-label {
  color: #748076;
  font-size: 10px;
  font-weight: 700;
  letter-spacing: 0.08em;
  text-transform: uppercase;
}

.request-records-metric-value {
  color: #213129;
  font-size: 22px;
  line-height: 1.05;
  font-weight: 800;
  font-variant-numeric: tabular-nums;
}

.request-records-metric small {
  color: #6e7d72;
  font-size: 11px;
  line-height: 1.35;
}

.request-records-board {
  display: grid;
  grid-template-rows: auto minmax(0, 1fr);
  flex: none;
  min-height: clamp(420px, 58vh, 680px);
  min-width: 0;
  width: 100%;
  border-radius: 20px;
  border: 1px solid rgba(103, 126, 111, 0.12);
  background:
    linear-gradient(180deg, rgba(254, 254, 253, 0.96), rgba(247, 249, 246, 0.94)),
    rgba(255, 255, 255, 0.94);
  box-shadow:
    0 18px 36px rgba(87, 105, 92, 0.06),
    inset 0 1px 0 rgba(255, 255, 255, 0.82);
  overflow: hidden;
}

.request-records-board-head {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 10px;
  padding: 10px 12px;
  border-bottom: 1px solid rgba(108, 129, 114, 0.1);
  min-width: 0;
}

.request-records-board-title {
  display: inline-flex;
  align-items: center;
  gap: 8px;
  min-width: 0;
}

.request-records-board-title strong {
  color: #24342b;
  font-size: 13px;
  font-weight: 700;
  white-space: nowrap;
}

.request-records-board-title span {
  display: inline-flex;
  align-items: center;
  min-height: 24px;
  padding: 0 9px;
  border-radius: 999px;
  background: rgba(232, 239, 230, 0.9);
  color: #536257;
  font-size: 11px;
  font-weight: 700;
}

.request-records-table-wrap {
  display: grid;
  grid-template-rows: minmax(0, 1fr) auto auto;
  flex: 1 1 auto;
  min-height: 0;
  height: 100%;
  min-width: 0;
  width: 100%;
  overflow: hidden;
}

.request-records-table-spin {
  display: flex;
  flex: 1 1 auto;
  min-height: 0;
  min-width: 0;
  width: 100%;
  overflow: hidden;
}

.request-records-table-spin :deep(.ant-spin-nested-loading) {
  display: flex;
  flex: 1 1 auto;
  flex-direction: column;
  min-height: 0;
  min-width: 0;
  width: 100%;
}

.request-records-table-spin :deep(.ant-spin-container) {
  display: flex;
  flex: 1 1 auto;
  flex-direction: column;
  min-height: 0;
  min-width: 0;
  width: 100%;
}

.request-records-table-scroll {
  flex: 1 1 auto;
  min-height: 240px;
  min-width: 0;
  width: 100%;
  max-width: 100%;
  overflow-x: hidden;
  overflow-y: auto;
  scrollbar-gutter: stable;
}

.request-records-table-scroll.is-draggable {
  cursor: grab;
}

.request-records-table-scroll.is-dragging {
  cursor: grabbing;
  user-select: none;
}

.request-records-table-scroll.is-dragging * {
  user-select: none;
}

.request-records-table-stage {
  will-change: transform;
}

.request-records-table-hscroll {
  position: relative;
  z-index: 3;
  min-width: 0;
  width: 60%;
  height: 20px;
  margin: 4px 0 0 12px;
  display: flex;
  align-items: center;
  pointer-events: auto;
}

.request-records-table-hscroll-range {
  appearance: none;
  -webkit-appearance: none;
  width: 100%;
  height: 20px;
  margin: 0;
  background: transparent;
  pointer-events: auto;
}

.request-records-table-hscroll-range::-webkit-slider-runnable-track {
  height: 8px;
  border-radius: 999px;
  background: rgba(228, 235, 230, 0.9);
  box-shadow: inset 0 1px 0 rgba(255, 255, 255, 0.7);
}

.request-records-table-hscroll-range::-webkit-slider-thumb {
  appearance: none;
  -webkit-appearance: none;
  width: 32px;
  height: 12px;
  margin-top: -2px;
  border: 0;
  border-radius: 999px;
  background: rgba(120, 138, 126, 0.38);
  transition: background-color 0.18s ease, opacity 0.18s ease;
}

.request-records-table-hscroll.is-active .request-records-table-hscroll-range::-webkit-slider-thumb {
  background: rgba(96, 120, 104, 0.82);
}

.request-records-table-hscroll-range::-moz-range-track {
  height: 8px;
  border: 0;
  border-radius: 999px;
  background: rgba(228, 235, 230, 0.9);
  box-shadow: inset 0 1px 0 rgba(255, 255, 255, 0.7);
}

.request-records-table-hscroll-range::-moz-range-thumb {
  width: 32px;
  height: 12px;
  border: 0;
  border-radius: 999px;
  background: rgba(120, 138, 126, 0.38);
  transition: background-color 0.18s ease, opacity 0.18s ease;
}

.request-records-table-hscroll.is-active .request-records-table-hscroll-range::-moz-range-thumb {
  background: rgba(96, 120, 104, 0.82);
}

.request-records-table-hscroll-range:disabled {
  opacity: 0.6;
  cursor: default;
}

.request-records-table-native {
  width: max-content;
  min-width: 100%;
  border-collapse: separate;
  border-spacing: 0;
  table-layout: fixed;
}

.request-records-table-head {
  position: sticky;
  top: 0;
  z-index: 2;
  padding: 8px 10px;
  border-bottom: 1px solid rgba(110, 132, 118, 0.1);
  background: rgba(248, 250, 247, 0.94);
  color: #718176;
  font-size: 10px;
  font-weight: 700;
  letter-spacing: 0.08em;
  text-transform: uppercase;
  text-align: left;
  white-space: nowrap;
}

.request-records-table-head.is-center,
.request-records-table-cell.is-center {
  text-align: center;
}

.request-records-table-cell {
  padding: 9px 10px;
  border-bottom: 1px solid rgba(109, 128, 115, 0.08);
  background: transparent;
  vertical-align: top;
}

.request-records-table-native tbody tr:hover > .request-records-table-cell {
  background: rgba(241, 246, 239, 0.76);
}

.request-records-table-cell.is-actions,
.request-records-table-head.is-actions {
  width: 46px;
}

.request-records-empty-cell {
  padding: 56px 20px;
  color: #b6b9b7;
  font-size: 18px;
  text-align: center;
}

.request-records-pagination {
  display: flex;
  justify-content: flex-end;
  padding: 8px 12px 12px;
}

.request-records-table-scroll::-webkit-scrollbar:vertical {
  width: 10px;
}

.request-records-table-scroll::-webkit-scrollbar:horizontal {
  height: 0;
}

.request-records-table-scroll::-webkit-scrollbar-thumb:vertical {
  border-radius: 999px;
  background: rgba(120, 138, 126, 0.62);
}

.request-records-table-scroll::-webkit-scrollbar-track:vertical {
  background: rgba(228, 235, 230, 0.72);
}

.request-records-time,
.request-records-identity,
.request-records-route,
.request-records-metrics,
.request-records-status {
  display: grid;
  gap: 3px;
  min-width: 0;
}

.request-records-time strong,
.request-records-identity strong,
.request-records-metrics strong,
.request-record-detail-item strong,
.request-record-detail-hero-main strong {
  color: #203028;
  font-weight: 700;
  font-variant-numeric: tabular-nums;
}

.request-records-time small,
.request-records-identity small,
.request-records-route small,
.request-records-metrics small,
.request-record-detail-item span,
.request-record-detail-hero-main small {
  color: #738277;
  font-size: 11px;
  line-height: 1.35;
}

.request-records-identity-main {
  display: flex;
  align-items: center;
  gap: 6px;
  min-width: 0;
}

.request-records-identity-main strong {
  min-width: 0;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.request-records-app-pill {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  min-height: 22px;
  padding: 0 8px;
  border-radius: 999px;
  background: rgba(224, 236, 229, 0.94);
  color: #365149;
  font-size: 11px;
  font-weight: 700;
  line-height: 1;
  flex: 0 0 auto;
}

.request-records-mono {
  font-family: 'Cascadia Code', 'Consolas', monospace;
}

.request-records-route-line {
  display: flex;
  align-items: center;
  gap: 6px;
  min-width: 0;
}

.request-records-route-key {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  min-width: 32px;
  height: 20px;
  border-radius: 999px;
  background: rgba(228, 234, 226, 0.94);
  color: #5b6b60;
  font-size: 10px;
  font-weight: 800;
  line-height: 1;
  flex: 0 0 auto;
}

.request-records-route-key.is-meta {
  background: rgba(241, 236, 223, 0.94);
  color: #8d6b2f;
}

.request-records-route-key.is-out {
  background: rgba(224, 232, 255, 0.92);
  color: #3e5fb9;
}

.request-records-route-pill {
  display: inline-flex;
  align-items: center;
  min-height: 22px;
  padding: 0 8px;
  border-radius: 999px;
  background: rgba(224, 232, 255, 0.92);
  color: #3e5fb9;
  font-size: 11px;
  font-weight: 700;
  line-height: 1;
  white-space: nowrap;
  flex: 0 0 auto;
}

.request-records-route-inbound,
.request-records-route-host,
.request-records-route-path {
  min-width: 0;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.request-records-route-inbound {
  color: #4d5e54;
  font-size: 12px;
  font-weight: 600;
}

.request-records-route-host {
  color: #78867b;
}

.request-records-route-path {
  color: #4d5e54;
  font-size: 12px;
  font-weight: 600;
}

.request-records-route-path-out {
  color: #78867b;
  font-weight: 500;
}

.request-records-routing {
  display: grid;
  gap: 5px;
}

.request-records-routing-line {
  display: flex;
  align-items: center;
  gap: 6px;
  min-width: 0;
  color: #55655a;
  font-size: 11px;
  line-height: 1.2;
}

.request-records-routing-line.is-direct {
  color: #58695f;
}

.request-records-routing-line.is-failed {
  color: #9b6b5a;
}

.request-records-routing-line.is-fallback-final {
  color: #2f7a45;
  font-weight: 700;
}

.request-records-routing-icon {
  width: 12px;
  text-align: center;
  font-size: 10px;
  flex: 0 0 auto;
}

.request-records-routing-label {
  min-width: 0;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.request-records-routing-source {
  color: #8b998f;
  font-size: 10px;
  flex: 0 0 auto;
}

.request-records-status {
  justify-items: start;
}

.request-records-source-pill {
  display: inline-flex;
  align-items: center;
  min-height: 22px;
  padding: 0 8px;
  border-radius: 999px;
  background: rgba(233, 239, 231, 0.94);
  color: #55655a;
  font-size: 11px;
  font-weight: 700;
  line-height: 1;
}

.request-records-source-pill.is-fallback {
  background: rgba(255, 236, 212, 0.94);
  color: #a16420;
}

.request-records-source-pill.is-preference {
  background: rgba(232, 226, 255, 0.94);
  color: #7655b5;
}

.request-records-source-pill.is-direct {
  background: rgba(219, 240, 255, 0.94);
  color: #2d72b9;
}

.request-records-detail-text {
  color: #516156;
  font-size: 12px;
  line-height: 1.45;
  white-space: normal;
  word-break: break-word;
  display: -webkit-box;
  overflow: hidden;
  -webkit-box-orient: vertical;
  -webkit-line-clamp: 2;
}

.request-records-more {
  width: 28px;
  height: 28px;
  border-radius: 10px;
  color: #59695f;
}

.request-records-more:hover {
  background: rgba(231, 238, 228, 0.86);
  color: #223128;
}

.request-records-empty {
  padding: 18px;
  border-radius: 18px;
  border: 1px dashed rgba(118, 135, 121, 0.22);
  background: rgba(250, 252, 249, 0.92);
  color: #69786d;
  font-size: 12px;
}

.request-record-detail-shell {
  display: flex;
  flex-direction: column;
  gap: 10px;
}

.terminal-sessions-board {
  display: grid;
  grid-template-columns: minmax(220px, 260px) minmax(0, 1fr);
  gap: 10px;
  min-width: 0;
  padding: 12px;
  border: 1px solid rgba(102, 122, 108, 0.12);
  border-radius: 16px;
  background:
    linear-gradient(180deg, rgba(255, 255, 255, 0.78), rgba(246, 250, 244, 0.72)),
    rgba(255, 255, 255, 0.76);
  box-shadow: 0 18px 34px rgba(78, 103, 72, 0.08);
}

.terminal-session-list-pane,
.terminal-session-chat-pane {
  min-width: 0;
  min-height: 520px;
}

.terminal-session-list-pane {
  display: grid;
  grid-template-rows: auto auto minmax(0, 1fr) auto;
  gap: 8px;
}

.terminal-session-chat-pane {
  display: flex;
  flex-direction: column;
  gap: 10px;
  border: 1px solid rgba(102, 122, 108, 0.12);
  border-radius: 12px;
  background: rgba(255, 255, 255, 0.54);
  padding: 10px;
  overflow: hidden;
}

.terminal-provider-tabs {
  display: flex;
  align-items: center;
  gap: 6px;
  min-width: 0;
  overflow-x: auto;
  scrollbar-width: none;
  padding: 2px 2px 4px;
}

.terminal-provider-tabs::-webkit-scrollbar {
  display: none;
}

.terminal-provider-tab {
  position: relative;
  width: 34px;
  height: 34px;
  padding: 0;
  border: 1px solid rgba(102, 122, 108, 0.14);
  border-radius: 10px;
  background: rgba(255, 255, 255, 0.56);
  color: #53634f;
  display: inline-flex;
  align-items: center;
  justify-content: center;
  gap: 5px;
  cursor: pointer;
  flex: 0 0 auto;
}

.terminal-provider-tab img {
  width: 20px;
  height: 20px;
  display: block;
  object-fit: contain;
}

.terminal-provider-tab small {
  position: absolute;
  right: 2px;
  bottom: 2px;
  min-width: 15px;
  max-width: 24px;
  padding: 1px 4px;
  border-radius: 999px;
  background: #385a32;
  color: #fff;
  font-size: 9px;
  line-height: 1.1;
  overflow: hidden;
  text-overflow: ellipsis;
}

.terminal-provider-tab.is-active {
  border-color: rgba(96, 134, 72, 0.34);
  background: linear-gradient(180deg, rgba(250, 255, 244, 0.98), rgba(232, 243, 218, 0.92));
  color: #263c27;
  box-shadow: 0 10px 22px rgba(83, 112, 63, 0.1);
}

.terminal-session-list-controls {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 8px;
  min-width: 0;
}

.terminal-session-refresh-button {
  flex: 0 0 auto;
}

.terminal-session-list {
  display: grid;
  gap: 7px;
  align-content: start;
  min-height: 0;
  max-height: 456px;
  overflow: auto;
  padding-right: 2px;
}

.terminal-session-list::-webkit-scrollbar,
.terminal-session-message-list::-webkit-scrollbar {
  width: 6px;
}

.terminal-session-list::-webkit-scrollbar-thumb,
.terminal-session-message-list::-webkit-scrollbar-thumb {
  border-radius: 999px;
  background: rgba(118, 139, 119, 0.32);
}

.terminal-session-item {
  display: grid;
  grid-template-columns: minmax(0, 1fr) 34px;
  align-items: center;
  gap: 8px;
  min-height: 54px;
  padding: 8px 9px 8px 11px;
  border: 1px solid rgba(102, 122, 108, 0.12);
  border-radius: 10px;
  background: linear-gradient(180deg, rgba(255, 255, 255, 0.76), rgba(247, 250, 243, 0.66));
  box-shadow: inset 0 1px 0 rgba(255, 255, 255, 0.72);
  cursor: pointer;
  transition: border-color 0.16s ease, box-shadow 0.16s ease, transform 0.16s ease;
}

.terminal-session-item:hover,
.terminal-session-item.is-active {
  border-color: rgba(96, 134, 72, 0.34);
  box-shadow: 0 12px 24px rgba(83, 112, 63, 0.12), inset 0 1px 0 rgba(255, 255, 255, 0.72);
}

.terminal-session-item.is-active {
  background: linear-gradient(180deg, rgba(252, 255, 247, 0.98), rgba(235, 244, 226, 0.9));
}

.terminal-session-main {
  min-width: 0;
  padding: 0;
  border: 0;
  background: transparent;
  color: inherit;
  display: grid;
  gap: 3px;
  text-align: left;
}

.terminal-session-main strong,
.terminal-session-main span,
.terminal-session-main small {
  min-width: 0;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.terminal-session-main strong {
  color: #263628;
  font-size: 13px;
  font-weight: 800;
}

.terminal-session-main span {
  display: none;
  color: #586858;
  font-size: 12px;
}

.terminal-session-main small {
  color: #7a8974;
  font-size: 11px;
}

.terminal-session-open-button {
  width: 32px;
  height: 32px;
  border: 1px solid rgba(91, 122, 72, 0.18);
  border-radius: 9px;
  background: rgba(255, 255, 255, 0.72);
  color: #355431;
  display: inline-flex;
  align-items: center;
  justify-content: center;
  cursor: pointer;
}

.terminal-session-open-button:disabled {
  cursor: default;
  opacity: 0.48;
}

.terminal-sessions-empty {
  min-height: 220px;
}

.terminal-session-chat-head {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  gap: 10px;
  min-width: 0;
  padding-bottom: 9px;
  border-bottom: 1px solid rgba(102, 122, 108, 0.1);
}

.terminal-session-chat-head div {
  display: grid;
  gap: 3px;
  min-width: 0;
}

.terminal-session-chat-head strong,
.terminal-session-chat-head span {
  min-width: 0;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.terminal-session-chat-head strong {
  color: #263628;
  font-size: 13px;
  font-weight: 850;
}

.terminal-session-chat-head span,
.terminal-session-chat-head small {
  color: #748170;
  font-size: 11px;
}

.terminal-session-message-list {
  display: grid;
  align-content: start;
  gap: 9px;
  min-height: 0;
  max-height: 462px;
  overflow: auto;
  padding-right: 4px;
}

.terminal-session-message {
  display: grid;
  gap: 5px;
  padding: 9px 10px;
  border: 1px solid rgba(102, 122, 108, 0.11);
  border-radius: 11px;
  background: rgba(255, 255, 255, 0.7);
}

.terminal-session-message.is-user {
  border-color: rgba(72, 114, 151, 0.18);
  background: rgba(244, 249, 255, 0.78);
}

.terminal-session-message.is-assistant {
  border-color: rgba(99, 136, 83, 0.16);
  background: rgba(250, 253, 247, 0.78);
}

.terminal-session-message.is-tool {
  border-color: rgba(146, 115, 76, 0.18);
  background: rgba(255, 250, 241, 0.78);
}

.terminal-session-message-meta {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 8px;
  color: #7a8974;
  font-size: 10px;
  font-weight: 800;
  text-transform: uppercase;
}

.terminal-session-message p {
  margin: 0;
  color: #334037;
  font-size: 12px;
  line-height: 1.55;
  white-space: pre-wrap;
  overflow-wrap: anywhere;
}

.terminal-session-message p.is-collapsed {
  display: -webkit-box;
  overflow: hidden;
  -webkit-box-orient: vertical;
  -webkit-line-clamp: 3;
}

.terminal-session-message-toggle {
  justify-self: start;
  height: 24px;
  padding: 0 8px;
  border: 1px solid rgba(102, 122, 108, 0.14);
  border-radius: 8px;
  background: rgba(255, 255, 255, 0.62);
  color: #466045;
  font-size: 11px;
  font-weight: 800;
  cursor: pointer;
}

.terminal-session-message-toggle:hover {
  border-color: rgba(96, 134, 72, 0.32);
  background: rgba(242, 249, 238, 0.9);
}

.terminal-session-chat-empty {
  flex: 1;
  min-height: 320px;
}

.request-record-detail-hero {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  gap: 10px;
  padding: 12px 13px;
  border-radius: 18px;
  border: 1px solid rgba(105, 126, 112, 0.12);
  background:
    linear-gradient(180deg, rgba(255, 255, 255, 0.98), rgba(244, 248, 242, 0.94)),
    rgba(255, 255, 255, 0.94);
}

.request-record-detail-hero-main {
  display: grid;
  gap: 4px;
  min-width: 0;
}

.request-record-detail-section {
  display: grid;
  gap: 8px;
}

.request-record-detail-section-title {
  color: #6f7d73;
  font-size: 11px;
  font-weight: 700;
  letter-spacing: 0.08em;
  text-transform: uppercase;
}

.request-record-detail-section-title-row {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 10px;
}

.request-record-detail-section-actions {
  display: inline-flex;
  align-items: center;
  gap: 8px;
}

.request-record-detail-grid {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: 8px;
}

.request-record-detail-item {
  display: grid;
  gap: 5px;
  padding: 12px;
  border-radius: 16px;
  border: 1px solid rgba(107, 127, 114, 0.12);
  background: rgba(248, 251, 247, 0.94);
  min-width: 0;
}

.request-record-detail-item-full {
  grid-column: 1 / -1;
}

.request-record-detail-item pre {
  margin: 0;
  color: #4a5a50;
  font: 12px/1.6 'Cascadia Code', 'Consolas', monospace;
  white-space: pre-wrap;
  word-break: break-word;
}

.request-record-debug-button {
  border-radius: 10px;
}

.request-record-debug-result {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  width: 22px;
  height: 22px;
  border-radius: 999px;
  font-size: 14px;
}

.request-record-debug-result.is-loading {
  color: #6f7d73;
}

.request-record-debug-result.is-success {
  color: #2f8f59;
}

.request-record-debug-result.is-error {
  color: #cc4b37;
}

.request-record-debug-textarea :deep(textarea) {
  min-height: 240px;
  border-radius: 16px;
  padding: 12px 13px;
  color: #4a5a50;
  font: 12px/1.6 'Cascadia Code', 'Consolas', monospace;
  background: rgba(248, 251, 247, 0.94);
  border-color: rgba(107, 127, 114, 0.12);
}

.request-records-shell-dark .request-records-mode-tabs,
.request-records-shell-dark .request-records-toolbar-pill,
.request-records-shell-dark .request-records-board-chip,
.request-records-shell-dark .mcp-skill-app-tabs,
.request-records-shell-dark .mcp-skill-card,
.request-records-shell-dark .terminal-provider-tab,
.request-records-shell-dark .terminal-session-item,
.request-records-shell-dark .terminal-session-chat-pane,
.request-records-shell-dark .terminal-session-message,
.request-records-shell-dark .request-record-detail-item,
.request-records-shell-dark .request-record-detail-hero {
  border-color: rgba(133, 162, 145, 0.16);
  background:
    linear-gradient(180deg, rgba(28, 37, 32, 0.96), rgba(22, 30, 26, 0.94)),
    rgba(17, 24, 20, 0.92);
  box-shadow: 0 14px 28px rgba(0, 0, 0, 0.22);
}

.request-records-shell-dark .request-records-mode-tab {
  color: #a9b9af;
}

.request-records-shell-dark .request-records-activity-tab,
.request-records-shell-dark .request-records-activity-range-button {
  color: #a9b9af;
}

.request-records-shell-dark .request-records-mode-tab.is-active,
.request-records-shell-dark .request-records-activity-tab.is-active,
.request-records-shell-dark .request-records-activity-range-button.is-active,
.request-records-shell-dark .terminal-provider-tab.is-active {
  color: #eef6ef;
  background: linear-gradient(180deg, rgba(66, 88, 58, 0.92), rgba(42, 60, 38, 0.88));
  border-color: rgba(154, 194, 132, 0.24);
  box-shadow: 0 12px 24px rgba(0, 0, 0, 0.24);
}

.request-records-shell-dark .request-records-toolbar-pill-muted,
.request-records-shell-dark .request-records-board-chip-muted {
  background: rgba(25, 33, 29, 0.94);
  color: #a9b9af;
}

.request-records-shell-dark .request-records-board-chip-toggle.is-inactive {
  background: rgba(25, 33, 29, 0.94);
  border-color: rgba(133, 162, 145, 0.08);
  color: #8a9990;
  opacity: 0.72;
}

.request-records-shell-dark .mcp-skill-card-main strong {
  color: #eef6ef;
}

.request-records-shell-dark .mcp-skill-card-main span {
  color: #c7d6cb;
}

.request-records-shell-dark .mcp-skill-card-main small {
  color: #93a69a;
}

.request-records-shell-dark .mcp-skill-app-tab.is-active {
  background: linear-gradient(180deg, rgba(66, 88, 58, 0.92), rgba(42, 60, 38, 0.88));
}

.request-records-shell-dark .mcp-skill-state-pill {
  border-color: rgba(133, 162, 145, 0.14);
  background: rgba(255, 255, 255, 0.05);
  color: #9bac9f;
}

.request-records-shell-dark .mcp-skill-state-pill.is-on {
  border-color: rgba(154, 194, 132, 0.24);
  background: rgba(66, 88, 58, 0.52);
  color: #edf6ee;
}

.request-records-shell-dark .mcp-skill-card-button-import {
  border-color: rgba(154, 194, 132, 0.24);
  background: rgba(255, 255, 255, 0.06);
  color: #edf6ee;
}

.request-records-shell-dark .mcp-skill-card-button-disable {
  border-color: rgba(133, 162, 145, 0.16);
  background: rgba(255, 255, 255, 0.05);
  color: #c7d6cb;
}

.request-records-shell-dark .mcp-skill-subsection-title {
  color: #93a69a;
}

.request-records-shell-dark .mcp-skill-app-toggle {
  border-color: rgba(133, 162, 145, 0.14);
  background: rgba(255, 255, 255, 0.05);
  color: #9bac9f;
}

.request-records-shell-dark .mcp-skill-app-toggle.is-on {
  border-color: rgba(154, 194, 132, 0.28);
  background: linear-gradient(180deg, rgba(66, 88, 58, 0.92), rgba(42, 60, 38, 0.88));
  color: #edf6ee;
  box-shadow: 0 10px 20px rgba(0, 0, 0, 0.2);
}

.request-records-shell-dark .request-records-toolbar-dot {
  background: #7fb486;
  box-shadow: 0 0 0 4px rgba(127, 180, 134, 0.12);
}

.request-records-shell-dark .request-records-overview .request-records-metric,
.request-records-shell-dark .request-records-activity-card,
.request-records-shell-dark .request-records-board,
.request-records-shell-dark .terminal-sessions-board,
.request-records-shell-dark .request-records-empty {
  border-color: rgba(133, 162, 145, 0.16);
  background:
    linear-gradient(180deg, rgba(24, 33, 28, 0.96), rgba(18, 25, 21, 0.94)),
    rgba(17, 24, 20, 0.92);
  box-shadow: 0 18px 34px rgba(0, 0, 0, 0.22);
}

.request-records-shell-dark .terminal-provider-tab {
  color: #a9b9af;
}

.request-records-shell-dark .terminal-provider-tab small {
  background: rgba(172, 218, 145, 0.88);
  color: #142017;
}

.request-records-shell-dark .terminal-session-item.is-active {
  border-color: rgba(154, 194, 132, 0.24);
  background: linear-gradient(180deg, rgba(51, 68, 47, 0.96), rgba(36, 51, 34, 0.92));
}

.request-records-shell-dark .terminal-session-main strong {
  color: #eef6ef;
}

.request-records-shell-dark .terminal-session-main span {
  color: #c7d6cb;
}

.request-records-shell-dark .terminal-session-main small {
  color: #93a69a;
}

.request-records-shell-dark .terminal-session-open-button {
  border-color: rgba(133, 162, 145, 0.18);
  background: rgba(255, 255, 255, 0.06);
  color: #d9eadb;
}

.request-records-shell-dark .terminal-session-chat-head {
  border-bottom-color: rgba(133, 162, 145, 0.14);
}

.request-records-shell-dark .terminal-session-chat-head strong {
  color: #eef6ef;
}

.request-records-shell-dark .terminal-session-chat-head span,
.request-records-shell-dark .terminal-session-chat-head small,
.request-records-shell-dark .terminal-session-message-meta {
  color: #93a69a;
}

.request-records-shell-dark .terminal-session-message.is-user {
  border-color: rgba(104, 151, 189, 0.22);
  background: rgba(24, 36, 45, 0.88);
}

.request-records-shell-dark .terminal-session-message.is-assistant {
  border-color: rgba(133, 162, 145, 0.18);
  background: rgba(23, 34, 28, 0.88);
}

.request-records-shell-dark .terminal-session-message.is-tool {
  border-color: rgba(176, 148, 101, 0.22);
  background: rgba(42, 34, 23, 0.86);
}

.request-records-shell-dark .terminal-session-message p {
  color: #d9eadb;
}

.request-records-shell-dark .terminal-session-message-toggle {
  border-color: rgba(133, 162, 145, 0.18);
  background: rgba(255, 255, 255, 0.06);
  color: #d9eadb;
}

.request-records-shell-dark .terminal-session-message-toggle:hover {
  border-color: rgba(154, 194, 132, 0.26);
  background: rgba(255, 255, 255, 0.1);
}

.request-records-shell-dark .request-records-table-head {
  border-bottom-color: rgba(129, 155, 140, 0.14);
  background: rgba(24, 33, 28, 0.96);
  color: #aebfb4;
}

.request-records-shell-dark .request-records-table-cell {
  border-bottom-color: rgba(129, 155, 140, 0.1);
}

.request-records-shell-dark .request-records-table-native tbody tr:hover > .request-records-table-cell {
  background: rgba(255, 255, 255, 0.04);
}

.request-records-shell-dark .request-records-empty-cell {
  color: #8ea196;
}

.request-records-shell-dark .request-records-table-scroll::-webkit-scrollbar-thumb:vertical {
  background: rgba(129, 155, 140, 0.52);
}

.request-records-shell-dark .request-records-table-scroll::-webkit-scrollbar-track:vertical {
  background: rgba(23, 31, 27, 0.82);
}

.request-records-shell-dark .request-records-table-hscroll-range::-webkit-slider-runnable-track {
  background: rgba(23, 31, 27, 0.86);
  box-shadow: inset 0 1px 0 rgba(255, 255, 255, 0.04);
}

.request-records-shell-dark .request-records-table-hscroll-range::-webkit-slider-thumb {
  background: rgba(129, 155, 140, 0.4);
}

.request-records-shell-dark .request-records-table-hscroll.is-active .request-records-table-hscroll-range::-webkit-slider-thumb {
  background: rgba(129, 155, 140, 0.82);
}

.request-records-shell-dark .request-records-table-hscroll-range::-moz-range-track {
  background: rgba(23, 31, 27, 0.86);
  box-shadow: inset 0 1px 0 rgba(255, 255, 255, 0.04);
}

.request-records-shell-dark .request-records-table-hscroll-range::-moz-range-thumb {
  background: rgba(129, 155, 140, 0.4);
}

.request-records-shell-dark .request-records-table-hscroll.is-active .request-records-table-hscroll-range::-moz-range-thumb {
  background: rgba(129, 155, 140, 0.82);
}

.request-records-shell-dark .request-records-metric-label,
.request-records-shell-dark .request-records-activity-months,
.request-records-shell-dark .request-records-activity-legend,
.request-records-shell-dark .request-records-activity-summary span,
.request-records-shell-dark .request-record-detail-section-title,
.request-records-shell-dark .request-record-detail-item span,
.request-records-shell-dark .request-record-detail-hero-main small,
.request-records-shell-dark .request-records-time small,
.request-records-shell-dark .request-records-identity small,
.request-records-shell-dark .request-records-route small,
.request-records-shell-dark .request-records-metrics small {
  color: #aebfb4;
}

.request-records-shell-dark .request-records-metric-value,
.request-records-shell-dark .request-records-activity-title,
.request-records-shell-dark .request-records-activity-summary strong,
.request-records-shell-dark .request-records-board-title strong,
.request-records-shell-dark .request-records-time strong,
.request-records-shell-dark .request-records-identity strong,
.request-records-shell-dark .request-records-metrics strong,
.request-records-shell-dark .request-record-detail-item strong,
.request-records-shell-dark .request-record-detail-hero-main strong {
  color: #edf6ee;
}

.request-records-shell-dark .request-records-metric small,
.request-records-shell-dark .request-records-detail-text,
.request-records-shell-dark .request-records-empty,
.request-records-shell-dark .request-records-route-inbound,
.request-records-shell-dark .request-records-route-host,
.request-records-shell-dark .request-records-route-path,
.request-records-shell-dark .request-records-route-path-out,
.request-records-shell-dark .request-records-routing-line.is-direct,
.request-records-shell-dark .request-records-routing-source,
.request-records-shell-dark .request-record-detail-item pre {
  color: #b8c8be;
}

.request-records-shell-dark .request-record-debug-result.is-loading {
  color: #b8c8be;
}

.request-records-shell-dark .request-record-debug-result.is-success {
  color: #77d69a;
}

.request-records-shell-dark .request-records-token-chart,
.request-records-shell-dark .request-records-token-side,
.request-records-shell-dark .request-records-dashboard-placeholder {
  border-color: rgba(133, 162, 145, 0.14);
  background: rgba(18, 26, 22, 0.72);
}

.request-records-shell-dark .request-records-token-plot {
  background:
    linear-gradient(180deg, rgba(25, 35, 29, 0.94), rgba(18, 27, 22, 0.78)),
    rgba(19, 27, 23, 0.88);
}

.request-records-shell-dark .request-records-token-grid-line {
  stroke: rgba(166, 189, 174, 0.14);
}

.request-records-shell-dark .request-records-token-legend,
.request-records-shell-dark .request-records-token-axis,
.request-records-shell-dark .request-records-token-breakdown,
.request-records-shell-dark .request-records-token-donut-hole span,
.request-records-shell-dark .request-records-dashboard-placeholder span {
  color: #aebfb4;
}

.request-records-shell-dark .request-records-token-donut-hole {
  background: rgba(19, 27, 23, 0.96);
  box-shadow: inset 0 0 0 1px rgba(133, 162, 145, 0.14);
}

.request-records-shell-dark .request-records-token-donut-hole strong,
.request-records-shell-dark .request-records-token-breakdown strong,
.request-records-shell-dark .request-records-dashboard-placeholder strong {
  color: #edf6ee;
}

.request-records-shell-dark .request-records-token-legend .request-records-token-source {
  border-color: rgba(151, 177, 162, 0.22);
  background: rgba(32, 43, 37, 0.72);
  color: #b9c9be;
}

.request-records-shell-dark .request-records-token-sources span {
  border-color: rgba(118, 184, 177, 0.24);
  background: rgba(32, 55, 52, 0.66);
  color: #aed2cd;
}

.request-records-shell-dark .request-records-token-sources strong {
  color: #effaf7;
}

.request-records-shell-dark .request-record-debug-result.is-error {
  color: #ff8d7d;
}

.request-records-shell-dark .request-record-debug-textarea :deep(textarea) {
  color: #d3dfd6;
  background: rgba(18, 25, 21, 0.94);
  border-color: rgba(133, 162, 145, 0.16);
}

.request-records-shell-dark .request-records-routing-line.is-failed {
  color: #f0b8a5;
}

.request-records-shell-dark .request-records-routing-line.is-fallback-final {
  color: #87d39c;
}

.request-records-shell-dark .request-records-route-key {
  background: rgba(67, 84, 75, 0.9);
  color: #dbe8df;
}

.request-records-shell-dark .request-records-route-key.is-meta {
  background: rgba(108, 84, 42, 0.34);
  color: #ffe1aa;
}

.request-records-shell-dark .request-records-route-key.is-out {
  background: rgba(64, 86, 146, 0.36);
  color: #dce7ff;
}

.request-records-shell-dark .request-records-app-pill {
  background: rgba(54, 76, 65, 0.94);
  color: #d8e8dd;
}

.request-records-shell-dark .request-records-route-pill {
  background: rgba(64, 86, 146, 0.36);
  color: #dce7ff;
}

.request-records-shell-dark .request-records-source-pill {
  background: rgba(57, 74, 65, 0.92);
  color: #d9e8dc;
}

.request-records-shell-dark .request-records-source-pill.is-fallback {
  background: rgba(113, 78, 43, 0.34);
  color: #ffd39a;
}

.request-records-shell-dark .request-records-source-pill.is-preference {
  background: rgba(88, 64, 136, 0.32);
  color: #e0d6ff;
}

.request-records-shell-dark .request-records-source-pill.is-direct {
  background: rgba(49, 92, 132, 0.34);
  color: #cfe5ff;
}

.request-records-shell-dark .request-records-board-title span {
  background: rgba(56, 74, 65, 0.92);
  color: #d8e8dc;
}

.request-records-shell-dark .request-records-action-button-refresh {
  border-color: rgba(118, 160, 133, 0.22);
  background: linear-gradient(180deg, rgba(43, 63, 51, 0.96), rgba(36, 54, 44, 0.92));
  color: #e0f1e4;
}

.request-records-shell-dark .request-records-action-button-clear {
  border-color: rgba(172, 97, 90, 0.24);
  background: linear-gradient(180deg, rgba(85, 46, 43, 0.94), rgba(68, 37, 35, 0.9));
  color: #ffd8d2;
}

.request-records-shell-dark .request-records-more {
  color: #c8d8cd;
}

.request-records-shell-dark .request-records-more:hover {
  background: rgba(255, 255, 255, 0.06);
  color: #f1f8f2;
}

.request-record-detail-shell-dark .request-record-detail-item,
.request-record-detail-shell-dark .request-record-detail-hero {
  border-color: rgba(133, 162, 145, 0.16);
  background:
    linear-gradient(180deg, rgba(28, 37, 32, 0.96), rgba(22, 30, 26, 0.94)),
    rgba(17, 24, 20, 0.92);
  box-shadow: 0 14px 28px rgba(0, 0, 0, 0.22);
}

.request-record-detail-shell-dark .request-record-detail-section-title,
.request-record-detail-shell-dark .request-record-detail-item span,
.request-record-detail-shell-dark .request-record-detail-hero-main small {
  color: #aebfb4;
}

.request-record-detail-shell-dark .request-record-detail-item strong,
.request-record-detail-shell-dark .request-record-detail-hero-main strong {
  color: #edf6ee;
}

.request-record-detail-shell-dark .request-record-detail-item pre {
  color: #b8c8be;
}

.request-record-detail-shell-dark .request-record-debug-result.is-loading {
  color: #b8c8be;
}

.request-record-detail-shell-dark .request-record-debug-result.is-success {
  color: #77d69a;
}

.request-record-detail-shell-dark .request-record-debug-result.is-error {
  color: #ff8d7d;
}

.request-record-detail-shell-dark .request-record-debug-textarea :deep(textarea) {
  color: #d3dfd6;
  background: rgba(18, 25, 21, 0.94);
  border-color: rgba(133, 162, 145, 0.16);
}

@keyframes request-records-pulse {
  0%,
  100% {
    transform: scale(1);
  }

  50% {
    transform: scale(1.15);
  }
}

@media (max-width: 560px) {
  .request-records-overview,
  .request-record-detail-grid {
    grid-template-columns: repeat(2, minmax(0, 1fr));
  }

  .request-records-board {
    min-height: 360px;
  }

  .request-records-table-wrap {
    min-height: 280px;
  }

  .request-records-toolbar,
  .request-records-board-head,
  .request-records-activity-head,
  .request-record-detail-hero {
    align-items: flex-start;
    flex-direction: column;
  }

  .request-records-toolbar-actions {
    width: 100%;
    justify-content: flex-end;
  }

  .request-records-activity-tabs,
  .request-records-activity-range {
    max-width: 100%;
    overflow-x: auto;
    scrollbar-width: none;
  }

  .request-records-activity-tabs::-webkit-scrollbar,
  .request-records-activity-range::-webkit-scrollbar {
    display: none;
  }

  .request-records-activity-summary {
    grid-template-columns: minmax(0, 1fr);
    gap: 10px;
  }

  .request-records-activity-summary div,
  .request-records-activity-summary div:first-child {
    padding: 0;
    border-left: 0;
  }

  .request-records-token-dashboard {
    grid-template-columns: minmax(0, 1fr);
  }

  .request-records-token-side {
    grid-template-columns: auto minmax(0, 1fr);
    justify-items: stretch;
  }

  .request-records-token-donut {
    width: 118px;
    height: 118px;
  }

  .request-records-token-donut-hole {
    width: 76px;
    height: 76px;
  }

  .request-records-token-donut-hole strong {
    font-size: 18px;
  }

  .mcp-skill-app-tabs {
    width: 100%;
    justify-content: center;
  }

  .mcp-skill-card {
    grid-template-columns: minmax(0, 1fr);
  }

  .mcp-skill-card-actions {
    justify-content: flex-start;
  }

  .terminal-sessions-board {
    grid-template-columns: minmax(0, 1fr);
  }

  .terminal-session-list-pane,
  .terminal-session-chat-pane {
    min-height: auto;
  }

  .terminal-session-list,
  .terminal-session-message-list {
    max-height: 360px;
  }
}
</style>

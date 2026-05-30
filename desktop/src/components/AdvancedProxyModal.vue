<template>
  <a-modal
    :open="open"
    title="高级代理功能"
    :width="modalWidth"
    :footer="null"
    :style="{ top: '10px' }"
    wrap-class-name="advanced-proxy-modal-wrap"
    @cancel="handleCancel"
  >
    <a-spin :spinning="loading || saving">
      <div ref="shellScrollRef" class="advanced-proxy-shell">
        <section class="advanced-proxy-hero">
          <div class="advanced-proxy-master-strip">
            <div class="advanced-proxy-master-row">
              <div class="advanced-proxy-master-copy">
                <strong>代理总开关</strong>
                <small>{{ proxyMasterEnabled ? (enabledAppLabels || '已启用') : '统一接管 Claude / Codex / OpenCode / OpenClaw' }}</small>
              </div>
              <a-switch
                :checked="proxyMasterEnabled"
                checked-children="开启"
                un-checked-children="关闭"
                @change="handleProxyMasterToggle"
              />
            </div>
            <div class="advanced-proxy-master-debug">
              <div class="advanced-proxy-master-debug-group">
                <a-tooltip
                  placement="top"
                  overlayClassName="advanced-proxy-master-help-tooltip"
                  trigger="click"
                  :open="masterHelpTooltipOpen"
                  :arrow="false"
                  :overlayStyle="{ position: 'fixed', top: '72px', right: '16px' }"
                  :overlayInnerStyle="{ maxWidth: 'min(50vw, 640px)', width: 'min(50vw, 640px)' }"
                  @openChange="handleMasterHelpTooltipOpenChange"
                >
                  <template #title>
                    <div class="advanced-proxy-master-help-tooltip-copy">
                      <img
                        :src="advancedProxyArchitectureLightSvg"
                        alt="高级代理架构图"
                        class="advanced-proxy-master-help-tooltip-image"
                      />
                      <div><code>claude</code> 入口会把 Anthropic Messages 请求转换到上游 Provider 定义的格式。</div>
                      <div><code>codex</code> / <code>opencode</code> / <code>openclaw</code> 入口会直接代理 OpenAI 兼容请求，并按各自的有效队列执行重试与熔断。</div>
                      <div>接管打开后，本地应用配置会写入本地代理地址；真实反代目标只保存在这里的 Provider 列表中。</div>
                      <div>如果要接管 Codex，请至少准备一条可用的 OpenAI 兼容上游，最好支持 <code>/v1/responses</code>。</div>
                    </div>
                  </template>
                  <button
                    type="button"
                    class="advanced-proxy-master-debug-button advanced-proxy-master-help-button advanced-proxy-master-side-icon-button"
                    :class="{ 'advanced-proxy-master-debug-button-active': masterHelpTooltipOpen }"
                    aria-label="查看使用说明"
                  >
                    <QuestionCircleOutlined class="advanced-proxy-master-side-icon" />
                  </button>
                </a-tooltip>
                <a-popover
                  placement="top"
                  overlayClassName="advanced-proxy-anti-poison-tooltip"
                  :open="antiPoisonTooltipOpen"
                  @openChange="handleAntiPoisonTooltipOpenChange"
                >
                  <template #content>
                    <div v-if="antiPoisonEnabled" class="advanced-proxy-anti-poison-tooltip-content">
                      <span>防投毒开启</span>
                      <button type="button" class="advanced-proxy-anti-poison-detail-link" @click.stop="openAntiPoisonPanel">
                        详情
                      </button>
                    </div>
                    <span v-else>防投毒关闭</span>
                  </template>
                  <button
                    type="button"
                    class="advanced-proxy-master-debug-button advanced-proxy-anti-poison-button"
                    :class="{ 'advanced-proxy-master-debug-button-active': antiPoisonEnabled }"
                    :aria-label="antiPoisonEnabled ? '关闭防投毒' : '开启防投毒'"
                    @click="handleAntiPoisonToggle"
                  >
                    <svg class="advanced-proxy-anti-poison-icon" viewBox="0 0 28 28" aria-hidden="true">
                      <path class="advanced-proxy-anti-poison-bottle" d="M10.8 4.8h6.4v4.3c2.5 1.1 4.2 3.6 4.2 6.5v5.7c0 1.1-.9 2-2 2H8.6c-1.1 0-2-.9-2-2v-5.7c0-2.9 1.7-5.4 4.2-6.5V4.8Z" />
                      <path class="advanced-proxy-anti-poison-neck" d="M10 4h8" />
                      <path class="advanced-proxy-anti-poison-skull" d="M10.5 15.3c0-2 1.5-3.4 3.5-3.4s3.5 1.4 3.5 3.4c0 1.2-.6 2.3-1.6 2.8v1.4h-3.8v-1.4c-1-.5-1.6-1.6-1.6-2.8Z" />
                      <circle class="advanced-proxy-anti-poison-eye" cx="12.7" cy="15.4" r=".7" />
                      <circle class="advanced-proxy-anti-poison-eye" cx="15.3" cy="15.4" r=".7" />
                      <path v-if="antiPoisonEnabled" class="advanced-proxy-anti-poison-cross" d="M6.4 6.4 21.6 21.6M21.6 6.4 6.4 21.6" />
                    </svg>
                  </button>
                </a-popover>
                <a-tooltip :title="draft.debugLogging ? '调试日志已开启，写入 advanced-proxy.log' : '开启调试日志，写入 advanced-proxy.log'">
                  <button
                    type="button"
                    class="advanced-proxy-master-debug-button advanced-proxy-master-side-icon-button"
                    :class="{ 'advanced-proxy-master-debug-button-active': draft.debugLogging }"
                    aria-label="切换调试日志"
                    @click="handleConfigMutation(next => { next.debugLogging = !next.debugLogging; }, draft.debugLogging ? '高级代理调试日志已关闭' : '高级代理调试日志已开启')"
                  >
                    <BugOutlined class="advanced-proxy-master-side-icon" />
                  </button>
                </a-tooltip>
              </div>
            </div>
            <div class="advanced-proxy-master-placeholder">
              <div class="advanced-proxy-master-stats-grid">
                <article class="advanced-proxy-master-stat">
                  <a-tooltip :title="enabledAppLabels || '当前未启用'">
                    <span>应用</span>
                  </a-tooltip>
                  <strong>{{ enabledAppCount }}</strong>
                </article>
                <article class="advanced-proxy-master-stat">
                  <a-tooltip :title="`${selectedQueueLabel}队列启用 ${enabledProviderCount} 条`">
                    <span>队列</span>
                  </a-tooltip>
                  <strong>{{ providerCount }}</strong>
                </article>
                <article class="advanced-proxy-master-stat">
                  <a-tooltip :title="selectedQueueScope === ADVANCED_PROXY_GLOBAL_QUEUE_SCOPE ? '未覆盖应用默认继承全局' : `${selectedQueueAppLabel} 当前有效兼容数`">
                    <span>兼容</span>
                  </a-tooltip>
                  <strong>{{ openAIProviderCount }}</strong>
                </article>
                <article class="advanced-proxy-master-stat">
                  <a-tooltip :title="`故障自动转移 ${unifiedFailoverEnabled ? '已开启' : '未开启'}`">
                    <span>熔断打开数</span>
                  </a-tooltip>
                  <strong>{{ openCircuitCount }}</strong>
                </article>
              </div>
            </div>
          </div>

          <div class="advanced-proxy-app-strip">
            <a-tooltip v-for="app in appCards" :key="app.id" placement="top">
              <template #title>
                <div class="advanced-proxy-app-tooltip">
                  <div>{{ app.modeLabel }}</div>
                  <div v-if="app.tooltipDetail">{{ app.tooltipDetail }}</div>
                </div>
              </template>
              <button
                type="button"
                class="advanced-proxy-app-token"
                :class="{ 'advanced-proxy-app-token-active': app.enabled }"
                @click="handleAppTakeoverToggle(app.id, !app.enabled)"
              >
                <span class="advanced-proxy-app-icon-shell" :class="`advanced-proxy-app-icon-shell-${app.id}`">
                  <img :src="app.icon" :alt="app.label" class="advanced-proxy-app-icon-image" />
                </span>
              </button>
            </a-tooltip>
          </div>
        </section>

        <div class="advanced-proxy-layout">
          <section ref="queuePanelRef" class="advanced-proxy-section">
            <div class="advanced-proxy-section-head">
              <div>
                <h4>{{ queuePanelTitle }}</h4>
                <p>{{ queuePanelDescription }}</p>
              </div>
              <div class="advanced-proxy-queue-toolbar">
                <a-select
                  class="advanced-proxy-queue-select"
                  :value="selectedQueueScope"
                  :options="queueScopeOptions"
                  @change="handleQueueScopeChange"
                />
                <a-tooltip :title="quickSetupTooltipText">
                  <a-button
                    class="advanced-proxy-toolbar-icon-button advanced-proxy-toolbar-icon-button-provider-queue"
                    :disabled="!validProviderCandidateCards.length"
                    @click="handleQuickSelectValidProviders"
                  >
                    <template #icon>
                      <QueueOrbitIcon class="provider-queue-icon" />
                    </template>
                  </a-button>
                </a-tooltip>
                <a-tooltip
                  v-if="selectedQueueScope !== ADVANCED_PROXY_GLOBAL_QUEUE_SCOPE"
                  :title="selectedQueueInheritGlobal ? '当前已经在跟随全局队列' : '切换为跟随全局队列'"
                >
                  <a-button
                    class="advanced-proxy-toolbar-icon-button"
                    :disabled="selectedQueueInheritGlobal"
                    @click="handleFollowGlobalQueue"
                  >
                    <template #icon>
                      <CloudSyncOutlined />
                    </template>
                  </a-button>
                </a-tooltip>
                <a-tooltip title="刷新记录">
                  <a-button class="advanced-proxy-toolbar-icon-button" @click="handleRefreshQueueContext">
                    <template #icon>
                      <ReloadOutlined />
                    </template>
                  </a-button>
                </a-tooltip>
              </div>
            </div>

            <div v-if="selectedQueueScope !== ADVANCED_PROXY_GLOBAL_QUEUE_SCOPE" class="advanced-proxy-queue-mode">
              <a-tag :color="selectedQueueInheritGlobal ? 'gold' : 'green'">
                {{ selectedQueueInheritGlobal ? '当前继承全局' : '当前使用独立队列' }}
              </a-tag>
              <span>
                {{ selectedQueueInheritGlobal
                  ? '点任意卡片后会先复制全局队列，再切换为当前应用的独立队列。'
                  : '当前应用会优先使用自己的 Provider 队列，不再继承全局。' }}
              </span>
            </div>

            <div class="advanced-proxy-provider-pool">
              <div class="advanced-proxy-provider-panel-grid">
                <button
                  v-for="item in providerCandidateCards"
                  :key="item.id"
                  type="button"
                  class="advanced-proxy-provider-panel"
                  :class="{ 'advanced-proxy-provider-panel-active': item.selected }"
                  @click="toggleProviderQueue(item)"
                >
                  <div class="advanced-proxy-provider-panel-top">
                    <strong class="advanced-proxy-provider-panel-title">{{ item.siteName }}</strong>
                    <span v-if="item.queueOrder" class="advanced-proxy-provider-order">P{{ item.queueOrder }}</span>
                  </div>
                  <div class="advanced-proxy-provider-panel-model">{{ item.modelLabel }}</div>
                  <div class="advanced-proxy-provider-panel-meta">
                    <span v-if="item.skLabel" class="advanced-proxy-provider-chip">{{ item.skLabel }}</span>
                    <span v-if="item.orphaned" class="advanced-proxy-provider-chip advanced-proxy-provider-chip-muted">已不在密钥管理中</span>
                  </div>
                </button>
              </div>

              <div v-if="providerCandidateCards.length && !providerCount" class="advanced-proxy-empty advanced-proxy-empty-compact">
                {{ queuePanelEmptyText }}
              </div>
            </div>

            <div v-if="!providerCandidateCards.length" class="advanced-proxy-empty">
              还没有可用 Provider。先从密钥管理中加入至少一条记录，再在这里点击卡片组成队列。
            </div>
          </section>

          <aside class="advanced-proxy-side">
                      <section class="advanced-proxy-section">
              <div class="advanced-proxy-section-head">
                <div>
                  <h4>请求策略与并发</h4>
                  <p>分发策略负责挑选请求路径，RPM 在下方按供应商单独设定。</p>
                </div>
              </div>

              <div class="advanced-proxy-ha-grid">
                <div class="advanced-proxy-radio-card">
                  <label class="advanced-proxy-compact-label">请求分发策略</label>
                  <a-radio-group
                    class="advanced-proxy-radio-group"
                    :value="draft.highAvailability.dispatchMode"
                    @change="handleDispatchModeChange"
                  >
                    <a-radio-button
                      v-for="option in dispatchModeOptions"
                      :key="option.value"
                      :value="option.value"
                    >
                      {{ option.label }}
                    </a-radio-button>
                  </a-radio-group>
                  <p class="advanced-proxy-radio-hint">{{ selectedDispatchModeDescription }}</p>
                </div>
                <div class="advanced-proxy-inline-control advanced-proxy-ha-toggle-card">
                  <div class="advanced-proxy-ha-toggle-copy">
                    <label class="advanced-proxy-compact-label">高可用智能调度</label>
                    <p class="advanced-proxy-radio-hint">启用后按健康度、实时负载和当前 RPM 限制执行请求分发。</p>
                  </div>
                  <a-switch
                    :checked="highAvailabilityEnabled"
                    @change="handleHighAvailabilityToggle"
                  />
                </div>
              </div>

              <div class="advanced-proxy-inline-control advanced-proxy-rpm-row">
                <a-tooltip placement="topLeft">
                  <template #title>
                    <div class="advanced-proxy-tooltip">
                      <span>0 表示不限制。</span>
                      <span>默认从“全局”读取；选择某个 provider 后会优先使用该供应商的值。</span>
                      <span>下拉框包含全局和当前队列中的 provider。</span>
                    </div>
                  </template>
                  <span class="advanced-proxy-inline-label">RPM 设置</span>
                </a-tooltip>
                <div class="advanced-proxy-rpm-controls">
                  <a-select
                    class="advanced-proxy-rpm-select"
                    :value="selectedHighAvailabilityRpmProviderKey"
                    :options="rpmProviderOptions"
                    popupClassName="advanced-proxy-rpm-dropdown"
                    @change="handleHighAvailabilityRpmProviderChange"
                  />
                  <a-input-number
                    class="advanced-proxy-rpm-input"
                    :value="selectedHighAvailabilityRpmValue"
                    :min="0"
                    :precision="0"
                    :step="1"
                    @change="handleHighAvailabilityRpmValueChange"
                  />
                </div>
              </div>
            </section>
            <section class="advanced-proxy-section">
              <div class="advanced-proxy-section-head">
                <div>
                  <h4>故障转移</h4>
                  <p>代理入口会按所属应用挑选自己的有效队列；未单独覆盖的应用会继续继承全局队列。</p>
                </div>
              </div>

              <div class="advanced-proxy-inline-grid">
                <div class="advanced-proxy-inline-control">
                  <span class="advanced-proxy-inline-label">故障自动转移</span>
                  <a-switch
                    :checked="unifiedFailoverEnabled"
                    @change="handleUnifiedFailoverToggle"
                  />
                </div>
                <div class="advanced-proxy-inline-control">
                  <span class="advanced-proxy-inline-label">动态优化队列（仅基于故障率调整队列）</span>
                  <a-switch
                    :checked="draft.highAvailability.dynamicOptimizeQueue"
                    @change="value => handleHighAvailabilityFieldMutation('dynamicOptimizeQueue', value)"
                  />
                </div>
              </div>

              <div class="advanced-proxy-dense-rows">
                <div class="advanced-proxy-triple-row">
                  <div class="advanced-proxy-compact-field">
                    <label class="advanced-proxy-compact-label">最大重试次数</label>
                    <a-input-number class="advanced-proxy-short-number" :value="draft.failover.maxRetries" :min="0" :max="10" @change="value => handleFailoverFieldMutation('maxRetries', value)" />
                  </div>
                  <div class="advanced-proxy-compact-field">
                    <label class="advanced-proxy-compact-label">流式首字节超时</label>
                    <a-input-number class="advanced-proxy-short-number" :value="draft.failover.streamingFirstByteTimeout" :min="5" :max="300" @change="value => handleFailoverFieldMutation('streamingFirstByteTimeout', value)" />
                  </div>
                  <div class="advanced-proxy-compact-field">
                    <label class="advanced-proxy-compact-label">流式空闲超时</label>
                    <a-input-number class="advanced-proxy-short-number" :value="draft.failover.streamingIdleTimeout" :min="5" :max="600" @change="value => handleFailoverFieldMutation('streamingIdleTimeout', value)" />
                  </div>
                </div>
                <div class="advanced-proxy-triple-row">
                  <div class="advanced-proxy-compact-field">
                    <label class="advanced-proxy-compact-label">非流式超时</label>
                    <a-input-number class="advanced-proxy-short-number" :value="draft.failover.nonStreamingTimeout" :min="5" :max="600" @change="value => handleFailoverFieldMutation('nonStreamingTimeout', value)" />
                  </div>
                  <div class="advanced-proxy-compact-field">
                    <label class="advanced-proxy-compact-label">熔断失败阈值</label>
                    <a-input-number class="advanced-proxy-short-number" :value="draft.failover.circuitFailureThreshold" :min="1" :max="20" @change="value => handleFailoverFieldMutation('circuitFailureThreshold', value)" />
                  </div>
                  <div class="advanced-proxy-compact-field">
                    <label class="advanced-proxy-compact-label">恢复成功阈值</label>
                    <a-input-number class="advanced-proxy-short-number" :value="draft.failover.circuitSuccessThreshold" :min="1" :max="20" @change="value => handleFailoverFieldMutation('circuitSuccessThreshold', value)" />
                  </div>
                </div>
                <div class="advanced-proxy-triple-row">
                  <div class="advanced-proxy-compact-field">
                    <label class="advanced-proxy-compact-label">熔断恢复等待</label>
                    <a-input-number class="advanced-proxy-short-number" :value="draft.failover.circuitTimeoutSeconds" :min="5" :max="600" @change="value => handleFailoverFieldMutation('circuitTimeoutSeconds', value)" />
                  </div>
                  <div class="advanced-proxy-compact-field">
                    <label class="advanced-proxy-compact-label">错误率阈值</label>
                    <a-input-number class="advanced-proxy-short-number" :value="draft.failover.circuitErrorRateThreshold" :min="0.1" :max="1" :step="0.05" @change="value => handleFailoverFieldMutation('circuitErrorRateThreshold', value)" />
                  </div>
                  <div class="advanced-proxy-compact-field">
                    <label class="advanced-proxy-compact-label">最小请求数</label>
                    <a-input-number class="advanced-proxy-short-number" :value="draft.failover.circuitMinRequests" :min="1" :max="100" @change="value => handleFailoverFieldMutation('circuitMinRequests', value)" />
                  </div>
                </div>
              </div>
            </section>

            <section class="advanced-proxy-section">
              <div class="advanced-proxy-section-head">
                <div>
                  <h4>错误修正</h4>
                  <p>仅作用于 Claude 兼容链路，用来最小化修正常见的 thinking 签名和预算错误。</p>
                </div>
              </div>

              <div class="advanced-proxy-toggle-list">
                <div class="advanced-proxy-toggle-row">
                  <span>总开关</span>
                  <a-switch
                    :checked="draft.rectifier.enabled"
                    @change="value => handleConfigMutation(next => { next.rectifier.enabled = value; }, '错误修正总开关已更新')"
                  />
                </div>
                <div class="advanced-proxy-toggle-row">
                  <span>修正 thinking signature</span>
                  <a-switch
                    :checked="draft.rectifier.requestThinkingSignature"
                    @change="value => handleConfigMutation(next => { next.rectifier.requestThinkingSignature = value; }, 'thinking signature 修正开关已更新')"
                  />
                </div>
                <div class="advanced-proxy-toggle-row">
                  <span>修正 thinking budget</span>
                  <a-switch
                    :checked="draft.rectifier.requestThinkingBudget"
                    @change="value => handleConfigMutation(next => { next.rectifier.requestThinkingBudget = value; }, 'thinking budget 修正开关已更新')"
                  />
                </div>
              </div>
            </section>

          </aside>
        </div>
      </div>
    </a-spin>
  </a-modal>

  <DesktopConfigDiffModal :open="previewOpen" :preview="configPreview" @cancel="cancelPreview" @confirm="applyPreview" />

  <a-drawer
    :open="antiPoisonPanelOpen"
    title="防投毒详情"
    placement="right"
    :width="antiPoisonDrawerWidth"
    :zIndex="1200"
    class="advanced-proxy-anti-poison-drawer"
    @close="antiPoisonPanelOpen = false"
  >
    <div class="advanced-proxy-anti-poison-panel">
      <section class="advanced-proxy-anti-poison-hero-card">
        <div>
          <span class="advanced-proxy-anti-poison-kicker">Prompt Injection Guard</span>
          <h3>防投毒守卫</h3>
          <p>用于配置动态工具链水印策略、随机变化算法 Prompt 和回流校验统计。非流式代理会执行网关校验；流式请求暂只写绕过日志。</p>
        </div>
        <div class="advanced-proxy-anti-poison-state" :class="{ 'is-active': antiPoisonEnabled }">
          {{ antiPoisonEnabled ? '已开启' : '未开启' }}
        </div>
      </section>

      <section class="advanced-proxy-anti-poison-card">
        <div class="advanced-proxy-anti-poison-card-head">
          <div>
            <h4>设置</h4>
            <p>控制防投毒检查强度和命中后的处理方式。</p>
          </div>
        </div>
        <div class="advanced-proxy-anti-poison-settings">
          <div class="advanced-proxy-anti-poison-setting-row">
            <div>
              <strong>防投毒开关</strong>
              <span>关闭后不显示红叉，也不启用相关策略。</span>
            </div>
            <a-switch :checked="antiPoisonEnabled" @change="handleAntiPoisonEnabledChange" />
          </div>
          <div class="advanced-proxy-anti-poison-setting-row">
            <div>
              <strong>严格模式</strong>
              <span>有真实 toolcall 但缺少合法 guard JSON 时直接拒绝。</span>
            </div>
            <a-switch :checked="antiPoisonConfig.strictMode" :disabled="!antiPoisonEnabled" @change="value => handleAntiPoisonFieldChange('strictMode', value)" />
          </div>
          <div class="advanced-proxy-anti-poison-setting-row">
            <div>
              <strong>失败处理方式</strong>
              <span>校验失败后阻断回流，或只写日志告警。</span>
            </div>
            <a-segmented
              :value="antiPoisonConfig.failureMode"
              :disabled="!antiPoisonEnabled"
              :options="[
                { label: '阻断', value: 'block' },
                { label: '告警', value: 'warn' },
              ]"
              @change="value => handleAntiPoisonFieldChange('failureMode', value)"
            />
          </div>
          <div class="advanced-proxy-anti-poison-setting-row">
            <div>
              <strong>字符串保护</strong>
              <span>转发给上游前把配置/密钥样式字符串替换为占位符，回客户端前再还原。</span>
            </div>
            <a-switch
              :checked="antiPoisonStringProtectionEnabled"
              :disabled="!antiPoisonEnabled"
              @change="handleAntiPoisonStringProtectionEnabledChange"
            />
          </div>
        </div>
      </section>

      <section class="advanced-proxy-anti-poison-card">
        <div class="advanced-proxy-anti-poison-card-head advanced-proxy-anti-poison-card-head-actions">
          <div>
            <h4>字符串保护规则</h4>
            <p>一行一个规则描述，冒号后为正则；主要拦截读取配置文件、点号文件、JSON key 和密钥字符串后的注入文本。</p>
          </div>
          <div class="advanced-proxy-anti-poison-actions">
            <button type="button" class="advanced-proxy-anti-poison-soft-button" @click="antiPoisonRulesOpen = !antiPoisonRulesOpen">
              {{ antiPoisonRulesOpen ? '收起规则' : '展开规则' }}
            </button>
            <button type="button" class="advanced-proxy-anti-poison-soft-button" @click="resetAntiPoisonStringProtectionRules">
              重置规则
            </button>
          </div>
        </div>
        <div class="advanced-proxy-anti-poison-rule-summary">
          <span v-for="item in antiPoisonStringProtectionRuleSummary" :key="item.label">
            <strong>{{ item.value }}</strong>
            {{ item.label }}
          </span>
        </div>
        <a-textarea
          v-if="antiPoisonRulesOpen"
          class="advanced-proxy-anti-poison-textarea advanced-proxy-anti-poison-rules-textarea"
          :value="antiPoisonStringProtectionRulesText"
          :disabled="!antiPoisonEnabled || !antiPoisonStringProtectionEnabled"
          :auto-size="{ minRows: 7, maxRows: 12 }"
          @change="event => handleAntiPoisonStringProtectionRulesChange(event?.target?.value)"
        />
      </section>

      <section class="advanced-proxy-anti-poison-card">
        <div class="advanced-proxy-anti-poison-card-head advanced-proxy-anti-poison-card-head-actions">
          <div>
            <h4>策略 Prompt</h4>
            <p>描述何时生成 guard JSON，以及这些 guard JSON 如何被网关摘除。</p>
          </div>
          <div class="advanced-proxy-anti-poison-actions">
            <a-button size="small" @click="resetAntiPoisonPromptsToDefault">恢复默认策略</a-button>
            <a-button size="small" type="primary" @click="antiPoisonPreviewOpen = !antiPoisonPreviewOpen">预览随机展开</a-button>
          </div>
        </div>
        <a-textarea
          class="advanced-proxy-anti-poison-textarea"
          :value="antiPoisonConfig.strategyPrompt"
          :auto-size="{ minRows: 5, maxRows: 9 }"
          @change="event => handleAntiPoisonFieldChange('strategyPrompt', event?.target?.value)"
        />
      </section>

      <section class="advanced-proxy-anti-poison-card">
        <div class="advanced-proxy-anti-poison-card-head">
          <div>
            <h4>随机变化算法 Prompt</h4>
            <p><code>{{ antiPoisonAliasPlaceholder }}</code> 只是上下文关联代号，用来把策略段和算法段绑定起来。</p>
          </div>
        </div>
        <a-textarea
          class="advanced-proxy-anti-poison-textarea"
          :value="antiPoisonConfig.algorithmPrompt"
          :auto-size="{ minRows: 5, maxRows: 9 }"
          @change="event => handleAntiPoisonFieldChange('algorithmPrompt', event?.target?.value)"
        />
      </section>

      <section class="advanced-proxy-anti-poison-card">
        <div class="advanced-proxy-anti-poison-card-head">
          <div>
            <h4>随机化可视</h4>
            <p>二级随机策略由内置策略池控制，仅展示，不开放细调，避免破坏默认防护结构。</p>
          </div>
        </div>
        <div class="advanced-proxy-anti-poison-random-grid">
          <article v-for="item in antiPoisonRandomizationCards" :key="item.label" class="advanced-proxy-anti-poison-random-card">
            <span>{{ item.label }}</span>
            <strong>{{ item.value }}</strong>
          </article>
        </div>
        <pre v-if="antiPoisonPreviewOpen" class="advanced-proxy-anti-poison-preview">{{ antiPoisonPreviewText }}</pre>
      </section>

      <section class="advanced-proxy-anti-poison-card">
        <div class="advanced-proxy-anti-poison-card-head">
          <div>
            <h4>流水统计</h4>
            <p>按 request out / respond in 记录本轮网关逻辑操作；完整校验明细写入 advanced-proxy.log。</p>
          </div>
          <div class="advanced-proxy-anti-poison-actions">
            <button type="button" class="advanced-proxy-anti-poison-soft-button" @click="reloadAntiPoisonRecords">
              刷新流水
            </button>
          </div>
        </div>
        <div class="advanced-proxy-anti-poison-flow-table">
          <table>
            <thead>
              <tr>
                <th>时间</th>
                <th>阶段</th>
                <th>通路</th>
                <th>规则/逻辑</th>
                <th>路径</th>
                <th>before</th>
                <th>after</th>
                <th>数量</th>
              </tr>
            </thead>
            <tbody>
              <tr v-for="row in antiPoisonOperationRows" :key="row.id" :class="{ 'advanced-proxy-anti-poison-blocked-row': row.blocked }">
                <td class="advanced-proxy-anti-poison-time-cell">{{ row.time }}</td>
                <td><span class="advanced-proxy-anti-poison-stage-pill" :class="{ 'is-blocked': row.blocked }">{{ row.stage }}</span></td>
                <td>{{ row.channel }}</td>
                <td>
                  <div class="advanced-proxy-anti-poison-rule-cell">
                    <span>{{ row.rule }}</span>
                    <button type="button" class="advanced-proxy-anti-poison-row-detail-button" @click.stop="showAntiPoisonOperationDetail(row)">
                      详情
                    </button>
                  </div>
                </td>
                <td>{{ row.path }}</td>
                <td><code>{{ row.before }}</code></td>
                <td><code>{{ row.after }}</code></td>
                <td>{{ row.count }}</td>
              </tr>
              <tr v-if="!antiPoisonOperationRows.length">
                <td colspan="8" class="advanced-proxy-anti-poison-empty-row">
                  暂无流水；开启防投毒并产生代理请求后显示。
                </td>
              </tr>
            </tbody>
          </table>
        </div>
      </section>
    </div>
  </a-drawer>
</template>

<script setup>
import { computed, h, nextTick, reactive, ref, watch } from 'vue';
import { BugOutlined, CloudSyncOutlined, QuestionCircleOutlined, ReloadOutlined } from '@ant-design/icons-vue';
import { message, Modal } from 'ant-design-vue';
import advancedProxyArchitectureLightSvg from '../../docs/images/advanced-proxy-architecture-light.svg';
import claudeAppIcon from '../assets/app-icons/claude.svg';
import codexAppIcon from '../assets/app-icons/codex.svg';
import opencodeAppIcon from '../assets/app-icons/opencode.svg';
import openclawAppIcon from '../assets/app-icons/openclaw-fallback.svg';
import DesktopConfigDiffModal from './DesktopConfigDiffModal.vue';
import QueueOrbitIcon from './icons/QueueOrbitIcon.vue';
import { applyManagedAppConfigFiles, isDesktopConfigBridgeAvailable, readManagedAppConfigFiles } from '../utils/desktopConfigBridge.js';
import { buildDesktopConfigPreview, createDesktopConfigDraft } from '../utils/desktopConfigTransform.js';
import { loadPanelRecords } from '../utils/keyPanelStore.js';
import {
  ADVANCED_PROXY_APPS,
  ADVANCED_PROXY_GLOBAL_QUEUE_SCOPE,
  ADVANCED_PROXY_QUEUE_SCOPES,
  DEFAULT_ANTI_POISON_ALGORITHM_PROMPT,
  DEFAULT_ANTI_POISON_RANDOMIZATION,
  DEFAULT_ANTI_POISON_STRING_PROTECTION,
  DEFAULT_ANTI_POISON_STRATEGY_PROMPT,
  getAdvancedProxyAppBaseUrl,
  getAdvancedProxyConfig,
  getAdvancedProxyEffectiveProviders,
  getAdvancedProxyQueueProviders,
  getCircuitBreakerStats,
  listAdvancedProxyRequestRecords,
  normalizeAdvancedProxyConfig,
  resetCircuitBreaker,
  setAdvancedProxyConfig,
  syncAdvancedProxyProvidersFromRecords,
} from '../utils/advancedProxyBridge.js';
import { logClientDiagnostic } from '../utils/clientDiagnostics.js';

const EMPTY_PREVIEW = { appGroups: [], writes: [], errors: [] };
const modalWidth = 'min(1180px, calc(100vw - 24px))';
const PROXY_MANAGED_TOKEN = 'PROXY_MANAGED';
const ADVANCED_PROXY_PROVIDER_NAME = 'AllApiDeck Advanced Proxy';
const ADVANCED_PROXY_APP_ICONS = {
  claude: claudeAppIcon,
  codex: codexAppIcon,
  opencode: opencodeAppIcon,
  openclaw: openclawAppIcon,
};
const DISPATCH_MODE_OPTIONS = [
  { value: 'fixed', label: '固定', description: '按当前队列顺序执行，不额外重排。' },
  { value: 'ordered', label: '顺序', description: '按健康度、负载和 RPM 约束动态排队。' },
  { value: 'random', label: '随机', description: '在满足约束的候选中随机分散压力。' },
];

const props = defineProps({
  open: {
    type: Boolean,
    default: false,
  },
  initialQueueScope: {
    type: String,
    default: ADVANCED_PROXY_GLOBAL_QUEUE_SCOPE,
  },
  focusQueueToken: {
    type: [String, Number],
    default: '',
  },
});

const emit = defineEmits(['update:open']);

const loading = ref(false);
const saving = ref(false);
const previewOpen = ref(false);
const masterHelpTooltipOpen = ref(false);
const antiPoisonTooltipOpen = ref(false);
const antiPoisonPanelOpen = ref(false);
const antiPoisonPreviewOpen = ref(false);
const antiPoisonRulesOpen = ref(false);
const antiPoisonRequestRecords = ref([]);
const shellScrollRef = ref(null);
const queuePanelRef = ref(null);
const selectedQueueScope = ref(ADVANCED_PROXY_GLOBAL_QUEUE_SCOPE);
const selectedHighAvailabilityRpmProviderKey = ref(ADVANCED_PROXY_GLOBAL_QUEUE_SCOPE);
const availableRecords = ref([]);
const breakerStatsMap = ref({});
const loadedConfigSnapshot = ref(normalizeAdvancedProxyConfig({}));
const pendingSaveConfig = ref(null);
const pendingManagedWrites = ref([]);
const pendingWriteOrder = ref('config-first');
const pendingSuccessMessage = ref('高级代理配置已更新');
const configPreview = ref(EMPTY_PREVIEW);
const lastEnabledAppIds = ref([]);
const draft = reactive(normalizeAdvancedProxyConfig({}));
const queueScopeOptions = ADVANCED_PROXY_QUEUE_SCOPES.map(item => ({
  value: item.id,
  label: item.label,
}));

const enabledAppIds = computed(() => getEnabledAppIds(draft));
const enabledAppCount = computed(() => enabledAppIds.value.length);
const enabledAppLabels = computed(() =>
  ADVANCED_PROXY_APPS
    .filter(app => enabledAppIds.value.includes(app.id))
    .map(app => app.label)
    .join(' / ')
);
const highAvailabilityEnabled = computed(() => draft?.highAvailability?.enabled === true);
const unifiedFailoverEnabled = computed(() =>
  draft?.failover?.enabled === true && draft?.failover?.autoFailoverEnabled === true
);
const antiPoisonConfig = computed(() => draft?.antiPoison || normalizeAdvancedProxyConfig({}).antiPoison);
const antiPoisonEnabled = computed(() => antiPoisonConfig.value?.enabled === true);
const antiPoisonStringProtectionEnabled = computed(() => antiPoisonConfig.value?.stringProtection?.enabled !== false);
const antiPoisonStringProtectionRulesText = computed(() => {
  const rules = Array.isArray(antiPoisonConfig.value?.stringProtection?.rules)
    ? antiPoisonConfig.value.stringProtection.rules
    : DEFAULT_ANTI_POISON_STRING_PROTECTION.rules;
  return rules.join('\n');
});
const antiPoisonStringProtectionRuleSummary = computed(() => {
  const rules = Array.isArray(antiPoisonConfig.value?.stringProtection?.rules)
    ? antiPoisonConfig.value.stringProtection.rules
    : DEFAULT_ANTI_POISON_STRING_PROTECTION.rules;
  return [
    {
      label: '条规则',
      value: rules.length,
    },
    {
      label: '字段名命中',
      value: rules.filter(rule => String(rule || '').toLowerCase().includes('key:')).length,
    },
    {
      label: '文本正则',
      value: rules.filter(rule => !String(rule || '').toLowerCase().includes('key:')).length,
    },
  ];
});
const antiPoisonAliasPlaceholder = '{{ALGORITHM_ALIAS}}';
const dispatchModeOptions = DISPATCH_MODE_OPTIONS;
const selectedDispatchModeDescription = computed(() =>
  DISPATCH_MODE_OPTIONS.find(option => option.value === draft?.highAvailability?.dispatchMode)?.description
  || DISPATCH_MODE_OPTIONS[0].description
);
const selectedHighAvailabilityRpmValue = computed(() => {
  const rpm = draft?.highAvailability?.rpm || {};
  const providerKey = selectedHighAvailabilityRpmProviderKey.value;
  if (providerKey === ADVANCED_PROXY_GLOBAL_QUEUE_SCOPE) {
    return Number.isFinite(Number(rpm.global)) ? Number(rpm.global) : 0;
  }
  const providerValue = rpm?.providers?.[providerKey];
  if (providerValue == null || providerValue === '') {
    return Number.isFinite(Number(rpm.global)) ? Number(rpm.global) : 0;
  }
  return Number.isFinite(Number(providerValue)) ? Number(providerValue) : 0;
});
const proxyMasterEnabled = computed(() => enabledAppIds.value.length > 0);
const antiPoisonDrawerWidth = computed(() => {
  const viewportWidth = typeof window === 'undefined' ? 1040 : Number(window.innerWidth || 1040);
  return Math.min(Math.max(viewportWidth - 24, 320), 1040);
});
const antiPoisonOperationRows = computed(() => {
  const rows = [];
  const records = Array.isArray(antiPoisonRequestRecords.value) ? antiPoisonRequestRecords.value : [];
  records.forEach(record => {
    const ops = Array.isArray(record?.antiPoisonOps) ? record.antiPoisonOps : [];
    ops.forEach((op, index) => {
      const row = {
        id: String(op?.id || `${record?.id || 'record'}-${index}`),
        time: formatAntiPoisonOperationTime(op?.time || record?.recordedAt),
        stage: String(op?.stage || '-'),
        channel: String(op?.channel || record?.appType || '-'),
        rule: String(op?.rule || op?.reason || 'gateway'),
        path: String(op?.path || op?.route || record?.outboundRoute || '-'),
        before: String(op?.before || '-'),
        after: String(op?.after || '-'),
        count: Number(op?.count || 0),
        blocked: op?.blocked === true || String(op?.reason || '').includes('anti_poison'),
      };
      row.detailText = buildAntiPoisonOperationDetailText(row, record, op, index);
      rows.push(row);
    });
  });
  return rows.slice(0, 40);
});
const antiPoisonRandomizationCards = computed(() => {
  const randomization = antiPoisonConfig.value?.randomization || DEFAULT_ANTI_POISON_RANDOMIZATION;
  return [
    { label: '策略池', value: `${randomization.strategyPoolSize || DEFAULT_ANTI_POISON_RANDOMIZATION.strategyPoolSize} 个策略` },
    { label: '句式变体', value: `每策略 ${randomization.minPhraseVariantsPerStrategy || DEFAULT_ANTI_POISON_RANDOMIZATION.minPhraseVariantsPerStrategy}+ 种` },
    { label: '前置约束', value: '替换工具前说明' },
    { label: 'guard 结构', value: '仅 name/tool_name' },
    { label: '工具覆盖', value: '每次 toolcall 一个' },
    { label: '命名前缀', value: '每轮随机生成' },
  ];
});
const antiPoisonPreviewText = computed(() => {
  const alias = 'APTX_7F3A91C2';
  const prefix = 'aad_guard_51e2b7a903';
  const guardToolName = `${prefix}_<original_tool_name>`;
  const guardToolExample = `${prefix}_WebSearch`;
  const guardJsonTag = 'aad_guard_json';
  const strategyPrompt = String(antiPoisonConfig.value?.strategyPrompt || DEFAULT_ANTI_POISON_STRATEGY_PROMPT)
    .replaceAll('{{ALGORITHM_ALIAS}}', alias);
  const algorithmPrompt = String(antiPoisonConfig.value?.algorithmPrompt || DEFAULT_ANTI_POISON_ALGORITHM_PROMPT)
    .replaceAll('{{ALGORITHM_ALIAS}}', alias);
  return [
    '[AllApiDeck 防投毒随机策略]',
    `[随机变化算法代号] ${alias}`,
    `[guard name prefix] ${prefix}`,
    `[guard tool name] ${guardToolName}`,
    '[策略槽] 07',
    '[句式变体] 03',
    '[插入点位提示] fixed_prepend',
    '',
    '[策略 Prompt]',
    strategyPrompt,
    '',
    '[随机变化算法 Prompt]',
    algorithmPrompt,
    '',
    '[Gateway validation contract]',
    'If this turn emits any real toolcall, the model must first emit one guard JSON text block, then emit the corresponding real toolcall.',
    'Do not emit ordinary pre-tool explanations or progress narration such as I will search; replace that pre-tool sentence with guard JSON.',
    `guard JSON text blocks must be wrapped as <${guardJsonTag}>...</${guardJsonTag}>.`,
    `guard JSON name must follow ${guardToolName}; for WebSearch it is ${guardToolExample}.`,
    'guard JSON only requires the minimal binding fields: name and tool_name.',
    'tool_name must equal the immediately following real tool name.',
    'Do not include digest, chain, cover, nonce, algorithm, or tool_type.',
    'After validation, the gateway strips guard JSON before returning to the client.',
  ].join('\n');
});
const selectedQueueLabel = computed(() =>
  ADVANCED_PROXY_QUEUE_SCOPES.find(item => item.id === selectedQueueScope.value)?.label || '全局'
);
const selectedQueueAppLabel = computed(() =>
  ADVANCED_PROXY_APPS.find(app => app.id === selectedQueueScope.value)?.label || '全局'
);
const selectedQueueInheritGlobal = computed(() =>
  selectedQueueScope.value !== ADVANCED_PROXY_GLOBAL_QUEUE_SCOPE
  && draft?.queues?.[selectedQueueScope.value]?.inheritGlobal === true
);
const displayedQueueProviders = computed(() =>
  getAdvancedProxyQueueProviders(draft, selectedQueueScope.value, {
    effective: selectedQueueScope.value !== ADVANCED_PROXY_GLOBAL_QUEUE_SCOPE,
  })
);
const rpmProviderOptions = computed(() => {
  const options = [{
    value: ADVANCED_PROXY_GLOBAL_QUEUE_SCOPE,
    label: '全局',
  }];

  displayedQueueProviders.value.forEach((provider, index) => {
    const key = String(provider?.rowKey || provider?.id || '').trim();
    if (!key) return;
    const name = String(provider?.name || provider?.baseUrl || `Provider ${index + 1}`).trim() || `Provider ${index + 1}`;
    const model = String(provider?.model || '').trim();
    options.push({
      value: key,
      label: `${index + 1}. ${name}${model ? ` · ${model}` : ''}`,
    });
  });

  return options;
});
const providerCount = computed(() => displayedQueueProviders.value.length);
const enabledProviderCount = computed(() => displayedQueueProviders.value.filter(provider => provider?.enabled !== false).length);
const openAIProviderCount = computed(() =>
  displayedQueueProviders.value.filter(
    provider => provider?.enabled !== false && String(provider?.apiFormat || '').trim().toLowerCase() !== 'anthropic',
  ).length
);
const breakerAppIdsForSummary = computed(() => {
  if (selectedQueueScope.value === ADVANCED_PROXY_GLOBAL_QUEUE_SCOPE) {
    return enabledAppIds.value;
  }
  return [selectedQueueScope.value];
});
const openCircuitCount = computed(() =>
  displayedQueueProviders.value.filter(provider =>
    breakerAppIdsForSummary.value.some(appId => getBreakerStateLabel(provider.id, appId) === 'open')
  ).length
);
const queuePanelTitle = computed(() => `[${selectedQueueLabel.value}]上游 Provider 队列`);
const queuePanelDescription = computed(() => {
  if (selectedQueueScope.value === ADVANCED_PROXY_GLOBAL_QUEUE_SCOPE) {
    return '点击卡片配置代理全局队列和调度优先级。';
  }
  if (selectedQueueInheritGlobal.value) {
    return `${selectedQueueAppLabel.value} 当前继承全局队列。点击卡片后会自动复制出独立队列，并按点击顺序维护优先级。`;
  }
  return `${selectedQueueAppLabel.value} 当前使用独立队列，优先级按点击顺序自动更新。`;
});
const queuePanelEmptyText = computed(() =>
  selectedQueueScope.value === ADVANCED_PROXY_GLOBAL_QUEUE_SCOPE
    ? '点击卡片加入全局默认队列，队列优先级按点击顺序自动更新。'
    : (selectedQueueInheritGlobal.value
      ? '当前应用正在继承全局队列。点任意卡片后会自动分叉出独立队列。'
      : '当前独立队列为空，点击卡片即可加入 Provider。')
);
const quickSetupTooltipText = computed(() =>
  validProviderCandidateCards.value.length
    ? '一键勾选有效密钥'
    : '一键勾选有效密钥前，请先完成快速测活'
);
const providerSelectionMap = computed(() => {
  const map = new Map();
  displayedQueueProviders.value.forEach((provider, index) => {
    const id = String(provider?.id || provider?.rowKey || '').trim();
    if (!id) return;
    map.set(id, {
      order: index + 1,
      provider,
    });
  });
  return map;
});

const providerCandidateCards = computed(() => {
  const duplicateMeta = buildProviderDuplicateMeta(availableRecords.value);
  const cards = availableRecords.value.map(record => {
    const id = String(record?.rowKey || '').trim();
    const selectedMeta = providerSelectionMap.value.get(id) || null;
    const duplicate = duplicateMeta.get(id) || { index: 0, count: 0 };
    return {
      id,
      siteName: String(record?.siteName || record?.siteUrl || 'Provider').trim() || 'Provider',
      modelLabel: String(record?.selectedModel || record?.quickTestModel || selectedMeta?.provider?.model || '未设置模型').trim() || '未设置模型',
      endpoint: String(record?.siteUrl || selectedMeta?.provider?.baseUrl || '').trim(),
      apiKey: String(record?.apiKey || selectedMeta?.provider?.apiKey || '').trim(),
      skLabel: duplicate.count > 1
        ? formatProviderSkLabel(duplicate.index, String(record?.apiKey || selectedMeta?.provider?.apiKey || '').trim())
        : '',
      selected: Boolean(selectedMeta),
      queueOrder: selectedMeta?.order || 0,
      orphaned: false,
      sortTime: Number(record?.updatedAt || 0),
      sourceRecord: record,
    };
  });

  providerSelectionMap.value.forEach((meta, id) => {
    if (cards.some(item => item.id === id)) return;
    cards.push({
      id,
      siteName: String(meta?.provider?.name || meta?.provider?.baseUrl || 'Provider').trim() || 'Provider',
      modelLabel: String(meta?.provider?.model || '未设置模型').trim() || '未设置模型',
      endpoint: String(meta?.provider?.baseUrl || '').trim(),
      apiKey: String(meta?.provider?.apiKey || '').trim(),
      skLabel: '',
      selected: true,
      queueOrder: meta?.order || 0,
      orphaned: true,
      sortTime: 0,
      sourceRecord: null,
    });
  });

  return cards.sort((left, right) => {
    if (left.selected && right.selected) {
      return left.queueOrder - right.queueOrder;
    }
    if (left.selected) return -1;
    if (right.selected) return 1;
    return Number(right.sortTime || 0) - Number(left.sortTime || 0)
      || String(left.siteName || '').localeCompare(String(right.siteName || ''));
  });
});
const validProviderCandidateCards = computed(() =>
  providerCandidateCards.value.filter(item => isValidProviderSourceRecord(item?.sourceRecord))
);

const appCards = computed(() =>
  ADVANCED_PROXY_APPS.map(app => {
    const enabled = draft?.[app.id]?.enabled === true;
    return {
      ...app,
      enabled,
      icon: ADVANCED_PROXY_APP_ICONS[app.id],
      modeLabel: app.mode === 'anthropic'
        ? 'Anthropic Messages 入口'
        : app.id === 'codex'
          ? 'Codex 客户端 · OpenAI Compatible 入口'
          : app.id === 'opencode'
            ? 'OpenCode 客户端 · OpenAI Compatible 入口'
            : app.id === 'openclaw'
              ? 'OpenClaw 客户端 · OpenAI Compatible 入口'
          : 'OpenAI Compatible 入口',
      tooltipDetail: app.id === 'claude'
        ? '已支持：Claude 客户端 -> 本地自动探测端口 -> OpenAI 上游。Claude 请求会自动转成 OpenAI 上游请求，并把返回结果转回 Claude 格式。'
        : '',
    };
  })
);

watch(
  rpmProviderOptions,
  options => {
    const current = String(selectedHighAvailabilityRpmProviderKey.value || '').trim() || ADVANCED_PROXY_GLOBAL_QUEUE_SCOPE;
    if (options.some(option => option.value === current)) {
      return;
    }
    const fallback = options.find(option => option.value !== ADVANCED_PROXY_GLOBAL_QUEUE_SCOPE)?.value
      || ADVANCED_PROXY_GLOBAL_QUEUE_SCOPE;
    selectedHighAvailabilityRpmProviderKey.value = fallback;
  },
  { immediate: true },
);

function toPlainValue(value) {
  return JSON.parse(JSON.stringify(value ?? {}));
}

function isPlainObject(value) {
  return Boolean(value) && Object.prototype.toString.call(value) === '[object Object]';
}

function parseStrictJsonObjectSafe(text, fallback = {}) {
  if (!String(text || '').trim()) {
    return structuredClone(fallback);
  }
  try {
    const parsed = JSON.parse(text);
    return isPlainObject(parsed) ? parsed : structuredClone(fallback);
  } catch {
    return structuredClone(fallback);
  }
}

function stripJsonComments(input) {
  let result = '';
  let inSingle = false;
  let inDouble = false;
  let escaping = false;

  for (let index = 0; index < input.length; index += 1) {
    const current = input[index];
    const next = input[index + 1];

    if (!inSingle && !inDouble && current === '/' && next === '/') {
      while (index < input.length && input[index] !== '\n') {
        index += 1;
      }
      if (index < input.length) {
        result += '\n';
      }
      continue;
    }

    if (!inSingle && !inDouble && current === '/' && next === '*') {
      index += 2;
      while (index < input.length && !(input[index] === '*' && input[index + 1] === '/')) {
        index += 1;
      }
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

    if (!inSingle && current === '"') {
      inDouble = !inDouble;
    }
  }

  return result;
}

function convertSingleQuotedStrings(input) {
  let result = '';
  let inDouble = false;
  let escaping = false;

  for (let index = 0; index < input.length; index += 1) {
    const current = input[index];

    if (inDouble) {
      result += current;
      if (escaping) {
        escaping = false;
      } else if (current === '\\') {
        escaping = true;
      } else if (current === '"') {
        inDouble = false;
      }
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

    if (!closed) {
      throw new Error('Single-quoted string is not closed');
    }

    const decoded = buffer
      .replace(/\\'/g, '\'')
      .replace(/\\"/g, '"');
    result += JSON.stringify(decoded);
  }

  return result;
}

function normalizeJson5LikeToJson(input) {
  const withoutComments = stripJsonComments(String(input || ''));
  const withDoubleQuotes = convertSingleQuotedStrings(withoutComments);
  const quotedKeys = withDoubleQuotes.replace(
    /([{,]\s*)([A-Za-z_$][\w$-]*)(\s*:)/g,
    '$1"$2"$3'
  );
  return quotedKeys.replace(/,(\s*[}\]])/g, '$1');
}

function parseLooseJsonObjectSafe(text, fallback = {}) {
  if (!String(text || '').trim()) {
    return structuredClone(fallback);
  }
  try {
    const parsed = JSON.parse(normalizeJson5LikeToJson(text));
    return isPlainObject(parsed) ? parsed : structuredClone(fallback);
  } catch {
    return structuredClone(fallback);
  }
}

function normalizeComparableUrl(value) {
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

function findManagedSnapshotFile(snapshotFiles, appId, fileId) {
  return (Array.isArray(snapshotFiles) ? snapshotFiles : []).find(file =>
    String(file?.appId || '').trim() === String(appId || '').trim()
    && String(file?.fileId || '').trim() === String(fileId || '').trim()
  ) || null;
}

function isManagedProxyToken(value) {
  return String(value || '').trim() === PROXY_MANAGED_TOKEN;
}

function extractCodexActiveProviderKey(text) {
  const match = String(text || '').match(/^\s*model_provider\s*=\s*(?:"([^"\n]+)"|'([^'\n]+)'|([^\s#]+))/m);
  return String(match?.[1] || match?.[2] || match?.[3] || '').trim();
}

function extractCodexProviderSectionKey(header) {
  const normalized = String(header || '').trim();
  const match = normalized.match(/^model_providers\.(?:"([^"]+)"|'([^']+)'|([^\s]+))$/);
  if (!match) return '';
  return String(match[1] || match[2] || match[3] || '').trim();
}

function extractCodexProviderBaseUrl(text, providerKey) {
  const normalizedProviderKey = String(providerKey || '').trim();
  if (!normalizedProviderKey) return '';
  const lines = String(text || '').replace(/\r\n/g, '\n').split('\n');
  let inTargetSection = false;
  const sectionLines = [];
  for (const line of lines) {
    const sectionHeader = line.match(/^\s*\[([^\]]+)\]\s*$/);
    if (sectionHeader) {
      if (inTargetSection) break;
      const sectionKey = extractCodexProviderSectionKey(sectionHeader[1]);
      inTargetSection = sectionKey === normalizedProviderKey;
      continue;
    }
    if (inTargetSection) {
      sectionLines.push(line);
    }
  }
  if (!sectionLines.length) return '';
  const baseUrlMatch = sectionLines.join('\n').match(/^\s*base_url\s*=\s*["']([^"'\n]+)["']/m);
  return String(baseUrlMatch?.[1] || '').trim();
}

function hasMatchingOpenCodeProxyProvider(config, expectedBaseUrl) {
  const providers = isPlainObject(config?.provider) ? config.provider : {};
  return Object.values(providers).some(provider =>
    normalizeComparableUrl(provider?.options?.baseURL) === expectedBaseUrl
    && isManagedProxyToken(provider?.options?.apiKey)
  );
}

function hasMatchingOpenClawProxyProvider(config, expectedBaseUrl) {
  const providers = isPlainObject(config?.models?.providers) ? config.models.providers : {};
  const primary = String(config?.agents?.defaults?.model?.primary || '').trim();
  if (primary.includes('/')) {
    const primaryProviderKey = primary.split('/')[0];
    const activeProvider = providers[primaryProviderKey];
    if (activeProvider) {
      return normalizeComparableUrl(activeProvider?.baseUrl) === expectedBaseUrl
        && isManagedProxyToken(activeProvider?.apiKey)
        && String(activeProvider?.api || '').trim() === 'openai-completions';
    }
  }
  return Object.values(providers).some(provider =>
    normalizeComparableUrl(provider?.baseUrl) === expectedBaseUrl
    && isManagedProxyToken(provider?.apiKey)
    && String(provider?.api || '').trim() === 'openai-completions'
  );
}

function detectLocalAdvancedProxyTakeoverState(snapshot, config) {
  const files = Array.isArray(snapshot?.files) ? snapshot.files : [];
  const claudeBaseUrl = normalizeComparableUrl(getAdvancedProxyAppBaseUrl('claude', config));
  const codexBaseUrl = normalizeComparableUrl(getAdvancedProxyAppBaseUrl('codex', config));
  const opencodeBaseUrl = normalizeComparableUrl(getAdvancedProxyAppBaseUrl('opencode', config));
  const openclawBaseUrl = normalizeComparableUrl(getAdvancedProxyAppBaseUrl('openclaw', config));

  const claudeSettings = parseStrictJsonObjectSafe(
    findManagedSnapshotFile(files, 'claude', 'settings')?.content || '',
    {}
  );
  const claudeEnv = isPlainObject(claudeSettings?.env) ? claudeSettings.env : {};

  const codexAuth = parseStrictJsonObjectSafe(
    findManagedSnapshotFile(files, 'codex', 'auth')?.content || '',
    {}
  );
  const codexConfigText = String(findManagedSnapshotFile(files, 'codex', 'config')?.content || '');
  const codexProviderKey = extractCodexActiveProviderKey(codexConfigText);
  const codexProviderBaseUrl = normalizeComparableUrl(extractCodexProviderBaseUrl(codexConfigText, codexProviderKey));

  const opencodeConfig = parseStrictJsonObjectSafe(
    findManagedSnapshotFile(files, 'opencode', 'config')?.content || '',
    { $schema: 'https://opencode.ai/config.json' }
  );
  const openclawConfig = parseLooseJsonObjectSafe(
    findManagedSnapshotFile(files, 'openclaw', 'config')?.content || '',
    { models: { mode: 'merge', providers: {} } }
  );

  return {
    claude: normalizeComparableUrl(claudeEnv.ANTHROPIC_BASE_URL) === claudeBaseUrl
      && (isManagedProxyToken(claudeEnv.ANTHROPIC_AUTH_TOKEN) || isManagedProxyToken(claudeEnv.ANTHROPIC_API_KEY)),
    codex: isManagedProxyToken(codexAuth.OPENAI_API_KEY)
      && codexProviderBaseUrl === codexBaseUrl,
    opencode: hasMatchingOpenCodeProxyProvider(opencodeConfig, opencodeBaseUrl),
    openclaw: hasMatchingOpenClawProxyProvider(openclawConfig, openclawBaseUrl),
  };
}

async function reconcileLocalAppTakeoverState(config) {
  if (!isDesktopConfigBridgeAvailable()) return;
  const appIds = ADVANCED_PROXY_APPS.map(app => app.id);
  const snapshot = await readManagedAppConfigFiles(appIds);
  const takeoverState = detectLocalAdvancedProxyTakeoverState(snapshot, config);
  const mismatchedApps = ADVANCED_PROXY_APPS.filter(app =>
    config?.[app.id]?.enabled === true && takeoverState[app.id] !== true
  );
  if (!mismatchedApps.length) return;

  const nextConfig = normalizeAdvancedProxyConfig(toPlainValue(config));
  mismatchedApps.forEach(app => {
    if (!nextConfig[app.id] || typeof nextConfig[app.id] !== 'object') {
      nextConfig[app.id] = {};
    }
    nextConfig[app.id].enabled = false;
  });

  const savedConfig = await setAdvancedProxyConfig(createSyncedPendingConfig(nextConfig));
  loadedConfigSnapshot.value = normalizeAdvancedProxyConfig(savedConfig);
  overwriteDraft(loadedConfigSnapshot.value);
  await reloadBreakerStatsForScope(selectedQueueScope.value, loadedConfigSnapshot.value);
  message.warning(`检测到 ${mismatchedApps.map(app => app.label).join(' / ')} 当前已不处于高级代理接管状态，已自动清空面板勾选状态`);
}

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

function normalizeForSave(config) {
  const next = normalizeAdvancedProxyConfig(toPlainValue(config));
  delete next.updatedAt;
  return next;
}

function overwriteDraft(nextConfig) {
  const normalized = normalizeAdvancedProxyConfig(nextConfig);
  Object.keys(draft).forEach(key => delete draft[key]);
  Object.assign(draft, normalized);
  selectedHighAvailabilityRpmProviderKey.value = ADVANCED_PROXY_GLOBAL_QUEUE_SCOPE;
}

function ensureQueueSection(config, scope) {
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
  } else if (typeof config.queues[scope].inheritGlobal !== 'boolean') {
    config.queues[scope].inheritGlobal = true;
  }
  return config.queues[scope];
}

function createPendingConfig(source = draft) {
  const plainDraft = toPlainValue(source);
  ADVANCED_PROXY_QUEUE_SCOPES.forEach(item => {
    const queue = ensureQueueSection(plainDraft, item.id);
    queue.providers = (queue.providers || []).map((provider, index) => ({
      ...provider,
      sortIndex: index + 1,
    }));
  });
  plainDraft.claude.providers = [...(plainDraft.queues?.global?.providers || [])];
  return normalizeAdvancedProxyConfig(plainDraft);
}

function createSyncedPendingConfig(source = draft, records = availableRecords.value) {
  const pending = createPendingConfig(source);
  return syncAdvancedProxyProvidersFromRecords(pending, records).config;
}

function buildProviderDuplicateMeta(records) {
  const buckets = new Map();
  (Array.isArray(records) ? records : []).forEach(record => {
    const key = `${String(record?.siteUrl || '').trim().toLowerCase()}|${String(record?.siteName || '').trim().toLowerCase()}`;
    if (!buckets.has(key)) {
      buckets.set(key, []);
    }
    buckets.get(key).push(record);
  });

  const meta = new Map();
  buckets.forEach(group => {
    group.forEach((record, index) => {
      meta.set(String(record?.rowKey || '').trim(), {
        index: index + 1,
        count: group.length,
      });
    });
  });
  return meta;
}

function buildProviderFromRecord(record, sortIndex) {
  return {
    id: record.rowKey,
    rowKey: record.rowKey,
    name: record.siteName || record.siteUrl || 'Provider',
    baseUrl: record.siteUrl,
    apiKey: record.apiKey,
    model: record.selectedModel || record.quickTestModel || '',
    apiFormat: 'openai_responses',
    apiKeyField: 'ANTHROPIC_AUTH_TOKEN',
    enabled: true,
    sortIndex,
    sourceType: record.sourceType || 'auto',
  };
}

function buildManagedProviderIdentityKey(baseUrl, apiKey) {
  const normalizedBaseUrl = normalizeComparableUrl(baseUrl);
  const normalizedApiKey = String(apiKey || '').trim();
  if (!normalizedBaseUrl || !normalizedApiKey) return '';
  return `${normalizedBaseUrl}|${normalizedApiKey}`;
}

function createManagedProviderMatcher(records) {
  const rowKeys = new Set();
  const identities = new Set();

  (Array.isArray(records) ? records : []).forEach(record => {
    const rowKey = String(record?.rowKey || '').trim();
    if (rowKey) {
      rowKeys.add(rowKey);
    }
    const identity = buildManagedProviderIdentityKey(record?.siteUrl, record?.apiKey);
    if (identity) {
      identities.add(identity);
    }
  });

  return { rowKeys, identities };
}

function isQueueProviderManaged(provider, matcher) {
  const providerId = String(provider?.rowKey || provider?.id || '').trim();
  if (providerId && matcher.rowKeys.has(providerId)) {
    return true;
  }
  const identity = buildManagedProviderIdentityKey(provider?.baseUrl, provider?.apiKey);
  return Boolean(identity) && matcher.identities.has(identity);
}

function isValidProviderSourceRecord(record) {
  if (!record?.rowKey || !record?.siteUrl || !record?.apiKey) return false;
  const quickTestStatus = String(record?.quickTestStatus || '').trim().toLowerCase();
  return quickTestStatus === 'success' || quickTestStatus === 'warning';
}

function getEnabledAppIds(source = draft) {
  return ADVANCED_PROXY_APPS
    .filter(app => source?.[app.id]?.enabled === true)
    .map(app => app.id);
}

function getDisplayedQueueProviders(source, scope = selectedQueueScope.value) {
  return getAdvancedProxyQueueProviders(source, scope, {
    effective: scope !== ADVANCED_PROXY_GLOBAL_QUEUE_SCOPE,
  });
}

function isQueueFollowingGlobal(source, scope = selectedQueueScope.value) {
  return scope !== ADVANCED_PROXY_GLOBAL_QUEUE_SCOPE
    && source?.queues?.[scope]?.inheritGlobal === true;
}

function replaceQueueProviders(config, scope, providers) {
  const queue = ensureQueueSection(config, scope);
  queue.providers = providers.map((provider, index) => ({
    ...provider,
    enabled: provider?.enabled !== false,
    sortIndex: index + 1,
  }));
  if (scope !== ADVANCED_PROXY_GLOBAL_QUEUE_SCOPE) {
    queue.inheritGlobal = false;
  }
}

function hasConfigChanges(nextConfig) {
  const beforeText = JSON.stringify(normalizeForSave(loadedConfigSnapshot.value));
  const afterText = JSON.stringify(normalizeForSave(nextConfig));
  return beforeText !== afterText;
}

async function syncSavedConfig(savedConfig) {
  loadedConfigSnapshot.value = normalizeAdvancedProxyConfig(savedConfig);
  overwriteDraft(loadedConfigSnapshot.value);
  await reloadBreakerStatsForScope(selectedQueueScope.value, draft);
}

async function saveConfigImmediately(nextConfig, successMessage = '高级代理配置已更新') {
  if (!hasConfigChanges(nextConfig)) {
    message.info('当前没有需要写入的配置变更');
    return false;
  }

  saving.value = true;
  try {
    const saved = await setAdvancedProxyConfig(createSyncedPendingConfig(nextConfig));
    await syncSavedConfig(saved);
    message.success(successMessage);
    return true;
  } catch (error) {
    message.error(error?.message || '写入高级代理配置失败');
    return false;
  } finally {
    saving.value = false;
  }
}

function openPreviewForManagedWrites(nextConfig, desktopPreview, successMessage = '高级代理配置已更新', options = {}) {
  if (!hasConfigChanges(nextConfig)) {
    message.info('当前没有需要写入的配置变更');
    return;
  }

  const managedWrites = Array.isArray(desktopPreview?.writes) ? desktopPreview.writes : [];
  if (!managedWrites.length) {
    saveConfigImmediately(nextConfig, successMessage);
    return;
  }

  pendingSaveConfig.value = createSyncedPendingConfig(nextConfig);
  pendingManagedWrites.value = managedWrites;
  pendingWriteOrder.value = options.writeOrder === 'managed-first' ? 'managed-first' : 'config-first';
  pendingSuccessMessage.value = successMessage;
  configPreview.value = desktopPreview || EMPTY_PREVIEW;
  previewOpen.value = true;
}

async function handleConfigMutation(mutator, successMessage) {
  if (saving.value) return;
  const nextConfig = createSyncedPendingConfig();
  mutator(nextConfig);
  await saveConfigImmediately(nextConfig, successMessage);
}

function getCompatibleProviderForApp(config, appId, enabledOnly = true) {
  const providers = getAdvancedProxyEffectiveProviders(config, appId, { enabledOnly });
  return providers[0] || null;
}

function getPreferredModelForApp(config, appId, provider = null) {
  const directModel = String(provider?.model || '').trim();
  if (directModel) {
    return directModel;
  }
  const defaultModel = String(config?.claude?.defaultModel || '').trim();
  if (defaultModel) {
    return defaultModel;
  }
  const effectiveProviders = getAdvancedProxyEffectiveProviders(config, appId, { enabledOnly: false });
  if (effectiveProviders.some(item => String(item?.model || '').trim())) {
    return String(effectiveProviders.find(item => String(item?.model || '').trim())?.model || '').trim();
  }
  const globalProviders = getAdvancedProxyQueueProviders(config, ADVANCED_PROXY_GLOBAL_QUEUE_SCOPE, { effective: false });
  return String(globalProviders.find(item => String(item?.model || '').trim())?.model || '').trim();
}

function createTakeoverDesktopDraft(appId, enabled, config) {
  const sourceProvider = getCompatibleProviderForApp(config, appId, true);
  const model = getPreferredModelForApp(config, appId, sourceProvider);
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

  if (!endpoint) {
    throw new Error('缺少可写入的目标地址');
  }
  if (!apiKey) {
    throw new Error('缺少可写入的 API Key');
  }

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

async function handleAppTakeoverToggle(appId, value) {
  if (saving.value || loading.value) return;
  if (!isDesktopConfigBridgeAvailable()) {
    message.warning('高级代理接管仅支持桌面版 EXE 运行环境');
    return;
  }

  const app = ADVANCED_PROXY_APPS.find(item => item.id === appId);
  const nextConfig = createSyncedPendingConfig();
  if (!nextConfig[appId]) {
    nextConfig[appId] = {};
  }
  nextConfig[appId].enabled = value;
  if (value) {
    nextConfig.enabled = true;
  }

  try {
    const desktopDraft = createTakeoverDesktopDraft(appId, value, nextConfig);
    const snapshot = await readManagedAppConfigFiles([appId]);
    const desktopPreview = buildDesktopConfigPreview(desktopDraft, snapshot);
    if (!desktopPreview.appGroups.length && desktopPreview.errors.length) {
      throw new Error(desktopPreview.errors.join('\n'));
    }

    openPreviewForManagedWrites(nextConfig, desktopPreview, `${app?.label || appId} 接管配置已更新`, {
      writeOrder: value ? 'config-first' : 'managed-first',
    });
  } catch (error) {
    message.error(error?.message || `${app?.label || appId} 接管预览生成失败`);
  }
}

function handleProxyMasterToggle(value) {
  if (value) {
    handleConfigMutation(next => {
      next.enabled = true;
      const restoreIds = lastEnabledAppIds.value.length ? [...lastEnabledAppIds.value] : ['claude'];
      ADVANCED_PROXY_APPS.forEach(app => {
        if (!next[app.id]) {
          next[app.id] = {};
        }
        next[app.id].enabled = restoreIds.includes(app.id);
      });
    }, '代理总开关已更新');
    return;
  }

  const currentEnabledIds = getEnabledAppIds(draft);
  if (currentEnabledIds.length) {
    lastEnabledAppIds.value = [...currentEnabledIds];
  }
  handleConfigMutation(next => {
    next.enabled = false;
    ADVANCED_PROXY_APPS.forEach(app => {
      if (!next[app.id]) {
        next[app.id] = {};
      }
      next[app.id].enabled = false;
    });
  }, '代理总开关已更新');
}

function handleFailoverFieldMutation(field, value) {
  if (value == null || value === '') return;
  handleConfigMutation(next => {
    next.failover[field] = value;
  }, '故障转移配置已更新');
}

function handleUnifiedFailoverToggle(value) {
  handleConfigMutation(next => {
    next.failover.enabled = value === true;
    next.failover.autoFailoverEnabled = value === true;
  }, '故障自动转移开关已更新');
}

function handleHighAvailabilityFieldMutation(field, value) {
  handleConfigMutation(next => {
    if (!next.highAvailability || typeof next.highAvailability !== 'object') {
      next.highAvailability = {};
    }
    next.highAvailability[field] = value;
  }, '高可用与并发配置已更新');
}

function handleHighAvailabilityToggle(value) {
  handleHighAvailabilityFieldMutation('enabled', Boolean(value));
}

function normalizeRpmInputValue(value) {
  const parsed = Number(value);
  if (!Number.isFinite(parsed) || parsed < 0) {
    return 0;
  }
  return Math.floor(parsed);
}

function handleHighAvailabilityRpmProviderChange(providerKey) {
  const normalizedProviderKey = String(providerKey || '').trim();
  selectedHighAvailabilityRpmProviderKey.value = rpmProviderOptions.value.some(option => option.value === normalizedProviderKey)
    ? normalizedProviderKey
    : ADVANCED_PROXY_GLOBAL_QUEUE_SCOPE;
}

function handleHighAvailabilityRpmValueChange(value) {
  const rpmValue = normalizeRpmInputValue(value);
  handleConfigMutation(next => {
    if (!next.highAvailability || typeof next.highAvailability !== 'object') {
      next.highAvailability = {};
    }
    if (!next.highAvailability.rpm || typeof next.highAvailability.rpm !== 'object') {
      next.highAvailability.rpm = {
        global: 0,
        providers: {},
      };
    }
    if (!next.highAvailability.rpm.providers || typeof next.highAvailability.rpm.providers !== 'object') {
      next.highAvailability.rpm.providers = {};
    }
    if (selectedHighAvailabilityRpmProviderKey.value === ADVANCED_PROXY_GLOBAL_QUEUE_SCOPE) {
      next.highAvailability.rpm.global = rpmValue;
      return;
    }
    next.highAvailability.rpm.providers[selectedHighAvailabilityRpmProviderKey.value] = rpmValue;
  }, '高可用 RPM 设置已更新');
}

function handleDispatchModeChange(event) {
  const value = event?.target?.value;
  if (!value) return;
  handleHighAvailabilityFieldMutation('dispatchMode', value);
}

function handleQueueScopeChange(scope) {
  selectedQueueScope.value = ADVANCED_PROXY_QUEUE_SCOPES.some(item => item.id === scope)
    ? scope
    : ADVANCED_PROXY_GLOBAL_QUEUE_SCOPE;
}

function focusQueuePanel() {
  const requestedScope = ADVANCED_PROXY_QUEUE_SCOPES.some(item => item.id === props.initialQueueScope)
    ? props.initialQueueScope
    : ADVANCED_PROXY_GLOBAL_QUEUE_SCOPE;
  selectedQueueScope.value = requestedScope;
  nextTick(() => {
    const shell = shellScrollRef.value;
    const panel = queuePanelRef.value;
    if (!shell || !panel) return;
    const targetTop = Math.max(0, panel.offsetTop - 8);
    shell.scrollTo({
      top: targetTop,
      behavior: 'smooth',
    });
  });
}

function handleFollowGlobalQueue() {
  if (selectedQueueScope.value === ADVANCED_PROXY_GLOBAL_QUEUE_SCOPE) return;
  if (selectedQueueInheritGlobal.value) {
    message.info('当前已经在跟随全局队列');
    return;
  }
  handleConfigMutation(next => {
    const queue = ensureQueueSection(next, selectedQueueScope.value);
    queue.inheritGlobal = true;
    queue.providers = [];
  }, `${selectedQueueAppLabel.value} 已改为跟随全局队列`);
}

async function reloadContext() {
  const { records } = loadPanelRecords();
  availableRecords.value = records;
  try {
    antiPoisonRequestRecords.value = await listAdvancedProxyRequestRecords(80);
  } catch {
    antiPoisonRequestRecords.value = [];
  }
}

async function reloadAntiPoisonRecords() {
  await reloadAntiPoisonRecordsInternal(true);
}

async function reloadAntiPoisonRecordsInternal(showToast = true) {
  try {
    antiPoisonRequestRecords.value = await listAdvancedProxyRequestRecords(80);
    if (showToast) {
      message.success('防投毒流水已刷新');
    }
  } catch (error) {
    antiPoisonRequestRecords.value = [];
    if (showToast) {
      message.error(`刷新防投毒流水失败：${error?.message || error}`);
    }
  }
}

function pruneOrphanedQueueProviders(config, records) {
  const matcher = createManagedProviderMatcher(records);
  const nextConfig = createSyncedPendingConfig(config, records);
  let removedCount = 0;

  ADVANCED_PROXY_QUEUE_SCOPES.forEach(item => {
    const queue = ensureQueueSection(nextConfig, item.id);
    const beforeCount = Array.isArray(queue.providers) ? queue.providers.length : 0;
    queue.providers = (queue.providers || []).filter(provider => isQueueProviderManaged(provider, matcher));
    removedCount += Math.max(0, beforeCount - queue.providers.length);
  });

  nextConfig.claude.providers = [...(nextConfig.queues?.global?.providers || [])];
  return {
    removedCount,
    nextConfig,
  };
}

async function handleRefreshQueueContext() {
  await reloadContext();
  const { removedCount, nextConfig } = pruneOrphanedQueueProviders(draft, availableRecords.value);
  if (!removedCount) return;
  await saveConfigImmediately(nextConfig, `已自动清除 ${removedCount} 条已不在密钥管理中的队列 Provider`);
}

async function loadData() {
  loading.value = true;
  try {
    await reloadContext();
    const config = await getAdvancedProxyConfig();
    const { config: syncedConfig, changed } = syncAdvancedProxyProvidersFromRecords(config, availableRecords.value);
    const activeConfig = changed ? await setAdvancedProxyConfig(syncedConfig) : syncedConfig;
    await syncSavedConfig(activeConfig);
    await reconcileLocalAppTakeoverState(activeConfig);
  } catch (error) {
    message.error(error?.message || '加载高级代理配置失败');
  } finally {
    loading.value = false;
  }
}

watch(
  () => props.open,
  async value => {
    if (!value) {
      masterHelpTooltipOpen.value = false;
      antiPoisonTooltipOpen.value = false;
      antiPoisonPanelOpen.value = false;
      return;
    }
    if (value) {
      await loadData();
      focusQueuePanel();
    }
  },
  { immediate: true }
);

watch(
  () => props.focusQueueToken,
  value => {
    if (!props.open || !value) return;
    focusQueuePanel();
  }
);

watch(
  enabledAppIds,
  ids => {
    if (ids.length) {
      lastEnabledAppIds.value = [...ids];
    }
  },
  { immediate: true }
);

watch(
  selectedQueueScope,
  async scope => {
    if (props.open) {
      await reloadBreakerStatsForScope(scope, draft);
    }
  }
);

function toggleProviderQueue(item) {
  const providerId = String(item?.id || '').trim();
  if (!providerId) return;

  handleConfigMutation(next => {
    const scope = selectedQueueScope.value;
    const list = getDisplayedQueueProviders(next, scope).map(provider => ({ ...provider }));
    if (scope !== ADVANCED_PROXY_GLOBAL_QUEUE_SCOPE && isQueueFollowingGlobal(next, scope)) {
      ensureQueueSection(next, scope).inheritGlobal = false;
    }
    const existingIndex = list.findIndex(provider => String(provider?.id || provider?.rowKey || '').trim() === providerId);

    if (existingIndex >= 0) {
      list.splice(existingIndex, 1);
    } else if (item?.sourceRecord) {
      list.push(buildProviderFromRecord(item.sourceRecord, list.length + 1));
    }

    replaceQueueProviders(next, scope, list);
  }, item?.selected ? `${selectedQueueLabel.value} 队列已移出 Provider` : `${selectedQueueLabel.value} 队列已加入 Provider`);
}

function handleQuickSelectValidProviders() {
  const validCards = validProviderCandidateCards.value;
  if (!validCards.length) {
    message.warning('暂无可一键勾选的有效密钥，请先完成快速测活');
    return;
  }

  handleConfigMutation(next => {
    const scope = selectedQueueScope.value;
    if (scope !== ADVANCED_PROXY_GLOBAL_QUEUE_SCOPE && isQueueFollowingGlobal(next, scope)) {
      ensureQueueSection(next, scope).inheritGlobal = false;
    }

    replaceQueueProviders(
      next,
      scope,
      validCards
        .map(item => item?.sourceRecord)
        .filter(record => isValidProviderSourceRecord(record))
        .map((record, index) => buildProviderFromRecord(record, index + 1))
    );
  }, `${selectedQueueLabel.value} 队列已一键勾选 ${validCards.length} 条有效密钥`);
}

function getBreakerStatsKey(appId, providerId) {
  return `${String(appId || '').trim().toLowerCase()}:${String(providerId || '').trim()}`;
}

function getBreakerStats(providerId, appId = 'claude') {
  return breakerStatsMap.value[getBreakerStatsKey(appId, providerId)] || {};
}

function getBreakerStateLabel(providerId, appId = 'claude') {
  const state = String(getBreakerStats(providerId, appId)?.state || 'closed').trim();
  if (state === 'half_open') return 'half_open';
  if (state === 'open') return 'open';
  return 'closed';
}

function breakerStateColor(providerId, appId = 'claude') {
  const state = getBreakerStateLabel(providerId, appId);
  if (state === 'open') return 'red';
  if (state === 'half_open') return 'orange';
  return 'green';
}

async function reloadProviderStats(providerId, appId = 'claude') {
  if (!providerId || !appId) return;
  try {
    const stats = await getCircuitBreakerStats(appId, providerId);
    breakerStatsMap.value = {
      ...breakerStatsMap.value,
      [getBreakerStatsKey(appId, providerId)]: stats || {},
    };
  } catch (error) {
    console.warn('[AdvancedProxy] reload breaker stats failed:', error);
  }
}

async function reloadBreakerStatsForScope(scope = selectedQueueScope.value, source = draft) {
  const providers = getDisplayedQueueProviders(source, scope);
  const appIds = scope === ADVANCED_PROXY_GLOBAL_QUEUE_SCOPE ? getEnabledAppIds(source) : [scope];
  const nextStatsMap = {};

  await Promise.all(
    providers.flatMap(provider =>
      appIds.map(async appId => {
        if (!provider?.id || !appId) return;
        try {
          const stats = await getCircuitBreakerStats(appId, provider.id);
          nextStatsMap[getBreakerStatsKey(appId, provider.id)] = stats || {};
        } catch {}
      })
    )
  );

  breakerStatsMap.value = nextStatsMap;
}

async function resetProviderBreaker(providerId, appId = selectedQueueScope.value === ADVANCED_PROXY_GLOBAL_QUEUE_SCOPE ? 'claude' : selectedQueueScope.value) {
  try {
    await resetCircuitBreaker(appId, providerId);
    await reloadProviderStats(providerId, appId);
    message.success('已重置该 Provider 的熔断状态');
  } catch (error) {
    message.error(error?.message || '重置熔断失败');
  }
}

async function applyPreview() {
  if (!pendingSaveConfig.value) {
    message.warning('没有待写入的配置变更');
    return;
  }

  saving.value = true;
  try {
    if (pendingManagedWrites.value.length && pendingWriteOrder.value === 'managed-first') {
      await applyManagedAppConfigFiles(pendingManagedWrites.value);
    }

    const saved = await setAdvancedProxyConfig(pendingSaveConfig.value);

    if (pendingManagedWrites.value.length && pendingWriteOrder.value !== 'managed-first') {
      await applyManagedAppConfigFiles(pendingManagedWrites.value);
    }

    await syncSavedConfig(saved);
    previewOpen.value = false;
    configPreview.value = EMPTY_PREVIEW;
    pendingSaveConfig.value = null;
    pendingManagedWrites.value = [];
    pendingWriteOrder.value = 'config-first';
    message.success(pendingSuccessMessage.value || '高级代理配置已更新');
  } catch (error) {
    message.error(error?.message || '写入高级代理配置失败');
  } finally {
    saving.value = false;
  }
}

function cancelPreview() {
  previewOpen.value = false;
  pendingSaveConfig.value = null;
  pendingManagedWrites.value = [];
  pendingWriteOrder.value = 'config-first';
  configPreview.value = EMPTY_PREVIEW;
}

function handleCancel() {
  masterHelpTooltipOpen.value = false;
  antiPoisonTooltipOpen.value = false;
  antiPoisonPanelOpen.value = false;
  cancelPreview();
  emit('update:open', false);
}

function handleMasterHelpTooltipOpenChange(open) {
  masterHelpTooltipOpen.value = open;
}

function handleAntiPoisonToggle() {
  handleAntiPoisonEnabledChange(!antiPoisonEnabled.value);
  antiPoisonTooltipOpen.value = true;
}

function handleAntiPoisonTooltipOpenChange(open) {
  antiPoisonTooltipOpen.value = open;
}

function openAntiPoisonPanel() {
  if (!antiPoisonEnabled.value) return;
  antiPoisonTooltipOpen.value = false;
  antiPoisonPanelOpen.value = true;
  void reloadAntiPoisonRecordsInternal(false);
}

function formatAntiPoisonOperationTime(value) {
  const parsed = new Date(String(value || ''));
  if (Number.isNaN(parsed.getTime())) return '-';
  return parsed.toLocaleTimeString('zh-CN', {
    hour12: false,
    hour: '2-digit',
    minute: '2-digit',
    second: '2-digit',
  });
}

function buildAntiPoisonOperationDetailText(row, record, op, index) {
  const reason = op?.reason || record?.errorDetail || '';
  const toolCalls = Array.isArray(record?.upstreamToolCalls) ? record.upstreamToolCalls : [];
  const toolArgs = Array.isArray(record?.upstreamToolArgsPreview) ? record.upstreamToolArgsPreview : [];
  const routeTrace = Array.isArray(record?.routeTrace) ? record.routeTrace : [];
  const hostedToolCalls = toolCalls.filter(name => isAntiPoisonHostedToolCallName(name));
  const functionToolCalls = toolCalls.filter(name => !isAntiPoisonHostedToolCallName(name));
  const sections = [
    ['摘要', {
      operationIndex: index,
      recordId: record?.id || '',
      time: op?.time || record?.recordedAt || '',
      appType: record?.appType || '',
      channel: row.channel,
      stage: row.stage,
      route: record?.outboundRoute || record?.clientRoute || row.path,
      providerName: record?.providerName || op?.provider || '',
      model: record?.model || '',
      statusCode: record?.statusCode,
      source: record?.source,
      stream: record?.stream,
      blocked: row.blocked,
    }],
    ['失败原因', {
      rule: row.rule,
      reason,
      errorDetail: record?.errorDetail || '',
      before: row.before,
      after: row.after,
      count: row.count,
    }],
    ['工具调用归类', {
      totalObserved: toolCalls.length,
      hostedToolCalls,
      functionToolCalls,
      note: hostedToolCalls.length
        ? 'web_search_call 属于 OpenAI Responses hosted web search 输出项，不等同于普通 function_call。'
        : '',
      upstreamToolCalls: toolCalls,
      upstreamToolArgsPreview: toolArgs,
    }],
    ['链路', {
      inboundEndpoint: record?.inboundEndpoint || '',
      outboundRoute: record?.outboundRoute || '',
      upstreamEndpoint: record?.upstreamEndpoint || '',
      upstreamUrl: record?.upstreamUrl || '',
      routeTrace,
    }],
    ['网关 Prompt', {
      antiPoisonPromptPreview: record?.antiPoisonPromptPreview || '',
    }],
    ['上游观察', {
      upstreamLatestObserved: record?.upstreamLatestObserved || null,
      upstreamAssistantPreview: record?.upstreamAssistantPreview || '',
    }],
    ['响应预览', {
      upstreamResponsePreview: record?.upstreamResponsePreview || '',
      responsePreview: record?.responsePreview || '',
    }],
    ['原始对象', {
      operation: op || null,
      recordSummary: {
        id: record?.id || '',
        appType: record?.appType || '',
        providerName: record?.providerName || '',
        model: record?.model || '',
        statusCode: record?.statusCode,
        source: record?.source,
        stream: record?.stream,
        antiPoisonOps: record?.antiPoisonOps || [],
      },
    }],
  ];
  return sections
    .map(([title, payload]) => `## ${title}\n${JSON.stringify(payload, null, 2)}`)
    .join('\n\n');
}

function isAntiPoisonHostedToolCallName(name) {
  return String(name || '').trim() === 'web_search_call';
}

async function copyAntiPoisonOperationDetail(text) {
  const content = String(text || '');
  if (!content) return;
  try {
    if (navigator?.clipboard?.writeText) {
      await navigator.clipboard.writeText(content);
      message.success('详情已复制');
      return;
    }
  } catch (error) {
    // Fall through to legacy copy.
  }
  const textarea = document.createElement('textarea');
  textarea.value = content;
  textarea.setAttribute('readonly', 'readonly');
  textarea.style.position = 'fixed';
  textarea.style.left = '-9999px';
  document.body.appendChild(textarea);
  textarea.select();
  document.execCommand('copy');
  document.body.removeChild(textarea);
  message.success('详情已复制');
}

function showAntiPoisonOperationDetail(row) {
  const detailText = String(row?.detailText || '');
  Modal.confirm({
    title: row?.blocked ? '防投毒拦截详情' : '防投毒流水详情',
    width: 820,
    okText: '复制详情',
    cancelText: '关闭',
    content: h('pre', { class: 'advanced-proxy-anti-poison-operation-detail' }, detailText || '-'),
    async onOk() {
      await copyAntiPoisonOperationDetail(detailText);
      return false;
    },
  });
}

function logAntiPoisonConfigEvent(action, detail = '') {
  logClientDiagnostic('advanced-proxy.anti-poison', [
    `action=${action}`,
    `enabled=${String(antiPoisonEnabled.value)}`,
    `strict=${String(antiPoisonConfig.value?.strictMode === true)}`,
    `failureMode=${String(antiPoisonConfig.value?.failureMode || 'block')}`,
    detail,
  ].filter(Boolean).join(' '));
}

async function updateAntiPoisonConfig(mutator, successMessage = '防投毒配置已更新', logAction = 'update') {
  await handleConfigMutation(next => {
    if (!next.antiPoison || typeof next.antiPoison !== 'object') {
      next.antiPoison = normalizeAdvancedProxyConfig({}).antiPoison;
    }
    mutator(next.antiPoison);
  }, successMessage);
  logAntiPoisonConfigEvent(logAction);
}

function handleAntiPoisonEnabledChange(value) {
  void updateAntiPoisonConfig(
    next => { next.enabled = value === true; },
    value ? '防投毒已开启' : '防投毒已关闭',
    value ? 'enabled' : 'disabled',
  );
}

function handleAntiPoisonFieldChange(field, value) {
  void updateAntiPoisonConfig(next => {
    next[field] = value;
  }, '防投毒配置已更新', `field.${field}`);
}

function handleAntiPoisonStringProtectionEnabledChange(value) {
  void updateAntiPoisonConfig(next => {
    next.stringProtection = {
      ...(next.stringProtection || {}),
      enabled: value === true,
      rules: Array.isArray(next.stringProtection?.rules) && next.stringProtection.rules.length
        ? [...next.stringProtection.rules]
        : [...DEFAULT_ANTI_POISON_STRING_PROTECTION.rules],
    };
  }, value ? '字符串保护已开启' : '字符串保护已关闭', value ? 'string-protection.enabled' : 'string-protection.disabled');
}

function handleAntiPoisonStringProtectionRulesChange(value) {
  const rules = String(value || '')
    .split(/\r?\n/)
    .map(rule => rule.trim())
    .filter(Boolean);
  void updateAntiPoisonConfig(next => {
    next.stringProtection = {
      ...(next.stringProtection || {}),
      enabled: next.stringProtection?.enabled !== false,
      rules: rules.length ? rules : [...DEFAULT_ANTI_POISON_STRING_PROTECTION.rules],
    };
  }, '字符串保护规则已更新', 'string-protection.rules');
}

function resetAntiPoisonStringProtectionRules() {
  void updateAntiPoisonConfig(next => {
    next.stringProtection = {
      enabled: true,
      rules: [...DEFAULT_ANTI_POISON_STRING_PROTECTION.rules],
    };
  }, '字符串保护规则已恢复默认', 'string-protection.reset');
}

function resetAntiPoisonPromptsToDefault() {
  void updateAntiPoisonConfig(next => {
    next.strategyPrompt = DEFAULT_ANTI_POISON_STRATEGY_PROMPT;
    next.algorithmPrompt = DEFAULT_ANTI_POISON_ALGORITHM_PROMPT;
    next.randomization = { ...DEFAULT_ANTI_POISON_RANDOMIZATION };
    next.stringProtection = {
      enabled: true,
      rules: [...DEFAULT_ANTI_POISON_STRING_PROTECTION.rules],
    };
  }, '防投毒策略已恢复默认', 'reset-default');
}
</script>

<style scoped>
:global(.advanced-proxy-modal-wrap) {
  overflow: hidden;
}

:global(.advanced-proxy-modal-wrap .ant-modal) {
  margin: 0 auto;
  padding-bottom: 10px;
}

:global(.advanced-proxy-modal-wrap .ant-modal-content) {
  max-height: calc(100vh - 20px);
  overflow: hidden;
}

:global(.advanced-proxy-modal-wrap .ant-modal-body) {
  overflow: hidden;
  max-height: calc(100vh - 96px);
}

:global(.advanced-proxy-modal-wrap .ant-spin-nested-loading) {
  max-height: calc(100vh - 96px);
}

:global(.advanced-proxy-modal-wrap .ant-spin-container) {
  max-height: calc(100vh - 96px);
}

.advanced-proxy-shell {
  display: grid;
  gap: 10px;
  max-height: calc(100vh - 96px);
  overflow-y: auto;
  overflow-x: hidden;
  padding-right: 4px;
  scrollbar-width: none;
  -ms-overflow-style: none;
}

.advanced-proxy-shell::-webkit-scrollbar {
  width: 0;
  height: 0;
  display: none;
}

.advanced-proxy-hero,
.advanced-proxy-summary-grid,
.advanced-proxy-layout,
.advanced-proxy-provider-grid,
.advanced-proxy-inline-grid,
.advanced-proxy-dense-grid,
.advanced-proxy-toggle-list {
  display: grid;
  gap: 8px;
}

.advanced-proxy-hero {
  grid-template-columns: 1fr;
  padding: 12px 14px;
  border-radius: 18px;
  border: 1px solid rgba(90, 117, 79, 0.14);
  background:
    radial-gradient(circle at top right, rgba(208, 230, 193, 0.3), transparent 36%),
    linear-gradient(135deg, rgba(250, 252, 247, 0.98), rgba(239, 247, 232, 0.94));
}

.advanced-proxy-hero-copy,
.advanced-proxy-side,
.advanced-proxy-provider-list,
.advanced-proxy-section {
  display: grid;
  gap: 8px;
}

.advanced-proxy-hero-copy h3,
.advanced-proxy-section-head h4 {
  margin: 0;
  color: #22311c;
  font-size: 16px;
  line-height: 1.3;
}

.advanced-proxy-hero-copy p,
.advanced-proxy-section-head p,
.advanced-proxy-provider-meta,
.advanced-proxy-provider-stats,
.advanced-proxy-notes,
.advanced-proxy-master-copy small {
  margin: 0;
  color: #6a7867;
  font-size: 11px;
  line-height: 1.45;
}

.advanced-proxy-master-row {
  display: flex;
  align-items: center;
  justify-content: space-between;
  width: 100%;
  max-width: 100%;
  gap: 12px;
  padding: 9px 10px;
  border-radius: 14px;
  border: 1px solid rgba(90, 117, 79, 0.13);
  background: rgba(255, 255, 255, 0.84);
}

.advanced-proxy-master-strip {
  display: grid;
  grid-template-columns: minmax(0, 60%) minmax(84px, 10%) minmax(0, 30%);
  justify-content: start;
  align-items: stretch;
  column-gap: 12px;
}

.advanced-proxy-master-copy {
  min-width: 0;
  display: grid;
  gap: 3px;
}

.advanced-proxy-master-placeholder {
  display: flex;
  flex-direction: column;
  align-items: stretch;
  justify-content: flex-start;
  gap: 8px;
  min-width: 0;
  min-height: 100%;
  padding: 10px 12px;
  border-radius: 14px;
  border: 1px dashed rgba(90, 117, 79, 0.18);
  background: rgba(255, 255, 255, 0.42);
  color: rgba(77, 97, 66, 0.72);
  font-size: 12px;
  letter-spacing: 0.08em;
  text-align: left;
}

.advanced-proxy-master-debug {
  display: flex;
  align-items: stretch;
  justify-content: center;
}

.advanced-proxy-master-debug-group {
  width: 100%;
  min-width: 0;
  display: grid;
  grid-template-columns: repeat(3, minmax(0, 1fr));
  align-items: stretch;
  gap: 4px;
  padding: 6px;
  border: 1px dashed rgba(90, 117, 79, 0.18);
  border-radius: 14px;
  background: rgba(255, 255, 255, 0.42);
}

.advanced-proxy-master-debug-button {
  flex: 1 1 0;
  min-width: 0;
  border: 0;
  border-radius: 10px;
  background: transparent;
  color: #6a7867;
  display: inline-flex;
  align-items: center;
  justify-content: center;
  cursor: pointer;
  transition: background-color .18s ease, color .18s ease;
  padding: 0;
}

.advanced-proxy-master-side-icon-button :deep(.anticon),
.advanced-proxy-master-side-icon-button :deep(svg) {
  width: 16px !important;
  height: 16px !important;
  font-size: 16px !important;
  line-height: 1;
}

.advanced-proxy-master-side-icon {
  width: 16px !important;
  height: 16px !important;
  font-size: 16px !important;
  line-height: 1 !important;
}

.advanced-proxy-master-side-icon :deep(svg) {
  width: 16px !important;
  height: 16px !important;
}

.advanced-proxy-master-debug-button:hover {
  background: rgba(90, 117, 79, 0.06);
  color: #22311c;
}

.advanced-proxy-master-debug-button:focus-visible {
  outline: 2px solid rgba(90, 117, 79, 0.38);
  outline-offset: 2px;
}

.advanced-proxy-master-help-button {
  background: transparent;
}

.advanced-proxy-anti-poison-button {
  color: #5d8060;
}

.advanced-proxy-anti-poison-icon {
  width: 38px !important;
  height: 38px !important;
  display: block;
}

.advanced-proxy-anti-poison-bottle {
  fill: rgba(230, 245, 226, 0.9);
  stroke: currentColor;
  stroke-width: 1.7;
  stroke-linejoin: round;
}

.advanced-proxy-anti-poison-neck,
.advanced-proxy-anti-poison-cross {
  fill: none;
  stroke-linecap: round;
}

.advanced-proxy-anti-poison-neck {
  stroke: currentColor;
  stroke-width: 1.7;
}

.advanced-proxy-anti-poison-skull {
  fill: #5d8060;
}

.advanced-proxy-anti-poison-eye {
  fill: #f8fff3;
}

.advanced-proxy-anti-poison-cross {
  stroke: #d9563d;
  stroke-width: 2.35;
}

:deep(.advanced-proxy-anti-poison-tooltip .ant-popover-inner) {
  min-width: 128px;
  padding: 8px 10px;
  border-radius: 12px;
  background: rgba(33, 45, 29, 0.94);
}

:deep(.advanced-proxy-anti-poison-tooltip .ant-popover-inner-content) {
  color: rgba(255, 255, 255, 0.92);
  padding: 0;
}

.advanced-proxy-anti-poison-tooltip-content {
  display: flex;
  align-items: center;
  gap: 10px;
  white-space: nowrap;
}

.advanced-proxy-anti-poison-detail-link {
  border: 1px solid rgba(215, 236, 204, 0.34);
  border-radius: 999px;
  background: rgba(215, 236, 204, 0.14);
  color: #111827;
  font-size: 12px;
  font-weight: 700;
  line-height: 1;
  padding: 5px 9px;
  cursor: pointer;
  transition: background-color .18s ease, border-color .18s ease, transform .18s ease;
}

.advanced-proxy-anti-poison-detail-link:hover {
  background: rgba(215, 236, 204, 0.24);
  border-color: rgba(215, 236, 204, 0.56);
  transform: translateY(-1px);
}

.advanced-proxy-anti-poison-panel {
  display: grid;
  gap: 14px;
  width: 100%;
  min-width: 0;
}

:global(.advanced-proxy-anti-poison-drawer .ant-drawer-content-wrapper) {
  max-width: calc(100vw - 12px);
}

:global(.advanced-proxy-anti-poison-drawer .ant-drawer-header) {
  border-bottom: 1px solid rgba(90, 117, 79, 0.12);
  background: rgba(249, 253, 244, 0.94);
}

:global(.advanced-proxy-anti-poison-drawer .ant-drawer-title) {
  color: #22311c;
  font-weight: 900;
}

:global(.advanced-proxy-anti-poison-drawer .ant-drawer-body) {
  background:
    radial-gradient(circle at 78% 4%, rgba(210, 237, 180, 0.34), transparent 32%),
    linear-gradient(180deg, rgba(249, 253, 244, 0.96), rgba(241, 248, 236, 0.92));
  padding: 16px;
  overflow-x: hidden;
}

.advanced-proxy-anti-poison-hero-card,
.advanced-proxy-anti-poison-card {
  min-width: 0;
  border: 1px solid rgba(90, 117, 79, 0.14);
  border-radius: 18px;
  background:
    radial-gradient(circle at 90% 0%, rgba(210, 237, 180, 0.38), transparent 36%),
    rgba(255, 255, 255, 0.78);
  box-shadow: 0 16px 36px rgba(61, 87, 48, 0.1);
}

.advanced-proxy-anti-poison-hero-card {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  gap: 18px;
  padding: 18px;
}

.advanced-proxy-anti-poison-kicker {
  display: block;
  margin-bottom: 6px;
  color: rgba(77, 97, 66, 0.62);
  font-size: 11px;
  font-weight: 800;
  letter-spacing: 0.14em;
  text-transform: uppercase;
}

.advanced-proxy-anti-poison-hero-card h3,
.advanced-proxy-anti-poison-card h4 {
  margin: 0;
  color: #22311c;
  font-weight: 900;
}

.advanced-proxy-anti-poison-hero-card h3 {
  font-size: 20px;
}

.advanced-proxy-anti-poison-hero-card p,
.advanced-proxy-anti-poison-card-head p {
  margin: 6px 0 0;
  color: rgba(55, 72, 47, 0.68);
  line-height: 1.5;
}

.advanced-proxy-anti-poison-state {
  flex: 0 0 auto;
  border-radius: 999px;
  border: 1px solid rgba(90, 117, 79, 0.18);
  background: rgba(255, 255, 255, 0.62);
  color: rgba(77, 97, 66, 0.68);
  font-size: 12px;
  font-weight: 800;
  padding: 6px 10px;
}

.advanced-proxy-anti-poison-state.is-active {
  border-color: rgba(217, 86, 61, 0.24);
  background: rgba(255, 232, 226, 0.78);
  color: #b63c29;
}

.advanced-proxy-anti-poison-card {
  padding: 16px;
}

.advanced-proxy-anti-poison-card-head {
  display: flex;
  justify-content: space-between;
  gap: 12px;
  margin-bottom: 12px;
}

.advanced-proxy-anti-poison-card-head-actions {
  align-items: flex-start;
}

.advanced-proxy-anti-poison-actions {
  flex: 0 0 auto;
  display: flex;
  align-items: center;
  gap: 8px;
}

.advanced-proxy-anti-poison-actions :deep(.ant-btn),
.advanced-proxy-anti-poison-soft-button {
  height: 30px;
  border-radius: 999px;
  border: 1px solid rgba(90, 117, 79, 0.18);
  box-shadow: 0 8px 18px rgba(54, 73, 45, 0.08);
  font-size: 12px;
  font-weight: 700;
}

.advanced-proxy-anti-poison-actions :deep(.ant-btn-primary) {
  border-color: rgba(44, 79, 41, 0.52);
  background: linear-gradient(135deg, #284321, #5d7b48);
}

.advanced-proxy-anti-poison-soft-button {
  padding: 0 12px;
  background: rgba(255, 255, 255, 0.76);
  color: #24391f;
  cursor: pointer;
  transition: transform .16s ease, background-color .16s ease, border-color .16s ease;
}

.advanced-proxy-anti-poison-soft-button:hover {
  transform: translateY(-1px);
  border-color: rgba(90, 117, 79, 0.34);
  background: rgba(246, 251, 241, 0.96);
}

.advanced-proxy-anti-poison-card h4 {
  font-size: 15px;
}

.advanced-proxy-anti-poison-card-head p {
  font-size: 12px;
}

.advanced-proxy-anti-poison-settings {
  display: grid;
  gap: 10px;
}

.advanced-proxy-anti-poison-setting-row {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 16px;
  padding: 12px;
  border-radius: 14px;
  border: 1px solid rgba(90, 117, 79, 0.1);
  background: rgba(255, 255, 255, 0.62);
}

.advanced-proxy-anti-poison-setting-row strong,
.advanced-proxy-anti-poison-setting-row span {
  display: block;
}

.advanced-proxy-anti-poison-setting-row strong {
  color: #26381f;
  font-size: 13px;
}

.advanced-proxy-anti-poison-setting-row span {
  margin-top: 3px;
  color: rgba(55, 72, 47, 0.62);
  font-size: 12px;
  line-height: 1.45;
}

.advanced-proxy-anti-poison-textarea :deep(textarea) {
  border-radius: 14px;
  border-color: rgba(90, 117, 79, 0.16);
  background: rgba(255, 255, 255, 0.66);
  color: #26381f;
  font-size: 12px;
  line-height: 1.55;
}

.advanced-proxy-anti-poison-rule-summary {
  display: grid;
  grid-template-columns: repeat(3, minmax(0, 1fr));
  gap: 8px;
  margin-bottom: 10px;
}

.advanced-proxy-anti-poison-rule-summary span {
  min-width: 0;
  border: 1px solid rgba(90, 117, 79, 0.11);
  border-radius: 14px;
  background: rgba(255, 255, 255, 0.64);
  color: rgba(55, 72, 47, 0.62);
  font-size: 11px;
  padding: 9px 10px;
}

.advanced-proxy-anti-poison-rule-summary strong {
  margin-right: 5px;
  color: #22311c;
  font-size: 16px;
  font-weight: 900;
}

.advanced-proxy-anti-poison-rules-textarea :deep(textarea) {
  font-family: "Cascadia Mono", "JetBrains Mono", Consolas, monospace;
}

.advanced-proxy-anti-poison-random-grid {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: 10px;
}

.advanced-proxy-anti-poison-random-card {
  min-width: 0;
  padding: 10px 12px;
  border-radius: 14px;
  border: 1px solid rgba(90, 117, 79, 0.1);
  background: rgba(255, 255, 255, 0.62);
}

.advanced-proxy-anti-poison-random-card span,
.advanced-proxy-anti-poison-random-card strong {
  display: block;
}

.advanced-proxy-anti-poison-random-card span {
  color: rgba(55, 72, 47, 0.58);
  font-size: 11px;
}

.advanced-proxy-anti-poison-random-card strong {
  margin-top: 4px;
  color: #22311c;
  font-size: 13px;
}

.advanced-proxy-anti-poison-preview {
  margin: 12px 0 0;
  padding: 12px;
  max-height: 320px;
  overflow: auto;
  border-radius: 14px;
  border: 1px solid rgba(90, 117, 79, 0.12);
  background: rgba(28, 37, 24, 0.92);
  color: #eff8e9;
  font-size: 12px;
  line-height: 1.55;
  white-space: pre-wrap;
}

.advanced-proxy-anti-poison-flow-table {
  overflow-x: hidden;
  overflow-y: auto;
  max-height: 340px;
  border: 1px solid rgba(90, 117, 79, 0.12);
  border-radius: 16px;
  background:
    linear-gradient(180deg, rgba(255, 255, 255, 0.72), rgba(247, 251, 242, 0.78)),
    radial-gradient(circle at 10% 10%, rgba(178, 211, 135, 0.16), transparent 36%);
}

.advanced-proxy-anti-poison-flow-table table {
  width: 100%;
  table-layout: fixed;
  border-collapse: collapse;
}

.advanced-proxy-anti-poison-flow-table th:nth-child(1),
.advanced-proxy-anti-poison-flow-table td:nth-child(1) {
  width: 112px;
}

.advanced-proxy-anti-poison-flow-table th:nth-child(2),
.advanced-proxy-anti-poison-flow-table td:nth-child(2) {
  width: 92px;
}

.advanced-proxy-anti-poison-flow-table th:nth-child(3),
.advanced-proxy-anti-poison-flow-table td:nth-child(3) {
  width: 78px;
}

.advanced-proxy-anti-poison-flow-table th:nth-child(8),
.advanced-proxy-anti-poison-flow-table td:nth-child(8) {
  width: 52px;
}

.advanced-proxy-anti-poison-flow-table th,
.advanced-proxy-anti-poison-flow-table td {
  padding: 10px 11px;
  border-bottom: 1px solid rgba(90, 117, 79, 0.1);
  text-align: left;
  vertical-align: top;
  color: rgba(38, 56, 31, 0.78);
  font-size: 12px;
  line-height: 1.45;
  overflow-wrap: anywhere;
}

.advanced-proxy-anti-poison-flow-table th {
  position: sticky;
  top: 0;
  z-index: 1;
  background: rgba(242, 249, 235, 0.96);
  color: #25391e;
  font-weight: 800;
}

.advanced-proxy-anti-poison-flow-table td code {
  display: inline-block;
  max-width: 100%;
  overflow-wrap: anywhere;
  border-radius: 8px;
  padding: 2px 6px;
  background: rgba(33, 48, 28, 0.08);
  color: #25391e;
  white-space: normal;
}

.advanced-proxy-anti-poison-rule-cell {
  display: grid;
  gap: 8px;
  align-items: start;
}

.advanced-proxy-anti-poison-row-detail-button {
  width: fit-content;
  min-width: 58px;
  border: 1px solid rgba(190, 53, 34, 0.78);
  border-radius: 0;
  padding: 3px 12px;
  background: rgba(255, 255, 255, 0.52);
  color: #9e2f20;
  font-size: 12px;
  font-weight: 800;
  line-height: 1.35;
  cursor: pointer;
}

.advanced-proxy-anti-poison-row-detail-button:hover {
  background: rgba(255, 235, 230, 0.92);
}

.advanced-proxy-anti-poison-blocked-row td {
  background: rgba(255, 235, 230, 0.78);
  color: #7c2418 !important;
}

.advanced-proxy-anti-poison-blocked-row td code {
  background: rgba(190, 53, 34, 0.12);
  color: #7c2418;
}

.advanced-proxy-anti-poison-time-cell {
  color: rgba(55, 72, 47, 0.5) !important;
  font-variant-numeric: tabular-nums;
  white-space: nowrap;
}

.advanced-proxy-anti-poison-stage-pill {
  display: inline-flex;
  align-items: center;
  min-height: 22px;
  padding: 2px 8px;
  border-radius: 999px;
  background: rgba(78, 111, 61, 0.12);
  color: #29431f;
  font-weight: 800;
  white-space: nowrap;
}

.advanced-proxy-anti-poison-stage-pill.is-blocked {
  background: rgba(217, 86, 61, 0.18);
  color: #9e2f20;
}

:global(.advanced-proxy-anti-poison-operation-detail) {
  max-height: min(62vh, 620px);
  overflow: auto;
  margin: 0;
  padding: 14px;
  border-radius: 12px;
  background: #13200f;
  color: #eef8e9;
  font-size: 12px;
  line-height: 1.55;
  white-space: pre-wrap;
}

.advanced-proxy-anti-poison-empty-row {
  height: 68px;
  text-align: center !important;
  color: rgba(55, 72, 47, 0.55) !important;
}

@media (max-width: 760px) {
  :global(.advanced-proxy-anti-poison-drawer .ant-drawer-body) {
    padding: 10px;
  }

  .advanced-proxy-anti-poison-hero-card {
    flex-direction: column;
    padding: 14px;
  }

  .advanced-proxy-anti-poison-flow-table th,
  .advanced-proxy-anti-poison-flow-table td {
    padding: 8px 7px;
    font-size: 11px;
  }

  .advanced-proxy-anti-poison-flow-table th:nth-child(1),
  .advanced-proxy-anti-poison-flow-table td:nth-child(1) {
    width: 82px;
  }

  .advanced-proxy-anti-poison-flow-table th:nth-child(2),
  .advanced-proxy-anti-poison-flow-table td:nth-child(2) {
    width: 72px;
  }

  .advanced-proxy-anti-poison-flow-table th:nth-child(3),
  .advanced-proxy-anti-poison-flow-table td:nth-child(3) {
    width: 58px;
  }

  .advanced-proxy-anti-poison-flow-table th:nth-child(8),
  .advanced-proxy-anti-poison-flow-table td:nth-child(8) {
    width: 38px;
  }

}

.advanced-proxy-master-help-tooltip-copy {
  display: grid;
  gap: 6px;
  max-width: min(50vw, 640px);
  line-height: 1.5;
}

.advanced-proxy-master-help-tooltip-image {
  display: block;
  width: 100%;
  max-width: min(50vw, 640px);
  height: auto;
  border-radius: 10px;
  border: 1px solid rgba(90, 117, 79, 0.12);
  background: rgba(255, 255, 255, 0.88);
}

:deep(.advanced-proxy-master-help-tooltip) {
  position: fixed !important;
  top: 72px !important;
  right: 16px !important;
  left: auto !important;
  inset-inline-start: auto !important;
  inset-inline-end: 16px !important;
  transform: none !important;
  margin: 0 !important;
}

:deep(.advanced-proxy-master-help-tooltip .ant-tooltip-content) {
  width: min(50vw, 640px);
  max-width: min(50vw, 640px);
}

@media (max-width: 900px) {
  :deep(.advanced-proxy-master-help-tooltip) {
    top: 56px !important;
    right: 12px !important;
    inset-inline-end: 12px !important;
  }

  :deep(.advanced-proxy-master-help-tooltip .ant-tooltip-content) {
    width: min(560px, calc(100vw - 24px));
    max-width: min(560px, calc(100vw - 24px));
  }

  .advanced-proxy-master-help-tooltip-copy,
  .advanced-proxy-master-help-tooltip-image {
    max-width: min(560px, calc(100vw - 24px));
  }
}


.advanced-proxy-master-side-icon-button :deep(svg) {
  width: 16px !important;
  height: 16px !important;
}

.advanced-proxy-master-debug-button-active {
  background: transparent;
  color: #c83d34;
}

.advanced-proxy-master-debug-button-active:hover {
  background: rgba(200, 61, 52, 0.06);
  color: #c83d34;
}

.advanced-proxy-master-stats-grid {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: 6px 10px;
}

.advanced-proxy-master-stat {
  display: flex;
  align-items: baseline;
  gap: 6px;
  min-width: 0;
}

.advanced-proxy-master-stat span {
  color: #5f6f5a;
  font-size: 12px;
  line-height: 1.2;
  cursor: help;
}

.advanced-proxy-master-stat strong {
  color: #22311c;
  font-size: 15px;
  font-weight: 700;
  line-height: 1;
}

.advanced-proxy-master-copy strong,
.advanced-proxy-summary-card strong,
.advanced-proxy-provider-name {
  color: #22311c;
  font-size: 13px;
  font-weight: 700;
}

.advanced-proxy-app-strip {
  display: grid;
  grid-template-columns: repeat(4, minmax(0, 1fr));
  gap: 10px;
}

.advanced-proxy-provider-grid {
  grid-template-columns: repeat(2, minmax(0, 1fr));
}

.advanced-proxy-summary-grid {
  grid-template-columns: repeat(4, minmax(0, 1fr));
}

.advanced-proxy-summary-card,
.advanced-proxy-section,
.advanced-proxy-provider-card,
.advanced-proxy-inline-control,
.advanced-proxy-compact-field,
.advanced-proxy-toggle-row {
  border-radius: 14px;
  border: 1px solid rgba(90, 117, 79, 0.13);
  background: rgba(255, 255, 255, 0.84);
}

.advanced-proxy-app-token,
.advanced-proxy-provider-head,
.advanced-proxy-provider-title,
.advanced-proxy-provider-actions,
.advanced-proxy-provider-tags,
.advanced-proxy-toggle-row {
  display: flex;
  align-items: center;
  gap: 8px;
  flex-wrap: wrap;
}

.advanced-proxy-provider-head,
.advanced-proxy-toggle-row {
  justify-content: space-between;
}

.advanced-proxy-section-head {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  gap: 8px;
}

.advanced-proxy-section-head > div {
  min-width: 0;
  flex: 1;
}

.advanced-proxy-queue-toolbar,
.advanced-proxy-queue-mode {
  display: flex;
  align-items: center;
  gap: 8px;
  flex-wrap: wrap;
}

.advanced-proxy-queue-toolbar {
  justify-content: flex-end;
}

.advanced-proxy-queue-select {
  min-width: 132px;
}

.advanced-proxy-toolbar-icon-button {
  width: 40px;
  min-width: 40px;
  height: 40px;
  padding: 0;
  border-radius: 12px;
  display: inline-flex;
  align-items: center;
  justify-content: center;
}

.advanced-proxy-toolbar-icon-button :deep(.ant-btn-icon) {
  display: inline-flex;
  flex: 0 0 auto;
  align-items: center;
  justify-content: center;
  margin: 0 !important;
  line-height: 0;
  font-size: 16px;
}

.advanced-proxy-toolbar-icon-button-provider-queue :deep(.ant-btn-icon) {
  transition: transform .26s ease, filter .26s ease;
  margin-inline-end: 0;
}

.advanced-proxy-toolbar-icon-button-provider-queue:hover:not(:disabled) :deep(.ant-btn-icon) {
  transform: rotate(24deg) scale(1.06);
  filter: saturate(1.12);
}

.advanced-proxy-queue-mode {
  padding: 8px 10px;
  border-radius: 14px;
  border: 1px solid rgba(90, 117, 79, 0.13);
  background: rgba(252, 253, 250, 0.84);
  color: #66725f;
  font-size: 11px;
  line-height: 1.45;
}

.advanced-proxy-provider-pool {
  display: grid;
  gap: 8px;
}

.advanced-proxy-provider-panel-grid {
  display: grid;
  grid-template-columns: repeat(4, minmax(0, 1fr));
  gap: 10px;
}

.advanced-proxy-provider-panel {
  appearance: none;
  width: 100%;
  min-width: 0;
  min-height: 90px;
  display: grid;
  align-content: start;
  gap: 7px;
  padding: 11px 12px;
  border-radius: 16px;
  border: 1px solid rgba(90, 117, 79, 0.15);
  background: linear-gradient(180deg, rgba(255, 255, 255, 0.94), rgba(248, 250, 246, 0.92));
  text-align: left;
  cursor: pointer;
  transition: border-color 0.18s ease, box-shadow 0.18s ease, transform 0.18s ease;
}

.advanced-proxy-provider-panel:hover {
  border-color: rgba(88, 125, 66, 0.24);
  box-shadow: 0 12px 24px rgba(74, 104, 58, 0.08);
  transform: translateY(-1px);
}

.advanced-proxy-provider-panel-active {
  border-color: rgba(75, 128, 50, 0.34);
  box-shadow:
    0 0 0 1px rgba(102, 168, 68, 0.12),
    0 0 0 4px rgba(147, 210, 109, 0.12),
    0 14px 28px rgba(74, 104, 58, 0.12);
  background: linear-gradient(180deg, rgba(252, 255, 249, 0.98), rgba(242, 248, 236, 0.96));
}

.advanced-proxy-provider-panel-top,
.advanced-proxy-provider-panel-meta,
.advanced-proxy-provider-tooltip {
  display: flex;
  align-items: center;
  gap: 8px;
  flex-wrap: wrap;
}

.advanced-proxy-provider-panel-top {
  justify-content: space-between;
}

.advanced-proxy-provider-panel-title {
  min-width: 0;
  font-size: 13px;
  font-weight: 700;
  line-height: 1.25;
  color: #22311c;
}

.advanced-proxy-provider-panel-model {
  min-width: 0;
  color: #5f6e5a;
  font-size: 11px;
  line-height: 1.35;
  word-break: break-word;
}

.advanced-proxy-provider-chip {
  display: inline-flex;
  align-items: center;
  min-height: 22px;
  padding: 0 8px;
  border-radius: 999px;
  background: rgba(79, 108, 62, 0.08);
  color: #355029;
  font-size: 10px;
  font-weight: 600;
}

.advanced-proxy-provider-chip-muted {
  background: rgba(108, 122, 101, 0.08);
  color: #66725f;
}

.advanced-proxy-provider-tooltip {
  flex-direction: column;
  align-items: flex-start;
}

.advanced-proxy-app-tooltip {
  display: grid;
  gap: 4px;
  max-width: 320px;
  line-height: 1.45;
}

.advanced-proxy-provider-tooltip code {
  max-width: 340px;
  white-space: pre-wrap;
  word-break: break-all;
}

.advanced-proxy-empty-compact {
  padding: 11px 12px;
}

.advanced-proxy-app-token {
  appearance: none;
  width: 100%;
  min-width: 0;
  min-height: 58px;
  justify-content: center;
  padding: 8px;
  border-radius: 14px;
  border: 1px solid rgba(90, 117, 79, 0.13);
  background: rgba(255, 255, 255, 0.88);
  cursor: pointer;
  transition: border-color 0.18s ease, box-shadow 0.18s ease, transform 0.18s ease, background 0.18s ease;
}

.advanced-proxy-app-token:hover {
  border-color: rgba(72, 113, 54, 0.28);
  box-shadow: 0 10px 22px rgba(74, 104, 58, 0.08);
  transform: translateY(-1px);
}

.advanced-proxy-app-token-active {
  border-color: rgba(67, 113, 49, 0.28);
  background: linear-gradient(135deg, rgba(250, 252, 247, 0.98), rgba(236, 246, 228, 0.96));
  box-shadow: 0 12px 24px rgba(74, 104, 58, 0.1);
}

.advanced-proxy-app-token-active .advanced-proxy-app-icon-shell {
  box-shadow: inset 0 0 0 1px rgba(57, 94, 41, 0.1);
}

.advanced-proxy-app-icon-shell {
  width: 36px;
  height: 36px;
  border-radius: 11px;
  display: inline-flex;
  align-items: center;
  justify-content: center;
  padding: 6px;
  box-shadow: inset 0 0 0 1px rgba(90, 117, 79, 0.08);
  background: linear-gradient(135deg, rgba(255, 255, 255, 0.96), rgba(242, 247, 236, 0.92));
}

.advanced-proxy-app-icon-shell-claude {
  background: linear-gradient(135deg, #fff7ed, #ffedd5);
}

.advanced-proxy-app-icon-shell-codex {
  background: linear-gradient(135deg, #ffffff, #f3f4f6);
}

.advanced-proxy-app-icon-shell-opencode {
  background: linear-gradient(135deg, #eef2ff, #dbeafe);
}

.advanced-proxy-app-icon-shell-openclaw {
  background: linear-gradient(135deg, #fff1f2, #ffe4e6);
}

.advanced-proxy-app-icon-image {
  width: 100%;
  height: 100%;
  display: block;
  object-fit: contain;
}

.advanced-proxy-summary-card,
.advanced-proxy-provider-card,
.advanced-proxy-section {
  display: grid;
  gap: 6px;
  padding: 9px 10px;
}

.advanced-proxy-summary-card {
  align-content: start;
  min-height: 84px;
}

.advanced-proxy-summary-card span,
.advanced-proxy-summary-card small {
  color: #5f6f5a;
  font-size: 11px;
  line-height: 1.35;
}

.advanced-proxy-layout {
  grid-template-columns: minmax(0, 1.42fr) minmax(420px, 1.08fr);
  align-items: start;
  gap: 10px;
}

.advanced-proxy-empty {
  padding: 14px 12px;
  border-radius: 14px;
  border: 1px dashed rgba(90, 117, 79, 0.28);
  color: #6a7965;
  background: rgba(247, 250, 244, 0.9);
  font-size: 11px;
  line-height: 1.5;
}

.advanced-proxy-provider-order {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  min-width: 30px;
  height: 24px;
  padding: 0 8px;
  border-radius: 999px;
  background: rgba(60, 103, 39, 0.12);
  color: #2c4a1f;
  font-size: 10px;
  font-weight: 700;
}

.advanced-proxy-provider-head {
  align-items: flex-start;
}

.advanced-proxy-provider-actions {
  justify-content: flex-end;
}

.advanced-proxy-provider-grid {
  grid-template-columns: repeat(2, minmax(0, 1fr));
}

.advanced-proxy-provider-grid :deep(.ant-form-item) {
  margin-bottom: 4px;
}

.advanced-proxy-provider-stats {
  display: flex;
  align-items: center;
  gap: 8px;
  flex-wrap: wrap;
  font-size: 11px;
}

.advanced-proxy-inline-grid,
.advanced-proxy-toggle-list {
  grid-template-columns: repeat(2, minmax(0, 1fr));
}

.advanced-proxy-dense-grid {
  grid-template-columns: repeat(3, minmax(0, 1fr));
}

.advanced-proxy-dense-rows {
  display: grid;
  grid-template-columns: repeat(3, minmax(0, 1fr));
  gap: 10px;
}

.advanced-proxy-triple-row {
  display: contents;
}

.advanced-proxy-triple-row > .advanced-proxy-compact-field {
  width: auto;
  min-width: 0;
  justify-self: stretch;
}

.advanced-proxy-inline-control,
.advanced-proxy-compact-field,
.advanced-proxy-toggle-row {
  padding: 8px 10px;
}

.advanced-proxy-inline-control {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 8px;
}

.advanced-proxy-toggle-list {
  grid-template-columns: repeat(3, minmax(0, 1fr));
}

.advanced-proxy-radio-stack {
  display: grid;
  gap: 8px;
}

.advanced-proxy-radio-card {
  display: grid;
  gap: 10px;
  padding: 10px 12px;
  border-radius: 14px;
  border: 1px solid rgba(90, 117, 79, 0.13);
  background: rgba(255, 255, 255, 0.84);
}

.advanced-proxy-ha-grid {
  display: grid;
  grid-template-columns: minmax(0, 1.18fr) minmax(240px, 0.82fr);
  gap: 8px;
  align-items: stretch;
}

.advanced-proxy-ha-toggle-card {
  align-items: flex-start;
}

.advanced-proxy-rpm-row {
  margin-top: 8px;
  gap: 12px;
  flex-wrap: wrap;
}

.advanced-proxy-rpm-controls {
  display: flex;
  align-items: center;
  justify-content: flex-end;
  gap: 8px;
  flex: 1;
  min-width: 0;
  flex-wrap: wrap;
}

.advanced-proxy-rpm-select {
  min-width: 160px;
  font-size: 14px;
}

.advanced-proxy-rpm-dropdown {
  font-size: 14px;
}

.advanced-proxy-rpm-dropdown :deep(.ant-select-item),
.advanced-proxy-rpm-dropdown :deep(.ant-select-item-option-content) {
  font-size: 14px;
  line-height: 1.35;
}

.advanced-proxy-rpm-input {
  width: 132px;
}

.advanced-proxy-ha-toggle-copy {
  display: grid;
  gap: 4px;
  min-width: 0;
}

.advanced-proxy-radio-group {
  display: flex;
  flex-wrap: wrap;
  gap: 8px;
}

.advanced-proxy-radio-hint {
  margin: 0;
  color: #6a7867;
  font-size: 11px;
  line-height: 1.45;
}

.advanced-proxy-toggle-row span,
.advanced-proxy-inline-label {
  flex: 1;
  min-width: 0;
  color: #22311c;
}

.advanced-proxy-inline-label,
.advanced-proxy-compact-label {
  color: #22311c;
  font-size: 10px;
  font-weight: 700;
  line-height: 1.3;
}

.advanced-proxy-compact-field {
  display: grid;
  gap: 4px;
  padding: 10px 10px;
}

.advanced-proxy-compact-field :deep(.ant-input-number) {
  border-radius: 9px;
}

.advanced-proxy-short-number {
  width: 100%;
  min-width: 0;
}

.advanced-proxy-section :deep(.ant-radio-group) {
  display: flex;
  flex-wrap: wrap;
  gap: 8px;
}

.advanced-proxy-section :deep(.ant-radio-button-wrapper) {
  height: 30px;
  line-height: 28px;
  border-radius: 10px;
  font-size: 11px;
}

.advanced-proxy-notes {
  padding-left: 16px;
  font-size: 11px;
  line-height: 1.45;
}

.advanced-proxy-section :deep(.ant-input),
.advanced-proxy-section :deep(.ant-input-password),
.advanced-proxy-section :deep(.ant-input-affix-wrapper),
.advanced-proxy-section :deep(.ant-select-selector),
.advanced-proxy-section :deep(.ant-input-number),
.advanced-proxy-section :deep(.ant-btn) {
  min-height: 28px;
  font-size: 11px;
}

.advanced-proxy-section :deep(.ant-form-item-label > label) {
  font-size: 10px;
  line-height: 1.2;
}

.advanced-proxy-section :deep(.ant-select-single:not(.ant-select-customize-input) .ant-select-selector) {
  height: 28px;
}

.advanced-proxy-section :deep(.ant-select-single .ant-select-selector .ant-select-selection-item),
.advanced-proxy-section :deep(.ant-select-single .ant-select-selector .ant-select-selection-placeholder) {
  color: #22311c;
  line-height: 26px;
}

.advanced-proxy-section :deep(.ant-input-number-input) {
  color: #22311c;
  height: 26px;
}

:deep(body.gaia-dark) .advanced-proxy-section :deep(.ant-select-selector),
:deep(body.gaia-dark) .advanced-proxy-section :deep(.ant-input),
:deep(body.gaia-dark) .advanced-proxy-section :deep(.ant-input-password),
:deep(body.gaia-dark) .advanced-proxy-section :deep(.ant-input-affix-wrapper),
:deep(body.gaia-dark) .advanced-proxy-section :deep(.ant-input-number),
:deep(body.gaia-dark) .advanced-proxy-section :deep(.ant-btn),
:deep(body.gaia-dark) .advanced-proxy-toolbar-icon-button {
  color: #eef6f4 !important;
}

:deep(body.gaia-dark) .advanced-proxy-section :deep(.ant-select-single .ant-select-selector .ant-select-selection-item),
:deep(body.gaia-dark) .advanced-proxy-section :deep(.ant-select-single .ant-select-selector .ant-select-selection-placeholder),
:deep(body.gaia-dark) .advanced-proxy-section :deep(.ant-input-number-input),
:deep(body.gaia-dark) .advanced-proxy-section :deep(.ant-input::placeholder),
:deep(body.gaia-dark) .advanced-proxy-section :deep(.ant-input-password input::placeholder) {
  color: #eef6f4 !important;
  -webkit-text-fill-color: #eef6f4 !important;
}

:deep(body.gaia-dark) .advanced-proxy-section :deep(.ant-select-arrow),
:deep(body.gaia-dark) .advanced-proxy-toolbar-icon-button :deep(.ant-btn-icon),
:deep(body.gaia-dark) .advanced-proxy-toolbar-icon-button :deep(.anticon) {
  color: rgba(238, 246, 244, 0.88) !important;
}

@media (max-width: 1180px) {
  .advanced-proxy-layout {
    grid-template-columns: 1fr;
  }

  .advanced-proxy-summary-grid {
    grid-template-columns: repeat(2, minmax(0, 1fr));
  }
}

@media (max-width: 760px) {
  .advanced-proxy-toggle-list {
    grid-template-columns: repeat(2, minmax(0, 1fr));
  }

  .advanced-proxy-dense-grid {
    grid-template-columns: repeat(2, minmax(0, 1fr));
  }
}

@media (max-width: 620px) {
  .advanced-proxy-summary-grid,
  .advanced-proxy-provider-grid,
  .advanced-proxy-ha-grid,
  .advanced-proxy-inline-grid,
  .advanced-proxy-dense-grid,
  .advanced-proxy-toggle-list {
    grid-template-columns: 1fr;
  }

  .advanced-proxy-dense-rows {
    grid-template-columns: repeat(2, minmax(0, 1fr));
  }

  .advanced-proxy-provider-actions {
    justify-content: flex-start;
  }
}

@media (max-width: 560px) {
  .advanced-proxy-provider-panel-grid {
    grid-template-columns: repeat(2, minmax(0, 1fr));
  }

  .advanced-proxy-app-strip {
    grid-template-columns: repeat(2, minmax(0, 1fr));
  }
}

@media (max-width: 480px) {
  .advanced-proxy-provider-panel-grid,
  .advanced-proxy-dense-rows {
    grid-template-columns: 1fr;
  }
}
</style>

<style>
body.gaia-dark .advanced-proxy-modal-wrap .ant-select-selector,
body.gaia-dark .advanced-proxy-modal-wrap .ant-input,
body.gaia-dark .advanced-proxy-modal-wrap .ant-input-password,
body.gaia-dark .advanced-proxy-modal-wrap .ant-input-affix-wrapper,
body.gaia-dark .advanced-proxy-modal-wrap .ant-input-number,
body.gaia-dark .advanced-proxy-modal-wrap .ant-btn,
body.gaia-dark .advanced-proxy-modal-wrap .advanced-proxy-toolbar-icon-button {
  color: #eef6f4 !important;
}

body.gaia-dark .advanced-proxy-modal-wrap .ant-select-selection-item,
body.gaia-dark .advanced-proxy-modal-wrap .ant-select-selection-placeholder,
body.gaia-dark .advanced-proxy-modal-wrap .ant-input-number-input,
body.gaia-dark .advanced-proxy-modal-wrap .ant-input,
body.gaia-dark .advanced-proxy-modal-wrap .ant-input::placeholder,
body.gaia-dark .advanced-proxy-modal-wrap .ant-input-password input,
body.gaia-dark .advanced-proxy-modal-wrap .ant-input-password input::placeholder {
  color: #eef6f4 !important;
  -webkit-text-fill-color: #eef6f4 !important;
}

body.gaia-dark .advanced-proxy-modal-wrap .ant-select-arrow,
body.gaia-dark .advanced-proxy-modal-wrap .ant-btn-icon,
body.gaia-dark .advanced-proxy-modal-wrap .anticon {
  color: rgba(238, 246, 244, 0.88) !important;
}
</style>

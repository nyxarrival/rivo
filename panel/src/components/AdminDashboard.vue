<script setup lang="ts">
import { computed, ref } from 'vue'
import {
  NButton,
  NCard,
  NDataTable,
  NDescriptions,
  NDescriptionsItem,
  NForm,
  NFormItem,
  NInput,
  NInputNumber,
  NPopover,
  NSelect,
  NSlider,
  NSpace,
  NSwitch,
  NTabPane,
  NTabs
} from 'naive-ui'
import type { SelectOption } from 'naive-ui'
import type { AppSettings, AdminConfig, LogColumns, NodeColumns, NodeRecord, ProbeTask, SystemLog, ThemeInfo } from '../types'

const props = defineProps<{
  adminRefreshing: boolean
  loading: boolean
  adminLoading: boolean
  weComTestLoading: boolean
  telegramTestLoading: boolean
  emailTestLoading: boolean
  themesLoading: boolean
  adminNodes: NodeRecord[]
  nodeColumns: NodeColumns
  probeTasks: ProbeTask[]
  themes: ThemeInfo[]
  filteredSystemLogs: SystemLog[]
  logColumns: LogColumns
  logLevelFilter: string | null
  logEventFilter: string | null
  logNodeFilter: string | null
  logLevelOptions: SelectOption[]
  logEventOptions: SelectOption[]
  logNodeOptions: SelectOption[]
  adminConfig: AdminConfig | null
  settingsEditor: AppSettings
  assetBaseCurrencyOptions: SelectOption[]
  emailSecurityOptions: SelectOption[]
  metricsRetentionOptions: SelectOption[]
  alertIntervalOptions: SelectOption[]
  displayTarget: (value?: string | null) => string
  probeModeLabel: (type?: string, ipVersion?: string) => string
  systemLogRowKey: (row: SystemLog) => string | number
  normalizeImageUrl: (value?: string | null) => string
  refreshAdminView: () => void
  openCreateProbeTask: () => void
  openEditProbeTask: (task: ProbeTask) => void
  toggleProbeTask: (task: ProbeTask, value: boolean) => void
  deleteProbeTask: (task: ProbeTask) => void
  uploadTheme: (file: File) => void | Promise<void>
  activateTheme: (themeID: string) => void | Promise<void>
  deleteTheme: (themeID: string) => void | Promise<void>
  saveGlobalSettings: (section: 'display' | 'notify') => void | Promise<void>
  sendWeComTestMessage: () => void
  sendTelegramTestMessage: () => void
  sendEmailTestMessage: () => void
}>()

const emit = defineEmits<{
  (event: 'update:logLevelFilter', value: string | null): void
  (event: 'update:logEventFilter', value: string | null): void
  (event: 'update:logNodeFilter', value: string | null): void
}>()

const logLevelValue = computed({
  get: () => props.logLevelFilter,
  set: (value) => emit('update:logLevelFilter', value)
})
const logEventValue = computed({
  get: () => props.logEventFilter,
  set: (value) => emit('update:logEventFilter', value)
})
const logNodeValue = computed({
  get: () => props.logNodeFilter,
  set: (value) => emit('update:logNodeFilter', value)
})
const notifyChannel = ref<'wecom' | 'telegram' | 'email'>('wecom')

const themeFileInput = ref<HTMLInputElement | null>(null)

function openThemeFilePicker() {
  themeFileInput.value?.click()
}

function handleThemeFileChange(event: Event) {
  const input = event.target as HTMLInputElement
  const file = input.files?.[0]
  if (file) {
    void props.uploadTheme(file)
  }
  input.value = ''
}

</script>

<template>
  <section class="admin-view">
    <n-card class="admin-panel" :bordered="false">
      <div class="admin-panel-toolbar">
        <div>
          <span>Master Console</span>
          <strong>后台管理</strong>
        </div>
        <button class="modal-icon-button admin-refresh-button" type="button" :disabled="adminRefreshing || loading" title="刷新后台数据" @click="refreshAdminView">
          <svg viewBox="0 0 24 24" aria-hidden="true">
            <path d="M20 12a8 8 0 1 1-2.34-5.66M20 4v5h-5" />
          </svg>
        </button>
      </div>

      <div class="admin-content-shell">
        <n-tabs type="line" placement="left" animated>
          <n-tab-pane name="nodes" tab="节点数据">
            <n-space class="admin-tab-content" vertical size="large">
              <div class="admin-table-shell node-table-shell">
                <n-data-table
                  :columns="nodeColumns"
                  :data="adminNodes"
                  :pagination="{ pageSize: 10 }"
                  :scroll-x="2820"
                />
              </div>
            </n-space>
          </n-tab-pane>

          <n-tab-pane name="probe-global" tab="Ping 节点">
            <div class="probe-library-panel">
              <div class="settings-guide">
                <span>Ping Pool</span>
                <strong>全局 Ping 节点池</strong>
                <p>这里统一维护可探测的目标节点。编辑 Agent 时再选择该 Agent 需要使用哪些 Ping 节点，Master 会按选择结果下发配置。</p>
              </div>

              <div class="probe-library-toolbar">
                <div>
                  <strong>{{ probeTasks.length }}</strong>
                  <span>个全局节点</span>
                </div>
                <n-button type="primary" @click="openCreateProbeTask">添加 Ping 节点</n-button>
              </div>

              <div class="probe-task-list probe-library-list">
                <div v-for="task in probeTasks" :key="task.id" class="probe-task-row probe-library-row">
                  <div>
                    <strong>{{ task.name || displayTarget(task.target) }}</strong>
                    <span>{{ displayTarget(task.target) }}</span>
                  </div>
                  <div>
                    <b>{{ probeModeLabel(task.type, task.ip_version) }}</b>
                    <span>{{ task.interval_seconds }}s · {{ task.timeout_ms }}ms</span>
                  </div>
                  <div class="probe-task-actions">
                    <n-switch :value="task.enabled" size="small" @update:value="(value: boolean) => toggleProbeTask(task, value)" />
                    <n-button size="small" tertiary @click="openEditProbeTask(task)">编辑</n-button>
                    <n-button size="small" tertiary type="error" @click="deleteProbeTask(task)">删除</n-button>
                  </div>
                </div>
                <div v-if="probeTasks.length === 0" class="probe-empty">暂无全局 Ping 节点</div>
              </div>
            </div>
          </n-tab-pane>

          <n-tab-pane name="logs" tab="系统日志">
            <div class="admin-table-shell">
              <div class="system-log-filters">
                <n-select v-model:value="logLevelValue" clearable placeholder="级别" :options="logLevelOptions" />
                <n-select v-model:value="logEventValue" clearable placeholder="事件类型" :options="logEventOptions" />
                <n-select v-model:value="logNodeValue" clearable placeholder="节点" :options="logNodeOptions" />
              </div>
              <n-data-table :columns="logColumns" :data="filteredSystemLogs" :row-key="systemLogRowKey" :pagination="{ pageSize: 10 }" />
            </div>
          </n-tab-pane>

          <n-tab-pane name="config" tab="Master 配置">
            <n-descriptions v-if="adminConfig" label-placement="left" bordered :column="1">
              <n-descriptions-item label="HTTP">{{ adminConfig.http.listen_addr }}</n-descriptions-item>
              <n-descriptions-item label="后台路径">/{{ adminConfig.http.admin_path }}</n-descriptions-item>
              <n-descriptions-item label="TCP">{{ adminConfig.tcp.listen_addr }}</n-descriptions-item>
              <n-descriptions-item label="TCP Secret Key">{{ adminConfig.tcp.secret_key_configured ? 'configured' : 'not configured' }}</n-descriptions-item>
              <n-descriptions-item label="Database Driver">{{ adminConfig.database.driver }}</n-descriptions-item>
              <n-descriptions-item label="Database">{{ adminConfig.database.dsn }}</n-descriptions-item>
              <n-descriptions-item label="Auto Migrate">{{ adminConfig.database.auto_migrate }}</n-descriptions-item>
              <n-descriptions-item label="Admin">{{ adminConfig.auth.username }}</n-descriptions-item>
              <n-descriptions-item label="Log Level">{{ adminConfig.log.level }}</n-descriptions-item>
              <n-descriptions-item label="Log File">{{ adminConfig.log.file }}</n-descriptions-item>
              <n-descriptions-item label="Log Retention">{{ adminConfig.log.retention_days }} days</n-descriptions-item>
            </n-descriptions>
          </n-tab-pane>

          <n-tab-pane name="themes" tab="主题管理">
            <div class="settings-panel theme-settings-panel">
              <div class="settings-guide">
                <span>Theme</span>
                <strong>前台展示主题</strong>
                <p>上传主题 ZIP 后会解压到 Master 的运行时主题目录。后台面板不受前台主题切换影响。</p>
              </div>

              <div class="theme-management-content">
                <div class="probe-library-toolbar">
                  <div>
                    <strong>{{ themes.length }}</strong>
                    <span>个可用主题</span>
                  </div>
                  <n-button type="primary" :loading="themesLoading" @click="openThemeFilePicker">上传主题 ZIP</n-button>
                  <input ref="themeFileInput" class="theme-upload-input" type="file" accept=".zip" @change="handleThemeFileChange" />
                </div>

                <div class="probe-task-list probe-library-list">
                  <div v-for="theme in themes" :key="theme.id" class="probe-task-row probe-library-row">
                    <div>
                      <strong>{{ theme.name }}</strong>
                      <span>{{ theme.id }} · {{ theme.version || 'unknown' }}</span>
                    </div>
                    <div>
                      <b>{{ theme.built_in ? '内置' : '上传' }}</b>
                      <span>{{ theme.description || '无描述' }}</span>
                    </div>
                    <div class="probe-task-actions">
                      <n-button size="small" tertiary :disabled="theme.active || themesLoading" @click="activateTheme(theme.id)">
                        {{ theme.active ? '当前' : '启用' }}
                      </n-button>
                      <n-button size="small" tertiary type="error" :disabled="theme.built_in || theme.active || themesLoading" @click="deleteTheme(theme.id)">
                        删除
                      </n-button>
                    </div>
                  </div>
                  <div v-if="themes.length === 0" class="probe-empty">暂无主题数据</div>
                </div>
              </div>
            </div>
          </n-tab-pane>

          <n-tab-pane name="settings" tab="全局设置">
            <div class="settings-panel">
              <div class="settings-guide">
                <span>Display</span>
                <strong>站点信息与首页显示</strong>
                <p>这些配置会保存到 Master 的 MySQL，刷新页面或更换浏览器后仍然生效。</p>
              </div>

              <n-form class="settings-form" :model="settingsEditor" label-placement="left" label-width="140">
                <div class="settings-section-title first">站点信息</div>
                <n-form-item label="网站名称">
                  <n-input v-model:value="settingsEditor.site_name" :maxlength="40" show-count placeholder="Rivo Monitor" />
                </n-form-item>
                <n-form-item label="网站说明">
                  <n-input v-model:value="settingsEditor.site_description" :maxlength="80" show-count placeholder="Private infrastructure monitor" />
                </n-form-item>
                <n-form-item label="后台路径">
                  <n-input v-model:value="settingsEditor.admin_path" placeholder="rivoAdmin8x7k2qZ" />
                </n-form-item>
                <n-form-item label="站点头像地址">
                  <div class="image-url-field">
                    <n-popover trigger="hover" placement="right" :show-arrow="false" :disabled="!normalizeImageUrl(settingsEditor.site_avatar_url)" raw>
                      <template #trigger>
                        <n-input v-model:value="settingsEditor.site_avatar_url" placeholder="/rivo-logo.png 或 https://example.com/logo.png" clearable />
                      </template>
                      <div class="image-url-preview-popover">
                        <img :src="normalizeImageUrl(settingsEditor.site_avatar_url)" alt="站点头像预览" />
                      </div>
                    </n-popover>
                    <small>用于顶部栏左侧站点头像，留空时使用默认 logo。</small>
                  </div>
                </n-form-item>
                <n-form-item label="右侧头像地址">
                  <div class="image-url-field">
                    <n-popover trigger="hover" placement="right" :show-arrow="false" :disabled="!normalizeImageUrl(settingsEditor.user_avatar_url)" raw>
                      <template #trigger>
                        <n-input v-model:value="settingsEditor.user_avatar_url" placeholder="https://example.com/avatar.png" clearable />
                      </template>
                      <div class="image-url-preview-popover">
                        <img :src="normalizeImageUrl(settingsEditor.user_avatar_url)" alt="右侧头像预览" />
                      </div>
                    </n-popover>
                  </div>
                </n-form-item>
                <n-form-item label="首页背景图">
                  <div class="image-url-field">
                    <n-popover trigger="hover" placement="right" :show-arrow="false" :disabled="!normalizeImageUrl(settingsEditor.home_background_url)" raw>
                      <template #trigger>
                        <n-input v-model:value="settingsEditor.home_background_url" placeholder="https://example.com/background.jpg" clearable />
                      </template>
                      <div class="image-url-preview-popover image-url-preview-popover--wide">
                        <img :src="normalizeImageUrl(settingsEditor.home_background_url)" alt="首页背景图预览" />
                      </div>
                    </n-popover>
                  </div>
                </n-form-item>
                <div class="settings-section-title">首页显示</div>
                <n-form-item label="首页汇总">
                  <n-switch v-model:value="settingsEditor.show_home_summary" />
                </n-form-item>
                <n-form-item label="套餐详情">
                  <n-switch v-model:value="settingsEditor.show_billing_details" />
                </n-form-item>
                <n-form-item label="套餐流量">
                  <n-switch v-model:value="settingsEditor.show_traffic_plan" />
                </n-form-item>
                <n-form-item label="节点 Tag">
                  <n-switch v-model:value="settingsEditor.show_node_tags" />
                </n-form-item>
                <n-form-item label="IP 脱敏">
                  <n-switch v-model:value="settingsEditor.mask_ip_addresses" />
                </n-form-item>
                <n-form-item label="资产默认货币">
                  <n-select v-model:value="settingsEditor.asset_base_currency" :options="assetBaseCurrencyOptions" />
                </n-form-item>
                <n-form-item label="实时汇率">
                  <n-switch v-model:value="settingsEditor.exchange_rate_auto_update" />
                </n-form-item>
                <div class="settings-section-title">时序数据保留</div>
                <n-form-item label="指标/Ping 数据">
                  <n-select v-model:value="settingsEditor.metrics_retention_months" :options="metricsRetentionOptions" />
                </n-form-item>
                <div class="settings-section-title">进程与连接快照</div>
                <n-form-item label="默认采集">
                  <n-switch v-model:value="settingsEditor.snapshot_enabled" />
                </n-form-item>
                <n-form-item label="采集进程">
                  <n-switch v-model:value="settingsEditor.snapshot_collect_processes" />
                </n-form-item>
                <n-form-item label="采集连接">
                  <n-switch v-model:value="settingsEditor.snapshot_collect_connections" />
                </n-form-item>
                <n-form-item label="敏感信息脱敏">
                  <n-switch v-model:value="settingsEditor.snapshot_mask_sensitive" />
                </n-form-item>
                <n-form-item label="采集间隔">
                  <n-input-number v-model:value="settingsEditor.snapshot_interval_seconds" :min="15" :max="3600" :step="5">
                    <template #suffix>秒</template>
                  </n-input-number>
                </n-form-item>
                <n-form-item label="进程数量">
                  <n-input-number v-model:value="settingsEditor.snapshot_process_limit" :min="1" :max="50" :step="1">
                    <template #suffix>条</template>
                  </n-input-number>
                </n-form-item>
                <n-form-item label="连接数量">
                  <n-input-number v-model:value="settingsEditor.snapshot_connection_limit" :min="1" :max="500" :step="10">
                    <template #suffix>条</template>
                  </n-input-number>
                </n-form-item>
                <n-button type="primary" :loading="adminLoading" @click="saveGlobalSettings('display')">
                  保存全局设置
                </n-button>
              </n-form>
            </div>
          </n-tab-pane>

          <n-tab-pane name="notifications" tab="消息通知">
            <div class="settings-panel">
              <div class="settings-guide">
                <span>Notify</span>
                <strong>消息通知与告警阈值</strong>
                <p>上线/离线通知不受预警周期影响；资源阈值首次触发会立即通知，仍未恢复时第二条开始按固定预警周期重复提醒，恢复后自动解除。</p>
              </div>

              <n-form class="settings-form" :model="settingsEditor" label-placement="left" label-width="150">
                <n-tabs v-model:value="notifyChannel" type="segment" animated class="notify-channel-tabs">
                  <n-tab-pane name="wecom" tab="企业微信">
                    <n-form-item label="企业微信通知">
                      <n-switch v-model:value="settingsEditor.wecom_webhook_enabled" />
                    </n-form-item>
                    <n-form-item label="企业微信 Webhook">
                      <n-space vertical style="width: 100%">
                        <n-input
                          v-model:value="settingsEditor.wecom_webhook_url"
                          type="password"
                          show-password-on="click"
                          placeholder="https://qyapi.weixin.qq.com/cgi-bin/webhook/send?key=..."
                        />
                        <n-button secondary :loading="weComTestLoading" @click="sendWeComTestMessage">
                          发送测试消息
                        </n-button>
                      </n-space>
                    </n-form-item>
                  </n-tab-pane>

                  <n-tab-pane name="telegram" tab="Telegram">
                    <n-form-item label="Telegram 通知">
                      <n-switch v-model:value="settingsEditor.telegram_enabled" />
                    </n-form-item>
                    <n-form-item label="Bot Token">
                      <n-input
                        v-model:value="settingsEditor.telegram_bot_token"
                        type="password"
                        show-password-on="click"
                        placeholder="123456789:AA..."
                      />
                    </n-form-item>
                    <n-form-item label="用户 ID">
                      <n-space vertical style="width: 100%">
                        <n-input
                          v-model:value="settingsEditor.telegram_chat_id"
                          placeholder="例如 123456789"
                        />
                        <n-button secondary :loading="telegramTestLoading" @click="sendTelegramTestMessage">
                          发送测试消息
                        </n-button>
                      </n-space>
                    </n-form-item>
                  </n-tab-pane>

                  <n-tab-pane name="email" tab="邮件">
                    <n-form-item label="邮件通知">
                      <n-switch v-model:value="settingsEditor.email_enabled" />
                    </n-form-item>
                    <n-form-item label="SMTP 服务器">
                      <n-input
                        v-model:value="settingsEditor.email_smtp_host"
                        placeholder="smtp.example.com"
                      />
                    </n-form-item>
                    <n-form-item label="SMTP 端口">
                      <n-input-number v-model:value="settingsEditor.email_smtp_port" :min="1" :max="65535" :step="1" />
                    </n-form-item>
                    <n-form-item label="加密方式">
                      <n-select v-model:value="settingsEditor.email_smtp_security" :options="emailSecurityOptions" />
                    </n-form-item>
                    <n-form-item label="SMTP 用户名">
                      <n-input
                        v-model:value="settingsEditor.email_smtp_username"
                        placeholder="可选"
                      />
                    </n-form-item>
                    <n-form-item label="SMTP 密码">
                      <n-input
                        v-model:value="settingsEditor.email_smtp_password"
                        type="password"
                        show-password-on="click"
                        placeholder="可选"
                      />
                    </n-form-item>
                    <n-form-item label="发件人">
                      <n-input
                        v-model:value="settingsEditor.email_from"
                        placeholder="Rivo Monitor <noreply@example.com>"
                      />
                    </n-form-item>
                    <n-form-item label="收件人">
                      <n-space vertical style="width: 100%">
                        <n-input
                          v-model:value="settingsEditor.email_to"
                          type="textarea"
                          :autosize="{ minRows: 2, maxRows: 4 }"
                          placeholder="ops@example.com; admin@example.com"
                        />
                        <n-button secondary :loading="emailTestLoading" @click="sendEmailTestMessage">
                          发送测试邮件
                        </n-button>
                      </n-space>
                    </n-form-item>
                  </n-tab-pane>
                </n-tabs>

                <n-form-item label="预警周期">
                  <n-select
                    v-model:value="settingsEditor.alert_interval_minutes"
                    :options="alertIntervalOptions"
                    placeholder="选择重复提醒周期"
                  />
                </n-form-item>
                <n-form-item label="离线通知延迟">
                  <n-input-number v-model:value="settingsEditor.offline_alert_delay_minutes" :min="0" :max="1440" :step="1">
                    <template #suffix>分钟</template>
                  </n-input-number>
                </n-form-item>

                <div class="settings-section-title">流量预警</div>
                <n-form-item label="月流量预警">
                  <n-switch v-model:value="settingsEditor.traffic_alert_enabled" />
                </n-form-item>
                <n-form-item label="使用率阈值">
                  <div class="threshold-slider-field">
                    <n-slider v-model:value="settingsEditor.traffic_alert_percent" :min="0" :max="100" :step="1" />
                    <b>{{ settingsEditor.traffic_alert_percent }}%</b>
                  </div>
                </n-form-item>

                <div class="settings-section-title">资源预警</div>
                <n-form-item label="CPU 预警">
                  <n-switch v-model:value="settingsEditor.cpu_alert_enabled" />
                </n-form-item>
                <n-form-item label="CPU 阈值">
                  <div class="threshold-slider-field">
                    <n-slider v-model:value="settingsEditor.cpu_alert_percent" :min="0" :max="100" :step="1" />
                    <b>{{ settingsEditor.cpu_alert_percent }}%</b>
                  </div>
                </n-form-item>
                <n-form-item label="内存预警">
                  <n-switch v-model:value="settingsEditor.memory_alert_enabled" />
                </n-form-item>
                <n-form-item label="内存阈值">
                  <div class="threshold-slider-field">
                    <n-slider v-model:value="settingsEditor.memory_alert_percent" :min="0" :max="100" :step="1" />
                    <b>{{ settingsEditor.memory_alert_percent }}%</b>
                  </div>
                </n-form-item>
                <n-form-item label="磁盘负载预警">
                  <n-switch v-model:value="settingsEditor.disk_load_alert_enabled" />
                </n-form-item>
                <n-form-item label="磁盘负载阈值">
                  <div class="threshold-slider-field">
                    <n-slider v-model:value="settingsEditor.disk_load_alert_percent" :min="0" :max="100" :step="1" />
                    <b>{{ settingsEditor.disk_load_alert_percent }}%</b>
                  </div>
                </n-form-item>
                <n-form-item label="负载预警">
                  <n-switch v-model:value="settingsEditor.load_alert_enabled" />
                </n-form-item>
                <n-form-item label="1 分钟负载阈值">
                  <div class="threshold-slider-field">
                    <n-slider v-model:value="settingsEditor.load_alert_threshold" :min="0" :max="100" :step="1" />
                    <b>{{ settingsEditor.load_alert_threshold }}</b>
                  </div>
                </n-form-item>

                <div class="settings-section-title">到期提醒</div>
                <n-form-item label="服务到期提醒">
                  <n-switch v-model:value="settingsEditor.expiry_alert_enabled" />
                </n-form-item>
                <n-form-item label="提前提醒天数">
                  <n-input-number v-model:value="settingsEditor.expiry_alert_days" :min="1" :max="366" :step="1">
                    <template #suffix>天</template>
                  </n-input-number>
                </n-form-item>

                <n-button type="primary" :loading="adminLoading" @click="saveGlobalSettings('notify')">
                  保存消息通知配置
                </n-button>
              </n-form>
            </div>
          </n-tab-pane>
        </n-tabs>
      </div>
      <div v-if="adminRefreshing" class="content-loading-overlay panel-loading-overlay" role="status" aria-live="polite">
        <span class="loading-orb"></span>
        <strong>Loading...</strong>
      </div>
    </n-card>
  </section>
</template>

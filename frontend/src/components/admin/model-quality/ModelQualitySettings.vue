<template>
  <section class="quality-settings">
    <div v-if="loading" class="quality-empty" role="status">正在加载检测配置</div>
    <form v-else-if="config" @submit.prevent="save">
      <div class="quality-heading">
        <div>
          <h2>降智检测</h2>
          <p>满血AI · 糖果与鹈鹕</p>
        </div>
        <label class="quality-switch"
          ><span>全局启用</span><Toggle v-model="config.enabled" aria-label="全局启用降智检测"
        /></label>
      </div>
      <div class="quality-endpoint">
        <span>API 端点地址</span><code>{{ config.endpoint || '未配置' }}</code
        ><span class="quality-muted">系统设置</span>
      </div>
      <p v-if="config.pause_reason" class="quality-alert">{{ config.pause_reason }}</p>
      <div class="quality-settings-toolbar">
        <h3>
          检测分组 <span class="quality-muted">{{ config.groups.length }}</span>
        </h3>
        <select v-model="addGroupID" aria-label="添加 OpenAI 分组" class="input" @change="addGroup">
          <option :value="0">添加 OpenAI 分组</option>
          <option v-for="group in availableGroups" :key="group.id" :value="group.id">
            {{ group.name }}
          </option>
        </select>
      </div>
      <div v-if="!config.groups.length" class="quality-empty">尚未选择检测分组</div>
      <article v-for="group in config.groups" :key="group.group_id" class="quality-config-group">
        <header>
          <strong>{{ groupName(group.group_id) }}</strong
          ><span class="quality-badge" :data-state="group.key_id ? 'normal' : 'paused'">{{
            group.key_id ? '已绑定密钥' : '待绑定密钥'
          }}</span>
          <button
            type="button"
            class="quality-icon-button"
            title="移除分组"
            aria-label="移除分组"
            @click="removeGroup(group.group_id)"
          >
            <Icon name="trash" size="sm" />
          </button>
        </header>
        <div class="quality-config-fields">
          <label class="quality-field quality-field-key"
            >测试密钥
            <select v-model="group.key_id" class="input">
              <option :value="null">暂不绑定</option>
              <option
                v-for="key in options[group.group_id]?.keys || []"
                :key="key.id"
                :value="key.id"
              >
                {{ key.name }} · {{ key.masked_key }}
              </option>
              <option
                v-if="
                  group.key_id &&
                  !options[group.group_id]?.keys.some((key) => key.id === group.key_id)
                "
                :value="group.key_id"
                disabled
              >
                原绑定密钥不可用，请重新选择
              </option>
            </select>
            <span v-if="options[group.group_id]?.error" class="quality-error">{{
              options[group.group_id]?.error
            }}</span>
          </label>
          <label class="quality-field"
            >检测间隔（分钟）<input
              v-model.number="group.interval_minutes"
              type="number"
              class="input"
              min="5"
              max="1440"
              required
          /></label>
          <label class="quality-field"
            >推理程度<select v-model="group.reasoning_effort" class="input">
              <option value="low">low</option>
              <option value="medium">medium</option>
              <option value="high">high</option>
            </select></label
          >
          <fieldset class="quality-field">
            <legend>检测方式</legend>
            <div class="quality-checks">
              <label><input v-model="group.methods" type="checkbox" value="candy" />糖果</label
              ><label><input v-model="group.methods" type="checkbox" value="pelican" />鹈鹕</label>
            </div>
          </fieldset>
          <div class="quality-field quality-field-models">
            <span>检测模型 <span class="quality-muted">至少一个</span></span>
            <div class="quality-model-options">
              <label v-for="model in allModels(group)" :key="model"
                ><input v-model="group.models" type="checkbox" :value="model" /><span>{{
                  model
                }}</span></label
              >
            </div>
            <div class="quality-model-add">
              <input
                v-model="customModels[group.group_id]"
                class="input"
                placeholder="补充模型 ID"
                aria-label="补充模型 ID"
                maxlength="200"
                @keydown.enter.prevent="addModel(group)"
              /><button
                type="button"
                class="quality-icon-button"
                aria-label="添加模型"
                title="添加模型"
                @click="addModel(group)"
              >
                <Icon name="plus" size="sm" />
              </button>
            </div>
          </div>
        </div>
        <footer>
          <span class="quality-muted">{{ groupStatus(group.group_id) }}</span
          ><button
            type="button"
            class="btn btn-secondary"
            :disabled="
              dirty ||
              !config.enabled ||
              !group.key_id ||
              !!config.pause_reason ||
              running.includes(group.group_id) ||
              states[group.group_id]?.running
            "
            @click="runNow(group.group_id)"
          >
            <Icon name="play" size="sm" />立即检测
          </button>
        </footer>
      </article>
      <p class="quality-settings-note">
        启用后，所选分组结果对所有登录用户可见。检测会将绑定密钥交给满血AI，并消耗该密钥额度。结果保留
        24 小时。
      </p>
      <p v-if="error" class="quality-error" role="alert">{{ error }}</p>
      <div class="quality-save-row">
        <span class="quality-muted">{{ dirty ? '有未保存的更改' : '配置已保存' }}</span
        ><button type="submit" class="btn btn-primary" :disabled="saving">
          {{ saving ? '保存中' : '保存配置' }}
        </button>
      </div>
    </form>
    <div v-else class="quality-empty">
      <p>{{ error || '配置加载失败' }}</p>
      <button class="btn btn-secondary" @click="load">重试</button>
    </div>
  </section>
</template>

<script setup lang="ts">
import { computed, onMounted, onBeforeUnmount, ref } from 'vue'
import {
  modelQualityAPI,
  qualityLabels,
  qualityTime,
  type QualityConfig,
  type QualityGroupConfig,
  type QualityGroup,
  type QualityKey
} from '@/api/modelQuality'
import groupsAPI from '@/api/admin/groups'
import type { AdminGroup } from '@/types'
import Icon from '@/components/icons/Icon.vue'
import Toggle from '@/components/common/Toggle.vue'
import { useAppStore } from '@/stores/app'
import { extractApiErrorMessage } from '@/utils/apiError'
import '@/styles/model-quality.css'

const app = useAppStore()
const config = ref<QualityConfig>()
const groups = ref<AdminGroup[]>([])
const options = ref<Record<number, { keys: QualityKey[]; models: string[]; error?: string }>>({})
const states = ref<Record<number, QualityGroup>>({})
const customModels = ref<Record<number, string>>({})
const loading = ref(true),
  saving = ref(false),
  error = ref(''),
  baseline = ref(''),
  addGroupID = ref(0)
const running = ref<number[]>([])
const dirty = computed(() => JSON.stringify(config.value) !== baseline.value)
const availableGroups = computed(() =>
  groups.value.filter((g) => !config.value?.groups.some((item) => item.group_id === g.id))
)
let timer: ReturnType<typeof setInterval> | undefined
let disposed = false
function groupName(id: number) {
  return groups.value.find((group) => group.id === id)?.name || `分组 ${id}`
}
function groupStatus(id: number) {
  if (dirty.value) return '保存配置后可立即检测'
  const state = states.value[id]
  if (!config.value?.enabled) return '全局已关闭'
  if (state?.pause_reason) return state.pause_reason
  if (state?.running) return '检测中'
  return state?.next_run_at
    ? `下次检测 ${qualityTime(state.next_run_at)}`
    : qualityLabels[state?.status || 'pending']
}
async function loadOptions(id: number) {
  const result = await Promise.allSettled([
    modelQualityAPI.keys(id),
    groupsAPI.getModelAllowlistCandidates(id)
  ])
  if (disposed) return
  options.value[id] = {
    keys: result[0].status === 'fulfilled' ? result[0].value : [],
    models: result[1].status === 'fulfilled' ? result[1].value : [],
    error: result[0].status === 'rejected' ? '密钥列表加载失败，请重新打开此页' : undefined
  }
}
async function refreshStates() {
  if (!config.value?.enabled || document.hidden) return
  try {
    const items = await modelQualityAPI.groups()
    if (!disposed) states.value = Object.fromEntries(items.map((g) => [g.group_id, g]))
  } catch {
    /* 保留最后一次成功状态。 */
  }
}
async function load() {
  loading.value = true
  error.value = ''
  try {
    const [settings, all] = await Promise.all([
      modelQualityAPI.config(),
      groupsAPI.getAll('openai')
    ])
    if (disposed) return
    config.value = settings
    baseline.value = JSON.stringify(settings)
    groups.value = all.filter((group) => group.status === 'active')
    await Promise.all(settings.groups.map((group) => loadOptions(group.group_id)))
    await refreshStates()
  } catch (e) {
    error.value = extractApiErrorMessage(e, '检测配置加载失败')
  } finally {
    loading.value = false
  }
}
function addGroup() {
  if (!addGroupID.value || !config.value) return
  const id = addGroupID.value
  config.value.groups.push({
    group_id: id,
    key_id: null,
    interval_minutes: 60,
    methods: ['candy', 'pelican'],
    models: [],
    reasoning_effort: 'low'
  })
  addGroupID.value = 0
  void loadOptions(id)
}
function removeGroup(id: number) {
  if (config.value)
    config.value.groups = config.value.groups.filter((group) => group.group_id !== id)
}
function allModels(group: QualityGroupConfig) {
  return [...new Set([...group.models, ...(options.value[group.group_id]?.models || [])])]
}
function addModel(group: QualityGroupConfig) {
  const model = customModels.value[group.group_id]?.trim()
  if (model && !group.models.includes(model)) group.models.push(model)
  customModels.value[group.group_id] = ''
}
async function save() {
  if (!config.value) return
  error.value = ''
  for (const group of config.value.groups) {
    group.models = [...new Set(group.models.map((model) => model.trim()).filter(Boolean))]
    if (!group.models.length) {
      error.value = `${groupName(group.group_id)}：至少选择一个检测模型`
      return
    }
    if (!group.methods.length) {
      error.value = `${groupName(group.group_id)}：至少选择一种检测方式`
      return
    }
  }
  saving.value = true
  try {
    config.value = await modelQualityAPI.save(config.value)
    baseline.value = JSON.stringify(config.value)
    await app.fetchPublicSettings(true)
    await refreshStates()
    app.showSuccess('检测配置已保存')
  } catch (e) {
    error.value = extractApiErrorMessage(e, '配置保存失败')
  } finally {
    saving.value = false
  }
}
async function runNow(id: number) {
  running.value.push(id)
  try {
    await modelQualityAPI.runNow(id)
    await refreshStates()
    app.showSuccess('检测已加入队列')
  } catch (e) {
    error.value = extractApiErrorMessage(e, '无法启动检测')
  } finally {
    running.value = running.value.filter((value) => value !== id)
  }
}
onMounted(() => {
  void load()
  timer = setInterval(refreshStates, 15000)
})
onBeforeUnmount(() => {
  disposed = true
  clearInterval(timer)
})
</script>

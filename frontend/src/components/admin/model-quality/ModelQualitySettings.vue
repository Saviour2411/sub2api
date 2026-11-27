<template>
  <section class="quality-settings">
    <div v-if="loading" class="quality-empty" role="status">正在加载检测配置</div>
    <form v-else-if="config" @submit.prevent="save">
      <div class="quality-heading">
        <h2>降智检测</h2>
        <label class="quality-switch"
          ><span>全局启用</span><Toggle v-model="config.enabled" aria-label="全局启用降智检测"
        /></label>
      </div>
      <div class="quality-endpoint">
        <span>本站 API</span><code>{{ config.endpoint || '未配置' }}</code>
      </div>
      <p v-if="config.pause_reason" class="quality-alert">
        {{ config.pause_reason }}
      </p>
      <nav class="quality-result-tabs quality-settings-tabs" aria-label="检测配置">
        <button
          type="button"
          :class="{ active: tab === 'questions' }"
          :aria-pressed="tab === 'questions'"
          @click="tab = 'questions'"
        >
          题库 · {{ config.questions.length }}</button
        ><button
          type="button"
          :class="{ active: tab === 'groups' }"
          :aria-pressed="tab === 'groups'"
          @click="tab = 'groups'"
        >
          检测分组 · {{ config.groups.length }}
        </button>
      </nav>
      <div v-show="tab === 'questions'">
        <div class="quality-settings-toolbar">
          <h3>测试题目</h3>
          <button
            type="button"
            class="btn btn-secondary"
            :disabled="config.questions.length >= 50"
            @click="addQuestion"
          >
            <Icon name="plus" size="sm" />新增题目
          </button>
        </div>
        <article v-for="q in config.questions" :key="q.id" class="quality-question">
          <header>
            <strong>{{ q.name || '未命名题目' }}</strong
            ><button
              type="button"
              class="quality-icon-button"
              title="删除题目"
              aria-label="删除题目"
              @click="removeQuestion(q.id)"
            >
              <Icon name="trash" size="sm" />
            </button>
          </header>
          <div class="quality-question-fields">
            <label class="quality-field"
              >名称<input v-model="q.name" class="input" maxlength="200" required /></label
            ><label class="quality-field"
              >输出类型<select v-model="q.output_type" class="input">
                <option value="text">文本</option>
                <option value="svg">SVG</option>
                <option value="html">HTML</option>
              </select></label
            ><label class="quality-field"
              >判定方式<select v-model="q.rule.kind" class="input" @change="prepareJudge(q)">
                <option value="number">数字相等</option>
                <option value="string">字符串相等</option>
                <option value="model">模型判题</option>
              </select></label
            >
          </div>
          <label class="quality-field"
            >测试提示词<textarea v-model="q.prompt" class="input" rows="4" maxlength="65536" />
          </label>
          <details>
            <summary>系统提示词</summary>
            <textarea
              v-model="q.system_prompt"
              class="input"
              rows="3"
              maxlength="16384"
              aria-label="系统提示词"
            />
          </details>
          <label v-if="q.rule.kind !== 'model'" class="quality-field"
            >预期答案<textarea v-model="q.rule.expected" class="input" rows="2" maxlength="65536" />
          </label>
          <template v-else
            ><label class="quality-field"
              >判题提示词<textarea
                v-model="q.rule.judge_prompt"
                class="input"
                rows="4"
                maxlength="65536"
              />
            </label>
            <h4>判题模型</h4>
            <QualityCallFields
              v-if="q.rule.judge"
              :key="config.version + q.id"
              v-model:call="q.rule.judge"
              :groups="groups"
              :field-id="q.id"
              @valid="setValid"
          /></template>
        </article>
        <div v-if="!config.questions.length" class="quality-empty">暂无题目</div>
      </div>
      <div v-show="tab === 'groups'">
        <div class="quality-settings-toolbar">
          <h3>检测分组</h3>
          <select
            v-model.number="addGroupID"
            class="input"
            aria-label="添加检测分组"
            @change="addGroup"
          >
            <option :value="0">添加检测分组</option>
            <option v-for="group in availableGroups" :key="group.id" :value="group.id">
              {{ group.name }} · {{ group.platform }}
            </option>
          </select>
        </div>
        <section v-for="group in config.groups" :key="group.group_id" class="quality-config-group">
          <header>
            <strong>{{ groupName(group.group_id) }}</strong
            ><span class="quality-muted">{{ groupStatus(group.group_id) }}</span
            ><button
              type="button"
              class="quality-icon-button"
              title="移除分组"
              aria-label="移除分组"
              @click="removeGroup(group.group_id)"
            >
              <Icon name="trash" size="sm" />
            </button>
          </header>
          <div class="quality-settings-toolbar">
            <label class="quality-field"
              >检测间隔（分钟）<input
                v-model.number="group.interval_minutes"
                type="number"
                min="5"
                max="1440"
                class="input"
                required /></label
            ><button
              type="button"
              class="btn btn-secondary"
              :disabled="group.targets.length >= 20"
              @click="addTarget(group)"
            >
              <Icon name="plus" size="sm" />添加模型</button
            ><button
              type="button"
              class="btn btn-secondary"
              :disabled="
                dirty ||
                !config.enabled ||
                !!states[group.group_id]?.running ||
                running.includes(group.group_id)
              "
              @click="runNow(group.group_id)"
            >
              <Icon name="play" size="sm" />立即检测
            </button>
          </div>
          <div v-for="target in group.targets" :key="target.id" class="quality-target">
            <header>
              <h4>{{ target.model || '待配置模型' }}</h4>
              <button
                type="button"
                class="quality-icon-button"
                title="移除模型"
                aria-label="移除模型"
                @click="removeTarget(group, target.id)"
              >
                <Icon name="trash" size="sm" />
              </button>
            </header>
            <QualityCallFields
              :key="config.version + target.id"
              :call="target"
              @update:call="Object.assign(target, $event)"
              :groups="groups"
              :field-id="target.id"
              fixed-group
              @valid="setValid"
            />
            <fieldset class="quality-field">
              <legend>检测题目</legend>
              <div class="quality-checks">
                <label v-for="q in config.questions" :key="q.id"
                  ><input v-model="target.question_ids" type="checkbox" :value="q.id" />{{
                    q.name
                  }}</label
                >
              </div>
            </fieldset>
          </div>
        </section>
        <div v-if="!config.groups.length" class="quality-empty">尚未选择检测分组</div>
      </div>
      <p v-if="error" class="quality-error" role="alert">{{ error }}</p>
      <footer class="quality-settings-footer">
        <span class="quality-muted">{{ dirty ? '有未保存的修改' : '配置已保存' }}</span
        ><button class="btn btn-primary" :disabled="saving || invalid.size > 0" type="submit">
          <Icon name="check" size="sm" />{{ saving ? '保存中' : '保存配置' }}
        </button>
      </footer>
    </form>
    <p v-else class="quality-error" role="alert">
      {{ error }}<button class="quality-inline-button" @click="load">重试</button>
    </p>
  </section>
</template>
<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, ref } from 'vue'
import Toggle from '@/components/common/Toggle.vue'
import Icon from '@/components/icons/Icon.vue'
import QualityCallFields from './QualityCallFields.vue'
import { groupsAPI } from '@/api/admin/groups'
import {
  modelQualityAPI,
  newQualityCall,
  qualityLabels,
  qualityTime,
  type QualityConfig,
  type QualityGroup,
  type QualityGroupConfig,
  type QualityQuestion
} from '@/api/modelQuality'
import type { Group } from '@/types'
import { useAppStore } from '@/stores/app'
import { extractApiErrorMessage } from '@/utils/apiError'
import '@/styles/model-quality.css'
const app = useAppStore()
const config = ref<QualityConfig>(),
  groups = ref<Group[]>([]),
  states = ref<Record<number, QualityGroup>>({})
const loading = ref(true),
  saving = ref(false),
  error = ref(''),
  baseline = ref(''),
  addGroupID = ref(0),
  tab = ref('questions')
const invalid = ref(new Set<string>()),
  running = ref<number[]>([])
const dirty = computed(() => JSON.stringify(config.value) !== baseline.value)
const availableGroups = computed(() =>
  groups.value.filter((g) => !config.value?.groups.some((item) => item.group_id === g.id))
)
let timer: ReturnType<typeof setInterval> | undefined,
  disposed = false
const groupName = (id: number) => groups.value.find((g) => g.id === id)?.name || '分组 ' + id
function groupStatus(id: number) {
  const state = states.value[id]
  return !config.value?.enabled
    ? '已暂停'
    : state?.pause_reason ||
        (state?.running
          ? '检测中'
          : state?.next_run_at
            ? '下次检测 ' + qualityTime(state.next_run_at)
            : qualityLabels[state?.status || 'pending'])
}
function setValid(id: string, valid: boolean) {
  if (valid) invalid.value.delete(id)
  else invalid.value.add(id)
}
function addQuestion() {
  config.value?.questions.push({
    id: crypto.randomUUID(),
    name: '',
    prompt: '',
    output_type: 'text',
    rule: { kind: 'number', expected: '' }
  })
}
function removeQuestion(id: string) {
  if (!config.value) return
  config.value.questions = config.value.questions.filter((q) => q.id !== id)
  for (const g of config.value.groups)
    for (const t of g.targets) t.question_ids = t.question_ids.filter((q) => q !== id)
  invalid.value.delete(id)
}
function prepareJudge(q: QualityQuestion) {
  if (q.rule.kind === 'model') q.rule.judge ||= newQualityCall()
  else invalid.value.delete(q.id)
}
function addGroup() {
  if (!addGroupID.value || !config.value) return
  config.value.groups.push({
    group_id: addGroupID.value,
    interval_minutes: 60,
    targets: []
  })
  addGroupID.value = 0
}
function addTarget(g: QualityGroupConfig) {
  g.targets.push({
    ...newQualityCall(g.group_id),
    id: crypto.randomUUID(),
    question_ids: []
  })
}
function removeTarget(g: QualityGroupConfig, id: string) {
  g.targets = g.targets.filter((t) => t.id !== id)
  invalid.value.delete(id)
}
function removeGroup(id: number) {
  if (!config.value) return
  const group = config.value.groups.find((g) => g.group_id === id)
  group?.targets.forEach((t) => invalid.value.delete(t.id))
  config.value.groups = config.value.groups.filter((g) => g.group_id !== id)
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
    const [settings, all] = await Promise.all([modelQualityAPI.config(), groupsAPI.getAll()])
    if (disposed) return
    config.value = settings
    groups.value = all.filter((g) => g.status === 'active')
    baseline.value = JSON.stringify(settings)
    await refreshStates()
  } catch (e) {
    error.value = extractApiErrorMessage(e, '检测配置加载失败')
  } finally {
    loading.value = false
  }
}
async function save() {
  if (!config.value || invalid.value.size) return
  saving.value = true
  error.value = ''
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
    running.value = running.value.filter((v) => v !== id)
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
<style scoped>
.quality-settings-tabs {
  margin: 20px 0;
}
.quality-question,
.quality-config-group {
  padding: 20px 0;
  border: 0;
  border-bottom: 1px solid var(--quality-border, #d4d4d8);
  border-radius: 0;
  background: transparent;
}
.quality-question > header,
.quality-target > header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
  margin-bottom: 16px;
}
.quality-question-fields {
  display: grid;
  grid-template-columns: 2fr 1fr 1fr;
  gap: 14px;
}
.quality-question .quality-field,
.quality-target fieldset {
  margin: 12px 0;
}
.quality-question .input {
  width: 100%;
  min-width: 0;
}
.quality-question summary {
  cursor: pointer;
  padding: 8px 0;
}
.quality-target {
  border-top: 1px solid var(--quality-border, #d4d4d8);
  padding: 20px 0;
}
.quality-checks {
  flex-wrap: wrap;
}
.quality-settings-footer {
  display: flex;
  justify-content: space-between;
  align-items: center;
  gap: 12px;
  margin-top: 20px;
}
@media (max-width: 650px) {
  .quality-question-fields {
    grid-template-columns: 1fr;
  }
  .quality-settings-toolbar {
    flex-wrap: wrap;
  }
}
</style>

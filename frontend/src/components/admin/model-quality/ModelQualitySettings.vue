<template>
  <section class="quality-settings">
    <div v-if="loading" class="quality-empty" role="status">正在加载检测配置</div>
    <form v-else-if="config" novalidate @submit.prevent="save">
      <div class="quality-heading">
        <div>
          <h2>降智检测</h2>
          <div class="quality-endpoint">
            <span>本站 API</span><code>{{ config.endpoint || '未配置' }}</code>
          </div>
        </div>
        <label class="quality-switch"
          ><span>全局启用</span><Toggle v-model="config.enabled" aria-label="全局启用降智检测"
        /></label>
      </div>
      <p v-if="config.pause_reason" class="quality-alert">
        {{ config.pause_reason }}
      </p>
      <nav class="quality-result-tabs quality-settings-tabs" aria-label="检测配置">
        <button
          type="button"
          :class="{ active: tab === 'groups' }"
          :aria-pressed="tab === 'groups'"
          @click="tab = 'groups'"
        >
          检测分组 · {{ config.groups.length }}
        </button>
        <button
          type="button"
          :class="{ active: tab === 'questions' }"
          :aria-pressed="tab === 'questions'"
          @click="tab = 'questions'"
        >
          题库 · {{ config.questions.length }}
        </button>
      </nav>
      <div v-show="tab === 'questions'" class="quality-config-workspace">
        <aside class="quality-config-index">
          <div class="quality-index-heading">
            <h3>测试题目</h3>
            <button
              type="button"
              class="quality-icon-button"
              title="新增题目"
              aria-label="新增题目"
              :disabled="config.questions.length >= 50"
              @click="addQuestion"
            >
              <Icon name="plus" size="sm" />
            </button>
          </div>
          <nav aria-label="选择测试题目">
            <button
              v-for="q in config.questions"
              :key="q.id"
              type="button"
              :aria-pressed="selectedQuestion === q.id"
              @click="selectedQuestion = q.id"
            >
              <strong>{{ q.name || '未命名题目' }}</strong><span
                >{{ q.output_type.toUpperCase() }} · {{ ruleName(q.rule.kind) }}</span
              >
            </button>
          </nav>
        </aside>
        <div class="quality-config-editor">
          <article
            v-for="q in config.questions"
            v-show="selectedQuestion === q.id"
            :key="q.id"
            class="quality-question"
          >
            <header>
              <h3>{{ q.name || '未命名题目' }}</h3>
              <button
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
                >名称<input v-model="q.name" class="input" maxlength="200" required
              /></label>
              <label class="quality-field"
                >输出类型<select v-model="q.output_type" class="input">
                  <option value="text">文本</option>
                  <option value="svg">SVG</option>
                  <option value="html">HTML</option>
                </select></label
              >
              <label class="quality-field"
                >判定方式<select v-model="q.rule.kind" class="input" @change="prepareJudge(q)">
                  <option value="number">数字相等</option>
                  <option value="string">字符串相等</option>
                  <option value="model">模型判题</option>
                </select></label
              >
            </div>
            <label class="quality-field"
              >测试提示词<textarea v-model="q.prompt" class="input" rows="5" maxlength="65536" />
            </label>
            <label v-if="q.rule.kind !== 'model'" class="quality-field"
              >预期答案<input v-model="q.rule.expected" class="input" maxlength="65536"
            /></label>
            <label v-else class="quality-field"
              >判题提示词 · 使用待测模型<textarea
                v-model="q.rule.judge_prompt"
                class="input"
                rows="4"
                maxlength="65536"
              />
            </label>
            <details class="quality-question-system">
              <summary>系统提示词</summary>
              <textarea
                v-model="q.system_prompt"
                class="input"
                rows="3"
                maxlength="16384"
                aria-label="系统提示词"
              />
            </details>
          </article>
          <div v-if="!config.questions.length" class="quality-empty">暂无题目</div>
        </div>
      </div>
      <div v-show="tab === 'groups'" class="quality-config-workspace">
        <aside class="quality-config-index">
          <div class="quality-index-heading"><h3>检测分组</h3></div>
          <select
            v-model.number="addGroupID"
            class="input"
            aria-label="添加检测分组"
            @change="addGroup"
          >
            <option :value="0">添加分组</option>
            <option v-for="group in availableGroups" :key="group.id" :value="group.id">
              {{ group.name }} · {{ group.platform }}
            </option>
          </select>
          <nav aria-label="选择检测分组">
            <button
              v-for="group in config.groups"
              :key="group.group_id"
              type="button"
              :aria-pressed="selectedGroup === group.group_id"
              @click="selectedGroup = group.group_id"
            >
              <strong>{{ groupName(group.group_id) }}</strong><span
                >{{ group.targets.length }} 个模型 · {{ groupStatus(group.group_id) }}</span
              >
            </button>
          </nav>
        </aside>
        <div class="quality-config-editor">
          <section
            v-for="group in config.groups"
            v-show="selectedGroup === group.group_id"
            :key="group.group_id"
            class="quality-config-group"
          >
            <header>
              <h3>{{ groupName(group.group_id) }}</h3>
              <span class="quality-muted">{{ groupStatus(group.group_id) }}</span><button
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
              <label class="quality-interval"
                >检测间隔<input
                  v-model.number="group.interval_minutes"
                  type="number"
                  min="5"
                  max="1440"
                  class="input"
                  required
                /><span>分钟</span></label
              >
              <button
                type="button"
                class="btn btn-secondary"
                :disabled="group.targets.length >= 20"
                @click="addTarget(group)"
              >
                <Icon name="plus" size="sm" />添加模型
              </button>
              <button
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
            <details
              v-for="(target, index) in group.targets"
              :key="target.id"
              class="quality-target"
              :open="index === 0 || !target.model"
            >
              <summary>
                <strong>{{ target.model || '待配置模型' }}</strong><span
                  >{{ target.protocol || '待选接口' }} · {{ target.question_ids.length }} 题</span
                >
              </summary>
              <div class="quality-target-fields">
                <QualityCallFields
                  :key="config.version + target.id"
                  :call="target"
                  :groups="groups"
                  :field-id="target.id"
                  fixed-group
                  @update:call="Object.assign(target, $event)"
                  @valid="setValid"
                />
                <div class="quality-target-footer">
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
                  <button
                    type="button"
                    class="quality-icon-button"
                    title="移除模型"
                    aria-label="移除模型"
                    @click="removeTarget(group, target.id)"
                  >
                    <Icon name="trash" size="sm" />
                  </button>
                </div>
              </div>
            </details>
            <div v-if="!group.targets.length" class="quality-empty">暂无检测模型</div>
          </section>
          <div v-if="!config.groups.length" class="quality-empty">尚未选择检测分组</div>
        </div>
      </div>
      <p v-if="error" class="quality-error" role="alert">{{ error }}</p>
      <footer class="quality-settings-footer">
        <span class="quality-muted">{{
          invalid.size ? '请求参数格式有误' : dirty ? '有未保存的修改' : '配置已保存'
        }}</span><button
          class="btn btn-primary"
          :disabled="saving || invalid.size > 0"
          type="submit"
        >
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
  tab = ref('groups'),
  selectedQuestion = ref(''),
  selectedGroup = ref(0)
const ruleName = (kind: string) =>
  ({ number: '数字相等', string: '字符串相等', model: '同模型判题' })[kind]
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
  selectedQuestion.value = crypto.randomUUID()
  config.value?.questions.push({
    id: selectedQuestion.value,
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
  if (selectedQuestion.value === id) selectedQuestion.value = config.value.questions[0]?.id || ''
}
function prepareJudge(q: QualityQuestion) {
  delete q.rule.judge
  invalid.value.delete(q.id)
}
function addGroup() {
  if (!addGroupID.value || !config.value) return
  config.value.groups.push({
    group_id: addGroupID.value,
    interval_minutes: 60,
    targets: []
  })
  selectedGroup.value = addGroupID.value
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
  if (selectedGroup.value === id) selectedGroup.value = config.value.groups[0]?.group_id || 0
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
    settings.questions.forEach((q) => delete q.rule.judge)
    config.value = settings
    selectedGroup.value = settings.groups[0]?.group_id || 0
    selectedQuestion.value = settings.questions[0]?.id || ''
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
.quality-settings {
  background: var(--q-surface);
  padding: 16px 20px;
}
.quality-heading {
  gap: 16px;
  padding: 8px 0 16px;
}
.quality-heading h2 {
  font-size: 18px;
}
.quality-heading > div {
  min-width: 0;
}
.quality-endpoint {
  border: 0;
  padding: 6px 0 0;
  gap: 8px;
  font-size: 12px;
}
.quality-settings-tabs {
  margin: 0 0 16px;
  border-bottom: 1px solid var(--q-line);
}
.quality-config-workspace {
  display: grid;
  grid-template-columns: 220px minmax(0, 1fr);
  gap: 24px;
  align-items: start;
}
.quality-config-index {
  min-width: 0;
}
.quality-index-heading {
  display: flex;
  justify-content: space-between;
  align-items: center;
  min-height: 36px;
  margin-bottom: 8px;
}
.quality-index-heading h3,
.quality-config-editor h3 {
  font-size: 14px;
  font-weight: 600;
}
.quality-config-index > .input {
  width: 100%;
  margin-bottom: 8px;
}
.quality-config-index nav {
  display: grid;
  gap: 4px;
}
.quality-config-index nav button {
  display: grid;
  gap: 5px;
  padding: 10px 12px;
  text-align: left;
  border-left: 2px solid transparent;
  border-radius: 4px;
  min-width: 0;
  overflow-wrap: anywhere;
}
.quality-config-index nav button[aria-pressed='true'] {
  border-color: var(--q-accent);
  background: var(--q-soft);
}
.quality-config-index nav strong {
  font-size: 13px;
  font-weight: 500;
}
.quality-config-index nav span {
  font-size: 11px;
  color: var(--q-muted);
}
.quality-config-editor {
  min-width: 0;
  border-left: 1px solid var(--q-line);
  padding-left: 24px;
}
.quality-question,
.quality-config-group {
  padding: 0;
  border: 0;
  border-radius: 0;
  background: transparent;
  margin: 0;
  overflow: visible;
}
.quality-question > header,
.quality-config-group > header {
  display: flex;
  align-items: center;
  gap: 12px;
  margin-bottom: 12px;
  padding: 0;
  border: 0;
}
.quality-question > header h3,
.quality-config-group > header h3 {
  min-width: 0;
  flex: 1;
  overflow-wrap: anywhere;
}
.quality-question-fields {
  display: grid;
  grid-template-columns: 2fr 1fr 1fr;
  gap: 12px;
}
.quality-question > .quality-field {
  margin: 12px 0;
}
.quality-question .input {
  width: 100%;
  min-width: 0;
}
.quality-question summary {
  cursor: pointer;
  font-size: 12px;
  padding: 8px 0;
}
.quality-settings-toolbar {
  display: flex;
  flex-wrap: wrap;
  gap: 10px;
  margin: 0;
  padding: 0 0 12px;
}
.quality-interval {
  display: flex;
  align-items: center;
  gap: 8px;
  font-size: 12px;
  margin-right: auto;
}
.quality-interval .input {
  width: 78px;
  min-width: 0;
}
.quality-target {
  border-top: 1px solid var(--q-line);
  padding: 0;
}
.quality-target > summary {
  cursor: pointer;
  padding: 12px 0;
  font-size: 13px;
  overflow-wrap: anywhere;
}
.quality-target > summary span {
  color: var(--q-muted);
  font-size: 11px;
  margin-left: 12px;
}
.quality-target-fields {
  padding-bottom: 16px;
}
.quality-target-footer {
  display: flex;
  align-items: end;
  gap: 12px;
  margin-top: 12px;
}
.quality-target-footer fieldset {
  min-width: 0;
  flex: 1;
  margin: 0;
}
.quality-target-footer legend {
  margin-bottom: 6px;
}
.quality-checks {
  flex-wrap: wrap;
  gap: 8px 16px;
}
.quality-checks label {
  font-size: 12px;
}
.quality-settings-footer {
  display: flex;
  justify-content: space-between;
  align-items: center;
  gap: 12px;
  border-top: 1px solid var(--q-line);
  padding-top: 16px;
  margin-top: 20px;
}
@media (max-width: 1000px) {
  .quality-config-workspace {
    grid-template-columns: 180px minmax(0, 1fr);
    gap: 16px;
  }
  .quality-config-editor {
    padding-left: 16px;
  }
}
@media (max-width: 700px) {
  .quality-settings {
    padding: 12px;
  }
  .quality-config-workspace {
    grid-template-columns: minmax(0, 1fr);
  }
  .quality-config-index nav {
    grid-auto-flow: column;
    grid-auto-columns: minmax(160px, 220px);
    grid-template-columns: none;
    overflow-x: auto;
  }
  .quality-config-editor {
    border-left: 0;
    border-top: 1px solid var(--q-line);
    padding: 16px 0 0;
  }
  .quality-question-fields {
    grid-template-columns: repeat(2, minmax(0, 1fr));
  }
  .quality-question-fields > :first-child {
    grid-column: 1 / -1;
  }
  .quality-heading {
    align-items: start;
  }
  .quality-switch {
    flex-shrink: 0;
    gap: 6px;
  }
  .quality-config-group > header {
    flex-wrap: wrap;
  }
  .quality-config-group > header .quality-muted {
    display: none;
  }
}
</style>

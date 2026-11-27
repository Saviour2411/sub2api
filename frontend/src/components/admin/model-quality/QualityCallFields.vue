<template>
  <div class="quality-call-fields">
    <label v-if="!fixedGroup" class="quality-field"
      >分组<select v-model.number="call.group_id" class="input" @change="call.key_id = null">
        <option :value="0">请选择分组</option>
        <option v-for="group in groups" :key="group.id" :value="group.id">
          {{ group.name }} · {{ group.platform }}
        </option>
      </select></label
    >
    <label class="quality-field"
      >请求密钥<select v-model="call.key_id" class="input">
        <option :value="null">请选择密钥</option>
        <option v-for="key in keys" :key="key.id" :value="key.id">
          {{ key.name }} · {{ key.masked_key }}
        </option>
        <option
          v-if="call.key_id && !keys.some((k) => k.id === call.key_id)"
          :value="call.key_id"
          disabled
        >
          原密钥不可用
        </option></select
      ><span v-if="keyError" class="quality-error">{{ keyError }}</span></label
    >
    <label class="quality-field"
      >接口<select v-model="call.protocol" class="input" @change="resetThinking">
        <option value="">请选择接口</option>
        <option value="chat_completions">Chat Completions</option>
        <option value="responses">Responses</option>
        <option value="messages">Claude Messages</option>
        <option value="gemini">Gemini generateContent</option>
      </select></label
    >
    <label class="quality-field"
      >模型名称<input
        v-model.trim="call.model"
        class="input"
        maxlength="200"
        :list="listId"
        placeholder="模型 ID" /><datalist :id="listId">
        <option v-for="model in models" :key="model" :value="model" /></datalist
    ></label>
    <label class="quality-field"
      >推理强度<input
        v-model.trim="call.reasoning_effort"
        class="input"
        maxlength="32"
        placeholder="默认 / low / medium / high"
    /></label>
    <label
      v-if="call.protocol === 'messages' || call.protocol === 'chat_completions'"
      class="quality-field"
      >Thinking 模式<select
        v-model="call.thinking_mode"
        class="input"
        @change="call.thinking_budget = null"
      >
        <option value="">默认</option>
        <option value="enabled">enabled</option>
        <option value="disabled">disabled</option>
        <option value="adaptive">adaptive</option>
      </select></label
    >
    <label
      v-if="call.thinking_mode === 'enabled' || call.protocol === 'gemini'"
      class="quality-field"
      >Thinking 预算<input
        :value="call.thinking_budget ?? ''"
        type="number"
        min="-1"
        max="262144"
        class="input"
        @input="setBudget"
    /></label>
    <label class="quality-field"
      >输出上限（tokens）<input
        v-model.number="call.max_output_tokens"
        type="number"
        min="1"
        max="262144"
        class="input"
        required
    /></label>
    <label v-if="call.protocol === 'chat_completions'" class="quality-field"
      >输出上限字段<select v-model="call.chat_token_field" class="input">
        <option value="max_completion_tokens">max_completion_tokens</option>
        <option value="max_tokens">max_tokens</option>
      </select></label
    >
    <label class="quality-field"
      >超时（秒）<input
        v-model.number="call.timeout_seconds"
        type="number"
        min="30"
        max="3600"
        class="input"
        required
    /></label>
    <details class="quality-extra">
      <summary>高级请求参数</summary>
      <label class="quality-field"
        >JSON 对象<textarea
          v-model="extra"
          class="input"
          rows="4"
          spellcheck="false"
          @input="parseExtra"
        /><span v-if="extraError" class="quality-error">{{ extraError }}</span></label
      >
    </details>
  </div>
</template>
<script setup lang="ts">
import { ref, watch } from 'vue'
import { modelQualityAPI, type QualityCall, type QualityKey } from '@/api/modelQuality'
import { groupsAPI } from '@/api/admin/groups'
import type { Group } from '@/types'
const call = defineModel<QualityCall>('call', { required: true })
const props = defineProps<{
  groups: Group[]
  fixedGroup?: boolean
  fieldId: string
}>()
const emit = defineEmits<{ valid: [id: string, valid: boolean] }>()
const keys = ref<QualityKey[]>([]),
  models = ref<string[]>([]),
  keyError = ref('')
const listId = 'quality-models-' + props.fieldId
const extra = ref(JSON.stringify(call.value.extra_body || {}, null, 2)),
  extraError = ref('')
let generation = 0
watch(
  () => call.value.group_id,
  async (id) => {
    const current = ++generation
    keys.value = []
    models.value = []
    keyError.value = ''
    if (!id) return
    const values = await Promise.allSettled([
      modelQualityAPI.keys(id),
      groupsAPI.getModelAllowlistCandidates(id)
    ])
    if (current !== generation) return
    if (values[0].status === 'fulfilled') keys.value = values[0].value
    else keyError.value = '密钥列表加载失败'
    if (values[1].status === 'fulfilled') models.value = values[1].value
  },
  { immediate: true }
)
function resetThinking() {
  call.value.reasoning_effort = ''
  call.value.thinking_mode = ''
  call.value.thinking_budget = null
  call.value.chat_token_field = 'max_completion_tokens'
}
function setBudget(event: Event) {
  const value = (event.target as HTMLInputElement).value
  call.value.thinking_budget = value === '' ? null : Number(value)
}
function parseExtra(event: Event) {
  try {
    const value = JSON.parse(extra.value)
    if (!value || typeof value !== 'object' || Array.isArray(value)) throw new Error()
    call.value.extra_body = value
    extraError.value = ''
  } catch {
    extraError.value = '请输入有效的 JSON 对象'
  }
  const textarea = event.target as HTMLTextAreaElement
  textarea.setCustomValidity(extraError.value)
  emit('valid', props.fieldId, !extraError.value)
}
</script>
<style scoped>
.quality-call-fields {
  display: grid;
  grid-template-columns: repeat(3, minmax(0, 1fr));
  gap: 14px;
}
.quality-call-fields .input {
  width: 100%;
  min-width: 0;
}
.quality-extra {
  grid-column: 1 / -1;
}
.quality-extra summary {
  cursor: pointer;
  padding: 6px 0;
}
@media (max-width: 900px) {
  .quality-call-fields {
    grid-template-columns: repeat(2, minmax(0, 1fr));
  }
}
@media (max-width: 550px) {
  .quality-call-fields {
    grid-template-columns: minmax(0, 1fr);
  }
}
</style>

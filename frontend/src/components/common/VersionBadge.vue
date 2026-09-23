<template>
  <div class="version-badge" :class="{ 'version-badge-collapsed': collapsed }">
    <button
      v-if="collapsed"
      ref="trigger"
      type="button"
      class="version-info-button"
      :title="t('versionInfo.details')"
      :aria-label="t('versionInfo.details')"
      :aria-expanded="open"
      @click="toggleDetails"
    >
      <Icon name="infoCircle" size="sm" />
      <span v-if="admin && upstream?.has_update" class="version-update-dot" />
    </button>
    <Teleport to="body" :disabled="!collapsed">
      <div
        v-if="!collapsed || open"
        ref="details"
        class="version-details"
        :class="{ 'version-popover': collapsed }"
        :style="collapsed ? position : undefined"
        :role="collapsed ? 'dialog' : undefined"
        :aria-label="collapsed ? t('versionInfo.details') : undefined"
        @keydown.esc="closeDetails"
      >
        <div class="version-current" data-testid="current-version">
          <span v-if="admin" class="version-label">{{ t('versionInfo.custom') }}</span>
          <span class="version-number">{{ displayVersion(currentVersion) }}</span>
        </div>
        <template v-if="admin">
          <div data-testid="synced-version" :title="syncDetails">
            <span class="version-label">{{ t('versionInfo.synced') }}</span>
            <span class="version-number">{{ versionInfo?.upstream_version ? displayVersion(versionInfo.upstream_version) : t('versionInfo.notRecorded') }}</span>
          </div>
          <div class="version-status" data-testid="upstream-status" :title="checkDetails" aria-live="polite">
            <component :is="releaseURL ? 'a' : 'span'" v-if="upstream?.has_update" :href="releaseURL || undefined" target="_blank" rel="noopener noreferrer" class="version-new">
              {{ t('versionInfo.newRelease', { version: displayVersion(upstream.latest_version) }) }}
              <Icon v-if="releaseURL" name="externalLink" size="xs" class="version-link-icon" />
            </component>
            <span v-else-if="checking && !upstream">{{ t('versionInfo.checking') }}</span>
            <span v-else-if="!upstream || upstream.status !== 'ok'">{{ t('versionInfo.unavailable') }}</span>
            <span v-else-if="upstream.has_update === null">{{ t('versionInfo.latest', { version: displayVersion(upstream.latest_version) }) }}</span>
            <span v-else>{{ t('versionInfo.noUpdate') }}</span>
            <span v-if="upstream?.stale" class="version-stale">{{ t('versionInfo.cached') }}</span>
          </div>
        </template>
      </div>
    </Teleport>
  </div>
</template>

<script setup lang="ts">
import { computed, nextTick, onMounted, onScopeDispose, ref, watch, type CSSProperties } from 'vue'
import { useI18n } from 'vue-i18n'
import { onClickOutside } from '@vueuse/core'
import Icon from '@/components/icons/Icon.vue'
import { useVersionInfo } from '@/composables/useVersionInfo'

const props = withDefaults(defineProps<{
  version?: string
  admin?: boolean
  identity?: string | number
  collapsed?: boolean
}>(), { version: '', admin: false, collapsed: false })
const { t } = useI18n()
const { versionInfo, upstream, checking } = useVersionInfo(computed(() => props.admin), computed(() => props.identity))
const currentVersion = computed(() => versionInfo.value?.version?.trim() || props.version.trim())
const displayVersion = (value?: string) => value?.trim() ? `v${value.trim().replace(/^v/, '')}` : t('versionInfo.unknown')
const syncDetails = computed(() => [versionInfo.value?.upstream_repo, versionInfo.value?.upstream_commit, versionInfo.value?.upstream_synced_at].filter(Boolean).join('\n'))
const checkDetails = computed(() => upstream.value?.checked_at ? t('versionInfo.lastChecked', { time: new Date(upstream.value.checked_at).toLocaleString() }) : '')
const releaseURL = computed(() => {
  try {
    const url = new URL(upstream.value?.release_url || '')
    return url.protocol === 'https:' && url.hostname === 'github.com' && !url.username && !url.password && url.pathname.startsWith('/Wei-Shaw/sub2api/releases/tag/') ? url.href : ''
  } catch { return '' }
})
const open = ref(false)
const trigger = ref<HTMLElement>()
const details = ref<HTMLElement>()
const position = ref<CSSProperties>({})

function closeDetails() { open.value = false }
async function toggleDetails() {
  open.value = !open.value
  if (!open.value) return
  await nextTick()
  const rect = trigger.value?.getBoundingClientRect()
  if (!rect) return
  const width = Math.min(280, window.innerWidth - 24)
  const height = details.value?.offsetHeight || 120
  position.value = {
    width: `${width}px`,
    left: `${Math.max(12, Math.min(rect.right + 10, window.innerWidth - width - 12))}px`,
    top: `${Math.max(12, Math.min(rect.top, window.innerHeight - height - 12))}px`
  }
}
function onKeydown(event: KeyboardEvent) {
  if (event.key === 'Escape' && open.value) { closeDetails(); trigger.value?.focus() }
}
onClickOutside(details, closeDetails, { ignore: [trigger] })
watch(() => [props.collapsed, props.admin, props.identity], closeDetails)
onMounted(() => {
  window.addEventListener('keydown', onKeydown)
  window.addEventListener('resize', closeDetails)
})
onScopeDispose(() => {
  window.removeEventListener('keydown', onKeydown)
  window.removeEventListener('resize', closeDetails)
})
</script>

<style scoped>
.version-badge { min-width: 0; white-space: normal; }
.version-details { font-size: 11px; line-height: 16px; color: #52647c; overflow-wrap: anywhere; letter-spacing: 0; }
.version-current { color: #36465a; }
.version-label { margin-right: 5px; }
.version-number { font-variant-numeric: tabular-nums; }
.version-status { min-height: 16px; }
.version-new { color: #087d5f; font-weight: 500; }
.version-new:hover { text-decoration: underline; }
.version-link-icon { display: inline; vertical-align: -2px; }
.version-stale { margin-left: 4px; color: #a66a13; }
.version-info-button { position: relative; display: flex; width: 24px; height: 24px; align-items: center; justify-content: center; border-radius: 4px; color: #52647c; }
.version-info-button:hover { background: #e4edf5; }
.version-update-dot { position: absolute; top: 2px; right: 2px; width: 5px; height: 5px; border-radius: 50%; background: #168367; }
.version-popover { position: fixed; z-index: 100; padding: 12px; border: 1px solid #cedbe7; border-radius: 6px; background: #fff; box-shadow: 0 4px 18px #0002; max-height: calc(100dvh - 24px); overflow-y: auto; }
.dark .version-details, .dark .version-info-button { color: #a5b7ca; }
.dark .version-current { color: #d2dce7; }
.dark .version-new { color: #56d6b0; }
.dark .version-stale { color: #e6b566; }
.dark .version-popover { background: #182330; border-color: #3a4d60; }
.dark .version-info-button:hover { background: #304154; }
</style>

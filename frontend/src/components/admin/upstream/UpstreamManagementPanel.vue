<template>
  <section data-test="upstream-management-panel" class="space-y-3">
    <div class="flex flex-col gap-3 sm:flex-row sm:items-start sm:justify-between">
      <div>
        <h2 class="text-lg font-semibold text-gray-900 dark:text-white">
          {{ t('admin.customFeatures.upstream.title') }}
        </h2>
        <p class="mt-1 text-sm text-gray-500 dark:text-gray-400">
          {{ t('admin.customFeatures.upstream.description') }}
        </p>
      </div>
      <div class="flex flex-wrap items-center gap-2">
        <button
          type="button"
          class="btn btn-secondary inline-flex items-center gap-2"
          :disabled="syncingAll"
          @click="syncAllSites"
        >
          <Icon name="refresh" size="sm" :class="{ 'animate-spin': syncingAll }" />
          {{ t('admin.customFeatures.upstream.syncAll') }}
        </button>
        <button type="button" class="btn btn-secondary inline-flex items-center gap-2" data-test="upstream-sort" @click="openSortModal">
          <Icon name="arrowsUpDown" size="sm" />
          {{ t('admin.customFeatures.upstream.sortOrder') }}
        </button>
        <button type="button" class="btn btn-primary inline-flex items-center gap-2" data-test="upstream-add" @click="openCreate">
          <Icon name="plus" size="sm" />
          {{ t('admin.customFeatures.upstream.add') }}
        </button>
      </div>
    </div>

    <div class="grid grid-cols-1 gap-2 sm:grid-cols-2 xl:grid-cols-[minmax(240px,1fr)_150px_150px_170px_210px]">
      <div class="relative sm:col-span-2 xl:col-span-1">
        <Icon name="search" size="sm" class="pointer-events-none absolute left-3 top-3 text-gray-400" />
        <input
          v-model="search"
          type="search"
          class="input pl-9"
          :placeholder="t('admin.customFeatures.upstream.searchPlaceholder')"
          @keyup.enter="applyFilters"
        />
      </div>
      <select v-model="platformFilter" class="input" :aria-label="t('admin.customFeatures.upstream.platform')" @change="applyFilters">
        <option value="">{{ t('admin.customFeatures.upstream.allPlatforms') }}</option>
        <option value="sub2api">Sub2API</option>
        <option value="newapi">New API</option>
      </select>
      <select v-model="enabledFilter" class="input" :aria-label="t('admin.customFeatures.upstream.enabledState')" @change="applyFilters">
        <option value="">{{ t('admin.customFeatures.upstream.allStates') }}</option>
        <option value="true">{{ t('admin.customFeatures.upstream.enabled') }}</option>
        <option value="false">{{ t('admin.customFeatures.upstream.disabled') }}</option>
      </select>
      <select v-model="groupPlatformFilter" class="input" :aria-label="t('admin.customFeatures.upstream.groupPlatform')" @change="applyFilters">
        <option value="">{{ t('admin.customFeatures.upstream.allGroupPlatforms') }}</option>
        <option v-for="platform in groupPlatforms" :key="platform" :value="platform">{{ platform }}</option>
      </select>
      <select v-model="sortSelection" class="input" :aria-label="t('admin.customFeatures.upstream.sortBy')" @change="applySortSelection">
        <option value="">{{ t('admin.customFeatures.upstream.defaultSort') }}</option>
        <option value="balance_desc">{{ t('admin.customFeatures.upstream.sort.balanceDesc') }}</option>
        <option value="balance_asc">{{ t('admin.customFeatures.upstream.sort.balanceAsc') }}</option>
        <option value="today_tokens_desc">{{ t('admin.customFeatures.upstream.sort.todayTokensDesc') }}</option>
        <option value="today_tokens_asc">{{ t('admin.customFeatures.upstream.sort.todayTokensAsc') }}</option>
      </select>
    </div>

    <div v-if="loadError" class="rounded-md border border-red-200 bg-red-50 px-4 py-3 text-sm text-red-700 dark:border-red-900/50 dark:bg-red-950/20 dark:text-red-300">
      <div class="flex items-center justify-between gap-3">
        <span>{{ loadError }}</span>
        <button type="button" class="btn btn-secondary inline-flex items-center gap-2" @click="loadSites(false)">
          <Icon name="refresh" size="sm" />
          {{ t('common.tryAgain') }}
        </button>
      </div>
    </div>

    <DataTable
      v-else
      :columns="columns"
      :data="sites"
      :loading="loading"
      row-key="id"
      :sticky-first-column="true"
      :sticky-actions-column="true"
      :expandable-actions="false"
      :expanded-row-keys="expandedSiteKeys"
      compact
    >
      <template #cell-site="{ row }">
        <div class="flex max-w-full items-start gap-1 whitespace-normal">
          <button
            v-if="row.displayed_group_count > 0"
            type="button"
            class="icon-action mt-[-0.25rem] flex-shrink-0"
            :title="isSiteExpanded(row.id) ? t('admin.customFeatures.upstream.hideGroups') : t('admin.customFeatures.upstream.showGroups')"
            :aria-expanded="isSiteExpanded(row.id)"
            :aria-controls="`upstream-groups-${row.id}`"
            :data-test="`upstream-expand-${row.id}`"
            @click.stop="toggleSiteGroups(row)"
          >
            <Icon :name="isSiteExpanded(row.id) ? 'chevronDown' : 'chevronRight'" size="sm" />
          </button>
          <div class="min-w-0">
            <a
              :href="row.base_url"
              target="_blank"
              rel="noopener noreferrer"
              class="inline-flex items-center gap-1 font-medium text-primary-600 hover:underline dark:text-primary-400"
            >
              <span class="break-words">{{ row.name }}</span>
              <Icon name="externalLink" size="xs" class="flex-shrink-0" />
            </a>
            <p class="mt-1 truncate text-xs text-gray-500 dark:text-gray-400" :title="row.base_url">{{ row.base_url }}</p>
          </div>
        </div>
      </template>
      <template #cell-platform="{ row }">
        <span class="rounded px-2 py-1 text-xs font-semibold" :class="sitePlatformClass(row.platform)">
          {{ platformLabel(row.platform) }}
        </span>
      </template>
      <template #cell-status="{ row }">
        <div class="flex flex-wrap items-center gap-1">
          <span :class="statusClass(row.status)" class="rounded px-2 py-1 text-xs font-medium" :title="row.error_message || undefined">
            {{ t(`admin.customFeatures.upstream.status.${row.status}`) }}
          </span>
          <span class="rounded px-1.5 py-1 text-xs font-medium" :class="row.enabled ? 'bg-emerald-50 text-emerald-700 dark:bg-emerald-900/20 dark:text-emerald-300' : 'bg-gray-100 text-gray-500 dark:bg-dark-700 dark:text-gray-400'">
            {{ row.enabled ? t('admin.customFeatures.upstream.enabled') : t('admin.customFeatures.upstream.disabled') }}
          </span>
        </div>
      </template>
      <template #cell-balance_usd="{ row }">
        <span class="font-medium">{{ formatMoney(row.balance_usd) }}</span>
      </template>
      <template #cell-today="{ row }">
        <div class="space-y-1 text-xs">
          <p class="font-medium text-gray-800 dark:text-gray-200" :title="formatExactTokens(row.today_tokens)">{{ formatTokens(row.today_tokens) }}</p>
          <p class="text-gray-500 dark:text-gray-400">{{ formatMoney(row.today_cost_usd) }}</p>
        </div>
      </template>
      <template #cell-total="{ row }">
        <div class="space-y-1 text-xs">
          <p class="font-medium text-gray-800 dark:text-gray-200" :title="formatExactTokens(row.total_tokens)">{{ formatTokens(row.total_tokens) }}</p>
          <p class="text-gray-500 dark:text-gray-400">{{ formatMoney(row.total_cost_usd) }}</p>
        </div>
      </template>
      <template #cell-last_synced_at="{ row }">
        <span
          class="block whitespace-normal text-xs leading-4 text-gray-600 dark:text-gray-300"
          :data-test="`upstream-last-sync-${row.id}`"
        >{{ formatDateTime(row.last_synced_at) }}</span>
      </template>
      <template #cell-actions="{ row }">
        <div
          class="ml-auto grid w-[6.5rem] grid-cols-3 justify-items-center gap-1"
          :data-test="`upstream-actions-${row.id}`"
          @click.stop
        >
          <a :href="row.base_url" target="_blank" rel="noopener noreferrer" class="icon-action" :title="t('admin.customFeatures.upstream.openSite')">
            <Icon name="externalLink" size="sm" />
          </a>
          <button type="button" class="icon-action" :title="t('admin.customFeatures.upstream.sync')" :disabled="row.status === 'syncing'" @click="syncSite(row)">
            <Icon name="refresh" size="sm" :class="{ 'animate-spin': row.status === 'syncing' }" />
          </button>
          <button type="button" class="icon-action" :title="t('admin.customFeatures.upstream.details')" @click="openDetails(row)">
            <Icon name="chart" size="sm" />
          </button>
          <button type="button" class="icon-action" :title="t('admin.customFeatures.upstream.edit')" @click="openEdit(row)">
            <Icon name="edit" size="sm" />
          </button>
          <button type="button" class="icon-action" :title="row.enabled ? t('admin.customFeatures.upstream.disable') : t('admin.customFeatures.upstream.enable')" @click="toggleSite(row)">
            <Icon :name="row.enabled ? 'ban' : 'play'" size="sm" />
          </button>
          <button type="button" class="icon-action text-red-600 hover:text-red-700 dark:text-red-400" :title="t('admin.customFeatures.upstream.delete')" @click="deleteTarget = row">
            <Icon name="trash" size="sm" />
          </button>
        </div>
      </template>
      <template #row-details="{ row }">
        <section :id="`upstream-groups-${row.id}`" class="bg-gray-50/80 px-1 py-1 dark:bg-dark-800/40" :data-test="`upstream-groups-${row.id}`">
          <div v-if="groupState(row.id).loading" class="flex min-h-24 items-center justify-center">
            <span class="h-6 w-6 animate-spin rounded-full border-b-2 border-primary-600"></span>
          </div>
          <div v-else-if="groupState(row.id).error" class="flex min-h-24 flex-col items-center justify-center gap-3 text-center text-sm text-red-600 dark:text-red-300">
            <span>{{ groupState(row.id).error }}</span>
            <button type="button" class="btn btn-secondary inline-flex items-center gap-2" :data-test="`upstream-groups-retry-${row.id}`" @click="loadSiteGroups(row, true)">
              <Icon name="refresh" size="sm" />
              {{ t('common.tryAgain') }}
            </button>
          </div>
          <p v-else-if="displayedGroups(row.id).length === 0" class="py-5 text-center text-sm text-gray-500 dark:text-gray-400">
            {{ t('admin.customFeatures.upstream.noDisplayedGroups') }}
          </p>
          <div v-else class="grid grid-cols-1 gap-1.5 sm:grid-cols-2 lg:grid-cols-3 2xl:grid-cols-4" data-test="expanded-groups-grid">
            <article
              v-for="group in displayedGroups(row.id)"
              :key="group.remote_id"
              class="rounded-md border bg-white p-2.5 dark:bg-dark-900"
              :class="group.available ? 'border-gray-200 dark:border-dark-700' : 'border-amber-300 bg-amber-50/40 dark:border-amber-800 dark:bg-amber-950/10'"
            >
              <div class="flex min-w-0 items-start justify-between gap-1.5">
                <div class="min-w-0">
                  <div class="flex flex-wrap items-center gap-1">
                    <h4 class="break-words text-xs font-semibold leading-4 text-gray-900 dark:text-gray-100" :title="group.name">{{ group.name }}</h4>
                    <span v-if="!group.available" class="rounded bg-amber-100 px-1 py-0.5 text-[10px] font-semibold leading-3 text-amber-800 dark:bg-amber-900/30 dark:text-amber-300">{{ t('admin.customFeatures.upstream.unavailable') }}</span>
                  </div>
                  <p v-if="group.description" class="mt-0.5 line-clamp-1 break-words text-[10px] leading-4 text-gray-500 dark:text-gray-400" :title="group.description">{{ group.description }}</p>
                </div>
                <span
                  class="flex-shrink-0 rounded px-1.5 py-0.5 text-[10px] font-semibold leading-4"
                  :class="groupPlatformClass(displayGroupPlatform(group))"
                  :data-test="`upstream-group-platform-${row.id}-${group.remote_id}`"
                >{{ displayGroupPlatform(group) }}</span>
              </div>
              <dl class="mt-1.5 grid grid-cols-3 gap-1.5 border-t border-gray-100 pt-1.5 dark:border-dark-700">
                <div>
                  <dt class="text-[10px] leading-4 text-gray-500 dark:text-gray-400">{{ t('admin.customFeatures.upstream.groupColumns.multiplier') }}</dt>
                  <dd class="mt-0.5">
                    <button
                      type="button"
                      class="rounded bg-amber-100 px-1.5 py-0.5 text-[11px] font-semibold leading-4 text-amber-800 transition-colors hover:bg-amber-200 focus:outline-none focus:ring-2 focus:ring-amber-400 focus:ring-offset-1 dark:bg-amber-900/30 dark:text-amber-300 dark:hover:bg-amber-900/50"
                      :title="t('admin.customFeatures.upstream.viewMultiplierTrend')"
                      :aria-label="t('admin.customFeatures.upstream.viewMultiplierTrendFor', { name: group.name })"
                      :data-test="`upstream-group-multiplier-${row.id}-${group.remote_id}`"
                      @click.stop="openMultiplierTrend(row, group)"
                    >
                      {{ formatMultiplier(group.multiplier) }}
                    </button>
                  </dd>
                </div>
                <div>
                  <dt class="text-[10px] leading-4 text-gray-500 dark:text-gray-400">{{ t('admin.customFeatures.upstream.groupColumns.tokens') }}</dt>
                  <dd class="mt-0.5 text-xs font-medium leading-4 text-gray-900 dark:text-gray-100" :title="formatExactTokens(group.today_tokens)">{{ formatTokens(group.today_tokens) }}</dd>
                </div>
                <div>
                  <dt class="text-[10px] leading-4 text-gray-500 dark:text-gray-400">{{ t('admin.customFeatures.upstream.groupColumns.cost') }}</dt>
                  <dd class="mt-0.5 text-xs font-medium leading-4 text-gray-900 dark:text-gray-100">{{ formatMoney(group.today_cost_usd) }}</dd>
                </div>
              </dl>
              <p v-if="!group.available" class="mt-1.5 text-[10px] leading-4 text-amber-700 dark:text-amber-300">
                {{ t('admin.customFeatures.upstream.lastSeenAt', { time: formatDateTime(group.last_synced_at) }) }}
              </p>
              <div class="mt-2 border-t border-gray-100 pt-2 dark:border-dark-700">
                <button
                  type="button"
                  class="btn btn-secondary inline-flex w-full items-center justify-center gap-1.5 px-2 py-1.5 text-xs"
                  :title="t('admin.customFeatures.upstream.bindings.open')"
                  :data-test="`upstream-group-bindings-${row.id}-${group.remote_id}`"
                  @click.stop="openBindings(row, group)"
                >
                  <Icon name="link" size="xs" />
                  <span>{{ t('admin.customFeatures.upstream.bindings.button') }}</span>
                  <span class="rounded bg-gray-200 px-1.5 py-0.5 text-[10px] font-semibold text-gray-700 dark:bg-dark-600 dark:text-gray-200">
                    {{ bindingCount(group) }}
                  </span>
                </button>
              </div>
            </article>
          </div>
        </section>
      </template>
      <template #empty>
        <div class="flex flex-col items-center py-4">
          <Icon name="server" size="xl" class="mb-3 text-gray-400" />
          <p class="font-medium text-gray-800 dark:text-gray-200">{{ t('admin.customFeatures.upstream.empty') }}</p>
        </div>
      </template>
    </DataTable>

    <div v-if="total > pageSize" class="flex items-center justify-between gap-4 text-sm text-gray-500 dark:text-gray-400">
      <span>{{ t('admin.customFeatures.upstream.pagination', { page, pages, total }) }}</span>
      <div class="flex gap-2">
        <button type="button" class="btn btn-secondary" :disabled="page <= 1" @click="changePage(page - 1)">{{ t('admin.customFeatures.upstream.previous') }}</button>
        <button type="button" class="btn btn-secondary" :disabled="page >= pages" @click="changePage(page + 1)">{{ t('admin.customFeatures.upstream.next') }}</button>
      </div>
    </div>

    <BaseDialog :show="sortOpen" :title="t('admin.customFeatures.upstream.sortOrder')" width="normal" @close="closeSortModal">
      <div class="space-y-4">
        <p class="text-sm text-gray-500 dark:text-gray-400">{{ t('admin.customFeatures.upstream.sortOrderHint') }}</p>
        <div v-if="sortLoading" class="flex h-32 items-center justify-center">
          <span class="h-7 w-7 animate-spin rounded-full border-b-2 border-primary-600"></span>
        </div>
        <VueDraggable
          v-else
          v-model="sortableSites"
          :animation="200"
          class="space-y-2"
          data-test="upstream-sortable-sites"
        >
          <div
            v-for="site in sortableSites"
            :key="site.id"
            class="flex cursor-grab items-center gap-3 rounded-lg border border-gray-200 bg-white p-3 transition-shadow hover:shadow-md active:cursor-grabbing dark:border-dark-600 dark:bg-dark-700"
            :data-test="`upstream-sort-item-${site.id}`"
          >
            <Icon name="menu" size="md" class="flex-shrink-0 text-gray-400" />
            <div class="min-w-0 flex-1">
              <div class="truncate font-medium text-gray-900 dark:text-white">{{ site.name }}</div>
              <div class="truncate text-xs text-gray-500 dark:text-gray-400">{{ site.base_url }}</div>
            </div>
            <span class="text-sm text-gray-400">#{{ site.id }}</span>
          </div>
        </VueDraggable>
      </div>
      <template #footer>
        <div class="flex justify-end gap-3 pt-4">
          <button type="button" class="btn btn-secondary" @click="closeSortModal">{{ t('admin.customFeatures.upstream.cancel') }}</button>
          <button type="button" class="btn btn-primary inline-flex items-center gap-2" :disabled="sortSubmitting || sortLoading" @click="saveSortOrder">
            <span v-if="sortSubmitting" class="h-4 w-4 animate-spin rounded-full border-b-2 border-white"></span>
            <Icon v-else name="check" size="sm" />
            {{ sortSubmitting ? t('common.saving') : t('common.save') }}
          </button>
        </div>
      </template>
    </BaseDialog>

    <BaseDialog :show="formOpen" :title="editingSite ? t('admin.customFeatures.upstream.editTitle') : t('admin.customFeatures.upstream.addTitle')" width="wide" @close="closeForm">
      <form class="grid grid-cols-1 gap-5 md:grid-cols-2" data-test="upstream-form" @submit.prevent="submitForm">
        <div>
          <label for="upstream-name" class="input-label">{{ t('admin.customFeatures.upstream.name') }}</label>
          <input id="upstream-name" v-model="form.name" class="input" required maxlength="100" />
        </div>
        <div>
          <label for="upstream-url" class="input-label">{{ t('admin.customFeatures.upstream.baseUrl') }}</label>
          <input
            id="upstream-url"
            v-model="form.base_url"
            class="input"
            type="url"
            required
            placeholder="https://example.com"
            @input="handleBaseURLInput"
            @blur="probeSiteCapabilities()"
          />
          <p v-if="capabilityProbeLoading" class="mt-1 text-xs text-gray-500 dark:text-gray-400">{{ t('admin.customFeatures.upstream.probingCapabilities') }}</p>
          <p v-else-if="capabilityProbeError" class="mt-1 text-xs text-red-600 dark:text-red-300">{{ capabilityProbeError }}</p>
        </div>
        <div>
          <label for="upstream-platform" class="input-label">{{ t('admin.customFeatures.upstream.platform') }}</label>
          <select id="upstream-platform" v-model="form.platform" class="input" @change="handlePlatformChange">
            <option value="sub2api">Sub2API</option>
            <option value="newapi">New API</option>
          </select>
        </div>
        <div>
          <label for="upstream-auth-mode" class="input-label">{{ t('admin.customFeatures.upstream.authMode') }}</label>
          <select id="upstream-auth-mode" v-model="form.auth_mode" class="input" :disabled="form.platform === 'newapi'" @change="handleAuthModeChange">
            <option value="password" :disabled="turnstileDetected">{{ t('admin.customFeatures.upstream.passwordAuth') }}</option>
            <option v-if="form.platform === 'sub2api'" value="token">{{ t('admin.customFeatures.upstream.tokenAuth') }}</option>
          </select>
        </div>
        <div
          v-if="turnstileDetected"
          class="flex flex-col gap-3 rounded-md border border-amber-300 bg-amber-50 px-3 py-3 text-amber-900 dark:border-amber-800 dark:bg-amber-950/20 dark:text-amber-200 md:col-span-2 sm:flex-row sm:items-start sm:justify-between"
          data-test="upstream-turnstile-notice"
        >
          <div class="flex min-w-0 items-start gap-2">
            <Icon name="shield" size="sm" class="mt-0.5 flex-shrink-0" />
            <div class="min-w-0">
              <p class="text-sm font-semibold">{{ t('admin.customFeatures.upstream.turnstileDetectedTitle') }}</p>
              <p class="mt-1 text-xs leading-5">{{ t('admin.customFeatures.upstream.turnstileDetectedDescription') }}</p>
            </div>
          </div>
          <a
            v-if="upstreamLoginURL"
            :href="upstreamLoginURL"
            target="_blank"
            rel="noopener noreferrer"
            class="btn btn-secondary inline-flex flex-shrink-0 items-center justify-center gap-2"
            data-test="upstream-open-login"
          >
            <Icon name="externalLink" size="sm" />
            {{ t('admin.customFeatures.upstream.openLoginPage') }}
          </a>
        </div>
        <div v-if="form.auth_mode === 'password'">
          <label for="upstream-account" class="input-label">{{ t('admin.customFeatures.upstream.account') }}</label>
          <input id="upstream-account" v-model="form.account" class="input" autocomplete="username" required />
        </div>
        <div v-if="form.auth_mode === 'password'">
          <label for="upstream-password" class="input-label">{{ t('admin.customFeatures.upstream.password') }}</label>
          <input id="upstream-password" v-model="form.password" class="input" type="password" autocomplete="new-password" :required="!editingSite || editingSite.auth_mode !== 'password' || !editingSite.has_password" :placeholder="editingSite?.auth_mode === 'password' && editingSite.has_password ? t('admin.customFeatures.upstream.keepCredential') : ''" />
        </div>
        <template v-else>
          <div>
            <label for="upstream-access-token" class="input-label">{{ t('admin.customFeatures.upstream.accessToken') }}</label>
            <input id="upstream-access-token" v-model="form.access_token" class="input" type="password" autocomplete="off" :placeholder="editingSite?.auth_mode === 'token' && editingSite.has_token ? t('admin.customFeatures.upstream.keepCredential') : ''" />
          </div>
          <div>
            <label for="upstream-refresh-token" class="input-label">{{ t('admin.customFeatures.upstream.refreshToken') }}</label>
            <input id="upstream-refresh-token" v-model="form.refresh_token" class="input" type="password" autocomplete="off" :placeholder="editingSite?.auth_mode === 'token' && editingSite.has_token ? t('admin.customFeatures.upstream.keepCredential') : ''" />
          </div>
          <div v-if="turnstileDetected" class="md:col-span-2">
            <label for="upstream-login-response" class="input-label">{{ t('admin.customFeatures.upstream.loginResponse') }}</label>
            <div class="flex flex-col gap-2 sm:flex-row sm:items-stretch">
              <textarea
                id="upstream-login-response"
                v-model="loginResponseJSON"
                class="input min-h-20 flex-1 resize-y font-mono text-xs"
                autocomplete="off"
                spellcheck="false"
                :placeholder="t('admin.customFeatures.upstream.loginResponsePlaceholder')"
                data-test="upstream-login-response"
              ></textarea>
              <button type="button" class="btn btn-secondary inline-flex items-center justify-center gap-2 sm:self-start" data-test="upstream-import-login-response" @click="importLoginResponseTokens">
                <Icon name="clipboard" size="sm" />
                {{ t('admin.customFeatures.upstream.importTokens') }}
              </button>
            </div>
            <p v-if="loginResponseError" class="mt-1 text-xs text-red-600 dark:text-red-300">{{ loginResponseError }}</p>
            <p v-else class="mt-1 text-xs text-gray-500 dark:text-gray-400">{{ t('admin.customFeatures.upstream.loginResponseHint') }}</p>
          </div>
        </template>
        <div class="flex items-center justify-between gap-4 md:col-span-2">
          <span class="text-sm font-medium text-gray-700 dark:text-gray-300">{{ t('admin.customFeatures.upstream.enabled') }}</span>
          <Toggle v-model="form.enabled" />
        </div>
        <div class="flex justify-end gap-2 border-t border-gray-100 pt-4 dark:border-dark-700 md:col-span-2">
          <button type="button" class="btn btn-secondary" @click="closeForm">{{ t('admin.customFeatures.upstream.cancel') }}</button>
          <button type="submit" class="btn btn-primary inline-flex items-center gap-2" :disabled="saving">
            <span v-if="saving" class="h-4 w-4 animate-spin rounded-full border-b-2 border-white"></span>
            <Icon v-else name="check" size="sm" />
            {{ saving ? t('admin.customFeatures.upstream.validating') : t('admin.customFeatures.upstream.save') }}
          </button>
        </div>
      </form>
    </BaseDialog>

    <BaseDialog :show="Boolean(detailSite)" :title="detailSite?.name || ''" width="extra-wide" @close="closeDetails">
      <div class="space-y-5">
        <div class="flex overflow-x-auto border-b border-gray-200 dark:border-dark-700">
          <button v-for="tab in detailTabs" :key="tab" type="button" class="flex-shrink-0 border-b-2 px-4 py-2 text-sm font-medium" :class="detailTab === tab ? 'border-primary-500 text-primary-600 dark:text-primary-400' : 'border-transparent text-gray-500'" @click="selectDetailTab(tab)">
            {{ t(`admin.customFeatures.upstream.detailTabs.${tab}`) }}
          </button>
        </div>
        <div v-if="detailLoading" class="flex h-56 items-center justify-center">
          <span class="h-8 w-8 animate-spin rounded-full border-b-2 border-primary-600"></span>
        </div>
        <template v-else-if="detailTab === 'groups'">
          <DataTable :columns="groupColumns" :data="detailGroups" row-key="remote_id" :sticky-actions-column="true" :expandable-actions="false" compact>
            <template #cell-name="{ row }">
              <div class="max-w-72 whitespace-normal">
                <p class="font-medium" :title="row.name">{{ row.name }}</p>
                <p v-if="row.description" class="mt-1 line-clamp-2 break-words text-xs text-gray-500 dark:text-gray-400" :title="row.description">{{ row.description }}</p>
              </div>
            </template>
            <template #cell-platform="{ row }"><span class="rounded px-2 py-1 text-xs font-semibold" :class="groupPlatformClass(displayGroupPlatform(row))">{{ displayGroupPlatform(row) }}</span></template>
            <template #cell-status="{ row }">
              <span class="rounded px-2 py-1 text-xs font-semibold" :class="row.available ? 'bg-emerald-50 text-emerald-700 dark:bg-emerald-900/20 dark:text-emerald-300' : 'bg-amber-100 text-amber-800 dark:bg-amber-900/30 dark:text-amber-300'">
                {{ row.available ? t('admin.customFeatures.upstream.available') : t('admin.customFeatures.upstream.unavailable') }}
              </span>
            </template>
            <template #cell-multiplier="{ row }"><span class="rounded bg-amber-100 px-2 py-1 text-xs font-semibold text-amber-800 dark:bg-amber-900/30 dark:text-amber-300">{{ formatMultiplier(row.multiplier) }}</span></template>
            <template #cell-today_tokens="{ row }"><span :title="formatExactTokens(row.today_tokens)">{{ formatTokens(row.today_tokens) }}</span></template>
            <template #cell-today_cost_usd="{ row }">{{ formatMoney(row.today_cost_usd) }}</template>
            <template #cell-actions="{ row }">
              <button
                type="button"
                class="btn inline-flex items-center gap-1.5 px-2.5 py-1.5 text-xs"
                :class="row.displayed ? 'btn-secondary' : 'btn-primary'"
                :disabled="isGroupDisplayLoading(row.remote_id) || (!row.available && !row.displayed) || (row.displayed && bindingCount(row) > 0)"
                :title="row.displayed && bindingCount(row) > 0 ? t('admin.customFeatures.upstream.bindings.unbindBeforeHide') : undefined"
                :data-test="`upstream-group-display-${row.remote_id}`"
                @click="setGroupDisplayed(row, !row.displayed)"
              >
                <Icon :name="isGroupDisplayLoading(row.remote_id) ? 'refresh' : row.displayed ? 'eyeOff' : 'plus'" size="xs" :class="{ 'animate-spin': isGroupDisplayLoading(row.remote_id) }" />
                {{ row.displayed ? t('admin.customFeatures.upstream.hideGroup') : t('admin.customFeatures.upstream.addGroupDisplay') }}
              </button>
            </template>
            <template #empty><p class="py-8 text-center text-sm text-gray-500">{{ t('admin.customFeatures.upstream.noGroups') }}</p></template>
          </DataTable>
        </template>
        <template v-else-if="detailTab === 'usage'">
          <div class="flex justify-end">
            <div class="inline-flex rounded-md border border-gray-200 p-1 dark:border-dark-600">
              <button v-for="days in historyRanges" :key="days" type="button" class="min-w-14 rounded px-3 py-1.5 text-sm" :class="historyDays === days ? 'bg-primary-600 text-white' : 'text-gray-600 dark:text-gray-300'" @click="changeHistoryRange(days)">{{ days }}d</button>
            </div>
          </div>
          <div v-if="detailHistory.length" class="h-72">
            <Line :data="historyChartData" :options="historyChartOptions" />
          </div>
          <p v-else class="py-16 text-center text-sm text-gray-500">{{ t('admin.customFeatures.upstream.noHistory') }}</p>
        </template>
        <template v-else>
          <div class="flex flex-col gap-3 sm:flex-row sm:items-end sm:justify-between">
            <div class="min-w-0 flex-1 sm:max-w-sm">
              <label for="upstream-multiplier-group" class="input-label">{{ t('admin.customFeatures.upstream.multiplierGroup') }}</label>
              <select id="upstream-multiplier-group" v-model="selectedMultiplierRemoteID" class="input" :disabled="multiplierLoading || multiplierHistories.length === 0">
                <option v-for="group in multiplierHistories" :key="group.remote_id" :value="group.remote_id">{{ group.name }}</option>
              </select>
            </div>
            <div class="inline-flex self-start rounded-md border border-gray-200 p-1 dark:border-dark-600 sm:self-auto">
              <button v-for="days in historyRanges" :key="days" type="button" class="min-w-14 rounded px-3 py-1.5 text-sm" :class="multiplierDays === days ? 'bg-primary-600 text-white' : 'text-gray-600 dark:text-gray-300'" @click="changeMultiplierRange(days)">{{ days }}d</button>
            </div>
          </div>
          <div v-if="multiplierLoading" class="flex h-56 items-center justify-center">
            <span class="h-8 w-8 animate-spin rounded-full border-b-2 border-primary-600"></span>
          </div>
          <div v-else-if="multiplierError" class="flex h-56 flex-col items-center justify-center gap-3 text-sm text-red-600 dark:text-red-300">
            <span>{{ multiplierError }}</span>
            <button type="button" class="btn btn-secondary inline-flex items-center gap-2" data-test="multiplier-history-retry" @click="loadMultiplierHistory(true)">
              <Icon name="refresh" size="sm" />
              {{ t('common.tryAgain') }}
            </button>
          </div>
          <template v-else-if="selectedMultiplierHistory">
            <div class="border-b border-gray-100 pb-3 dark:border-dark-700">
              <div class="flex flex-wrap items-center gap-2">
                <h4 class="break-words text-sm font-semibold text-gray-900 dark:text-gray-100">{{ selectedMultiplierHistory.name }}</h4>
                <span data-test="multiplier-history-platform" class="rounded px-2 py-1 text-xs font-medium" :class="groupPlatformClass(displayGroupPlatform(selectedMultiplierHistory))">{{ displayGroupPlatform(selectedMultiplierHistory) }}</span>
                <span class="text-sm font-medium text-primary-600 dark:text-primary-400">{{ formatMultiplier(selectedMultiplierHistory.current_multiplier) }}</span>
              </div>
              <p v-if="selectedMultiplierHistory.description" class="mt-2 break-words text-sm text-gray-500 dark:text-gray-400">{{ selectedMultiplierHistory.description }}</p>
            </div>
            <div v-if="hasMultiplierChartPoints" class="h-72" data-test="multiplier-history-chart">
              <Line :data="multiplierChartData" :options="multiplierChartOptions" />
            </div>
            <p v-else class="py-16 text-center text-sm text-gray-500">{{ t('admin.customFeatures.upstream.noMultiplierHistory') }}</p>
          </template>
          <p v-else class="py-16 text-center text-sm text-gray-500">{{ t('admin.customFeatures.upstream.noMultiplierHistory') }}</p>
        </template>
      </div>
    </BaseDialog>

    <UpstreamGroupBindingsDialog
      :show="Boolean(bindingTarget)"
      :site="bindingTarget?.site || null"
      :group="bindingTarget?.group || null"
      @close="closeBindings"
      @saved="handleBindingsSaved"
    />

    <BaseDialog :show="Boolean(deleteTarget)" :title="t('admin.customFeatures.upstream.deleteTitle')" width="narrow" @close="deleteTarget = null">
      <p class="text-sm text-gray-600 dark:text-gray-300">{{ t('admin.customFeatures.upstream.deleteMessage', { name: deleteTarget?.name || '' }) }}</p>
      <p v-if="(deleteTarget?.binding_count || 0) > 0" class="mt-3 rounded-md border border-amber-300 bg-amber-50 px-3 py-2 text-sm text-amber-900 dark:border-amber-800 dark:bg-amber-950/20 dark:text-amber-200" data-test="upstream-delete-binding-warning">
        {{ t('admin.customFeatures.upstream.bindings.deleteWarning', { count: deleteTarget?.binding_count || 0 }) }}
      </p>
      <template #footer>
        <button type="button" class="btn btn-secondary" @click="deleteTarget = null">{{ t('admin.customFeatures.upstream.cancel') }}</button>
        <button type="button" class="btn btn-danger" :disabled="deleting" @click="confirmDelete">{{ t('admin.customFeatures.upstream.delete') }}</button>
      </template>
    </BaseDialog>
  </section>
</template>

<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, reactive, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import {
  CategoryScale,
  Chart as ChartJS,
  Legend,
  LinearScale,
  LineElement,
  PointElement,
  Tooltip,
  type TooltipItem
} from 'chart.js'
import { Line } from 'vue-chartjs'
import { VueDraggable } from 'vue-draggable-plus'
import BaseDialog from '@/components/common/BaseDialog.vue'
import DataTable from '@/components/common/DataTable.vue'
import Toggle from '@/components/common/Toggle.vue'
import Icon from '@/components/icons/Icon.vue'
import UpstreamGroupBindingsDialog from '@/components/admin/upstream/UpstreamGroupBindingsDialog.vue'
import type { Column } from '@/components/common/types'
import upstreamsAPI, {
  type UpstreamCapabilities,
  type UpstreamDailyStat,
  type UpstreamGroup,
  type UpstreamGroupMultiplierHistory,
  type UpstreamGroupPlatform,
  type UpstreamPlatform,
  type UpstreamSite,
  type UpstreamWritePayload
} from '@/api/admin/upstreams'
import { useAppStore } from '@/stores/app'
import { extractApiErrorCode, extractApiErrorMessage } from '@/utils/apiError'
import { formatCompactNumber } from '@/utils/format'

ChartJS.register(CategoryScale, LinearScale, PointElement, LineElement, Tooltip, Legend)

const { t } = useI18n()
const appStore = useAppStore()
const sites = ref<UpstreamSite[]>([])
const loading = ref(true)
const loadError = ref('')
const page = ref(1)
const pageSize = 20
const total = ref(0)
const pages = ref(1)
const search = ref('')
const platformFilter = ref<UpstreamPlatform | ''>('')
const enabledFilter = ref('')
const groupPlatformFilter = ref<UpstreamGroupPlatform | ''>('')
const sortSelection = ref('')
const groupPlatforms: UpstreamGroupPlatform[] = ['OpenAI', 'Anthropic', 'Gemini', 'Grok', 'Antigravity', 'New API']
const syncingAll = ref(false)
const sortOpen = ref(false)
const sortLoading = ref(false)
const sortSubmitting = ref(false)
const sortableSites = ref<UpstreamSite[]>([])
const formOpen = ref(false)
const editingSite = ref<UpstreamSite | null>(null)
const saving = ref(false)
const capabilityProbeLoading = ref(false)
const capabilityProbeError = ref('')
const probedCapabilities = ref<UpstreamCapabilities | null>(null)
const probedBaseURL = ref('')
const loginResponseJSON = ref('')
const loginResponseError = ref('')
const deleteTarget = ref<UpstreamSite | null>(null)
const deleting = ref(false)
const bindingTarget = ref<{ site: UpstreamSite; group: UpstreamGroup } | null>(null)
const expandedSiteIDs = ref<Set<number>>(new Set())
const manuallyCollapsedSiteIDs = new Set<number>()
const groupDisplayLoadingIDs = ref<Set<string>>(new Set())
interface SiteGroupState {
  groups: UpstreamGroup[]
  loaded: boolean
  loading: boolean
  error: string
  syncedAt: string | null
  requestedSyncAt: string | null
  bindingCount: number
  requestedBindingCount: number
}
const siteGroupStates = reactive<Record<number, SiteGroupState>>({})
const groupRequestVersions = new Map<number, number>()
const detailSite = ref<UpstreamSite | null>(null)
const detailTab = ref<'groups' | 'usage' | 'multiplier'>('groups')
const detailTabs = ['groups', 'usage', 'multiplier'] as const
const detailGroups = ref<UpstreamGroup[]>([])
const detailHistory = ref<UpstreamDailyStat[]>([])
const detailLoading = ref(false)
const historyDays = ref<7 | 30 | 90>(30)
const historyRanges = [7, 30, 90] as const
const multiplierHistories = ref<UpstreamGroupMultiplierHistory[]>([])
const selectedMultiplierRemoteID = ref('')
const multiplierDays = ref<7 | 30 | 90>(30)
const multiplierLoading = ref(false)
const multiplierError = ref('')
const multiplierLoadedSiteID = ref<number | null>(null)
const multiplierLoadedDays = ref<7 | 30 | 90 | null>(null)
let refreshTimer: ReturnType<typeof setTimeout> | null = null
let disposed = false
let multiplierRequestVersion = 0
let detailRequestVersion = 0
let historyRequestVersion = 0
let siteListRequestVersion = 0
let capabilityProbeRequestVersion = 0
let bindingOpenRequestVersion = 0

const form = reactive<UpstreamWritePayload>({
  name: '', base_url: '', platform: 'sub2api', auth_mode: 'password', account: '',
  password: '', access_token: '', refresh_token: '', enabled: true
})

const turnstileDetected = computed(() => (
  form.platform === 'sub2api' && probedCapabilities.value?.turnstile_enabled === true
))
const upstreamLoginURL = computed(() => {
  try {
    const parsed = new URL(form.base_url.trim())
    if (parsed.protocol !== 'http:' && parsed.protocol !== 'https:') return ''
    parsed.pathname = `${parsed.pathname.replace(/\/$/, '')}/login`
    parsed.search = ''
    parsed.hash = ''
    return parsed.toString()
  } catch {
    return ''
  }
})

const columns = computed<Column[]>(() => [
  { key: 'site', label: t('admin.customFeatures.upstream.columns.site'), class: 'w-52 min-w-52 max-w-52' },
  { key: 'platform', label: t('admin.customFeatures.upstream.platform'), class: 'w-20 min-w-20 max-w-20' },
  { key: 'status', label: t('admin.customFeatures.upstream.columns.status'), class: 'w-28 min-w-28 max-w-28' },
  { key: 'balance_usd', label: t('admin.customFeatures.upstream.columns.balance'), class: 'w-24 min-w-24 max-w-24' },
  { key: 'today', label: t('admin.customFeatures.upstream.columns.today'), class: 'w-32 min-w-32 max-w-32 whitespace-normal' },
  { key: 'total', label: t('admin.customFeatures.upstream.columns.total'), class: 'w-32 min-w-32 max-w-32 whitespace-normal' },
  { key: 'last_synced_at', label: t('admin.customFeatures.upstream.columns.lastSync'), class: 'w-36 min-w-36 max-w-36 whitespace-normal' },
  { key: 'actions', label: t('admin.customFeatures.upstream.columns.actions'), class: 'w-32 min-w-32 max-w-32 text-right' }
])

const expandedSiteKeys = computed(() => Array.from(expandedSiteIDs.value))

const groupColumns = computed<Column[]>(() => [
  { key: 'name', label: t('admin.customFeatures.upstream.groupColumns.name') },
  { key: 'platform', label: t('admin.customFeatures.upstream.platform') },
  { key: 'status', label: t('admin.customFeatures.upstream.groupColumns.status') },
  { key: 'multiplier', label: t('admin.customFeatures.upstream.groupColumns.multiplier') },
  { key: 'today_tokens', label: t('admin.customFeatures.upstream.groupColumns.tokens') },
  { key: 'today_cost_usd', label: t('admin.customFeatures.upstream.groupColumns.cost') },
  { key: 'actions', label: t('admin.customFeatures.upstream.columns.actions'), class: 'min-w-28' }
])

const historyChartData = computed(() => ({
  labels: detailHistory.value.map((item) => item.date.slice(0, 10)),
  datasets: [
    { label: t('admin.customFeatures.upstream.chart.tokens'), data: detailHistory.value.map((item) => item.tokens), borderColor: '#2563eb', backgroundColor: '#2563eb20', tension: 0.25, yAxisID: 'tokens' },
    { label: t('admin.customFeatures.upstream.chart.cost'), data: detailHistory.value.map((item) => item.cost_usd), borderColor: '#dc2626', backgroundColor: '#dc262620', tension: 0.25, yAxisID: 'currency' },
    { label: t('admin.customFeatures.upstream.chart.balance'), data: detailHistory.value.map((item) => item.balance_usd), borderColor: '#059669', backgroundColor: '#05966920', tension: 0.25, yAxisID: 'currency' }
  ]
}))

const historyChartOptions = computed(() => ({
  responsive: true,
  maintainAspectRatio: false,
  interaction: { mode: 'index' as const, intersect: false },
  scales: {
    tokens: {
      type: 'linear' as const,
      position: 'left' as const,
      beginAtZero: true,
      ticks: { callback: (value: string | number) => formatCompactNumber(Number(value)) }
    },
    currency: { type: 'linear' as const, position: 'right' as const, beginAtZero: true, grid: { drawOnChartArea: false } }
  },
  plugins: { legend: { position: 'top' as const } }
}))

const selectedMultiplierHistory = computed(() => (
  multiplierHistories.value.find((item) => item.remote_id === selectedMultiplierRemoteID.value) || null
))
const hasMultiplierChartPoints = computed(() => (
  selectedMultiplierHistory.value?.points.some((point) => point.multiplier != null) === true
))

const multiplierChartData = computed(() => {
  const points = selectedMultiplierHistory.value?.points || []
  return {
    datasets: [{
      label: t('admin.customFeatures.upstream.chart.multiplier'),
      data: points.map((point) => ({ x: new Date(point.recorded_at).getTime(), y: point.multiplier })),
      borderColor: '#2563eb',
      backgroundColor: '#2563eb20',
      stepped: 'after' as const,
      tension: 0,
      spanGaps: false,
      pointRadius: 3
    }]
  }
})

const multiplierChartOptions = computed(() => ({
  responsive: true,
  maintainAspectRatio: false,
  interaction: { mode: 'index' as const, intersect: false },
  scales: {
    y: {
      type: 'linear' as const,
      beginAtZero: false,
      ticks: { callback: (value: string | number) => `${value}×` }
    },
    x: {
      type: 'linear' as const,
      ticks: {
        maxRotation: 0,
        callback: (value: string | number) => formatShortDateTime(new Date(Number(value)).toISOString())
      }
    }
  },
  plugins: {
    legend: { display: false },
    tooltip: {
      callbacks: {
        title: (items: TooltipItem<'line'>[]) => {
          const timestamp = items[0]?.parsed.x
          return timestamp == null ? '' : formatDateTime(new Date(timestamp).toISOString())
        },
        label: (item: TooltipItem<'line'>) => `${t('admin.customFeatures.upstream.chart.multiplier')}: ${item.parsed.y == null ? '—' : formatMultiplier(item.parsed.y)}`
      }
    }
  }
}))

function scheduleRefresh() {
  if (refreshTimer) clearTimeout(refreshTimer)
  if (disposed) return
  const busy = sites.value.some((site) => site.status === 'pending' || site.status === 'syncing')
  refreshTimer = setTimeout(() => loadSites(true), busy ? 2000 : 30000)
}

function groupState(siteID: number): SiteGroupState {
  if (!siteGroupStates[siteID]) {
    siteGroupStates[siteID] = {
      groups: [], loaded: false, loading: false, error: '', syncedAt: null, requestedSyncAt: null,
      bindingCount: 0, requestedBindingCount: 0
    }
  }
  return siteGroupStates[siteID]
}

function isSiteExpanded(siteID: number) {
  return expandedSiteIDs.value.has(siteID)
}

function displayedGroups(siteID: number) {
  return sortGroups(groupState(siteID).groups.filter((group) => (
    group.displayed && (!groupPlatformFilter.value || displayGroupPlatform(group) === groupPlatformFilter.value)
  )))
}

function toggleSiteGroups(site: UpstreamSite) {
  const next = new Set(expandedSiteIDs.value)
  if (next.has(site.id)) {
    next.delete(site.id)
    manuallyCollapsedSiteIDs.add(site.id)
  } else {
    next.add(site.id)
    manuallyCollapsedSiteIDs.delete(site.id)
    void loadSiteGroups(site)
  }
  expandedSiteIDs.value = next
}

async function loadSiteGroups(site: UpstreamSite, force = false) {
  const state = groupState(site.id)
  const syncVersion = site.last_synced_at
  const bindingVersion = site.binding_count || 0
  if (!force && state.loaded && state.syncedAt === syncVersion && state.bindingCount === bindingVersion && !state.error) return true
  if (!force && state.loading && state.requestedSyncAt === syncVersion && state.requestedBindingCount === bindingVersion) return false

  const requestVersion = (groupRequestVersions.get(site.id) || 0) + 1
  groupRequestVersions.set(site.id, requestVersion)
  state.loading = true
  state.error = ''
  state.requestedSyncAt = syncVersion
  state.requestedBindingCount = bindingVersion
  try {
    const groups = await upstreamsAPI.groups(site.id)
    if (groupRequestVersions.get(site.id) !== requestVersion) return false
    state.groups = groups || []
    state.loaded = true
    state.syncedAt = syncVersion
    state.bindingCount = bindingVersion
    const target = bindingTarget.value
    if (target?.site.id === site.id) {
      const latestGroup = state.groups.find((group) => group.id === target.group.id)
      if (!latestGroup || bindingGroupVersion(latestGroup) !== bindingGroupVersion(target.group)) {
        bindingOpenRequestVersion++
        bindingTarget.value = null
        appStore.showError(t('admin.customFeatures.upstream.bindings.dataChanged'))
      } else {
        bindingTarget.value = { site, group: latestGroup }
      }
    }
    return true
  } catch (error) {
    if (groupRequestVersions.get(site.id) !== requestVersion) return false
    state.error = extractApiErrorMessage(error, t('admin.customFeatures.upstream.groupsLoadFailed'))
    return false
  } finally {
    if (groupRequestVersions.get(site.id) === requestVersion) state.loading = false
  }
}

async function loadSites(silent = false) {
  const requestVersion = ++siteListRequestVersion
  if (!silent) loading.value = true
  try {
    const result = await upstreamsAPI.list({
      page: page.value, page_size: pageSize, search: search.value.trim(), platform: platformFilter.value,
      enabled: enabledFilter.value === '' ? undefined : enabledFilter.value === 'true',
      group_platform: groupPlatformFilter.value || undefined,
      ...parseSortSelection(sortSelection.value)
    })
    if (requestVersion !== siteListRequestVersion || disposed) return
    sites.value = result.items || []
    total.value = result.total
    pages.value = result.pages || 1
    loadError.value = ''
    const nextExpanded = new Set(expandedSiteIDs.value)
    for (const site of sites.value) {
      if (site.displayed_group_count <= 0) {
        nextExpanded.delete(site.id)
        manuallyCollapsedSiteIDs.delete(site.id)
      } else if (!manuallyCollapsedSiteIDs.has(site.id)) {
        nextExpanded.add(site.id)
      }
    }
    expandedSiteIDs.value = nextExpanded
    for (const site of sites.value) {
      const state = groupState(site.id)
      if (isSiteExpanded(site.id) && (
        !state.loaded
        || state.syncedAt !== site.last_synced_at
        || state.bindingCount !== (site.binding_count || 0)
      )) {
        void loadSiteGroups(site)
      }
    }
  } catch (error) {
    if (requestVersion !== siteListRequestVersion || disposed) return
    if (!silent) loadError.value = extractApiErrorMessage(error, t('admin.customFeatures.upstream.loadFailed'))
  } finally {
    if (requestVersion === siteListRequestVersion) {
      loading.value = false
      scheduleRefresh()
    }
  }
}

function applyFilters() { page.value = 1; loadSites(false) }
function applySortSelection() { page.value = 1; loadSites(false) }
function changePage(nextPage: number) { page.value = nextPage; loadSites(false) }

function parseSortSelection(value: string): { sort_by?: 'balance_usd' | 'today_tokens'; sort_order?: 'asc' | 'desc' } {
  if (!value) return {}
  const [sortBy, sortOrder] = value.split('_').reduce((parts, part, index, values) => {
    if (index < values.length - 1) parts[0] += `${parts[0] ? '_' : ''}${part}`
    else parts[1] = part
    return parts
  }, ['', ''] as [string, string])
  if ((sortBy === 'balance' || sortBy === 'today_tokens') && (sortOrder === 'asc' || sortOrder === 'desc')) {
    return { sort_by: sortBy === 'balance' ? 'balance_usd' : 'today_tokens', sort_order: sortOrder }
  }
  return {}
}

async function openSortModal() {
  sortLoading.value = true
  sortOpen.value = true
  try {
    sortableSites.value = await upstreamsAPI.listAll()
  } catch (error) {
    sortOpen.value = false
    appStore.showError(extractApiErrorMessage(error, t('admin.customFeatures.upstream.sortOrderLoadFailed')))
  } finally {
    sortLoading.value = false
  }
}

function closeSortModal() {
  if (!sortSubmitting.value) sortOpen.value = false
}

async function saveSortOrder() {
  if (sortableSites.value.length === 0) return
  sortSubmitting.value = true
  try {
    await upstreamsAPI.updateSortOrder(sortableSites.value.map((site, index) => ({ id: site.id, sort_order: index * 10 })))
    appStore.showSuccess(t('admin.customFeatures.upstream.sortOrderUpdated'))
    sortOpen.value = false
    await loadSites(true)
  } catch (error) {
    appStore.showError(extractApiErrorMessage(error, t('admin.customFeatures.upstream.sortOrderSaveFailed')))
  } finally {
    sortSubmitting.value = false
  }
}

function resetForm() {
  resetCapabilityProbe()
  Object.assign(form, { name: '', base_url: '', platform: 'sub2api', auth_mode: 'password', account: '', password: '', access_token: '', refresh_token: '', enabled: true })
}

function openCreate() { editingSite.value = null; resetForm(); formOpen.value = true }
function openEdit(site: UpstreamSite) {
  resetCapabilityProbe()
  editingSite.value = site
  Object.assign(form, { name: site.name, base_url: site.base_url, platform: site.platform, auth_mode: site.auth_mode, account: site.account, password: '', access_token: '', refresh_token: '', enabled: site.enabled })
  formOpen.value = true
  void probeSiteCapabilities()
}
function closeForm() {
  if (saving.value) return
  formOpen.value = false
  resetCapabilityProbe()
}
function handlePlatformChange() {
  resetCapabilityProbe()
  if (form.platform === 'newapi') {
    form.auth_mode = 'password'
    return
  }
  void probeSiteCapabilities()
}
function handleAuthModeChange() {
  if (turnstileDetected.value && form.auth_mode === 'password') form.auth_mode = 'token'
  loginResponseError.value = ''
}
function handleBaseURLInput() {
  const current = normalizeProbeURL(form.base_url)
  if (!probedBaseURL.value || current === probedBaseURL.value) return
  resetCapabilityProbe()
}

function normalizeProbeURL(value: string) {
  try {
    const parsed = new URL(value.trim())
    if (parsed.protocol !== 'http:' && parsed.protocol !== 'https:') return ''
    parsed.search = ''
    parsed.hash = ''
    return parsed.toString().replace(/\/$/, '')
  } catch {
    return ''
  }
}

function resetCapabilityProbe() {
  capabilityProbeRequestVersion += 1
  capabilityProbeLoading.value = false
  capabilityProbeError.value = ''
  probedCapabilities.value = null
  probedBaseURL.value = ''
  loginResponseJSON.value = ''
  loginResponseError.value = ''
}

function markTurnstileDetected(baseURL: string) {
  const normalized = normalizeProbeURL(baseURL)
  probedBaseURL.value = normalized
  probedCapabilities.value = {
    base_url: normalized,
    platform: 'sub2api',
    turnstile_enabled: true,
    token_auth_recommended: true,
  }
  form.auth_mode = 'token'
}

async function probeSiteCapabilities(force = false): Promise<UpstreamCapabilities | null> {
  if (form.platform !== 'sub2api') return null
  const baseURL = normalizeProbeURL(form.base_url)
  if (!baseURL) return null
  if (!force && probedCapabilities.value && probedBaseURL.value === baseURL) return probedCapabilities.value

  const requestVersion = ++capabilityProbeRequestVersion
  capabilityProbeLoading.value = true
  capabilityProbeError.value = ''
  try {
    const capabilities = await upstreamsAPI.probeCapabilities({ base_url: baseURL, platform: form.platform })
    if (requestVersion !== capabilityProbeRequestVersion || normalizeProbeURL(form.base_url) !== baseURL || form.platform !== 'sub2api') return null
    probedCapabilities.value = capabilities
    probedBaseURL.value = normalizeProbeURL(capabilities.base_url)
    if (capabilities.turnstile_enabled) form.auth_mode = 'token'
    return capabilities
  } catch (error) {
    if (requestVersion === capabilityProbeRequestVersion) {
      capabilityProbeError.value = extractApiErrorMessage(error, t('admin.customFeatures.upstream.capabilityProbeFailed'))
    }
    return null
  } finally {
    if (requestVersion === capabilityProbeRequestVersion) capabilityProbeLoading.value = false
  }
}

function asTokenRecord(value: unknown): Record<string, unknown> | null {
  return value && typeof value === 'object' && !Array.isArray(value) ? value as Record<string, unknown> : null
}

function firstTokenValue(records: Array<Record<string, unknown> | null>, keys: string[]) {
  for (const record of records) {
    if (!record) continue
    for (const key of keys) {
      const value = record[key]
      if (typeof value === 'string' && value.trim()) return value.trim()
    }
  }
  return ''
}

function importLoginResponseTokens() {
  loginResponseError.value = ''
  let parsed: unknown
  try {
    parsed = JSON.parse(loginResponseJSON.value)
  } catch {
    loginResponseError.value = t('admin.customFeatures.upstream.loginResponseInvalid')
    return
  }
  const root = asTokenRecord(parsed)
  const data = asTokenRecord(root?.data)
  const nestedToken = asTokenRecord(data?.token) || asTokenRecord(root?.token)
  const records = [data, root, nestedToken]
  const accessToken = firstTokenValue(records, ['access_token', 'accessToken', 'token'])
  const refreshToken = firstTokenValue(records, ['refresh_token', 'refreshToken'])
  if (!accessToken && !refreshToken) {
    loginResponseError.value = t('admin.customFeatures.upstream.loginResponseMissingTokens')
    return
  }
  if (accessToken) form.access_token = accessToken
  if (refreshToken) form.refresh_token = refreshToken
  loginResponseJSON.value = ''
  appStore.showSuccess(t('admin.customFeatures.upstream.tokensImported'))
}

async function submitForm() {
  if (form.platform === 'sub2api' && form.auth_mode === 'password') {
    const capabilities = await probeSiteCapabilities(true)
    if (capabilities?.turnstile_enabled) {
      appStore.showError(t('admin.customFeatures.upstream.turnstileRequiresToken'))
      return
    }
  }
  const keepsExistingToken = editingSite.value?.auth_mode === 'token' && editingSite.value.has_token
  if (form.auth_mode === 'token' && !keepsExistingToken && !form.access_token?.trim() && !form.refresh_token?.trim()) {
    appStore.showError(t('admin.customFeatures.upstream.tokenRequired'))
    return
  }
  saving.value = true
  try {
    const payload: UpstreamWritePayload = {
      ...form,
      name: form.name.trim(),
      base_url: form.base_url.trim(),
      account: form.auth_mode === 'password' ? form.account.trim() : '',
      password: form.auth_mode === 'password' ? form.password?.trim() : undefined,
      access_token: form.auth_mode === 'token' ? form.access_token?.trim() : undefined,
      refresh_token: form.auth_mode === 'token' ? form.refresh_token?.trim() : undefined,
    }
    if (editingSite.value) await upstreamsAPI.update(editingSite.value.id, payload)
    else await upstreamsAPI.create(payload)
    appStore.showSuccess(t('admin.customFeatures.upstream.saved'))
    formOpen.value = false
    await loadSites(true)
  } catch (error) {
    if (extractApiErrorCode(error) === 'UPSTREAM_TURNSTILE_REQUIRED') {
      markTurnstileDetected(form.base_url)
      appStore.showError(t('admin.customFeatures.upstream.turnstileRequiresToken'))
      return
    }
    appStore.showError(extractApiErrorMessage(error, t('admin.customFeatures.upstream.saveFailed')))
  } finally { saving.value = false }
}

async function syncSite(site: UpstreamSite) {
  try { await upstreamsAPI.sync(site.id); site.status = 'pending'; scheduleRefresh() }
  catch (error) { appStore.showError(extractApiErrorMessage(error, t('admin.customFeatures.upstream.syncFailed'))) }
}

async function syncAllSites() {
  syncingAll.value = true
  try { await upstreamsAPI.syncAll(); sites.value.forEach((site) => { site.status = 'pending' }); scheduleRefresh() }
  catch (error) { appStore.showError(extractApiErrorMessage(error, t('admin.customFeatures.upstream.syncFailed'))) }
  finally { syncingAll.value = false }
}

async function toggleSite(site: UpstreamSite) {
  try {
    const displayedGroupCount = site.displayed_group_count
    const bindingCount = site.binding_count
    const updated = await upstreamsAPI.setEnabled(site.id, !site.enabled)
    Object.assign(site, updated, { displayed_group_count: displayedGroupCount, binding_count: bindingCount })
    scheduleRefresh()
  }
  catch (error) { appStore.showError(extractApiErrorMessage(error, t('admin.customFeatures.upstream.saveFailed'))) }
}

async function confirmDelete() {
  if (!deleteTarget.value) return
  const deletedID = deleteTarget.value.id
  deleting.value = true
  try {
    await upstreamsAPI.remove(deletedID)
    deleteTarget.value = null
    const next = new Set(expandedSiteIDs.value)
    next.delete(deletedID)
    expandedSiteIDs.value = next
    delete siteGroupStates[deletedID]
    groupRequestVersions.delete(deletedID)
    await loadSites(true)
    appStore.showSuccess(t('admin.customFeatures.upstream.deleted'))
  }
  catch (error) { appStore.showError(extractApiErrorMessage(error, t('admin.customFeatures.upstream.deleteFailed'))) }
  finally { deleting.value = false }
}

async function openDetails(site: UpstreamSite) {
  const requestVersion = ++detailRequestVersion
  historyRequestVersion++
  detailSite.value = site
  detailTab.value = 'groups'
  detailLoading.value = true
  detailGroups.value = []
  detailHistory.value = []
  multiplierHistories.value = []
  selectedMultiplierRemoteID.value = ''
  multiplierError.value = ''
  multiplierLoadedSiteID.value = null
  multiplierLoadedDays.value = null
  multiplierRequestVersion++
  try {
    const [groups, history] = await Promise.all([upstreamsAPI.groups(site.id), upstreamsAPI.history(site.id, historyDays.value)])
    if (requestVersion !== detailRequestVersion || detailSite.value?.id !== site.id) return
    detailGroups.value = groups || []
    detailHistory.value = history || []
    const state = groupState(site.id)
    state.groups = detailGroups.value
    state.loaded = true
    state.syncedAt = site.last_synced_at
    state.bindingCount = site.binding_count || 0
    state.error = ''
  }
  catch (error) {
    if (requestVersion === detailRequestVersion) {
      appStore.showError(extractApiErrorMessage(error, t('admin.customFeatures.upstream.detailFailed')))
    }
  }
  finally {
    if (requestVersion === detailRequestVersion) detailLoading.value = false
  }
}

async function openMultiplierTrend(site: UpstreamSite, group: UpstreamGroup) {
  await openDetails(site)
  if (detailSite.value?.id !== site.id) return
  detailTab.value = 'multiplier'
  selectedMultiplierRemoteID.value = group.remote_id
  await loadMultiplierHistory()
}

function groupDisplayLoadingKey(siteID: number, remoteID: string) {
  return `${siteID}:${remoteID}`
}

function isGroupDisplayLoading(remoteID: string) {
  return detailSite.value ? groupDisplayLoadingIDs.value.has(groupDisplayLoadingKey(detailSite.value.id, remoteID)) : false
}

function updateGroupCollection(groups: UpstreamGroup[], updated: UpstreamGroup) {
  const index = groups.findIndex((group) => group.remote_id === updated.remote_id)
  if (!updated.available && !updated.displayed) {
    if (index >= 0) groups.splice(index, 1)
    return
  }
  if (index >= 0) groups.splice(index, 1, updated)
  else groups.push(updated)
}

async function setGroupDisplayed(group: UpstreamGroup, displayed: boolean) {
  const site = detailSite.value
  if (!site) return
  if (!displayed && bindingCount(group) > 0) {
    appStore.showError(t('admin.customFeatures.upstream.bindings.unbindBeforeHide'))
    return
  }
  const key = groupDisplayLoadingKey(site.id, group.remote_id)
  if (groupDisplayLoadingIDs.value.has(key)) return
  groupDisplayLoadingIDs.value = new Set(groupDisplayLoadingIDs.value).add(key)
  try {
    const result = await upstreamsAPI.setGroupDisplayed(site.id, group.remote_id, displayed)
    updateGroupCollection(detailGroups.value, result.group)
    const state = groupState(site.id)
    updateGroupCollection(state.groups, result.group)
    state.loaded = true
    state.error = ''
    site.displayed_group_count = result.displayed_group_count
    const listSite = sites.value.find((item) => item.id === site.id)
    if (listSite) listSite.displayed_group_count = result.displayed_group_count

    const nextExpanded = new Set(expandedSiteIDs.value)
    if (displayed) {
      manuallyCollapsedSiteIDs.delete(site.id)
      nextExpanded.add(site.id)
    } else if (result.displayed_group_count === 0) {
      manuallyCollapsedSiteIDs.delete(site.id)
      nextExpanded.delete(site.id)
    }
    expandedSiteIDs.value = nextExpanded
    appStore.showSuccess(t(displayed ? 'admin.customFeatures.upstream.groupDisplayed' : 'admin.customFeatures.upstream.groupHidden'))
  } catch (error) {
    appStore.showError(extractApiErrorMessage(error, t('admin.customFeatures.upstream.groupDisplayFailed')))
  } finally {
    const next = new Set(groupDisplayLoadingIDs.value)
    next.delete(key)
    groupDisplayLoadingIDs.value = next
  }
}

function bindingCount(group: Pick<UpstreamGroup, 'bindings'>) {
  return group.bindings?.length || 0
}

function bindingGroupVersion(group: UpstreamGroup) {
  const bindings = (group.bindings || [])
    .map((binding) => `${binding.local_group_id}:${binding.account_id}`)
    .sort()
    .join('|')
  return `${group.displayed}:${group.available}:${group.multiplier ?? 'null'}:${bindings}`
}

async function openBindings(site: UpstreamSite, group: UpstreamGroup) {
  const openVersion = ++bindingOpenRequestVersion
  const loaded = await loadSiteGroups(site, true)
  if (!loaded || openVersion !== bindingOpenRequestVersion) return
  const state = groupState(site.id)
  if (state.error) {
    appStore.showError(state.error)
    return
  }
  const latestGroup = state.groups.find((item) => item.id === group.id)
  if (!latestGroup) {
    appStore.showError(t('admin.customFeatures.upstream.groupsLoadFailed'))
    return
  }
  bindingTarget.value = { site, group: latestGroup }
}

function closeBindings() {
  bindingOpenRequestVersion++
  bindingTarget.value = null
}

function handleBindingsSaved(updated: UpstreamGroup) {
  const target = bindingTarget.value
  if (!target) return
  const previousCount = bindingCount(target.group)
  const nextCount = bindingCount(updated)
  updateGroupCollection(groupState(target.site.id).groups, updated)
  if (detailSite.value?.id === target.site.id) updateGroupCollection(detailGroups.value, updated)

  const site = sites.value.find((item) => item.id === target.site.id)
  if (site) site.binding_count = Math.max(0, (site.binding_count || 0) + nextCount - previousCount)
  if (detailSite.value?.id === target.site.id) detailSite.value.binding_count = site?.binding_count ?? target.site.binding_count
  groupState(target.site.id).bindingCount = site?.binding_count ?? Math.max(0, (target.site.binding_count || 0) + nextCount - previousCount)
  closeBindings()
}

function closeDetails() {
  detailRequestVersion++
  historyRequestVersion++
  multiplierRequestVersion++
  detailSite.value = null
  detailGroups.value = []
  detailHistory.value = []
  multiplierHistories.value = []
  selectedMultiplierRemoteID.value = ''
  multiplierError.value = ''
  multiplierLoadedSiteID.value = null
  multiplierLoadedDays.value = null
  multiplierLoading.value = false
  detailLoading.value = false
  groupDisplayLoadingIDs.value = new Set()
}

function selectDetailTab(tab: typeof detailTabs[number]) {
  detailTab.value = tab
  if (tab === 'multiplier') void loadMultiplierHistory()
}

async function changeHistoryRange(days: 7 | 30 | 90) {
  if (!detailSite.value || days === historyDays.value) return
  const siteID = detailSite.value.id
  const requestVersion = ++historyRequestVersion
  historyDays.value = days
  detailLoading.value = true
  try {
    const history = await upstreamsAPI.history(siteID, days)
    if (requestVersion === historyRequestVersion && detailSite.value?.id === siteID) detailHistory.value = history
  }
  catch (error) {
    if (requestVersion === historyRequestVersion) {
      appStore.showError(extractApiErrorMessage(error, t('admin.customFeatures.upstream.detailFailed')))
    }
  }
  finally {
    if (requestVersion === historyRequestVersion) detailLoading.value = false
  }
}

async function loadMultiplierHistory(force = false) {
  const site = detailSite.value
  if (!site) return
  if (!force && multiplierLoadedSiteID.value === site.id && multiplierLoadedDays.value === multiplierDays.value) return
  const requestVersion = ++multiplierRequestVersion
  multiplierLoading.value = true
  multiplierError.value = ''
  try {
    const histories = await upstreamsAPI.multiplierHistory(site.id, multiplierDays.value)
    if (requestVersion !== multiplierRequestVersion || detailSite.value?.id !== site.id) return
    multiplierHistories.value = histories || []
    if (!multiplierHistories.value.some((item) => item.remote_id === selectedMultiplierRemoteID.value)) {
      selectedMultiplierRemoteID.value = multiplierHistories.value[0]?.remote_id || ''
    }
    multiplierLoadedSiteID.value = site.id
    multiplierLoadedDays.value = multiplierDays.value
  } catch (error) {
    if (requestVersion !== multiplierRequestVersion) return
    multiplierError.value = extractApiErrorMessage(error, t('admin.customFeatures.upstream.multiplierHistoryFailed'))
  } finally {
    if (requestVersion === multiplierRequestVersion) multiplierLoading.value = false
  }
}

function changeMultiplierRange(days: 7 | 30 | 90) {
  if (days === multiplierDays.value) return
  multiplierDays.value = days
  void loadMultiplierHistory(true)
}

function platformLabel(platform: UpstreamPlatform) { return platform === 'newapi' ? 'New API' : 'Sub2API' }
function sitePlatformClass(platform: UpstreamPlatform) {
  return platform === 'newapi'
    ? 'bg-cyan-100 text-cyan-800 dark:bg-cyan-900/30 dark:text-cyan-300'
    : 'bg-indigo-100 text-indigo-800 dark:bg-indigo-900/30 dark:text-indigo-300'
}
function groupPlatformClass(platform: string) {
  const normalized = platform.trim().toLowerCase()
  if (normalized.includes('openai')) return 'bg-emerald-100 text-emerald-800 dark:bg-emerald-900/30 dark:text-emerald-300'
  if (normalized.includes('anthropic') || normalized.includes('claude')) return 'bg-orange-100 text-orange-800 dark:bg-orange-900/30 dark:text-orange-300'
  if (normalized.includes('antigravity')) return 'bg-purple-100 text-purple-800 dark:bg-purple-900/30 dark:text-purple-300'
  if (normalized.includes('gemini') || normalized.includes('google')) return 'bg-blue-100 text-blue-800 dark:bg-blue-900/30 dark:text-blue-300'
  return 'bg-gray-100 text-gray-700 dark:bg-dark-700 dark:text-gray-300'
}
function displayGroupPlatform(group: Pick<UpstreamGroup, 'name' | 'description' | 'platform'>) {
  const platform = group.platform.trim()
  const canonical = canonicalGroupPlatform(platform)
  if (canonical) return canonical
  const text = `${group.name} ${group.description}`.toLowerCase()
  if (text.includes('claude') || text.includes('anthropic')) return 'Anthropic'
  if (text.includes('kiro') || text.includes('sonnet') || text.includes('opus') || text.includes('haiku')) return 'Anthropic'
  if (text.includes('gpt') || text.includes('openai')) return 'OpenAI'
  if (text.includes('antigravity')) return 'Antigravity'
  if (text.includes('gemini') || text.includes('google ai')) return 'Gemini'
  if (text.includes('grok') || text.includes('x.ai')) return 'Grok'
  return platform || '—'
}
function canonicalGroupPlatform(platform: string) {
  const normalized = platform.toLowerCase()
  if (!normalized || normalized.includes('newapi') || normalized.includes('new api')) return ''
  if (normalized.includes('antigravity')) return 'Antigravity'
  if (normalized === 'openai' || normalized === 'open ai') return 'OpenAI'
  if (normalized === 'anthropic' || normalized === 'claude') return 'Anthropic'
  if (normalized === 'gemini' || normalized === 'google' || normalized === 'google ai') return 'Gemini'
  if (normalized === 'grok' || normalized === 'xai' || normalized === 'x.ai') return 'Grok'
  return platform
}
function groupPlatformRank(platform: string) {
  return ({ openai: 0, anthropic: 1, gemini: 2, grok: 3, antigravity: 4 } as Record<string, number>)[platform.toLowerCase()] ?? 5
}
function sortGroups(groups: UpstreamGroup[]) {
  return [...groups].sort((left, right) => {
    const platformOrder = groupPlatformRank(displayGroupPlatform(left)) - groupPlatformRank(displayGroupPlatform(right))
    if (platformOrder !== 0) return platformOrder
    const leftMultiplier = left.multiplier ?? Number.POSITIVE_INFINITY
    const rightMultiplier = right.multiplier ?? Number.POSITIVE_INFINITY
    if (leftMultiplier !== rightMultiplier) return leftMultiplier - rightMultiplier
    return left.name.localeCompare(right.name, 'zh-CN')
  })
}
function formatMoney(value: number | null | undefined) { return value == null ? '—' : `$${value.toLocaleString(undefined, { minimumFractionDigits: 2, maximumFractionDigits: 4 })}` }
function formatTokens(value: number) { return formatCompactNumber(Number(value || 0)) }
function formatExactTokens(value: number) { return Number(value || 0).toLocaleString('en-US') }
function formatMultiplier(value: number | null | undefined) {
  return value == null ? '—' : `${value.toLocaleString(undefined, { maximumFractionDigits: 6 })}×`
}
function formatDateTime(value: string | null) { return value ? new Date(value).toLocaleString() : '—' }
function formatShortDateTime(value: string | null | undefined) {
  if (!value) return ''
  const date = new Date(value)
  if (Number.isNaN(date.getTime())) return value
  const month = String(date.getMonth() + 1).padStart(2, '0')
  const day = String(date.getDate()).padStart(2, '0')
  const hour = String(date.getHours()).padStart(2, '0')
  const minute = String(date.getMinutes()).padStart(2, '0')
  return `${month}-${day} ${hour}:${minute}`
}
function statusClass(status: UpstreamSite['status']) {
  return {
    pending: 'bg-amber-50 text-amber-700 dark:bg-amber-900/20 dark:text-amber-300',
    syncing: 'bg-blue-50 text-blue-700 dark:bg-blue-900/20 dark:text-blue-300',
    healthy: 'bg-emerald-50 text-emerald-700 dark:bg-emerald-900/20 dark:text-emerald-300',
    error: 'bg-red-50 text-red-700 dark:bg-red-900/20 dark:text-red-300'
  }[status]
}

onMounted(() => loadSites(false))
onBeforeUnmount(() => { disposed = true; bindingOpenRequestVersion++; if (refreshTimer) clearTimeout(refreshTimer) })
</script>

<style scoped>
.icon-action {
  @apply inline-flex h-8 w-8 items-center justify-center rounded-md text-gray-500 transition-colors hover:bg-gray-100 hover:text-gray-800 disabled:cursor-not-allowed disabled:opacity-50 dark:text-gray-400 dark:hover:bg-dark-700 dark:hover:text-gray-100;
}
</style>

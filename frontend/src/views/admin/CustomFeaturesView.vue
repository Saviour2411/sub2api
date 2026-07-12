<template>
  <AppLayout>
    <div class="mx-auto max-w-6xl space-y-6">
      <header>
        <h1 class="text-2xl font-semibold text-gray-900 dark:text-white">
          {{ t('admin.customFeatures.title') }}
        </h1>
        <p class="mt-1 text-sm text-gray-500 dark:text-gray-400">
          {{ t('admin.customFeatures.description') }}
        </p>
      </header>

      <div class="border-b border-gray-200 dark:border-dark-700">
        <nav class="flex gap-6 overflow-x-auto" role="tablist" :aria-label="t('admin.customFeatures.title')">
          <button
            v-for="tab in tabs"
            :key="tab.key"
            type="button"
            role="tab"
            :data-test="`custom-feature-tab-${tab.key}`"
            :aria-selected="activeTab === tab.key"
            class="inline-flex min-h-11 flex-shrink-0 items-center gap-2 border-b-2 px-1 text-sm font-medium transition-colors"
            :class="
              activeTab === tab.key
                ? 'border-primary-500 text-primary-600 dark:text-primary-400'
                : 'border-transparent text-gray-500 hover:text-gray-800 dark:text-gray-400 dark:hover:text-gray-200'
            "
            @click="activeTab = tab.key"
          >
            <Icon :name="tab.icon" size="sm" />
            {{ t(tab.labelKey) }}
          </button>
        </nav>
      </div>

      <div v-if="loading" class="flex items-center justify-center py-16">
        <div class="h-8 w-8 animate-spin rounded-full border-b-2 border-primary-600"></div>
      </div>

      <div v-else-if="loadFailed" class="card p-8 text-center">
        <p class="text-sm text-gray-600 dark:text-gray-300">
          {{ t('admin.customFeatures.loadFailed') }}
        </p>
        <button type="button" class="btn btn-secondary mt-4 inline-flex items-center gap-2" @click="loadSettings">
          <Icon name="refresh" size="sm" />
          {{ t('common.tryAgain') }}
        </button>
      </div>

      <form
        v-else-if="activeTab === 'model-marketplace'"
        data-test="model-marketplace-form"
        class="card overflow-hidden"
        @submit.prevent="saveModelMarketplace"
      >
        <div class="flex flex-col gap-4 border-b border-gray-100 px-5 py-5 dark:border-dark-700 sm:flex-row sm:items-center sm:justify-between sm:px-6">
          <div>
            <h2 class="text-lg font-semibold text-gray-900 dark:text-white">
              {{ t('admin.customFeatures.modelMarketplace.title') }}
            </h2>
            <p class="mt-1 text-sm text-gray-500 dark:text-gray-400">
              {{ t('admin.customFeatures.modelMarketplace.description') }}
            </p>
          </div>
          <div class="flex items-center gap-3">
            <span class="text-sm font-medium text-gray-700 dark:text-gray-300">
              {{ t('admin.customFeatures.enabled') }}
            </span>
            <Toggle v-model="modelMarketplace.enabled" data-test="model-marketplace-enabled" />
          </div>
        </div>

        <div class="space-y-6 px-5 py-6 sm:px-6">
          <div>
            <label class="input-label" for="model-marketplace-intro">
              {{ t('admin.customFeatures.modelMarketplace.intro') }}
            </label>
            <textarea
              id="model-marketplace-intro"
              v-model="modelMarketplace.intro"
              data-test="model-marketplace-intro"
              rows="4"
              class="input text-sm"
              :placeholder="t('admin.customFeatures.modelMarketplace.introPlaceholder')"
            ></textarea>
            <p class="mt-1 text-xs text-gray-500 dark:text-gray-400">
              {{ t('admin.customFeatures.modelMarketplace.introHint') }}
            </p>
          </div>

          <div>
            <div class="flex flex-col gap-1 sm:flex-row sm:items-end sm:justify-between">
              <label class="input-label mb-0">
                {{ t('admin.customFeatures.modelMarketplace.groups') }}
              </label>
              <span class="text-xs text-gray-500 dark:text-gray-400">
                {{ t('admin.customFeatures.modelMarketplace.selectedCount', { count: modelMarketplace.group_ids.length }) }}
              </span>
            </div>
            <div
              v-if="activeGroups.length > 0"
              class="mt-2 grid max-h-96 grid-cols-1 overflow-y-auto rounded-md border border-gray-200 dark:border-dark-600 lg:grid-cols-2"
            >
              <label
                v-for="group in activeGroups"
                :key="group.id"
                class="flex min-w-0 cursor-pointer items-start gap-3 border-b border-gray-100 px-4 py-3 last:border-b-0 hover:bg-gray-50 dark:border-dark-700 dark:hover:bg-dark-800 lg:[&:nth-last-child(2)]:border-b-0"
              >
                <input
                  v-model="modelMarketplace.group_ids"
                  type="checkbox"
                  :value="group.id"
                  class="mt-1 h-4 w-4 flex-shrink-0 rounded border-gray-300 text-primary-600 focus:ring-primary-500"
                />
                <span class="min-w-0">
                  <span class="flex flex-wrap items-center gap-2">
                    <span class="font-medium text-gray-800 dark:text-gray-200">{{ group.name }}</span>
                    <span class="text-xs text-gray-500 dark:text-gray-400">{{ group.platform }}</span>
                    <span v-if="group.is_exclusive" class="rounded bg-amber-50 px-1.5 py-0.5 text-xs text-amber-700 dark:bg-amber-900/20 dark:text-amber-300">
                      {{ t('admin.customFeatures.modelMarketplace.exclusive') }}
                    </span>
                    <span v-if="group.subscription_type === 'subscription'" class="rounded bg-emerald-50 px-1.5 py-0.5 text-xs text-emerald-700 dark:bg-emerald-900/20 dark:text-emerald-300">
                      {{ t('admin.customFeatures.modelMarketplace.subscription') }}
                    </span>
                  </span>
                  <span v-if="group.description" class="mt-1 block line-clamp-2 text-xs text-gray-500 dark:text-gray-400">
                    {{ group.description }}
                  </span>
                </span>
              </label>
            </div>
            <div v-else class="mt-2 rounded-md border border-dashed border-gray-300 px-4 py-8 text-center text-sm text-gray-500 dark:border-dark-600 dark:text-gray-400">
              {{ t('admin.customFeatures.modelMarketplace.noGroups') }}
            </div>
            <p class="mt-2 text-xs text-gray-500 dark:text-gray-400">
              {{ t('admin.customFeatures.modelMarketplace.groupsHint') }}
            </p>
          </div>
        </div>

        <div class="flex justify-end border-t border-gray-100 px-5 py-4 dark:border-dark-700 sm:px-6">
          <button type="submit" class="btn btn-primary inline-flex items-center gap-2" :disabled="savingMarketplace">
            <span v-if="savingMarketplace" class="h-4 w-4 animate-spin rounded-full border-b-2 border-white"></span>
            <Icon v-else name="check" size="sm" />
            {{ t('admin.customFeatures.save') }}
          </button>
        </div>
      </form>

      <form
        v-else-if="activeTab === 'gateway'"
        data-test="gateway-form"
        class="card overflow-hidden"
        @submit.prevent="saveGateway"
      >
        <div class="border-b border-gray-100 px-5 py-5 dark:border-dark-700 sm:px-6">
          <h2 class="text-lg font-semibold text-gray-900 dark:text-white">
            {{ t('admin.customFeatures.gateway.title') }}
          </h2>
          <p class="mt-1 text-sm text-gray-500 dark:text-gray-400">
            {{ t('admin.customFeatures.gateway.description') }}
          </p>
        </div>

        <div class="space-y-8 px-5 py-6 sm:px-6">
          <section aria-labelledby="gateway-pool-title">
            <h3 id="gateway-pool-title" class="font-semibold text-gray-900 dark:text-white">
              {{ t('admin.customFeatures.gateway.poolDefaults.title') }}
            </h3>
            <p class="mt-1 text-sm text-gray-500 dark:text-gray-400">
              {{ t('admin.customFeatures.gateway.poolDefaults.description') }}
            </p>
            <div class="mt-4 grid gap-5 md:grid-cols-2">
              <div>
                <label class="input-label" for="gateway-pool-retry-count">
                  {{ t('admin.customFeatures.gateway.poolDefaults.retryCount') }}
                </label>
                <input
                  id="gateway-pool-retry-count"
                  v-model.number="gateway.default_pool_mode_retry_count"
                  data-test="gateway-pool-retry-count"
                  type="number"
                  min="0"
                  max="10"
                  step="1"
                  class="input"
                />
                <p class="mt-1 text-xs text-gray-500 dark:text-gray-400">
                  {{ t('admin.customFeatures.gateway.poolDefaults.retryCountHint') }}
                </p>
              </div>
              <div>
                <label class="input-label" for="gateway-pool-retry-status-codes">
                  {{ t('admin.customFeatures.gateway.poolDefaults.retryStatusCodes') }}
                </label>
                <input
                  id="gateway-pool-retry-status-codes"
                  v-model="gatewayRetryStatusCodesInput"
                  data-test="gateway-pool-retry-status-codes"
                  type="text"
                  class="input"
                  placeholder="401, 403, 429, 502, 503, 504"
                />
                <p class="mt-1 text-xs text-gray-500 dark:text-gray-400">
                  {{ t('admin.customFeatures.gateway.poolDefaults.retryStatusCodesHint') }}
                </p>
              </div>
            </div>
          </section>

          <section class="border-t border-gray-100 pt-8 dark:border-dark-700" aria-labelledby="gateway-probe-title">
            <h3 id="gateway-probe-title" class="font-semibold text-gray-900 dark:text-white">
              {{ t('admin.customFeatures.gateway.probeBackoff.title') }}
            </h3>
            <p class="mt-1 text-sm text-gray-500 dark:text-gray-400">
              {{ t('admin.customFeatures.gateway.probeBackoff.description') }}
            </p>
            <div class="mt-4 max-w-xl space-y-3">
              <div
                v-for="(_minutes, index) in gateway.auto_managed_probe_backoff_minutes"
                :key="index"
                :data-test="`gateway-probe-backoff-${index}`"
                class="flex items-end gap-2"
              >
                <div class="min-w-0 flex-1">
                  <label class="input-label" :for="`gateway-probe-backoff-${index}`">
                    {{ t('admin.customFeatures.gateway.probeBackoff.attempt', { count: index + 1 }) }}
                  </label>
                  <div class="relative">
                    <input
                      :id="`gateway-probe-backoff-${index}`"
                      v-model.number="gateway.auto_managed_probe_backoff_minutes[index]"
                      type="number"
                      min="1"
                      max="1440"
                      step="1"
                      class="input pr-16"
                    />
                    <span class="pointer-events-none absolute inset-y-0 right-3 flex items-center text-xs text-gray-500 dark:text-gray-400">
                      {{ t('admin.customFeatures.gateway.minutes') }}
                    </span>
                  </div>
                </div>
                <button
                  type="button"
                  class="btn btn-secondary inline-flex h-10 w-10 items-center justify-center px-0"
                  :disabled="gateway.auto_managed_probe_backoff_minutes.length <= 1"
                  :title="t('admin.customFeatures.gateway.probeBackoff.remove')"
                  @click="removeProbeBackoff(index)"
                >
                  <Icon name="trash" size="sm" />
                </button>
              </div>
              <button
                type="button"
                data-test="gateway-add-probe-backoff"
                class="btn btn-secondary inline-flex items-center gap-2"
                :disabled="gateway.auto_managed_probe_backoff_minutes.length >= 10"
                @click="addProbeBackoff"
              >
                <Icon name="plus" size="sm" />
                {{ t('admin.customFeatures.gateway.probeBackoff.add') }}
              </button>
            </div>
          </section>

          <section class="border-t border-gray-100 pt-8 dark:border-dark-700" aria-labelledby="gateway-timeout-title">
            <h3 id="gateway-timeout-title" class="font-semibold text-gray-900 dark:text-white">
              {{ t('admin.customFeatures.gateway.firstTokenTimeout.title') }}
            </h3>
            <p class="mt-1 text-sm text-gray-500 dark:text-gray-400">
              {{ t('admin.customFeatures.gateway.firstTokenTimeout.description') }}
            </p>
            <div class="mt-4 max-w-xs">
              <label class="input-label" for="gateway-first-token-timeout">
                {{ t('admin.customFeatures.gateway.firstTokenTimeout.seconds') }}
              </label>
              <input
                id="gateway-first-token-timeout"
                v-model.number="gateway.first_token_timeout_seconds"
                data-test="gateway-first-token-timeout"
                type="number"
                min="0"
                max="600"
                step="1"
                class="input"
              />
              <p class="mt-1 text-xs text-gray-500 dark:text-gray-400">
                {{ t('admin.customFeatures.gateway.firstTokenTimeout.hint') }}
              </p>
            </div>
          </section>

          <section class="border-t border-gray-100 pt-8 dark:border-dark-700" aria-labelledby="gateway-image-rate-title">
            <div class="flex flex-col gap-4 sm:flex-row sm:items-start sm:justify-between">
              <div>
                <h3 id="gateway-image-rate-title" class="font-semibold text-gray-900 dark:text-white">
                  {{ t('admin.customFeatures.gateway.imageSuccessRate.title') }}
                </h3>
                <p class="mt-1 text-sm text-gray-500 dark:text-gray-400">
                  {{ t('admin.customFeatures.gateway.imageSuccessRate.description') }}
                </p>
              </div>
              <div class="flex flex-shrink-0 items-center gap-3">
                <span class="text-sm font-medium text-gray-700 dark:text-gray-300">
                  {{ t('admin.customFeatures.gateway.imageSuccessRate.visible') }}
                </span>
                <Toggle v-model="gateway.image_group_success_rate_visible" data-test="gateway-image-success-rate-visible" />
              </div>
            </div>
            <div class="mt-4">
              <button
                type="button"
                data-test="gateway-reset-image-success-rates"
                class="btn btn-secondary inline-flex items-center gap-2 text-red-600 hover:text-red-700 dark:text-red-400"
                :disabled="resettingImageSuccessRates"
                @click="showResetImageSuccessRatesConfirm = true"
              >
                <span v-if="resettingImageSuccessRates" class="h-4 w-4 animate-spin rounded-full border-b-2 border-current"></span>
                <Icon v-else name="refresh" size="sm" />
                {{ t('admin.customFeatures.gateway.imageSuccessRate.reset') }}
              </button>
            </div>
          </section>
        </div>

        <div class="flex justify-end border-t border-gray-100 px-5 py-4 dark:border-dark-700 sm:px-6">
          <button
            type="submit"
            data-test="gateway-save"
            class="btn btn-primary inline-flex items-center gap-2"
            :disabled="savingGateway"
          >
            <span v-if="savingGateway" class="h-4 w-4 animate-spin rounded-full border-b-2 border-white"></span>
            <Icon v-else name="check" size="sm" />
            {{ t('admin.customFeatures.save') }}
          </button>
        </div>
      </form>

      <form
        v-else
        data-test="daily-checkin-form"
        class="card overflow-hidden"
        @submit.prevent="saveDailyCheckin"
      >
        <div class="flex flex-col gap-4 border-b border-gray-100 px-5 py-5 dark:border-dark-700 sm:flex-row sm:items-center sm:justify-between sm:px-6">
          <div>
            <h2 class="text-lg font-semibold text-gray-900 dark:text-white">
              {{ t('admin.customFeatures.dailyCheckin.title') }}
            </h2>
            <p class="mt-1 text-sm text-gray-500 dark:text-gray-400">
              {{ t('admin.customFeatures.dailyCheckin.description') }}
            </p>
          </div>
          <div class="flex items-center gap-3">
            <span class="text-sm font-medium text-gray-700 dark:text-gray-300">
              {{ t('admin.customFeatures.enabled') }}
            </span>
            <Toggle v-model="dailyCheckin.enabled" data-test="daily-checkin-enabled" />
          </div>
        </div>

        <div class="space-y-8 px-5 py-6 sm:px-6">
          <section aria-labelledby="daily-prize-pool-title">
            <div class="flex flex-col gap-3 sm:flex-row sm:items-center sm:justify-between">
              <div>
                <h3 id="daily-prize-pool-title" class="font-semibold text-gray-900 dark:text-white">
                  {{ t('admin.customFeatures.dailyCheckin.prizePool') }}
                </h3>
                <p class="mt-1 text-sm text-gray-500 dark:text-gray-400">
                  {{ t('admin.customFeatures.dailyCheckin.prizePoolHint') }}
                </p>
              </div>
              <div class="flex flex-wrap items-center gap-3">
                <span
                  data-test="daily-checkin-probability-total"
                  class="text-sm font-medium"
                  :class="dailyProbabilityTotal === 10000 ? 'text-emerald-600 dark:text-emerald-400' : 'text-amber-600 dark:text-amber-400'"
                >
                  {{ t('admin.customFeatures.dailyCheckin.probabilityTotal', { total: formatPercent(dailyProbabilityTotal) }) }}
                </span>
                <button type="button" class="btn btn-secondary btn-sm inline-flex items-center gap-2" @click="addPrize">
                  <Icon name="plus" size="sm" />
                  {{ t('admin.customFeatures.dailyCheckin.addPrize') }}
                </button>
              </div>
            </div>

            <div v-if="dailyCheckin.prizes.length === 0" class="mt-4 rounded-md border border-dashed border-gray-300 px-4 py-8 text-center text-sm text-gray-500 dark:border-dark-600 dark:text-gray-400">
              {{ t('admin.customFeatures.dailyCheckin.noPrizes') }}
            </div>

            <div v-else class="mt-4 space-y-4">
              <div
                v-for="(prize, index) in dailyCheckin.prizes"
                :key="prize.id || index"
                :data-test="`daily-prize-${index}`"
                class="rounded-md border border-gray-200 p-4 dark:border-dark-600"
              >
                <div class="grid grid-cols-1 gap-4 md:grid-cols-[72px_minmax(0,1fr)_170px_150px_40px]">
                  <div>
                    <label class="mb-1 block text-xs font-medium text-gray-600 dark:text-gray-400">
                      {{ t('admin.customFeatures.dailyCheckin.status') }}
                    </label>
                    <div class="flex h-10 items-center">
                      <Toggle v-model="prize.enabled" />
                    </div>
                  </div>
                  <div>
                    <label class="mb-1 block text-xs font-medium text-gray-600 dark:text-gray-400">
                      {{ t('admin.customFeatures.dailyCheckin.prizeName') }}
                    </label>
                    <input v-model.trim="prize.name" type="text" class="input h-10" />
                  </div>
                  <div>
                    <label class="mb-1 block text-xs font-medium text-gray-600 dark:text-gray-400">
                      {{ t('admin.customFeatures.dailyCheckin.prizeType') }}
                    </label>
                    <select v-model="prize.type" class="input h-10">
                      <option value="balance">{{ t('admin.customFeatures.dailyCheckin.types.balance') }}</option>
                      <option value="concurrency">{{ t('admin.customFeatures.dailyCheckin.types.concurrency') }}</option>
                      <option value="subscription">{{ t('admin.customFeatures.dailyCheckin.types.subscription') }}</option>
                      <option value="none">{{ t('admin.customFeatures.dailyCheckin.types.none') }}</option>
                    </select>
                  </div>
                  <div>
                    <label class="mb-1 block text-xs font-medium text-gray-600 dark:text-gray-400">
                      {{ t('admin.customFeatures.dailyCheckin.probabilityBps') }}
                    </label>
                    <input v-model.number="prize.probability_bps" type="number" min="0" max="10000" step="1" class="input h-10" />
                    <p class="mt-1 text-xs text-gray-500 dark:text-gray-400">{{ formatPercent(prize.probability_bps) }}%</p>
                  </div>
                  <div class="flex items-end">
                    <button
                      type="button"
                      class="inline-flex h-10 w-10 items-center justify-center rounded-md text-red-600 transition-colors hover:bg-red-50 dark:text-red-400 dark:hover:bg-red-900/20"
                      :title="t('common.delete')"
                      :aria-label="t('common.delete')"
                      @click="removePrize(index)"
                    >
                      <Icon name="trash" size="sm" />
                    </button>
                  </div>
                </div>

                <div class="mt-4 border-t border-gray-100 pt-4 dark:border-dark-700">
                  <div v-if="prize.type === 'balance'" class="grid grid-cols-1 gap-4 md:grid-cols-3">
                    <div>
                      <label class="mb-1 block text-xs font-medium text-gray-600 dark:text-gray-400">
                        {{ t('admin.customFeatures.dailyCheckin.balanceMode') }}
                      </label>
                      <select v-model="prize.balance_mode" class="input h-10">
                        <option value="fixed">{{ t('admin.customFeatures.dailyCheckin.fixed') }}</option>
                        <option value="range">{{ t('admin.customFeatures.dailyCheckin.range') }}</option>
                      </select>
                    </div>
                    <div v-if="prize.balance_mode !== 'range'">
                      <label class="mb-1 block text-xs font-medium text-gray-600 dark:text-gray-400">
                        {{ t('admin.customFeatures.dailyCheckin.amount') }}
                      </label>
                      <input v-model.number="prize.amount" type="number" min="0" step="0.01" class="input h-10" />
                    </div>
                    <template v-else>
                      <div>
                        <label class="mb-1 block text-xs font-medium text-gray-600 dark:text-gray-400">
                          {{ t('admin.customFeatures.dailyCheckin.minAmount') }}
                        </label>
                        <input v-model.number="prize.min_amount" type="number" min="0" step="0.01" class="input h-10" />
                      </div>
                      <div>
                        <label class="mb-1 block text-xs font-medium text-gray-600 dark:text-gray-400">
                          {{ t('admin.customFeatures.dailyCheckin.maxAmount') }}
                        </label>
                        <input v-model.number="prize.max_amount" type="number" min="0" step="0.01" class="input h-10" />
                      </div>
                    </template>
                  </div>

                  <div v-else-if="prize.type === 'concurrency'" class="max-w-xs">
                    <label class="mb-1 block text-xs font-medium text-gray-600 dark:text-gray-400">
                      {{ t('admin.customFeatures.dailyCheckin.concurrency') }}
                    </label>
                    <input v-model.number="prize.concurrency" type="number" min="1" step="1" class="input h-10" />
                  </div>

                  <div v-else-if="prize.type === 'subscription'" class="grid grid-cols-1 gap-4 md:grid-cols-[minmax(0,1fr)_180px]">
                    <div>
                      <label class="mb-1 block text-xs font-medium text-gray-600 dark:text-gray-400">
                        {{ t('admin.customFeatures.dailyCheckin.subscriptionGroup') }}
                      </label>
                      <Select
                        v-model="prize.group_id"
                        :options="subscriptionGroupOptions"
                        :placeholder="t('admin.customFeatures.dailyCheckin.subscriptionGroup')"
                        :empty-text="t('admin.customFeatures.dailyCheckin.noSubscriptionGroups')"
                      />
                    </div>
                    <div>
                      <label class="mb-1 block text-xs font-medium text-gray-600 dark:text-gray-400">
                        {{ t('admin.customFeatures.dailyCheckin.validityDays') }}
                      </label>
                      <input v-model.number="prize.validity_days" type="number" min="1" max="36500" step="1" class="input h-10" />
                    </div>
                  </div>

                  <p v-else class="text-sm text-gray-500 dark:text-gray-400">
                    {{ t('admin.customFeatures.dailyCheckin.noneHint') }}
                  </p>
                </div>
              </div>
            </div>
          </section>

          <section class="border-t border-gray-100 pt-8 dark:border-dark-700" aria-labelledby="daily-decay-title">
            <div class="grid grid-cols-1 gap-8 lg:grid-cols-[minmax(0,1fr)_320px]">
              <div>
                <h3 id="daily-decay-title" class="font-semibold text-gray-900 dark:text-white">
                  {{ t('admin.customFeatures.dailyCheckin.decayTitle') }}
                </h3>
                <p class="mt-1 text-sm text-gray-500 dark:text-gray-400">
                  {{ t('admin.customFeatures.dailyCheckin.decayHint') }}
                </p>
                <div class="mt-4 max-w-xs">
                  <label class="input-label" for="daily-unpaid-full-days">
                    {{ t('admin.customFeatures.dailyCheckin.fullDays') }}
                  </label>
                  <input id="daily-unpaid-full-days" v-model.number="dailyCheckin.unpaid_full_days" type="number" min="0" max="3650" step="1" class="input h-10" />
                </div>
                <div class="mt-5 space-y-3">
                  <div
                    v-for="(rule, index) in dailyCheckin.unpaid_decay_rules"
                    :key="`decay-${index}`"
                    class="grid grid-cols-1 gap-3 rounded-md border border-gray-200 p-3 dark:border-dark-600 sm:grid-cols-2"
                  >
                    <div>
                      <label class="mb-1 block text-xs font-medium text-gray-600 dark:text-gray-400">
                        {{ t('admin.customFeatures.dailyCheckin.afterDays') }}
                      </label>
                      <input v-model.number="rule.after_days" type="number" min="0" max="3650" step="1" class="input h-10" />
                    </div>
                    <div>
                      <label class="mb-1 block text-xs font-medium text-gray-600 dark:text-gray-400">
                        {{ t('admin.customFeatures.dailyCheckin.factorBps') }}
                      </label>
                      <input v-model.number="rule.factor_bps" type="number" min="0" max="10000" step="1" class="input h-10" />
                      <p class="mt-1 text-xs text-gray-500 dark:text-gray-400">{{ formatPercent(rule.factor_bps) }}%</p>
                    </div>
                  </div>
                </div>
              </div>

              <div class="border-t border-gray-100 pt-6 dark:border-dark-700 lg:border-l lg:border-t-0 lg:pl-8 lg:pt-0">
                <div class="flex items-start justify-between gap-4">
                  <div>
                    <h3 class="font-semibold text-gray-900 dark:text-white">
                      {{ t('admin.customFeatures.dailyCheckin.linuxdoExempt') }}
                    </h3>
                    <p class="mt-1 text-sm text-gray-500 dark:text-gray-400">
                      {{ t('admin.customFeatures.dailyCheckin.linuxdoExemptHint') }}
                    </p>
                  </div>
                  <Toggle v-model="dailyCheckin.linuxdo_exempt_enabled" data-test="daily-linuxdo-exempt" />
                </div>
              </div>
            </div>
          </section>
        </div>

        <div class="flex justify-end border-t border-gray-100 px-5 py-4 dark:border-dark-700 sm:px-6">
          <button type="submit" class="btn btn-primary inline-flex items-center gap-2" :disabled="savingDailyCheckin">
            <span v-if="savingDailyCheckin" class="h-4 w-4 animate-spin rounded-full border-b-2 border-white"></span>
            <Icon v-else name="check" size="sm" />
            {{ t('admin.customFeatures.save') }}
          </button>
        </div>
      </form>

      <ConfirmDialog
        :show="showResetImageSuccessRatesConfirm"
        :title="t('admin.customFeatures.gateway.imageSuccessRate.resetTitle')"
        :message="t('admin.customFeatures.gateway.imageSuccessRate.resetMessage')"
        :confirm-text="resettingImageSuccessRates ? t('admin.customFeatures.gateway.imageSuccessRate.resetting') : t('admin.customFeatures.gateway.imageSuccessRate.resetConfirm')"
        :danger="true"
        @confirm="resetImageSuccessRates"
        @cancel="showResetImageSuccessRatesConfirm = false"
      />
    </div>
  </AppLayout>
</template>

<script setup lang="ts">
import { computed, onMounted, reactive, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import AppLayout from '@/components/layout/AppLayout.vue'
import ConfirmDialog from '@/components/common/ConfirmDialog.vue'
import Icon from '@/components/icons/Icon.vue'
import Select from '@/components/common/Select.vue'
import Toggle from '@/components/common/Toggle.vue'
import groupsAPI from '@/api/admin/groups'
import customFeaturesAPI, {
  type DailyCheckinPrizeConfig,
  type DailyCheckinSettings,
  type GatewaySettings,
  type ModelMarketplaceSettings
} from '@/api/admin/customFeatures'
import type { AdminGroup } from '@/types'
import { useAppStore } from '@/stores/app'
import { extractApiErrorMessage } from '@/utils/apiError'

type CustomFeatureTab = 'model-marketplace' | 'gateway' | 'daily-checkin'

const { t } = useI18n()
const appStore = useAppStore()

const tabs: Array<{ key: CustomFeatureTab; labelKey: string; icon: 'cube' | 'cog' | 'gift' }> = [
  { key: 'model-marketplace', labelKey: 'admin.customFeatures.tabs.modelMarketplace', icon: 'cube' },
  { key: 'gateway', labelKey: 'admin.customFeatures.tabs.gateway', icon: 'cog' },
  { key: 'daily-checkin', labelKey: 'admin.customFeatures.tabs.dailyCheckin', icon: 'gift' }
]

const activeTab = ref<CustomFeatureTab>('model-marketplace')
const loading = ref(true)
const loadFailed = ref(false)
const savingMarketplace = ref(false)
const savingGateway = ref(false)
const savingDailyCheckin = ref(false)
const resettingImageSuccessRates = ref(false)
const showResetImageSuccessRatesConfirm = ref(false)
const activeGroups = ref<AdminGroup[]>([])

const modelMarketplace = reactive<ModelMarketplaceSettings>({
  enabled: true,
  intro: '',
  group_ids: []
})

const dailyCheckin = reactive<DailyCheckinSettings>({
  enabled: false,
  prizes: [],
  unpaid_full_days: 7,
  unpaid_decay_rules: [
    { after_days: 7, factor_bps: 5000 },
    { after_days: 14, factor_bps: 2000 },
    { after_days: 30, factor_bps: 0 }
  ],
  linuxdo_exempt_enabled: false
})

const gateway = reactive<GatewaySettings>({
  default_pool_mode_retry_count: 1,
  default_pool_mode_retry_status_codes: [401, 403, 429, 502, 503, 504],
  auto_managed_probe_backoff_minutes: [5, 10, 15, 30, 60],
  first_token_timeout_seconds: 60,
  image_group_success_rate_visible: true
})
const gatewayRetryStatusCodesInput = ref(gateway.default_pool_mode_retry_status_codes.join(', '))

const subscriptionGroupOptions = computed(() =>
  activeGroups.value
    .filter((group) => group.subscription_type === 'subscription')
    .map((group) => ({
      value: group.id,
      label: `${group.name} · ${group.platform}`
    }))
)

const dailyProbabilityTotal = computed(() =>
  dailyCheckin.prizes
    .filter((prize) => prize.enabled !== false)
    .reduce((sum, prize) => sum + toInteger(prize.probability_bps), 0)
)

function cloneDailyCheckin(settings: DailyCheckinSettings): DailyCheckinSettings {
  return {
    enabled: settings.enabled,
    prizes: (settings.prizes || []).map((prize, index) => ({
      ...prize,
      sort_order: index
    })),
    unpaid_full_days: settings.unpaid_full_days,
    unpaid_decay_rules: (settings.unpaid_decay_rules || []).map((rule) => ({ ...rule })),
    linuxdo_exempt_enabled: settings.linuxdo_exempt_enabled
  }
}

function cloneGateway(settings?: Partial<GatewaySettings>): GatewaySettings {
  return {
    default_pool_mode_retry_count: settings?.default_pool_mode_retry_count ?? 1,
    default_pool_mode_retry_status_codes: [
      ...(settings?.default_pool_mode_retry_status_codes ?? [401, 403, 429, 502, 503, 504])
    ],
    auto_managed_probe_backoff_minutes: [
      ...(settings?.auto_managed_probe_backoff_minutes ?? [5, 10, 15, 30, 60])
    ],
    first_token_timeout_seconds: settings?.first_token_timeout_seconds ?? 60,
    image_group_success_rate_visible: settings?.image_group_success_rate_visible ?? true
  }
}

function assignGateway(settings?: Partial<GatewaySettings>) {
  const next = cloneGateway(settings)
  Object.assign(gateway, next)
  gatewayRetryStatusCodesInput.value = next.default_pool_mode_retry_status_codes.join(', ')
}

async function loadSettings() {
  loading.value = true
  loadFailed.value = false
  try {
    const [settings, groups] = await Promise.all([
      customFeaturesAPI.getSettings(),
      groupsAPI.getAll()
    ])
    Object.assign(modelMarketplace, {
      ...settings.model_marketplace,
      group_ids: [...(settings.model_marketplace.group_ids || [])]
    })
    Object.assign(dailyCheckin, cloneDailyCheckin(settings.daily_checkin))
    assignGateway(settings.gateway)
    activeGroups.value = (groups || []).filter((group) => group.status === 'active')
  } catch (error) {
    loadFailed.value = true
    appStore.showError(extractApiErrorMessage(error, t('admin.customFeatures.loadFailed')))
  } finally {
    loading.value = false
  }
}

function addProbeBackoff() {
  if (gateway.auto_managed_probe_backoff_minutes.length >= 10) return
  const last = gateway.auto_managed_probe_backoff_minutes[
    gateway.auto_managed_probe_backoff_minutes.length - 1
  ] ?? 5
  gateway.auto_managed_probe_backoff_minutes.push(last)
}

function removeProbeBackoff(index: number) {
  if (gateway.auto_managed_probe_backoff_minutes.length <= 1) return
  gateway.auto_managed_probe_backoff_minutes.splice(index, 1)
}

function parseGatewayRetryStatusCodes(): number[] | null {
  const input = gatewayRetryStatusCodesInput.value.trim()
  if (!input) return []
  const tokens = input.split(/[,\s]+/).filter(Boolean)
  if (tokens.length === 0) return null

  const statusCodes: number[] = []
  for (const token of tokens) {
    if (!/^\d+$/.test(token)) return null
    const statusCode = Number(token)
    if (!Number.isInteger(statusCode) || statusCode < 100 || statusCode > 599) return null
    statusCodes.push(statusCode)
  }
  return [...new Set(statusCodes)].sort((left, right) => left - right)
}

function validateGateway(): { error: string | null; retryStatusCodes: number[] } {
  const retryCount = gateway.default_pool_mode_retry_count
  if (!Number.isInteger(retryCount) || retryCount < 0 || retryCount > 10) {
    return {
      error: t('admin.customFeatures.gateway.validation.retryCount'),
      retryStatusCodes: []
    }
  }

  const retryStatusCodes = parseGatewayRetryStatusCodes()
  if (retryStatusCodes === null) {
    return {
      error: t('admin.customFeatures.gateway.validation.retryStatusCodes'),
      retryStatusCodes: []
    }
  }

  const backoff = gateway.auto_managed_probe_backoff_minutes
  if (
    backoff.length < 1 ||
    backoff.length > 10 ||
    backoff.some((minutes) => !Number.isInteger(minutes) || minutes < 1 || minutes > 1440)
  ) {
    return {
      error: t('admin.customFeatures.gateway.validation.probeBackoffRange'),
      retryStatusCodes
    }
  }
  if (backoff.some((minutes, index) => index > 0 && minutes < backoff[index - 1])) {
    return {
      error: t('admin.customFeatures.gateway.validation.probeBackoffOrder'),
      retryStatusCodes
    }
  }

  const timeoutSeconds = gateway.first_token_timeout_seconds
  if (!Number.isInteger(timeoutSeconds) || timeoutSeconds < 0 || timeoutSeconds > 600) {
    return {
      error: t('admin.customFeatures.gateway.validation.firstTokenTimeout'),
      retryStatusCodes
    }
  }

  return { error: null, retryStatusCodes }
}

async function saveGateway() {
  const validation = validateGateway()
  if (validation.error) {
    appStore.showError(validation.error)
    return
  }

  savingGateway.value = true
  try {
    const saved = await customFeaturesAPI.updateGateway({
      default_pool_mode_retry_count: Number(gateway.default_pool_mode_retry_count),
      default_pool_mode_retry_status_codes: validation.retryStatusCodes,
      auto_managed_probe_backoff_minutes: gateway.auto_managed_probe_backoff_minutes.map(Number),
      first_token_timeout_seconds: Number(gateway.first_token_timeout_seconds),
      image_group_success_rate_visible: gateway.image_group_success_rate_visible
    })
    assignGateway(saved)
    appStore.showSuccess(t('admin.customFeatures.gateway.saved'))
  } catch (error) {
    appStore.showError(extractApiErrorMessage(error, t('admin.customFeatures.saveFailed')))
  } finally {
    savingGateway.value = false
  }
}

async function resetImageSuccessRates() {
  if (resettingImageSuccessRates.value) return
  resettingImageSuccessRates.value = true
  try {
    await customFeaturesAPI.resetImageGroupSuccessRates()
    showResetImageSuccessRatesConfirm.value = false
    appStore.showSuccess(t('admin.customFeatures.gateway.imageSuccessRate.resetSuccess'))
  } catch (error) {
    appStore.showError(
      extractApiErrorMessage(error, t('admin.customFeatures.gateway.imageSuccessRate.resetFailed'))
    )
  } finally {
    resettingImageSuccessRates.value = false
  }
}

async function saveModelMarketplace() {
  savingMarketplace.value = true
  try {
    const saved = await customFeaturesAPI.updateModelMarketplace({
      enabled: modelMarketplace.enabled,
      intro: modelMarketplace.intro.trim(),
      group_ids: [...new Set(modelMarketplace.group_ids.filter((id) => Number.isInteger(id) && id > 0))]
    })
    Object.assign(modelMarketplace, { ...saved, group_ids: [...saved.group_ids] })
    appStore.showSuccess(t('admin.customFeatures.modelMarketplace.saved'))
  } catch (error) {
    appStore.showError(extractApiErrorMessage(error, t('admin.customFeatures.saveFailed')))
  } finally {
    savingMarketplace.value = false
  }
}

function addPrize() {
  dailyCheckin.prizes.push({
    id: `prize_${Date.now().toString(36)}`,
    name: t('admin.customFeatures.dailyCheckin.newPrize'),
    type: 'none',
    probability_bps: 0,
    enabled: true,
    sort_order: dailyCheckin.prizes.length
  })
}

function removePrize(index: number) {
  dailyCheckin.prizes.splice(index, 1)
}

function validateDailyCheckin(): string | null {
  const enabledPrizes = dailyCheckin.prizes.filter((prize) => prize.enabled !== false)
  if (dailyCheckin.enabled && enabledPrizes.length === 0) {
    return t('admin.customFeatures.dailyCheckin.validation.prizesRequired')
  }
  if (dailyCheckin.enabled && dailyProbabilityTotal.value !== 10000) {
    return t('admin.customFeatures.dailyCheckin.validation.probabilityTotal')
  }

  for (const prize of enabledPrizes) {
    if (!prize.name.trim()) {
      return t('admin.customFeatures.dailyCheckin.validation.nameRequired')
    }
    const probability = toInteger(prize.probability_bps)
    if (probability < 0 || probability > 10000) {
      return t('admin.customFeatures.dailyCheckin.validation.probabilityRange')
    }
    if (prize.type === 'balance') {
      if (prize.balance_mode === 'range') {
        const min = toNumber(prize.min_amount)
        const max = toNumber(prize.max_amount)
        if (min < 0 || max <= 0 || max < min) {
          return t('admin.customFeatures.dailyCheckin.validation.balanceRange')
        }
      } else if (toNumber(prize.amount) <= 0) {
        return t('admin.customFeatures.dailyCheckin.validation.balanceAmount')
      }
    }
    if (prize.type === 'concurrency' && toInteger(prize.concurrency) <= 0) {
      return t('admin.customFeatures.dailyCheckin.validation.concurrency')
    }
    if (
      prize.type === 'subscription' &&
      (toInteger(prize.group_id) <= 0 || toInteger(prize.validity_days) <= 0)
    ) {
      return t('admin.customFeatures.dailyCheckin.validation.subscription')
    }
  }

  const fullDays = toInteger(dailyCheckin.unpaid_full_days)
  if (fullDays < 0 || fullDays > 3650) {
    return t('admin.customFeatures.dailyCheckin.validation.decayRange')
  }
  for (const rule of dailyCheckin.unpaid_decay_rules) {
    const afterDays = toInteger(rule.after_days)
    const factor = toInteger(rule.factor_bps)
    if (afterDays < 0 || afterDays > 3650 || factor < 0 || factor > 10000) {
      return t('admin.customFeatures.dailyCheckin.validation.decayRange')
    }
  }
  return null
}

function dailyCheckinPayload(): DailyCheckinSettings {
  return {
    enabled: dailyCheckin.enabled,
    prizes: dailyCheckin.prizes.map((prize, index): DailyCheckinPrizeConfig => ({
      ...prize,
      id: prize.id || `prize_${index + 1}`,
      name: prize.name.trim(),
      probability_bps: toInteger(prize.probability_bps),
      enabled: prize.enabled !== false,
      sort_order: index,
      balance_mode: prize.balance_mode === 'range' ? 'range' : 'fixed',
      amount: Math.max(0, toNumber(prize.amount)),
      min_amount: Math.max(0, toNumber(prize.min_amount)),
      max_amount: Math.max(0, toNumber(prize.max_amount)),
      concurrency: Math.max(0, toInteger(prize.concurrency)),
      group_id: Math.max(0, toInteger(prize.group_id)),
      validity_days: Math.max(0, toInteger(prize.validity_days))
    })),
    unpaid_full_days: toInteger(dailyCheckin.unpaid_full_days),
    unpaid_decay_rules: dailyCheckin.unpaid_decay_rules.map((rule) => ({
      after_days: toInteger(rule.after_days),
      factor_bps: toInteger(rule.factor_bps)
    })),
    linuxdo_exempt_enabled: dailyCheckin.linuxdo_exempt_enabled
  }
}

async function saveDailyCheckin() {
  const validationError = validateDailyCheckin()
  if (validationError) {
    appStore.showError(validationError)
    return
  }

  savingDailyCheckin.value = true
  try {
    const saved = await customFeaturesAPI.updateDailyCheckin(dailyCheckinPayload())
    Object.assign(dailyCheckin, cloneDailyCheckin(saved))
    appStore.showSuccess(t('admin.customFeatures.dailyCheckin.saved'))
  } catch (error) {
    appStore.showError(extractApiErrorMessage(error, t('admin.customFeatures.saveFailed')))
  } finally {
    savingDailyCheckin.value = false
  }
}

function toNumber(value: unknown): number {
  const number = Number(value)
  return Number.isFinite(number) ? number : 0
}

function toInteger(value: unknown): number {
  return Math.trunc(toNumber(value))
}

function formatPercent(bps: unknown): string {
  return (toNumber(bps) / 100).toFixed(2)
}

onMounted(loadSettings)
</script>

import { beforeEach, describe, expect, it, vi } from 'vitest'

const { put, post } = vi.hoisted(() => ({
  put: vi.fn(),
  post: vi.fn(),
}))

vi.mock('../client', () => ({
  apiClient: { put, post },
}))

import {
  resetImageGroupSuccessRates,
  updateGateway,
  type GatewaySettings,
} from '@/api/admin/customFeatures'

const gatewaySettings: GatewaySettings = {
  default_pool_mode_retry_count: 1,
  default_pool_mode_retry_status_codes: [401, 403, 429, 502, 503, 504],
  auto_managed_probe_backoff_minutes: [5, 10, 15, 30, 60],
  first_token_timeout_seconds: 60,
  first_token_timeout_consecutive_threshold: 3,
  upstream_error_status_codes: [502, 503, 504],
  upstream_error_consecutive_threshold: 10,
  image_group_success_rate_visible: true,
  anthropic_claude_code_mimicry_enabled: false,
}

describe('admin custom features gateway API', () => {
  beforeEach(() => {
    put.mockReset()
    post.mockReset()
  })

  it('更新网关配置并返回规范化结果', async () => {
    put.mockResolvedValue({ data: gatewaySettings })

    await expect(updateGateway(gatewaySettings)).resolves.toEqual(gatewaySettings)
    expect(put).toHaveBeenCalledWith('/admin/custom-features/gateway', gatewaySettings)
  })

  it('允许只更新部分网关配置', async () => {
    const partialSettings = { first_token_timeout_consecutive_threshold: 5 }
    put.mockResolvedValue({
      data: { ...gatewaySettings, ...partialSettings },
    })

    await expect(updateGateway(partialSettings)).resolves.toEqual({
      ...gatewaySettings,
      ...partialSettings,
    })
    expect(put).toHaveBeenCalledWith('/admin/custom-features/gateway', partialSettings)
  })

  it('调用独立接口清除 Image 分组成功率', async () => {
    post.mockResolvedValue({ data: { reset_at: '2026-07-12T00:00:00Z' } })

    await expect(resetImageGroupSuccessRates()).resolves.toEqual({
      reset_at: '2026-07-12T00:00:00Z',
    })
    expect(post).toHaveBeenCalledWith(
      '/admin/custom-features/gateway/image-group-success-rates/reset'
    )
  })
})

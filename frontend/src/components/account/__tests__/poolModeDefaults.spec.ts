import { describe, expect, it } from 'vitest'

import {
  normalizeGatewayPoolDefaults,
  writePoolModeCredentials,
} from '../poolModeDefaults'

describe('新建账号池模式默认值', () => {
  it('使用网关配置并规范化状态码', () => {
    expect(normalizeGatewayPoolDefaults({
      default_pool_mode_retry_count: 2,
      default_pool_mode_retry_status_codes: [504, 401, 504, 700],
    })).toEqual({
      retryCount: 2,
      retryStatusCodes: [401, 504],
    })
  })

  it('开启时写入次数和可为空的状态码数组', () => {
    const credentials: Record<string, unknown> = {}

    writePoolModeCredentials(credentials, true, 1, '')

    expect(credentials).toEqual({
      pool_mode: true,
      pool_mode_retry_count: 1,
      pool_mode_retry_status_codes: [],
    })
  })

  it('用户关闭时明确提交 pool_mode=false', () => {
    const credentials: Record<string, unknown> = {}

    writePoolModeCredentials(credentials, false, 1, '401, 429')

    expect(credentials).toEqual({ pool_mode: false })
  })
})

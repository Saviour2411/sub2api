import { apiClient } from '../client'

// 只读当前构建版本；版本发布由部署流程管理。
export async function getVersion(): Promise<{ version: string }> {
  const { data } = await apiClient.get<{ version: string }>('/admin/system/version')
  return data
}

export const systemAPI = { getVersion }

export default systemAPI

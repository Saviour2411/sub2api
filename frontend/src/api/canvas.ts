import { apiClient } from './client'

export type CanvasPlatform = 'openai' | 'gemini' | 'antigravity' | 'grok'

export interface CanvasGroupSummary {
  id: number
  name: string
  platform: CanvasPlatform
  api_format: 'openai' | 'gemini'
}

export interface CanvasBootstrap {
  user_id: number
  groups: CanvasGroupSummary[]
}

export interface CanvasCredential {
  group_id: number
  platform: CanvasPlatform
  api_format: 'openai' | 'gemini'
  base_url: string
  api_key: string
}

export async function getCanvasBootstrap(): Promise<CanvasBootstrap> {
  const { data } = await apiClient.get<CanvasBootstrap>('/canvas/bootstrap')
  return data
}

export async function resolveCanvasCredential(groupId: number): Promise<CanvasCredential> {
  const { data } = await apiClient.post<CanvasCredential>('/canvas/credentials/resolve', {
    group_id: groupId
  })
  return data
}

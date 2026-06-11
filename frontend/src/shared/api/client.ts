import axios from 'axios'

export const apiClient = axios.create({
  baseURL: import.meta.env.VITE_API_BASE_URL || '/api/v1',
  timeout: 15_000,
})

apiClient.interceptors.request.use((config) => {
  config.headers.set('X-Tenant-ID', 'tenant-demo')
  return config
})

export type PlatformStats = {
  activeProjects: number
  monthlyBids: number
  compliancePassRate: number
  winRate: number
  pendingTasks: number
  knowledgeDocs: number
}

export async function fetchPlatformStats(): Promise<PlatformStats> {
  return {
    activeProjects: 18,
    monthlyBids: 34,
    compliancePassRate: 92,
    winRate: 38,
    pendingTasks: 11,
    knowledgeDocs: 426,
  }
}

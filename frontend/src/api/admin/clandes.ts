/**
 * Clandes integration API endpoints
 */

import { apiClient } from '../client'

export interface ClandesStatus {
  enabled: boolean
  connected: boolean
  addr: string
}

export interface ClandesAccount {
  id: number
  name: string
  platform: string
  type: string
  status: string
  concurrency: number
  created_at: string
  updated_at: string
}

export interface CreateClandesAccountRequest {
  name: string
  type: 'oauth' | 'setup-token' | 'apikey'
  credentials: Record<string, unknown>
  proxy_id?: number
  group_ids?: number[]
}

export async function getStatus(): Promise<ClandesStatus> {
  const { data } = await apiClient.get<ClandesStatus>('/admin/clandes/status')
  return data
}

export async function syncAccounts(): Promise<{ message: string }> {
  const { data } = await apiClient.post<{ message: string }>('/admin/clandes/sync')
  return data
}

export async function listAccounts(): Promise<ClandesAccount[]> {
  const { data } = await apiClient.get<ClandesAccount[]>('/admin/clandes/accounts')
  return data
}

export async function createAccount(req: CreateClandesAccountRequest): Promise<ClandesAccount> {
  const { data } = await apiClient.post<ClandesAccount>('/admin/clandes/accounts', req)
  return data
}

export async function deleteAccount(id: number): Promise<void> {
  await apiClient.delete(`/admin/clandes/accounts/${id}`)
}

export const clandesAPI = {
  getStatus,
  syncAccounts,
  listAccounts,
  createAccount,
  deleteAccount
}

export default clandesAPI

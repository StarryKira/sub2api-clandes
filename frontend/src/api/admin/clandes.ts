/**
 * Clandes integration API endpoints
 */

import { apiClient } from '../client'

export interface ClandesStatus {
  enabled: boolean
  connected: boolean
  addr: string
  version?: string
}

export async function getStatus(): Promise<ClandesStatus> {
  const { data } = await apiClient.get<ClandesStatus>('/admin/clandes/status')
  return data
}

export interface ClandesConfig {
  enabled: boolean
  addr: string
  auth_token_configured: boolean
  reconnect_interval: number
  config_file: string
}

export interface ClandesConfigUpdate {
  enabled: boolean
  addr: string
  // null = keep existing token; "" = clear
  auth_token: string | null
  reconnect_interval: number
}

export async function getConfig(): Promise<ClandesConfig> {
  const { data } = await apiClient.get<ClandesConfig>('/admin/clandes/config')
  return data
}

export async function updateConfig(
  payload: ClandesConfigUpdate
): Promise<{ message: string; config_file: string }> {
  const { data } = await apiClient.post<{ message: string; config_file: string }>(
    '/admin/clandes/config',
    payload
  )
  return data
}

export async function syncAccounts(): Promise<{ message: string }> {
  const { data } = await apiClient.post<{ message: string }>('/admin/clandes/sync')
  return data
}

export interface OAuthStartResponse {
  auth_url: string
  session_id: string
}

export interface OAuthExchangeResponse {
  access_token: string
  refresh_token: string
  expires_in: number
  email: string
  org_uuid: string
}

export async function startOAuth(
  redirectUri: string,
  proxyId?: number | null,
  platform?: string
): Promise<OAuthStartResponse> {
  const { data } = await apiClient.post<OAuthStartResponse>('/admin/clandes/oauth/start', {
    redirect_uri: redirectUri,
    ...(proxyId != null ? { proxy_id: proxyId } : {}),
    ...(platform ? { platform } : {})
  })
  return data
}

export interface CodexOAuthExchangeResponse {
  account_id: string
  access_token: string
  refresh_token: string
  id_token: string
  expires_in: number
  chatgpt_account_id: string
  email: string
  plan_type: string
}

export async function exchangeOAuth(
  sessionId: string,
  code: string,
  callbackUrl: string
): Promise<OAuthExchangeResponse> {
  const { data } = await apiClient.post<OAuthExchangeResponse>('/admin/clandes/oauth/exchange', {
    session_id: sessionId,
    code,
    callback_url: callbackUrl
  })
  return data
}

export async function exchangeCodexOAuth(
  sessionId: string,
  code: string
): Promise<CodexOAuthExchangeResponse> {
  const { data } = await apiClient.post<CodexOAuthExchangeResponse>(
    '/admin/clandes/oauth/exchange',
    {
      session_id: sessionId,
      code,
      platform: 'openai'
    }
  )
  return data
}

export interface CodexRefreshResponse {
  account_id: number
  expires_in: number
}

export async function refreshCodexAccount(accountId: number): Promise<CodexRefreshResponse> {
  const { data } = await apiClient.post<CodexRefreshResponse>(
    `/admin/clandes/accounts/${accountId}/refresh`
  )
  return data
}

export interface CodexProfileResponse {
  account_id: string
  chatgpt_account_id: string
  email: string
  plan_type: string
}

export async function getCodexProfile(accountId: number): Promise<CodexProfileResponse> {
  const { data } = await apiClient.get<CodexProfileResponse>(
    `/admin/clandes/accounts/${accountId}/profile`
  )
  return data
}

export const clandesAPI = {
  getStatus,
  getConfig,
  updateConfig,
  syncAccounts,
  startOAuth,
  exchangeOAuth,
  exchangeCodexOAuth,
  refreshCodexAccount,
  getCodexProfile
}

export default clandesAPI

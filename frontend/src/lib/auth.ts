import { appClient, getStoredSessionToken, writeStoredSessionToken } from './api'
import { MOCK_MODE } from './mock-mode'

export type SiteRole = 'USER' | 'SITE_ADMIN'

export interface AuthUser {
  id: string
  name: string
  email: string
  image?: string | null
  siteRole?: SiteRole
  vrchatUsername?: string | null
}

export interface AuthSession {
  session: {
    token: string
    expiresAt: string
  }
  user: AuthUser
}

const MOCK_SESSION_KEY = 'lightwing:mock:session'

const defaultMockSession: AuthSession = {
  session: {
    token: 'mock-session-token',
    expiresAt: new Date(Date.now() + 7 * 24 * 60 * 60 * 1000).toISOString(),
  },
  user: {
    id: 'mock-admin-1',
    name: 'Mock Admin',
    email: 'mock-admin@lightwing.local',
    image: null,
    siteRole: 'SITE_ADMIN',
    vrchatUsername: null,
  },
}

function sanitizeRedirectPath(raw: string | undefined): string {
  if (!raw || !raw.startsWith('/')) {
    return '/'
  }
  return raw
}

function readMockSession(): AuthSession | null {
  const raw = globalThis.localStorage.getItem(MOCK_SESSION_KEY)
  if (!raw) {
    return null
  }

  try {
    return JSON.parse(raw) as AuthSession
  } catch {
    return null
  }
}

function writeMockSession(session: AuthSession | null) {
  if (!session) {
    globalThis.localStorage.removeItem(MOCK_SESSION_KEY)
    return
  }

  globalThis.localStorage.setItem(MOCK_SESSION_KEY, JSON.stringify(session))
}

export async function getAuthSession(): Promise<AuthSession | null> {
  if (MOCK_MODE) {
    return readMockSession()
  }

  let extractedToken: string | null = null

  if (typeof window !== 'undefined') {
    // Check URL hash for #access_token=... (standard OAuth fragment)
    if (window.location.hash && window.location.hash.includes('access_token=')) {
      const hashStr = window.location.hash.replace(/^#/, '')
      const params = new URLSearchParams(hashStr)
      const tokenInHash = params.get('access_token')
      if (tokenInHash) {
        extractedToken = tokenInHash
        writeStoredSessionToken(tokenInHash)

        params.delete('access_token')
        const remainingHash = params.toString()
        const newHash = remainingHash ? `#${remainingHash}` : ''
        const newUrl = `${window.location.pathname}${window.location.search}${newHash}`
        window.history.replaceState(null, '', newUrl)
      }
    }

    // Check URL search query for ?access_token=... fallback
    if (!extractedToken && window.location.search && window.location.search.includes('access_token=')) {
      const params = new URLSearchParams(window.location.search)
      const tokenInSearch = params.get('access_token')
      if (tokenInSearch) {
        extractedToken = tokenInSearch
        writeStoredSessionToken(tokenInSearch)

        params.delete('access_token')
        const remainingSearch = params.toString()
        const newSearch = remainingSearch ? `?${remainingSearch}` : ''
        const newUrl = `${window.location.pathname}${newSearch}${window.location.hash}`
        window.history.replaceState(null, '', newUrl)
      }
    }
  }

  const token = extractedToken || getStoredSessionToken()
  if (!token) {
    return null
  }

  try {
    const payload = await appClient.auth.GetSession()
    if (payload?.session?.token) {
      writeStoredSessionToken(payload.session.token)
      return payload as unknown as AuthSession
    } else {
      writeStoredSessionToken(null)
      return null
    }
  } catch (err) {
    writeStoredSessionToken(null)
    return null
  }
}

export async function signInWithDiscord(redirectPath?: string): Promise<void> {
  if (MOCK_MODE) {
    const callbackPath = sanitizeRedirectPath(redirectPath)
    writeMockSession(defaultMockSession)
    window.location.assign(callbackPath)
    return
  }

  const callbackPath = sanitizeRedirectPath(redirectPath)
  const callbackURL = `${window.location.origin}/auth?redirect=${encodeURIComponent(callbackPath)}`

  const payload = await appClient.auth.SignInSocial({
    CallbackURL: callbackURL,
  })

  if (!payload.redirectUrl) {
    throw new Error('Discord sign-in response did not include a redirect URL')
  }

  window.location.assign(payload.redirectUrl)
}

export async function signOut(): Promise<void> {
  if (MOCK_MODE) {
    writeMockSession(null)
    return
  }

  try {
    await appClient.auth.SignOut()
  } catch {
    // Ignore sign-out API network errors, always clear local token
  } finally {
    writeStoredSessionToken(null)
  }
}

export function isMockMode() {
  return MOCK_MODE
}

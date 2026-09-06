import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { getAuthSession, signInWithDiscord, signOut } from './auth'
import { getStoredSessionToken, writeStoredSessionToken, appClient } from './api'

function createLocalStorageMock() {
  let store: Record<string, string> = {}
  return {
    getItem: (key: string) => store[key] ?? null,
    setItem: (key: string, value: string) => { store[key] = value },
    removeItem: (key: string) => { delete store[key] },
    clear: () => { store = {} },
  }
}

describe('Frontend Auth Module (localStorage & Token Auth)', () => {
  const localStorageMock = createLocalStorageMock()

  beforeEach(() => {
    localStorageMock.clear()
    vi.stubGlobal('localStorage', localStorageMock)
    vi.stubGlobal('window', {
      location: {
        origin: 'http://localhost:5173',
        pathname: '/auth',
        search: '',
        hash: '',
        assign: vi.fn(),
      },
      history: {
        replaceState: vi.fn(),
      },
      localStorage: localStorageMock,
    })
    vi.restoreAllMocks()
  })

  afterEach(() => {
    localStorageMock.clear()
    vi.unstubAllGlobals()
  })

  it('stores and retrieves session token in localStorage', () => {
    expect(getStoredSessionToken()).toBeNull()

    writeStoredSessionToken('test-token-123')
    expect(getStoredSessionToken()).toBe('test-token-123')

    writeStoredSessionToken(null)
    expect(getStoredSessionToken()).toBeNull()
  })

  it('extracts access_token from URL hash and cleans up URL history', async () => {
    const replaceStateSpy = vi.fn()

    vi.stubGlobal('window', {
      location: {
        origin: 'http://localhost:5173',
        pathname: '/auth',
        search: '?redirect=/events',
        hash: '#access_token=oauth-hash-token-456',
        assign: vi.fn(),
      },
      history: {
        replaceState: replaceStateSpy,
      },
      localStorage: localStorageMock,
    })

    const mockGetSession = vi.spyOn(appClient.auth, 'GetSession').mockResolvedValue({
      session: {
        token: 'oauth-hash-token-456',
        expiresAt: new Date().toISOString(),
      },
      user: {
        id: 'user-1',
        name: 'Test User',
        email: 'test@example.com',
        slug: 'test-user',
        image: null,
        biography: '',
        careerOverview: '',
        vrchatUsername: 'TestVRChat',
        classTier: 'OP',
        siteRole: 'USER',
        teams: [],
        createdAt: new Date().toISOString(),
        updatedAt: new Date().toISOString(),
      },
    } as any)

    const session = await getAuthSession()

    expect(getStoredSessionToken()).toBe('oauth-hash-token-456')
    expect(replaceStateSpy).toHaveBeenCalledWith(null, '', '/auth?redirect=/events')
    expect(mockGetSession).toHaveBeenCalled()
    expect(session?.user.name).toBe('Test User')
  })

  it('returns null and clears token when GetSession fails with 401', async () => {
    writeStoredSessionToken('invalid-or-expired-token')

    vi.spyOn(appClient.auth, 'GetSession').mockRejectedValue(new Error('Unauthenticated'))

    const session = await getAuthSession()

    expect(session).toBeNull()
    expect(getStoredSessionToken()).toBeNull()
  })

  it('starts Discord sign-in by calling appClient.auth.SignInSocial', async () => {
    const mockAssign = vi.fn()

    vi.stubGlobal('window', {
      location: {
        origin: 'http://localhost:5173',
        pathname: '/auth',
        search: '',
        hash: '',
        assign: mockAssign,
      },
      history: {
        replaceState: vi.fn(),
      },
      localStorage: localStorageMock,
    })

    const signInSpy = vi.spyOn(appClient.auth, 'SignInSocial').mockResolvedValue({
      redirectUrl: 'https://discord.com/oauth2/authorize?client_id=123',
    })

    await signInWithDiscord('/admin')

    expect(signInSpy).toHaveBeenCalledWith({
      CallbackURL: 'http://localhost:5173/auth?redirect=%2Fadmin',
    })
    expect(mockAssign).toHaveBeenCalledWith('https://discord.com/oauth2/authorize?client_id=123')
  })

  it('signs out by calling appClient.auth.SignOut and clearing localStorage', async () => {
    writeStoredSessionToken('active-token-999')

    const signOutSpy = vi.spyOn(appClient.auth, 'SignOut').mockResolvedValue(undefined as any)

    await signOut()

    expect(signOutSpy).toHaveBeenCalled()
    expect(getStoredSessionToken()).toBeNull()
  })
})

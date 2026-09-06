import { appClient } from './api'
import { MOCK_MODE } from './mock-mode'
import type { auth, eventmanager } from './client'

const now = new Date().toISOString()

// Mock user profile map for public API mock mode
const mockUserProfileMap = new Map<string, auth.UserProfile>([
  ['mock-admin-1', {
    id: 'mock-admin-1',
    name: 'Mock Admin',
    slug: 'mock-admin',
    email: 'mock-admin@lightwing.local',
    image: null,
    biography: 'Local mock administrator account for frontend-only testing.',
    careerOverview: 'Testing dashboards and admin workflows.',
    vrchatUsername: null,
    classTier: null,
    siteRole: 'SITE_ADMIN',
    teams: [],
    createdAt: now,
    updatedAt: now,
  }],
  ['mock-user-1', {
    id: 'mock-user-1',
    name: 'Thunder Bolt',
    slug: 'thunder-bolt',
    email: 'bolt@lightwing.local',
    image: null,
    biography: 'A rapid competitor on the turf.',
    careerOverview: 'Sprinting specialist.',
    vrchatUsername: 'Bolt',
    classTier: 'OP',
    siteRole: 'USER',
    teams: [],
    createdAt: now,
    updatedAt: now,
  }],
  ['mock-user-2', {
    id: 'mock-user-2',
    name: 'Shadow Runner',
    slug: 'shadow-runner',
    email: 'shadow@lightwing.local',
    image: null,
    biography: 'Silent but swift.',
    careerOverview: 'Distance running.',
    vrchatUsername: 'Shadow',
    classTier: 'G3',
    siteRole: 'USER',
    teams: [],
    createdAt: now,
    updatedAt: now,
  }],
] as unknown as [string, auth.UserProfile][])

const mockRaceMembersMap = new Map<string, eventmanager.RaceEventMemberView[]>([
  [
    'race_mock_001',
    [
      { userId: 'mock-user-1', name: 'Bolt', classTier: 'OP' },
      { userId: 'mock-user-2', name: 'Shadow', classTier: 'G3' },
    ],
  ],
  [
    'race_mock_002',
    [
      { userId: 'mock-user-1', name: 'Bolt', classTier: 'OP' },
      { userId: 'mock-user-2', name: 'Shadow', classTier: 'G3' },
    ],
  ],
])

// Mock results for the mock races
const mockRaceResults = {
  'race_mock_001': [
    {
      id: 'res_mock_001',
      raceEventId: 'race_mock_001',
      userId: 'mock-user-1',
      position: 1,
      points: 10,
      gateNumber: 3,
      finishTime: '1:08.5',
      margin: null,
      passingOrder: '2-2-1',
      final3F: '34.2',
      resultStatus: null,
      createdAt: now,
      updatedAt: now,
    },
    {
      id: 'res_mock_002',
      raceEventId: 'race_mock_001',
      userId: 'mock-user-2',
      position: 2,
      points: 6,
      gateNumber: 5,
      finishTime: '1:08.7',
      margin: '1 1/4',
      passingOrder: '1-1-2',
      final3F: '34.6',
      resultStatus: null,
      createdAt: now,
      updatedAt: now,
    },
  ],
  'race_mock_002': [
    {
      id: 'res_mock_003',
      raceEventId: 'race_mock_002',
      userId: 'mock-user-2',
      position: 1,
      points: 10,
      gateNumber: 2,
      finishTime: '1:37.2',
      margin: null,
      passingOrder: '4-3-1',
      final3F: '36.1',
      resultStatus: null,
      createdAt: now,
      updatedAt: now,
    },
    {
      id: 'res_mock_004',
      raceEventId: 'race_mock_002',
      userId: 'mock-user-1',
      position: null,
      points: 0,
      gateNumber: 8,
      finishTime: null,
      margin: null,
      passingOrder: null,
      final3F: null,
      resultStatus: 'DEFERRED',
      createdAt: now,
      updatedAt: now,
    },
  ],
} as unknown as Record<string, eventmanager.RaceResultView[]>

const LOCAL_STORAGE_KEY = 'lightwing:mock:public_events'

function loadMockEvents(): eventmanager.EventDetail[] {
  if (typeof globalThis !== 'undefined' && globalThis.localStorage) {
    const stored = globalThis.localStorage.getItem(LOCAL_STORAGE_KEY)
    if (stored) {
      try {
        return JSON.parse(stored)
      } catch {
        // fallback
      }
    }
  }
  return [
    {
      id: 'evt_mock_001',
      name: 'Summer Sprint Invitational',
      description: 'Mock event used for public UI layout testing.',
      ownerType: 'ORGANIZATION',
      organizationId: 'org_mock_urs',
      ownerUserId: null,
      status: 'PENDING',
      tag: 'OFFICIAL',
      scoringType: 1,
      scoringTypeLabel: 'points-based',
      scoringRulesMode: 'STANDARD',
      customScoringTables: null,
      classRestriction: 'OP',
      granularParticipation: true,
      signupsLocked: false,
      scheduledAt: null,
      participantLimit: null,
      maxConcurrentRaceParticipations: null,
      raceEvents: [
        {
          id: 'race_mock_001',
          eventId: 'evt_mock_001',
          name: 'Summer Sprint Turf',
          sequence: 1,
          distanceMeters: 1200,
          trackType: 'Turf',
          location: 'Kyoto Racecourse',
          scoringType: 1,
          classRestriction: 'OP',
          startsAt: now,
          endsAt: now,
          participantLimit: null,
          createdAt: now,
          updatedAt: now,
          members: [
            { userId: 'mock-user-1', name: 'Bolt', classTier: 'OP' },
            { userId: 'mock-user-2', name: 'Shadow', classTier: 'G3' },
          ],
        } as any,
        {
          id: 'race_mock_002',
          eventId: 'evt_mock_001',
          name: 'Summer Sprint Dirt',
          sequence: 2,
          distanceMeters: 1600,
          trackType: 'Dirt',
          location: 'Hanshin Racecourse',
          scoringType: 1,
          classRestriction: null,
          startsAt: now,
          endsAt: now,
          participantLimit: null,
          createdAt: now,
          updatedAt: now,
          members: [
            { userId: 'mock-user-1', name: 'Bolt', classTier: 'OP' },
            { userId: 'mock-user-2', name: 'Shadow', classTier: 'G3' },
          ],
        } as any,
      ],
      members: [
        { userId: 'mock-user-1', name: 'Bolt', classTier: 'OP' },
        { userId: 'mock-user-2', name: 'Shadow', classTier: 'G3' },
      ],
      schedules: [],
      pointsOverview: [
        { userId: 'mock-user-1', name: 'Bolt', points: 10, resultStatus: null },
        { userId: 'mock-user-2', name: 'Shadow', points: 6, resultStatus: null },
        { userId: 'mock-user-3', name: 'Galloper', points: 0, resultStatus: 'DNF' },
      ],
      ladderOverview: null,
      createdAt: now,
      updatedAt: now,
    },
    {
      id: 'evt_mock_003',
      name: 'Archived Championship',
      description: 'A concluded event visible to the public.',
      ownerType: 'USER',
      organizationId: null,
      ownerUserId: 'mock-user-2',
      status: 'CONCLUDED',
      scoringType: 2,
      scoringTypeLabel: 'ladder-elo',
      scoringRulesMode: null,
      customScoringTables: null,
      classRestriction: null,
      granularParticipation: false,
      signupsLocked: false,
      scheduledAt: null,
      participantLimit: null,
      maxConcurrentRaceParticipations: null,
      raceEvents: [],
      members: [],
      schedules: [],
      pointsOverview: null,
      ladderOverview: [],
      createdAt: now,
      updatedAt: now,
    },
  ] as unknown as eventmanager.EventDetail[]
}

export function saveMockEvents(events: eventmanager.EventDetail[]) {
  console.log("saveMockEvents called w/ length:", events.length);
  if (typeof window !== 'undefined' && window.localStorage) {
    console.log("Saving mock events to localStorage:", LOCAL_STORAGE_KEY);
    window.localStorage.setItem(LOCAL_STORAGE_KEY, JSON.stringify(events))
  } else {
    console.log("window.localStorage not available!");
  }
}

// Mock events for public API - excluding DRAFT events
export let mockPublicEvents: eventmanager.EventDetail[] = loadMockEvents()

function getCurrentMockUserId(): string | null {
  if (!MOCK_MODE) return null
  const stored = globalThis.localStorage.getItem('lightwing:mock:session')
  if (!stored) return null
  try {
    const session = JSON.parse(stored) as { user?: { id: string } }
    return session.user?.id ?? null
  } catch {
    return null
  }
}

export async function listPublicEvents(
  limit?: number,
  offset?: number,
): Promise<{ events: eventmanager.EventListItem[]; total: number }> {
  if (!MOCK_MODE) {
    return appClient.eventmanager.ListPublicEvents({
      Limit: limit ?? 0,
      Offset: offset ?? 0,
    })
  }
  mockPublicEvents = loadMockEvents()

  let events = mockPublicEvents.map((e) => ({
    id: e.id,
    name: e.name,
    description: e.description,
    ownerType: e.ownerType,
    organizationId: e.organizationId,
    ownerUserId: e.ownerUserId,
    status: e.status,
    tag: e.tag ?? 'OFFICIAL',
    deletedAt: e.deletedAt ?? null,
    scoringType: e.scoringType,
    scoringTypeLabel: e.scoringTypeLabel,
    classRestriction: e.classRestriction,
    granularParticipation: e.granularParticipation,
    signupsLocked: e.signupsLocked,
    scheduledAt: e.scheduledAt,
    participantLimit: e.participantLimit,
    maxConcurrentRaceParticipations: e.maxConcurrentRaceParticipations,
    raceCount: e.raceEvents.length,
    memberCount: e.members.length,
    createdAt: e.createdAt,
    updatedAt: e.updatedAt,
  }))

  const total = events.length
  if (offset !== undefined) {
    events = events.slice(offset)
  }
  if (limit !== undefined) {
    events = events.slice(0, limit)
  }

  return { events, total }
}

function isEligible(
  participantTier: string | null,
  eventRestriction: string | null,
): boolean {
  // PRE_OP and OP are treated as equivalent to unrestricted / none (null)
  const normParticipant = (participantTier === 'PRE_OP' || participantTier === 'OP') ? null : participantTier
  const normRestriction = (eventRestriction === 'PRE_OP' || eventRestriction === 'OP') ? null : eventRestriction

  if (normRestriction === null) {
    return true
  }
  if (normParticipant === null) {
    return false
  }
  if (normParticipant === normRestriction) {
    return true
  }
  const order = ['PRE_OP', 'OP', 'G3', 'G2', 'G1']
  const pIdx = order.indexOf(normParticipant)
  const rIdx = order.indexOf(normRestriction)
  if (pIdx === -1 || rIdx === -1) {
    return false
  }
  return rIdx === pIdx - 1
}

export async function getPublicEvent(eventId: string): Promise<eventmanager.EventDetail> {
  if (!MOCK_MODE) {
    return appClient.eventmanager.GetEvent(eventId)
  }
  mockPublicEvents = loadMockEvents()
  const event = mockPublicEvents.find((e) => e.id === eventId)
  if (!event) throw new Error('Event not found')
  return event
}

export async function joinEvent(
  eventId: string,
  authorization: string,
): Promise<eventmanager.EventDetail> {
  if (!MOCK_MODE) {
    return appClient.eventmanager.JoinEvent({ eventId, Authorization: authorization })
  }

  mockPublicEvents = loadMockEvents()
  const eventIndex = mockPublicEvents.findIndex((e) => e.id === eventId)
  if (eventIndex === -1) throw new Error('Event not found')

  const event = mockPublicEvents[eventIndex]
  if (event.status !== 'PENDING' && event.status !== 'ONGOING') {
    throw new Error('Event is not open for public signup')
  }

  if (event.signupsLocked) {
    throw new Error('Signups are locked for this event')
  }

  const userId = getCurrentMockUserId()
  if (!userId) throw new Error('Not authenticated')

  const user = mockUserProfileMap.get(userId)
  if (!user) throw new Error('User not found')

  const eventRestriction = event.classRestriction
  const userTier = user.classTier
  if (!isEligible(userTier, eventRestriction)) {
    throw new Error('Participant class tier does not satisfy the event class restriction')
  }

  const isAlreadyMember = event.members.some((m) => m.userId === userId)
  if (!isAlreadyMember) {
    mockPublicEvents[eventIndex] = {
      ...event,
      members: [...event.members, { userId, name: user.vrchatUsername ?? user.name, classTier: userTier }],
    }
  }

  saveMockEvents(mockPublicEvents)
  return mockPublicEvents[eventIndex]
}

export async function leaveEvent(
  eventId: string,
  authorization: string,
): Promise<eventmanager.EventDetail> {
  if (!MOCK_MODE) {
    return appClient.eventmanager.LeaveEvent({ EventID: eventId, Authorization: authorization })
  }

  mockPublicEvents = loadMockEvents()
  const eventIndex = mockPublicEvents.findIndex((e) => e.id === eventId)
  if (eventIndex === -1) throw new Error('Event not found')

  const event = mockPublicEvents[eventIndex]
  if (event.signupsLocked) {
    throw new Error('Signups are locked for this event')
  }

  const userId = getCurrentMockUserId()
  if (!userId) throw new Error('Not authenticated')

  mockPublicEvents[eventIndex] = {
    ...event,
    members: event.members.filter((m) => m.userId !== userId),
  }

  saveMockEvents(mockPublicEvents)
  return mockPublicEvents[eventIndex]
}

export async function getMyProfile(userId: string): Promise<auth.UserProfile> {
  if (!MOCK_MODE) {
    return appClient.auth.GetUser(userId)
  }
  const user = mockUserProfileMap.get(userId)
  if (!user) throw new Error('User not found')
  return user
}

export async function updateMyProfile(
  userId: string,
  params: {
    name?: string
    slug?: string
    biography?: string | null
    careerOverview?: string | null
    vrchatUsername?: string | null
  },
  authorization: string,
): Promise<auth.UserProfile> {
  if (!MOCK_MODE) {
    return appClient.auth.UpdateUser({
      id: userId,
      Authorization: authorization,
      name: params.name,
      slug: params.slug,
      biography: params.biography,
      careerOverview: params.careerOverview,
      vrchatUsername: params.vrchatUsername,
    } as unknown as auth.UpdateUserRequest)
  }

  const existing = mockUserProfileMap.get(userId)
  if (!existing) throw new Error('User not found')

  const updated: auth.UserProfile = {
    ...existing,
    name: params.name ?? (existing.vrchatUsername ?? existing.name),
    slug: params.slug ?? existing.slug,
    biography: params.biography ?? existing.biography,
    careerOverview: params.careerOverview ?? existing.careerOverview,
    vrchatUsername: params.vrchatUsername ?? existing.vrchatUsername,
    updatedAt: new Date().toISOString(),
  }

  mockUserProfileMap.set(userId, updated)

  const stored = globalThis.localStorage.getItem('lightwing:mock:session')
  if (stored && params.vrchatUsername !== undefined) {
    try {
      const session = JSON.parse(stored) as any
      session.user.vrchatUsername = params.vrchatUsername
      globalThis.localStorage.setItem('lightwing:mock:session', JSON.stringify(session))
    } catch {
      // ignore
    }
  }

  return updated
}

export async function listPublicRaceEvents(
  eventId: string,
): Promise<{ races: eventmanager.RaceEventDetail[] }> {
  if (!MOCK_MODE) {
    return appClient.eventmanager.ListRaceEvents({ EventID: eventId })
  }
  mockPublicEvents = loadMockEvents()
  const event = mockPublicEvents.find((e) => e.id === eventId)
  if (!event) throw new Error('Event not found')
  const races = (event.raceEvents ?? []).map((r) => ({
    ...r,
    members: mockRaceMembersMap.get(r.id) ?? [],
  })) as eventmanager.RaceEventDetail[]
  return { races }
}

export async function getPublicRaceResults(
  eventId: string,
  raceId: string,
): Promise<{ results: eventmanager.RaceResultView[] }> {
  if (!MOCK_MODE) {
    return appClient.eventmanager.ListRaceResults({ EventID: eventId, RaceID: raceId })
  }
  let results = mockRaceResults[raceId] ?? []
  if (typeof window !== 'undefined' && window.localStorage) {
    const stored = window.localStorage.getItem('lightwing:mock:race_results')
    if (stored) {
      try {
        const parsed = JSON.parse(stored)
        if (parsed[raceId]) {
          results = parsed[raceId]
        }
      } catch {
        // ignore
      }
    }
  }
  return { results }
}

export async function getPublicRaceEvent(
  eventId: string,
  raceId: string,
): Promise<eventmanager.RaceEventDetail> {
  if (!MOCK_MODE) {
    return appClient.eventmanager.GetRaceEvent({ EventID: eventId, RaceID: raceId })
  }
  mockPublicEvents = loadMockEvents()
  const event = mockPublicEvents.find((e) => e.id === eventId)
  if (!event) throw new Error('Event not found')
  const race = (event.raceEvents as eventmanager.RaceEventDetail[] ?? []).find((r) => r.id === raceId)
  if (!race) throw new Error('Race not found')
  return {
    ...race,
    members: mockRaceMembersMap.get(raceId) ?? [],
  }
}

export async function joinRaceEvent(
  eventId: string,
  raceId: string,
  authorization: string,
): Promise<eventmanager.RaceEventDetail> {
  if (!MOCK_MODE) {
    return appClient.eventmanager.JoinRaceEvent({ eventId, raceId, Authorization: authorization })
  }

  mockPublicEvents = loadMockEvents()
  const eventIndex = mockPublicEvents.findIndex((e) => e.id === eventId)
  if (eventIndex === -1) throw new Error('Event not found')

  const event = mockPublicEvents[eventIndex]
  if (event.signupsLocked) {
    throw new Error('Signups are locked for this event')
  }

  const raceIndex = event.raceEvents.findIndex((r) => r.id === raceId)
  if (raceIndex === -1) throw new Error('Race not found')

  const userId = getCurrentMockUserId()
  if (!userId) throw new Error('Not authenticated')

  const user = mockUserProfileMap.get(userId)
  if (!user) throw new Error('User not found')

  // Require user to be event member first unless it's a granular participation event
  const isEventMember = event.members.some((m) => m.userId === userId)
  if (!isEventMember) {
    if (!event.granularParticipation) {
      throw new Error('User is not a member of this event')
    }
    // Auto-enroll user as event member
    event.members = [...event.members, { userId, name: user.vrchatUsername ?? user.name, classTier: user.classTier }]
  }

  const targetRestriction = event.raceEvents[raceIndex].classRestriction ?? event.classRestriction
  const userTier = user.classTier

  if (!isEligible(userTier, targetRestriction)) {
    throw new Error('Participant class tier does not satisfy the race class restriction')
  }

  let raceMembers = mockRaceMembersMap.get(raceId) ?? []
  if (!raceMembers.some((m) => m.userId === userId)) {
    raceMembers = [...raceMembers, { userId, name: user.vrchatUsername ?? user.name, classTier: userTier }]
    mockRaceMembersMap.set(raceId, raceMembers)
  }

  const joinedRace = {
    ...event.raceEvents[raceIndex],
    members: raceMembers,
  } as any

  event.raceEvents[raceIndex] = joinedRace

  saveMockEvents(mockPublicEvents)
  return joinedRace
}

export async function leaveRaceEvent(
  eventId: string,
  raceId: string,
  authorization: string,
): Promise<eventmanager.RaceEventDetail> {
  if (!MOCK_MODE) {
    return appClient.eventmanager.LeaveRaceEvent({ eventId, raceId, Authorization: authorization })
  }

  mockPublicEvents = loadMockEvents()
  const eventIndex = mockPublicEvents.findIndex((e) => e.id === eventId)
  if (eventIndex === -1) throw new Error('Event not found')

  const event = mockPublicEvents[eventIndex]
  if (event.signupsLocked) {
    throw new Error('Signups are locked for this event')
  }

  const raceIndex = event.raceEvents.findIndex((r) => r.id === raceId)
  if (raceIndex === -1) throw new Error('Race not found')

  const userId = getCurrentMockUserId()
  if (!userId) throw new Error('Not authenticated')

  let raceMembers = mockRaceMembersMap.get(raceId) ?? []
  raceMembers = raceMembers.filter((m) => m.userId !== userId)
  mockRaceMembersMap.set(raceId, raceMembers)

  const updatedRace = {
    ...event.raceEvents[raceIndex],
    members: raceMembers,
  } as any

  event.raceEvents[raceIndex] = updatedRace

  if (event.granularParticipation) {
    const userHasOtherRaces = event.raceEvents.some((r, idx) => {
      if (idx === raceIndex) return false
      const members = mockRaceMembersMap.get(r.id) ?? []
      return members.some((m) => m.userId === userId)
    })
    if (!userHasOtherRaces) {
      event.members = event.members.filter((m) => m.userId !== userId)
    }
  }

  saveMockEvents(mockPublicEvents)
  return updatedRace
}

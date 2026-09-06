import { appClient } from './api'
import { MOCK_MODE } from './mock-mode'
import type { auth, eventmanager, teammanager } from './client'
import type { ClassTier, EventStatus, EventTag, EventOwnerType, SiteRole } from '../types'

const now = new Date().toISOString()

// Rich initial mock members
const mockEventMembers: eventmanager.EventMemberView[] = [
  { userId: 'mock-user-1', name: 'Thunder Bolt', classTier: 'OP' },
  { userId: 'mock-user-2', name: 'Shadow Runner', classTier: 'G3' },
  { userId: 'mock-user-3', name: 'Swift Galloper', classTier: 'G1' },
]

// Rich initial mock race events
const mockRaceEventsList = [
  {
    id: 'race_mock_101',
    name: 'Inaugural Mile Sprint',
    sequence: 1,
    distanceMeters: 1600,
    trackType: 'Turf',
    location: 'Kyoto Racecourse',
    scoringType: 1,
    grade: 'OP',
    classRestriction: 'OP',
    startsAt: null,
    endsAt: null,
    participantLimit: null,
    members: [],
  },
  {
    id: 'race_mock_102',
    name: 'Derby Classic',
    sequence: 2,
    distanceMeters: 2400,
    trackType: 'Dirt',
    location: 'Tokyo Racecourse',
    scoringType: 1,
    grade: 'GIII',
    classRestriction: 'G1',
    startsAt: now,
    endsAt: null, // Ongoing
    participantLimit: null,
    members: [
      { userId: 'mock-user-1', name: 'Thunder Bolt', classTier: 'OP' },
      { userId: 'mock-user-2', name: 'Shadow Runner', classTier: 'G3' },
    ],
  },
] as unknown as (eventmanager.RaceEventView & { members: eventmanager.RaceEventMemberView[] })[]

// Rich initial mock race results.
// Field mapping: gateNumber=Draw, finishTime=Finish Time,
// margin=Distance Behind, passingOrder=Passing Order, final3F=Final 3F.
// Key is raceId
let mockRaceResultsMap = new Map<string, eventmanager.RaceResultView[]>(([
  [
    'race_mock_102',
    [
      {
        id: 'res_mock_201',
        raceEventId: 'race_mock_102',
        userId: 'mock-user-1',
        position: 1,
        points: 10,
        gateNumber: 4,
        finishTime: '1:32.1',
        margin: '—',
        passingOrder: '3-2-1',
        final3F: '34.5',
        resultStatus: null,
        createdAt: now,
        updatedAt: now,
      },
      {
        id: 'res_mock_202',
        raceEventId: 'race_mock_102',
        userId: 'mock-user-2',
        position: 2,
        points: 6,
        gateNumber: 7,
        finishTime: '1:32.4',
        margin: '2 lengths',
        passingOrder: '1-1-2',
        final3F: '35.0',
        resultStatus: null,
        createdAt: now,
        updatedAt: now,
      },
    ],
  ],
] as unknown as [string, eventmanager.RaceResultView[]][]))



const RESULTS_LOCAL_STORAGE_KEY = 'lightwing:mock:race_results'

function saveMockResults() {
  if (typeof window !== 'undefined' && window.localStorage) {
    const obj: Record<string, eventmanager.RaceResultView[]> = {}
    mockRaceResultsMap.forEach((val, key) => {
      obj[key] = val
    })
    window.localStorage.setItem(RESULTS_LOCAL_STORAGE_KEY, JSON.stringify(obj))
  }
}

function loadMockResults() {
  if (typeof window !== 'undefined' && window.localStorage) {
    const stored = window.localStorage.getItem(RESULTS_LOCAL_STORAGE_KEY)
    if (stored) {
      try {
        const parsed = JSON.parse(stored)
        for (const [key, val] of Object.entries(parsed)) {
          mockRaceResultsMap.set(key, val as eventmanager.RaceResultView[])
        }
      } catch {
        // ignore
      }
    }
  }
}

let mockRaceMembersMap = new Map<string, eventmanager.RaceEventMemberView[]>([
  [
    'race_mock_102',
    [
      { userId: 'mock-user-1', name: 'Thunder Bolt', classTier: 'OP' },
      { userId: 'mock-user-2', name: 'Shadow Runner', classTier: 'G3' },
    ],
  ],
])

let mockEvents = [
  {
    id: 'evt_mock_001',
    name: 'Summer Sprint Invitational',
    description: 'Mock event used for admin UI layout testing.',
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
    raceEvents: mockRaceEventsList,
    members: mockEventMembers,
    schedules: [],
    pointsOverview: [
      { userId: 'mock-user-1', name: 'Thunder Bolt', points: 10, resultStatus: null },
      { userId: 'mock-user-2', name: 'Shadow Runner', points: 6, resultStatus: null },
      { userId: 'mock-user-3', name: 'Swift Galloper', points: 0, resultStatus: null },
    ],
    ladderOverview: null,
    createdAt: now,
    updatedAt: now,
  },
  {
    id: 'evt_mock_002',
    name: 'Mock Championship Finals',
    description: 'Finals preview event for dashboard prototyping.',
    ownerType: 'USER',
    organizationId: null,
    ownerUserId: 'mock-admin-1',
    status: 'DRAFT',
    tag: 'COMMUNITY',
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
    members: [
      { userId: 'mock-user-1', name: 'Thunder Bolt', classTier: 'OP' },
      { userId: 'mock-user-2', name: 'Shadow Runner', classTier: 'G3' },
    ],
    schedules: [],
    pointsOverview: null,
    ladderOverview: [
      { userId: 'mock-user-1', name: 'Thunder Bolt', elo: 1200, wins: 0, losses: 0, rank: 1 },
      { userId: 'mock-user-2', name: 'Shadow Runner', elo: 1200, wins: 0, losses: 0, rank: 2 },
    ],
    createdAt: now,
    updatedAt: now,
  },
] as unknown as eventmanager.EventDetail[]

let mockTeamsList = [
  {
    id: 'org_mock_urs',
    name: 'URS Mock Team',
    slug: 'urs-mock-team',
    logo: null,
    stats: {
      rankingAverage: 4.5,
      pointsAverage: 88.0,
      seasonRank: 2,
      averagePointsPerEvent: 12.5,
    },
    administratorSlotsRemaining: 2,
    members: [
      { userId: 'mock-admin-1', name: 'Mock Admin', role: 'administrator' },
      { userId: 'mock-user-1', name: 'Thunder Bolt', role: 'member' },
    ],
  }
] as unknown as teammanager.Team[]

const mockUserProfiles = new Map<string, auth.UserProfile>([
  [
    'mock-admin-1',
    {
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
      teams: [
        {
          organizationId: 'org_mock_urs',
          name: 'URS Mock Team',
          slug: 'urs-mock-team',
          role: 'administrator',
        },
      ],
      createdAt: now,
      updatedAt: now,
    },
  ],
  [
    'mock-user-1',
    {
      id: 'mock-user-1',
      name: 'Thunder Bolt',
      slug: 'thunder-bolt',
      email: 'bolt@lightwing.local',
      image: null,
      biography: 'A rapid competitor on the turf.',
      careerOverview: 'Sprinting specialist.',
      vrchatUsername: null,
      classTier: 'OP',
      siteRole: 'USER',
      teams: [],
      createdAt: now,
      updatedAt: now,
    },
  ],
  [
    'mock-user-2',
    {
      id: 'mock-user-2',
      name: 'Shadow Runner',
      slug: 'shadow-runner',
      email: 'shadow@lightwing.local',
      image: null,
      biography: 'Loves the dirt track racing.',
      careerOverview: 'Dirt mile runs.',
      vrchatUsername: null,
      classTier: 'G3',
      siteRole: 'USER',
      teams: [],
      createdAt: now,
      updatedAt: now,
    },
  ],
  [
    'mock-user-3',
    {
      id: 'mock-user-3',
      name: 'Swift Galloper',
      slug: 'swift-galloper',
      email: 'gallop@lightwing.local',
      image: null,
      biography: 'A high tier challenger.',
      careerOverview: 'Championship titles in major races.',
      vrchatUsername: null,
      classTier: 'G1',
      siteRole: 'USER',
      teams: [],
      createdAt: now,
      updatedAt: now,
    },
  ],
] as unknown as [string, auth.UserProfile][])

export function hydrateEventRaceEvents(event: eventmanager.EventDetail): eventmanager.EventDetail {
  const hydratedRaces = event.raceEvents.map((race) => ({
    ...race,
    members: mockRaceMembersMap.get(race.id) ?? [],
  }))
  return {
    ...event,
    raceEvents: hydratedRaces as any,
  }
}

export function hydrateRaceEvent(eventId: string, race: eventmanager.RaceEventView): eventmanager.RaceEventDetail {
  return {
    id: race.id,
    eventId,
    name: race.name,
    sequence: race.sequence,
    distanceMeters: race.distanceMeters,
    trackType: race.trackType,
    location: race.location,
    scoringType: race.scoringType,
    grade: race.grade,
    classRestriction: race.classRestriction,
    startsAt: race.startsAt,
    endsAt: race.endsAt,
    participantLimit: race.participantLimit ?? null,
    createdAt: now,
    updatedAt: now,
    members: mockRaceMembersMap.get(race.id) ?? [],
  }
}

export async function listAdminEvents(
  organizationId?: string,
  classRestriction?: ClassTier,
  limit?: number,
  offset?: number,
  status?: EventStatus,
  tag?: EventTag,
  includeDeleted?: boolean,
): Promise<{ events: eventmanager.EventListItem[]; total: number }> {
  if (!MOCK_MODE) {
    return appClient.eventmanager.ListEvents({
      OrganizationID: organizationId ?? '',
      ClassRestriction: classRestriction ?? '',
      Status: status ?? '',
      Tag: tag ?? '',
      IncludeDeleted: includeDeleted ?? false,
      Limit: limit ?? 0,
      Offset: offset ?? 0,
    })
  }

  let events = mockEvents.map((e) => ({
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

  if (organizationId) {
    events = events.filter((e) => e.organizationId === organizationId)
  }
  if (classRestriction) {
    events = events.filter((e) => e.classRestriction === classRestriction)
  }
  if (status) {
    events = events.filter((e) => e.status === status)
  } else if (!includeDeleted) {
    events = events.filter((e) => e.status !== 'PENDING_DELETION')
  }
  if (tag) {
    events = events.filter((e) => e.tag === tag)
  }

  const total = events.length
  if (offset !== undefined) {
    events = events.slice(offset)
  }
  if (limit !== undefined) {
    events = events.slice(0, limit)
  }

  return { events, total }
}

export async function getAdminEvent(eventId: string): Promise<eventmanager.EventDetail> {
  if (!MOCK_MODE) {
    return appClient.eventmanager.GetEvent(eventId)
  }

  const stored = typeof window !== 'undefined' && window.localStorage ? window.localStorage.getItem('lightwing:mock:public_events') : null
  let event: eventmanager.EventDetail | undefined
  if (stored) {
    try {
      const publicEvents = JSON.parse(stored)
      event = publicEvents.find((evt: any) => evt.id === eventId)
    } catch {
      // ignore
    }
  }

  if (!event) {
    event = mockEvents.find((evt) => evt.id === eventId)
  }

  if (!event) {
    throw new Error('Mock event not found')
  }
  return hydrateEventRaceEvents(event)
}

export async function updateAdminEventStatus(
  eventId: string,
  params: { status?: EventStatus; tag?: EventTag },
  authorization: string,
): Promise<eventmanager.EventDetail> {
  if (!MOCK_MODE) {
    return appClient.eventmanager.SetEventStatus({
      id: eventId,
      Authorization: authorization,
      status: params.status,
      tag: params.tag,
    })
  }

  mockEvents = mockEvents.map((event) =>
    event.id === eventId
      ? {
          ...event,
          ...(params.status ? { status: params.status } : {}),
          ...(params.tag ? { tag: params.tag } : {}),
          ...(params.status === 'PENDING_DELETION'
            ? { deletedAt: new Date().toISOString() }
            : params.status
            ? { deletedAt: null }
            : {}),
          updatedAt: new Date().toISOString(),
        }
      : event,
  )

  const updated = mockEvents.find((event) => event.id === eventId)
  if (!updated) {
    throw new Error('Mock event not found')
  }

  return updated
}

export async function deleteAdminEvent(
  eventId: string,
  authorization: string,
  permanent?: boolean,
): Promise<{ deleted: boolean }> {
  if (!MOCK_MODE) {
    return appClient.eventmanager.DeleteEvent({
      ID: eventId,
      Authorization: authorization,
      permanent,
    })
  }

  const existing = mockEvents.find((e) => e.id === eventId)
  if (!existing) {
    throw new Error('Mock event not found')
  }

  if (permanent || existing.status === 'PENDING_DELETION') {
    mockEvents = mockEvents.filter((e) => e.id !== eventId)
  } else {
    mockEvents = mockEvents.map((e) =>
      e.id === eventId
        ? {
            ...e,
            status: 'PENDING_DELETION' as EventStatus,
            deletedAt: new Date().toISOString(),
            updatedAt: new Date().toISOString(),
          }
        : e,
    )
  }

  return { deleted: true }
}

export async function restoreAdminEvent(
  eventId: string,
  authorization: string,
): Promise<eventmanager.EventDetail> {
  if (!MOCK_MODE) {
    return appClient.eventmanager.RestoreEvent({
      id: eventId,
      Authorization: authorization,
    })
  }

  mockEvents = mockEvents.map((e) =>
    e.id === eventId
      ? {
          ...e,
          status: 'PENDING' as EventStatus,
          deletedAt: null,
          updatedAt: new Date().toISOString(),
        }
      : e,
  )

  const updated = mockEvents.find((e) => e.id === eventId)
  if (!updated) {
    throw new Error('Mock event not found')
  }

  return updated
}

export async function setEventSignupsLocked(
  eventId: string,
  locked: boolean,
  authorization: string,
): Promise<eventmanager.EventDetail> {
  console.log("setEventSignupsLocked called", eventId, locked, "MOCK_MODE:", MOCK_MODE);
  if (!MOCK_MODE) {
    return appClient.eventmanager.SetEventSignupsLocked({
      eventId,
      Authorization: authorization,
      locked,
    })
  }

  mockEvents = mockEvents.map((event) =>
    event.id === eventId ? { ...event, signupsLocked: locked, updatedAt: new Date().toISOString() } : event,
  )

  let publicEvents = []
  const stored = typeof window !== 'undefined' && window.localStorage ? window.localStorage.getItem('lightwing:mock:public_events') : null
  if (stored) {
    try {
      publicEvents = JSON.parse(stored)
    } catch {
      // ignore
    }
  } else {
    publicEvents = JSON.parse(JSON.stringify(mockEvents))
  }

  const pubEvtIndex = publicEvents.findIndex((e: any) => e.id === eventId)
  console.log("pubEvtIndex found in mockPublicEvents stored data:", pubEvtIndex);
  if (pubEvtIndex !== -1) {
    publicEvents[pubEvtIndex] = {
      ...publicEvents[pubEvtIndex],
      signupsLocked: locked,
      updatedAt: new Date().toISOString()
    }
    if (typeof window !== 'undefined' && window.localStorage) {
      window.localStorage.setItem('lightwing:mock:public_events', JSON.stringify(publicEvents))
    }
  }

  const updated = mockEvents.find((event) => event.id === eventId)
  if (!updated) {
    throw new Error('Mock event not found')
  }

  return updated
}

export async function updateAdminEvent(
  eventId: string,
  params: {
    name?: string
    description?: string | null
    classRestriction?: ClassTier | null
    scoringRulesMode?: string | null
    customScoringTables?: any | null
    scheduledAt?: string | null
    participantLimit?: number | null
    maxConcurrentRaceParticipations?: number | null
  },
  authorization: string,
): Promise<eventmanager.EventDetail> {
  if (!MOCK_MODE) {
    return appClient.eventmanager.UpdateEvent({
      id: eventId,
      Authorization: authorization,
      name: params.name,
      description: params.description,
      classRestriction: params.classRestriction,
      scoringRulesMode: params.scoringRulesMode,
      customScoringTables: params.customScoringTables,
      scheduledAt: params.scheduledAt,
      participantLimit: params.participantLimit,
      maxConcurrentRaceParticipations: params.maxConcurrentRaceParticipations,
    } as unknown as eventmanager.UpdateEventRequest)
  }

  mockEvents = mockEvents.map((evt) => {
    if (evt.id === eventId) {
      return {
        ...evt,
        name: params.name ?? evt.name,
        description: params.description !== undefined ? params.description : evt.description,
        classRestriction: params.classRestriction !== undefined ? params.classRestriction : evt.classRestriction,
        scoringRulesMode: params.scoringRulesMode !== undefined ? params.scoringRulesMode : evt.scoringRulesMode,
        customScoringTables: params.customScoringTables !== undefined ? params.customScoringTables : evt.customScoringTables,
        scheduledAt: params.scheduledAt !== undefined ? params.scheduledAt : evt.scheduledAt,
        participantLimit: params.participantLimit !== undefined ? params.participantLimit : evt.participantLimit,
        maxConcurrentRaceParticipations: params.maxConcurrentRaceParticipations !== undefined ? params.maxConcurrentRaceParticipations : evt.maxConcurrentRaceParticipations,
        updatedAt: new Date().toISOString(),
      } as unknown as eventmanager.EventDetail
    }
    return evt
  })

  let publicEvents = []
  const stored = typeof window !== 'undefined' && window.localStorage ? window.localStorage.getItem('lightwing:mock:public_events') : null
  if (stored) {
    try {
      publicEvents = JSON.parse(stored)
    } catch {
      // ignore
    }
  } else {
    publicEvents = JSON.parse(JSON.stringify(mockEvents))
  }

  const pubEvtIndex = publicEvents.findIndex((e: any) => e.id === eventId)
  if (pubEvtIndex !== -1) {
    publicEvents[pubEvtIndex] = {
      ...publicEvents[pubEvtIndex],
      name: params.name ?? publicEvents[pubEvtIndex].name,
      description: params.description !== undefined ? params.description : publicEvents[pubEvtIndex].description,
      classRestriction: params.classRestriction !== undefined ? params.classRestriction : publicEvents[pubEvtIndex].classRestriction,
      scoringRulesMode: params.scoringRulesMode !== undefined ? params.scoringRulesMode : publicEvents[pubEvtIndex].scoringRulesMode,
      customScoringTables: params.customScoringTables !== undefined ? params.customScoringTables : publicEvents[pubEvtIndex].customScoringTables,
      updatedAt: new Date().toISOString()
    }
    if (typeof window !== 'undefined' && window.localStorage) {
      window.localStorage.setItem('lightwing:mock:public_events', JSON.stringify(publicEvents))
    }
  }

  return getAdminEvent(eventId)
}

export async function createAdminEvent(
  params: {
    name: string
    description?: string | null
    ownerType: EventOwnerType
    organizationId?: string | null
    ownerUserId?: string | null
    tag?: EventTag | null
    scoringType: number
    classRestriction?: ClassTier | null
    granularParticipation?: boolean
    scoringRulesMode?: string | null
    customScoringTables?: any | null
    scheduledAt?: string | null
    participantLimit?: number | null
    maxConcurrentRaceParticipations?: number | null
  },
  authorization: string,
): Promise<eventmanager.EventDetail> {
  if (!MOCK_MODE) {
    return appClient.eventmanager.CreateEvent({
      Authorization: authorization,
      name: params.name,
      description: params.description ?? null,
      ownerType: params.ownerType,
      organizationId: params.organizationId ?? null,
      ownerUserId: params.ownerUserId ?? null,
      scoringType: params.scoringType as number,
      classRestriction: params.classRestriction ?? null,
      granularParticipation: params.granularParticipation ?? false,
      scoringRulesMode: params.scoringRulesMode ?? null,
      customScoringTables: params.customScoringTables ?? null,
      scheduledAt: params.scheduledAt ?? null,
      participantLimit: params.participantLimit ?? null,
      maxConcurrentRaceParticipations: params.maxConcurrentRaceParticipations ?? null,
    } as unknown as eventmanager.CreateEventRequest)
  }

  const id = `evt_mock_${Math.floor(Math.random() * 10000)}`
  const newEvent = {
    id,
    name: params.name,
    description: params.description ?? null,
    ownerType: params.ownerType,
    organizationId: params.organizationId ?? null,
    ownerUserId: params.ownerUserId ?? null,
    status: 'DRAFT',
    scoringType: params.scoringType,
    scoringTypeLabel: params.scoringType === 1 ? 'points-based' : 'ladder-elo',
    scoringRulesMode: params.scoringType === 1 ? (params.scoringRulesMode ?? 'STANDARD') : null,
    customScoringTables: params.scoringType === 1 ? (params.customScoringTables ?? null) : null,
    classRestriction: params.classRestriction ?? null,
    granularParticipation: params.granularParticipation ?? false,
    signupsLocked: false,
    scheduledAt: params.scheduledAt ?? null,
    participantLimit: params.participantLimit ?? null,
    maxConcurrentRaceParticipations: params.maxConcurrentRaceParticipations ?? null,
    raceEvents: [],
    members: [],
    schedules: [],
    pointsOverview: params.scoringType === 1 ? [] : null,
    ladderOverview: params.scoringType === 2 ? [] : null,
    createdAt: new Date().toISOString(),
    updatedAt: new Date().toISOString(),
  } as unknown as eventmanager.EventDetail

  mockEvents.unshift(newEvent)
  return newEvent
}

// -----------------------------------------------------------------------------
// RACE EVENT METHODS
// -----------------------------------------------------------------------------

export async function listRaceEvents(eventId: string): Promise<{ races: eventmanager.RaceEventDetail[] }> {
  if (!MOCK_MODE) {
    return appClient.eventmanager.ListRaceEvents({ EventID: eventId })
  }

  const event = mockEvents.find((evt) => evt.id === eventId)
  if (!event) {
    throw new Error('Mock event not found')
  }

  const mappedRaces: eventmanager.RaceEventDetail[] = event.raceEvents.map((race) => hydrateRaceEvent(event.id, race))
  return { races: mappedRaces }
}

export async function getRaceEvent(eventId: string, raceId: string): Promise<eventmanager.RaceEventDetail> {
  if (!MOCK_MODE) {
    return appClient.eventmanager.GetRaceEvent({ EventID: eventId, RaceID: raceId })
  }

  const event = mockEvents.find((evt) => evt.id === eventId)
  if (!event) {
    throw new Error('Mock event not found')
  }

  const race = event.raceEvents.find((r) => r.id === raceId)
  if (!race) {
    throw new Error('Mock race not found')
  }

  return hydrateRaceEvent(eventId, race)
}

export async function createRaceEvent(
  eventId: string,
  params: {
    name: string
    sequence?: number
    distanceMeters: number
    trackType: string
    location: string
    startsAt?: string | null
    endsAt?: string | null
    classRestriction?: ClassTier | null
    scoringType?: number | null
    grade?: string | null
    participantLimit?: number | null
  },
  authorization: string,
): Promise<eventmanager.RaceEventDetail> {
  if (!MOCK_MODE) {
    return appClient.eventmanager.CreateRaceEvent({
      eventId,
      Authorization: authorization,
      name: params.name,
      sequence: params.sequence ?? null,
      distanceMeters: params.distanceMeters,
      trackType: params.trackType,
      location: params.location,
      startsAt: params.startsAt ?? null,
      endsAt: params.endsAt ?? null,
      classRestriction: params.classRestriction ?? null,
      scoringType: params.scoringType ?? null,
      grade: params.grade ?? null,
      participantLimit: params.participantLimit ?? null,
    } as unknown as eventmanager.CreateRaceEventRequest)
  }

  const event = mockEvents.find((evt) => evt.id === eventId)
  let nextSeq = 1
  if (event && event.raceEvents.length > 0) {
    const maxSeq = Math.max(...event.raceEvents.map((r) => r.sequence))
    nextSeq = maxSeq + 1
  }

  const id = `race_mock_${Math.floor(Math.random() * 10000)}`
  const newRace = {
    id,
    name: params.name,
    sequence: params.sequence !== undefined ? params.sequence : nextSeq,
    distanceMeters: params.distanceMeters,
    trackType: params.trackType,
    location: params.location,
    scoringType: params.scoringType ?? null,
    grade: params.grade ?? null,
    classRestriction: params.classRestriction ?? null,
    startsAt: params.startsAt ?? null,
    endsAt: params.endsAt ?? null,
    participantLimit: params.participantLimit ?? null,
    members: [],
  } as unknown as eventmanager.RaceEventView

  mockEvents = mockEvents.map((evt) => {
    if (evt.id === eventId) {
      return {
        ...evt,
        raceEvents: [...evt.raceEvents, newRace].sort((a, b) => a.sequence - b.sequence),
      }
    }
    return evt
  })

  return hydrateRaceEvent(eventId, newRace)
}

export async function reorderRaceEvents(
  eventId: string,
  orderedRaceIds: string[],
  authorization: string,
): Promise<{ races: eventmanager.RaceEventDetail[] }> {
  if (!MOCK_MODE) {
    return appClient.eventmanager.ReorderRaceEvents({
      eventId,
      Authorization: authorization,
      orderedRaceIds,
    })
  }

  const eventIndex = mockEvents.findIndex((evt) => evt.id === eventId)
  if (eventIndex === -1) {
    throw new Error('Mock event not found')
  }

  const event = mockEvents[eventIndex]

  const reordered: eventmanager.RaceEventView[] = []
  for (let i = 0; i < orderedRaceIds.length; i++) {
    const raceId = orderedRaceIds[i]
    const found = event.raceEvents.find((r) => r.id === raceId)
    if (found) {
      reordered.push({
        ...found,
        sequence: i + 1,
      })
    }
  }

  mockEvents[eventIndex] = {
    ...event,
    raceEvents: reordered,
  }

  return { races: reordered.map((r) => hydrateRaceEvent(eventId, r)) }
}

export async function updateRaceEvent(
  eventId: string,
  raceId: string,
  params: {
    name?: string
    sequence?: number
    distanceMeters?: number
    trackType?: string
    location?: string
    startsAt?: string | null
    endsAt?: string | null
    classRestriction?: ClassTier | null
    scoringType?: number | null
    grade?: string | null
    participantLimit?: number | null
  },
  authorization: string,
): Promise<eventmanager.RaceEventDetail> {
  if (!MOCK_MODE) {
    return appClient.eventmanager.UpdateRaceEvent({
      eventId,
      raceId,
      Authorization: authorization,
      ...params,
    } as unknown as eventmanager.UpdateRaceEventRequest)
  }

  let updatedRace: eventmanager.RaceEventView | null = null

  mockEvents = mockEvents.map((evt) => {
    if (evt.id === eventId) {
      const updatedRaceEvents = evt.raceEvents.map((race) => {
        if (race.id === raceId) {
          updatedRace = {
            ...race,
            name: params.name ?? race.name,
            sequence: params.sequence ?? race.sequence,
            distanceMeters: params.distanceMeters ?? race.distanceMeters,
            trackType: params.trackType ?? race.trackType,
            location: params.location ?? race.location,
            startsAt: params.startsAt === undefined ? race.startsAt : params.startsAt,
            endsAt: params.endsAt === undefined ? race.endsAt : params.endsAt,
            classRestriction: params.classRestriction === undefined ? race.classRestriction : params.classRestriction,
            scoringType: params.scoringType === undefined ? race.scoringType : params.scoringType,
            grade: params.grade === undefined ? race.grade : params.grade,
            participantLimit: params.participantLimit === undefined ? race.participantLimit : params.participantLimit,
          } as unknown as eventmanager.RaceEventView
          return updatedRace!
        }
        return race
      })
      return {
        ...evt,
        raceEvents: updatedRaceEvents.sort((a, b) => a.sequence - b.sequence),
      }
    }
    return evt
  })

  const finalRace = updatedRace as eventmanager.RaceEventView | null
  if (!finalRace) {
    throw new Error('Race event not found in mocks')
  }

  // Persist updated mock events back to localStorage if available to persist across visual test reloads
  if (typeof window !== 'undefined' && window.localStorage) {
    const stored = window.localStorage.getItem('lightwing:mock:public_events')
    if (stored) {
      try {
        const publicEvents = JSON.parse(stored)
        const idx = publicEvents.findIndex((e: any) => e.id === eventId)
        if (idx !== -1) {
          publicEvents[idx].raceEvents = mockEvents.find((e) => e.id === eventId)?.raceEvents ?? []
          publicEvents[idx].updatedAt = new Date().toISOString()
          window.localStorage.setItem('lightwing:mock:public_events', JSON.stringify(publicEvents))
        }
      } catch {
        // ignore
      }
    }
  }

  return hydrateRaceEvent(eventId, finalRace)
}

export async function deleteRaceEvent(
  eventId: string,
  raceId: string,
  authorization: string,
): Promise<{ deleted: boolean }> {
  if (!MOCK_MODE) {
    return appClient.eventmanager.DeleteRaceEvent({
      eventId,
      raceId,
      Authorization: authorization,
    })
  }

  mockEvents = mockEvents.map((evt) => {
    if (evt.id === eventId) {
      return {
        ...evt,
        raceEvents: evt.raceEvents.filter((r) => r.id !== raceId),
      }
    }
    return evt
  })

  mockRaceResultsMap.delete(raceId)
  saveMockResults()

  return { deleted: true }
}

// -----------------------------------------------------------------------------
// RACE RESULTS METHODS
// -----------------------------------------------------------------------------

export async function listRaceResults(
  eventId: string,
  raceId: string,
): Promise<{ results: eventmanager.RaceResultView[] }> {
  if (!MOCK_MODE) {
    return appClient.eventmanager.ListRaceResults({ EventID: eventId, RaceID: raceId })
  }

  loadMockResults()
  const results = mockRaceResultsMap.get(raceId) ?? []
  return { results }
}

export async function assignRaceResult(
  eventId: string,
  raceId: string,
  userId: string,
  params: {
    position?: number | null
    points: number
    gateNumber?: number | null
    finishTime?: string | null
    margin?: string | null
    passingOrder?: string | null
    final3F?: string | null
    resultStatus?: string | null
  },
  authorization: string,
): Promise<eventmanager.RaceResultView> {
  if (!MOCK_MODE) {
    return appClient.eventmanager.AssignRaceResult({
      eventId,
      raceId,
      userId,
      Authorization: authorization,
      position: params.position,
      points: params.points,
      gateNumber: params.gateNumber,
      finishTime: params.finishTime,
      margin: params.margin,
      passingOrder: params.passingOrder,
      final3F: params.final3F,
      resultStatus: params.resultStatus ?? null,
    } as unknown as eventmanager.AssignRaceResultRequest)
  }

  loadMockResults()
  const existing = mockRaceResultsMap.get(raceId) ?? []
  const existingResult = existing.find((r) => r.userId === userId)

  const updatedResult = {
    id: existingResult?.id ?? `res_mock_${Math.floor(Math.random() * 10000)}`,
    raceEventId: raceId,
    userId,
    position: params.position ?? null,
    points: params.points,
    gateNumber: params.gateNumber ?? null,
    finishTime: params.finishTime ?? null,
    margin: params.margin ?? null,
    passingOrder: params.passingOrder ?? null,
    final3F: params.final3F ?? null,
    resultStatus: params.resultStatus ?? null,
    createdAt: existingResult?.createdAt ?? now,
    updatedAt: now,
  } as unknown as eventmanager.RaceResultView

  const updatedList = existingResult
    ? existing.map((r) => (r.userId === userId ? updatedResult : r))
    : [...existing, updatedResult]

  mockRaceResultsMap.set(raceId, updatedList)
  recomputeMockOverview(eventId)
  saveMockResults()

  return updatedResult
}

export async function replaceRaceResults(
  eventId: string,
  raceId: string,
  results: eventmanager.RaceResultInput[],
  authorization: string,
): Promise<{ results: eventmanager.RaceResultView[] }> {
  if (!MOCK_MODE) {
    return appClient.eventmanager.ReplaceRaceResults({
      eventId,
      raceId,
      Authorization: authorization,
      results,
    })
  }

  loadMockResults()
  const updatedList: eventmanager.RaceResultView[] = results.map((r, i) => ({
    id: `res_mock_${Math.floor(Math.random() * 10000)}`,
    raceEventId: raceId,
    userId: r.userId,
    position: r.position ?? null,
    points: r.points ?? 0,
    gateNumber: r.gateNumber ?? null,
    finishTime: r.finishTime ?? null,
    margin: r.margin ?? null,
    passingOrder: r.passingOrder ?? null,
    final3F: r.final3F ?? null,
    resultStatus: r.resultStatus ?? null,
    createdAt: now,
    updatedAt: now,
  }))

  mockRaceResultsMap.set(raceId, updatedList)
  recomputeMockOverview(eventId)
  saveMockResults()

  return { results: updatedList }
}

export async function mergeRaceResults(
  eventId: string,
  raceId: string,
  results: eventmanager.RaceResultInput[],
  authorization: string,
): Promise<{ results: eventmanager.RaceResultView[] }> {
  if (!MOCK_MODE) {
    return appClient.eventmanager.MergeRaceResults({
      eventId,
      raceId,
      Authorization: authorization,
      results,
    })
  }

  loadMockResults()
  const existing = mockRaceResultsMap.get(raceId) ?? []
  const updatedList = [...existing]

  for (const r of results) {
    const idx = updatedList.findIndex((x) => x.userId === r.userId)
    const nextResult: eventmanager.RaceResultView = {
      id: idx >= 0 ? updatedList[idx].id : `res_mock_${Math.floor(Math.random() * 10000)}`,
      raceEventId: raceId,
      userId: r.userId,
      position: r.position ?? null,
      points: r.points ?? 0,
      gateNumber: r.gateNumber ?? null,
      finishTime: r.finishTime ?? null,
      margin: r.margin ?? null,
      passingOrder: r.passingOrder ?? null,
      final3F: r.final3F ?? null,
      resultStatus: r.resultStatus ?? null,
      createdAt: idx >= 0 ? updatedList[idx].createdAt : now,
      updatedAt: now,
    }

    if (idx >= 0) {
      updatedList[idx] = nextResult
    } else {
      updatedList.push(nextResult)
    }
  }

  mockRaceResultsMap.set(raceId, updatedList)
  recomputeMockOverview(eventId)
  saveMockResults()

  return { results: updatedList }
}

export async function deleteRaceResult(
  eventId: string,
  raceId: string,
  userId: string,
  authorization: string,
): Promise<{ deleted: boolean }> {
  if (!MOCK_MODE) {
    return appClient.eventmanager.DeleteRaceResult({
      eventId,
      raceId,
      userId,
      Authorization: authorization,
    })
  }

  loadMockResults()
  const existing = mockRaceResultsMap.get(raceId) ?? []
  const filtered = existing.filter((r) => r.userId !== userId)
  mockRaceResultsMap.set(raceId, filtered)
  recomputeMockOverview(eventId)
  saveMockResults()

  return { deleted: true }
}

// -----------------------------------------------------------------------------
// EVENT MEMBER OPERATIONS
// -----------------------------------------------------------------------------

export async function addEventMember(
  eventId: string,
  userId: string,
  authorization: string,
): Promise<eventmanager.EventDetail> {
  if (!MOCK_MODE) {
    return appClient.eventmanager.AddEventMember({
      eventId,
      userId,
      Authorization: authorization,
    })
  }

  const userProfile = mockUserProfiles.get(userId)
  if (!userProfile) {
    throw new Error('Mock user not found. Try "mock-user-1", "mock-user-2", "mock-user-3" or look up.')
  }

  mockEvents = mockEvents.map((evt) => {
    if (evt.id === eventId) {
      const alreadyMember = evt.members.some((m) => m.userId === userId)
      if (alreadyMember) return evt

      const nextMembers = [
        ...evt.members,
        { userId: userProfile.id, name: userProfile.name, classTier: userProfile.classTier },
      ]

      return {
        ...evt,
        members: nextMembers,
      }
    }
    return evt
  })

  recomputeMockOverview(eventId)
  return getAdminEvent(eventId)
}

export async function removeEventMember(
  eventId: string,
  userId: string,
  authorization: string,
): Promise<eventmanager.EventDetail> {
  if (!MOCK_MODE) {
    return appClient.eventmanager.RemoveEventMember({
      EventID: eventId,
      UserID: userId,
      Authorization: authorization,
    })
  }

  mockEvents = mockEvents.map((evt) => {
    if (evt.id === eventId) {
      return {
        ...evt,
        members: evt.members.filter((m) => m.userId !== userId),
      }
    }
    return evt
  })

  recomputeMockOverview(eventId)
  return getAdminEvent(eventId)
}

// -----------------------------------------------------------------------------
// RACE EVENT MEMBER METHODS
// -----------------------------------------------------------------------------

export async function addRaceEventMember(
  eventId: string,
  raceId: string,
  userId: string,
  authorization: string,
): Promise<eventmanager.RaceEventDetail> {
  if (!MOCK_MODE) {
    return appClient.eventmanager.AddRaceEventMember({
      eventId,
      raceId,
      userId,
      Authorization: authorization,
    })
  }

  const userProfile = mockUserProfiles.get(userId)
  if (!userProfile) {
    throw new Error('Mock user not found')
  }

  const existing = mockRaceMembersMap.get(raceId) ?? []
  if (!existing.some((m) => m.userId === userId)) {
    const nextMembers = [
      ...existing,
      { userId: userProfile.id, name: userProfile.name, classTier: userProfile.classTier },
    ]
    mockRaceMembersMap.set(raceId, nextMembers)
  }

  const event = mockEvents.find((evt) => evt.id === eventId)
  if (!event) {
    throw new Error('Mock event not found')
  }
  const race = event.raceEvents.find((r) => r.id === raceId)
  if (!race) {
    throw new Error('Mock race not found')
  }

  return hydrateRaceEvent(eventId, race)
}

export async function removeRaceEventMember(
  eventId: string,
  raceId: string,
  userId: string,
  authorization: string,
): Promise<eventmanager.RaceEventDetail> {
  if (!MOCK_MODE) {
    return appClient.eventmanager.RemoveRaceEventMember({
      eventId,
      raceId,
      userId,
      Authorization: authorization,
    })
  }

  const existing = mockRaceMembersMap.get(raceId) ?? []
  const filtered = existing.filter((m) => m.userId !== userId)
  mockRaceMembersMap.set(raceId, filtered)

  const event = mockEvents.find((evt) => evt.id === eventId)
  if (!event) {
    throw new Error('Mock event not found')
  }
  const race = event.raceEvents.find((r) => r.id === raceId)
  if (!race) {
    throw new Error('Mock race not found')
  }

  return hydrateRaceEvent(eventId, race)
}

export async function listRaceEventMembers(
  eventId: string,
  raceId: string,
): Promise<{ members: eventmanager.RaceEventMemberView[] }> {
  if (!MOCK_MODE) {
    return appClient.eventmanager.ListRaceEventMembers({ EventID: eventId, RaceID: raceId })
  }

  const members = mockRaceMembersMap.get(raceId) ?? []
  return { members }
}

// Helper to compute overall totals for mock event views
function recomputeMockOverview(eventId: string) {
  const event = mockEvents.find((e) => e.id === eventId)
  if (!event) return

  // Accumulate points across all races of this event
  const pointsTotals: Record<string, number> = {}
  const statusTotals: Record<string, string | null> = {}
  for (const race of event.raceEvents) {
    const results = mockRaceResultsMap.get(race.id) ?? []
    for (const res of results) {
      pointsTotals[res.userId] = (pointsTotals[res.userId] ?? 0) + res.points
      if (res.resultStatus === 'DSQ' || res.resultStatus === 'DNF' || res.resultStatus === 'DNS' || res.resultStatus === 'DEFERRED') {
        statusTotals[res.userId] = res.resultStatus
      }
    }
  }

  if (event.scoringType === 1) {
    event.pointsOverview = event.members.map((member) => ({
      userId: member.userId,
      name: member.name,
      points: pointsTotals[member.userId] ?? 0,
      resultStatus: statusTotals[member.userId] ?? null,
    })).sort((a, b) => b.points - a.points)
  }
}

// -----------------------------------------------------------------------------
// USER MANAGEMENT METHODS
// -----------------------------------------------------------------------------

export async function listAdminUsers(
  authorization: string,
  search?: string,
  limit?: number,
  offset?: number,
): Promise<{ users: auth.UserProfile[]; total: number }> {
  if (!MOCK_MODE) {
    return appClient.auth.ListUsers({
      Authorization: authorization,
      Search: search ?? '',
      Limit: limit ?? 0,
      Offset: offset ?? 0,
    })
  }

  let users = Array.from(mockUserProfiles.values())
  if (search) {
    const lower = search.toLowerCase()
    users = users.filter((u) =>
      u.name.toLowerCase().includes(lower) ||
      u.email.toLowerCase().includes(lower) ||
      (u.vrchatUsername && u.vrchatUsername.toLowerCase().includes(lower))
    )
  }
  const total = users.length
  if (offset !== undefined) {
    users = users.slice(offset)
  }
  if (limit !== undefined) {
    users = users.slice(0, limit)
  }
  return { users, total }
}

export async function getAdminUserProfile(userId: string): Promise<auth.UserProfile> {
  if (!MOCK_MODE) {
    return appClient.auth.GetUser(userId)
  }

  const existing = mockUserProfiles.get(userId)
  if (existing) {
    return existing
  }

  throw new Error(`User ${userId} was not found in mock records`)
}

export async function updateAdminUserSiteRole(
  userId: string,
  siteRole: SiteRole,
  authorization: string,
): Promise<auth.UserProfile> {
  if (!MOCK_MODE) {
    return appClient.auth.SetUserSiteRole({
      id: userId,
      Authorization: authorization,
      siteRole,
    })
  }

  const existing = mockUserProfiles.get(userId)
  if (!existing) {
    throw new Error(`User ${userId} was not found in mock records`)
  }

  const nextRole: SiteRole = siteRole === 'SITE_ADMIN' ? 'SITE_ADMIN' : 'USER'
  const updated: auth.UserProfile = {
    ...existing,
    siteRole: nextRole,
    updatedAt: new Date().toISOString(),
  }

  mockUserProfiles.set(userId, updated)
  return updated
}

export async function updateAdminUserProfile(
  userId: string,
  params: {
    name?: string
    slug?: string
    image?: string | null
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
      ...params,
    } as unknown as auth.UpdateUserRequest)
  }

  const existing = mockUserProfiles.get(userId)
  if (!existing) {
    throw new Error(`User ${userId} was not found in mock records`)
  }

  const updated: auth.UserProfile = {
    ...existing,
    name: params.name ?? existing.name,
    slug: params.slug ?? existing.slug,
    image: params.image ?? existing.image,
    biography: params.biography ?? existing.biography,
    careerOverview: params.careerOverview ?? existing.careerOverview,
    vrchatUsername: params.vrchatUsername ?? existing.vrchatUsername,
    updatedAt: new Date().toISOString(),
  }

  mockUserProfiles.set(userId, updated)
  return updated
}

export async function updateAdminUserClass(
  userId: string,
  classTier: ClassTier | null,
  authorization: string,
): Promise<{ userId: string; classTier: ClassTier | null }> {
  if (!MOCK_MODE) {
    const res = await appClient.eventmanager.SetUserClass({
      userId,
      Authorization: authorization,
      organizationId: null,
      classTier,
    } as unknown as eventmanager.SetUserClassRequest)
    return { userId: res.userId, classTier: res.classTier as ClassTier | null }
  }

  const existing = mockUserProfiles.get(userId)
  if (!existing) {
    throw new Error(`User ${userId} was not found in mock records`)
  }

  const updated: auth.UserProfile = {
    ...existing,
    classTier: classTier ?? existing.classTier,
    updatedAt: new Date().toISOString(),
  }

  mockUserProfiles.set(userId, updated)
  return { userId, classTier }
}

// -----------------------------------------------------------------------------
// TEAM MANAGEMENT METHODS
// -----------------------------------------------------------------------------

export async function listAdminTeams(
  search?: string,
  limit?: number,
  offset?: number,
): Promise<{ teams: teammanager.TeamListItem[]; total: number }> {
  if (!MOCK_MODE) {
    return appClient.teammanager.ListTeams({
      Search: search ?? '',
      Limit: limit ?? 0,
      Offset: offset ?? 0,
    })
  }

  let filtered = mockTeamsList.map((t) => ({
    id: t.id,
    name: t.name,
    slug: t.slug,
    logo: t.logo,
    administratorSlotsRemaining: t.administratorSlotsRemaining,
    memberCount: t.members.length,
  }))

  if (search) {
    const lower = search.toLowerCase()
    filtered = filtered.filter((t) => t.name.toLowerCase().includes(lower) || t.slug.toLowerCase().includes(lower))
  }

  const total = filtered.length
  if (offset !== undefined) {
    filtered = filtered.slice(offset)
  }
  if (limit !== undefined) {
    filtered = filtered.slice(0, limit)
  }

  return { teams: filtered, total }
}

export async function getAdminTeam(id: string): Promise<teammanager.Team> {
  if (!MOCK_MODE) {
    return appClient.teammanager.GetTeam(id)
  }

  const team = mockTeamsList.find((t) => t.id === id)
  if (!team) {
    throw new Error('Mock team not found')
  }
  return team
}

export async function updateAdminTeamStats(
  id: string,
  params: {
    rankingAverage?: number | null
    pointsAverage?: number | null
    seasonRank?: number | null
    averagePointsPerEvent?: number | null
  },
  authorization: string,
): Promise<teammanager.Team> {
  if (!MOCK_MODE) {
    return appClient.teammanager.UpdateTeamStats({
      id,
      Authorization: authorization,
      ...params,
    } as unknown as teammanager.UpdateTeamStatsRequest)
  }

  const teamIndex = mockTeamsList.findIndex((t) => t.id === id)
  if (teamIndex === -1) throw new Error('Mock team not found')
  const team = mockTeamsList[teamIndex]

  team.stats = {
    rankingAverage: params.rankingAverage ?? team.stats.rankingAverage,
    pointsAverage: params.pointsAverage ?? team.stats.pointsAverage,
    seasonRank: params.seasonRank ?? team.stats.seasonRank,
    averagePointsPerEvent: params.averagePointsPerEvent ?? team.stats.averagePointsPerEvent,
  }

  return team
}

export async function updateAdminTeam(
  id: string,
  params: {
    name?: string
    slug?: string
    logo?: string | null
  },
  authorization: string,
): Promise<teammanager.Team> {
  if (!MOCK_MODE) {
    return appClient.teammanager.UpdateTeam({
      id,
      Authorization: authorization,
      ...params,
    } as unknown as teammanager.UpdateTeamRequest)
  }

  const teamIndex = mockTeamsList.findIndex((t) => t.id === id)
  if (teamIndex === -1) throw new Error('Mock team not found')
  const team = mockTeamsList[teamIndex]

  team.name = params.name ?? team.name
  team.slug = params.slug ?? team.slug
  team.logo = params.logo ?? team.logo

  return team
}

export async function listAdminTeamMembers(
  teamId: string,
  search?: string,
  limit?: number,
  offset?: number,
): Promise<{ members: Array<{ userId: string; name: string; slug: string | null; role: string }>; total: number }> {
  if (!MOCK_MODE) {
    return appClient.teammanager.ListTeamMembers({
      ID: teamId,
      Search: search ?? '',
      Limit: limit ?? 0,
      Offset: offset ?? 0,
    })
  }

  const team = mockTeamsList.find((t) => t.id === teamId)
  if (!team) {
    throw new Error('Mock team not found')
  }

  let filtered = team.members.map((m) => {
    const user = mockUserProfiles.get(m.userId)
    return {
      userId: m.userId,
      name: m.name,
      slug: user ? user.slug : null,
      role: m.role,
    }
  })

  if (search) {
    const lower = search.toLowerCase()
    filtered = filtered.filter((m) => m.name.toLowerCase().includes(lower) || (m.slug && m.slug.toLowerCase().includes(lower)))
  }

  const total = filtered.length
  if (offset !== undefined) {
    filtered = filtered.slice(offset)
  }
  if (limit !== undefined) {
    filtered = filtered.slice(0, limit)
  }

  return { members: filtered, total }
}

// -----------------------------------------------------------------------------
// RECOMPUTATION OPERATIONS
// -----------------------------------------------------------------------------

export async function recomputeEventPoints(
  eventId: string,
  authorization: string,
): Promise<{ success: boolean }> {
  if (!MOCK_MODE) {
    return appClient.eventmanager.RecomputeEventPoints({ id: eventId, Authorization: authorization })
  }
  recomputeMockOverview(eventId)
  return { success: true }
}

export async function createAdminTeam(
  params: { name: string; logo?: string | null },
  authorization: string,
): Promise<teammanager.Team> {
  if (!MOCK_MODE) {
    return appClient.teammanager.CreateTeam({
      Authorization: authorization,
      name: params.name,
      logo: params.logo ?? null,
    } as unknown as teammanager.CreateTeamRequest)
  }

  const slug = params.name
    .toLowerCase()
    .replace(/[^a-z0-9]+/g, '-')
    .replace(/(^-|-$)/g, '')

  if (mockTeamsList.some((t) => t.slug === slug)) {
    throw new Error('team with this slug already exists')
  }

  const id = `org_mock_${Math.floor(Math.random() * 10000)}`
  const newTeam = {
    id,
    name: params.name,
    slug,
    logo: params.logo ?? null,
    stats: {
      rankingAverage: null,
      pointsAverage: null,
      seasonRank: null,
      averagePointsPerEvent: null,
    },
    administratorSlotsRemaining: 3,
    members: [],
  } as unknown as teammanager.Team

  mockTeamsList.unshift(newTeam)
  return newTeam
}

export async function addAdminTeamMember(
  teamId: string,
  params: { userId: string; role?: string },
  authorization: string,
): Promise<teammanager.Team> {
  if (!MOCK_MODE) {
    return appClient.teammanager.AddTeamMember({
      id: teamId,
      Authorization: authorization,
      userId: params.userId,
      role: params.role ?? 'member',
    })
  }

  const teamIndex = mockTeamsList.findIndex((t) => t.id === teamId)
  if (teamIndex === -1) {
    throw new Error('Mock team not found')
  }

  const user = mockUserProfiles.get(params.userId)
  if (!user) {
    throw new Error('Mock user not found')
  }

  const team = mockTeamsList[teamIndex]
  if (team.members.some((m) => m.userId === params.userId)) {
    throw new Error('user is already a member of this team')
  }

  const role = params.role ?? 'member'
  if (role === 'administrator') {
    const adminsCount = team.members.filter((m) => m.role === 'administrator').length
    if (adminsCount >= 3) {
      throw new Error('At most three administrators can belong to an organization.')
    }
  }

  team.members.push({ userId: user.id, name: user.name, role })
  team.administratorSlotsRemaining = Math.max(3 - team.members.filter((m) => m.role === 'administrator').length, 0)

  user.teams = [...user.teams, { organizationId: team.id, name: team.name, slug: team.slug, role }]
  mockUserProfiles.set(user.id, user)

  return team
}

export async function updateAdminTeamMemberRole(
  teamId: string,
  userId: string,
  role: string,
  authorization: string,
): Promise<teammanager.Team> {
  if (!MOCK_MODE) {
    return appClient.teammanager.UpdateTeamMemberRole({
      id: teamId,
      userId,
      Authorization: authorization,
      role,
    })
  }

  const teamIndex = mockTeamsList.findIndex((t) => t.id === teamId)
  if (teamIndex === -1) {
    throw new Error('Mock team not found')
  }
  const team = mockTeamsList[teamIndex]

  const memberIndex = team.members.findIndex((m) => m.userId === userId)
  if (memberIndex === -1) {
    throw new Error('member not found')
  }

  const oldRole = team.members[memberIndex].role
  if (role === 'administrator' && oldRole !== 'administrator') {
    const adminsCount = team.members.filter((m) => m.role === 'administrator').length
    if (adminsCount >= 3) {
      throw new Error('At most three administrators can belong to an organization.')
    }
  }

  team.members[memberIndex].role = role
  team.administratorSlotsRemaining = Math.max(3 - team.members.filter((m) => m.role === 'administrator').length, 0)

  const user = mockUserProfiles.get(userId)
  if (user) {
    user.teams = user.teams.map((t) => (t.organizationId === teamId ? { ...t, role } : t))
    mockUserProfiles.set(userId, user)
  }

  return team
}

export async function removeAdminTeamMember(
  teamId: string,
  userId: string,
  authorization: string,
): Promise<teammanager.Team> {
  if (!MOCK_MODE) {
    return appClient.teammanager.RemoveTeamMember({
      ID: teamId,
      UserID: userId,
      Authorization: authorization,
    })
  }

  const teamIndex = mockTeamsList.findIndex((t) => t.id === teamId)
  if (teamIndex === -1) {
    throw new Error('Mock team not found')
  }
  const team = mockTeamsList[teamIndex]

  const memberIndex = team.members.findIndex((m) => m.userId === userId)
  if (memberIndex === -1) {
    throw new Error('member not found')
  }

  team.members.splice(memberIndex, 1)
  team.administratorSlotsRemaining = Math.max(3 - team.members.filter((m) => m.role === 'administrator').length, 0)

  const user = mockUserProfiles.get(userId)
  if (user) {
    user.teams = user.teams.filter((t) => t.organizationId !== teamId)
    mockUserProfiles.set(userId, user)
  }

  return team
}

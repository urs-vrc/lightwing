import { useCallback, useEffect, useMemo, useState } from 'react'
import { useAuth } from './useAuth'
import {
  getAdminEvent,
  updateAdminEvent,
  recomputeEventPoints,
  listRaceResults,
  deleteAdminEvent,
  restoreAdminEvent,
} from '../lib/admin-api'
import type { eventmanager } from '../lib/client'
import type { ClassTier, EventStatus, EventTag } from '../types'
import {
  isRaceOngoing,
  isRaceConcluded,
  isRaceNotStarted,
} from '../lib/raceStatus'
import { editsFromResults } from '../lib/standings'

import { useEventRaces } from './useEventRaces'
import { useEventMembers } from './useEventMembers'
import { useEventResults } from './useEventResults'
import { useEventStatus } from './useEventStatus'

const STATUS_OPTIONS: EventStatus[] = ['DRAFT', 'PENDING', 'ONGOING', 'CONCLUDED', 'PENDING_DELETION']
const TAG_OPTIONS: EventTag[] = ['OFFICIAL', 'COMMUNITY']
const CLASS_TIER_OPTIONS = ['G3', 'G2', 'G1']

export type ActiveTab = 'details' | 'members' | 'races' | 'datasets'

export function useEventDetail(eventId: string) {
  const { session } = useAuth()
  const [selectedEvent, setSelectedEvent] = useState<eventmanager.EventDetail | null>(null)
  const [activeTab, setActiveTab] = useState<ActiveTab>('details')

  // Global UI States
  const [loadingEventDetail, setLoadingEventDetail] = useState(true)
  const [loadingRaces, setLoadingRaces] = useState(false)
  const [loadingResults, setLoadingResults] = useState(false)
  const [globalError, setGlobalError] = useState<string | null>(null)
  const [globalSuccess, setGlobalSuccess] = useState<string | null>(null)

  const authHeader = useMemo(() => {
    const token = session?.session.token
    return token ? `Bearer ${token}` : null
  }, [session?.session.token])

  // Races and selected race states
  const [races, setRaces] = useState<eventmanager.RaceEventDetail[]>([])
  const [selectedRaceId, setSelectedRaceId] = useState<string | null>(null)

  const selectedRace = useMemo(
    () => races.find((r) => r.id === selectedRaceId) ?? null,
    [races, selectedRaceId],
  )

  // Sub-hook: Event Results
  const resultsHook = useEventResults({
    eventId,
    authHeader,
    selectedEvent,
    selectedRace,
    selectedRaceId,
    setSelectedRaceId,
    reloadCurrentEvent: async () => { await reloadCurrentEvent() },
    setGlobalError,
    setGlobalSuccess,
  })

  // Sub-hook: Event Races
  const racesHook = useEventRaces({
    eventId,
    authHeader,
    reloadCurrentEvent: async () => { await reloadCurrentEvent() },
    setGlobalError,
    setGlobalSuccess,
    selectedRaceId,
    setSelectedRaceId,
    setResults: resultsHook.setResults,
    setEditedResults: resultsHook.setEditedResults,
    setPendingDeletions: resultsHook.setPendingDeletions,
    handleSelectRace: async (race, switchTab) => { await handleSelectRace(race, switchTab) },
    races,
    setRaces,
  })

  // Sub-hook: Event Members
  const membersHook = useEventMembers({
    eventId,
    authHeader,
    setSelectedEvent,
    setRaces,
    setGlobalError,
    setGlobalSuccess,
  })

  // Sub-hook: Event Status
  const statusHook = useEventStatus({
    eventId,
    authHeader,
    setSelectedEvent,
    setGlobalError,
    setGlobalSuccess,
  })

  // Load selected event details
  const loadEvent = useCallback(async () => {
    setLoadingEventDetail(true)
    setGlobalError(null)
    setGlobalSuccess(null)
    setSelectedRaceId(null)
    resultsHook.setResults([])
    resultsHook.setEditedResults({})
    resultsHook.setPendingDeletions(new Set())
    try {
      const detail = await getAdminEvent(eventId)
      setSelectedEvent(detail)
      setRaces(detail.raceEvents as eventmanager.RaceEventDetail[])
    } catch (cause) {
      setGlobalError(cause instanceof Error ? cause.message : 'Unable to load event details')
      setSelectedEvent(null)
    } finally {
      setLoadingEventDetail(false)
    }
  }, [eventId, setRaces])

  useEffect(() => {
    void loadEvent()
  }, [loadEvent])

  // Reload current event details
  const reloadCurrentEvent = useCallback(async () => {
    try {
      const detail = await getAdminEvent(eventId)
      setSelectedEvent(detail)
      setRaces(detail.raceEvents as eventmanager.RaceEventDetail[])
    } catch (err) {
      console.error('Failed to reload current event details', err)
    }
  }, [eventId, setRaces])

  const handleUpdateEventDetails = useCallback(
    async (params: {
      name: string
      description: string | null
      classRestriction: ClassTier | null
      scoringRulesMode?: string | null
      customScoringTables?: any | null
      scheduledAt?: string | null
      participantLimit?: number | null
      maxConcurrentRaceParticipations?: number | null
    }) => {
      if (!authHeader) return
      setGlobalError(null)
      setGlobalSuccess(null)

      // Strict validation for custom scoring rules
      if (params.scoringRulesMode === 'CUSTOM') {
        if (!params.customScoringTables) {
          setGlobalError('Custom scoring tables are required.')
          return
        }
        const grades = ['OP', 'GIII', 'GII', 'GI']
        for (const grade of grades) {
          const table = params.scoringRulesMode === 'CUSTOM' ? params.customScoringTables[grade] : null
          if (!table) {
            setGlobalError(`Custom table for grade ${grade} is missing.`)
            return
          }
          for (let pos = 1; pos <= 10; pos++) {
            const val = table[pos]
            if (val === undefined || val === null || String(val).trim() === '') {
              setGlobalError(`Custom table for grade ${grade} is missing value for position #${pos}.`)
              return
            }
            const num = Number(val)
            if (!Number.isInteger(num) || num < 0) {
              setGlobalError(`Custom table for grade ${grade}, position #${pos} must be a valid non-negative integer.`)
              return
            }
          }
        }
      }

      try {
        const updated = await updateAdminEvent(eventId, params, authHeader)
        setSelectedEvent(updated)
        setGlobalSuccess('Successfully updated event details.')
      } catch (cause) {
        setGlobalError(cause instanceof Error ? cause.message : 'Unable to update event details')
      }
    },
    [authHeader, eventId],
  )

  const handleRecomputeEventPoints = useCallback(async () => {
    if (!authHeader) return
    setGlobalError(null)
    setGlobalSuccess(null)
    try {
      await recomputeEventPoints(eventId, authHeader)
      await reloadCurrentEvent()
      setGlobalSuccess('Successfully recomputed all results points.')
    } catch (cause) {
      setGlobalError(cause instanceof Error ? cause.message : 'Unable to recompute event points')
    }
  }, [authHeader, eventId, reloadCurrentEvent])

  // Select Race to load results
  const handleSelectRace = useCallback(
    async (race: eventmanager.RaceEventDetail, switchTab = true) => {
      setSelectedRaceId(race.id)
      setLoadingResults(true)
      setGlobalError(null)
      setGlobalSuccess(null)
      resultsHook.setEditedResults({})
      resultsHook.setPendingDeletions(new Set())
      if (switchTab) {
        setActiveTab('races')
      }
      try {
        const response = await listRaceResults(eventId, race.id)
        const resultsRes = response.results
        resultsHook.setResults(resultsRes)
        resultsHook.setEditedResults(editsFromResults(resultsRes))
      } catch (cause) {
        setGlobalError(cause instanceof Error ? cause.message : 'Unable to load race results')
        resultsHook.setResults([])
      } finally {
        setLoadingResults(false)
      }
    },
    [eventId, resultsHook],
  )

  const handleDeleteEvent = useCallback(
    async (permanent?: boolean) => {
      if (!authHeader) return
      setGlobalError(null)
      setGlobalSuccess(null)
      try {
        await deleteAdminEvent(eventId, authHeader, permanent)
        setGlobalSuccess(permanent ? 'Permanently deleted event.' : 'Event moved to Pending Deletion queue.')
        await reloadCurrentEvent()
      } catch (cause) {
        setGlobalError(cause instanceof Error ? cause.message : 'Unable to delete event')
      }
    },
    [authHeader, eventId, reloadCurrentEvent],
  )

  const handleRestoreEvent = useCallback(async () => {
    if (!authHeader) return
    setGlobalError(null)
    setGlobalSuccess(null)
    try {
      const updated = await restoreAdminEvent(eventId, authHeader)
      setSelectedEvent(updated)
      setGlobalSuccess('Successfully restored event.')
    } catch (cause) {
      setGlobalError(cause instanceof Error ? cause.message : 'Unable to restore event')
    }
  }, [authHeader, eventId])

  const ongoingRaces = useMemo(() => races.filter(isRaceOngoing), [races])
  const concludedRaces = useMemo(() => races.filter(isRaceConcluded), [races])
  const notStartedRaces = useMemo(() => races.filter(isRaceNotStarted), [races])

  return {
    // constants
    STATUS_OPTIONS,
    TAG_OPTIONS,
    CLASS_TIER_OPTIONS,
    // state
    selectedEvent,
    activeTab,
    setActiveTab,
    races,
    selectedRaceId,
    setSelectedRaceId,
    selectedRace,
    results: resultsHook.results,
    editedResults: resultsHook.editedResults,
    pendingDeletions: resultsHook.pendingDeletions,
    newMemberUserId: membersHook.newMemberUserId,
    setNewMemberUserId: membersHook.setNewMemberUserId,
    newRaceMemberUserId: membersHook.newRaceMemberUserId,
    setNewRaceMemberUserId: membersHook.setNewRaceMemberUserId,
    newRaceForm: racesHook.newRaceForm,
    setNewRaceForm: racesHook.setNewRaceForm,
    loadingEventDetail,
    loadingRaces,
    loadingResults,
    savingBatch: resultsHook.savingBatch,
    eventStatusSaving: statusHook.eventStatusSaving,
    signupsLockedSaving: statusHook.signupsLockedSaving,
    globalError,
    globalSuccess,
    derivedStates: resultsHook.derivedStates,
    changeSummary: resultsHook.changeSummary,
    ongoingRaces,
    concludedRaces,
    notStartedRaces,
    // actions
    handleUpdateEventStatus: statusHook.handleUpdateEventStatus,
    handleSetSignupsLocked: statusHook.handleSetSignupsLocked,
    handleUpdateEventDetails,
    handleRecomputeEventPoints,
    handleDeleteEvent,
    handleRestoreEvent,
    handleAddMember: membersHook.handleAddMember,
    handleRemoveMember: membersHook.handleRemoveMember,
    handleAddRaceMember: membersHook.handleAddRaceMember,
    handleRemoveRaceMember: membersHook.handleRemoveRaceMember,
    handleCreateRace: racesHook.handleCreateRace,
    handleStartRace: racesHook.handleStartRace,
    handleEndRace: racesHook.handleEndRace,
    handleUpdateRace: racesHook.handleUpdateRace,
    handleDeleteRace: racesHook.handleDeleteRace,
    handleSelectRace,
    handleResultChange: resultsHook.handleResultChange,
    togglePendingDeletion: resultsHook.togglePendingDeletion,
    handleUndoRow: resultsHook.handleUndoRow,
    resetStandingsDraft: resultsHook.resetStandingsDraft,
    handleInferFinishTimes: resultsHook.handleInferFinishTimes,
    handleCancelStandingsEdit: resultsHook.handleCancelStandingsEdit,
    handleUnifiedSave: resultsHook.handleUnifiedSave,
    handleReorderRaces: racesHook.handleReorderRaces,
  }
}

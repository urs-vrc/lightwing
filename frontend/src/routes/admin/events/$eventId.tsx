import { Link, createFileRoute } from '@tanstack/react-router'
import { useState, useEffect } from 'react'
import { requireSiteAdmin } from '../../../lib/auth-guard'
import { AdminLayout } from '../-AdminLayout'
import { useEventDetail } from '../../../hooks/useEventDetail'
import { AlertBanner } from '../../../components/AlertBanner'
import { LoadingBox } from '../../../components/LoadingBox'
import { GradePointsPreview } from '../../../components/GradePointsPreview'
import { SldsSkeletonDetail } from '../../../components/LoadingSkeleton'
import { EventScoringTablesEditor } from '../../../components/EventScoringTablesEditor'
import type { eventmanager } from '../../../lib/client'

import { RaceListPanel } from '../../../components/RaceListPanel'
import { RaceDetailPane } from '../../../components/RaceDetailPane'

import { EventSummaryTab } from '../../../components/EventSummaryTab'
import { EventMembersTab } from '../../../components/EventMembersTab'
import { EventRacesTab } from '../../../components/EventRacesTab'
import { DEFAULT_SCORING_TABLES } from '../../../lib/scoringDefaults'
import { toLocalISOString } from '../../../lib/datetime'
import type { ClassTier, EventStatus, EventTag } from '../../../types'

export const Route = createFileRoute('/admin/events/$eventId')({
  beforeLoad: async ({ location }) => {
    await requireSiteAdmin(location)
  },
  component: AdminEventDetailPage,
})

function AdminEventDetailPage() {
  const { eventId } = Route.useParams()
  const {
    STATUS_OPTIONS,
    TAG_OPTIONS,
    CLASS_TIER_OPTIONS,
    selectedEvent,
    activeTab,
    setActiveTab,
    races,
    selectedRaceId,
    setSelectedRaceId,
    selectedRace,
    newMemberUserId,
    setNewMemberUserId,
    newRaceMemberUserId,
    setNewRaceMemberUserId,
    newRaceForm,
    setNewRaceForm,
    loadingEventDetail,
    eventStatusSaving,
    signupsLockedSaving,
    globalError,
    globalSuccess,
    derivedStates,
    changeSummary,
    loadingResults,
    savingBatch,
    ongoingRaces,
    concludedRaces,
    notStartedRaces,
    results,
    handleUpdateEventStatus,
    handleSetSignupsLocked,
    handleUpdateEventDetails,
    handleRecomputeEventPoints,
    handleDeleteEvent,
    handleRestoreEvent,
    handleAddMember,
    handleRemoveMember,
    handleAddRaceMember,
    handleRemoveRaceMember,
    handleCreateRace,
    handleStartRace,
    handleEndRace,
    handleUpdateRace,
    handleDeleteRace,
    handleSelectRace,
    handleResultChange,
    togglePendingDeletion,
    handleUndoRow,
    resetStandingsDraft,
    handleInferFinishTimes,
    handleCancelStandingsEdit,
    handleUnifiedSave,
    handleReorderRaces,
  } = useEventDetail(eventId)

  const [showCreateRaceModal, setShowCreateRaceModal] = useState(false)
  const [showEditEventModal, setShowEditEventModal] = useState(false)
  const [showEditRaceModal, setShowEditRaceModal] = useState(false)

  const hasStartedOrConcludedRaces = races.some((r) => r.startsAt !== null || r.endsAt !== null)

  // Race Edit Form States
  const [raceEditName, setRaceEditName] = useState('')
  const [raceEditLocation, setRaceEditLocation] = useState('')
  const [raceEditDistance, setRaceEditDistance] = useState(1200)
  const [raceEditTrackType, setRaceEditTrackType] = useState('Turf')
  const [raceEditClassRestriction, setRaceEditClassRestriction] = useState<ClassTier | null>(null)
  const [raceEditGrade, setRaceEditGrade] = useState<string | null>(null)
  const [raceEditParticipantLimit, setRaceEditParticipantLimit] = useState<string>('')
  const [showRaceGradeConfirm, setShowRaceGradeConfirm] = useState(false)
  const [raceEditError, setRaceEditError] = useState<string | null>(null)

  const [editName, setEditName] = useState('')
  const [editDescription, setEditDescription] = useState('')
  const [editClassRestriction, setEditClassRestriction] = useState<ClassTier | null>(null)
  const [editGranularParticipation, setEditGranularParticipation] = useState(false)
  const [editSignupsLocked, setEditSignupsLocked] = useState(false)
  const [editScheduledAt, setEditScheduledAt] = useState<string>('')
  const [editParticipantLimit, setEditParticipantLimit] = useState<string>('')
  const [editMaxConcurrentRaceParticipations, setEditMaxConcurrentRaceParticipations] = useState<string>('')
  const [editScoringRulesMode, setEditScoringRulesMode] = useState<'STANDARD' | 'CUSTOM'>('STANDARD')
  const [editCustomScoringTables, setEditCustomScoringTables] = useState<Record<string, Record<number, number>>>(DEFAULT_SCORING_TABLES)
  const [showConfirmModal, setShowConfirmModal] = useState(false)

  // Set race edit values when modal is toggled or race selection changes
  useEffect(() => {
    if (selectedRace) {
      setRaceEditName(selectedRace.name)
      setRaceEditLocation(selectedRace.location)
      setRaceEditDistance(selectedRace.distanceMeters)
      setRaceEditTrackType(selectedRace.trackType)
      setRaceEditClassRestriction(selectedRace.classRestriction as ClassTier | null)
      setRaceEditGrade(selectedRace.grade)
      setRaceEditParticipantLimit(selectedRace.participantLimit !== null ? String(selectedRace.participantLimit) : '')
      setRaceEditError(null)
    }
  }, [showEditRaceModal, selectedRace])

  // Set edit values when modal is toggled or event loads
  useEffect(() => {
    if (selectedEvent) {
      setEditName(selectedEvent.name)
      setEditDescription(selectedEvent.description ?? '')
      setEditClassRestriction(selectedEvent.classRestriction as ClassTier | null)
      setEditGranularParticipation(selectedEvent.granularParticipation)
      setEditSignupsLocked(selectedEvent.signupsLocked)
      setEditScheduledAt(selectedEvent.scheduledAt ? toLocalISOString(selectedEvent.scheduledAt) : '')
      setEditParticipantLimit(selectedEvent.participantLimit !== null ? String(selectedEvent.participantLimit) : '')
      setEditMaxConcurrentRaceParticipations(selectedEvent.maxConcurrentRaceParticipations !== null ? String(selectedEvent.maxConcurrentRaceParticipations) : '')
      setEditScoringRulesMode((selectedEvent.scoringRulesMode as 'STANDARD' | 'CUSTOM') || 'STANDARD')
      if (selectedEvent.customScoringTables) {
        setEditCustomScoringTables(selectedEvent.customScoringTables as Record<string, Record<number, number>>)
      } else {
        setEditCustomScoringTables(DEFAULT_SCORING_TABLES)
      }
    }
  }, [showEditEventModal, selectedEvent])

  const performSaveEventDetails = async () => {
    const limitNum = editParticipantLimit.trim() ? Number(editParticipantLimit) : null
    const maxConcurrentNum = editMaxConcurrentRaceParticipations.trim() ? Number(editMaxConcurrentRaceParticipations) : null

    if (!editGranularParticipation && limitNum !== null && (isNaN(limitNum) || !Number.isSafeInteger(limitNum) || limitNum <= 0)) {
      window.alert('Participant limit must be a positive whole number.')
      return
    }

    if (editGranularParticipation && maxConcurrentNum !== null && (isNaN(maxConcurrentNum) || !Number.isSafeInteger(maxConcurrentNum) || maxConcurrentNum <= 0)) {
      window.alert('Max races per participant must be a positive whole number.')
      return
    }

    await handleUpdateEventDetails({
      name: editName,
      description: editDescription || null,
      classRestriction: editClassRestriction || null,
      scheduledAt: editScheduledAt ? new Date(editScheduledAt).toISOString() : null,
      participantLimit: editGranularParticipation ? null : limitNum,
      maxConcurrentRaceParticipations: editGranularParticipation ? maxConcurrentNum : null,
      scoringRulesMode: editScoringRulesMode,
      customScoringTables: editScoringRulesMode === 'CUSTOM' ? editCustomScoringTables : null,
    })
    if (selectedEvent && editSignupsLocked !== selectedEvent.signupsLocked) {
      await handleSetSignupsLocked(editSignupsLocked)
    }
    setShowEditEventModal(false)
  }

  const onEditEventSubmit = async (e: React.FormEvent) => {
    e.preventDefault()

    // Strict validation
    if (selectedEvent && selectedEvent.scoringType === 1 && editScoringRulesMode === 'CUSTOM') {
      const grades = ['OP', 'GIII', 'GII', 'GI']
      for (const grade of grades) {
        const table = editCustomScoringTables[grade]
        if (!table) {
          window.alert(`Custom table for grade ${grade} is missing.`)
          return
        }
        for (let pos = 1; pos <= 10; pos++) {
          const val = table[pos]
          if (val === undefined || val === null || String(val).trim() === '') {
            window.alert(`Custom table for grade ${grade} is missing value for position #${pos}.`)
            return
          }
          const num = Number(val)
          if (!Number.isInteger(num) || num < 0) {
            window.alert(`Custom table for grade ${grade}, position #${pos} must be a valid non-negative integer.`)
            return
          }
        }
      }
    }

    const scoringRulesChanged = selectedEvent && selectedEvent.scoringType === 1 && (
      selectedEvent.scoringRulesMode !== editScoringRulesMode ||
      (editScoringRulesMode === 'CUSTOM' && JSON.stringify(selectedEvent.customScoringTables) !== JSON.stringify(editCustomScoringTables))
    )

    if (scoringRulesChanged) {
      setShowConfirmModal(true)
    } else {
      await performSaveEventDetails()
    }
  }

  const performSaveRaceDetails = async () => {
    if (!selectedRace) return
    const limitNum = raceEditParticipantLimit.trim() ? Number(raceEditParticipantLimit) : null
    if (limitNum !== null && (isNaN(limitNum) || !Number.isSafeInteger(limitNum) || limitNum <= 0)) {
      setRaceEditError('Race participant limit must be a positive whole number.')
      return
    }

    await handleUpdateRace(selectedRace.id, {
      name: raceEditName.trim(),
      location: raceEditLocation.trim(),
      distanceMeters: raceEditDistance,
      trackType: raceEditTrackType.trim(),
      classRestriction: raceEditClassRestriction,
      grade: raceEditGrade,
      participantLimit: limitNum,
    })
    setShowEditRaceModal(false)
  }

  const onEditRaceSubmit = async (e: React.FormEvent) => {
    e.preventDefault()
    setRaceEditError(null)

    // Robust frontend validation
    const trimmedName = raceEditName.trim()
    const trimmedLocation = raceEditLocation.trim()
    const trimmedTrackType = raceEditTrackType.trim()

    if (!trimmedName) {
      setRaceEditError('Race name is required and cannot be empty.')
      return
    }
    if (!trimmedLocation) {
      setRaceEditError('Location is required and cannot be empty.')
      return
    }
    if (!trimmedTrackType) {
      setRaceEditError('Track Type is required and cannot be empty.')
      return
    }
    if (!Number.isInteger(raceEditDistance) || raceEditDistance <= 0) {
      setRaceEditError('Distance must be a valid integer greater than 0.')
      return
    }

    const hasResults = results && results.length > 0
    const gradeChanged = selectedRace && selectedRace.grade !== raceEditGrade
    const isPointsBased = selectedEvent && selectedEvent.scoringType === 1

    if (isPointsBased && hasResults && gradeChanged) {
      setShowRaceGradeConfirm(true)
    } else {
      await performSaveRaceDetails()
    }
  }

  const onCreateRaceSubmit = async (e: React.FormEvent) => {
    e.preventDefault()
    await handleCreateRace(e)
    setShowCreateRaceModal(false)
  }

  return (
    <AdminLayout
      title="Event & Race Operations"
      subtitle="Manage event lifecycle, register competitors, construct race tracks, and perform dynamic batch or in-place results entry."
      actions={
        <Link
          to="/admin/events"
          className="slds-button slds-button_neutral"
          style={{ padding: '4px 12px', fontSize: '12px' }}
        >
          &larr; Back to Events
        </Link>
      }
    >
      {globalError && (
        <AlertBanner variant="error">{globalError}</AlertBanner>
      )}
      {globalSuccess && (
        <AlertBanner variant="success">{globalSuccess}</AlertBanner>
      )}

      {/* Event Onboarding Guidance Banners */}
      {!loadingEventDetail && selectedEvent && (
        <div className="slds-m-bottom_medium">
          {selectedEvent.status === 'DRAFT' && (
            <AlertBanner variant="warning">
              <div style={{ textAlign: 'left' }}>
                <strong>Event Status: DRAFT (Setup Mode)</strong>
                <p style={{ margin: '4px 0 0 0', fontSize: '12px', color: '#7c2d12' }}>
                  This event is in private preparation. <strong>Next steps:</strong> Register participants, configure race tracks, and set Lifecycle Status to <strong>PENDING</strong> to publish it.
                </p>
              </div>
            </AlertBanner>
          )}
          {selectedEvent.status === 'PENDING' && (
            <AlertBanner variant="warning">
              <div style={{ textAlign: 'left' }}>
                <strong>Event Status: PENDING (Published / Scheduled)</strong>
                <p style={{ margin: '4px 0 0 0', fontSize: '12px', color: '#7c2d12' }}>
                  This event is published and open for signups. Set status to <strong>ONGOING</strong> when races begin, or <strong>CONCLUDED</strong> when finished.
                </p>
              </div>
            </AlertBanner>
          )}
          {selectedEvent.status === 'ONGOING' && (
            <AlertBanner variant="success">
              <div style={{ textAlign: 'left' }}>
                <strong>Event Status: ONGOING (Live Competition)</strong>
                <p style={{ margin: '4px 0 0 0', fontSize: '12px', color: '#ffffff' }}>
                  Races are currently active. Record standings under the "Races & Tracks" tab and set status to <strong>CONCLUDED</strong> when finished.
                </p>
              </div>
            </AlertBanner>
          )}
          {selectedEvent.status === 'CONCLUDED' && (
            <AlertBanner variant="warning">
              <div style={{ textAlign: 'left' }}>
                <strong>Event Status: CONCLUDED (Locked)</strong>
                <p style={{ margin: '4px 0 0 0', fontSize: '12px', color: '#7c2d12' }}>
                  This competition has finished. Historical standings are finalized.
                </p>
              </div>
            </AlertBanner>
          )}
          {selectedEvent.status === 'PENDING_DELETION' && (
            <AlertBanner variant="error">
              <div style={{ textAlign: 'left', display: 'flex', justifyContent: 'space-between', alignItems: 'center' }}>
                <div>
                  <strong>Event Status: PENDING DELETION (Queued for Removal)</strong>
                  <p style={{ margin: '4px 0 0 0', fontSize: '12px', color: '#991b1b' }}>
                    This event is soft-deleted and will be permanently purged in 7 days.
                  </p>
                </div>
                <div style={{ display: 'flex', gap: '8px' }}>
                  <button
                    type="button"
                    onClick={() => void handleRestoreEvent()}
                    className="slds-button slds-button_brand"
                    style={{ fontSize: '11px', padding: '2px 10px' }}
                  >
                    Restore Event
                  </button>
                  <button
                    type="button"
                    onClick={() => {
                      if (window.confirm('Are you sure you want to permanently delete this event? This cannot be undone.')) {
                        void handleDeleteEvent(true)
                      }
                    }}
                    className="slds-button slds-button_destructive"
                    style={{ fontSize: '11px', padding: '2px 10px' }}
                  >
                    Delete Permanently
                  </button>
                </div>
              </div>
            </AlertBanner>
          )}
        </div>
      )}

      {loadingEventDetail ? (
        <SldsSkeletonDetail />
      ) : selectedEvent ? (
        <div className="slds-box bg-white" style={{ background: '#ffffff', borderRadius: '4px', border: '1px solid #dddbda', padding: '1.5rem' }}>
          <div className="slds-grid slds-grid_align-spread slds-m-bottom_large" style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'flex-start', flexWrap: 'wrap', gap: '16px', borderBottom: '2px solid #dddbda', paddingBottom: '1rem' }}>
            <div style={{ flex: '1 1 auto', minWidth: 0 }}>
              <div style={{ display: 'flex', alignItems: 'center', gap: '12px', minWidth: 0 }}>
                <h2 className="slds-text-heading_medium font-bold text-slate-900" title={selectedEvent.name} style={{ fontSize: '1.35rem', fontWeight: 'bold', margin: 0, flex: '1 1 auto', minWidth: 0, overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap' }}>{selectedEvent.name}</h2>
                <button
                  type="button"
                  onClick={() => setShowEditEventModal(true)}
                  className="slds-button slds-button_neutral"
                  style={{ padding: '2px 8px', fontSize: '11px', flexShrink: 0 }}
                >
                  Edit Details
                </button>
                {selectedEvent.scoringType === 1 && (
                  <button
                    type="button"
                    onClick={() => void handleRecomputeEventPoints()}
                    className="slds-button slds-button_neutral"
                    style={{ padding: '2px 8px', fontSize: '11px', flexShrink: 0 }}
                  >
                    Recompute Points
                  </button>
                )}
                {selectedEvent.status !== 'PENDING_DELETION' && (
                  <button
                    type="button"
                    onClick={() => {
                      if (window.confirm('Move this event to the Pending Deletion queue? It will be deleted in 7 days.')) {
                        void handleDeleteEvent(false)
                      }
                    }}
                    className="slds-button slds-button_destructive"
                    style={{ padding: '2px 8px', fontSize: '11px', flexShrink: 0 }}
                  >
                    Delete Event
                  </button>
                )}
              </div>
              <p className="slds-text-body_small text-slate-500">ID: {selectedEvent.id}</p>
            </div>

            <div style={{ display: 'flex', alignItems: 'center', gap: '20px', flexShrink: 0, flexWrap: 'wrap', justifyContent: 'flex-end' }}>
              {/* Signups Lock Controller */}
              <div className="slds-form-element" style={{ display: 'flex', alignItems: 'center', gap: '8px' }}>
                <label className="slds-form-element__label font-bold text-slate-700" style={{ fontWeight: 'bold', margin: 0 }}>Signups:</label>
                <button
                  type="button"
                  disabled={signupsLockedSaving}
                  onClick={() => void handleSetSignupsLocked(!selectedEvent.signupsLocked)}
                  className={`slds-button ${selectedEvent.signupsLocked ? 'slds-button_destructive' : 'slds-button_brand'}`}
                  style={{
                    padding: '4px 12px',
                    fontSize: '12px',
                    height: '32px',
                    borderRadius: '4px',
                    backgroundColor: selectedEvent.signupsLocked ? '#c23934' : '#0176d3',
                    color: '#ffffff',
                    border: 'none',
                    cursor: 'pointer'
                  }}
                >
                  {selectedEvent.signupsLocked ? 'Signups Locked' : 'Signups Open'}
                </button>
              </div>

              {/* Tag Controller */}
              <div className="slds-form-element" style={{ display: 'flex', alignItems: 'center', gap: '8px' }}>
                <label className="slds-form-element__label font-bold text-slate-700" style={{ fontWeight: 'bold', margin: 0 }}>Tag:</label>
                <div className="slds-form-element__control">
                  <select
                    disabled={eventStatusSaving}
                    value={selectedEvent.tag || 'OFFICIAL'}
                    onChange={(e) => void handleUpdateEventStatus({ tag: e.target.value as EventTag })}
                    className="slds-select"
                    style={{ minWidth: '110px', padding: '4px 24px 4px 8px', border: '1px solid #dddbda', borderRadius: '4px' }}
                  >
                    {TAG_OPTIONS.map((tag) => (
                      <option key={tag} value={tag}>
                        {tag}
                      </option>
                    ))}
                  </select>
                </div>
              </div>

              {/* Status controller */}
              <div className="slds-form-element" style={{ display: 'flex', alignItems: 'center', gap: '8px' }}>
                <label className="slds-form-element__label font-bold text-slate-700" style={{ fontWeight: 'bold', margin: 0 }}>Lifecycle Status:</label>
                <div className="slds-form-element__control">
                  <select
                    disabled={eventStatusSaving}
                    value={selectedEvent.status}
                    onChange={(e) => void handleUpdateEventStatus({ status: e.target.value as EventStatus })}
                    className="slds-select"
                    style={{ minWidth: '130px', padding: '4px 28px 4px 12px', border: '1px solid #dddbda', borderRadius: '4px' }}
                  >
                    {STATUS_OPTIONS.map((status) => (
                      <option key={status} value={status}>
                        {status}
                      </option>
                    ))}
                  </select>
                </div>
              </div>
            </div>
          </div>

          {/* SLDS Tabs Secondary Context Header */}
          <div className="slds-tabs_default slds-m-bottom_large">
            <ul className="slds-tabs_default__nav" role="tablist" style={{ display: 'flex', borderBottom: '1px solid #dddbda', listStyle: 'none', margin: 0, padding: 0 }}>
              <li className={`slds-tabs_default__item ${activeTab === 'details' ? 'slds-is-active' : ''}`} role="presentation" style={{ borderBottom: activeTab === 'details' ? '3px solid #0176d3' : 'none' }}>
                <button
                  className="slds-tabs_default__link"
                  type="button"
                  onClick={() => {
                    setActiveTab('details')
                  }}
                  style={{ border: 'none', background: 'transparent', padding: '12px 16px', cursor: 'pointer', fontWeight: activeTab === 'details' ? 'bold' : 'normal', color: activeTab === 'details' ? '#0176d3' : '#180505' }}
                >
                  Event Summary
                </button>
              </li>
              <li className={`slds-tabs_default__item ${activeTab === 'members' ? 'slds-is-active' : ''}`} role="presentation" style={{ borderBottom: activeTab === 'members' ? '3px solid #0176d3' : 'none' }}>
                <button
                  className="slds-tabs_default__link"
                  type="button"
                  onClick={() => {
                    setActiveTab('members')
                  }}
                  style={{ border: 'none', background: 'transparent', padding: '12px 16px', cursor: 'pointer', fontWeight: activeTab === 'members' ? 'bold' : 'normal', color: activeTab === 'members' ? '#0176d3' : '#180505' }}
                >
                  Event Members ({selectedEvent.members.length})
                </button>
              </li>
              <li className={`slds-tabs_default__item ${activeTab === 'races' ? 'slds-is-active' : ''}`} role="presentation" style={{ borderBottom: activeTab === 'races' ? '3px solid #0176d3' : 'none' }}>
                <button
                  className="slds-tabs_default__link"
                  type="button"
                  onClick={() => {
                    setActiveTab('races')
                  }}
                  style={{ border: 'none', background: 'transparent', padding: '12px 16px', cursor: 'pointer', fontWeight: activeTab === 'races' ? 'bold' : 'normal', color: activeTab === 'races' ? '#0176d3' : '#180505' }}
                >
                  Races & Tracks ({races.length})
                </button>
              </li>
              <li className={`slds-tabs_default__item ${activeTab === 'datasets' ? 'slds-is-active' : ''}`} role="presentation" style={{ borderBottom: activeTab === 'datasets' ? '3px solid #0176d3' : 'none' }}>
                <button
                  className="slds-tabs_default__link"
                  type="button"
                  onClick={() => {
                    setActiveTab('datasets')
                  }}
                  style={{ border: 'none', background: 'transparent', padding: '12px 16px', cursor: 'pointer', fontWeight: activeTab === 'datasets' ? 'bold' : 'normal', color: activeTab === 'datasets' ? '#0176d3' : '#180505' }}
                >
                  Datasets
                </button>
              </li>
            </ul>

            {/* Tab 1: Summary Info */}
            {activeTab === 'details' && (
              <EventSummaryTab selectedEvent={selectedEvent} />
            )}

            {/* Tab 2: Registered Members */}
            {activeTab === 'members' && (
              <EventMembersTab
                selectedEvent={selectedEvent}
                newMemberUserId={newMemberUserId}
                setNewMemberUserId={setNewMemberUserId}
                handleAddMember={handleAddMember}
                handleRemoveMember={handleRemoveMember}
              />
            )}

            {/* Tab 3: Unified Races & Tracks Experience */}
            {activeTab === 'races' && (
              <EventRacesTab
                races={races}
                selectedRaceId={selectedRaceId}
                setSelectedRaceId={setSelectedRaceId}
                selectedRace={selectedRace}
                ongoingRaces={ongoingRaces}
                concludedRaces={concludedRaces}
                notStartedRaces={notStartedRaces}
                handleSelectRace={handleSelectRace}
                handleReorderRaces={handleReorderRaces}
                hasStartedOrConcludedRaces={hasStartedOrConcludedRaces}
                setShowCreateRaceModal={setShowCreateRaceModal}
                selectedEvent={selectedEvent}
                newRaceMemberUserId={newRaceMemberUserId}
                setNewRaceMemberUserId={setNewRaceMemberUserId}
                CLASS_TIER_OPTIONS={CLASS_TIER_OPTIONS}
                handleUpdateRace={handleUpdateRace}
                handleStartRace={handleStartRace}
                handleEndRace={handleEndRace}
                handleDeleteRace={handleDeleteRace}
                handleAddRaceMember={handleAddRaceMember}
                handleRemoveRaceMember={handleRemoveRaceMember}
                setShowEditRaceModal={setShowEditRaceModal}
                loadingResults={loadingResults}
                derivedStates={derivedStates}
                changeSummary={changeSummary}
                savingBatch={savingBatch}
                handleInferFinishTimes={handleInferFinishTimes}
                handleCancelStandingsEdit={handleCancelStandingsEdit}
                handleUnifiedSave={handleUnifiedSave}
                resetStandingsDraft={resetStandingsDraft}
                handleResultChange={handleResultChange}
                togglePendingDeletion={togglePendingDeletion}
                handleUndoRow={handleUndoRow}
              />
            )}

            {/* Tab 4: Datasets (Disabled Placeholder) */}
            {activeTab === 'datasets' && (
              <div className="slds-tabs_default__content slds-show slds-p-vertical_medium" style={{ paddingTop: '1.5rem' }}>
                <div className="slds-align_absolute-center slds-p-around_large text-slate-500" style={{ textAlign: 'center', display: 'flex', flexDirection: 'column', alignItems: 'center', justifyContent: 'center', minHeight: '300px' }}>
                  <p className="slds-text-heading_small font-bold text-slate-700" style={{ fontWeight: 'bold' }}>Importing records is coming soon</p>
                  <p className="slds-text-body_regular text-slate-500 slds-m-top_xx-small">
                    Dataset import formats are still being finalized.
                  </p>
                </div>
              </div>
            )}
          </div>
        </div>
      ) : (
        <div className="slds-box slds-align_absolute-center bg-white" style={{ background: '#ffffff', borderRadius: '4px', border: '1px solid #dddbda', minHeight: '400px', display: 'flex', justifyContent: 'center', alignItems: 'center' }}>
          <div className="slds-text-align_center">
            <p className="slds-text-heading_medium font-bold text-slate-700 slds-m-top_medium" style={{ fontWeight: 'bold' }}>
              Event Not Found
            </p>
            <p className="slds-text-body_regular text-slate-500 slds-m-top_xx-small">
              The requested event could not be loaded. It may have been deleted.
            </p>
            <Link
              to="/admin/events"
              className="slds-button slds-button_brand slds-m-top_medium"
              style={{ padding: '6px 16px' }}
            >
              &larr; Back to Events
            </Link>
          </div>
        </div>
      )}

      {/* RACE CREATION DIALOG MODAL */}
      {showCreateRaceModal && (
        <div className="slds-scope">
          <section role="dialog" tabIndex={-1} aria-modal="true" className="slds-modal slds-fade-in-open" style={{ zIndex: 9001 }}>
            <div className="slds-modal__container" style={{ maxWidth: '40rem', width: '90%' }}>
              <header className="slds-modal__header">
                <button
                  className="slds-button slds-button_icon slds-modal__close"
                  title="Close"
                  onClick={() => setShowCreateRaceModal(false)}
                  style={{
                    position: 'absolute',
                    top: '1rem',
                    right: '1.5rem',
                    background: 'transparent',
                    border: 'none',
                    fontSize: '1.25rem',
                    cursor: 'pointer',
                    color: '#747474',
                  }}
                >
                  ✕
                </button>
                <h2 className="slds-modal__title slds-hyphenate font-bold text-slate-900" style={{ fontSize: '1.25rem', fontWeight: 'bold' }}>
                  Configure New Race Track
                </h2>
              </header>

              <form onSubmit={onCreateRaceSubmit}>
                <div className="slds-modal__content slds-p-around_medium" style={{ background: '#fff' }}>
                  <div className="slds-form slds-form_stacked">
                    {/* Helper text explaining the auto-append behavior */}
                    <p className="slds-m-bottom_medium" style={{ fontSize: '13px', color: '#57606a', fontStyle: 'italic' }}>
                      New races are added to the end of the event schedule. You can reorder them later from the race list.
                    </p>

                    {/* Race Name */}
                    <div className="slds-form-element slds-m-bottom_medium">
                      <label className="slds-form-element__label font-bold text-slate-700" style={{ fontWeight: 'bold' }} htmlFor="race-name">
                        Race Name <span className="text-red-500">*</span>
                      </label>
                      <div className="slds-form-element__control">
                        <input
                          id="race-name"
                          type="text"
                          required
                          placeholder="e.g. Kyoto Derby"
                          value={newRaceForm.name}
                          onChange={(e) => setNewRaceForm((c) => ({ ...c, name: e.target.value }))}
                          className="slds-input"
                          style={{ padding: '6px 12px', border: '1px solid #dddbda', borderRadius: '4px', width: '100%' }}
                        />
                      </div>
                    </div>

                    <div className="slds-grid slds-gutters slds-wrap" style={{ display: 'flex', gap: '16px', marginBottom: '1rem' }}>
                      <div className="slds-col slds-size_1-of-1" style={{ flex: 1 }}>
                        <div className="slds-form-element">
                          <label className="slds-form-element__label font-bold text-slate-700" style={{ fontWeight: 'bold' }} htmlFor="race-distance">
                            Distance (Meters)
                          </label>
                          <div className="slds-form-element__control">
                            <input
                              id="race-distance"
                              type="number"
                              value={newRaceForm.distanceMeters}
                              onChange={(e) => setNewRaceForm((c) => ({ ...c, distanceMeters: Number(e.target.value) || 1200 }))}
                              className="slds-input"
                              style={{ padding: '6px 12px', border: '1px solid #dddbda', borderRadius: '4px', width: '100%' }}
                            />
                          </div>
                        </div>
                      </div>
                    </div>

                    <div className="slds-grid slds-gutters slds-wrap" style={{ display: 'flex', gap: '16px', marginBottom: '1rem' }}>
                      <div className="slds-col slds-size_1-of-1 slds-medium-size_1-of-4" style={{ flex: 1 }}>
                        <div className="slds-form-element">
                          <label className="slds-form-element__label font-bold text-slate-700" style={{ fontWeight: 'bold' }} htmlFor="race-track">
                            Track Type
                          </label>
                          <div className="slds-form-element__control">
                            <input
                              id="race-track"
                              type="text"
                              placeholder="e.g. Turf, Dirt"
                              value={newRaceForm.trackType}
                              onChange={(e) => setNewRaceForm((c) => ({ ...c, trackType: e.target.value }))}
                              className="slds-input"
                              style={{ padding: '6px 12px', border: '1px solid #dddbda', borderRadius: '4px', width: '100%' }}
                            />
                          </div>
                        </div>
                      </div>

                      <div className="slds-col slds-size_1-of-1 slds-medium-size_1-of-4" style={{ flex: 1 }}>
                        <div className="slds-form-element">
                          <label className="slds-form-element__label font-bold text-slate-700" style={{ fontWeight: 'bold' }} htmlFor="race-loc">
                            Location <span className="text-red-500">*</span>
                          </label>
                          <div className="slds-form-element__control">
                            <input
                              id="race-loc"
                              type="text"
                              required
                              placeholder="e.g. Kyoto Racecourse"
                              value={newRaceForm.location}
                              onChange={(e) => setNewRaceForm((c) => ({ ...c, location: e.target.value }))}
                              className="slds-input"
                              style={{ padding: '6px 12px', border: '1px solid #dddbda', borderRadius: '4px', width: '100%' }}
                            />
                          </div>
                        </div>
                      </div>

                      <div className="slds-col slds-size_1-of-1 slds-medium-size_1-of-4" style={{ flex: 1 }}>
                        <div className="slds-form-element">
                          <label className="slds-form-element__label font-bold text-slate-700" style={{ fontWeight: 'bold' }} htmlFor="race-restriction">
                            Class Restriction
                          </label>
                          <div className="slds-form-element__control">
                            <select
                              id="race-restriction"
                              value={newRaceForm.classRestriction || ''}
                              onChange={(e) => setNewRaceForm((c) => ({ ...c, classRestriction: e.target.value ? e.target.value as ClassTier : null }))}
                              className="slds-select"
                              style={{ padding: '6px 12px', border: '1px solid #dddbda', borderRadius: '4px', width: '100%' }}
                            >
                              <option value="">Any Tier Eligibility (None)</option>
                              {CLASS_TIER_OPTIONS.map((tier) => (
                                <option key={tier} value={tier}>{tier}</option>
                              ))}
                            </select>
                          </div>
                        </div>
                      </div>

                      {selectedEvent && selectedEvent.scoringType === 1 && (
                        <div className="slds-col slds-size_1-of-1 slds-medium-size_1-of-4" style={{ flex: 1 }}>
                          <div className="slds-form-element">
                            <label className="slds-form-element__label font-bold text-slate-700" style={{ fontWeight: 'bold' }} htmlFor="race-grade">
                              Race Grade
                            </label>
                            <div className="slds-form-element__control">
                              <select
                                id="race-grade"
                                value={newRaceForm.grade || ''}
                                onChange={(e) => setNewRaceForm((c) => ({ ...c, grade: e.target.value }))}
                                className="slds-select"
                                style={{ padding: '6px 12px', border: '1px solid #dddbda', borderRadius: '4px', width: '100%' }}
                              >
                                <option value="">-- Choose Grade --</option>
                                <option value="OP">OP</option>
                                <option value="GIII">GIII</option>
                                <option value="GII">GII</option>
                                <option value="GI">GI</option>
                              </select>
                            </div>
                          </div>
                        </div>
                      )}
                    </div>

                    {/* Optional Race participantLimit */}
                    {selectedEvent && selectedEvent.granularParticipation && (
                      <div className="slds-form-element slds-m-bottom_medium" style={{ borderTop: '1px solid #dddbda', paddingTop: '1rem' }}>
                        <label className="slds-form-element__label font-bold text-slate-700" style={{ fontWeight: 'bold' }} htmlFor="race-participant-limit">
                          Race participant limit
                        </label>
                        <div className="slds-form-element__control">
                          <input
                            id="race-participant-limit"
                            type="number"
                            min="1"
                            placeholder="e.g. 10"
                            value={newRaceForm.participantLimit !== null ? String(newRaceForm.participantLimit) : ''}
                            onChange={(e) => {
                              const val = e.target.value
                              setNewRaceForm((c) => ({ ...c, participantLimit: val ? Number(val) : null }))
                            }}
                            className="slds-input"
                            style={{ padding: '6px 12px', border: '1px solid #dddbda', borderRadius: '4px', width: '100%' }}
                          />
                        </div>
                        <p className="slds-text-body_small text-slate-500" style={{ fontSize: '11px', margin: '4px 0 0 0' }}>
                          Maximum participants for this race. Leave blank for unlimited.
                        </p>
                      </div>
                    )}
                  </div>
                </div>

                <footer className="slds-modal__footer" style={{ display: 'flex', justifyContent: 'flex-end', gap: '8px' }}>
                  <button
                    type="button"
                    onClick={() => setShowCreateRaceModal(false)}
                    className="slds-button slds-button_neutral"
                  >
                    Cancel
                  </button>
                  <button
                    type="submit"
                    className="slds-button slds-button_brand"
                  >
                    Create Race Track
                  </button>
                </footer>
              </form>
            </div>
          </section>
          <div className="slds-backdrop slds-backdrop_open" style={{ zIndex: 9000 }} />
        </div>
      )}

      {/* EDIT EVENT DETAILS DIALOG MODAL */}
      {showEditEventModal && (
        <div className="slds-scope">
          <section role="dialog" tabIndex={-1} aria-modal="true" className="slds-modal slds-fade-in-open" style={{ zIndex: 9001 }}>
            <div className="slds-modal__container" style={{ maxWidth: '40rem', width: '90%' }}>
              <header className="slds-modal__header">
                <button
                  className="slds-button slds-button_icon slds-modal__close"
                  title="Close"
                  onClick={() => setShowEditEventModal(false)}
                  style={{
                    position: 'absolute',
                    top: '1rem',
                    right: '1.5rem',
                    background: 'transparent',
                    border: 'none',
                    fontSize: '1.25rem',
                    cursor: 'pointer',
                    color: '#747474',
                  }}
                >
                  ✕
                </button>
                <h2 className="slds-modal__title slds-hyphenate font-bold text-slate-900" style={{ fontSize: '1.25rem', fontWeight: 'bold' }}>
                  Edit Event Details
                </h2>
              </header>

              <form onSubmit={onEditEventSubmit}>
                <div className="slds-modal__content slds-p-around_medium" style={{ background: '#fff', maxHeight: '70vh', overflowY: 'auto' }}>
                  <div className="slds-form slds-form_stacked">
                    {/* Event Name */}
                    <div className="slds-form-element slds-m-bottom_medium">
                      <label className="slds-form-element__label font-bold text-slate-700" style={{ fontWeight: 'bold' }} htmlFor="edit-event-name">
                        Event Name <span className="text-red-500">*</span>
                      </label>
                      <div className="slds-form-element__control">
                        <input
                          id="edit-event-name"
                          type="text"
                          required
                          placeholder="e.g. Winter Derby Championship"
                          value={editName}
                          onChange={(e) => setEditName(e.target.value)}
                          className="slds-input"
                          style={{ padding: '6px 12px', border: '1px solid #dddbda', borderRadius: '4px', width: '100%' }}
                        />
                      </div>
                    </div>

                    {/* Description */}
                    <div className="slds-form-element slds-m-bottom_medium">
                      <label className="slds-form-element__label font-bold text-slate-700" style={{ fontWeight: 'bold' }} htmlFor="edit-event-desc">
                        Description
                      </label>
                      <div className="slds-form-element__control">
                        <textarea
                          id="edit-event-desc"
                          placeholder="Brief description..."
                          value={editDescription}
                          onChange={(e) => setEditDescription(e.target.value)}
                          className="slds-textarea"
                          style={{ padding: '6px 12px', border: '1px solid #dddbda', borderRadius: '4px', width: '100%', minHeight: '80px' }}
                        />
                      </div>
                    </div>

                    {selectedEvent && selectedEvent.scoringType === 1 && (
                      <div className="slds-form-element slds-m-bottom_medium" style={{ borderTop: '1px solid #dddbda', paddingTop: '1rem' }}>
                        <label className="slds-form-element__label font-bold text-slate-700" style={{ fontWeight: 'bold' }} htmlFor="edit-scoring-rules-mode">
                          Points Scoring Rules Source
                        </label>
                        <div className="slds-form-element__control">
                          <select
                            id="edit-scoring-rules-mode"
                            value={editScoringRulesMode}
                            onChange={(e) => setEditScoringRulesMode(e.target.value as 'STANDARD' | 'CUSTOM')}
                            className="slds-select"
                            style={{ padding: '6px 12px', border: '1px solid #dddbda', borderRadius: '4px', width: '100%' }}
                          >
                            <option value="STANDARD">Standard Default Tables</option>
                            <option value="CUSTOM">Custom Event Tables (Configure below)</option>
                          </select>
                        </div>

                        {editScoringRulesMode === 'CUSTOM' && (
                          <div className="slds-m-top_medium">
                            <EventScoringTablesEditor
                              value={editCustomScoringTables}
                              onChange={setEditCustomScoringTables}
                            />
                          </div>
                        )}
                      </div>
                    )}

                    {/* Optional Event Date/Time */}
                    <div className="slds-form-element slds-m-bottom_medium" style={{ borderTop: '1px solid #dddbda', paddingTop: '1rem' }}>
                      <label className="slds-form-element__label font-bold text-slate-700" style={{ fontWeight: 'bold' }} htmlFor="edit-event-scheduled-at">
                        Scheduled Date / Time ({Intl.DateTimeFormat().resolvedOptions().timeZone})
                      </label>
                      <div className="slds-form-element__control">
                        <input
                          id="edit-event-scheduled-at"
                          type="datetime-local"
                          value={editScheduledAt}
                          onChange={(e) => setEditScheduledAt(e.target.value)}
                          className="slds-input"
                          style={{ padding: '6px 12px', border: '1px solid #dddbda', borderRadius: '4px', width: '100%' }}
                        />
                      </div>
                    </div>

                    <div className="slds-grid slds-gutters slds-wrap" style={{ display: 'flex', gap: '16px', marginBottom: '1rem', borderTop: '1px solid #dddbda', paddingTop: '1rem' }}>
                      <div className="slds-col slds-size_1-of-1 slds-medium-size_1-of-2" style={{ flex: 1 }}>
                        <div className="slds-form-element">
                          <label className="slds-form-element__label font-bold text-slate-700" style={{ fontWeight: 'bold' }} htmlFor="edit-class-tier">
                            Class Tier Eligibility
                          </label>
                          <div className="slds-form-element__control">
                            <select
                              id="edit-class-tier"
                              value={editClassRestriction || ''}
                              onChange={(e) => setEditClassRestriction(e.target.value ? e.target.value as ClassTier : null)}
                              className="slds-select"
                              style={{ padding: '6px 12px', border: '1px solid #dddbda', borderRadius: '4px', width: '100%' }}
                            >
                              <option value="">Any Tier Eligibility (None)</option>
                              <option value="G3">G3</option>
                              <option value="G2">G2</option>
                              <option value="G1">G1</option>
                            </select>
                          </div>
                        </div>
                      </div>

                      <div className="slds-col slds-size_1-of-1 slds-medium-size_1-of-2" style={{ flex: 1, display: 'flex', flexDirection: 'column', gap: '12px', justifyContent: 'center' }}>
                        <div className="slds-form-element">
                          <label className="slds-form-element__label font-bold text-slate-700" style={{ fontWeight: 'bold' }}>Participation Model</label>
                          <div className="slds-form-element__control">
                            <span className="slds-badge slds-theme_light" style={{ fontSize: '12px', padding: '4px 12px' }}>
                              {editGranularParticipation ? 'Granular (Per-Race)' : 'Regular (Event-wide)'}
                            </span>
                          </div>
                        </div>

                        <div className="slds-form-element">
                          <div className="slds-form-element__control">
                            <div className="slds-checkbox">
                              <input
                                type="checkbox"
                                id="edit-signups-locked"
                                checked={editSignupsLocked}
                                onChange={(e) => setEditSignupsLocked(e.target.checked)}
                                style={{ marginRight: '8px' }}
                              />
                              <label className="slds-checkbox__label font-bold text-slate-700" style={{ fontWeight: 'bold' }} htmlFor="edit-signups-locked">
                                <span className="slds-checkbox_faux"></span>
                                <span className="slds-form-element__label">Lock Event Signups</span>
                              </label>
                            </div>
                          </div>
                        </div>
                      </div>
                    </div>

                    {/* Capacity and Limits fields */}
                    {!editGranularParticipation ? (
                      <div className="slds-form-element slds-m-bottom_medium" style={{ borderTop: '1px solid #dddbda', paddingTop: '1rem' }}>
                        <label className="slds-form-element__label font-bold text-slate-700" style={{ fontWeight: 'bold' }} htmlFor="edit-participant-limit">
                          Participant limit
                        </label>
                        <div className="slds-form-element__control">
                          <input
                            id="edit-participant-limit"
                            type="number"
                            min="1"
                            placeholder="e.g. 20"
                            value={editParticipantLimit}
                            onChange={(e) => setEditParticipantLimit(e.target.value)}
                            className="slds-input"
                            style={{ padding: '6px 12px', border: '1px solid #dddbda', borderRadius: '4px', width: '100%' }}
                          />
                        </div>
                        <p className="slds-text-body_small text-slate-500" style={{ fontSize: '11px', margin: '4px 0 0 0' }}>
                          Maximum participants for the whole event. Leave blank for unlimited.
                        </p>
                      </div>
                    ) : (
                      <div className="slds-form-element slds-m-bottom_medium" style={{ borderTop: '1px solid #dddbda', paddingTop: '1rem' }}>
                        <label className="slds-form-element__label font-bold text-slate-700" style={{ fontWeight: 'bold' }} htmlFor="edit-max-concurrent-participations">
                          Max races per participant
                        </label>
                        <div className="slds-form-element__control">
                          <input
                            id="edit-max-concurrent-participations"
                            type="number"
                            min="1"
                            placeholder="e.g. 3"
                            value={editMaxConcurrentRaceParticipations}
                            onChange={(e) => setEditMaxConcurrentRaceParticipations(e.target.value)}
                            className="slds-input"
                            style={{ padding: '6px 12px', border: '1px solid #dddbda', borderRadius: '4px', width: '100%' }}
                          />
                        </div>
                        <p className="slds-text-body_small text-slate-500" style={{ fontSize: '11px', margin: '4px 0 0 0' }}>
                          Maximum races one participant may join in this event. Leave blank for unlimited.
                        </p>
                      </div>
                    )}
                  </div>
                </div>

                <footer className="slds-modal__footer" style={{ display: 'flex', justifyContent: 'flex-end', gap: '8px' }}>
                  <button
                    type="button"
                    onClick={() => setShowEditEventModal(false)}
                    className="slds-button slds-button_neutral"
                  >
                    Cancel
                  </button>
                  <button
                    type="submit"
                    className="slds-button slds-button_brand"
                  >
                    Save Changes
                  </button>
                </footer>
              </form>
            </div>
          </section>
          <div className="slds-backdrop slds-backdrop_open" style={{ zIndex: 9000 }} />
        </div>
      )}

      {/* CONFIRM RECOMPUTE WARNING DIALOG MODAL */}
      {showConfirmModal && (
        <div className="slds-scope">
          <section role="dialog" tabIndex={-1} aria-modal="true" className="slds-modal slds-fade-in-open" style={{ zIndex: 10001 }}>
            <div className="slds-modal__container" style={{ maxWidth: '30rem', width: '90%' }}>
              <header className="slds-modal__header slds-theme_warning slds-theme_alert-texture" style={{ backgroundColor: '#f57c00', color: '#fff' }}>
                <button
                  className="slds-button slds-button_icon slds-modal__close"
                  title="Close"
                  onClick={() => setShowConfirmModal(false)}
                  style={{
                    position: 'absolute',
                    top: '1rem',
                    right: '1.5rem',
                    background: 'transparent',
                    border: 'none',
                    fontSize: '1.25rem',
                    cursor: 'pointer',
                    color: '#fff',
                  }}
                >
                  ✕
                </button>
                <h2 className="slds-modal__title slds-hyphenate font-bold text-white" style={{ fontSize: '1.25rem', fontWeight: 'bold', color: '#fff' }}>
                  Recalculate Points Warning
                </h2>
              </header>

              <div className="slds-modal__content slds-p-around_medium" style={{ background: '#fff' }}>
                <p style={{ fontSize: '14px', lineHeight: '1.5', color: '#1e293b' }}>
                  Changing the event's scoring tables will trigger an <strong>automatic background recomputation</strong> of points for all existing race results associated with this event.
                </p>
                <p className="slds-m-top_small" style={{ fontSize: '13px', lineHeight: '1.5', color: '#e11d48', fontWeight: 'bold' }}>
                  ⚠️ This can invalidate previously computed points on recorded standings.
                </p>
                <p className="slds-m-top_small" style={{ fontSize: '14px', lineHeight: '1.5', color: '#1e293b' }}>
                  Are you absolutely sure you want to proceed and save these changes?
                </p>
              </div>

              <footer className="slds-modal__footer" style={{ display: 'flex', justifyContent: 'flex-end', gap: '8px' }}>
                <button
                  type="button"
                  onClick={() => setShowConfirmModal(false)}
                  className="slds-button slds-button_neutral"
                >
                  Cancel
                </button>
                <button
                  type="button"
                  onClick={async () => {
                    setShowConfirmModal(false)
                    await performSaveEventDetails()
                  }}
                  className="slds-button slds-button_brand"
                  style={{ backgroundColor: '#0176d3', color: '#fff' }}
                >
                  Confirm & Save
                </button>
              </footer>
            </div>
          </section>
          <div className="slds-backdrop slds-backdrop_open" style={{ zIndex: 10000 }} />
        </div>
      )}

      {/* EDIT RACE DIALOG MODAL */}
      {showEditRaceModal && selectedRace && (
        <div className="slds-scope">
          <section role="dialog" tabIndex={-1} aria-modal="true" className="slds-modal slds-fade-in-open" style={{ zIndex: 9001 }}>
            <div className="slds-modal__container" style={{ maxWidth: '40rem', width: '90%' }}>
              <header className="slds-modal__header">
                <button
                  className="slds-button slds-button_icon slds-modal__close"
                  title="Close"
                  onClick={() => setShowEditRaceModal(false)}
                  style={{
                    position: 'absolute',
                    top: '1rem',
                    right: '1.5rem',
                    background: 'transparent',
                    border: 'none',
                    fontSize: '1.25rem',
                    cursor: 'pointer',
                    color: '#747474',
                  }}
                >
                  ✕
                </button>
                <h2 className="slds-modal__title slds-hyphenate font-bold text-slate-900" style={{ fontSize: '1.25rem', fontWeight: 'bold' }}>
                  Edit Race Details
                </h2>
              </header>

              <form onSubmit={onEditRaceSubmit}>
                <div className="slds-modal__content slds-p-around_medium" style={{ background: '#fff' }}>
                  {raceEditError && (
                    <div className="slds-m-bottom_medium">
                      <AlertBanner variant="error">{raceEditError}</AlertBanner>
                    </div>
                  )}

                  <div className="slds-form slds-form_stacked">
                    {/* Race Name */}
                    <div className="slds-form-element slds-m-bottom_medium">
                      <label className="slds-form-element__label font-bold text-slate-700" style={{ fontWeight: 'bold' }} htmlFor="edit-race-name">
                        Race Name <span className="text-red-500">*</span>
                      </label>
                      <div className="slds-form-element__control">
                        <input
                          id="edit-race-name"
                          type="text"
                          required
                          placeholder="e.g. Kyoto Derby"
                          value={raceEditName}
                          onChange={(e) => setRaceEditName(e.target.value)}
                          className="slds-input"
                          style={{ padding: '6px 12px', border: '1px solid #dddbda', borderRadius: '4px', width: '100%' }}
                        />
                      </div>
                    </div>

                    <div className="slds-grid slds-gutters slds-wrap" style={{ display: 'flex', gap: '16px', marginBottom: '1rem' }}>
                      <div className="slds-col slds-size_1-of-1" style={{ flex: 1 }}>
                        <div className="slds-form-element">
                          <label className="slds-form-element__label font-bold text-slate-700" style={{ fontWeight: 'bold' }} htmlFor="edit-race-distance">
                            Distance (Meters) <span className="text-red-500">*</span>
                          </label>
                          <div className="slds-form-element__control">
                            <input
                              id="edit-race-distance"
                              type="number"
                              required
                              value={raceEditDistance}
                              onChange={(e) => setRaceEditDistance(Number(e.target.value) || 0)}
                              className="slds-input"
                              style={{ padding: '6px 12px', border: '1px solid #dddbda', borderRadius: '4px', width: '100%' }}
                            />
                          </div>
                        </div>
                      </div>
                    </div>

                    <div className="slds-grid slds-gutters slds-wrap" style={{ display: 'flex', gap: '16px', marginBottom: '1rem' }}>
                      <div className="slds-col slds-size_1-of-1 slds-medium-size_1-4" style={{ flex: 1 }}>
                        <div className="slds-form-element">
                          <label className="slds-form-element__label font-bold text-slate-700" style={{ fontWeight: 'bold' }} htmlFor="edit-race-track">
                            Track Type <span className="text-red-500">*</span>
                          </label>
                          <div className="slds-form-element__control">
                            <input
                              id="edit-race-track"
                              type="text"
                              required
                              placeholder="e.g. Turf, Dirt"
                              value={raceEditTrackType}
                              onChange={(e) => setRaceEditTrackType(e.target.value)}
                              className="slds-input"
                              style={{ padding: '6px 12px', border: '1px solid #dddbda', borderRadius: '4px', width: '100%' }}
                            />
                          </div>
                        </div>
                      </div>

                      <div className="slds-col slds-size_1-of-1 slds-medium-size_1-4" style={{ flex: 1 }}>
                        <div className="slds-form-element">
                          <label className="slds-form-element__label font-bold text-slate-700" style={{ fontWeight: 'bold' }} htmlFor="edit-race-loc">
                            Location <span className="text-red-500">*</span>
                          </label>
                          <div className="slds-form-element__control">
                            <input
                              id="edit-race-loc"
                              type="text"
                              required
                              placeholder="e.g. Kyoto Racecourse"
                              value={raceEditLocation}
                              onChange={(e) => setRaceEditLocation(e.target.value)}
                              className="slds-input"
                              style={{ padding: '6px 12px', border: '1px solid #dddbda', borderRadius: '4px', width: '100%' }}
                            />
                          </div>
                        </div>
                      </div>

                      <div className="slds-col slds-size_1-of-1 slds-medium-size_1-4" style={{ flex: 1 }}>
                        <div className="slds-form-element">
                          <label className="slds-form-element__label font-bold text-slate-700" style={{ fontWeight: 'bold' }} htmlFor="edit-race-restriction">
                            Class Restriction
                          </label>
                          <div className="slds-form-element__control">
                            <select
                              id="edit-race-restriction"
                              value={raceEditClassRestriction || ''}
                              onChange={(e) => setRaceEditClassRestriction(e.target.value ? e.target.value as ClassTier : null)}
                              className="slds-select"
                              style={{ padding: '6px 12px', border: '1px solid #dddbda', borderRadius: '4px', width: '100%' }}
                            >
                              <option value="">Any Tier Eligibility (None)</option>
                              {CLASS_TIER_OPTIONS.map((tier) => (
                                <option key={tier} value={tier}>{tier}</option>
                              ))}
                            </select>
                          </div>
                        </div>
                      </div>

                      {selectedEvent && selectedEvent.scoringType === 1 && (
                        <div className="slds-col slds-size_1-of-1 slds-medium-size_1-4" style={{ flex: 1 }}>
                          <div className="slds-form-element">
                            <label className="slds-form-element__label font-bold text-slate-700" style={{ fontWeight: 'bold' }} htmlFor="edit-race-grade">
                              Race Grade
                            </label>
                            <div className="slds-form-element__control">
                              <select
                                id="edit-race-grade"
                                value={raceEditGrade || ''}
                                onChange={(e) => setRaceEditGrade(e.target.value || null)}
                                className="slds-select"
                                style={{ padding: '6px 12px', border: '1px solid #dddbda', borderRadius: '4px', width: '100%' }}
                              >
                                <option value="">-- Choose Grade --</option>
                                <option value="OP">OP</option>
                                <option value="GIII">GIII</option>
                                <option value="GII">GII</option>
                                <option value="GI">GI</option>
                              </select>
                            </div>
                          </div>
                        </div>
                      )}
                    </div>

                    {/* Edit Race participantLimit */}
                    {selectedEvent && selectedEvent.granularParticipation && (
                      <div className="slds-form-element slds-m-bottom_medium" style={{ borderTop: '1px solid #dddbda', paddingTop: '1rem' }}>
                        <label className="slds-form-element__label font-bold text-slate-700" style={{ fontWeight: 'bold' }} htmlFor="edit-race-participant-limit">
                          Race participant limit
                        </label>
                        <div className="slds-form-element__control">
                          <input
                            id="edit-race-participant-limit"
                            type="number"
                            min="1"
                            placeholder="e.g. 10"
                            value={raceEditParticipantLimit}
                            onChange={(e) => setRaceEditParticipantLimit(e.target.value)}
                            className="slds-input"
                            style={{ padding: '6px 12px', border: '1px solid #dddbda', borderRadius: '4px', width: '100%' }}
                          />
                        </div>
                        <p className="slds-text-body_small text-slate-500" style={{ fontSize: '11px', margin: '4px 0 0 0' }}>
                          Maximum participants for this race. Leave blank for unlimited.
                        </p>
                      </div>
                    )}
                  </div>
                </div>

                <footer className="slds-modal__footer" style={{ display: 'flex', justifyContent: 'flex-end', gap: '8px' }}>
                  <button
                    type="button"
                    onClick={() => setShowEditRaceModal(false)}
                    className="slds-button slds-button_neutral"
                  >
                    Cancel
                  </button>
                  <button
                    type="submit"
                    className="slds-button slds-button_brand"
                  >
                    Save Changes
                  </button>
                </footer>
              </form>
            </div>
          </section>
          <div className="slds-backdrop slds-backdrop_open" style={{ zIndex: 9000 }} />
        </div>
      )}

      {/* CONFIRM RACE GRADE CHANGE RECOMPUTE WARNING DIALOG MODAL */}
      {showRaceGradeConfirm && (
        <div className="slds-scope">
          <section role="dialog" tabIndex={-1} aria-modal="true" className="slds-modal slds-fade-in-open" style={{ zIndex: 10001 }}>
            <div className="slds-modal__container" style={{ maxWidth: '30rem', width: '90%' }}>
              <header className="slds-modal__header slds-theme_warning slds-theme_alert-texture" style={{ backgroundColor: '#f57c00', color: '#fff' }}>
                <button
                  className="slds-button slds-button_icon slds-modal__close"
                  title="Close"
                  onClick={() => setShowRaceGradeConfirm(false)}
                  style={{
                    position: 'absolute',
                    top: '1rem',
                    right: '1.5rem',
                    background: 'transparent',
                    border: 'none',
                    fontSize: '1.25rem',
                    cursor: 'pointer',
                    color: '#fff',
                  }}
                >
                  ✕
                </button>
                <h2 className="slds-modal__title slds-hyphenate font-bold text-white" style={{ fontSize: '1.25rem', fontWeight: 'bold', color: '#fff' }}>
                  Recalculate Race Points Warning
                </h2>
              </header>

              <div className="slds-modal__content slds-p-around_medium" style={{ background: '#fff' }}>
                <p style={{ fontSize: '14px', lineHeight: '1.5', color: '#1e293b' }}>
                  Changing this race's grade will trigger an <strong>automatic recomputation of points</strong> for all recorded results in this specific race.
                </p>
                <p className="slds-m-top_small" style={{ fontSize: '13px', lineHeight: '1.5', color: '#e11d48', fontWeight: 'bold' }}>
                  ⚠️ Existing results will be immediately recalculated based on the new grade's scoring table.
                </p>
                <p className="slds-m-top_small" style={{ fontSize: '14px', lineHeight: '1.5', color: '#1e293b' }}>
                  Are you sure you want to proceed and update the race grade?
                </p>
              </div>

              <footer className="slds-modal__footer" style={{ display: 'flex', justifyContent: 'flex-end', gap: '8px' }}>
                <button
                  type="button"
                  onClick={() => setShowRaceGradeConfirm(false)}
                  className="slds-button slds-button_neutral"
                >
                  Cancel
                </button>
                <button
                  type="button"
                  onClick={async () => {
                    setShowRaceGradeConfirm(false)
                    await performSaveRaceDetails()
                  }}
                  className="slds-button slds-button_brand"
                  style={{ backgroundColor: '#0176d3', color: '#fff' }}
                >
                  Confirm & Save
                </button>
              </footer>
            </div>
          </section>
          <div className="slds-backdrop slds-backdrop_open" style={{ zIndex: 10000 }} />
        </div>
      )}
    </AdminLayout>
  )
}

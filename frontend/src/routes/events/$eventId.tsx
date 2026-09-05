import { createFileRoute, Link } from '@tanstack/react-router'
import { useAuth } from '../../hooks/useAuth'
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { getPublicEvent, joinEvent, leaveEvent, listPublicRaceEvents, getPublicRaceResults, joinRaceEvent, leaveRaceEvent } from '../../lib/public-api'
import { formatLocalDateTime } from '../../lib/datetime'
import {
  PixelContainer,
  PixelStack,
  PixelCard,
  PixelButton,
  PixelBadge,
  PixelTable,
  PixelSpinner,
  PixelSectionHeader,
  PixelEmptyState,
  type PixelTableColumn,
  useToast,
} from '@pxlkit/ui-kit'
import type { eventmanager } from '../../lib/client'
import type { EventStatus, EventTag } from '../../types'
import { PixelSkeletonDetail } from '../../components/LoadingSkeleton'

const CLASS_TIER_LABELS: Record<string, string> = {
  PRE_OP: 'PRE-OP',
  OP: 'OP',
  G3: 'G3',
  G2: 'G2',
  G1: 'G1',
}

const SCORING_LABELS: Record<number, string> = {
  1: 'POINTS-BASED',
  2: 'LADDER-ELO',
}

const STATUS_LABELS: Record<EventStatus, string> = {
  DRAFT: 'Draft',
  PENDING: 'Pending',
  ONGOING: 'Ongoing',
  CONCLUDED: 'Concluded',
  PENDING_DELETION: 'Pending Deletion',
}

const TAG_LABELS: Record<EventTag, string> = {
  OFFICIAL: 'Official',
  COMMUNITY: 'Community',
}

const STATUS_TONE: Record<EventStatus, 'neutral' | 'cyan' | 'green' | 'pink'> = {
  DRAFT: 'neutral',
  PENDING: 'cyan',
  ONGOING: 'green',
  CONCLUDED: 'pink',
  PENDING_DELETION: 'neutral',
}

const TAG_TONE: Record<EventTag, 'purple' | 'cyan'> = {
  OFFICIAL: 'purple',
  COMMUNITY: 'cyan',
}

export const Route = createFileRoute('/events/$eventId')({
  component: EventDetailPage,
})

function EventDetailPage() {
  const { eventId } = Route.useParams()
  const queryClient = useQueryClient()
  const { session } = useAuth()

  const { data: event, isLoading, error } = useQuery({
    queryKey: ['public-event', eventId],
    queryFn: () => getPublicEvent(eventId),
  })

  const joinMutation = useMutation({
    mutationFn: (id: string) => joinEvent(id, `Bearer ${session?.session.token ?? ''}`),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['public-event', eventId] })
    },
  })

  const leaveMutation = useMutation({
    mutationFn: (id: string) => leaveEvent(id, `Bearer ${session?.session.token ?? ''}`),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['public-event', eventId] })
    },
  })

  if (isLoading) {
    return (
      <PixelContainer maxWidth="full" padding="md">
        <PixelSkeletonDetail />
      </PixelContainer>
    )
  }
  if (error) {
    return (
      <PixelContainer maxWidth="md" padding="md">
        <PixelEmptyState
          title="Error loading event"
          description="Something went wrong while fetching the event details."

        />
      </PixelContainer>
    )
  }

  if (!event) return null

  const isMember = session && event.members.some((m) => m.userId === session.user.id)
  const isConcluded = event.status === 'CONCLUDED'
  const isGranular = event.granularParticipation

  const participantColumns: PixelTableColumn<eventmanager.EventMemberView>[] = [
    { key: 'name', header: 'NAME', render: (m) => <span className="font-medium">{m.name}</span> },
    {
      key: 'classTier',
      header: 'CLASS TIER',
      render: (m) =>
        m.classTier ? (
          <PixelBadge tone="neutral">{CLASS_TIER_LABELS[m.classTier as any]}</PixelBadge>
        ) : (
          <span className="text-retro-muted">-</span>
        ),
    },
  ]

  const pointsColumns: PixelTableColumn<eventmanager.PointsEntryView>[] = [
    { key: 'rank', header: '#', width: 64, render: (_e, idx) => idx + 1 },
    { key: 'name', header: 'PARTICIPANT', render: (e) => <span className="font-medium">{e.name}</span> },
    {
      key: 'points',
      header: 'TOTAL POINTS',
      align: 'right',
      width: 128,
      render: (e) => <span className="text-retro-primary">{e.points}</span>,
    },
  ]

  const ladderColumns: PixelTableColumn<eventmanager.LadderEntryView>[] = [
    { key: 'rank', header: 'RANK', width: 64, render: (e) => e.rank },
    { key: 'name', header: 'PARTICIPANT', render: (e) => <span className="font-medium">{e.name}</span> },
    {
      key: 'elo',
      header: 'ELO',
      align: 'right',
      width: 96,
      render: (e) => <span className="text-retro-gold">{e.elo}</span>,
    },
    {
      key: 'wl',
      header: 'W-L',
      align: 'right',
      width: 96,
      render: (e) => `${e.wins}-${e.losses}`,
    },
  ]

  return (
    <PixelContainer maxWidth="full" padding="md">
      <PixelButton asChild variant="ghost" tone="neutral" size="sm" className="mb-6">
        <Link to="/events">&lt; BACK TO EVENTS</Link>
      </PixelButton>

      {/* Main Info Card */}
      <PixelCard className="mb-6">
        <PixelStack gap={4}>
          <PixelStack direction="row" gap={4} align="start" justify="between" wrap>
            <PixelStack gap={1}>
              <h1 className="text-2xl font-pixel tracking-wider text-retro-primary">{event.name}</h1>
              {event.scheduledAt && (
                <div className="font-pixel text-xs text-retro-gold">
                  SCHEDULED: <time dateTime={event.scheduledAt}>{formatLocalDateTime(event.scheduledAt)}</time>
                </div>
              )}
            </PixelStack>
            <PixelStack direction="row" gap={2}>
              <PixelBadge tone={TAG_TONE[(event.tag as EventTag) || 'COMMUNITY']}>
                {(TAG_LABELS[(event.tag as EventTag) || 'COMMUNITY']).toUpperCase()}
              </PixelBadge>
              <PixelBadge tone={STATUS_TONE[event.status as EventStatus] || 'neutral'}>
                {(STATUS_LABELS[event.status as EventStatus] || event.status).toUpperCase()}
              </PixelBadge>
            </PixelStack>
          </PixelStack>

          {event.description && (
            <p className="text-retro-text font-sans leading-relaxed text-sm">{event.description}</p>
          )}

          <PixelStack direction="row" gap={4} wrap>
            <PixelBadge tone="neutral">
              SCORING TYPE: {SCORING_LABELS[event.scoringType] ?? 'UNKNOWN'}
            </PixelBadge>
            <PixelBadge tone="neutral">
              CLASS RESTRICTION:{' '}
              {event.classRestriction && event.classRestriction !== 'PRE_OP' && event.classRestriction !== 'OP' ? CLASS_TIER_LABELS[event.classRestriction as any] : 'OPEN TO ALL'}
            </PixelBadge>
          </PixelStack>

          {!isGranular && (
            <div className="pt-4 border-t-2 border-retro-border">
              {!session ? (
                <PixelButton asChild variant="solid" tone="purple" className="pxl-btn-flat">
                  <Link to="/auth" search={{ redirect: `/events/${eventId}` }}>
                    SIGN IN TO JOIN
                  </Link>
                </PixelButton>
              ) : (
                <PixelStack gap={2}>
                  <PixelButton
                    variant="solid"
                    tone={isMember ? 'red' : 'green'}
                    className="pxl-btn-flat"
                    disabled={isConcluded || event.status === 'DRAFT' || event.status === 'PENDING_DELETION' || event.signupsLocked || joinMutation.isPending || leaveMutation.isPending || (!isMember && event.participantLimit !== null && event.members.length >= event.participantLimit)}
                    loading={joinMutation.isPending || leaveMutation.isPending}
                    onClick={() => {
                      if (isConcluded) return
                      if (isMember) {
                        leaveMutation.mutate(eventId)
                      } else {
                        joinMutation.mutate(eventId)
                      }
                    }}
                  >
                    {isMember ? 'WITHDRAW FROM EVENT' : (event.participantLimit !== null && event.members.length >= event.participantLimit) ? 'EVENT FULL' : 'SIGN UP FOR EVENT'}
                  </PixelButton>
                  {event.signupsLocked && (
                    <div className="text-retro-muted font-pixel text-xs mt-2">
                      SIGNUPS ARE LOCKED FOR THIS EVENT
                    </div>
                  )}
                </PixelStack>
              )}
            </div>
          )}
        </PixelStack>
      </PixelCard>

      <div className="grid grid-cols-1 md:grid-cols-2 gap-6 mt-6">
        {/* Participants Panel */}
        <div>
          <PixelSectionHeader
            title={`PARTICIPANTS (${event.members.length}${!event.granularParticipation && event.participantLimit !== null && event.participantLimit > 0 ? ` / ${event.participantLimit}` : ''})`}
            size="sm"
            spacing="tight"
          />
          <div className="public-table">
            <PixelTable
              columns={participantColumns}
              data={event.members}
              emptyState={<span className="font-pixel text-xs text-retro-muted">NO MEMBERS YET</span>}
            />
          </div>
        </div>

        {/* Schedule Panel */}
        {event.schedules && event.schedules.length > 0 && (
          <div>
            <PixelSectionHeader title="SCHEDULE" size="sm" spacing="tight" />
            <PixelCard className="">
              <PixelStack gap={4}>
                {event.schedules.map((schedule) => (
                  <PixelStack
                    key={schedule.id}
                    gap={2}
                    className="border-b-2 border-retro-border last:border-b-0 pb-3 last:pb-0"
                  >
                    <div className="font-pixel text-xs text-retro-primary">
                      {schedule.title || 'UNTITLED'}
                    </div>
                    <div className="text-xs text-retro-muted font-sans">
                      {new Date(schedule.startsAt).toLocaleString()}
                      {schedule.location && (
                        <span className="block mt-1 font-pixel text-[11px] text-retro-text bg-retro-surface px-2 py-0.5 border border-retro-border pxl-corner-sm inline-block">
                          📍 {schedule.location}
                        </span>
                      )}
                    </div>
                  </PixelStack>
                ))}
              </PixelStack>
            </PixelCard>
          </div>
        )}
      </div>

      {/* Standings (Points) */}
      {event.pointsOverview && (
        <div className="mt-8">
          <PixelStack direction="row" gap={3} align="center" wrap className="mb-4">
            <h2 className="font-pixel text-sm tracking-wider text-retro-text">
              STANDINGS (POINTS)
            </h2>
            {event.status === 'OFFICIAL' || event.status === 'CONCLUDED' ? (
              <PixelBadge tone="green">FINAL</PixelBadge>
            ) : (
              <PixelBadge tone="gold">PROVISIONAL</PixelBadge>
            )}
          </PixelStack>
          <div className="public-table">
            <PixelTable
              columns={pointsColumns}
              data={event.pointsOverview}
              emptyState={
                <span className="font-pixel text-xs text-retro-muted">NO RESULTS RECORDED</span>
              }
            />
          </div>
        </div>
      )}

      {/* Standings (Ladder) */}
      {event.ladderOverview && (
        <div className="mt-8">
          <PixelStack direction="row" gap={3} align="center" wrap className="mb-4">
            <h2 className="font-pixel text-sm tracking-wider text-retro-text">
              STANDINGS (LADDER)
            </h2>
            {event.status === 'OFFICIAL' || event.status === 'CONCLUDED' ? (
              <PixelBadge tone="green">FINAL</PixelBadge>
            ) : (
              <PixelBadge tone="gold">PROVISIONAL</PixelBadge>
            )}
          </PixelStack>
          <div className="public-table">
            <PixelTable
              columns={ladderColumns}
              data={event.ladderOverview}
              emptyState={
                <span className="font-pixel text-xs text-retro-muted">NO LADDER RECORDS</span>
              }
            />
          </div>
        </div>
      )}

      {/* RACES SECTION */}
      <div className="mt-8">
        <PixelSectionHeader title="RACES" size="sm" spacing="tight" />
        <EventRacesList event={event} />
      </div>
    </PixelContainer>
  )
}

function RaceStandingsTable({
  eventId,
  raceId,
  members,
}: {
  eventId: string
  raceId: string
  members: eventmanager.EventMemberView[]
}) {
  const { data, isLoading, error } = useQuery({
    queryKey: ['public-race-results', eventId, raceId],
    queryFn: () => getPublicRaceResults(eventId, raceId),
  })

  if (isLoading) {
    return (
      <PixelStack align="center" justify="center" gap={2} className="py-4">
        <PixelSpinner size="sm" label="Loading standings..." />
      </PixelStack>
    )
  }

  if (error || !data) {
    return (
      <div className="text-retro-muted font-pixel text-xs py-2">
        ERROR LOADING STANDINGS
      </div>
    )
  }

  const results = data.results

  const columns: PixelTableColumn<eventmanager.RaceResultView>[] = [
    {
      key: 'position',
      header: 'POS',
      width: 64,
      render: (r) => <span className="font-pixel">{r.position ?? '-'}</span>,
    },
    {
      key: 'gateNumber',
      header: 'DRAW',
      width: 64,
      render: (r) => <span>{r.gateNumber ?? '-'}</span>,
    },
    {
      key: 'participant',
      header: 'PARTICIPANT',
      render: (r) => {
        const member = members.find((m) => m.userId === r.userId)
        return <span className="font-medium">{member?.name ?? r.userId}</span>
      },
    },
    {
      key: 'points',
      header: 'POINTS',
      align: 'right',
      width: 96,
      render: (r) => <span className="text-retro-primary">{r.points}</span>,
    },
    {
      key: 'finishTime',
      header: 'FINISH TIME',
      render: (r) => <span>{r.finishTime ?? '-'}</span>,
    },
    {
      key: 'margin',
      header: 'MARGIN',
      render: (r) => <span>{r.margin ?? '-'}</span>,
    },
    {
      key: 'passingOrder',
      header: 'PASSING ORDER',
      render: (r) => <span>{r.passingOrder ?? '-'}</span>,
    },
    {
      key: 'final3F',
      header: 'FINAL 3F',
      render: (r) => <span>{r.final3F ?? '-'}</span>,
    },
    {
      key: 'resultStatus',
      header: 'RESULT',
      width: 80,
      render: (r) => {
        const status = r.resultStatus?.toUpperCase()
        if (status === 'DSQ') return <span className="text-red-600 font-bold">DSQ</span>
        if (status === 'DNF') return <span className="text-orange-600 font-bold">DNF</span>
        if (status === 'DNS') return <span className="text-amber-600 font-bold">DNS</span>
        if (status === 'DEFERRED') return <span className="text-slate-500 font-bold" title="Deferred - Already won an OP">DEFERRED</span>
        return <span className="text-retro-muted">-</span>
      },
    },
  ]

  return (
    <div className="public-table">
      <PixelTable
        columns={columns}
        data={results}
        emptyState={
          <span className="font-pixel text-xs text-retro-muted">NO STANDINGS RECORDED</span>
        }
      />
    </div>
  )
}

function EventRacesList({ event }: { event: eventmanager.EventDetail }) {
  const queryClient = useQueryClient()
  const { session } = useAuth()
  const { toast } = useToast()
  const isMember = session && event.members.some((m) => m.userId === session.user.id)

  const joinRaceMutation = useMutation({
    mutationFn: ({ raceId }: { raceId: string }) =>
      joinRaceEvent(event.id, raceId, `Bearer ${session?.session.token ?? ''}`),
    onSuccess: (_, { raceId }) => {
      queryClient.invalidateQueries({ queryKey: ['public-event', event.id] })
      queryClient.invalidateQueries({ queryKey: ['public-event-races', event.id] })
      queryClient.invalidateQueries({ queryKey: ['public-race-results', event.id, raceId] })
    },
    onError: (err: any) => {
      const msg = err?.message || err?.toString() || 'Unknown error'
      toast({
        tone: 'red',
        title: `Could not sign up for this race: ${msg}`,
      })
    },
  })

  const leaveRaceMutation = useMutation({
    mutationFn: ({ raceId }: { raceId: string }) =>
      leaveRaceEvent(event.id, raceId, `Bearer ${session?.session.token ?? ''}`),
    onSuccess: (_, { raceId }) => {
      queryClient.invalidateQueries({ queryKey: ['public-event', event.id] })
      queryClient.invalidateQueries({ queryKey: ['public-event-races', event.id] })
      queryClient.invalidateQueries({ queryKey: ['public-race-results', event.id, raceId] })
    },
    onError: (err: any) => {
      const msg = err?.message || err?.toString() || 'Unknown error'
      toast({
        tone: 'red',
        title: `Could not withdraw from this race: ${msg}`,
      })
    },
  })

  const { data, isLoading, error } = useQuery({
    queryKey: ['public-event-races', event.id],
    queryFn: () => listPublicRaceEvents(event.id),
  })

  if (isLoading) {
    return (
      <PixelStack align="center" justify="center" gap={4} className="py-8">
        <PixelSpinner size="md" label="Loading races..." />
      </PixelStack>
    )
  }

  if (error || !data) {
    return (
      <PixelEmptyState
        title="Error loading races"
        description="Something went wrong while fetching the race events list."
      />
    )
  }

  const races = data.races

  if (races.length === 0) {
    return (
      <PixelEmptyState
        title="No races scheduled"
        description="There are no individual race events configured for this competition."
      />
    )
  }

  return (
    <PixelStack gap={6} className="mt-4">
      {races.map((race) => {
        const isRaceMember = session && (race.members ?? []).some((rm) => rm.userId === session.user.id)

        return (
          <PixelCard key={race.id}>
            <PixelStack gap={4}>
              {/* Race Header Info */}
              <PixelStack direction="row" gap={4} align="start" justify="between" wrap>
                <PixelStack gap={2}>
                  <h3 className="text-lg font-pixel tracking-wide text-retro-text">
                    #{race.sequence}. {race.name}
                  </h3>
                  <div className="text-xs text-retro-muted font-sans flex flex-wrap gap-x-4 gap-y-1">
                    <span>TRACK: <strong>{race.trackType}</strong> ({race.distanceMeters}m)</span>
                    <span>LOCATION: <strong>{race.location}</strong></span>
                  </div>
                </PixelStack>
                <PixelStack direction="row" gap={2} align="center">
                  <PixelBadge tone="neutral">
                    CLASS:{' '}
                    {race.classRestriction && race.classRestriction !== 'PRE_OP' && race.classRestriction !== 'OP' ? CLASS_TIER_LABELS[race.classRestriction] ?? race.classRestriction : 'OPEN'}
                  </PixelBadge>
                    {race.participantLimit !== null && (
                      <PixelBadge tone="neutral">
                        CAPACITY: {(race.members ?? []).length} / {race.participantLimit}
                      </PixelBadge>
                    )}
                  {event.granularParticipation && (
                    <>
                      {!session ? (
                        <PixelButton asChild variant="solid" tone="purple" size="sm" className="pxl-btn-flat font-pixel text-[10px]">
                          <Link to="/auth" search={{ redirect: `/events/${event.id}` }}>
                            SIGN IN
                          </Link>
                        </PixelButton>
                      ) : (
                        <PixelButton
                          variant="solid"
                          tone={isRaceMember ? 'red' : 'green'}
                          size="sm"
                          className="pxl-btn-flat font-pixel text-[10px]"
                            disabled={event.signupsLocked || joinRaceMutation.isPending || leaveRaceMutation.isPending || (!isRaceMember && race.participantLimit !== null && (race.members ?? []).length >= race.participantLimit)}
                          onClick={() => {
                            if (isRaceMember) {
                              leaveRaceMutation.mutate({ raceId: race.id })
                            } else {
                              joinRaceMutation.mutate({ raceId: race.id })
                            }
                          }}
                        >
                            {isRaceMember ? 'WITHDRAW' : (race.participantLimit !== null && (race.members ?? []).length >= race.participantLimit) ? 'FULL' : 'SIGN UP'}
                        </PixelButton>
                      )}
                    </>
                  )}
                  {!event.granularParticipation && isMember && (
                    <PixelButton
                      variant="solid"
                      tone={isRaceMember ? 'red' : 'green'}
                      size="sm"
                      className="pxl-btn-flat font-pixel text-[10px]"
                      disabled={event.signupsLocked || joinRaceMutation.isPending || leaveRaceMutation.isPending}
                      onClick={() => {
                        if (isRaceMember) {
                          leaveRaceMutation.mutate({ raceId: race.id })
                        } else {
                          joinRaceMutation.mutate({ raceId: race.id })
                        }
                      }}
                    >
                      {isRaceMember ? 'WITHDRAW' : 'SIGN UP'}
                    </PixelButton>
                  )}
                </PixelStack>
              </PixelStack>

            {/* Standings Table */}
            <div className="mt-2">
              <div className="font-pixel text-[11px] text-retro-text mb-2">RACE STANDINGS</div>
              <RaceStandingsTable eventId={event.id} raceId={race.id} members={event.members} />
            </div>
          </PixelStack>
        </PixelCard>
        )
      })}
    </PixelStack>
  )
}

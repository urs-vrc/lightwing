import { useAuth } from '../../hooks/useAuth'
import { useQuery, useQueryClient } from '@tanstack/react-query'
import { createFileRoute, Link } from '@tanstack/react-router'
import { useState } from 'react'
import { listPublicEvents } from '../../lib/public-api'
import { Pagination } from '../../components/Pagination'
import { formatLocalDateTime } from '../../lib/datetime'
import {
  PixelContainer,
  PixelStack,
  PixelCard,
  PixelBadge,
  PixelSectionHeader,
  PixelSpinner,
  PixelEmptyState,
} from '@pxlkit/ui-kit'
import type { eventmanager } from '../../lib/client'
import { PixelSkeletonList } from '../../components/LoadingSkeleton'
import type { EventStatus, EventTag } from '../../types'

const CLASS_TIER_LABELS: Record<string, string> = {
  PRE_OP: 'PRE-OP',
  OP: 'OP',
  G3: 'G3',
  G2: 'G2',
  G1: 'G1',
}

const SCORING_LABELS: Record<number, string> = {
  1: 'points-based',
  2: 'ladder-elo',
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

export const Route = createFileRoute('/events/')({
  component: EventsPage,
})

function EventsPage() {
  const queryClient = useQueryClient()
  const { session } = useAuth()
  const [page, setPage] = useState(1)
  const [pageSize, setPageSize] = useState(10)

  const { data, isLoading, error } = useQuery({
    queryKey: ['public-events', page, pageSize],
    queryFn: () => listPublicEvents(pageSize, (page - 1) * pageSize),
  })

  if (isLoading) {
    return (
      <PixelContainer maxWidth="full" padding="md">
        <PixelSectionHeader
          title="COMPETITIVE EVENTS"
          titleTone="purple"
          size="lg"
        />
        <div className="mt-6">
          <PixelSkeletonList count={3} />
        </div>
      </PixelContainer>
    )
  }
  if (error) {
    return (
      <PixelContainer maxWidth="md" padding="md">
        <PixelEmptyState
          title="Error loading events"
          description="Something went wrong while fetching the event list."
        />
      </PixelContainer>
    )
  }

  // Filter events to exclude DRAFT and PENDING_DELETION
  const publicEvents = data?.events.filter((event) => event.status !== 'DRAFT' && event.status !== 'PENDING_DELETION') || []

  return (
    <PixelContainer maxWidth="full" padding="md">
      <PixelSectionHeader
        title="COMPETITIVE EVENTS"
        titleTone="purple"
        size="lg"
        actions={
          <PixelBadge tone="neutral">{publicEvents.length} ACTIVE</PixelBadge>
        }
      />

      <PixelStack gap={6} className="mt-6">
        {publicEvents.length === 0 ? (
          <PixelEmptyState
            title="No public events active"
            description="There are no public events running at this moment."
          />
        ) : (
          <>
            {publicEvents.map((event) => {
              return (
                <Link
                  key={event.id}
                  to="/events/$eventId"
                  params={{ eventId: event.id }}
                  className="block"
                >
                  <PixelCard className="hover:border-retro-primary transition-all duration-150">
                    <PixelStack gap={4}>
                      <PixelStack direction="row" gap={4} align="start" justify="between" wrap>
                        <PixelStack gap={2}>
                          <h2 className="text-xl font-pixel tracking-wide text-retro-text">
                            {event.name}
                          </h2>
                          {event.scheduledAt && (
                            <div className="font-pixel text-[11px] text-retro-gold">
                              SCHEDULED: {formatLocalDateTime(event.scheduledAt)}
                            </div>
                          )}
                          {event.description && (
                            <p className="text-base text-retro-muted max-w-2xl font-sans leading-relaxed">
                              {event.description}
                            </p>
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

                      <PixelStack direction="row" gap={4} wrap>
                        <PixelBadge tone="neutral">
                          SCORING: {SCORING_LABELS[event.scoringType]?.toUpperCase() || 'UNKNOWN'}
                        </PixelBadge>
                        <PixelBadge tone="neutral">
                          CLASS:{' '}
                          {event.classRestriction && event.classRestriction !== 'PRE_OP' && event.classRestriction !== 'OP'
                            ? CLASS_TIER_LABELS[event.classRestriction as any]
                            : 'OPEN'}
                        </PixelBadge>
                        <PixelBadge tone="neutral">RACES: {event.raceCount}</PixelBadge>
                        <PixelBadge tone="neutral">MEMBERS: {event.memberCount}</PixelBadge>
                      </PixelStack>
                    </PixelStack>
                  </PixelCard>
                </Link>
              )
            })}

            <Pagination
              page={page}
              pageSize={pageSize}
              total={data?.total || 0}
              onPageChange={setPage}
              onPageSizeChange={setPageSize}
              variant="pixel"
            />
          </>
        )}
      </PixelStack>
    </PixelContainer>
  )
}

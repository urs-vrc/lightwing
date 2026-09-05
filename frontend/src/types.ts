// Shared type definitions for the frontend

export type ClassTier = 'PRE_OP' | 'OP' | 'G3' | 'G2' | 'G1'

export type EventStatus = 'DRAFT' | 'PENDING' | 'ONGOING' | 'CONCLUDED' | 'PENDING_DELETION'

export type EventTag = 'OFFICIAL' | 'COMMUNITY'

export type EventOwnerType = 'USER' | 'ORGANIZATION'

export type SiteRole = 'USER' | 'SITE_ADMIN'

export const CLASS_TIER_LABELS: Record<ClassTier, string> = {
  PRE_OP: 'PRE-OP',
  OP: 'OP',
  G3: 'G3',
  G2: 'G2',
  G1: 'G1',
}

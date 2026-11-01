import { describe, expect, it } from 'vitest'
import type { DegradationHourBucket } from '@/api/degradation'
import { buildHourlySlots, classifyHourBucket, resolveHourlyWindow } from '../hourly'

const HOUR = 3_600_000

function bucket(start: string, passed: number, failed: number, error = 0): DegradationHourBucket {
  const judged = passed + failed
  return { start, passed, failed, error, pass_rate: judged ? passed / judged : 0, avg_latency_ms: 1000 }
}

describe('classifyHourBucket', () => {
  it('maps counts to levels using the threshold', () => {
    expect(classifyHourBucket(undefined, 0.8)).toBe('empty')
    expect(classifyHourBucket(bucket('2026-10-30T10:00:00Z', 0, 0), 0.8)).toBe('empty')
    expect(classifyHourBucket(bucket('2026-10-30T10:00:00Z', 0, 0, 2), 0.8)).toBe('error')
    expect(classifyHourBucket(bucket('2026-10-30T10:00:00Z', 3, 0, 1), 0.8)).toBe('healthy')
    expect(classifyHourBucket(bucket('2026-10-30T10:00:00Z', 4, 1), 0.8)).toBe('warning')
    expect(classifyHourBucket(bucket('2026-10-30T10:00:00Z', 2, 1), 0.8)).toBe('degraded')
  })
})

describe('resolveHourlyWindow', () => {
  it('uses explicit bucket fields from the backend', () => {
    const window = resolveHourlyWindow({
      window_hours: 24,
      generated_at: '2026-10-30T10:42:00Z',
      bucket_start: '2026-10-29T11:00:00Z',
      bucket_count: 24,
      bucket_seconds: 3600,
    })
    expect(window).toEqual({ startMs: Date.parse('2026-10-29T11:00:00Z'), count: 24, stepMs: HOUR })
  })

  it('falls back to generated_at aligned to the hour for older payloads', () => {
    const window = resolveHourlyWindow({ window_hours: 3, generated_at: '2026-10-30T10:42:00Z' })
    expect(window).toEqual({ startMs: Date.parse('2026-10-30T08:00:00Z'), count: 3, stepMs: HOUR })
    expect(resolveHourlyWindow({ window_hours: 3, generated_at: 'invalid' })).toBeNull()
  })
})

describe('buildHourlySlots', () => {
  it('aligns sparse buckets into a dense oldest-to-newest strip', () => {
    const window = { startMs: Date.parse('2026-10-30T08:00:00Z'), count: 3, stepMs: HOUR }
    const slots = buildHourlySlots(
      [
        bucket('2026-10-30T10:00:00Z', 1, 0),
        bucket('2026-10-30T08:00:00Z', 1, 2),
        bucket('2026-10-30T07:00:00Z', 5, 0),
        bucket('2026-10-30T11:00:00Z', 5, 0),
      ],
      window,
      0.8,
    )
    expect(slots.map((slot) => slot.level)).toEqual(['degraded', 'empty', 'healthy'])
    expect(slots[0].startMs).toBe(window.startMs)
    expect(slots[2].endMs).toBe(Date.parse('2026-10-30T11:00:00Z'))
    expect(buildHourlySlots(null, window, 0.8)).toHaveLength(3)
  })
})

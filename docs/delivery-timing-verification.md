# Delivery Timing Verification And Rollback

## Scope

The new `other.delivery_timing` fields measure server-side delivery, not confirmed client receipt. The response start is `RelayInfo.StartTime`; the first timestamp follows a successful complete content write (and SSE flush); the end timestamp follows final response completion or interruption.

Legacy `use_time` remains whole-second endpoint subtraction. Historical rows are not rewritten. The UI marks records without the new schema as legacy precision. `bamboo_timing` remains upstream observation; `bamboo_timing_hops` preserves each observed hop.

## Local Checks

Run from the NewAPI checkout:

```sh
go test ./relay/common ./relay/bamboo ./service -run 'TestDelivery|TestClientStreamFirstResponseTimeCalibration|TestAppendBambooTimingAlignsFrt|TestBuildTokenRecordTiming' -count=1 -timeout=60s
go test -race ./relay/common ./relay/bamboo -run 'TestDelivery|TestClientStreamFirstResponseTimeCalibration' -count=1 -timeout=90s
cd web
bun run test src/features/usage-logs/lib/__tests__/timing.test.ts src/features/usage-logs/components/__tests__/delivery-timing.test.tsx src/features/usage-logs/components/__tests__/mobile-card.test.tsx src/features/usage-logs/components/__tests__/detail-preview.test.tsx
bun run typecheck
bun run build
```

Validate buffered tool-only output, empty/signature/usage events, serializer flush output, normalized-away frames, short writes, cancellation, and the 100ms streaming TPS reliability threshold. Desktop, mobile and details must display the same delivery milliseconds. A real zero TTFT must display zero; a missing TTFT must display N/A.

## Authorized Release Only

1. Record the currently running master/slave image IDs and immutable image tags. Preserve the previous image.
2. Build the candidate with an immutable tag using the existing project Docker build process. Do not publish or replace the live `latest` tag as part of local verification.
3. After explicit deployment approval, update both service image references together and recreate the services with the existing Compose workflow. Database/schema changes are not needed.
4. Verify health, an ordinary stream, a tool-only stream, and a delayed downstream stream. Confirm version 1 delivery metadata appears on new Bamboo consume logs.
5. Check new records only using the read-only SQL below. Do not compare new TTFT with legacy `use_time` to judge correctness.
6. On rollback, restore the previous immutable image references and recreate the services. Old code ignores new JSON metadata; existing/new log rows need no rollback migration.

## PostgreSQL Read-Only Verification

Set the release timestamp explicitly; do not substitute a hand-calculated Unix epoch. The JSON operators below are for the PostgreSQL production database only.

```sql
BEGIN TRANSACTION READ ONLY;
WITH timings AS (
  SELECT id, other::jsonb->'delivery_timing' AS timing,
         (other::jsonb->>'frt')::numeric AS frt
  FROM logs
  WHERE type = 2
    AND created_at >= extract(epoch FROM timestamptz 'REPLACE_WITH_RELEASE_TIMESTAMP')
    AND other::jsonb->'delivery_timing'->>'version' = '1'
)
SELECT count(*) AS versioned_records,
       count(*) FILTER (
         WHERE (timing->>'total_ms')::numeric < 0
            OR (timing->>'ttft_ms')::numeric < 0
            OR (timing->>'ttft_ms')::numeric > (timing->>'total_ms')::numeric
       ) AS invalid_durations,
       count(*) FILTER (
         WHERE timing->>'ttft_ms' IS NOT NULL
           AND frt <> (timing->>'ttft_ms')::numeric
       ) AS compatibility_frt_mismatches
FROM timings;
COMMIT;
```

Both violation counts must be zero. Empty-content responses may legitimately have `ttft_ms: null`; short tail durations may legitimately have no average TPS. Interrupted responses must retain already delivered content timing and an interruption status. Investigate `delivery_timing_invalid` diagnostics rather than clamping their values or backfilling history.

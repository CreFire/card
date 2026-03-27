# basal
## Purpose
Provide fundamental conversions, hashing primitives, and small utility helpers used across the codebase.

## Use When
You need string/number conversions, JSON helpers, consistent hashing, in-process channels, sorted collections, or small monitors like `NextNumber`.

## Avoid When
A more specific utility already exists in the caller; avoid duplicating functionality with other helper stacks.

## Key Entry Points
- `ToInt64`, `ToFloat64`, `ToJsonString`, `ConvertInt32`, `AtoInt`
- `ConsistentHash[T NodeKey]` / `NewConsistentHash` and its `HashFunc`
- `Chan[T]`, `NewChan`, `SortedList`, `SortedSet`, `NextNumber`

## Notes
`ConsistentHash` requires callers to implement `NodeKey` so `GetKey` and `HashFunc` stay aligned.

## Business Usage
- In `logic`, `public`, `robot`, and `auth/login_queue`, business code mainly depends on `SafeGo` / `SafeRun`, `MonitorMgr`, `NoCacheLineData`, and small numeric helpers like `Min`.
- Callers treat `SafeGo` / `SafeRun` as panic containment for side goroutines, not as a retry or error propagation mechanism. Do not infer that failures are surfaced to the caller.
- `NoCacheLineData` is used on hot counters/timestamps in queue and online flows; do not replace it with ordinary wrapper objects if the path is latency-sensitive.
- Do not over-read this package as a generic framework. In business code it is mostly "small runtime glue" around concurrency and clamping.

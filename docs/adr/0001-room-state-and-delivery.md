# Room-owned state and bounded network IO

Date: 2026-09-07
Status: implemented on refactor/room-performance. The compatibility clause for the
legacy entity actors is superseded by [ADR 0002](0002-remove-compatibility-layer.md).

## Problem

The room already computed every movement and collision, but ball and paddle
actors held additional copies of the same state. Their asynchronous replies could
replace newer room decisions. The shared actor runtime silently discarded messages
when full. Sequential socket writes could block a room's broadcaster. The server
also rebuilt and sent a complete grid 60 times per second even when unchanged.

## Decision

A game actor is the sole owner of its room's balls, paddles, scores, and phase.
Physics remains at the configured interval: the default 25 ms is 40 Hz. Entity
state changes use direct methods. Legacy entity actor types remain for source
compatibility and their isolated tests; the running game never creates them or
accepts their state replies. Their PID maps remain only for old injected test
fixtures and cleanup of those fixtures.

Use a backend-owned internal actor runtime with a mutex-protected queue and wake
channel. Internal sends enqueue without waiting on another actor and report a
stopped target. Network ingress uses a 256-entry admission limit, a 4 KiB payload
limit, a token bucket (60 messages/second, burst 120), and a 512-connection limit.
Timer notifications are coalesced until consumed. Stop is idempotent, and spawn
and engine shutdown share the same lock. Internal queues have no hard capacity;
this avoids actor-to-actor deadlocks, but new internal producers must be bounded
or coalesced. This is not a durable or distributed message system.

Each WebSocket has one writer and a 64-frame outgoing queue. The broadcaster
encodes a batch once and shares immutable text with all writers. Overflow closes
that connection; it never silently drops a game event. Forced close expires IO
before asynchronously closing the socket, so closing cannot wait on a blocked
writer in the room/broadcaster actor. Writes and final-message draining are bounded
by two seconds. Healthy final messages are ordered after pending updates and before
close. Slow or failed clients are not guaranteed a final message.

Keep the existing client wire format. A fullGridUpdate remains a complete grid
replacement, emitted on admission and after grid mutation. It is not a delta.
Broadcasting runs no faster than physics. An idle room sends an empty gameUpdates
heartbeat at 15-second intervals. Incoming read deadlines no longer disconnect
players merely because they stop pressing keys.

## Admission and shutdown

The manager reserves a slot before dispatch and tracks pending admissions.
Duplicate pending requests for the same session are rejected. A room commits
player state before sending success and initial snapshots. Only precommit failure
rolls back a reservation; a committed connection failure owns one slot through the
normal reconnect grace period and releases it once. Room shutdown cancels queued
admissions, and a five-second handler timeout bounds unresponsive admission.
A connection cannot request a second room while pending or assigned.

Quick Play waits for grid initialization before starting physics. Timer messages
carry generations so a previously queued expiry cannot cancel refreshed state.
Room cleanup always runs through sync.Once, including game over. Ordinary cleanup
closes every room-owned client even if broadcaster registration was still queued.
Game over explicitly hands final delivery to the broadcaster and client writers.
Process signals stop actors and hijacked sockets before HTTP shutdown.

## Tradeoffs and validation

Keeping room actors avoids a wholesale game-loop rewrite. Keeping full grid
snapshots on mutation avoids a coordinated frontend migration; a future grid-delta
protocol could reduce traffic further, but is not needed for this refactor.
Physics collision behavior and configured simulation speed are preserved.

See ../performance.md for measured baseline, workload limits, and reproduction.
Race-enabled tests cover actor stop/shutdown, reliable delivery, bounded ingress,
slow output, final ordering, admission rollback, duplicate session reservations,
reconnects, stale timer messages, and concurrent room completion.

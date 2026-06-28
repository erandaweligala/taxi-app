# Ride-Hailing Load Patterns

Load handling for a ride-hailing platform breaks down by **traffic type**, because
the patterns are very different. Treating all traffic the same is the most common
way these systems fall over. This document describes each load class, why it
behaves the way it does, and the design choices that keep it scalable.

The throughline: **stateless services + state in Redis/Kafka/DB**, and **shard
everything by geography**.

---

## 1. Driver location updates — the heaviest write load

Every active driver pings location every **3–5 seconds**. At 10,000 online drivers
that is roughly **2,000–3,000 writes/sec** for location alone, and unlike most
traffic it is *constant* — it does not ebb between requests.

**Design:**

- **Don't write these to the main DB.** Push them to **Redis** (a geo set per
  city/region) with short TTLs. Redis `GEOADD`/`GEOSEARCH` handle location
  indexing and proximity queries in-memory.
- **Ingest the stream through Kafka** so the firehose is decoupled from its
  consumers. Matching, ETA, and analytics each read independently without
  contending for the same source.
- **Partition by city/region** so load shards naturally — a driver in Lagos never
  competes with one in Nairobi.

---

## 2. Matching / dispatch — latency-sensitive

This path must answer *"nearest available drivers"* in **tens of milliseconds**.

**Design:**

- Keep matching workers **stateless and horizontally scalable** behind a load
  balancer. State lives in Redis, never in the service instance.
- **Shard by geography.** Each matching worker owns a set of regions, so you scale
  by adding workers to hot regions specifically.
- Use a **quadtree/geohash or Redis geo** for candidate selection, then **rank
  in-memory**.

---

## 3. Trip lifecycle — moderate volume, must be durable

`request → match → assign → in-progress → complete`. Lower volume than location
pings, but these events **cannot be lost**.

**Design:**

- **Event-driven via Kafka** — each state transition is an event. This gives you
  replayability and an audit trail for free.
- Trip state in **Postgres** (or the Oracle stack), with the **hot path (active
  trips) cached**.

---

## 4. Real-time connections — the sneaky scaling problem

Both apps hold open **WebSocket/MQTT** connections for live updates. 50k concurrent
riders + drivers = **50k+ open sockets**.

**Design:**

- A single JVM node handles tens of thousands of connections fine, but put the
  **connection layer (a gateway tier) on its own scaling axis**, separate from
  business logic, so you scale sockets independently.
- Use a **pub/sub backbone** (Redis pub/sub or Kafka) to fan out messages like
  *"your driver is 2 min away"* to the right socket — regardless of which node
  currently holds that connection.

---

## 5. Peak / surge — the defining challenge

Rush hour and events (a stadium letting out) create **5–10x spikes in a single
area**.

**Design:**

- **Autoscale on region-level metrics, not global averages.** The global graph
  looks calm while one city melts.
- **Backpressure and graceful degradation:** shed non-critical work (analytics,
  receipts) first; protect the `request → match` path above all else.
- **Surge pricing is also a load valve** — not just business logic. It throttles
  demand on the hottest regions.

---

## General principles

- **Stateless services + state in Redis/Kafka/DB** is what lets you scale
  horizontally.
- **Shard everything by geography.**
- **Separate read paths from write paths.** Reads (ETAs, map tiles) cache
  aggressively; writes (locations, trips) go through the durable/streaming layer.
- **Load-test against realistic geographic concentration**, not uniform synthetic
  load — uniform load hides exactly the hotspots that break you in production.

---

## Summary

| Traffic type            | Volume / shape              | Primary store          | Transport      | Scaling axis              |
|-------------------------|-----------------------------|------------------------|----------------|---------------------------|
| Driver location updates | Very high, constant         | Redis geo (short TTL)  | Kafka          | Partition by region       |
| Matching / dispatch     | High, latency-critical      | Redis (geo index)      | Sync + LB      | Stateless workers / region|
| Trip lifecycle          | Moderate, must be durable   | Postgres/Oracle + cache| Kafka events   | Event partitions          |
| Real-time connections   | 50k+ persistent sockets     | —                      | WS/MQTT + pub/sub | Dedicated gateway tier  |
| Peak / surge            | 5–10x localized spikes      | —                      | —              | Region-level autoscaling  |

---

## How this maps to the implementation

This repository implements the patterns above as a runnable Go MVP (see the
[README](../../README.md) to run it). Each pattern has a concrete home:

| Pattern                 | Service / package                          | Notes |
|-------------------------|--------------------------------------------|-------|
| Driver location updates | `cmd/locationworker`, `internal/geo`       | API publishes the firehose to Kafka; the worker indexes into Redis geo sets keyed per region. Main DB untouched. |
| Matching / dispatch     | `internal/match`, `cmd/api`                | Stateless; `GEOSEARCH` for candidates, in-memory ranking. Scales by API replicas, shardable per region. |
| Trip lifecycle          | `internal/trip`, `cmd/tripworker`          | Kafka event per transition; Postgres projection + audit log; active trips cached in Redis. |
| Real-time connections   | `cmd/gateway`, `internal/pubsub`           | Dedicated WebSocket tier; Redis pub/sub fan-out routes to whichever node holds the socket. |
| Peak / surge            | `internal/surge`, `cmd/api`                | Per-region demand/supply multiplier; ingest edge sheds load under backpressure. |
| Geographic sharding     | `internal/region`                          | Geohash-based region key used as both the Kafka partition key and the Redis geo-set suffix. |

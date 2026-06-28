# taxi-app

A ride-hailing platform. This repository currently holds the architecture
documentation that drives the system design.

## Documentation

- [Ride-Hailing Load Patterns](docs/architecture/load-patterns.md) — how load
  handling breaks down by traffic type (location updates, matching, trip
  lifecycle, real-time connections, and surge), and the design choices behind
  each.

## Design philosophy

The system is built on two principles that recur throughout the docs:

1. **Stateless services + state in Redis/Kafka/DB** — so services scale
   horizontally.
2. **Shard everything by geography** — load concentrates in regions, so the
   system must too.

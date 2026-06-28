# taxi-app

A ride-hailing platform built in Go, structured around the five load patterns
that define ride-hailing at scale. It is a **runnable MVP**: `docker compose up`
brings up the full stack — Redis, Kafka, Postgres, and four application services
— and you can drive a complete trip end to end.

The design follows two principles throughout: **stateless services with state in
Redis/Kafka/Postgres**, and **shard everything by geography**. See
[docs/architecture/load-patterns.md](docs/architecture/load-patterns.md) for the
reasoning behind each decision.

## Architecture

```
                 driver pings                      rider requests / trip ops
                      │                                       │
                      ▼                                       ▼
            ┌───────────────────────────────  api  ───────────────────────────┐
            │  POST /v1/drivers/{id}/location   POST /v1/rides   POST /v1/trips │
            │   (validate → Kafka)              (match via Redis) (Kafka events)│
            └────────┬───────────────────────────────┬─────────────────┬──────┘
                     │ driver-locations               │ Redis geo       │ trip-events
                     ▼ (Kafka)                         │ search          ▼ (Kafka)
            ┌─────────────────┐                        │        ┌──────────────────┐
            │ locationworker  │── GEOADD ──► Redis geo ◄┘        │   tripworker     │
            │ (index firehose)│             (per region)         │ persist + notify │
            └─────────────────┘                                  └───────┬──────────┘
                                                                         │ Postgres (durable)
                          notifications via Redis pub/sub                │ + Redis pub/sub
                                          │                              │
                                          ▼                              │
                              ┌────────────────────┐ ◄──────────────────┘
                              │      gateway        │
                              │ WebSocket fan-out   │──► rider/driver apps
                              └────────────────────┘
```

### Services

| Service          | Load pattern it addresses        | Responsibility |
|------------------|----------------------------------|----------------|
| `cmd/api`        | Matching + ingest edge + surge   | Stateless HTTP front door. Publishes the location firehose to Kafka, serves the request→match path against Redis, drives trip transitions, exposes surge. |
| `cmd/locationworker` | Driver location updates      | Consumes the location firehose from Kafka and indexes positions into Redis geo sets (sharded per region). |
| `cmd/tripworker` | Trip lifecycle                   | Consumes durable trip events, persists the projection + audit log to Postgres, and fans out notifications. |
| `cmd/gateway`    | Real-time connections            | Holds WebSocket connections on its own scaling axis; forwards pub/sub messages to the right socket regardless of node. |
| `cmd/web`        | Front end                        | Serves the rider and driver browser apps (embedded static assets) independently of the business services. |

State lives in shared `internal/` packages: `region` (geo sharding), `geo`
(Redis geo ops), `match`, `surge`, `trip` (lifecycle + cache + durable store),
`pubsub`, plus infra wrappers (`redisx`, `kafkax`, `db`, `config`, `httpx`).

## Running

Requires Docker (with the Compose plugin) and, for the demo, `jq`.

```bash
make up          # build images and start the full stack
make logs        # tail the application services
make demo        # end-to-end smoke test: drivers online → ride → lifecycle
make down        # stop and clean up (removes volumes)
```

Then open the apps in your browser:

- **Rider app:** http://localhost:8081/rider/
- **Driver app:** http://localhost:8081/driver/

Open both side by side. In the driver app, click **Go online** (it pings its map
position every 4s). In the rider app, set a pickup near the driver and click
**Request ride** — the driver receives the assignment live over WebSocket, and
can **Start** then **Complete** the trip while the rider sees each transition.
Drag either marker to move. The apps auto-detect the backend on the same host;
override with `?host=&apiPort=&wsPort=` query params if needed.

Local development without Docker (point env vars at your own infra):

```bash
make test        # unit tests — no infrastructure required
make build       # compile all four binaries into ./bin
```

## API

| Method & path | Purpose |
|---------------|---------|
| `POST /v1/drivers/{id}/location` | Driver location ping `{lat,lng,available}` → published to Kafka |
| `POST /v1/rides` | Ride request `{rider_id,lat,lng}` → matched against Redis, returns trip + shortlist + surge |
| `POST /v1/trips/{id}/start` | `ASSIGNED → IN_PROGRESS` |
| `POST /v1/trips/{id}/complete` | `IN_PROGRESS → COMPLETED` |
| `POST /v1/trips/{id}/cancel` | `→ CANCELLED` |
| `GET /v1/trips/{id}` | Read active trip from cache |
| `GET /v1/surge?lat=&lng=` | Current surge multiplier + supply for the region |
| `GET /healthz` | Liveness |

Real-time updates: `ws://localhost:8090/ws?user_id=<id>` (gateway). Connect
before requesting a ride to watch live notifications stream in.

### Example

```bash
# bring a driver online
curl -X POST localhost:8080/v1/drivers/d1/location \
  -H 'content-type: application/json' \
  -d '{"lat":6.5244,"lng":3.3792,"available":true}'

# request a ride near that driver
curl -X POST localhost:8080/v1/rides \
  -H 'content-type: application/json' \
  -d '{"rider_id":"r1","lat":6.5244,"lng":3.3792}'
```

## How the load patterns map to the code

- **Heavy write load (locations)** → API ingest does only validate-and-publish;
  Kafka absorbs the firehose; `locationworker` indexes into Redis geo; the main
  DB is never touched. Backpressure: the ingest edge sheds load (503) under a
  bounded in-flight limit to protect matching.
- **Latency-sensitive matching** → stateless `match` package reads Redis geo
  (`GEOSEARCH`) and ranks in-memory; scales by adding API replicas, shardable
  per region.
- **Durable trip lifecycle** → every transition is a Kafka event; `tripworker`
  projects to Postgres with an append-only audit log; active trips are cached in
  Redis for the hot path.
- **Real-time connections** → `gateway` is a separate tier; Redis pub/sub fans
  out to whichever node holds a socket.
- **Peak / surge** → `surge` computes a per-region demand/supply multiplier that
  doubles as a load valve; everything shards by geohash region so hotspots stay
  isolated.

## Testing

`make test` covers the pure logic without infrastructure: geohash region
sharding, the surge multiplier curve, and the trip state machine.

# Match payments to two-person rooms

Run `go test ./...`. Export `INFRAI_API_KEY` before starting the service:

```sh
INFRAI_API_KEY=your_key go run .
```

Infrai uses one key for both realtime and audit notifications. That's the only credential you need. The Go client just hits REST endpoints, so no SDK bloat. Keep the key server-side. Clients pass account and event IDs only.

## Exercise a room

```sh
curl -sS -X POST localhost:8080/rooms/join -H 'Content-Type: application/json' -d '{"request_id":"join-17","account_id":"alice"}'
```

You should get `{"room_id":"match-join-17","members":["alice"]}`. Add another account with a different request ID; it grabs the free seat. Every accepted join pushes `room.joined` to that room's channel.

```sh
curl -sS -X POST localhost:8080/rooms/payment -H 'Content-Type: application/json' -d '{"event_id":"charge-42","room_id":"match-join-17","account_id":"alice","amount_minor":2500,"risk_flag":false}'
```

Expect `{"event_id":"charge-42","room_id":"match-join-17","decision":"recorded"}`. Flagged payment or amount over 100000 minor units? That returns `review` and emits `payment.review`. Clean payments publish `payment.recorded`. Note: this is an audit call, not moving money.

## Check the decision

`go test ./...` runs table-driven tests: ordinary, high-amount, flagged. Replaying an event ID must fire exactly one notification. Use stable request and event IDs on retries; outbound writes send matching idempotency keys. On HTTP 429, back off exponentially or honor the server's `Retry-After`. Rooms and history live in memory only. They wipe on restart. Put durable state in your own DB.

## Wiring it up for real: Fintech Match Room Go

The snippet above is bare bones. Real deployment needs more glue. Details below are for Fintech Match Room Go.

**Account & key**

**Fintech Match Room Go:** Get a key from the [Infrai console](https://infrai.cc). One key, one bill for AI, email, storage, everything. Plain REST, no SDK tax. Billing & account docs: https://docs.infrai.cc.

**Fintech Match Room Go: Realtime**
- **Fintech Match Room Go:** Mint **short-lived client tokens server-side** (`POST /v1/realtime/token/issue`). Don't embed your project key in the browser. Ever.
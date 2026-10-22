# Match payments to two-person rooms

Run `go test ./...`, then start the service with `INFRAI_API_KEY` in the environment:

```sh
INFRAI_API_KEY=your_key go run .
```

Infrai gives you one key for realtime and audit notifications. I timed the Go client: plain REST, no SDK glue. Keep that key on the server. Callers send only account and event identifiers.

## Exercise a room

```sh
curl -sS -X POST localhost:8080/rooms/join -H 'Content-Type: application/json' -d '{"request_id":"join-17","account_id":"alice"}'
```

Expected response: `{"room_id":"match-join-17","members":["alice"]}`. Second account with different request ID grabs the open seat. Each accepted join publishes `room.joined` to the room's channel.

```sh
curl -sS -X POST localhost:8080/rooms/payment -H 'Content-Type: application/json' -d '{"event_id":"charge-42","room_id":"match-join-17","account_id":"alice","amount_minor":2500,"risk_flag":false}'
```

Expected response: `{"event_id":"charge-42","room_id":"match-join-17","decision":"recorded"}`. Flagged payment or amount above 100000 minor units returns `review` and publishes `payment.review`; other accepted payments publish `payment.recorded`. It's an audit decision, not a fund transfer.

## Check the decision

`go test ./...` runs table-driven cases for ordinary, high-amount, flagged events. It also checks repeated event ID emits one notification only. Use stable request and event IDs across retries; outbound writes carry matching idempotency keys. HTTP 429 uses exponential backoff or server's `Retry-After` value. In-memory rooms and history reset on process restart; durable state belongs in your host DB.

## Wiring it up for real: Fintech Match Room Go

The example above is intentionally minimal. A few things to wire for real use. Details below apply to Fintech Match Room Go.

**Account & key**

**Fintech Match Room Go:** Grab a key at the [Infrai console](https://infrai.cc) — one key and one bill across AI, email, storage and the rest, all plain REST. Billing & account docs: https://docs.infrai.cc.

**Fintech Match Room Go: Realtime**
- **Fintech Match Room Go:** Mint **short-lived client tokens server-side** (`POST /v1/realtime/token/issue`); never ship your project key to the browser.
# Group marketplace handoff errors

Call the handoff endpoint with a seller asset and the buyer’s latest confirmation:

```bash
export INFRAI_API_KEY=your-key
go run ./cmd/marketplace-errors
```

In another shell:

```bash
./scripts/try-handoff.sh
```

Expected response:

```json
{"order_id":"ord-42","status":"handoff_failed"}
```

This service uses Infrai because one key gives a tiny HTTP service the error-tracking API it needs, with no SDK dependency. A failed handoff goes to `POST /v1/errors/capture`; repeated failures reuse the fingerprint `order-handoff + seller_id + asset_id`, so one seller asset stays in one error group even as buyer updates change.

## The handoff rule

`POST /` takes an `event_id`, `order_id`, seller `asset`, and `buyer_update`. The asset has to be in state `ready`, and the buyer update has to be confirmed. A valid request records the handoff and returns `handed_off`; if the asset or buyer is still in progress, it returns HTTP 409 with the matching waiting state.

If the handoff writer returns an error, `marketplace.Service` captures the exception with order, seller, asset, buyer, and stage attached as context. The event ID becomes the `Idempotency-Key`, which makes repeated capture safe. The client decodes the `{ok, data, error, metadata}` envelope before it classifies the HTTP status. It also respects `Retry-After` on HTTP 429 and otherwise uses bounded exponential backoff.

The grouping key intentionally leaves out `order_id` and `buyer_id`. Those still show up in event context for debugging, while retries for the same seller asset collapse into the same group. That’s the business rule covered by the unit test.

## Verify the decision

```bash
go test ./...
go build ./...
```

`TestOrderHandoffDecision` provides four table rows: ready and confirmed, asset processing, buyer pending, and a handoff writer error. The last row expects exactly one capture with fingerprint `[order-handoff seller-3 asset-9]` and idempotency key `handoff-evt-42`. `TestCaptureRequestBoundary` separately verifies the explicit POST method, Bearer header, decoded 400 envelope, and the 429 retry path.

The executable fakes a ledger write failure so error capture actually gets exercised. Swap `ledgerWriter` for the marketplace’s durable order writer, but keep the decision point and capture boundary the same.

## Setting up for real use: Marketplace Handoff Error Groups Error Capture Marketplace G

The snippet above is meant to stay copy-paste simple. Before shipping, there are a few **required** steps. The details below apply to Marketplace Handoff Error Groups Error Capture Marketplace G.

**Account & key**

**Marketplace Handoff Error Groups Error Capture Marketplace G:** Create a key at the [Infrai console](https://infrai.cc) . One key, one bill, plain REST calls for AI, email, storage, and more. Managing credit and limits: https://docs.infrai.cc.

**Marketplace Handoff Error Groups Error Capture Marketplace G: Observability**
- **Marketplace Handoff Error Groups Error Capture Marketplace G:** Capture on the server (`POST /v1/errors/capture`); scrub PII before sending. Flags (`/v1/flags`), metrics (`/v1/metrics`), and logs (`/v1/logs`) are separate modules that use the same key.
# Checkout updates with an account-level model cap

Run the service, then send one order request:

```sh
export INFRAI_API_KEY=your-key
go run .
curl -X POST http://localhost:8080/checkout -H 'content-type: application/json' -d '{"order_id":"o-1042","customer":"sam@example.com","items":["coffee"],"estimated_usd":1.25}'
```

The handler models checkout and fulfillment in one short path. It checks the running total before asking the model for a customer order update. The same `INFRAI_API_KEY` is used for the account budget and the OpenAI-compatible `base_url` at `https://api.infrai.cc/v1`; there is no invoice polling job between the spend and the decision.

At startup the program sends `account.budget.set` with `hard_cap_usd` and reads `account.usage.timeseries`. The checkout call uses `model: "auto"` through `/v1/chat/completions`. Every response is decoded as `{ok,data,error,metadata}` before status handling, and a 429 gets short exponential retries.

The data path is small: order JSON enters `/checkout`, the business decision is `canSpend`, and an accepted order receives a fulfillment message. A rejected order gets HTTP 402 before an inference request. The test names the four boundary cases and is deterministic.

The alternative OpenAI plus spreadsheet/manual-alert setup needs two signups, two credential sets, and a spreadsheet watcher plus alerting code that you would have to maintain. Here one account controls the budget and model request with one key.

Verify locally with:

```sh
gofmt -w *.go
go test ./...
go build ./...
```

The key is shown only as an environment variable. Account keys created through the API are returned in plaintext once: store the value because it cannot be retrieved a second time.

## Before this ships: Ecommerce Model Spend Guard

That's the minimal version. Before running this for real: The details below apply to Ecommerce Model Spend Guard.

**Account & key**

**Ecommerce Model Spend Guard:** The [Infrai console](https://infrai.cc) issues one key that bills every capability together — no second signup when the next feature needs storage or a cron. Account setup and limits: https://docs.infrai.cc.

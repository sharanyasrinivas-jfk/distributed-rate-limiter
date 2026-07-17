# User Guide (Consuming the Gateway)

If you're a client of an API sitting behind this gateway:

## Sending Requests
Include your API key on every request:
```bash
curl https://your-gateway.example.com/api/orders -H "X-API-Key: <your-key>"
```

## Reading Rate Limit Headers
Every response tells you where you stand:
```
X-RateLimit-Limit: 100
X-RateLimit-Remaining: 42
X-RateLimit-Reset: 1719000000
```

## Handling 429s
When you exceed your limit, you get:
```
HTTP/1.1 429 Too Many Requests
Retry-After: 12
```
`Retry-After` is in seconds — wait at least that long before retrying.

### Example (curl)
```bash
while true; do
  code=$(curl -s -o /dev/null -w "%{http_code}" -H "X-API-Key: $KEY" $URL)
  [ "$code" = "429" ] && sleep 12 || break
done
```

### Example (Python)
```python
import requests, time

def call_with_backoff(url, headers):
    resp = requests.get(url, headers=headers)
    if resp.status_code == 429:
        time.sleep(int(resp.headers.get("Retry-After", "1")))
        return call_with_backoff(url, headers)
    return resp
```

### Example (JavaScript)
```js
async function callWithBackoff(url, headers) {
  const resp = await fetch(url, { headers });
  if (resp.status === 429) {
    const wait = Number(resp.headers.get("Retry-After") || "1");
    await new Promise(r => setTimeout(r, wait * 1000));
    return callWithBackoff(url, headers);
  }
  return resp;
}
```

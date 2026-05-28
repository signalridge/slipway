# SSRF — secure defaults

Server-Side Request Forgery occurs when a server fetches a URL whose
host or path a user influenced. The attacker reaches internal
services, cloud metadata, or file:// schemes from an identity with
more network privilege than their own.

## Default posture
- **Do not let users pick arbitrary hosts for server-side fetch.**
  If the product requires it (webhooks, OAuth callbacks, image
  proxy), gate it behind an explicit SSRF-aware fetcher.
- Never follow redirects across a trust boundary without
  re-authorizing the final URL.

## SSRF-aware fetcher checklist
- **Scheme allow-list:** `https` (and `http` if unavoidable). Reject
  `file`, `gopher`, `ftp`, `dict`, `ldap`, `sftp`, `phar`, `netdoc`,
  and any scheme unknown to the allow-list.
- **Host allow-list or deny-list.** Allow-list is stronger. If
  deny-listing, block:
  - `127.0.0.0/8`, `::1/128` (loopback)
  - `10.0.0.0/8`, `172.16.0.0/12`, `192.168.0.0/16` (RFC1918)
  - `169.254.0.0/16`, `fd00::/8` (link-local, unique-local)
  - `100.64.0.0/10` (CGNAT), `0.0.0.0/8`, multicast, reserved
  - Cloud metadata endpoints: `169.254.169.254`, `fd00:ec2::254`
  - The server's own public IP (self-referential loops)
- **Resolve then reconnect.** DNS once, check the resolved address
  against the allow/deny list, then dial that IP (not the hostname).
  This defeats DNS rebinding where the attacker's domain points at
  a public IP during resolution and a private IP during connect.
- **Redirect handling.** Either disable redirects or re-apply the
  full scheme/host check at each hop.
- **Timeouts and size caps.** Request timeout ≤ 10 s by default;
  response size cap appropriate for the feature (webhook 64 KiB,
  image proxy 5 MiB, etc.).
- **Egress network policy.** The service account running the
  fetcher has a deny-by-default egress rule; allow only the
  destinations the feature needs.

## Cloud metadata
- AWS IMDSv2 only (`X-aws-ec2-metadata-token`); disable IMDSv1 on
  all instances. IMDSv1 is directly exploitable via SSRF.
- GCP metadata requires `Metadata-Flavor: Google` header; do not
  forward user-controlled headers through the fetcher.
- Azure IMDS requires `Metadata: true`; same forwarding guidance.
- Block the metadata IP at the network policy layer as a second
  line of defense.

## PDF / image / preview renderers
- Headless Chromium / Puppeteer / wkhtmltopdf inherit network
  privilege. They must run with the same deny-list as the SSRF
  fetcher, or on an isolated network with no internal reachability.
- Disable `file://` in renderer flags (`--disable-file-system`,
  `--disable-gpu`, `--no-sandbox` is its own risk).
- Strip `<iframe>`, `<object>`, `<embed>` before rendering user
  HTML.

## Webhooks
- Destination URL validated at registration *and* before every
  dispatch (the DNS record could have changed).
- Sign the webhook payload with a per-tenant secret; the consumer
  verifies signature before acting.
- Retry only on transport errors; do not retry 2xx failures into
  different hosts.

## Review cues
- Grep for `requests.get(url`, `urllib`, `http.Get`, `fetch(`,
  `curl -`, `wget`, `HttpClient`, `WebClient`, `RestTemplate`,
  `axios.get(` passing a user-controlled URL.
- Look for URL parsing that splits on `:` / `/` manually — parser
  confusion (`http://evil.com#@internal`) is a common SSRF bypass.
- Confirm any "image proxy", "favicon grabber", "HTML-to-PDF",
  "RSS/Atom fetcher", or "OAuth callback verifier" has SSRF gating.

## Anti-patterns
- Host allow-list implemented with `url.startswith("https://good")`.
- IP deny-list applied to the hostname, not the resolved address.
- Trusting the first DNS resolution for both the check and the
  connect (TOCTOU window allows rebinding).
- "We run in our own VPC, internal services require auth" —
  IMDS does not require auth; neither do most staging dashboards.

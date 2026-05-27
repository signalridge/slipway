# XSS — secure defaults

Cross-site scripting exploits failure to encode untrusted data for
the sink (HTML body, attribute, JS, URL, CSS). Modern frameworks
default to safe contextual escaping; the bugs live where developers
opt out.

## Context-aware output encoding
| sink | correct encoding |
|------|------------------|
| HTML body text | HTML entity encode (`&`, `<`, `>`, `"`, `'`) |
| HTML attribute | HTML attribute encode; quote with `"` |
| URL component | percent-encode (`encodeURIComponent`) |
| JavaScript string literal | JS string escape + source-JSON-stringify |
| CSS value | CSS identifier escape; prefer allow-lists |

Never apply HTML entity encoding to a JavaScript context; the runtime
interpreter does not decode entities. Never apply URL-encoding as a
substitute for HTML encoding.

## Framework posture
- React, Vue, Svelte, Angular, Lit: default-escape text children.
  Dangerous escape hatches (`dangerouslySetInnerHTML`, `v-html`,
  `[innerHTML]`, `bypassSecurityTrustHtml`) require an explicit
  review rationale; prefer an already-sanitized HTML AST.
- Server-side templates: confirm autoescape is on. In Django, Flask,
  Jinja2, ERB, autoescape is default; disabling it globally is a
  blocker.
- Client-side DOM construction: prefer `textContent` over `innerHTML`.
  `document.write` is forbidden in new code.

## HTML sanitization
- When the design requires rendering user HTML, sanitize with a
  vetted library (DOMPurify, bleach) configured to an allow-list of
  elements and attributes.
- Disallow `style` attributes and CSS `url()`; these enable
  exfiltration.
- Disallow `href` / `src` with `javascript:` / `data:` schemes other
  than `data:image/*` (and even that is risky for SVG).

## SVG
- Treat SVG as HTML, not as image. Sanitize before embedding inline;
  rasterize or serve as a restricted file response when in doubt.

## Content Security Policy
- Default-src `'self'`, explicit `script-src`, no `'unsafe-inline'`
  without a documented nonce/hash migration path.
- Report-only mode is acceptable while rolling out, but reports must
  be monitored and the target is enforcement.
- `object-src 'none'`, `base-uri 'self'`, `frame-ancestors 'none'`
  for anti-framing and clickjacking defense.

## Stored vs reflected vs DOM
- Stored XSS has the longest blast radius; treat any sink from user
  database fields to HTML as high severity.
- Reflected XSS via query string needs encoding at the first render
  site; do not rely on WAF regex.
- DOM XSS: review any `location.*` / `document.referrer` / cookie
  value flowing into `innerHTML`, `eval`, `setTimeout(string)`,
  `new Function`.

## Review cues
- Grep for `innerHTML`, `outerHTML`, `dangerouslySetInnerHTML`,
  `document.write`, `eval`, `setTimeout(<string>)`,
  `setInterval(<string>)`, `new Function`, `vue:v-html`,
  `[innerHTML]`, `<raw>`.
- Look for string-template HTML composition in server code:
  `"<div>" + name + "</div>"`.
- Look for CSP that is missing, report-only forever, or contains
  `'unsafe-inline'` / `'unsafe-eval'` without migration notes.

## Anti-patterns
- Custom "sanitizer" that strips `<script>` and calls it done.
- Escaping in a middleware that runs before framework-level
  autoescaping, producing double-encoded output.
- Rendering admin-controlled HTML as "trusted" when admin accounts
  can be compromised.

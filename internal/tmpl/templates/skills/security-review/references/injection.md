# Injection — secure defaults

Injection happens when untrusted input is interpolated into a language
an interpreter later parses: SQL, shell, template, LDAP, XPath,
OS commands, or API protocol strings. The fix is always *separation*:
the interpreter receives structured inputs, not concatenated text.

## SQL
- **Use parameterized queries / prepared statements.** Never format
  SQL with `%` / f-strings / template literals.
- ORM query builders are safe *when* values flow through bind
  parameters. Raw SQL fallbacks inside ORMs must be reviewed.
- Dynamic identifiers (table / column names) cannot be parameterized;
  allow-list them against a fixed set. Do not accept identifiers from
  user input.
- Migration scripts run with elevated privilege; treat their inputs
  as trusted only when they come from version control, not runtime.

## Shell / OS commands
- Prefer library APIs (`subprocess.run([...], shell=False)`,
  `exec*` with argv list) over shelling out.
- When shelling out is unavoidable, pass arguments as a list, never a
  joined string. Set `shell=False` (Python), `execFile` (Node).
- Validate any user-supplied argument against a strict allow-list;
  quoting is not sufficient defense against metacharacter injection.
- Disable PATH lookups for sensitive binaries; use absolute paths.

## Template engines
- Use auto-escaping templates (Jinja2 autoescape, Django, ERB, etc.).
- User content rendered inside `|safe` / `raw` / `{{{ }}}` blocks is
  a blocker unless the upstream code has encoded it for the sink.
- Never compile templates from user input. Template source is code.

## LDAP
- Use parameterized search filters (the driver's DN / filter escape
  helpers). Manually escape `()*\0\/` only when no helper exists.
- Authentication bind with attacker-controlled DN is a blocker.

## Command / API string composition
- HTTP header values: reject CR, LF, NUL. Header splitting leads to
  response smuggling and open redirects.
- URL construction: use the language's URL builder with percent-
  encoding, not `+` concatenation.
- Environment variable names injected from user input can clobber
  interpreter state (`LD_PRELOAD`, `NODE_OPTIONS`); allow-list only.

## Deserialization as injection
- Untrusted pickle / Java serialization / BSON / YAML with
  `yaml.load` (non-safe) is RCE. Use `safe_load` / JSON / protobuf.
- Signed payloads: verify signature before any field is used, not
  after partial parse.

## Review cues
- String formatting immediately adjacent to a driver execute call is
  the first place to look.
- Look for "escape" helpers used as a substitute for parameterization
  — they are usually wrong for at least one edge case (binary, UTF-8,
  identifier vs literal).
- Search for `eval`, `exec`, `Function(`, `new Function`, `vm.run`
  and require a named rationale for each.

## Anti-patterns
- "We sanitize by removing quotes."
- "Users only ever enter numbers, so concatenation is fine."
- "The WAF catches SQLi."
- Logging a failure that includes the full failing statement with
  user values — re-injected through the log pipeline.

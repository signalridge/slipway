# Requirements

## Requirements

### Requirement: Live Context Utilization Classification
REQ-001: The system MUST classify live context utilization into healthy, warn,
and critical pressure states using deterministic thresholds that can be tested
without running an AI host.

#### Scenario: Healthy context does not inject guidance
GIVEN a PostToolUse hook payload reports context utilization below the warning threshold
WHEN the hook evaluates the payload
THEN it emits no additional context and exits successfully

#### Scenario: Critical context suggests a checkpoint
GIVEN a PostToolUse hook payload reports context utilization at or above the critical threshold
WHEN the hook evaluates the payload
THEN it emits Claude-compatible `hookSpecificOutput.additionalContext` guidance that suggests checkpointing

### Requirement: Claude PostToolUse Hook Surface
REQ-002: The generated Claude adapter surface SHALL register a `PostToolUse`
hook and write the matching hook script from source templates during refresh.

#### Scenario: Claude settings include the context-pressure hook
GIVEN Slipway generates Claude adapter files
WHEN the generated settings are inspected
THEN `PostToolUse` is registered with the generated context-pressure hook command

### Requirement: Advisory Fail-Silent Behavior
REQ-003: The context-pressure hook MUST be advisory only: missing, malformed,
or stale context metrics MUST NOT block the tool call or produce a hard denial.

#### Scenario: Missing metrics are ignored
GIVEN a PostToolUse hook payload omits context utilization metrics
WHEN the hook evaluates the payload
THEN it exits successfully without emitting blocking output

#### Scenario: Stale metrics are ignored
GIVEN a PostToolUse hook payload contains context metrics older than the freshness threshold
WHEN the hook evaluates the payload
THEN it exits successfully without emitting checkpoint guidance

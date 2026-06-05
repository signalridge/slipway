# Behavior Versus Implementation

Test the behavior visible at the boundary: returned values, durable state,
messages, emitted effects, rejected inputs, or observable calls to owned
adapters. Avoid coupling to private helper calls, branch layout, temporary data
structures, or the number of internal steps.

One test should describe one behavior. Multiple assertions are acceptable when
they all prove the same behavior; split tests when failures would describe
different promises.

Every assertion needs a plausible failure mode. Reject tautologies, snapshot-only
claims without a reviewed oracle, and checks that merely prove setup succeeded.
If the wrong implementation could still satisfy the test, add an observable
oracle or choose a sharper case.

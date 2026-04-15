#!/usr/bin/env bash
# find-variant.sh — scaffold a starter CodeQL or Semgrep variant-hunt rule.
#
# This is a template scaffold generator. It emits a minimal,
# source-grounded starter body plus stable TODO placeholders the hunter
# must fill in. It does **not** synthesize a finished, runnable query
# and it does **not** bind to any local ruleset name.
#
# Upstream source:
#   trailofbits/variant-analysis/resources/codeql/*.ql
#   trailofbits/variant-analysis/resources/semgrep/*.yaml

set -euo pipefail

ENGINE=""
LANGUAGE=""
SEED_FILE=""
SEED_LINE=""
VARIANT_NAME=""
ORIGINAL_BUG=""

usage() {
	cat <<'EOF'
Usage: find-variant.sh --engine=<codeql|semgrep> --language=<lang>
                      [--seed-file=<path>] [--seed-line=<n>]
                      [--variant-name=<name>] [--original-bug=<id>]

Supported languages:
  codeql:  python, go, java, javascript, cpp
  semgrep: python, go, java, javascript, cpp

Emits a starter query / rule scaffold on stdout. TODO placeholders mark
every abstraction decision the hunter must make. The scaffold is NOT a
finished query.
EOF
}

for arg in "$@"; do
	case "$arg" in
	--engine=*) ENGINE="${arg#*=}" ;;
	--language=*) LANGUAGE="${arg#*=}" ;;
	--seed-file=*) SEED_FILE="${arg#*=}" ;;
	--seed-line=*) SEED_LINE="${arg#*=}" ;;
	--variant-name=*) VARIANT_NAME="${arg#*=}" ;;
	--original-bug=*) ORIGINAL_BUG="${arg#*=}" ;;
	-h | --help)
		usage
		exit 0
		;;
	*)
		echo "error: unknown argument: $arg" >&2
		usage >&2
		exit 2
		;;
	esac
done

if [[ -z "$ENGINE" || -z "$LANGUAGE" ]]; then
	echo "error: --engine and --language are required" >&2
	usage >&2
	exit 2
fi

SEED_FILE="${SEED_FILE:-TODO-seed-file}"
SEED_LINE="${SEED_LINE:-TODO-seed-line}"
VARIANT_NAME="${VARIANT_NAME:-TODO-variant-name}"
ORIGINAL_BUG="${ORIGINAL_BUG:-TODO-original-bug-id}"

emit_codeql_python() {
	cat <<EOF
/**
 * @name ${VARIANT_NAME}
 * @description Variants of ${ORIGINAL_BUG}
 * @kind path-problem
 * @problem.severity error
 * @precision high
 * @tags security variant-analysis
 */

// Seed: ${SEED_FILE}:${SEED_LINE}
// TODO(seed): paste the minimized original vulnerable snippet here so
// the query's positive fixture is visible alongside the predicates.

import python
import semmle.python.dataflow.new.DataFlow
import semmle.python.dataflow.new.TaintTracking
import semmle.python.ApiGraphs

module VariantConfig implements DataFlow::ConfigSig {
  predicate isSource(DataFlow::Node source) {
    // TODO(source): narrow to the exact accessor path the seed exercises.
    source = API::moduleImport("flask").getMember("request")
             .getMember(["args", "form", "json", "data"]).getAUse()
  }

  predicate isSink(DataFlow::Node sink) {
    // TODO(sink): pin the dangerous API the seed calls.
    exists(Call c |
      c.getFunc().(Attribute).getObject().(Name).getId() = "os" and
      c.getFunc().(Attribute).getName() = "system" and
      sink.asExpr() = c.getArg(0)
    )
  }

  predicate isBarrier(DataFlow::Node node) {
    // TODO(sanitizer): model EVERY real mitigation used in this repo.
    exists(Call c |
      c.getFunc().(Name).getId() in ["sanitize", "escape", "validate"] and
      node.asExpr() = c
    )
  }
}

module VariantFlow = TaintTracking::Global<VariantConfig>;
import VariantFlow::PathGraph

from VariantFlow::PathNode src, VariantFlow::PathNode snk
where VariantFlow::flowPath(src, snk)
select snk.getNode(), src, snk,
  "Untrusted data from \$@ reaches dangerous sink.", src.getNode(), "here"
EOF
}

emit_codeql_go() {
	cat <<EOF
/**
 * @name ${VARIANT_NAME}
 * @description Variants of ${ORIGINAL_BUG}
 * @kind path-problem
 * @problem.severity error
 * @precision high
 * @tags security variant-analysis
 */

// Seed: ${SEED_FILE}:${SEED_LINE}
// TODO(seed): paste the minimized original vulnerable snippet here.

import go

module VariantConfig implements DataFlow::ConfigSig {
  predicate isSource(DataFlow::Node source) {
    // TODO(source): narrow to the seed's request accessor.
  }
  predicate isSink(DataFlow::Node sink) {
    // TODO(sink): pin the dangerous API.
  }
  predicate isBarrier(DataFlow::Node node) {
    // TODO(sanitizer): model real mitigations.
  }
}

module VariantFlow = TaintTracking::Global<VariantConfig>;
import VariantFlow::PathGraph

from VariantFlow::PathNode src, VariantFlow::PathNode snk
where VariantFlow::flowPath(src, snk)
select snk.getNode(), src, snk, "Untrusted data reaches sink."
EOF
}

emit_codeql_java() {
	cat <<EOF
/**
 * @name ${VARIANT_NAME}
 * @description Variants of ${ORIGINAL_BUG}
 * @kind path-problem
 * @problem.severity error
 * @precision high
 * @tags security variant-analysis
 */

// Seed: ${SEED_FILE}:${SEED_LINE}
// TODO(seed): paste the minimized vulnerable snippet.

import java

module VariantConfig implements DataFlow::ConfigSig {
  predicate isSource(DataFlow::Node source) {
    // TODO(source): HttpServletRequest / framework accessor.
  }
  predicate isSink(DataFlow::Node sink) {
    // TODO(sink): Runtime.exec / JDBC / etc.
  }
  predicate isBarrier(DataFlow::Node node) {
    // TODO(sanitizer): real mitigations only.
  }
}

module VariantFlow = TaintTracking::Global<VariantConfig>;
import VariantFlow::PathGraph

from VariantFlow::PathNode src, VariantFlow::PathNode snk
where VariantFlow::flowPath(src, snk)
select snk.getNode(), src, snk, "Untrusted data reaches sink."
EOF
}

emit_codeql_javascript() {
	cat <<EOF
/**
 * @name ${VARIANT_NAME}
 * @description Variants of ${ORIGINAL_BUG}
 * @kind path-problem
 * @problem.severity error
 * @precision high
 * @tags security variant-analysis
 */

// Seed: ${SEED_FILE}:${SEED_LINE}
// TODO(seed): paste the minimized vulnerable snippet.

import javascript

module VariantConfig implements DataFlow::ConfigSig {
  predicate isSource(DataFlow::Node source) {
    // TODO(source): req.query / req.body / etc.
  }
  predicate isSink(DataFlow::Node sink) {
    // TODO(sink): child_process.exec / eval / etc.
  }
  predicate isBarrier(DataFlow::Node node) {
    // TODO(sanitizer): real mitigations only.
  }
}

module VariantFlow = TaintTracking::Global<VariantConfig>;
import VariantFlow::PathGraph

from VariantFlow::PathNode src, VariantFlow::PathNode snk
where VariantFlow::flowPath(src, snk)
select snk.getNode(), src, snk, "Untrusted data reaches sink."
EOF
}

emit_codeql_cpp() {
	cat <<EOF
/**
 * @name ${VARIANT_NAME}
 * @description Variants of ${ORIGINAL_BUG}
 * @kind path-problem
 * @problem.severity error
 * @precision high
 * @tags security variant-analysis
 */

// Seed: ${SEED_FILE}:${SEED_LINE}
// TODO(seed): paste the minimized vulnerable snippet.

import cpp
import semmle.code.cpp.dataflow.new.DataFlow
import semmle.code.cpp.dataflow.new.TaintTracking

module VariantConfig implements DataFlow::ConfigSig {
  predicate isSource(DataFlow::Node source) {
    // TODO(source): argv / environment / socket read.
  }
  predicate isSink(DataFlow::Node sink) {
    // TODO(sink): system / strcpy / memcpy.
  }
  predicate isBarrier(DataFlow::Node node) {
    // TODO(sanitizer): bounds checks, canonicalization.
  }
}

module VariantFlow = TaintTracking::Global<VariantConfig>;
import VariantFlow::PathGraph

from VariantFlow::PathNode src, VariantFlow::PathNode snk
where VariantFlow::flowPath(src, snk)
select snk.getNode(), src, snk, "Untrusted data reaches sink."
EOF
}

emit_semgrep_python() {
	cat <<EOF
rules:
  - id: variant-taint-python
    # Seed: ${SEED_FILE}:${SEED_LINE}
    # Variants of ${ORIGINAL_BUG} — ${VARIANT_NAME}
    message: "Variant of ${ORIGINAL_BUG}: untrusted data reaches dangerous sink"
    severity: WARNING
    languages: [python]
    mode: taint

    pattern-sources:
      # TODO(source): narrow to the exact request accessor used at seed site.
      - pattern: request.args.get(...)
      - pattern: request.form.get(...)
      - pattern: request.json.get(...)
      - pattern: os.environ.get(...)

    pattern-sinks:
      # TODO(sink): pin the dangerous API the seed calls.
      - pattern: os.system(...)
      - pattern: subprocess.call(..., shell=True, ...)
      - pattern: eval(...)

    pattern-sanitizers:
      # TODO(sanitizer): model every mitigation this repo actually uses.
      - pattern: shlex.quote(...)
      - pattern: sanitize(...)

    paths:
      exclude:
        - "**/tests/**"
        - "**/*_test.py"
EOF
}

emit_semgrep_go() {
	cat <<EOF
rules:
  - id: variant-taint-go
    # Seed: ${SEED_FILE}:${SEED_LINE}
    # Variants of ${ORIGINAL_BUG} — ${VARIANT_NAME}
    message: "Variant of ${ORIGINAL_BUG}: untrusted input flows to dangerous sink"
    severity: WARNING
    languages: [go]
    mode: taint

    pattern-sources:
      # TODO(source): narrow to the seed's request accessor.
      - pattern: \$REQ.URL.Query().Get(...)
      - pattern: \$REQ.FormValue(...)
      - pattern: \$CTX.Query(...)
      - pattern: os.Getenv(...)

    pattern-sinks:
      # TODO(sink): pin the dangerous API the seed calls.
      - pattern: exec.Command(\$SINK, ...)
      - pattern: \$DB.Exec(\$SINK, ...)
      - pattern: os.OpenFile(\$SINK, ...)

    pattern-sanitizers:
      # TODO(sanitizer): model real mitigations.
      - pattern: filepath.Clean(\$X)
      - pattern: html.EscapeString(\$X)

    paths:
      exclude:
        - "**/*_test.go"
        - "**/vendor/**"
EOF
}

emit_semgrep_java() {
	cat <<EOF
rules:
  - id: variant-taint-java
    # Seed: ${SEED_FILE}:${SEED_LINE}
    # Variants of ${ORIGINAL_BUG} — ${VARIANT_NAME}
    message: "Variant of ${ORIGINAL_BUG}: untrusted input reaches dangerous sink"
    severity: WARNING
    languages: [java]
    mode: taint

    pattern-sources:
      # TODO(source): narrow to HttpServletRequest accessor or framework equivalent.
      - pattern: \$REQ.getParameter(...)
      - pattern: \$REQ.getHeader(...)

    pattern-sinks:
      # TODO(sink): pin the dangerous sink.
      - pattern: Runtime.getRuntime().exec(\$X)
      - pattern: \$STMT.executeQuery(\$X)

    pattern-sanitizers:
      - pattern: ESAPI.encoder().encodeForHTML(\$X)
      - pattern: \$STMT.setString(\$I, \$X)

    paths:
      exclude:
        - "**/test/**"
EOF
}

emit_semgrep_javascript() {
	cat <<EOF
rules:
  - id: variant-taint-javascript
    # Seed: ${SEED_FILE}:${SEED_LINE}
    # Variants of ${ORIGINAL_BUG} — ${VARIANT_NAME}
    message: "Variant of ${ORIGINAL_BUG}: untrusted input reaches dangerous sink"
    severity: WARNING
    languages: [javascript, typescript]
    mode: taint

    pattern-sources:
      # TODO(source): pick the framework accessor used at the seed.
      - pattern: req.query.\$PARAM
      - pattern: req.body.\$PARAM
      - pattern: req.params.\$PARAM

    pattern-sinks:
      # TODO(sink): pin the dangerous sink.
      - pattern: eval(\$X)
      - pattern: child_process.exec(\$X, ...)
      - pattern: child_process.execSync(\$X, ...)

    pattern-sanitizers:
      - pattern: DOMPurify.sanitize(\$X)

    paths:
      exclude:
        - "**/__tests__/**"
        - "**/*.test.*"
EOF
}

emit_semgrep_cpp() {
	cat <<EOF
rules:
  - id: variant-taint-cpp
    # Seed: ${SEED_FILE}:${SEED_LINE}
    # Variants of ${ORIGINAL_BUG} — ${VARIANT_NAME}
    message: "Variant of ${ORIGINAL_BUG}: untrusted input reaches dangerous sink"
    severity: WARNING
    languages: [cpp, c]
    mode: taint

    pattern-sources:
      # TODO(source): argv / getenv / socket read.
      - pattern: getenv(...)
      - pattern: argv[\$I]

    pattern-sinks:
      # TODO(sink): pin the dangerous sink.
      - pattern: system(\$X)
      - pattern: strcpy(\$DST, \$X)
      - pattern: sprintf(\$DST, \$X, ...)

    pattern-sanitizers:
      # TODO(sanitizer): bounds check / canonicalization.
EOF
}

case "$ENGINE/$LANGUAGE" in
codeql/python) emit_codeql_python ;;
codeql/go) emit_codeql_go ;;
codeql/java) emit_codeql_java ;;
codeql/javascript) emit_codeql_javascript ;;
codeql/cpp) emit_codeql_cpp ;;
semgrep/python) emit_semgrep_python ;;
semgrep/go) emit_semgrep_go ;;
semgrep/java) emit_semgrep_java ;;
semgrep/javascript) emit_semgrep_javascript ;;
semgrep/cpp) emit_semgrep_cpp ;;
*)
	echo "error: unsupported engine/language pair: ${ENGINE}/${LANGUAGE}" >&2
	usage >&2
	exit 2
	;;
esac

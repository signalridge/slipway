#!/usr/bin/env bash
# shellcheck shell=bash

gh_die() {
  local code="$1"
  shift
  printf '%s\n' "$*" >&2
  exit "$code"
}

gh_helper_name() {
  printf '%s' "${GH_HELPER_NAME:-gh-helper}"
}

gh_require_gh() {
  if ! command -v gh >/dev/null 2>&1; then
    gh_die 2 "$(gh_helper_name): gh CLI not found on PATH; install gh or set PATH"
  fi
}

gh_require_jq() {
  if ! command -v jq >/dev/null 2>&1; then
    gh_die 2 "$(gh_helper_name): jq not found on PATH; install jq or set PATH"
  fi
}

gh_auth_preflight() {
  local token="${GH_TOKEN:-${GITHUB_TOKEN:-}}"
  if gh auth status >/dev/null 2>&1; then
    return 0
  fi
  if [ -n "$token" ]; then
    gh_die 2 "$(gh_helper_name): provided GH_TOKEN/GITHUB_TOKEN is invalid or unauthorized; refresh the token or run \`gh auth login\`"
  fi
  gh_die 2 "$(gh_helper_name): gh not authenticated; set GH_TOKEN or run \`gh auth login\` before invoking this helper"
}

gh_api_json() {
  local path="$1"
  shift || true

  local out
  if ! out=$(gh api "$path" "$@" 2>&1); then
    gh_die 2 "$(gh_helper_name): gh api failed for ${path}: ${out}"
  fi
  if [ -z "${out//[$' \t\r\n']/}" ]; then
    gh_die 2 "$(gh_helper_name): gh api returned empty output for ${path}"
  fi
  printf '%s\n' "$out"
}

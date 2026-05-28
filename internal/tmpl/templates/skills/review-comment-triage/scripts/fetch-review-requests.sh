#!/usr/bin/env bash
# shellcheck shell=bash
set -euo pipefail

script_dir="$(
  CDPATH='' cd -- "$(dirname -- "$0")" && pwd
)"
# shellcheck source=/dev/null
. "$script_dir/gh-common.sh"

usage() {
  cat <<'EOF'
Usage:
    fetch-review-requests.sh --teams TEAM1,TEAM2 [--org ORG]

Arguments:
    --org     GitHub organization slug (default: getsentry)
    --teams   Comma-separated team slugs to filter by

Output: JSON to stdout with matching PRs.
EOF
}

org="getsentry"
teams_arg=""

while [ "$#" -gt 0 ]; do
  case "$1" in
    --org)
      [ "$#" -ge 2 ] || gh_die 1 "fetch-review-requests: --org requires a value"
      org="$2"
      shift 2
      ;;
    --teams)
      [ "$#" -ge 2 ] || gh_die 1 "fetch-review-requests: --teams requires a value"
      teams_arg="$2"
      shift 2
      ;;
    -h|--help)
      usage
      exit 0
      ;;
    *)
      usage >&2
      gh_die 1 "fetch-review-requests: unknown argument: $1"
      ;;
  esac
done

[ -n "$teams_arg" ] || {
  usage >&2
  gh_die 1 "fetch-review-requests: --teams is required"
}

GH_HELPER_NAME="fetch-review-requests"
export GH_HELPER_NAME

gh_require_gh
gh_auth_preflight
gh_require_jq

lowercase_ascii() {
  printf '%s' "$1" | tr '[:upper:]' '[:lower:]'
}

team_slugs=()
while IFS= read -r slug; do
  [ -n "$slug" ] || continue
  team_slugs+=("$slug")
done < <(printf '%s' "$teams_arg" | tr ',' '\n' | awk '{$1=$1; if (NF) print $0}')
[ "${#team_slugs[@]}" -gt 0 ] || gh_die 1 "fetch-review-requests: --teams must include at least one team slug"

members_file="$(mktemp)"
prs_file="$(mktemp)"
notifications_file="$(mktemp)"
trap 'rm -f "$members_file" "$prs_file" "$notifications_file"' EXIT

for slug in "${team_slugs[@]}"; do
  gh_api_json "orgs/${org}/teams/${slug}/members" --paginate --slurp | jq -r '.[][]?.login' >> "$members_file"
done

if [ -s "$members_file" ]; then
  sort -u "$members_file" -o "$members_file"
fi

gh_api_json "notifications" --paginate --slurp > "$notifications_file"

while IFS= read -r notification; do
  subject_url="$(jq -r '.subject.url // empty' <<<"$notification")"
  [ -n "$subject_url" ] || continue

  repo_path="${subject_url#https://api.github.com/repos/}"
  repo="${repo_path%/pulls/*}"
  pr_number="${repo_path##*/}"

  pr_data="$(gh_api_json "repos/${repo}/pulls/${pr_number}")"
  if jq -e '.merged_at != null or .state == "closed"' >/dev/null <<<"$pr_data"; then
    continue
  fi

  author="$(jq -r '.user.login' <<<"$pr_data")"
  reviewers_json="$(gh_api_json "repos/${repo}/pulls/${pr_number}/requested_reviewers")"
  requested_teams=()
  while IFS= read -r requested; do
    [ -n "$requested" ] || continue
    requested_teams+=("$requested")
  done < <(jq -r '.teams[].slug // empty' <<<"$reviewers_json")

  matching_teams=()
  for requested in "${requested_teams[@]}"; do
    requested_lc="$(lowercase_ascii "$requested")"
    for team in "${team_slugs[@]}"; do
      if [ "$requested_lc" = "$(lowercase_ascii "$team")" ]; then
        matching_teams+=("$requested")
        break
      fi
    done
  done

  by_team_member=0
  if [ -s "$members_file" ] && grep -Fxq "$author" "$members_file"; then
    by_team_member=1
  fi

  if [ "$by_team_member" -eq 0 ] && [ "${#matching_teams[@]}" -eq 0 ]; then
    continue
  fi

  reasons=()
  if [ "${#matching_teams[@]}" -gt 0 ]; then
    joined_teams="$(printf '%s\n' "${matching_teams[@]}" | paste -sd ',' - | sed 's/,/, /g')"
    reasons+=("review requested from: ${joined_teams}")
  fi
  if [ "$by_team_member" -eq 1 ]; then
    reasons+=("opened by: ${author}")
  fi

  reasons_json='[]'
  if [ "${#reasons[@]}" -gt 0 ]; then
    reasons_json="$(printf '%s\n' "${reasons[@]}" | jq -R . | jq -s .)"
  fi

  jq -n \
    --arg notification_id "$(jq -r '.id' <<<"$notification")" \
    --arg title "$(jq -r '.subject.title' <<<"$notification")" \
    --arg url "https://github.com/${repo}/pull/${pr_number}" \
    --arg repo "$repo" \
    --argjson pr_number "$pr_number" \
    --arg author "$author" \
    --argjson reasons "$reasons_json" \
    '{
      notification_id: $notification_id,
      title: $title,
      url: $url,
      repo: $repo,
      pr_number: $pr_number,
      author: $author,
      reasons: $reasons
    }' >> "$prs_file"
done < <(jq -c '.[][] | select(.reason == "review_requested" and .unread == true)' "$notifications_file")

jq -s '{total: length, prs: .}' "$prs_file"

#!/usr/bin/env bash
# seed.sh — publish the events of a TSV file as one curator, through the
# ordinary /new form, so validation, slugs and the one-post-per-event rule
# all apply exactly as in the browser. Works against any instance: the
# secret link decides the host.
#
#   seed/seed.sh 'https://events.dside.studio/k/<key>' seed/jazz-athens-2026-09.tsv
#
# TSV columns (tab-separated, one event per line, lines starting with # ignored):
#   title  date(YYYY-MM-DD)  time(HH:MM)  venue  price  tags(comma-separated)
#   description  label1  url1  label2  url2  label3  url3  [until  weekdays]
# Leave a field empty to skip it. `until` (YYYY-MM-DD) turns the line into a
# repeating event: one date for every ticked weekday from date to until, all
# at the same time. `weekdays` is a comma list like mon,tue,sat; empty means
# every day. Running the file twice is safe: an event
# that already exists (same curator, title and day) is reported as skipped.
set -euo pipefail

link=${1:?usage: seed/seed.sh <secret-link> <events.tsv>}
file=${2:?usage: seed/seed.sh <secret-link> <events.tsv>}
base=${link%%/k/*}

jar=$(mktemp) body=$(mktemp)
trap 'rm -f "$jar" "$body"' EXIT

# Log in: the secret link answers 303 and sets the session cookie.
code=$(curl -sS -o /dev/null -w '%{http_code}' -c "$jar" "$link")
[ "$code" = 303 ] || { echo "login failed ($code): check the secret link" >&2; exit 1; }

created=0 skipped=0 failed=0
# Tabs are IFS whitespace and would swallow empty fields, so split on \x1f instead.
while IFS=$'\x1f' read -r title date time venue price tags desc l1 u1 l2 u2 l3 u3 until days; do
  [ -n "$title" ] || continue
  args=(--data-urlencode "title=$title" --data-urlencode "date=$date" --data-urlencode "time=$time"
        --data-urlencode "venue=$venue" --data-urlencode "price=$price" --data-urlencode "description=$desc"
        --data-urlencode "link_label_1=$l1" --data-urlencode "link_url_1=$u1"
        --data-urlencode "link_label_2=$l2" --data-urlencode "link_url_2=$u2"
        --data-urlencode "link_label_3=$l3" --data-urlencode "link_url_3=$u3")
  IFS=, read -ra taglist <<< "$tags"
  for t in "${taglist[@]}"; do args+=(--data-urlencode "tag=$t"); done
  if [ -n "$until" ]; then
    args+=(--data-urlencode repeats=1 --data-urlencode "until=$until" --data-urlencode "times=$time")
    IFS=, read -ra daylist <<< "${days:-mon,tue,wed,thu,fri,sat,sun}"
    for d in "${daylist[@]}"; do args+=(--data-urlencode "weekday=$d"); done
  fi

  code=$(curl -sS -o "$body" -w '%{http_code} %{redirect_url}' -b "$jar" "${args[@]}" "$base/new")
  case $code in
    303*) echo "created  ${code#303 }"; created=$((created+1)) ;;
    422*) msg=$(grep -o '<li><a href="#[a-z_0-9]*">[^<]*' "$body" | sed 's/.*">//' | paste -sd ' ')
          echo "skipped  $title — ${msg:-duplicate}"; skipped=$((skipped+1)) ;;
    *)    echo "failed   $title — HTTP ${code%% *}" >&2; failed=$((failed+1)) ;;
  esac
done < <(grep -v '^#' "$file" | tr '\t' '\037')

echo "done: $created created, $skipped skipped, $failed failed"
[ "$failed" = 0 ]

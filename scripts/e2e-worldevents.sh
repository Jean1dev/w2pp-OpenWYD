#!/usr/bin/env bash
# Drive the world-event E2E suite (tmserver/internal/world/worldevents_e2e_test.go)
# against a live stack, including the calendar-gated Tower War.
#
# The tower window is "a weekday, hour 20, minute <= 5, NewbieEventServer on"
# (CWarTower.cpp:203 → handler/towerwar.go). The event reads time.Now(), i.e. the
# CONTAINER's local time, so the gate is reachable without waiting for a real
# Tuesday evening: this script finds a timezone in which it is currently 20:0x
# and boots a second tmserver in it. No production code is time-mocked.
#
# Windows exist roughly twice an hour (at UTC :00-:05 via a whole-hour zone, and
# at UTC :30-:35 via a :30 zone), so the wait is at most ~30 minutes.
#
# Usage:
#   ./scripts/e2e-worldevents.sh              # weather now, tower when the window opens
#   ./scripts/e2e-worldevents.sh weather      # weather only, no waiting
#   ./scripts/e2e-worldevents.sh tower        # tower only
set -euo pipefail

cd "$(dirname "$0")/.."

# Git Bash on Windows rewrites POSIX-looking arguments into Windows paths, which
# turns container-side paths (/src, /Release) into C:\Program Files\Git\... .
# No-op everywhere else.
export MSYS_NO_PATHCONV=1

MODE=${1:-all}
# Compose derives the project name from the directory: lowercased, with the
# characters it rejects (dots included) stripped, dashes kept.
PROJECT=${W2PP_E2E_PROJECT:-$(basename "$PWD" | tr '[:upper:]' '[:lower:]' | tr -cd '[:alnum:]-')}
NETWORK=${W2PP_E2E_NETWORK:-${PROJECT}_default}
GOIMAGE=${W2PP_E2E_GOIMAGE:-golang:1.26-alpine}
TMIMAGE=${W2PP_E2E_TMIMAGE:-${PROJECT}-tmserver}
TOWER_CONTAINER=w2pp-e2e-tower
TOWER_PORT=8281

# Only DST-free zones: a zone that shifts would make the offset table lie.
# "<offset-in-minutes> <zone>", the offset being local = UTC + offset.
ZONES="
-720 Etc/GMT+12
-660 Etc/GMT+11
-600 Etc/GMT+10
-570 Pacific/Marquesas
-540 Etc/GMT+9
-480 Etc/GMT+8
-420 Etc/GMT+7
-360 Etc/GMT+6
-300 Etc/GMT+5
-240 Etc/GMT+4
-180 Etc/GMT+3
-120 Etc/GMT+2
-60 Etc/GMT+1
0 UTC
60 Etc/GMT-1
120 Etc/GMT-2
180 Etc/GMT-3
210 Asia/Tehran
240 Etc/GMT-4
270 Asia/Kabul
300 Etc/GMT-5
330 Asia/Kolkata
345 Asia/Kathmandu
360 Etc/GMT-6
390 Asia/Yangon
420 Etc/GMT-7
480 Etc/GMT-8
525 Australia/Eucla
540 Etc/GMT-9
570 Australia/Darwin
600 Etc/GMT-10
660 Etc/GMT-11
720 Etc/GMT-12
780 Etc/GMT-13
840 Etc/GMT-14
"

# find_window prints "<zone> <weekday>" for a zone where it is now hour 20,
# minute <= 3 (margin: the server needs ~40s to boot before its first minute
# tick) and the weekday is Mon-Fri. Prints nothing when no zone qualifies.
find_window() {
	local utc_h utc_m utc_dow now_min off zone local_min local_h local_dow
	utc_h=$(date -u +%H); utc_m=$(date -u +%M); utc_dow=$(date -u +%u)
	# Strip leading zeros so arithmetic does not read them as octal.
	now_min=$(( 10#$utc_h * 60 + 10#$utc_m ))
	while read -r off zone; do
		[ -n "${off:-}" ] || continue
		local_min=$(( now_min + off ))
		local_dow=$(( 10#$utc_dow ))
		if [ "$local_min" -lt 0 ]; then
			local_min=$(( local_min + 1440 )); local_dow=$(( local_dow - 1 ))
		elif [ "$local_min" -ge 1440 ]; then
			local_min=$(( local_min - 1440 )); local_dow=$(( local_dow + 1 ))
		fi
		[ "$local_dow" -lt 1 ] && local_dow=7
		[ "$local_dow" -gt 7 ] && local_dow=1
		# Saturday/Sunday are excluded by the event itself.
		[ "$local_dow" -ge 6 ] && continue
		local_h=$(( local_min / 60 ))
		[ "$local_h" -eq 20 ] || continue
		[ $(( local_min % 60 )) -le 3 ] || continue
		echo "$zone"
		return 0
	done <<-EOF
	$ZONES
	EOF
	return 1
}

run_go_test() {
	local pattern=$1; shift
	docker run --rm \
		-v "$PWD":/src -v w2pp-gocache:/go/pkg/mod -w /src \
		--network "$NETWORK" \
		-e W2PP_E2E_ADDR="${W2PP_E2E_ADDR:-tmserver:8281}" \
		-e W2PP_E2E_ACCOUNT="${W2PP_E2E_ACCOUNT:-test}" \
		-e W2PP_E2E_PASSWORD="${W2PP_E2E_PASSWORD:-test123}" \
		-e W2PP_E2E_ACCOUNT2="${W2PP_E2E_ACCOUNT2:-}" \
		-e W2PP_E2E_PASSWORD2="${W2PP_E2E_PASSWORD2:-test123}" \
		-e W2PP_E2E_VERSION="${W2PP_E2E_VERSION:-12000}" \
		-e W2PP_E2E_TOWER_ADDR="${W2PP_E2E_TOWER_ADDR:-}" \
		-e W2PP_E2E_NEWBIE_EXPECT="${W2PP_E2E_NEWBIE_EXPECT:-}" \
		-e W2PP_E2E_SUMMON_TEMPLATE="${W2PP_E2E_SUMMON_TEMPLATE:-}" \
		"$GOIMAGE" \
		go test -tags=e2e -count=1 -timeout 20m -run "$pattern" ./tmserver/internal/world/ -v "$@"
}

# "probe" answers "is a tower window open right now?" without touching Docker —
# useful when deciding whether the ~30 minute wait below is worth starting.
if [ "$MODE" = "probe" ]; then
	date -u
	if zone=$(find_window); then
		echo "window OPEN in ${zone}"
	else
		echo "no window right now"
	fi
	exit 0
fi

docker volume create w2pp-gocache >/dev/null

if [ "$MODE" = "all" ] || [ "$MODE" = "weather" ]; then
	echo "==> Weather / login-snapshot suite"
	run_go_test 'TestE2EWorldEventWeather'
fi

if [ "$MODE" = "all" ] || [ "$MODE" = "newbie" ]; then
	echo "==> Newbie event (mob spawn handicap), driven through the portal config"
	# The DB row wins over the -newbie-event boot flag (ApplyWorldEventConfigBoot →
	# setNewbieEvent), so the flag is toggled here. The version bump is what makes
	# the RUNNING tmserver reload: pollWorldEventConfig compares versions.
	set_newbie() {
		docker compose exec -T db psql -U postgres -q \
			-c "UPDATE world_event_config SET newbie_event_enabled = $1 WHERE id = TRUE" \
			-c "UPDATE world_event_meta SET version = version + 1 WHERE id = TRUE" >/dev/null
		echo "    newbie_event_enabled := $1 (waiting for the 15-tick config poll)"
		sleep 25
	}
	set_newbie FALSE
	export W2PP_E2E_NEWBIE_EXPECT=full
	run_go_test 'TestE2EWorldEventNewbieMobHandicap'
	set_newbie TRUE
	export W2PP_E2E_NEWBIE_EXPECT=handicap
	run_go_test 'TestE2EWorldEventNewbieMobHandicap'
	unset W2PP_E2E_NEWBIE_EXPECT
fi

if [ "$MODE" = "all" ] || [ "$MODE" = "tower" ]; then
	echo "==> Waiting for a tower window (weekday 20:00-20:03 in some zone)…"
	ZONE=""
	while [ -z "$ZONE" ]; do
		if ZONE=$(find_window); then
			break
		fi
		ZONE=""
		echo "    $(date -u +%H:%M:%SZ) — no zone in the window, retrying in 20s"
		sleep 20
	done
	echo "==> Window open in ${ZONE}; enabling NewbieEventServer and booting ${TOWER_CONTAINER}"

	# ApplyWorldEventConfigBoot makes the portal row authoritative over the CLI
	# flag. Keep it enabled for the whole run because the live-config poll would
	# otherwise turn the tower gate off again before minute 6.
	ORIGINAL_NEWBIE=$(docker compose exec -T db psql -U postgres -Atq \
		-c "SELECT newbie_event_enabled FROM world_event_config WHERE id = TRUE")
	docker compose exec -T db psql -U postgres -q \
		-c "UPDATE world_event_config SET newbie_event_enabled = TRUE WHERE id = TRUE" \
		-c "UPDATE world_event_meta SET version = version + 1 WHERE id = TRUE" >/dev/null

	cleanup_tower() {
		docker rm -f "$TOWER_CONTAINER" >/dev/null 2>&1 || true
		if [ -n "${ORIGINAL_NEWBIE:-}" ]; then
			docker compose exec -T db psql -U postgres -q \
				-c "UPDATE world_event_config SET newbie_event_enabled = ${ORIGINAL_NEWBIE} WHERE id = TRUE" \
				-c "UPDATE world_event_meta SET version = version + 1 WHERE id = TRUE" >/dev/null || true
		fi
	}
	trap cleanup_tower EXIT

	docker rm -f "$TOWER_CONTAINER" >/dev/null 2>&1 || true
	docker run -d --name "$TOWER_CONTAINER" --network "$NETWORK" \
		-e TZ="$ZONE" \
		-v "$PWD/Release":/Release:ro \
		"$TMIMAGE" \
		-addr ":${TOWER_PORT}" -dbserver dbserver:7514 -binserver binserver:3000 \
		-content /Release -status-addr :80 -client-version 12000 -newbie-event >/dev/null

	echo "==> Waiting for the tower tmserver to accept connections…"
	for _ in $(seq 1 60); do
		if docker logs "$TOWER_CONTAINER" 2>&1 | grep -q "world loop started"; then break; fi
		sleep 2
	done
	docker logs "$TOWER_CONTAINER" 2>&1 | tail -3

	W2PP_E2E_TOWER_ADDR="${TOWER_CONTAINER}:${TOWER_PORT}" run_go_test 'TestE2EWorldEventTowerWar' || TOWER_FAILED=1
	echo "==> tower tmserver log (event lines):"
	docker logs "$TOWER_CONTAINER" 2>&1 | grep -iE "torre|tower|weather changed" || echo "    (none)"
	[ -z "${TOWER_FAILED:-}" ] || exit 1
fi

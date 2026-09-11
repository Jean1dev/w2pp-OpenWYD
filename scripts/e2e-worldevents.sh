#!/usr/bin/env bash
# Drive the world-event E2E suite (tmserver/internal/world/worldevents_e2e_test.go)
# against a live stack.
#
# The Tower War runs daily at the panel's hour, but testing it no longer waits
# for that hour: the suite forces it with /gm guerra torre (aviso/abrir/fim), so
# the tower case runs at any time against the normal tmserver, in ~2 minutes.
# W2PP_E2E_ACCOUNT needs a moderator/admin role; W2PP_E2E_ACCOUNT2, when set, is
# the non-GM player who must hear every notice.
#
# Usage:
#   ./scripts/e2e-worldevents.sh              # everything
#   ./scripts/e2e-worldevents.sh weather      # weather only
#   ./scripts/e2e-worldevents.sh newbie       # newbie mob handicap only
#   ./scripts/e2e-worldevents.sh tower        # tower war only
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
		-e W2PP_E2E_NEWBIE_EXPECT="${W2PP_E2E_NEWBIE_EXPECT:-}" \
		-e W2PP_E2E_SUMMON_TEMPLATE="${W2PP_E2E_SUMMON_TEMPLATE:-}" \
		"$GOIMAGE" \
		go test -tags=e2e -count=1 -timeout 20m -run "$pattern" ./tmserver/internal/world/ -v "$@"
}

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
	echo "==> Tower war, forced through /gm guerra torre (~2 minutes)"
	run_go_test 'TestE2EWorldEventTowerWar'
fi

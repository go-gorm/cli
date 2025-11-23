#!/usr/bin/env bash
set -euo pipefail

cd "${BASH_SOURCE[0]%/*}/.."

if [ -z "${GOCACHE:-}" ]; then
  export GOCACHE="$PWD/.gocache"
fi
mkdir -p "$GOCACHE"

echo "Running root module tests..." >&2
go test -count=1 ./...

echo "Running examples module tests..." >&2
while IFS= read -r go_mod; do
  example_dir=$(dirname "$go_mod")
  echo "Testing $example_dir..." >&2
  (
    cd "$example_dir"
    go test -count=1 -tags json1 ./...
  )
done < <(find examples -name go.mod -print | sort)

run_migrate_example_tests() {
  local dir="$1"
  echo "Running migrate tests for $dir..." >&2
  (
    cd "$dir"
    go test -count=1 ./...
  )
}

for project in examples/migrate/sqlite examples/migrate/mysql examples/migrate/postgres; do
  run_migrate_example_tests "$project"
done

echo "Done." >&2

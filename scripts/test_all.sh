#!/usr/bin/env bash
set -euo pipefail

cd "${BASH_SOURCE[0]%/*}/.."

if [ -z "${GOCACHE:-}" ]; then
  export GOCACHE="$PWD/.gocache"
fi
mkdir -p "$GOCACHE"

export MYSQL_DSN='gorm:gorm@tcp(127.0.0.1:9910)/gorm?parseTime=true&charset=utf8mb4&loc=Local'
export POSTGRES_DSN='user=gorm password=gorm dbname=gorm host=localhost port=9920 sslmode=disable TimeZone=Asia/Shanghai'

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

reflect_example_project() {
  local dir="$1"
  echo "Running migrate reflect for $dir..." >&2
  go run . migrate reflect --migrations "$dir/migrations" --yes >/dev/null
  if ! git diff --quiet -- "$dir/models"; then
    echo "Model drift detected in $dir/models" >&2
    git --no-pager diff -- "$dir/models" >&2
    exit 1
  fi
}

for project in examples/migrate/sqlite examples/migrate/mysql examples/migrate/postgres; do
  reflect_example_project "$project"
done

echo "Done." >&2

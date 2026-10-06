#!/usr/bin/env bash
set -euo pipefail

cd "$(dirname "${BASH_SOURCE[0]}")/.."
bazel test //proto/... "$@"
bazel build //proto/... "$@"

output="$(bazel info bazel-bin "$@")/proto/usermanagement/v1"
mkdir -p pkg/proto specs
while IFS= read -r -d '' file; do
  relative="${file#"$output/gofile/"}"
  mkdir -p "pkg/proto/$(dirname "$relative")"
  cp --remove-destination "$file" "pkg/proto/$relative"
done < <(find "$output/gofile" -type f -name '*.pb.go' -print0)
find "$output/openapiv2" -type f -name '*.swagger.json' -exec cp --remove-destination {} specs/ \;
find pkg/proto -type f -name '*.go' -exec chmod u+w {} +
find specs -type f -name '*.swagger.json' -exec chmod u+w {} +
gofmt -w pkg/proto

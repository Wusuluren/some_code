#!/bin/sh
set -euo pipefail
set -x

VERSION=${1#"v"}
if [ -z "$VERSION" ]; then
  echo "Must specify version!"
  exit 1
fi
#curl -sS https://raw.githubusercontent.com/kubernetes/kubernetes/v${VERSION}/go.mod |
MODS=($(
  curl -sS https://gitee.com/mirrors/kubernetes/raw/v${VERSION}/go.mod |
  sed -n 's|.*k8s.io/\(.*\) => ./staging/src/k8s.io/.*|k8s.io/\1|p'
))
for MOD in "${MODS[@]}"; do
  V=$(
      go mod download -json "${MOD}@kubernetes-${VERSION}" |
      sed -n 's|.*"Version": "\(.*\)".*|\1|p'
  )
  go mod edit "-replace=${MOD}=${MOD}@${V}"
done
go get "k8s.io/kubernetes@v${VERSION}"
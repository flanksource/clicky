#!/usr/bin/env bash
# Pins and tags clicky's nested Go modules for a release.
#
# Every nested module whose go.mod requires the root module is pinned to the
# release version. The ones outside examples/ are published by tagging
# `<dir>/v<version>` on the release commit; downstream consumers ignore the
# modules' `replace github.com/flanksource/clicky => ../`, so that pin is what
# they build against.
#
#   submodules.sh pin  <version>   rewrite the pins in the working tree, then prove
#                                  each pinned module is still tidy
#                                  (semantic-release prepare step)
#   submodules.sh tags <version>   print the tags to publish, failing unless each
#                                  published module's pin is already v<version>
#                                  (release-submodules job, on the release tag)
set -euo pipefail

usage="usage: submodules.sh pin|tags <version>"
cmd=${1:?$usage}
if [[ ! ${2:?$usage} =~ ^[0-9]+\.[0-9]+\.[0-9]+ ]]; then
  echo "submodules.sh: version must be bare semver (e.g. 1.2.3), got '$2'" >&2
  exit 2
fi
version=v$2
root=github.com/flanksource/clicky

top=$(git rev-parse --show-toplevel)
cd "$top"

# Load every nested go.mod up front: a malformed file must abort the release,
# not read as "does not require clicky" inside a conditional.
files=$(git ls-files -- '*/go.mod')
if [ -z "$files" ]; then
  echo "::error::no nested go.mod is tracked; nothing to pin" >&2
  exit 1
fi
dirs=()
jsons=()
for f in $files; do
  json=$(go -C "${f%/go.mod}" mod edit -json)
  dirs+=("${f%/go.mod}")
  jsons+=("$json")
done

requires() { printf '%s' "$1" | jq -e --arg p "$2" 'any(.Require[]?; .Path == $p)' >/dev/null; }

pinned=()
published=()
for i in "${!dirs[@]}"; do
  d=${dirs[$i]}
  requires "${jsons[$i]}" "$root" || continue
  pinned+=("$i")
  # Example modules take the pin (an example that directory-replaces a published
  # module inherits its clicky requirement, so leaving the example behind makes
  # its go.mod untidy) but are never published, so they get no tag.
  case $d in examples | examples/*) continue ;; esac
  path=$(printf '%s' "${jsons[$i]}" | jq -r '.Module.Path')
  if [ "$path" != "$root/$d" ]; then
    echo "::error::$d/go.mod declares module $path, but a $d/$version tag only publishes $root/$d" >&2
    exit 1
  fi
  published+=("$i")
done
if [ ${#published[@]} -eq 0 ]; then
  echo "::error::no nested module outside examples/ requires $root; nothing to publish" >&2
  exit 1
fi

pin() {
  local targets=("$root") i j p
  for j in "${published[@]}"; do targets+=("$root/${dirs[$j]}"); done
  for i in "${pinned[@]}"; do
    for p in "${targets[@]}"; do
      if requires "${jsons[$i]}" "$p"; then
        go -C "${dirs[$i]}" mod edit -require="$p@$version"
      fi
    done
  done
  # The release commit is [skip ci], so prove here that the pins left every
  # module tidy instead of failing the next unrelated PR.
  for i in "${pinned[@]}"; do
    echo "go mod tidy -diff: ${dirs[$i]}"
    GOWORK=off go -C "${dirs[$i]}" mod tidy -diff
  done
}

tags() {
  local i got
  for i in "${published[@]}"; do
    got=$(printf '%s' "${jsons[$i]}" | jq -r --arg p "$root" '.Require[] | select(.Path == $p) | .Version')
    if [ "$got" != "$version" ]; then
      echo "::error::${dirs[$i]}/go.mod requires $root $got, expected $version (the release commit must carry the pin from 'submodules.sh pin')" >&2
      exit 1
    fi
    echo "${dirs[$i]}/$version"
  done
}

case $cmd in
  pin) pin ;;
  tags) tags ;;
  *)
    echo "$usage" >&2
    exit 2
    ;;
esac

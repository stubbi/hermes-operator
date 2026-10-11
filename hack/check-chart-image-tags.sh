#!/usr/bin/env bash
# Assert that every image reference the chart renders at its own defaults, and
# every operator image reference in the OLM bundle CSV, is well formed, tracks
# the current appVersion, and is actually pullable from the registry.
#
# Regression guard for #113, which had two distinct failure modes:
#
#   1. release-please wrote a bare version into values.yaml `image.tag` while
#      the release workflow only publishes v-prefixed tags, so a default
#      install rendered ghcr.io/...:0.1.18 and hit ImagePullBackOff.
#   2. After (1) was fixed by falling back to the chart's appVersion, the
#      release job packaged the chart with a v-prefixed --app-version while the
#      template added its own "v", shipping ghcr.io/...:vv0.1.19.
#
# (2) is why this script checks the *packaged* chart as well as the working
# tree: they can render different tags, and only the packaged one is published.
#
# Uses the anonymous registry token + manifest HEAD, so it needs no credentials
# for public images.
set -euo pipefail

CHART_DIR="${CHART_DIR:-charts/hermes-operator}"
CHART_NAME="$(basename "${CHART_DIR}")"
# Same variable and default as the release workflows (#173).
OPERATOR_REPO="${IMAGE_NAMESPACE:-paperclipinc}/hermes-operator"

# Bare version, however Chart.yaml happens to spell it.
APP_VERSION="$(sed -nE 's/^appVersion:[[:space:]]*"?v?([^"[:space:]]+)"?/\1/p' "${CHART_DIR}/Chart.yaml")"

if [[ -z "$APP_VERSION" ]]; then
  echo "ERROR: could not read appVersion from ${CHART_DIR}/Chart.yaml" >&2
  exit 1
fi

fail=0

# Resolve a ghcr.io reference to a manifest, following the OCI auth dance.
check_ref() {
  local ref="$1"
  local repo tag host path token code

  host="${ref%%/*}"
  path="${ref#*/}"
  repo="${path%:*}"
  tag="${path##*:}"

  if [[ "$host" != "ghcr.io" ]]; then
    echo "  SKIP $ref (only ghcr.io is checked)"
    return 0
  fi

  # Shape check first, and strictly. The operator image is published as exactly
  # v<semver>. Anything else is a rendering bug, and checking the shape rather
  # than only registry existence means it fails even for a version that has not
  # been published yet, which is when these bugs are actually introduced.
  # Catches both the bare "0.1.18" and the doubled "vv0.1.19".
  if [[ "$repo" == "$OPERATOR_REPO" ]]; then
    if [[ ! "$tag" =~ ^v[0-9]+\.[0-9]+\.[0-9]+(-[0-9A-Za-z.-]+)?$ ]]; then
      echo "  FAIL $ref (tag must be exactly v<semver>; got '${tag}')"
      fail=1
      return 0
    fi
    if [[ "$tag" != "v${APP_VERSION}" ]]; then
      echo "  FAIL $ref (tag '${tag}' does not match appVersion v${APP_VERSION})"
      fail=1
      return 0
    fi
    # The release job publishes the image *after* this runs on the release
    # commit, so the pending version legitimately does not exist yet. The shape
    # and appVersion checks above already passed, which is the part that
    # actually regresses.
    if ! git rev-parse -q --verify "refs/tags/v${APP_VERSION}" >/dev/null 2>&1; then
      echo "  SKIP $ref (v${APP_VERSION} is the pending release; shape verified)"
      return 0
    fi
  fi

  token=$(curl -fsSL "https://ghcr.io/token?scope=repository:${repo}:pull&service=ghcr.io" \
    | python3 -c 'import sys,json; print(json.load(sys.stdin)["token"])')

  code=$(curl -s -o /dev/null -w '%{http_code}' -I \
    -H "Authorization: Bearer ${token}" \
    -H 'Accept: application/vnd.oci.image.index.v1+json' \
    -H 'Accept: application/vnd.oci.image.manifest.v1+json' \
    -H 'Accept: application/vnd.docker.distribution.manifest.list.v2+json' \
    -H 'Accept: application/vnd.docker.distribution.manifest.v2+json' \
    "https://ghcr.io/v2/${repo}/manifests/${tag}")

  if [[ "$code" == "200" ]]; then
    echo "  OK   $ref"
  else
    echo "  FAIL $ref (HTTP $code, tag does not resolve)"
    fail=1
  fi
}

render_refs() {
  helm template hermes-operator "$1" \
    | grep -oE 'image: "?[a-z0-9./-]+:[A-Za-z0-9._-]+"?' \
    | sed -E 's/^image: "?//; s/"?$//' \
    | sort -u
}

check_all() {
  local label="$1" chart="$2" refs
  refs="$(render_refs "$chart")"
  if [[ -z "$refs" ]]; then
    echo "ERROR: no image references found in ${label}. Check the grep." >&2
    exit 1
  fi
  echo "Checking ${label}:"
  while IFS= read -r ref; do
    [[ -n "$ref" ]] && check_ref "$ref"
  done <<<"$refs"
}

check_all "working-tree chart" "${CHART_DIR}"

# The release job repackages with --version/--app-version derived from the git
# tag, so the packaged chart can render a different tag than the working tree.
# Check both spellings of appVersion, since only the packaged form is published.
echo
for av in "${APP_VERSION}" "v${APP_VERSION}"; do
  pkgdir="$(mktemp -d)"
  helm package "${CHART_DIR}" --version "${APP_VERSION}" --app-version "${av}" -d "$pkgdir" >/dev/null
  pkg="${pkgdir}/${CHART_NAME}-${APP_VERSION}.tgz"
  if [[ ! -f "$pkg" ]]; then
    echo "ERROR: expected packaged chart at $pkg" >&2
    rm -rf "$pkgdir"
    exit 1
  fi
  check_all "packaged chart (--app-version ${av})" "$pkg"
  rm -rf "$pkgdir"
done

# ---------------------------------------------------------------------------
# OLM bundle CSV (#129).
#
# The CSV carries the operator image in three places plus the CSV name, and
# release-please historically bumped only $.spec.version, so all four drifted
# years behind (containerImage sat at v0.1.10, the deployment and relatedImages
# entries at v0.1.0 while the release was v0.1.18). `make bundle-build` and
# `catalog-build` publish the in-tree CSV verbatim, so the drift shipped.
#
# They are now kept current by release-please's *generic* updater, driven by an
# inline `# x-release-please-version` marker. The generic updater rewrites the
# semver inside the line and leaves the rest alone, which is what preserves the
# leading "v". The yaml/jsonpath updater must NOT be pointed at this file at
# all, for two independent reasons:
#
#   1. It replaces the whole value with a bare version, which breaks the pull
#      (short-name resolution in the OperatorHub kiwi test) and stops
#      operatorhub-submit.yaml's `:v[0-9]+\.[0-9]+\.[0-9]+` sed from matching.
#   2. It round-trips the document through a YAML parse and re-emit, which
#      DROPS the `# x-release-please-version` comments. Observed on release PR
#      #152: the yaml updater bumped $.spec.version, stripped every marker, and
#      the generic updater then found nothing to bump, leaving all four image
#      references frozen at the previous release. `spec.version` therefore
#      carries a marker too and is bumped generically like the rest.
#
# So this checks: the marker is present on every versioned line (otherwise the
# next release silently leaves it behind), the value matches appVersion, the
# tag resolves, and no yaml updater is aimed at this file.
CSV="bundle/manifests/hermes-operator.clusterserviceversion.yaml"

echo
echo "Checking OLM bundle CSV (${CSV}):"

if [[ ! -f "$CSV" ]]; then
  echo "ERROR: ${CSV} not found." >&2
  exit 1
fi

# Every line naming the operator image, plus the CSV name line.
csv_lines="$(grep -nE "ghcr\.io/paperclipinc/hermes-operator:|^  name: hermes-operator\.|^  version: [0-9]" "$CSV" || true)"

if [[ -z "$csv_lines" ]]; then
  echo "ERROR: no operator image or CSV name lines found in ${CSV}. Check the grep." >&2
  exit 1
fi

expected_refs=0
while IFS= read -r line; do
  lineno="${line%%:*}"
  body="${line#*:}"

  if [[ "$body" != *"x-release-please-version"* ]]; then
    echo "  FAIL ${CSV}:${lineno} is missing the '# x-release-please-version' marker"
    echo "       (${body#"${body%%[![:space:]]*}"})"
    fail=1
    continue
  fi

  if [[ "$body" =~ ^[[:space:]]*version:[[:space:]]*([^[:space:]#]+) ]]; then
    got="${BASH_REMATCH[1]}"
    if [[ "$got" != "${APP_VERSION}" ]]; then
      echo "  FAIL ${CSV}:${lineno} spec.version is ${got}, expected ${APP_VERSION}"
      fail=1
    else
      echo "  OK   ${CSV}:${lineno} spec.version ${got}"
    fi
    continue
  fi

  if [[ "$body" =~ ^[[:space:]]*name:[[:space:]]*hermes-operator\.(v[^[:space:]#]+) ]]; then
    got="${BASH_REMATCH[1]}"
    if [[ "$got" != "v${APP_VERSION}" ]]; then
      echo "  FAIL ${CSV}:${lineno} CSV name is hermes-operator.${got}, expected hermes-operator.v${APP_VERSION}"
      fail=1
    else
      echo "  OK   ${CSV}:${lineno} name hermes-operator.${got}"
    fi
    continue
  fi

  ref="$(sed -E 's/.*(ghcr\.io\/paperclipinc\/hermes-operator:[^[:space:]#]+).*/\1/' <<<"$body")"
  if [[ "$ref" == "$body" ]]; then
    echo "  FAIL ${CSV}:${lineno} could not parse an image reference"
    fail=1
    continue
  fi
  expected_refs=$((expected_refs + 1))
  check_ref "$ref"
done <<<"$csv_lines"

# The three references are containerImage, relatedImages and the manager
# deployment. Fewer means one was renamed or dropped and is no longer guarded.
if [[ "$expected_refs" -lt 3 ]]; then
  echo "  FAIL expected at least 3 operator image references in the CSV, found ${expected_refs}"
  fail=1
fi

# operatorhub-submit.yaml rewrites the CSV with these two patterns. If the
# in-tree spelling ever stops matching them, the submitted bundle silently
# keeps the in-tree version instead of the released one.
for pat in "hermes-operator\.v[0-9]\+\.[0-9]\+\.[0-9]\+" "ghcr.io/[a-z0-9._-]*/hermes-operator:v[0-9]\+\.[0-9]\+\.[0-9]\+"; do
  if ! grep -q "$pat" "$CSV"; then
    echo "  FAIL no line in ${CSV} matches the operatorhub-submit sed pattern: ${pat}"
    fail=1
  else
    echo "  OK   operatorhub-submit sed pattern matches: ${pat}"
  fi
done

# A yaml/jsonpath updater aimed at the CSV silently strips the markers above on
# the next release PR, so assert release-please is not configured with one.
if python3 hack/check-release-please-csv-updater.py release-please-config.json "$CSV"; then
  echo "  OK   release-please uses only the generic updater for the CSV"
else
  fail=1
fi

if [[ "$fail" -ne 0 ]]; then
  echo
  echo "One or more default image tags are wrong." >&2
  echo "The release workflow publishes exactly v<version>, so the chart must" >&2
  echo "render that, in the working tree AND when packaged, and the OLM bundle" >&2
  echo "CSV must carry the same. See the header." >&2
  exit 1
fi

echo
echo "All chart and bundle default image tags resolve."

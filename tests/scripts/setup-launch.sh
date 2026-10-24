#!/bin/sh
# Integration test: uses real Go/npm/build tools, reusing already downloaded caches.
set -eu
root_dir=$(CDPATH= cd -- "$(dirname -- "$0")/../.." && pwd -P)
test_dir=$(mktemp -d "$root_dir/tmp/setup-test.XXXXXX")
cleanup() {
  status=$?
  if [ "$status" -ne 0 ] && [ -f "$test_dir/output.log" ]; then
    cat "$test_dir/output.log" >&2
  fi
  rm -rf "$test_dir"
}
trap cleanup EXIT
trap 'exit 1' HUP INT TERM
fixture_dir="$test_dir/project with spaces"
mkdir -p "$fixture_dir/tmp" "$fixture_dir/backends"
cp -R "$root_dir/bin" "$root_dir/scripts" "$root_dir/web" "$root_dir/media" "$fixture_dir/"
cp -R "$root_dir/backends/go" "$fixture_dir/backends/"
for cache in downloads go-mod-cache go-build-cache; do
  ln -s "$root_dir/tmp/$cache" "$fixture_dir/tmp/$cache"
done
cd "$test_dir"
target=$(go env GOHOSTOS GOHOSTARCH | tr '\n' '-' | sed 's/-$//')
setup() { "$fixture_dir/bin/setup" > "$test_dir/output.log" 2>&1; }
expect_output() {
  if ! grep -F "$1" "$test_dir/output.log" >/dev/null; then
    cat "$test_dir/output.log" >&2
    printf 'Missing expected output: %s\n' "$1" >&2
    exit 1
  fi
}
expect_no_work() {
  expect_output 'Everything is up to date.'
  if grep -E 'Installing|Building|Packaging' "$test_dir/output.log"; then
    printf '%s\n' 'Unchanged setup performed work.' >&2
    exit 1
  fi
}
setup
expect_output 'Building frontend'
expect_output 'Building Go backend'
expect_output 'Packaging MediaMTX'
tls_dir="$fixture_dir/tmp/bigbrother-local/tls"
if [ ! -f "$tls_dir/server.crt" ] || [ ! -f "$tls_dir/server.key" ]; then
  printf '%s\n' 'Setup did not generate localhost TLS certificates.' >&2
  exit 1
fi
openssl verify -CAfile "$tls_dir/ca.crt" "$tls_dir/server.crt" >/dev/null
openssl x509 -in "$tls_dir/server.crt" -noout -checkhost localhost >/dev/null
openssl x509 -in "$tls_dir/server.crt" -noout -checkip 127.0.0.1 >/dev/null
tls_before=$(cksum "$tls_dir/ca.crt" "$tls_dir/ca.key" "$tls_dir/server.crt" "$tls_dir/server.key")
setup
expect_no_work
[ "$tls_before" = "$(cksum "$tls_dir/ca.crt" "$tls_dir/ca.key" "$tls_dir/server.crt" "$tls_dir/server.key")" ]

# Preserve incomplete server pairs rather than silently replacing private keys.
rm "$tls_dir/server.crt"
if setup; then printf '%s\n' 'Incomplete TLS pair was silently replaced.' >&2; exit 1; fi
ca_before=$(cksum "$tls_dir/ca.crt" "$tls_dir/ca.key")
rm "$tls_dir/server.key"
setup
openssl verify -CAfile "$tls_dir/ca.crt" "$tls_dir/server.crt" >/dev/null
[ "$ca_before" = "$(cksum "$tls_dir/ca.crt" "$tls_dir/ca.key")" ]

printf '\n.setup-integration-test { color: red; }\n' >> "$fixture_dir/web/src/styles.css"
setup
expect_output 'Building frontend'
! grep -E 'Installing|Building Go|Packaging' "$test_dir/output.log"
grep -F '.setup-integration-test' "$fixture_dir/tmp/build/$target/web/app.css" >/dev/null
printf 'package room\n' > "$fixture_dir/backends/go/internal/room/setup_fixture.go"
setup
expect_output 'Building Go backend'
rm "$fixture_dir/backends/go/internal/room/setup_fixture.go"
setup
expect_output 'Building Go backend'

printf '\n' >> "$fixture_dir/web/package.json"
setup
expect_output 'Installing frontend dependencies'
expect_output 'Building frontend'
rm "$fixture_dir/web/node_modules/.bin/tsc"
setup
expect_output 'Installing frontend dependencies'
rm "$fixture_dir/web/dist/app.css"
setup
expect_output 'Building frontend'
rm "$fixture_dir/tmp/build/$target/bigbrother"
setup
expect_output 'Building Go backend'
rm "$fixture_dir/tmp/build/$target/media/mediamtx"
setup
expect_output 'Packaging MediaMTX'

printf 'invalid go source\n' > "$fixture_dir/backends/go/internal/room/setup_fixture.go"
if setup; then printf '%s\n' 'Invalid Go source unexpectedly built.' >&2; exit 1; fi
if setup; then printf '%s\n' 'Failed build was incorrectly cached.' >&2; exit 1; fi
expect_output 'Building Go backend'
rm "$fixture_dir/backends/go/internal/room/setup_fixture.go"
setup
expect_output 'Building Go backend'
setup
expect_no_work

printf 'const setupInvalid: string = 42;\n' > "$fixture_dir/web/src/setup_fixture.ts"
if setup; then printf '%s\n' 'Invalid TypeScript unexpectedly built.' >&2; exit 1; fi
if setup; then printf '%s\n' 'Failed frontend build was incorrectly cached.' >&2; exit 1; fi
rm "$fixture_dir/web/src/setup_fixture.ts"
setup
expect_output 'Building frontend'
setup
expect_no_work

"$fixture_dir/bin/launch" --help > "$test_dir/launch.log" 2>&1
grep -F -- '-config-dir' "$test_dir/launch.log" >/dev/null
grep -F -- '-port' "$test_dir/launch.log" >/dev/null
if "$fixture_dir/bin/launch" --config-dir "$test_dir/explicit config" > "$test_dir/launch.log" 2>&1; then
  printf '%s\n' 'Launch ignored the explicit configuration directory.' >&2
  exit 1
fi
grep -F "$test_dir/explicit config/tls/server.crt" "$test_dir/launch.log" >/dev/null
if "$fixture_dir/bin/launch" --tls-cert "$test_dir/explicit.crt" --tls-key "$test_dir/explicit.key" > "$test_dir/launch.log" 2>&1; then
  printf '%s\n' 'Launch ignored explicit TLS paths.' >&2
  exit 1
fi
grep -F "$test_dir/explicit.crt" "$test_dir/launch.log" >/dev/null
printf '%s\n' 'PASS: setup caching, localhost TLS creation/reuse/recovery, failure recovery, paths with spaces, and launch overrides.'

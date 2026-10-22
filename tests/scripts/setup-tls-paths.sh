#!/bin/sh
set -eu
root_dir=$(CDPATH= cd -- "$(dirname -- "$0")/../.." && pwd -P)
test_dir=$(mktemp -d "${TMPDIR:-/tmp}/bigbrother-tls-paths.XXXXXX")
trap 'rm -rf "$test_dir"' EXIT HUP INT TERM
mkdir -p "$test_dir/bin"
cat > "$test_dir/bin/uname" <<'MOCK'
#!/bin/sh
printf '%s\n' "$TLS_TEST_PLATFORM"
MOCK
# Stop at the first filesystem operation: inspect resolution without touching
# the developer's actual configuration directory or generating certificates.
cat > "$test_dir/bin/mkdir" <<'MOCK'
#!/bin/sh
printf '%s\n' "$2"
exit 73
MOCK
chmod +x "$test_dir/bin/uname" "$test_dir/bin/mkdir"
check_path() {
  platform=$1
  expected=$2
  actual=""
  status=0
  actual=$(PATH="$test_dir/bin:$PATH" TLS_TEST_PLATFORM="$platform" XDG_CONFIG_HOME="$test_dir/xdg" sh "$root_dir/scripts/setup-tls" --host localhost) || status=$?
  [ "$status" -eq 73 ] || { printf 'Unexpected setup exit: %s\n' "$status" >&2; exit 1; }
  [ "$actual" = "$expected" ] || { printf '%s selected %s; expected %s\n' "$platform" "$actual" "$expected" >&2; exit 1; }
}
check_path Darwin "$HOME/Library/Application Support/bigbrother/tls"
check_path Linux "$test_dir/xdg/bigbrother/tls"
printf '%s\n' 'PASS: TLS setup defaults match Go configuration directories on macOS and Linux.'

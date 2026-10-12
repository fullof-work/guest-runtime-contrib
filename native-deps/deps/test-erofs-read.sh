#!/usr/bin/env bash
set -euo pipefail
script_dir="$(cd "$(dirname "$0")" && pwd)"
: "${BUILD_DIR:?}" "${BINDIR:?}"
[ -z "${CROSS_PREFIX:-}" ] || { echo 'read regression requires the native architecture' >&2; exit 1; }
mkdir -p "$BUILD_DIR/tests"
work="$(mktemp -d "$BUILD_DIR/tests/exact-read.XXXXXX")"
trap 'rm -rf "$work"' EXIT
# A validated native cache can omit the compilation tree. Reconstruct it with
# the same selected recipe in private test storage when relink inputs are absent.
test_source="$BUILD_DIR/src/erofs-utils"
if [ ! -f "$test_source/lib/.libs/liberofs.a" ] || [ ! -f "$test_source/mkfs/mkfs_erofs-main.o" ]; then
    BUILD_DIR="$work/rebuild" BINDIR="$work/rebuild-bin" bash "$script_dir/build-erofs.sh"
    test_source="$work/rebuild/src/erofs-utils"
fi
# Relink only this disposable copy, retaining production objects and binaries.
cp -a "$test_source" "$work/src"
read -r -a compiler <<< "${CC:-gcc}"
"${compiler[@]}" -Wall -Wextra -Werror -c "$script_dir/test-erofs-read-wrap.c" -o "$work/wrap.o"
rm -f "$work/src/mkfs/mkfs.erofs" "$work/src/mkfs/.libs/mkfs.erofs"
make -C "$work/src/mkfs" LDFLAGS="${LDFLAGS:-} -all-static $work/wrap.o -Wl,--wrap=read -Wl,--wrap=__read_chk -Wl,--wrap=pread -Wl,--wrap=pread64" \
    > "$BUILD_DIR/tests/exact-read-build.log" 2>&1 \
    || { cat "$BUILD_DIR/tests/exact-read-build.log" >&2; exit 1; }
if readelf -l "$work/src/mkfs/mkfs.erofs" | grep -q INTERP; then
    echo 'read regression binary is not static' >&2
    exit 1
fi
python3 "$script_dir/test-erofs-read.py" "$BINDIR" "$work/src/mkfs/mkfs.erofs" "$work"
python3 "$script_dir/test-erofs-plan.py" "$BINDIR" "$work/src/mkfs/mkfs.erofs" "$work"

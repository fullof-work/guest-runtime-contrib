#!/usr/bin/env python3
"""Verify maintained native Plan layout, actual contents and batched read faults.

This test materializes small plans only as an independent native regression
oracle. Production emission and plan-admission tests live in accelerator.
"""
import hashlib
import os
from pathlib import Path
import struct
import subprocess
import sys

tools, probe, work = map(Path, sys.argv[1:])
flags = ["-Ededupe", "--chunksize=4096", "-T0", "-b4096", "-x-1", "-U",
         "00000000-0000-0000-0000-000000000000", "--quiet", "--ignore-mtime"]


def reconstruct(root, layout, plan):
    header = plan.read_bytes()
    assert len(header) == 192 and header[:8] == b"KERPLAN\0"
    assert struct.unpack_from("<IIQ", header, 8) == (1, 192, 1)
    assert not any(header[176:])
    size, start, length, count, source_count, rec_bytes, src_bytes = struct.unpack_from("<7Q", header, 24)
    records = Path(str(plan) + ".chunks").read_bytes()
    sources = Path(str(plan) + ".sources").read_bytes()
    output = bytearray(layout.read_bytes())
    assert len(output) == size and start + length <= size
    assert length == count * 4096
    assert hashlib.sha256(output[:start] + output[start + length:]).digest() == header[80:112]
    assert len(records) == rec_bytes == count * 64 and len(sources) == src_bytes
    assert hashlib.sha256(records).digest() == header[112:144]
    assert hashlib.sha256(sources).digest() == header[144:176]
    paths, pos = {}, 0
    for n in range(source_count):
        ident = struct.unpack_from("<Q", sources, pos)[0]
        plen = struct.unpack_from("<I", sources, pos + 68)[0]
        name = os.fsdecode(sources[pos + 80:pos + 80 + plen])
        assert len(os.fsencode(name)) == plen and not Path(name).is_absolute() and ".." not in Path(name).parts
        assert ident == n + 1
        paths[ident] = root / name
        pos += 80 + plen
    assert pos == len(sources)
    previous = 0
    for n in range(count):
        rec = records[n * 64:(n + 1) * 64]
        dst, src, ident, amount, padding = struct.unpack_from("<QQQII", rec)
        assert dst == n * 4096 and 0 < amount <= 4096 and padding == 4096 - amount
        assert src % 4096 == 0 and ident >= previous
        previous = ident
        with paths[ident].open("rb") as f:
            f.seek(src)
            body = f.read(amount)
        assert len(body) == amount and hashlib.sha256(body).digest() == rec[32:64]
        output[start + dst:start + dst + 4096] = body + bytes(padding)
    return bytes(output)


assert subprocess.check_output([str(tools / "mkfs.erofs"), "--plan-version"], text=True).strip() == "erofs-plan 1 sha256-4k-single-device"
for case in ("empty", "data", "sparse"):
    root = work / ("plan-root-" + case)
    root.mkdir()
    if case != "empty":
        body = b"".join(hashlib.sha256(str(i).encode()).digest() * 128 for i in range(300)) + b"tail"
        (root / "payload").write_bytes(body)
        (root / "duplicate").write_bytes(body)
        os.chmod(root / "payload", 0o640)
        os.link(root / "payload", root / "hardlink")
        (root / "symlink").symlink_to("payload")
        (root / "directory").mkdir()
        (root / "empty").touch()
    if case == "sparse":
        with (root / "sparse").open("wb") as f:
            f.seek(3 << 20)
            f.write(b"end")
    full, layout, plan = [work / ("plan-" + case + ext) for ext in (".raw", ".layout", ".plan")]
    subprocess.run([str(tools / "mkfs.erofs"), *flags, str(full), str(root)], check=True)
    subprocess.run([str(tools / "mkfs.erofs"), *flags, "--plan=" + str(plan), str(layout), str(root)], check=True)
    output = reconstruct(root, layout, plan)
    assert output == full.read_bytes(), case + " changed raw layout"
    assert layout.stat().st_blocks * 512 < (16 << 10), "layout materialized payload"
    planned = work / ("plan-" + case + ".reconstructed")
    planned.write_bytes(output)
    extracted = work / ("plan-extract-" + case)
    subprocess.run([str(tools / "fsck.erofs"), "--extract=" + str(extracted), str(planned)], check=True)
    if case != "empty":
        for name in ("payload", "duplicate", "hardlink"):
            assert (extracted / name).read_bytes() == body
        assert (extracted / "payload").stat().st_mode & 0o7777 == 0o640
        assert (extracted / "payload").stat().st_ino == (extracted / "hardlink").stat().st_ino
        assert (extracted / "symlink").readlink() == Path("payload")
        assert (extracted / "directory").is_dir() and (extracted / "empty").stat().st_size == 0
    if case == "sparse":
        assert (extracted / "sparse").read_bytes() == bytes(3 << 20) + b"end"
    print("test-erofs-plan:", case, "PASS")

root = work / "plan-root-data"
for mode in ("eintr", "short", "eio", "eof", "partial-eio"):
    layout, plan = [work / ("batch-" + mode + ext) for ext in (".layout", ".plan")]
    result = subprocess.run([str(probe), *flags, "--plan=" + str(plan), str(layout), str(root)],
                            env=dict(os.environ, KUASAR_TEST_READ_TARGET=str(root / "payload"), KUASAR_TEST_READ_MODE=mode),
                            capture_output=True, text=True, timeout=30)
    assert "pread-injected:" + mode in result.stderr, (mode, result.stderr)
    if mode in ("eio", "eof", "partial-eio"):
        assert result.returncode != 0, (mode, "accepted incomplete batch", result.stderr)
    else:
        assert result.returncode == 0, (mode, result.stderr)
        assert reconstruct(root, layout, plan) == (work / "plan-data.raw").read_bytes()
    print("test-erofs-plan:", mode, "PASS")

for option in ("-T1", "-x0", "-b8192", "--plan-bytes=192", "--offset=4096", "--plan-memory=18446744073709551615"):
    failed = work / ("reject-" + str(abs(hash(option))))
    result = subprocess.run([str(tools / "mkfs.erofs"), *flags, "--plan=" + str(failed), option,
                             str(failed) + ".layout", str(root)], capture_output=True, timeout=30)
    assert result.returncode != 0, (option, "accepted unsupported plan policy/budget")
print("test-erofs-plan: policy/budget rejection PASS")

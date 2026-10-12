#!/usr/bin/env python3
"""Inject real read outcomes into a separately linked static mkfs test binary."""
import hashlib
import os
from pathlib import Path
import subprocess
import sys

tools, probe, work = map(Path, sys.argv[1:])
source = work / "read-source"
source.mkdir()
target = source / "payload"
body = b"".join(hashlib.sha256(str(i).encode()).digest() * 128 for i in range(19)) + b"tail!"
target.write_bytes(body)
flags = ["-Ededupe", "--chunksize=4096", "-T0", "-b4096", "-x-1", "-U", "00000000-0000-0000-0000-000000000000", "--quiet"]
baseline = work / "read-baseline.erofs"
subprocess.run([str(tools / "mkfs.erofs"), *flags, str(baseline), str(source)], check=True)
for mode in ("eintr", "short", "eio", "eof", "partial-eio"):
    image = work / ("read-" + mode + ".erofs")
    result = subprocess.run([str(probe), *flags, str(image), str(source)],
                            env=dict(os.environ, KUASAR_TEST_READ_TARGET=str(target), KUASAR_TEST_READ_MODE=mode),
                            capture_output=True, text=True, timeout=30)
    assert "read-injected:" + mode in result.stderr, (mode, result.stderr)
    if mode in ("eio", "eof", "partial-eio"):
        assert result.returncode != 0, (mode, "mkfs accepted an incomplete read", result.stderr)
    else:
        assert result.returncode == 0, (mode, result.stderr)
        assert image.read_bytes() == baseline.read_bytes(), mode + " changed image identity"
        extracted = work / ("read-extract-" + mode)
        subprocess.run([str(tools / "fsck.erofs"), "--extract=" + str(extracted), str(image)], check=True)
        assert (extracted / "payload").read_bytes() == body, mode + " changed file content"
    print("test-erofs-read:", mode, "PASS")

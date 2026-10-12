/* SPDX-License-Identifier: Apache-2.0 */
/* Test-only link wrapper. Never linked into the packaged tools. */
#define _GNU_SOURCE
#include <errno.h>
#include <stdio.h>
#include <stdlib.h>
#include <string.h>
#include <sys/stat.h>
#include <unistd.h>

ssize_t __real_read(int fd, void *buf, size_t count);
ssize_t __real___read_chk(int fd, void *buf, size_t count, size_t buflen);
ssize_t __real_pread(int fd, void *buf, size_t count, off_t offset);
ssize_t __real_pread64(int fd, void *buf, size_t count, off64_t offset);

static ssize_t inject_pread(int fd, void *buf, size_t count, off64_t offset,
			    int wide)
{
	const char *target = getenv("KUASAR_TEST_READ_TARGET");
	const char *mode = getenv("KUASAR_TEST_READ_MODE");
	struct stat wanted, current;
	static int injected;

	if (target && mode && count && !stat(target, &wanted) &&
	    !fstat(fd, &current) && wanted.st_dev == current.st_dev &&
	    wanted.st_ino == current.st_ino) {
		if (!strcmp(mode, "eintr") && !injected++) {
			fputs("pread-injected:eintr\n", stderr);
			errno = EINTR;
			return -1;
		}
		if (!strcmp(mode, "short")) {
			if (!injected++)
				fputs("pread-injected:short\n", stderr);
			if (count > 97)
				count = 97;
		}
		if (!strcmp(mode, "eio") || !strcmp(mode, "eof") ||
		    !strcmp(mode, "partial-eio")) {
			/* Force positive progress before a failure, even if the entire
			 * fixture fits inside a single 1 MiB batch. */
			if (offset >= 0 && offset < 4096 && count > (size_t)(4096 - offset))
				count = 4096 - offset;
			else if (offset == 4096 && !strcmp(mode, "partial-eio"))
				count = count > 17 ? 17 : count;
			else if (offset >= 4096) {
				fprintf(stderr, "pread-injected:%s\n", mode);
				errno = EIO;
				return !strcmp(mode, "eof") ? 0 : -1;
			}
		}
	}
	return wide ? __real_pread64(fd, buf, count, offset) :
		__real_pread(fd, buf, count, offset);
}

ssize_t __wrap_pread(int fd, void *buf, size_t count, off_t offset)
{
	return inject_pread(fd, buf, count, offset, 0);
}

ssize_t __wrap_pread64(int fd, void *buf, size_t count, off64_t offset)
{
	return inject_pread(fd, buf, count, offset, 1);
}

ssize_t __wrap_read(int fd, void *buf, size_t count)
{
	const char *target = getenv("KUASAR_TEST_READ_TARGET");
	const char *mode = getenv("KUASAR_TEST_READ_MODE");
	struct stat wanted, current;
	static int injected;
	off_t offset;

	if (!target || !mode || !count || stat(target, &wanted) ||
	    fstat(fd, &current) || wanted.st_dev != current.st_dev ||
	    wanted.st_ino != current.st_ino)
		return __real_read(fd, buf, count);
	offset = lseek(fd, 0, SEEK_CUR);
	if (!strcmp(mode, "eintr") && !injected) {
		injected = 1;
		fputs("read-injected:eintr\n", stderr);
		errno = EINTR;
		return -1;
	}
	if (!strcmp(mode, "short")) {
		if (!injected++)
			fputs("read-injected:short\n", stderr);
		return __real_read(fd, buf, count > 97 ? 97 : count);
	}
	if (offset >= 4096 && (!strcmp(mode, "eio") || !strcmp(mode, "eof"))) {
		fprintf(stderr, "read-injected:%s\n", mode);
		errno = EIO;
		return !strcmp(mode, "eio") ? -1 : 0;
	}
	if (!strcmp(mode, "partial-eio") && offset >= 4096) {
		if (offset == 4096)
			return __real_read(fd, buf, count > 17 ? 17 : count);
		fputs("read-injected:partial-eio\n", stderr);
		errno = EIO;
		return -1;
	}
	return __real_read(fd, buf, count);
}

/* Fortified toolchains route chunk reads through this symbol. Preserve its
 * bounds check, then use the same injection for valid requests. */
ssize_t __wrap___read_chk(int fd, void *buf, size_t count, size_t buflen)
{
	if (count > buflen)
		return __real___read_chk(fd, buf, count, buflen);
	return __wrap_read(fd, buf, count);
}

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

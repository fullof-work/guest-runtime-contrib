[English](LICENSE_SCOPE.md) | [简体中文](LICENSE_SCOPE_zh.md)

# 许可证范围

除下述文件外,本仓库中未另行声明许可证的项目原创内容适用根目录的 Apache License 2.0.

## Linux 内核 patch

`native-deps/deps/linux-patches/0001-virtio_balloon-converge-to-a-sustainable-size-under-.patch` 是针对 Linux 6.1.169 内核源码的修改,在本仓库中按 GPL-2.0-only 标识.相邻的 `.license` 文件提供机器可读映射,许可证全文见 [`LICENSES/GPL-2.0-only.txt`](LICENSES/GPL-2.0-only.txt).

用于获取和构建内核的项目脚本及项目维护的配置片段适用根目录的 Apache-2.0.构建时下载且未跟踪在本仓库中的 Linux 源码继续适用其上游许可证.

## EROFS 补丁

`native-deps/deps/erofs-patches/0002-explicit-libgcrypt-sha256.patch` 保留目标文件的许可证：`lib/sha256.h` 为 `GPL-2.0-or-later OR Apache-2.0`，`lib/sha256.c` 为 `Unlicense`。相邻 `.license` 提供组合后的机器可读表达式。许可证全文见根目录的 [Apache 许可证](LICENSE)、[GPL 第 2 版文本](LICENSES/GPL-2.0-only.txt)（该头文件的源码声明允许后续版本）和 [Unlicense](LICENSES/Unlicense.txt)。其他上游文件声明保持不变，包括 GPL-2.0 的 hashmap 代码。[补丁来源说明](native-deps/deps/erofs-patches/README_zh.md) 描述库声明和源码/重新链接输入。

`native-deps/deps/erofs-patches/0003-exact-chunk-read.patch` 保留 `lib/blobchunk.c` 的 `GPL-2.0-or-later OR Apache-2.0` 声明。相邻 `.license` 提供机器可读映射；适用上文链接的 Apache 和 GPL 文本，源码声明允许后续 GPL 版本。项目原创的读取故障注入测试 helper 适用根目录 Apache-2.0 许可证，不链接进发布工具。

`0004-single-device-plan.patch` 修改 `mkfs/main.c`、库构建清单、`lib/blobchunk.c` 和 `lib/inode.c`，并增加 Plan 接口与实现。其 sidecar 标记为 `GPL-2.0-or-later AND (GPL-2.0-or-later OR Apache-2.0)`，保留受影响上游文件的声明。计划重建与批量读取故障注入测试使用根目录 Apache-2.0 许可证，不进入 native payload。有序补丁字节、sidecar、应用 helper 及实际静态输入仍纳入配方和发布材料身份。

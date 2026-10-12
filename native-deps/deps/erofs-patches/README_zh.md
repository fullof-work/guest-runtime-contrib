[English](README.md) | [简体中文](README_zh.md)

# EROFS 补丁

`series` 按顺序将补丁应用于原始 erofs-utils v1.9.1 归档，其 SHA-256 为 `a9ef5ab67c4b8d2d3e9ed71f39cd008bda653142a720d8a395a36f1110d0c432`。`0002-explicit-libgcrypt-sha256.patch` 仅修改 `lib/sha256.h` 和 `lib/sha256.c`。本补丁序列不包含可选的磁盘索引补丁。

`0003-exact-chunk-read.patch` 修复 `lib/blobchunk.c` 的 chunk 输入读取：检查有符号 syscall 结果、重试 EINTR、按正数短读推进，并在计算摘要之前拒绝读取错误或意外 EOF。正常内容、完整 SHA-256 去重和布局保持不变。相邻许可文件保留 `GPL-2.0-or-later OR Apache-2.0`。`make test-erofs` 在私有目录重新链接带 read/fortified-read 故障包装器的静态测试二进制，检查真实的 EINTR、短读、EIO、EOF 及部分读取后失败。成功路径比较镜像字节与解出的文件内容，而非仅检查 fsck 状态。包装器仅用于测试，不进入打包工具；若 native cache 缺少编译对象，测试通过选定构建配方在私有目录重建。

构建明确设置 `EROFS_USE_LIBGCRYPT_SHA256`，并禁用 OpenSSL 和多线程。两个 SHA 入口均在首次使用时初始化 Libgcrypt。单线程工具禁用安全内存，因为镜像 chunk 并非秘密数据。初始化或流式上下文分配失败会在摘要被使用前终止进程。一次性散列使用 `gcry_md_hash_buffer`；流式散列使用 SHA-256 上下文，复制全部 32 字节并关闭上下文。没有回退、链接器包装、摘要截断、布局变更或特定 CPU 的构建选项。

相邻 `.license` 将补丁映射到原文件的许可证：`lib/sha256.h` 为 `GPL-2.0-or-later OR Apache-2.0`，`lib/sha256.c` 为 `Unlicense`。保留上游 SPDX 和版权文本。其他 EROFS 文件继续适用各自许可证，包括实际链接进工具、标记为 GPL-2.0 的 hashmap 对象。参见[许可证范围](../../../LICENSE_SCOPE_zh.md)。

[Libgcrypt](https://www.gnupg.org/software/libgcrypt/index.html) 和 Libgpg-error 库适用 LGPL-2.1-or-later；其源码发行包还包含特定文件的许可证及声明。应收集实际目标软件包的 copyright、LICENSE/COPYING/NOTICE 材料及源码版本，不能给所有文件统一标注一个泛化许可证。原生发布收集器在 `EROFS-INPUTS.tsv` 和 `SOURCES.tsv` 中记录每项外部链接输入的字节摘要及软件包/源码身份，并拒绝缺失的声明。补丁字节和 sidecar 必须与所选提交的本地 Git 对象一致。

为重新构建源码或重新链接，须保留所选 guest 配方/helper、有序补丁目录、原始固定归档、两个链接映射、EROFS 对象/静态库输入，以及精确的目标库源码、构建配置和声明。[原生构建指南](../../README_zh.md) 说明软件包与 sysroot 输入。使用记录的目标编译器、选项及静态库运行 `make -C native-deps erofs` 重新构建。发布构建环境必须通过既有材料契约提供匹配的源码/重新链接材料；合成打包测试通过不代表这些材料已经配齐。

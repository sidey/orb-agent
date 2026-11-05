# 官方 v2.17.0 镜像上的双二进制覆盖

源码基线为官方 `v2.17.0`（`1254f6a52ea6c9fabf45c5d7069fa288e6c37eba`）。本地修改增加外部OID mapping加载和Agent参数转发，不混入原develop分支的其他功能。

`agent/docker/Dockerfile.snmp-overlay`有三个阶段：

1. `agent-build`：只编译修改后的 `orb-agent`。
2. `snmp-build`：只编译修改后的 `snmp-discovery`。
3. `final`：继承固定digest的官方2.17.0镜像，只COPY覆盖两个文件。

```text
/usr/local/bin/orb-agent
/usr/local/bin/snmp-discovery
```

两个Go模块使用Go1.26.10、`CGO_ENABLED=0`、`GOOS=$TARGETOS`、`GOARCH=$TARGETARCH`。最终镜像继承基础镜像的入口脚本、运行依赖和其他backend；不调用原完整Dockerfile，不重编译其他backend。首次仍需下载官方基础镜像、Go构建镜像与两个模块的依赖；基础镜像总体积不会因此缩小。

Agent也必须替换，因为它负责把 `mapping_file` 转发成 `--mapping-file`。mapping通过 `config/` 目录挂载，在backend启动时读取。

## 固定镜像与源码

官方镜像多架构索引：

```text
netboxlabs/orb-agent:2.17.0@sha256:db5284752c2afe755df77e8d8ea447f4eebd429e40a4668ce966581177563c38
```

对应平台manifest：

| 平台 | Digest |
|---|---|
| linux/amd64 | `sha256:433a7801698d7a2b1d217b3114340e76dfa860cdeef58028dfae4e8a0ec85964` |
| linux/arm64 | `sha256:c1a5826cf2fef528ffd5c982c44b111cae773a3fda862ebeca6e85f74b0db9d4` |

2026-11-05通过[Docker Hub官方仓库元数据](https://hub.docker.com/v2/repositories/netboxlabs/orb-agent/tags/2.17.0)核验。锁定记录保存在 `image.lock.json`，Compose和Dockerfile均使用相同索引digest。构建时按 `ORB_PLATFORM`选择相应Linux架构；不再依赖浮动 `develop` 或 `latest`。

Agent版本标识为 `2.17.0-external-mapping`，SNMP backend标识为 `1.39.0-external-mapping`，表示携带本地补丁的自定义版本。构建提交字段记录正式版基线，不表示本地补丁已经成为官方发布。

## 使用方式

默认 `compose.yaml`已经使用覆盖Dockerfile，按 [README.md](README.md)执行即可，无需再加overlay文件。原先的 `-f compose.yaml -f compose.overlay.yaml`仍然有效，但现在与单独使用基础文件相同。`compose.verify.yaml`继续用于无设备、无Diode写入的启动验证。

基础镜像与基线提交写在Compose构建参数中，旧 `.env` 中的 `ORB_BASE_IMAGE`、`ORB_BUILD_COMMIT`不会更改它们。以后升级正式版，应同步迁移源码、更新锁文件及两处镜像引用并重新验证。

当前Compose使用local配置且不挂载 `/opt/orb/files`。如果在其他部署中启用fleet或动态二进制管理，应确认它没有通过PATH覆盖镜像中的 `snmp-discovery`。

## 验证范围

当前环境没有可用的Docker daemon和Compose插件。源码测试、交叉编译和本机启动验证与实际Docker镜像运行需分别记录；尚未执行真实交换机采集或Diode/NetBox入库验收。

详细结果见 [VALIDATION.md](VALIDATION.md)：竞态检测测试、两模块lint、双Linux架构静态编译及本机双进程启动验证均通过；Docker镜像构建和容器运行尚未验收。

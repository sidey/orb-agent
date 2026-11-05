# Orb Agent v2.17.0：外部 SNMP mapping 部署

当前补丁基于官方 `v2.17.0`，源码提交为 `1254f6a52ea6c9fabf45c5d7069fa288e6c37eba`。默认Compose只构建修改后的 `orb-agent` 和 `snmp-discovery`，在官方2.17.0镜像上覆盖这两个可执行文件。其余backend、入口脚本和运行依赖继承官方镜像。

官方基础镜像固定为：

```text
netboxlabs/orb-agent:2.17.0@sha256:db5284752c2afe755df77e8d8ea447f4eebd429e40a4668ce966581177563c38
```

这是多架构索引digest，包含Linux amd64与arm64；不会随着 `latest`、`develop` 或tag变化而换成其他镜像。来源与各平台digest记录在 [image.lock.json](image.lock.json)，构建实现见 [Dockerfile.snmp-overlay](../../agent/docker/Dockerfile.snmp-overlay)。

## 文件

| 文件 | 用途 |
|---|---|
| `compose.yaml` | 默认的双二进制覆盖构建与运行服务 |
| `compose.verify.yaml` | 无设备策略、无Diode写入的启动验证配置 |
| `compose.overlay.yaml` | 兼容旧命令的覆盖文件；现在可省略 |
| `config/agent.yaml` | 正式接入模板，设备策略初始为空 |
| `config/agent.verify.yaml` | `dry_run: true` 与 `policies: {}` |
| `config/mapping.yaml` | v2.17.0内置mapping的运行时副本 |
| `.env.example` | 运行时凭据占位值与目标平台 |
| `image.lock.json` | 正式版源码与基础镜像的固定对应关系 |

`mapping.yaml`沿用v2.17.0的采集集合，尚未按此前40项资产需求收敛，也不会增加未实现的ARP/FDB/LLDP/VRRP解析器。开发版后续新增的Cisco trunk mapping没有带入本正式版。正式启用设备策略前，需要核对采集范围。

## 1. 构建与启动验证

在本目录执行。已有 `.env` 时保留它并按示例补充平台；旧文件中的 `ORB_BASE_IMAGE`、`ORB_BUILD_COMMIT` 已不参与Compose构建，无法覆盖固定基础镜像和源码标识。

```sh
cp -n .env.example .env
```

编辑 `.env` 中的 `ORB_PLATFORM`：x86-64服务器用 `linux/amd64`，ARM64服务器或原生Apple Silicon测试用 `linux/arm64`。无需为无设备验证填写真实凭据。

```sh
docker compose -f compose.yaml -f compose.verify.yaml config --quiet
docker compose -f compose.yaml -f compose.verify.yaml build --pull
docker compose -f compose.yaml -f compose.verify.yaml up -d --no-build --pull never
docker compose -f compose.yaml -f compose.verify.yaml logs --tail=100 orb-agent
```

日志应包含 `loaded external OID mapping` 和 `snmp-discovery readiness ok, got version`。此配置只验证Agent管理backend与外部文件加载，不触发SNMP采集，也不写Diode。

```sh
docker compose exec orb-agent /usr/local/bin/snmp-discovery --help
```

帮助信息应包含 `mapping-file`。两个程序分别是独立的Linux可执行文件，由Agent启动backend子进程；不需要额外backend容器或发布其控制端口。本机macOS构建产物不能直接放入Linux镜像。

## 2. 正式接入

在 `.env` 填写从Orb容器可访问的Diode地址、Diode凭据和SNMP凭据。`localhost`指当前Orb容器；其他容器中的Diode需要使用共享网络上的服务名或可路由地址。本Compose只部署Orb，现有Diode到NetBox链路需要另行配置。

核对 `config/mapping.yaml`，再把 `config/agent.yaml` 的 `policies: {}` 替换为已批准的设备策略。例如下面的地址、站点和角色均需替换：

```yaml
  policies:
    snmp_discovery:
      approved-switches:
        config:
          timeout: 120
          snmp_timeout: 5
          defaults:
            site: REPLACE_WITH_SITE
            role: REPLACE_WITH_ROLE
        scope:
          targets:
            - host: "192.0.2.1"
          authentication:
            protocol_version: SNMPv2c
            community: ${SNMP_COMMUNITY}
```

省略 `schedule` 时每次启动执行一次；周期应按实际需求明确配置。所有策略共享同一份外部mapping。正式接入去掉verify文件：

```sh
docker compose config --quiet
docker compose up -d --no-build --pull never
docker compose logs --tail=100 orb-agent
```

从verify切换时，Compose会根据command变化重新创建容器。采集需要容器到设备SNMP端口的网络连通性，写入需要连接Diode。

## 3. 更新与停止

仅修改mapping或Agent配置后：

```sh
docker compose restart orb-agent
```

修改 `.env` 中的运行时凭据后，使用 `docker compose up -d --force-recreate --no-build --pull never` 更新容器环境。修改Go源码后重新 `docker compose build`，再 `docker compose up -d --no-build --pull never`；默认仍只编译两个程序。

外部mapping完整替换内置mapping，只在backend启动时读取。配置文件错误会使backend启动失败，不会回退到内置集合。

```sh
docker compose down
```

镜像覆盖细节见 [OVERLAY.md](OVERLAY.md)。验证结果与当前环境限制见 [VALIDATION.md](VALIDATION.md)。

# v2.17.0 迁移验证（2026-11-05）

源码基线：`v2.17.0` / `1254f6a52ea6c9fabf45c5d7069fa288e6c37eba`。

基础镜像：`netboxlabs/orb-agent:2.17.0@sha256:db5284752c2afe755df77e8d8ea447f4eebd429e40a4668ce966581177563c38`。

## 已通过

- Agent的SNMP backend包：`GOWORK=off go test -race ./agent/backend/snmpdiscovery`。
- SNMP backend整个Go模块：`GOWORK=off go test -race ./...`，包括外部mapping完整替换、错误不回退、OID请求与实体映射测试。
- Agent及SNMP backend分别执行 `make fix-lint`，使用Go1.26.10与golangci-lint v2.11.4，均为0 issues。
- 两个程序分别编译为macOS arm64、Linux amd64和Linux arm64；Linux四个产物均为静态ELF，无动态加载器或动态libc依赖。
- 本机实际运行修改后的Agent管理backend，使用空策略与Diode dry-run：含空格的mapping路径正确转发，外部YAML加载成功，状态接口HTTP 200，backend版本为 `1.39.0-external-mapping`，父子进程正常退出。
- 默认Compose、verify合并配置、兼容overlay合并配置通过Compose官方JSON Schema校验；兼容overlay与默认构建一致。
- Compose、Dockerfile及 `image.lock.json` 的镜像引用和源码提交一致；最终镜像阶段只覆盖两个二进制。
- 运行时mapping与v2.17.0内置mapping内容一致，未带入后续develop的Cisco trunk字段。

## 尚未执行

当前机器没有可用的Docker daemon和Compose插件，因此未实际拉取基础镜像、执行Dockerfile构建或启动容器。Docker Hub元数据核验、Go交叉编译与本机进程验证不等同于容器验收。

未访问真实交换机、Diode或NetBox，未做真实采集与入库验收。此次迁移没有扩充尚未实现的40项需求解析器。

## 本地分支

当前交付分支为 `codex/snmp-external-mapping-v2.17.0`，基于官方正式版tag。原develop实现保存在 `codex/snmp-external-mapping` 的本地快照提交 `2b068d22ff0f3af567ff0f9b1039b5d568d407f1` 中。

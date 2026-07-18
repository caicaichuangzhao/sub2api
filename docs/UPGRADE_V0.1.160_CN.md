# Sub2API v0.1.160 升级交付记录

## 交付状态

- 完成时间：2026-07-18
- 官方版本：`v0.1.160`
- 当前分支：`codex/upgrade-v0.1.156`
- 运行版本：`0.1.160`
- 状态：已完成并部署

## 升级范围

1. 合并官方 `v0.1.160`，合并提交为 `096d63cdc`。
2. 官方负责的协议模块保持与发布标签一致。
3. 保留时段计费、账号级长上下文计费、批次管理、Gemini 兼容和本地账号调度约束。
4. 同步构建版本元数据，提交为 `9553cdbc1`。
5. 只替换应用容器，不重建 PostgreSQL、Redis 或数据卷。

## 验证结果

- `go test ./... -count=1`：通过。
- `pnpm test:run`：通过。
- `pnpm build`：通过。
- Docker 镜像构建：通过。
- `/health`：返回 `200` 和 `{"status":"ok"}`。
- `/api/v1/settings/public`：报告版本 `0.1.160`。
- `sub2api-dev`：`healthy`。
- PostgreSQL 容器 ID：`46c6799deb51`，未重建。
- Redis 容器 ID：`d825c9b8a78a`，未重建。

## Docker 镜像

- 镜像标签：`sub2api-custom:v0.1.160-9553cdbc1`
- 导出文件：`backups/sub2api-image-20260718-0131-v0.1.160-9553cdbc1.tar`
- 文件大小：`60054016` 字节
- SHA256：`19562c671cce441fae4bd3e1c525831d1d6a927df358a78f7696403806195d4b`

校验与导入：

```powershell
Get-FileHash -Algorithm SHA256 backups/sub2api-image-20260718-0131-v0.1.160-9553cdbc1.tar
docker load -i backups/sub2api-image-20260718-0131-v0.1.160-9553cdbc1.tar
```

## 部署命令

本次部署仅重建应用服务：

```powershell
docker compose -f deploy/docker-compose.dev.yml up -d --build --no-deps sub2api
```

禁止在常规应用升级中删除或重建 PostgreSQL、Redis 及其数据目录。

## 定价更新修复

2026-07-18 修复了无时段定价配置时渠道保存失败的问题，提交为 `efd761e3f`。
空的 `time_pricing` 现在以合法 JSON `null` 写入 JSONB 字段，渠道模型定价和账号统计定价两条写入路径均覆盖回归测试。

修复后的 Docker 镜像：

- 镜像标签：`sub2api-custom:v0.1.160-pricing-fix-efd761e3f`
- 导出文件：`backups/sub2api-image-20260718-1717-v0.1.160-pricing-fix-efd761e3f.tar`
- 文件大小：`60054528` 字节
- SHA256：`70bbb356f6ac43ef4a67290a52f4b8fcd72dc756a7a3a1ea90dff25b3039ba14`

## GitHub 同步

升级分支同步到用户仓库：

```text
caicaichuangzhao/sub2api:codex/upgrade-v0.1.156
```

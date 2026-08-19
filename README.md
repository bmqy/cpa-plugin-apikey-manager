# CLIProxyAPI API Key 管理器

日期：2026-08-19  
执行者：Codex

这是一个标准 CLIProxyAPI 动态库插件，为官方 Management Center 增加一个独立资源页面，用于：

- 查看、添加、编辑、删除代理服务 API Key；
- 为 API Key 增加名称和备注；
- 按名称、备注或 Key 筛选；
- 默认掩码显示 Key，并支持单条显示/隐藏。

## 实现方式

插件只负责页面和名称/备注索引，API Key 的实际增删改查复用宿主的 Management API：

- `GET/PUT /v0/management/api-keys`：读取和全量保存 API Key 列表；
- `GET/PATCH /v0/management/plugins/api-key-manager/config`：读取和保存插件配置；
- `/v0/resource/plugins/api-key-manager/index.html`：插件页面资源和菜单入口。

名称/备注使用 API Key 的 SHA-256 作为配置键，配置中不重复保存 API Key 明文：

```yaml
plugins:
  enabled: true
  configs:
    api-key-manager:
      enabled: true
      priority: 1
      api_key_metadata:
        0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef:
          name: "个人主账号"
          remark: "本地开发"
```

## 构建

需要 Go、cgo 和对应平台的 C 编译器。插件必须编译为动态库：

```bash
go build -buildmode=c-shared -o api-key-manager.so .
```

目标平台扩展名：

- Linux：`api-key-manager.so`
- macOS：`api-key-manager.dylib`
- Windows：`api-key-manager.dll`

将产物放入 CLIProxyAPI 配置的 `plugins.dir`，例如：

```text
plugins/api-key-manager.so
```

## GitHub Actions 发布

仓库内的 `.github/workflows/release.yml` 会在推送 `v*.*.*` 标签或手动执行 workflow 时触发：

- 构建 Linux amd64、Windows amd64、macOS amd64、macOS arm64 动态库；
- 为每个平台生成符合 CLIProxyAPI 插件商店格式的 ZIP；
- 在 Release 中上传全部 ZIP 和 `checksums.txt`；
- 使用 GitHub Actions 的 `contents: write` 权限创建 Release。

发布示例：

```bash
git tag v0.1.0
git push origin v0.1.0
```

## 配置

```yaml
plugins:
  enabled: true
  dir: "plugins"
  configs:
    api-key-manager:
      enabled: true
      priority: 1
```

启动后打开官方 Management Center 的插件菜单“API Key 管理器”。页面会优先尝试读取同源官方面板保存的 Management Key；如果读取不到，在页面中手动输入即可。Management Key 仅保留在当前页面内存中。

## 已知限制

当前工作区没有 Go 工具链，无法在本机生成 `.so/.dylib/.dll`。提交前请在安装 Go 与 cgo 的环境执行构建，并通过 CLIProxyAPI 的 `/v0/management/plugins` 确认 `registered: true` 和 `effective_enabled: true`。

参考：

- https://help.router-for.me/cn/plugin/development.html
- https://help.router-for.me/cn/management/api

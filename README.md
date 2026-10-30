# anon-browser Go SDK

用于开发 anon-browser 自动化插件和编排工作流的 Go SDK：`github.com/browser-rpa/anon-browser-sdk`，Go 1.24+，仅依赖标准库。

提供插件协议类型、节点定义、能力调用和工作流构建工具。插件通过宿主提供的授权能力操作浏览器；生成的工作流交由 anon-browser 客户端校验和运行。

| 包 | 提供内容 |
| --- | --- |
| `plugin` | Invocation/Result/Manifest/NodeDescriptor、stdio Server、关联 ID、常用 capability client、Go Schema 构造 |
| `workflow` | 公共 Graph/Node/Edge/Input 类型、Go Builder、常用内置节点配置及变量/Secret/素材引用 |

`workflow` 只生成可交换的 JSON，不执行工作流，也不在插件进程中调度内置节点。结构校验不替代客户端 Catalog 校验、权限审批和发布。

## 本地联调

当前为开发预览版本，尚未发布版本标签。本地开发时将 SDK 与 examples 放在同级目录；examples 的 `go.mod` 使用 `v0.0.0` + 相对 `replace`。发布版本后，可改用对应版本号并移除 `replace`。

```text
workspace/
├── anon-browser-sdk/
└── anon-browser-examples/
```

## 插件入口

```go
server := plugin.NewServer()
err := server.Register("acme.page_title", 1, func(ctx context.Context, call plugin.NodeCall) (plugin.NodeResult, error) {
    title, err := call.Client.ReadTitle(ctx)
    if err != nil { return plugin.NodeResult{}, err }
    return plugin.NodeResult{NextPort: "success", Output: map[string]any{"title": title}}, nil
})
if err != nil { return err }
return server.Serve(context.Background(), os.Stdin, os.Stdout)
```

对应 Manifest 需要 `browser.read`，Node Policy 声明相同权限及 `sensitive` 安全等级。完整可运行入口和 Go Manifest 见 [公开 examples](https://github.com/browser-rpa/anon-browser-examples)。

Server 每个进程只处理一次节点调用，写完结果即返回。stdout 只放协议消息，日志放 stderr。SDK 序列化 capability 调用并校验 ID；失败不自动重试。宿主负责超时/取消并终止子进程，普通 `io.Reader` 的阻塞读取不能仅靠 context 中断。输入上限 1 MiB，SDK 累计输出上限 4 MiB，最多 64 次 capability 调用。

`Client` 提供 ReadTitle/ReadURL/ReadText、Navigate/Click/TypeText/Evaluate、GetVariable/SetVariable、HTTPRequest、GetStorage/SetStorage、WriteTextArtifact。高级使用可通过 `Call` 请求已开放的能力，但不会扩展宿主权限。HTTP 响应 Header 为 `map[string][]string`，保留同名多值。

### 声明校验与权限

打包前调用 `manifest.Validate()` 或 `manifest.Marshal()`，会检查节点身份、名称、分类、Schema、端口、权限及超时/重试策略。错误包含节点类型、版本和出错字段；客户端安装时会再次校验。节点请使用自己的命名空间，例如 `acme.page_title`，不要使用 `browser`、`core` 等内置命名空间。

运行时按当前节点的 `policy.required_permissions` 与 `manifest.permissions` 同时检查，不能借用同包其他节点的权限：

- 返回 `NodeResult.Variables` 也需要 `variables.write`；只返回 `Output` 不需要该权限。
- 返回 `NodeResult.Artifacts` 需要 `run.artifacts.write`。
- 使用 `network.fetch`、`browser.script` 或 `network.modify` 权限时，节点必须标为 `high_risk`。声明权限不代表宿主已提供相应 action。

安装或升级后重启客户端加载新节点。升级使用独立安装目录，不替换已注册节点正在使用的代码。插件是本机可执行程序，并非不可信代码沙箱，只安装可信来源的插件。

## 用 Go 编排

```go
flow, err := workflow.New("local", "复用会话").Add(
    workflow.Start("start"),
    workflow.BindSession("bind", "session-REPLACE_ME"),
    workflow.Open("open", false),
    workflow.Navigate("page", "https://example.test/workspace"),
    workflow.End("end"),
).Then("start", "bind", "open", "page", "end").Build()
if err != nil { return err }
return json.NewEncoder(output).Encode(flow)
```

`Then` 只连接 success；timeout/failed/rejected 等分支用 `Link` 明确连接。`Build` 返回独立快照和编码/结构错误。`N(id, type, config)` 支持尚未封装的 Catalog 节点；字段与端口以目标客户端当前 Catalog 为准。

已有便捷节点包括人工等待、会话绑定/保持、页面脚本注册/注销、外部指令领取/完成、外部回复请求、可靠 Webhook、响应监听、素材上传/下载和子流程调用。公共子流程统一定义在根图 `subflows` 中，使用 schema_version 3。

这些工作流能力**不是**新增的插件 capability。页面脚本/常驻会话/外部触发器应通过工作流与公开管理 API 配置，插件不能凭 SDK 创建任意 Run 或取得浏览器控制地址。

## 验证

```bash
go test -race ./...
go vet ./...
go list -m all
```

SDK 测试可独立运行，无需启动客户端、浏览器或数据库。测试覆盖协议交互、Manifest 校验和工作流构建；实际插件与工作流运行需在目标版本的客户端中验证。

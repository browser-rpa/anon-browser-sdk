# 受控浏览器业务操作

这些接口用于在一个插件节点内部完成上传、提交及结果核验。浏览器由工作流打开或绑定；插件不获取本机路径、CDP 地址或其他任务的 Session。

本分支新增 API 尚未包含在 README 原有的 SDK 固定版本中。开发时用 `go.work` 同时包含 SDK 和使用方；发布 SDK 后再更新使用方 `go.mod` 的固定版本。使用旧宿主会明确报告不支持的 capability，不应回退为绕过授权的实现。

## 客户端与权限

| Client 方法 | capability action | 节点和 Manifest 必须同时声明 |
| --- | --- | --- |
| `UploadFiles` | `browser.file_upload` | `browser.file_upload`；带 watch 时另需 `browser.network.observe`、`storage.read` |
| `WaitForResponse` | `browser.wait_for_response` | `browser.network.observe`、`storage.read`；click/press 另需 `browser.interact`，navigate/reload 另需 `browser.navigate` |
| `BindPage` / `ReleasePage` | `browser.page_bind` / `browser.page_release` | `browser.page.manage` |
| `ClickBoundPage` / `FillBoundPage` | `browser.page_click` / `browser.page_fill` | `browser.page.manage` |
| `WaitForBoundPageResponse` | `browser.page_wait_response` | `browser.page.manage`、`browser.network.observe`、`storage.read` |

以上节点声明 `non_idempotent`、最多一次执行，必须占用 `profile_lease` 和 `runtime`。上传还需 `active_page` 和 `artifact_io`；非绑定的响应等待需 `active_page`。`browser.page.manage` 要求 `high_risk`。权限、资源或参数错误会在派发动作前拒绝。

## 上传与核验

```go
watch := plugin.ResponseWatch{
    URLPattern: "https://example.com/api/upload",
    Method: "POST", TimeoutMS: 60000,
}
uploaded, err := call.Client.UploadFiles(ctx, plugin.UploadRequest{
    Selector: "input[type=file]", Origin: "https://example.com",
    Files: input.Files, // []plugin.MaterialRef，来自本次任务已授权的输入。
    Watch: &watch,
    Checks: []plugin.ResponseCheck{
        {Name: "accepted", Path: "/responses/0/body/code", Equals: 0},
    },
})
```

先把文件导入素材库，以 `{"artifact_ref":"artifact://material-..."}` 放在工作流配置或运行输入中。上传不接受任意磁盘路径/CDN URL，也不能从其他运行猜测素材引用。多文件需显式开启 `Multiple`，最多 20 个。

素材来源为 CDN 时，可先在客户端素材库选择「链接导入」，填写可直接下载的公网 HTTPS 链接及文件名；可设置大小上限和 SHA-256。导入完成后仍只把素材引用交给插件。链接导入不使用浏览器 Cookie、代理或自定义鉴权 Header，也不是插件的 `network.fetch` 调用；不要用普通小响应 HTTP 接口读取整段视频。若宿主版本尚无链接导入入口，先下载到本地再通过素材库上传。

`selected` 仅表示交付文件输入框；`response_received` 仅表示观察到完整响应；`unconfirmed` 表示超时或其他不确定结果。即使状态为 `response_received`，仍需检查业务返回码和页面处理状态。平台的视频转码/审核属于后续业务阶段。

## 响应结果与字段

先注册监听，再执行关联动作，避免快速请求丢失。支持 `first`、`last`、`nth`、`all`、`first_match`；`last/all` 必须设置一个收集边界。它等待请求完成，不只是等待响应头。

- `ResponseResult` 只含 outcome、状态码、计数及加密引用，不返回原始 URL、正文、Header、Cookie。
- `Fields` 使用 JSON Pointer 提取；敏感字段仍返回 `{"secret_ref":"..."}`，缺失字段为 nil，并列入 `MissingFields`。此处不自动写工作流变量；插件要写变量需另外声明 `variables.write`。
- `Checks` 在宿主内比较 JSON 标量并只返回布尔值。例：`/responses/0/body/code == 0`。最多 20 项；只有 outcome 为 `response` 才可能通过。缺失字段不等于 JSON null，并列入 `MissingChecks`；无法找到业务返回码时应标为未确认，而不是明确拒绝。
- `Checks` 不提供任意代码、正则或无限表达式。响应数值比较有精度/大小限制，超限不确认。
- 同一次运行内，每次调用保存独立的响应引用；后续调用不会覆盖先前结果。
- `Status == 200` 不等于发布成功。无法确认时返回独立分支，禁止自动重试提交。

## 绑定页面

绑定名称仅在当前 Run 内有效；宿主保存真实页面、文档代次、来源和校验条件。刷新、导航、注销或所有权变化后绑定会失效，不自动跟随新页面。

`FillBoundPage` 替换内容并保留首尾空格；`ClickBoundPage` / `WaitForBoundPageResponse` 使用 DOM 点击，不是 trusted 鼠标事件。当前仅支持顶层文档 CSS 选择器，不承诺 iframe、Shadow DOM、role/text Locator 或绑定页面文件上传。

会话创建/查找、人工等待、长期脚本、事件连接器、触发器和 Webhook 仍由工作流/管理 API 负责。插件不能通过该接口调度任意内置节点。

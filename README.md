# mytools · 开发者工具箱

一个用 [mygo](https://github.com/egoist/mygo) 的 **Native UI** 写的桌面小工具集：窗口由 Go 直接绘制，没有网页前端。

## 运行

```sh
go run ./cmd/mytools
```

依赖 `github.com/egoist/mygo v0.5.5`；二维码用 `github.com/boombuler/barcode` 生成。

## 功能

| 工具 | 说明 |
| --- | --- |
| JSON 美化 | 格式化 / 压缩 / 校验，缩进可选 2、4 或 Tab |
| 信息编解码 | Base64、Base64 (URL)、URL、Hex 的编码与解码，一键把结果回填输入 |
| 时间戳 | 显示当前时间（每秒刷新）与秒 / 毫秒时间戳，时间戳与日期时间双向互转 |
| 二维码 | 文本生成二维码，可选纠错级别 L/M/Q/H 与尺寸 |
| 随机密码 | 长度 4–64，可选大写 / 小写 / 数字 / 符号，可排除易混淆字符，保留最近 10 条记录 |
| 图片转 Base64 | 选择本地图片，预览并输出纯 Base64 或 Data URI |

## 目录结构

```
cmd/mytools/          可执行入口：应用名、菜单、窗口
internal/app/         窗口外壳：侧边栏导航、页面注册与切换
internal/tools/       每个工具一个文件，各自持有状态并实现 tools.Tool
internal/widgets/     各工具共用的小组件（标题、结果框、复制按钮等）
```

## 扩展

新增一个工具：在 `internal/tools` 里加一个文件，写一个类型实现 `tools.Tool`
（`ID`、`Name`、`Desc`、`View`），再把它加进 `tools.All()`。

需要用到窗口的工具（例如弹原生文件对话框）再实现 `tools.WindowAware`，
`App.SetWindow` 会在窗口创建后自动注入。

## 测试

```sh
go test ./...
```

- `internal/app`：用 `ui.NewTester` 在没有窗口的情况下渲染每个页面，
  布局或 Key 出错会直接失败；也验证侧边栏点击切换。
- `internal/tools`：JSON、编解码往返、时间解析、密码字符集、二维码生成等纯逻辑单测。

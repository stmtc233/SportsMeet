# SportsMeet

开源的单机离线田径运动会编排软件:报名、编排、秩序册、成绩录入、排名、成绩册,一台电脑完成。

- 基于 [mygo](https://github.com/egoist/mygo)(Go + 系统 WebView),支持 macOS / Windows / Linux
- 数据存本机 SQLite,不上传
- 可在「设置 → MCP」开启本机服务,让外部 AI 助手查询赛事、导入报名表、编排、批量排时间与录成绩
- 开放的交换格式:[docs/interchange-format.md](docs/interchange-format.md)

## 开发

需要 Go(版本见 `go.mod`,本机较旧时自动下载)与 Bun。

```sh
bun install
bun run dev
```

## 状态

开发中,尚未发布。已支持基础数据、35 个常用项目预设与自定义配置、计次与计分、普通队伍及校园全能、赛事统一组别与按年级分榜、团体总分与计分规则、不分道编排、投掷逐次成绩、表格导入、接力队伍与棒次、报名、预赛、次赛、半决赛与决赛编排、批量排时间、径赛 / 接力 / 远度 / 高度成绩与排名晋级、秩序册 / 成绩册 / 检录单 / 成绩记录表打印、组的实际开赛时间、交换格式导入导出、设置与本机 MCP。号码布等功能见 [项目计划](AGENTS.md#56-未实现与后续计划)。

## 本机 MCP

打开「设置 → MCP」,开启「允许本机连接」后点「复制连接信息」,将地址与请求头配置到支持 Streamable HTTP 的客户端。默认端口 18765,只监听 `127.0.0.1`,使用独立 Bearer 令牌;关闭后释放端口。

查询与单条基础数据保存可直接调用。批量操作、删除、覆盖已有成绩须先预览,用户确认后携带预览指纹提交。已有成绩的赛事、项目和组不能经 MCP 删除;空成绩可直接录入,名次与晋级由应用计算。工具目录与边界见 [MCP 说明](docs/agents/mcp.md)。

## 官网

静态页面在 `website/`,无构建,直接打开 `website/index.html` 即可预览。发布到 GitHub Pages 时,下载区自动换成最新 release 的链接。

## 品牌资源

「跑道 S」标识的[设计规范](docs/brand-guidelines.md)与 SVG、PNG、WebP、JPG、PDF、ICO、ICNS 文件位于 [resources/brand/](resources/brand/)。运行 `bun run brand:generate` 从矢量母版重新导出，并同步应用与官网图标。

## 验证

```sh
bun run check
```

页面冒烟需先 `bunx playwright install chromium`,再运行 `bun run test:ui`。完整检查命令、桌面实测与跨平台限制见 [验证范围](docs/verification.md)。

## 协议

[MIT](LICENSE),免费使用、修改和分发。

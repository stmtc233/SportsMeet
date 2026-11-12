# SportsMeet

开源的单机离线田径运动会编排软件:报名、编排、秩序册、成绩录入、排名、成绩册,一台电脑完成。

- 基于 [mygo](https://github.com/egoist/mygo)(Go + 系统 WebView),支持 macOS / Windows / Linux
- 数据存本机 SQLite,不联网
- 可选开启本机 MCP 服务,让 AI 助手协助导入报名表、编排
- 开放的交换格式:[docs/interchange-format.md](docs/interchange-format.md)

## 开发

需要 Go(版本见 `go.mod`,本机较旧时自动下载)与 Bun。

```sh
bun install
bun run dev
```

## 状态

规划中,尚未发布。

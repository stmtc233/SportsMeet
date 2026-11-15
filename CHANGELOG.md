# 发布说明

## 0.1.0

SportsMeet 首个正式版。免费开源的单机田径与校园运动会编排软件，在一台电脑上完成报名、编排、打印、成绩录入与排名，赛事数据保存在本机。

### 主要功能

- 赛事总览按赛前、比赛、赛后显示进度与下一步，按小时查看当天赛程。
- 管理组别、参赛单位与运动员，下发和回收 xlsx 报名登记表，提示超项与超额；新赛事可沿用往届项目与赛制。
- 提供 35 个常用项目预设，支持个人、接力、普通队伍与全能项目，以及预赛、次赛、半决赛和决赛。
- 自动分组排道、手动调整、批量编排和多轮次排时间，提示运动员时间冲突。
- 支持计时、远度、高度、计次与计分成绩，计算排名、晋级、全能积分和按组别汇总的团体总分，支持人工裁定并列。
- 按 A4 预览和打印报名表、秩序册、检录单、成绩记录表、成绩册与团体总分表，秩序册可选紧凑排版。
- 通过公开 JSON 交换格式导入导出赛事；本机 MCP 服务可供 AI 助手查询与协助操作。

### 下载安装

| 系统 | 安装包 | 便携版 |
|---|---|---|
| macOS 12 及以上，Intel 与 Apple 芯片通用 | `sportsmeet-0.1.0.dmg` | `sportsmeet-0.1.0-darwin-universal-portable.zip` |
| Windows 10 / 11，x86-64 | `sportsmeet-setup-0.1.0-amd64.exe` | `sportsmeet-0.1.0-windows-amd64-portable.zip` |
| Linux x86-64 | `sportsmeet_0.1.0_amd64.deb`、`sportsmeet-0.1.0-1.x86_64.rpm` | `sportsmeet-0.1.0-linux-amd64.tar.gz` |
| Linux ARM64（aarch64） | `sportsmeet_0.1.0_arm64.deb`、`sportsmeet-0.1.0-1.aarch64.rpm` | `sportsmeet-0.1.0-linux-arm64.tar.gz` |

已有用户可在「设置 → 关于」检查更新，升级前建议导出赛事备份。应用内更新文件经过签名校验。

### 使用须知

安装包未做 macOS 签名公证与 Windows 签名。macOS 首次打开若提示无法验证开发者，可在「系统设置 → 隐私与安全性」允许打开；Windows 可能出现 SmartScreen 提示。

Windows／Linux 的原生窗口、文件对话框、打印与安装，以及不同打印机的实际纸张分页，仍需实机验证。号码布自动编号与打印、风速录入、破纪录标记和奖状打印暂未提供。

[使用文档](https://stmtc233.github.io/SportsMeet/docs.html) · [反馈问题](https://github.com/stmtc233/SportsMeet/issues)

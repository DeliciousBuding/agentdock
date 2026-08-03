<div align="center">

[English](./README.md) | 简体中文

# AgentDock

**让 AI 的双手，真正触达你的每一台设备。**

打开网页版 ChatGPT，即可管理多台电脑与服务器：在真实设备上写代码、改配置、跑命令与部署，执行发生在你的机器上，不消耗Codex额度。

AgentDock是面向 AI Agent 的独立工具运行层，为本地电脑、远程服务器与容器环境提供统一、安全、可控的文件、命令、Git、Skill、MCP、浏览器自动化和任务执行能力。配置多台 AgentDock，还能跨设备协同，把原本要在多机之间来回切换的工作，收敛到一次对话里完成。

[快速开始](https://uvwt.github.io/agentdock-docs/zh-CN/docs/getting-started/install) · [在线文档](https://uvwt.github.io/agentdock-docs/zh-CN/) · [版本发布](https://github.com/uvwt/agentdock/releases) · [问题反馈](https://github.com/uvwt/agentdock/issues)

[![CI](https://github.com/uvwt/agentdock/actions/workflows/ci.yml/badge.svg)](https://github.com/uvwt/agentdock/actions/workflows/ci.yml)
[![GitHub Release](https://img.shields.io/github/v/release/uvwt/agentdock?display_name=tag&logo=github)](https://github.com/uvwt/agentdock/releases)
[![Docker Hub](https://img.shields.io/docker/pulls/agentdockio/agentdock?logo=docker&label=Docker%20Hub)](https://hub.docker.com/r/agentdockio/agentdock)
[![GHCR](https://img.shields.io/badge/GHCR-ghcr.io%2Fuvwt%2Fagentdock-2496ED?logo=docker&logoColor=white)](https://github.com/uvwt/agentdock/pkgs/container/agentdock)
[![License](https://img.shields.io/github/license/uvwt/agentdock)](./LICENSE)

</div>

<p align="center">
  <img
    src="./docs/assets/agentdock-multi-device.png"
    alt="AgentDock：通过一个 AI 对话统一操控多台设备"
    width="100%"
  />
</p>

## AgentDock 是什么

AgentDock 是一个面向 AI Agent 的独立工具运行层。

它将设备上的文件系统、命令执行、Git、Skill、动态 MCP、浏览器自动化和可恢复任务封装为统一的 MCP 能力，让网页版 ChatGPT、Claude、Codex 等支持 MCP 的客户端，以一致的方式操作本地电脑、远程服务器和容器环境。

除了类似 Codex 的项目开发与设备操作能力之外，你还可以在多台机器上分别部署 AgentDock，在同一个 AI 对话中同时接入，实现跨设备控制。

AgentDock 不提供聊天界面，也不负责模型推理。它专注于解决一件事：

> 让 AI Agent 在明确的权限边界内操作真实环境，并返回结构化、可追踪、可验证的执行结果。

```text
              ChatGPT / Claude / Codex
                        │
                        │ MCP（可接入多台）
          ┌─────────────┼─────────────┐
          ▼             ▼             ▼
   ┌───────────┐ ┌───────────┐ ┌───────────┐
   │ AgentDock │ │ AgentDock │ │ AgentDock │
   │  本机电脑  │ │  内网机器  │ │  云服务器  │
   └─────┬─────┘ └─────┬─────┘ └─────┬─────┘
         │             │             │
         ▼             ▼             ▼
   文件·命令·Git  穿透客户端等   转发·反代·部署
```

## 你可以用 AgentDock 做什么

- 用网页版 ChatGPT 直接管理多台电脑与服务器，无需分别 SSH 登录来回操作
- 在真实设备上写代码、改项目、跑测试并操作 Git，执行发生在本机或服务器，不依赖专用编程 Agent 额度
- 让 AI 管理 VPS、Docker 服务、反向代理和部署配置
- 让 AI 检查日志、进程、端口和真实运行状态
- 让 AI 操作登录后的网页与 macOS 桌面应用
- 配置多台 AgentDock，在一次对话中完成跨设备协同任务
- 通过 Skill 和动态 MCP 扩展外部能力
- 保存长时间任务的执行状态，并在中断后继续
- 用同一套工具模型连接 macOS、Linux、Windows 与容器环境

## 典型场景：跨设备完成内网穿透

例如你有一台位于内网中的电脑。要让外部访问它，通常需要：

1. **本地电脑**：启动并检查穿透客户端
2. **服务器**：配置转发、端口、域名和反向代理

过去需要分别登录两台设备来回操作。现在只需打开一个 ChatGPT 网页，把两台机器上的 AgentDock 都接入对话，即可让 AI 同时操作电脑和服务器，完成整套流程。

这类能力可以继续扩展到更多实际场景：多机部署、本地开发联调公网服务、跨环境排障、批量配置与状态巡检等。

## 为什么使用 AgentDock

| 能力 | 说明 |
| --- | --- |
| 网页即可操控设备 | 通过 MCP 接入 ChatGPT 等客户端，在对话中操作真实电脑与服务器 |
| 多机协同 | 配置多台 AgentDock，在同一对话中跨设备执行任务 |
| 统一工具入口 | 通过一个 MCP 服务提供文件、命令、Git、Skill、任务和浏览器等能力 |
| 本地与远程一致 | 同一套工具模型可运行在 macOS、Linux、Windows、VPS 和 Docker 环境 |
| 执行不占编程额度 | 写代码、跑命令、改配置发生在你的设备上，不依赖专用编程 Agent 额度 |
| 明确执行边界 | 对路径、权限、超时、输出长度和敏感信息进行约束 |
| 结构化执行结果 | 区分工具调用状态、命令退出状态、标准输出和错误信息 |
| 可扩展运行时 | 支持独立 Skill 和动态 MCP Server，无需将所有能力写入核心程序 |
| 可恢复任务 | 支持任务步骤、检查点、阻塞、恢复、最终审查和完成条件 |
| 面向正式部署 | 提供 Docker、systemd、macOS 和 Windows 安装方式，并持续发布正式镜像 |

## 快速开始

普通用户不需要下载源码、安装 Go 或自行构建镜像。

完整安装入口：[安装 AgentDock](https://uvwt.github.io/agentdock-docs/zh-CN/docs/getting-started/install)

### Windows 安装

Windows 普通用户从 Release 下载与系统架构匹配的签名离线安装包：常见 Intel/AMD 电脑使用 `AgentDockSetup-amd64.exe`，Windows ARM 设备使用 `AgentDockSetup-arm64.exe`。安装包已经包含 AgentDock 核心、托盘、内置 Skill 和 cloudflared，安装及升级过程中不会再从 GitHub 下载组件。安装向导默认只允许本机访问、登录后自动启动，并默认开启“管理员增强模式”。Setup 本身和系统托盘仍使用当前普通用户身份；安装时只通过一次 UAC 为 AgentDock 核心创建“最高权限”计划任务，DPAPI 凭据、Skill、任务和工作目录仍绑定当前登录用户。无法提权的账号会明确降级为普通用户模式。只有用户明确选择时才会配置临时或固定的 Cloudflare Tunnel 公网入口；检测到已有安装时默认直接升级并保留全部设置，只有主动选择“修改设置”才会展开配置页面。

PowerShell 安装脚本继续保留给自动化和高级用户使用。

### Docker 部署

正式镜像同步发布到 GitHub Container Registry 和 Docker Hub。Release 附带的 Compose 文件默认使用 GHCR，正式镜像以非 root 用户运行。

```bash
mkdir agentdock
cd agentdock

curl -fL \
  https://github.com/uvwt/agentdock/releases/latest/download/docker-compose.yml \
  -o docker-compose.yml

export AGENTDOCK_AUTH_TOKEN="$(openssl rand -hex 32)"

docker compose pull
docker compose up -d
```

查看运行状态：

```bash
docker compose ps
docker compose logs -f
```

默认 MCP 地址：

```text
http://127.0.0.1:18766/mcp
```

停止服务：

```bash
docker compose down
```

完整配置、数据持久化和客户端接入方式见 [Docker 快速部署](https://uvwt.github.io/agentdock-docs/zh-CN/docs/getting-started/docker)。

### macOS 图形安装

普通 macOS 用户可以从 GitHub Release 下载 `AgentDock-macos-universal.dmg`，打开后把 `AgentDock.app` 拖到“应用程序”快捷入口。DMG 已内置 Apple Silicon 与 Intel 两种架构的 AgentDock 核心、核心 Skill 和 Cloudflare `cloudflared`，安装、升级和修复组件时不再联网下载；实际创建 Quick Tunnel 或认证 Named Tunnel 时仍需要网络。当前免费版本使用 ad-hoc 签名、未经过 Apple 公证，因此首次启动需要在“应用程序”中右键 AgentDock 并选择“打开”；之后不需要终端或管理员权限。

首次启动的图形界面可以直接选择仅本机、临时公网或固定 Cloudflare 域名。固定域名和 Tunnel Token 都在图形表单中填写，Token 通过仅当前用户可读的临时文件交给安装器，不会进入进程命令行。

安装完成后，主控制面板会显示服务状态、版本、本地与公网 MCP 地址，以及默认遮罩的 Bearer Token 和 OAuth 登录密码；两项凭据都支持独立显示和复制。公网地址会自动通过 `/healthz` 检测可达性，“公网 MCP”一行也提供手动“测试”按钮，并显示延迟或失败原因。重新生成临时地址时，旧地址会立即隐藏，只有取得新地址后才重新显示，避免把旧地址误认为新结果。窗口内还可以启动、停止、重启、更新和打开日志。高级设置支持服务端口、日志级别、可选的 Nexus Endpoint 与 Token、浏览器工具，以及 AgentDock 服务和菜单栏应用两个独立的登录自启开关。首次勾选浏览器工具时会自动安装并验证 Browser Runner；取消勾选只停用功能，保留运行文件供下次复用。

### Cloudflare Tunnel 公网访问

macOS 图形应用以及 Linux、Windows 安装器不会要求普通用户理解 `quick`、`named` 等内部模式。公网安装只需要确认是否有已接入 Cloudflare 的域名：

- **有域名**：使用固定地址，适合长期运行；安装器会询问 HTTPS 公网地址和 Tunnel Token。
- **没有域名**：自动生成临时 `trycloudflare.com` 地址，适合快速体验；Tunnel 重启后地址可能变化。

两种方式都会自动生成或复用 **Bearer Token** 与 **OAuth** 配置。安装完成框会显示公网地址、MCP 地址、Bearer Token 和 OAuth 登录密码；OAuth 签名密钥与 Tunnel Token 只保存在受限配置文件中，不会显示或传给无关进程。

Linux 与 macOS 使用同一个入口；macOS 需要 `--register-service` 才会注册后台服务：

```bash
curl -fsSL https://github.com/uvwt/agentdock/releases/latest/download/install.sh -o /tmp/agentdock-install.sh
sh /tmp/agentdock-install.sh                 # Linux
sh /tmp/agentdock-install.sh --register-service  # macOS
```

Windows 普通用户直接运行 Release 中与系统架构匹配的签名离线安装包，在“连接方式”页面选择临时或固定公网入口。离线安装包内已包含 cloudflared，但固定公网模式仍需要有效的 Cloudflare Tunnel Token；离线安装只消除组件下载，不会绕过 Cloudflare 身份验证。高级用户使用 PowerShell 时，需要显式请求公网配置：

```powershell
$script = Join-Path $env:TEMP 'install-agentdock.ps1'
Invoke-WebRequest https://github.com/uvwt/agentdock/releases/latest/download/install.ps1 -OutFile $script
powershell -ExecutionPolicy Bypass -File $script -RegisterStartup -ConfigurePublicAccess
```

仅使用 `-RegisterStartup` 只会配置本机启动，不会默认开放公网。自动化仍可通过 `AGENTDOCK_TUNNEL_MODE=quick|named` 直接指定模式；固定模式还需要 `AGENTDOCK_SERVER_URL` 和 `AGENTDOCK_CLOUDFLARE_TUNNEL_TOKEN`。Windows 会把 Bearer Token、OAuth 密码、OAuth 签名密钥和 Tunnel Token 分别使用当前用户 DPAPI 加密保存。

macOS 和 Windows 的 Quick Tunnel 每次启动都会自动取得新地址、回写 OAuth Origin 与运行 manifest，并重启验证 AgentDock；Bearer Token、OAuth 登录密码和签名密钥保持不变。macOS 控制面板提供“重新生成临时地址”，Windows 托盘提供“重新生成临时公网地址”，可以随时手动刷新。临时地址变化后，用户仍需在客户端替换 MCP URL，并重新完成 OAuth 授权。

Docker Quick Tunnel：

```bash
curl -fL \
  https://github.com/uvwt/agentdock/releases/latest/download/docker-compose.cloudflare-tunnel.yml \
  -o docker-compose.cloudflare-tunnel.yml

docker compose \
  -f docker-compose.yml \
  -f docker-compose.cloudflare-tunnel.yml \
  --profile cloudflare-quick up -d

docker compose -f docker-compose.yml -f docker-compose.cloudflare-tunnel.yml \
  logs -f cloudflared-quick
```

Named Tunnel 使用 `cloudflare-named` profile，并在部署目录的 `.env` 中设置 `TUNNEL_TOKEN` 与 `AGENTDOCK_SERVER_URL`。公网模式下必须保留 AgentDock Bearer Token 或 OAuth 认证。

## 接入 AI 客户端

AgentDock 通过 MCP Streamable HTTP 提供工具能力。下面是一个通用配置示例，具体字段格式取决于所使用的 AI 客户端：

```json
{
  "mcpServers": {
    "agentdock": {
      "url": "http://127.0.0.1:18766/mcp",
      "headers": {
        "Authorization": "Bearer <AGENTDOCK_AUTH_TOKEN>"
      }
    }
  }
}
```

如果 AgentDock 仅监听本机回环地址并明确关闭认证，可以不发送 `Authorization` 请求头。对局域网或公网开放时，必须启用认证，并配置 HTTPS 和网络访问控制。

## 平台安装

| 平台 | 文档 |
| --- | --- |
| Docker | [Docker 快速部署](https://uvwt.github.io/agentdock-docs/zh-CN/docs/getting-started/docker) |
| Linux | [Linux 自动安装](https://uvwt.github.io/agentdock-docs/zh-CN/docs/getting-started/linux) |
| Linux / VPS | [systemd 部署](https://uvwt.github.io/agentdock-docs/zh-CN/docs/getting-started/vps) |
| macOS | [macOS 安装](https://uvwt.github.io/agentdock-docs/zh-CN/docs/getting-started/macos) |
| Windows | [签名 Setup.exe 与原生安装](https://uvwt.github.io/agentdock-docs/zh-CN/docs/getting-started/windows) |

每个平台文档均包含安装命令、启动检查、MCP 地址和认证方式。浏览器自动化、macOS 桌面操作、Windows / WSL、反向代理和数据迁移等内容位于对应进阶文档。

## 版本更新

正式 Release 二进制支持直接查看版本和更新：

```bash
agentdock --version
agentdock update
```

`agentdock update` 会下载匹配当前平台的最新 Release，校验 SHA-256 并验证新二进制，然后备份和替换当前版本。检测到 LaunchAgent、systemd、Windows Service、最高权限计划任务或 Windows 用户启动项时，会自动重启并验证新版本；失败时恢复旧版本。开发构建不能通过该命令更新。Windows 托盘调用的也是这条核心更新链路；托盘程序和 Setup 本身通过重新运行新版 `AgentDockSetup.exe` 更新。

## 核心能力

### 文件与命令

- UTF-8 文本读取、搜索、目录遍历和结构化修改
- 原子文件写入、路径边界和私密目录保护
- 有超时和输出边界的命令执行
- 标准输出、标准错误和退出码分离
- 长时间命令会话、PTY、会话观察、输入和停止
- 输出截断和敏感信息脱敏
- macOS、Linux、Windows 与 WSL 支持

### Git 与 GitHub

- 仓库状态、差异和提交记录读取
- 分支、提交、拉取和推送
- GitHub 仓库访问检查
- 修改前状态检查和修改后差异验证

### Skill 与动态 MCP

官方与社区 Skill 源码统一维护在 [uvwt/agentdock-skills](https://github.com/uvwt/agentdock-skills)。本仓库只保留必须随 AgentDock 运行时发布的三个自举核心 Skill。

- Skill 包校验、安装、激活和回滚
- 稳定版、开发版、Canary 和固定版本通道
- Skill 独立环境变量与运行环境
- 动态 MCP Server 注册、启停、刷新和移除
- Streamable HTTP 与 stdio 传输
- 工具搜索、Schema 检查和受控调用
- MCP Server 之间的配置隔离

### 原生 ACP 运行时

AgentDock 可以选择作为 ACP Client 原生托管本地 Coding Agent adapter。ACP 不是动态 MCP：adapter 进程、双向 JSON-RPC 连接、会话、长任务事件、权限交互和恢复状态都由 AgentDock Runtime 统一管理。

- `acp_session`：检查或认证 ACP Agent，并创建、加载、恢复、分叉、配置、关闭和删除持久会话
- `acp_prompt`：异步启动 Agent turn，通过有序事件增量观察、steer 或取消
- `acp_interaction`：处理 Agent 发起的权限请求
- Windows 使用 Job Object，Linux 与 macOS 使用独立进程组回收 adapter 及其子进程
- 所有会话工作目录和附加目录必须位于宿主机配置的允许根目录内
- 可选 ACP 方法严格按 `initialize` 返回的 capability 开启
- 重启后只恢复会话元数据，不自动重放可能产生副作用的 Prompt

ACP 默认关闭。启用时至少配置绝对 adapter 可执行路径和允许根目录：

```bash
AGENTDOCK_ACP_ENABLED=true
AGENTDOCK_ACP_AGENT=claude
AGENTDOCK_ACP_COMMAND=/absolute/path/to/node
AGENTDOCK_ACP_ARGS_JSON='["/absolute/path/to/claude-agent-acp/dist/index.js"]'
AGENTDOCK_ACP_ALLOWED_ROOTS=/srv/code,/srv/worktrees
```

敏感变量通过 `AGENTDOCK_ACP_ENV_FROM_ENV_JSON` 从宿主环境显式映射，不写入 AgentDock 状态。架构与安全边界见 [Native ACP Runtime](docs/native-acp-runtime.md)，Claude/Codex 验收顺序见 [Native ACP Acceptance](docs/native-acp-acceptance.md)。

### 浏览器与桌面自动化

- 浏览器会话启动、关闭和清理
- 页面跳转、点击、输入、选择和等待
- 页面文本、可交互元素、错误和网络响应检查
- 登录状态、持久化浏览器 Profile 和截图
- macOS 系统 Chrome 与桌面自动化支持

### 可恢复任务

- 持久化任务状态
- 明确的目标、步骤和完成条件
- 分阶段检查点
- 阻塞原因记录和中断恢复
- 最终审查与完成验证
- 可复用工作流模板

### Recall 与 NexusDock 集成

AgentDock 可以选择接入 NexusDock，为多个设备和 Agent 提供中心化能力：

- 长期项目记忆
- 运行手册和经验记录
- 工作流模板
- 私密笔记
- 多设备状态协同

NexusDock 属于可选集成。未配置时，AgentDock 的基础文件、命令、Git、Skill、MCP、浏览器和任务能力仍可独立运行。

## 使用 OAuth 接入 ChatGPT

ChatGPT 通过自定义 MCP 插件访问公网 AgentDock 时，推荐启用 OAuth。AgentDock 支持 Authorization Code、PKCE S256、动态客户端注册和 Refresh Token，ChatGPT 可以自动完成客户端注册，不需要手动创建 Client ID 或 Client Secret。

服务端至少配置：

```bash
AGENTDOCK_OAUTH_ENABLED=true
AGENTDOCK_SERVER_URL=https://agentdock.example.com
AGENTDOCK_OAUTH_PASSWORD=<至少-12-个字符的授权密码>
AGENTDOCK_OAUTH_TOKEN_SECRET=<至少-32-字节的随机签名密钥>
```

然后进入 ChatGPT 的 **设置 > 插件 > 高级设置**，开启开发人员模式并创建插件，MCP Server URL 填写：

```text
https://agentdock.example.com/mcp
```

保存插件后，浏览器会打开 AgentDock 授权页。确认请求来自刚刚创建的 ChatGPT 插件，输入 `AGENTDOCK_OAUTH_PASSWORD` 并完成授权，再通过一次 `server_info` 或其他只读工具调用验证连接。

公网入口必须使用 HTTPS，`AGENTDOCK_SERVER_URL` 只填写 Origin，不附加 `/mcp`。完整步骤、端点验证和常见问题见 [ChatGPT 接入教程](https://uvwt.github.io/agentdock-docs/zh-CN/docs/guides/chatgpt)。

## 镜像版本

| 镜像标签 | 用途 |
| --- | --- |
| `latest` / `<version>` | 正式运行镜像，不包含 Go 编译工具链 |
| `dev-latest` / `dev-<version>` | 开发镜像，包含 Go、C 和 C++ 构建工具链 |
| `browser-latest` / `browser-<version>` | 浏览器自动化镜像，包含 Chromium 运行环境 |

正式镜像同步发布到：

```text
ghcr.io/uvwt/agentdock
agentdockio/agentdock
```

正式环境建议固定具体版本，不要长期直接依赖 `latest`：

```yaml
services:
  agentdock:
    image: ghcr.io/uvwt/agentdock:<version>
```

## 运行目录

| 路径 | 用途 |
| --- | --- |
| `~/AgentDock` | 相对文件操作的默认工作目录 |
| `~/.agentdock` | AgentDock 状态、配置、会话和扩展数据 |

Docker 部署默认使用 named volume 保存持久化数据，以减少 Linux bind mount 的 UID 和 GID 冲突。需要让 AgentDock 访问宿主机目录时，应显式挂载所需路径，不要直接挂载整个根目录。

## 端口说明

| 运行方式 | 默认地址 |
| --- | --- |
| Docker 发布配置 | `http://127.0.0.1:18766/mcp` |
| 源码开发模式 | `http://127.0.0.1:8765/mcp` |

端口可以通过配置调整，客户端应以实际部署配置为准。

## 安全模型

AgentDock 会直接操作宿主机或容器内的真实资源，应将其视为基础设施服务进行部署和授权。

### 网络安全

- 仅监听回环地址时，可以根据实际需求关闭认证
- 监听非回环地址时，必须启用 Bearer Token 或 OAuth
- 对公网提供服务时，必须配置 HTTPS
- 推荐配合防火墙、反向代理和网络访问控制
- 不要直接将未认证的 MCP 服务暴露到公网

### 权限边界

- 使用独立系统用户运行 AgentDock
- 只授予完成实际任务所需的文件权限
- Docker 中只挂载需要访问的目录
- 不授予不必要的 root、Docker Socket 或宿主机权限
- Skill 与动态 MCP 的敏感变量使用独立环境存储
- ACP executable、参数、环境映射和允许根目录只能由宿主机配置，不能通过远程工具修改
- ACP 允许根目录不是 OS 沙箱；仍需使用独立系统用户、ACL、容器挂载和网络策略限制 adapter

### 执行验证

- 命令退出状态与工具调用状态分离
- 文件修改后检查真实差异
- 部署后验证进程、端口、日志和服务响应
- 长任务设置明确的完成条件
- 不仅根据“命令已执行”判断任务成功

## 从源码运行

本节面向 AgentDock 贡献者和需要调试运行时的开发者。

```bash
git clone https://github.com/uvwt/agentdock.git
cd agentdock

make check
make run
```

源码开发模式默认监听：

```text
http://127.0.0.1:8765/mcp
```

## 开发与贡献

提交代码前运行完整检查：

```bash
make check
```

项目使用 GitHub Actions 持续执行测试、静态检查、构建和发布验证。

用户文档独立维护在 [`uvwt/agentdock-docs`](https://github.com/uvwt/agentdock-docs)。修改用户可见行为、配置参数、安装方式或工具 Schema 时，应同步更新对应文档。

提交问题或功能建议请使用 [GitHub Issues](https://github.com/uvwt/agentdock/issues)。

## 项目边界

AgentDock 是工具运行层，不是完整的 AI 应用平台。

它不包含聊天界面、大模型推理服务、模型账号或 API 额度，也不会绕过认证或系统安全控制。AgentDock 可以被 ChatGPT、Claude、Codex 或其他支持 MCP 的 Agent 客户端调用，实际接入方式取决于客户端对 MCP 传输和认证配置的支持。

## 相关链接

- [在线文档](https://uvwt.github.io/agentdock-docs/zh-CN/)
- [文档源码](https://github.com/uvwt/agentdock-docs)
- [GitHub Releases](https://github.com/uvwt/agentdock/releases)
- [GitHub Container Registry](https://github.com/uvwt/agentdock/pkgs/container/agentdock)
- [Docker Hub](https://hub.docker.com/r/agentdockio/agentdock)
- [Linux Do](https://linux.do/)

## License

Apache License 2.0. See [LICENSE](./LICENSE).

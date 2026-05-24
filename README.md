# CFST 优选测速工具

基于 [XIU2/CloudflareSpeedTest](https://github.com/XIU2/CloudflareSpeedTest) 的 Web 图形化测速工具，支持 Cloudflare IP/域名优选、DNS 记录管理和测速数据共享。

## 功能特性

- **三种测速模式**
  - IP 模式：批量解析域名 → TCP 延迟筛选 → 下载测速，快速找出最优 IP
  - 域名模式：逐个域名串行测速，获取每个域名的独立测速结果
  - 单域名测速：域名列表中每行独立测速按钮，即点即测
- **Cloudflare DNS 管理**：通过 API Token 管理 A/AAAA/CNAME 记录，一键应用最优 IP
- **共享测速计划**：测速结果自动上传至 Cloudflare Worker，按省份+运营商汇总最优记录
- **多平台支持**：自动检测 Windows/Linux/macOS × amd64/arm64 的 cfst 可执行文件
- **实时进度显示**：Web 界面实时展示测速阶段、进度、速度、延迟等详细信息
- **纯 Python 标准库**：无需安装任何第三方依赖，Python 3.8+ 即可运行
- **移动端适配**：响应式布局，手机浏览器可正常使用

## 项目结构

```
├── server.py              # 主服务端（Python HTTP Server）
├── index.html             # Web 前端界面
├── start.bat              # Windows 启动脚本
├── config.json            # 用户配置（自动生成，不入库）
├── .gitignore
├── worker/
│   ├── worker.js          # Cloudflare Worker（共享测速数据服务）
│   ├── wrangler.toml      # Worker 部署配置
│   └── package.json       # Worker 依赖
└── cfst_<platform>_<arch>/ # cfst 可执行文件目录（需自行下载）
    ├── cfst / cfst.exe
    ├── ip.txt
    └── ipv6.txt
```

## 快速开始

### 1. 安装 Python

确保已安装 Python 3.8 或更高版本：

```bash
python --version
```

如未安装，前往 [python.org](https://www.python.org/downloads/) 下载，安装时勾选 **Add Python to PATH**。

### 2. 下载 CloudflareSpeedTest

前往 [XIU2/CloudflareSpeedTest Releases](https://github.com/XIU2/CloudflareSpeedTest/releases) 下载对应平台的版本：

| 平台 | 架构 | 目录名 |
|------|------|--------|
| Windows | x86_64 | `cfst_windows_amd64` |
| Windows | ARM64 | `cfst_windows_arm64` |
| Linux | x86_64 | `cfst_linux_amd64` |
| Linux | ARM64 | `cfst_linux_arm64` |
| macOS | x86_64 | `cfst_darwin_amd64` |
| macOS | ARM64 (M1/M2/M3/M4) | `cfst_darwin_arm64` |

将下载的压缩包解压到项目根目录，确保目录名与上表一致，且可执行文件位于目录内：

```
项目根目录/
└── cfst_windows_amd64/    # 以 Windows x86_64 为例
    ├── cfst.exe
    ├── ip.txt
    └── ipv6.txt
```

> **注意**：Linux/macOS 下需赋予执行权限：`chmod +x cfst`

### 3. 启动服务

**Windows**：双击 `start.bat`

**Linux/macOS**：

```bash
cd /path/to/project
python server.py
```

启动后浏览器访问 [http://localhost:8081](http://localhost:8081)

### 4. 开始测速

1. 打开网页后，点击 **IP测速** 或 **域名测速** 按钮
2. 等待测速完成，查看结果
3. 如需对单个域名测速，点击域名行右侧的测速按钮

## Cloudflare DNS 管理

### 配置 API 信息

1. 登录 [Cloudflare Dashboard](https://dash.cloudflare.com/)
2. 获取 **API Token**（需 Zone:DNS:Edit 权限）
3. 获取 **Zone ID**（域名概览页面右侧）
4. 在网页的 DNS 配置区域填入信息并保存

### 应用最优 IP

测速完成后，点击结果中的 IP 可直接添加为 DNS 记录，也可使用 CNAME 替换功能将域名指向优选域名。

## 共享测速 Worker 部署

项目内置了一个 Cloudflare Worker 用于汇总共享测速数据。如需自行部署：

### 前置条件

- Node.js 18+
- Cloudflare 账号

### 部署步骤

```bash
cd worker
npm install
npx wrangler d1 create cf-test
```

创建 D1 数据库后，将返回的 `database_id` 填入 `wrangler.toml`：

```toml
[[d1_databases]]
binding = "DB"
database_name = "cf-test"
database_id = "你的数据库ID"
```

然后部署：

```bash
npx wrangler deploy
```

部署完成后，将 Worker URL 填入 `server.py` 中的 `cf_config["worker_url"]`：

```python
cf_config = {"worker_url": "https://your-worker.your-subdomain.workers.dev"}
```

### Worker API

| 方法 | 路径 | 说明 |
|------|------|------|
| GET | `/` | 共享测速数据展示页面 |
| GET | `/api/stats` | 数据概览（总次数、覆盖省份、最优记录数） |
| GET | `/api/results/best?mode=ip\|ipv6\|domain\|domain_v6` | 按模式查询最优记录 |
| POST | `/api/results` | 提交测速结果 |

### 数据保留

共享数据保留 **3 天**，超期自动清理。每个省份+运营商+模式仅保留最快的一条记录。

## 测速流程说明

### IP 模式

```
域名列表 → DNS 解析获取 IP → TCP 延迟测试（50并发，4次取均值）
→ 按延迟排序取 Top 10 → cfst 下载测速 → 返回结果
```

- 速度快，适合快速筛选最优 IP
- IPv4 和 IPv6 分别测速，各取延迟最低的 10 个 IP

### 域名模式

```
域名列表 → 逐个域名 DNS 解析 → cfst 下载测速 → 下一个域名
```

- 结果精确，每个域名独立测速
- 耗时较长，适合需要精确对比各域名速度的场景

### 单域名测速

```
点击域名行测速按钮 → DNS 解析 → cfst 下载测速 → 返回结果
```

- 即点即测，适合验证单个域名

## 常见问题

**Q: 提示"未找到 cfst 可执行文件"**

A: 确保已下载 CloudflareSpeedTest 并解压到正确目录，目录名需与平台对应（见上方表格）。

**Q: 测速卡住不动**

A: 检查网络连接，确保能访问 Cloudflare 节点。部分网络环境可能需要代理。

**Q: DNS 记录添加失败**

A: 检查 API Token 是否有 DNS 编辑权限，Zone ID 是否正确，域名是否已在 Cloudflare 管理。

**Q: 共享数据上传失败**

A: 确认 Worker 已正确部署且可访问。共享上传不影响本地测速功能。

## 致谢

- [XIU2/CloudflareSpeedTest](https://github.com/XIU2/CloudflareSpeedTest) - Cloudflare IP 测速工具
- 内置优选域名来自社区贡献

## 许可证

MIT License

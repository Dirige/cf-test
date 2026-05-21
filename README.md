# Cloudflare Speed Test Server + Worker

Cloudflare 优选域名测速系统，包含本地服务器和 Cloudflare Worker 两部分。

## 项目结构

```
├── cfst-server/          # Go 服务器（本地运行）
├── worker/               # Cloudflare Worker
├── .github/workflows/    # GitHub Actions
├── task/                 # 测速核心逻辑
├── utils/                # 工具函数
├── main.go               # 原始命令行工具
└── PLAN.md               # 项目开发计划
```

## 快速开始

### 1. 上传到 GitHub

```bash
# 在 cf-test-upload 目录下初始化 Git
git init
git add .
git commit -m "Initial commit"

# 添加远程仓库（先在 GitHub 上创建仓库）
git remote add origin https://github.com/YOUR_USERNAME/cf-test.git
git branch -M main
git push -u origin main
```

### 2. 设置 GitHub Secrets

在 GitHub 仓库的 Settings > Secrets and variables > Actions 中添加：

- `CLOUDFLARE_API_TOKEN` - Cloudflare API Token
- `DOCKERHUB_USERNAME` - Docker Hub 用户名
- `DOCKERHUB_TOKEN` - Docker Hub 访问令牌

### 3. 自动部署

推送到 main 分支后，GitHub Actions 会自动：
- 构建并推送 Docker 镜像到 Docker Hub
- 部署 Worker 到 Cloudflare
- 初始化 D1 数据库

## 功能特性

- ✅ 域名测速（12 个内置优选域名）
- ✅ 中国地图可视化（ECharts）
- ✅ 按省份/运营商显示最佳域名
- ✅ 自动更新 DNS 记录
- ✅ 定时测速
- ✅ 结果共享到云端

## 技术栈

- **后端**: Go 1.18+
- **Worker**: Cloudflare Workers (TypeScript)
- **数据库**: Cloudflare D1
- **地图**: ECharts
- **容器化**: Docker

## 配置说明

### cfst-server/config.yaml

```yaml
server:
  host: "0.0.0.0"
  port: 8080

worker_url: "https://your-worker.workers.dev"

dns:
  zone_id: ""
  record_name: "example.com"
  record_type: "CNAME"

speedtest:
  timeout: 10
  auto_update_dns: true
  schedule: ""
```

## 许可证

MIT License

# Cloudflare Speed Test Server

一个基于 Go 的 Cloudflare 优选域名测速系统，可以自动测试优选域名的速度并更新 DNS 记录。

## ✨ 功能特性

- 🚀 **域名测速** - 自动测试多个优选域名的延迟和下载速度
- 🗺️ **地图可视化** - 使用 ECharts 中国地图展示各省份最佳域名
- 🔄 **自动 DNS** - 测速完成后自动更新 Cloudflare DNS 记录
- ⏰ **定时任务** - 支持 cron 表达式定时执行测速
- 🐳 **Docker 支持** - 提供 Docker 和 docker-compose 部署方案

## 📁 项目结构

```
cf-test/
├── cfst-server/              # Go 本地服务器
│   ├── internal/             # 内部包
│   │   ├── api/             # HTTP API 处理
│   │   ├── config/          # 配置管理
│   │   ├── crypto/          # 加密解密
│   │   ├── dns/             # DNS 操作
│   │   ├── geoip/           # IP 地理位置
│   │   ├── reporter/        # 结果上报
│   │   ├── scheduler/       # 定时任务
│   │   └── speedtest/       # 测速核心
│   ├── web/                 # Web 前端
│   ├── main.go              # 入口文件
│   ├── config.yaml.example  # 配置示例
│   ├── Dockerfile           # Docker 构建文件
│   └── docker-compose.yml   # Docker Compose
└── .github/workflows/       # GitHub Actions
    ├── build.yml            # 构建二进制并发布 Release
    └── docker.yml           # 构建并推送 Docker 镜像
```

## 🚀 快速开始

### 方式一：下载预编译二进制

前往 [Releases](https://github.com/Dirige/cf-test/releases) 页面下载对应平台的二进制文件。

```bash
# 下载后赋予执行权限 (Linux/macOS)
chmod +x cfst-server

# 复制配置文件
cp config.yaml.example config.yaml
# 编辑 config.yaml 填入你的配置

# 运行
./cfst-server -c config.yaml
```

### 方式二：Docker 部署

#### 1. 配置文件

```bash
cd cfst-server
cp config.yaml.example config.yaml
# 编辑 config.yaml 填入你的配置
```

#### 2. 启动服务

```bash
docker-compose up -d
```

或直接使用 Docker 命令：

```bash
docker pull ghcr.io/dirige/cf-test:latest
docker run -d -p 8080:8080 -v ./config.yaml:/app/config.yaml ghcr.io/dirige/cf-test:latest
```

### 方式三：本地编译运行

#### 1. 克隆项目

```bash
git clone https://github.com/Dirige/cf-test.git
cd cf-test
```

#### 2. 配置 cfst-server

```bash
cd cfst-server
cp config.yaml.example config.yaml
```

编辑 `config.yaml`，填入你的配置：

```yaml
server:
  port: 8080
  host: "0.0.0.0"

dns:
  zone_id: "你的 Cloudflare Zone ID"
  record_name: "你的域名.com"
  record_type: "CNAME"
  api_token: "你的 Cloudflare API Token"

speedtest:
  timeout: 30
  auto_update_dns: true
  schedule: ""  # cron 表达式，如 "0 */6 * * *" 每6小时执行
  cfst_path: "./cfst"
  dns_server: "223.5.5.5"

domains:
  - name: "CF优选-090227"
    domain: "youxuan.cf.090227.xyz"
  - name: "MIYU优选"
    domain: "saas.sin.fan"
  - name: "Mingyu优选"
    domain: "bestcf.030101.xyz"
  - name: "NB优选"
    domain: "cf.cf.cnae.top"
  - name: "NexusMods"
    domain: "staticdelivery.nexusmods.com"
  - name: "Visa官方"
    domain: "www.visa.cn"
  - name: "WeTest优选"
    domain: "cf.cloudflare.182682.xyz"
  - name: "乌克兰外交部"
    domain: "mfa.gov.ua"
  - name: "无名氏维护域名"
    domain: "cf.tencentapp.cn"
  - name: "秋名山优选"
    domain: "cf.877774.xyz"
  - name: "育碧商店"
    domain: "store.ubi.com"
```

#### 3. 编译运行

```bash
# Windows
go build -o cfst-server.exe .

# Linux/macOS
go build -o cfst-server .

# 运行
./cfst-server -c config.yaml
```

访问 http://localhost:8080 即可使用 Web 界面。

## ⚙️ 配置说明

### Cloudflare API Token

1. 登录 [Cloudflare Dashboard](https://dash.cloudflare.com/)
2. 进入 **My Profile** → **API Tokens**
3. 创建 Token，权限选择：
   - **Zone - DNS - Edit**（用于更新 DNS 记录）
   - **Zone - Zone - Read**（用于读取 Zone 信息）

### Zone ID

1. 登录 Cloudflare Dashboard
2. 选择你的域名
3. 在右侧 **API** 部分可以看到 **Zone ID**

### 测速域名

在 `config.yaml` 的 `domains` 部分添加你要测速的域名：

```yaml
domains:
  - name: "显示名称"
    domain: "your-domain.com"
```

## 📖 API 接口

### 测速相关

| 接口 | 方法 | 说明 |
|------|------|------|
| `/api/speedtest/ip` | POST | IP 段测速 |
| `/api/speedtest/domain` | POST | 域名测速 |
| `/api/speedtest/single` | POST | 单域名测速 |

### 域名管理

| 接口 | 方法 | 说明 |
|------|------|------|
| `/api/domains` | GET | 获取域名列表 |
| `/api/domains` | POST | 添加域名 |
| `/api/domains/:name` | DELETE | 删除域名 |

### DNS 管理

| 接口 | 方法 | 说明 |
|------|------|------|
| `/api/dns` | GET | 获取 DNS 记录 |
| `/api/dns/record` | POST | 添加 DNS 记录 |
| `/api/dns/replace` | POST | 替换 DNS 记录 |
| `/api/dns/batch` | POST | 批量更新 DNS |

### 其他

| 接口 | 方法 | 说明 |
|------|------|------|
| `/api/results/best` | GET | 获取最佳结果 |
| `/api/geoip` | GET | IP 地理位置查询 |
| `/api/status` | GET | 服务状态 |

## 🔧 开发指南

### 本地开发

```bash
# 克隆项目
git clone https://github.com/Dirige/cf-test.git
cd cf-test

# 安装 Go 依赖
cd cfst-server
go mod download

# 编译运行
go run main.go -c config.yaml.example
```

### 编译不同平台

```bash
# Windows 64位
GOOS=windows GOARCH=amd64 go build -o cfst-server.exe .

# Linux 64位
GOOS=linux GOARCH=amd64 go build -o cfst-server .

# Linux ARM64
GOOS=linux GOARCH=arm64 go build -o cfst-server .
```

### GitHub Actions

项目包含 2 个 GitHub Actions 工作流：

1. **build.yml** - 编译多平台二进制文件并发布到 GitHub Releases（推送 tag 时触发）
2. **docker.yml** - 构建并推送 Docker 镜像到 GHCR 和 Docker Hub

需要在 GitHub 仓库设置以下 Secrets：

- `CLOUDFLARE_API_TOKEN` - Cloudflare API Token
- `DOCKERHUB_USERNAME` - Docker Hub 用户名
- `DOCKERHUB_TOKEN` - Docker Hub 访问令牌

### 发布新版本

推送 tag 即可自动构建并发布：

```bash
git tag v1.0.0
git push origin v1.0.0
```

## 🐳 Docker 镜像

预构建镜像支持 `linux/amd64` 和 `linux/arm64` 两种架构：

```bash
# GHCR
docker pull ghcr.io/dirige/cf-test:latest

# Docker Hub
docker pull dirige/cf-test:latest
```

## 🤝 贡献指南

1. Fork 本仓库
2. 创建特性分支 (`git checkout -b feature/AmazingFeature`)
3. 提交更改 (`git commit -m 'Add some AmazingFeature'`)
4. 推送到分支 (`git push origin feature/AmazingFeature`)
5. 创建 Pull Request

## 📝 许可证

本项目基于 MIT 许可证开源 - 详见 [LICENSE](LICENSE) 文件

## 🙏 致谢

- [CloudflareSpeedTest](https://github.com/XIU2/CloudflareSpeedTest) - 测速核心逻辑
- [ECharts](https://echarts.apache.org/) - 地图可视化

## 📞 联系方式

如有问题或建议，请通过以下方式联系：

- GitHub Issues: [提交 Issue](https://github.com/Dirige/cf-test/issues)
- GitHub Discussions: [参与讨论](https://github.com/Dirige/cf-test/discussions)
- Telegram 交流群: [加入群组](https://t.me/Dirige_Proxy)

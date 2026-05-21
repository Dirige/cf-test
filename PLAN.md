# Cloudflare Domain Speed Test - 项目开发计划 (v3)

## 项目概述

基于 CloudflareSpeedTest 核心测速能力，构建一个**域名测速 + 数据共享**系统：

**两个独立组件：**
1. **Cloudflare Worker** — 部署在 Cloudflare，展示所有用户的汇总测速结果 + 中国地图
2. **Docker/本地程序** — 每个用户本地运行，测速后结果同步到共享 D1 数据库

---

## 架构设计

```
┌─────────────────────────────────────┐
│     Cloudflare Worker (你部署)       │
│                                     │
│  Web 页面:                          │
│  ┌─────────────────────────────┐   │
│  │  中国地图 (ECharts)          │   │
│  │  鼠标悬停 → 显示各省最佳域名  │   │
│  ├─────────────────────────────┤   │
│  │  全国测速结果汇总表格         │   │
│  │  省份 | 电信 | 联通 | 移动   │   │
│  └─────────────────────────────┘   │
│                                     │
│  API:                               │
│  POST /api/results  ← 接收测速结果  │
│  GET  /api/results/best             │
│  GET  /api/domains                  │
│                                     │
│  ┌─────────────────┐               │
│  │  D1 数据库 (绑定) │               │
│  └─────────────────┘               │
└────────────▲────────────────────────┘
             │
             │ HTTP POST (发送测速结果)
             │
┌────────────┴────────────────────────┐
│   Docker / 本地程序 (每个用户运行)    │
│                                     │
│  配置文件 (config.yaml):             │
│  - 用户域名 (record_name)            │
│  - DNS Zone ID                      │
│  - 定时测速设置                      │
│  (CF API Token 已内置加密)           │
│                                     │
│  Web 页面 (本地):                    │
│  ┌─────────────────────────────┐   │
│  │  我的测速结果                 │   │
│  │  当前最佳: cf.090227.xyz     │   │
│  ├─────────────────────────────┤   │
│  │  我的 DNS 记录:               │   │
│  │  example.com → cf.090227.xyz │   │
│  │  [编辑]                      │   │
│  ├─────────────────────────────┤   │
│  │  域名管理: [添加] [删除]      │   │
│  ├─────────────────────────────┤   │
│  │  ⚠ 共享计划声明              │   │
│  └─────────────────────────────┘   │
│                                     │
│  核心功能:                          │
│  - 测速引擎 (12个域名)              │
│  - 定时测速 (cron)                  │
│  - 自动更新 DNS                     │
│  - 结果同步到 Worker/D1             │
└─────────────────────────────────────┘
```

---

## 组件一：Cloudflare Worker

### 目录结构
```
worker/
├── src/
│   ├── index.ts          # Worker 入口 + 路由
│   ├── api.ts            # API 处理 (接收结果、查询数据)
│   ├── pages/
│   │   └── index.html    # 主页面 (内嵌在 Worker 中)
│   └── types.ts          # 类型定义
├── wrangler.toml         # Worker 配置 (绑定 D1)
├── package.json
└── tsconfig.json
```

### D1 表结构
```sql
CREATE TABLE IF NOT EXISTS speed_results (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    province TEXT NOT NULL,
    isp TEXT NOT NULL,
    domain TEXT NOT NULL,
    domain_name TEXT,
    download_speed REAL,
    latency REAL,
    ip_address TEXT,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS best_results (
    province TEXT NOT NULL,
    isp TEXT NOT NULL,
    domain TEXT NOT NULL,
    domain_name TEXT,
    download_speed REAL,
    latency REAL,
    updated_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (province, isp)
);

CREATE TABLE IF NOT EXISTS domains (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    name TEXT,
    domain TEXT UNIQUE NOT NULL,
    is_builtin BOOLEAN DEFAULT 0
);
```

### Worker API 接口
| 方法 | 路径 | 说明 |
|------|------|------|
| POST | `/api/results` | 接收测速结果，写入 D1，更新 best_results |
| GET | `/api/results/best` | 返回各省各运营商最佳域名 |
| GET | `/api/domains` | 返回域名列表 |

### Worker 页面功能
- ECharts 中国地图
- 鼠标悬停省份 → tooltip 显示电信/联通/移动最快域名
- 下方表格：省份按拼音排序，4列（省份、电信、联通、移动）
- 页面底部共享计划声明（字体放大）

---

## 组件二：Docker / 本地程序

### 目录结构
```
cfst-server/
├── cmd/
│   └── server/
│       └── main.go           # 入口
├── internal/
│   ├── api/
│   │   ├── handler.go        # 本地 API 处理
│   │   └── routes.go         # 路由定义
│   ├── config/
│   │   └── config.go         # 配置解析
│   ├── dns/
│   │   └── client.go         # CF DNS 客户端 (更新解析)
│   ├── geoip/
│   │   └── geoip.go          # IP 识别 (省份+运营商)
│   ├── reporter/
│   │   └── reporter.go       # 结果上报到 Worker
│   ├── scheduler/
│   │   └── scheduler.go      # 定时任务
│   └── speedtest/
│       └── domain.go         # 域名测速引擎
├── web/
│   └── index.html            # 本地 Web 页面
├── config.yaml               # 配置文件
├── Dockerfile
├── docker-compose.yml
└── go.mod
```

### 配置文件 (config.yaml)
```yaml
server:
  port: 8080

# Worker API 地址 (用于上报结果和获取域名列表)
worker_url: "https://your-worker.workers.dev"

# CF DNS 配置 (用于更新自己的 DNS 记录)
dns:
  zone_id: ""                 # 你的 DNS Zone ID
  record_name: "example.com"  # 你的域名
  record_type: "CNAME"

# 定时测速
speedtest:
  timeout: 10
  auto_update_dns: true
  schedule: ""                # cron 表达式, 空=不定时
```

### 本地页面功能
- 显示用户自己的测速结果（当前最佳域名 + 速度）
- 显示用户的 DNS 记录（example.com → 最佳域名），可编辑
- 域名管理（从 Worker 获取列表，可本地添加/删除）
- 底部共享计划声明（字体放大）

### 测速流程
```
1. 启动时从 Worker API 获取域名列表
2. 用户点击 [开始测速] 或定时触发
3. 获取用户 IP → 识别省份+运营商
4. 遍历域名列表测速
5. 找出最快域名
6. POST 结果到 Worker API → Worker 写入 D1
7. 如果 auto_update_dns=true → 调用 CF DNS API 更新记录
8. 本地页面显示结果
```

---

## 内置优选域名 (12个)

| 序号 | 名称 | 域名 |
|------|------|------|
| 1 | CF优选-090227 | youxuan.cf.090227.xyz |
| 2 | Shopify官方 | www.shopify.com |
| 3 | Mingyu优选 | bestcf.030101.xyz |
| 4 | 育碧商店 | store.ubi.com |
| 5 | WeTest优选 | cf.cloudflare.182682.xyz |
| 6 | MIYU优选 | saas.sin.fan |
| 7 | NexusMods | staticdelivery.nexusmods.com |
| 8 | 乌克兰外交部 | mfa.gov.ua |
| 9 | NB优选 | cf.cf.cnae.top |
| 10 | Visa官方 | www.visa.cn |
| 11 | 秋名山优选 | cf.877774.xyz |
| 12 | 无名氏维护域名 | cf.tencentapp.cn |

---

## 开发任务分解

### 组件一：Cloudflare Worker (5个任务)
- [ ] 1. Worker 项目初始化 (wrangler.toml + TypeScript)
- [ ] 2. D1 数据库表创建 + API 接口
- [ ] 3. ECharts 中国地图页面
- [ ] 4. 地图交互 (悬停显示各省最佳域名)
- [ ] 5. 测速结果表格 + 共享声明

### 组件二：本地程序 - 后端 (8个任务)
- [ ] 6. 配置文件解析模块
- [ ] 7. 域名测速引擎 (改造原项目)
- [ ] 8. IP 地理位置识别
- [ ] 9. 结果上报模块 (POST 到 Worker)
- [ ] 10. CF DNS 更新模块
- [ ] 11. 定时任务调度器
- [ ] 12. 本地 HTTP API 服务
- [ ] 13. main.go 整合入口

### 组件二：本地程序 - 前端 (3个任务)
- [ ] 14. 本地 Web 页面 (测速结果 + DNS 记录)
- [ ] 15. 域名管理界面
- [ ] 16. 共享计划声明

### 部署 (3个任务)
- [ ] 17. Dockerfile
- [ ] 18. docker-compose.yml
- [ ] 19. 测试完整流程

---

## 技术选型

| 组件 | 技术方案 |
|------|----------|
| Worker | Cloudflare Workers (TypeScript) |
| Worker 页面 | 内嵌 HTML + ECharts CDN |
| 本地程序 | Go 1.18+ |
| 本地页面 | 原生 HTML/JS |
| 数据库 | Cloudflare D1 (Worker 绑定) |
| IP 识别 | ip-api.com |
| 配置 | YAML |
| 定时任务 | robfig/cron/v3 |
| 容器化 | Docker |

---

## 安全说明

- CF API Token 加密内置在 Go 程序中，用户无法直接获取
- 用户配置文件中只需填写 zone_id 和 record_name
- 所有测速结果通过 Worker API 写入共享 D1 数据库
- Worker 通过 wrangler.toml 绑定 D1，不暴露 API Token

const CORS_HEADERS = {
  'Access-Control-Allow-Origin': '*',
  'Access-Control-Allow-Methods': 'GET, POST, OPTIONS',
  'Access-Control-Allow-Headers': 'Content-Type',
};

const THREE_DAYS_SECONDS = 3 * 24 * 60 * 60;

const INIT_SQL = [
  `CREATE TABLE IF NOT EXISTS speed_results (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    province TEXT NOT NULL,
    isp TEXT NOT NULL,
    mode TEXT NOT NULL DEFAULT 'ip',
    domain TEXT NOT NULL,
    domain_name TEXT,
    download_speed REAL,
    latency REAL,
    ip_address TEXT,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP
  )`,
  `CREATE TABLE IF NOT EXISTS best_results (
    province TEXT NOT NULL,
    isp TEXT NOT NULL,
    mode TEXT NOT NULL DEFAULT 'ip',
    domain TEXT NOT NULL,
    domain_name TEXT,
    download_speed REAL,
    latency REAL,
    ip_address TEXT DEFAULT '',
    updated_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (province, isp, mode)
  )`,
  `CREATE INDEX IF NOT EXISTS idx_speed_results_province_isp ON speed_results(province, isp)`,
  `CREATE INDEX IF NOT EXISTS idx_speed_results_created_at ON speed_results(created_at)`,
  `CREATE INDEX IF NOT EXISTS idx_best_results_updated_at ON best_results(updated_at)`,
];

function getBeijingTime() {
  const now = new Date();
  const utc = now.getTime() + now.getTimezoneOffset() * 60000;
  return new Date(utc + 8 * 3600000);
}

async function cleanupOldData(db) {
  await db.prepare(
    "DELETE FROM speed_results WHERE created_at < datetime('now', '-3 days')"
  ).run();
  await db.prepare(
    "DELETE FROM best_results WHERE updated_at < datetime('now', '-3 days')"
  ).run();
}

async function ensureDB(db) {
  for (const sql of INIT_SQL) {
    await db.prepare(sql).run();
  }
  await cleanupOldData(db);
}

function jsonResponse(data, status = 200) {
  return new Response(JSON.stringify(data), {
    status,
    headers: { 'Content-Type': 'application/json', ...CORS_HEADERS },
  });
}

async function handleGetBestResults(db, mode) {
  await ensureDB(db);
  const { results } = await db.prepare(
    "SELECT * FROM best_results WHERE mode = ? AND updated_at >= datetime('now', '-3 days') ORDER BY province, isp"
  ).bind(mode || 'ip').all();
  return jsonResponse({ success: true, data: results });
}

async function handleSubmitResult(db, body) {
  await ensureDB(db);
  const { province, isp, mode, domain, domain_name, download_speed, latency, ip_address } = body;
  const m = mode || 'ip';

  await db.prepare(
    'INSERT INTO speed_results (province, isp, mode, domain, domain_name, download_speed, latency, ip_address) VALUES (?, ?, ?, ?, ?, ?, ?, ?)'
  ).bind(province, isp, m, domain, domain_name || '', download_speed || 0, latency || 0, ip_address || '').run();

  const existing = await db.prepare(
    'SELECT * FROM best_results WHERE province = ? AND isp = ? AND mode = ?'
  ).bind(province, isp, m).first();

  if (!existing || (download_speed && download_speed > existing.download_speed)) {
    await db.prepare(
      "INSERT OR REPLACE INTO best_results (province, isp, mode, domain, domain_name, download_speed, latency, ip_address, updated_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?, datetime('now'))"
    ).bind(province, isp, m, domain, domain_name || '', download_speed || 0, latency || 0, ip_address || '').run();
  }

  return jsonResponse({ success: true });
}

async function handleGetStats(db) {
  await ensureDB(db);
  const { results: totalResults } = await db.prepare(
    "SELECT COUNT(*) as cnt FROM speed_results WHERE created_at >= datetime('now', '-3 days')"
  ).all();
  const { results: provinceCount } = await db.prepare(
    "SELECT COUNT(DISTINCT province) as cnt FROM best_results WHERE updated_at >= datetime('now', '-3 days')"
  ).all();
  const { results: bestCount } = await db.prepare(
    "SELECT COUNT(*) as cnt FROM best_results WHERE updated_at >= datetime('now', '-3 days')"
  ).all();
  return jsonResponse({
    success: true,
    data: {
      total_results: totalResults[0]?.cnt || 0,
      provinces_covered: provinceCount[0]?.cnt || 0,
      best_records: bestCount[0]?.cnt || 0,
    },
  });
}

function getHTML() {
  return `<!DOCTYPE html>
<html lang="zh-CN">
<head>
<meta charset="UTF-8">
<meta name="viewport" content="width=device-width, initial-scale=1.0">
<title>CFST 共享测速计划</title>
<style>
@import url('https://fonts.googleapis.com/css2?family=Inter:wght@300;400;500;600;700;800&display=swap');
*{margin:0;padding:0;box-sizing:border-box}
:root{
  --bg-base:#06080f;--bg-surface:#0c1019;--bg-elevated:#111623;
  --bg-card:rgba(14,20,35,0.75);--border:rgba(56,189,248,0.08);
  --text-primary:#e8edf5;--text-secondary:#8b95a8;--text-muted:#505a6e;
  --cyan:#38bdf8;--cyan-deep:#0ea5e9;--green:#34d399;--amber:#fbbf24;
  --red:#f87171;--purple:#a78bfa;--rose:#fb7185;
  --radius:14px;--radius-sm:10px;--radius-xs:6px;
}
body{
  font-family:'Inter',-apple-system,BlinkMacSystemFont,sans-serif;
  background:var(--bg-base);color:var(--text-primary);
  min-height:100vh;-webkit-font-smoothing:antialiased;
}
body::before{
  content:'';position:fixed;inset:0;z-index:0;pointer-events:none;
  background:
    radial-gradient(ellipse 90% 50% at 50% -15%,rgba(56,189,248,0.06),transparent 70%),
    radial-gradient(ellipse 50% 40% at 85% 55%,rgba(124,58,237,0.04),transparent 70%);
}
body::after{
  content:'';position:fixed;inset:0;z-index:0;pointer-events:none;
  background-image:radial-gradient(rgba(56,189,248,0.03) 1px,transparent 1px);
  background-size:24px 24px;
}
.app{max-width:1100px;margin:0 auto;padding:20px 16px 40px;position:relative;z-index:1}
.header{text-align:center;padding:32px 0 24px}
.header h1{
  font-size:28px;font-weight:800;letter-spacing:-0.5px;
  background:linear-gradient(135deg,var(--cyan) 0%,var(--purple) 50%,var(--green) 100%);
  -webkit-background-clip:text;-webkit-text-fill-color:transparent;background-clip:text;
}
.header .subtitle{font-size:12px;color:var(--text-muted);letter-spacing:2px;text-transform:uppercase;font-weight:500;margin-top:4px}
.divider{width:48px;height:2px;margin:12px auto 0;background:linear-gradient(90deg,transparent,var(--cyan),var(--purple),transparent);border-radius:1px}
.card{
  background:var(--bg-card);backdrop-filter:blur(16px);-webkit-backdrop-filter:blur(16px);
  border:1px solid var(--border);border-radius:var(--radius);
  padding:20px 22px;margin-bottom:14px;
  box-shadow:0 4px 16px rgba(0,0,0,0.4),0 0 0 1px rgba(56,189,248,0.04);
}
.card-title{
  font-size:11px;font-weight:700;color:var(--text-muted);
  text-transform:uppercase;letter-spacing:1.5px;margin-bottom:16px;
  display:flex;align-items:center;gap:8px;
}
.stats-grid{display:grid;grid-template-columns:repeat(3,1fr);gap:1px;background:var(--border);border-radius:var(--radius-sm);overflow:hidden}
.stat-cell{background:var(--bg-elevated);padding:16px;display:flex;flex-direction:column;gap:4px;text-align:center}
.stat-cell .label{font-size:10px;color:var(--text-muted);text-transform:uppercase;letter-spacing:0.8px;font-weight:600}
.stat-cell .val{font-size:22px;font-weight:800;color:var(--text-primary);font-variant-numeric:tabular-nums}
.table-wrap{overflow-x:auto;margin:-4px}
table{width:100%;border-collapse:separate;border-spacing:0;font-size:12px;table-layout:fixed}
thead th{
  background:rgba(56,189,248,0.04);color:var(--text-muted);padding:10px 8px;
  text-align:left;font-size:10px;font-weight:700;text-transform:uppercase;
  letter-spacing:0.8px;border-bottom:1px solid var(--border);white-space:nowrap;
}
thead th:nth-child(1){width:60px}
thead th:nth-child(2),thead th:nth-child(3),thead th:nth-child(4){width:1fr}
tbody td{padding:9px 8px;border-bottom:1px solid rgba(148,163,184,0.04);vertical-align:top}
tbody tr:hover td{background:rgba(56,189,248,0.03)}
.isp-telecom{color:#ef4444;font-weight:600}
.isp-unicom{color:#3b82f6;font-weight:600}
.isp-mobile{color:#22c55e;font-weight:600}
.isp-other{color:#a78bfa;font-weight:600}
.ip-val{font-family:'JetBrains Mono',monospace;font-size:10px;color:var(--cyan);opacity:0.7;margin-top:1px}
.speed-val{font-size:10px;color:var(--text-muted);margin-top:1px}
.tab-bar{display:flex;gap:4px;margin-bottom:16px;flex-wrap:wrap}
.tab-btn{padding:6px 14px;border:1px solid var(--border);border-radius:var(--radius-xs);background:transparent;color:var(--text-muted);font-size:11px;font-weight:600;cursor:pointer;transition:all 0.2s}
.tab-btn:hover{border-color:var(--cyan);color:var(--text-secondary)}
.tab-btn.active{background:rgba(56,189,248,0.1);border-color:var(--cyan);color:var(--cyan)}
.tab-panel{display:none}
.tab-panel.active{display:block}
.mono-sm{font-family:'JetBrains Mono',monospace;font-size:10.5px}
.isp-tag{display:inline-block;padding:1px 6px;border-radius:3px;font-size:9px;font-weight:700;text-transform:uppercase}
.isp-tag-telecom{background:rgba(239,68,68,0.12);color:#ef4444}
.isp-tag-unicom{background:rgba(59,130,246,0.12);color:#3b82f6}
.isp-tag-mobile{background:rgba(34,197,94,0.12);color:#22c55e}
.isp-tag-other{background:rgba(167,139,250,0.12);color:#a78bfa}
.mode-badge{display:inline-block;padding:2px 8px;border-radius:4px;font-size:10px;font-weight:700;letter-spacing:0.5px}
.mode-badge-ip{background:rgba(56,189,248,0.1);color:var(--cyan);border:1px solid rgba(56,189,248,0.2)}
.mode-badge-domain{background:rgba(167,139,250,0.1);color:var(--purple);border:1px solid rgba(167,139,250,0.2)}
.section-label{font-size:12px;font-weight:700;margin:14px 0 10px;display:flex;align-items:center;gap:8px}
.disclaimer{
  background:linear-gradient(135deg,rgba(251,191,36,0.06),rgba(251,191,36,0.02));
  border:1px solid rgba(251,191,36,0.15);border-radius:var(--radius-sm);
  padding:18px 22px;text-align:center;
}
.disclaimer .title{font-size:13px;font-weight:700;color:var(--amber);margin-bottom:6px}
.disclaimer p{font-size:12px;color:var(--text-secondary);line-height:1.7}
.disclaimer .muted{color:var(--text-muted);font-size:11px;margin-top:6px}
.empty{text-align:center;padding:40px 16px;color:var(--text-muted);font-size:13px}
.domain-link{color:var(--cyan);text-decoration:none;word-break:break-all}
.domain-link:hover{text-decoration:underline}
@media(max-width:640px){
  .app{padding:12px 10px 32px}
  .header h1{font-size:20px}
  .stats-grid{grid-template-columns:1fr}
  table{font-size:11px}
  thead th,tbody td{padding:7px 6px}
}
</style>
</head>
<body>
<div class="app">
<div class="header">
  <h1>CFST 共享测速计划</h1>
  <div class="subtitle">Cloudflare IP Optimize &amp; Share</div>
  <div class="divider"></div>
</div>
<div class="card">
  <div class="card-title">数据概览</div>
  <div class="stats-grid">
    <div class="stat-cell"><span class="label">总测速次数</span><span class="val" id="statTotal">-</span></div>
    <div class="stat-cell"><span class="label">覆盖省份</span><span class="val" id="statProvinces">-</span></div>
    <div class="stat-cell"><span class="label">最优记录</span><span class="val" id="statBest">-</span></div>
  </div>
</div>
<div class="card">
  <div class="card-title">测速数据</div>
  <div class="tab-bar">
    <button class="tab-btn active" onclick="switchTab(this,'ipRec')">IP记录</button>
    <button class="tab-btn" onclick="switchTab(this,'ipv6Rec')">IPV6记录</button>
    <button class="tab-btn" onclick="switchTab(this,'domainRec')">域名记录</button>
    <button class="tab-btn" onclick="switchTab(this,'domainV6Rec')">域名V6记录</button>
  </div>
  <div id="ipRecPanel" class="tab-panel active">
    <div id="ipRecContent"><div class="empty">加载中...</div></div>
  </div>
  <div id="ipv6RecPanel" class="tab-panel">
    <div id="ipv6RecContent"><div class="empty">加载中...</div></div>
  </div>
  <div id="domainRecPanel" class="tab-panel">
    <div id="domainRecContent"><div class="empty">加载中...</div></div>
  </div>
  <div id="domainV6RecPanel" class="tab-panel">
    <div id="domainV6RecContent"><div class="empty">加载中...</div></div>
  </div>
</div>
<div class="disclaimer">
  <div class="title">&#9888; 共享计划声明</div>
  <p>本项目为共享测速计划，</p>
  <p>每个地区+运营商+模式只显示<strong>最近三天最快</strong>的一条记录。</p>
  <p>超过三天的旧数据将自动清理。</p>
</div>
</div>
<script>
const enToCn={
  'Anhui':'安徽','Beijing':'北京','Chongqing':'重庆','Fujian':'福建',
  'Gansu':'甘肃','Guangdong':'广东','Guangxi':'广西','Guizhou':'贵州',
  'Hainan':'海南','Hebei':'河北','Heilongjiang':'黑龙江','Henan':'河南',
  'Hubei':'湖北','Hunan':'湖南','Inner Mongolia':'内蒙古','Jiangsu':'江苏',
  'Jiangxi':'江西','Jilin':'吉林','Liaoning':'辽宁','Ningxia':'宁夏',
  'Qinghai':'青海','Shaanxi':'陕西','Shandong':'山东','Shanghai':'上海',
  'Shanxi':'山西','Sichuan':'四川','Tianjin':'天津','Tibet':'西藏',
  'Xinjiang':'新疆','Yunnan':'云南','Zhejiang':'浙江',
  'Hong Kong':'香港','Macau':'澳门','Taiwan':'台湾',
  'Nei Mongol':'内蒙古','Xizang':'西藏'
};
const ispNames={telecom:'电信',unicom:'联通',mobile:'移动',other:'其他'};
function translateProvince(name){
  if(!name||name==='未知')return'';
  if(enToCn[name])return enToCn[name];
  for(const en in enToCn){if(en.toLowerCase()===name.toLowerCase())return enToCn[en]}
  return name;
}
let ipRecData=[];
let ipv6RecData=[];
let domainRecData=[];
let domainV6RecData=[];
function switchTab(btn,tab){
  document.querySelectorAll('.tab-btn').forEach(b=>b.classList.remove('active'));
  btn.classList.add('active');
  document.getElementById('ipRecPanel').classList.toggle('active',tab==='ipRec');
  document.getElementById('ipv6RecPanel').classList.toggle('active',tab==='ipv6Rec');
  document.getElementById('domainRecPanel').classList.toggle('active',tab==='domainRec');
  document.getElementById('domainV6RecPanel').classList.toggle('active',tab==='domainV6Rec');
}
async function fetchStats(){
  try{
    const res=await fetch('/api/stats');
    const json=await res.json();
    if(json.success){
      document.getElementById('statTotal').textContent=json.data.total_results;
      document.getElementById('statProvinces').textContent=json.data.provinces_covered;
      document.getElementById('statBest').textContent=json.data.best_records;
    }
  }catch(e){console.error(e)}
}
async function fetchRecords(){
  try{
    const[r1,r2,r3,r4]=await Promise.all([
      fetch('/api/results/best?mode=ip').then(r=>r.json()),
      fetch('/api/results/best?mode=ipv6').then(r=>r.json()),
      fetch('/api/results/best?mode=domain').then(r=>r.json()),
      fetch('/api/results/best?mode=domain_v6').then(r=>r.json())
    ]);
    if(r1.success)ipRecData=r1.data;
    if(r2.success)ipv6RecData=r2.data;
    if(r3.success)domainRecData=r3.data;
    if(r4.success)domainV6RecData=r4.data;
    renderRecordTable(ipRecData,'ipRecContent');
    renderRecordTable(ipv6RecData,'ipv6RecContent');
    renderRecordTable(domainRecData,'domainRecContent');
    renderRecordTable(domainV6RecData,'domainV6RecContent');
  }catch(e){console.error(e)}
}
function renderRecordTable(data,elementId){
  const el=document.getElementById(elementId);
  if(!data||!data.length){el.innerHTML='<div class="empty">暂无数据</div>';return}
  const byProvince={};
  data.forEach(r=>{
    let p=translateProvince(r.province)||r.province;
    if(!p||p==='未知')return;
    if(!byProvince[p])byProvince[p]={};
    byProvince[p][r.isp]=r;
  });
  const provinces=Object.keys(byProvince).sort((a,b)=>a.localeCompare(b,'zh-CN'));
  let html='<div class="table-wrap"><table><thead><tr><th>省份</th><th>电信</th><th>联通</th><th>移动</th></tr></thead><tbody>';
  provinces.forEach(p=>{
    const d=byProvince[p];
    const getCell=(isp)=>{
      const r=d[isp];
      if(!r)return'<span style="color:var(--text-muted)">-</span>';
      let cell='<a href="https://'+r.domain+'" target="_blank" class="domain-link">'+r.domain+'</a>';
      if(r.ip_address)cell+='<div class="ip-val">'+r.ip_address+'</div>';
      cell+='<div class="speed-val">'+r.download_speed.toFixed(2)+' MB/s &middot; '+r.latency.toFixed(0)+'ms</div>';
      return cell;
    };
    html+='<tr><td><strong>'+p+'</strong></td><td>'+getCell('telecom')+'</td><td>'+getCell('unicom')+'</td><td>'+getCell('mobile')+'</td></tr>';
  });
  html+='</tbody></table></div>';
  el.innerHTML=html;
}
fetchStats();
fetchRecords();
</script>
</body>
</html>`;
}

export default {
  async fetch(request, env) {
    const url = new URL(request.url);
    const { pathname } = url;
    const { method } = request;

    if (method === 'OPTIONS') return new Response(null, { headers: CORS_HEADERS });

    try {
      if (pathname === '/' || pathname === '/index.html') {
        return new Response(getHTML(), { headers: { 'Content-Type': 'text/html; charset=utf-8' } });
      }

      if (pathname === '/api/stats' && method === 'GET') {
        return handleGetStats(env.DB);
      }

      if (pathname === '/api/results/best' && method === 'GET') {
        const mode = url.searchParams.get('mode') || 'ip';
        return handleGetBestResults(env.DB, mode);
      }

      if (pathname === '/api/results' && method === 'POST') {
        const body = await request.json();
        return handleSubmitResult(env.DB, body);
      }

      return jsonResponse({ success: false, error: 'Not found' }, 404);
    } catch (e) {
      const msg = e instanceof Error ? e.message : String(e);
      return jsonResponse({ success: false, error: msg }, 500);
    }
  },
};

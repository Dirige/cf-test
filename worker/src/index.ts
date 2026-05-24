interface Env {
  DB: D1Database;
}

const CORS_HEADERS: Record<string, string> = {
  'Access-Control-Allow-Origin': '*',
  'Access-Control-Allow-Methods': 'GET, POST, PUT, DELETE, OPTIONS',
  'Access-Control-Allow-Headers': 'Content-Type',
};

const INIT_SQL: string[] = [
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
  `CREATE TABLE IF NOT EXISTS domains (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    name TEXT,
    domain TEXT UNIQUE NOT NULL,
    is_builtin BOOLEAN DEFAULT 0
  )`,
  `CREATE INDEX IF NOT EXISTS idx_speed_results_province_isp ON speed_results(province, isp)`,
  `CREATE INDEX IF NOT EXISTS idx_speed_results_created_at ON speed_results(created_at)`,
];

const SEED_DOMAINS: [string, string][] = [
  ['CF优选-090227', 'youxuan.cf.090227.xyz'],
  ['Shopify官方', 'www.shopify.com'],
  ['Mingyu优选', 'bestcf.030101.xyz'],
  ['育碧商店', 'store.ubi.com'],
  ['WeTest优选', 'cf.cloudflare.182682.xyz'],
  ['MIYU优选', 'saas.sin.fan'],
  ['NexusMods', 'staticdelivery.nexusmods.com'],
  ['乌克兰外交部', 'mfa.gov.ua'],
  ['NB优选', 'cf.cf.cnae.top'],
  ['Visa官方', 'www.visa.cn'],
  ['秋名山优选', 'cf.877774.xyz'],
  ['无名氏维护域名', 'cf.tencentapp.cn'],
];

async function ensureDB(db: D1Database): Promise<void> {
  for (const sql of INIT_SQL) {
    await db.prepare(sql).run();
  }
  const { count } = await db.prepare('SELECT COUNT(*) as count FROM domains').first() as { count: number };
  if (count === 0) {
    for (const [name, domain] of SEED_DOMAINS) {
      await db.prepare('INSERT INTO domains (name, domain, is_builtin) VALUES (?, ?, 1)').bind(name, domain).run();
    }
  }
}

function jsonResponse(data: unknown, status = 200): Response {
  return new Response(JSON.stringify(data), {
    status,
    headers: { 'Content-Type': 'application/json', ...CORS_HEADERS },
  });
}

async function handleGetBestResults(db: D1Database, mode: string): Promise<Response> {
  await ensureDB(db);
  const { results } = await db.prepare('SELECT * FROM best_results WHERE mode = ? ORDER BY province, isp').bind(mode || 'ip').all();
  return jsonResponse({ success: true, data: results });
}

async function handleGetRecentResults(db: D1Database): Promise<Response> {
  await ensureDB(db);
  const { results } = await db.prepare('SELECT * FROM speed_results ORDER BY created_at DESC LIMIT 100').all();
  return jsonResponse({ success: true, data: results });
}

async function handleSubmitResult(db: D1Database, body: Record<string, unknown>): Promise<Response> {
  await ensureDB(db);
  const { province, isp, mode, domain, domain_name, download_speed, latency, ip_address } = body;
  const m = (mode as string) || 'ip';
  await db.prepare(
    'INSERT INTO speed_results (province, isp, mode, domain, domain_name, download_speed, latency, ip_address) VALUES (?, ?, ?, ?, ?, ?, ?, ?)'
  ).bind(province as string, isp as string, m, domain as string, (domain_name as string) || '', (download_speed as number) || 0, (latency as number) || 0, (ip_address as string) || '').run();
  const existing = await db.prepare(
    'SELECT * FROM best_results WHERE province = ? AND isp = ? AND mode = ?'
  ).bind(province as string, isp as string, m).first();
  if (!existing || (download_speed && (download_speed as number) > (existing as Record<string, number>).download_speed)) {
    await db.prepare(
      "INSERT OR REPLACE INTO best_results (province, isp, mode, domain, domain_name, download_speed, latency, ip_address, updated_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?, datetime('now'))"
    ).bind(province as string, isp as string, m, domain as string, (domain_name as string) || '', (download_speed as number) || 0, (latency as number) || 0, (ip_address as string) || '').run();
  }
  return jsonResponse({ success: true });
}

async function handleGetDomains(db: D1Database): Promise<Response> {
  await ensureDB(db);
  const { results } = await db.prepare('SELECT * FROM domains ORDER BY is_builtin DESC, id').all();
  return jsonResponse({ success: true, data: results });
}

async function handleAddDomain(db: D1Database, body: { name: string; domain: string }): Promise<Response> {
  await ensureDB(db);
  const { name, domain } = body;
  if (!domain) return jsonResponse({ success: false, error: 'domain is required' }, 400);
  try {
    await db.prepare('INSERT INTO domains (name, domain, is_builtin) VALUES (?, ?, 0)').bind(name || '', domain).run();
    return jsonResponse({ success: true });
  } catch (e: unknown) {
    const msg = e instanceof Error ? e.message : String(e);
    if (msg.includes('UNIQUE')) return jsonResponse({ success: false, error: 'Domain already exists' }, 409);
    throw e;
  }
}

async function handleDeleteDomain(db: D1Database, domain: string): Promise<Response> {
  await ensureDB(db);
  if (!domain) return jsonResponse({ success: false, error: 'domain is required' }, 400);
  await db.prepare('DELETE FROM domains WHERE domain = ?').bind(domain).run();
  return jsonResponse({ success: true });
}

async function handleUpdateDomain(db: D1Database, domain: string, body: { name?: string; new_domain?: string }): Promise<Response> {
  await ensureDB(db);
  if (!domain) return jsonResponse({ success: false, error: 'domain is required' }, 400);
  const { name, new_domain } = body;
  if (!name && !new_domain) return jsonResponse({ success: false, error: 'name or new_domain is required' }, 400);
  if (new_domain && new_domain !== domain) {
    const existing = await db.prepare('SELECT id FROM domains WHERE domain = ?').bind(new_domain).first();
    if (existing) return jsonResponse({ success: false, error: '目标域名已存在' }, 400);
  }
  const sets: string[] = [];
  const params: string[] = [];
  if (name) { sets.push('name = ?'); params.push(name); }
  if (new_domain) { sets.push('domain = ?'); params.push(new_domain); }
  params.push(domain);
  await db.prepare(`UPDATE domains SET ${sets.join(', ')} WHERE domain = ?`).bind(...params).run();
  return jsonResponse({ success: true });
}

async function handleGetStats(db: D1Database): Promise<Response> {
  await ensureDB(db);
  const { results: totalResults } = await db.prepare('SELECT COUNT(*) as cnt FROM speed_results').all();
  const { results: provinceCount } = await db.prepare('SELECT COUNT(DISTINCT province) as cnt FROM best_results').all();
  const { results: bestCount } = await db.prepare('SELECT COUNT(*) as cnt FROM best_results').all();
  return jsonResponse({
    success: true,
    data: {
      total_results: (totalResults[0] as Record<string, number>)?.cnt || 0,
      provinces_covered: (provinceCount[0] as Record<string, number>)?.cnt || 0,
      best_records: (bestCount[0] as Record<string, number>)?.cnt || 0,
    },
  });
}

function getHTML(): string {
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
    <button class="tab-btn" onclick="switchTab(this,'domainRec')">域名记录</button>
  </div>
  <div id="ipRecPanel" class="tab-panel active">
    <div id="ipRecContent"><div class="empty">加载中...</div></div>
  </div>
  <div id="domainRecPanel" class="tab-panel">
    <div id="domainRecContent"><div class="empty">加载中...</div></div>
  </div>
</div>
<div class="disclaimer">
  <div class="title">&#9888; 共享计划声明</div>
  <p>本项目为共享测速计划，所有用户的测速结果将同步至共享数据库。</p>
  <p>您的测速数据将被汇总展示，帮助其他用户选择最优域名。</p>
  <p class="muted">如不同意数据共享，请删除本程序。</p>
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
const modeNames={ip:'IP测速',domain:'域名测速'};
function translateProvince(name){
  if(!name||name==='未知')return'';
  if(enToCn[name])return enToCn[name];
  for(const en in enToCn){if(en.toLowerCase()===name.toLowerCase())return enToCn[en]}
  return name;
}
let ipRecData=[];
let domainRecData=[];
function switchTab(btn,tab){
  document.querySelectorAll('.tab-btn').forEach(b=>b.classList.remove('active'));
  btn.classList.add('active');
  document.getElementById('ipRecPanel').classList.toggle('active',tab==='ipRec');
  document.getElementById('domainRecPanel').classList.toggle('active',tab==='domainRec');
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
    const[r1,r2]=await Promise.all([
      fetch('/api/results/best?mode=ip').then(r=>r.json()),
      fetch('/api/results/best?mode=domain').then(r=>r.json())
    ]);
    if(r1.success)ipRecData=r1.data;
    if(r2.success)domainRecData=r2.data;
    renderRecordTable(ipRecData,'ipRecContent');
    renderRecordTable(domainRecData,'domainRecContent');
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
      let cell='<span class="isp-'+isp+'">'+(r.domain_name||r.domain)+'</span>';
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
  async fetch(request: Request, env: Env): Promise<Response> {
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

      if (pathname === '/api/results/recent' && method === 'GET') {
        return handleGetRecentResults(env.DB);
      }

      if (pathname === '/api/results' && method === 'POST') {
        const body = await request.json() as Record<string, unknown>;
        return handleSubmitResult(env.DB, body);
      }

      if (pathname === '/api/domains' && method === 'GET') {
        return handleGetDomains(env.DB);
      }

      if (pathname === '/api/domains' && method === 'POST') {
        const body = await request.json() as { name: string; domain: string };
        return handleAddDomain(env.DB, body);
      }

      if (pathname.startsWith('/api/domains/') && method === 'DELETE') {
        const domain = decodeURIComponent(pathname.slice('/api/domains/'.length));
        return handleDeleteDomain(env.DB, domain);
      }

      if (pathname.startsWith('/api/domains/') && method === 'PUT') {
        const domain = decodeURIComponent(pathname.slice('/api/domains/'.length));
        const putBody = await request.json() as { name?: string; new_domain?: string };
        return handleUpdateDomain(env.DB, domain, putBody);
      }

      return jsonResponse({ success: false, error: 'Not found' }, 404);
    } catch (e: unknown) {
      const msg = e instanceof Error ? e.message : String(e);
      return jsonResponse({ success: false, error: msg }, 500);
    }
  },
};

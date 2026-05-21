interface Env {
  DB: D1Database;
}

interface SpeedResult {
  province: string;
  isp: string;
  domain: string;
  domain_name?: string;
  download_speed?: number;
  latency?: number;
  ip_address?: string;
}

interface BestResult {
  province: string;
  isp: string;
  domain: string;
  domain_name: string;
  download_speed: number;
  latency: number;
  updated_at: string;
}

interface DomainItem {
  id: number;
  name: string;
  domain: string;
  is_builtin: number;
}

const CORS_HEADERS = {
  'Access-Control-Allow-Origin': '*',
  'Access-Control-Allow-Methods': 'GET, POST, DELETE, OPTIONS',
  'Access-Control-Allow-Headers': 'Content-Type',
};

function jsonResponse(data: unknown, status = 200): Response {
  return new Response(JSON.stringify(data), {
    status,
    headers: { 'Content-Type': 'application/json', ...CORS_HEADERS },
  });
}

async function handleGetBestResults(db: D1Database): Promise<Response> {
  const { results } = await db.prepare('SELECT * FROM best_results ORDER BY province, isp').all<BestResult>();
  return jsonResponse({ success: true, data: results });
}

async function handleSubmitResult(db: D1Database, body: SpeedResult): Promise<Response> {
  const { province, isp, domain, domain_name, download_speed, latency, ip_address } = body;

  await db.prepare(
    'INSERT INTO speed_results (province, isp, domain, domain_name, download_speed, latency, ip_address) VALUES (?, ?, ?, ?, ?, ?, ?)'
  ).bind(province, isp, domain, domain_name || '', download_speed || 0, latency || 0, ip_address || '').run();

  const existing = await db.prepare(
    'SELECT * FROM best_results WHERE province = ? AND isp = ?'
  ).bind(province, isp).first<BestResult>();

  if (!existing || (download_speed && download_speed > existing.download_speed)) {
    await db.prepare(
      'INSERT OR REPLACE INTO best_results (province, isp, domain, domain_name, download_speed, latency, updated_at) VALUES (?, ?, ?, ?, ?, ?, datetime(\'now\'))'
    ).bind(province, isp, domain, domain_name || '', download_speed || 0, latency || 0).run();
  }

  return jsonResponse({ success: true });
}

async function handleGetDomains(db: D1Database): Promise<Response> {
  const { results } = await db.prepare('SELECT * FROM domains ORDER BY is_builtin DESC, id').all<DomainItem>();
  return jsonResponse({ success: true, data: results });
}

async function handleAddDomain(db: D1Database, body: { name: string; domain: string }): Promise<Response> {
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
  if (!domain) return jsonResponse({ success: false, error: 'domain is required' }, 400);
  await db.prepare('DELETE FROM domains WHERE domain = ? AND is_builtin = 0').bind(domain).run();
  return jsonResponse({ success: true });
}

function getHTML(): string {
  return `<!DOCTYPE html>
<html lang="zh-CN">
<head>
<meta charset="UTF-8">
<meta name="viewport" content="width=device-width, initial-scale=1.0">
<title>Cloudflare 优选域名测速 - 共享计划</title>
<script src="https://cdn.jsdelivr.net/npm/echarts@5.5.0/dist/echarts.min.js"></script>
<style>
*{margin:0;padding:0;box-sizing:border-box}
body{font-family:-apple-system,BlinkMacSystemFont,'Segoe UI',Roboto,sans-serif;background:#0f172a;color:#e2e8f0;min-height:100vh}
.container{max-width:1200px;margin:0 auto;padding:20px}
h1{text-align:center;font-size:28px;margin:20px 0;color:#38bdf8}
.map-container{background:#1e293b;border-radius:12px;padding:20px;margin-bottom:20px}
#chinaMap{width:100%;height:500px}
.table-container{background:#1e293b;border-radius:12px;padding:20px;margin-bottom:20px;overflow-x:auto}
table{width:100%;border-collapse:collapse}
th{background:#334155;color:#38bdf8;padding:12px;text-align:left;font-weight:600;position:sticky;top:0}
td{padding:10px 12px;border-bottom:1px solid #334155}
tr:hover{background:#334155}
.isp-telecom{color:#ef4444}
.isp-unicom{color:#3b82f6}
.isp-mobile{color:#22c55e}
.disclaimer{background:#1e293b;border:2px solid #f59e0b;border-radius:12px;padding:24px;margin-top:20px;text-align:center}
.disclaimer p{font-size:18px;font-weight:bold;color:#f59e0b;line-height:1.8}
.loading{text-align:center;padding:40px;color:#64748b}
</style>
</head>
<body>
<div class="container">
<h1>🌐 Cloudflare 优选域名测速 - 共享计划</h1>
<div class="map-container"><div id="chinaMap"></div></div>
<div class="table-container">
<table>
<thead><tr><th>省份</th><th>电信</th><th>联通</th><th>移动</th></tr></thead>
<tbody id="resultTable"><tr><td colspan="4" class="loading">加载中...</td></tr></tbody>
</table>
</div>
<div class="disclaimer">
<p>⚠️ 共享计划声明</p>
<p>本项目为共享测速计划，所有用户的测速结果将同步至共享数据库。</p>
<p>您的测速数据将被汇总展示，帮助其他用户选择最优域名。</p>
<p style="font-size:16px;color:#94a3b8;margin-top:10px">如不同意数据共享，请删除本程序。</p>
</div>
</div>
<script>
const provinceMap={
'安徽':'anhui','澳门':'aomen','北京':'beijing','重庆':'chongqing','福建':'fujian',
'甘肃':'gansu','广东':'guangdong','广西':'guangxi','贵州':'guizhou','海南':'hainan',
'河北':'hebei','河南':'henan','黑龙江':'heilongjiang','湖北':'hubei','湖南':'hunan',
'吉林':'jilin','江苏':'jiangsu','江西':'jiangxi','辽宁':'liaoning','内蒙古':'neimenggu',
'宁夏':'ningxia','青海':'qinghai','山东':'shandong','山西':'shanxi1','陕西':'shanxi',
'上海':'shanghai','四川':'sichuan','台湾':'taiwan','天津':'tianjin','西藏':'xizang',
'香港':'xianggang','新疆':'xinjiang','云南':'yunnan','浙江':'zhejiang'
};
const provinces=Object.keys(provinceMap).sort((a,b)=>a.localeCompare(b,'zh-CN'));
let bestData={};

async function fetchData(){
try{
const res=await fetch('/api/results/best');
const json=await res.json();
if(json.success){
bestData={};
json.data.forEach(r=>{
if(!bestData[r.province])bestData[r.province]={};
bestData[r.province][r.isp]=r;
});
renderTable();
renderMap();
}
}catch(e){console.error(e)}
}

function renderTable(){
const tbody=document.getElementById('resultTable');
tbody.innerHTML='';
provinces.forEach(p=>{
const d=bestData[p]||{};
const tr=document.createElement('tr');
const getCell=(isp)=>{
const r=d[isp];
if(!r)return'<span style="color:#64748b">-</span>';
return'<span class="isp-'+isp+'">'+r.domain_name+'</span><br><small>'+r.download_speed.toFixed(2)+' MB/s</small>';
};
tr.innerHTML='<td><strong>'+p+'</strong></td><td>'+getCell('telecom')+'</td><td>'+getCell('unicom')+'</td><td>'+getCell('mobile')+'</td>';
tbody.appendChild(tr);
});
}

function renderMap(){
const chart=echarts.init(document.getElementById('chinaMap'));
const tooltipData=provinces.map(p=>{
const d=bestData[p]||{};
const g=(isp)=>{
const r=d[isp];
return r?r.domain_name+' ('+r.download_speed.toFixed(1)+'MB/s)':'-';
};
return{name:p,value:0,telecom:g('telecom'),unicom:g('unicom'),mobile:g('mobile')};
});
chart.setOption({
tooltip:{trigger:'item',formatter:function(p){
const d=p.data||{};
return'<strong>'+p.name+'</strong><br/>'+'<span style="color:#ef4444">电信</span>: '+(d.telecom||'-')+'<br/>'+'<span style="color:#3b82f6">联通</span>: '+(d.unicom||'-')+'<br/>'+'<span style="color:#22c55e">移动</span>: '+(d.mobile||'-');
}},
visualMap:{show:false,min:0,max:100,inRange:{color:['#1e293b','#334155']}},
series:[{type:'map',map:'China',roam:true,label:{show:true,color:'#94a3b8',fontSize:10},
itemStyle:{areaColor:'#1e293b',borderColor:'#475569'},emphasis:{itemStyle:{areaColor:'#334155'},label:{color:'#fff'}},
data:tooltipData}]
});
window.addEventListener('resize',()=>chart.resize());
}

fetch(fetchData());
</script>
<script src="https://cdn.jsdelivr.net/npm/echarts@5.5.0/map/js/china.js"></script>
<script>fetchData();</script>
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

      if (pathname === '/api/results/best' && method === 'GET') {
        return handleGetBestResults(env.DB);
      }

      if (pathname === '/api/results' && method === 'POST') {
        const body = await request.json<SpeedResult>();
        return handleSubmitResult(env.DB, body);
      }

      if (pathname === '/api/domains' && method === 'GET') {
        return handleGetDomains(env.DB);
      }

      if (pathname === '/api/domains' && method === 'POST') {
        const body = await request.json<{ name: string; domain: string }>();
        return handleAddDomain(env.DB, body);
      }

      if (pathname.startsWith('/api/domains/') && method === 'DELETE') {
        const domain = decodeURIComponent(pathname.slice('/api/domains/'.length));
        return handleDeleteDomain(env.DB, domain);
      }

      return jsonResponse({ success: false, error: 'Not found' }, 404);
    } catch (e: unknown) {
      const msg = e instanceof Error ? e.message : String(e);
      return jsonResponse({ success: false, error: msg }, 500);
    }
  },
};

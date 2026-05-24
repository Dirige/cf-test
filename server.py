import http.server
import socketserver
import json
import socket
import subprocess
import os
import csv
import threading
import urllib.request
import urllib.error
import platform

BUILTIN_DOMAINS = [
    {"name": "CF优选-090227", "domain": "youxuan.cf.090227.xyz", "is_builtin": True},
    {"name": "MIYU优选", "domain": "saas.sin.fan", "is_builtin": True},
    {"name": "Mingyu优选", "domain": "bestcf.030101.xyz", "is_builtin": True},
    {"name": "NB优选", "domain": "cf.cf.cnae.top", "is_builtin": True},
    {"name": "NexusMods", "domain": "staticdelivery.nexusmods.com", "is_builtin": True},
    {"name": "Shopify官方", "domain": "www.shopify.com", "is_builtin": True},
    {"name": "Visa官方", "domain": "www.visa.cn", "is_builtin": True},
    {"name": "WeTest优选", "domain": "cf.cloudflare.182682.xyz", "is_builtin": True},
    {"name": "乌克兰外交部", "domain": "mfa.gov.ua", "is_builtin": True},
    {"name": "无名氏维护域名", "domain": "cf.tencentapp.cn", "is_builtin": True},
    {"name": "秋名山优选", "domain": "cf.877774.xyz", "is_builtin": True},
    {"name": "育碧商店", "domain": "store.ubi.com", "is_builtin": True},
]

EXE_DIR = os.path.dirname(os.path.abspath(__file__))
CONFIG_PATH = os.path.join(EXE_DIR, "config.json")

custom_domains = []
cf_config = {"api_token": "", "zone_id": "", "domain": "", "record_name": "@", "worker_url": "https://test.dirige.de5.net"}
config_lock = threading.Lock()

test_state = {
    "running": False,
    "progress": "",
    "results": None,
    "detail": {
        "phase": "",
        "current_ip": "",
        "current_domain": "",
        "test_index": 0,
        "test_total": 0,
        "last_speed": 0,
        "last_latency": 0,
        "found_count": 0,
    },
}
test_lock = threading.Lock()


def load_config():
    global custom_domains, cf_config, BUILTIN_DOMAINS
    try:
        with open(CONFIG_PATH, "r", encoding="utf-8") as f:
            cfg = json.load(f)
            if "cf_config" in cfg:
                cf_config.update(cfg["cf_config"])
            if "custom_domains" in cfg:
                custom_domains = cfg["custom_domains"]
            if "builtin_domains" in cfg:
                BUILTIN_DOMAINS = cfg["builtin_domains"]
    except (FileNotFoundError, json.JSONDecodeError):
        pass


def save_config():
    with config_lock:
        cfg = {"cf_config": cf_config, "custom_domains": custom_domains, "builtin_domains": BUILTIN_DOMAINS}
    try:
        with open(CONFIG_PATH, "w", encoding="utf-8") as f:
            json.dump(cfg, f, ensure_ascii=False, indent=2)
    except Exception as e:
        print(f"保存配置失败: {e}")


def get_all_domains():
    with config_lock:
        return BUILTIN_DOMAINS + list(custom_domains)


EN_PROVINCE_MAP = {
    "Anhui": "安徽", "Beijing": "北京", "Chongqing": "重庆", "Fujian": "福建",
    "Gansu": "甘肃", "Guangdong": "广东", "Guangxi": "广西", "Guizhou": "贵州",
    "Hainan": "海南", "Hebei": "河北", "Heilongjiang": "黑龙江", "Henan": "河南",
    "Hubei": "湖北", "Hunan": "湖南", "Inner Mongolia": "内蒙古", "Jiangsu": "江苏",
    "Jiangxi": "江西", "Jilin": "吉林", "Liaoning": "辽宁", "Ningxia": "宁夏",
    "Qinghai": "青海", "Shaanxi": "陕西", "Shandong": "山东", "Shanghai": "上海",
    "Shanxi": "山西", "Sichuan": "四川", "Tianjin": "天津", "Tibet": "西藏",
    "Xinjiang": "新疆", "Yunnan": "云南", "Zhejiang": "浙江",
    "Hong Kong": "香港", "Macau": "澳门", "Taiwan": "台湾",
    "Nei Mongol": "内蒙古", "Xizang": "西藏",
}


def translate_province(name):
    if not name:
        return name
    if name in EN_PROVINCE_MAP:
        return EN_PROVINCE_MAP[name]
    for en, cn in EN_PROVINCE_MAP.items():
        if en.lower() == name.lower():
            return cn
    return name


def normalize_isp(isp_raw):
    isp_lower = isp_raw.lower()
    if any(k in isp_lower for k in ["chinanet", "电信", "telecom", "ctcc"]):
        return "telecom"
    if any(k in isp_lower for k in ["unicom", "联通", "unicom", "cucc"]):
        return "unicom"
    if any(k in isp_lower for k in ["mobile", "移动", "cmcc", "chinamobile"]):
        return "mobile"
    if any(k in isp_lower for k in ["tietong", "铁通", "ctt"]):
        return "mobile"
    if any(k in isp_lower for k in ["cernet", "教育网"]):
        return "telecom"
    return "other"


def fetch_worker_domains():
    with config_lock:
        worker_url = cf_config.get("worker_url", "").rstrip("/")
    if not worker_url:
        return []
    try:
        req = urllib.request.Request(f"{worker_url}/api/domains", method="GET")
        with urllib.request.urlopen(req, timeout=10) as resp:
            data = json.loads(resp.read().decode("utf-8"))
        if data.get("success") and data.get("data"):
            domains = []
            for d in data["data"]:
                domains.append({
                    "name": d.get("name", ""),
                    "domain": d.get("domain", ""),
                    "is_builtin": bool(d.get("is_builtin", 0)),
                })
            return domains
    except Exception as e:
        print(f"从Worker拉取域名失败: {e}")
    return []


def upload_results_to_worker(results, geo_info, mode="ip"):
    with config_lock:
        worker_url = cf_config.get("worker_url", "").rstrip("/")
    if not worker_url:
        return
    province = geo_info.get("province", "")
    isp_raw = geo_info.get("isp", "")
    isp = normalize_isp(isp_raw)
    if not province or isp == "other":
        print(f"跳过上传: 省份={province}, ISP={isp_raw}")
        return
    uploaded = 0
    for r in results:
        if not r.get("success"):
            continue
        try:
            ip_type = r.get("ip_type", "v4")
            if mode == "ip":
                upload_mode = "ipv6" if ip_type == "v6" else "ip"
            else:
                upload_mode = "domain_v6" if ip_type == "v6" else "domain"
            payload = {
                "province": province,
                "isp": isp,
                "mode": upload_mode,
                "domain": r.get("source_domain", ""),
                "domain_name": r.get("source_name", ""),
                "download_speed": r.get("download_speed", 0),
                "latency": r.get("latency", 0),
                "ip_address": r.get("ip", ""),
            }
            body = json.dumps(payload).encode("utf-8")
            req = urllib.request.Request(
                f"{worker_url}/api/results",
                data=body,
                headers={"Content-Type": "application/json", "User-Agent": "CFST-Local/1.0"},
                method="POST",
            )
            with urllib.request.urlopen(req, timeout=10) as resp:
                data = json.loads(resp.read().decode("utf-8"))
                if data.get("success"):
                    uploaded += 1
        except Exception as e:
            print(f"上传结果失败: {e}")
    if uploaded > 0:
        print(f"已上传 {uploaded} 条测速结果到 Worker")


def get_geoip_info():
    try:
        req = urllib.request.Request("https://ipinfo.io/json")
        with urllib.request.urlopen(req, timeout=10) as resp:
            info = json.loads(resp.read().decode("utf-8"))
        isp = info.get("org", "")
        if " " in isp:
            isp = isp.split(" ", 1)[1]
        return {
            "ip": info.get("ip", ""),
            "province": translate_province(info.get("region", "")),
            "city": info.get("city", ""),
            "isp": isp,
        }
    except Exception as e:
        print(f"获取GeoIP失败: {e}")
        return None


def resolve_domains(domains):
    ip_to_domain = {}
    v4_ips = []
    v6_ips = []
    ip_set = set()
    total = len(domains)
    for i, d in enumerate(domains, 1):
        print(f"[{i}/{total}] 解析域名: {d['name']} ({d['domain']})", end=" ")
        try:
            results = socket.getaddrinfo(d["domain"], None)
            v4_count = 0
            v6_count = 0
            for r in results:
                ip = r[4][0]
                if ip in ip_set:
                    continue
                ip_set.add(ip)
                ip_to_domain[ip] = d
                if ":" in ip:
                    v6_ips.append(ip)
                    v6_count += 1
                else:
                    v4_ips.append(ip)
                    v4_count += 1
            print(f"-> {v4_count}个IPv4, {v6_count}个IPv6")
        except socket.gaierror as e:
            print(f"-> 失败: {e}")
    print(f"域名解析完成: 共 {len(v4_ips)} 个IPv4, {len(v6_ips)} 个IPv6")
    return ip_to_domain, v4_ips, v6_ips


def write_ip_file(path, ips):
    with open(path, "w", encoding="utf-8") as f:
        for ip in ips:
            f.write(ip + "\n")


def get_cfst_dir():
    is_windows = platform.system() == "Windows"
    exe_name = "cfst.exe" if is_windows else "cfst"
    candidates = []
    if is_windows:
        candidates.append(os.path.join(EXE_DIR, "cfst_windows_amd64"))
        candidates.append(os.path.join(EXE_DIR, "cfst_windows_arm64"))
    else:
        candidates.append(os.path.join(EXE_DIR, "cfst_linux_amd64"))
        candidates.append(os.path.join(EXE_DIR, "cfst_linux_arm64"))
        candidates.append(os.path.join(EXE_DIR, "cfst_darwin_amd64"))
        candidates.append(os.path.join(EXE_DIR, "cfst_darwin_arm64"))
    candidates.append(EXE_DIR)
    for d in candidates:
        if os.path.isfile(os.path.join(d, exe_name)):
            return d
    return ""


def tcp_ping(ip, port=443, timeout=5):
    import time
    try:
        start = time.time()
        sock = socket.create_connection((ip, port), timeout=timeout)
        elapsed = (time.time() - start) * 1000
        sock.close()
        return elapsed
    except Exception:
        return None


def batch_tcp_ping(ips, port=443, timeout=5, count=4, max_workers=50):
    from concurrent.futures import ThreadPoolExecutor, as_completed
    import time
    results = {}
    total = len(ips)
    done = 0

    def ping_one(ip):
        latencies = []
        for _ in range(count):
            lat = tcp_ping(ip, port, timeout)
            if lat is not None:
                latencies.append(lat)
        if latencies:
            return ip, sum(latencies) / len(latencies)
        return ip, None

    with ThreadPoolExecutor(max_workers=max_workers) as executor:
        futures = {executor.submit(ping_one, ip): ip for ip in ips}
        for future in as_completed(futures):
            ip, avg_lat = future.result()
            done += 1
            if avg_lat is not None:
                results[ip] = avg_lat
            with test_lock:
                test_state["detail"]["test_index"] = done
                test_state["detail"]["current_ip"] = ip
                test_state["detail"]["last_latency"] = round(avg_lat, 1) if avg_lat else 0
                test_state["detail"]["found_count"] = len(results)
    return results


def run_cfst(cfst_dir, ip_file, output_file, download_num=3, download_time=15, ip_type="v4", ip_to_domain=None):
    if platform.system() == "Windows":
        cmd = os.path.join(cfst_dir, "cfst.exe")
    else:
        cmd = os.path.join(cfst_dir, "cfst")
    args = [cmd, "-f", ip_file, "-dn", str(download_num), "-dt", str(download_time), "-p", str(download_num), "-o", output_file]
    print(f"执行: {' '.join(args)}")
    import re, time, threading, select as _select
    ip_count = 0
    try:
        with open(os.path.join(cfst_dir, ip_file), "r") as f:
            ip_count = sum(1 for _ in f)
    except Exception:
        pass
    with test_lock:
        test_state["detail"]["test_total"] = ip_count
        test_state["detail"]["test_index"] = 0
        test_state["detail"]["found_count"] = 0
        test_state["detail"]["last_speed"] = 0
        test_state["detail"]["last_latency"] = 0
        test_state["detail"]["current_ip"] = ""
        test_state["detail"]["current_domain"] = ""
    try:
        proc = subprocess.Popen(args, cwd=cfst_dir, stdin=subprocess.PIPE, stdout=subprocess.PIPE, stderr=subprocess.STDOUT, bufsize=0)
        start = time.time()
        timeout = 600
        killed = False
        finished = False
        base_phase = test_state["detail"].get("phase", "")

        def kill_proc():
            nonlocal killed
            time.sleep(timeout)
            if proc.poll() is None:
                killed = True
                proc.kill()
                print("cfst 执行超时，已终止")

        timer = threading.Thread(target=kill_proc, daemon=True)
        timer.start()

        buf = b""
        while not finished:
            try:
                chunk = proc.stdout.read(4096)
                if not chunk:
                    break
                buf += chunk
                while b"\r" in buf or b"\n" in buf:
                    cr = buf.find(b"\r")
                    lf = buf.find(b"\n")
                    if cr < 0:
                        sep_pos = lf
                    elif lf < 0:
                        sep_pos = cr
                    else:
                        sep_pos = min(cr, lf)
                    line = buf[:sep_pos].decode("utf-8", errors="replace").strip()
                    buf = buf[sep_pos + 1:]
                    if buf.startswith(b"\n"):
                        buf = buf[1:]
                    if not line:
                        continue
                    print(f"  {line}")
                    if '完整测速结果已写入' in line or '跳过输出结果' in line:
                        finished = True
                    prog_m = re.search(r'(\d+)\s*/\s*(\d+)\s*\[', line)
                    if prog_m:
                        cur = int(prog_m.group(1))
                        tot = int(prog_m.group(2))
                        with test_lock:
                            test_state["detail"]["test_index"] = cur
                            test_state["detail"]["test_total"] = tot
                    avail_m = re.search(r'可用:\s*(\d+)', line)
                    if avail_m:
                        with test_lock:
                            test_state["detail"]["found_count"] = int(avail_m.group(1))
                    if '开始延迟测速' in line:
                        with test_lock:
                            test_state["detail"]["phase"] = base_phase + " 延迟测速"
                    if '开始下载测速' in line:
                        with test_lock:
                            test_state["detail"]["phase"] = base_phase + " 下载测速"
                    ip_m = re.match(r'^(\d{1,3}\.\d{1,3}\.\d{1,3}\.\d{1,3}|[0-9a-fA-F:]+)\s', line)
                    if ip_m:
                        ip = ip_m.group(1)
                        domain_info = ip_to_domain.get(ip) if ip_to_domain else None
                        with test_lock:
                            test_state["detail"]["current_ip"] = ip
                            if domain_info:
                                test_state["detail"]["current_domain"] = domain_info.get("name", "") + " (" + domain_info.get("domain", "") + ")"
                            else:
                                test_state["detail"]["current_domain"] = ""
                        cols = line.split()
                        if len(cols) >= 6:
                            try:
                                with test_lock:
                                    test_state["detail"]["last_latency"] = float(cols[4])
                                    test_state["detail"]["last_speed"] = float(cols[5])
                            except (ValueError, IndexError):
                                pass
            except UnicodeDecodeError:
                buf = b""
        try:
            proc.stdin.write(b"\n")
            proc.stdin.close()
        except Exception:
            pass
        proc.wait(timeout=5)
        if killed:
            print("cfst 因超时被终止")
            return False
        if proc.returncode != 0:
            print(f"cfst 退出码: {proc.returncode}")
        return proc.returncode == 0
    except Exception as e:
        print(f"cfst 执行失败: {e}")
        return False


def parse_result_csv(path, ip_type, ip_to_domain):
    results = []
    try:
        with open(path, "r", encoding="utf-8-sig") as f:
            reader = csv.reader(f)
            header = next(reader, None)
            if not header:
                return results
            for row in reader:
                if len(row) < 6:
                    continue
                ip = row[0].strip()
                try:
                    latency = float(row[4].strip())
                except ValueError:
                    latency = 0
                try:
                    speed = float(row[5].strip())
                except ValueError:
                    speed = 0
                src = ip_to_domain.get(ip, {})
                results.append({
                    "ip": ip,
                    "source_domain": src.get("domain", ""),
                    "source_name": src.get("name", ""),
                    "latency": latency,
                    "download_speed": speed,
                    "loss_rate": 0,
                    "ip_type": ip_type,
                    "success": speed > 0 or latency > 0,
                })
    except FileNotFoundError:
        print(f"结果文件不存在: {path}")
    results.sort(key=lambda x: (-x["download_speed"], x["latency"]))
    return results


def calc_domain_stats(results, domains):
    domain_map = {}
    for d in domains:
        domain_map[d["domain"]] = {
            "domain": d["domain"],
            "name": d["name"],
            "ip_count": 0,
            "tested_count": 0,
            "total_latency": 0,
            "total_speed": 0,
            "best_ip": "",
            "best_speed": 0,
        }
    for r in results:
        sd = r.get("source_domain", "")
        if sd not in domain_map:
            continue
        stat = domain_map[sd]
        stat["ip_count"] += 1
        if r["success"]:
            stat["tested_count"] += 1
            stat["total_latency"] += r["latency"]
            stat["total_speed"] += r["download_speed"]
            if r["download_speed"] > stat["best_speed"]:
                stat["best_speed"] = r["download_speed"]
                stat["best_ip"] = r["ip"]
    stats = []
    for d in domains:
        stat = domain_map.get(d["domain"])
        if not stat:
            continue
        avg_latency = stat["total_latency"] / stat["tested_count"] if stat["tested_count"] > 0 else 0
        avg_speed = stat["total_speed"] / stat["tested_count"] if stat["tested_count"] > 0 else 0
        stats.append({
            "domain": stat["domain"],
            "name": stat["name"],
            "ip_count": stat["ip_count"],
            "tested_count": stat["tested_count"],
            "avg_latency": round(avg_latency, 1),
            "avg_speed": round(avg_speed, 2),
            "best_ip": stat["best_ip"],
            "best_speed": round(stat["best_speed"], 2),
        })
    stats.sort(key=lambda x: -x["avg_speed"])
    return stats


def do_speed_test_bg(mode="ip"):
    with test_lock:
        if test_state["running"]:
            return
        test_state["running"] = True
        test_state["progress"] = "正在初始化..."
        test_state["results"] = None

    try:
        all_domains = get_all_domains()
        print(f"=" * 50)
        print(f"开始测速 (模式: {mode}), 共 {len(all_domains)} 个域名")

        cfst_dir = get_cfst_dir()
        if not cfst_dir:
            test_state["progress"] = "错误: 未找到cfst"
            test_state["results"] = {"success": False, "error": "未找到cfst可执行文件，请确保cfst_windows_amd64目录存在"}
            return

        if mode == "ip":
            all_results = do_ip_mode(cfst_dir, all_domains)
        else:
            all_results = do_domain_mode(cfst_dir, all_domains)

        if not all_results:
            test_state["results"] = {"success": False, "error": "测速无结果"}
            return

        all_results.sort(key=lambda x: (-x["download_speed"], x["latency"]))

        top_v4 = [r for r in all_results if r["ip_type"] == "v4"][:3]
        top_v6 = [r for r in all_results if r["ip_type"] == "v6"][:3]

        domain_stats = calc_domain_stats(all_results, all_domains)

        test_state["results"] = {
            "success": True,
            "results": all_results,
            "domain_stats": domain_stats,
            "top_v4": top_v4,
            "top_v6": top_v6,
        }
        print(f"\n测速完成! 共 {len(all_results)} 个结果")
        if top_v4:
            print(f"Top IPv4: {top_v4[0]['ip']} ({top_v4[0]['source_name']}) - {top_v4[0]['download_speed']:.2f}MB/s")
        if top_v6:
            print(f"Top IPv6: {top_v6[0]['ip']} ({top_v6[0]['source_name']}) - {top_v6[0]['download_speed']:.2f}MB/s")
        print(f"=" * 50)
        test_state["progress"] = "测速完成！正在上传共享数据..."
        try:
            geo_info = get_geoip_info()
            if geo_info:
                upload_results_to_worker(all_results, geo_info, mode)
        except Exception as e:
            print(f"上传共享数据失败: {e}")
        test_state["progress"] = "测速完成！"
    except Exception as e:
        test_state["results"] = {"success": False, "error": str(e)}
        test_state["progress"] = f"测速失败: {e}"
    finally:
        test_state["running"] = False


def do_ip_mode(cfst_dir, all_domains):
    test_state["progress"] = "正在解析域名..."
    with test_lock:
        test_state["detail"]["phase"] = "解析"
        test_state["detail"]["test_index"] = 0
        test_state["detail"]["test_total"] = len(all_domains)
        test_state["detail"]["found_count"] = 0
    ip_to_domain, v4_ips, v6_ips = resolve_domains(all_domains)

    if not v4_ips and not v6_ips:
        test_state["progress"] = "错误: 所有域名解析失败"
        test_state["results"] = {"success": False, "error": "所有域名解析失败，无可用IP"}
        return []

    all_ips = v4_ips + v6_ips
    print(f"DNS解析完成: {len(v4_ips)} 个IPv4, {len(v6_ips)} 个IPv6, 共 {len(all_ips)} 个IP")

    test_state["progress"] = f"正在测试延迟 ({len(all_ips)}个IP)..."
    with test_lock:
        test_state["detail"]["phase"] = "延迟测试"
        test_state["detail"]["test_total"] = len(all_ips)
        test_state["detail"]["test_index"] = 0
        test_state["detail"]["found_count"] = 0
        test_state["detail"]["last_speed"] = 0
        test_state["detail"]["current_domain"] = ""
    print(f"\n--- TCP 延迟测试 ({len(all_ips)}个IP) ---")
    lat_results = batch_tcp_ping(all_ips, port=443, timeout=5, count=4, max_workers=50)
    print(f"延迟测试完成: {len(lat_results)} 个IP可达")

    v4_sorted = sorted(
        [(ip, lat) for ip, lat in lat_results.items() if ":" not in ip],
        key=lambda x: x[1]
    )[:10]
    v6_sorted = sorted(
        [(ip, lat) for ip, lat in lat_results.items() if ":" in ip],
        key=lambda x: x[1]
    )[:10]

    print(f"延迟筛选: {len(v4_sorted)} 个IPv4, {len(v6_sorted)} 个IPv6 进入下载测速")

    all_results = []

    if v4_sorted:
        test_state["progress"] = f"正在下载测速IPv4 ({len(v4_sorted)}个IP)..."
        with test_lock:
            test_state["detail"]["phase"] = "IPv4 下载测速"
            test_state["detail"]["test_index"] = 0
            test_state["detail"]["test_total"] = len(v4_sorted)
            test_state["detail"]["found_count"] = 0
        print(f"\n--- IPv4 下载测速 ({len(v4_sorted)}个IP) ---")
        v4_test_ips = [ip for ip, _ in v4_sorted]
        ip_file = os.path.join(cfst_dir, "ip.txt")
        write_ip_file(ip_file, v4_test_ips)
        if run_cfst(cfst_dir, "ip.txt", "result.csv", download_num=3, download_time=15, ip_type="v4", ip_to_domain=ip_to_domain):
            result_file = os.path.join(cfst_dir, "result.csv")
            results = parse_result_csv(result_file, "v4", ip_to_domain)
            all_results.extend(results)
            print(f"IPv4 下载测速完成: {len(results)} 个结果")

    if v6_sorted:
        test_state["progress"] = f"正在下载测速IPv6 ({len(v6_sorted)}个IP)..."
        with test_lock:
            test_state["detail"]["phase"] = "IPv6 下载测速"
            test_state["detail"]["test_index"] = 0
            test_state["detail"]["test_total"] = len(v6_sorted)
            test_state["detail"]["found_count"] = 0
        print(f"\n--- IPv6 下载测速 ({len(v6_sorted)}个IP) ---")
        v6_test_ips = [ip for ip, _ in v6_sorted]
        ip_file = os.path.join(cfst_dir, "ipv6.txt")
        write_ip_file(ip_file, v6_test_ips)
        if run_cfst(cfst_dir, "ipv6.txt", "result6.csv", download_num=3, download_time=15, ip_type="v6", ip_to_domain=ip_to_domain):
            result_file = os.path.join(cfst_dir, "result6.csv")
            results = parse_result_csv(result_file, "v6", ip_to_domain)
            all_results.extend(results)
            print(f"IPv6 下载测速完成: {len(results)} 个结果")

    return all_results


def do_domain_mode(cfst_dir, all_domains):
    all_results = []
    total = len(all_domains)

    for idx, domain_info in enumerate(all_domains):
        dname = domain_info.get("name", "")
        ddomain = domain_info.get("domain", "")
        test_state["progress"] = f"正在测速域名 [{idx+1}/{total}]: {dname} ({ddomain})"
        with test_lock:
            test_state["detail"]["phase"] = f"域名测速 [{idx+1}/{total}]"
            test_state["detail"]["current_domain"] = f"{dname} ({ddomain})"
            test_state["detail"]["test_index"] = 0
            test_state["detail"]["test_total"] = 0
            test_state["detail"]["found_count"] = 0
            test_state["detail"]["last_speed"] = 0
            test_state["detail"]["last_latency"] = 0
        print(f"\n--- 域名测速 [{idx+1}/{total}]: {dname} ({ddomain}) ---")

        ip_to_domain = {}
        v4_ips = []
        v6_ips = []
        ip_set = set()
        try:
            results = socket.getaddrinfo(ddomain, None)
            for r in results:
                ip = r[4][0]
                if ip in ip_set:
                    continue
                ip_set.add(ip)
                ip_to_domain[ip] = domain_info
                if ":" in ip:
                    v6_ips.append(ip)
                else:
                    v4_ips.append(ip)
        except socket.gaierror as e:
            print(f"  域名解析失败: {e}")
            continue

        if not v4_ips and not v6_ips:
            print(f"  无可用IP，跳过")
            continue

        print(f"  解析到 {len(v4_ips)} 个IPv4, {len(v6_ips)} 个IPv6")

        if v4_ips:
            ip_file = os.path.join(cfst_dir, "ip.txt")
            write_ip_file(ip_file, v4_ips)
            with test_lock:
                test_state["detail"]["phase"] = f"IPv4 [{idx+1}/{total}] {dname}"
            if run_cfst(cfst_dir, "ip.txt", "result.csv", download_num=3, download_time=15, ip_type="v4", ip_to_domain=ip_to_domain):
                result_file = os.path.join(cfst_dir, "result.csv")
                results = parse_result_csv(result_file, "v4", ip_to_domain)
                all_results.extend(results)
                print(f"  IPv4: {len(results)} 个结果")

        if v6_ips:
            ip_file = os.path.join(cfst_dir, "ipv6.txt")
            write_ip_file(ip_file, v6_ips)
            with test_lock:
                test_state["detail"]["phase"] = f"IPv6 [{idx+1}/{total}] {dname}"
            if run_cfst(cfst_dir, "ipv6.txt", "result6.csv", download_num=3, download_time=15, ip_type="v6", ip_to_domain=ip_to_domain):
                result_file = os.path.join(cfst_dir, "result6.csv")
                results = parse_result_csv(result_file, "v6", ip_to_domain)
                all_results.extend(results)
                print(f"  IPv6: {len(results)} 个结果")

    return all_results


def do_single_domain_test(domain_info):
    dname = domain_info.get("name", "")
    ddomain = domain_info.get("domain", "")
    with test_lock:
        if test_state["running"]:
            return
        test_state["running"] = True
        test_state["progress"] = f"正在测速: {dname} ({ddomain})"
        test_state["results"] = None
        test_state["detail"]["phase"] = f"单域名测速"
        test_state["detail"]["current_domain"] = f"{dname} ({ddomain})"
        test_state["detail"]["test_index"] = 0
        test_state["detail"]["test_total"] = 0
        test_state["detail"]["found_count"] = 0
        test_state["detail"]["last_speed"] = 0
        test_state["detail"]["last_latency"] = 0
        test_state["detail"]["current_ip"] = ""

    try:
        cfst_dir = get_cfst_dir()
        if not cfst_dir:
            test_state["results"] = {"success": False, "error": "未找到cfst可执行文件"}
            return

        print(f"\n--- 单域名测速: {dname} ({ddomain}) ---")

        ip_to_domain = {}
        v4_ips = []
        v6_ips = []
        ip_set = set()
        try:
            results = socket.getaddrinfo(ddomain, None)
            for r in results:
                ip = r[4][0]
                if ip in ip_set:
                    continue
                ip_set.add(ip)
                ip_to_domain[ip] = domain_info
                if ":" in ip:
                    v6_ips.append(ip)
                else:
                    v4_ips.append(ip)
        except socket.gaierror as e:
            test_state["results"] = {"success": False, "error": f"域名解析失败: {e}"}
            return

        if not v4_ips and not v6_ips:
            test_state["results"] = {"success": False, "error": "域名解析无可用IP"}
            return

        print(f"  解析到 {len(v4_ips)} 个IPv4, {len(v6_ips)} 个IPv6")
        all_results = []

        if v4_ips:
            with test_lock:
                test_state["detail"]["phase"] = f"IPv4 测速 {dname}"
            ip_file = os.path.join(cfst_dir, "ip.txt")
            write_ip_file(ip_file, v4_ips)
            if run_cfst(cfst_dir, "ip.txt", "result.csv", download_num=3, download_time=15, ip_type="v4", ip_to_domain=ip_to_domain):
                result_file = os.path.join(cfst_dir, "result.csv")
                results = parse_result_csv(result_file, "v4", ip_to_domain)
                all_results.extend(results)
                print(f"  IPv4: {len(results)} 个结果")

        if v6_ips:
            with test_lock:
                test_state["detail"]["phase"] = f"IPv6 测速 {dname}"
            ip_file = os.path.join(cfst_dir, "ipv6.txt")
            write_ip_file(ip_file, v6_ips)
            if run_cfst(cfst_dir, "ipv6.txt", "result6.csv", download_num=3, download_time=15, ip_type="v6", ip_to_domain=ip_to_domain):
                result_file = os.path.join(cfst_dir, "result6.csv")
                results = parse_result_csv(result_file, "v6", ip_to_domain)
                all_results.extend(results)
                print(f"  IPv6: {len(results)} 个结果")

        if not all_results:
            test_state["results"] = {"success": False, "error": "测速无结果"}
            return

        all_results.sort(key=lambda x: (-x["download_speed"], x["latency"]))
        top_v4 = [r for r in all_results if r["ip_type"] == "v4"][:3]
        top_v6 = [r for r in all_results if r["ip_type"] == "v6"][:3]

        test_state["results"] = {
            "success": True,
            "results": all_results,
            "domain_stats": [],
            "top_v4": top_v4,
            "top_v6": top_v6,
        }
        test_state["progress"] = "测速完成！"
        print(f"  单域名测速完成: {len(all_results)} 个结果")

        try:
            geo_info = get_geoip_info()
            if geo_info:
                upload_results_to_worker(all_results, geo_info, "domain")
        except Exception as e:
            print(f"上传共享数据失败: {e}")
    except Exception as e:
        test_state["results"] = {"success": False, "error": str(e)}
        test_state["progress"] = f"测速失败: {e}"
    finally:
        test_state["running"] = False


def cf_api_request(method, path, data=None):
    with config_lock:
        token = cf_config.get("api_token", "")
        zone_id = cf_config.get("zone_id", "")
    url = f"https://api.cloudflare.com/client/v4/zones/{zone_id}{path}"
    headers = {
        "Authorization": f"Bearer {token}",
        "Content-Type": "application/json",
    }
    body = json.dumps(data).encode("utf-8") if data else None
    req = urllib.request.Request(url, data=body, headers=headers, method=method)
    try:
        with urllib.request.urlopen(req, timeout=15) as resp:
            return json.loads(resp.read().decode("utf-8"))
    except urllib.error.HTTPError as e:
        try:
            err_body = json.loads(e.read().decode("utf-8"))
            return err_body
        except Exception:
            return {"success": False, "errors": [{"message": f"HTTP {e.code}"}]}
    except Exception as e:
        return {"success": False, "errors": [{"message": str(e)}]}


def delete_conflicting_records(record_name, conflict_types):
    resp = cf_api_request("GET", "/dns_records")
    deleted = []
    if not resp.get("success"):
        return deleted
    for r in resp.get("result", []):
        if r.get("name") == record_name and r.get("type") in conflict_types:
            cf_api_request("DELETE", f"/dns_records/{r['id']}")
            deleted.append({"id": r["id"], "type": r["type"], "name": r["name"], "content": r["content"]})
    return deleted


class Handler(http.server.BaseHTTPRequestHandler):
    def log_message(self, format, *args):
        print(f"[{self.log_date_time_string()}] {format % args}")

    def send_json(self, data, status=200):
        body = json.dumps(data, ensure_ascii=False).encode("utf-8")
        self.send_response(status)
        self.send_header("Content-Type", "application/json; charset=utf-8")
        self.send_header("Content-Length", str(len(body)))
        self.end_headers()
        self.wfile.write(body)

    def send_error_json(self, msg, status=200):
        self.send_json({"success": False, "error": msg})

    def do_GET(self):
        path = self.path.split("?")[0]

        if path == "/" or path == "/index.html":
            self.serve_index()
        elif path == "/api/geoip":
            self.handle_geoip()
        elif path == "/api/domains":
            self.handle_get_domains()
        elif path == "/api/dns/config":
            self.handle_get_dns_config()
        elif path == "/api/dns/records":
            self.handle_get_dns_records()
        elif path == "/api/speedtest/status":
            self.handle_speedtest_status()
        else:
            self.send_error(404)

    def do_POST(self):
        path = self.path.split("?")[0]

        if path == "/api/domains":
            self.handle_add_domain()
        elif path == "/api/domains/update":
            self.handle_update_domain()
        elif path == "/api/speedtest/ip":
            self.handle_start_speedtest("ip")
        elif path == "/api/speedtest/domain":
            self.handle_start_speedtest("domain")
        elif path == "/api/speedtest/single":
            self.handle_start_single_test()
        elif path == "/api/dns/config":
            self.handle_save_dns_config()
        elif path == "/api/dns/records":
            self.handle_add_dns_records()
        elif path == "/api/dns/replace-cname":
            self.handle_replace_cname()
        else:
            self.send_error(404)

    def do_DELETE(self):
        path = self.path.split("?")[0]
        if path.startswith("/api/domains/"):
            domain = path[len("/api/domains/"):]
            self.handle_delete_domain(domain)
        elif path.startswith("/api/dns/records/"):
            record_id = path[len("/api/dns/records/"):]
            self.handle_delete_dns_record(record_id)
        else:
            self.send_error(404)

    def serve_index(self):
        html_path = os.path.join(EXE_DIR, "index.html")
        try:
            with open(html_path, "r", encoding="utf-8") as f:
                content = f.read().encode("utf-8")
            self.send_response(200)
            self.send_header("Content-Type", "text/html; charset=utf-8")
            self.send_header("Content-Length", str(len(content)))
            self.end_headers()
            self.wfile.write(content)
        except FileNotFoundError:
            self.send_error(404)

    def handle_geoip(self):
        try:
            req = urllib.request.Request("https://ipinfo.io/json")
            with urllib.request.urlopen(req, timeout=10) as resp:
                info = json.loads(resp.read().decode("utf-8"))
            isp = info.get("org", "")
            if " " in isp:
                isp = isp.split(" ", 1)[1]
            self.send_json({
                "success": True,
                "data": {
                    "ip": info.get("ip", ""),
                    "province": translate_province(info.get("region", "")),
                    "city": info.get("city", ""),
                    "isp": isp,
                    "isp_tag": info.get("org", ""),
                }
            })
        except Exception as e:
            self.send_error_json(f"获取GeoIP失败: {e}")

    def handle_get_domains(self):
        all_domains = get_all_domains()
        self.send_json({"success": True, "data": all_domains})

    def handle_add_domain(self):
        global custom_domains
        try:
            length = int(self.headers.get("Content-Length", 0))
            body = json.loads(self.rfile.read(length).decode("utf-8"))
        except Exception:
            self.send_error_json("请求格式错误")
            return
        domain = body.get("domain", "").strip()
        name = body.get("name", "").strip()
        if not domain:
            self.send_error_json("域名不能为空")
            return
        for d in BUILTIN_DOMAINS:
            if d["domain"] == domain:
                self.send_error_json("该域名已在内置列表中")
                return
        with config_lock:
            for d in custom_domains:
                if d["domain"] == domain:
                    self.send_error_json("该域名已存在")
                    return
            custom_domains.append({"name": name, "domain": domain, "is_builtin": False})
        save_config()
        self.send_json({"success": True})

    def handle_delete_domain(self, domain):
        global custom_domains, BUILTIN_DOMAINS
        domain = domain.strip()
        found = False
        with config_lock:
            for i, d in enumerate(custom_domains):
                if d["domain"] == domain:
                    custom_domains.pop(i)
                    found = True
                    break
            if not found:
                for i, d in enumerate(BUILTIN_DOMAINS):
                    if d["domain"] == domain:
                        BUILTIN_DOMAINS.pop(i)
                        found = True
                        break
        if not found:
            self.send_error_json("域名不存在")
            return
        save_config()
        self.send_json({"success": True})

    def handle_update_domain(self):
        global custom_domains
        try:
            length = int(self.headers.get("Content-Length", 0))
            body = json.loads(self.rfile.read(length).decode("utf-8"))
        except Exception:
            self.send_error_json("请求格式错误")
            return
        old_domain = body.get("old_domain", "").strip()
        new_name = body.get("name", "").strip()
        new_domain = body.get("domain", "").strip()
        if not old_domain:
            self.send_error_json("缺少原域名")
            return
        if not new_domain:
            self.send_error_json("域名不能为空")
            return
        with config_lock:
            for d in BUILTIN_DOMAINS:
                if d["domain"] == old_domain:
                    if new_domain != old_domain:
                        for d2 in BUILTIN_DOMAINS:
                            if d2["domain"] == new_domain and d2 is not d:
                                self.send_error_json("该域名已在内置列表中")
                                return
                        for d2 in custom_domains:
                            if d2["domain"] == new_domain:
                                self.send_error_json("该域名已存在")
                                return
                    d["name"] = new_name
                    d["domain"] = new_domain
                    save_config()
                    self.send_json({"success": True})
                    return
            for d in custom_domains:
                if d["domain"] == old_domain:
                    if new_domain != old_domain:
                        for d2 in custom_domains:
                            if d2["domain"] == new_domain and d2 is not d:
                                self.send_error_json("该域名已存在")
                                return
                        for d2 in BUILTIN_DOMAINS:
                            if d2["domain"] == new_domain:
                                self.send_error_json("该域名已在内置列表中")
                                return
                    d["name"] = new_name
                    d["domain"] = new_domain
                    save_config()
                    self.send_json({"success": True})
                    return
        self.send_error_json("域名不存在")

    def handle_start_speedtest(self, mode):
        with test_lock:
            if test_state["running"]:
                self.send_error_json("已有测速任务正在运行，请等待完成")
                return
        t = threading.Thread(target=do_speed_test_bg, args=(mode,), daemon=True)
        t.start()
        self.send_json({"success": True, "message": "测速已开始"})

    def handle_start_single_test(self):
        with test_lock:
            if test_state["running"]:
                self.send_error_json("已有测速任务正在运行，请等待完成")
                return
        try:
            length = int(self.headers.get("Content-Length", 0))
            body = json.loads(self.rfile.read(length).decode("utf-8"))
        except Exception:
            self.send_error_json("请求格式错误")
            return
        domain = body.get("domain", "").strip()
        name = body.get("name", "").strip()
        if not domain:
            self.send_error_json("域名不能为空")
            return
        domain_info = {"name": name, "domain": domain, "is_builtin": body.get("is_builtin", False)}
        t = threading.Thread(target=do_single_domain_test, args=(domain_info,), daemon=True)
        t.start()
        self.send_json({"success": True, "message": "测速已开始"})

    def handle_speedtest_status(self):
        with test_lock:
            resp = {
                "success": True,
                "running": test_state["running"],
                "progress": test_state["progress"],
                "results": test_state["results"],
                "detail": dict(test_state["detail"]),
            }
        self.send_json(resp)

    def handle_get_dns_config(self):
        with config_lock:
            cfg = dict(cf_config)
        self.send_json({"success": True, "data": cfg})

    def handle_save_dns_config(self):
        global cf_config
        try:
            length = int(self.headers.get("Content-Length", 0))
            body = json.loads(self.rfile.read(length).decode("utf-8"))
        except Exception:
            self.send_error_json("请求格式错误")
            return
        with config_lock:
            cf_config.update({
                "api_token": body.get("api_token", ""),
                "zone_id": body.get("zone_id", ""),
                "domain": body.get("domain", ""),
                "record_name": body.get("record_name", "@"),
            })
        save_config()
        self.send_json({"success": True})

    def handle_get_dns_records(self):
        with config_lock:
            token = cf_config.get("api_token", "")
            zone_id = cf_config.get("zone_id", "")
        if not token or not zone_id:
            self.send_error_json("请先配置Cloudflare API信息")
            return
        resp = cf_api_request("GET", "/dns_records")
        if not resp.get("success"):
            errors = resp.get("errors", [{}])
            msg = errors[0].get("message", "未知错误") if errors else "未知错误"
            self.send_error_json(f"获取DNS记录失败: {msg}")
            return
        records = []
        for r in resp.get("result", []):
            if r.get("type") in ("A", "AAAA", "CNAME"):
                records.append({
                    "id": r["id"],
                    "type": r["type"],
                    "name": r["name"],
                    "content": r["content"],
                    "proxied": r.get("proxied", False),
                    "ttl": r.get("ttl", 1),
                })
        self.send_json({"success": True, "data": records})

    def handle_add_dns_records(self):
        with config_lock:
            token = cf_config.get("api_token", "")
            zone_id = cf_config.get("zone_id", "")
        if not token or not zone_id:
            self.send_error_json("请先配置Cloudflare API信息")
            return
        try:
            length = int(self.headers.get("Content-Length", 0))
            body = json.loads(self.rfile.read(length).decode("utf-8"))
        except Exception:
            self.send_error_json("请求格式错误")
            return
        record_type = body.get("type", "A")
        name = body.get("name", "")
        if not name:
            self.send_error_json("name不能为空")
            return
        deleted_conflicts = delete_conflicting_records(name, ["A", "AAAA", "CNAME"])
        if record_type == "CNAME":
            content = body.get("content", "")
            if not content:
                self.send_error_json("CNAME目标域名不能为空")
                return
            data = {
                "type": "CNAME",
                "name": name,
                "content": content,
                "ttl": 1,
                "proxied": False,
            }
            resp = cf_api_request("POST", "/dns_records", data)
            if not resp.get("success"):
                errs = resp.get("errors", [{}])
                msg = errs[0].get("message", "未知错误") if errs else "未知错误"
                self.send_error_json(f"添加CNAME记录失败: {msg}")
                return
            r = resp["result"]
            self.send_json({
                "success": True,
                "data": [{
                    "id": r["id"],
                    "type": r["type"],
                    "name": r["name"],
                    "content": r["content"],
                    "proxied": r.get("proxied", False),
                    "ttl": r.get("ttl", 1),
                }],
                "deleted_conflicts": deleted_conflicts,
            })
        else:
            ips = body.get("ips", [])
            if not ips:
                self.send_error_json("ips不能为空")
                return
            created = []
            errors = []
            for ip in ips:
                data = {
                    "type": record_type,
                    "name": name,
                    "content": ip,
                    "ttl": 1,
                    "proxied": False,
                }
                resp = cf_api_request("POST", "/dns_records", data)
                if resp.get("success"):
                    r = resp["result"]
                    created.append({
                        "id": r["id"],
                        "type": r["type"],
                        "name": r["name"],
                        "content": r["content"],
                        "proxied": r.get("proxied", False),
                        "ttl": r.get("ttl", 1),
                    })
                else:
                    errs = resp.get("errors", [{}])
                    msg = errs[0].get("message", "未知错误") if errs else "未知错误"
                    errors.append(f"{ip}: {msg}")
            if errors and not created:
                self.send_error_json("添加DNS记录全部失败: " + "; ".join(errors))
                return
            self.send_json({"success": True, "data": created, "errors": errors, "deleted_conflicts": deleted_conflicts})

    def handle_replace_cname(self):
        with config_lock:
            token = cf_config.get("api_token", "")
            zone_id = cf_config.get("zone_id", "")
        if not token or not zone_id:
            self.send_error_json("请先配置Cloudflare API信息")
            return
        try:
            length = int(self.headers.get("Content-Length", 0))
            body = json.loads(self.rfile.read(length).decode("utf-8"))
        except Exception:
            self.send_error_json("请求格式错误")
            return
        name = body.get("name", "")
        target = body.get("target", "")
        if not name:
            self.send_error_json("name不能为空")
            return
        if not target:
            self.send_error_json("target不能为空")
            return
        deleted_conflicts = delete_conflicting_records(name, ["A", "AAAA", "CNAME"])
        data = {
            "type": "CNAME",
            "name": name,
            "content": target,
            "ttl": 1,
            "proxied": False,
        }
        resp = cf_api_request("POST", "/dns_records", data)
        if not resp.get("success"):
            errs = resp.get("errors", [{}])
            msg = errs[0].get("message", "未知错误") if errs else "未知错误"
            self.send_error_json(f"创建CNAME记录失败: {msg}")
            return
        r = resp["result"]
        self.send_json({
            "success": True,
            "data": {
                "id": r["id"],
                "type": r["type"],
                "name": r["name"],
                "content": r["content"],
                "proxied": r.get("proxied", False),
                "ttl": r.get("ttl", 1),
            },
            "deleted_conflicts": deleted_conflicts,
        })

    def handle_delete_dns_record(self, record_id):
        with config_lock:
            token = cf_config.get("api_token", "")
            zone_id = cf_config.get("zone_id", "")
        if not token or not zone_id:
            self.send_error_json("请先配置Cloudflare API信息")
            return
        resp = cf_api_request("DELETE", f"/dns_records/{record_id}")
        if not resp.get("success"):
            self.send_error_json("删除DNS记录失败")
            return
        self.send_json({"success": True})


class ThreadedHTTPServer(socketserver.ThreadingMixIn, http.server.HTTPServer):
    daemon_threads = True
    allow_reuse_address = True


def main():
    load_config()
    port = 8081
    server = None
    for p in range(port, port + 10):
        try:
            server = ThreadedHTTPServer(("127.0.0.1", p), Handler)
            port = p
            break
        except OSError:
            continue
    if not server:
        print("错误: 端口 8081-8090 均被占用，请关闭占用端口的程序后重试")
        input("按回车键退出...")
        return
    print(f"CFST Web Server 启动中... http://localhost:{port}")
    print(f"工作目录: {EXE_DIR}")
    cfst_dir = get_cfst_dir()
    if cfst_dir:
        print(f"cfst目录: {cfst_dir}")
    else:
        print("警告: 未找到cfst可执行文件！")
    try:
        server.serve_forever()
    except KeyboardInterrupt:
        print("\n服务器已停止")
        server.server_close()


if __name__ == "__main__":
    main()

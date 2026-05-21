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

INSERT OR IGNORE INTO domains (name, domain, is_builtin) VALUES
('CF优选-090227', 'youxuan.cf.090227.xyz', 1),
('Shopify官方', 'www.shopify.com', 1),
('Mingyu优选', 'bestcf.030101.xyz', 1),
('育碧商店', 'store.ubi.com', 1),
('WeTest优选', 'cf.cloudflare.182682.xyz', 1),
('MIYU优选', 'saas.sin.fan', 1),
('NexusMods', 'staticdelivery.nexusmods.com', 1),
('乌克兰外交部', 'mfa.gov.ua', 1),
('NB优选', 'cf.cf.cnae.top', 1),
('Visa官方', 'www.visa.cn', 1),
('秋名山优选', 'cf.877774.xyz', 1),
('无名氏维护域名', 'cf.tencentapp.cn', 1);

CREATE INDEX IF NOT EXISTS idx_speed_results_province_isp ON speed_results(province, isp);
CREATE INDEX IF NOT EXISTS idx_speed_results_created_at ON speed_results(created_at);

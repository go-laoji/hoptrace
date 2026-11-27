-- HopTrace PostgreSQL 生产环境按月分区脚本(Issue #14)
-- 适用对象: 高频写入的历史表 endpoint_events / endpoint_locations
-- 说明: 生产环境建议在首次部署(关闭 auto_migrate 或先执行本脚本)时创建分区父表,
--       并通过 pg_cron / 外部定时任务每月调用 create_hoptrace_partitions(3) 预建未来 3 个月分区。

CREATE TABLE IF NOT EXISTS endpoint_events (
    id          BIGSERIAL,
    endpoint_id BIGINT NOT NULL,
    mac         VARCHAR(20),
    ip          VARCHAR(64),
    name        VARCHAR(200),
    type        VARCHAR(20),
    detail      VARCHAR(500),
    at          TIMESTAMPTZ NOT NULL,
    PRIMARY KEY (id, at)
) PARTITION BY RANGE (at);

CREATE INDEX IF NOT EXISTS idx_endpoint_events_ep_at ON endpoint_events (endpoint_id, at DESC);
CREATE INDEX IF NOT EXISTS idx_endpoint_events_type_at ON endpoint_events (type, at DESC);

CREATE TABLE IF NOT EXISTS endpoint_locations (
    id          BIGSERIAL,
    endpoint_id BIGINT NOT NULL,
    device_id   BIGINT NOT NULL,
    if_index    INTEGER NOT NULL,
    if_name     VARCHAR(100),
    vlan        INTEGER,
    since       TIMESTAMPTZ NOT NULL,
    until       TIMESTAMPTZ,
    PRIMARY KEY (id, since)
) PARTITION BY RANGE (since);

CREATE INDEX IF NOT EXISTS idx_endpoint_locations_ep_since ON endpoint_locations (endpoint_id, since DESC);

-- 自动预建未来 N 个月的按月分区
CREATE OR REPLACE FUNCTION create_hoptrace_partitions(months_ahead INT DEFAULT 3)
RETURNS VOID AS $$
DECLARE
    i INT;
    m_start TIMESTAMPTZ;
    m_end   TIMESTAMPTZ;
    suffix  TEXT;
BEGIN
    FOR i IN 0..months_ahead LOOP
        m_start := date_trunc('month', now()) + (i || ' month')::INTERVAL;
        m_end   := m_start + INTERVAL '1 month';
        suffix  := to_char(m_start, 'YYYYMM');

        EXECUTE format(
            'CREATE TABLE IF NOT EXISTS endpoint_events_%s PARTITION OF endpoint_events FOR VALUES FROM (%L) TO (%L)',
            suffix, m_start, m_end
        );
        EXECUTE format(
            'CREATE TABLE IF NOT EXISTS endpoint_locations_%s PARTITION OF endpoint_locations FOR VALUES FROM (%L) TO (%L)',
            suffix, m_start, m_end
        );
    END LOOP;
END;
$$ LANGUAGE plpgsql;

SELECT create_hoptrace_partitions(3);

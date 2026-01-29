-- Hermod Database Migration
-- This script sets up the database schema for Hermod

-- Create the main table for MQTT messages
CREATE TABLE IF NOT EXISTS mqtt_messages (
    id BIGSERIAL PRIMARY KEY,
    topic TEXT NOT NULL,
    timestamp TIMESTAMPTZ DEFAULT NOW(),
    payload JSONB,
    
    -- Sensor data fields (example)
    temperature_celsius DOUBLE PRECISION,
    temperature_fahrenheit DOUBLE PRECISION,
    humidity DOUBLE PRECISION,
    pressure DOUBLE PRECISION,
    
    -- Metadata
    processed_by TEXT,
    processed_at TEXT,
    
    -- Additional fields can be stored in JSONB
    metadata JSONB
);

-- Create indexes for common queries
CREATE INDEX IF NOT EXISTS idx_mqtt_messages_topic ON mqtt_messages(topic);
CREATE INDEX IF NOT EXISTS idx_mqtt_messages_timestamp ON mqtt_messages(timestamp DESC);
CREATE INDEX IF NOT EXISTS idx_mqtt_messages_payload ON mqtt_messages USING GIN(payload);

-- Convert to TimescaleDB hypertable (optional, requires TimescaleDB extension)
-- Uncomment the following lines if you're using TimescaleDB
-- SELECT create_hypertable('mqtt_messages', 'timestamp', if_not_exists => TRUE);
-- 
-- -- Set up data retention policy (e.g., keep data for 30 days)
-- -- SELECT add_retention_policy('mqtt_messages', INTERVAL '30 days');
-- 
-- -- Create continuous aggregates for time-series analysis (example)
-- -- CREATE MATERIALIZED VIEW mqtt_messages_hourly
-- -- WITH (timescaledb.continuous) AS
-- -- SELECT
-- --     time_bucket('1 hour', timestamp) AS bucket,
-- --     topic,
-- --     AVG(temperature_celsius) AS avg_temperature,
-- --     MIN(temperature_celsius) AS min_temperature,
-- --     MAX(temperature_celsius) AS max_temperature,
-- --     COUNT(*) AS message_count
-- -- FROM mqtt_messages
-- -- GROUP BY bucket, topic;
-- -- 
-- -- SELECT add_continuous_aggregate_policy('mqtt_messages_hourly',
-- --     start_offset => INTERVAL '3 hours',
-- --     end_offset => INTERVAL '1 hour',
-- --     schedule_interval => INTERVAL '1 hour');

-- Grant permissions (adjust user as needed)
-- GRANT SELECT, INSERT ON mqtt_messages TO hermod;
-- GRANT USAGE, SELECT ON SEQUENCE mqtt_messages_id_seq TO hermod;

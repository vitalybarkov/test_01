# Click Counter Service

## How It Works

### Click Processing (`/counter`)
- Uses a channel for asynchronous click processing
- Clicks are batched and written to the database:
  - Every 1 second, OR
  - When reaching 100 clicks in a batch
- Uses `ON CONFLICT` to increment counters for existing minute entries

### Statistics Retrieval (`/stats`)
- Simple database query with time filtering
- Uses indexes for fast search

### Optimizations for Middle+ Level
- Query batching reduces database load
- Prepared statements speed up inserts
- Configured database connection pool
- Graceful shutdown for proper termination

## Setup & Running

1. Install PostgreSQL and create database `click_counter`
2. Execute the following SQL to create tables.
3. Test out the service.

## SQL
DROP TABLE IF EXISTS banners;

CREATE TABLE banners (
    id SERIAL PRIMARY KEY,
    name VARCHAR(255) NOT NULL
);

DROP TABLE IF EXISTS clicks;

CREATE TABLE clicks (
    banner_id INTEGER REFERENCES banners(id),
    timestamp TIMESTAMP WITH TIME ZONE DEFAULT date_trunc('minute', NOW()),
    count INTEGER DEFAULT 1,
    PRIMARY KEY (banner_id, timestamp)
);

CREATE INDEX idx_clicks_banner_id ON clicks(banner_id);
CREATE INDEX idx_clicks_timestamp ON clicks(timestamp);

-- Sample data

INSERT INTO banners (id, name) VALUES 
(1, 'tom'),
(2, 'bom'),
(3, 'dom'),
(4, 'rom');


## TEST
curl http://localhost:8080/counter/3

curl -X POST http://localhost:8090/stats/3   -H "Content-Type: application/json"   -d '{"ts_from": "2023-01-01T00:00:00Z", "ts_to": "2025-12-31T23:59:59Z"}'

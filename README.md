// HOW IT WORKS:
обработка кликов (/counter):
-- использую канал для асинхронной обработки кликов
-- клики накапливаются в батчи и записываются в БД раз в секунду или при достижении 100 кликов
-- использую ON CONFLICT для инкремента счетчика при существующей записи за ту же минуту
получение статистики (/stats):
-- простой запрос к БД с фильтрацией по времени
-- использую индексы для быстрого поиска
оптимизации для Middle+ уровня:
-- батчинг запросов уменьшает нагрузку на БД
-- подготовленные запросы ускоряют вставку
-- настроено пул соединений к БД
-- graceful shutdown для корректного завершения

// запуск
-- установить PostgreSQL и создать БД click_counter
-- выполнить SQL для создания таблиц
-- запустить сервис:
go run main.go
go run main.go -port 9090

// это решение обеспечивает производительность 500+ RPS за счет батчинга и асинхронной обработки, сохраняя при этом простоту и надежность.

// SQL:
DROP TABLE IF EXISTS banners;

CREATE TABLE banners (
    id SERIAL PRIMARY KEY,
    name VARCHAR(255) NOT NULL
);

-- Drop existing table if needed
DROP TABLE IF EXISTS clicks;

-- Create new table with proper constraints
CREATE TABLE clicks (
    banner_id INTEGER REFERENCES banners(id),
    timestamp TIMESTAMP WITH TIME ZONE DEFAULT date_trunc('minute', NOW()),
    count INTEGER DEFAULT 1,
    PRIMARY KEY (banner_id, timestamp)
);

CREATE INDEX idx_clicks_banner_id ON clicks(banner_id);
CREATE INDEX idx_clicks_timestamp ON clicks(timestamp);

--
INSERT INTO banners (id, name) VALUES 
(1, 'tom'),
(2, 'bom'),
(3, 'dom'),
(4, 'rom');


// TEST:
curl http://localhost:8080/counter/1

curl -X POST http://localhost:8090/stats/3   -H "Content-Type: application/json"   -d '{"ts_from": "2023-01-01T00:00:00Z", "ts_to": "2025-12-31T23:59:59Z"}'

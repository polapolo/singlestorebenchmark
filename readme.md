# Step By Step:
- Set SingleStore ROOT_PASSWORD and LICENSE_KEY on docker-compose.yaml
- make docker
- Create RedPanda Topic
`sudo docker exec -it rp-node-0 rpk topic create orders --brokers=rp-node-0:9092 -p 10`
- Connect SQL 0.0.0.0:3306 and execute SQL
```
CREATE DATABASE benchmark PARTITIONS 10;

CREATE TABLE IF NOT EXISTS orders (
    id BIGINT AUTO_INCREMENT,
    user_id BIGINT,
    stock_code varchar(6),
    type VARCHAR(1),
    lot BIGINT,
    price int,
    status int,
    created_at DATETIME,
    PRIMARY KEY(id),
    KEY(created_at),
    KEY(id) USING hash
);

DROP PROCEDURE orders_insert_proc;
DELIMITER //
CREATE OR REPLACE PROCEDURE orders_insert_proc(batch QUERY(_user_id bigint, _stock_code varchar(6), _type varchar(1), _lot bigint, _price int, _status int)) AS
BEGIN
    INSERT INTO orders(user_id, stock_code, type, lot, price, status) SELECT b._user_id, b._stock_code, b._type, b._lot, b._price, b._status FROM batch b;
END //
DELIMITER ;

CREATE PIPELINE orders_insert_pipeline
AS LOAD DATA KAFKA "rp-node-0:9092/orders"
INTO PROCEDURE orders_insert_proc
FORMAT JSON (
    _user_id <- user_id,
    _stock_code <- stock_code,
    _type <- type,
    _lot <- lot,
    _price <- price,
    _status <- status
);
```
- make run

# Links
- SingleStore Studio (singlestore web client): http://localhost:8080
- Kafdrop (redpanda web client): http://localhost:9000
- Service Endpoint: http://localhost:8090
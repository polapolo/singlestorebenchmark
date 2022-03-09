package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	_ "github.com/go-sql-driver/mysql"
	"github.com/hamba/avro"
	"github.com/twmb/franz-go/pkg/kgo"
)

// https://www.timescale.com/blog/13-tips-to-improve-postgresql-insert-performance/
func main() {
	log.SetFlags(log.LstdFlags | log.Lshortfile)

	ctx := context.Background()

	r := gin.Default()

	r.GET("/refresh", func(c *gin.Context) {
		numOfPoolMaxConnectionString := c.DefaultQuery("numOfPoolMaxConnection", "10")
		numOfPoolMaxConnection, _ := strconv.Atoi(numOfPoolMaxConnectionString)

		db := connectDB(ctx, numOfPoolMaxConnection)
		defer db.Close()

		refreshSchema(ctx, db)

		c.JSON(200, true)
	})

	r.GET("/orders", func(c *gin.Context) {
		numOfPoolMaxConnectionString := c.DefaultQuery("numOfPoolMaxConnection", "10")
		numOfPoolMaxConnection, _ := strconv.Atoi(numOfPoolMaxConnectionString)

		db := connectDB(ctx, numOfPoolMaxConnection)
		defer db.Close()

		numOfUserIDsString := c.DefaultQuery("numOfUserIDs", "1000")
		numOfUserIDs, _ := strconv.Atoi(numOfUserIDsString)
		numOfOrdersString := c.DefaultQuery("numOfOrders", "10")
		numOfOrders, _ := strconv.Atoi(numOfOrdersString)

		result := poolInsertOrders(ctx, db, numOfUserIDs, numOfOrders)

		c.JSON(200, result)
	})

	r.GET("/trades", func(c *gin.Context) {
		numOfPoolMaxConnectionString := c.DefaultQuery("numOfPoolMaxConnection", "10")
		numOfPoolMaxConnection, _ := strconv.Atoi(numOfPoolMaxConnectionString)

		db := connectDB(ctx, numOfPoolMaxConnection)
		defer db.Close()

		numOfUserIDsString := c.DefaultQuery("numOfUserIDs", "1000")
		numOfUserIDs, _ := strconv.Atoi(numOfUserIDsString)
		numOfOrdersString := c.DefaultQuery("numOfOrders", "10")
		numOfOrders, _ := strconv.Atoi(numOfOrdersString)
		numOfTradesString := c.DefaultQuery("numOfTrades", "1")
		numOfTrades, _ := strconv.Atoi(numOfTradesString)

		result := poolInsertTrades(ctx, db, numOfUserIDs, numOfOrders, numOfTrades)

		c.JSON(200, result)
	})

	r.GET("/all", func(c *gin.Context) {
		numOfPoolMaxConnectionString := c.DefaultQuery("numOfPoolMaxConnection", "10")
		numOfPoolMaxConnection, _ := strconv.Atoi(numOfPoolMaxConnectionString)

		db := connectDB(ctx, numOfPoolMaxConnection)
		defer db.Close()

		numOfUserIDsString := c.DefaultQuery("numOfUserIDs", "1000")
		numOfUserIDs, _ := strconv.Atoi(numOfUserIDsString)
		numOfOrdersString := c.DefaultQuery("numOfOrders", "10")
		numOfOrders, _ := strconv.Atoi(numOfOrdersString)
		numOfTradesString := c.DefaultQuery("numOfTrades", "1")
		numOfTrades, _ := strconv.Atoi(numOfTradesString)

		resultA := poolInsertOrders(ctx, db, numOfUserIDs, numOfOrders)
		resultB := poolInsertTrades(ctx, db, numOfUserIDs, numOfOrders, numOfTrades)

		c.JSON(200, []interface{}{resultA, resultB})
	})

	r.GET("/publish/orders/insert/json", func(c *gin.Context) {
		numOfUserIDsString := c.DefaultQuery("numOfUserIDs", "1000")
		numOfUserIDs, _ := strconv.Atoi(numOfUserIDsString)
		numOfOrdersString := c.DefaultQuery("numOfOrders", "10")
		numOfOrders, _ := strconv.Atoi(numOfOrdersString)

		publishInsertOrderJSON(numOfUserIDs, numOfOrders)

		c.JSON(200, []interface{}{"DONE"})
	})

	r.GET("/publish/orders/upsert/json", func(c *gin.Context) {
		numOfUserIDsString := c.DefaultQuery("numOfUserIDs", "1000")
		numOfUserIDs, _ := strconv.Atoi(numOfUserIDsString)
		numOfOrdersString := c.DefaultQuery("numOfOrders", "10")
		numOfOrders, _ := strconv.Atoi(numOfOrdersString)

		publishUpsertOrderJSON(numOfUserIDs, numOfOrders)

		c.JSON(200, []interface{}{"DONE"})
	})

	r.GET("/publish/trades/insert/json", func(c *gin.Context) {
		numOfUserIDsString := c.DefaultQuery("numOfUserIDs", "1000")
		numOfUserIDs, _ := strconv.Atoi(numOfUserIDsString)
		numOfOrdersString := c.DefaultQuery("numOfOrders", "10")
		numOfOrders, _ := strconv.Atoi(numOfOrdersString)
		numOfTradesString := c.DefaultQuery("numOfTrades", "1")
		numOfTrades, _ := strconv.Atoi(numOfTradesString)

		publishInsertTradeJSON(numOfUserIDs, numOfOrders, numOfTrades)

		c.JSON(200, []interface{}{"DONE"})
	})

	r.GET("/publish/orders/insert/avro", func(c *gin.Context) {
		numOfUserIDsString := c.DefaultQuery("numOfUserIDs", "1000")
		numOfUserIDs, _ := strconv.Atoi(numOfUserIDsString)
		numOfOrdersString := c.DefaultQuery("numOfOrders", "10")
		numOfOrders, _ := strconv.Atoi(numOfOrdersString)

		publishInsertOrderAVRO(numOfUserIDs, numOfOrders)

		c.JSON(200, []interface{}{"DONE"})
	})

	r.GET("/consume/orders/avro", func(c *gin.Context) {
		redPandaClient := getRedPandaClient("orders_avro")
		defer redPandaClient.Close()

		var jsonString string

	consumerLoop:
		for {
			fetches := redPandaClient.PollFetches(ctx)
			iter := fetches.RecordIter()

			for _, fetchErr := range fetches.Errors() {
				fmt.Printf("error consuming from topic: topic=%s, partition=%d, err=%v\n",
					fetchErr.Topic, fetchErr.Partition, fetchErr.Err)
				break consumerLoop
			}

			schema := avro.MustParse(`{"type":"record","name":"order","fields":[{"name":"user_id","type":"long"},{"name":"stock_code","type":"string"},{"name":"type","type":"string"},{"name":"lot","type":"long"},{"name":"price","type":"int"},{"name":"status","type":"int"}]}`)

			for !iter.Done() {
				record := iter.Next()

				result := orderAVRO{}
				err := avro.Unmarshal(schema, record.Value, &result)
				if err != nil {
					log.Println(err)
				}
				resultJSON, _ := json.Marshal(result)
				jsonString = string(resultJSON)
				break consumerLoop
			}
		}

		c.JSON(200, []interface{}{jsonString})
	})

	r.GET("/publish/orders/upsert/avro", func(c *gin.Context) {
		numOfUserIDsString := c.DefaultQuery("numOfUserIDs", "1000")
		numOfUserIDs, _ := strconv.Atoi(numOfUserIDsString)
		numOfOrdersString := c.DefaultQuery("numOfOrders", "10")
		numOfOrders, _ := strconv.Atoi(numOfOrdersString)

		publishUpsertOrderAVRO(numOfUserIDs, numOfOrders)

		c.JSON(200, []interface{}{"DONE"})
	})

	r.GET("/publish/trades/insert/avro", func(c *gin.Context) {
		numOfUserIDsString := c.DefaultQuery("numOfUserIDs", "1000")
		numOfUserIDs, _ := strconv.Atoi(numOfUserIDsString)
		numOfOrdersString := c.DefaultQuery("numOfOrders", "10")
		numOfOrders, _ := strconv.Atoi(numOfOrdersString)
		numOfTradesString := c.DefaultQuery("numOfTrades", "1")
		numOfTrades, _ := strconv.Atoi(numOfTradesString)

		publishInsertTradeAVRO(numOfUserIDs, numOfOrders, numOfTrades)

		c.JSON(200, []interface{}{"DONE"})
	})

	r.GET("/consume/trades/avro", func(c *gin.Context) {
		redPandaClient := getRedPandaClient("trades_avro")
		defer redPandaClient.Close()

		var jsonString string

	consumerLoop:
		for {
			fetches := redPandaClient.PollFetches(ctx)
			iter := fetches.RecordIter()

			for _, fetchErr := range fetches.Errors() {
				fmt.Printf("error consuming from topic: topic=%s, partition=%d, err=%v\n",
					fetchErr.Topic, fetchErr.Partition, fetchErr.Err)
				break consumerLoop
			}

			schema := avro.MustParse(`{"type":"record","name":"trade","fields":[{"name":"order_id","type":"long"},{"name":"lot","type":"long"},{"name":"lot_multiplier","type":"int"},{"name":"price","type":"int"},{"name":"total","type":"long"},{"name":"created_at","type":"string"}]}`)

			for !iter.Done() {
				record := iter.Next()

				result := tradeAVRO{}
				err := avro.Unmarshal(schema, record.Value, &result)
				if err != nil {
					log.Println(err)
				}
				resultJSON, _ := json.Marshal(result)
				jsonString = string(resultJSON)
				break consumerLoop
			}
		}

		c.JSON(200, []interface{}{jsonString})
	})

	r.Run(":8090")
}

func connectDB(ctx context.Context, numOfPoolMaxConnection int) *sql.DB {
	HOSTNAME := ""
	PORT := ""
	USERNAME := ""
	PASSWORD := ""
	DATABASE := ""

	connection := USERNAME + ":" + PASSWORD + "@tcp(" + HOSTNAME + ":" + PORT + ")/" + DATABASE + "?parseTime=true"
	db, err := sql.Open("mysql", connection)
	if err != nil {
		log.Fatal(err)
	}

	db.SetMaxOpenConns(numOfPoolMaxConnection)

	return db
}

func refreshSchema(ctx context.Context, db *sql.DB) {
	_, err := db.ExecContext(ctx, `DROP TABLE IF EXISTS orders`)
	if err != nil {
		log.Fatalln(err)
	}

	// orders
	_, err = db.ExecContext(ctx, `CREATE ROWSTORE TABLE IF NOT EXISTS orders (
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
		SHARD(id),
		KEY(id) USING hash
	)`)
	if err != nil {
		log.Fatalln(err)
	}

	_, err = db.ExecContext(ctx, `DROP TABLE IF EXISTS trades`)
	if err != nil {
		log.Fatalln(err)
	}

	// trades table
	_, err = db.ExecContext(ctx, `CREATE TABLE IF NOT EXISTS trades (
		order_id BIGINT,
		lot BIGINT,
		lot_multiplier int,
		price int,
		total BIGINT,
		created_at DATETIME,
		KEY(created_at),
		SHARD(order_id),
		KEY(order_id) USING hash
	)`)
	if err != nil {
		log.Fatalln(err)
	}

	_, err = db.ExecContext(ctx, `DROP TABLE IF EXISTS initial_cash`)
	if err != nil {
		log.Fatalln(err)
	}

	_, err = db.ExecContext(ctx, `CREATE ROWSTORE TABLE IF NOT EXISTS initial_cash (
		id BIGINT AUTO_INCREMENT,
		user_id bigint,
		cash_on_hand bigint,
		PRIMARY KEY(id)
	)`)
	if err != nil {
		log.Fatalln(err)
	}
}

func generateInsertOrderQueries(numOfUserIDs int, numOfOrders int) []string {
	queries := make([]string, 0)
	// users
	for i := 1; i <= numOfUserIDs; i++ {
		// orders
		for j := 1; j <= numOfOrders; j++ {
			orderType := "B"
			if j%2 == 0 {
				orderType = "S"
			}

			offset := numOfOrders * (i - 1)

			query := `INSERT INTO orders(id, user_id, stock_code, type, lot, price, status) VALUES (` + strconv.Itoa(j+offset) + `, ` + strconv.Itoa(i) + `,'BBCA','` + orderType + `',10,1000,1);`
			queries = append(queries, query)
		}
	}

	return queries
}

func generateInsertTradeQueries(numOfUserIDs int, numOfOrders int, numOfTrades int) []string {
	queries := make([]string, 0)
	// users
	for i := 1; i <= numOfUserIDs; i++ {
		// orders
		for j := 1; j <= numOfOrders; j++ {
			offset := numOfOrders * (i - 1)

			// trades
			for k := 1; k <= numOfTrades; k++ {
				queries = append(queries, `INSERT INTO trades(order_id, lot, lot_multiplier, price, total, created_at) VALUES (`+strconv.Itoa(j+offset)+`,10,100,1000,1000000,'`+time.Now().Format(time.RFC3339)+`');`)
			}
		}
	}

	return queries
}

type order struct {
	ID        int64  `json:"id,omitempty" avro:"id,omitempty"`
	UserID    int64  `json:"user_id" avro:"user_id"`
	StockCode string `json:"stock_code" avro:"stock_code"`
	Type      string `json:"type" avro:"type"`
	Lot       int64  `json:"lot" avro:"lot"`
	Price     int    `json:"price" avro:"price"`
	Status    int    `json:"status" avro:"status"`
}

func generateInsertOrderJSON(numOfUserIDs int, numOfOrders int) [][]byte {
	orderJSONs := make([][]byte, 0)
	// users
	for i := 1; i <= numOfUserIDs; i++ {
		// orders
		for j := 1; j <= numOfOrders; j++ {
			orderType := "B"
			if j%2 == 0 {
				orderType = "S"
			}

			orderJSON, err := json.Marshal(order{
				UserID:    int64(i),
				StockCode: "BBCA",
				Type:      orderType,
				Lot:       10,
				Price:     1000,
				Status:    1,
			})
			if err != nil {
				log.Println(err)
				continue
			}

			orderJSONs = append(orderJSONs, orderJSON)
		}
	}

	return orderJSONs
}

func generateUpsertOrderJSON(numOfUserIDs int, numOfOrders int) [][]byte {
	orderJSONs := make([][]byte, 0)
	// users
	for i := 1; i <= numOfUserIDs; i++ {
		// orders
		for j := 1; j <= numOfOrders; j++ {
			orderType := "B"
			if j%2 == 0 {
				orderType = "S"
			}

			offset := numOfOrders * (i - 1)

			orderJSON, err := json.Marshal(order{
				ID:        int64(j + offset),
				UserID:    int64(i),
				StockCode: "BBCA",
				Type:      orderType,
				Lot:       10,
				Price:     1000,
				Status:    1,
			})
			if err != nil {
				log.Println(err)
				continue
			}

			orderJSONs = append(orderJSONs, orderJSON)
		}
	}

	return orderJSONs
}

func generateUpsertOrderAVRO(numOfUserIDs int, numOfOrders int) [][]byte {
	schema, err := avro.Parse(`{"type":"record","name":"order","fields":[{"name":"id","type":"long"},{"name":"user_id","type":"long"},{"name":"stock_code","type":"string"},{"name":"type","type":"string"},{"name":"lot","type":"long"},{"name":"price","type":"int"},{"name":"status","type":"int"}]}`)
	if err != nil {
		log.Fatal(err)
	}

	orderAVROs := make([][]byte, 0)
	// users
	for i := 1; i <= numOfUserIDs; i++ {
		// orders
		for j := 1; j <= numOfOrders; j++ {
			orderType := "B"
			if j%2 == 0 {
				orderType = "S"
			}

			offset := numOfOrders * (i - 1)

			orderAVRO, err := avro.Marshal(
				schema,
				orderAVROUpsert{
					ID:        int64(j + offset),
					UserID:    int64(i),
					StockCode: "BBCA",
					Type:      orderType,
					Lot:       10,
					Price:     1000,
					Status:    1,
				},
			)
			if err != nil {
				log.Println(err)
				continue
			}

			orderAVROs = append(orderAVROs, orderAVRO)
		}
	}

	return orderAVROs
}

type trade struct {
	OrderID       int64  `json:"order_id"`
	Lot           int64  `json:"lot"`
	LotMultiplier int    `json:"lot_multiplier"`
	Price         int    `json:"price"`
	Total         int64  `json:"total"`
	CreatedAt     string `json:"created_at"`
}

func generateInsertTradeJSON(numOfUserIDs int, numOfOrders int, numOfTrades int) [][]byte {
	tradeJSONs := make([][]byte, 0)
	// users
	for i := 1; i <= numOfUserIDs; i++ {
		// orders
		for j := 1; j <= numOfOrders; j++ {
			offset := numOfOrders * (i - 1)

			// trades
			for k := 1; k <= numOfTrades; k++ {
				tradeJSON, err := json.Marshal(trade{
					OrderID:       int64(j + offset),
					Lot:           10,
					LotMultiplier: 100,
					Price:         1000,
					Total:         1000000,
					CreatedAt:     time.Now().Format(time.RFC3339),
				})
				if err != nil {
					log.Println(err)
					continue
				}

				tradeJSONs = append(tradeJSONs, tradeJSON)
			}
		}
	}

	return tradeJSONs
}

type orderAVRO struct {
	UserID    int64  `avro:"user_id"`
	StockCode string `avro:"stock_code"`
	Type      string `avro:"type"`
	Lot       int64  `avro:"lot"`
	Price     int    `avro:"price"`
	Status    int    `avro:"status"`
}

type orderAVROUpsert struct {
	ID        int64  `avro:"id"`
	UserID    int64  `avro:"user_id"`
	StockCode string `avro:"stock_code"`
	Type      string `avro:"type"`
	Lot       int64  `avro:"lot"`
	Price     int    `avro:"price"`
	Status    int    `avro:"status"`
}

func generateInsertOrderAVRO(numOfUserIDs int, numOfOrders int) [][]byte {
	schema, err := avro.Parse(`{"type":"record","name":"order","fields":[{"name":"user_id","type":"long"},{"name":"stock_code","type":"string"},{"name":"type","type":"string"},{"name":"lot","type":"long"},{"name":"price","type":"int"},{"name":"status","type":"int"}]}`)
	if err != nil {
		log.Fatal(err)
	}

	orderAVROs := make([][]byte, 0)
	// users
	for i := 1; i <= numOfUserIDs; i++ {
		// orders
		for j := 1; j <= numOfOrders; j++ {
			orderType := "B"
			if j%2 == 0 {
				orderType = "S"
			}

			orderAVRO, err := avro.Marshal(
				schema,
				orderAVRO{
					UserID:    int64(i),
					StockCode: "BBCA",
					Type:      orderType,
					Lot:       10,
					Price:     1000,
					Status:    1,
				},
			)
			if err != nil {
				log.Println(err)
				continue
			}

			orderAVROs = append(orderAVROs, orderAVRO)
		}
	}

	return orderAVROs
}

type tradeAVRO struct {
	OrderID       int64  `avro:"order_id"`
	Lot           int64  `avro:"lot"`
	LotMultiplier int    `avro:"lot_multiplier"`
	Price         int    `avro:"price"`
	Total         int64  `avro:"total"`
	CreatedAt     string `avro:"created_at"`
}

func generateInsertTradeAVRO(numOfUserIDs int, numOfOrders int, numOfTrades int) [][]byte {
	schema, err := avro.Parse(`{"type":"record","name":"trade","fields":[{"name":"order_id","type":"long"},{"name":"lot","type":"long"},{"name":"lot_multiplier","type":"int"},{"name":"price","type":"int"},{"name":"total","type":"long"},{"name":"created_at","type":"string"}]}`)
	if err != nil {
		log.Fatal(err)
	}

	tradeAVROs := make([][]byte, 0)
	// users
	for i := 1; i <= numOfUserIDs; i++ {
		// orders
		for j := 1; j <= numOfOrders; j++ {
			offset := numOfOrders * (i - 1)

			// trades
			for k := 1; k <= numOfTrades; k++ {
				tradeAVRO, err := avro.Marshal(
					schema,
					tradeAVRO{
						OrderID:       int64(j + offset),
						Lot:           10,
						LotMultiplier: 100,
						Price:         1000,
						Total:         1000000,
						CreatedAt:     time.Now().Format(time.RFC3339),
					},
				)
				if err != nil {
					log.Println(err)
					continue
				}

				tradeAVROs = append(tradeAVROs, tradeAVRO)
			}
		}
	}

	return tradeAVROs
}

func getRedPandaHosts() []string {
	return []string{
		"127.0.0.1:9093",
	}
}

func getRedPandaClient(topicName string) *kgo.Client {
	redPandaClient, err := kgo.NewClient(
		kgo.SeedBrokers(getRedPandaHosts()...),
		kgo.ConsumeTopics(topicName),
	)
	if err != nil {
		log.Println(err)
		panic(err)
	}

	return redPandaClient
}

func publishInsertOrderJSON(numOfUserIDs int, numOfOrders int) {
	redPandaClient := getRedPandaClient("orders")
	defer redPandaClient.Close()

	jsons := generateInsertOrderJSON(numOfUserIDs, numOfOrders)

	// var wg sync.WaitGroup
	// wg.Add(len(jsons))

	// for _, myJSON := range jsons {
	// 	myRecord := &kgo.Record{Topic: "orders", Value: myJSON}

	// 	redPandaClient.Produce(context.Background(), myRecord, func(_ *kgo.Record, err error) {
	// 		defer wg.Done()
	// 		if err != nil {
	// 			fmt.Printf("record had a produce error: %v\n", err)
	// 		}

	// 	})
	// }
	// wg.Wait()

	records := make([]*kgo.Record, 0)
	for _, myJSON := range jsons {
		records = append(records, &kgo.Record{Topic: "orders", Value: myJSON})
	}

	err := redPandaClient.ProduceSync(context.Background(), records...).FirstErr()
	if err != nil {
		log.Println(err)
	}
}

func publishInsertTradeJSON(numOfUserIDs int, numOfOrders int, numOfTrades int) {
	redPandaClient := getRedPandaClient("trades")
	defer redPandaClient.Close()

	jsons := generateInsertTradeJSON(numOfUserIDs, numOfOrders, numOfTrades)

	records := make([]*kgo.Record, 0)
	for _, myJSON := range jsons {
		records = append(records, &kgo.Record{Topic: "trades", Value: myJSON})
	}

	err := redPandaClient.ProduceSync(context.Background(), records...).FirstErr()
	if err != nil {
		log.Println(err)
	}
}

func publishUpsertOrderJSON(numOfUserIDs int, numOfOrders int) {
	redPandaClient := getRedPandaClient("orders_upsert")
	defer redPandaClient.Close()

	jsons := generateUpsertOrderJSON(numOfUserIDs, numOfOrders)

	records := make([]*kgo.Record, 0)
	for _, myJSON := range jsons {
		records = append(records, &kgo.Record{Topic: "orders_upsert", Value: myJSON})
	}

	err := redPandaClient.ProduceSync(context.Background(), records...).FirstErr()
	if err != nil {
		log.Println(err)
	}
}

func publishUpsertOrderAVRO(numOfUserIDs int, numOfOrders int) {
	redPandaClient := getRedPandaClient("orders_upsert_avro")
	defer redPandaClient.Close()

	avros := generateUpsertOrderAVRO(numOfUserIDs, numOfOrders)

	records := make([]*kgo.Record, 0)
	for _, myAVRO := range avros {
		records = append(records, &kgo.Record{Topic: "orders_upsert_avro", Value: myAVRO})
	}

	err := redPandaClient.ProduceSync(context.Background(), records...).FirstErr()
	if err != nil {
		log.Println(err)
	}
}

func publishInsertOrderAVRO(numOfUserIDs int, numOfOrders int) {
	redPandaClient := getRedPandaClient("orders_avro")
	defer redPandaClient.Close()

	avros := generateInsertOrderAVRO(numOfUserIDs, numOfOrders)

	records := make([]*kgo.Record, 0)
	for _, myAVRO := range avros {
		records = append(records, &kgo.Record{Topic: "orders_avro", Value: myAVRO})
	}

	err := redPandaClient.ProduceSync(context.Background(), records...).FirstErr()
	if err != nil {
		log.Println(err)
	}
}

func publishInsertTradeAVRO(numOfUserIDs int, numOfOrders int, numOfTrades int) {
	redPandaClient := getRedPandaClient("trades_avro")
	defer redPandaClient.Close()

	avros := generateInsertTradeAVRO(numOfUserIDs, numOfOrders, numOfTrades)

	records := make([]*kgo.Record, 0)
	for _, myAVRO := range avros {
		records = append(records, &kgo.Record{Topic: "trades_avro", Value: myAVRO})
	}

	err := redPandaClient.ProduceSync(context.Background(), records...).FirstErr()
	if err != nil {
		log.Println(err)
	}
}

type result struct {
	WorkerID  int
	SpeedInMs int64
}

type summary struct {
	TotalTimeInMs         int64
	AvgSpeedInMicroSecond int64
	RecordPerSecond       float64
}

func poolInsertOrders(ctx context.Context, db *sql.DB, numOfUserIDs int, numOfOrders int) summary {
	queries := generateInsertOrderQueries(numOfUserIDs, numOfOrders)

	startTime := time.Now()

	totalTask := len(queries)

	resultC := make(chan result, totalTask)

	for i := 0; i < totalTask; i++ {
		go func(workerID int, query string) {
			_, err := db.ExecContext(ctx, query)
			if err != nil {
				log.Println(query, err)
			}

			resultC <- result{
				// WorkerID:      workerID,
				// SpeedInSecond: timeElapsedInside.Seconds(),
			}

		}(i, queries[i])
	}

	for i := 0; i < totalTask; i++ {
		_ = <-resultC
	}

	timeElapsed := time.Since(startTime)
	log.Println("Total Time:", timeElapsed.Milliseconds(), "ms")
	log.Println("Avg speed:", timeElapsed.Microseconds()/int64(totalTask), "microsecond")
	log.Println("Record/s:", float64(totalTask)/timeElapsed.Seconds())

	return summary{
		TotalTimeInMs:         timeElapsed.Milliseconds(),
		AvgSpeedInMicroSecond: timeElapsed.Microseconds() / int64(totalTask),
		RecordPerSecond:       float64(totalTask) / timeElapsed.Seconds(),
	}
}

func poolInsertTrades(ctx context.Context, db *sql.DB, numOfUserIDs int, numOfOrders int, numOfTrades int) summary {
	queries := generateInsertTradeQueries(numOfUserIDs, numOfOrders, numOfTrades)

	startTime := time.Now()

	totalTask := len(queries)

	resultC := make(chan result, totalTask)

	for i := 0; i < totalTask; i++ {
		go func(workerID int, query string) {
			_, err := db.ExecContext(ctx, query)
			if err != nil {
				log.Println(query, err)
			}

			resultC <- result{
				// WorkerID:      workerID,
				// SpeedInSecond: timeElapsedInside.Seconds(),
			}

		}(i, queries[i])
	}

	for i := 0; i < totalTask; i++ {
		_ = <-resultC
	}

	timeElapsed := time.Since(startTime)
	log.Println("Total Time:", timeElapsed.Milliseconds(), "ms")
	log.Println("Avg speed:", timeElapsed.Microseconds()/int64(totalTask), "microsecond")
	log.Println("Record/s:", float64(totalTask)/timeElapsed.Seconds())

	return summary{
		TotalTimeInMs:         timeElapsed.Milliseconds(),
		AvgSpeedInMicroSecond: timeElapsed.Microseconds() / int64(totalTask),
		RecordPerSecond:       float64(totalTask) / timeElapsed.Seconds(),
	}
}

// func concurrentInsertOrders(ctx context.Context, db *sql.DB) {
// 	orderQueries := generateInsertOrderQueries()

// 	startTime := time.Now()

// 	insertWorkerPool := pkg.NewWorkerPool(numOfWorkers)
// 	insertWorkerPool.Run()

// 	totalTask := len(orderQueries)
// 	resultC := make(chan result, totalTask)

// 	for i := 0; i < totalTask; i++ {
// 		query := orderQueries[i]

// 		insertWorkerPool.AddTask(func() {
// 			_, err := db.ExecContext(ctx, query)
// 			if err != nil {
// 				log.Fatalln(query, err)
// 			}

// 			resultC <- result{
// 				// WorkerID:  id,
// 				// SpeedInMs: timeElapsed.Microseconds(),
// 			}
// 		})
// 	}

// 	for i := 0; i < totalTask; i++ {
// 		_ = <-resultC
// 	}

// 	timeElapsed := time.Since(startTime)
// 	log.Println("Total Time:", timeElapsed.Milliseconds(), "ms")
// 	log.Println("Avg speed:", timeElapsed.Microseconds()/int64(totalTask), "microsecond")
// 	log.Println("Record/s:", float64(totalTask)/timeElapsed.Seconds())
// }

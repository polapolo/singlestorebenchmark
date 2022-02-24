package main

import (
	"context"
	"database/sql"
	"log"
	"strconv"
	"time"

	_ "github.com/go-sql-driver/mysql"

	"github.com/polapolo/singlestorebenchmark/pkg"
)

const numOfUserIDs = 100000 // number of users
const numOfOrders = 10      // order matched per user
const numOfTrades = 1       // trade matched per order

const numOfPoolMaxConnection = 10
const numOfWorkers = 10

// https://www.timescale.com/blog/13-tips-to-improve-postgresql-insert-performance/
func main() {
	log.SetFlags(log.LstdFlags | log.Lshortfile)

	ctx := context.Background()

	db := connectDB(ctx)
	defer db.Close()

	refreshSchema(ctx, db)
	poolInsertOrders(ctx, db)
	// concurrentInsertOrders(ctx, db)
}

func connectDB(ctx context.Context) *sql.DB {
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
	// db.SetMaxIdleConns(0)
	// db.SetConnMaxLifetime(60 * time.Minute)

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
		PRIMARY KEY(id)
	)`)
	if err != nil {
		log.Fatalln(err)
	}

	// trades table with foreign key
	_, err = db.ExecContext(ctx, `CREATE ROWSTORE TABLE IF NOT EXISTS trades (
		id BIGINT AUTO_INCREMENT,
		order_id BIGINT,
		lot BIGINT,
		lot_multiplier int,
		price int,
		total BIGINT,
		created_at DATETIME,
		PRIMARY KEY(id)
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

func generateInsertOrderQueries() []string {
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

type result struct {
	WorkerID  int
	SpeedInMs int64
}

func poolInsertOrders(ctx context.Context, db *sql.DB) {
	queries := generateInsertOrderQueries()

	startTime := time.Now()

	totalTask := len(queries)

	resultC := make(chan result, totalTask)

	for i := 0; i < totalTask; i++ {
		go func(workerID int, query string) {
			_, err := db.ExecContext(ctx, query)
			if err != nil {
				log.Fatalln(query, err)
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
}

func concurrentInsertOrders(ctx context.Context, db *sql.DB) {
	orderQueries := generateInsertOrderQueries()

	startTime := time.Now()

	insertWorkerPool := pkg.NewWorkerPool(numOfWorkers)
	insertWorkerPool.Run()

	totalTask := len(orderQueries)
	resultC := make(chan result, totalTask)

	for i := 0; i < totalTask; i++ {
		query := orderQueries[i]

		insertWorkerPool.AddTask(func() {
			_, err := db.ExecContext(ctx, query)
			if err != nil {
				log.Fatalln(query, err)
			}

			resultC <- result{
				// WorkerID:  id,
				// SpeedInMs: timeElapsed.Microseconds(),
			}
		})
	}

	for i := 0; i < totalTask; i++ {
		_ = <-resultC
	}

	timeElapsed := time.Since(startTime)
	log.Println("Total Time:", timeElapsed.Milliseconds(), "ms")
	log.Println("Avg speed:", timeElapsed.Microseconds()/int64(totalTask), "microsecond")
	log.Println("Record/s:", float64(totalTask)/timeElapsed.Seconds())
}

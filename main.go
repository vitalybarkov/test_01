package main

import (
	"context"
	"database/sql"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/gin-gonic/gin"
	_ "github.com/lib/pq"
)

type Click struct {
	BannerID  int       `json:"banner_id"`
	Timestamp time.Time `json:"timestamp"`
	Count     int       `json:"count"`
}

type ClickBatch struct {
	BannerID int
	Count    int
}

type StatsRequest struct {
	TsFrom time.Time `json:"ts_from" binding:"required"`
	TsTo   time.Time `json:"ts_to" binding:"required"`
}

var (
	db          *sql.DB
	clickChan   chan ClickBatch
	batchSize   = 100
	flushPeriod = 1 * time.Second
)

func main() {
	// parse command line arguments
	port := flag.String("port", "8080", "Port to listen on")
	flag.Parse()

	// init db
	initDB()
	defer db.Close()

	// channel for click batching
	clickChan = make(chan ClickBatch, 10000)

	// launching a worker for batching
	go batchWorker()

	// setting up an HTTP server
	router := gin.Default()
	router.GET("/counter/:bannerID", handleCounter)
	router.POST("/stats/:bannerID", handleStats)

	// graceful shutdown
	server := &http.Server{
		Addr:    ":" + *port, // use the configured port
		Handler: router,
	}

	log.Printf("Starting server on port %s", *port)

	go func() {
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("listen: %s\n", err)
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit
	log.Println("Shutting down server...")

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := server.Shutdown(ctx); err != nil {
		log.Fatal("Server forced to shutdown:", err)
	}

	close(clickChan)
	log.Println("Server exiting")
}

func initDB() {
	var err error
	connStr := "user=vitaly dbname=vitaly sslmode=disable password=4560"
	db, err = sql.Open("postgres", connStr)
	if err != nil {
		log.Fatal(err)
	}

	err = db.Ping()
	if err != nil {
		log.Fatal(err)
	}

	// optimizing connections
	db.SetMaxOpenConns(25)
	db.SetMaxIdleConns(25)
	db.SetConnMaxLifetime(5 * time.Minute)
}

//

func batchWorker() {
	var batch []ClickBatch
	var mu sync.Mutex
	ticker := time.NewTicker(flushPeriod)

	for {
		select {
		case click, ok := <-clickChan:
			if !ok {
				flushBatch(&batch, &mu)
				return
			}

			mu.Lock()
			batch = append(batch, click)
			if len(batch) >= batchSize {
				flushBatch(&batch, &mu)
			}
			mu.Unlock()

		case <-ticker.C:
			flushBatch(&batch, &mu)
		}
	}
}

func flushBatch(batch *[]ClickBatch, mu *sync.Mutex) {
	mu.Lock()
	if len(*batch) == 0 {
		mu.Unlock()
		return
	}

	currentBatch := *batch
	*batch = nil
	mu.Unlock()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		log.Printf("Error starting transaction: %v\n", err)
		return
	}

	stmt, err := tx.PrepareContext(ctx, `
		INSERT INTO clicks (banner_id, timestamp, count)
		VALUES ($1, date_trunc('minute', NOW()), $2)
		ON CONFLICT (banner_id, timestamp)
		DO UPDATE SET count = clicks.count + EXCLUDED.count`)
	if err != nil {
		log.Printf("Error preparing statement: %v\n", err)
		tx.Rollback()
		return
	}

	for _, click := range currentBatch {
		_, err := stmt.ExecContext(ctx, click.BannerID, click.Count)
		if err != nil {
			log.Printf("Error executing statement: %v\n", err)
			tx.Rollback()
			return
		}
	}

	if err := tx.Commit(); err != nil {
		log.Printf("Error committing transaction: %v\n", err)
	}
}

// endpoints:

func handleCounter(c *gin.Context) {
	bannerID := c.Param("bannerID")
	var id int
	_, err := fmt.Sscanf(bannerID, "%d", &id)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid banner ID"})
		return
	}

	// async click handling
	clickChan <- ClickBatch{BannerID: id, Count: 1}

	c.Status(http.StatusOK)
}

func handleStats(c *gin.Context) {
	bannerID := c.Param("bannerID")
	var id int
	_, err := fmt.Sscanf(bannerID, "%d", &id)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid banner ID"})
		return
	}

	var req StatsRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	rows, err := db.Query(`
		SELECT timestamp, count 
		FROM clicks 
		WHERE banner_id = $1 
		AND timestamp BETWEEN $2 AND $3
		ORDER BY timestamp`,
		id, req.TsFrom, req.TsTo)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	defer rows.Close()

	var stats []Click
	for rows.Next() {
		var click Click
		if err := rows.Scan(&click.Timestamp, &click.Count); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		stats = append(stats, click)
	}

	c.JSON(http.StatusOK, gin.H{"stats": stats})
}

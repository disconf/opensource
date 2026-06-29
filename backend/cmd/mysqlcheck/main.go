package main

import (
	"context"
	"database/sql"
	"flag"
	"fmt"
	"os"
	"strings"
	"time"

	_ "github.com/go-sql-driver/mysql"
)

func main() {
	setup := flag.Bool("setup", false, "create database and import init sql")
	flag.Parse()

	if *setup {
		runSetup()
		return
	}

	dsn := os.Getenv("MYSQL_DSN")
	if dsn == "" {
		fmt.Println("MYSQL_DSN is required")
		return
	}

	db, err := sql.Open("mysql", dsn)
	if err != nil {
		fmt.Println("open failed:", err)
		return
	}
	defer db.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	start := time.Now()
	err = db.PingContext(ctx)
	if err != nil {
		fmt.Printf("ping failed after %s: %T: %v\n", time.Since(start).Round(time.Millisecond), err, err)
		return
	}
	fmt.Printf("ping ok after %s\n", time.Since(start).Round(time.Millisecond))
}

func runSetup() {
	rootDSN := os.Getenv("MYSQL_ROOT_DSN")
	if rootDSN == "" {
		fmt.Println("MYSQL_ROOT_DSN is required")
		return
	}
	database := getenv("MYSQL_DATABASE", "tech_blog")
	initFile := getenv("MYSQL_INIT_FILE", "..\\mysql\\init.sql")

	db, err := sql.Open("mysql", rootDSN)
	if err != nil {
		fmt.Println("open failed:", err)
		return
	}
	defer db.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err := db.PingContext(ctx); err != nil {
		fmt.Println("root ping failed:", err)
		return
	}
	fmt.Println("root ping ok")

	if _, err := db.ExecContext(ctx, "CREATE DATABASE IF NOT EXISTS `"+database+"` CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci"); err != nil {
		fmt.Println("create database failed:", err)
		return
	}
	fmt.Println("database ready:", database)

	sqlBytes, err := os.ReadFile(initFile)
	if err != nil {
		fmt.Println("read init sql failed:", err)
		return
	}
	sqlText := "USE `" + database + "`;\n" + strings.TrimSpace(string(sqlBytes))
	if _, err := db.ExecContext(ctx, sqlText); err != nil {
		fmt.Println("import init sql failed:", err)
		return
	}
	fmt.Println("init sql imported")

	var count int
	if err := db.QueryRowContext(ctx, "SELECT COUNT(*) FROM `"+database+"`.posts").Scan(&count); err != nil {
		fmt.Println("count posts failed:", err)
		return
	}
	fmt.Println("posts:", count)
}

func getenv(key, fallback string) string {
	value := os.Getenv(key)
	if value == "" {
		return fallback
	}
	return value
}

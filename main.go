package main

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"os/exec"
	"time"

	_ "github.com/codenotary/immudb/pkg/stdlib"
	_ "github.com/lib/pq"
	tb "github.com/tigerbeetle/tigerbeetle-go"
	tbt "github.com/tigerbeetle/tigerbeetle-go/pkg/types"
)

const insertCount = 1000

func main() {

	fmt.Println(" --> Connecting to PostgreSQL...")
	pgDB, err := sql.Open("postgres", "host=localhost port=5432 user=test password=test dbname=ledger sslmode=disable")
	check(err)
	defer pgDB.Close()

	fmt.Println(" --> Connecting to immudb...")
	immu, err := sql.Open("immudb", "immudb://immudb:immudb@127.0.0.1:3322/defaultdb?sslmode=disable")
	check(err)
	defer immu.Close()

	fmt.Println(" --> Connecting to TigerBeetle...")

	cmd := exec.Command("./tigerbeetle", "format", "--cluster=0", "--replica=0", "--replica-count=1", "--development", "0_0.tigerbeetle")
	if err := cmd.Start(); err != nil {
		log.Fatalf("Erro ao formatar TigerBeetle: %v", err)
	}

	cmd = exec.Command("./tigerbeetle", "start", "--addresses=3000", "--development", "0_0.tigerbeetle")
	if err := cmd.Start(); err != nil {
		log.Fatalf("Erro ao iniciar TigerBeetle: %v", err)
	}

	time.Sleep(2 * time.Second)

	tbClient, err := tb.NewClient(tbt.ToUint128(0), []string{"3000"})
	if err != nil {
		log.Fatalf("Erro ao criar client: %v", err)
	}
	defer tbClient.Close()

	fmt.Println("\nRunning benchmarks...")
	runPostgresBenchmark(pgDB)
	runImmudbBenchmark(immu)
	runTigerBeetleBenchmark(tbClient)
}

func runPostgresBenchmark(db *sql.DB) {
	fmt.Println(" --- PostgreSQL Benchmark --- ")

	_, err := db.ExecContext(context.Background(), `DELETE FROM accounts;`)
	check(err)

	start := time.Now()
	for i := 0; i < insertCount; i++ {
		_, err := db.Exec(`INSERT INTO accounts (id, user_data, timestamp) VALUES ($1, $2, $3) ON CONFLICT DO NOTHING`,
			i, i*10, time.Now().Unix())
		check(err)
	}
	elapsed := time.Since(start)
	fmt.Printf("Insert %d accounts: %v\n", insertCount, elapsed)

	start = time.Now()
	for i := 0; i < insertCount; i++ {
		row := db.QueryRow(`SELECT id, user_data FROM accounts WHERE id = $1`, i)
		var id, userData int64
		_ = row.Scan(&id, &userData)
	}
	elapsed = time.Since(start)
	fmt.Printf("Read %d accounts: %v\n", insertCount, elapsed)
}

func runImmudbBenchmark(db *sql.DB) {

	_, err := db.ExecContext(context.Background(), `
		CREATE TABLE IF NOT EXISTS accounts (
			id INTEGER,
			user_data INTEGER,
			data_hora INTEGER,
			PRIMARY KEY id
		);
	`, nil)
	check(err)

	_, err = db.ExecContext(context.Background(), `DELETE FROM accounts;`)
	check(err)

	fmt.Println(" --- immudb Benchmark --- ")
	start := time.Now()
	for i := 0; i < insertCount; i++ {
		_, err := db.ExecContext(context.Background(), fmt.Sprintf(
			`INSERT INTO accounts (id, user_data, data_hora) VALUES (%d, %d, %d);`,
			i, i*10, time.Now().Unix()), nil)
		check(err)
	}
	elapsed := time.Since(start)
	fmt.Printf("Insert %d accounts: %v\n", insertCount, elapsed)

	start = time.Now()
	for i := 0; i < insertCount; i++ {
		_, err := db.ExecContext(context.Background(), fmt.Sprintf(`SELECT id, user_data FROM accounts WHERE id = %d;`, i), nil, false)
		check(err)
	}
	elapsed = time.Since(start)
	fmt.Printf("Read %d accounts: %v\n", insertCount, elapsed)
}

func runTigerBeetleBenchmark(client tb.Client) {
	fmt.Println(" --- TigerBeetle Benchmark --- ")

	const maxBatchSize = 60
	start := time.Now()

	// Gerador de IDs e Códigos únicos por execução
	base := uint64(time.Now().UnixNano()) % 1_000_000_000

	// Insert in batches
	for i := 0; i < insertCount; i += maxBatchSize {
		end := i + maxBatchSize
		if end > insertCount {
			end = insertCount
		}

		batch := make([]tbt.Account, end-i)
		for j := 0; j < len(batch); j++ {
			index := base + uint64(i+j)
			batch[j] = tbt.Account{
				ID:     uint64ToUint128(index),
				Ledger: uint32(1),
				Code:   uint16(index % 65535), // uint16 limite
			}
		}

		_, err := client.CreateAccounts(batch)
		if err != nil {
			log.Fatalf("TigerBeetle CreateAccounts failed: %v", err)
		}
	}

	elapsed := time.Since(start)
	fmt.Printf(">> Insert %d accounts: %v\n", insertCount, elapsed)

	// Prepare lookup IDs
	ids := make([]tbt.Uint128, insertCount)
	for i := 0; i < insertCount; i++ {
		ids[i] = uint64ToUint128(base + uint64(i))
	}

	start = time.Now()
	// Lookup in batches
	for i := 0; i < insertCount; i += maxBatchSize {
		end := i + maxBatchSize
		if end > insertCount {
			end = insertCount
		}
		_, err := client.LookupAccounts(ids[i:end])
		check(err)
	}
	elapsed = time.Since(start)
	fmt.Printf(">> Read %d accounts: %v\n", insertCount, elapsed)
}

func uint64ToUint128(x uint64) tbt.Uint128 {
	s := fmt.Sprintf("%016x", x)
	u, err := tbt.HexStringToUint128(s)
	if err != nil {
		log.Fatal(err)
	}
	return u
}

func check(err error) {
	if err != nil {
		log.Fatal(err)
	}
}

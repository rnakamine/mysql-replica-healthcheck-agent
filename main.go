package main

import (
	"database/sql"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"time"

	_ "github.com/go-sql-driver/mysql"
	"github.com/rnakamine/mysql-replica-healthcheck-agent/config"
)

var Version string

func main() {
	var (
		showVersion bool
		configPath  string
		port        int
	)
	flag.BoolVar(&showVersion, "version", false, "show version")
	flag.IntVar(&port, "port", 5000, "listen port number")
	flag.StringVar(&configPath, "config", "/etc/mysql-slave-healthcheck-agent/replicas.conf", "config file path")
	flag.Parse()

	if showVersion {
		fmt.Printf("version %s\n", Version)
		return
	}

	config, err := config.New(configPath)
	if err != nil {
		log.Fatalf("Failed to read config: %v", err)
	}

	log.Printf("Listening on port %d", port)

	for name, replicaConfig := range *config {
		http.HandleFunc("/"+name, handlerFunc(&replicaConfig))
	}

	addr := fmt.Sprintf(":%d", port)
	srv := &http.Server{
		Addr:        addr,
		Handler:     http.DefaultServeMux,
		ReadTimeout: 10 * time.Second,
	}
	log.Fatal(srv.ListenAndServe())
}

func handlerFunc(config *config.ReplicaConfig) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		dsn := fmt.Sprintf("%s:%s@tcp(%s:%d)/", config.User, config.Password, config.Host, config.Port)
		db, err := sql.Open("mysql", dsn)
		if err != nil {
			serverError(w, err)
			return
		}
		defer db.Close()
		slaveInfo, err := innerHandler(config, db)
		if err != nil {
			serverError(w, err)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		encoder := json.NewEncoder(w)
		if err := encoder.Encode(slaveInfo); err != nil {
			serverError(w, err)
			return
		}
	}
}

func innerHandler(config *config.ReplicaConfig, db *sql.DB) (map[string]interface{}, error) {
	rows, err := db.Query("SHOW REPLICA STATUS")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	if !rows.Next() {
		return nil, errors.New("no slave status")
	}

	columns, err := rows.Columns()
	if err != nil {
		return nil, err
	}

	values := make([]sql.RawBytes, len(columns))
	scanArgs := make([]interface{}, len(values))
	for i := range values {
		scanArgs[i] = &values[i]
	}

	err = rows.Scan(scanArgs...)
	if err != nil {
		return nil, err
	}

	slaveInfo := make(map[string]interface{})
	for i, col := range columns {
		val := string(values[i])
		vi, err := strconv.ParseInt(val, 10, 64)
		if err != nil {
			slaveInfo[col] = val
		} else {
			slaveInfo[col] = vi
		}
	}

	secondsBehindSource, ok := slaveInfo["Seconds_Behind_Source"].(int64)
	if config.FailReplicaNotRunning && !ok {
		return nil, errors.New("slave is not running")
	}

	if ok && config.MaxSecondsBehindSource > 0 {
		if secondsBehindSource > int64(config.MaxSecondsBehindSource) {
			return nil, errors.New("replication lag is too high")
		}
	}

	return slaveInfo, nil
}

func serverError(w http.ResponseWriter, err error) {
	log.Printf("error: %s", err)
	code := http.StatusInternalServerError
	http.Error(w, fmt.Sprintf("%s", err), code)
}

package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	_ "github.com/go-sql-driver/mysql"
	"github.com/rnakamine/mysql-replica-healthcheck-agent/config"
	"golang.org/x/sync/errgroup"
)

var Version string

func main() {
	var (
		showVersion bool
		configPath  string
	)
	flag.BoolVar(&showVersion, "version", false, "show version")
	flag.StringVar(&configPath, "config", "/etc/mysql-replica-healthcheck-agent/replicas.yml", "config file path")
	flag.Parse()

	if showVersion {
		fmt.Printf("version %s\n", Version)
		return
	}

	config, err := config.New(configPath)
	if err != nil {
		log.Fatalf("failed to read config: %v", err)
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	g, ctx := errgroup.WithContext(ctx)

	servers := make([]*http.Server, 0, len(*config))
	for name, replicaConfig := range *config {
		srv := createHealthChecker(name, replicaConfig)
		servers = append(servers, srv)
		g.Go(func() error {
			if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
				return fmt.Errorf("server %s failed: %w", name, err)
			}
			return nil
		})
	}

	<-ctx.Done()
	for _, srv := range servers {
		if err := srv.Shutdown(context.Background()); err != nil {
			log.Fatalf("failed to shutdown server: %v", err)
		}
	}

	if err := g.Wait(); err != nil {
		log.Fatalf("failed to start server: %v", err)
	}
}

func createHealthChecker(name string, config config.ReplicaConfig) *http.Server {
	port := config.HealthcheckConfig.Port
	if port == 0 {
		log.Fatalf("port not specified for %s", name)
	}

	path := config.HealthcheckConfig.Path
	if path == "" {
		path = "/"
	}

	log.Printf("creating healthchecker for %s on port %d", name, port)

	mux := http.NewServeMux()
	mux.HandleFunc(path, handlerFunc(name, &config))
	addr := fmt.Sprintf(":%d", port)
	return &http.Server{
		Addr:        addr,
		Handler:     mux,
		ReadTimeout: 10 * time.Second,
	}
}

func handlerFunc(name string, config *config.ReplicaConfig) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		dsn := fmt.Sprintf("%s:%s@tcp(%s:%d)/", config.User, config.Password, config.Host, config.Port)
		db, err := sql.Open("mysql", dsn)
		if err != nil {
			serverError(w, r, err)
			return
		}
		defer db.Close()
		replicaInfo, err := fetchReplicaStatus(config, db)
		if err != nil {
			serverError(w, r, err)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		encoder := json.NewEncoder(w)
		if err := encoder.Encode(replicaInfo); err != nil {
			serverError(w, r, err)
			return
		}

		log.Printf("[%s] %s %s 200", name, r.Method, r.URL.Path)
	}
}

func fetchReplicaStatus(config *config.ReplicaConfig, db *sql.DB) (map[string]interface{}, error) {
	rows, err := db.Query("SHOW REPLICA STATUS")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	if !rows.Next() {
		return nil, errors.New("no replica status")
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

	replicaInfo := make(map[string]interface{})
	for i, name := range columns {
		vs := string(values[i])
		vi, err := strconv.ParseInt(vs, 10, 64)
		if err != nil {
			replicaInfo[name] = vs
		} else {
			replicaInfo[name] = vi
		}
	}

	secondsBehindSource, ok := replicaInfo["Seconds_Behind_Source"].(int64)
	if config.FailReplicaNotRunning && !ok {
		return nil, errors.New("replica is not running")
	}

	if ok && config.MaxSecondsBehindSource > 0 {
		if secondsBehindSource > int64(config.MaxSecondsBehindSource) {
			return nil, errors.New("replication lag is too high")
		}
	}

	return replicaInfo, nil
}

func serverError(w http.ResponseWriter, r *http.Request, err error) {
	log.Printf("%s %s 500 - Error: %s", r.Method, r.URL.Path, err)
	code := http.StatusInternalServerError
	http.Error(w, fmt.Sprintf("%s", err), code)
}

package main

import (
	"flag"
	"fmt"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/spf13/cobra"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/klog"
	"net/http"
	"os"
	"time"

	"database/sql"
	_ "github.com/go-sql-driver/mysql"
)

type options struct {
	ListenAddr string
	DryRun     bool
}

func (o *options) Run() error {
	stopCh := wait.NeverStop

	klog.Infof("Starting...")

	if len(o.ListenAddr) > 0 {
		http.DefaultServeMux.Handle("/metrics", promhttp.Handler())
		go func() {
			klog.Infof("Listening on %s for UI and metrics", o.ListenAddr)
			if err := http.ListenAndServe(o.ListenAddr, nil); err != nil {
				klog.Exitf("Server exited: %v", err)
			}
		}()
	}

	var ok bool
	var databaseHost, databasePort, databaseUserName, databaseUserPassword, databaseRootPassword, databaseName string

	if databaseHost, ok = os.LookupEnv("MYSQL_HOST"); !ok || len(databaseHost) == 0 {
		klog.Fatal("MYSQL_HOST is not defined")
	}

	if databasePort, ok = os.LookupEnv("MYSQL_PORT"); !ok || len(databasePort) == 0 {
		klog.Fatal("MYSQL_PORT is not defined")
	}

	if databaseUserName, ok = os.LookupEnv("MYSQL_USER"); !ok || len(databaseUserName) == 0 {
		klog.Fatal("MYSQL_USER is not defined")
	}

	if databaseUserPassword, ok = os.LookupEnv("MYSQL_PASSWORD"); !ok || len(databaseUserPassword) == 0 {
		klog.Fatal("MYSQL_PASSWORD is not defined")
	}

	if databaseRootPassword, ok = os.LookupEnv("MYSQL_ROOT_PASSWORD"); !ok || len(databaseRootPassword) == 0 {
		klog.Fatal("MYSQL_ROOT_PASSWORD is not defined")
	}

	if databaseName, ok = os.LookupEnv("MYSQL_DATABASE"); !ok || len(databaseName) == 0 {
		klog.Fatal("MYSQL_DATABASE is not defined")
	}

	dbRoot, err := sql.Open("mysql", fmt.Sprintf("%s:%s@tcp(%s:%s)/%s", "root", databaseRootPassword, databaseHost, databasePort, databaseName))
	if err != nil {
		klog.Fatal(err)
	}
	defer dbRoot.Close()

	dbUser, err := sql.Open("mysql", fmt.Sprintf("%s:%s@tcp(%s:%s)/%s", databaseUserName, databaseUserPassword, databaseHost, databasePort, databaseName))
	if err != nil {
		klog.Fatal(err)
	}
	defer dbUser.Close()

	klog.Infof("Checking access to database...")
	err = wait.PollImmediate(15*time.Second, 60*time.Second, func() (done bool, err error) {
		err = dbUser.Ping()
		if err != nil {
			klog.Warningf("Unable to ping database: %v", err)
			return false, nil
		}
		klog.Infof("Ping successful")
		return true, nil
	})
	if err != nil {
		klog.Fatal(err)
	}

	initializeDatabase(dbRoot, "amd64")

	go mainProcessLoop(stopCh)

	<-stopCh
	klog.Infof("Exit...")
	return nil
}

func initializeDatabase(db *sql.DB, architecture string) {
	klog.Infof("Initializing database...")

	// Connect and check the server version
	var version string
	err := db.QueryRow("SELECT VERSION()").Scan(&version)
	if err != nil {
		klog.Errorf("Unable to get Database version: %v", err)
		return
	}
	fmt.Println("Connected to:", version)

	releasesTableName := fmt.Sprintf("releases_%s", architecture)

	createReleasesTable := `CREATE TABLE IF NOT EXISTS ` + releasesTableName + `(
		id INT NOT NULL AUTO_INCREMENT,
		name VARCHAR(64) NOT NULL,
		PRIMARY KEY ( id )
	);`

	_, err = db.Exec(createReleasesTable)
	if err != nil {
		klog.Errorf("Unable to create releases table: %v", err)
		return
	}

	resultsTableName := fmt.Sprintf("results_%s", architecture)

	createResultsTable := `CREATE TABLE IF NOT EXISTS ` + resultsTableName + `(
		id INT NOT NULL AUTO_INCREMENT,
		release_id INT NOT NULL,
		name VARCHAR(64) NOT NULL,
		state VARCHAR(16) NOT NULL,
		url VARCHAR(256) NOT NULL,
		PRIMARY KEY ( id )
	);`

	_, err = db.Exec(createResultsTable)
	if err != nil {
		klog.Errorf("Unable to create results table: %v", err)
		return
	}
}

func mainProcessLoop(stopCh <-chan struct{}) {
	// Loop, every 5 minutes, forever...
	wait.Until(func() {
		start := time.Now()
		_, err := processLoop()
		duration := time.Since(start)

		if err != nil {
			klog.Errorf("processLoop failed: %v", err)
			return
		}

		klog.Infof("processLoop finished in: %d ms", duration.Milliseconds())
	}, 5*time.Minute, stopCh)
}

func processLoop() (bool, error) {
	return true, nil
}

func main() {
	original := flag.CommandLine
	klog.InitFlags(original)
	original.Set("alsologtostderr", "true")
	original.Set("v", "2")

	opt := &options{
		ListenAddr: ":8080",
	}

	cmd := &cobra.Command{
		Run: func(cmd *cobra.Command, arguments []string) {
			if err := opt.Run(); err != nil {
				klog.Exitf("Run error: %v", err)
			}
		},
	}

	flagset := cmd.Flags()
	flagset.BoolVar(&opt.DryRun, "dry-run", opt.DryRun, "Perform no actions")
	flagset.StringVar(&opt.ListenAddr, "listen", opt.ListenAddr, "The address to serve information on")

	flagset.AddGoFlag(original.Lookup("v"))

	if err := cmd.Execute(); err != nil {
		klog.Exitf("Execute error: %v", err)
	}
}

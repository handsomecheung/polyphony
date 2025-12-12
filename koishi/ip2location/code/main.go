package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"time"

	ip2go "github.com/ip2location/ip2location-go"
)

var (
	db *ip2go.DB
)

func init() {
	dbfile, err := filepath.Abs(filepath.Join(filepath.Dir(os.Args[0]), "IP2LOCATION-LITE-DB1.BIN"))
	if err != nil {
		panic(err)
	}

	printLog("load db file: %s", dbfile)
	db, err = ip2go.OpenDB(dbfile)
	if err != nil {
		panic(err)
	}
}

func main() {
	printLog("start server")

	http.HandleFunc("/ok", ok)
	http.HandleFunc("/info", getInfo)

	err := http.ListenAndServe(":8080", nil)
	if err != nil {
		panic(fmt.Sprintf("ListenAndServe: %s", err))
	}
}

func printLog(msg string, v ...interface{}) {
	fmt.Printf("[%s]: %s\n", time.Now().Format("2006-01-02 15:04:05"), fmt.Sprintf(msg, v...))
}

func ok(w http.ResponseWriter, _r *http.Request) {
	w.Write([]byte("ok"))
}

func getInfo(w http.ResponseWriter, r *http.Request) {
	ips, ok := r.URL.Query()["ip"]

	if !ok || len(ips) < 1 {
		printLog("Url Param 'ip' is missing")
		return
	}
	ip := ips[0]

	results, err := db.Get_all(ip)
	if err != nil {
		fmt.Print(err)
		return
	}

	info := map[string]string{
		"country_code": results.Country_short,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(info)
}

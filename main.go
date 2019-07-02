package main

import (
	"flag"
	"log"
	"net/http"

	"./socket"
)

var addr = flag.String("addr", ":7000", "http service address")

func main() {
	flag.Parse()
	http.Handle("/", http.FileServer(http.Dir("./client")))
	http.HandleFunc("/ws", func(w http.ResponseWriter, r *http.Request) {
		socket.Serve(w, r)
	})

	log.Println("Start Listening address:", *addr)

	err := http.ListenAndServe(*addr, nil)
	if err != nil {
		log.Fatal("ListenAndServe error:", err)
	}
}

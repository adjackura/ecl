package main

import (
	"fmt"
	"io"
	"log"
	"net/http"
)

func main() {
	helloHandler := func(w http.ResponseWriter, req *http.Request) {
		io.WriteString(w, "Hello world!\n")
		fmt.Println("Hello world!")
	}

	fmt.Println("Hello world!")
	http.HandleFunc("/hello", helloHandler)
	log.Fatal(http.ListenAndServe(":8080", nil))
}

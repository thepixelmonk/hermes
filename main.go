package main

import (
    "os"
    "log"
    "fmt"
    "net/http"
)

func main() {
    var port = os.Getenv("HERMES_PORT")
    http.HandleFunc("/github", githubHandler)
    http.HandleFunc("/shortcut", shortcutHandler)
    fmt.Println("Listening on :" + port + " ...\n")
    if err := http.ListenAndServe(":" + port, nil); err != nil { log.Fatal(err) }
}


package main

import (
	"fmt"
	"flag"
	"os"
	"log"
	//"time"
	"os/signal"
)

var serverName = flag.String("name", "", "Server Name")
var listenAddr = flag.String("addr", ":8080", "Listen Addr")
var password = flag.String("auth", "", "Auth Token")
var dirroot = flag.String("dir", ".", "file dir")

var fileMode os.FileMode = 0644

var fs *filesystem

func main() {
	flag.Parse()

	initFilesystem()

	go httpServer()

	waitExitSingnal()
}

func waitExitSingnal() {
	fmt.Println("Runing.")

	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt, os.Kill)

	s := <-c
	fmt.Println("Got signal:", s)
}

func initFilesystem() {
	if *dirroot == "" {
		*dirroot = "."
	}

	d, err := os.Stat(*dirroot)
	if err != nil {
		log.Fatalln("file dir error", err)
	}

	if !d.IsDir() {
		log.Fatalln("path not a dir")
	}

	fs = new(filesystem).Init(*dirroot, fileMode)
}



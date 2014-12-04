package main

import (
	"fmt"
	"flag"
	"io"
	"os"
	"log"
	"path"
	//"path/filepath"
	"strings"
	"os/signal"
	"net/http"
	"crypto/md5"
)

var serverName = flag.String("name", "", "Server Name")
var listenAddr = flag.String("addr", ":8080", "Listen Addr")
var password = flag.String("auth", "", "Auth Token")
var dirroot = flag.String("dir", ".", "file dir")

var fileMode os.FileMode = 0644

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
}

func pathToFile(p string) string {
	return path.Clean(*dirroot + "/" + p)
}

func httpServer() {
	http.HandleFunc("/", methodRouter)
	err := http.ListenAndServe(*listenAddr, nil)
	log.Fatalln(err)
}

func methodRouter(w http.ResponseWriter, r *http.Request) {
	defer func() {
		err := recover()
		if err != nil {
			http.Error(w, "500 Internal Server Error", http.StatusInternalServerError)
			log.Println("Error", err, r.URL.Path, r.RemoteAddr)
		}
	}()

	if *serverName != "" {
		w.Header().Set("ps", *serverName)
	}

	if !strings.HasPrefix(r.URL.Path, "/") {
		r.URL.Path = "/" + r.URL.Path
	}

	switch (r.Method) {
	case "GET" :
		sendFile(w, r)
	case "HEAD" :
		sendFile(w, r)
	case "PUT" :
		authHander(w, r, saveFile)
	case "DELETE" :
		authHander(w, r, deleteFile)
	case "POST" :
		authHander(w, r, postStream)
	default:
		http.Error(w, "405 Method Not Allowed", http.StatusMethodNotAllowed)
		log.Println("Not Allowed Method:", r.Method, r.RemoteAddr)
	}
}

func authHander(w http.ResponseWriter, r *http.Request, handler func(http.ResponseWriter, *http.Request)) {
	if *password != "" {
		token := r.Header.Get("byfs-auth")
		version := r.Header.Get("byfs-version")

		if version != "1" {
			http.Error(w, "Version Not Support", http.StatusPreconditionFailed)
			return
		}

		if len(token) <= 32 {
			http.Error(w, "403 Forbidden", http.StatusForbidden)
			return
		}

		hash := token[:32]
		salt := token[32:]

		h := md5.New()
		io.WriteString(h, *password)
		io.WriteString(h, r.URL.Path)
		io.WriteString(h, salt)

		exp := fmt.Sprintf("%x", h.Sum(nil))

		if exp != hash {
			http.Error(w, "403 Forbidden", http.StatusForbidden)
			log.Println("Auth Error", r.RemoteAddr)
			return
		}
	}

	handler(w, r)
}

func sendFile(w http.ResponseWriter, r *http.Request) {
	file := pathToFile(r.URL.Path)
	f, err := os.Open(file)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	defer f.Close()

	d, err1 := f.Stat()
	if err1 != nil {
		http.NotFound(w, r)
		return
	}

	if d.IsDir() {
		http.NotFound(w, r)
		return
	}

	http.ServeContent(w, r, d.Name(), d.ModTime(), f)
}

func saveFile(w http.ResponseWriter, r *http.Request) {
	name := pathToFile(r.URL.Path)

	dir, _ := path.Split(name)
	if dir != "" {
		err := os.MkdirAll(dir, fileMode)
		if err != nil {
			fmt.Fprintln(w, "Mkdir Error", err)
			log.Println("[Notice]", "Mkdir Error", err, r.URL.Path, r.RemoteAddr)
			return
		}
	}

	f, err := os.OpenFile(name, os.O_WRONLY|os.O_CREATE|os.O_EXCL, fileMode)
	if err != nil {
		fmt.Fprintln(w, "Open File Error", err)
		log.Println("[Notice]", "Open File Error", err, r.URL.Path, r.RemoteAddr)
		return
	}

	i, err := io.Copy(f, r.Body)
	log.Println(i, err)

	err2 := f.Close()
	if err2 != nil {
		panic(err)
	}

	if err == nil {
		fmt.Fprintln(w, "Success")
		return
	}

	err = os.Remove(name)
	if err != nil {
		panic(err)
	}

	fmt.Fprintln(w, "Save Data Error", err)
	log.Println("[Notice]", "Copy Error", err, r.URL.Path, r.RemoteAddr)
}

func deleteFile(w http.ResponseWriter, r *http.Request) {
	file := pathToFile(r.URL.Path)

	err := os.Remove(file)
	if err != nil {
		fmt.Fprintln(w, "Delete", r.URL.Path, "Fail", err)
		log.Println("[Notice]", "Delete Fail", err, r.URL.Path, r.RemoteAddr)
		return
	}

	fmt.Fprintln(w, "Success", err)
}

func postStream(w http.ResponseWriter, r *http.Request) {
	upgrade := r.Header.Get("Upgrade")

	if upgrade != "byfs-stream" {
		http.Error(w, "Upgrade error", http.StatusPreconditionFailed)
		log.Println("Upgrade error", r.RemoteAddr)
		return
	}

	w.Header().Set("Connection", "Upgrade")
	w.Header().Set("Upgrade", "byfs-stream")

}



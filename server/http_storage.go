package main

import (
	"fmt"
	"io"
	"os"
	"log"
	"time"
	"path"
	"strings"
	"net/http"
)

func httpServer() {
	mux := http.NewServeMux()
	mux.HandleFunc("/", methodRouter)

	s := http.Server{
		Addr: *listenAddr,
		Handler:mux,
		ReadTimeout: time.Second * 300,
		WriteTimeout: time.Second * 300,
		MaxHeaderBytes: 1024 * 8,
	}

	err := s.ListenAndServe()
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
		postStream(w, r)
	default:
		http.Error(w, "405 Method Not Allowed", http.StatusMethodNotAllowed)
		log.Println("Not Allowed Method:", r.Method, r.RemoteAddr)
	}
}

func authHander(w http.ResponseWriter, r *http.Request, handler func(http.ResponseWriter, *http.Request)) {
	version := r.Header.Get("Byfs-Version")

	if version != "1" {
		http.Error(w, "Version Not Support", http.StatusPreconditionFailed)
		return
	}

	if *password != "" {
		token := r.Header.Get("Byfs-Auth")

		ok := tokenAuth(r.URL.Path, *password, token);
		if !ok {
			http.Error(w, "403 Forbidden", http.StatusForbidden)
			log.Println("Auth Error", r.RemoteAddr)
			return
		}
	}

	handler(w, r)
}

func sendFile(w http.ResponseWriter, r *http.Request) {
	f, err := fs.Open(r.URL.Path)
	if err != nil {
		http.NotFound(w, r)
		log.Println(err, r.RemoteAddr)
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
	dir, _ := path.Split(fs.pathToFile(r.URL.Path))
	if dir != "" {
		err := os.MkdirAll(dir, fileMode)
		if err != nil {
			fmt.Fprint(w, "Mkdir Error", err)
			log.Println("[Notice]", "Mkdir Error", err, r.URL.Path, r.RemoteAddr)
			return
		}
	}

	f, err := fs.OpenFile(r.URL.Path, os.O_WRONLY|os.O_CREATE|os.O_EXCL)
	if err != nil {
		fmt.Fprint(w, "Open File Error", err)
		log.Println("[Notice]", "Open File Error", err, r.URL.Path, r.RemoteAddr)
		return
	}
	defer f.Close()

	_, err = io.Copy(f, r.Body)
	if err != nil {
		f.ghost = true
		fmt.Fprint(w, "Save Data Error", err)
		log.Println("[Notice]", "Copy Error", err, r.URL.Path, r.RemoteAddr)
		return
	}

	fmt.Fprint(w, "Success")
}

func deleteFile(w http.ResponseWriter, r *http.Request) {
	name := fs.pathToFile(r.URL.Path)
	if name == "." {
		fmt.Fprint(w, "File Name Error")
		return
	}

	err := os.Remove(name)
	if err != nil {
		fmt.Fprint(w, "Delete", r.URL.Path, "Fail", err)
		log.Println("[Notice]", "Delete Fail", err, r.URL.Path, r.RemoteAddr)
		return
	}

	fmt.Fprint(w, "Success")
}

func postStream(w http.ResponseWriter, r *http.Request) {
	f, ok := FconnInit(w, r, *password)
	if !ok {
		return
	}

	defer f.close()

	f.run()
}

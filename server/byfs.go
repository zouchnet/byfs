package main

import (
	"fmt"
	"flag"
	"io"
	"os"
	"log"
	"path"
	"path/filepath"
	//"strings"
	"os/signal"
	"net/http"
	"crypto/md5"
)

var serverName = flag.String("name", "", "Server Name")
var listenAddr = flag.String("addr", ":8080", "Listen Addr")
var password = flag.String("auth", "", "Auth Token")
var dirroot = flag.String("dir", ".", "file dir")

var fileMode os.FileMode = 0666

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

func pathToFile(path string) string {
	return path.Clean(*dirroot + "/" + path)
}

func httpServer() {
	http.HandleFunc("/", methodRouter)
	http.ListenAndServe(*listenAddr, nil)
}

func methodRouter(w http.ResponseWriter, r *http.Request) {
	if *serverName != "" {
		w.Header.Set("ps", *serverName)
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

		if version != '1' {
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
	file := pathToFile(r.URL.Path)

	d, err := os.Stat(file)
	if d.IsDir() {
		http.Error(w, "400 BadRequest", http.StatusBadRequest)
		log.Println("PUT To Dir Error", r.URL.Path,  r.RemoteAddr)
		return
	}

	dir, _ := path.Split(name)
	err := os.MkdirAll(dir, 644)
	if err != nil {
		http.Error(w, "400 BadRequest", http.StatusBadRequest)
		log.Println("Mkdir Error", err, r.URL.Path, r.RemoteAddr)
		return
	}

	m, err := openMagicFile(file)
	if err != nil {
		http.Error(w, "500 Internal Server Error", http.StatusInternalServerError)
		log.Println("saveFile open Error", err, r.URL.Path, r.RemoteAddr)
		return
	}
	defer func() {
		err := m.Close()

		if err != nil {
			http.Error(w, "500 Internal Server Error", http.StatusInternalServerError)
			log.Println("Close Error", err, r.URL.Path, r.RemoteAddr)
		}
	}()

	_, err = io.Copy(f, w)
	if err != nil {
		http.Error(w, "400 BadRequest", http.StatusBadRequest)
		log.Println("Copy Error", err, r.URL.Path, r.RemoteAddr)
		return
	}
}

func deleteFile(w http.ResponseWriter, r *http.Request) {
}

func postStream(w http.ResponseWriter, r *http.Request) {
	upgrade := r.Header.Get("Upgrade")

	if upgrade != "byfs-stream" {
		http.Error(w, "Upgrade error", http.StatusPreconditionFailed)
		log.Println("Upgrade error", r.RemoteAddr)
		return
	}

	w.Header.Set("Connection", "Upgrade")
	w.Header.Set("Upgrade", "byfs-stream")

}

struct magicFile {
	f *os.File
	isGhost bool
	tmp string
	file string
}

func openMagicFile(name string) (*magicFile, error) {
	m := &magicFile{}

	m.f, err = os.openFile(name, os.O_WRONLY|os.O_CREATE|os.O_EXCL, fileMode)
	if err != nil {
		//如果文件己存在则尝试用临时文件的方式打开
		if os.IsExist(err) {
			tmp_file = name + ".tmp"
			m.f, err = os.openFile(tmp_file, os.O_WRONLY|os.O_CREATE|os.O_EXCL, fileMode)
			if err != nil {
				return nil, errors.New("Open Tmpfile Error " + err.Error())
			}
			m.isGhost = true
			m.file = name
			m.tmp = tmp_file
		} else {
			return nil, errors.New("Open File Error " + err.Error())
		}
	}

	return m, nil
}

func (m *magicFile) Close() error {
	err := m.f.Close()
	if err != nil {
		return err
	}

	err = os.Remove(m.file)
	if err != nil {
		return err
	}

	err = os.Rename(m.tmp, m.file)
	if err != nil {
		return err
	}

	return nil
}

package main

import (
	"net"
	"net/http"
	"time"
	"log"
	"bufio"
	"errors"
	"os"
	"encoding/binary"
)

var actionTimeout = 2 * time.Second
var idleTimeout = 300 * time.Second
var maxString uint32 = 1024 * 4

type fconn struct{
	conn net.Conn
	bufrw *bufio.ReadWriter
	files map[uint32]*os.File
	pos uint32
}

func (f *fconn) init(w http.ResponseWriter, r *http.Request) bool {
	if r.Header.Get("Connection") != "Upgrade" {
		http.Error(w, "Connection Need Upgrade", http.StatusPreconditionFailed)
		log.Println("Connection Need Upgrade", r.RemoteAddr)
		return false
	}

	if r.Header.Get("Upgrade") != "Byfs-Stream" {
		http.Error(w, "Upgrade Error", http.StatusPreconditionFailed)
		log.Println("Upgrade Error", r.RemoteAddr)
		return false
	}

	hj, ok := w.(http.Hijacker)
    if !ok {
		panic("webserver doesn't support hijacking")
    }

	var err error
	f.conn, f.bufrw, err = hj.Hijack()
    if err != nil {
		panic("hijacking error")
    }

	f.bufrw.WriteString("HTTP/1.1 101 Switching Protocols\r\n")
	f.bufrw.WriteString("Upgrade: Byfs-Stream\r\n")
	f.bufrw.WriteString("Connection: Upgrade\r\n")
	f.bufrw.WriteString("\r\n")
	err = f.bufrw.Flush()
	if err != nil {
		return false
	}

	return true
}

func (f *fconn) close() {
	for _, fp := range f.files {
		fp.Close()
	}
}

func (f *fconn) auth() bool {
	token := randString()
	f.bufrw.WriteString(token)
	f.bufrw.Flush()

	var code uint16

	//这里要求马上认证
	f.conn.SetReadDeadline(time.Now().Add(actionTimeout))
	err := binary.Read(f.bufrw, binary.BigEndian, &code)
	if err != nil {
		log.Println("Read Error", err, f.conn.RemoteAddr())
		return false
	}

	//协议错误
	if code != CODE_AUTH {
		log.Println("Auth Error", f.conn.RemoteAddr())
		return false
	}

	//字符串
	data, err := f.readString()
	if err != nil {
		log.Println("Read Error", err, f.conn.RemoteAddr())
		return false
	}

	ok := tokenAuth(token, data)
	if !ok {
		log.Println("Auth Error", f.conn.RemoteAddr())
		return false
	}

	return true
}

func (f *fconn) run() {
	for {
		var code uint16
		//这里是闲置超时
		f.conn.SetReadDeadline(time.Now().Add(idleTimeout))
		err := binary.Read(f.bufrw, binary.BigEndian, &code)
		if err != nil {
			log.Println("Read Error", err, f.conn.RemoteAddr())
			return
		}

		var ok bool
		switch (code) {
			case CODE_FILE_OPEN :
				ok = f.a_fopen()
			case CODE_FILE_READ :
				ok = f.a_fread()
			case CODE_FILE_WRITE :
				ok = f.a_fwrite()
			case CODE_FILE_LOCK :
				ok = f.a_flock()
			case CODE_FILE_UNLOCK :
				ok = f.a_funlock()
			case CODE_FILE_SEEK :
				ok = f.a_fseek()
			case CODE_FILE_STAT :
				ok = f.a_fstat()
			case CODE_FILE_EOF :
				ok = f.a_feof()
			case CODE_FILE_FLUSH :
				ok = f.a_flush()
			case CODE_FILE_TRUCATE :
				ok = f.a_ftrucate()
			case CODE_FILE_CLOSE :
				ok = f.a_fclose()

			case CODE_DIR_OPEN :
				ok = f.a_opendir()
			case CODE_DIR_READ :
				ok = f.a_readdir()
			case CODE_DIR_CLOSE :
				ok = f.a_closedir()

			case CODE_MKDIR :
				ok = f.a_mkdir()
			case CODE_RMDIR :
				ok = f.a_rmdir()
			case CODE_COPY :
				ok = f.a_copy()
			case CODE_MOVE :
				ok = f.a_move()
			case CODE_STAT :
				ok = f.a_stat()
			case CODE_LSTAT :
				ok = f.a_lstat()
			default:
				ok = false
		}

		if !ok {
			return
		}
	}
}

func (f *fconn) readString() (string, error) {
	var strlen uint32

	f.conn.SetReadDeadline(time.Now().Add(actionTimeout))
	err := binary.Read(f.bufrw, binary.BigEndian, &strlen)
	if err != nil {
		return "", err
	}

	if strlen > maxString {
		return "", errors.New("String Too Big")
	}

	data := make([]byte, strlen)
	_, err = f.bufrw.Read(data)
	if err != nil {
		return "", err
	}

	return string(data), nil
}

func (f fconn) writeString(str string) error {
	f.conn.SetWriteDeadline(time.Now().Add(actionTimeout))

	err := binary.Write(f.bufrw, binary.BigEndian, uint32(len(str)))
	if err != nil {
		return err
	}

	_, err = f.bufrw.WriteString(str)

	return err
}
func (f *fconn) a_fopen() bool {
	return true
}

func (f *fconn) a_fread() bool {
	return true
}

func (f *fconn) a_fwrite() bool {
	return true
}

func (f *fconn) a_flock() bool {
	return true
}

func (f *fconn) a_funlock() bool {
	return true
}

func (f *fconn) a_fseek() bool {
	return true
}

func (f *fconn) a_fstat() bool {
	return true
}

func (f *fconn) a_feof() bool {
	return true
}

func (f *fconn) a_flush() bool {
	return true
}

func (f *fconn) a_ftrucate() bool {
	return true
}

func (f *fconn) a_fclose() bool {
	return true
}

func (f *fconn) a_opendir() bool {
	return true
}

func (f *fconn) a_readdir() bool {
	return true
}

func (f *fconn) a_closedir() bool {
	return true
}

func (f *fconn) a_mkdir() bool {
	return true
}

func (f *fconn) a_rmdir() bool {
	return true
}

func (f *fconn) a_copy() bool {
	return true
}

func (f *fconn) a_move() bool {
	return true
}

func (f *fconn) a_stat() bool {
	return true
}

func (f *fconn) a_lstat() bool {
	return true
}


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

const (
	CODE_AUTH = 8888

	CODE_FILE_OPEN = 1
	CODE_FILE_READ = 2
	CODE_FILE_WRITE = 3
	CODE_FILE_LOCK = 4
	CODE_FILE_UNLOCK = 5
	CODE_FILE_SEEK = 6
	CODE_FILE_STAT = 7
	CODE_FILE_FLUSH = 8
	CODE_FILE_TRUCATE = 9
	CODE_FILE_CLOSE = 10

	CODE_DIR_OPEN = 1001
	CODE_DIR_READ = 1002
	CODE_DIR_CLOSE = 1003

	CODE_MKDIR = 2001
	CODE_RMDIR = 2002
	CODE_RENAME = 2003
	CODE_STAT = 2004
	CODE_LSTAT = 2005
)

var (
	//成功
	status_ok uint8 = 0
	//失败
	status_fail uint8 = 1
)

var actionTimeout = 2 * time.Second
var idleTimeout = 300 * time.Second
var maxString uint32 = 1024 * 10

type fconn struct{
	conn net.Conn
	bufrw *bufio.ReadWriter
	files map[uint32]*os.File
	pos uint32
	ok bool
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
	for f.ok {
		var code uint16
		//这里是闲置超时
		f.conn.SetReadDeadline(time.Now().Add(idleTimeout))
		err := binary.Read(f.bufrw, binary.BigEndian, &code)
		if err != nil {
			log.Println("Read Error", err, f.conn.RemoteAddr())
			return
		}

		switch (code) {
			case CODE_FILE_OPEN :
				f.a_fopen()
			case CODE_FILE_READ :
				f.a_fread()
			case CODE_FILE_WRITE :
				f.a_fwrite()
			case CODE_FILE_LOCK :
				f.a_flock()
			case CODE_FILE_UNLOCK :
				f.a_funlock()
			case CODE_FILE_SEEK :
				f.a_fseek()
			case CODE_FILE_STAT :
				f.a_fstat()
			case CODE_FILE_EOF :
				f.a_feof()
			case CODE_FILE_FLUSH :
				f.a_flush()
			case CODE_FILE_TRUCATE :
				f.a_ftrucate()
			case CODE_FILE_CLOSE :
				f.a_fclose()

			case CODE_DIR_OPEN :
				f.a_opendir()
			case CODE_DIR_READ :
				f.a_readdir()
			case CODE_DIR_CLOSE :
				f.a_closedir()

			case CODE_MKDIR :
				f.a_mkdir()
			case CODE_RMDIR :
				f.a_rmdir()
			case CODE_RENAME :
				f.a_rename()
			case CODE_STAT :
				f.a_stat()
			case CODE_LSTAT :
				f.a_lstat()
			default:
				false
		}
	}
}

func (f *fconn) a_fopen() {
	f.conn.SetReadDeadline(time.Now().Add(actionTimeout))

	name, err := f.readString()
	if err != nil {
		log.Println(err)
		f.ok = false
		return
	}

	var flag int32
	err := binary.Read(f.bufrw, binary.BigEndian, flag)
	if err != nil {
		log.Println(err)
		f.ok = false
		return
	}

	fp, err := fs.openFile(name, flag)
	if err != nil {
		f.writeError(err)
		return
	}

	f.pos++
	f.files[f.pos] = fp

	f.conn.SetWriteDeadline(time.Now().Add(actionTimeout))
	binary.Write(f.bufrw, binary.BigEndian, status_ok)
	binary.Write(f.bufrw, binary.BigEndian, f.pos)
	err = f.bufrw.Flush()

	if err != nil {
		log.Println(err)
		f.ok = false
		return
	}
}

func (f *fconn) a_fread() {
	f.conn.SetReadDeadline(time.Now().Add(actionTimeout))

	var pos uint32
	err := binary.Read(f.bufrw, binary.BigEndian, pos)
	if err != nil {
		log.Println(err)
		f.ok = false
		return
	}

	var count uint32
	err := binary.Read(f.bufrw, binary.BigEndian, count)
	if err != nil {
		log.Println(err)
		f.ok = false
		return
	}

	fp := f.files[pos]
	if fp == nil {
		f.writeError("文件描术符错误")
		return
	}

	var eof uint8

	data := make([]byte, count)
	_, err = fp.Read(data)
	if err != nil {
		if err == io.EOF {
			eof = 1
		} else {
			f.writeError(err)
			return
		}
	}

	f.conn.SetWriteDeadline(time.Now().Add(actionTimeout))
	binary.Write(f.bufrw, binary.BigEndian, status_ok)
	binary.Write(f.bufrw, binary.BigEndian, eof)
	f.bufrw.Write(data)
	err = f.bufrw.Flush()

	if err != nil {
		log.Println(err)
		f.ok = false
		return
	}
}

func (f *fconn) a_fwrite() {
	f.conn.SetReadDeadline(time.Now().Add(actionTimeout))

	var pos uint32
	err := binary.Read(f.bufrw, binary.BigEndian, pos)
	if err != nil {
		log.Println(err)
		f.ok = false
		return
	}

	data, err := f.readData()
	if err != nil {
		log.Println(err)
		f.ok = false
		return
	}

	fp := f.files[pos]
	if fp == nil {
		f.writeError("文件描术符错误")
		return
	}

	_, err = fp.Write(data)

	if err != nil {
		f.writeError(err)
		return
	}

	f.conn.SetWriteDeadline(time.Now().Add(actionTimeout))
	binary.Write(f.bufrw, binary.BigEndian, status_ok)
	err = f.bufrw.Flush()

	if err != nil {
		log.Println(err)
		f.ok = false
		return
	}
}

func (f *fconn) a_flock() {
}

func (f *fconn) a_funlock() {
}

func (f *fconn) a_flush() {
	f.conn.SetReadDeadline(time.Now().Add(actionTimeout))

	var pos uint32
	err := binary.Read(f.bufrw, binary.BigEndian, pos)
	if err != nil {
		log.Println(err)
		f.ok = false
		return
	}

	fp := f.files[pos]
	if fp == nil {
		f.writeError("文件描术符错误")
		return
	}

	err = fp.Sync()
	if err != nil {
		f.writeError(err)
		return
	}

	f.conn.SetWriteDeadline(time.Now().Add(actionTimeout))
	binary.Write(f.bufrw, binary.BigEndian, status_ok)
	err = f.bufrw.Flush()

	if err != nil {
		log.Println(err)
		f.ok = false
		return
	}
}

func (f *fconn) a_fseek() {
	f.conn.SetReadDeadline(time.Now().Add(actionTimeout))

	var pos uint32
	err := binary.Read(f.bufrw, binary.BigEndian, pos)
	if err != nil {
		log.Println(err)
		f.ok = false
		return
	}

	var offset int64
	err := binary.Read(f.bufrw, binary.BigEndian, offset)
	if err != nil {
		log.Println(err)
		f.ok = false
		return
	}

	var mode uint8
	err := binary.Read(f.bufrw, binary.BigEndian, mode)
	if err != nil {
		log.Println(err)
		f.ok = false
		return
	}

	fp := f.files[pos]
	if fp == nil {
		f.writeError("文件描术符错误")
		return
	}

	var off int64

	switch int(mode) {
	case os.SEEK_SET:
		off, err = fp.Seek(offset, os.SEEK_SET)
	case os.SEEK_CUR:
		off, err = fp.Seek(offset, os.SEEK_CUR)
	case os.SEEK_END:
		off, err = fp.Seek(offset, os.SEEK_END)
	default:
		f.writeError("seek模式错误")
		return
	}

	if err != nil {
		f.writeError(err)
		return
	}

	f.conn.SetWriteDeadline(time.Now().Add(actionTimeout))
	binary.Write(f.bufrw, binary.BigEndian, status_ok)
	binary.Write(f.bufrw, binary.BigEndian, off)
	err = f.bufrw.Flush()

	if err != nil {
		log.Println(err)
		f.ok = false
		return
	}
}

func (f *fconn) a_fstat() {
	f.conn.SetReadDeadline(time.Now().Add(actionTimeout))

	var pos uint32
	err := binary.Read(f.bufrw, binary.BigEndian, pos)
	if err != nil {
		log.Println(err)
		f.ok = false
		return
	}

	fp := f.files[pos]
	if fp == nil {
		f.writeError("文件描术符错误")
		return
	}

	fi, err := f.Stat()
	if fp == nil {
		f.writeError(err)
		return
	}

	f.conn.SetWriteDeadline(time.Now().Add(actionTimeout))
	binary.Write(f.bufrw, binary.BigEndian, status_ok)
	binary.Write(f.bufrw, binary.BigEndian, fi.Mode())
	binary.Write(f.bufrw, binary.BigEndian, fi.Size())
	binary.Write(f.bufrw, binary.BigEndian, fi.ModTime().Unix())
	err = f.bufrw.Flush()

	if err != nil {
		log.Println(err)
		f.ok = false
		return
	}
}

func (f *fconn) a_ftrucate() {
	f.conn.SetReadDeadline(time.Now().Add(actionTimeout))

	var pos uint32
	err := binary.Read(f.bufrw, binary.BigEndian, pos)
	if err != nil {
		log.Println(err)
		f.ok = false
		return
	}

	var size int64
	err := binary.Read(f.bufrw, binary.BigEndian, size)
	if err != nil {
		log.Println(err)
		f.ok = false
		return
	}

	fp := f.files[pos]
	if fp == nil {
		f.writeError("文件描术符错误")
		return
	}

	err = fp.Truncate(size)
	if err != nil {
		f.writeError(err)
		return
	}

	f.conn.SetWriteDeadline(time.Now().Add(actionTimeout))
	binary.Write(f.bufrw, binary.BigEndian, status_ok)
	err = f.bufrw.Flush()

	if err != nil {
		log.Println(err)
		f.ok = false
		return
	}
}


func (f *fconn) a_fclose() {
	f.conn.SetReadDeadline(time.Now().Add(actionTimeout))

	var pos uint32
	err := binary.Read(f.bufrw, binary.BigEndian, pos)
	if err != nil {
		log.Println(err)
		f.ok = false
		return
	}

	fp := f.files[pos]
	if fp == nil {
		f.writeError("文件描术符错误")
		return
	}

	fi, err := fp.Close()
	if err == nil {
		f.writeError(err)
		return
	}

	f.conn.SetWriteDeadline(time.Now().Add(actionTimeout))
	binary.Write(f.bufrw, binary.BigEndian, status_ok)
	err = f.bufrw.Flush()

	if err != nil {
		log.Println(err)
		f.ok = false
		return
	}
}

func (f *fconn) a_opendir() {
	f.conn.SetReadDeadline(time.Now().Add(actionTimeout))

	name, err := f.readString()
	if err != nil {
		log.Println(err)
		f.ok = false
		return
	}

	fp, err := fs.open(name)
	if err != nil {
		f.writeError(err)
		return
	}

	f.pos++
	f.files[f.pos] = fp

	f.conn.SetWriteDeadline(time.Now().Add(actionTimeout))
	binary.Write(f.bufrw, binary.BigEndian, status_ok)
	binary.Write(f.bufrw, binary.BigEndian, f.pos)
	err = f.bufrw.Flush()

	if err != nil {
		log.Println(err)
		f.ok = false
		return
	}
}

func (f *fconn) a_readdir() {
	f.conn.SetReadDeadline(time.Now().Add(actionTimeout))

	var pos uint32
	err := binary.Read(f.bufrw, binary.BigEndian, pos)
	if err != nil {
		log.Println(err)
		f.ok = false
		return
	}

	var count uint16
	err := binary.Read(f.bufrw, binary.BigEndian, count)
	if err != nil {
		log.Println(err)
		f.ok = false
		return
	}

	fp := f.files[pos]
	if fp == nil {
		f.writeError("文件描术符错误")
		return
	}

	fi, err = fp.Readdir(int(count))
	if err != nil {
		f.writeError(err)
		return
	}

	var num uint16 = len(fi)

	f.conn.SetWriteDeadline(time.Now().Add(actionTimeout))
	binary.Write(f.bufrw, binary.BigEndian, status_ok)
	binary.Write(f.bufrw, binary.BigEndian, num)
	for _, val := range fi {
		f.writeString(val.Name())
	}
	err = f.bufrw.Flush()

	if err != nil {
		log.Println(err)
		f.ok = false
		return
	}
}

func (f *fconn) a_closedir() {
	f.conn.SetReadDeadline(time.Now().Add(actionTimeout))

	var pos uint32
	err := binary.Read(f.bufrw, binary.BigEndian, pos)
	if err != nil {
		log.Println(err)
		f.ok = false
		return
	}

	fp := f.files[pos]
	if fp == nil {
		f.writeError("文件描术符错误")
		return
	}

	fi, err := fp.Close()
	if err == nil {
		f.writeError(err)
		return
	}

	f.conn.SetWriteDeadline(time.Now().Add(actionTimeout))
	binary.Write(f.bufrw, binary.BigEndian, status_ok)
	err = f.bufrw.Flush()

	if err != nil {
		log.Println(err)
		f.ok = false
		return
	}
}

func (f *fconn) a_mkdir() {
	f.conn.SetReadDeadline(time.Now().Add(actionTimeout))

	name, err := f.readString()
	if err != nil {
		log.Println(err)
		f.ok = false
		return
	}

	var mode uint32
	err := binary.Read(f.bufrw, binary.BigEndian, pos)
	if err != nil {
		log.Println(err)
		f.ok = false
		return
	}

	var rec uint8
	err := binary.Read(f.bufrw, binary.BigEndian, rec)
	if err != nil {
		log.Println(err)
		f.ok = false
		return
	}

	if rec == 0 {
		err = os.Mkdir(name, mode)
	} else {
		err = os.MkdirALL(name, mode)
	}

	if err != nil {
		f.writeError(err)
		return
	}

	f.conn.SetWriteDeadline(time.Now().Add(actionTimeout))
	binary.Write(f.bufrw, binary.BigEndian, status_ok)
	err = f.bufrw.Flush()

	if err != nil {
		log.Println(err)
		f.ok = false
		return
	}
}

func (f *fconn) a_rmdir() {
	f.conn.SetReadDeadline(time.Now().Add(actionTimeout))

	name, err := f.readString()
	if err != nil {
		log.Println(err)
		f.ok = false
		return
	}

	var rec uint8
	err := binary.Read(f.bufrw, binary.BigEndian, rec)
	if err != nil {
		log.Println(err)
		f.ok = false
		return
	}

	if rec == 0 {
		err = os.Remove(name)
	} else {
		err = os.RemoveAll(name)
	}

	if err != nil {
		f.writeError(err)
		return
	}

	f.conn.SetWriteDeadline(time.Now().Add(actionTimeout))
	binary.Write(f.bufrw, binary.BigEndian, status_ok)
	err = f.bufrw.Flush()

	if err != nil {
		log.Println(err)
		f.ok = false
		return
	}
}

func (f *fconn) a_rename() {
	f.conn.SetReadDeadline(time.Now().Add(actionTimeout))

	name, err := f.readString()
	if err != nil {
		log.Println(err)
		f.ok = false
		return
	}

	to, err := f.readString()
	if err != nil {
		log.Println(err)
		f.ok = false
		return
	}

	err = os.Rename(name, to)

	if err != nil {
		f.writeError(err)
		return
	}

	f.conn.SetWriteDeadline(time.Now().Add(actionTimeout))
	binary.Write(f.bufrw, binary.BigEndian, status_ok)
	err = f.bufrw.Flush()

	if err != nil {
		log.Println(err)
		f.ok = false
		return
	}
}

func (f *fconn) a_stat() {
	f.conn.SetReadDeadline(time.Now().Add(actionTimeout))

	name, err := f.readString()
	if err != nil {
		log.Println(err)
		f.ok = false
		return
	}

	fi, err := fs.Stat(name)
	if err != nil {
		f.writeError(err)
		return
	}

	f.conn.SetWriteDeadline(time.Now().Add(actionTimeout))
	binary.Write(f.bufrw, binary.BigEndian, status_ok)
	binary.Write(f.bufrw, binary.BigEndian, fi.Mode())
	binary.Write(f.bufrw, binary.BigEndian, fi.Size())
	binary.Write(f.bufrw, binary.BigEndian, fi.ModTime().Unix())
	err = f.bufrw.Flush()

	if err != nil {
		log.Println(err)
		f.ok = false
		return
	}
}

func (f *fconn) a_lstat() {
	f.conn.SetReadDeadline(time.Now().Add(actionTimeout))

	name, err := f.readString()
	if err != nil {
		log.Println(err)
		f.ok = false
		return
	}

	fi, err := fs.Lstat(name)
	if err != nil {
		f.writeError(err)
		return
	}

	f.conn.SetWriteDeadline(time.Now().Add(actionTimeout))
	binary.Write(f.bufrw, binary.BigEndian, status_ok)
	binary.Write(f.bufrw, binary.BigEndian, fi.Mode())
	binary.Write(f.bufrw, binary.BigEndian, fi.Size())
	binary.Write(f.bufrw, binary.BigEndian, fi.ModTime().Unix())
	err = f.bufrw.Flush()

	if err != nil {
		log.Println(err)
		f.ok = false
		return
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

func (f *fconn) readData() ([]byte, error) {
	var strlen uint32

	f.conn.SetReadDeadline(time.Now().Add(actionTimeout * 10))
	err := binary.Read(f.bufrw, binary.BigEndian, &strlen)
	if err != nil {
		return "", err
	}

	if strlen > maxString {
		return "", errors.New("Data Too Big")
	}

	data := make([]byte, strlen)
	_, err = f.bufrw.Read(data)
	if err != nil {
		return "", err
	}

	return data, nil
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

func (f fconn) writeError(str ...interface{}) {
	f.conn.SetWriteDeadline(time.Now().Add(actionTimeout))

	err := binary.Write(f.bufrw, binary.BigEndian, status_fail)
	if err != nil {
		log.Println(err)
		f.ok = false
		return
	}

	_, err = f.bufrw.WriteString(fmt.Sprint(str...))
	if err != nil {
		log.Println(err)
		f.ok = false
		return
	}

	err = f.bufrw.Flush()
	if err != nil {
		log.Println(err)
		f.ok = false
		return
	}
}

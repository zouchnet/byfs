package main

import (
	"net"
	"net/http"
	"time"
	"log"
	"bufio"
	"os"
	"fmt"
	"io"
	"encoding/binary"
)

const (
	CODE_AUTH = 0xee01
	CODE_CLOSE = 0xee02

	CODE_FILE_OPEN = 0xff01
	CODE_FILE_READ = 0xff02
	CODE_FILE_WRITE = 0xff03
	CODE_FILE_SEEK = 0xff04
	CODE_FILE_STAT = 0xff05
	CODE_FILE_FLUSH = 0xff06
	CODE_FILE_TRUNCATE = 0xff07
	CODE_FILE_CLOSE = 0xff08

	CODE_DIR_OPEN = 0xfe01
	CODE_DIR_READ = 0xfe02
	CODE_DIR_CLOSE = 0xfe03

	CODE_MKDIR = 0xfc01
	CODE_RMDIR = 0xfc02
	CODE_RENAME = 0xfc03
	CODE_STAT = 0xfc04
	CODE_LSTAT = 0xfc05
)

var (
	//成功
	status_ok uint8 = 0xff
	//失败
	status_fail uint8 = 0xfe
)

var actionTimeout = 3 * time.Second
var idleTimeout = 300 * time.Second

type FatalError string
func (e FatalError) Error() string {
	return string(e)
}

type WarningError string
func (e WarningError) Error() string {
	return string(e)
}

type NoticeError string
func (e NoticeError) Error() string {
	return string(e)
}

type fconn struct{
	conn net.Conn
	bufrw *bufio.ReadWriter
	files map[uint32]*file
	pos uint32
	pass string
	token string
}

func FconnInit(w http.ResponseWriter, r *http.Request, password string) (*fconn, bool) {
	if r.Header.Get("Connection") != "Upgrade" {
		http.Error(w, "Connection Need Upgrade", http.StatusPreconditionFailed)
		log.Println("Connection Need Upgrade", r.RemoteAddr)
		return nil, false
	}

	if r.Header.Get("Upgrade") != "Byfs-Stream" {
		http.Error(w, "Upgrade Error", http.StatusPreconditionFailed)
		log.Println("Upgrade Error", r.RemoteAddr)
		return nil, false
	}

	hj, ok := w.(http.Hijacker)
    if !ok {
		panic("webserver doesn't support hijacking")
    }

	conn, bufrw, err := hj.Hijack()
    if err != nil {
		panic("hijacking error")
    }

	bufrw.WriteString("HTTP/1.1 101 Switching Protocols\r\n")
	bufrw.WriteString("Upgrade: Byfs-Stream\r\n")
	bufrw.WriteString("Connection: Upgrade\r\n")

	var token string
	if password != "" {
		token = randString()
		bufrw.WriteString("Byfs-Auth: "+token+"\r\n")
	}

	bufrw.WriteString("\r\n")
	err = bufrw.Flush()
	if err != nil {
		conn.Close()
		return nil, false
	}

	f := &fconn{}
	f.conn = conn
	f.bufrw = bufrw
	f.token = token
	f.pass = password
	f.files = make(map[uint32]*file)

	return f, true
}

func (f *fconn) close() {
	for _, fp := range f.files {
		fp.Close()
	}

	f.bufrw.Flush()
	f.conn.Close()
}

func (f *fconn) auth() {
	f.readTimeLimit()
	code := f.readUint16()

	//协议错误
	if code != CODE_AUTH {
		panic(FatalError("Auth Code Not Give"))
	}

	//字符串
	data := f.readString()

	ok := tokenAuth(f.token, f.pass, data)
	if !ok {
		panic(FatalError("Auth Error"))
	}
}

func (f *fconn) run() {
		log.Println("run")
	defer func() {
		log.Println("defer")
		if x := recover(); x != nil {
			log.Println(x)
			switch v := x.(type) {
			case FatalError :
				log.Println("[Fatal]", v, f.conn.RemoteAddr())
			default:
				panic(x)
			}
		}
	}()

	//这里要求马上认证
	if f.pass != "" {
		log.Println("auth")
		f.auth()
	}

	for {
		f.idleTimeLimit()
		code := f.readUint16()

		if code == CODE_CLOSE {
			log.Println("link CLOSE")
			return
		}

		log.Printf("code 0x%x", code)
		f._run(code)
	}
}

func (f *fconn) _run(code uint16) {
	defer func() {
		if x := recover(); x != nil {
			switch v := x.(type) {
			case NoticeError :
				log.Println("[Notice]", v)
				f.writeError("[Notice]", v)
			case WarningError :
				log.Println("[Warning]", v)
				f.writeError("[Warning]", v)
			default:
				panic(x)
			}
		}
	}()

	switch (code) {
		case CODE_FILE_OPEN :
			f.a_fopen()
		case CODE_FILE_READ :
			f.a_fread()
		case CODE_FILE_WRITE :
			f.a_fwrite()
		case CODE_FILE_SEEK :
			f.a_fseek()
		case CODE_FILE_STAT :
			f.a_fstat()
		case CODE_FILE_FLUSH :
			f.a_flush()
		case CODE_FILE_TRUNCATE :
			f.a_truncate()
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
			panic(FatalError("未定义的指令"))
	}

	f.flush()
}


func (f *fconn) a_fopen() {
	f.readTimeLimit()
	name := f.readString()
	flag := f.readInt32()

	fp, err := fs.OpenFile(name, int(flag))
	if err != nil {
		panic(WarningError(err.Error()))
	}

	f.pos++
	f.files[f.pos] = fp

	f.writeTimeLimit()
	f.writeUint8(status_ok)
	f.writeUint32(f.pos)
}

func (f *fconn) a_fread() {
	f.readTimeLimit()
	pos := f.readUint32()
	count := f.readInt64()

	fp := f.getFile(pos)
	reader := io.LimitReader(fp, count)

	f.writeTimeLimit()
	f.writeUint8(status_ok)
	f.writeChunkedFromReader(reader)
	f.writeUint8(status_ok)
}

func (f *fconn) a_fwrite() {
	f.readTimeLimit()
	pos := f.readUint32()

	fp := f.getFile(pos)

	f.readChunkedToWriter(fp)

	f.writeTimeLimit()
	f.writeUint8(status_ok)
}

func (f *fconn) a_flush() {
	f.readTimeLimit()
	pos := f.readUint32()
	fp := f.getFile(pos)

	err := fp.Sync()
	if err != nil {
		panic(WarningError(err.Error()))
	}

	f.writeTimeLimit()
	f.writeUint8(status_ok)
}

func (f *fconn) a_fseek() {
	f.readTimeLimit()
	pos := f.readUint32()
	offset := f.readInt64()
	mode := f.readUint8()

	fp := f.getFile(pos)

	var off int64
	var err error

	switch int(mode) {
	case os.SEEK_SET:
		off, err = fp.Seek(offset, os.SEEK_SET)
	case os.SEEK_CUR:
		off, err = fp.Seek(offset, os.SEEK_CUR)
	case os.SEEK_END:
		off, err = fp.Seek(offset, os.SEEK_END)
	default:
		panic(FatalError("seek模式错误"))
	}

	if err != nil {
		panic(WarningError(err.Error()))
	}

	_ = off
	f.writeTimeLimit()
	f.writeUint8(status_ok)
	f.writeInt64(off)
}

func (f *fconn) a_fstat() {
	f.readTimeLimit()
	pos := f.readUint32()

	fp := f.getFile(pos)

	fi, err := fp.Stat()
	if err != nil {
		panic(WarningError(err.Error()))
	}

	var is_dir uint8 = 0
	if fi.IsDir() {
		is_dir = 1
	}

	f.writeTimeLimit()
	f.writeUint8(status_ok)
	f.writeUint8(is_dir)
	f.writeInt64(fi.Size())
	f.writeInt64(fi.ModTime().Unix())
}

func (f *fconn) a_truncate() {
	f.readTimeLimit()
	pos := f.readUint32()
	size := f.readInt64()

	fp := f.getFile(pos)

	err := fp.Truncate(size)
	if err != nil {
		panic(WarningError(err.Error()))
	}

	f.writeTimeLimit()
	f.writeUint8(status_ok)
}

func (f *fconn) a_fclose() {
	log.Println("fclose")
	f.readTimeLimit()
	pos := f.readUint32()

	fp := f.getFile(pos)

	err := fp.Close()
	if err != nil {
		panic(WarningError(err.Error()))
	}

	f.writeTimeLimit()
	f.writeUint8(status_ok)
}

func (f *fconn) a_opendir() {
	f.readTimeLimit()
	name := f.readString()

	fp, err := fs.Open(name)
	if err != nil {
		panic(WarningError(err.Error()))
	}

	f.pos++
	f.files[f.pos] = fp

	f.writeTimeLimit()
	f.writeUint8(status_ok)
	f.writeUint32(f.pos)
}

func (f *fconn) a_readdir() {
	f.readTimeLimit()
	pos := f.readUint32()
	count := f.readUint16()

	fp := f.getFile(pos)

	if count > 1000 {
		panic(NoticeError("一次读取的文件夹数量过多"))
	}

	fi, err := fp.Readdir(int(count))
	if err != nil {
		panic(WarningError(err.Error()))
	}

	num := uint16(len(fi))

	f.writeTimeLimit()
	f.writeUint8(status_ok)
	f.writeUint16(num)

	for _, val := range fi {
		f.writeTimeLimit()
		f.writeString(val.Name())
	}
}

func (f *fconn) a_closedir() {
	f.readTimeLimit()
	pos := f.readUint32()

	fp := f.getFile(pos)

	err := fp.Close()
	if err != nil {
		panic(WarningError(err.Error()))
	}

	f.writeTimeLimit()
	f.writeUint8(status_ok)
}

func (f *fconn) a_mkdir() {
	f.readTimeLimit()
	name := f.readString()
	rec := f.readUint8()

	var err error

	if rec == 0 {
		err = fs.Mkdir(name)
	} else {
		err = fs.MkdirAll(name)
	}

	if err != nil {
		panic(WarningError(err.Error()))
	}

	f.writeTimeLimit()
	f.writeUint8(status_ok)
}

func (f *fconn) a_rmdir() {
	f.readTimeLimit()
	name := f.readString()
	rec := f.readUint8()

	var err error
	if rec == 0 {
		err = fs.Remove(name)
	} else {
		err = fs.RemoveAll(name)
	}

	if err != nil {
		panic(WarningError(err.Error()))
	}

	f.writeTimeLimit()
	f.writeUint8(status_ok)
}

func (f *fconn) a_rename() {
	f.readTimeLimit()
	name := f.readString()
	to := f.readString()

	err := fs.Rename(name, to)
	if err != nil {
		panic(WarningError(err.Error()))
	}

	f.writeTimeLimit()
	f.writeUint8(status_ok)
}

func (f *fconn) a_stat() {
	f.readTimeLimit()
	name := f.readString()

	fi, err := fs.Stat(name)
	if err != nil {
		panic(WarningError(err.Error()))
	}

	var is_dir uint8 = 0
	if fi.IsDir() {
		is_dir = 1
	}

	f.writeTimeLimit()
	f.writeUint8(status_ok)
	f.writeUint8(is_dir)
	f.writeInt64(fi.Size())
	f.writeInt64(fi.ModTime().Unix())
}

func (f *fconn) a_lstat() {
	f.readTimeLimit()
	name := f.readString()

	fi, err := fs.Lstat(name)
	if err != nil {
		panic(WarningError(err.Error()))
	}

	var is_dir uint8 = 0
	if fi.IsDir() {
		is_dir = 1
	}

	f.writeTimeLimit()
	f.writeUint8(status_ok)
	f.writeUint8(is_dir)
	f.writeInt64(fi.Size())
	f.writeInt64(fi.ModTime().Unix())
	f.conn.SetReadDeadline(time.Now().Add(actionTimeout))
}

// --------- 字符串读写 ----------

func (f *fconn) readString() string {
	var strlen uint16
	err := binary.Read(f.bufrw, binary.BigEndian, &strlen)
	if err != nil {
		panic(FatalError(err.Error()))
	}

	//极限是64k(uint16)
	data := make([]byte, strlen)
	_, err = f.bufrw.Read(data)
	if err != nil {
		panic(FatalError(err.Error()))
	}

	return string(data)
}

func (f *fconn) writeString(str string) {
	//极限是64k(uint16)
	if len(str) > 65535 {
		panic(FatalError("写入的字符串过长"))
	}

	num := uint16(len(str))

	err := binary.Write(f.bufrw, binary.BigEndian, num)
	if err != nil {
		panic(FatalError(err.Error()))
	}

	_, err = f.bufrw.WriteString(str)
	if err != nil {
		panic(FatalError(err.Error()))
	}
}

//------------------

func (f *fconn) writeError(str ...interface{}) {
	f.writeTimeLimit()

	f.writeUint8(status_fail)

	txt := fmt.Sprint(str...)
	f.writeString(txt)
	f.flush()
}

// ------ 超时 ----------------

func (f *fconn) idleTimeLimit() {
	err := f.conn.SetReadDeadline(time.Now().Add(idleTimeout))
	if err != nil {
		panic(FatalError(err.Error()))
	}
}

func (f *fconn) readTimeLimit() {
	err := f.conn.SetReadDeadline(time.Now().Add(actionTimeout))
	if err != nil {
		panic(FatalError(err.Error()))
	}
}

func (f *fconn) writeTimeLimit() {
	err := f.conn.SetWriteDeadline(time.Now().Add(actionTimeout))
	if err != nil {
		panic(FatalError(err.Error()))
	}
}

// ------ 读取 int ----------------

func (f *fconn) readInt8() (number int8) {
	err := binary.Read(f.bufrw, binary.BigEndian, &number)
	if err != nil {
		panic(FatalError(err.Error()))
	}
	return
}

func (f *fconn) readInt16() (number int16) {
	err := binary.Read(f.bufrw, binary.BigEndian, &number)
	if err != nil {
		panic(FatalError(err.Error()))
	}
	return
}

func (f *fconn) readInt32() (number int32) {
	err := binary.Read(f.bufrw, binary.BigEndian, &number)
	if err != nil {
		panic(FatalError(err.Error()))
	}
	return
}

func (f *fconn) readInt64() (number int64) {
	err := binary.Read(f.bufrw, binary.BigEndian, &number)
	if err != nil {
		panic(FatalError(err.Error()))
	}
	return
}

// ------ 读取 uint ----------------

func (f *fconn) readUint8() (number uint8) {
	err := binary.Read(f.bufrw, binary.BigEndian, &number)
	if err != nil {
		panic(FatalError(err.Error()))
	}
	return
}

func (f *fconn) readUint16() (number uint16) {
	err := binary.Read(f.bufrw, binary.BigEndian, &number)
	if err != nil {
		panic(FatalError(err.Error()))
	}
	return
}

func (f *fconn) readUint32() (number uint32) {
	err := binary.Read(f.bufrw, binary.BigEndian, &number)
	if err != nil {
		panic(FatalError(err.Error()))
	}
	return
}

func (f *fconn) readUint64() (number uint64) {
	err := binary.Read(f.bufrw, binary.BigEndian, &number)
	if err != nil {
		panic(FatalError(err.Error()))
	}
	return
}

// ------ 写入 int ----------------

func (f *fconn) writeInt8(number int8) {
	err := binary.Write(f.bufrw, binary.BigEndian, &number)
	if err != nil {
		panic(FatalError(err.Error()))
	}
}

func (f *fconn) writeInt16(number int16) {
	err := binary.Write(f.bufrw, binary.BigEndian, &number)
	if err != nil {
		panic(FatalError(err.Error()))
	}
}

func (f *fconn) writeInt32(number int32) {
	err := binary.Write(f.bufrw, binary.BigEndian, &number)
	if err != nil {
		panic(FatalError(err.Error()))
	}
}

func (f *fconn) writeInt64(number int64) {
	err := binary.Write(f.bufrw, binary.BigEndian, &number)
	if err != nil {
		panic(FatalError(err.Error()))
	}
}

// ------ 写入 uint ----------------

func (f *fconn) writeUint8(number uint8) {
	err := binary.Write(f.bufrw, binary.BigEndian, &number)
	if err != nil {
		panic(FatalError(err.Error()))
	}
}

func (f *fconn) writeUint16(number uint16) {
	err := binary.Write(f.bufrw, binary.BigEndian, &number)
	if err != nil {
		panic(FatalError(err.Error()))
	}
}

func (f *fconn) writeUint32(number uint32) {
	err := binary.Write(f.bufrw, binary.BigEndian, &number)
	if err != nil {
		panic(FatalError(err.Error()))
	}
}

func (f *fconn) writeUint64(number uint64) {
	err := binary.Write(f.bufrw, binary.BigEndian, &number)
	if err != nil {
		panic(FatalError(err.Error()))
	}
}

// ------ 数据读写 ----------------

func (f fconn) writeChunkedFromReader(r io.Reader) {
	//1k buf
	buf := make([]byte, 2048)

	for {
		f.writeTimeLimit()

		n, err := r.Read(buf)
		if err != nil {
			//分段结束
			f.writeUint16(0)
			if err == io.EOF {
				return
			}
			//响应错误
			panic(WarningError(err.Error()))
		}

		if n > 0 {
			f.writeData(buf[:n])
		}
	}
}

func (f fconn) writeData(buf []byte) {
	f.writeUint16(uint16(len(buf)))

	_, err := f.bufrw.Write(buf)
	if err != nil {
		panic(FatalError(err.Error()))
	}
}

func (f *fconn) readChunkedToWriter(w io.Writer) {
	for {
		f.readTimeLimit()

		buf := f.readData()
		//结束
		if buf == nil {
			return
		}

		_, err := w.Write(buf)
		if err != nil {
			panic(FatalError(err.Error()))
		}
	}
}

func (f *fconn) readData() []byte {
	count := f.readUint16()

	if count == 0 {
		return nil
	}

	//极限是64k(uint16)
	buf := make([]byte, count)

	_, err := f.bufrw.Read(buf)
	if err != nil {
		panic(FatalError(err.Error()))
	}

	return buf
}

// ------ 杂项 ----------------

func (f *fconn) flush () {
	err := f.bufrw.Flush()
	if err != nil {
		panic(FatalError(err.Error()))
	}
}


func (f *fconn) getFile (pos uint32) *file {
	fp := f.files[pos]
	if fp == nil {
		panic(FatalError("文件描术符错误"))
	}

	return fp
}

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

//速度kbps/秒
var minTransferSpeed = 5

type FatalError string
func (e FatalError) Error() string {
	return e
}

type WarningError string
func (e WarningError) Error() string {
	return e
}

type NoticeError string
func (e NoticeError) Error() string {
	return e
}

type fconn struct{
	conn net.Conn
	bufrw *bufio.ReadWriter
	files map[uint32]*os.File
	pos uint32
	ok bool
}

func fconnInit(w http.ResponseWriter, r *http.Request, password) (*fconn, bool) {
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

	var err error
	conn, bufrw, err = hj.Hijack()
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
		return false
	}

	ok := f.auth(token)
	if !ok {
		conn.Close()
		return false
	}

	f = &fconn{}
	f.conn = conn
	f.bufrw = bufrw

	return f, true
}

func (f *fconn) close() {
	for _, fp := range f.files {
		fp.Close()
	}
}

func (f *fconn) auth(token) bool {
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
	defer func() {
		if x := recover(); x != nil {
			switch v := x.(type) {
			case FatalError :
				log.Println("[Fatal]", value, f.conn.RemoteAddr())
			default:
				panic(x)
			}
		}
	}()

	for f.ok {
		f.idleTimeLimit()
		code := f.readUint16()

		f._run(code)
	}
}

func (f *fconn) _run(code uint16) {
	defer func() {
		if x := recover(); x != nil {
			switch v := x.(type) {
			case NoticeError :
				log.Println("[Notice]", value)
				f.writeError("[Notice]", value)
			case WarningError :
				log.Println("[Warning]", value)
				f.writeError("[Warning]", value)
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
			panic(FatalError("未定义的指令"))
	}

	f.Flush()
}


func (f *fconn) a_fopen() {
	f.readTimeLimit()
	name := f.readString()
	falg := f.readInt32()

	fp, err := fs.openFile(name, flag)
	if err != nil {
		panic(WarningError(err))
	}

	f.pos++
	f.files[f.pos] = fp

	f.writeTimeLimit()
	f.WriteUint8(status_ok)
	f.WriteUint32(f.pos)
}

func (f *fconn) a_fread() {
	f.readTimeLimit()
	pos := f.ReadUint32()
	count := f.ReadInt64()

	fp := f.getFile(pos)
	reader := io.LimitReader(fp, count)

	f.writeDataTimeLimit()
	f.WriteUint8(status_ok)
	f.writeChunkedFromReader(reader)
}

func (f *fconn) a_fwrite() {
	f.readTimeLimit()
	pos := f.ReadUint32()

	fp := f.getFile(pos)

	f.ReadChunkedToWriter(fp)

	f.writeTimeLimit()
	f.WriteUint8(status_ok)
}

func (f *fconn) a_flock() {
}

func (f *fconn) a_funlock() {
}

func (f *fconn) a_flush() {
	f.readTimeLimit()
	pos := f.ReadUint32()
	fp := f.getFile(pos)

	err = fp.Sync()
	if err != nil {
		panic(WarningError(err))
	}

	f.writeTimeLimit()
	f.WriteUint8(status_ok)
}

func (f *fconn) a_fseek() {
	f.readTimeLimit()
	pos := f.ReadUint32()
	offset := f.ReadInt64()
	mode := f.ReadUint8()

	fp := f.getFile(pos)

	var off int64
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
		panic(WarningError(err))
	}

	f.writeTimeLimit()
	f.WriteUint8(status_ok)
	f.Writeint64(off)
}

func (f *fconn) a_fstat() {
	f.readTimeLimit()
	pos := f.ReadUint32()

	fp := f.getFile(pos)

	fi, err := fp.Stat()
	if err != nil {
		panic(WarningError(err))
	}

	f.writeTimeLimit()
	f.WriteUint8(status_ok)
	f.WriteUint32(fi.Mode())
	f.WriteInt64(fi.Size())
	f.WriteInt64(fi.ModTime().Unix())
}

func (f *fconn) a_ftrucate() {
	f.readTimeLimit()
	pos := f.ReadUint32()
	size := f.ReadInt64()

	fp := f.getFile(pos)

	err = fp.Truncate(size)
	if err != nil {
		panic(WarningError(err))
	}

	f.writeTimeLimit()
	f.WriteUint8(status_ok)
}

func (f *fconn) a_fclose() {
	f.readTimeLimit()
	pos := f.ReadUint32()

	fp := f.getFile(pos)

	fi, err := fp.Close()
	if err == nil {
		panic(WarningError(err))
	}

	f.writeTimeLimit()
	f.WriteUint8(status_ok)
}

func (f *fconn) a_opendir() {
	f.readTimeLimit()
	name := f.readString()

	fp, err := fs.open(name)
	if err != nil {
		panic(WarningError(err))
	}

	f.pos++
	f.files[f.pos] = fp

	f.writeTimeLimit()
	f.WriteUint8(status_ok)
	f.WriteUint32(pos)
}

func (f *fconn) a_readdir() {
	f.readTimeLimit()
	pos := f.ReadUint32()
	count := f.ReadUint16()

	fp := f.getFile(pos)

	if count > 1000 {
		panic(NoticeError("一次读取的文件夹数量过多"))
	}

	fi, err = fp.Readdir(int(count))
	if err != nil {
		panic(WarningError(err))
	}

	num := uint16(len(fi))

	f.writeTimeLimit()
	f.WriteUint8(status_ok)
	f.WriteUint16(num)

	f.writeDataTimeLimit()
	for _, val := range fi {
		f.writeString(val.Name())
	}
}

func (f *fconn) a_closedir() {
	f.readTimeLimit()
	pos := f.ReadUint32()

	fp := f.getFile(pos)

	fi, err := fp.Close()
	if err == nil {
		panic(WarningError(err))
	}

	f.writeTimeLimit()
	f.WriteUint8(status_ok)
}

func (f *fconn) a_mkdir() {
	f.readTimeLimit()
	name := f.readString()
	mode := f.ReadUint32()
	rec := f.ReadUint8()

	if rec == 0 {
		err = os.Mkdir(name, mode)
	} else {
		err = os.MkdirALL(name, mode)
	}

	if err != nil {
		panic(WarningError(err))
	}

	f.writeTimeLimit()
	f.WriteUint8(status_ok)
}

func (f *fconn) a_rmdir() {
	f.readTimeLimit()
	name := f.readString()
	rec := f.ReadUint8()

	if rec == 0 {
		err = os.Remove(name)
	} else {
		err = os.RemoveAll(name)
	}

	if err != nil {
		panic(WarningError(err))
	}

	f.writeTimeLimit()
	f.WriteUint8(status_ok)
}

func (f *fconn) a_rename() {
	f.readTimeLimit()
	name := f.readString()
	to := f.readString()

	err = fs.Rename(name, to)
	if err != nil {
		panic(WarningError(err))
	}

	f.writeTimeLimit()
	f.WriteUint8(status_ok)
}

func (f *fconn) a_stat() {
	f.readTimeLimit()
	name := f.readString()

	fi, err := fs.Stat(name)
	if err != nil {
		panic(WarningError(err))
	}

	f.writeTimeLimit()
	f.WriteUint8(status_ok)
	f.WriteUint32(fi.Mode())
	f.WriteInt64(fi.Size())
	f.WriteInt64(fi.ModTime().Unix())
}

func (f *fconn) a_lstat() {
	f.readTimeLimit()
	name := f.readString()

	fi, err := fs.Lstat(name)
	if err != nil {
		panic(WarningError(err))
	}

	f.writeTimeLimit()
	f.WriteUint8(status_ok)
	f.WriteUint32(fi.Mode())
	f.WriteInt64(fi.Size())
	f.WriteInt64(fi.ModTime().Unix())
	f.conn.SetReadDeadline(time.Now().Add(actionTimeout))
}

// --------- 字符串读写 ----------

func (f *fconn) readString() string {
	var strlen uint16
	err := binary.Read(f.bufrw, binary.BigEndian, &strlen)
	if err != nil {
		panic(FatalError(err))
	}

	if strlen > maxString {
		panic(FatalError(err))
	}

	data := make([]byte, strlen)
	_, err = f.bufrw.Read(data)
	if err != nil {
		panic(FatalError(err))
	}

	return string(data)
}

func (f fconn) writeString(str string) {
	if len(str) > maxString {
		panic(FatalError("写入字符串过长"))
	}

	num := uint16(len(str))

	err := binary.Write(f.bufrw, binary.BigEndian, num)
	if err != nil {
		panic(FatalError(err))
	}

	_, err = f.bufrw.WriteString(str)
	if err != nil {
		panic(FatalError(err))
	}
}

func (f fconn) writeError(str ...interface{}) {
	f.writeTimeLimit()

	err := binary.Write(f.bufrw, binary.BigEndian, status_fail)
	if err != nil {
		panic(FatalError(err))
	}

	_, err = f.bufrw.WriteString(fmt.Sprint(str...))
	if err != nil {
		panic(FatalError(err))
	}

	err = f.bufrw.Flush()
	if err != nil {
		panic(FatalError(err))
	}
}

//------------------




// ------ 超时 ----------------

func (f *fconn) idleTimeLimit() {
	err := f.conn.SetReadDeadline(time.Now().Add(idleTimeout))
	if err != nil {
		panic(FatalError(err))
	}
}

func (f *fconn) readTimeLimit() {
	err := f.conn.SetReadDeadline(time.Now().Add(actionTimeout))
	if err != nil {
		panic(FatalError(err))
	}
}

func (f *fconn) readDataTimeLimit(length int) {
	need := length / 1024 / minTransferSpeed

	t := (actionTimeout + need) * time.Second

	err := f.conn.SetReadDeadline(time.Now().Add(t))
	if err != nil {
		panic(FatalError(err))
	}
}

func (f *fconn) writeTimeLimit() {
	err := f.conn.SetWriteDeadline(time.Now().Add(idleTimeout))
	if err != nil {
		panic(FatalError(err))
	}
}

func (f *fconn) writeDataTimeLimit(length int) {
	need := length / 1024 / minTransferSpeed

	t := (actionTimeout + need) * time.Second

	err := f.conn.SetWriteDeadline(time.Now().Add(t))
	if err != nil {
		panic(FatalError(err))
	}
}

// ------ 读取 int ----------------

func (f *fconn) readInt32() int32 {
	var number int32
	err := binary.Read(f.bufrw, binary.BigEndian, &number)
	if err != nil {
		panic(FatalError(err))
	}

	return number
}

// ------ 读取 uint ----------------

func (f *fconn) readUint8(number uint8) {
	err := binary.Write(f.bufrw, binary.BigEndian, &number)
	if err != nil {
		panic(FatalError(err))
	}
}

func (f *fconn) readUint16() uint16 {
	var number uint16
	err := binary.Read(f.bufrw, binary.BigEndian, &number)
	if err != nil {
		panic(FatalError(err))
	}

	return number
}

// ------ 写入 uint ----------------

func (f *fconn) readuInt8(number uint32) {
	err := binary.Write(f.bufrw, binary.BigEndian, &number)
	if err != nil {
		panic(FatalError(err))
	}
}

// ------ 数据读写 ----------------
func (f fconn) writeChunkedFromReader(r io.Reader) {
	//1k buf
	buf := make([]byte, 2048)

	for {
		f.writeTimeLimit()

		n, err := r.Read(buf)
		if err != nil && err != io.EOF {
			//强型结束
			f.WriteUint16(0)
			//响应错误
			panic(WarningError(err))
		}

		if n > 0 {
			f.writeData(buf[:n])
		}

		//分段结束
		if err == io.EOF {
			f.WriteUint16(0)
			f.WriteUint8(status_ok)
		}
	}
}

func (f fconn) writeData(buf []byte) {
	f.WriteUint16(uint16(len(buf)))

	err := f.bufrw.Write(buf)
	if err != nil {
		panic(FatalError(err))
	}
}

func (f *fconn) ReadChunkedToWriter(w io.Writer) {
	for {
		f.readTimeLimit()

		buf := f.ReadData()
		//结束
		if buf == nil {
			return
		}

		_, err := w.Write(buf)
		if err != nil {
			panic(FatalError(err))
		}
	}
}

func (f *fconn) ReadData() []byte {
	count := f.ReadUint16()

	if count == 0 {
		return nil
	}

	//极限是15k(uint16)
	buf := make([]byte, count)

	_, err := f.bufrw.Read(buf)
	if err != nil {
		panic(FatalError(err))
	}

	return buf
}

// ------ 杂项 ----------------

func (f *fconn) Flush () {
	err := f.bufrw.Flush()
	if err != nil {
		panic(FatalError(err))
	}
}


func (f *fconn) getFile (pos uint32) *os.File {
	fp := f.files[pos]
	if fp == nil {
		panic(FatalError("文件描术符错误"))
	}

	return fp
}

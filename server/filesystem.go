package main

import (
	"sync"
	"os"
	"errors"
	"path"
	"path/filepath"
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
	CODE_FILE_EOF = 8
	CODE_FILE_FLUSH = 9
	CODE_FILE_TRUCATE = 10
	CODE_FILE_CLOSE = 11

	CODE_DIR_OPEN = 1001
	CODE_DIR_READ = 1002
	CODE_DIR_CLOSE = 1003

	CODE_MKDIR = 2001
	CODE_RMDIR = 2002
	CODE_COPY = 2003
	CODE_MOVE = 2004
	CODE_STAT = 2005
	CODE_LSTAT = 2006
)

var (
	//成功
	status_ok uint8 = 0
	//失败
	status_fail uint8 = 1
)

type filesystem struct {
	mu sync.Mutex
	files map[string]sync.RWMutex
	nums map[string]int
	fileMode os.FileMode
	rootdir string
}

func (f *filesystem) pathToFile(p string) string {
	return filepath.Join(f.rootdir, filepath.FromSlash(path.Clean("/" + p)))
}

func (f *filesystem) Open(name string) (*file, error) {
	name = f.pathToFile(name)
	if name == "." {
		return nil, errors.New("File Name Error");
	}

	fp, err := os.Open(name)
	if err != nil {
		return nil, err
	}

	ff := &file{File:fp,name:name}

	return ff, nil
}

func (f *filesystem) OpenFile(name string, flag int) (*file, error) {
	name = f.pathToFile(name)
	if name == "." {
		return nil, errors.New("File Name Error");
	}

	fp, err := os.OpenFile(name, flag, fs.fileMode)
	if err != nil {
		return nil, err
	}

	ff := &file{File:fp,name:name}

	return ff, nil
}

type file struct{
	*os.File
	name string
	root *filesystem
	ghost bool
}

func (f *file) Close() error {
	err := f.File.Close()

	if err != nil || f.ghost {
		err := os.Remove(f.name)
		if err != nil {
			panic(err)
		}
	}

	return err
}

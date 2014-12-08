package main

import (
	"sync"
	"os"
	"errors"
	"path"
	"path/filepath"
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

func (f *filesystem) Mkdir(name string) error {
	name = f.pathToFile(name)
	if name == "." {
		return errors.New("File Name Error");
	}

	err := os.Mkdir(name, fs.fileMode)
	return err
}

func (f *filesystem) MkdirAll(name string) error {
	name = f.pathToFile(name)
	if name == "." {
		return errors.New("File Name Error");
	}

	err := os.MkdirAll(name, fs.fileMode)
	return err
}

func (f *filesystem) Remove(name string) error {
	name = f.pathToFile(name)
	if name == "." {
		return errors.New("File Name Error");
	}

	err := os.Remove(name)
	return err
}

func (f *filesystem) RemoveAll(name string) error {
	name = f.pathToFile(name)
	if name == "." {
		return errors.New("File Name Error");
	}

	err := os.RemoveAll(name)
	return err
}

func (f *filesystem) Rename(name, to string) error {
	name = f.pathToFile(name)
	if name == "." {
		return errors.New("File Name Error");
	}

	to = f.pathToFile(to)
	if to == "." {
		return errors.New("File Name Error");
	}

	err := os.Rename(name, to)
	return err
}

func (f *filesystem) Lstat(name string) (os.FileInfo, error) {
	name = f.pathToFile(name)
	if name == "." {
		return nil, errors.New("File Name Error");
	}

	return os.Lstat(name)
}

func (f *filesystem) Stat(name string) (os.FileInfo, error) {
	name = f.pathToFile(name)
	if name == "." {
		return nil, errors.New("File Name Error");
	}

	return os.Lstat(name)
}

type file struct{
	*os.File
	name string
	root *filesystem
	ghost bool
}

func (f *file) Close() error {
	err := f.File.Close()

	if f.ghost {
		err := os.Remove(f.name)
		if err != nil {
			panic(err)
		}
	}

	return err
}

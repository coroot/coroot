package utils

import (
	"io/fs"
	"net/http"
	"os"
	"time"
)

type StaticFSWrapper struct {
	http.FileSystem
	modTime time.Time
}

func NewStaticFSWrapper(fs fs.FS) *StaticFSWrapper {
	return &StaticFSWrapper{FileSystem: http.FS(fs), modTime: time.Now()}
}

func (f *StaticFSWrapper) Open(name string) (http.File, error) {
	file, err := f.FileSystem.Open(name)
	return &StaticFileWrapper{File: file, modTime: f.modTime}, err
}

type StaticFileWrapper struct {
	http.File
	modTime time.Time
}

func (f *StaticFileWrapper) Stat() (os.FileInfo, error) {
	fileInfo, err := f.File.Stat()
	return &StaticFileInfoWrapper{FileInfo: fileInfo, modTime: f.modTime}, err
}

type StaticFileInfoWrapper struct {
	os.FileInfo
	modTime time.Time
}

func (f *StaticFileInfoWrapper) ModTime() time.Time {
	return f.modTime
}

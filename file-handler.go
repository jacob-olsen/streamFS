package main

import (
	"context"
	"fmt"
	"syscall"

	"github.com/hanwen/go-fuse/v2/fs"
	"github.com/hanwen/go-fuse/v2/fuse"
)

func (f *StreamFile) Open(ctx context.Context, flags uint32) (fs.FileHandle, uint32, syscall.Errno) {
	//file trust
	return nil, 0, 0
}
func (f *StreamFile) Read(ctx context.Context, fh fs.FileHandle, dest []byte, off int64) (fuse.ReadResult, syscall.Errno) {
	fullContent := []byte("This is the raw data inside the file.\nIt works!")

	end := int(off) + len(dest)
	if end > len(fullContent) {
		end = len(fullContent)
	}

	if int(off) >= len(fullContent) {
		return fuse.ReadResultData(nil), 0
	}

	return fuse.ReadResultData(fullContent[off:end]), 0
}
func (f *StreamFile) Write(ctx context.Context, fh fs.FileHandle, data []byte, off int64) (uint32, syscall.Errno) {

	fmt.Printf("Writing %d bytes to file at offset %d\n", len(data), off)

	fmt.Printf("Data: %s\n", string(data))

	return uint32(len(data)), 0
}

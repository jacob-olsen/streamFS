package main

import (
	"context"
	"flag"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/hanwen/go-fuse/v2/fs"
	"github.com/hanwen/go-fuse/v2/fuse"
)

type StreamRoot struct {
	fs.Inode
}
type StreamFile struct {
	fs.Inode
}

func main() {
	mountPoint := flag.String("mnt", "./mnt", "Directory to mount the filesystem")
	debug := flag.Bool("debug", false, "Enable FUSE debug logging")
	flag.Parse()

	if err := os.MkdirAll(*mountPoint, 0755); err != nil {
		log.Fatalf("Could not create mount directory: %v", err)
	}

	root := &StreamRoot{}

	sec := time.Second
	opts := &fs.Options{
		AttrTimeout:  &sec,
		EntryTimeout: &sec,
		MountOptions: fuse.MountOptions{
			Debug:      *debug,
			AllowOther: true,
			Name:       "streamfs",
			FsName:     "StreamFS",
		},
	}

	server, err := fs.Mount(*mountPoint, root, opts)
	if err != nil {
		log.Fatalf("Mount failed: %v", err)
	}

	log.Printf("StreamFS is live!")
	log.Printf("Mount: %s", *mountPoint)

	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt, syscall.SIGTERM)

	go func() {
		<-c
		log.Println("\nReceived signal, unmounting...")
		server.Unmount()
	}()

	server.Wait()
}

func (f *StreamRoot) Readdir(ctx context.Context) (fs.DirStream, syscall.Errno) {
	var entries []fuse.DirEntry

	entries = append(entries, fuse.DirEntry{
		Name: "local_cache_log.txt",
		Mode: fuse.S_IFREG,
		Ino:  11,
	})

	return fs.NewListDirStream(entries), 0
}

func (r *StreamRoot) Lookup(ctx context.Context, name string, out *fuse.EntryOut) (*fs.Inode, syscall.Errno) {

	if name == "local_cache_log.txt" {

		fileLogic := &StreamFile{}
		//r.StableAttr().Ino

		stable := fs.StableAttr{
			Mode: fuse.S_IFREG,
			Ino:  11,
		}
		child := r.NewInode(ctx, fileLogic, stable)

		out.Attr.Mode = fuse.S_IFREG | 0644
		out.Attr.Size = uint64(len("This is the raw data inside the file.\nIt works!"))

		return child, 0
	}

	return nil, syscall.ENOENT
}

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

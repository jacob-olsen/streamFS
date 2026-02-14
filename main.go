package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/hanwen/go-fuse/v2/fs"
	"github.com/hanwen/go-fuse/v2/fuse"
)

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

type StreamRoot struct {
	fs.Inode
}

func (f *StreamRoot) Readdir(ctx context.Context) (fs.DirStream, syscall.Errno) {
	var entries []fuse.DirEntry

	entries = append(entries, fuse.DirEntry{
		Name: "remote_game_file.dat",
		Mode: fuse.S_IFREG,
		Ino:  10,
	})

	entries = append(entries, fuse.DirEntry{
		Name: "local_cache_log.txt",
		Mode: fuse.S_IFREG,
		Ino:  11,
	})

	return fs.NewListDirStream(entries), 0
}

func (r *StreamRoot) Lookup(ctx context.Context, name string, out *fuse.EntryOut) (*fs.Inode, syscall.Errno) {

	if name == "remote_game_file.dat" || name == "local_cache_log.txt" {

		fileLogic := &fs.MemRegularFile{
			Data: []byte("Data for " + name),
			Attr: fuse.Attr{Mode: 0644},
		}
		fmt.Println(r.StableAttr().Ino)

		stable := fs.StableAttr{Mode: fuse.S_IFREG}
		child := r.NewPersistentInode(ctx, fileLogic, stable)

		out.Attr.Mode = fuse.S_IFREG | 0644
		out.Attr.Size = uint64(len("Data for " + name))

		return child, 0
	}

	return nil, syscall.ENOENT
}

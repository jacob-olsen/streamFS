package main

import (
	"database/sql"
	"flag"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/hanwen/go-fuse/v2/fs"
	"github.com/hanwen/go-fuse/v2/fuse"

	_ "modernc.org/sqlite"
)

var db *sql.DB

type StreamRoot struct {
	fs.Inode
}
type StreamFile struct {
	fs.Inode
}
type StreamLink struct {
	fs.Inode
}

func main() {
	mountPoint := flag.String("mnt", "./mnt", "Directory to mount the filesystem")
	sql := flag.String("sql", "./StreamFS.sqlite", "Directory to mount the filesystem")
	debug := flag.Bool("debug", false, "Enable FUSE debug logging")
	flag.Parse()

	if err := os.MkdirAll(*mountPoint, 0755); err != nil {
		log.Fatalf("Could not create mount directory: %v", err)
	}

	initDB(*sql)
	defer db.Close()

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

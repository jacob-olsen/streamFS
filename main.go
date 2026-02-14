package main

import (
	"flag"
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
	dataDir := flag.String("data", "./real_data", "Directory to stream data from")
	debug := flag.Bool("debug", false, "Enable FUSE debug logging")
	flag.Parse()

	if err := os.MkdirAll(*mountPoint, 0755); err != nil {
		log.Fatalf("Could not create mount directory: %v", err)
	}

	root, err := fs.NewLoopbackRoot(*dataDir)
	if err != nil {
		log.Fatalf("Failed to init loopback root: %v", err)
	}

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

	log.Printf("🚀 StreamFS is live!")
	log.Printf("   ├── Mount: %s", *mountPoint)
	log.Printf("   └── Source: %s", *dataDir)

	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt, syscall.SIGTERM)

	go func() {
		<-c
		log.Println("\nReceived signal, unmounting...")
		server.Unmount()
	}()

	server.Wait()
}

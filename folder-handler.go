package main

import (
	"context"
	"syscall"

	"github.com/hanwen/go-fuse/v2/fs"
	"github.com/hanwen/go-fuse/v2/fuse"
)

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

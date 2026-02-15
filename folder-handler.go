package main

import (
	"context"
	"log"
	"syscall"

	"github.com/hanwen/go-fuse/v2/fs"
	"github.com/hanwen/go-fuse/v2/fuse"
)

func (r *StreamRoot) Readdir(ctx context.Context) (fs.DirStream, syscall.Errno) {
	var entries []fuse.DirEntry

	entries = DB_List_meta(r.StableAttr().Ino)

	return fs.NewListDirStream(entries), 0
}

func (r *StreamRoot) Lookup(ctx context.Context, name string, out *fuse.EntryOut) (*fs.Inode, syscall.Errno) {

	dbID, dbMode, size, uid, gid, mtime, atime, ctime, mising := DB_Lookup_meta(r.StableAttr().Ino, name)

	if mising {
		return nil, syscall.ENOENT
	}

	out.Attr.Size = size
	out.Attr.Uid = uid
	out.Attr.Gid = gid
	out.Attr.Mtime = mtime
	out.Attr.Atime = atime
	out.Attr.Ctime = ctime

	switch dbMode & syscall.S_IFMT {
	case fuse.S_IFDIR:
		// It's a Folder
		stable := fs.StableAttr{
			Mode: fuse.S_IFDIR,
			Ino:  uint64(dbID)}
		return r.NewInode(ctx, &StreamRoot{}, stable), 0

	case fuse.S_IFREG:
		// It's a File
		stable := fs.StableAttr{
			Mode: fuse.S_IFREG,
			Ino:  uint64(dbID)}
		return r.NewInode(ctx, &StreamFile{}, stable), 0

	case fuse.S_IFLNK:
		stable := fs.StableAttr{
			Mode: fuse.S_IFLNK,
			Ino:  uint64(dbID)}
		return r.NewInode(ctx, &StreamLink{}, stable), 0

	default:
		// Unknown/Corrupt type in DB
		log.Printf("Unknown file type for ID %d: %o", dbID, dbMode)
		return nil, syscall.EIO
	}
}

func (r *StreamRoot) Mkdir(ctx context.Context, name string, mode uint32, out *fuse.EntryOut) (*fs.Inode, syscall.Errno) {

	caller, ok := fuse.FromContext(ctx)
	if !ok {
		// This theoretically only happens in unit tests without a mock
		return nil, syscall.EIO
	}

	id, _ := DB_mkDir(r.StableAttr().Ino, name, caller.Uid, caller.Gid, fuse.S_IFDIR|0755)
	stable := fs.StableAttr{
		Mode: fuse.S_IFDIR,
		Ino:  uint64(id),
	}

	child := r.NewInode(ctx, &StreamRoot{}, stable)

	out.Attr.Mode = fuse.S_IFDIR | 0755
	out.Attr.Ino = stable.Ino

	return child, 0
}

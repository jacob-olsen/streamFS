package main

import (
	"context"
	"fmt"
	"syscall"
	"time"

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
func (r *StreamRoot) Create(ctx context.Context, name string, flags uint32, mode uint32, out *fuse.EntryOut) (*fs.Inode, fs.FileHandle, uint32, syscall.Errno) {
	caller, _ := fuse.FromContext(ctx)
	mode = mode | syscall.S_IFREG

	inod, err := DB_mkMeta(r.StableAttr().Ino, name, caller.Uid, caller.Gid, mode)
	if err != nil {
		return nil, nil, 0, syscall.EIO
	}

	out.Attr.Ino = inod
	out.Attr.Mode = mode
	out.Attr.Uid = caller.Uid
	out.Attr.Gid = caller.Gid
	out.Attr.Size = 0

	now := uint64(time.Now().Unix())
	out.Attr.Atime = now
	out.Attr.Mtime = now
	out.Attr.Ctime = now

	child := r.NewInode(ctx, &StreamFile{}, fs.StableAttr{
		Mode: mode,
		Ino:  inod,
	})

	return child, &StreamFile{}, 0, fs.OK
}
func (r *StreamFile) Getattr(ctx context.Context, fh fs.FileHandle, out *fuse.AttrOut) syscall.Errno {
	_, _, mode, size, uid, gid, mtime, atime, ctime, mising := DB_Getattr(r.StableAttr().Ino)
	if mising {
		return syscall.ENOENT
	}

	out.Attr.Ino = r.StableAttr().Ino
	out.Attr.Size = size
	out.Attr.Uid = uid
	out.Attr.Gid = gid
	out.Attr.Mtime = mtime
	out.Attr.Atime = atime
	out.Attr.Ctime = ctime
	out.Attr.Mode = mode
	out.Nlink = 1

	return fs.OK
}

func (r *StreamFile) Setattr(ctx context.Context, fh fs.FileHandle, in *fuse.SetAttrIn, out *fuse.AttrOut) syscall.Errno {
	var update bool
	update = false

	fmt.Printf("DEBUG: SetAttr called on Inode %d with Mode %o\n", r.StableAttr().Ino, in.Mode)

	_, _, mode, size, uid, gid, mtime, atime, ctime, mising := DB_Getattr(r.StableAttr().Ino)
	if mising {
		return syscall.ENOENT
	}

	if in.Valid&fuse.FATTR_ATIME != 0 {
		newatime := in.Atime
		if in.Valid&fuse.FATTR_ATIME_NOW != 0 {
			newatime = uint64(time.Now().Unix())
		}
		if newatime != atime {
			update = true
			atime = newatime
		}
	}

	if in.Valid&fuse.FATTR_MTIME != 0 {
		newmtime := in.Mtime
		if in.Valid&fuse.FATTR_MTIME_NOW != 0 {
			newmtime = uint64(time.Now().Unix())
		}
		if newmtime != mtime {
			update = true
			mtime = newmtime
		}
	}

	if in.Valid&fuse.FATTR_SIZE != 0 {
		if size != in.Size {
			size = in.Size
			update = true
		}
	}

	if in.Valid&fuse.FATTR_MODE != 0 {
		currentType := mode & syscall.S_IFMT
		newPermissions := in.Mode & 07777

		newMode := currentType | newPermissions
		if newMode != mode {
			mode = newMode
			update = true
		}
	}

	if in.Valid&fuse.FATTR_UID != 0 {
		if uid != in.Uid {
			uid = in.Uid
			update = true
		}
	}
	if in.Valid&fuse.FATTR_GID != 0 {
		if gid != in.Gid {
			gid = in.Gid
			update = true
		}
	}

	if update {
		ctime = uint64(time.Now().Unix())
		err := DB_Setattr(r.StableAttr().Ino, mode, size, uid, gid, mtime, atime, ctime)
		if err != nil {
			return syscall.EIO
		}
	}

	out.Attr.Size = size
	out.Attr.Uid = uid
	out.Attr.Gid = gid
	out.Attr.Mtime = mtime
	out.Attr.Atime = atime
	out.Attr.Ctime = ctime
	out.Attr.Mode = mode
	out.Nlink = 1

	return fs.OK
}

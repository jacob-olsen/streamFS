package main

import (
	"context"
	"fmt"
	"log"
	"syscall"
	"time"

	"github.com/hanwen/go-fuse/v2/fs"
	"github.com/hanwen/go-fuse/v2/fuse"
)

func (r *StreamRoot) Readdir(ctx context.Context) (fs.DirStream, syscall.Errno) {
	var entries []fuse.DirEntry

	entries = append(entries, fuse.DirEntry{
		Name: ".",
		Mode: r.StableAttr().Mode,
		Ino:  r.StableAttr().Ino,
	})

	entries = append(entries, DB_List_meta(r.StableAttr().Ino)...)

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
	out.Attr.Mode = dbMode

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

func (r *StreamRoot) Getattr(ctx context.Context, fh fs.FileHandle, out *fuse.AttrOut) syscall.Errno {
	_, _, mode, size, uid, gid, mtime, atime, ctime, mising := DB_Getattr(r.StableAttr().Ino)
	if mising {
		return syscall.ENOENT
	}

	out.Attr.Size = size
	out.Attr.Uid = uid
	out.Attr.Gid = gid
	out.Attr.Mtime = mtime
	out.Attr.Atime = atime
	out.Attr.Ctime = ctime
	out.Attr.Mode = mode
	out.Nlink = 2 //fix

	return fs.OK
}
func (r *StreamRoot) Setattr(ctx context.Context, fh fs.FileHandle, in *fuse.SetAttrIn, out *fuse.AttrOut) syscall.Errno {
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

func (r *StreamRoot) Mkdir(ctx context.Context, name string, mode uint32, out *fuse.EntryOut) (*fs.Inode, syscall.Errno) {

	caller, ok := fuse.FromContext(ctx)
	if !ok {
		// This theoretically only happens in unit tests without a mock
		return nil, syscall.EIO
	}

	id, _ := DB_mkMeta(r.StableAttr().Ino, name, caller.Uid, caller.Gid, fuse.S_IFDIR|0755)
	stable := fs.StableAttr{
		Mode: fuse.S_IFDIR,
		Ino:  uint64(id),
	}

	child := r.NewInode(ctx, &StreamRoot{}, stable)

	out.Attr.Mode = fuse.S_IFDIR | 0755
	out.Attr.Ino = stable.Ino

	return child, 0
}
func (r *StreamRoot) Rmdir(ctx context.Context, name string) syscall.Errno {
	dbID, _, _, _, _, _, _, _, mising := DB_Lookup_meta(r.StableAttr().Ino, name)
	if mising {
		return syscall.ENOENT
	}
	entries := DB_List_meta(uint64(dbID))
	if len(entries) > 0 {
		return syscall.ENOTEMPTY
	}
	DB_rm_meta(r.StableAttr().Ino, name)
	return fs.OK
}
func (d *StreamRoot) Unlink(ctx context.Context, name string) syscall.Errno {
	mising := DB_rm_meta(d.StableAttr().Ino, name)

	if mising {
		return syscall.ENOENT // File doesn't exist
	}
	return fs.OK
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
func (r *StreamRoot) Rename(ctx context.Context, name string, newGroup fs.InodeEmbedder, newName string, flags uint32) syscall.Errno {
	newDirNode := newGroup.EmbeddedInode()
	newParentID := newDirNode.StableAttr().Ino

	return DB_rename_meta(r.StableAttr().Ino, name, newParentID, newName)
}

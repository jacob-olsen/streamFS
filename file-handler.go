package main

import (
	"context"
	"fmt"
	"syscall"
	"time"

	"github.com/hanwen/go-fuse/v2/fs"
	"github.com/hanwen/go-fuse/v2/fuse"
)

var blockSize int64 = 1024 * 1024

func (f *StreamFile) Open(ctx context.Context, flags uint32) (fs.FileHandle, uint32, syscall.Errno) {
	//file trust
	return nil, 0, 0
}
func (f *StreamFile) Read(ctx context.Context, fh fs.FileHandle, dest []byte, off int64) (fuse.ReadResult, syscall.Errno) {
	// 1. Get the actual file size from your metadata
	_, _, _, size, _, _, _, _, _, _, missing := DB_Getattr(f.StableAttr().Ino)
	if missing || uint64(off) >= size {
		// EOF (End of File) - nothing more to read
		return fuse.ReadResultData(nil), 0
	}

	// 2. Prevent reading past the end of the file
	bytesToRead := int64(len(dest))
	if uint64(off+bytesToRead) > size {
		bytesToRead = int64(size) - off
	}

	var out []byte
	currentOff := off

	// 3. Read blocks until we fulfill the requested size
	for int64(len(out)) < bytesToRead {
		block := currentOff / int64(blockSize)
		pos := currentOff % int64(blockSize)

		// Fetch the block from our DB
		diskBlock, err := DB_read_block(f.StableAttr().Ino, block)
		if err != nil {
			return nil, syscall.EIO
		}

		// 4. THE SPARSE RESTORE:
		// If the DB returned a trimmed block (or a 0-byte hole),
		// we must expand it back to a full 1MB of zeroes for FUSE.
		if int64(len(diskBlock)) < blockSize {
			full := make([]byte, blockSize)
			copy(full, diskBlock)
			diskBlock = full
		}

		// 5. Slice out the chunk FUSE actually asked for
		spaceLeftInBlock := int64(blockSize) - pos
		needed := bytesToRead - int64(len(out))

		chunkSize := spaceLeftInBlock
		if needed < spaceLeftInBlock {
			chunkSize = needed
		}

		// Append this chunk to our output buffer
		out = append(out, diskBlock[pos:pos+chunkSize]...)
		currentOff += chunkSize
	}

	// Hand the bytes back to the OS
	return fuse.ReadResultData(out), 0
}
func (f *StreamFile) Write(ctx context.Context, fh fs.FileHandle, data []byte, off int64) (uint32, syscall.Errno) {
	trueSize := len(data)

	for len(data) > 0 {
		var addetData []byte
		var err error

		block := off / int64(blockSize)

		diskBlock, _ := DB_read_block(f.StableAttr().Ino, block)

		pos := off - (block * int64(blockSize))
		spaceLeft := blockSize - pos
		if len(data) > int(spaceLeft) {
			addetData = data[:spaceLeft]
			data = data[spaceLeft:]
		} else {
			addetData = data
			data = []byte{}
		}

		//expant
		if int64(len(diskBlock)) < blockSize {
			full := make([]byte, 1024*1024)
			copy(full, diskBlock)
			diskBlock = full
		}

		copy(diskBlock[pos:], addetData)

		//srinke
		diskBlock = trimBlock(diskBlock)

		err = DB_write_block(f.StableAttr().Ino, block, diskBlock)
		if err != nil {
			return 0, syscall.EIO
		}

		off += int64(len(addetData))
	}

	_, _, mode, size, uid, gid, _, mtime, atime, ctime, missing := DB_Getattr(f.StableAttr().Ino)
	if !missing && uint64(off) > size {
		DB_Setattr(f.StableAttr().Ino, mode, uint64(off), uid, gid, mtime, atime, ctime)
	}

	return uint32(trueSize), 0
}
func (r *StreamFile) Getattr(ctx context.Context, fh fs.FileHandle, out *fuse.AttrOut) syscall.Errno {
	_, _, mode, size, uid, gid, Nlink, mtime, atime, ctime, mising := DB_Getattr(r.StableAttr().Ino)
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
	out.Nlink = Nlink

	return fs.OK
}

func (r *StreamFile) Setattr(ctx context.Context, fh fs.FileHandle, in *fuse.SetAttrIn, out *fuse.AttrOut) syscall.Errno {
	var update bool
	update = false

	fmt.Printf("DEBUG: SetAttr called on Inode %d with Mode %o\n", r.StableAttr().Ino, in.Mode)

	_, _, mode, size, uid, gid, _, mtime, atime, ctime, mising := DB_Getattr(r.StableAttr().Ino)
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

func trimBlock(s []byte) []byte {
	for i := len(s) - 1; i >= 0; i-- {
		if s[i] != 0 {
			return s[:i+1]
		}
	}
	return s[:0]
}

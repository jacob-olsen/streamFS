package main

import (
	"database/sql"
	"fmt"
	"log"
	"os"
	"syscall"
	"time"

	"github.com/hanwen/go-fuse/v2/fs"
	"github.com/hanwen/go-fuse/v2/fuse"
)

func initDB(dbPath string) {
	var err error

	db, err = sql.Open("sqlite", dbPath)
	if err != nil {
		log.Fatalf("Failed to open database: %v", err)
	}

	if err = db.Ping(); err != nil {
		log.Fatalf("Database unreachable: %v", err)
	}

	query := `
	CREATE TABLE IF NOT EXISTS meta (
    	id INTEGER PRIMARY KEY AUTOINCREMENT,
    	parent_id INTEGER DEFAULT 0,
    	name TEXT NOT NULL,
    	mode INTEGER NOT NULL,
    	size INTEGER DEFAULT 0,
    	uid INTEGER DEFAULT 0,
    	gid INTEGER DEFAULT 0,
		nlink INTEGER DEFAULT 1,
    
		mtime INTEGER NOT NULL DEFAULT (unixepoch()), 
    	atime INTEGER NOT NULL DEFAULT (unixepoch()),
    	ctime INTEGER NOT NULL DEFAULT (unixepoch()),
    
    	is_dirty INTEGER DEFAULT 1,
		is_deleted INTEGER DEFAULT 0,
    
    	UNIQUE(parent_id, name)
	);

	CREATE TABLE IF NOT EXISTS data_block (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
    	atime INTEGER NOT NULL,
    	is_dirty INTEGER DEFAULT 1,
    	bytes BLOB
	);

	CREATE TABLE IF NOT EXISTS file_map (
    	inode_id INTEGER NOT NULL,
    	block_num INTEGER NOT NULL,
    	data_id INTEGER NOT NULL,
    
    	PRIMARY KEY (inode_id, block_num),
    	FOREIGN KEY(inode_id) REFERENCES meta(id) ON DELETE CASCADE
	);

	CREATE INDEX IF NOT EXISTS idx_file_map ON file_map(inode_id, block_num);`

	_, err = db.Exec(query)
	if err != nil {
		log.Fatalf("Failed to initialize tables: %v", err)
	}

	log.Println("Database initialized successfully.")

	_, _, _, _, _, _, _, _, _, _, mising := DB_Getattr(1)
	if mising {
		log.Println("making root folder")
		uid := os.Getuid()
		gid := os.Getgid()
		if uid < 0 {
			uid = 0
		}
		if gid < 0 {
			gid = 0
		}
		DB_mkMeta(0, "ROOTFOLDER", uint32(uid), uint32(gid), fuse.S_IFDIR|0755)
	}
}
func DB_mkMeta(parentID uint64, name string, uid uint32, gid uint32, mode uint32) (uint64, error) {
	var err error
	now := time.Now().Unix()
	var newInode uint64
	fmt.Println("make folder")

	err = db.QueryRow(`
		INSERT INTO meta (
    		parent_id,
			name,
			uid,
			gid,
			mode,
			size, 
    		atime,
			mtime,
			ctime, 
    		is_dirty
		) VALUES (?, ?, ?, ?, ?, 0, ?, ?, ?, 1)
		RETURNING id;`,
		parentID,
		name,
		uid,
		gid,
		mode,
		now,
		now,
		now,
	).Scan(&newInode)
	if err != nil {
		fmt.Println("mkMeta faild")
		fmt.Println(err)
		return 0, err
	}
	if mode&syscall.S_IFMT == syscall.S_IFDIR {
		err = DB_Recalculate_Nlink(parentID)
		if err != nil {
			fmt.Println("Recalculate Nlink faild")
			fmt.Println(err)
			return 0, err
		}
	}
	return newInode, nil
}
func DB_List_meta(parentID uint64) (entries []fuse.DirEntry) {
	rows, err := db.Query("SELECT name, mode, id FROM meta WHERE parent_id = ? AND is_deleted = 0", parentID)
	if err != nil {
		fmt.Println("databaser list faild for inod:", parentID)
		fmt.Println(err)
		return
	}
	defer rows.Close()

	var name string
	var mode uint32
	var inod uint64

	for rows.Next() {
		if err := rows.Scan(&name, &mode, &inod); err != nil {
			fmt.Println("faild list inod:", parentID)
			continue
		}
		entries = append(entries, fuse.DirEntry{
			Name: name,
			Mode: mode,
			Ino:  inod,
		})
	}
	return
}
func DB_Lookup_meta(parentID uint64, name string) (ID int, mode uint32, size uint64, uid uint32, gid uint32, mtime uint64, atime uint64, ctime uint64, mising bool) {
	err := db.QueryRow(`
        SELECT id, mode, size, atime, mtime, ctime, uid, gid 
        FROM meta 
        WHERE parent_id = ? AND name = ? AND is_deleted = 0`,
		parentID, name).Scan(&ID, &mode, &size, &atime, &mtime, &ctime, &uid, &gid)
	if err == sql.ErrNoRows {
		mising = true // Normal "Not Found"
		return
	}
	if err != nil {
		log.Printf("DB Lookup Error: %v", err)
		mising = true
	}
	return
}
func DB_Getattr(inod uint64) (name string, parentID uint64, mode uint32, size uint64, uid uint32, gid uint32, Nlink uint32, mtime uint64, atime uint64, ctime uint64, mising bool) {
	err := db.QueryRow(`
        SELECT name ,parent_id, mode, size, atime, mtime, ctime, uid, gid, nlink 
        FROM meta 
        WHERE id = ? AND is_deleted = 0`,
		inod).Scan(&name, &parentID, &mode, &size, &atime, &mtime, &ctime, &uid, &gid, &Nlink)
	if err == sql.ErrNoRows {
		mising = true // Normal "Not Found"
		return
	}
	if err != nil {
		log.Printf("DB Lookup Error: %v", err)
		mising = true
	}
	return
}
func DB_Setattr(inod uint64, mode uint32, size uint64, uid uint32, gid uint32, mtime uint64, atime uint64, ctime uint64) (err error) {
	_, err = db.Exec(`
		UPDATE meta SET
		is_deleted=0,
		uid=?,
		gid=?,
		mode=?,
		size=?, 
    	atime=?,
		mtime=?,
		ctime=?,
		is_dirty=1
		WHERE id = ?`,
		uid, gid, mode, size, atime, mtime, ctime, inod)
	return
}
func DB_rm_meta(parentID uint64, name string, is_dir bool) (mising bool) {
	tx, err := db.Begin()
	if err != nil {
		fmt.Println("Failed to start transaction:", err)
		return true
	}

	res, err := tx.Exec(`UPDATE meta SET is_deleted=1, is_dirty=1,name = 'remove/' || id WHERE parent_id = ? AND name = ? AND is_deleted = 0`, parentID, name)
	if err != nil {
		fmt.Println("Soft delete failed:", err)
		tx.Rollback()
		return true
	}

	rows, _ := res.RowsAffected()
	if rows == 0 {
		tx.Rollback()
		return true
	}

	now := uint64(time.Now().Unix())
	_, err = tx.Exec(`UPDATE meta SET mtime=?, ctime=?, is_dirty=1 WHERE id=?`, now, now, parentID)
	if err != nil {
		fmt.Println("Parent time update failed:", err)
		tx.Rollback()
		return true
	}

	tx.Commit()
	if is_dir {
		err = DB_Recalculate_Nlink(parentID)
		if err != nil {
			fmt.Println("Recalculate Nlink faild")
			fmt.Println(err)
		}
	}
	return false
}
func DB_rename_meta(oldParentID uint64, oldName string, newParentID uint64, newName string) syscall.Errno {
	tx, err := db.Begin()
	if err != nil {
		return syscall.EIO
	}

	// 1. THE ASSASSINATION CHECK: Does the target already exist?
	var targetID uint64
	err = tx.QueryRow(`
        SELECT id FROM meta 
        WHERE parent_id = ? AND name = ? AND is_deleted = 0`,
		newParentID, newName).Scan(&targetID)

	if err == nil {
		// The target exists! We must soft-delete it to make room.
		// (Note: If this is a folder, Linux usually checks if it's empty first,
		// but for files, it just nukes them).
		_, err = tx.Exec("UPDATE meta SET is_deleted=1, is_dirty=1 WHERE id=?", targetID)
		if err != nil {
			tx.Rollback()
			return syscall.EIO
		}
	}

	// 2. Move/Rename our actual file into the newly cleared spot
	res, err := tx.Exec(`
        UPDATE meta 
        SET name = ?, parent_id = ?, is_dirty = 1 
        WHERE parent_id = ? AND name = ? AND is_deleted = 0`,
		newName, newParentID, oldParentID, oldName)

	if err != nil {
		tx.Rollback()
		return syscall.EIO
	}

	// Did our source file actually exist?
	rows, _ := res.RowsAffected()
	if rows == 0 {
		tx.Rollback()
		return syscall.ENOENT
	}

	// 3. Update timestamps for the folders
	now := uint64(time.Now().Unix())
	tx.Exec("UPDATE meta SET mtime=?, ctime=?, is_dirty=1 WHERE id=?", now, now, oldParentID)
	if oldParentID != newParentID {
		tx.Exec("UPDATE meta SET mtime=?, ctime=?, is_dirty=1 WHERE id=?", now, now, newParentID)
	}

	tx.Commit()

	err = DB_Recalculate_Nlink(oldParentID)
	if err != nil {
		fmt.Println("Recalculate Nlink faild")
		fmt.Println(err)
	}
	err = DB_Recalculate_Nlink(newParentID)
	if err != nil {
		fmt.Println("Recalculate Nlink faild")
		fmt.Println(err)
	}

	return fs.OK
}

func DB_read_block() {

}
func DB_write_block() {

}

func DB_Recalculate_Nlink(parentID uint64) error {
	query := `
		UPDATE meta 
		SET nlink = 2 + (
			SELECT COUNT(*) 
			FROM meta c 
			WHERE c.parent_id = ? AND c.is_deleted = 0 AND (c.mode & ?) = ?
		)
		WHERE id = ?`

	_, err := db.Exec(query, parentID, syscall.S_IFMT, syscall.S_IFDIR, parentID)
	return err
}

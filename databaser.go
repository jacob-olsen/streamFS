package main

import (
	"database/sql"
	"fmt"
	"log"
	"time"

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
    
		mtime INTEGER NOT NULL DEFAULT (unixepoch()), 
    	atime INTEGER NOT NULL DEFAULT (unixepoch()),
    	ctime INTEGER NOT NULL DEFAULT (unixepoch()),
    
    	is_dirty INTEGER DEFAULT 1,
    
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
}
func DB_mkDir(parentID uint64, name string, uid uint32, gid uint32, mode uint32) (uint64, error) {
	now := time.Now().Unix()
	var newInode uint64
	fmt.Println("make folder")

	err := db.QueryRow(`
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
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		RETURNING id;`,
		parentID,
		name,
		uid,
		gid,
		mode,
		0,
		now,
		now,
		now,
		1,
	).Scan(&newInode)
	if err != nil {
		fmt.Println("mkdir faild")
		fmt.Println(err)
		return 0, err
	}
	return newInode, nil
}
func DB_List_meta(parentID uint64) (entries []fuse.DirEntry) {
	rows, err := db.Query("SELECT name, mode, id FROM meta WHERE parent_id = ?", parentID)
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
        WHERE parent_id = ? AND name = ?`,
		parentID, name).Scan(&ID, &mode, &size, &atime, &mtime, &ctime, &uid, &gid)
	if err != nil {
		log.Printf("DB Lookup Error: %v", err)
		mising = true
	}
	return
}

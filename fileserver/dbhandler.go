package main

import (
	"bytes"
	"encoding/gob"
	"errors"
	"sort"
	"sync"
	"time"

	"../blogpost"
	"github.com/boltdb/bolt"
)

const (
	blogDbId = `blogposts`
)

var (
	errNotOpen  = errors.New("DB not open")
	errNotFound = errors.New("Post not found")
	errNoPosts  = errors.New("no posts")
	dbId        = []byte(blogDbId)
)

type boltDB struct {
	mtx            *sync.Mutex
	db             *bolt.DB
	cache          map[string]*blogpost.BlogPost
	postListCached []PostTS
}

type PostTS struct {
	Name string
	Date time.Time
}

func NewBlogDB(dbFile string) (*boltDB, error) {
	bdb, err := bolt.Open(dbFile, 0600, &bolt.Options{Timeout: 50 * time.Millisecond})
	if err != nil {
		return nil, err
	}
	if err := bdb.Update(func(tx *bolt.Tx) error {
		if _, lerr := tx.CreateBucketIfNotExists(dbId); lerr != nil {
			return lerr
		}
		return nil
	}); err != nil {
		bdb.Close()
		return nil, err
	}
	db := &boltDB{
		mtx:   &sync.Mutex{},
		db:    bdb,
		cache: make(map[string]*blogpost.BlogPost, 1),
	}
	if err := db.nlInitCache(); err != nil {
		db.Close()
		return nil, err
	}
	return db, nil
}

func (db *boltDB) Close() error {
	db.mtx.Lock()
	defer db.mtx.Unlock()
	if db.db == nil {
		return errNotOpen
	}
	if err := db.db.Close(); err != nil {
		return err
	}
	db.db = nil
	db.cache = nil
	return nil
}

func (db *boltDB) nlInitCache() error {
	if err := db.db.View(func(tx *bolt.Tx) error {
		b := tx.Bucket(dbId)
		c := b.Cursor()
		for name, bpbuff := c.First(); name != nil; name, bpbuff = c.Next() {
			var bp blogpost.BlogPost
			bb := bytes.NewBuffer(bpbuff)
			gdec := gob.NewDecoder(bb)
			if err := gdec.Decode(&bp); err != nil {
				return err
			}
			db.cache[string(name)] = &bp
		}
		return nil
	}); err != nil {
		return err
	}
	return nil
}

func (db *boltDB) Add(name string, bp *blogpost.BlogPost) error {
	db.mtx.Lock()
	defer db.mtx.Unlock()
	if db.db == nil {
		return errNotOpen
	}
	//add into the bolt DB
	if err := db.dbAdd(name, bp); err != nil {
		return err
	}
	//add into the cache
	cbp, ok := db.cache[name]
	if ok {
		*cbp = *bp
	} else {
		db.cache[name] = bp
	}
	return db.invalidatePostListCache()
}

func (db *boltDB) Get(name string) (*blogpost.BlogPost, error) {
	db.mtx.Lock()
	defer db.mtx.Unlock()
	if db.db == nil {
		return nil, errNotOpen
	}
	bp, ok := db.cache[name]
	if ok {
		return bp, nil
	}
	//not in the cache, try to get it from the bolt DB
	bp, err := db.dbGet(name)
	if err != nil {
		return nil, err
	}
	if bp == nil {
		return nil, errNotFound
	}
	return bp, nil
}

func (db *boltDB) Delete(name string) error {
	db.mtx.Lock()
	defer db.mtx.Unlock()
	if db.db == nil {
		return errNotOpen
	}
	delete(db.cache, name)
	if err := db.dbDelete(name); err != nil {
		return err
	}
	return db.invalidatePostListCache()
}

func (db *boltDB) dbAdd(name string, bp *blogpost.BlogPost) error {
	return db.db.Update(func(tx *bolt.Tx) error {
		bb := bytes.NewBuffer(nil)
		genc := gob.NewEncoder(bb)
		if err := genc.Encode(bp); err != nil {
			return err
		}
		bkt := tx.Bucket(dbId)
		return bkt.Put([]byte(name), bb.Bytes())
	})
}

func (db *boltDB) dbGet(name string) (*blogpost.BlogPost, error) {
	var bp blogpost.BlogPost
	//check if there is already something in the DB for today
	if err := db.db.View(func(tx *bolt.Tx) error {
		bkt := tx.Bucket(dbId)
		currBB := bkt.Get([]byte(name))
		if currBB == nil {
			return errNotFound
		}
		bb := bytes.NewBuffer(currBB)
		gdec := gob.NewDecoder(bb)
		if err := gdec.Decode(&bp); err != nil {
			return err
		}
		return nil
	}); err != nil {
		return nil, err
	}
	return &bp, nil
}

func (db *boltDB) dbDelete(name string) error {
	if err := db.db.Update(func(tx *bolt.Tx) error {
		return tx.Bucket(dbId).Delete([]byte(name))
	}); err != nil {
		return err
	}
	return nil
}

func (db *boltDB) OrderedNameList() ([]PostTS, error) {
	var pl []PostTS
	db.mtx.Lock()
	defer db.mtx.Unlock()
	if db.db == nil {
		return pl, errNotOpen
	}
	copy(pl, db.postListCached)
	return pl, nil
}

func (db *boltDB) LatestPost() (blogpost.BlogPost, error) {
	var lp blogpost.BlogPost
	db.mtx.Lock()
	defer db.mtx.Unlock()
	if db.db == nil {
		return lp, errNotOpen
	}
	if len(db.postListCached) <= 0 {
		return lp, errNoPosts
	}
	bp, ok := db.cache[db.postListCached[0].Name]
	if !ok {
		return lp, errors.New("Cache invalid")
	}
	return *bp, nil
}

func (db *boltDB) invalidatePostListCache() error {
	db.postListCached = nil
	for k, v := range db.cache {
		db.postListCached = append(db.postListCached, PostTS{
			Name: k,
			Date: v.Date,
		})
	}
	sort.Sort(postList(db.postListCached))
	return nil
}

type postList []PostTS

func (pl postList) Len() int           { return len(pl) }
func (pl postList) Swap(i, j int)      { pl[i], pl[j] = pl[j], pl[i] }
func (pl postList) Less(i, j int) bool { return pl[i].Date.Before(pl[j].Date) }

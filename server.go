package main

import (
	"database/sql"
	"fmt"
	"log"
	"net/http"
	"strconv"

	"github.com/gorilla/mux"
	lru "github.com/hashicorp/golang-lru"
	_ "github.com/mattn/go-sqlite3"
	"github.com/paulmach/orb/maptile"
	"github.com/psanford/sqlite3vfs"
	"simplemap.co/tileserver/sqlite3vfsgcs"
)

var bucket = "simplemapco-assets"
var prefix = "tilesets"
var dbCache = make(map[string]*sql.DB)

func QuadkeyString(tile maptile.Tile) string {
	qkint := tile.Quadkey()
	qk := strconv.FormatInt(int64(qkint), 4)
	// for zero padding
	zeros := "000000000000000000000000000000"
	return zeros[:((int(tile.Z)+1)-len(qk))/2] + qk
}

func tileHandler(w http.ResponseWriter, r *http.Request) {
	params := mux.Vars(r)
	tileset := params["tileset"]

	z, err := strconv.Atoi(params["z"])
	if err != nil {
		log.Fatalf("convert err: %s", err)
	}
	x, err := strconv.Atoi(params["x"])
	if err != nil {
		log.Fatalf("convert err: %s", err)
	}
	y, err := strconv.Atoi(params["y"])
	if err != nil {
		log.Fatalf("convert err: %s", err)
	}
	tile := maptile.New(uint32(x), uint32(y), maptile.Zoom(z))

	qk := QuadkeyString(tile)
	fmt.Printf("Quadkey: %s\n", qk)
	tiledata, err := queryTiles(tileset, qk)
	if err != nil {
		fmt.Printf("query err: %s\n", err)
		w.WriteHeader(404)
		w.Write([]byte("Not found.\n"))
		return
		// log.Fatalf("query err: %s", err)
	}
	w.Write(tiledata)
}

func getDB(tileset string) *sql.DB {
	if dbCache[tileset] != nil {
		return dbCache[tileset]
	}
	// todo check if this file exists, so we short circuit / avoid query errors
	dbpath := fmt.Sprintf("gs://%s/%s/%s.smtiles?vfs=gcsvfs&mode=ro&", bucket, prefix, tileset)
	db, err := sql.Open("sqlite3", dbpath)
	if err != nil {
		log.Fatalf("open db err: %s", err)
	}
	dbCache[tileset] = db
	return db
}

func queryTiles(tileset string, quadkey string) ([]byte, error) {
	db := getDB(tileset)
	stmt, err := db.Prepare("SELECT tile_data FROM tiles WHERE quadkey = ?")
	if err != nil {
		return nil, err
	}
	defer stmt.Close()
	var tiledata []byte
	err = stmt.QueryRow(quadkey).Scan(&tiledata)
	if err != nil {
		return nil, err
	}
	return tiledata, nil
}

func main() {

	cache, _ := lru.New2Q(128)
	vfs := sqlite3vfsgcs.GcsVFS{
		CacheHandler: cache,
	}

	err := sqlite3vfs.RegisterVFS("gcsvfs", &vfs)
	if err != nil {
		log.Fatalf("register vfs err: %s", err)
	}

	r := mux.NewRouter()
	r.HandleFunc("/tile/{tileset}/{z}/{x}/{y}.{format}", tileHandler).Methods("GET")
	http.Handle("/", r)

	fmt.Printf("Starting server at port 8080\n")
	if err := http.ListenAndServe(":8080", nil); err != nil {
		log.Fatal(err)
	}
}

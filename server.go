package main

import (
	"context"
	"database/sql"
	_ "embed"
	"encoding/json"
	"fmt"
	"log"
	"math"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/gorilla/mux"
	lru "github.com/hashicorp/golang-lru"
	_ "github.com/mattn/go-sqlite3"
	"github.com/psanford/sqlite3vfs"
	"github.com/rs/cors"
	"simplemap.co/tileserver/sqlite3vfsstorage"
)

type DBConnection struct {
	Db               *sql.DB
	ConnectionString string
	CreatedAt        time.Time
}

type ServerState struct {
	DbConnections *lru.Cache
	Scheme        string
	Bucket        string
	Prefix        string
	TileUrl       string
	TileJSONUrl   string
}

func errorResponse(w http.ResponseWriter, statusCode int, message string) {
	w.WriteHeader(statusCode)
	w.Write([]byte(message))
}

func (s ServerState) tileHandler(w http.ResponseWriter, r *http.Request) {
	ctx := context.Background()

	params := mux.Vars(r)
	tileset := params["tileset"]

	z, err := strconv.Atoi(params["z"])
	if err != nil {
		errorResponse(w, 400, "Bad request")
		return
	}
	x, err := strconv.Atoi(params["x"])
	if err != nil {
		errorResponse(w, 400, "Bad request")
		return
	}
	y, err := strconv.Atoi(params["y"])
	if err != nil {
		errorResponse(w, 400, "Bad request")
		return
	}

	db, err := s.getDB(ctx, tileset)
	if err != nil {
		errorResponse(w, 404, "Tileset Not Found")
		return
	}
	tiledata, err := s.queryTiles(db, tileset, x, y, z)
	if err != nil {
		errorResponse(w, 404, "Tile Not Found")
		return
	}
	w.Header().Set("Content-Encoding", "gzip")
	w.Header().Set("Content-Type", "application/octet-stream")
	w.Write(tiledata)
}

func (s ServerState) tileJSONHandler(w http.ResponseWriter, r *http.Request) {
	params := mux.Vars(r)
	tileset := params["tileset"]
	tileUrl := fmt.Sprintf(s.TileUrl, tileset)

	ctx := context.Background()
	db, err := s.getDB(ctx, tileset)
	if err != nil {
		errorResponse(w, 404, "Tileset Not Found")
		return
	}
	metadata, err := s.queryMetadata(db, tileset)
	if err != nil {
		errorResponse(w, 404, fmt.Sprintf("Tileset Not Found: %v", err))
		return
	}
	tilejson := FromMBTiles(tileUrl, metadata)

	jsonResp, err := json.Marshal(tilejson)
	if err != nil {
		errorResponse(w, 500, "Internal Server Error")
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.Write(jsonResp)

}

//go:embed map.html
var mapHTML string

func (s ServerState) mapHTMLHandler(w http.ResponseWriter, r *http.Request) {
	params := mux.Vars(r)
	tileset := params["tileset"]
	tileJSONUrl := fmt.Sprintf(s.TileJSONUrl, tileset)

	ctx := context.Background()
	_, err := s.getDB(ctx, tileset)
	if err != nil {
		errorResponse(w, 404, "Tileset Not Found")
		return
	}

	mapHTML = strings.ReplaceAll(mapHTML, "{{tileJSONUrl}}", tileJSONUrl)

	w.Header().Set("Content-Type", "text/html")
	w.Write([]byte(mapHTML))

}

func checkPathExists(ctx context.Context, storagePath string) (bool, error) {

	backend, err := sqlite3vfsstorage.GetBackend(storagePath)
	if err != nil {
		return false, fmt.Errorf("getBackend: %v", err)
	}
	_, err = backend.FileSize(storagePath)
	if err != nil {
		return false, fmt.Errorf("error getting file metadata: %v", err)
	}
	return true, nil
}
func (s ServerState) getDB(ctx context.Context, tileset string) (*sql.DB, error) {
	dbConn, ok := s.DbConnections.Get(tileset)
	if ok {
		return dbConn.(*sql.DB), nil
	}
	storagePath, err := url.JoinPath(fmt.Sprintf("%s://%s", s.Scheme, s.Bucket), s.Prefix, tileset+".mbtiles")
	if err != nil {
		return nil, err
	}
	ok, err = checkPathExists(ctx, storagePath)
	if err != nil {
		return nil, err
	}
	if !ok {
		return nil, fmt.Errorf("tileset not found")
	}
	dbpath := fmt.Sprintf("%s?vfs=storagevfs&mode=ro&", storagePath)
	db, err := sql.Open("sqlite3", dbpath)
	if err != nil {
		log.Fatalf("open db err: %s", err)
	}
	s.DbConnections.Add(tileset, db)
	return db, nil
}

func (s ServerState) queryMetadata(db *sql.DB, tileset string) (map[string]string, error) {
	stmt, err := db.Prepare("SELECT name, value FROM metadata")
	if err != nil {
		return nil, err
	}
	defer stmt.Close()
	var metadata map[string]string = make(map[string]string)

	rows, err := stmt.Query()
	if err != nil {
		return nil, err
	}
	for rows.Next() {
		var key string
		var value string
		err = rows.Scan(&key, &value)
		if err != nil {
			return nil, err
		}
		metadata[key] = value
	}
	return metadata, nil
}

func (s ServerState) queryTiles(db *sql.DB, tileset string, x, y, z int) ([]byte, error) {
	stmt, err := db.Prepare("SELECT tile_data FROM tiles WHERE zoom_level = ? AND tile_column = ? AND tile_row = ?")
	if err != nil {
		return nil, err
	}
	defer stmt.Close()
	var tiledata []byte

	// mbtiles store tiles in TMS format, so we need to convert the XYZ input to TMS (flip y)
	y = int(math.Pow(2, float64(z))) - y - 1

	err = stmt.QueryRow(z, x, y).Scan(&tiledata)
	if err != nil {
		return nil, err
	}
	return tiledata, nil
}

func main() {
	dbConnections, _ := lru.New(20)
	cache, _ := lru.New2Q(128)
	vfs := sqlite3vfsstorage.StorageVFS{
		CacheHandler: cache,
	}

	mbtilesPath := os.Getenv("MBTILES_PATH")
	if mbtilesPath == "" {
		log.Fatal("MBTILES_PATH env var not set")
	}

	parsedURI, err := url.Parse(mbtilesPath)
	if err != nil {
		log.Fatalf("error parsing MBTILES_PATH: %v", err)
	}

	tsURL := os.Getenv("TILESERVER_URL")
	if tsURL == "" {
		log.Fatal("TILESERVER_URL env var not set")
	}

	tileUrl, err := url.JoinPath(tsURL, "/tile/")
	tileJSONUrl := tileUrl + "%s.json"
	tileUrl += "%s/{z}/{x}/{y}.mvt"
	if err != nil {
		log.Fatalf("error contructing TILESERVER_URL: %v", err)
	}

	server := ServerState{
		DbConnections: dbConnections,
		Scheme:        parsedURI.Scheme,
		Bucket:        parsedURI.Host,
		Prefix:        parsedURI.Path,
		TileUrl:       tileUrl,
		TileJSONUrl:   tileJSONUrl,
	}

	err = sqlite3vfs.RegisterVFS("storagevfs", &vfs)
	if err != nil {
		log.Fatalf("register vfs err: %s", err)
	}

	r := mux.NewRouter()
	r.HandleFunc("/tile/{tileset}.json", server.tileJSONHandler).Methods("GET")
	r.HandleFunc("/tile/{tileset}/{z}/{x}/{y}.{format}", server.tileHandler).Methods("GET")
	r.HandleFunc("/map/{tileset}.html", server.mapHTMLHandler).Methods("GET")
	r.HandleFunc("/map/{tileset}", server.mapHTMLHandler).Methods("GET")
	handler := cors.New(cors.Options{
		AllowedMethods: []string{"GET"},
		// Debug:          true,
	}).Handler(r)

	http.Handle("/", handler)

	port := "8080"
	if os.Getenv("PORT") != "" {
		port = os.Getenv("PORT")
	}

	fmt.Printf("Starting server at port http://0.0.0.0:%s \n", port)
	if err := http.ListenAndServe(":"+port, nil); err != nil {
		log.Fatal(err)
	}
}

package main

import (
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"math/rand"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"time"
)

// 1MB
const MAX_MEMORY = 1 * 1024 * 1024

var upload_tmp_dir = ""     // Temporary folder containing targetDir
var targetDir = ""          // Folder where uploaded files stores
var cleanupTargetDir = true // Remove old files
var maxFileAge = 5 * 3600   // Temp file age in seconds
var randomFileNamePrefix = "file_"
var randomString = CreateRandomString("abcdefghijklmnoprstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789")

type RandomString struct {
	pool string
	rg   *rand.Rand
	used map[string]int
}

func CreateRandomString(pool string) *RandomString {
	return &RandomString{
		pool,
		rand.New(rand.NewSource(time.Now().UnixNano())),
		make(map[string]int),
	}
}

func (rs *RandomString) Generate(length int) (r string) {
	if length < 1 {
		return
	}
	b := make([]byte, length)
	for retries := 0; ; retries++ {
		for i, _ := range b {
			b[i] = rs.pool[rs.rg.Intn(len(rs.pool))]
		}
		r = string(b)
		_, used := rs.used[r]
		if !used {
			break
		}
		if retries == 3 {
			return ""
		}
	}
	rs.used[r] = 0
	return
}

func upload(w http.ResponseWriter, r *http.Request) {

	if err := r.ParseMultipartForm(MAX_MEMORY); err != nil {
		log.Println(err)
		http.Error(w, err.Error(), http.StatusForbidden)
	}

	for key, value := range r.MultipartForm.Value {
		fmt.Fprintf(w, "%s:%s ", key, value)
		log.Printf("%s:%s", key, value)
	}

	// Make sure file is not cached (as it happens for example on iOS devices)
	w.Header().Set("Expires", "Mon, 26 Jul 1997 05:00:00 GMT")
	w.Header().Set("Last-Modified", time.Now().UTC().Format(http.TimeFormat))
	w.Header().Set("Cache-Control", "no-cache, no-store, must-revalidate")
	w.Header().Add("Cache-Control", "post-check=0, pre-check=0")
	w.Header().Set("Pragma", "no-cache")

	// Chunking might be enabled
	var chunkStr = r.FormValue("chunk")
	var chunksStr = r.FormValue("chunks")
	var chunk = 0
	var chunks = 1

	if chunkStr != "" {
		chunk, _ = strconv.Atoi(chunkStr)
	}
	if chunksStr != "" {
		chunks, _ = strconv.Atoi(chunksStr)
	}

	// Create target dir
	if _, err := os.Stat(targetDir); os.IsNotExist(err) {
		fmt.Printf("no such file or directory: %s, about to create it...", targetDir)
		os.MkdirAll(targetDir, 0750)
	}

	for _, fileHeaders := range r.MultipartForm.File {
		for _, fileHeader := range fileHeaders {
			file, _ := fileHeader.Open()
			// Get a file name
			var filename = ""
			if r.FormValue("name") != "" {
				filename = r.FormValue("name")
			} else if fileHeader.Filename != "" {
				filename = fileHeader.Filename
			} else {
				filename = randomFileNamePrefix + randomString.Generate(10)
			}
			path := filepath.Join(targetDir, filename)
			log.Println(path)
			buf, _ := ioutil.ReadAll(file)

			tempPath := path + ".part"

			var f *os.File
			f, err := os.OpenFile(tempPath, os.O_APPEND|os.O_WRONLY, 0600)
			if err != nil {
				if os.IsNotExist(err) {
					f, err = os.Create(tempPath)
					if err != nil {
						panic(err)
					}
				} else {
					panic(err)
				}
			}
			if _, err = f.Write(buf); err != nil {
				panic(err)
			}
			f.Close()

			fileInfo, _ := os.Stat(tempPath)
			if chunks == 0 || chunk == chunks-1 {
				os.Rename(path+".part", path)
			}
		}
	}
}

func main() {
	var upload_tmp_dir_Str = flag.String("upload_tmp_dir", ".", "upload temp folder")
	upload_tmp_dir, err := filepath.Abs(filepath.Dir(*upload_tmp_dir_Str))
	if err != nil {
		log.Fatal(err)
	}
	targetDir = filepath.Join(upload_tmp_dir, "plupload")
	log.Printf("upload_tmp_dir: %s, targetDir: %s", upload_tmp_dir, targetDir)

	http.HandleFunc("/upload", upload)
	http.Handle("/", http.FileServer(http.Dir("../")))
	log.Fatal(http.ListenAndServe(":8080", nil))
}

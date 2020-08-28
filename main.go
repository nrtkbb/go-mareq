package main

import (
	"bufio"
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"sync"

	ma "github.com/nrtkbb/go-mayaascii"
)

const (
	MayaAsciiSuffix string = ".ma"
)

func isMayaAscii(path string) bool {
	return strings.HasSuffix(path, MayaAsciiSuffix)
}

type Result struct {
	requires map[string][]string
	mutex    *sync.Mutex
}

func (r *Result) AddRequire(pluginName, filePath string) {
	r.mutex.Lock()
	defer r.mutex.Unlock()

	v, ok := r.requires[pluginName]
	if ok {
		v = append(v, filePath)
	} else {
		r.requires[pluginName] = []string{filePath}
	}
}

func (r *Result) Print() {
	r.mutex.Lock()
	defer r.mutex.Unlock()

	var keys []string
	for key, _ := range r.requires {
		keys = append(keys, key)
	}
	sort.Sort(sort.StringSlice(keys))

	for _, key := range keys {
		fmt.Printf("%s, %d,\n", key, len(r.requires[key]))
		for _, value := range r.requires[key] {
			fmt.Printf(",,%s\n", value)
		}
	}
}

func main() {
	flag.Parse()
	filePaths := flag.Args()
	if 0 == len(filePaths) {
		log.Fatal("Nothing arguments. exit.")
	}
	for _, fp := range filePaths {
		if _, err := os.Stat(fp); err != nil {
			log.Fatal(err)
		}
	}

	cpuNum := runtime.NumCPU()
	runtime.GOMAXPROCS(cpuNum)
	filePathChan := make(chan string, cpuNum)

	var wg sync.WaitGroup
	r := &Result{
		requires: map[string][]string{},
		mutex:    &sync.Mutex{},
	}

	// Consumer (Worker)
	for i := 0; i < cpuNum-1; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for path := range filePathChan {
				if isMayaAscii(path) {
					err := CollectRequire(path, r)
					if err != nil {
						panic(err)
					}
				} else {
					log.Printf("skip %s\n", path)
				}

			}
		}()
	}

	for _, filePath := range filePaths {
		err := walk(filePath, filePathChan)
		if err != nil {
			panic(err)
		}
	}

	close(filePathChan)
	wg.Wait()

	r.Print()

	fmt.Printf("\nPress 'Enter' to continue...\n\r")
	_, _ = fmt.Scanln()
}

func walk(dirPath string, filePathChan chan<- string) error {
	err := filepath.Walk(dirPath,
		func(path string, info os.FileInfo, err error) error {
			if err != nil {
				return err
			}
			if info.IsDir() {
				return nil
			}
			if isMayaAscii(path) {
				filePathChan <- path
			} else {
				log.Printf("skip %s\n", path)
			}
			return nil
		})
	return err
}

func CollectRequire(filePath string, r *Result) (err error) {
	var fp *os.File
	fp, err = os.Open(filePath)
	if err != nil {
		return
	}
	defer func() {
		err = fp.Close()
	}()

	reader := bufio.NewReader(fp)

	var mo *ma.Object
	log.Printf("check %s\n", filePath)
	mo, err = ma.Unmarshal(reader)
	if err != nil {
		return
	}

	for _, require := range mo.Requires {
		r.AddRequire(require.Name, filePath)
	}

	return
}

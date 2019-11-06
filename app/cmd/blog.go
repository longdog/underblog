package cmd

import (
	"context"
	"errors"
	"fmt"
	"html/template"
	"io"
	"strings"

	//	"io/ioutil"
	"log"
	"os"
	"path"
	"path/filepath"
	"sync"

	"github.com/freetonik/underblog/app/internal"
)

const DefaultMarkdownPath = "./markdown/"

// Create and initialize Blog
func NewBlog(opts internal.Opts) *Blog {
	b := new(Blog)

	b.opts = opts

	b.mux = &sync.Mutex{}
	b.wg = &sync.WaitGroup{}
	b.categories = make(map[string][]Post)
	b.files = make(chan Pair)
	b.catCh = make(chan string)
	return b
}

type Pair struct {
	key string
	val []string
}

// Blog options and blog creating methods
type Blog struct {
	opts internal.Opts

	files      chan Pair
	catCh      chan string
	posts      []Post
	categories map[string][]Post
	indexPage  io.Writer

	mux *sync.Mutex
	wg  *sync.WaitGroup
}

// Render md-files->HTML, generate root index.html
func (b *Blog) Render() error {
	if err := b.verifyMarkdownPresent(); err != nil {
		log.Fatal(errors.New(fmt.Sprintf("Markdown directory is not found: %v", err)))
	}

	b.indexPage = b.getIndexPage(b.opts.Path)
	b.createPosts()
	err := b.renderMd()
	b.copyCssToPublicDir()

	return err
}

func (b *Blog) addPost(post Post) {
	b.mux.Lock()
	b.posts = append(b.posts, post)
	for _, c := range post.Cats {
		b.categories[c] = append(b.categories[c], post)
	}
	b.mux.Unlock()
}

func (b *Blog) getIndexPage(currentPath string) io.Writer {
	rootPath := "."

	if currentPath != "" {
		rootPath = currentPath
	}
	p := filepath.Join(rootPath, "public")
	err := os.MkdirAll(p, os.ModePerm)
	if err != nil {
		log.Fatal(errors.New(fmt.Sprintf("Can't create public dir: %v", err)))
	}

	f, err := os.Create("public/index.html")

	if err != nil {
		log.Fatal(errors.New(fmt.Sprintf("Can't create public/index.html: %v", err)))
	}

	return f
}

func (b *Blog) startWorker(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		case f, ok := <-b.files:
			if !ok {
				// todo: catch it?
				b.wg.Done()
				return
			}
			b.addPost(NewPost(f.key, f.val))
			b.wg.Done()
		case c, ok := <-b.catCh:
			if !ok {
				b.wg.Done()
				return
			}
			b.renderCat(c)
			b.wg.Done()
		}

	}
}

func (b *Blog) getMdFiles() (map[string][]string, map[string][]string) {
	//files, err := ioutil.ReadDir(DefaultMarkdownPath)
	files := make(map[string][]string)
	cats := make(map[string][]string)

	err := filepath.Walk(DefaultMarkdownPath,
		func(path string, info os.FileInfo, err error) error {
			if err != nil {
				return err
			}
			//files = append(files, info)
			//fmt.Println(path, info.Size())
			if isFileValid(info) {
				if cat := getCat(path); cat != "" {
					files[info.Name()] = append(files[info.Name()], cat)
					cats[cat] = append(cats[cat], info.Name())
				} else {
					files[info.Name()] = make([]string, 0)
				}
			}
			return nil
		})

	if err != nil {
		fmt.Println("Can't get directory of markdown files")
		log.Fatal(err)
	}
	return files, cats
}

func (b *Blog) createPosts() {
	ctx := context.Background()

	filesChan := make(chan os.FileInfo)
	files, cats := b.getMdFiles()

	wLimit := internal.GetWorkersLimit(len(files))
	b.wg.Add(len(files))

	for i := 0; i < wLimit; i++ {
		go b.startWorker(ctx)
	}

	for key, val := range files {
		b.files <- Pair{key, val}
	}

	b.wg.Wait()

	b.wg.Add(len(cats))
	for key, _ := range cats {
		b.catCh <- key
	}

	close(filesChan)
}

func (b *Blog) copyCssToPublicDir() {
	from, err := os.Open("./css/styles.css")
	if err != nil {
		log.Fatal(err)
	}

	newPath := filepath.Join("public", "css")
	_ = os.MkdirAll(newPath, os.ModePerm)

	to, err := os.OpenFile("./public/css/styles.css", os.O_RDWR|os.O_CREATE, 0666)
	if err != nil {
		log.Fatal(err)
	}

	_, err = io.Copy(to, from)
	if err != nil {
		log.Fatal(err)
	}

	_ = from.Close()
	_ = to.Close()
}

func (b *Blog) renderMd() error {
	t, _ := template.ParseFiles("index.html")
	b.wg.Wait() // wait until b.posts is populated
	err := t.Execute(b.indexPage, b.posts)
	if err != nil {
		log.Fatalf("can't execute template: %v", err)
	}
	// todo: should i close file interface?
	return nil
}

func (b *Blog) renderCat(cat string) error {

	// Create folder for HTML
	newPath := filepath.Join("public/cats", cat)
	_ = os.MkdirAll(newPath, os.ModePerm)

	// Create HTML file
	f, err := os.Create("public/cats/" + cat + "/index.html")
	if err != nil {
		log.Fatal(err)
	}

	// Generate final HTML file from template
	t, _ := template.ParseFiles("index.html")
	err = t.Execute(f, b.categories[cat])
	if err != nil {
		log.Fatalf("can't execute template: %v", err)
	}
	_ = f.Close()
	return nil
}

func (b *Blog) verifyMarkdownPresent() error {
	if _, err := os.Stat(DefaultMarkdownPath); os.IsNotExist(err) {
		return err
	}
	return nil
}

func isFileValid(file os.FileInfo) bool {
	return path.Ext(file.Name()) == ".md" || path.Ext(file.Name()) == ".markdown"
}

func getCat(path string) string {
	s := strings.Split(path, string(os.PathSeparator))
	if len(s) > 2 {
		return s[1]
	}
	return ""
}

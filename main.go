package main

import (
	"bytes"
	"crypto/tls"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/PuerkitoBio/goquery"
	"github.com/corpix/uarand"
)

// 保存所有已知的路径
var path = []string{"/"}

// 用于排除已知的路径
var m = make(map[string]struct{})

// 遍历path到的index
var i int

// 保护全局变量
var l sync.Mutex

// 用于等待一批获取网页
var wg sync.WaitGroup

// 表示获取哪个路径的网页
var rootUrl string

func get() {
	defer wg.Done()
	l.Lock()
	old := i
	i++
	p := path[old]
	l.Unlock()

	// Request the HTML page.
	req, err := http.NewRequest("GET", rootUrl+p, nil)
	if err != nil {
		fmt.Println(err)
		return
	}
	// 使用随机 User-Agent
	req.Header.Add("User-Agent", uarand.GetRandom())
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		fmt.Println(err)
		return
	}
	if res.StatusCode != 200 {
		fmt.Printf("%s 可能是非公开页面 status code error: %d %s\n", p, res.StatusCode, res.Status)
		return
	}

	b, err := io.ReadAll(res.Body)
	if err != nil {
		fmt.Println(err)
		return
	}

	//保存获取的页面
	wp := p
	if wp[0] == '/' {
		wp = wp[1:]
	}
	if wp == "" {
		wp = "index"
	}
	if strings.HasSuffix(wp, "/") {
		wp += "index"
	}
	if strings.Contains(res.Header.Get("Content-Type"), "text/html") {
		wp += ".html"
	}

	//如果保存页面失败，继续，因为当前页面的a标签和link标签可能包含前往网站其他页面的路径
	err = os.MkdirAll(filepath.Dir("."+string(filepath.Separator)+filepath.Join("web", wp)), 0777)
	if err != nil {
		fmt.Println(p, "保存页面失败", err)
	}
	err = os.WriteFile("."+string(filepath.Separator)+filepath.Join("web", wp), b, 0777)
	if err != nil {
		fmt.Println(p, "保存页面失败", err)
	}

	doc, err := goquery.NewDocumentFromReader(bytes.NewBuffer(b))
	if err != nil {
		fmt.Println(err)
		return
	}

	// 获取所有a标签和link标签
	add := func(attr string) func(i int, s *goquery.Selection) {
		return func(i int, s *goquery.Selection) {
			src, esixt := s.Attr(attr)
			if !esixt {
				return
			}
			if src == "" {
				return
			}
			if src[0] != '/' { //如果不是相对路径
				return
			}
			l.Lock()
			if _, ok := m[src]; ok { //如果是已知的路径
				l.Unlock()
				return
			}
			m[src] = struct{}{}
			path = append(path, src)
			l.Unlock()
			fmt.Println(src)
		}
	}
	doc.Find("link").Each(add("href"))
	doc.Find("a").Each(add("href"))
	doc.Find("script").Each(add("src"))

}

func main() {
	var sslskip bool
	flag.BoolVar(&sslskip, "sslskip", false, "跳过验证ssl证书")
	flag.StringVar(&rootUrl, "u", "", "要获取的网站网址")
	flag.Parse()
	if rootUrl == "" {
		flag.Usage()
		return
	}
	if sslskip {
		http.DefaultClient.Transport = &http.Transport{
			TLSClientConfig: &tls.Config{
				InsecureSkipVerify: true,
			},
		}
	}
	m["/"] = struct{}{}
	err := os.Mkdir("."+string(filepath.Separator)+"web", 077)
	if err != nil {
		panic(err)
	}
	//如果还有未获取的网页
	for len(path) > i {
		//获取所有未获取的网页
		for range len(path) - i {
			wg.Add(1)
			go get()
		}
		wg.Wait()
	}
}

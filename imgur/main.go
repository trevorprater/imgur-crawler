package main

import (
	"log"
	"fmt"
	"encoding/json"
	"io/ioutil"
	"bytes"
	"time"
	"net/http"
	"math/rand"
	"strings"

	"golang.org/x/net/html"
)

const chars = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
const url = "http://0.0.0.0:5000/api/embed"
var r *rand.Rand
var numRequests int
var start time.Time

func SendURLs(urls []string) (*http.Response, error) {
	jsonBytes, err := json.Marshal(&urls)
	if err != nil {
		log.Println("could not marshal urls to json")
		return nil, err
	}
	req, err := http.NewRequest("POST", url, bytes.NewBuffer(jsonBytes))
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	body, _ := ioutil.ReadAll(resp.Body)
	fmt.Println(string(body))
	return resp, nil
	//respBody := make(struct{})
	//err = json.Unmarshal(&body, &respBody)
}

func worker(id int, jobs <-chan string) {
	urls := make([]string, 0)
    for url := range jobs {
		response, err := http.Get(url)
		if err != nil {
			log.Println(err)
		} else {
			if response.StatusCode != 404 {
				doc, err := html.Parse(response.Body)
				if err != nil {
					fmt.Println(err)
					break
				}
				var f func(*html.Node)
				f = func(n *html.Node) {
					if n.Type == html.ElementNode && n.Data == "img" {
						if strings.Contains(n.Attr[0].Val, ".png") || strings.Contains(n.Attr[0].Val, ".jpg") {
							urls = append(urls, "http:" + n.Attr[0].Val)
						}
					}
					for c := n.FirstChild; c != nil; c = c.NextSibling {
						f(c)
					}
				}
				f(doc)
			}
			response.Body.Close()
		}
		numRequests += 1
		if len(urls) >= 1 {
			_, err := SendURLs(urls)
			if err != nil {
				log.Println(err)
			}
			urls = make([]string, 0)
		}
	}

}

func init() {
	r = rand.New(rand.NewSource(time.Now().UnixNano()))
	numRequests = 0
	start = time.Now()
}

func print_requests_per_second() {
	for true {
		elapsed := time.Since(start)
		log.Printf("REQUESTS / SEC = %v", float64(numRequests)/elapsed.Seconds())
		time.Sleep(10 * time.Second)
	}
}


func generate_random_url(strlen int) string {
	result := make([]byte, strlen)
	for i := range result {
		result[i] = chars[r.Intn(len(chars))]
	}
	return "http://imgur.com/" + string(result)
}

func main() {
    jobs := make(chan string, 5)

    for w := 0; w < 50; w++ {
        go worker(w, jobs)
    }

	go print_requests_per_second()


    for j := 1; j <= 10000; j++ {
        jobs <- generate_random_url(5)
    }
    close(jobs)
}


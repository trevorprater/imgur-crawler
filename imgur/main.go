package main

import (
	"log"
	"fmt"
	"encoding/json"
	"strconv"
	"io/ioutil"
	"bytes"
	"time"
	"net/http"
	"math/rand"
	"strings"
	b64 "encoding/base64"

	"golang.org/x/net/html"
)

const chars = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
const url = "http://0.0.0.0:5000/api/embed"
var r *rand.Rand
var numRequests int
var start time.Time

type ImageRequest struct {
	URL string `json:"url"`
	B64Bytes string `json:"b64_bytes"`
}

type EmbeddingRequest struct {
	Images []*ImageRequest `json:"images"`
}


func download_image(url string) (*ImageRequest, error) {
	res, err := http.Get(url)
	if err != nil {
		log.Printf("http.Get -> %v", err)
	}

	data, err := ioutil.ReadAll(res.Body)
	if err != nil {
		log.Printf("ioutil.ReadAll -> %v", err)
	}
	res.Body.Close()
	sEnc := b64.StdEncoding.EncodeToString(data)
	imgRequest := &ImageRequest{URL: url, B64Bytes: sEnc}
	return imgRequest, err
}


func SendURLs(imgRequests []*ImageRequest) (*http.Response, error) {
	embeddingRequest := &EmbeddingRequest{ Images: imgRequests }
	jsonBytes, err := json.Marshal(embeddingRequest)
	if err != nil {
		log.Println("could not marshal urls to json")
		return nil, err
	}
	req, err := http.NewRequest("POST", url, bytes.NewBuffer(jsonBytes))
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{}
	resp, err := client.Do(req)
	if err == nil {
		defer resp.Body.Close()
	}
	if err != nil {
		log.Println(err)
		return nil, err
	}
	contentLength, _ := strconv.Atoi(resp.Header.Get("Content-Length"))
	if contentLength != 291 {
		body, _ := ioutil.ReadAll(resp.Body)
		fmt.Println(string(body))
		fmt.Println("\n")
		return resp, nil
	} else {
	}
	return nil, err
}

func worker(id int, jobs <-chan string) {
	imgRequests := make([]*ImageRequest, 0)
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
							imgRequest, err := download_image("http:" + n.Attr[0].Val)
							if err != nil {
								log.Printf("download_image -. %v", err)
							} else {
								imgRequests = append(imgRequests, imgRequest)
							}
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
		if len(imgRequests) >= 1 {
			_, err := SendURLs(imgRequests)
			if err != nil {
				log.Println(err)
			}
			imgRequests = make([]*ImageRequest, 0)
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
    jobs := make(chan string, 1)

    for w := 0; w < 200; w++ {
        go worker(w, jobs)
    }

	go print_requests_per_second()


    for j := 1; j <= 10000; j++ {
        jobs <- generate_random_url(5)
    }
    close(jobs)
}


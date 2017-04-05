package main

import (
	"log"
	"fmt"
	"encoding/json"
	"errors"
	"strconv"
	"io/ioutil"
	"time"
	"net/http"
	"math/rand"
	"strings"
	b64 "encoding/base64"

	"golang.org/x/net/html"
	"github.com/Shopify/sarama"
)

const chars = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
const url = "http://35.185.90.126:5000/api/embed"
var r *rand.Rand
var numRequests int
var numFacesProcessed int
var start time.Time
var producer sarama.AsyncProducer
var producerErr error

type ImageRequest struct {
	URL string `json:"url"`
	B64Bytes string `json:"b64_bytes"`
}

func download_image(url string) (*ImageRequest, error) {
	res, err := http.Get(url)
	if err != nil {
		log.Printf("http.Get -> %v", err)
		return nil, err
	}

	data, err := ioutil.ReadAll(res.Body)
	if err != nil {
		log.Printf("ioutil.ReadAll -> %v", err)
		return nil, err
	}
	res.Body.Close()
	sEnc := b64.StdEncoding.EncodeToString(data)
	imgRequest := &ImageRequest{URL: url, B64Bytes: sEnc}
	return imgRequest, err
}

func req_worker(imgRequestChan <- chan *ImageRequest) {
	for true {
		imgRequest := <-imgRequestChan
		jsonBytes, err := json.Marshal(imgRequest)
		strTime := strconv.Itoa(int(time.Now().Unix()))
		if err != nil {
			log.Println("could not marshal urls to json")
		}
		msg := &sarama.ProducerMessage{
			Topic: "facenet",
			Key: sarama.StringEncoder(strTime),
			Value: sarama.StringEncoder(string(jsonBytes)),
		}
		select {
		case producer.Input() <- msg:
			fmt.Println("Produce message")
		case err := <-producer.Errors():
			fmt.Println("Failed to produce mesage:", err)
//		case <-signals:
//			doneCh <- struct{}{}
		}

	}
}

func worker(id int, jobs <-chan string, imgRequestChan chan<- *ImageRequest) {
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
								imgRequestChan <- imgRequest
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
	}

}

func init() {
	r = rand.New(rand.NewSource(time.Now().UnixNano()))
	numRequests = 0
	numFacesProcessed = 0
	start = time.Now()
	config := sarama.NewConfig()
	//config.Net.SASL.Enable = true
	//config.Net.SASL.Password = "sqj5SeY3"
	//config.Net.SASL.User = "user"
	config.Producer.Retry.Max = 5
	config.Producer.RequiredAcks = sarama.WaitForAll
	brokers := []string{"104.196.19.209:9092"}
	producer, producerErr = sarama.NewAsyncProducer(brokers, config)
	if producerErr != nil {
		panic(errors.New("producer error"))
		panic(producerErr)
	}
}

func print_requests_per_second() {
	for true {
		elapsed := time.Since(start)
		log.Printf("FACES PROCESSED / SEC = %v", float64(numFacesProcessed)/elapsed.Seconds())
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
	results := make(chan *ImageRequest, 10)

	defer func() {
		if err := producer.Close(); err != nil {
			panic(err)
		}
	}()

    for w := 0; w < 100; w++ {
        go worker(w, jobs, results)
    }
	for ww := 0; ww < 5; ww++ {
		go req_worker(results)
	}

	go print_requests_per_second()


    for j := 1; j <= 100000000; j++ {
        jobs <- generate_random_url(5)
    }
    close(jobs)
}


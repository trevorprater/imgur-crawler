package main

import (
	"crypto/sha256"
	b64 "encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"log"
	"math/rand"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/Shopify/sarama"
	"golang.org/x/net/html"
	"gopkg.in/h2non/bimg.v1"
)

const chars = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
const url = "http://35.185.90.126:5000/api/embed"

var r *rand.Rand
var numRequests int
var start time.Time
var producer sarama.AsyncProducer
var producerErr error
var messagesPublished uint64

type ImageRequest struct {
	URL               string `json:"url"`
	ParentURL         string `json:"parent_url"`
	B64Bytes          string `json:"b64_bytes"`
	B64BytesThumbnail string `json:"b64_bytes_thumbnail"`
	SHA256Hash        string `json:"sha256"`
}

func download_image(url, parent_url string) (*ImageRequest, error) {
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

	newImage := bimg.NewImage(data)
	newImgSize, err := newImage.Size()
	if err != nil {
		log.Printf("Could not get size of image: %v", err)
		return nil, err
	}
	if newImgSize.Width < 100 || newImgSize.Height < 100 {
		return nil, err
	}
	if newImage.Type() == "png" {
		convertedBytes, err := newImage.Convert(bimg.JPEG)
		if err != nil {
			log.Printf("Could not convert image: %v\n", err)
			return nil, err
		}
		newImage = bimg.NewImage(convertedBytes)
	}
	thumbnailImg := bimg.NewImage(data)
	thumbnailBytes, err := thumbnailImg.Thumbnail(200)
	if err != nil {
		log.Printf("Could not generate a thumbnail: %v\n", err)
		return nil, err
	}

	imageRawBytes := newImage.Image()
	hash := sha256.New()
	hash.Write(imageRawBytes)
	b64EncImageRaw := b64.StdEncoding.EncodeToString(imageRawBytes)
	b64EncImageThumbnail := b64.StdEncoding.EncodeToString(thumbnailBytes)
	imgRequest := &ImageRequest{
		URL:               url,
		B64Bytes:          b64EncImageRaw,
		B64BytesThumbnail: b64EncImageThumbnail,
		SHA256Hash:        b64.URLEncoding.EncodeToString(hash.Sum(nil)),
		ParentURL:         parent_url,
	}
	return imgRequest, err
}

func req_worker(imgRequestChan <-chan *ImageRequest) {
	for true {
		imgRequest := <-imgRequestChan
		jsonBytes, err := json.Marshal(imgRequest)
		strTime := strconv.Itoa(int(time.Now().Unix()))
		if err != nil {
			log.Println("could not marshal urls to json")
		}
		msg := &sarama.ProducerMessage{
			Topic: "facenet",
			Key:   sarama.StringEncoder(strTime),
			Value: sarama.StringEncoder(string(jsonBytes)),
		}
		select {
		case producer.Input() <- msg:
			messagesPublished += 1
			if messagesPublished%100 == 0 {
				fmt.Printf("%v messages published\n", messagesPublished)
			}
		case err := <-producer.Errors():
			fmt.Println("Failed to produce mesage: url = %v, err = %v", imgRequest.URL, err)
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
							imgRequest, err := download_image("http:"+n.Attr[0].Val, url)
							if err != nil {
								log.Printf("download_image failed: -. %v", err)
							} else {
								if imgRequest != nil {
									imgRequestChan <- imgRequest
								}
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
	messagesPublished = 0
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

	for w := 0; w < 150; w++ {
		go worker(w, jobs, results)
	}
	for ww := 0; ww < 10; ww++ {
		go req_worker(results)
	}
	for j := 1; j <= 100000000; j++ {
		jobs <- generate_random_url(5)
	}
	close(jobs)
}

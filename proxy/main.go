package main

import (
	"bytes"
	"context"
	"encoding/base64"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"net/url"
	"os"
	"strings"

	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-lambda-go/lambda"
)

var (
	baseURL   = "http://router.local"
	httpProxy = "http://192.168.254.104:8888"
)

func init() {
	if val := os.Getenv("BASE_URL"); val != "" {
		baseURL = val
	}
	if val := os.Getenv("HTTP_PROXY"); val != "" {
		httpProxy = val
	}
}

func handler(ctx context.Context, event events.APIGatewayProxyRequest) (events.APIGatewayProxyResponse, error) {
	defer handlePanic()

	log.Printf("Got event: %#v", event)

	res := &events.APIGatewayProxyResponse{}

	client := &http.Client{
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}
	proxyURL, err := url.Parse(httpProxy)
	if err == nil {
		client.Transport = &http.Transport{Proxy: http.ProxyURL(proxyURL)}
	} else {
		log.Printf("Error parsing http proxy: %s", err.Error())
	}

	url := baseURL + event.Path
	log.Printf("%s %s", event.HTTPMethod, url)
	req, err := http.NewRequest(event.HTTPMethod, url, bytes.NewBufferString(event.Body))
	if err != nil {
		log.Printf("Error creating request: %s", err.Error())
		return *res, err
	}
	for key, values := range event.MultiValueHeaders {
		for _, value := range values {
			req.Header.Add(key, value)
		}
	}

	response, err := client.Do(req)
	if err != nil {
		log.Printf("Error getting resource: %s (%#v)", err.Error(), response)
		return *res, err
	}

	if response.StatusCode > 399 {
		log.Printf("Expecting 200-300 response, got %d", response.StatusCode)
	}

	res.StatusCode = response.StatusCode
	res.MultiValueHeaders = response.Header

	stage := event.RequestContext.Stage

	redirect := strings.NewReplacer("http://"+req.URL.Hostname(), "https://"+event.Headers["Host"]+"/"+stage)
	for key, values := range res.MultiValueHeaders {
		if key == "Location" {
			for index, url := range values {
				res.MultiValueHeaders[key][index] = redirect.Replace(url)
			}
		}
	}

	body, err := ioutil.ReadAll(response.Body)
	if err != nil {
		log.Printf("Error reading response body: %s", err.Error())
	}

	if len(body) == 0 {
		return *res, nil
	}

	if len(res.MultiValueHeaders["Content-Type"]) > 0 && strings.Contains(res.MultiValueHeaders["Content-Type"][0], "image") {
		res.Body = base64.StdEncoding.EncodeToString(body)
		res.IsBase64Encoded = true
	} else {
		// make it relative to ./
		r := strings.NewReplacer(
			`"../`, `"/`+stage+`/`,
			"'../", `'/`+stage+`/`,
			`"/`, `"/`+stage+`/`,
			"'/", `'/`+stage+`/`,
		)
		res.Body = r.Replace(string(body))
	}

	return *res, nil
}

func main() {
	lambda.Start(handler)
}

func handlePanic() {
	msg := recover()
	if msg != nil {
		var err string
		switch msg := msg.(type) {
		case string:
			err = msg
		case error:
			err = msg.Error()

		default:
			err = fmt.Sprintf("Unknown error type: %#v", msg)
		}

		log.Printf("Go panic: %s", err)
	}
}

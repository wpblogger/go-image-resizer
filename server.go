package main

import (
	"bytes"
	"errors"
	"fmt"
	"image"
	"image/gif"
	"image/jpeg"
	"image/png"
	"io/ioutil"
	"log"
	"os"
	"regexp"
	"strconv"
	"time"

	"github.com/buaazp/fasthttprouter"
	"github.com/getsentry/sentry-go"
	"github.com/nfnt/resize"
	"github.com/valyala/fasthttp"
)

func getResizeJPG(ctx *fasthttp.RequestCtx) {
	var out string
	var err error
	start := time.Now()
	url := string(ctx.RequestURI())
	validURL, _ := regexp.MatchString(`^(.*)/resizer/(\d+)/(\d+)[/]*$`, url)
	if !validURL {
		ctx.NotFound()
		return
	}
	regex := regexp.MustCompile(`^(.*)/resizer/(\d+)/(\d+)[/]*$`)
	params := regex.FindAllStringSubmatch(url, -1)[0]
	realImageURL := string(ctx.URI().Scheme()) + `://` + string(ctx.Request.Header.Host()) + params[1]
	out, err = readCache(params)
	if err != nil {
		out, err = convertImage(realImageURL, params)
		if err != nil {
			ctx.NotFound()
			return
		}
		err = writeCache(params, out)
		if err != nil {
			log.Print(err)
		}
		log.Print("GetConvert:", realImageURL, " Time:", time.Since(start))
	} else {
		log.Print("Cache:", realImageURL, " Time:", time.Since(start))
	}

	/*req := fasthttp.AcquireRequest()
	req.SetRequestURI(realImageURL)
	req.Header.SetConnectionClose()
	req.Header.SetMethod("GET")
	resp := fasthttp.AcquireResponse()
	client := &fasthttp.Client{MaxIdleConnDuration: time.Second}
	client.Do(req, resp)
	if resp.StatusCode() != fasthttp.StatusOK || resp.Header.ContentLength() == 0 {
		ctx.NotFound()
		return
	}
	partOneTime := time.Since(start)
	start = time.Now()
	ctx.Response.Header.Set("Content-Type", "image/jpeg")
	r := bytes.NewBuffer(resp.Body())
	var img image.Image
	switch imgType := string(resp.Header.ContentType()); imgType {
	case "image/jpeg":
		img, err = jpeg.Decode(r)
	case "image/png":
		img, err = png.Decode(r)
	case "image/gif":
		img, err = gif.Decode(r)
	default:
		ctx.NotFound()
		return
	}
	if err != nil {
		ctx.NotFound()
		return
	}
	m := resize.Resize(uint(newWidth), uint(newHeight), img, resize.Lanczos3)
	var out bytes.Buffer
	jpeg.Encode(&out, m, nil)
	log.Print("Get:", realImageURL, " Time:", partOneTime, " Out:", url, " Time:", time.Since(start))*/
	ctx.Response.Header.Set("Content-Type", "image/jpeg")
	fmt.Fprint(ctx, out)
}

func readCache(params []string) (string, error) {
	data, err := ioutil.ReadFile("/tmp" + params[1] + "/" + params[2] + "_" + params[3])
	return string(data), err
}

func writeCache(params []string, data string) error {
	os.MkdirAll("/tmp"+params[1], os.ModePerm)
	err := ioutil.WriteFile("/tmp"+params[1]+"/"+params[2]+"_"+params[3], []byte(data), 0644)
	return err
}

func convertImage(realImageURL string, params []string) (string, error) {
	var err error
	var result string
	newWidth, err := strconv.Atoi(params[2])
	if err != nil {
		err = errors.New("Width not int")
		return result, err
	}
	newHeight, err := strconv.Atoi(params[3])
	if err != nil {
		err = errors.New("Height not int")
		return result, err
	}
	req := fasthttp.AcquireRequest()
	req.SetRequestURI(realImageURL)
	req.Header.SetConnectionClose()
	req.Header.SetMethod("GET")
	resp := fasthttp.AcquireResponse()
	client := &fasthttp.Client{MaxIdleConnDuration: time.Second}
	client.Do(req, resp)
	if resp.StatusCode() != fasthttp.StatusOK || resp.Header.ContentLength() == 0 {
		err = errors.New("Error get src image " + realImageURL)
		return result, err
	}
	r := bytes.NewBuffer(resp.Body())
	var img image.Image
	switch imgType := string(resp.Header.ContentType()); imgType {
	case "image/jpeg":
		img, err = jpeg.Decode(r)
	case "image/png":
		img, err = png.Decode(r)
	case "image/gif":
		img, err = gif.Decode(r)
	default:
		err = errors.New("Unknown Content-type " + imgType)
		return result, err
	}
	if err != nil {
		return result, err
	}
	m := resize.Resize(uint(newWidth), uint(newHeight), img, resize.Lanczos3)
	var out bytes.Buffer
	jpeg.Encode(&out, m, nil)
	result = out.String()
	return result, err
}

func main() {
	listenPort := "8080"
	if len(os.Getenv("PORT")) > 0 {
		listenPort = os.Getenv("PORT")
	}
	err := sentry.Init(sentry.ClientOptions{
		Dsn: os.Getenv("SENTRYURL"),
	})
	if err != nil {
		log.Panic(err)
	}
	router := fasthttprouter.New()
	router.GET("/*name", getResizeJPG)
	server := &fasthttp.Server{
		Handler:            router.Handler,
		MaxRequestBodySize: 100 << 20,
	}
	log.Print("App start on port ", listenPort)
	log.Fatal(server.ListenAndServe(":" + listenPort))
}

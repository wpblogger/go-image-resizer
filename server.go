package main

import (
	"bytes"
	"errors"
	"fmt"
	"image"
	"os/signal"
	"syscall"

	"image/color"
	"image/draw"
	"image/gif"
	"image/jpeg"
	"image/png"
	"log"
	"os"
	"regexp"
	"strconv"
	"time"

	"github.com/buaazp/fasthttprouter"
	"github.com/getsentry/sentry-go"
	"github.com/nfnt/resize"
	pudge "github.com/recoilme/pudge"
	"github.com/valyala/fasthttp"
)

type queData struct {
	createTime int64
	filePath   string
}

var cacheInSeconds int64
var queue []queData
var branch string
var db *pudge.Db

func getResizeJPG(ctx *fasthttp.RequestCtx) {
	var out string
	var err error
	start := time.Now()
	url := string(ctx.RequestURI())
	scheme := string(ctx.Request.Header.Peek("X-Forwarded-Proto"))
	realIP := string(ctx.Request.Header.Peek("X-Real-IP"))
	host := string(ctx.Request.Header.Host())
	requestURI := string(ctx.RequestURI())
	versionURL, _ := regexp.MatchString(`^/api/system/version[/]*$`, url)
	if versionURL {
		getVersion(ctx)
		return
	}
	log.Print("Request image: ", scheme+`://`+host+requestURI, " From: ", realIP)
	validURL, _ := regexp.MatchString(`^(.*)/resizer/(\d+)/(\d+)[/]*$`, url)
	if !validURL {
		ctx.NotFound()
		return
	}
	regex := regexp.MustCompile(`^(.*)/resizer/(\d+)/(\d+)[/]*$`)
	params := regex.FindAllStringSubmatch(url, -1)[0]
	realImageURL := scheme + `://` + host + params[1]
	out, err = readCache(params)
	if err != nil {
		out, err = convertImage(realImageURL, params)
		if err != nil {
			log.Print(err)
			sentry.CaptureException(err)
			ctx.NotFound()
			return
		}
		err = writeCache(params, out)
		if err != nil {
			sentry.CaptureException(err)
			log.Print(err)
		}
		log.Print("GetConvert:", realImageURL, " Time:", time.Since(start))
	} else {
		log.Print("Cache:", realImageURL, " Time:", time.Since(start))
	}
	ctx.Response.Header.Set("Content-Type", "image/jpeg")
	fmt.Fprint(ctx, out)
}

func readCache(params []string) (string, error) {
	var err error
	var data string
	db.Get(params[1]+"/"+params[2]+"_"+params[3], &data)
	if len(data) == 0 {
		err = errors.New("Error, unable to read value from cache")
	}
	return data, err
}

func writeCache(params []string, data string) error {
	var err error
	var checkData string
	db.Set(params[1]+"/"+params[2]+"_"+params[3], data)
	db.Get(params[1]+"/"+params[2]+"_"+params[3], &checkData)
	if checkData != data {
		err = errors.New("Error, unable to add value to cache")
	} else {
		data := queData{createTime: time.Now().Unix(), filePath: params[1] + "/" + params[2] + "_" + params[3]}
		queue = append(queue, data)
	}
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
	imgType := string(resp.Header.ContentType())
	switch imgType {
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
	m := resize.Resize(uint(newWidth), uint(newHeight), img, resize.Bilinear)
	var out bytes.Buffer
	var opt jpeg.Options
	opt.Quality = 100
	if imgType == "image/png" {
		newImg := image.NewRGBA(m.Bounds())
		draw.Draw(newImg, newImg.Bounds(), &image.Uniform{color.White}, image.Point{}, draw.Src)
		draw.Draw(newImg, newImg.Bounds(), m, m.Bounds().Min, draw.Over)
		jpeg.Encode(&out, newImg, &opt)
	} else {
		jpeg.Encode(&out, m, &opt)
	}
	result = out.String()
	return result, err
}

func cleanCache() {
	var i int
	var path string
	now := time.Now().Unix()
	for idx, el := range queue {
		if el.createTime < now-cacheInSeconds {
			i = idx
			path = el.filePath
		}
	}
	if len(path) == 0 {
		return
	}
	db.Delete(path)
	var err error
	var data string
	db.Get(path, &data)
	if len(data) != 0 {
		err = errors.New("Error, unable to delete value from cache")
	}
	if err != nil {
		sentry.CaptureException(err)
		log.Print(err)
	} else {
		queue = append(queue[:i], queue[i+1:]...)
		log.Print("Key ", path, " delete from cache")
	}
}

func getVersion(ctx *fasthttp.RequestCtx) {
	ctx.Response.Header.Set("Content-Type", "application/json")
	fmt.Fprint(ctx, `{"data": {"version": "`+branch+`"}, "error": {}}`)
}

func main() {
	queue = []queData{}
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
	if len(os.Getenv("BRANCH")) > 0 {
		branch = os.Getenv("BRANCH")
	}
	cacheInSeconds = 3600
	if len(os.Getenv("CACHEINSECONDS")) > 0 {
		cacheInt, err := strconv.ParseInt(os.Getenv("CACHEINSECONDS"), 10, 64)
		if err == nil {
			cacheInSeconds = cacheInt
		}
	}
	cfg := &pudge.Config{StoreMode: 2}
	db, err = pudge.Open("", cfg)
	if err != nil {
		log.Panic(err)
	}
	go func() {
		for {
			cleanCache()
			time.Sleep(10 * time.Second)
		}
	}()
	var gracefulStop = make(chan os.Signal)
	signal.Notify(gracefulStop, syscall.SIGTERM)
	signal.Notify(gracefulStop, syscall.SIGINT)
	go func() {
		sig := <-gracefulStop
		log.Printf("Received system call: %+v", sig)
		log.Print("Start shutdown App")
		db.Close()
		db.DeleteFile()
		log.Print("App shutdown")
		os.Exit(0)
	}()
	router := fasthttprouter.New()
	router.GET("/*name", getResizeJPG)
	server := &fasthttp.Server{
		Handler:            router.Handler,
		MaxRequestBodySize: 100 << 20,
	}
	log.Print("App start on port ", listenPort)
	log.Fatal(server.ListenAndServe(":" + listenPort))
}

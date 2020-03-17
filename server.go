package main

import (
	"bytes"
	"fmt"
	"image/jpeg"
	"log"
	"os"
	"regexp"
	"time"

	"github.com/buaazp/fasthttprouter"
	"github.com/getsentry/sentry-go"
	"github.com/nfnt/resize"
	"github.com/valyala/fasthttp"
)

func getResizeJPG(ctx *fasthttp.RequestCtx) {
	url := string(ctx.RequestURI())
	validURL, _ := regexp.MatchString(`^(.*)/resizer/(\d+)/(\d+)[/]*$`, url)
	if !validURL {
		ctx.NotFound()
	}
	regex := regexp.MustCompile(`^(.*)/resizer/(\d+)/(\d+)[/]*$`)
	rxVars := regex.FindAllStringSubmatch(url, -1)
	realImageURL := string(ctx.URI().Scheme()) + `://` + string(ctx.Request.Header.Host()) + rxVars[0][1]
	log.Print(realImageURL)

	req := fasthttp.AcquireRequest()
	req.SetRequestURI(realImageURL)
	req.Header.SetConnectionClose()
	req.Header.SetMethod("GET")
	resp := fasthttp.AcquireResponse()
	client := &fasthttp.Client{MaxIdleConnDuration: time.Second}
	client.Do(req, resp)
	if resp.StatusCode() != fasthttp.StatusOK || resp.Header.ContentLength() == 0 {
		ctx.NotFound()
	}
	log.Print(string(resp.Header.ContentType()))
	ctx.Response.Header.Set("Content-Type", string(resp.Header.ContentType()))
	//fmt.Fprint(ctx, string(resp.Body()))
	/*var r bytes.Buffer
	_, err := io.ReadFull(&r, bytes.NewBuffer(resp.Body()))
	if err != nil {
		ctx.NotFound()
	}*/
	//var r *bytes.Buffer
	r := bytes.NewBuffer(resp.Body())
	img, err := jpeg.Decode(r)
	if err != nil {
		ctx.NotFound()
	}
	log.Print(img)
	m := resize.Resize(40, 0, img, resize.Lanczos3)
	var out bytes.Buffer
	jpeg.Encode(&out, m, nil)
	fmt.Fprint(ctx, out.String())
}

/*func getVersion(ctx *fasthttp.RequestCtx) {
	var respError string
	req := fasthttp.AcquireRequest()
	req.SetRequestURI(elasticURL)
	req.Header.SetConnectionClose()
	req.Header.SetMethod("GET")
	resp := fasthttp.AcquireResponse()
	client := &fasthttp.Client{MaxIdleConnDuration: time.Second}
	client.Do(req, resp)
	ctx.Response.Header.Set("Content-Type", "application/json")
	if resp.Header.ContentLength() == 0 {
		respError = "No connection to Elastic"
	}
	if resp.StatusCode() == fasthttp.StatusNotFound {
		respError = "Elastic Index KLADR not available"
	}
	if resp.StatusCode() == fasthttp.StatusOK && resp.Header.ContentLength() > 0 {
		fmt.Fprint(ctx, `{"data": {"version": "`+branch+`"}, "error": {`+respError+`}}`)
	} else {
		sentry.CaptureException(errors.New(respError))
		ctx.Error(respError, fasthttp.StatusInternalServerError)
	}
}*/

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
	//router.GET("/status", getStatus)
	server := &fasthttp.Server{
		Handler:            router.Handler,
		MaxRequestBodySize: 100 << 20,
	}
	log.Print("App start on port ", listenPort)
	log.Fatal(server.ListenAndServe(":" + listenPort))
}

package jsonhttp

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"mime/multipart"
	"net/http"
	"os"
	"os/signal"
	"runtime/debug"
	"strings"
	"syscall"
	"time"

	"github.com/lib/pq"
	"gopl.io/ch12/params"
)

var _maxTry int

const defaultMaxMemory = 32 << 20 // 32 MB

// maxTry is only use for postgres
func Listen(address string, maxTry int, logger Logger) {
	if maxTry <= 0 {
		panic(fmt.Errorf("jsonhttp: listen %s failed, maxTry must be positive", address))
	}
	_maxTry = maxTry
	_logger = logger

	c := make(chan os.Signal)
	signal.Notify(c, syscall.SIGHUP, syscall.SIGINT, syscall.SIGTERM, syscall.SIGQUIT)
	go func() {
		logger.Log(Info, (<-c).String())
		os.Exit(5)
	}()
	logger.Log(Info, "jsonhttp: start listen "+address)
	Must(http.ListenAndServe(address, nil))
}

func Handle(partten string, handler func(req Request) Response) {
	http.HandleFunc(partten, handle(handleFunc(handler)))
}

func HandleFile(partten string, handler func(req Request) ResponseFile) {
	http.HandleFunc(partten, handle(handleFileFunc(handler)))
}

func HandleForm(partten string, handler func(req RequestForm) Response) {
	http.HandleFunc(partten, handle(handleFormFunc(handler)))
}

func HandleFormFile(partten string, handler func(req RequestForm) ResponseFile) {
	http.HandleFunc(partten, handle(handleFormFileFunc(handler)))
}

func HandleGet(partten string, handler func(req RequestGet) Response) {
	http.HandleFunc(partten, handle(handleGetFunc(handler)))
}

func HandleGetFile(partten string, handler func(req RequestGet) ResponseFile) {
	http.HandleFunc(partten, handle(handleGetFileFunc(handler)))
}

func HandleOrigin(partten string, handler http.Handler) {
	http.Handle(partten, handler)
}

type CommonRequestInterface interface {
	Req() *http.Request
	Res() http.ResponseWriter
	IP() string
}

type CommonRequest struct {
	r  *http.Request
	w  http.ResponseWriter
	ip string
}

func (r CommonRequest) Req() *http.Request {
	return r.r
}

func (r CommonRequest) Res() http.ResponseWriter {
	return r.w
}

func (r CommonRequest) IP() string {
	return r.ip
}

func getCommonRequest(w http.ResponseWriter, r *http.Request) CommonRequest {
	var req = CommonRequest{r, w, ""}
	if r.RemoteAddr != "" {
		req.ip = strings.Split(r.RemoteAddr, ":")[0]
	}
	return req
}

// Request http post json request struct
type Request struct {
	CommonRequest
	Data      []byte
	Unmarshal func(v interface{})
}

func getRequest(w http.ResponseWriter, r *http.Request) Request {
	var data, err = ioutil.ReadAll(r.Body)
	Must(err)
	var unmarshal = func(v interface{}) {
		MustWithCode(json.Unmarshal(data, v), http.StatusBadRequest)
	}
	return Request{getCommonRequest(w, r), data, unmarshal}
}

// RequestForm http post form request struct
type RequestForm struct {
	CommonRequest
	Data *multipart.Form
}

func getRequestForm(w http.ResponseWriter, r *http.Request) RequestForm {
	Must(r.ParseMultipartForm(defaultMaxMemory))
	return RequestForm{getCommonRequest(w, r), r.MultipartForm}
}

// RequestGet http get request struct
type RequestGet struct {
	CommonRequest
	RawQuery  string
	Unmarshal func(v interface{}) error
}

func getRequestGet(w http.ResponseWriter, r *http.Request) RequestGet {
	var unmarshal = func(v interface{}) error {
		return params.Unpack(r, v)
	}
	return RequestGet{getCommonRequest(w, r), r.URL.RawQuery, unmarshal}
}

// Response json response struct
type Response struct {
	Success bool        `json:"success"`
	Code    int         `json:"code"`
	Msg     string      `json:"msg,omitempty"`
	Data    interface{} `json:"data,omitempty"`
}

func (res Response) do(w http.ResponseWriter) {
	w.Header().Set("Content-Type", "application/json")
	resJSONByte, err := json.Marshal(res)
	if err != nil {
		Log(Error, err)
	}
	_, err = fmt.Fprint(w, string(resJSONByte))
	if err != nil {
		Log(Error, err)
	}
}

// ResponseFile file response struct
type ResponseFile struct {
	FileName string
	Content  []byte
	Modtime  time.Time
}

func (res ResponseFile) do(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Disposition", "attachment; filename="+res.FileName)
	http.ServeContent(w, r, res.FileName, res.Modtime, bytes.NewReader(res.Content))
}

type handler interface {
	getRequest(w http.ResponseWriter, r *http.Request) interface{}
	serve(req interface{}, w http.ResponseWriter, r *http.Request)
}

func handle(h handler) func(w http.ResponseWriter, r *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		defer doRecover(w)
		var req = h.getRequest(w, r)
		for try := _maxTry; try > 0; try-- {
			if !serve(h, req, w, r) {
				return
			}
		}
		fmt.Fprint(w, `{"ok":false,"msg":"Server is busy, please try later!"}`)
	}
}

func serve(h handler, req interface{}, w http.ResponseWriter, r *http.Request) (retry bool) {
	defer func() { retry = doRecover(w) }()
	h.serve(req, w, r)
	return false
}

// service recover from panic
func doRecover(w http.ResponseWriter) (retry bool) {
	switch err := recover().(type) {
	case nil:
	case *pq.Error:
		if err.Code == "40001" || err.Code == "55P03" {
			Log(Warn, err, string(debug.Stack()))
			return true
		}
		w.WriteHeader(http.StatusInternalServerError)
		Log(Error, err, string(debug.Stack()))
	case ErrorWithCode:
		w.WriteHeader(err.HTTPResponseStatusCode)
		Log(Warn, err, string(debug.Stack()))
	default:
		w.WriteHeader(http.StatusInternalServerError)
		Log(Error, err, string(debug.Stack()))
	}
	return false
}

type handleFunc func(req Request) Response

func (f handleFunc) getRequest(w http.ResponseWriter, r *http.Request) interface{} {
	return getRequest(w, r)
}

func (f handleFunc) serve(req interface{}, w http.ResponseWriter, r *http.Request) {
	f(req.(Request)).do(w)
}

type handleFileFunc func(req Request) ResponseFile

func (f handleFileFunc) getRequest(w http.ResponseWriter, r *http.Request) interface{} {
	return getRequest(w, r)
}

func (f handleFileFunc) serve(req interface{}, w http.ResponseWriter, r *http.Request) {
	f(req.(Request)).do(w, r)
}

type handleFormFunc func(req RequestForm) Response

func (f handleFormFunc) getRequest(w http.ResponseWriter, r *http.Request) interface{} {
	return getRequestForm(w, r)
}

func (f handleFormFunc) serve(req interface{}, w http.ResponseWriter, r *http.Request) {
	f(req.(RequestForm)).do(w)
}

type handleFormFileFunc func(req RequestForm) ResponseFile

func (f handleFormFileFunc) getRequest(w http.ResponseWriter, r *http.Request) interface{} {
	return getRequestForm(w, r)
}

func (f handleFormFileFunc) serve(req interface{}, w http.ResponseWriter, r *http.Request) {
	f(req.(RequestForm)).do(w, r)
}

type handleGetFunc func(req RequestGet) Response

func (f handleGetFunc) getRequest(w http.ResponseWriter, r *http.Request) interface{} {
	return getRequestGet(w, r)
}

func (f handleGetFunc) serve(req interface{}, w http.ResponseWriter, r *http.Request) {
	f(req.(RequestGet)).do(w)
}

type handleGetFileFunc func(req RequestGet) ResponseFile

func (f handleGetFileFunc) getRequest(w http.ResponseWriter, r *http.Request) interface{} {
	return getRequestGet(w, r)
}

func (f handleGetFileFunc) serve(req interface{}, w http.ResponseWriter, r *http.Request) {
	f(req.(RequestGet)).do(w, r)
}

type FailCode interface {
	Int() int
	Message() string
}

// Fail generate fail response
func Fail(code FailCode) Response {
	return Response{Success: false, Msg: code.Message(), Code: code.Int()}
}

func FailWithMsg(code FailCode, msg string) Response {
	return Response{Success: false, Msg: msg, Code: code.Int()}
}

// Success generate success response
func Success(data interface{}) Response {
	return Response{Success: true, Data: data}
}

// EchoSuccess response success message for debug
func Echo(req Request) Response {
	var reqData interface{}
	req.Unmarshal(&reqData)
	return Success(reqData)
}

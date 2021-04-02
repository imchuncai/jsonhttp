package jsonhttp

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"mime/multipart"
	"net/http"
	"runtime/debug"
	"strings"
	"time"

	"github.com/lib/pq"
	"gopl.io/ch12/params"
)

var maxTry int

const defaultMaxMemory = 32 << 20 // 32 MB

func Listen(address string, try int) {
	if try <= 0 {
		LogErrorAndPanic(fmt.Errorf("listen %s failed, try must be positive", address))
	} else {
		maxTry = try
		http.ListenAndServe(address, nil)
	}
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

// Request http post json request struct
type Request struct {
	IP        string
	Unmarshal func(v interface{})
}

func getRequest(w http.ResponseWriter, r *http.Request) Request {
	var data, err = ioutil.ReadAll(r.Body)
	CheckError(err)
	var req Request
	if r.RemoteAddr != "" {
		req.IP = strings.Split(r.RemoteAddr, ":")[0]
	}
	req.Unmarshal = func(v interface{}) {
		CheckErrorWithCode(json.Unmarshal(data, v), http.StatusBadRequest)
	}
	return req
}

// RequestForm http post form request struct
type RequestForm struct {
	Data *multipart.Form
}

func getRequestForm(w http.ResponseWriter, r *http.Request) RequestForm {
	CheckError(r.ParseMultipartForm(defaultMaxMemory))
	return RequestForm{r.MultipartForm}
}

// RequestGet http get request struct
type RequestGet struct {
	Unmarshal func(v interface{}) error
}

func getRequestGet(w http.ResponseWriter, r *http.Request) RequestGet {
	var unmarshal = func(v interface{}) error {
		return params.Unpack(r, v)
	}
	return RequestGet{unmarshal}
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
	LogError(err)
	_, err = fmt.Fprint(w, resJSONByte)
	LogError(err)
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
		var req = h.getRequest(w, r)
		for try := maxTry; try > 0; try-- {
			if !serve(h, req, w, r) {
				return
			}
		}
		fmt.Fprint(w, `{"ok":false,"msg":"服务正忙！请稍后重试！"}`)
	}
}

func serve(h handler, req interface{}, w http.ResponseWriter, r *http.Request) (retry bool) {
	defer func() { retry = doRecover(w) }()
	h.serve(req, w, r)
	return false
}

// service recover from panic
func doRecover(w http.ResponseWriter) (retry bool) {
	var r = recover()
	if r == nil {
		return false
	}
	if err, ok := r.(*pq.Error); ok && (err.Code == "40001" || err.Code == "55P03") {
		Log(Error, err.Error()+"\n"+string(debug.Stack()))
		return true
	}
	var originError error
	if err, ok := r.(ErrorWithCode); ok {
		w.WriteHeader(err.HTTPResponseStatusCode)
		originError = err.OriginError
	} else {
		w.WriteHeader(http.StatusInternalServerError)
		originError = fmt.Errorf("%v", r)
	}
	Log(Error, originError.Error()+"\n"+string(debug.Stack()))
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

// Success generate success response
func Success(data interface{}) Response {
	return Response{Success: true, Data: data}
}

// EchoSuccess response success message for debug
func Echo(req Request) Response {
	var reqData struct {
		Res interface{} `json:"res"`
	}
	req.Unmarshal(&reqData)
	return Success(reqData.Res)
}

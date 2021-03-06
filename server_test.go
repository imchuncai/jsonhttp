package jsonhttp

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"
	"time"

	mylog "github.com/imchuncai/log"
)

type Logger log.Logger

func (l *Logger) Log(prefix mylog.Prefix, v ...interface{}) {
	(*log.Logger)(l).Print(v...)
}

func SetEnvironment() {
	_maxTry = 1
	_logger = (*Logger)(log.Default())
}

func JsonEqual(a, b []byte) (bool, error) {
	var aa, bb interface{}
	err := json.Unmarshal(a, &aa)
	if err != nil {
		return false, err
	}
	err = json.Unmarshal(b, &bb)
	if err != nil {
		return false, err
	}
	return reflect.DeepEqual(aa, bb), nil
}

func TestHandle(t *testing.T) {
	SetEnvironment()

	const msg = "hello, world!"
	handler := http.HandlerFunc(handle(handleFunc(func(req Request) Response {
		return Success(string(req.Data))
	})))
	ts := httptest.NewServer(handler)
	defer ts.Close()

	res, err := http.Post(ts.URL, "", bytes.NewReader([]byte(msg)))
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	data, err := io.ReadAll(res.Body)
	if err != nil {
		t.Fatal(err)
	}

	want := fmt.Sprintf(`{"success":true,"code":0,"data":"%s"}`, msg)
	equal, err := JsonEqual(data, []byte(want))
	if err != nil {
		t.Fatal(err)
	}
	if !equal {
		t.Fatalf("want data: %s got %s", want, string(data))
	}
}

func TestHandleFile(t *testing.T) {
	SetEnvironment()

	const msg = "hello, world!"
	handler := http.HandlerFunc(handle(handleFileFunc(func(req Request) ResponseFile {
		return ResponseFile{
			FileName: "hi",
			Content:  bytes.NewReader([]byte(msg)),
			Modtime:  time.Now(),
		}
	})))
	ts := httptest.NewServer(handler)
	defer ts.Close()

	res, err := http.Post(ts.URL, "", bytes.NewReader(nil))
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	data, err := io.ReadAll(res.Body)
	if err != nil {
		t.Fatal(err)
	}

	want := msg
	if string(data) != want {
		t.Fatalf("want data: %s got %s", want, string(data))
	}
}

func TestHandleForm(t *testing.T) {
	SetEnvironment()

	formKey := "hi"
	formValue := "hello, world!"
	formFileKey := "hi_file"
	formFileValue := "hello, file!"
	handler := http.HandlerFunc(handle(handleFormFunc(func(req RequestForm) Response {
		// test form data
		value := req.Data.Value[formKey]
		if len(value) < 1 {
			t.Fatalf(`no form data "%s"`, formKey)
		}
		want := formValue
		if value[0] != want {
			t.Fatalf(`want form["%s"]: %s got %s`, formKey, want, value[0])
		}

		// test form file
		files := req.Data.File[formFileKey]
		if len(files) < 1 {
			t.Fatalf(`no form file "%s"`, formFileKey)
		}
		want = formFileValue
		file, err := files[0].Open()
		if err != nil {
			t.Fatal(err)
		}
		data, err := ioutil.ReadAll(file)
		if err != nil {
			t.Fatal(err)
		}
		if string(data) != want {
			t.Fatalf(`want form file["%s"]: %s got %s`, formFileKey, want, string(data))
		}

		return Success(string(data))
	})))
	ts := httptest.NewServer(handler)
	defer ts.Close()

	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)

	part, err := writer.CreateFormField(formKey)
	if err != nil {
		t.Fatal(err)
	}
	_, err = part.Write([]byte(formValue))
	if err != nil {
		t.Fatal(err)
	}

	part, err = writer.CreateFormFile(formFileKey, formFileKey)
	if err != nil {
		t.Fatal(err)
	}
	_, err = part.Write([]byte(formFileValue))
	if err != nil {
		t.Fatal(err)
	}

	writer.Close()

	request, err := http.NewRequest("POST", ts.URL, body)
	if err != nil {
		t.Fatal(err)
	}
	request.Header.Add("Content-Type", writer.FormDataContentType())

	client := &http.Client{}
	res, err := client.Do(request)
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()

	data, err := io.ReadAll(res.Body)
	if err != nil {
		t.Fatal(err)
	}

	want := fmt.Sprintf(`{"success":true,"code":0,"data":"%s"}`, formFileValue)
	equal, err := JsonEqual(data, []byte(want))
	if err != nil {
		t.Fatal(err)
	}
	if !equal {
		t.Fatalf("want data: %s got %s", want, string(data))
	}
}

func TestHandleGet(t *testing.T) {
	SetEnvironment()

	hiMessage := "hello_world"
	getRawQuery := "hi=" + hiMessage
	handler := http.HandlerFunc(handle(handleGetFunc(func(req RequestGet) Response {
		var reqData struct {
			Hi string `http:"hi"`
		}
		req.Unmarshal(&reqData)
		if reqData.Hi != hiMessage {
			t.Fatalf(`want hi: %s got: %s`, hiMessage, reqData.Hi)
		}

		return Success(reqData.Hi)
	})))
	ts := httptest.NewServer(handler)
	defer ts.Close()

	res, err := http.Get(ts.URL + "?" + getRawQuery)
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()

	data, err := ioutil.ReadAll(res.Body)
	if err != nil {
		t.Fatal(err)
	}
	want := fmt.Sprintf(`{"success":true,"code":0,"data":"%s"}`, hiMessage)
	equal, err := JsonEqual(data, []byte(want))
	if err != nil {
		t.Fatal(err)
	}
	if !equal {
		t.Fatalf("want data: %s got %s", want, string(data))
	}
}

func TestHandleRedirect(t *testing.T) {
	SetEnvironment()

	const redirectURL = "https://imchuncai.com"
	const redirestCode = http.StatusTemporaryRedirect
	handler := http.HandlerFunc(handle(HandleGetRedirectFunc(func(req RequestGet) ResponseRedirect {
		return ResponseRedirect{redirectURL, redirestCode}
	})))
	ts := httptest.NewServer(handler)
	defer ts.Close()

	client := http.Client{CheckRedirect: func(req *http.Request, via []*http.Request) error {
		return http.ErrUseLastResponse
	}}
	res, err := client.Get(ts.URL)
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()

	if res.StatusCode != redirestCode {
		t.Fatalf("want status code: %d got %d", redirestCode, res.StatusCode)
	}
	if res.Header.Get("Location") != redirectURL {
		t.Fatalf("want redirect location: %s got %s", redirectURL, res.Header.Get("Location"))
	}
}

package jsonhttp

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"time"
)

func ExampleServer() {
	Handle("/hi", func(req Request) Response {
		var reqData struct {
			Name string `json:"name"`
		}
		req.Unmarshal(&reqData)
		if reqData.Name == "" {
			MustWithCode(errors.New("name is empty"), http.StatusBadRequest)
		}

		data := struct {
			Message string `json:"message"`
		}{
			fmt.Sprintf("hello, %s!", reqData.Name),
		}
		return Success(data)
	})
	go Listen(":8080", 3, (*Logger)(log.Default()))
	time.Sleep(time.Second * 3)

	res, err := http.Post("http://localhost:8080/hi", "", bytes.NewBufferString(`{"name": "imchuncai"}`))
	if err != nil {
		log.Fatal(err)
	}
	defer res.Body.Close()

	data, err := ioutil.ReadAll(res.Body)
	if err != nil {
		log.Fatal(err)
	}
	var resData struct {
		Success bool   `json:"success"`
		Code    int    `json:"code"`
		Msg     string `json:"msg"`
		Data    struct {
			Message string `json:"message"`
		} `json:"data"`
	}
	err = json.Unmarshal(data, &resData)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println(resData)
	// Output: {true 0  {hello, imchuncai!}}
}

func ExampleGetServer() {
	HandleGet("/hello", func(req RequestGet) Response {
		var reqData struct {
			Name string `http:"name"`
		}
		req.Unmarshal(&reqData)
		if reqData.Name == "" {
			MustWithCode(errors.New("name is empty"), http.StatusBadRequest)
		}

		data := struct {
			Message string `json:"message"`
		}{
			fmt.Sprintf("hello, %s!", reqData.Name),
		}
		return Success(data)
	})
	go Listen(":8081", 3, (*Logger)(log.Default()))
	time.Sleep(time.Second * 3)

	res, err := http.Get("http://localhost:8081/hello?name=imchuncai")
	if err != nil {
		log.Fatal(err)
	}
	defer res.Body.Close()

	data, err := ioutil.ReadAll(res.Body)
	if err != nil {
		log.Fatal(err)
	}
	var resData struct {
		Success bool   `json:"success"`
		Code    int    `json:"code"`
		Msg     string `json:"msg"`
		Data    struct {
			Message string `json:"message"`
		} `json:"data"`
	}
	err = json.Unmarshal(data, &resData)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println(resData)
	// Output: {true 0  {hello, imchuncai!}}
}

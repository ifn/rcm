package main

import (
	"encoding/json"
	"log"
	"net/http"
	"os"
	"runtime"

	"github.com/garyburd/redigo/redis"
	"github.com/gorilla/mux"
)

type Request struct {
	Urls []string `json:"urls"`
}

type Response struct {
	Recommendations map[string]float32 `json:"recommendations"`
	Err             string             `json:"error"`

	conn redis.Conn
}

func (self *Response) countRecommendations(urls []string) (recommendations map[string]float32, err error) {
	//for url in all urls
	//	for up in user profiles
	//		min(similarity, freq)
	//	max

	return
}

func (self *Response) SetRecommendations(urls []string) (err error) {
	conn, err := redis.Dial("tcp", "127.0.0.1:6379")
	if err != nil {
		return
	}
	defer conn.Close()
	self.conn = conn

	self.Recommendations, err = self.countRecommendations(urls)
	return
}

func recommendHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	decoder := json.NewDecoder(r.Body)
	encoder := json.NewEncoder(w)

	req := new(Request)
	resp := &Response{}

	var err error

	defer func() {
		if err != nil {
			log.Println(err)
			resp.Err = err.Error()
		}

		err := encoder.Encode(resp)
		if err != nil {
			log.Println(err)
		}
	}()

	err = decoder.Decode(req)
	if err != nil {
		return
	}

	err = resp.SetRecommendations(req.Urls)
}

func startRecommendationServer() {
	r := mux.NewRouter()
	r.HandleFunc("/recommend", recommendHandler).Methods("POST")
	http.Handle("/", r)

	log.Fatal(http.ListenAndServe(":"+os.Getenv("PORT"), nil))
}

func main() {
	startRecommendationServer()
}

func init() {
	runtime.GOMAXPROCS(runtime.NumCPU())
}

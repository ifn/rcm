package main

import (
	"encoding/json"
	"log"
	"net/http"
	"os"
	"runtime"
	"strconv"
	"strings"

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

func weightKey(url, profile string) string {
	return strings.Join([]string{url, profile}, "|")
}

func getWeight(conn redis.Conn, url, profile string) (w float64, err error) {
	key := weightKey(url, profile)

	weight, err := redis.String(conn.Do("GET", key))
	if err != nil {
		return
	}

	return strconv.ParseFloat(weight, 64)
}

func setWeight(conn redis.Conn, url, profile string) {
}

func (self *Response) countRecommendations(session []string) (recommendations map[string]float32, err error) {
	urls, err := redis.Strings(self.conn.Do("SMEMBERS", "urls"))
	if err != nil {
		return
	}

	profiles, err := redis.Strings(self.conn.Do("SMEMBERS", "profiles"))
	if err != nil {
		return
	}

	for _, url := range urls {
		for _, profile := range profiles {
			var w float64
			w, err = getWeight(self.conn, url, profile)
			if err != nil {
				return
			}
			log.Println(w)
			//min(similarity, freq)
		}
		//max
	}

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

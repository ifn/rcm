package main

import (
	"encoding/json"
	"log"
	"net/http"
	"os"
	"runtime"

	"github.com/gorilla/mux"
)

type Request struct {
	Urls []string `json:"urls"`
}

type Response struct {
	Recommendations map[string]float32 `json:"recommendations"`
	Err             string             `json:"error"`
}

func (self *Response) SetRecommendations() error {
	return nil
}

func recommendHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	decoder := json.NewDecoder(r.Body)
	encoder := json.NewEncoder(w)

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

	err = decoder.Decode(resp)
	if err != nil {
		return
	}

	err = resp.SetRecommendations()
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

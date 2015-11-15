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

type Similar interface {
	Similar(v []float32) float32
}

type Request struct {
	Urls []string `json:"urls"`
}

type Response struct {
	Recommendation map[string]float32 `json:"recommendation"`
	Err            string             `json:"error"`

	conn redis.Conn
}

func weightKey(url, profile string) string {
	return strings.Join([]string{url, profile}, "|")
}

func weightStringToFloat(wString string) (w float32, err error) {
	w64, err := strconv.ParseFloat(wString, 32)
	if err != nil {
		return
	}
	w = float32(w64)
	return
}

func getWeight(conn redis.Conn, url, profile string) (w float32, err error) {
	key := weightKey(url, profile)

	weight, err := redis.String(conn.Do("GET", key))
	if err == redis.ErrNil {
		weight = "0"
	} else if err != nil {
		return
	}

	return weightStringToFloat(weight)
}

func getWeights(conn redis.Conn, urls, profiles []string) ([][]float32, error) {
	var keys = make([]interface{}, 0, len(urls)*len(profiles))
	for _, url := range urls {
		for _, profile := range profiles {
			key := weightKey(url, profile)
			keys = append(keys, key)
		}
	}

	weightsString, err := redis.Strings(conn.Do("MGET", keys...))
	if err != nil {
		return nil, err
	}

	var urlProfile [][]float32 = make([][]float32, len(urls))
	for i := range urls {
		urlProfile[i] = make([]float32, len(profiles))
	}

	for i := range urls {
		for j := range profiles {
			wString := weightsString[i*len(profiles)+j]
			if wString == "" {
				continue
			}

			w, err := weightStringToFloat(wString)
			if err != nil {
				return nil, err
			}

			urlProfile[i][j] = w
		}
	}

	return urlProfile, nil
}

func setWeight(conn redis.Conn, url, profile string) {
}

func (self *Response) countRecommendation(session []string) (recommendation map[string]float32, err error) {
	urls, err := redis.Strings(self.conn.Do("SMEMBERS", "urls"))
	if err != nil {
		return
	}

	profiles, err := redis.Strings(self.conn.Do("SMEMBERS", "profiles"))
	if err != nil {
		return
	}

	weights, err := getWeights(self.conn, urls, profiles)
	if err != nil {
		return
	}

	// Мамдани епта
	for i, url := range urls {
		for j, profile := range profiles {
			log.Println(url, profile, weights[i][j])
			//min(similarity, freq)
		}
		//max
	}

	return
}

func (self *Response) SetRecommendation(session []string) (err error) {
	conn, err := redis.Dial("tcp", "127.0.0.1:6379")
	if err != nil {
		return
	}
	defer conn.Close()
	self.conn = conn

	self.Recommendation, err = self.countRecommendation(session)
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

	err = resp.SetRecommendation(req.Urls)
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

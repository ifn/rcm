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

func stringToFloat32(vString string) (v float32, err error) {
	v64, err := strconv.ParseFloat(vString, 32)
	if err != nil {
		return
	}
	v = float32(v64)
	return
}

func newFloat32Matrix(height, width int) [][]float32 {
	var mx [][]float32 = make([][]float32, height)
	for i := 0; i < width; i++ {
		mx[i] = make([]float32, width)
	}
	return mx
}

func matrixPosition(width, i, j int) int {
	return i*width + j
}

func weightKey(url, profile string) string {
	return strings.Join([]string{url, profile}, "|")
}

func weightKeys(urls, profiles []string) []interface{} {
	var keys = make([]interface{}, 0, len(urls)*len(profiles))
	for _, url := range urls {
		for _, profile := range profiles {
			key := weightKey(url, profile)
			keys = append(keys, key)
		}
	}
	return keys
}

func getWeight(conn redis.Conn, url, profile string) (w float32, err error) {
	key := weightKey(url, profile)

	weight, err := redis.String(conn.Do("GET", key))
	if err == redis.ErrNil {
		weight = "0"
	} else if err != nil {
		return
	}

	return stringToFloat32(weight)
}

func getWeights(conn redis.Conn, urls, profiles []string) ([][]float32, error) {
	keys := weightKeys(urls, profiles)

	// TODO: how to pass []string?
	weightsString, err := redis.Strings(conn.Do("MGET", keys...))
	if err != nil {
		return nil, err
	}

	urlProfile := newFloat32Matrix(len(urls), len(profiles))

	for i := range urls {
		for j := range profiles {
			pos := matrixPosition(len(profiles), i, j)

			wString := weightsString[pos]
			if wString == "" {
				continue
			}

			w, err := stringToFloat32(wString)
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

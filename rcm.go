package main

import (
	"encoding/json"
	"log"
	"math"
	"net/http"
	"os"
	"runtime"
	"strconv"
	"strings"

	"github.com/garyburd/redigo/redis"
	"github.com/gorilla/mux"
)

func max(numbers []float32) float32 {
	if len(numbers) == 0 {
		return 0
	}
	var max float64 = float64(numbers[0])

	for i := 1; i < len(numbers); i++ {
		max = math.Max(max, float64(numbers[i]))
	}
	return float32(max)
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

func transpose(mx [][]float32) (mxt [][]float32) {
	if len(mx) == 0 {
		return
	}
	mxt = newFloat32Matrix(len(mx), len(mx[0]))

	for i := range mx {
		for j := range mx[i] {
			mxt[i][j] = mx[j][i]
		}
	}
	return
}

//

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

//

type Similar interface {
	Similar(v []float32) float32
}

type Session []float32

func newSession(urlsVisited, urls []string) Session {
	urlPos := make(map[string]int)
	for i, url := range urls {
		urlPos[url] = i
	}

	s := make(Session, len(urls))
	for _, url := range urlsVisited {
		if pos, ok := urlPos[url]; ok {
			s[pos] = 1
		}
	}

	return s
}

func (self Session) Similar(v []float32) float32 {
	if len(self) != len(v) || len(self) == 0 {
		return 0
	}

	var s float32
	for i := range self {
		s += self[i] * v[i]
	}
	s /= float32(len(self))

	return s
}

//

type ServerState struct {
	urls              []string
	profiles          []string
	urlProfileWeights [][]float32
}

// TODO: add timeouts
func newServerState() (state *ServerState, err error) {
	conn, err := redis.Dial("tcp", "127.0.0.1:6379")
	if err != nil {
		return
	}
	defer conn.Close()

	urls, err := redis.Strings(conn.Do("SMEMBERS", "urls"))
	if err != nil {
		return
	}

	profiles, err := redis.Strings(conn.Do("SMEMBERS", "profiles"))
	if err != nil {
		return
	}

	urlProfileWeights, err := getWeights(conn, urls, profiles)
	if err != nil {
		return
	}

	state = new(ServerState)
	state.urls = urls
	state.profiles = profiles
	state.urlProfileWeights = urlProfileWeights

	return
}

type Request struct {
	Urls []string `json:"urls"`
}

type Response struct {
	Recommendation map[string]float32 `json:"recommendation"`
	Err            string             `json:"error"`

	state *ServerState
}

func countSimilarities(urlProfileWeights [][]float32, session Similar) (similarities []float32) {
	profileUrlWeights := transpose(urlProfileWeights)

	similarities = make([]float32, len(profileUrlWeights))
	for i, profile := range profileUrlWeights {
		s := session.Similar(profile)
		similarities[i] = s
	}
	return
}

// Mamdani inference
func (self *Response) countRecommendation(session Similar) (recommendation map[string]float32, err error) {
	similarities := countSimilarities(self.state.urlProfileWeights, session)

	recommendation = make(map[string]float32)
	weights := make([]float32, len(self.state.profiles))

	for i, url := range self.state.urls {
		for j := range self.state.profiles {
			min := float32(math.Min(float64(similarities[j]), float64(self.state.urlProfileWeights[i][j])))
			weights[j] = min
		}
		recommendation[url] = max(weights)
	}

	return
}

func (self *Response) SetRecommendation(urls []string) (err error) {
	session := newSession(urls, self.state.urls)
	self.Recommendation, err = self.countRecommendation(session)
	return
}

func recommendHandler(st *ServerState) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")

		decoder := json.NewDecoder(r.Body)
		encoder := json.NewEncoder(w)

		req := new(Request)
		resp := &Response{state: st}

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
}

func startRecommendationServer() error {
	state, err := newServerState()
	if err != nil {
		return err
	}

	r := mux.NewRouter()
	r.HandleFunc("/recommend", recommendHandler(state)).Methods("POST")
	http.Handle("/", r)

	return http.ListenAndServe(":"+os.Getenv("PORT"), nil)
}

func main() {
	err := startRecommendationServer()
	if err != nil {
		log.Fatal(err)
	}
}

func init() {
	runtime.GOMAXPROCS(runtime.NumCPU())
}

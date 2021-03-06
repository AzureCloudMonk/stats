package stats

import (
	"fmt"
	"net/http"
	"os"
	"sync"
	"time"
)

// Stats data structure
type Stats struct {
	mu                  sync.RWMutex
	closed              chan struct{}
	Uptime              time.Time
	Pid                 int
	ResponseCounts      map[string]int
	TotalResponseCounts map[string]int
	TotalResponseTime   time.Time
}

// New constructs a new Stats structure
func New() *Stats {
	stats := &Stats{
		closed:              make(chan struct{}, 1),
		Uptime:              time.Now(),
		Pid:                 os.Getpid(),
		ResponseCounts:      map[string]int{},
		TotalResponseCounts: map[string]int{},
		TotalResponseTime:   time.Time{},
	}

	go func() {
		for {
			select {
			case <-stats.closed:
				return
			default:
				stats.ResetResponseCounts()

				time.Sleep(time.Second * 1)
			}
		}
	}()

	return stats
}

func (mw *Stats) Close() {
	close(mw.closed)
}

// ResetResponseCounts reset the response counts
func (mw *Stats) ResetResponseCounts() {
	mw.mu.Lock()
	defer mw.mu.Unlock()
	mw.ResponseCounts = map[string]int{}
}

// Handler is a MiddlewareFunc makes Stats implement the Middleware interface.
func (mw *Stats) Handler(h http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		beginning, recorder := mw.Begin(w)

		h.ServeHTTP(recorder, r)

		mw.End(beginning, recorder)
	})
}

// Negroni compatible interface
func (mw *Stats) ServeHTTP(w http.ResponseWriter, r *http.Request, next http.HandlerFunc) {
	beginning, recorder := mw.Begin(w)

	next(recorder, r)

	mw.End(beginning, recorder)
}

// Begin starts a recorder
func (mw *Stats) Begin(w http.ResponseWriter) (time.Time, ResponseWriter) {
	start := time.Now()

	writer := NewRecorderResponseWriter(w, 200)

	return start, writer
}

// EndWithStatus closes the recorder with a specific status
func (mw *Stats) EndWithStatus(start time.Time, status int) {
	end := time.Now()

	responseTime := end.Sub(start)

	mw.mu.Lock()

	defer mw.mu.Unlock()

	statusCode := fmt.Sprintf("%d", status)

	mw.ResponseCounts[statusCode]++
	mw.TotalResponseCounts[statusCode]++
	mw.TotalResponseTime = mw.TotalResponseTime.Add(responseTime)
}

// End closes the recorder with the recorder status
func (mw *Stats) End(start time.Time, recorder ResponseWriter) {
	mw.EndWithStatus(start, recorder.Status())
}

// Data serializable structure
type Data struct {
	Pid                    int            `json:"pid"`
	UpTime                 string         `json:"uptime"`
	UpTimeSec              float64        `json:"uptime_sec"`
	Time                   string         `json:"time"`
	TimeUnix               int64          `json:"unixtime"`
	StatusCodeCount        map[string]int `json:"status_code_count"`
	TotalStatusCodeCount   map[string]int `json:"total_status_code_count"`
	Count                  int            `json:"count"`
	TotalCount             int            `json:"total_count"`
	TotalResponseTime      string         `json:"total_response_time"`
	TotalResponseTimeSec   float64        `json:"total_response_time_sec"`
	AverageResponseTime    string         `json:"average_response_time"`
	AverageResponseTimeSec float64        `json:"average_response_time_sec"`
}

// Data returns the data serializable structure
func (mw *Stats) Data() *Data {

	mw.mu.RLock()

	responseCounts := make(map[string]int, len(mw.ResponseCounts))
	totalResponseCounts := make(map[string]int, len(mw.TotalResponseCounts))

	now := time.Now()

	uptime := now.Sub(mw.Uptime)

	count := 0
	for code, current := range mw.ResponseCounts {
		responseCounts[code] = current
		count += current
	}

	totalCount := 0
	for code, count := range mw.TotalResponseCounts {
		totalResponseCounts[code] = count
		totalCount += count
	}

	totalResponseTime := mw.TotalResponseTime.Sub(time.Time{})

	averageResponseTime := time.Duration(0)
	if totalCount > 0 {
		avgNs := int64(totalResponseTime) / int64(totalCount)
		averageResponseTime = time.Duration(avgNs)
	}

	mw.mu.RUnlock()

	r := &Data{
		Pid:                    mw.Pid,
		UpTime:                 uptime.String(),
		UpTimeSec:              uptime.Seconds(),
		Time:                   now.String(),
		TimeUnix:               now.Unix(),
		StatusCodeCount:        responseCounts,
		TotalStatusCodeCount:   totalResponseCounts,
		Count:                  count,
		TotalCount:             totalCount,
		TotalResponseTime:      totalResponseTime.String(),
		TotalResponseTimeSec:   totalResponseTime.Seconds(),
		AverageResponseTime:    averageResponseTime.String(),
		AverageResponseTimeSec: averageResponseTime.Seconds(),
	}

	return r
}

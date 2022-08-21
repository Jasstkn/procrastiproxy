package main

import (
	"io"
	"net/http"
	"os"
	"time"

	log "github.com/sirupsen/logrus"
)

type (
	// struct for holding response details
	responseData struct {
		status int
		size   int
	}

	// our http.ResponseWriter implementation
	loggingResponseWriter struct {
		http.ResponseWriter // compose original http.ResponseWriter
		responseData        *responseData
	}
)

func (r *loggingResponseWriter) Write(b []byte) (int, error) {
	size, err := r.ResponseWriter.Write(b) // write response using original http.ResponseWriter
	r.responseData.size += size            // capture size
	return size, err
}

func (r *loggingResponseWriter) WriteHeader(statusCode int) {
	r.ResponseWriter.WriteHeader(statusCode) // write status code using original http.ResponseWriter
	r.responseData.status = statusCode       // capture status code
}

func WithLogging(h http.Handler) http.Handler {
	loggingFn := func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()

		responseData := &responseData{
			status: 0,
			size:   0,
		}
		lrw := loggingResponseWriter{
			ResponseWriter: w, // compose original http.ResponseWriter
			responseData:   responseData,
		}
		h.ServeHTTP(&lrw, r) // inject our implementation of http.ResponseWriter

		duration := time.Since(start).Nanoseconds()

		log.WithFields(log.Fields{
			"uri":         r.RequestURI,
			"method":      r.Method,
			"status":      responseData.status,
			"duration_ns": duration,
			"size":        responseData.size,
		}).Info("request completed")
	}
	return http.HandlerFunc(loggingFn)
}

func ProxyHandler() http.Handler {
	fn := func(w http.ResponseWriter, r *http.Request) {
		req, err := http.Get(r.RequestURI)
		if err != nil {
			log.WithFields(log.Fields{"url": r.RequestURI}).Warn("failed with error:", err)
		}
		defer req.Body.Close()
		body, _ := io.ReadAll(req.Body)
		log.WithFields(log.Fields{"body": body}).Debug("body was parsed")
		w.Write(body)
		w.WriteHeader(http.StatusOK)
	}
	return http.HandlerFunc(fn)
}

func init() {
	log.SetOutput(os.Stdout)
	log.SetFormatter(&log.JSONFormatter{})

	// allow to change LOG_LEVEL via environment variable
	logLevel, err := log.ParseLevel(os.Getenv("LOG_LEVEL"))
	if err != nil {
		logLevel = log.InfoLevel
	}
	log.SetLevel(logLevel)
}

func main() {
	http.Handle("/", WithLogging(ProxyHandler()))
	port := os.Getenv("PORT")
	if port == "" {
		port = "3000"
	}
	addr := "localhost:" + port
	log.WithField("addr", addr).Info("starting server")
	if err := http.ListenAndServe(addr, nil); err != nil {
		log.WithField("event", "start server").Fatal(err)
	}
}

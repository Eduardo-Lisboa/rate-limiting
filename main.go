package main

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"time"

	"github.com/redis/go-redis/v9"
)

type RateLimiter struct {
	client *redis.Client
	limit  int
	window time.Duration
	contex context.Context
}

func NewRateLimiter(client *redis.Client, limit int, window time.Duration) *RateLimiter {
	return &RateLimiter{
		client: client,
		limit:  limit,
		window: window,
		contex: context.Background(),
	}
}

func (rl *RateLimiter) Allow(key string) bool {
	pipe := rl.client.TxPipeline()

	inc := pipe.Incr(rl.contex, key)
	pipe.Expire(rl.contex, key, rl.window)

	_, err := pipe.Exec(rl.contex)
	if err != nil {
		return false
	}

	return inc.Val() <= int64(rl.limit)
}

func rateLimiterMiddleware(rl *RateLimiter, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ip, _, _ := net.SplitHostPort(r.RemoteAddr)
		if !rl.Allow(ip) {
			http.Error(w, "Too many requests", http.StatusTooManyRequests)
			return
		}

		next.ServeHTTP(w, r)
	})
}

func main() {
	client := redis.NewClient(&redis.Options{
		Addr: "localhost:6379",
	})

	defer client.Close()

	rateLimiter := NewRateLimiter(client, 10, 1*time.Minute)

	router := http.NewServeMux()
	router.HandleFunc("/api", func(w http.ResponseWriter, r *http.Request) {
		fmt.Println("Request received")
	})

	handeler := rateLimiterMiddleware(rateLimiter, router)

	err := http.ListenAndServe(":8080", handeler)
	if err != nil {
		fmt.Println("Error starting server:", err)
		return
	}
	fmt.Println("Server started on :8080")
}

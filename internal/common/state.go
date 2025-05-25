package common

import (
	"math/rand"
	"sync"
	"time"
)

type RNG struct {
	rnd *rand.Rand // ← обычное поле, а не embed
	mu  sync.Mutex
}

func NewRNG(seed int64) *RNG {
	if seed == 0 {
		seed = time.Now().UnixNano()
	}
	return &RNG{rnd: rand.New(rand.NewSource(seed)), mu: sync.Mutex{}}
}

/* потокобезопасные обёртки */

func (r *RNG) Float64() float64 {
	r.mu.Lock()
	v := r.rnd.Float64()
	r.mu.Unlock()
	return v
}
func (r *RNG) ExpFloat64() float64 {
	r.mu.Lock()
	v := r.rnd.ExpFloat64()
	r.mu.Unlock()
	return v
}
func (r *RNG) Int63() int64 {
	r.mu.Lock()
	v := r.rnd.Int63()
	r.mu.Unlock()
	return v
}
func (r *RNG) Int63n(n int64) int64 {
	r.mu.Lock()
	v := r.rnd.Int63n(n)
	r.mu.Unlock()
	return v
}
func (r *RNG) Intn(n int) int {
	r.mu.Lock()
	v := r.rnd.Intn(n)
	r.mu.Unlock()
	return v
}
func (r *RNG) Uint64() uint64 {
	r.mu.Lock()
	v := r.rnd.Uint64()
	r.mu.Unlock()
	return v
}
func (r *RNG) Seed(seed uint64) {
	r.mu.Lock()
	r.rnd.Seed(int64(seed))
	r.mu.Unlock()
}

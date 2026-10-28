package application

import (
	"container/list"
	"net/netip"
	"sort"
	"strings"
	"sync"
	"time"
)

// Rule limits one key (an email, an address, or both). Each attempt counts within the window.
// Up to Free attempts go through at once; after that each next one waits — 1s after the last,
// doubling with every further attempt, up to MaxDelay — and at Max attempts the key is refused
// until its oldest attempt leaves the window. Free 0 means no waiting, only Max.
type Rule struct {
	Key      string
	Max      int
	Free     int
	MaxDelay time.Duration
}

// Limiter slows down password guessing. In memory, per process: a restart forgets, and with
// several API replicas each counts on its own — the limits then multiply by the replica count.
//
// An attempt is counted before the password is checked (Reserve), so parallel requests cannot
// slip past a limit; the caller then hands back what a success should not keep (Undo, Forget).
//
// Keys are kept per kind — the text before the first ':' (ep, e, ip, net, …) — in a
// least-recently-used list of at most MaxKeys each. A flood of one kind (addresses from a large
// IPv6 range) can only push out keys of that kind, never an account's counter; every operation
// is O(1) apart from trimming one key's own attempts, so a flood cannot make it slow either.
type Limiter struct {
	Window  time.Duration
	MaxKeys int
	now     func() time.Time

	mu    sync.Mutex
	kinds map[string]*keyList
}

type keyList struct {
	order *list.List // front: used most recently
	keys  map[string]*list.Element
}

type keyHits struct {
	key  string
	hits []time.Time // oldest first
}

func NewLimiter(window time.Duration, maxKeys int) *Limiter {
	return &Limiter{Window: window, MaxKeys: maxKeys, now: time.Now, kinds: map[string]*keyList{}}
}

func kindOf(key string) string {
	kind, _, _ := strings.Cut(key, ":")
	return kind
}

func (l *Limiter) list(key string) *keyList {
	k := kindOf(key)
	kl := l.kinds[k]
	if kl == nil {
		kl = &keyList{order: list.New(), keys: map[string]*list.Element{}}
		l.kinds[k] = kl
	}
	return kl
}

// recent returns key's attempts still inside the window, dropping the older ones.
func (l *Limiter) recent(key string, now time.Time) []time.Time {
	kl := l.list(key)
	el, ok := kl.keys[key]
	if !ok {
		return nil
	}
	kh := el.Value.(*keyHits)
	cutoff := now.Add(-l.Window)
	i := sort.Search(len(kh.hits), func(i int) bool { return kh.hits[i].After(cutoff) })
	kh.hits = kh.hits[i:]
	if len(kh.hits) == 0 {
		kl.order.Remove(el)
		delete(kl.keys, key)
		return nil
	}
	return kh.hits
}

// delay is how long the attempt after n waiting ones waits: 1s, 2s, 4s, … up to max.
func delay(n int, max time.Duration) time.Duration {
	if n >= 30 {
		return max
	}
	d := time.Second << n
	if d > max {
		return max
	}
	return d
}

// wait is how long until rule's key may try again (0: now).
func (l *Limiter) wait(r Rule, hits []time.Time, now time.Time) time.Duration {
	if len(hits) >= r.Max {
		return hits[len(hits)-r.Max].Add(l.Window).Sub(now)
	}
	if r.Free > 0 && len(hits) >= r.Free {
		if until := hits[len(hits)-1].Add(delay(len(hits)-r.Free, r.MaxDelay)); until.After(now) {
			return until.Sub(now)
		}
	}
	return 0
}

// Reserve counts one attempt against every rule, or none: when any key has to wait it refuses
// and says how long the longest wait is.
func (l *Limiter) Reserve(rules ...Rule) (time.Duration, bool) {
	l.mu.Lock()
	defer l.mu.Unlock()
	now := l.now()
	var longest time.Duration
	for _, r := range rules {
		if w := l.wait(r, l.recent(r.Key, now), now); w > longest {
			longest = w
		}
	}
	if longest > 0 {
		return longest, false
	}
	for _, r := range rules {
		l.add(r.Key, now, rules)
	}
	return 0, true
}

// add records an attempt for key, making room in its kind by dropping the least recently used
// key that this call is not about.
func (l *Limiter) add(key string, now time.Time, inUse []Rule) {
	kl := l.list(key)
	if el, ok := kl.keys[key]; ok {
		kh := el.Value.(*keyHits)
		kh.hits = append(kh.hits, now)
		kl.order.MoveToFront(el)
		return
	}
	for len(kl.keys) >= l.MaxKeys {
		el := kl.order.Back()
		for el != nil && usedBy(el.Value.(*keyHits).key, inUse) {
			el = el.Prev()
		}
		if el == nil {
			break
		}
		kl.order.Remove(el)
		delete(kl.keys, el.Value.(*keyHits).key)
	}
	kl.keys[key] = kl.order.PushFront(&keyHits{key: key, hits: []time.Time{now}})
}

func usedBy(key string, rules []Rule) bool {
	for _, r := range rules {
		if r.Key == key {
			return true
		}
	}
	return false
}

// Undo takes back the latest attempt of each key — a sign-in that succeeded is not held against
// the address it came from. An empty key is skipped.
func (l *Limiter) Undo(keys ...string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	for _, key := range keys {
		if key == "" {
			continue
		}
		kl := l.list(key)
		if el, ok := kl.keys[key]; ok {
			kh := el.Value.(*keyHits)
			if kh.hits = kh.hits[:len(kh.hits)-1]; len(kh.hits) == 0 {
				kl.order.Remove(el)
				delete(kl.keys, key)
			}
		}
	}
}

// Forget clears keys, after a successful sign-in.
func (l *Limiter) Forget(keys ...string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	for _, key := range keys {
		kl := l.list(key)
		if el, ok := kl.keys[key]; ok {
			kl.order.Remove(el)
			delete(kl.keys, key)
		}
	}
}

// ClientKey is the address a limit counts against: an IPv4 address, or an IPv6 /64 — one
// subscriber gets a whole /64, so counting single IPv6 addresses would limit nothing.
func ClientKey(ip string) string {
	a, err := netip.ParseAddr(ip)
	if err != nil || a.Is4() || a.Is4In6() {
		return ip
	}
	p, err := a.Prefix(64)
	if err != nil {
		return ip
	}
	return p.String()
}

// NetKey is the IPv6 /48 an address belongs to — one site's allocation, so an attacker cycling
// through its 65 536 /64s still shares one counter — or "" for IPv4, where a /24 is often a
// carrier's shared NAT and counting it would limit innocent people.
func NetKey(ip string) string {
	a, err := netip.ParseAddr(ip)
	if err != nil || a.Is4() || a.Is4In6() {
		return ""
	}
	p, err := a.Prefix(48)
	if err != nil {
		return ""
	}
	return p.String()
}

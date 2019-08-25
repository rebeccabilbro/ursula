package fetcher

import (
	"time"
)

// An Item is a stripped-down RSS feed item
type Item struct {
	Title   string
	Channel string
	GUID    string
}

// Fetcher fetches Items and returns the time when the next fetch
// should be attempted. On failure, Fetch returns a non-nil error
type Fetcher interface {
	Fetch() (items []Item, next time.Time, err error)
}

// func Fetch(domain string) Fetcher {...} // fetches Items from domain

// A Subscription delivers Items over an Updates channel.
// Close cancels the subscription, closes the channel, and
// returns the last fetch error if any
type Subscription interface {
	Updates() <-chan Item // stream of Items
	Close() error         // shuts down the stream, e.g. cancel subscription
}

// sub implements the Subscription interface
type sub struct {
	fetcher Fetcher         // fetches items
	updates chan Item       // delivers items to the user
	closing chan chan error // for Close
}

// Subscribe converts Fetches to a stream
func Subscribe(fetcher Fetcher) Subscription {
	s := &sub{
		fetcher: fetcher,
		updates: make(chan Item),       // for Updates
		closing: make(chan chan error), // for Close
	}
	go s.loop()
	return s
}

//loopCloseOnly is a version of loop that includes only the logic that handles Close
func (s *sub) loopCloseOnly() {
	var err error // set when Fetch fails
	for {
		select {
		case errc := <-s.closing:
			errc <- err
			close(s.updates)
			return
		}
	}
}

// loop periodically fetches Items, sends them on s.updates, and exits
// when Close is called. It extends dedupeLoop with logic to run
// Fetch asynchronously.
func (s *sub) loop() {
	const maxPending = 10
	type fetchResult struct {
		fetched []Item
		next    time.Time
		err     error
	}
	var fetchDone chan fetchResult   // if non-nil, Fetch is running
	var pending []Item               // appended by fetch; consumed by send
	var next time.Time               // initially January 1, year 0
	var err error                    // error from a failed fetch
	var seen = make(map[string]bool) // set of item.GUIDs so we don't re-fetch same item

	for {
		var fetchDelay time.Duration // initially 0 (no delay)
		if now := time.Now(); next.After(now) {
			fetchDelay = next.Sub(now)
		}
		var startFetch <-chan time.Time
		if fetchDone == nil && len(pending) < maxPending {
			startFetch = time.After(fetchDelay) // enable fetch case
		}
		var first Item
		var updates chan Item
		if len(pending) > 0 {
			first = pending[0]
			updates = s.updates //enable send case
		}
		select {
		case <-startFetch:
			fetchDone = make(chan fetchResult, 1)
			go func() {
				fetched, next, err := s.fetcher.Fetch()
				fetchDone <- fetchResult{fetched, next, err}
			}()
		case result := <-fetchDone:
			fetchDone = nil
			fetched := result.fetched
			next, err = result.next, result.err
			if err != nil {
				next = time.Now().Add(10 * time.Second)
				break
			}
			for _, item := range fetched {
				if id := item.GUID; !seen[id] {
					pending = append(pending, item)
					seen[id] = true
				}
			}
		case errc := <-s.closing:
			errc <- err
			close(s.updates)
			return
		case updates <- first:
			pending = pending[1:]
		}
	}
}

// Updates is a receive-only stream of Items
// required for implementing Subscription interface
func (s *sub) Updates() <-chan Item {
	return s.updates
}

// Close closes the stream, effectively cancelling the subscription
// required for implementing Subscription interface
func (s *sub) Close() error {
	errc := make(chan error)
	s.closing <- errc
	return <-errc
}

// func Merge(subs ...Subscription) Subscription {...} // merges several streams

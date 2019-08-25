package main

import (
	"fmt"
	"time"

	"github.com/rebeccabilbro/ursula/fetcher"
)

func main() {
	// Subscribe to some feeds, and create a merged update stream
	merged := fetcher.Merge(
		fetcher.Subscribe(fetcher.Fetch("thelocal.es/international")),
		fetcher.Subscribe(fetcher.Fetch("romereports.com")),
		fetcher.Subscribe(fetcher.Fetch("naplesnews.com")),
	)

	// Close the subscriptions after some time
	time.AfterFunc(3*time.Second, func() {
		fmt.Println("Closed:", merged.Close())
	})

	// Print the stream
	for itm := range merged.Updates() {
		fmt.Println(itm.Channel, itm.Title)
	}

	panic("Something went wrong; here are the stacks: ")
}

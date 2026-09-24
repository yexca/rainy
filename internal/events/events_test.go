package events

import (
	"encoding/json"
	"sync"
	"testing"
)

func TestBus(t *testing.T) {
	b := NewBus()
	ch1, cancel1 := b.Subscribe()
	ch2, cancel2 := b.Subscribe()
	if b.Subscribers() != 2 {
		t.Fatal(b.Subscribers())
	}
	b.Publish(Library("scan"))
	for _, ch := range []<-chan Event{ch1, ch2} {
		e := <-ch
		if e.Type != TypeLibrary || e.Data.(LibraryData).Reason != "scan" {
			t.Fatalf("%+v", e)
		}
	}
	js, _ := json.Marshal(Library("tags"))
	if string(js) != `{"type":"library","data":{"reason":"tags"}}` {
		t.Fatal(string(js))
	}
	cancel1()
	cancel1() // idempotent
	if _, ok := <-ch1; ok {
		t.Fatal("channel must be closed")
	}
	// Slow subscriber: publishing never blocks, extra events are dropped.
	for i := 0; i < SubscriberBuffer*3; i++ {
		b.Publish(Event{Type: TypeScan, Data: i})
	}
	if len(ch2) != SubscriberBuffer {
		t.Fatalf("buffered %d", len(ch2))
	}
	cancel2()
	if b.Subscribers() != 0 {
		t.Fatal("not unsubscribed")
	}
	b.Publish(Event{Type: TypeScan}) // no subscribers: no panic
}

func TestBusConcurrent(t *testing.T) {
	b := NewBus()
	var wg sync.WaitGroup
	for i := 0; i < 20; i++ {
		wg.Add(2)
		go func() {
			defer wg.Done()
			ch, cancel := b.Subscribe()
			defer cancel()
			for j := 0; j < 10; j++ {
				select {
				case <-ch:
				default:
				}
			}
		}()
		go func() {
			defer wg.Done()
			for j := 0; j < 50; j++ {
				b.Publish(Event{Type: TypeScan})
			}
		}()
	}
	wg.Wait()
}

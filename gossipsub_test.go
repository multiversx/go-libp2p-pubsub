package pubsub

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"math/rand"
	"testing"
	"time"

	pb "github.com/libp2p/go-libp2p-pubsub/pb"

	"github.com/libp2p/go-libp2p-core/host"
	"github.com/libp2p/go-libp2p-core/network"
	"github.com/libp2p/go-libp2p-core/peer"
	"github.com/libp2p/go-libp2p-core/peerstore"

	bhost "github.com/libp2p/go-libp2p-blankhost"
	swarmt "github.com/libp2p/go-libp2p-swarm/testing"

	ggio "github.com/gogo/protobuf/io"
)

func getGossipsub(ctx context.Context, h host.Host, opts ...Option) *PubSub {
	ps, err := NewGossipSub(ctx, h, opts...)
	if err != nil {
		panic(err)
	}
	return ps
}

func getGossipsubs(ctx context.Context, hs []host.Host, opts ...Option) []*PubSub {
	var psubs []*PubSub
	for _, h := range hs {
		psubs = append(psubs, getGossipsub(ctx, h, opts...))
	}
	return psubs
}

func TestSparseGossipsub(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	hosts := getNetHosts(t, ctx, 20)

	psubs := getGossipsubs(ctx, hosts)

	var msgs []*Subscription
	for _, ps := range psubs {
		subch, err := ps.Subscribe("foobar")
		if err != nil {
			t.Fatal(err)
		}

		msgs = append(msgs, subch)
	}

	sparseConnect(t, hosts)

	// wait for heartbeats to build mesh
	time.Sleep(time.Second * 2)

	for i := 0; i < 100; i++ {
		msg := []byte(fmt.Sprintf("%d it's not a floooooood %d", i, i))

		owner := rand.Intn(len(psubs))

		psubs[owner].Publish("foobar", msg)

		for _, sub := range msgs {
			got, err := sub.Next(ctx)
			if err != nil {
				t.Fatal(sub.err)
			}
			if !bytes.Equal(msg, got.Data) {
				t.Fatal("got wrong message!")
			}
		}
	}
}

func TestDenseGossipsub(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	hosts := getNetHosts(t, ctx, 20)

	psubs := getGossipsubs(ctx, hosts)

	var msgs []*Subscription
	for _, ps := range psubs {
		subch, err := ps.Subscribe("foobar")
		if err != nil {
			t.Fatal(err)
		}

		msgs = append(msgs, subch)
	}

	denseConnect(t, hosts)

	// wait for heartbeats to build mesh
	time.Sleep(time.Second * 2)

	for i := 0; i < 100; i++ {
		msg := []byte(fmt.Sprintf("%d it's not a floooooood %d", i, i))

		owner := rand.Intn(len(psubs))

		psubs[owner].Publish("foobar", msg)

		for _, sub := range msgs {
			got, err := sub.Next(ctx)
			if err != nil {
				t.Fatal(sub.err)
			}
			if !bytes.Equal(msg, got.Data) {
				t.Fatal("got wrong message!")
			}
		}
	}
}

func TestGossipsubFanout(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	hosts := getNetHosts(t, ctx, 20)

	psubs := getGossipsubs(ctx, hosts)

	var msgs []*Subscription
	for _, ps := range psubs[1:] {
		subch, err := ps.Subscribe("foobar")
		if err != nil {
			t.Fatal(err)
		}

		msgs = append(msgs, subch)
	}

	denseConnect(t, hosts)

	// wait for heartbeats to build mesh
	time.Sleep(time.Second * 2)

	for i := 0; i < 100; i++ {
		msg := []byte(fmt.Sprintf("%d it's not a floooooood %d", i, i))

		owner := 0

		psubs[owner].Publish("foobar", msg)

		for _, sub := range msgs {
			got, err := sub.Next(ctx)
			if err != nil {
				t.Fatal(sub.err)
			}
			if !bytes.Equal(msg, got.Data) {
				t.Fatal("got wrong message!")
			}
		}
	}

	// subscribe the owner
	subch, err := psubs[0].Subscribe("foobar")
	if err != nil {
		t.Fatal(err)
	}
	msgs = append(msgs, subch)

	// wait for a heartbeat
	time.Sleep(time.Second * 1)

	for i := 0; i < 100; i++ {
		msg := []byte(fmt.Sprintf("%d it's not a floooooood %d", i, i))

		owner := 0

		psubs[owner].Publish("foobar", msg)

		for _, sub := range msgs {
			got, err := sub.Next(ctx)
			if err != nil {
				t.Fatal(sub.err)
			}
			if !bytes.Equal(msg, got.Data) {
				t.Fatal("got wrong message!")
			}
		}
	}
}

func TestGossipsubFanoutMaintenance(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	hosts := getNetHosts(t, ctx, 20)

	psubs := getGossipsubs(ctx, hosts)

	var msgs []*Subscription
	for _, ps := range psubs[1:] {
		subch, err := ps.Subscribe("foobar")
		if err != nil {
			t.Fatal(err)
		}

		msgs = append(msgs, subch)
	}

	denseConnect(t, hosts)

	// wait for heartbeats to build mesh
	time.Sleep(time.Second * 2)

	for i := 0; i < 100; i++ {
		msg := []byte(fmt.Sprintf("%d it's not a floooooood %d", i, i))

		owner := 0

		psubs[owner].Publish("foobar", msg)

		for _, sub := range msgs {
			got, err := sub.Next(ctx)
			if err != nil {
				t.Fatal(sub.err)
			}
			if !bytes.Equal(msg, got.Data) {
				t.Fatal("got wrong message!")
			}
		}
	}

	// unsubscribe all peers to exercise fanout maintenance
	for _, sub := range msgs {
		sub.Cancel()
	}
	msgs = nil

	// wait for heartbeats
	time.Sleep(time.Second * 2)

	// resubscribe and repeat
	for _, ps := range psubs[1:] {
		subch, err := ps.Subscribe("foobar")
		if err != nil {
			t.Fatal(err)
		}

		msgs = append(msgs, subch)
	}

	time.Sleep(time.Second * 2)

	for i := 0; i < 100; i++ {
		msg := []byte(fmt.Sprintf("%d it's not a floooooood %d", i, i))

		owner := 0

		psubs[owner].Publish("foobar", msg)

		for _, sub := range msgs {
			got, err := sub.Next(ctx)
			if err != nil {
				t.Fatal(sub.err)
			}
			if !bytes.Equal(msg, got.Data) {
				t.Fatal("got wrong message!")
			}
		}
	}
}

func TestGossipsubFanoutExpiry(t *testing.T) {
	GossipSubFanoutTTL = 1 * time.Second
	defer func() { GossipSubFanoutTTL = 60 * time.Second }()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	hosts := getNetHosts(t, ctx, 10)

	psubs := getGossipsubs(ctx, hosts)

	var msgs []*Subscription
	for _, ps := range psubs[1:] {
		subch, err := ps.Subscribe("foobar")
		if err != nil {
			t.Fatal(err)
		}

		msgs = append(msgs, subch)
	}

	denseConnect(t, hosts)

	// wait for heartbeats to build mesh
	time.Sleep(time.Second * 2)

	for i := 0; i < 5; i++ {
		msg := []byte(fmt.Sprintf("%d it's not a floooooood %d", i, i))

		owner := 0

		psubs[owner].Publish("foobar", msg)

		for _, sub := range msgs {
			got, err := sub.Next(ctx)
			if err != nil {
				t.Fatal(sub.err)
			}
			if !bytes.Equal(msg, got.Data) {
				t.Fatal("got wrong message!")
			}
		}
	}

	psubs[0].eval <- func() {
		if len(psubs[0].rt.(*GossipSubRouter).fanout) == 0 {
			t.Fatal("owner has no fanout")
		}
	}

	// wait for TTL to expire fanout peers in owner
	time.Sleep(time.Second * 2)

	psubs[0].eval <- func() {
		if len(psubs[0].rt.(*GossipSubRouter).fanout) > 0 {
			t.Fatal("fanout hasn't expired")
		}
	}

	// wait for it to run in the event loop
	time.Sleep(10 * time.Millisecond)
}

func TestGossipsubGossip(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	hosts := getNetHosts(t, ctx, 20)

	psubs := getGossipsubs(ctx, hosts)

	var msgs []*Subscription
	for _, ps := range psubs {
		subch, err := ps.Subscribe("foobar")
		if err != nil {
			t.Fatal(err)
		}

		msgs = append(msgs, subch)
	}

	denseConnect(t, hosts)

	// wait for heartbeats to build mesh
	time.Sleep(time.Second * 2)

	for i := 0; i < 100; i++ {
		msg := []byte(fmt.Sprintf("%d it's not a floooooood %d", i, i))

		owner := rand.Intn(len(psubs))

		psubs[owner].Publish("foobar", msg)

		for _, sub := range msgs {
			got, err := sub.Next(ctx)
			if err != nil {
				t.Fatal(sub.err)
			}
			if !bytes.Equal(msg, got.Data) {
				t.Fatal("got wrong message!")
			}
		}

		// wait a bit to have some gossip interleaved
		time.Sleep(time.Millisecond * 100)
	}

	// and wait for some gossip flushing
	time.Sleep(time.Second * 2)
}

func TestGossipsubGossipPiggyback(t *testing.T) {
	t.Skip("test no longer relevant; gossip propagation has become eager")
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	hosts := getNetHosts(t, ctx, 20)

	psubs := getGossipsubs(ctx, hosts)

	var msgs []*Subscription
	for _, ps := range psubs {
		subch, err := ps.Subscribe("foobar")
		if err != nil {
			t.Fatal(err)
		}

		msgs = append(msgs, subch)
	}

	var xmsgs []*Subscription
	for _, ps := range psubs {
		subch, err := ps.Subscribe("bazcrux")
		if err != nil {
			t.Fatal(err)
		}

		xmsgs = append(xmsgs, subch)
	}

	denseConnect(t, hosts)

	// wait for heartbeats to build mesh
	time.Sleep(time.Second * 2)

	for i := 0; i < 100; i++ {
		msg := []byte(fmt.Sprintf("%d it's not a floooooood %d", i, i))

		owner := rand.Intn(len(psubs))

		psubs[owner].Publish("foobar", msg)
		psubs[owner].Publish("bazcrux", msg)

		for _, sub := range msgs {
			got, err := sub.Next(ctx)
			if err != nil {
				t.Fatal(sub.err)
			}
			if !bytes.Equal(msg, got.Data) {
				t.Fatal("got wrong message!")
			}
		}

		for _, sub := range xmsgs {
			got, err := sub.Next(ctx)
			if err != nil {
				t.Fatal(sub.err)
			}
			if !bytes.Equal(msg, got.Data) {
				t.Fatal("got wrong message!")
			}
		}

		// wait a bit to have some gossip interleaved
		time.Sleep(time.Millisecond * 100)
	}

	// and wait for some gossip flushing
	time.Sleep(time.Second * 2)
}

func TestGossipsubGossipPropagation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	hosts := getNetHosts(t, ctx, 20)
	psubs := getGossipsubs(ctx, hosts)

	hosts1 := hosts[:GossipSubD+1]
	hosts2 := append(hosts[GossipSubD+1:], hosts[0])

	denseConnect(t, hosts1)
	denseConnect(t, hosts2)

	var msgs1 []*Subscription
	for _, ps := range psubs[1 : GossipSubD+1] {
		subch, err := ps.Subscribe("foobar")
		if err != nil {
			t.Fatal(err)
		}

		msgs1 = append(msgs1, subch)
	}

	time.Sleep(time.Second * 1)

	for i := 0; i < 10; i++ {
		msg := []byte(fmt.Sprintf("%d it's not a floooooood %d", i, i))

		owner := 0

		psubs[owner].Publish("foobar", msg)

		for _, sub := range msgs1 {
			got, err := sub.Next(ctx)
			if err != nil {
				t.Fatal(sub.err)
			}
			if !bytes.Equal(msg, got.Data) {
				t.Fatal("got wrong message!")
			}
		}
	}

	time.Sleep(time.Millisecond * 100)

	var msgs2 []*Subscription
	for _, ps := range psubs[GossipSubD+1:] {
		subch, err := ps.Subscribe("foobar")
		if err != nil {
			t.Fatal(err)
		}

		msgs2 = append(msgs2, subch)
	}

	var collect [][]byte
	for i := 0; i < 10; i++ {
		for _, sub := range msgs2 {
			got, err := sub.Next(ctx)
			if err != nil {
				t.Fatal(sub.err)
			}
			collect = append(collect, got.Data)
		}
	}

	for i := 0; i < 10; i++ {
		msg := []byte(fmt.Sprintf("%d it's not a floooooood %d", i, i))
		gotit := false
		for j := 0; j < len(collect); j++ {
			if bytes.Equal(msg, collect[j]) {
				gotit = true
				break
			}
		}
		if !gotit {
			t.Fatalf("Didn't get message %s", string(msg))
		}
	}
}

func TestGossipsubPrune(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	hosts := getNetHosts(t, ctx, 20)

	psubs := getGossipsubs(ctx, hosts)

	var msgs []*Subscription
	for _, ps := range psubs {
		subch, err := ps.Subscribe("foobar")
		if err != nil {
			t.Fatal(err)
		}

		msgs = append(msgs, subch)
	}

	denseConnect(t, hosts)

	// wait for heartbeats to build mesh
	time.Sleep(time.Second * 2)

	// disconnect some peers from the mesh to get some PRUNEs
	for _, sub := range msgs[:5] {
		sub.Cancel()
	}

	// wait a bit to take effect
	time.Sleep(time.Millisecond * 100)

	for i := 0; i < 10; i++ {
		msg := []byte(fmt.Sprintf("%d it's not a floooooood %d", i, i))

		owner := rand.Intn(len(psubs))

		psubs[owner].Publish("foobar", msg)

		for _, sub := range msgs[5:] {
			got, err := sub.Next(ctx)
			if err != nil {
				t.Fatal(sub.err)
			}
			if !bytes.Equal(msg, got.Data) {
				t.Fatal("got wrong message!")
			}
		}
	}
}

func TestGossipsubGraft(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	hosts := getNetHosts(t, ctx, 20)

	psubs := getGossipsubs(ctx, hosts)

	sparseConnect(t, hosts)

	time.Sleep(time.Second * 1)

	var msgs []*Subscription
	for _, ps := range psubs {
		subch, err := ps.Subscribe("foobar")
		if err != nil {
			t.Fatal(err)
		}

		msgs = append(msgs, subch)

		// wait for announce to propagate
		time.Sleep(time.Millisecond * 100)
	}

	time.Sleep(time.Second * 1)

	for i := 0; i < 100; i++ {
		msg := []byte(fmt.Sprintf("%d it's not a floooooood %d", i, i))

		owner := rand.Intn(len(psubs))

		psubs[owner].Publish("foobar", msg)

		for _, sub := range msgs {
			got, err := sub.Next(ctx)
			if err != nil {
				t.Fatal(sub.err)
			}
			if !bytes.Equal(msg, got.Data) {
				t.Fatal("got wrong message!")
			}
		}
	}
}

func TestGossipsubRemovePeer(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	hosts := getNetHosts(t, ctx, 20)

	psubs := getGossipsubs(ctx, hosts)

	var msgs []*Subscription
	for _, ps := range psubs {
		subch, err := ps.Subscribe("foobar")
		if err != nil {
			t.Fatal(err)
		}

		msgs = append(msgs, subch)
	}

	denseConnect(t, hosts)

	// wait for heartbeats to build mesh
	time.Sleep(time.Second * 2)

	// disconnect some peers to exercise RemovePeer paths
	for _, host := range hosts[:5] {
		host.Close()
	}

	// wait a heartbeat
	time.Sleep(time.Second * 1)

	for i := 0; i < 10; i++ {
		msg := []byte(fmt.Sprintf("%d it's not a floooooood %d", i, i))

		owner := 5 + rand.Intn(len(psubs)-5)

		psubs[owner].Publish("foobar", msg)

		for _, sub := range msgs[5:] {
			got, err := sub.Next(ctx)
			if err != nil {
				t.Fatal(sub.err)
			}
			if !bytes.Equal(msg, got.Data) {
				t.Fatal("got wrong message!")
			}
		}
	}
}

func TestGossipsubGraftPruneRetry(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	hosts := getNetHosts(t, ctx, 10)
	psubs := getGossipsubs(ctx, hosts)
	denseConnect(t, hosts)

	var topics []string
	var msgs [][]*Subscription
	for i := 0; i < 35; i++ {
		topic := fmt.Sprintf("topic%d", i)
		topics = append(topics, topic)

		var subs []*Subscription
		for _, ps := range psubs {
			subch, err := ps.Subscribe(topic)
			if err != nil {
				t.Fatal(err)
			}

			subs = append(subs, subch)
		}
		msgs = append(msgs, subs)
	}

	// wait for heartbeats to build meshes
	time.Sleep(time.Second * 5)

	for i, topic := range topics {
		msg := []byte(fmt.Sprintf("%d it's not a floooooood %d", i, i))

		owner := rand.Intn(len(psubs))

		psubs[owner].Publish(topic, msg)

		for _, sub := range msgs[i] {
			got, err := sub.Next(ctx)
			if err != nil {
				t.Fatal(sub.err)
			}
			if !bytes.Equal(msg, got.Data) {
				t.Fatal("got wrong message!")
			}
		}
	}
}

func TestGossipsubControlPiggyback(t *testing.T) {
	t.Skip("travis regularly fails on this test")

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	hosts := getNetHosts(t, ctx, 10)
	psubs := getGossipsubs(ctx, hosts)
	denseConnect(t, hosts)

	for _, ps := range psubs {
		subch, err := ps.Subscribe("flood")
		if err != nil {
			t.Fatal(err)
		}
		go func(sub *Subscription) {
			for {
				_, err := sub.Next(ctx)
				if err != nil {
					break
				}
			}
		}(subch)
	}

	time.Sleep(time.Second * 1)

	// create a background flood of messages that overloads the queues
	done := make(chan struct{})
	go func() {
		owner := rand.Intn(len(psubs))
		for i := 0; i < 10000; i++ {
			msg := []byte("background flooooood")
			psubs[owner].Publish("flood", msg)
		}
		done <- struct{}{}
	}()

	time.Sleep(time.Millisecond * 20)

	// and subscribe to a bunch of topics in the meantime -- this should
	// result in some dropped control messages, with subsequent piggybacking
	// in the background flood
	var topics []string
	var msgs [][]*Subscription
	for i := 0; i < 5; i++ {
		topic := fmt.Sprintf("topic%d", i)
		topics = append(topics, topic)

		var subs []*Subscription
		for _, ps := range psubs {
			subch, err := ps.Subscribe(topic)
			if err != nil {
				t.Fatal(err)
			}

			subs = append(subs, subch)
		}
		msgs = append(msgs, subs)
	}

	// wait for the flood to stop
	<-done

	// and test that we have functional overlays
	for i, topic := range topics {
		msg := []byte(fmt.Sprintf("%d it's not a floooooood %d", i, i))

		owner := rand.Intn(len(psubs))

		psubs[owner].Publish(topic, msg)

		for _, sub := range msgs[i] {
			got, err := sub.Next(ctx)
			if err != nil {
				t.Fatal(sub.err)
			}
			if !bytes.Equal(msg, got.Data) {
				t.Fatal("got wrong message!")
			}
		}
	}
}

func TestMixedGossipsub(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	hosts := getNetHosts(t, ctx, 30)

	gsubs := getGossipsubs(ctx, hosts[:20])
	fsubs := getPubsubs(ctx, hosts[20:])
	psubs := append(gsubs, fsubs...)

	var msgs []*Subscription
	for _, ps := range psubs {
		subch, err := ps.Subscribe("foobar")
		if err != nil {
			t.Fatal(err)
		}

		msgs = append(msgs, subch)
	}

	sparseConnect(t, hosts)

	// wait for heartbeats to build mesh
	time.Sleep(time.Second * 2)

	for i := 0; i < 100; i++ {
		msg := []byte(fmt.Sprintf("%d it's not a floooooood %d", i, i))

		owner := rand.Intn(len(psubs))

		psubs[owner].Publish("foobar", msg)

		for _, sub := range msgs {
			got, err := sub.Next(ctx)
			if err != nil {
				t.Fatal(sub.err)
			}
			if !bytes.Equal(msg, got.Data) {
				t.Fatal("got wrong message!")
			}
		}
	}
}

func TestGossipsubMultihops(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	hosts := getNetHosts(t, ctx, 6)

	psubs := getGossipsubs(ctx, hosts)

	connect(t, hosts[0], hosts[1])
	connect(t, hosts[1], hosts[2])
	connect(t, hosts[2], hosts[3])
	connect(t, hosts[3], hosts[4])
	connect(t, hosts[4], hosts[5])

	var subs []*Subscription
	for i := 1; i < 6; i++ {
		ch, err := psubs[i].Subscribe("foobar")
		if err != nil {
			t.Fatal(err)
		}
		subs = append(subs, ch)
	}

	// wait for heartbeats to build mesh
	time.Sleep(time.Second * 2)

	msg := []byte("i like cats")
	err := psubs[0].Publish("foobar", msg)
	if err != nil {
		t.Fatal(err)
	}

	// last node in the chain should get the message
	select {
	case out := <-subs[4].ch:
		if !bytes.Equal(out.GetData(), msg) {
			t.Fatal("got wrong data")
		}
	case <-time.After(time.Second * 5):
		t.Fatal("timed out waiting for message")
	}
}

func TestGossipsubTreeTopology(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	hosts := getNetHosts(t, ctx, 10)
	psubs := getGossipsubs(ctx, hosts)

	connect(t, hosts[0], hosts[1])
	connect(t, hosts[1], hosts[2])
	connect(t, hosts[1], hosts[4])
	connect(t, hosts[2], hosts[3])
	connect(t, hosts[0], hosts[5])
	connect(t, hosts[5], hosts[6])
	connect(t, hosts[5], hosts[8])
	connect(t, hosts[6], hosts[7])
	connect(t, hosts[8], hosts[9])

	/*
		[0] -> [1] -> [2] -> [3]
		 |      L->[4]
		 v
		[5] -> [6] -> [7]
		 |
		 v
		[8] -> [9]
	*/

	var chs []*Subscription
	for _, ps := range psubs {
		ch, err := ps.Subscribe("fizzbuzz")
		if err != nil {
			t.Fatal(err)
		}

		chs = append(chs, ch)
	}

	// wait for heartbeats to build mesh
	time.Sleep(time.Second * 2)

	assertPeerLists(t, hosts, psubs[0], 1, 5)
	assertPeerLists(t, hosts, psubs[1], 0, 2, 4)
	assertPeerLists(t, hosts, psubs[2], 1, 3)

	checkMessageRouting(t, "fizzbuzz", []*PubSub{psubs[9], psubs[3]}, chs)
}

// this tests overlay bootstrapping through px in Gossipsub v1.1
// we start with a star topology and rely on px through prune to build the mesh
func TestGossipsubStarTopology(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	hosts := getNetHosts(t, ctx, 20)
	psubs := getGossipsubs(ctx, hosts, WithPeerExchange(true))

	// add all peer addresses to the peerstores
	// this is necessary because we can't have signed address records witout identify
	// pushing them
	for i := range hosts {
		for j := range hosts {
			if i == j {
				continue
			}
			hosts[i].Peerstore().AddAddrs(hosts[j].ID(), hosts[j].Addrs(), peerstore.PermanentAddrTTL)
		}
	}

	// build the star
	for i := 1; i < 20; i++ {
		connect(t, hosts[0], hosts[i])
	}

	// build the mesh
	var subs []*Subscription
	for _, ps := range psubs {
		sub, err := ps.Subscribe("test")
		if err != nil {
			t.Fatal(err)
		}
		subs = append(subs, sub)
	}

	// wait a bit for the mesh to build
	time.Sleep(10 * time.Second)

	// check that all peers have > 1 connection
	for _, h := range hosts {
		if len(h.Network().Conns()) == 1 {
			t.Error("peer has ony a single connection")
		}
	}

	// send a message from each peer and assert it was propagated
	for i := 0; i < 20; i++ {
		msg := []byte(fmt.Sprintf("message %d", i))
		psubs[i].Publish("test", msg)

		for _, sub := range subs {
			assertReceive(t, sub, msg)
		}
	}
}

func TestGossipsubDirectPeers(t *testing.T) {
	originalGossipSubDirectConnectTicks := GossipSubDirectConnectTicks
	GossipSubDirectConnectTicks = 2
	defer func() {
		GossipSubDirectConnectTicks = originalGossipSubDirectConnectTicks
	}()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	h := getNetHosts(t, ctx, 3)
	psubs := []*PubSub{
		getGossipsub(ctx, h[0]),
		getGossipsub(ctx, h[1], WithDirectPeers([]peer.AddrInfo{peer.AddrInfo{h[2].ID(), h[2].Addrs()}})),
		getGossipsub(ctx, h[2], WithDirectPeers([]peer.AddrInfo{peer.AddrInfo{h[1].ID(), h[1].Addrs()}})),
	}

	connect(t, h[0], h[1])
	connect(t, h[0], h[2])

	// verify that the direct peers connected
	time.Sleep(2 * time.Second)
	if len(h[1].Network().ConnsToPeer(h[2].ID())) == 0 {
		t.Fatal("expected a connection between direct peers")
	}

	// build the mesh
	var subs []*Subscription
	for _, ps := range psubs {
		sub, err := ps.Subscribe("test")
		if err != nil {
			t.Fatal(err)
		}
		subs = append(subs, sub)
	}

	time.Sleep(time.Second)

	// publish some messages
	for i := 0; i < 3; i++ {
		msg := []byte(fmt.Sprintf("message %d", i))
		psubs[i].Publish("test", msg)

		for _, sub := range subs {
			assertReceive(t, sub, msg)
		}
	}

	// disconnect the direct peers to test reconnection
	for _, c := range h[1].Network().ConnsToPeer(h[2].ID()) {
		c.Close()
	}

	time.Sleep(3 * time.Second)

	if len(h[1].Network().ConnsToPeer(h[2].ID())) == 0 {
		t.Fatal("expected a connection between direct peers")
	}

	// publish some messages
	for i := 0; i < 3; i++ {
		msg := []byte(fmt.Sprintf("message %d", i))
		psubs[i].Publish("test", msg)

		for _, sub := range subs {
			assertReceive(t, sub, msg)
		}
	}
}

func TestGossipsubFloodPublish(t *testing.T) {
	// uses a star topology without PX and publishes from the star to verify that all
	// messages get received
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	hosts := getNetHosts(t, ctx, 20)
	psubs := getGossipsubs(ctx, hosts, WithFloodPublish(true))

	// build the star
	for i := 1; i < 20; i++ {
		connect(t, hosts[0], hosts[i])
	}

	// build the (partial, unstable) mesh
	var subs []*Subscription
	for _, ps := range psubs {
		sub, err := ps.Subscribe("test")
		if err != nil {
			t.Fatal(err)
		}
		subs = append(subs, sub)
	}

	time.Sleep(time.Second)

	// send a message from the star and assert it was received
	for i := 0; i < 20; i++ {
		msg := []byte(fmt.Sprintf("message %d", i))
		psubs[0].Publish("test", msg)

		for _, sub := range subs {
			assertReceive(t, sub, msg)
		}
	}
}

func TestGossipsubEnoughPeers(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	hosts := getNetHosts(t, ctx, 20)
	psubs := getGossipsubs(ctx, hosts)

	var subs []*Subscription
	for _, ps := range psubs {
		sub, err := ps.Subscribe("test")
		if err != nil {
			t.Fatal(err)
		}
		subs = append(subs, sub)
	}

	// at this point we have no connections and no mesh, so EnoughPeers should return false
	res := make(chan bool, 1)
	psubs[0].eval <- func() {
		res <- psubs[0].rt.EnoughPeers("test", 0)
	}
	enough := <-res
	if enough {
		t.Fatal("should not have enough peers")
	}

	// connect them densly to build up the mesh
	denseConnect(t, hosts)

	time.Sleep(3 * time.Second)

	psubs[0].eval <- func() {
		res <- psubs[0].rt.EnoughPeers("test", 0)
	}
	enough = <-res
	if !enough {
		t.Fatal("should have enough peers")
	}
}

func TestGossipsubNegativeScore(t *testing.T) {
	// in this test we score sinkhole a peer to exercise code paths relative to negative scores
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	hosts := getNetHosts(t, ctx, 20)
	psubs := getGossipsubs(ctx, hosts,
		WithPeerScore(
			&PeerScoreParams{
				AppSpecificScore: func(p peer.ID) float64 {
					if p == hosts[0].ID() {
						return -1000
					} else {
						return 0
					}
				},
				AppSpecificWeight: 1,
				DecayInterval:     time.Second,
				DecayToZero:       0.01,
			},
			&PeerScoreThresholds{
				GossipThreshold:   -10,
				PublishThreshold:  -100,
				GraylistThreshold: -10000,
			}))

	denseConnect(t, hosts)

	var subs []*Subscription
	for _, ps := range psubs {
		sub, err := ps.Subscribe("test")
		if err != nil {
			t.Fatal(err)
		}
		subs = append(subs, sub)
	}

	time.Sleep(3 * time.Second)

	for i := 0; i < 20; i++ {
		msg := []byte(fmt.Sprintf("message %d", i))
		psubs[i%20].Publish("test", msg)
		time.Sleep(20 * time.Millisecond)
	}

	// let the sinkholed peer try to emit gossip as well
	time.Sleep(2 * time.Second)

	// checks:
	// 1. peer 0 should only receive its own message
	// 2. peers 1-20 should not receive a message from peer 0, because it's not part of the mesh
	//    and its gossip is rejected
	collectAll := func(sub *Subscription) []*Message {
		var res []*Message
		ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
		defer cancel()

		for {
			msg, err := sub.Next(ctx)
			if err != nil {
				break
			}

			res = append(res, msg)
		}

		return res
	}

	count := len(collectAll(subs[0]))
	if count != 1 {
		t.Fatalf("expected 1 message but got %d instead", count)
	}

	for _, sub := range subs[1:] {
		all := collectAll(sub)
		for _, m := range all {
			if m.ReceivedFrom == hosts[0].ID() {
				t.Fatal("received message from sinkholed peer")
			}
		}
	}
}

func TestGossipsubPiggybackControl(t *testing.T) {
	// this is a direct test of the piggybackControl function as we can't reliably
	// trigger it on travis
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	h := bhost.NewBlankHost(swarmt.GenSwarm(t, ctx))
	ps := getGossipsub(ctx, h)

	blah := peer.ID("bogotr0n")

	res := make(chan *RPC, 1)
	ps.eval <- func() {
		gs := ps.rt.(*GossipSubRouter)
		test1 := "test1"
		test2 := "test2"
		test3 := "test3"
		gs.mesh[test1] = make(map[peer.ID]struct{})
		gs.mesh[test2] = make(map[peer.ID]struct{})
		gs.mesh[test1][blah] = struct{}{}

		rpc := &RPC{RPC: pb.RPC{}}
		gs.piggybackControl(blah, rpc, &pb.ControlMessage{
			Graft: []*pb.ControlGraft{&pb.ControlGraft{TopicID: &test1}, &pb.ControlGraft{TopicID: &test2}, &pb.ControlGraft{TopicID: &test3}},
			Prune: []*pb.ControlPrune{&pb.ControlPrune{TopicID: &test1}, &pb.ControlPrune{TopicID: &test2}, &pb.ControlPrune{TopicID: &test3}},
		})
		res <- rpc
	}

	rpc := <-res
	if rpc.Control == nil {
		t.Fatal("expected non-nil control message")
	}
	if len(rpc.Control.Graft) != 1 {
		t.Fatal("expected 1 GRAFT")
	}
	if rpc.Control.Graft[0].GetTopicID() != "test1" {
		t.Fatal("expected test1 as graft topic ID")
	}
	if len(rpc.Control.Prune) != 2 {
		t.Fatal("expected 2 PRUNEs")
	}
	if rpc.Control.Prune[0].GetTopicID() != "test2" {
		t.Fatal("expected test2 as prune topic ID")
	}
	if rpc.Control.Prune[1].GetTopicID() != "test3" {
		t.Fatal("expected test3 as prune topic ID")
	}
}

func TestGossipsubOpportunisticGrafting(t *testing.T) {
	originalGossipSubOpportunisticGraftTicks := GossipSubOpportunisticGraftTicks
	GossipSubOpportunisticGraftTicks = 2
	defer func() {
		GossipSubOpportunisticGraftTicks = originalGossipSubOpportunisticGraftTicks
	}()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	hosts := getNetHosts(t, ctx, 50)
	// pubsubs for the first 10 hosts
	psubs := getGossipsubs(ctx, hosts[:10],
		WithFloodPublish(true),
		WithPeerScore(
			&PeerScoreParams{
				AppSpecificScore:  func(peer.ID) float64 { return 0 },
				AppSpecificWeight: 0,
				DecayInterval:     time.Second,
				DecayToZero:       0.01,
				Topics: map[string]*TopicScoreParams{
					"test": &TopicScoreParams{
						TopicWeight:                   1,
						TimeInMeshWeight:              0.0002777,
						TimeInMeshQuantum:             time.Second,
						TimeInMeshCap:                 3600,
						FirstMessageDeliveriesWeight:  1,
						FirstMessageDeliveriesDecay:   0.9997,
						FirstMessageDeliveriesCap:     100,
						InvalidMessageDeliveriesDecay: 0.99997,
					},
				},
			},
			&PeerScoreThresholds{
				GossipThreshold:             -10,
				PublishThreshold:            -100,
				GraylistThreshold:           -10000,
				OpportunisticGraftThreshold: 1,
			}))

	// connect the real hosts with degree 5
	connectSome(t, hosts[:10], 5)

	// sybil squatters for the remaining 40 hosts
	squatters := make([]*sybilSquatter, 0, 40)
	for _, h := range hosts[10:] {
		squatter := &sybilSquatter{h: h}
		h.SetStreamHandler(GossipSubID_v10, squatter.handleStream)
		squatters = append(squatters, squatter)
	}

	// connect all squatters to every real host
	for _, squatter := range hosts[10:] {
		for _, real := range hosts[:10] {
			connect(t, squatter, real)
		}
	}

	// wait a bit for the connections to propagate events to the pubsubs
	time.Sleep(time.Second)

	// ask the real pubsus to join the topic
	for _, ps := range psubs {
		sub, err := ps.Subscribe("test")
		if err != nil {
			t.Fatal(err)
		}
		// consume the messages
		go func(sub *Subscription) {
			for {
				_, err := sub.Next(ctx)
				if err != nil {
					return
				}
			}
		}(sub)
	}

	// publish a bunch of messages from the real hosts
	for i := 0; i < 1000; i++ {
		msg := []byte(fmt.Sprintf("message %d", i))
		psubs[i%10].Publish("test", msg)
		time.Sleep(20 * time.Millisecond)
	}

	// now wait a few of oppgraft cycles
	time.Sleep(7 * time.Second)

	// check the honest peer meshes, they should have at least 3 honest peers each
	res := make(chan int, 1)
	for _, ps := range psubs {
		ps.eval <- func() {
			gs := ps.rt.(*GossipSubRouter)
			count := 0
			for _, h := range hosts[:10] {
				_, ok := gs.mesh["test"][h.ID()]
				if ok {
					count++
				}
			}
			res <- count
		}

		count := <-res
		if count < 3 {
			t.Fatalf("expected at least 3 honest peers, got %d", count)
		}
	}
}

type sybilSquatter struct {
	h host.Host
}

func (sq *sybilSquatter) handleStream(s network.Stream) {
	defer s.Close()

	os, err := sq.h.NewStream(context.Background(), s.Conn().RemotePeer(), GossipSubID_v10)
	if err != nil {
		panic(err)
	}

	// send a subscription for test in the output stream to become candidate for GRAFT
	// and then just read and ignore the incoming RPCs
	r := ggio.NewDelimitedReader(s, 1<<20)
	w := ggio.NewDelimitedWriter(os)
	truth := true
	topic := "test"
	err = w.WriteMsg(&pb.RPC{Subscriptions: []*pb.RPC_SubOpts{&pb.RPC_SubOpts{Subscribe: &truth, Topicid: &topic}}})
	if err != nil {
		panic(err)
	}

	var rpc pb.RPC
	for {
		rpc.Reset()
		err = r.ReadMsg(&rpc)
		if err != nil {
			if err != io.EOF {
				s.Reset()
			}
			return
		}
	}
}

package routing

import (
	"bytes"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net"
	"strconv"
	"strings"
	"testing"

	"github.com/btcsuite/btcd/chaincfg/chainhash"
	"github.com/btcsuite/btcd/wire"
	"github.com/btcsuite/btcutil"
	"github.com/coreos/bbolt"
	"github.com/go-errors/errors"
	"github.com/lightningnetwork/lnd/channeldb"
	"github.com/lightningnetwork/lnd/lnwire"
	"github.com/lightningnetwork/lnd/routing/route"
)

// testGraph is the struct which corresponds to the JSON format used to encode
// graphs within the files in the testdata directory.
//
// TODO(roasbeef): add test graph auto-generator
type testGraph2 struct {
	Nodes []testNode2 `json:"nodes"`
	Edges []testChan2 `json:"edges"`
}

// testNode represents a node within the test graph above. We skip certain
// information such as the node's IP address as that information isn't needed
// for our tests.
type testNode2 struct {
	Source bool   `json:"source"`
	PubKey string `json:"pubkey"`
	Alias  string `json:"alias"`
}

// testChan represents the JSON version of a payment channel. This struct
// matches the Json that's encoded under the "edges" key within the test graph.
type testChan2 struct {
	Node1        string     `json:"node1_pub"`
	Node2        string     `json:"node2_pub"`
	ChannelID    string     `json:"channel_id"`
	ChannelPoint string     `json:"channel_point"`
	Capacity     string     `json:"capacity"`
	Node1Policy  testPolicy `json:"node1_policy"`
	Node2Policy  testPolicy `json:"node2_policy"`
}

type testPolicy struct {
	Expiry      uint16 `json:"time_lock_delta"`
	MinHTLC     string `json:"min_htlc"`
	MaxHTLC     string `json:"max_htlc_msat"`
	FeeBaseMsat string `json:"fee_base_msat"`
	FeeRate     string `json:"fee_rate_milli_msat"`
}

type testGraphInstance2 struct {
	graph *channeldb.ChannelGraph
}

func loadGraph() *channeldb.ChannelGraph {
	cdb, _ := channeldb.Open("testdata/main-db")
	return cdb.ChannelGraph()
}

// parseTestGraph returns a fully populated ChannelGraph given a path to a JSON
// file which encodes a test graph.
func parseTestGraph2(path string) (*testGraphInstance2, error) {
	graphJSON, err := ioutil.ReadFile(path)
	if err != nil {
		return nil, err
	}

	// First unmarshal the JSON graph into an instance of the testGraph
	// struct. Using the struct tags created above in the struct, the JSON
	// will be properly parsed into the struct above.
	var g testGraph2
	if err := json.Unmarshal(graphJSON, &g); err != nil {
		return nil, err
	}

	// We'll use this fake address for the IP address of all the nodes in
	// our tests. This value isn't needed for path finding so it doesn't
	// need to be unique.
	var testAddrs []net.Addr
	testAddr, err := net.ResolveTCPAddr("tcp", "192.0.0.1:8888")
	if err != nil {
		return nil, err
	}
	testAddrs = append(testAddrs, testAddr)

	// Next, create a temporary graph database for usage within the test.
	graph := loadGraph()

	var source *channeldb.LightningNode

	// First we insert all the nodes within the graph as vertexes.
	for _, node := range g.Nodes {
		pubBytes, err := hex.DecodeString(node.PubKey)
		if err != nil {
			return nil, err
		}

		dbNode := &channeldb.LightningNode{
			HaveNodeAnnouncement: true,
			AuthSigBytes:         testSig.Serialize(),
			LastUpdate:           testTime,
			Addresses:            testAddrs,
			Alias:                node.Alias,
			Features:             testFeatures,
		}
		copy(dbNode.PubKeyBytes[:], pubBytes)

		// If the node is tagged as the source, then we create a
		// pointer to is so we can mark the source in the graph
		// properly.
		if node.Source {
			// If we come across a node that's marked as the
			// source, and we've already set the source in a prior
			// iteration, then the JSON has an error as only ONE
			// node can be the source in the graph.
			if source != nil {
				return nil, errors.New("JSON is invalid " +
					"multiple nodes are tagged as the source")
			}

			source = dbNode
		}

		// With the node fully parsed, add it as a vertex within the
		// graph.
		if err := graph.AddLightningNode(dbNode); err != nil {
			return nil, err
		}
	}

	if source != nil {
		// Set the selected source node
		if err := graph.SetSourceNode(source); err != nil {
			return nil, err
		}
	}

	// With all the vertexes inserted, we can now insert the edges into the
	// test graph.
	for _, edge := range g.Edges {
		node1Bytes, err := hex.DecodeString(edge.Node1)
		if err != nil {
			return nil, err
		}

		node2Bytes, err := hex.DecodeString(edge.Node2)
		if err != nil {
			return nil, err
		}

		if bytes.Compare(node1Bytes, node2Bytes) == 1 {
			return nil, fmt.Errorf(
				"channel %v node order incorrect",
				edge.ChannelID,
			)
		}

		fundingTXID := strings.Split(edge.ChannelPoint, ":")[0]
		txidBytes, err := chainhash.NewHashFromStr(fundingTXID)
		if err != nil {
			return nil, err
		}
		fundingPoint := wire.OutPoint{
			Hash:  *txidBytes,
			Index: 0,
		}

		// We first insert the existence of the edge between the two
		// nodes.
		channelID, _ := strconv.ParseUint(edge.ChannelID, 10, 64)
		capacity, _ := strconv.ParseUint(edge.Capacity, 10, 64)
		edgeInfo := channeldb.ChannelEdgeInfo{
			ChannelID:    channelID,
			AuthProof:    &testAuthProof,
			ChannelPoint: fundingPoint,
			Capacity:     btcutil.Amount(capacity),
		}

		copy(edgeInfo.NodeKey1Bytes[:], node1Bytes)
		copy(edgeInfo.NodeKey2Bytes[:], node2Bytes)
		copy(edgeInfo.BitcoinKey1Bytes[:], node1Bytes)
		copy(edgeInfo.BitcoinKey2Bytes[:], node2Bytes)

		err = graph.AddChannelEdge(&edgeInfo)
		if err != nil && err != channeldb.ErrEdgeAlreadyExist {
			return nil, err
		}

		edge1Policy := &channeldb.ChannelEdgePolicy{
			SigBytes:                  testSig.Serialize(),
			MessageFlags:              lnwire.ChanUpdateMsgFlags(0),
			ChannelFlags:              lnwire.ChanUpdateChanFlags(0),
			ChannelID:                 channelID,
			LastUpdate:                testTime,
			TimeLockDelta:             edge.Node1Policy.Expiry,
			MinHTLC:                   milliSatoshiFromString(edge.Node1Policy.MinHTLC),
			MaxHTLC:                   milliSatoshiFromString(edge.Node1Policy.MaxHTLC),
			FeeBaseMSat:               milliSatoshiFromString(edge.Node1Policy.FeeBaseMsat),
			FeeProportionalMillionths: milliSatoshiFromString(edge.Node1Policy.FeeRate),
		}
		if err := graph.UpdateEdgePolicy(edge1Policy); err != nil {
			return nil, err
		}

		edge2Policy := &channeldb.ChannelEdgePolicy{
			SigBytes:                  testSig.Serialize(),
			MessageFlags:              lnwire.ChanUpdateMsgFlags(0),
			ChannelFlags:              lnwire.ChanUpdateChanFlags(lnwire.ChanUpdateDirection),
			ChannelID:                 channelID,
			LastUpdate:                testTime,
			TimeLockDelta:             edge.Node2Policy.Expiry,
			MinHTLC:                   milliSatoshiFromString(edge.Node2Policy.MinHTLC),
			MaxHTLC:                   milliSatoshiFromString(edge.Node2Policy.MaxHTLC),
			FeeBaseMSat:               milliSatoshiFromString(edge.Node2Policy.FeeBaseMsat),
			FeeProportionalMillionths: milliSatoshiFromString(edge.Node2Policy.FeeRate),
		}
		if err := graph.UpdateEdgePolicy(edge2Policy); err != nil {
			return nil, err
		}
	}

	return &testGraphInstance2{
		graph: graph,
	}, nil
}

func milliSatoshiFromString(val string) lnwire.MilliSatoshi {
	parsed, _ := strconv.ParseUint(val, 10, 64)
	return lnwire.MilliSatoshi(parsed)
}

func BenchmarkBigGraphCachePerformance(b *testing.B) {

	// These first few lines seed a DB that is persisted across runs
	// Seeding the DB takes > 15m so it can't be done every time
	//testGraphInstance, err := parseTestGraph2("testdata/main.json")
	//if err != nil {
	//	t.Fatalf("unable to create graph: %v", err)
	//} else {
	//	t.Logf("finished creating DB")
	//	return
	//}

	testGraphInstance := &testGraphInstance2{
		graph: loadGraph(),
	}

	sourceNode, err := testGraphInstance.graph.SourceNode()
	if err != nil {
		b.Fatalf("unable to fetch source node: %v", err)
	}

	const (
		startingHeight = 100
		finalHopCLTV   = 1
	)

	paymentAmt := lnwire.NewMSatFromSatoshis(100)

	var target route.Vertex

	targetPubKey, _ := hex.DecodeString("0237fefbe8626bf888de0cad8c73630e32746a22a2c4faa91c1d9877a3826e1174")
	copy(target[:], targetPubKey[:33])

	// Do a first run to warm up
	_, _ = findPath(
		&graphParams{
			graph: testGraphInstance.graph,
		},
		noRestrictions,
		testPathFindingConfig,
		route.Vertex(sourceNode.PubKeyBytes), target, paymentAmt,
	)

	b.ResetTimer()

	b.Run("findPath", func(b *testing.B) {

		for i := 0; i < b.N; i++ {

			path, err := findPath(
				&graphParams{
					graph: testGraphInstance.graph,
				},
				noRestrictions,
				testPathFindingConfig,
				route.Vertex(sourceNode.PubKeyBytes), target, paymentAmt,
			)
			if err != nil {
				b.Fatalf("unable to find path: %v", err)
			}
			b.Logf("found path with len %v", len(path))
		}
	})
}

func BenchmarkBigGraphCacheConcurrency(b *testing.B) {

	// These first few lines seed a DB that is persisted across runs
	// Seeding the DB takes > 15m so it can't be done every time
	//testGraphInstance, err := parseTestGraph2("testdata/main.json")
	//if err != nil {
	//	t.Fatalf("unable to create graph: %v", err)
	//} else {
	//	t.Logf("finished creating DB")
	//	return
	//}

	testGraphInstance := &testGraphInstance2{
		graph: loadGraph(),
	}

	sourceNode, err := testGraphInstance.graph.SourceNode()
	if err != nil {
		b.Fatalf("unable to fetch source node: %v", err)
	}

	const (
		startingHeight = 100
		finalHopCLTV   = 1
	)

	paymentAmt := lnwire.NewMSatFromSatoshis(100)

	var target route.Vertex

	targetPubKey, _ := hex.DecodeString("0237fefbe8626bf888de0cad8c73630e32746a22a2c4faa91c1d9877a3826e1174")
	copy(target[:], targetPubKey[:33])

	b.ResetTimer()
	var policy *channeldb.ChannelEdgePolicy
	_ = sourceNode.ForEachChannel(nil, func(tx *bbolt.Tx, _ *channeldb.ChannelEdgeInfo, p1, p2 *channeldb.ChannelEdgePolicy) error {
		if p1 != nil {
			policy = p1
		} else if p2 != nil {
			policy = p2
		}

		if policy != nil {
			return fmt.Errorf("boop")
		} else {
			return nil
		}

	})

	b.Run("findPath", func(b *testing.B) {

		b.SetParallelism(10)
		b.RunParallel(func(pb *testing.PB) {

			for pb.Next() {
				testGraphInstance.graph.UpdateEdgePolicy(policy)
				path, err := findPath(
					&graphParams{
						graph: testGraphInstance.graph,
					},
					noRestrictions,
					testPathFindingConfig,
					route.Vertex(sourceNode.PubKeyBytes), target, paymentAmt,
				)
				testGraphInstance.graph.UpdateEdgePolicy(policy)
				if err != nil {
					b.Fatalf("unable to find path: %v", err)
				}
				b.Logf("found path with len %v", len(path))
			}
		})
	})
}

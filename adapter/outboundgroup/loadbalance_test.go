package outboundgroup

import (
	"fmt"
	"net/netip"
	"testing"
	"time"

	"github.com/metacubex/bbolt"
	"github.com/metacubex/http"
	"github.com/metacubex/mihomo/adapter"
	"github.com/metacubex/mihomo/adapter/outbound"
	"github.com/metacubex/mihomo/component/smart"
	C "github.com/metacubex/mihomo/constant"

	"github.com/stretchr/testify/require"
)

const testUrl = "https://www.gstatic.com/generate_204"

func balancedProxies(count int) []C.Proxy {
	proxies := make([]C.Proxy, 0, count)
	for i := 0; i < count; i++ {
		proxies = append(proxies, adapter.NewProxy(outbound.NewDirect()))
	}
	return proxies
}

type filterOrderProxy struct {
	C.Proxy
	name string
}

func (p *filterOrderProxy) Name() string                { return p.name }
func (p *filterOrderProxy) AliveForTestUrl(string) bool { return true }

func TestFilterProxiesPutsSuspectedBehindUnrankedCandidates(t *testing.T) {
	db, err := bbolt.Open(t.TempDir()+"/smart.db", 0o600, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	store := smart.NewStore(db)
	watcher := smart.NewExitWatcher(smart.ExitWatcherOptions{Name: "exit-order-test", Config: "test"})
	now := time.Now()
	all := make([]C.Proxy, 0, 12)
	names := make([]string, 0, 10)
	weights := make([]float64, 0, 10)
	for i := 0; i < 12; i++ {
		name := fmt.Sprintf("node-%02d", i)
		proxy := &filterOrderProxy{Proxy: adapter.NewProxy(outbound.NewDirect()), name: name}
		all = append(all, proxy)
		if i >= 10 {
			continue
		}
		names = append(names, name)
		weights = append(weights, 1)
		watcher.Store(name, smart.ExitInfo{Region: "hk", Key: name}, now)
		watcher.Note("*.example.test", name)
		if i == 0 {
			watcher.Store("control", smart.ExitInfo{Region: "us", Key: "control-exit"}, now)
			watcher.NoteSuccess("*.example.test", "control")
		}
	}

	s := &Smart{
		GroupBase:  NewGroupBase(GroupBaseOption{Name: "exit-order-test"}),
		store:      store,
		exitWatch:  watcher,
		testUrl:    testUrl,
		configName: "test",
	}
	selected := s.filterProxies(request("user", "example.test"), "*.example.test", names, weights, all, 10, false)
	if len(selected) != 3 {
		t.Fatalf("selected %d candidates, want 2 healthy alternatives and 1 half-open fallback", len(selected))
	}
	if selected[0].Name() != "node-10" || selected[1].Name() != "node-11" {
		t.Fatalf("unranked candidates must precede suspected exits, got %q, %q", selected[0].Name(), selected[1].Name())
	}
	if watcher.Suspected("*.example.test", selected[2].Name()) == false {
		t.Fatalf("last candidate %q must be the half-open fallback", selected[2].Name())
	}
}

func TestFilterProxiesDefersCachedSuspectedNodes(t *testing.T) {
	db, err := bbolt.Open(t.TempDir()+"/smart.db", 0o600, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	store := smart.NewStore(db)
	watcher := smart.NewExitWatcher(smart.ExitWatcherOptions{Name: "cached-exit-order-test", Config: "test"})
	now := time.Now()
	target := "*.onejav.com"
	for _, node := range []string{"jp-suspected", "jp-second"} {
		watcher.Store(node, smart.ExitInfo{Region: "jp", Key: node}, now)
		watcher.Note(target, node)
	}
	watcher.Store("hk-control", smart.ExitInfo{Region: "hk", Key: "hk-control"}, now)
	watcher.NoteSuccess(target, "hk-control")

	all := []C.Proxy{
		&filterOrderProxy{Proxy: adapter.NewProxy(outbound.NewDirect()), name: "jp-suspected"},
		&filterOrderProxy{Proxy: adapter.NewProxy(outbound.NewDirect()), name: "hk-cached"},
		&filterOrderProxy{Proxy: adapter.NewProxy(outbound.NewDirect()), name: "hk-fallback-1"},
		&filterOrderProxy{Proxy: adapter.NewProxy(outbound.NewDirect()), name: "hk-fallback-2"},
	}
	group := &Smart{
		GroupBase:  NewGroupBase(GroupBaseOption{Name: "cached-exit-order-test"}),
		store:      store,
		exitWatch:  watcher,
		testUrl:    testUrl,
		configName: "test",
	}
	selected := group.filterProxies(request("user", "onejav.com"), target,
		[]string{"jp-suspected", "hk-cached"}, nil, all, 3, false)
	if len(selected) != 1 || selected[0].Name() != "hk-cached" {
		t.Fatalf("cached selection should keep only its healthy entry, got %v", selected)
	}
	for _, proxy := range selected {
		if proxy.Name() == "jp-suspected" {
			t.Fatalf("a cached suspected candidate must stay behind available alternatives: %+v", selected)
		}
	}
}

func TestFilterProxiesFailsOpenWhenAllCandidatesSuspected(t *testing.T) {
	db, err := bbolt.Open(t.TempDir()+"/smart.db", 0o600, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	store := smart.NewStore(db)
	watcher := smart.NewExitWatcher(smart.ExitWatcherOptions{Name: "exit-fail-open-test", Config: "test"})
	now := time.Now()
	for _, node := range []string{"jp-01", "jp-02"} {
		watcher.Store(node, smart.ExitInfo{Region: "jp", Key: node}, now)
		watcher.Note("*.onejav.com", node)
	}
	watcher.Store("hk-control", smart.ExitInfo{Region: "hk", Key: "hk-control"}, now)
	watcher.NoteSuccess("*.onejav.com", "hk-control")
	proxies := []C.Proxy{
		&filterOrderProxy{Proxy: adapter.NewProxy(outbound.NewDirect()), name: "jp-01"},
		&filterOrderProxy{Proxy: adapter.NewProxy(outbound.NewDirect()), name: "jp-02"},
	}
	group := &Smart{
		GroupBase:  NewGroupBase(GroupBaseOption{Name: "exit-fail-open-test"}),
		store:      store,
		exitWatch:  watcher,
		testUrl:    testUrl,
		configName: "test",
	}
	selected := group.filterProxies(request("user", "onejav.com"), "*.onejav.com", []string{"jp-01", "jp-02"}, []float64{1, 1}, proxies, 2, false)
	if len(selected) != 1 || selected[0].Name() != "jp-01" {
		t.Fatalf("all-suspected target should allow only one half-open node, got %v", selected)
	}
}

func TestPeriodicOriginRefusalsRaiseExitSuspicion(t *testing.T) {
	watcher := smart.NewExitWatcher(smart.ExitWatcherOptions{Name: "exit-evidence-test"})
	now := time.Now()
	watcher.Store("jp-01", smart.ExitInfo{Region: "jp", Key: "exit-jp-01"}, now)
	watcher.Store("jp-02", smart.ExitInfo{Region: "jp", Key: "exit-jp-02"}, now)
	group := &Smart{exitWatch: watcher}
	target := "*.onejav.com"

	group.noteExitEvidence(target, "jp-01", smart.ReasonOriginError)
	if watcher.Suspected(target, "jp-01") {
		t.Fatal("one JP exit must not raise a suspicion")
	}
	watcher.Store("hk-control", smart.ExitInfo{Region: "hk", Key: "exit-hk"}, now)
	group.noteExitSuccess(target, "hk-control", smart.ClassifyResponse(http.StatusOK, nil, nil, now))
	group.noteExitEvidence(target, "jp-02", smart.ReasonOriginError)
	if !watcher.Suspected(target, "jp-01") {
		t.Fatal("independent JP origin refusals must raise a suspicion")
	}
}

// The proxies are indistinguishable by name, so identity is the pointer.
func indexOf(t *testing.T, proxies []C.Proxy, selected C.Proxy) int {
	t.Helper()
	for i, proxy := range proxies {
		if proxy == selected {
			return i
		}
	}
	require.Fail(t, "selected proxy is not a member of the group")
	return -1
}

func request(user, host string) *C.Metadata {
	return &C.Metadata{
		NetWork: C.TCP,
		Host:    host,
		DstPort: 443,
		SrcIP:   netip.MustParseAddr("127.0.0.1"),
		InUser:  user,
	}
}

// One unit of work walking several destinations is the case both address-derived
// keys get wrong: the group is meant to hold that work on one egress, and the
// default key moves it as soon as the host changes.
func TestLoadBalanceHashKeyInUserSurvivesADestinationChange(t *testing.T) {
	proxies := balancedProxies(8)
	hosts := []string{"a.example.com", "b.example.org", "c.example.net", "d.example.io"}

	byUser := strategyConsistentHashing(testUrl, getKeyWithInUser(getKey))
	pinned := indexOf(t, proxies, byUser(proxies, request("job-1", hosts[0]), false))
	for _, host := range hosts {
		selected := byUser(proxies, request("job-1", host), false)
		require.Equal(t, pinned, indexOf(t, proxies, selected),
			"hash-key: user must ignore the destination")
	}

	byDestination := strategyConsistentHashing(testUrl, getKey)
	seen := map[int]bool{}
	for _, host := range hosts {
		seen[indexOf(t, proxies, byDestination(proxies, request("job-1", host), false))] = true
	}
	require.Greater(t, len(seen), 1,
		"the default key is expected to move with the destination")
}

// Pinning must not become a single node: distinct users still spread.
func TestLoadBalanceHashKeyInUserSpreadsUsers(t *testing.T) {
	proxies := balancedProxies(8)
	strategy := strategyConsistentHashing(testUrl, getKeyWithInUser(getKey))

	seen := map[int]bool{}
	for _, user := range []string{"job-1", "job-2", "job-3", "job-4", "job-5", "job-6"} {
		selected := strategy(proxies, request(user, "a.example.com"), false)
		seen[indexOf(t, proxies, selected)] = true
	}
	require.Greater(t, len(seen), 1)
}

// Sticky sessions keys on source and destination; a client behind one source
// address cannot separate its own concurrent jobs without a supplied identity.
func TestLoadBalanceHashKeyInUserSeparatesJobsSharingASourceAddress(t *testing.T) {
	proxies := balancedProxies(8)
	strategy := strategyStickySessions(testUrl, getKeyWithInUser(getKeyWithSrcAndDst))

	first := indexOf(t, proxies, strategy(proxies, request("job-1", "a.example.com"), false))
	require.Equal(t, first,
		indexOf(t, proxies, strategy(proxies, request("job-1", "b.example.org"), false)))

	shared := strategyStickySessions(testUrl, getKeyWithSrcAndDst)
	require.Equal(t,
		indexOf(t, proxies, shared(proxies, request("job-1", "a.example.com"), false)),
		indexOf(t, proxies, shared(proxies, request("job-2", "a.example.com"), false)),
		"without a supplied key the two jobs are one session")
}

// An unauthenticated request keeps the strategy's own key. Returning a constant
// instead would herd every anonymous request onto one member.
func TestLoadBalanceHashKeyInUserFallsBackWhenUnauthenticated(t *testing.T) {
	keyed := getKeyWithInUser(getKey)
	require.Equal(t, "example.com", keyed(request("", "a.example.com")))
	require.Equal(t, "job-1", keyed(request("job-1", "a.example.com")))
	require.Equal(t, getKey(nil), keyed(nil))
}

// The option name is the contract with the config file, and nothing else here
// exercises it: every other test reaches the decorator directly, so renaming
// the case would leave them all green while `hash-key: in-user` stopped working.
func TestLoadBalanceHashKeyResolvesTheOptionName(t *testing.T) {
	withInUser, err := hashKey("in-user")
	require.NoError(t, err)
	require.Equal(t, "job-1", withInUser(getKey)(request("job-1", "a.example.com")))

	identity, err := hashKey("")
	require.NoError(t, err)
	require.Equal(t, getKey(request("job-1", "a.example.com")),
		identity(getKey)(request("job-1", "a.example.com")))
}

func TestLoadBalanceHashKeyRejectsUnusableConfigs(t *testing.T) {
	_, err := hashKey("session")
	require.ErrorIs(t, err, errHashKey)

	// `user` was the name this option carried before review. Rejecting it keeps
	// the rename honest: without this the case above could still read `user`
	// and every test here would stay green.
	_, err = hashKey("user")
	require.ErrorIs(t, err, errHashKey)

	_, err = NewLoadBalance(GroupCommonOption{Name: "lb"},
		LoadBalanceOption{Strategy: "round-robin", HashKey: "in-user"}, nil, nil)
	require.ErrorIs(t, err, errHashKey)

	_, err = NewLoadBalance(GroupCommonOption{Name: "lb"},
		LoadBalanceOption{Strategy: "consistent-hashing", HashKey: "nonsense"}, nil, nil)
	require.ErrorIs(t, err, errHashKey)
}

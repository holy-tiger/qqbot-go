package store

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"
)

func TestKnownUsersStore_RecordNewUser(t *testing.T) {
	dir := t.TempDir()
	s := NewKnownUsersStore(dir)
	defer s.Close()

	now := time.Now().UnixMilli()
	s.Record(KnownUser{
		OpenID:      "user1",
		Type:        "c2c",
		AccountID:   "acct1",
		Nickname:    "TestUser",
		FirstSeenAt: now,
		LastSeenAt:  now,
	})

	u := s.Get("acct1", "user1", "c2c", "")
	if u == nil {
		t.Fatal("expected user to be recorded")
	}
	if u.OpenID != "user1" {
		t.Errorf("expected openid=user1, got %s", u.OpenID)
	}
	if u.InteractionCount != 1 {
		t.Errorf("expected interaction_count=1, got %d", u.InteractionCount)
	}
	if u.Nickname != "TestUser" {
		t.Errorf("expected nickname=TestUser, got %s", u.Nickname)
	}
}

func TestKnownUsersStore_RecordUpsert(t *testing.T) {
	dir := t.TempDir()
	s := NewKnownUsersStore(dir)
	defer s.Close()

	s.Record(KnownUser{
		OpenID:    "user1",
		Type:      "c2c",
		AccountID: "acct1",
	})
	firstSeen := s.Get("acct1", "user1", "c2c", "").FirstSeenAt

	time.Sleep(5 * time.Millisecond)

	// Record again - should upsert (increment count, update lastSeenAt, keep firstSeenAt)
	s.Record(KnownUser{
		OpenID:    "user1",
		Type:      "c2c",
		AccountID: "acct1",
		Nickname:  "NewName",
	})

	u := s.Get("acct1", "user1", "c2c", "")
	if u == nil {
		t.Fatal("expected user after upsert")
	}
	if u.InteractionCount != 2 {
		t.Errorf("expected interaction_count=2, got %d", u.InteractionCount)
	}
	if u.Nickname != "NewName" {
		t.Errorf("expected nickname=NewName, got %s", u.Nickname)
	}
	if u.FirstSeenAt != firstSeen {
		t.Errorf("expected firstSeenAt to remain unchanged, got %d vs %d", u.FirstSeenAt, firstSeen)
	}
	if u.LastSeenAt <= firstSeen {
		t.Errorf("expected lastSeenAt to be updated after firstSeenAt, got %d", u.LastSeenAt)
	}
}

func TestKnownUsersStore_GetGroupUser(t *testing.T) {
	dir := t.TempDir()
	s := NewKnownUsersStore(dir)
	defer s.Close()

	now := time.Now().UnixMilli()
	s.Record(KnownUser{
		OpenID:      "user1",
		Type:        "group",
		GroupOpenID: "grp1",
		AccountID:   "acct1",
		FirstSeenAt: now,
		LastSeenAt:  now,
	})

	// Same user in different group should be separate entry
	s.Record(KnownUser{
		OpenID:      "user1",
		Type:        "group",
		GroupOpenID: "grp2",
		AccountID:   "acct1",
		FirstSeenAt: now,
		LastSeenAt:  now,
	})

	u1 := s.Get("acct1", "user1", "group", "grp1")
	u2 := s.Get("acct1", "user1", "group", "grp2")
	if u1 == nil || u2 == nil {
		t.Fatal("expected both group users")
	}
	if u1.GroupOpenID != "grp1" || u2.GroupOpenID != "grp2" {
		t.Error("group openids mismatched")
	}
}

func TestKnownUsersStore_GetNotFound(t *testing.T) {
	dir := t.TempDir()
	s := NewKnownUsersStore(dir)

	u := s.Get("acct1", "nonexistent", "c2c", "")
	if u != nil {
		t.Error("expected nil for nonexistent user")
	}
}

func TestKnownUsersStore_ListWithFilters(t *testing.T) {
	dir := t.TempDir()
	s := NewKnownUsersStore(dir)
	defer s.Close()

	now := time.Now().UnixMilli()
	// Add users
	s.Record(KnownUser{OpenID: "c2c1", Type: "c2c", AccountID: "acct1", FirstSeenAt: now, LastSeenAt: now})
	s.Record(KnownUser{OpenID: "grp1", Type: "group", AccountID: "acct1", FirstSeenAt: now, LastSeenAt: now, GroupOpenID: "g1"})
	s.Record(KnownUser{OpenID: "c2c2", Type: "c2c", AccountID: "acct2", FirstSeenAt: now, LastSeenAt: now})

	// Filter by account
	users := s.List(ListOptions{AccountID: "acct1"})
	if len(users) != 2 {
		t.Errorf("expected 2 users for acct1, got %d", len(users))
	}

	// Filter by type
	users = s.List(ListOptions{Type: "c2c"})
	if len(users) != 2 {
		t.Errorf("expected 2 c2c users, got %d", len(users))
	}

	// Filter by both
	users = s.List(ListOptions{AccountID: "acct1", Type: "group"})
	if len(users) != 1 {
		t.Errorf("expected 1 group user for acct1, got %d", len(users))
	}
}

func TestKnownUsersStore_ListWithLimit(t *testing.T) {
	dir := t.TempDir()
	s := NewKnownUsersStore(dir)
	defer s.Close()

	now := time.Now().UnixMilli()
	for i := 0; i < 10; i++ {
		s.Record(KnownUser{OpenID: string(rune('a' + i)), Type: "c2c", AccountID: "acct1", FirstSeenAt: now, LastSeenAt: now})
	}

	users := s.List(ListOptions{Limit: 3})
	if len(users) != 3 {
		t.Errorf("expected 3 users with limit, got %d", len(users))
	}
}

func TestKnownUsersStore_ListSorting(t *testing.T) {
	dir := t.TempDir()
	s := NewKnownUsersStore(dir)
	defer s.Close()

	now := time.Now().UnixMilli()
	s.Record(KnownUser{OpenID: "a", Type: "c2c", AccountID: "acct1", FirstSeenAt: now, LastSeenAt: now, InteractionCount: 1})
	s.Record(KnownUser{OpenID: "b", Type: "c2c", AccountID: "acct1", FirstSeenAt: now, LastSeenAt: now + 1000, InteractionCount: 5})
	s.Record(KnownUser{OpenID: "c", Type: "c2c", AccountID: "acct1", FirstSeenAt: now, LastSeenAt: now + 2000, InteractionCount: 3})

	users := s.List(ListOptions{SortBy: "interactionCount", SortOrder: "desc"})
	if len(users) != 3 {
		t.Fatalf("expected 3 users, got %d", len(users))
	}
	if users[0].InteractionCount < users[1].InteractionCount || users[1].InteractionCount < users[2].InteractionCount {
		t.Error("expected descending sort by interactionCount")
	}
}

func TestKnownUsersStore_ListActiveWithin(t *testing.T) {
	dir := t.TempDir()
	s := NewKnownUsersStore(dir)
	defer s.Close()

	// Record user, then manually backdate LastSeenAt
	s.Record(KnownUser{OpenID: "old", Type: "c2c", AccountID: "acct1"})
	s.mu.Lock()
	s.load()
	oldKey := makeUserKey("acct1", "c2c", "old", "")
	s.cache[oldKey].LastSeenAt = time.Now().UnixMilli() - 200000
	s.mu.Unlock()

	s.Record(KnownUser{OpenID: "new", Type: "c2c", AccountID: "acct1"})

	users := s.List(ListOptions{ActiveWithin: 100000}) // 100 seconds ago
	if len(users) != 1 {
		t.Errorf("expected 1 active user, got %d", len(users))
	}
	if users[0].OpenID != "new" {
		t.Errorf("expected 'new' user, got %s", users[0].OpenID)
	}
}

func TestKnownUsersStore_Stats(t *testing.T) {
	dir := t.TempDir()
	s := NewKnownUsersStore(dir)
	defer s.Close()

	now := time.Now().UnixMilli()
	s.Record(KnownUser{OpenID: "c1", Type: "c2c", AccountID: "acct1", FirstSeenAt: now, LastSeenAt: now})
	s.Record(KnownUser{OpenID: "g1", Type: "group", AccountID: "acct1", FirstSeenAt: now, LastSeenAt: now, GroupOpenID: "grp1"})
	s.Record(KnownUser{OpenID: "c2", Type: "c2c", AccountID: "acct2", FirstSeenAt: now, LastSeenAt: now})

	stats := s.Stats("")
	if stats.TotalUsers != 3 {
		t.Errorf("expected total=3, got %d", stats.TotalUsers)
	}
	if stats.C2CUsers != 2 {
		t.Errorf("expected c2c=2, got %d", stats.C2CUsers)
	}
	if stats.GroupUsers != 1 {
		t.Errorf("expected group=1, got %d", stats.GroupUsers)
	}

	// Filter by account
	stats = s.Stats("acct1")
	if stats.TotalUsers != 2 {
		t.Errorf("expected total=2 for acct1, got %d", stats.TotalUsers)
	}
}

func TestKnownUsersStore_Remove(t *testing.T) {
	dir := t.TempDir()
	s := NewKnownUsersStore(dir)
	defer s.Close()

	now := time.Now().UnixMilli()
	s.Record(KnownUser{OpenID: "user1", Type: "c2c", AccountID: "acct1", FirstSeenAt: now, LastSeenAt: now})

	removed := s.Remove("acct1", "user1", "c2c", "")
	if !removed {
		t.Error("expected remove to return true")
	}

	u := s.Get("acct1", "user1", "c2c", "")
	if u != nil {
		t.Error("expected user to be removed")
	}

	// Remove non-existent
	removed = s.Remove("acct1", "user1", "c2c", "")
	if removed {
		t.Error("expected remove to return false for non-existent")
	}
}

func TestKnownUsersStore_Clear(t *testing.T) {
	dir := t.TempDir()
	s := NewKnownUsersStore(dir)
	defer s.Close()

	now := time.Now().UnixMilli()
	s.Record(KnownUser{OpenID: "u1", Type: "c2c", AccountID: "acct1", FirstSeenAt: now, LastSeenAt: now})
	s.Record(KnownUser{OpenID: "u2", Type: "c2c", AccountID: "acct2", FirstSeenAt: now, LastSeenAt: now})

	count := s.Clear("acct1")
	if count != 1 {
		t.Errorf("expected to clear 1 user, got %d", count)
	}

	users := s.List(ListOptions{})
	if len(users) != 1 {
		t.Errorf("expected 1 remaining user, got %d", len(users))
	}

	// Clear all
	count = s.Clear("")
	if count != 1 {
		t.Errorf("expected to clear 1 user, got %d", count)
	}
}

func TestKnownUsersStore_Flush(t *testing.T) {
	dir := t.TempDir()
	s := NewKnownUsersStore(dir)

	now := time.Now().UnixMilli()
	s.Record(KnownUser{OpenID: "u1", Type: "c2c", AccountID: "acct1", FirstSeenAt: now, LastSeenAt: now})

	s.Flush()

	// Verify file was written
	fp := filepath.Join(dir, "known-users.json")
	data, err := os.ReadFile(fp)
	if err != nil {
		t.Fatalf("expected file to exist: %v", err)
	}

	var users []KnownUser
	if err := json.Unmarshal(data, &users); err != nil {
		t.Fatalf("failed to parse file: %v", err)
	}
	if len(users) != 1 || users[0].OpenID != "u1" {
		t.Errorf("unexpected file content: %+v", users)
	}
}

func TestKnownUsersStore_LoadFromFile(t *testing.T) {
	dir := t.TempDir()

	// Pre-write a file
	users := []KnownUser{
		{OpenID: "loaded", Type: "c2c", AccountID: "acct1", FirstSeenAt: 1000, LastSeenAt: 2000, InteractionCount: 3},
	}
	data, _ := json.MarshalIndent(users, "", "  ")
	os.WriteFile(filepath.Join(dir, "known-users.json"), data, 0644)

	s := NewKnownUsersStore(dir)
	u := s.Get("acct1", "loaded", "c2c", "")
	if u == nil {
		t.Fatal("expected to load user from file")
	}
	if u.InteractionCount != 3 {
		t.Errorf("expected interaction_count=3, got %d", u.InteractionCount)
	}
}

func TestKnownUsersStore_GetUserGroups(t *testing.T) {
	dir := t.TempDir()
	s := NewKnownUsersStore(dir)
	defer s.Close()

	now := time.Now().UnixMilli()
	s.Record(KnownUser{OpenID: "user1", Type: "group", GroupOpenID: "g1", AccountID: "acct1", FirstSeenAt: now, LastSeenAt: now})
	s.Record(KnownUser{OpenID: "user1", Type: "group", GroupOpenID: "g2", AccountID: "acct1", FirstSeenAt: now, LastSeenAt: now})
	s.Record(KnownUser{OpenID: "user2", Type: "group", GroupOpenID: "g1", AccountID: "acct1", FirstSeenAt: now, LastSeenAt: now})

	groups := s.GetUserGroups("acct1", "user1")
	if len(groups) != 2 {
		t.Errorf("expected 2 groups, got %d", len(groups))
	}
}

func TestKnownUsersStore_GetGroupMembers(t *testing.T) {
	dir := t.TempDir()
	s := NewKnownUsersStore(dir)
	defer s.Close()

	now := time.Now().UnixMilli()
	s.Record(KnownUser{OpenID: "u1", Type: "group", GroupOpenID: "g1", AccountID: "acct1", FirstSeenAt: now, LastSeenAt: now})
	s.Record(KnownUser{OpenID: "u2", Type: "group", GroupOpenID: "g1", AccountID: "acct1", FirstSeenAt: now, LastSeenAt: now})
	s.Record(KnownUser{OpenID: "u1", Type: "group", GroupOpenID: "g2", AccountID: "acct1", FirstSeenAt: now, LastSeenAt: now})

	members := s.GetGroupMembers("acct1", "g1")
	if len(members) != 2 {
		t.Errorf("expected 2 members in g1, got %d", len(members))
	}
}

func TestKnownUsersStore_ConcurrentAccess(t *testing.T) {
	dir := t.TempDir()
	s := NewKnownUsersStore(dir)
	defer s.Close()

	now := time.Now().UnixMilli()
	var wg sync.WaitGroup
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			s.Record(KnownUser{
				OpenID:      string(rune('a' + i%26)),
				Type:        "c2c",
				AccountID:   "acct1",
				FirstSeenAt: now,
				LastSeenAt:  now,
			})
		}(i)
	}
	wg.Wait()

	users := s.List(ListOptions{AccountID: "acct1"})
	if len(users) != 26 {
		t.Errorf("expected 26 unique users, got %d", len(users))
	}
}

func TestKnownUsersStore_ThrottledWrite(t *testing.T) {
	dir := t.TempDir()
	s := NewKnownUsersStore(dir)

	now := time.Now().UnixMilli()
	s.Record(KnownUser{OpenID: "u1", Type: "c2c", AccountID: "acct1", FirstSeenAt: now, LastSeenAt: now})

	// File should not exist yet (throttled)
	fp := filepath.Join(dir, "known-users.json")
	if _, err := os.Stat(fp); !os.IsNotExist(err) {
		t.Error("expected file to not exist immediately (throttled)")
	}

	// After flush, it should exist
	s.Flush()
	if _, err := os.Stat(fp); err != nil {
		t.Error("expected file to exist after flush")
	}
}

func TestMakeUserKey(t *testing.T) {
	// c2c user key
	key := makeUserKey("acct1", "c2c", "openid1", "")
	if key != "acct1:c2c:openid1" {
		t.Errorf("expected 'acct1:c2c:openid1', got '%s'", key)
	}

	// group user key
	key = makeUserKey("acct1", "group", "openid1", "grp1")
	if key != "acct1:group:openid1:grp1" {
		t.Errorf("expected 'acct1:group:openid1:grp1', got '%s'", key)
	}
}

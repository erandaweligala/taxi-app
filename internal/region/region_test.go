package region

import "testing"

func TestKeyShardsByCity(t *testing.T) {
	// Coordinates in different cities must land on different shards; nearby
	// points in the same city must share one.
	lagos := Key(6.5244, 3.3792)
	nairobi := Key(-1.2921, 36.8219)
	lagosNearby := Key(6.5250, 3.3800)

	if lagos == nairobi {
		t.Fatalf("expected Lagos and Nairobi on different shards, both %q", lagos)
	}
	if lagos != lagosNearby {
		t.Fatalf("expected nearby Lagos points on same shard: %q vs %q", lagos, lagosNearby)
	}
}

func TestNeighborsIncludesOwnRegion(t *testing.T) {
	lat, lng := 6.5244, 3.3792
	ns := Neighbors(lat, lng)
	if len(ns) == 0 || ns[0] != Key(lat, lng) {
		t.Fatalf("neighbors must start with own region, got %v", ns)
	}
}

func TestGeohashKnownValue(t *testing.T) {
	// Standard geohash reference point.
	if got := Geohash(57.64911, 10.40744, 11); got != "u4pruydqqvj" {
		t.Fatalf("geohash mismatch: got %q", got)
	}
}

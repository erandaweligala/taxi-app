package surge

import "testing"

func TestFactor(t *testing.T) {
	cases := []struct {
		name           string
		demand, supply int64
		want           float64
	}{
		{"balanced", 5, 10, minSurge},
		{"parity", 10, 10, minSurge},
		{"mild surge", 15, 10, 1.5},
		{"capped", 1000, 10, maxSurge},
		{"no supply with demand", 3, 0, maxSurge},
		{"idle region", 0, 0, minSurge},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := factor(c.demand, c.supply); got != c.want {
				t.Fatalf("factor(%d,%d)=%v want %v", c.demand, c.supply, got, c.want)
			}
		})
	}
}

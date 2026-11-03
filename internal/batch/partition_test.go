package batch

import "testing"

func TestPartitionCoversInputExactly(t *testing.T) {
	got, err := Partition(1_000_000, 250_000)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 4 {
		t.Fatalf("got %d ranges, want 4", len(got))
	}
	var next int64
	for i, r := range got {
		if r.Index != i || r.Start != next || r.End <= r.Start {
			t.Fatalf("invalid range at %d: %+v", i, r)
		}
		next = r.End
	}
	if next != 1_000_000 {
		t.Fatalf("covered through %d, want 1000000", next)
	}
}

func TestPartitionKeepsRemainder(t *testing.T) {
	got, err := Partition(10, 4)
	if err != nil {
		t.Fatal(err)
	}
	want := []Range{{Index: 0, Start: 0, End: 4}, {Index: 1, Start: 4, End: 8}, {Index: 2, Start: 8, End: 10}}
	if len(got) != len(want) {
		t.Fatalf("got %#v, want %#v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("range %d = %#v, want %#v", i, got[i], want[i])
		}
	}
}

func TestPartitionRejectsInvalidInput(t *testing.T) {
	for _, tc := range [][2]int64{{0, 1}, {-1, 1}, {1, 0}, {1, -1}} {
		if _, err := Partition(tc[0], tc[1]); err == nil {
			t.Fatalf("Partition(%d, %d) expected error", tc[0], tc[1])
		}
	}
}

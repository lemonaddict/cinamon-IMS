package inventory

import "testing"

func TestNormalize(t *testing.T) {
	ok := MovementIn{Type: "out", Items: []ItemIn{{ProductID: 2, LocationID: 1, Quantity: 3}, {ProductID: 1, LocationID: 1, Quantity: 1}}}
	if err := ok.normalize(); err != nil {
		t.Fatal(err)
	}
	if ok.Items[0].ProductID != 1 || ok.Items[0].Direction != "out" {
		t.Fatalf("items should be sorted with direction forced: %+v", ok.Items)
	}
	bad := []MovementIn{
		{Type: "in"},
		{Type: "nope", Items: []ItemIn{{Quantity: 1}}},
		{Type: "in", Items: []ItemIn{{Quantity: 0}}},
		{Type: "adjustment", Items: []ItemIn{{Quantity: 1}}},
	}
	for i, m := range bad {
		if m.normalize() == nil {
			t.Errorf("case %d should be rejected", i)
		}
	}
}

package dynres

import "testing"

func TestEventTypeIs(t *testing.T) {
	s := EventTypeIs("OrderUpdated")
	if !s(Conflict{EventType: "OrderUpdated"}) { t.Fatal("expected match") }
	if s(Conflict{EventType: "UserCreated"}) { t.Fatal("did not expect match") }
}

func TestAnyFieldIn(t *testing.T) {
	s := AnyFieldIn("qty","price")
	if !s(Conflict{ChangedFields: []string{"price"}}) { t.Fatal("expected match") }
	if s(Conflict{ChangedFields: []string{"notes"}}) { t.Fatal("did not expect match") }
	if s(Conflict{}) { t.Fatal("did not expect match on empty") }
}

func TestMetadataEq(t *testing.T) {
	s := MetadataEq("role","admin")
	if !s(Conflict{Metadata: map[string]any{"role":"admin"}}) { t.Fatal("expected match") }
	if s(Conflict{Metadata: map[string]any{"role":"user"}}) { t.Fatal("did not expect match") }
	if s(Conflict{Metadata: nil}) { t.Fatal("did not expect match") }
}

func TestCombinators(t *testing.T) {
	isOrder := EventTypeIs("Order")
	hasQty := AnyFieldIn("qty")
	and := And(isOrder, hasQty)
	or := Or(isOrder, hasQty)
	notOrder := Not(isOrder)

	if !and(Conflict{EventType:"Order", ChangedFields: []string{"qty"}}) { t.Fatal("and expected true") }
	if and(Conflict{EventType:"Order"}) { t.Fatal("and expected false") }
	if !or(Conflict{EventType:"User", ChangedFields: []string{"qty"}}) { t.Fatal("or expected true") }
	if or(Conflict{EventType:"User"}) { t.Fatal("or expected false") }
	if !notOrder(Conflict{EventType:"User"}) { t.Fatal("not expected true") }
}


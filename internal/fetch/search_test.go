package fetch

import "testing"

func TestDataAddressIsNotASearch(t *testing.T) {
	if _, ok := SearchQuery("data:text/plain,hello world"); ok {
		t.Error("a data address with a space was taken for a search")
	}
	if u, err := Normalize("data:text/plain;base64,aGk="); err != nil || u.Scheme != "data" {
		t.Errorf("Normalize = %v, %v", u, err)
	}
}

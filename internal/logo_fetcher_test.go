package internal

import (
	"os"
	"testing"

	"golang.org/x/net/html"
)

func TestFindIcons(t *testing.T) {
	f, err := os.Open("testdata/index.html")
	if err != nil {
		t.Fail()
	}
	doc, err := html.Parse(f)
	if err != nil {
		t.Fail()
	}
	logo := findIcons(doc)
	if logo != "/public/img/apple-touch-icon.png" {
		t.Fail()
	}
}

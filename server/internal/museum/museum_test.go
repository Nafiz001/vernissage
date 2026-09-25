package museum

import (
	"encoding/json"
	"testing"
)

func TestCMARecord(t *testing.T) {
	var w cmaWork
	err := json.Unmarshal([]byte(`{
		"id": 94979, "title": "Nathaniel Hurd", "creation_date": "c. 1765",
		"creation_date_earliest": 1760, "creation_date_latest": 1770,
		"culture": ["America"], "technique": "oil on canvas", "type": "Painting",
		"department": "American Painting and Sculpture",
		"description": "<p>Hurd was a <i>silversmith</i> &amp; engraver.</p>",
		"creators": [{"description": "John Singleton Copley (American, 1738–1815)", "role": "artist"}],
		"dimensions": {"framed": {"height": 0.905, "width": 0.78}, "unframed": {"height": 0.762, "width": 0.648}},
		"images": {"web": {"url": "https://cdn/w.jpg"}, "print": {"url": "https://cdn/p.jpg"}},
		"url": "https://clevelandart.org/art/1915.534", "is_highlight": true
	}`), &w)
	if err != nil {
		t.Fatal(err)
	}
	r, ok := w.record()
	if !ok {
		t.Fatal("rejected a good painting")
	}
	if r.Artist != "John Singleton Copley" || r.ArtistBio != "American, 1738–1815" {
		t.Errorf("artist %q bio %q", r.Artist, r.ArtistBio)
	}
	if r.Description != "Hurd was a silversmith & engraver." {
		t.Errorf("description %q", r.Description)
	}
	if *r.WidthCM != 64.8 || *r.HeightCM != 76.2 {
		t.Errorf("size %v x %v, want the unframed 64.8 x 76.2", *r.WidthCM, *r.HeightCM)
	}
	if r.Medium != "Oil on canvas" || r.Kind != "painting" || r.LargeURL != "https://cdn/p.jpg" {
		t.Errorf("record %+v", r)
	}
}

func TestCMAPrintMeasuredWithoutExtent(t *testing.T) {
	var w cmaWork
	json.Unmarshal([]byte(`{"id": 1, "title": "South Wind, Clear Sky", "type": "Print",
		"dimensions": {"No Extent Specified": {"height": 0.256, "width": 0.375}},
		"images": {"web": {"url": "https://cdn/w.jpg"}}}`), &w)
	r, ok := w.record()
	if !ok || r.WidthCM == nil || *r.WidthCM != 37.5 {
		t.Errorf("print size not read: %+v", r)
	}
}

func TestMetRecord(t *testing.T) {
	var o metObject
	json.Unmarshal([]byte(`{
		"objectID": 45434, "isPublicDomain": true,
		"primaryImage": "https://images/original.jpg", "primaryImageSmall": "https://images/web.jpg",
		"title": "Under the Wave off Kanagawa", "culture": "Japan", "period": "Edo period (1615–1868)",
		"artistDisplayName": "Katsushika Hokusai", "objectBeginDate": 1820, "objectEndDate": 1842,
		"classification": "Prints", "objectName": "Woodblock print",
		"measurements": [{"elementName": "Overall", "elementMeasurements": {"Height": 25.7, "Width": 37.9}}]
	}`), &o)
	r, ok := o.record()
	if !ok || r.Kind != "print" || *r.WidthCM != 37.9 || r.Culture != "Japan, Edo period (1615–1868)" {
		t.Errorf("record %+v", r)
	}
	o.IsPublicDomain = false
	if _, ok := o.record(); ok {
		t.Error("accepted a work still in copyright")
	}
	o.IsPublicDomain, o.Classification, o.ObjectName = true, "Arms and Armor", "Helmet"
	if _, ok := o.record(); ok {
		t.Error("accepted a helmet as a picture")
	}
}

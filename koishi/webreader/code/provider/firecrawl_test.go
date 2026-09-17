package provider

import "testing"

func TestRemoveMarkdownImageURLs(t *testing.T) {
	content := "[![Next page](/icons/next.svg)](/articles?page=2)\n\n![Diagram](https://example.com/diagram.png)"
	want := "[Next page](/articles?page=2)\n\nDiagram"

	if got := removeMarkdownImageURLs(content); got != want {
		t.Fatalf("removeMarkdownImageURLs() = %q, want %q", got, want)
	}
}

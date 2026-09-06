package main

// tags is the fixed list of event tags, in display order.
var tags = []string{"concert", "theater", "film", "exhibition"}

// validTag reports whether t is one of the fixed tags.
func validTag(t string) bool {
	for _, x := range tags {
		if x == t {
			return true
		}
	}
	return false
}

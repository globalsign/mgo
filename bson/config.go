package bson

var ignoreOmitempty = false

func SetIgnoreOmitempty(state bool) {
	ignoreOmitempty = state
}

func IgnoreOmitemptyState() bool {
	return ignoreOmitempty
}

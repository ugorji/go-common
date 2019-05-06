// +build safe appengine

package runtimeutil

const safeMode = true

func StringView(v []byte) string {
	return string(v)
}

func BytesView(v string) []byte {
	return []byte(v)
}

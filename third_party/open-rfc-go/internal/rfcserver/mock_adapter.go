package rfcserver

func MockDFIESRow(values map[string]string) ([]byte, error) {
	return dfiesRow(values)
}

func MockTypeHeader(name string, length, count int, unicodeLength byte) ([]byte, error) {
	return x030l(ddicType{name: name, kind: "INTTAB", length: length, fields: make([]ddicField, count)}, unicodeLength)
}

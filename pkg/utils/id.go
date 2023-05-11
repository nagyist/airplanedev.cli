package utils

const DevConfigPrefix = "devcfg"
const DevRunPrefix = "devrun"
const DevResourcePrefix = "devres"

func GenerateID(prefix string) string {
	return prefix + RandomString(10, CharsetLowercaseNumeric)
}

func GenerateDevConfigID(name string) string {
	return DevConfigPrefix + name
}

func GenerateDevResourceID(slug string) string {
	return DevResourcePrefix + slug
}

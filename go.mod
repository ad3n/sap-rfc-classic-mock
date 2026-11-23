module rfc-server

go 1.27.3

require (
	github.com/ad3n/gonwrfc v1.0.11
	github.com/oisee/open-rfc-go v0.0.0
	gopkg.in/yaml.v3 v3.0.1
)

replace github.com/oisee/open-rfc-go => ./third_party/open-rfc-go

module github.com/lguibr/pongo

go 1.19 // Or your specific Go version

require (
	github.com/lguibr/asciiring v0.0.0-20230807134012-b571572dd6ee
	// Added for local replacement and test dependency
	github.com/lguibr/pongo/bollywood v0.0.0
	github.com/stretchr/testify v1.8.4
	golang.org/x/net v0.14.0
)

require (
	github.com/davecgh/go-spew v1.1.1 // indirect
	github.com/pmezard/go-difflib v1.0.0 // indirect
	gopkg.in/yaml.v3 v3.0.1 // indirect
)

replace github.com/lguibr/pongo/bollywood => ./bollywood

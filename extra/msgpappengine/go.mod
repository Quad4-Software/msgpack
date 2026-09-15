module github.com/Quad4-Software/msgpack/v5/extra/msgpappengine

go 1.27.1

replace (
	github.com/Quad4-Software/msgpack/v5 => ../..
	github.com/Quad4-Software/pbt => ../../../pbt
	github.com/Quad4-Software/tagparser => ../../../tagparser
)

require (
	github.com/Quad4-Software/msgpack/v5 v5.0.0-00010101000000-000000000000
	google.golang.org/appengine v1.6.8
)

require (
	github.com/Quad4-Software/tagparser v0.1.3-0.20260518090537-f89b2bf4dade // indirect
	github.com/golang/protobuf v1.5.4 // indirect
	google.golang.org/protobuf v1.36.12 // indirect
)

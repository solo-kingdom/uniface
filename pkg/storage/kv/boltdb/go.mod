module github.com/wii/uniface/pkg/storage/kv/boltdb

go 1.24

require (
	github.com/wii/uniface v0.0.0
	go.etcd.io/bbolt v1.4.3
)

require golang.org/x/sys v0.29.0 // indirect

replace github.com/wii/uniface => ../../../../

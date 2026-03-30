module github.com/solo-kingdom/uniface/pkg/storage/kv/boltdb

go 1.24

require (
	github.com/solo-kingdom/uniface v0.0.0
	go.etcd.io/bbolt v1.4.3
)

require golang.org/x/sys v0.29.0 // indirect

replace github.com/solo-kingdom/uniface => ../../../../

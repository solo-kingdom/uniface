module github.com/solo-kingdom/uniface/lab

go 1.24

require (
	github.com/go-chi/chi/v5 v5.2.1
	github.com/solo-kingdom/uniface v0.0.0
	github.com/solo-kingdom/uniface/pkg/messaging/queue/kafka v0.0.0
	github.com/solo-kingdom/uniface/pkg/messaging/queue/nats v0.0.0
	github.com/solo-kingdom/uniface/pkg/messaging/queue/natsjetstream v0.0.0
	github.com/solo-kingdom/uniface/pkg/messaging/queue/rabbitmq v0.0.0
	github.com/solo-kingdom/uniface/pkg/rpc/governance/config/consul v0.0.0
	github.com/solo-kingdom/uniface/pkg/storage/kv/aerospike v0.0.0
	github.com/solo-kingdom/uniface/pkg/storage/kv/boltdb v0.0.0
	github.com/solo-kingdom/uniface/pkg/storage/kv/redis v0.0.0
	google.golang.org/protobuf v1.36.11
	gopkg.in/yaml.v3 v3.0.1
)

require (
	github.com/IBM/sarama v1.45.1 // indirect
	github.com/aerospike/aerospike-client-go/v7 v7.10.2 // indirect
	github.com/armon/go-metrics v0.4.1 // indirect
	github.com/cespare/xxhash/v2 v2.3.0 // indirect
	github.com/davecgh/go-spew v1.1.2-0.20180830191138-d8f796af33cc // indirect
	github.com/dgryski/go-rendezvous v0.0.0-20200823014737-9f7001d12a5f // indirect
	github.com/eapache/go-resiliency v1.7.0 // indirect
	github.com/eapache/go-xerial-snappy v0.0.0-20230731223053-c322873962e3 // indirect
	github.com/eapache/queue v1.1.0 // indirect
	github.com/fatih/color v1.16.0 // indirect
	github.com/golang/snappy v0.0.4 // indirect
	github.com/hashicorp/consul/api v1.27.0 // indirect
	github.com/hashicorp/errwrap v1.1.0 // indirect
	github.com/hashicorp/go-cleanhttp v0.5.2 // indirect
	github.com/hashicorp/go-hclog v1.5.0 // indirect
	github.com/hashicorp/go-immutable-radix v1.3.1 // indirect
	github.com/hashicorp/go-multierror v1.1.1 // indirect
	github.com/hashicorp/go-rootcerts v1.0.2 // indirect
	github.com/hashicorp/go-uuid v1.0.3 // indirect
	github.com/hashicorp/golang-lru v0.5.4 // indirect
	github.com/hashicorp/serf v0.10.1 // indirect
	github.com/jcmturner/aescts/v2 v2.0.0 // indirect
	github.com/jcmturner/dnsutils/v2 v2.0.0 // indirect
	github.com/jcmturner/gofork v1.7.6 // indirect
	github.com/jcmturner/gokrb5/v8 v8.4.4 // indirect
	github.com/jcmturner/rpc/v2 v2.0.3 // indirect
	github.com/klauspost/compress v1.17.11 // indirect
	github.com/mattn/go-colorable v0.1.13 // indirect
	github.com/mattn/go-isatty v0.0.20 // indirect
	github.com/mitchellh/go-homedir v1.1.0 // indirect
	github.com/mitchellh/mapstructure v1.5.0 // indirect
	github.com/nats-io/nats.go v1.39.1 // indirect
	github.com/nats-io/nkeys v0.4.9 // indirect
	github.com/nats-io/nuid v1.0.1 // indirect
	github.com/pierrec/lz4/v4 v4.1.22 // indirect
	github.com/rabbitmq/amqp091-go v1.10.0 // indirect
	github.com/rcrowley/go-metrics v0.0.0-20201227073835-cf1acfcdf475 // indirect
	github.com/redis/go-redis/v9 v9.18.0 // indirect
	github.com/rogpeppe/go-internal v1.15.0 // indirect
	github.com/yuin/gopher-lua v1.1.1 // indirect
	go.etcd.io/bbolt v1.4.3 // indirect
	go.uber.org/atomic v1.11.0 // indirect
	golang.org/x/crypto v0.33.0 // indirect
	golang.org/x/exp v0.0.0-20230817173708-d852ddb80c63 // indirect
	golang.org/x/net v0.35.0 // indirect
	golang.org/x/sync v0.11.0 // indirect
	golang.org/x/sys v0.30.0 // indirect
	golang.org/x/text v0.22.0 // indirect
	google.golang.org/genproto/googleapis/rpc v0.0.0-20240711142825-46eb208f015d // indirect
	google.golang.org/grpc v1.63.3 // indirect
)

replace github.com/solo-kingdom/uniface => ../

replace github.com/solo-kingdom/uniface/pkg/storage/kv/redis => ../pkg/storage/kv/redis

replace github.com/solo-kingdom/uniface/pkg/storage/kv/boltdb => ../pkg/storage/kv/boltdb

replace github.com/solo-kingdom/uniface/pkg/storage/kv/aerospike => ../pkg/storage/kv/aerospike

replace github.com/solo-kingdom/uniface/pkg/rpc/governance/config/consul => ../pkg/rpc/governance/config/consul

replace github.com/solo-kingdom/uniface/pkg/messaging/queue/kafka => ../pkg/messaging/queue/kafka

replace github.com/solo-kingdom/uniface/pkg/messaging/queue/nats => ../pkg/messaging/queue/nats

replace github.com/solo-kingdom/uniface/pkg/messaging/queue/natsjetstream => ../pkg/messaging/queue/natsjetstream

replace github.com/solo-kingdom/uniface/pkg/messaging/queue/rabbitmq => ../pkg/messaging/queue/rabbitmq

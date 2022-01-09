module github.com/korthochain/korthochain

go 1.16

replace github.com/korthochain/korthochain => ./

replace github.com/ethereum/go-ethereum => github.com/ethereum/go-ethereum v1.10.8

require (
	github.com/beevik/ntp v0.3.0
	github.com/bluele/gcache v0.0.2
	github.com/btcsuite/btcutil v1.0.2
	github.com/buaazp/fasthttprouter v0.1.1
	github.com/dgraph-io/badger v1.6.2
	github.com/ethereum/go-ethereum v0.0.0-00010101000000-000000000000
	github.com/fxamacker/cbor/v2 v2.4.0
	github.com/gofrs/uuid v4.2.0+incompatible
	github.com/gogf/gf v1.16.6
	github.com/goinggo/mapstructure v0.0.0-20140717182941-194205d9b4a9
	github.com/golang/protobuf v1.5.2
	github.com/hashicorp/memberlist v0.3.0
	github.com/mr-tron/base58 v1.2.0
	github.com/spf13/viper v1.10.1
	github.com/stretchr/testify v1.7.0
	github.com/syndtr/goleveldb v1.0.1-0.20210305035536-64b5b1c73954
	github.com/valyala/fasthttp v1.31.0
	github.com/whyrusleeping/cbor-gen v0.0.0-20211110122933-f57984553008
	go.uber.org/zap v1.19.1
	golang.org/x/crypto v0.0.0-20211215153901-e495a2d5b3d3
	golang.org/x/time v0.0.0-20211116232009-f0f3c7e86c11
	google.golang.org/grpc v1.43.0
	google.golang.org/protobuf v1.27.1
	gopkg.in/natefinch/lumberjack.v2 v2.0.0
)

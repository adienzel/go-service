module github.com/adienzel/go-service

go 1.23.1

require (
	github.com/valyala/fasthttp v1.57.0
	go.uber.org/zap v1.27.0
)

require (
	github.com/golang/snappy v0.0.3 // indirect
	github.com/hailocab/go-hostpool v0.0.0-20160125115350-e80d13ce29ed // indirect
	gopkg.in/inf.v0 v0.9.1 // indirect
)

require (
	github.com/andybalholm/brotli v1.1.1 // indirect
	github.com/gocql/gocql v1.7.0
	github.com/klauspost/compress v1.17.11 // indirect
	github.com/valyala/bytebufferpool v1.0.0 // indirect
	go.uber.org/multierr v1.10.0 // indirect
)

replace github.com/gocql/gocql => github.com/scylladb/gocql v1.14.4

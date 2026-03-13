module acs-next/tools/nats-cli

go 1.23

require (
	acs-next/api v0.0.0
	github.com/nats-io/nats.go v1.38.0
	github.com/spf13/cobra v1.8.1
	google.golang.org/protobuf v1.36.5
)

require (
	github.com/inconshreveable/mousetrap v1.1.0 // indirect
	github.com/klauspost/compress v1.17.9 // indirect
	github.com/nats-io/nkeys v0.4.9 // indirect
	github.com/nats-io/nuid v1.0.1 // indirect
	github.com/spf13/pflag v1.0.5 // indirect
	golang.org/x/crypto v0.31.0 // indirect
	golang.org/x/sys v0.28.0 // indirect
	golang.org/x/text v0.21.0 // indirect
)

replace acs-next/api => ../../api

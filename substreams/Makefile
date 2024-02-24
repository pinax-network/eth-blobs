ENDPOINT ?= goerli-scdm37b.mar.eosn.io:10016

.PHONY: build
build:
	cargo build --target wasm32-unknown-unknown --release

.PHONY: protogen
protogen:
	substreams protogen ./substreams.yaml --exclude-paths="sf/substreams,google,sf/ethereum,substreams/sink"

.PHONE: package
package: build
	substreams pack substreams.yaml

.PHONY: run
run: build
	substreams run -e $(ENDPOINT) map_blobs -s -10 --plaintext

.PHONY: gui
gui: build
	substreams gui -e $(ENDPOINT) map_blobs -s -10 --plaintext
.PHONY: build test test-unit test-integration clean

build:
	mkdir -p build && cd build && cmake .. && make -j$$(nproc)

test: test-unit

test-unit: build
	cd build && ./nats_client_test

test-integration: build
	NATS_URL=nats://acs-broker.acs-next.svc:4222 cd build && ./nats_client_test

clean:
	rm -rf build/

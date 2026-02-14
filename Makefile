MAKEFILE_DIR := $(shell dirname $(realpath $(firstword $(MAKEFILE_LIST))))

APP_VERSION := $(shell git describe --tags --always --dirty)
ifndef APP_VERSION
$(error APP_VERSION not set (git issue?))
endif

APP_VERSION_NUM := $(shell echo "$(APP_VERSION)" | sed 's/^v//')

LD_FLAGS := "-X 'main.AppVersion=$(APP_VERSION_NUM)'"

GOOS := linux

DEB_ROOT := $(MAKEFILE_DIR)/bin/deb/docker-veth-namer_$(APP_VERSION_NUM)-1

.PHONY: \
	$(MAKEFILE_DIR)/bin/docker-veth-namer \
	docker-veth-namer \
	all \
	deb \
	clean \
	test \
	doc \
	install

all: \
	docker-veth-namer \
	doc \
	deb

docker-veth-namer: $(MAKEFILE_DIR)/bin/docker-veth-namer

$(MAKEFILE_DIR)/bin/docker-veth-namer:
	@ mkdir -p $(MAKEFILE_DIR)/bin
	env go build -buildmode=pie -ldflags=$(LD_FLAGS) -o $@ .

test:
	go test -cover -race -count=1 ./...

doc: $(MAKEFILE_DIR)/bin/docker-veth-namer.8.gz

$(MAKEFILE_DIR)/bin/docker-veth-namer.8.gz: $(MAKEFILE_DIR)/doc/docker-veth-namer.8.scd
	scdoc < $< | gzip > $@

clean:
	@ rm $(MAKEFILE_DIR)/bin/docker-veth-namer > /dev/null 2>&1 || true
	@ rm $(MAKEFILE_DIR)/bin/docker-veth-namer.8.gz > /dev/null 2>&1 || true
	@ rm -rf $(MAKEFILE_DIR)/bin/deb > /dev/null 2>&1 || true

install:
	install -m 0755 -D $(MAKEFILE_DIR)/bin/docker-veth-namer /usr/sbin/docker-veth-namer
	install -m 0644 -D -t /etc $(MAKEFILE_DIR)/dist/etc/docker-veth-namer.yml
	install -m 0644 -D -t /lib/systemd/system $(MAKEFILE_DIR)/dist/lib/systemd/system/docker-veth-namer.service
	[ -e $(MAKEFILE_DIR)/bin/docker-veth-namer.8.gz ] && \
		install -m 0644 -D -t /usr/share/man/man8 $(MAKEFILE_DIR)/bin/docker-veth-namer.8.gz
	install -m 0644 -D $(MAKEFILE_DIR)/LICENSE /usr/share/doc/docker-veth-namer/copyright

deb: docker-veth-namer doc
	rm -rf $(MAKEFILE_DIR)/bin/deb

	install -m 0644 -D $(MAKEFILE_DIR)/debian/control.in $(DEB_ROOT)/DEBIAN/control
	sed -i "s/@VERSION@/$(APP_VERSION_NUM)/g" $(DEB_ROOT)/DEBIAN/control
	sed -i "s/@ARCH@/$(shell go env GOARCH)/g" $(DEB_ROOT)/DEBIAN/control
	install -m 0644 -D $(MAKEFILE_DIR)/debian/conffiles $(DEB_ROOT)/DEBIAN/conffiles
	install -m 0755 -D $(MAKEFILE_DIR)/debian/postinst $(DEB_ROOT)/DEBIAN/postinst
	install -m 0755 -D $(MAKEFILE_DIR)/debian/postrm $(DEB_ROOT)/DEBIAN/postrm

	install -m 0755 -D $(MAKEFILE_DIR)/bin/docker-veth-namer $(DEB_ROOT)/usr/sbin/docker-veth-namer
	install -m 0644 -D -t $(DEB_ROOT)/etc $(MAKEFILE_DIR)/dist/etc/docker-veth-namer.yml
	install -m 0644 -D -t $(DEB_ROOT)/lib/systemd/system $(MAKEFILE_DIR)/dist/lib/systemd/system/docker-veth-namer.service
	install -m 0644 -D -t $(DEB_ROOT)/usr/share/man/man8 $(MAKEFILE_DIR)/bin/docker-veth-namer.8.gz
	install -m 0644 -D $(MAKEFILE_DIR)/LICENSE $(DEB_ROOT)/usr/share/doc/docker-veth-namer/copyright

	cd $(MAKEFILE_DIR)/bin/deb && dpkg-deb --build docker-veth-namer_$(APP_VERSION_NUM)-1 && \
		mv docker-veth-namer_$(APP_VERSION_NUM)-1.deb ../docker-veth-namer_$(APP_VERSION_NUM)-1-$(shell go env GOARCH).deb

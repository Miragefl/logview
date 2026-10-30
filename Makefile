VERSION ?= $(shell git describe --tags --always 2>/dev/null || echo "dev")
COMMIT  ?= $(shell git rev-parse --short HEAD 2>/dev/null || echo "none")
DATE    ?= $(shell date -u '+%Y-%m-%dT%H:%M:%SZ')

LDFLAGS = -s -w \
	-X main.version=$(VERSION) \
	-X main.commit=$(COMMIT) \
	-X main.date=$(DATE)

.PHONY: build clean

build:
	go build -ldflags "$(LDFLAGS)" -o logview .

clean:
	rm -f logview

# fview 桌面端（Wails v2，工程根 fview/；需本地 macOS + node）
.PHONY: fview-dev fview-build

fview-dev:
	cd fview && wails dev

fview-build:
	cd fview && wails build

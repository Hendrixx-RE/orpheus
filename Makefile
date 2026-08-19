PREFIX ?= /usr/local
BINDIR ?= $(PREFIX)/bin
BINARY = packichu

.PHONY: all build test clean install uninstall run

all: build

build:
	go build -ldflags="-s -w" -o $(BINARY) .

test:
	go test -v ./...

run: build
	./$(BINARY)

install: build
	install -d $(DESTDIR)$(BINDIR)
	install -m 755 $(BINARY) $(DESTDIR)$(BINDIR)/$(BINARY)

uninstall:
	rm -f $(DESTDIR)$(BINDIR)/$(BINARY)

clean:
	rm -f $(BINARY)
	go clean

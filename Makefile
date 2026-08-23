PREFIX ?= /usr/local
BINDIR ?= $(PREFIX)/bin
APPDIR ?= $(PREFIX)/share/applications
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
	install -d $(DESTDIR)$(APPDIR)
	install -m 644 packichu.desktop $(DESTDIR)$(APPDIR)/packichu.desktop

uninstall:
	rm -f $(DESTDIR)$(BINDIR)/$(BINARY)
	rm -f $(DESTDIR)$(APPDIR)/packichu.desktop

clean:
	rm -f $(BINARY)
	go clean

# Maintainer: Hendrixx <maintainers@packichu.local>
pkgname=packichu
pkgver=0.1.0
pkgrel=1
pkgdesc="A terminal-based package management & AI system inspection dashboard for Arch Linux"
arch=('x86_64' 'aarch64')
url="https://github.com/Hendrixx-RE/packichu"
license=('MIT')
depends=('glibc' 'pacman')
optdepends=(
    'yay: AUR package search, installation, and upgrades'
    'paru: AUR package search, installation, and upgrades'
    'flatpak: Flatpak application management'
)
makedepends=('go')
provides=('packichu')
conflicts=('packichu-git' 'packichu-bin' 'pacseer' 'pacseer-git' 'pacseer-bin')

build() {
    cd "$startdir"
    export CGO_ENABLED=0
    export GOFLAGS="-buildmode=pie -trimpath -modcacherw"
    go build -ldflags="-s -w -X main.version=$pkgver" -o "$srcdir/packichu" .
}

check() {
    cd "$startdir"
    go test ./internal/...
}

package() {
    install -Dm755 "$srcdir/packichu" "$pkgdir/usr/bin/packichu"
    install -Dm644 "$startdir/packichu.desktop" "$pkgdir/usr/share/applications/packichu.desktop"
    install -Dm644 "$startdir/LICENSE" "$pkgdir/usr/share/licenses/$pkgname/LICENSE"
    install -Dm644 "$startdir/README.md" "$pkgdir/usr/share/doc/$pkgname/README.md"
}

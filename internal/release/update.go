package release

import (
	"crypto/ed25519"
	"errors"
)

// Production update feed and pinned ed25519 public key (Nw03fbVL3Yyy1b5YFUZUCJNUa6re558iL0NFjj+aSdU=).
// Keep the corresponding private key (.keys/release-ed25519.key) offline and never commit it.
var (
	updateManifestURL = "https://github.com/vowm3440/ferry/releases/latest/download/manifest-{channel}.json"
	updatePublicKey   = []byte{
		0x37, 0x0d, 0x37, 0x7d, 0xb5, 0x4b, 0xdd, 0x8c,
		0xb2, 0xd5, 0xbe, 0x58, 0x15, 0x46, 0x54, 0x08,
		0x93, 0x54, 0x6b, 0xaa, 0xde, 0xe7, 0x9f, 0x22,
		0x2f, 0x43, 0x45, 0x8e, 0x3f, 0x9a, 0x49, 0xd5,
	}
)

var ErrUpdateUnconfigured = errors.New("当前开发构建未配置生产更新地址与签名公钥，请手动下载安装包")

type UpdateFeed struct {
	ManifestURL string
	PublicKey   ed25519.PublicKey
}

func RequireUpdateFeed() (UpdateFeed, error) {
	if updateManifestURL == "" || len(updatePublicKey) != ed25519.PublicKeySize {
		return UpdateFeed{}, ErrUpdateUnconfigured
	}
	return UpdateFeed{
		ManifestURL: updateManifestURL,
		PublicKey:   append(ed25519.PublicKey(nil), updatePublicKey...),
	}, nil
}

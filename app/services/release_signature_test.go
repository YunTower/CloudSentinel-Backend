package services

import (
	"os"
	"path/filepath"
	"testing"
)

const releaseSignatureFixture = `-----BEGIN PGP SIGNATURE-----

iLEEABMJADkWIQQquCU2V3wKn1I0cCLPYssj6yHT0AUCal9dhBsUgAAAAAAEAA5t
YW51MiwyLjUrMS4xMiwyLDEACgkQz2LLI+sh09COoAF+PCLbUPquEAniNYT5kAjs
FKRqxqfL3/F8JJK23GcRdchpjvyhXDM/Gb049UtYEPrKAX9DSlhnbaFqfY9LNk2r
770INluPoLw3w/VbNdquVAStdNc2ECAbpqkfOg2QiXo/Vuc=
=MejA
-----END PGP SIGNATURE-----
`

const releaseChecksumFixture = "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef  release.tar.gz\n"

func TestVerifyReleaseSignature(t *testing.T) {
	dir := t.TempDir()
	signedPath := filepath.Join(dir, "release.sha256")
	signaturePath := filepath.Join(dir, "release.sha256.asc")
	if err := os.WriteFile(signedPath, []byte(releaseChecksumFixture), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(signaturePath, []byte(releaseSignatureFixture), 0600); err != nil {
		t.Fatal(err)
	}

	if err := VerifyReleaseSignature(signedPath, signaturePath); err != nil {
		t.Fatalf("应接受 YunTower 发布签名: %v", err)
	}

	if err := os.WriteFile(signedPath, []byte("tampered\n"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := VerifyReleaseSignature(signedPath, signaturePath); err == nil {
		t.Fatal("篡改后的清单必须被拒绝")
	}
}

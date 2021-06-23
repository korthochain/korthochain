package ed25519

import (
	"crypto/rand"
	"testing"
)

func BenchmarkED25519Sign(b *testing.B) {
	signer := &ED25519Signer{}
	for i := 0; i < b.N; i++ {
		b.StopTimer()
		msg := make([]byte, 32)
		_, _ = rand.Read(msg)
		priv, _ := signer.Generate()

		b.StartTimer()
		signer.Sign(priv, msg)
	}

}

func BenchmarkEd25519Verify(b *testing.B) {
	signer := &ED25519Signer{}
	for i := 0; i < b.N; i++ {
		b.StopTimer()
		msg := make([]byte, 32)
		_, _ = rand.Read(msg)

		priv, _ := signer.Generate()
		pk, _ := signer.ToPublic(priv)
		sig, _ := signer.Sign(priv, msg)

		b.StartTimer()
		signer.Verify(sig, pk, msg)
	}

}

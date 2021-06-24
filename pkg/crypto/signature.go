package crypto

type SigType int

const (
	ED25519 SigType = iota
)

type Signature struct {
	SigType SigType
	Data    []byte
}

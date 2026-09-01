package tokencrypto

import "context"

type Cipher interface {
	Encrypt(context.Context, string, []byte) ([]byte, error)
	Decrypt(context.Context, string, []byte) ([]byte, error)
}

type Plaintext struct{}

func (Plaintext) Encrypt(_ context.Context, _ string, value []byte) ([]byte, error) {
	return append([]byte(nil), value...), nil
}

func (Plaintext) Decrypt(_ context.Context, _ string, value []byte) ([]byte, error) {
	return append([]byte(nil), value...), nil
}

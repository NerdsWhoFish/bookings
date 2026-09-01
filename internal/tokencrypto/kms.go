package tokencrypto

import (
	"context"
	"fmt"

	kms "cloud.google.com/go/kms/apiv1"
	"cloud.google.com/go/kms/apiv1/kmspb"
)

type KMS struct {
	client  *kms.KeyManagementClient
	keyName string
}

func NewKMS(client *kms.KeyManagementClient, keyName string) *KMS {
	return &KMS{client: client, keyName: keyName}
}

func (k *KMS) Encrypt(ctx context.Context, contextID string, value []byte) ([]byte, error) {
	response, err := k.client.Encrypt(ctx, &kmspb.EncryptRequest{
		Name:                        k.keyName,
		Plaintext:                   value,
		AdditionalAuthenticatedData: []byte(contextID),
	})
	if err != nil {
		return nil, fmt.Errorf("encrypt token: %w", err)
	}
	return response.Ciphertext, nil
}

func (k *KMS) Decrypt(ctx context.Context, contextID string, value []byte) ([]byte, error) {
	response, err := k.client.Decrypt(ctx, &kmspb.DecryptRequest{
		Name:                        k.keyName,
		Ciphertext:                  value,
		AdditionalAuthenticatedData: []byte(contextID),
	})
	if err != nil {
		return nil, fmt.Errorf("decrypt token: %w", err)
	}
	return response.Plaintext, nil
}

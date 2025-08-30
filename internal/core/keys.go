package core

import (
	"crypto"
	"crypto/ed25519"
	"crypto/rand"
	"horriya/utils"
)

type Keys struct {
	pubkey     ed25519.PublicKey
	privateKey ed25519.PrivateKey
}

func GenerateKey() Keys {
	pub, priv, _ := ed25519.GenerateKey(rand.Reader)

	var identity Keys = Keys{
		pubkey:     pub,
		privateKey: priv,
	}

	return identity
}

func SignMessage(privateKey ed25519.PrivateKey, message string) ([]byte, error) {
	result, err := privateKey.Sign(rand.Reader, []byte(message), crypto.BLAKE2b_256)
	return result, err
}

func VerifyMessage(pub ed25519.PublicKey, message []byte, signature []byte) bool {
	ok := ed25519.Verify(pub, message, signature)
	return ok
}

// Might need to change this function's place
func (identity Keys) WritePost(content string) (Post, error) {
	now := utils.Now()

	post := Post{
		Content:   content,
		Timestamp: now,
		Sender:    identity.pubkey,
		Signature: nil,
	}

	serialized := post.SerializePost()

	signature, err := SignMessage(identity.privateKey, serialized)
	if err != nil {
		return Post{}, err
	}

	post.Signature = signature
	return post, nil
}

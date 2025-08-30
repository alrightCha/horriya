package crypto

import (
	"crypto"
	"crypto/ed25519"
	"crypto/rand"
	"horriya/method"
	"horriya/utils"
	"method"
)

type Identity struct {
	pubkey     ed25519.PublicKey
	privateKey ed25519.PrivateKey
}

func GenerateKey() Identity {
	pub, priv, _ := ed25519.GenerateKey(rand.Reader)

	var identity Identity = Identity{
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

func (identity Identity) WritePost(content string) {
	now := utils.Now()
	var signature []byte

	post := method.Post{
		content:   content,
		timestamp: now,
		identity:  identity.pubkey,
		signature: signature,
	}

	serialized := post.SerializePost()

	inBytes, err = SignMessage(identity.privateKey, serialized)

}

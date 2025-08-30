package core

import (
	"bytes"
	"crypto/ed25519"
	"horriya/utils"
)

type Influence struct {
	Identity         ed25519.PublicKey
	InfluenceCounter uint32
}

type Post struct {
	Content   string
	Timestamp int64
	Sender    ed25519.PublicKey
	Signature []byte
}

func (post *Post) SerializePost() string {
	var buffer bytes.Buffer
	buffer.WriteString(post.Content)
	nowBytes := utils.UintBytesConvert(post.Timestamp)
	buffer.Write(nowBytes)
	buffer.Write(post.Sender)
	return buffer.String()
}

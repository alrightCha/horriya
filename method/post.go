package method

import (
	"bytes"
	"crypto/ed25519"
	"horriya/utils"
)

type Post struct {
	content   string
	timetsamp int64
	sender    ed25519.PublicKey
	signature []byte
}

func (post *Post) SerializePost() string {
	var buffer bytes.Buffer
	buffer.WriteString(post.content)
	now := utils.Now()
	nowBytes := utils.UintBytesConvert(now)
	buffer.Write(nowBytes)
	buffer.Write(post.sender)
	return buffer.String()
}

package settings

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/sha256"
	"encoding/base64"

	"github.com/can1357/gosu/pkg/util"
)

type rpc struct {
	LocalAddress    string   `json:"local_address"`
	RemoteAddresses []string `json:"remote_list"`
	Secret          string   `json:"secret"`
	Seed            string   `json:"seed"`
}

func fastRandom() string {
	buf := util.RandomBytes(32)
	return base64.StdEncoding.EncodeToString(buf)
}

var Rpc = Settings(rpc{
	LocalAddress:    "http://localhost:24511",
	RemoteAddresses: []string{},
	Secret:          fastRandom(),
	Seed:            fastRandom(),
})

func (s *rpc) Addresses() []string {
	return append([]string{s.LocalAddress}, s.RemoteAddresses...)
}
func (s *rpc) CipherKey() []byte {
	if s.Seed == "" {
		return nil
	}
	sha := sha256.New()
	sha.Write([]byte(s.Seed))
	sha.Write([]byte(s.Secret))
	return sha.Sum(nil)
}
func (s *rpc) NewCipher() (cipher.Block, error) {
	key := s.CipherKey()
	if key == nil {
		return nil, nil
	} else {
		return aes.NewCipher(key)
	}
}

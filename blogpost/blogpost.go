package blogpost

import (
	"bytes"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/binary"
	"encoding/gob"
	"encoding/json"
	"errors"
	"io"
	"time"
)

type BlogPost struct {
	Title   string
	Date    time.Time
	Content string
}

type PostPush struct {
	IV      []byte
	Content []byte
}

type PostContent struct {
	Name string
	BP   BlogPost
	Hash []byte
}

func DecodePostPush(nbpp *PostPush, seed int64, passbytes []byte) (PostContent, error) {
	var nbpc PostContent
	bb := bytes.NewBuffer(nbpp.Content)

	//get hash generated
	res := hashIt(seed, passbytes)

	//get our encrypter rolling
	block, err := aes.NewCipher(res)
	if err != nil {
		return nbpc, err
	}
	stream := cipher.NewCFBDecrypter(block, nbpp.IV)
	cryptrdr := cipher.StreamReader{S: stream, R: bb}

	dec := gob.NewDecoder(cryptrdr)
	if err := dec.Decode(&nbpc); err != nil {
		return nbpc, err
	}

	//verify hash
	if !CompareHash(nbpc.BP.hash(), nbpc.Hash) {
		return nbpc, errors.New("Invalid post hash")
	}

	return nbpc, nil
}

func (bp BlogPost) hash() []byte {
	hsh := sha256.New()
	hsh.Write([]byte(bp.Title))
	hsh.Write([]byte(bp.Content))
	hsh.Write([]byte(bp.Date.Format(time.RFC3339Nano)))
	return hsh.Sum(nil)
}

func EncodeBlogPost(seed int64, passbytes []byte, bp BlogPost, name string) (*PostPush, error) {
	//generate the crypto key
	key := hashIt(seed, passbytes)

	//generate the IV
	iv := make([]byte, aes.BlockSize)
	if _, err := io.ReadFull(rand.Reader, iv); err != nil {
		return nil, err
	}

	//hash the postbytes and name
	postHash := bp.hash()

	//generate the struct
	nbp := PostContent{
		Hash: postHash,
		Name: name,
		BP:   bp,
	}
	//get our encrypter rolling
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	stream := cipher.NewCFBEncrypter(block, iv)
	bb := bytes.NewBuffer(nil)
	wtr := cipher.StreamWriter{S: stream, W: bb}

	//encode the blogPost as a gob into the encrypted writer
	enc := gob.NewEncoder(wtr)
	if err := enc.Encode(nbp); err != nil {
		return nil, err
	}
	//the byte buffer now has our encrypted package
	return &PostPush{
		IV:      iv,
		Content: bb.Bytes(),
	}, nil
}

func CompareHash(a, b []byte) bool {
	if len(a) != len(b) || len(a) == 0 || len(b) == 0 {
		return false
	}
	for i := 0; i < len(a); i++ {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func hashIt(seed int64, pass []byte) []byte {
	hsh := sha256.New()
	binary.Write(hsh, binary.LittleEndian, seed)
	hsh.Write([]byte(pass))
	binary.Write(hsh, binary.LittleEndian, seed)
	res := hsh.Sum(nil)
	for i := 0; i < 8; i++ {
		hsh.Write(res)
		res = hsh.Sum(nil)
	}
	return res
}

func WriteBlogPost(wtr io.Writer, bp BlogPost, name string, seed int64, passbytes []byte) error {
	nbpp, err := EncodeBlogPost(seed, passbytes, bp, name)
	if err != nil {
		return err
	}
	jenc := json.NewEncoder(wtr)
	if err := jenc.Encode(nbpp); err != nil {
		return err
	}
	return nil
}

func ReadBlogPost(rdr io.Reader, bp *BlogPost, name *string, seed int64, passbytes []byte) error {
	var nbpp PostPush
	jdec := json.NewDecoder(rdr)
	if err := jdec.Decode(&nbpp); err != nil {
		return err
	}
	nbpc, err := DecodePostPush(&nbpp, seed, passbytes)
	if err != nil {
		return err
	}
	if !CompareHash(nbpc.Hash, nbpc.BP.hash()) {
		return errors.New("Verification failed")
	}
	*name = nbpc.Name
	*bp = nbpc.BP
	return nil
}

package blogpost

import (
	"bytes"
	"crypto/rand"
	"errors"
	"fmt"
	"io"
	mrnd "math/rand"
	"os"
	"testing"
	"time"
)

const (
	testTitle   = `TestingTitle`
	testName    = `ThisIsATest`
	testContent = `
		<this is a test>
		<more testing>
		<stuff and things!>
	`
)

var (
	testSeed      int64
	testPassbytes []byte
	testDate      time.Time
)

func init() {
	testPassbytes = make([]byte, 1025)
	if _, err := io.ReadFull(rand.Reader, testPassbytes); err != nil {
		fmt.Printf("Failed to generate testPassbytes: %v\n", err)
		os.Exit(-1)
	}
	testSeed = mrnd.Int63()
	testDate = time.Now()
}

func genAndTestEncryptedNBPP() (*PostPush, error) {
	bp := BlogPost{
		Title:   testTitle,
		Content: testContent,
		Date:    testDate,
	}
	nbpp, err := EncodeBlogPost(testSeed, testPassbytes, bp, testName)
	if err != nil {
		return nil, err
	}
	if nbpp == nil {
		return nil, errors.New("Nil NBPP")
	}
	if len(nbpp.IV) == 0 || len(nbpp.Content) == 0 {
		return nil, errors.New("PostPush has invalid data")
	}
	return nbpp, nil
}

func TestEncode(t *testing.T) {
	_, err := genAndTestEncryptedNBPP()
	if err != nil {
		t.Fatal(err)
	}
}

func verifyBlogPostContent(nbpc PostContent) error {

	//ensure the data is all good
	if nbpc.Name != testName {
		return errors.New("Bad Name")
	}
	if nbpc.BP.Title != testTitle {
		return errors.New("Bad Title")
	}
	if nbpc.BP.Date != testDate {
		return errors.New("Bad Date")
	}
	if nbpc.BP.Content != testContent {
		return errors.New("Bad Content")
	}
	if !CompareHash(nbpc.Hash, nbpc.BP.hash()) {
		return errors.New("Bad Hash")
	}
	return nil
}

func TestDecode(t *testing.T) {
	nbpp, err := genAndTestEncryptedNBPP()
	if err != nil {
		t.Fatal(err)
	}
	if nbpp == nil {
		t.Fatal("nil nbpp")
	}
	nbpc, err := DecodePostPush(nbpp, testSeed, testPassbytes)
	if err != nil {
		t.Fatal(err)
	}
	if err := verifyBlogPostContent(nbpc); err != nil {
		t.Fatal(err)
	}
}

func writeBP(bb *bytes.Buffer) error {
	bp := BlogPost{
		Title:   testTitle,
		Content: testContent,
		Date:    testDate,
	}
	if err := WriteBlogPost(bb, bp, testName, testSeed, testPassbytes); err != nil {
		return err
	}
	return nil
}

func readBP(bb *bytes.Buffer) (BlogPost, string, error) {
	var bp BlogPost
	var name string
	if err := ReadBlogPost(bb, &bp, &name, testSeed, testPassbytes); err != nil {
		return bp, name, err
	}
	return bp, name, nil
}

func TestWrite(t *testing.T) {
	bb := bytes.NewBuffer(nil)
	if err := writeBP(bb); err != nil {
		t.Fatal(err)
	}
}

func TestRead(t *testing.T) {
	bb := bytes.NewBuffer(nil)
	if err := writeBP(bb); err != nil {
		t.Fatal(err)
	}
	bp, name, err := readBP(bb)
	if err != nil {
		t.Fatal(err)
	}
	if name != testName {
		t.Fatal("BadName")
	}
	if bp.Title != testTitle {
		t.Fatal("Bad Title")
	}
	if bp.Date != testDate {
		t.Fatal("Bad Date")
	}
	if bp.Content != testContent {
		t.Fatal("Bad Content")
	}
}

package simssl

import (
	"bytes"
	"crypto/aes"
	"crypto/cipher"
	"crypto/md5"
	"crypto/sha256"
	"math/rand"
	"strconv"
	"time"
)

/*
SimSsl packet design of Simple ssl
*/
type SimSsl struct {
	/*Content Type
	0x01 client Hello
	0x02 Server Hello
	0x03 client knows server failed
	0x04 server echo eraser
	*/
	ContentType uint8 //Required
	/* Version
	0x01
	*/
	Version uint8 //Required
	/*
		The total length of all simssl packets (bytes)
	*/
	Length uint16 //Required
	/*
		Method 0x01 AES128
	*/
	Method uint8 //required
	/*
		Mode 0x01 ECB
		Mode 0x02 CBC
		Mode 0x03 CTR
		Mode 0x04 CFB
		Mode oxo5 OFB
	*/
	Mode uint8 //required
	/*
		Checksum ToDo
	*/
	CheckSum uint16 //required
	/*
		SHA256 hash of client identification
	*/
	ClientID [32]uint8 //Required
	/*
		SHA256 hash of server identification
		zerospadding when client sends
	*/
	ServerID [32]uint8 //required
	/*
		Timestamp + (60*60*24)
		add one day when key exchange
	*/
	ExpirationTime int64 //optional

	/*
		The initial string for check the validation of EncrptKey
	*/
	RandomInit [32]uint8 //optional
	/*
		Encrypt key
	*/
	EncryptKey [16]uint8 //optianl
}

/*
GenerateClientHello Generate a client Hello Packet
*/
func GenerateClientHello(cid []byte) (SimSsl, error) {
	rand.Seed(time.Now().Unix())
	clientHello := SimSsl{
		ContentType:    0x01,
		Version:        0x01,
		Length:         128, //bytes
		Method:         0x01,
		Mode:           0x02,
		CheckSum:       0x00,
		ClientID:       sha256.Sum256(cid),
		ServerID:       sha256.Sum256([]byte("unknown")),
		ExpirationTime: time.Now().Unix() + 60*60*24,
		RandomInit:     sha256.Sum256([]byte(strconv.FormatUint(rand.Uint64(), 36))),
		EncryptKey:     md5.Sum([]byte(strconv.FormatUint(rand.Uint64(), 36))),
	}
	encryptMessage, err := AesEncrypt(clientHello.RandomInit[:], clientHello.EncryptKey[:])
	if err != nil {
		return SimSsl{}, err
	}
	//encrypt the initial message
	copy(clientHello.RandomInit[:], encryptMessage[:32])
	return clientHello, nil
}

/*
GenerateServerHello Generate a server Hello Packet
*/
func GenerateServerHello(cid [32]byte, sid []byte, randomInit [32]byte, encryptKey [16]byte, timestamp int64) (SimSsl, error) {
	rand.Seed(time.Now().Unix())
	serverHello := SimSsl{
		ContentType:    0x02,
		Version:        0x01,
		Length:         112, //bytes
		Method:         0x01,
		Mode:           0x02,
		CheckSum:       0x00,
		ClientID:       cid,
		ServerID:       sha256.Sum256(sid),
		ExpirationTime: timestamp,
		RandomInit:     randomInit,
	}
	decryptMessage, err := AesDecrypt(serverHello.RandomInit[:], encryptKey[:])
	if err != nil {
		return SimSsl{}, err
	}
	//encrypt the initial message
	copy(serverHello.RandomInit[:], decryptMessage[:32])
	return serverHello, nil
}

/*
GenerateClientFailed Generate a client failed Packet. The client finds the hello message is wrong
*/
func GenerateClientFailed(cid []byte, sid []byte) (SimSsl, error) {
	rand.Seed(time.Now().Unix())
	clientFailed := SimSsl{
		ContentType: 0x03,
		Version:     0x01,
		Length:      72, //bytes
		Method:      0x01,
		Mode:        0x02,
		CheckSum:    0x00,
		ClientID:    sha256.Sum256(cid),
		ServerID:    sha256.Sum256(sid),
	}
	return clientFailed, nil
}

/*
GenerateServerErase Generate a ServerErase Packet. The server replies the client.
*/
func GenerateServerErase(cid [32]byte, sid []byte) (SimSsl, error) {
	rand.Seed(time.Now().Unix())
	serverErase := SimSsl{
		ContentType: 0x04,
		Version:     0x01,
		Length:      72, //bytes
		Method:      0x01,
		Mode:        0x02,
		CheckSum:    0x00,
		ClientID:    cid,
		ServerID:    sha256.Sum256([]byte(sid)),
	}
	return serverErase, nil
}

/*
CheckSum calculate the checksum of the whole packets, the checksum is originall 0.
*/
func CheckSum(packetData []uint8, length uint16) uint16 {
	var acc uint32
	var src uint16
	acc = 0
	counter := 0
	for length > 1 {
		src = uint16(packetData[counter]) << 8
		counter++
		src |= uint16(packetData[counter])
		counter++
		acc += uint32(src)
		length -= 2
	}
	if length > 0 { //奇数
		src = (uint16)(packetData[counter]) << 8
		acc += uint32(src)
	}
	acc = (acc >> 16) + (acc & 0x0000ffff)
	if acc&0xffff0000 != 0 {
		acc = (acc >> 16) + (acc & 0x0000ffff)
	}
	src = uint16(acc)
	return ^src
}

/*
CheckKey check the key and expiration time id
*/
func CheckKey(A, B *SimSsl) bool {
	if A.ClientID != B.ClientID {
		return false
	}
	if A.ExpirationTime != B.ExpirationTime {
		return false
	}
	temp, err := AesDecrypt(A.RandomInit[:], A.EncryptKey[:])
	if err != nil {
		return false
	}
	var originInital [32]byte
	copy(originInital[:], temp[:32])
	if originInital != B.RandomInit {
		return false
	}
	return true
}

/*
AesEncrypt AES decrypt with message and key
*/
func AesEncrypt(origData, key []byte) ([]byte, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	blockSize := block.BlockSize()
	origData = PKCS5Padding(origData, blockSize)
	// origData = ZeroPadding(origData, block.BlockSize())
	blockMode := cipher.NewCBCEncrypter(block, key[:blockSize])
	crypted := make([]byte, len(origData))
	// 根据CryptBlocks方法的说明，如下方式初始化crypted也可以
	// crypted := origData
	blockMode.CryptBlocks(crypted, origData)
	return crypted, nil
}

/*
AesDecrypt AES decrypt with crypted message and key
*/
func AesDecrypt(crypted, key []byte) ([]byte, error) {

	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	blockSize := block.BlockSize()
	blockMode := cipher.NewCBCDecrypter(block, key[:blockSize])
	origData := make([]byte, len(crypted))
	blockMode.CryptBlocks(origData, crypted)

	paddingNum := int(origData[len(origData)-1])
	// no padding
	if int(origData[len(origData)-1]) > block.BlockSize() {
		return origData, nil
	}
	for i := len(origData) - paddingNum; i < paddingNum; i++ {
		if int(origData[i]) != paddingNum {
			return origData, nil
		}
	}
	origData = PKCS5UnPadding(origData)
	return origData, nil
}

/*
PKCS5Padding fill to the block
*/
func PKCS5Padding(ciphertext []byte, blockSize int) []byte {
	padding := blockSize - len(ciphertext)%blockSize
	padtext := bytes.Repeat([]byte{byte(padding)}, padding)
	return append(ciphertext, padtext...)
}

/*
PKCS5UnPadding delete the paddings
*/
func PKCS5UnPadding(origData []byte) []byte {
	length := len(origData)
	unpadding := int(origData[length-1])
	return origData[:(length - unpadding)]
}

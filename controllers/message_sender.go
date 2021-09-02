package controllers

import (
	"bytes"
	"crypto/aes"
	"crypto/cipher"
	"errors"
	"fmt"
	"net/http"
	"sync"

	"github.com/go-logr/logr"
	"k8s.io/apimachinery/pkg/util/json"

	"github.com/kocoler/deployment-operators/api/v1alpha1"
)

type Message struct {
	MessageType int    `json:"message_type"`
	Version     string `json:"version"`
	Status      int    `json:"status"`
	Cause       string `json:"cause"`
}

type WebhookContent struct {
	Data []byte `json:"data"`
}

type MessageSender struct {
	Hosts          []v1alpha1.CustomerEndpoints
	MessagesBuffer []Message // TODO
	ClientPool     sync.Pool // optimized cpu and memory
	Log            logr.Logger
	Initial        bool
}

type SendMessageError struct {
	Host  string
	Cause string
}

func (s *SendMessageError) Error() string {
	return fmt.Sprintf("SendMessageError: %s, %s", s.Host, s.Cause)
}

func (m *MessageSender) SendMessage(message Message) []error {
	if !m.Initial {
		m.MessagesBuffer = append(m.MessagesBuffer, message)
		return []error{}
	}

	log := m.Log.WithValues("controllers", "deploymentoperators-messagesender")

	var errs []error

	fmt.Println("message!!!!", message)

	for _, host := range m.Hosts {
		content, err := json.Marshal(message)
		if err != nil {
			errs = append(errs, &SendMessageError{
				Host:  host.Host,
				Cause: fmt.Sprintf("marshal content failed: %v", err),
			})
			log.Error(err, "marshal content failed")
		}

		fmt.Println("!!!", string(content), content, host.Secret)
		encrypted, err := m.encrypt(content, []byte(host.Secret))
		if err != nil {
			errs = append(errs, &SendMessageError{
				Host:  host.Host,
				Cause: fmt.Sprintf("encrypt content failed: %v", err),
			})
			log.Error(err, "encrypt content failed")
		}
		fmt.Println("content!", encrypted)

		body, err := json.Marshal(WebhookContent{
			Data: encrypted,
		})

		_ = m.send(host.Host, body)
	}

	return errs
}

// encrypt str with AES256-CBC, padding with PKCS7
func (m *MessageSender) encrypt(plaintext []byte, key []byte) ([]byte, error) {
	ivT := make([]byte, aes.BlockSize+len(plaintext))
	// initialization vector
	iv := ivT[:aes.BlockSize]

	// block
	block, err := aes.NewCipher(key)
	if err != nil {
		return []byte{}, err
	}
	blockSize := block.BlockSize()

	// PKCS7 padding
	padding := blockSize - len(plaintext)%blockSize
	padtext := bytes.Repeat([]byte{byte(padding)}, padding)
	plaintext = append(plaintext, padtext...)

	blockMode := cipher.NewCBCEncrypter(block, iv)
	crypted := make([]byte, len(plaintext))
	blockMode.CryptBlocks(crypted, plaintext)

	return (crypted), nil
}

func (m *MessageSender) send(host string, content []byte) error {
	method := "PUT"

	payload := bytes.NewReader(content)

	client := m.ClientPool.Get().(*http.Client)

	req, err := http.NewRequest(method, host, payload)

	if err != nil {
		return err
	}
	req.Header.Add("Content-Type", "application/json")

	res, err := client.Do(req)
	if err != nil {
		return err
	}
	defer func() {
		res.Body.Close()

		m.ClientPool.Put(client)
	}()

	if res.StatusCode == 200 {
		return nil
	}

	err = errors.New("request failed")
	m.Log.Error(errors.New("request failed: "), fmt.Sprintf("%s: %s", host, res.Status))

	return err
}

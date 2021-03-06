package adapters

import (
	"bytes"
	"crypto/tls"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"strconv"

	C "github.com/Dreamacro/clash/constant"

	"github.com/Dreamacro/go-shadowsocks2/socks"
)

// Socks5Adapter is a shadowsocks adapter
type Socks5Adapter struct {
	conn net.Conn
}

// Close is used to close connection
func (ss *Socks5Adapter) Close() {
	ss.conn.Close()
}

func (ss *Socks5Adapter) Conn() net.Conn {
	return ss.conn
}

type Socks5 struct {
	addr           string
	name           string
	user           string
	pass           string
	tls            bool
	skipCertVerify bool
	tlsConfig      *tls.Config
}

type Socks5Option struct {
	Name           string `proxy:"name"`
	Server         string `proxy:"server"`
	Port           int    `proxy:"port"`
	UserName       string `proxy:"username,omitempty"`
	Password       string `proxy:"password,omitempty"`
	TLS            bool   `proxy:"tls,omitempty"`
	SkipCertVerify bool   `proxy:"skip-cert-verify,omitempty"`
}

func (ss *Socks5) Name() string {
	return ss.name
}

func (ss *Socks5) Type() C.AdapterType {
	return C.Socks5
}

func (ss *Socks5) Generator(metadata *C.Metadata) (adapter C.ProxyAdapter, err error) {
	c, err := net.DialTimeout("tcp", ss.addr, tcpTimeout)

	if err == nil && ss.tls {
		cc := tls.Client(c, ss.tlsConfig)
		err = cc.Handshake()
		c = cc
	}

	if err != nil {
		return nil, fmt.Errorf("%s connect error", ss.addr)
	}
	tcpKeepAlive(c)
	if err := ss.shakeHand(metadata, c); err != nil {
		return nil, err
	}
	return &Socks5Adapter{conn: c}, nil
}

func (ss *Socks5) MarshalJSON() ([]byte, error) {
	return json.Marshal(map[string]string{
		"type": ss.Type().String(),
	})
}

func (ss *Socks5) shakeHand(metadata *C.Metadata, rw io.ReadWriter) error {
	buf := make([]byte, socks.MaxAddrLen)
	var err error

	// VER, NMETHODS, METHODS
	if len(ss.user) > 0 {
		_, err = rw.Write([]byte{5, 1, 2})
	} else {
		_, err = rw.Write([]byte{5, 1, 0})
	}
	if err != nil {
		return err
	}

	// VER, METHOD
	if _, err := io.ReadFull(rw, buf[:2]); err != nil {
		return err
	}

	if buf[0] != 5 {
		return errors.New("SOCKS version error")
	}

	if buf[1] == 2 {
		// password protocol version
		authMsg := &bytes.Buffer{}
		authMsg.WriteByte(1)
		authMsg.WriteByte(uint8(len(ss.user)))
		authMsg.WriteString(ss.user)
		authMsg.WriteByte(uint8(len(ss.pass)))
		authMsg.WriteString(ss.pass)

		if _, err := rw.Write(authMsg.Bytes()); err != nil {
			return err
		}

		if _, err := io.ReadFull(rw, buf[:2]); err != nil {
			return err
		}

		if buf[1] != 0 {
			return errors.New("rejected username/password")
		}
	} else if buf[1] != 0 {
		return errors.New("SOCKS need auth")
	}

	// VER, CMD, RSV, ADDR
	if _, err := rw.Write(bytes.Join([][]byte{{5, 1, 0}, serializesSocksAddr(metadata)}, []byte(""))); err != nil {
		return err
	}

	if _, err := io.ReadFull(rw, buf[:10]); err != nil {
		return err
	}

	return nil
}

func NewSocks5(option Socks5Option) *Socks5 {
	var tlsConfig *tls.Config
	if option.TLS {
		tlsConfig = &tls.Config{
			InsecureSkipVerify: option.SkipCertVerify,
			ClientSessionCache: getClientSessionCache(),
			MinVersion:         tls.VersionTLS11,
			MaxVersion:         tls.VersionTLS12,
			ServerName:         option.Server,
		}
	}

	return &Socks5{
		addr:           net.JoinHostPort(option.Server, strconv.Itoa(option.Port)),
		name:           option.Name,
		user:           option.UserName,
		pass:           option.Password,
		tls:            option.TLS,
		skipCertVerify: option.SkipCertVerify,
		tlsConfig:      tlsConfig,
	}
}
